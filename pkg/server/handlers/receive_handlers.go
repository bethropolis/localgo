package handlers

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/clipboard"
	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/history"
	"github.com/bethropolis/localgo/pkg/httputil"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/services"
	"github.com/bethropolis/localgo/pkg/storage"
	"github.com/charmbracelet/huh"
	"go.uber.org/zap"
)

// maxTextSize is the maximum bytes read from a text/plain body before
// falling back to saving as a file (prevents memory exhaustion).
const maxTextSize = 1 * 1024 * 1024 // 1 MB

// ReceiveHandler handles file receiving requests (/prepare-upload, /upload, /cancel).
type ReceiveHandler struct {
	config         *config.Config
	receiveService *services.ReceiveService
	logger         *zap.SugaredLogger
	historyLog     *history.Logger
	promptMutex    sync.Mutex
}

// NewReceiveHandler creates a new ReceiveHandler.
func NewReceiveHandler(cfg *config.Config, receiveService *services.ReceiveService, historyLog *history.Logger, logger *zap.SugaredLogger) *ReceiveHandler {
	return &ReceiveHandler{
		config:         cfg,
		receiveService: receiveService,
		logger:         logger,
		historyLog:     historyLog,
	}
}

// PrepareUploadHandlerV2 handles POST /v2/prepare-upload requests.
func (h *ReceiveHandler) PrepareUploadHandlerV2(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Received /prepare-upload request")
	if r.Method != http.MethodPost {
		httputil.RespondError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	// --- PIN Check ---
	if h.config.PIN != "" {
		pin := r.URL.Query().Get("pin")
		if subtle.ConstantTimeCompare([]byte(pin), []byte(h.config.PIN)) != 1 {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid PIN")
			return
		}
	}

	// --- Basic Session Check ---
	// Concurrent sessions are now supported, so we no longer block if a session exists.

	// --- Decode Request ---
	// Limit request body to prevent memory exhaustion from massive JSON (1 MB limit)
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024*1024))
	var requestDto model.PrepareUploadRequestDto
	err := decoder.Decode(&requestDto)
	if err != nil {
		h.logger.Errorf("Error decoding /prepare-upload request from %s: %v", r.RemoteAddr, err)
		httputil.RespondError(w, http.StatusBadRequest, "Request body malformed")
		return
	}
	defer r.Body.Close()

	if len(requestDto.Files) == 0 {
		httputil.RespondError(w, http.StatusBadRequest, "Request must contain at least one file")
		return
	}

	// --- Check Disk Space ---
	var totalSize int64
	for _, f := range requestDto.Files {
		totalSize += f.Size
	}

	freeSpace, fsErr := storage.CheckFreeSpace(h.config.DownloadDir)
	if fsErr == nil {
		const safetyBuffer = 50 * 1024 * 1024
		if freeSpace < uint64(totalSize)+safetyBuffer {
			h.logger.Warnf("Rejected transfer from %s: Insufficient disk space (Required: %s, Available: %s)",
				requestDto.Info.Alias, cli.FormatBytes(totalSize), cli.FormatBytes(int64(freeSpace)))
			httputil.RespondError(w, http.StatusBadRequest, "Insufficient storage space on receiver")
			return
		}
	}

	h.logger.Infof("PrepareUpload request from %s (%s) for %d files:", requestDto.Info.Alias, r.RemoteAddr, len(requestDto.Files))

	// Extract IP from RemoteAddr
	senderIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	sender := model.DeviceInfo{
		Alias:       requestDto.Info.Alias,
		Version:     requestDto.Info.Version,
		DeviceModel: requestDto.Info.DeviceModel,
		DeviceType:  requestDto.Info.DeviceType,
		Fingerprint: requestDto.Info.Fingerprint,
		IP:          senderIP,
	}

	// --- Interactive Accept/Reject Prompt ---
	if !h.config.AutoAccept {
		h.promptMutex.Lock()
		accepted := h.promptUserForAcceptance(sender, requestDto.Files)
		h.promptMutex.Unlock()

		if !accepted {
			h.logger.Infof("Transfer rejected by user")
			httputil.RespondError(w, http.StatusForbidden, "Rejected") // 403 Forbidden
			return
		}
	}

	// --- Simulate Acceptance & Create Session ---
	session, err := h.receiveService.CreateSession(sender, requestDto.Files)
	if err != nil {
		httputil.RespondError(w, http.StatusConflict, "Blocked by another session") // 409 Conflict
		return
	}

	responseTokens := make(map[string]string)
	for fileID, file := range session.Files {
		responseTokens[fileID] = file.Token
	}

	h.logger.Infof("Created SessionID: %s and File Tokens. Awaiting /upload requests.", session.SessionID)

	// --- Respond ---
	responseDto := model.PrepareUploadResponseDto{
		SessionID: session.SessionID,
		Files:     responseTokens,
	}
	httputil.RespondJSON(w, http.StatusOK, responseDto)
}

