package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/logging"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/handlers"
	"github.com/bethropolis/localgo/pkg/server/services"
)

var testLoggerDownload = logging.NewQuiet()

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

func TestWebShareHandler_RendersFiles(t *testing.T) {
	handler, sendService, _ := setupDownloadHandler(t, &config.Config{Alias: "Studio-PC"})

	files := map[string]model.FileDto{
		"b": {ID: "b", FileName: "photo.jpg", Size: 2048, FileType: "image/jpeg"},
		"a": {ID: "a", FileName: "notes.txt", Size: 12, FileType: "text/plain"},
	}
	_, err := sendService.CreateSession(files, map[string]string{"a": "/tmp/a", "b": "/tmp/b"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.WebShareHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"LocalGo Share",
		"Studio-PC",
		"notes.txt",
		"photo.jpg",
		"2 files",
		`class="file-icon image"`,
		"no-store",
	} {
		if want == "no-store" {
			if rr.Header().Get("Cache-Control") != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", rr.Header().Get("Cache-Control"))
			}
			continue
		}
		if !strings.Contains(body, want) {
			t.Errorf("response missing %q", want)
		}
	}
	// Sorted by name: notes before photo
	if i, j := strings.Index(body, "notes.txt"), strings.Index(body, "photo.jpg"); i < 0 || j < 0 || i > j {
		t.Errorf("expected notes.txt before photo.jpg in body")
	}
}

func TestWebShareHandler_PINLockAndError(t *testing.T) {
	handler, sendService, _ := setupDownloadHandler(t, &config.Config{Alias: "Phone", PIN: "4242"})
	_, _ = sendService.CreateSession(
		map[string]model.FileDto{"f": {ID: "f", FileName: "secret.pdf", Size: 100}},
		map[string]string{"f": "/tmp/secret.pdf"},
	)

	// Locked without PIN
	rr := httptest.NewRecorder()
	handler.WebShareHandler(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rr.Body.String()
	if !strings.Contains(body, "PIN protected") {
		t.Error("expected PIN lock screen")
	}
	if strings.Contains(body, "secret.pdf") {
		t.Error("file name should be hidden while locked")
	}
	if strings.Contains(body, "Incorrect PIN") {
		t.Error("should not show incorrect PIN when none was submitted")
	}

	// Wrong PIN
	rr = httptest.NewRecorder()
	handler.WebShareHandler(rr, httptest.NewRequest(http.MethodGet, "/?pin=0000", nil))
	if !strings.Contains(rr.Body.String(), "Incorrect PIN") {
		t.Error("expected incorrect PIN message")
	}

	// Correct PIN
	rr = httptest.NewRecorder()
	handler.WebShareHandler(rr, httptest.NewRequest(http.MethodGet, "/?pin=4242", nil))
	if !strings.Contains(rr.Body.String(), "secret.pdf") {
		t.Error("expected unlocked file list")
	}
}
