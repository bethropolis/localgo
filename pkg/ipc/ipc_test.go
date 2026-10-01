package ipc

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/handlers"
)

type fakeController struct {
	status  Status
	devices []*model.Device
	pending *handlers.PendingRegistry
}

func (f fakeController) Status() Status                      { return f.status }
func (f fakeController) Devices() []*model.Device            { return f.devices }
func (f fakeController) Pending() []handlers.PendingTransfer { return f.pending.List() }
func (f fakeController) DecidePending(id string, accept bool) bool {
	return f.pending.Decide(id, accept)
}

func unixClient(sockPath string) *http.Client {
	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sockPath)
		},
	}}
}

func startTestIPC(t *testing.T, ctrl Controller) (*IPCServer, *http.Client) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("unix control socket is not supported on Windows")
	}
	sockPath := filepath.Join(t.TempDir(), "ipc.sock")
	srv, err := startOnPath(sockPath, ctrl)
	if err != nil {
		t.Fatalf("startOnPath failed: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv, unixClient(sockPath)
}

func TestIPCStatusAndDevices(t *testing.T) {
	ctrl := fakeController{
		status:  Status{Alias: "Test", Port: 53317, Protocol: "https", Version: "dev"},
		devices: []*model.Device{{Alias: "Phone", IP: "192.168.1.5", Port: 53317}},
		pending: handlers.NewPendingRegistry(),
	}
	_, client := startTestIPC(t, ctrl)

	resp, err := client.Get("http://ipc/v1/status")
	if err != nil {
		t.Fatalf("GET /v1/status failed: %v", err)
	}
	defer resp.Body.Close()
	var st Status
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatalf("bad status JSON: %v", err)
	}
	if st.Alias != "Test" || st.Port != 53317 {
		t.Errorf("unexpected status: %+v", st)
	}

	resp, err = client.Get("http://ipc/v1/devices")
	if err != nil {
		t.Fatalf("GET /v1/devices failed: %v", err)
	}
	defer resp.Body.Close()
	var devices []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&devices); err != nil {
		t.Fatalf("bad devices JSON: %v", err)
	}
	if len(devices) != 1 || devices[0]["alias"] != "Phone" {
		t.Errorf("unexpected devices: %v", devices)
	}
}

func TestIPCDecideRoundTrip(t *testing.T) {
	reg := handlers.NewPendingRegistry()
	ctrl := fakeController{pending: reg}
	_, client := startTestIPC(t, ctrl)

	id, _ := reg.Add(handlers.PendingTransfer{SenderAlias: "Phone"})

	// Pending shows up.
	resp, err := client.Get("http://ipc/v1/pending")
	if err != nil {
		t.Fatalf("GET /v1/pending failed: %v", err)
	}
	var pendings []handlers.PendingTransfer
	if err := json.NewDecoder(resp.Body).Decode(&pendings); err != nil {
		resp.Body.Close()
		t.Fatalf("bad pending JSON: %v", err)
	}
	resp.Body.Close()
	if len(pendings) != 1 || pendings[0].ID != id {
		t.Fatalf("unexpected pending list: %+v", pendings)
	}

	// Accept works once...
	resp, err = client.Get("http://ipc/v1/transfer/accept?pendingId=" + id)
	if err != nil {
		t.Fatalf("accept failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on accept, got %v", resp.StatusCode)
	}

	// ...then the ID is gone.
	resp, err = client.Get("http://ipc/v1/transfer/reject?pendingId=" + id)
	if err != nil {
		t.Fatalf("second decide failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 on spent ID, got %v", resp.StatusCode)
	}

	// Missing ID is a 400.
	resp, err = client.Get("http://ipc/v1/transfer/accept")
	if err != nil {
		t.Fatalf("accept without ID failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 on missing pendingId, got %v", resp.StatusCode)
	}
}
