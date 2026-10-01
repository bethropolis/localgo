// Package ipc exposes a local control socket so third-party programs
// (GUIs, trays, scripts, daemons) can drive a running `localgo serve`
// instance: query status and peers, list pending transfers, and accept or
// reject them programmatically instead of via --auto-accept or the
// interactive terminal prompt.
//
// The socket is Unix-domain (a 0700 directory under the user cache dir).
// Windows is not supported yet: starting the server errors out clearly
// instead of binding a half-working endpoint.
package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/bethropolis/localgo/pkg/model"
	"github.com/bethropolis/localgo/pkg/server/handlers"
)

// Controller is implemented by the serve command: it backs the IPC
// endpoints with live server state.
type Controller interface {
	// Status describes the running server.
	Status() Status
	// Devices lists recently discovered peers.
	Devices() []*model.Device
	// Pending lists transfers awaiting an accept/reject decision.
	Pending() []handlers.PendingTransfer
	// DecidePending records a decision. False means the ID is unknown
	// (answered, timed out, or never existed).
	DecidePending(pendingID string, accept bool) bool
}

// Status describes a running serve instance.
type Status struct {
	Alias        string `json:"alias"`
	Fingerprint  string `json:"fingerprint"`
	Port         int    `json:"port"`
	Protocol     string `json:"protocol"`
	Version      string `json:"version"`
	DownloadDir  string `json:"downloadDir"`
	AutoAccept   bool   `json:"autoAccept"`
	PendingCount int    `json:"pendingCount"`
}

// SocketPath returns the control socket path for this user.
func SocketPath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	return filepath.Join(cacheDir, "localgo", "ipc.sock")
}

// IPCServer is a running control socket.
type IPCServer struct {
	listener net.Listener
	sockPath string
	srv      *http.Server
}

// SocketPath returns the bound socket path.
func (s *IPCServer) SocketPath() string { return s.sockPath }

// Close stops the server and removes the socket file.
func (s *IPCServer) Close() error {
	err := s.srv.Close()
	if cErr := s.listener.Close(); err == nil {
		err = cErr
	}
	if rErr := os.Remove(s.sockPath); err == nil {
		err = rErr
	}
	return err
}

// StartIPCServer starts the control socket for ctrl. Callers own the
// returned server and must Close it on shutdown.
func StartIPCServer(ctrl Controller) (*IPCServer, error) {
	return startOnPath(SocketPath(), ctrl)
}

// startOnPath binds the control socket at an explicit path (used by tests
// to stay hermetic instead of touching the real user cache dir).
func startOnPath(sockPath string, ctrl Controller) (*IPCServer, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("IPC control socket is not supported on Windows yet (use --auto-accept)")
	}
	if err := os.MkdirAll(filepath.Dir(sockPath), 0700); err != nil {
		return nil, fmt.Errorf("IPC socket dir: %w", err)
	}
	_ = os.Remove(sockPath) // drop stale socket from an unclean shutdown

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("IPC socket listen: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/status", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, ctrl.Status())
	})
	mux.HandleFunc("GET /v1/devices", func(w http.ResponseWriter, r *http.Request) {
		devices := ctrl.Devices()
		if devices == nil {
			devices = []*model.Device{}
		}
		writeJSON(w, devices)
	})
	mux.HandleFunc("GET /v1/pending", func(w http.ResponseWriter, r *http.Request) {
		pending := ctrl.Pending()
		if pending == nil {
			pending = []handlers.PendingTransfer{}
		}
		writeJSON(w, pending)
	})
	mux.HandleFunc("GET /v1/transfer/accept", func(w http.ResponseWriter, r *http.Request) {
		decide(w, r, ctrl, true)
	})
	mux.HandleFunc("GET /v1/transfer/reject", func(w http.ResponseWriter, r *http.Request) {
		decide(w, r, ctrl, false)
	})
	// POST variants for programmatic clients that prefer a body-less POST.
	mux.HandleFunc("POST /v1/transfer/accept", func(w http.ResponseWriter, r *http.Request) {
		decide(w, r, ctrl, true)
	})
	mux.HandleFunc("POST /v1/transfer/reject", func(w http.ResponseWriter, r *http.Request) {
		decide(w, r, ctrl, false)
	})

	srv := &http.Server{Handler: mux}
	ipcSrv := &IPCServer{listener: listener, sockPath: sockPath, srv: srv}
	go func() {
		_ = srv.Serve(listener)
	}()
	return ipcSrv, nil
}

func decide(w http.ResponseWriter, r *http.Request, ctrl Controller, accept bool) {
	id := r.URL.Query().Get("pendingId")
	if id == "" {
		http.Error(w, "missing pendingId", http.StatusBadRequest)
		return
	}
	if !ctrl.DecidePending(id, accept) {
		http.Error(w, "unknown or expired pendingId", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
