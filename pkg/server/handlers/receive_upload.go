package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/bethropolis/localgo/pkg/clipboard"
	"github.com/bethropolis/localgo/pkg/history"
	"github.com/bethropolis/localgo/pkg/httputil"
	"github.com/bethropolis/localgo/pkg/server/services"
	"github.com/bethropolis/localgo/pkg/storage"
)

func (h *ReceiveHandler) UploadHandlerV2(w http.ResponseWriter, r *http.Request) {
	if h.shutdownCtx.Err() != nil {
		h.logger.Warn("Rejecting /upload — server is shutting down")
		httputil.RespondError(w, http.StatusServiceUnavailable, "Server shutting down")
		return
	}

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
		displayName := fileInfo.Dto.FileName
		if fileInfo.Dto.Preview != nil && *fileInfo.Dto.Preview != "" {
			preview := *fileInfo.Dto.Preview
			if len(preview) > 20 {
				preview = preview[:20] + "…"
			}
			displayName = preview
		}
		trackProgress = session.Progress.AddBar(displayName, fileInfo.Dto.Size)
	}

	// --- Progress Callback ---
	onProgress := func(bytesWritten int64) {
		if trackProgress != nil {
			trackProgress(bytesWritten)
		}
	}

	// --- Body Size Limit ---
	// Cap body to the declared file size to prevent disk DoS.
	// A peer can't send more bytes than they declared in prepare-upload.
	if fileInfo.Dto.Size < 0 {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid file size")
		return
	}
	bodyReader := io.LimitReader(r.Body, fileInfo.Dto.Size)
	bodyReader = &shutdownAwareReader{Reader: bodyReader, ctx: h.shutdownCtx}
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
			w.WriteHeader(http.StatusOK)
			return
		} else {
			// Clipboard unavailable — fall back to file.
			h.logger.Warnf("Clipboard unavailable (%v), saving text as file instead", clipErr)
		}

		// Fall-back: save the full stream as a file.
		if err := h.saveTextAsFile(session, reqSessionId, reqFileId, rawFileName, bodyReader, textBytes, modified, accessed, onProgress); err != nil {
			if strings.Contains(err.Error(), "invalid filename") {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid filename")
				return
			}
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save file")
			return
		}
		w.WriteHeader(http.StatusOK)
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

	w.WriteHeader(http.StatusOK)
}

// saveTextAsFile saves text content as a file when clipboard is unavailable or text is too large.
// Returns nil on success; caller writes HTTP status.
func (h *ReceiveHandler) saveTextAsFile(session *services.ActiveReceiveSession, reqSessionId, reqFileId, rawFileName string, bodyReader io.Reader, textBytes []byte, modified, accessed *string, onProgress func(int64)) error {
	var combinedReader io.Reader
	if int64(len(textBytes)) > maxTextSize {
		combinedReader = io.MultiReader(bytes.NewReader(textBytes), bodyReader)
	} else {
		combinedReader = bytes.NewReader(textBytes)
	}
	destinationPath := storage.ResolveDuplicateFilename(h.config.DownloadDir, rawFileName)
	cleanPath := filepath.Clean(destinationPath)
	if !strings.HasPrefix(cleanPath, filepath.Clean(h.config.DownloadDir)+string(filepath.Separator)) &&
		cleanPath != filepath.Clean(h.config.DownloadDir) {
		h.logger.Errorf("Path traversal attempt detected in text fallback: %s", rawFileName)
		return fmt.Errorf("invalid filename")
	}
	savErr := storage.SaveStreamToFileWithMetadata(
		combinedReader, destinationPath, int64(len(textBytes)), modified, accessed, nil, onProgress, h.logger,
	)
	if savErr != nil {
		h.logger.Errorf("Error saving text file %s: %v", rawFileName, savErr)
		h.logTransfer(session.Sender.Alias, session.Sender.IP, rawFileName, destinationPath, int64(len(textBytes)), "text/plain", history.StatusFailed)
		return fmt.Errorf("failed to save file: %w", savErr)
	}
	h.logger.Infof("Saved text as file: %s", destinationPath)
	h.receiveService.RemoveFileFromSession(reqSessionId, reqFileId)
	h.logTransfer(session.Sender.Alias, session.Sender.IP, rawFileName, destinationPath, int64(len(textBytes)), "text/plain", history.StatusReceived)
	h.runExecHook(destinationPath, rawFileName, session.Sender.Alias, session.Sender.IP, int64(len(textBytes)))
	return nil
}

// shutdownAwareReader aborts Read when the shutdown context is cancelled,
// allowing in-flight uploads to terminate promptly on Ctrl+C so the server
// shuts down within the graceful timeout instead of hitting deadline exceeded.
type shutdownAwareReader struct {
	io.Reader
	ctx context.Context
}

func (r *shutdownAwareReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}
