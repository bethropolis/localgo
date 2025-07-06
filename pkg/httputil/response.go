// Package httputil provides HTTP utilities for LocalGo
package httputil

import (
	"encoding/json"
	"log"
	"net/http"
)

// Error represents an error response
type Error struct {
	Error string `json:"error"`
}

// RespondJSON sends a JSON response
func RespondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Set status code
	w.WriteHeader(statusCode)

	// Marshal data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Printf("Failed to marshal JSON response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Write response
	if _, err := w.Write(jsonData); err != nil {
		log.Printf("Failed to write JSON response: %v", err)
	}
}

// RespondError sends an error response
func RespondError(w http.ResponseWriter, statusCode int, message string) {
	errorResponse := Error{
		Error: message,
	}

	RespondJSON(w, statusCode, errorResponse)
}

// RespondOK sends an OK response with no content
func RespondOK(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
}
