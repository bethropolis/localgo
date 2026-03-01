package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/handlers"
	"github.com/bethropolis/localgo/pkg/server/services"
	"go.uber.org/zap"
)

var testLoggerDownload = zap.NewNop().Sugar()

func setupDownloadHandler(t *testing.T, cfg *config.Config) (*handlers.DownloadHandler, *services.SendService, string) {
	tempDir := t.TempDir()
	if cfg == nil {
		cfg = &config.Config{
			Alias: "SenderDevice",
		}
	}

	sendService := services.NewSendService()
	handler := handlers.NewDownloadHandler(cfg, sendService, testLoggerDownload)
	return handler, sendService, tempDir
}

func TestPrepareDownloadHandler_NoSession(t *testing.T) {
	handler, _, _ := setupDownloadHandler(t, nil)

	req, _ := http.NewRequest(http.MethodPost, "/v2/prepare-download", nil)
	rr := httptest.NewRecorder()

	handler.PrepareDownloadHandler(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
	}
}

func TestPrepareDownloadHandler_Success(t *testing.T) {
	handler, sendService, _ := setupDownloadHandler(t, nil)

	files := map[string]model.FileDto{"file1": {ID: "file1", FileName: "test.txt", Size: 10}}
	filePaths := map[string]string{"file1": "/tmp/test.txt"}
	session, _ := sendService.CreateSession(files, filePaths)

	req, _ := http.NewRequest(http.MethodPost, "/v2/prepare-download", nil)
	rr := httptest.NewRecorder()

	handler.PrepareDownloadHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var respDto model.ReceiveRequestResponseDto
	if err := json.NewDecoder(rr.Body).Decode(&respDto); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if respDto.SessionID != session.SessionID {
		t.Errorf("expected session ID %s, got %s", session.SessionID, respDto.SessionID)
	}
	if len(respDto.Files) != 1 {
		t.Errorf("expected 1 file in response, got %d", len(respDto.Files))
	}
	if !respDto.Info.Download {
		t.Errorf("expected info.download to be true")
	}
}

func TestPrepareDownloadHandler_PINValidation(t *testing.T) {
	cfg := &config.Config{
		PIN: "1234",
	}
	handler, _, _ := setupDownloadHandler(t, cfg)

	tests := []struct {
		name       string
		pin        string
		wantStatus int
	}{
		{"Valid PIN", "1234", http.StatusNotFound}, // Not found because no session is set up, but PIN was accepted
		{"Invalid PIN", "9999", http.StatusUnauthorized},
		{"Missing PIN", "", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, "/v2/prepare-download?pin="+tt.pin, nil)
			rr := httptest.NewRecorder()

			handler.PrepareDownloadHandler(rr, req)

			if status := rr.Code; status != tt.wantStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.wantStatus)
			}
		})
	}
}

func TestDownloadHandler_Success(t *testing.T) {
	handler, sendService, tempDir := setupDownloadHandler(t, nil)

	// Create a dummy file to serve
	filePath := filepath.Join(tempDir, "test_download.txt")
	fileContent := "hello download"
	if err := os.WriteFile(filePath, []byte(fileContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	files := map[string]model.FileDto{"file1": {ID: "file1", FileName: "test_download.txt", Size: int64(len(fileContent))}}
	filePaths := map[string]string{"file1": filePath}
	session, _ := sendService.CreateSession(files, filePaths)

	req, _ := http.NewRequest(http.MethodGet, "/v2/download?sessionId="+session.SessionID+"&fileId=file1", nil)
	rr := httptest.NewRecorder()

	handler.DownloadHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if rr.Body.String() != fileContent {
		t.Errorf("expected body %s, got %s", fileContent, rr.Body.String())
	}
}

func TestDownloadHandler_MissingParams(t *testing.T) {
	handler, _, _ := setupDownloadHandler(t, nil)

	req, _ := http.NewRequest(http.MethodGet, "/v2/download?sessionId=123", nil) // missing fileId
	rr := httptest.NewRecorder()

	handler.DownloadHandler(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestDownloadHandler_InvalidSession(t *testing.T) {
	handler, _, _ := setupDownloadHandler(t, nil)

	req, _ := http.NewRequest(http.MethodGet, "/v2/download?sessionId=invalid&fileId=123", nil)
	rr := httptest.NewRecorder()

	handler.DownloadHandler(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusNotFound)
	}
}