// UploadHandlerV2 handles POST /v2/upload requests.
func (h *ReceiveHandler) UploadHandlerV2(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Received /upload request")
	if r.Method != http.MethodPost {
		httputil.RespondError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	// --- Get Query Params ---
	query := r.URL.Query()
	reqSessionId := query.Get("sessionId")
	reqFileId := query.Get("fileId")
	reqToken := query.Get("token")

	if reqSessionId == "" || reqFileId == "" || reqToken == "" {
		httputil.RespondError(w, http.StatusBadRequest, "Missing query parameters (sessionId, fileId, token)")
		return
	}

	// --- Validate Session and Token ---
	session := h.receiveService.GetSessionByID(reqSessionId)
	if session == nil {
		h.logger.Warnf("Invalid sessionId '%s' for /upload", reqSessionId)
		httputil.RespondError(w, http.StatusForbidden, "Invalid session ID") // 403 Forbidden
		return
	}

	// Validate sender IP matches the one from prepare-upload
	reqIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	if reqIP != session.Sender.IP {
		h.logger.Warnf("IP mismatch for /upload: request from %s, expected %s", reqIP, session.Sender.IP)
		httputil.RespondError(w, http.StatusForbidden, fmt.Sprintf("Invalid IP address: %s", reqIP)) // 403 Forbidden
		return
	}

	fileInfo, ok := session.Files[reqFileId]
	if !ok || fileInfo.Token != reqToken {
		h.logger.Warnf("Invalid fileId '%s' or token '%s' for session '%s'", reqFileId, reqToken, reqSessionId)
		httputil.RespondError(w, http.StatusForbidden, "Invalid fileId or token") // 403 Forbidden
		return
	}

	// --- File Saving ---
	rawFileName := fileInfo.Dto.FileName
	destinationPath := storage.ResolveDuplicateFilename(h.config.DownloadDir, rawFileName)

	// Path traversal prevention: ensure the resolved path is still within DownloadDir
	cleanPath := filepath.Clean(destinationPath)
	if !strings.HasPrefix(cleanPath, filepath.Clean(h.config.DownloadDir)+string(filepath.Separator)) &&
		cleanPath != filepath.Clean(h.config.DownloadDir) {
		h.logger.Errorf("Path traversal attempt detected: %s -> %s", rawFileName, cleanPath)
		httputil.RespondError(w, http.StatusBadRequest, "Invalid filename")
		return
	}

	h.logger.Infof("Starting save for file: %s (ID: %s) to %s", fileInfo.Dto.FileName, reqFileId, destinationPath)

	var trackProgress func(int64)
	if !h.config.Quiet && session.Progress != nil {
		trackProgress = session.Progress.AddBar(fileInfo.Dto.FileName, fileInfo.Dto.Size)
	}

	// --- Progress Callback ---
	onProgress := func(bytesWritten int64) {
		if trackProgress != nil {
			trackProgress(bytesWritten)
		}
	}

	// --- Body Size Limit ---
	maxBodySize := h.config.MaxBodySize
	if maxBodySize <= 0 {
		maxBodySize = 100 * 1024 * 1024 * 1024 // 100GB default
	}
	bodyReader := http.MaxBytesReader(w, r.Body, maxBodySize)
	defer r.Body.Close()

	var modified, accessed *string
	if fileInfo.Dto.Metadata != nil {
		modified = fileInfo.Dto.Metadata.Modified
		accessed = fileInfo.Dto.Metadata.Accessed
	}

	// --- Text/Clipboard Handling ---
	// When the incoming transfer is plain text and clipboard is not disabled,
	// try to copy the content directly to the system clipboard instead of writing
	// to disk.  On failure (headless / no display server) fall through to the
	// normal file-save path so the content is never lost.
	if strings.HasPrefix(fileInfo.Dto.FileType, "text/plain") && !h.config.NoClipboard {
		limited := io.LimitReader(bodyReader, maxTextSize+1)
		textBytes, readErr := io.ReadAll(limited)

		if readErr != nil {
			h.logger.Errorf("Error reading text body for clipboard (file %s): %v", fileInfo.Dto.FileName, readErr)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to read text content")
			return
		}

		text := string(textBytes)

		if int64(len(textBytes)) > maxTextSize {
			// Text is too large for clipboard; save to file instead.
			h.logger.Warnf("Text transfer too large for clipboard (%d bytes), saving to file", len(textBytes))
		} else if clipErr := clipboard.Write(text); clipErr == nil {
			// Successfully copied to clipboard.
			preview := text
			if len(preview) > 80 {
				preview = preview[:80] + "…"
			}
			h.logger.Infof("Copied text to clipboard from %s: %q", fileInfo.Dto.FileName, preview)

			// Mark the progress bar as completed since no file write occurs
			onProgress(fileInfo.Dto.Size)

			h.receiveService.RemoveFileFromSession(reqSessionId, reqFileId)
			h.logTransfer(session.Sender.Alias, session.Sender.IP, rawFileName, "<clipboard>", int64(len(textBytes)), fileInfo.Dto.FileType, history.StatusClipboard)
			h.runExecHook("<clipboard>", rawFileName, session.Sender.Alias, session.Sender.IP, int64(len(textBytes)))
			httputil.RespondJSON(w, http.StatusOK, struct{}{})
			return
		} else {
			// Clipboard unavailable — fall back to file.
			h.logger.Warnf("Clipboard unavailable (%v), saving text as file instead", clipErr)
		}

		// Fall-back: save the full stream as a file.
		var combinedReader io.Reader
		if int64(len(textBytes)) > maxTextSize {
			// Re-combine the already-read prefix with the remaining socket stream.
			combinedReader = io.MultiReader(bytes.NewReader(textBytes), bodyReader)
		} else {
			combinedReader = bytes.NewReader(textBytes)
		}
		destinationPath = storage.ResolveDuplicateFilename(h.config.DownloadDir, rawFileName)
		cleanPath = filepath.Clean(destinationPath)
		if !strings.HasPrefix(cleanPath, filepath.Clean(h.config.DownloadDir)+string(filepath.Separator)) &&
			cleanPath != filepath.Clean(h.config.DownloadDir) {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid filename")
			return
		}
		savErr := storage.SaveStreamToFileWithMetadata(
			combinedReader, destinationPath, fileInfo.Dto.Size, modified, accessed, fileInfo.Dto.SHA256, onProgress, h.logger,
		)
		if savErr != nil {
			h.logger.Errorf("Error saving text file %s: %v", fileInfo.Dto.FileName, savErr)
			h.logTransfer(session.Sender.Alias, session.Sender.IP, rawFileName, destinationPath, int64(len(textBytes)), fileInfo.Dto.FileType, history.StatusFailed)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save file")
			return
		}
		h.logger.Infof("Saved text as file: %s", destinationPath)
		h.receiveService.RemoveFileFromSession(reqSessionId, reqFileId)
		h.logTransfer(session.Sender.Alias, session.Sender.IP, rawFileName, destinationPath, int64(len(textBytes)), fileInfo.Dto.FileType, history.StatusReceived)
		h.runExecHook(destinationPath, rawFileName, session.Sender.Alias, session.Sender.IP, int64(len(textBytes)))
		httputil.RespondJSON(w, http.StatusOK, struct{}{})
		return
	}

	err := storage.SaveStreamToFileWithMetadata(bodyReader, destinationPath, fileInfo.Dto.Size, modified, accessed, fileInfo.Dto.SHA256, onProgress, h.logger)

	if err != nil {
		h.logger.Errorf("Error saving file %s (ID: %s): %v", fileInfo.Dto.FileName, reqFileId, err)
		h.logTransfer(session.Sender.Alias, session.Sender.IP, rawFileName, destinationPath, fileInfo.Dto.Size, fileInfo.Dto.FileType, history.StatusFailed)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to save file")
		return
	}

	// --- Success ---
	h.logger.Infof("Finished saving file: %s (ID: %s)", fileInfo.Dto.FileName, reqFileId)

	h.receiveService.RemoveFileFromSession(reqSessionId, reqFileId)

	h.logTransfer(session.Sender.Alias, session.Sender.IP, rawFileName, destinationPath, fileInfo.Dto.Size, fileInfo.Dto.FileType, history.StatusReceived)
	h.runExecHook(destinationPath, rawFileName, session.Sender.Alias, session.Sender.IP, fileInfo.Dto.Size)

	httputil.RespondJSON(w, http.StatusOK, struct{}{})
}

// PrepareUploadHandlerV1 handles POST /v1/prepare-upload requests (older protocol).
func (h *ReceiveHandler) PrepareUploadHandlerV1(w http.ResponseWriter, r *http.Request) {
	// This is a simplified version for V1. It will be removed in the future.
	h.PrepareUploadHandlerV2(w, r)
}

func (h *ReceiveHandler) promptUserForAcceptance(sender model.DeviceInfo, files map[string]model.FileDto) bool {
	if cli.IsContainer() {
		return false
	}

	fileCount := len(files)
	var totalSize int64
	for _, f := range files {
		totalSize += f.Size
	}

	cli.Notify("LocalGo: Incoming Transfer",
		fmt.Sprintf("%s wants to send you %d file(s) (%s)", sender.Alias, fileCount, cli.FormatBytes(totalSize)))

	// Build a structured summary of the incoming files
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("From: %s (IP: %s)\n\nFiles:\n", sender.Alias, sender.IP))

	count := 0
	for _, file := range files {
		if count >= 5 {
			sb.WriteString(fmt.Sprintf("  ... and %d more files\n", fileCount-5))
			break
		}
		isText := strings.HasPrefix(file.FileType, "text/plain")
		if isText {
			preview := ""
			if file.Preview != nil && *file.Preview != "" {
				preview = *file.Preview
				if len(preview) > 50 {
					preview = preview[:50] + "…"
				}
				sb.WriteString(fmt.Sprintf("  %s [Text] %q\n", cli.IconFile, preview))
			} else {
				sb.WriteString(fmt.Sprintf("  %s [Text] %s (%s)\n", cli.IconFile, file.FileName, cli.FormatBytes(file.Size)))
			}
		} else {
			sb.WriteString(fmt.Sprintf("  %s %s (%s)\n", cli.IconFile, file.FileName, cli.FormatBytes(file.Size)))
		}
		count++
	}

	if totalSize > 0 {
		sb.WriteString(fmt.Sprintf("\nTotal Size: %s", cli.FormatBytes(totalSize)))
	}

	var accept bool = true

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Accept Incoming File Transfer?").
				Description(sb.String()).
				Value(&accept).
				Affirmative("Accept").
				Negative("Reject"),
		),
	).WithTheme(huh.ThemeCharm())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := form.RunWithContext(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\n%s Transfer automatically rejected.\n", cli.WarningStyle.Render(cli.IconWarning))
		return false
	}

	return accept
}

