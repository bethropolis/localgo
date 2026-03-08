// Package httputil provides HTTP utilities for LocalGo
package httputil

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// logger is an optional package-level logger set via SetLogger.
// Falls back to the global zap logger when nil.
var logger *zap.SugaredLogger

// SetLogger configures the package-level logger used by httputil helpers.
// Call this once at server startup with the same logger used by handlers.
func SetLogger(l *zap.SugaredLogger) {
	logger = l
}

func logError(msg string, err error) {
	if logger != nil {
		logger.Errorw(msg, "error", err)
	} else {
		zap.L().Error(msg, zap.Error(err))
	}
}

// Error represents an error response
type Error struct {
	Error string `json:"error"`
}

// RespondJSON sends a JSON response. The data is marshalled before any headers
// are written so that a marshal failure can still produce a proper 500 response.
func RespondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		logError("Failed to marshal JSON response", err)
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if _, err := w.Write(jsonData); err != nil {
		logError("Failed to write JSON response", err)
	}
}

// RespondError sends an error response
func RespondError(w http.ResponseWriter, statusCode int, message string) {
	RespondJSON(w, statusCode, Error{Error: message})
}

// RespondOK sends an OK response with no content
func RespondOK(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
}
