
package handlers

import (
	"fmt"
	"net/http"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/httputil"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/services"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// DownloadHandler handles file downloading requests.
type DownloadHandler struct {
	config      *config.Config
	sendService *services.SendService
}

// NewDownloadHandler creates a new DownloadHandler.
func NewDownloadHandler(cfg *config.Config, sendService *services.SendService) *DownloadHandler {
	return &DownloadHandler{
		config:      cfg,
		sendService: sendService,
	}
}

// PrepareDownloadHandler handles POST /v2/prepare-download requests.
func (h *DownloadHandler) PrepareDownloadHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Info("Received /prepare-download request")

	// --- PIN Check ---
	if h.config.PIN != "" {
		pin := r.URL.Query().Get("pin")
		if pin != h.config.PIN {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid PIN")
			return
		}
	}

	// For now, we'll just create a session with a dummy file.
	// In the future, this will be triggered by a `send` command.
	dummyFiles := map[string]model.FileDto{
		uuid.NewString(): {
			ID:       "dummy-file-id",
			FileName: "dummy.txt",
			Size:     12,
			FileType: "text/plain",
		},
	}

	session, err := h.sendService.CreateSession(dummyFiles)
	if err != nil {
		httputil.RespondError(w, http.StatusConflict, "Blocked by another session")
		return
	}

	response := model.ReceiveRequestResponseDto{
		Info:      h.config.ToInfoDto(),
		SessionID: session.SessionID,
		Files:     session.Files,
	}

	httputil.RespondJSON(w, http.StatusOK, response)
}

// DownloadHandler handles GET /v2/download requests.
func (h *DownloadHandler) DownloadHandler(w http.ResponseWriter, r *http.Request) {
	logrus.Info("Received /download request")

	query := r.URL.Query()
	sessionId := query.Get("sessionId")
	fileId := query.Get("fileId")

	if sessionId == "" || fileId == "" {
		httputil.RespondError(w, http.StatusBadRequest, "Missing sessionId or fileId parameter")
		return
	}

	session := h.sendService.GetSessionByID(sessionId)
	if session == nil {
		httputil.RespondError(w, http.StatusNotFound, "Session not found")
		return
	}

	file, ok := session.Files[fileId]
	if !ok {
		httputil.RespondError(w, http.StatusNotFound, "File not found")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", file.FileName))
	w.Header().Set("Content-Type", file.FileType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", file.Size))

	// For now, just send a dummy file.
	// In the future, this will read the actual file from storage.
	fmt.Fprint(w, "Hello, World")
}