func (h *ReceiveHandler) logTransfer(senderAlias, senderIP, fileName, filePath string, size int64, fileType, status string) {
	if h.historyLog == nil {
		return
	}
	entry := history.Entry{
		SenderAlias: senderAlias,
		SenderIP:    senderIP,
		FileName:    fileName,
		FilePath:    filePath,
		FileSize:    size,
		FileType:    fileType,
		Status:      status,
	}
	if err := h.historyLog.Log(entry); err != nil {
		h.logger.Errorf("Failed to log transfer history: %v", err)
	}
}

func (h *ReceiveHandler) runExecHook(filePath, fileName, senderAlias, senderIP string, fileSize int64) {
	if h.config.ExecHook == "" {
		return
	}

	go func() {
		h.logger.Infof("Running exec hook: %s", h.config.ExecHook)
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", h.config.ExecHook)
		} else {
			cmd = exec.Command("sh", "-c", h.config.ExecHook)
		}
		cmd.Env = append(os.Environ(),
			"LOCALGO_FILE="+filePath,
			"LOCALGO_NAME="+fileName,
			fmt.Sprintf("LOCALGO_SIZE=%d", fileSize),
			"LOCALGO_ALIAS="+senderAlias,
			"LOCALGO_IP="+senderIP,
		)
		output, err := cmd.CombinedOutput()
		if err != nil {
			h.logger.Errorf("Exec hook failed: %v, output: %s", err, string(output))
		} else {
			h.logger.Debugf("Exec hook completed, output: %s", string(output))
		}
	}()
}

