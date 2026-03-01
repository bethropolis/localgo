package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/handlers"
	"github.com/bethropolis/localgo/pkg/server/services"
	"go.uber.org/zap"
)

var testLogger = zap.NewNop().Sugar()

func setupReceiveHandler(t *testing.T, cfg *config.Config) (*handlers.ReceiveHandler, *services.ReceiveService, string) {
	tempDir := t.TempDir()
	if cfg == nil {
		cfg = &config.Config{
			DownloadDir: tempDir,
			AutoAccept:  true,
		}
	} else if cfg.DownloadDir == "" {
		cfg.DownloadDir = tempDir
	}

	receiveService := services.NewReceiveService()
	handler := handlers.NewReceiveHandler(cfg, receiveService, testLogger)
	return handler, receiveService, tempDir
}

func TestPrepareUploadHandlerV2_Success(t *testing.T) {
	handler, _, _ := setupReceiveHandler(t, nil)

	deviceModel := "PC"
	reqDto := model.PrepareUploadRequestDto{
		Info: model.InfoDto{Alias: "TestSender", DeviceModel: &deviceModel},
		Files: map[string]model.FileDto{
			"file1": {ID: "file1", FileName: "test.txt", Size: 10},
		},
	}
	body, _ := json.Marshal(reqDto)

	req, _ := http.NewRequest(http.MethodPost, "/v2/prepare-upload", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.100:12345"
	rr := httptest.NewRecorder()

	handler.PrepareUploadHandlerV2(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var respDto model.PrepareUploadResponseDto
	if err := json.NewDecoder(rr.Body).Decode(&respDto); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if respDto.SessionID == "" {
		t.Errorf("expected session ID to be set")
	}
	if len(respDto.Files) != 1 {
		t.Errorf("expected 1 file in response, got %d", len(respDto.Files))
	}
	if token, ok := respDto.Files["file1"]; !ok || token == "" {
		t.Errorf("expected token for file1")
	}
}

func TestPrepareUploadHandlerV2_PINValidation(t *testing.T) {
	cfg := &config.Config{
		PIN:        "1234",
		AutoAccept: true,
	}
	handler, _, _ := setupReceiveHandler(t, cfg)

	reqDto := model.PrepareUploadRequestDto{
		Files: map[string]model.FileDto{"file1": {ID: "file1", FileName: "test.txt", Size: 10}},
	}
	body, _ := json.Marshal(reqDto)

	tests := []struct {
		name       string
		pin        string
		wantStatus int
	}{
		{"Valid PIN", "1234", http.StatusOK},
		{"Invalid PIN", "9999", http.StatusUnauthorized},
		{"Missing PIN", "", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Recreate receive service to avoid session conflict
			handler, _, _ = setupReceiveHandler(t, cfg)

			req, _ := http.NewRequest(http.MethodPost, "/v2/prepare-upload?pin="+tt.pin, bytes.NewReader(body))
			req.RemoteAddr = "192.168.1.100:12345"
			rr := httptest.NewRecorder()

			handler.PrepareUploadHandlerV2(rr, req)

			if status := rr.Code; status != tt.wantStatus {
				t.Errorf("handler returned wrong status code: got %v want %v", status, tt.wantStatus)
			}
		})
	}
}

func TestPrepareUploadHandlerV2_ConcurrentSessionConflict(t *testing.T) {
	handler, receiveService, _ := setupReceiveHandler(t, nil)

	// Create an active session
	receiveService.CreateSession(model.DeviceInfo{IP: "192.168.1.100"}, map[string]model.FileDto{"f": {ID: "f"}})

	reqDto := model.PrepareUploadRequestDto{
		Files: map[string]model.FileDto{"file1": {ID: "file1", FileName: "test.txt", Size: 10}},
	}
	body, _ := json.Marshal(reqDto)
	req, _ := http.NewRequest(http.MethodPost, "/v2/prepare-upload", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.101:12345"
	rr := httptest.NewRecorder()

	handler.PrepareUploadHandlerV2(rr, req)

	if status := rr.Code; status != http.StatusConflict {
		t.Errorf("handler returned wrong status code for conflict: got %v want %v", status, http.StatusConflict)
	}
}

func TestUploadHandlerV2_PathTraversalRejection(t *testing.T) {
	handler, receiveService, _ := setupReceiveHandler(t, nil)

	// Create an active session with a malicious file name
	files := map[string]model.FileDto{
		"malicious": {ID: "malicious", FileName: "../../../../etc/passwd", Size: 10},
	}
	session, _ := receiveService.CreateSession(model.DeviceInfo{IP: "192.168.1.100"}, files)

	// We need the generated token for the request
	var token string
	for _, f := range session.Files {
		token = f.Token
		break
	}

	req, _ := http.NewRequest(http.MethodPost, "/v2/upload?sessionId="+session.SessionID+"&fileId=malicious&token="+token, strings.NewReader("bad data"))
	req.RemoteAddr = "192.168.1.100:12345" // IP must match
	rr := httptest.NewRecorder()

	handler.UploadHandlerV2(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("expected path traversal attempt to be rejected with 400 Bad Request, got %v", status)
	}
}

func TestUploadHandlerV2_IPMismatch(t *testing.T) {
	handler, receiveService, _ := setupReceiveHandler(t, nil)

	files := map[string]model.FileDto{
		"file1": {ID: "file1", FileName: "test.txt", Size: 10},
	}
	session, _ := receiveService.CreateSession(model.DeviceInfo{IP: "192.168.1.100"}, files)

	var token string
	for _, f := range session.Files {
		token = f.Token
		break
	}

	req, _ := http.NewRequest(http.MethodPost, "/v2/upload?sessionId="+session.SessionID+"&fileId=file1&token="+token, strings.NewReader("some data"))
	req.RemoteAddr = "192.168.1.101:54321" // IP mismatch
	rr := httptest.NewRecorder()

	handler.UploadHandlerV2(rr, req)

	if status := rr.Code; status != http.StatusForbidden {
		t.Errorf("expected IP mismatch to be rejected with 403 Forbidden, got %v", status)
	}
}

func TestUploadHandlerV2_Success(t *testing.T) {
	handler, receiveService, tempDir := setupReceiveHandler(t, nil)

	files := map[string]model.FileDto{
		"file1": {ID: "file1", FileName: "success.txt", Size: 9},
	}
	session, _ := receiveService.CreateSession(model.DeviceInfo{IP: "192.168.1.100"}, files)

	var token string
	for _, f := range session.Files {
		token = f.Token
		break
	}

	fileContent := "test data"
	req, _ := http.NewRequest(http.MethodPost, "/v2/upload?sessionId="+session.SessionID+"&fileId=file1&token="+token, strings.NewReader(fileContent))
	req.RemoteAddr = "192.168.1.100:12345"
	rr := httptest.NewRecorder()

	handler.UploadHandlerV2(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Verify file was written
	writtenContent, err := os.ReadFile(filepath.Join(tempDir, "success.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(writtenContent) != fileContent {
		t.Errorf("file content mismatch: got %s, want %s", string(writtenContent), fileContent)
	}
}

func TestCancelHandler(t *testing.T) {
	handler, receiveService, _ := setupReceiveHandler(t, nil)

	session, _ := receiveService.CreateSession(model.DeviceInfo{IP: "192.168.1.100"}, map[string]model.FileDto{"f": {ID: "f"}})

	req, _ := http.NewRequest(http.MethodPost, "/v2/cancel?sessionId="+session.SessionID, nil)
	rr := httptest.NewRecorder()

	handler.CancelHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if receiveService.GetSession() != nil {
		t.Errorf("expected session to be cancelled/nil")
	}
}
