package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/httputil"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/services"
	"github.com/bethropolis/localgo/pkg/storage"
	"github.com/sirupsen/logrus"
)

// ReceiveHandler handles file receiving requests (/prepare-upload, /upload, /cancel).
type ReceiveHandler struct {
	config         *config.Config
	receiveService *services.ReceiveService
}

// NewReceiveHandler creates a new ReceiveHandler.
func NewReceiveHandler(cfg *config.Config, receiveService *services.ReceiveService) *ReceiveHandler {
	return &ReceiveHandler{
		config:         cfg,
		receiveService: receiveService,
	}
}

// PrepareUploadHandlerV2 handles POST /v2/prepare-upload requests.
func (h *ReceiveHandler) PrepareUploadHandlerV2(w http.ResponseWriter, r *http.Request) {
	logrus.Info("Received /prepare-upload request")
	if r.Method != http.MethodPost {
		httputil.RespondError(w, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	// --- PIN Check ---
	if h.config.PIN != "" {
		pin := r.URL.Query().Get("pin")
		if pin != h.config.PIN {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid PIN")
			return
		}
	}

	// --- Basic Session Check ---
	if h.receiveService.GetSession() != nil {
		logrus.Warnf("Blocking /prepare-upload from %s: Session already active (ID: %s)", r.RemoteAddr, h.receiveService.GetSession().SessionID)
		httputil.RespondError(w, http.StatusConflict, "Blocked by another session") // 409 Conflict
		return
	}

	// --- Decode Request ---
	var requestDto model.PrepareUploadRequestDto
	err := json.NewDecoder(r.Body).Decode(&requestDto)
	if err != nil {
		logrus.Errorf("Error decoding /prepare-upload request from %s: %v", r.RemoteAddr, err)
		httputil.RespondError(w, http.StatusBadRequest, "Request body malformed")
		return
	}
	defer r.Body.Close()

	if len(requestDto.Files) == 0 {
		httputil.RespondError(w, http.StatusBadRequest, "Request must contain at least one file")
		return
	}

	logrus.Infof("PrepareUpload request from %s (%s) for %d files:", requestDto.Info.Alias, r.RemoteAddr, len(requestDto.Files))

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
		accepted := h.promptUserForAcceptance(sender, requestDto.Files)
		if !accepted {
			logrus.Infof("Transfer rejected by user")
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

	logrus.Infof("Created SessionID: %s and File Tokens. Awaiting /upload requests.", session.SessionID)

	// --- Respond ---
	responseDto := model.PrepareUploadResponseDto{
		SessionID: session.SessionID,
		Files:     responseTokens,
	}
	httputil.RespondJSON(w, http.StatusOK, responseDto)
}

// UploadHandlerV2 handles POST /v2/upload requests.
func (h *ReceiveHandler) UploadHandlerV2(w http.ResponseWriter, r *http.Request) {
	logrus.Info("Received /upload request")
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
		logrus.Warnf("Invalid sessionId '%s' for /upload", reqSessionId)
		httputil.RespondError(w, http.StatusForbidden, "Invalid session ID") // 403 Forbidden
		return
	}

	// Validate sender IP matches the one from prepare-upload
	reqIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	if reqIP != session.Sender.IP {
		logrus.Warnf("IP mismatch for /upload: request from %s, expected %s", reqIP, session.Sender.IP)
		httputil.RespondError(w, http.StatusForbidden, fmt.Sprintf("Invalid IP address: %s", reqIP)) // 403 Forbidden
		return
	}

	fileInfo, ok := session.Files[reqFileId]
	if !ok || fileInfo.Token != reqToken {
		logrus.Warnf("Invalid fileId '%s' or token '%s' for session '%s'", reqFileId, reqToken, reqSessionId)
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
		logrus.Errorf("Path traversal attempt detected: %s -> %s", rawFileName, cleanPath)
		httputil.RespondError(w, http.StatusBadRequest, "Invalid filename")
		return
	}

	logrus.Infof("Starting save for file: %s (ID: %s) to %s", fileInfo.Dto.FileName, reqFileId, destinationPath)

	// --- Progress Callback ---
	onProgress := func(bytesWritten int64) {
		if bytesWritten%(1024*1024) == 0 || bytesWritten == fileInfo.Dto.Size {
			logrus.Infof("Progress for %s (%s): %d / %d bytes", fileInfo.Dto.FileName, reqFileId, bytesWritten, fileInfo.Dto.Size)
		}
	}

	// --- Body Size Limit ---
	maxBodySize := h.config.MaxBodySize
	if maxBodySize <= 0 {
		maxBodySize = 100 * 1024 * 1024 * 1024 // 100GB default
	}
	bodyReader := http.MaxBytesReader(w, r.Body, maxBodySize)

	var modified, accessed *string
	if fileInfo.Dto.Metadata != nil {
		modified = fileInfo.Dto.Metadata.Modified
		accessed = fileInfo.Dto.Metadata.Accessed
	}

	err := storage.SaveStreamToFileWithMetadata(bodyReader, destinationPath, modified, accessed, onProgress)
	defer r.Body.Close()

	if err != nil {
		logrus.Errorf("Error saving file %s (ID: %s): %v", fileInfo.Dto.FileName, reqFileId, err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to save file")
		return
	}

	// --- Success ---
	logrus.Infof("Finished saving file: %s (ID: %s)", fileInfo.Dto.FileName, reqFileId)

	h.receiveService.RemoveFileFromSession(reqSessionId, reqFileId)

	httputil.RespondJSON(w, http.StatusOK, nil)
}

// PrepareUploadHandlerV1 handles POST /v1/prepare-upload requests (older protocol).
func (h *ReceiveHandler) PrepareUploadHandlerV1(w http.ResponseWriter, r *http.Request) {
	// This is a simplified version for V1. It will be removed in the future.
	h.PrepareUploadHandlerV2(w, r)
}

func (h *ReceiveHandler) promptUserForAcceptance(sender model.DeviceInfo, files map[string]model.FileDto) bool {
	// Output to stderr to avoid interfering with stdout
	fmt.Fprintf(os.Stderr, "\n%s Incoming file transfer %s\n", cli.ColorYellow, cli.ColorReset)
	fmt.Fprintf(os.Stderr, "From: %s%s%s (IP: %s)\n", cli.ColorCyan, sender.Alias, cli.ColorReset, sender.IP)

	var totalSize int64
	fmt.Fprintf(os.Stderr, "Files:\n")
	for _, file := range files {
		fmt.Fprintf(os.Stderr, "  - %s (%s)\n", file.FileName, cli.FormatBytes(file.Size))
		totalSize += file.Size
	}
	fmt.Fprintf(os.Stderr, "Total size: %s\n", cli.FormatBytes(totalSize))

	fmt.Fprintf(os.Stderr, "\nAccept transfer? [Y/n] (auto-rejects in 30s): ")

	// Read with timeout - use context to prevent goroutine leak
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ch := make(chan string, 1)
	go func() {
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
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

// CancelHandler handles POST /v2/cancel requests.
func (h *ReceiveHandler) CancelHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Info("Received /cancel request")
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
		logrus.Infof("Canceling session %s at user request.", reqSessionId)
		h.receiveService.CloseSession()
		httputil.RespondJSON(w, http.StatusOK, nil)
	} else {
		logrus.Warnf("Ignoring /cancel for unknown or mismatched session ID: %s", reqSessionId)
		httputil.RespondError(w, http.StatusNotFound, "Session not found")
	}
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

	return filepath.Join(dir, baseName)
}
