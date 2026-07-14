package handlers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/bethropolis/localgo/pkg/clipboard"
	"github.com/bethropolis/localgo/pkg/history"
	"github.com/bethropolis/localgo/pkg/httputil"
	"github.com/bethropolis/localgo/pkg/model"
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
	reqIP, _, _ := net.SplitHostPort(r.RemoteAddr)

	if reqSessionId == "" || reqFileId == "" || reqToken == "" {
		httputil.RespondError(w, http.StatusBadRequest, "Missing query parameters (sessionId, fileId, token)")
		return
	}

	// --- Atomic Claim: validates session, IP, fileId, token under mutex ---
	dto, sender, err := h.receiveService.ClaimFile(reqSessionId, reqFileId, reqToken, reqIP)
	if err != nil {
		h.logger.Warnf("/upload claim failed for session=%s file=%s from %s: %v", reqSessionId, reqFileId, reqIP, err)
		switch {
		case errors.Is(err, services.ErrSessionNotFound):
			httputil.RespondError(w, http.StatusForbidden, "Invalid session ID")
		case errors.Is(err, services.ErrIPMismatch):
			httputil.RespondError(w, http.StatusForbidden, fmt.Sprintf("Invalid IP address: %s", reqIP))
		case errors.Is(err, services.ErrInvalidFileToken):
			httputil.RespondError(w, http.StatusForbidden, "Invalid fileId or token")
		case errors.Is(err, services.ErrAlreadyUploading), errors.Is(err, services.ErrAlreadyCompleted):
			httputil.RespondError(w, http.StatusConflict, "File already being uploaded")
		default:
			httputil.RespondError(w, http.StatusForbidden, "Invalid request")
		}
		return
	}

	// --- File Saving ---
	rawFileName := dto.FileName
	destinationPath := storage.ResolveDuplicateFilename(h.config.DownloadDir, rawFileName)

	// Path traversal prevention: ensure the resolved path is still within DownloadDir
	cleanPath := filepath.Clean(destinationPath)
	if !strings.HasPrefix(cleanPath, filepath.Clean(h.config.DownloadDir)+string(filepath.Separator)) &&
		cleanPath != filepath.Clean(h.config.DownloadDir) {
		h.logger.Errorf("Path traversal attempt detected: %s -> %s", rawFileName, cleanPath)
		h.receiveService.FailFile(reqSessionId, reqFileId)
		httputil.RespondError(w, http.StatusBadRequest, "Invalid filename")
		return
	}

	h.logger.Infof("Starting save for file: %s (ID: %s) to %s", dto.FileName, reqFileId, destinationPath)

	var trackProgress func(int64)
	progress := h.receiveService.GetSessionProgress(reqSessionId)
	if !h.config.Quiet && progress != nil {
		displayName := dto.FileName
		if dto.Preview != nil && *dto.Preview != "" {
			preview := *dto.Preview
			if len(preview) > 20 {
				preview = preview[:20] + "…"
			}
			displayName = preview
		}
		trackProgress = progress.AddBar(displayName, dto.Size)
	}

	// --- Progress Callback ---
	onProgress := func(bytesWritten int64) {
		if trackProgress != nil {
			trackProgress(bytesWritten)
		}
	}

	// --- Body Size Limit ---
	// Cap body to the declared file size to prevent disk DoS.
	if dto.Size < 0 {
		h.receiveService.FailFile(reqSessionId, reqFileId)
		httputil.RespondError(w, http.StatusBadRequest, "Invalid file size")
		return
	}
	bodyReader := io.LimitReader(r.Body, dto.Size)
	bodyReader = &shutdownAwareReader{Reader: bodyReader, ctx: h.shutdownCtx}
	defer r.Body.Close()

	var modified, accessed *string
	if dto.Metadata != nil {
		modified = dto.Metadata.Modified
		accessed = dto.Metadata.Accessed
	}

	// --- Text/Clipboard Handling ---
	if strings.HasPrefix(dto.FileType, "text/plain") && !h.config.NoClipboard {
		limited := io.LimitReader(bodyReader, maxTextSize+1)
		textBytes, readErr := io.ReadAll(limited)

		if readErr != nil {
			h.logger.Errorf("Error reading text body for clipboard (file %s): %v", dto.FileName, readErr)
			h.receiveService.FailFile(reqSessionId, reqFileId)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to read text content")
			return
		}

		text := string(textBytes)

		if int64(len(textBytes)) > maxTextSize {
			h.logger.Warnf("Text transfer too large for clipboard (%d bytes), saving to file", len(textBytes))
		} else if clipErr := clipboard.Write(text); clipErr == nil {
			preview := text
			if len(preview) > 80 {
				preview = preview[:80] + "…"
			}
			h.logger.Infof("Copied text to clipboard from %s: %q", dto.FileName, preview)
			onProgress(dto.Size)
			h.receiveService.CompleteFile(reqSessionId, reqFileId)
			h.logTransfer(sender.Alias, sender.IP, rawFileName, "<clipboard>", int64(len(textBytes)), dto.FileType, history.StatusClipboard)
			h.runExecHook("<clipboard>", rawFileName, sender.Alias, sender.IP, int64(len(textBytes)))
			w.WriteHeader(http.StatusOK)
			return
		} else {
			h.logger.Warnf("Clipboard unavailable (%v), saving text as file instead", clipErr)
		}

		// Fall-back: save the full stream as a file.
		if err := h.saveTextAsFileTo(sender, reqSessionId, reqFileId, rawFileName, bodyReader, textBytes, modified, accessed, onProgress); err != nil {
			h.receiveService.FailFile(reqSessionId, reqFileId)
			if strings.Contains(err.Error(), "invalid filename") {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid filename")
				return
			}
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save file")
			return
		}
		h.receiveService.CompleteFile(reqSessionId, reqFileId)
		w.WriteHeader(http.StatusOK)
		return
	}

	// --- Binary File Save ---
	err = storage.SaveStreamToFileWithMetadata(bodyReader, destinationPath, dto.Size, modified, accessed, dto.SHA256, onProgress, h.logger)
	if err != nil {
		h.logger.Errorf("Error saving file %s (ID: %s): %v", dto.FileName, reqFileId, err)
		h.receiveService.FailFile(reqSessionId, reqFileId)
		h.logTransfer(sender.Alias, sender.IP, rawFileName, destinationPath, dto.Size, dto.FileType, history.StatusFailed)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to save file")
		return
	}

	// --- Success ---
	h.logger.Infof("Finished saving file: %s (ID: %s)", dto.FileName, reqFileId)
	h.receiveService.CompleteFile(reqSessionId, reqFileId)
	h.logTransfer(sender.Alias, sender.IP, rawFileName, destinationPath, dto.Size, dto.FileType, history.StatusReceived)
	h.runExecHook(destinationPath, rawFileName, sender.Alias, sender.IP, dto.Size)
	w.WriteHeader(http.StatusOK)
}

// saveTextAsFileTo saves text content as a file when clipboard is unavailable or text is too large.
// Returns nil on success; caller writes HTTP status and calls CompleteFile.
func (h *ReceiveHandler) saveTextAsFileTo(sender model.DeviceInfo, reqSessionId, reqFileId, rawFileName string, bodyReader io.Reader, textBytes []byte, modified, accessed *string, onProgress func(int64)) error {
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
		h.logTransfer(sender.Alias, sender.IP, rawFileName, destinationPath, int64(len(textBytes)), "text/plain", history.StatusFailed)
		return fmt.Errorf("failed to save file: %w", savErr)
	}
	h.logger.Infof("Saved text as file: %s", destinationPath)
	h.logTransfer(sender.Alias, sender.IP, rawFileName, destinationPath, int64(len(textBytes)), "text/plain", history.StatusReceived)
	h.runExecHook(destinationPath, rawFileName, sender.Alias, sender.IP, int64(len(textBytes)))
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
