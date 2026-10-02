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
	"sync/atomic"
	"testing"
	"time"

	"github.com/bethropolis/localgo/pkg/cli"
	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/crypto"
	"github.com/bethropolis/localgo/pkg/model"
)

// receiverFor builds a test device pointing at an httptest server.
func receiverFor(t *testing.T, server *httptest.Server) *model.Device {
	t.Helper()
	hostPort := strings.TrimPrefix(server.URL, "http://")
	ipStr, portStr, err := splitHostPort(hostPort)
	if err != nil {
		t.Fatalf("bad test server URL %q: %v", server.URL, err)
	}
	port, _ := strconv.Atoi(portStr)
	return &model.Device{IP: ipStr, Port: port, Protocol: model.ProtocolTypeHTTP, Alias: "Receiver"}
}

func splitHostPort(hostPort string) (string, string, error) {
	idx := strings.LastIndex(hostPort, ":")
	if idx < 0 {
		return "", "", io.ErrUnexpectedEOF
	}
	return hostPort[:idx], hostPort[idx+1:], nil
}

func testSendConfig() *config.Config {
	return &config.Config{
		Alias:           "Sender",
		SecurityContext: &crypto.StoredSecurityContext{CertificateHash: "hash"},
	}
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestSendPrepare401_NoPIN_NoPrompt verifies that a 401 PIN challenge fails
// immediately (no TTY in tests) instead of retrying blindly.
func TestSendPrepare401_NoPIN_NoPrompt(t *testing.T) {
	var prepares atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prepares.Add(1)
		http.Error(w, "Invalid PIN", http.StatusUnauthorized)
	}))
	defer server.Close()

	file := writeTempFile(t, "a.txt", "hello")
	device := receiverFor(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := SendToDevice(ctx, testSendConfig(), device, []string{file}, testLoggerSend)
	if err == nil {
		t.Fatal("expected error when receiver requires a PIN that cannot be provided")
	}
	if !strings.Contains(err.Error(), "PIN required") {
		t.Errorf("expected PIN-required error, got %v", err)
	}
	if got := prepares.Load(); got != 1 {
		t.Errorf("expected exactly 1 prepare attempt (no blind retry), got %d", got)
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns what
// was written (used to assert on NDJSON events).
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// TestSendPrepare401_JSONMode_EmitsPinRequired verifies --json surfaces a
// pin_required event (and never prompts) when the receiver demands a PIN.
func TestSendPrepare401_JSONMode_EmitsPinRequired(t *testing.T) {
	cli.SetJSONMode(true)
	defer cli.SetJSONMode(false)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Invalid PIN", http.StatusUnauthorized)
	}))
	defer server.Close()

	file := writeTempFile(t, "a.txt", "hello")
	device := receiverFor(t, server)

	var err error
	out := captureStdout(t, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err = SendToDevice(ctx, testSendConfig(), device, []string{file}, testLoggerSend)
	})
	if err == nil {
		t.Fatal("expected error when receiver requires a PIN")
	}
	if !strings.Contains(out, `"type":"pin_required"`) {
		t.Errorf("expected a pin_required NDJSON event, got stdout: %q", out)
	}
}

// TestSendCancelOnContextCancel verifies an interrupted transfer posts /cancel
// so the receiver frees its session immediately.
func TestSendCancelOnContextCancel(t *testing.T) {
	cancelled := make(chan string, 1)
	uploadStarted := make(chan struct{})
	var startOnce atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/localsend/v2/prepare-upload":
			// Echo back the sender's real file IDs so the upload proceeds.
			var req model.PrepareUploadRequestDto
			json.NewDecoder(r.Body).Decode(&req)
			files := make(map[string]string, len(req.Files))
			for id := range req.Files {
				files[id] = "tok-1"
			}
			body, _ := json.Marshal(model.PrepareUploadResponseDto{
				SessionID: "sess-cancel-1",
				Files:     files,
			})
			w.Write(body)

		case "/api/localsend/v2/upload":
			if startOnce.CompareAndSwap(false, true) {
				close(uploadStarted)
			}
			// Stalled receiver: never answer, but unblock on cleanup so
			// httptest.Server.Close cannot hang the test.
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}

		case "/api/localsend/v2/cancel":
			cancelled <- r.URL.Query().Get("sessionId")
			w.WriteHeader(http.StatusOK)

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	bigFile := writeTempFile(t, "big.bin", strings.Repeat("x", 1<<20))
	device := receiverFor(t, server)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- SendToDevice(ctx, testSendConfig(), device, []string{bigFile}, testLoggerSend)
	}()

	select {
	case <-uploadStarted:
	case <-time.After(10 * time.Second):
		t.Fatal("upload never started")
	}
	cancel() // simulate Ctrl+C / interrupt

	select {
	case sessionID := <-cancelled:
		if sessionID != "sess-cancel-1" {
			t.Errorf("expected /cancel for session sess-cancel-1, got %q", sessionID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("receiver was never told to cancel the session")
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Error("expected SendToDevice to return an error after cancellation")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("SendToDevice did not return after cancellation")
	}
}

// TestSendSuccessNoCancel verifies a successful transfer does NOT post /cancel
// (the session is already gone; the receiver treats it as success anyway).
func TestSendSuccessNoCancel(t *testing.T) {
	var cancelCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/localsend/v2/prepare-upload":
			var req model.PrepareUploadRequestDto
			json.NewDecoder(r.Body).Decode(&req)
			var fileID string
			for id := range req.Files {
				fileID = id
			}
			w.Write([]byte(`{"sessionId":"sess-ok","files":{"` + fileID + `":"tok"}}`))
		case "/api/localsend/v2/upload":
			io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
		case "/api/localsend/v2/cancel":
			cancelCalls.Add(1)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	file := writeTempFile(t, "ok.txt", "hello")
	device := receiverFor(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := SendToDevice(ctx, testSendConfig(), device, []string{file}, testLoggerSend); err != nil {
		t.Fatalf("SendToDevice failed: %v", err)
	}
	if got := cancelCalls.Load(); got != 0 {
		t.Errorf("successful transfer must not post /cancel, got %d calls", got)
	}
}
