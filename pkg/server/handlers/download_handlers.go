package handlers

import (
	"crypto/subtle"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/httputil"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/services"
	"go.uber.org/zap"
)

// DownloadHandler handles file downloading requests.
type DownloadHandler struct {
	config      *config.Config
	sendService *services.SendService
	logger      *zap.SugaredLogger
	shutdownFn  func() // optional; set by Server for --once support
}

// SetShutdownFn registers a shutdown callback (used for --once mode).
func (h *DownloadHandler) SetShutdownFn(fn func()) {
	h.shutdownFn = fn
}

const webShareHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>LocalGo File Share</title>
    <style>
        :root { --bg: #0f172a; --card: #1e293b; --accent: #38bdf8; --btn: #0284c7; --text: #f8fafc; }
        body { font-family: system-ui, -apple-system, sans-serif; background: var(--bg); color: var(--text); display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; padding: 1rem; }
        .card { background: var(--card); border-radius: 12px; padding: 2rem; max-width: 480px; width: 100%; box-shadow: 0 10px 25px -5px rgba(0,0,0,0.3); }
        h1 { font-size: 1.5rem; margin: 0 0 0.5rem 0; color: var(--accent); }
        .alias { font-size: 0.875rem; color: #94a3b8; margin-bottom: 1.5rem; }
        .file-list { list-style: none; padding: 0; margin: 0 0 1.5rem 0; }
        .file-item { display: flex; justify-content: space-between; align-items: center; padding: 0.85rem 0; border-bottom: 1px solid #334155; }
        .file-name { font-weight: 500; word-break: break-all; }
        .file-size { font-size: 0.8rem; color: #94a3b8; display: block; margin-top: 0.2rem; }
        .btn { display: inline-block; background: var(--btn); color: white; text-decoration: none; padding: 0.6rem 1.2rem; border-radius: 6px; font-weight: 600; border: none; cursor: pointer; transition: background 0.2s; }
        .btn:hover { background: #0369a1; }
        .pin-form { display: flex; gap: 0.5rem; margin-top: 1rem; }
        .pin-input { flex: 1; padding: 0.6rem; border-radius: 6px; border: 1px solid #334155; background: #0f172a; color: white; }
    </style>
</head>
<body>
    <div class="card">
        <h1>LocalGo File Share</h1>
        <div class="alias">Shared by <strong>{{.Alias}}</strong></div>
        {{if .PinLocked}}
        <p style="color: #f1f5f9;">This share is PIN protected.</p>
        <form method="GET" action="/" class="pin-form">
            <input type="password" name="pin" placeholder="Enter PIN" class="pin-input" required autofocus>
            <button type="submit" class="btn">Unlock</button>
        </form>
        {{else}}
        <ul class="file-list">
            {{range $id, $f := .Files}}
            <li class="file-item">
                <div>
                    <div class="file-name">{{$f.FileName}}</div>
                    <span class="file-size">{{formatBytes $f.Size}}</span>
                </div>
                <a href="/api/localsend/v2/download?sessionId={{$.SessionID}}&fileId={{$id}}{{if $.PIN}}&pin={{$.PIN}}{{end}}" class="btn">Download</a>
            </li>
            {{end}}
        </ul>
        {{end}}
    </div>
</body>
</html>`

// WebShareData holds template data for the web landing page.
type WebShareData struct {
	Alias     string
	SessionID string
	Files     map[string]model.FileDto
	PinLocked bool
	PIN       string
}

// WebShareHandler serves the root web landing page for browser access.
func (h *DownloadHandler) WebShareHandler(w http.ResponseWriter, r *http.Request) {
	session := h.sendService.GetSession()
	if session == nil {
		http.Error(w, "No files are currently being shared.", http.StatusNotFound)
		return
	}

	pinLocked := false
	pin := r.URL.Query().Get("pin")
	if h.config.PIN != "" {
		if subtle.ConstantTimeCompare([]byte(pin), []byte(h.config.PIN)) != 1 {
			pinLocked = true
		}
	}

	funcMap := template.FuncMap{
		"formatBytes": cli.FormatBytes,
	}

	tmpl, err := template.New("webshare").Funcs(funcMap).Parse(webShareHTML)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	alias := h.config.Alias
	if h.config.Private {
		alias = "Anonymous"
	}

	data := WebShareData{
		Alias:     alias,
		SessionID: session.SessionID,
		Files:     session.Files,
		PinLocked: pinLocked,
		PIN:       pin,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
}

// NewDownloadHandler creates a new DownloadHandler.
func NewDownloadHandler(cfg *config.Config, sendService *services.SendService, logger *zap.SugaredLogger) *DownloadHandler {
	return &DownloadHandler{
		config:      cfg,
		sendService: sendService,
		logger:      logger,
	}
}

// PrepareDownloadHandler handles POST /v2/prepare-download requests.
func (h *DownloadHandler) PrepareDownloadHandler(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Received /prepare-download request")

	// --- PIN Check ---
	if h.config.PIN != "" {
		pin := r.URL.Query().Get("pin")
		if subtle.ConstantTimeCompare([]byte(pin), []byte(h.config.PIN)) != 1 {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid PIN")
			return
		}
	}

	var session *services.ActiveSendSession
	if sessionID := r.URL.Query().Get("sessionId"); sessionID != "" {
		session = h.sendService.GetSessionByID(sessionID)
	} else {
		session = h.sendService.GetSession()
	}
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
	h.logger.Info("Received /download request")

	// --- PIN Check ---
	if h.config.PIN != "" {
		pin := r.URL.Query().Get("pin")
		if subtle.ConstantTimeCompare([]byte(pin), []byte(h.config.PIN)) != 1 {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid PIN")
			return
		}
	}

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
		h.logger.Errorf("Failed to open file for download: %v", err)
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
		h.logger.Errorf("Failed to write file to response: %v", err)
	} else {
		h.logger.Infof("Successfully sent file: %s", fileDto.FileName)
		cli.PrintSuccess("Downloaded by %s: %s (%s)", r.RemoteAddr, fileDto.FileName, cli.FormatBytes(fileDto.Size))

		if h.config.ShareOnce {
			go func() {
				time.Sleep(500 * time.Millisecond)
				cli.PrintInfo("Download completed (--once mode). Stopping server...")
				if h.shutdownFn != nil {
					h.shutdownFn()
				}
			}()
		}
	}
}