// CancelHandler handles POST /v2/cancel requests.
func (h *ReceiveHandler) CancelHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Received /cancel request")
	if r.Method != http.MethodPost {
		httputil.RespondError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	reqSessionId := r.URL.Query().Get("sessionId")
	if reqSessionId == "" {
		httputil.RespondError(w, http.StatusBadRequest, "Missing sessionId parameter")
		return
	}

	session := h.receiveService.GetSessionByID(reqSessionId)
	if session != nil {
		h.logger.Infof("Canceling session %s at user request.", reqSessionId)
		h.receiveService.CloseSession(reqSessionId)
		if h.config.OpenDir && !cli.IsContainer() {
			go func() {
				var cmd string
				var args []string
				if runtime.GOOS == "windows" {
					cmd = "explorer.exe"
					args = []string{h.config.DownloadDir}
				} else if runtime.GOOS == "darwin" {
					cmd = "open"
					args = []string{h.config.DownloadDir}
				} else {
					cmd = "xdg-open"
					args = []string{h.config.DownloadDir}
				}
				exec.Command(cmd, args...).Run()
			}()
		}
	} else {
		// Session is already gone (completed or previously cancelled).
		// The LocalSend protocol always sends /cancel as a cleanup step after a
		// successful transfer, so this is the normal post-upload flow — return 200.
		h.logger.Infof("/cancel received for already-closed session %s — treating as success.", reqSessionId)
	}
	httputil.RespondJSON(w, http.StatusOK, struct{}{})
}
