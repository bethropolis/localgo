package handlers

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"

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
	destinationPath := filepath.Join(h.config.DownloadDir, fileInfo.Dto.FileName) // Example path

	logrus.Infof("Starting save for file: %s (ID: %s) to %s", fileInfo.Dto.FileName, reqFileId, destinationPath)

	// Define progress callback
	onProgress := func(bytesWritten int64) {
		if bytesWritten%(1024*1024) == 0 || bytesWritten == fileInfo.Dto.Size {
			logrus.Infof("Progress for %s (%s): %d / %d bytes", fileInfo.Dto.FileName, reqFileId, bytesWritten, fileInfo.Dto.Size)
		}
	}

	err := storage.SaveStreamToFile(r.Body, destinationPath, onProgress)
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

