package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/httputil"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/services"
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

	session := h.sendService.GetSession()
	if session == nil {
		httputil.RespondError(w, http.StatusNotFound, "No active sharing session")
		return
	}

	info := h.config.ToInfoDto()
	info.Download = true

	response := model.ReceiveRequestResponseDto{
		Info:      info,
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

	fileDto, ok := session.Files[fileId]
	if !ok {
		httputil.RespondError(w, http.StatusNotFound, "File not found in session")
		return
	}

	localPath, ok := session.FilePaths[fileId]
	if !ok {
		httputil.RespondError(w, http.StatusInternalServerError, "File path mapping missing")
		return
	}

	file, err := os.Open(localPath)
	if err != nil {
		logrus.Errorf("Failed to open file for download: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to read file")
		return
	}
	defer file.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileDto.FileName))
	w.Header().Set("Content-Type", fileDto.FileType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileDto.Size))
	w.WriteHeader(http.StatusOK)

	_, err = io.Copy(w, file)
	if err != nil {
		logrus.Errorf("Failed to write file to response: %v", err)
	} else {
		logrus.Infof("Successfully sent file: %s", fileDto.FileName)
	}
}
