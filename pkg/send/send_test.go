package send

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/crypto"
	"github.com/bethropolis/localgo/pkg/model"
	"go.uber.org/zap"
)

var testLoggerSend = zap.NewNop().Sugar()

func TestSendFiles_HappyPath(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test.txt")
	fileContent := "hello world"
	if err := os.WriteFile(filePath, []byte(fileContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	sessionID := "test-session-123"
	token := "test-token-456"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/localsend/v2/prepare-upload":
			var req model.PrepareUploadRequestDto
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("failed to decode prepare request: %v", err)
				http.Error(w, "Bad Request", http.StatusBadRequest)
				return
			}

			if len(req.Files) != 1 {
				t.Errorf("expected 1 file, got %d", len(req.Files))
			}

			var fileID string
			for id := range req.Files {
				fileID = id
				break
			}

			resp := model.PrepareUploadResponseDto{
				SessionID: sessionID,
				Files: map[string]string{
					fileID: token,
				},
			}
			json.NewEncoder(w).Encode(resp)

		case "/api/localsend/v2/upload":
			query := r.URL.Query()
			if query.Get("sessionId") != sessionID {
				t.Errorf("wrong session ID: got %s, want %s", query.Get("sessionId"), sessionID)
			}
			if query.Get("token") != token {
				t.Errorf("wrong token: got %s, want %s", query.Get("token"), token)
			}

			body, _ := io.ReadAll(r.Body)
			if string(body) != fileContent {
				t.Errorf("wrong file content: got %s, want %s", string(body), fileContent)
			}

			w.WriteHeader(http.StatusOK)

		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Parse the port from httptest URL
	portStr := strings.Split(server.URL, ":")[2]
	port, _ := strconv.Atoi(portStr)

	// Create a dummy config
	cfg := &config.Config{
		Alias: "Sender",
		SecurityContext: &crypto.StoredSecurityContext{
			CertificateHash: "hash",
		},
	}

	ipStr := strings.Split(strings.TrimPrefix(server.URL, "http://"), ":")[0]

	// Create target device based on test server info
	device := &model.Device{
		IP:       ipStr,
		Port:     port,
		Protocol: model.ProtocolTypeHTTP,
		Alias:    "Receiver",
	}

	// Use background context with a timeout just in case
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := sendToDevice(ctx, cfg, device, []string{filePath}, testLoggerSend)
	if err != nil {
		t.Fatalf("sendToDevice failed: %v", err)
	}
}
