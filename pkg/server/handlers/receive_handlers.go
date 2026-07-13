package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"sync"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/history"
	"github.com/bethropolis/localgo/pkg/httputil"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/services"
	"github.com/bethropolis/localgo/pkg/storage"
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
	shutdownCtx    context.Context
}

// NewReceiveHandler creates a new ReceiveHandler.
func NewReceiveHandler(cfg *config.Config, receiveService *services.ReceiveService, historyLog *history.Logger, shutdownCtx context.Context, logger *zap.SugaredLogger) *ReceiveHandler {
	return &ReceiveHandler{
		config:         cfg,
		receiveService: receiveService,
		logger:         logger,
		historyLog:     historyLog,
		shutdownCtx:    shutdownCtx,
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
		h.logger.Info("Received empty file list on prepare-upload, returning 204 Finished")
		w.WriteHeader(http.StatusNoContent)
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

// PrepareUploadHandlerV1 handles POST /v1/prepare-upload requests (older protocol).
func (h *ReceiveHandler) PrepareUploadHandlerV1(w http.ResponseWriter, r *http.Request) {
	h.PrepareUploadHandlerV2(w, r)
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
				} else if _, err := exec.LookPath("xdg-open"); err == nil {
					cmd = "xdg-open"
					args = []string{h.config.DownloadDir}
				} else {
					h.logger.Debugf("xdg-open not found in PATH, skip opening download dir")
					return
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
	w.WriteHeader(http.StatusOK)
}
