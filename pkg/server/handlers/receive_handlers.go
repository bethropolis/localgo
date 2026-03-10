package handlers

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	"github.com/schollz/progressbar/v3"
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
	destinationPath := resolveDuplicateFilename(h.config.DownloadDir, rawFileName)

	// Path traversal prevention: ensure the resolved path is still within DownloadDir
	cleanPath := filepath.Clean(destinationPath)
	if !strings.HasPrefix(cleanPath, filepath.Clean(h.config.DownloadDir)+string(filepath.Separator)) &&
		cleanPath != filepath.Clean(h.config.DownloadDir) {
		h.logger.Errorf("Path traversal attempt detected: %s -> %s", rawFileName, cleanPath)
		httputil.RespondError(w, http.StatusBadRequest, "Invalid filename")
		return
	}

	h.logger.Infof("Starting save for file: %s (ID: %s) to %s", fileInfo.Dto.FileName, reqFileId, destinationPath)

	var bar *progressbar.ProgressBar
	if !h.config.Quiet {
		desc := fileInfo.Dto.FileName
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		bar = progressbar.NewOptions64(
			fileInfo.Dto.Size,
			progressbar.OptionSetDescription(desc),
			progressbar.OptionSetWriter(os.Stderr),
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(20),
			progressbar.OptionThrottle(65*time.Millisecond),
			progressbar.OptionShowCount(),
			progressbar.OptionOnCompletion(func() {
				fmt.Fprint(os.Stderr, "\n")
			}),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionSetRenderBlankState(true),
		)
	}

	// --- Progress Callback ---
	onProgress := func(bytesWritten int64) {
		if bar != nil {
			bar.Set64(bytesWritten)
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
			h.receiveService.RemoveFileFromSession(reqSessionId, reqFileId)
			h.logTransfer(session.Sender.Alias, session.Sender.IP, rawFileName, "<clipboard>", int64(len(textBytes)), fileInfo.Dto.FileType, history.StatusClipboard)
			h.runExecHook("<clipboard>", rawFileName, session.Sender.Alias, session.Sender.IP, int64(len(textBytes)))
			httputil.RespondJSON(w, http.StatusOK, struct{}{})
			return
		} else {
			// Clipboard unavailable — fall back to file.
			h.logger.Warnf("Clipboard unavailable (%v), saving text as file instead", clipErr)
		}

		// Fall-back: save the already-read bytes as a file.
		destinationPath = resolveDuplicateFilename(h.config.DownloadDir, rawFileName)
		cleanPath = filepath.Clean(destinationPath)
		if !strings.HasPrefix(cleanPath, filepath.Clean(h.config.DownloadDir)+string(filepath.Separator)) &&
			cleanPath != filepath.Clean(h.config.DownloadDir) {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid filename")
			return
		}
		savErr := storage.SaveStreamToFileWithMetadata(
			strings.NewReader(text), destinationPath, modified, accessed, onProgress, h.logger,
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

	err := storage.SaveStreamToFileWithMetadata(bodyReader, destinationPath, modified, accessed, onProgress, h.logger)

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

// stdinReader is used to prevent creating multiple bufio.Readers on os.Stdin
// which can steal bytes from each other.
var stdinReader = bufio.NewReader(os.Stdin)

func (h *ReceiveHandler) promptUserForAcceptance(sender model.DeviceInfo, files map[string]model.FileDto) bool {
	// Output to stderr to avoid interfering with stdout
	fmt.Fprintf(os.Stderr, "\n%s Incoming transfer %s\n", cli.ColorYellow, cli.ColorReset)
	fmt.Fprintf(os.Stderr, "From: %s%s%s (IP: %s)\n", cli.ColorCyan, sender.Alias, cli.ColorReset, sender.IP)

	var totalSize int64
	var hasText bool
	fmt.Fprintf(os.Stderr, "Files:\n")
	for _, file := range files {
		isText := strings.HasPrefix(file.FileType, "text/plain")
		if isText {
			hasText = true
			preview := ""
			if file.Preview != nil && *file.Preview != "" {
				preview = *file.Preview
				if len(preview) > 80 {
					preview = preview[:80] + "…"
				}
				fmt.Fprintf(os.Stderr, "  - [text] %q\n", preview)
			} else {
				fmt.Fprintf(os.Stderr, "  - [text] %s (%s)\n", file.FileName, cli.FormatBytes(file.Size))
			}
		} else {
			fmt.Fprintf(os.Stderr, "  - %s (%s)\n", file.FileName, cli.FormatBytes(file.Size))
		}
		totalSize += file.Size
	}
	if !hasText {
		fmt.Fprintf(os.Stderr, "Total size: %s\n", cli.FormatBytes(totalSize))
	}
	if hasText && !h.config.NoClipboard {
		fmt.Fprintf(os.Stderr, "(text will be copied to clipboard)\n")
	}

	fmt.Fprintf(os.Stderr, "\nAccept transfer? [Y/n] (auto-rejects in 30s): ")

	// Read with timeout - use context to prevent goroutine leak
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch := make(chan string, 1)
	go func() {
		input, _ := stdinReader.ReadString('\n')
		select {
		case ch <- strings.TrimSpace(strings.ToLower(input)):
		case <-ctx.Done():
		}
	}()

	select {
	case input := <-ch:
		return input == "" || input == "y" || input == "yes"
	case <-ctx.Done():
		fmt.Fprintf(os.Stderr, "\nTransfer auto-rejected (timeout).\n")
		return false
	}
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
		cmd := exec.Command("sh", "-c", h.config.ExecHook)
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
	} else {
		// Session is already gone (completed or previously cancelled).
		// The LocalSend protocol always sends /cancel as a cleanup step after a
		// successful transfer, so this is the normal post-upload flow — return 200.
		h.logger.Infof("/cancel received for already-closed session %s — treating as success.", reqSessionId)
	}
	httputil.RespondJSON(w, http.StatusOK, struct{}{})
}

func resolveDuplicateFilename(dir, baseName string) string {
	ext := filepath.Ext(baseName)
	nameWithoutExt := strings.TrimSuffix(baseName, ext)

	candidate := filepath.Join(dir, baseName)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}

	for i := 1; i <= 999; i++ {
		newName := fmt.Sprintf("%s (%d)%s", nameWithoutExt, i, ext)
		candidate = filepath.Join(dir, newName)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}

	// Fallback to avoid silent overwrite if (1) through (999) are all taken
	randomBytes := make([]byte, 3)
	rand.Read(randomBytes)
	newName := fmt.Sprintf("%s_%s%s", nameWithoutExt, hex.EncodeToString(randomBytes), ext)
	return filepath.Join(dir, newName)
}
