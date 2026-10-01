package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bethropolis/localgo/pkg/config"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/handlers"
	"github.com/bethropolis/localgo/pkg/server/services"
)

func TestPendingRegistryDecideRoundTrip(t *testing.T) {
	reg := handlers.NewPendingRegistry()

	id, ch := reg.Add(handlers.PendingTransfer{SenderAlias: "Phone", Files: []handlers.PendingFile{
		{ID: "f1", Name: "a.txt", Size: 3, Type: "text/plain"},
	}})

	if len(reg.List()) != 1 {
		t.Fatalf("expected 1 pending transfer, got %d", len(reg.List()))
	}
	if reg.List()[0].ID != id {
		t.Errorf("expected listed ID %q, got %q", id, reg.List()[0].ID)
	}

	if !reg.Decide(id, true) {
		t.Fatal("expected Decide on live ID to report true")
	}
	select {
	case accept := <-ch:
		if !accept {
			t.Error("expected accept=true on decision channel")
		}
	default:
		t.Error("expected decision to be delivered without blocking")
	}

	// Second decision for the same ID must report false.
	if reg.Decide(id, false) {
		t.Error("expected second Decide to report false")
	}

	reg.Remove(id)
	if len(reg.List()) != 0 {
		t.Errorf("expected empty registry after Remove, got %d", len(reg.List()))
	}
}

func TestPendingRegistryDecideUnknown(t *testing.T) {
	reg := handlers.NewPendingRegistry()
	if reg.Decide("nope", true) {
		t.Error("expected Decide on unknown ID to report false")
	}
	reg.Remove("nope") // must not panic
}

// waitForPending polls the registry until an entry appears or the deadline
// passes. The handler goroutine registers its pending transfer asynchronously.
func waitForPending(t *testing.T, reg *handlers.PendingRegistry) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if pendings := reg.List(); len(pendings) > 0 {
			return pendings[0].ID
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for pending transfer to register")
	return ""
}

// TestPrepareUpload_IPCHookAccept verifies the full external-decision flow:
// prepare blocks on the hook, an IPC accept lets it through to a session.
func TestPrepareUpload_IPCHookAccept(t *testing.T) {
	cfg := &config.Config{DownloadDir: t.TempDir(), AutoAccept: false}
	receiveService := services.NewReceiveService()
	handler := handlers.NewReceiveHandler(cfg, receiveService, nil, context.Background(), testLogger)
	reg := handler.EnableTransferHook()

	reqDto := model.PrepareUploadRequestDto{
		Info:  model.InfoDto{Alias: "HookSender"},
		Files: map[string]model.FileDto{"f1": {ID: "f1", FileName: "hook.txt", Size: 5}},
	}
	body, _ := json.Marshal(reqDto)
	req, _ := http.NewRequest(http.MethodPost, "/v2/prepare-upload", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.100:12345"

	rr := httptest.NewRecorder()
	done := make(chan int, 1)
	go func() {
		handler.PrepareUploadHandlerV2(rr, req)
		done <- rr.Code
	}()

	pendingID := waitForPending(t, reg)
	if !reg.Decide(pendingID, true) {
		t.Fatalf("expected Decide(%q) to succeed", pendingID)
	}

	select {
	case status := <-done:
		if status != http.StatusOK {
			t.Errorf("expected 200 after IPC accept, got %v", status)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("handler did not finish after IPC accept")
	}
	if len(reg.List()) != 0 {
		t.Errorf("expected registry cleanup after decision, got %d entries", len(reg.List()))
	}
}

// TestPrepareUpload_IPCHookReject verifies an external reject ends the
// prepare with 403 and no session.
func TestPrepareUpload_IPCHookReject(t *testing.T) {
	cfg := &config.Config{DownloadDir: t.TempDir(), AutoAccept: false}
	receiveService := services.NewReceiveService()
	handler := handlers.NewReceiveHandler(cfg, receiveService, nil, context.Background(), testLogger)
	reg := handler.EnableTransferHook()

	reqDto := model.PrepareUploadRequestDto{
		Info:  model.InfoDto{Alias: "HookSender"},
		Files: map[string]model.FileDto{"f1": {ID: "f1", FileName: "hook.txt", Size: 5}},
	}
	body, _ := json.Marshal(reqDto)
	req, _ := http.NewRequest(http.MethodPost, "/v2/prepare-upload", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.100:12345"

	rr := httptest.NewRecorder()
	done := make(chan int, 1)
	go func() {
		handler.PrepareUploadHandlerV2(rr, req)
		done <- rr.Code
	}()

	pendingID := waitForPending(t, reg)
	if !reg.Decide(pendingID, false) {
		t.Fatalf("expected Decide(%q) to succeed", pendingID)
	}

	select {
	case status := <-done:
		if status != http.StatusForbidden {
			t.Errorf("expected 403 after IPC reject, got %v", status)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("handler did not finish after IPC reject")
	}
}

// TestPrepareUpload_IPCHookShutdown verifies server shutdown unblocks an
// awaiting prepare instead of hanging it.
func TestPrepareUpload_IPCHookShutdown(t *testing.T) {
	cfg := &config.Config{DownloadDir: t.TempDir(), AutoAccept: false}
	shutdownCtx, cancel := context.WithCancel(context.Background())
	receiveService := services.NewReceiveService()
	handler := handlers.NewReceiveHandler(cfg, receiveService, nil, shutdownCtx, testLogger)
	reg := handler.EnableTransferHook()

	reqDto := model.PrepareUploadRequestDto{
		Info:  model.InfoDto{Alias: "HookSender"},
		Files: map[string]model.FileDto{"f1": {ID: "f1", FileName: "hook.txt", Size: 5}},
	}
	body, _ := json.Marshal(reqDto)
	req, _ := http.NewRequest(http.MethodPost, "/v2/prepare-upload", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.100:12345"

	rr := httptest.NewRecorder()
	done := make(chan int, 1)
	go func() {
		handler.PrepareUploadHandlerV2(rr, req)
		done <- rr.Code
	}()

	waitForPending(t, reg)
	cancel()

	select {
	case <-done:
		// Any status is fine; the point is the handler returned.
	case <-time.After(10 * time.Second):
		t.Fatal("handler did not return after shutdown")
	}
}
