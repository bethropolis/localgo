package ipc

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bethropolis/localgo/pkg/events"
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

func startTestIPC(t *testing.T, ctrl Controller, broker *events.Broker) (*IPCServer, *http.Client) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("unix control socket is not supported on Windows")
	}
	sockPath := filepath.Join(t.TempDir(), "ipc.sock")
	srv, err := startOnPath(sockPath, ctrl, broker)
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
		pending: handlers.NewPendingRegistry(nil),
	}
	_, client := startTestIPC(t, ctrl, events.NewBroker())

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
	reg := handlers.NewPendingRegistry(nil)
	ctrl := fakeController{pending: reg}
	_, client := startTestIPC(t, ctrl, events.NewBroker())

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

// TestIPCEventsStream verifies the SSE endpoint emits broker events with
// Last-Event-ID style ids and named event types.
func TestIPCEventsStream(t *testing.T) {
	reg := handlers.NewPendingRegistry(nil)
	broker := events.NewBroker()
	// The registry publishes through its own broker reference; wire it in.
	reg = handlers.NewPendingRegistry(broker)
	ctrl := fakeController{pending: reg}
	_, client := startTestIPC(t, ctrl, broker)

	req, err := http.NewRequest(http.MethodGet, "http://ipc/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/events failed: %v", err)
	}
	defer resp.Body.Close()
	sse := newSSEReader(resp.Body)

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}

	// Publish after subscribing so we only assert on the live stream.
	id, _ := reg.Add(handlers.PendingTransfer{
		SenderAlias: "Phone",
		SenderIP:    "192.168.1.5",
		Files:       []handlers.PendingFile{{ID: "f1", Name: "a.txt", Size: 3}},
	})
	_ = id

	frame, err := sse.readFrame()
	if err != nil {
		t.Fatalf("failed to read SSE frame: %v", err)
	}
	if frame.event != events.TypeTransferPending {
		t.Errorf("expected event %q, got %q", events.TypeTransferPending, frame.event)
	}
	if frame.id == 0 {
		t.Error("expected non-zero SSE event id")
	}

	var evt events.Event
	if err := json.Unmarshal(frame.data, &evt); err != nil {
		t.Fatalf("SSE data is not valid JSON: %v", err)
	}
	if evt.Type != events.TypeTransferPending {
		t.Errorf("unexpected event type in payload: %q", evt.Type)
	}
	pending, ok := evt.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected payload shape: %#v", evt.Data)
	}
	if pending["senderAlias"] != "Phone" {
		t.Errorf("expected senderAlias in payload, got %v", pending["senderAlias"])
	}
}

// waitForSubscriber blocks until the broker has more subscribers than
// baseline, so published events cannot be lost to a subscribe/publish race.
// A baseline (rather than "> 0") is required because sibling tests may still
// hold connections open.
func waitForSubscriber(t *testing.T, broker *events.Broker, baseline int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if broker.SubscriberCount() > baseline {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for SSE subscriber")
}

// sseReader wraps an SSE response body. One bufio.Scanner must be shared
// across frames: a fresh scanner per frame discards already-buffered bytes and
// then blocks waiting for data that will never arrive.
type sseReader struct {
	sc *bufio.Scanner
}

func newSSEReader(r io.Reader) *sseReader {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	return &sseReader{sc: sc}
}

type sseFrame struct {
	id    uint64
	event string
	data  []byte
}

// readFrame reads one complete SSE frame (terminated by a blank line).
func (r *sseReader) readFrame() (sseFrame, error) {
	var frame sseFrame
	var data []byte
	for r.sc.Scan() {
		line := r.sc.Text()
		switch {
		case line == "":
			if len(data) > 0 {
				frame.data = data
				return frame, nil
			}
		case strings.HasPrefix(line, "id: "):
			frame.id, _ = strconv.ParseUint(strings.TrimPrefix(line, "id: "), 10, 64)
		case strings.HasPrefix(line, "event: "):
			frame.event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			data = []byte(strings.TrimPrefix(line, "data: "))
		}
	}
	if err := r.sc.Err(); err != nil {
		return frame, err
	}
	return frame, io.ErrUnexpectedEOF
}

// openSSE issues GET /v1/events and returns a frame reader for the stream.
func openSSE(t *testing.T, client *http.Client, header map[string]string) (*sseReader, func()) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://ipc/v1/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/events failed: %v", err)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		resp.Body.Close()
		t.Fatalf("expected text/event-stream, got %q", ct)
	}
	return newSSEReader(resp.Body), func() { resp.Body.Close() }
}

func TestIPCEventsAllTypes(t *testing.T) {
	broker := events.NewBroker()
	reg := handlers.NewPendingRegistry(broker)
	_, client := startTestIPC(t, fakeController{pending: reg}, broker)

	baseline := broker.SubscriberCount()
	sse, closeStream := openSSE(t, client, nil)
	defer closeStream()

	want := []string{
		events.TypeTransferPending,
		events.TypeTransferProgress,
		events.TypeTransferComplete,
		events.TypeTransferRejected,
		events.TypeDeviceDiscovered,
	}
	waitForSubscriber(t, broker, baseline)
	for _, typ := range want {
		broker.Publish(typ, map[string]string{"kind": typ})
	}

	for i, typ := range want {
		frame, err := sse.readFrame()
		if err != nil {
			t.Fatalf("reading frame %d: %v", i, err)
		}
		if frame.event != typ {
			t.Errorf("event %d: got %q, want %q", i, frame.event, typ)
		}
		if frame.id == 0 {
			t.Errorf("event %d: expected non-zero SSE id", i)
		}
	}
}

// TestIPCEventsNoReplayWithoutLastEventID verifies a fresh SSE connection does
// not replay retained history (clients would otherwise see stale events).
func TestIPCEventsNoReplayWithoutLastEventID(t *testing.T) {
	broker := events.NewBroker()
	_, client := startTestIPC(t, fakeController{pending: handlers.NewPendingRegistry(nil)}, broker)

	// History from before any client connected.
	broker.Publish(events.TypeTransferComplete, map[string]string{"file": "old.bin"})

	baseline := broker.SubscriberCount()
	sse, closeStream := openSSE(t, client, nil)
	defer closeStream()

	waitForSubscriber(t, broker, baseline)
	broker.Publish(events.TypeDeviceDiscovered, map[string]string{"alias": "Phone"})

	frame, err := sse.readFrame()
	if err != nil {
		t.Fatalf("failed to read SSE frame: %v", err)
	}
	if frame.event != events.TypeDeviceDiscovered {
		t.Errorf("expected the live %s event first, got %q (stale history replayed?)", events.TypeDeviceDiscovered, frame.event)
	}
}

// TestIPCEventsResumesWithLastEventID verifies reconnecting clients can replay
// missed events via Last-Event-ID.
func TestIPCEventsResumesWithLastEventID(t *testing.T) {
	broker := events.NewBroker()
	_, client := startTestIPC(t, fakeController{pending: handlers.NewPendingRegistry(nil)}, broker)

	broker.Publish(events.TypeTransferPending, map[string]string{"file": "a.txt"})
	missed := broker.HistorySince(0)
	if len(missed) != 1 {
		t.Fatalf("expected 1 retained event, got %d", len(missed))
	}
	broker.Publish(events.TypeTransferComplete, map[string]string{"file": "a.txt"})

	baseline := broker.SubscriberCount()
	sse, closeStream := openSSE(t, client, map[string]string{
		"Last-Event-ID": strconv.FormatUint(missed[len(missed)-1].ID, 10),
	})
	defer closeStream()

	waitForSubscriber(t, broker, baseline)
	frame, err := sse.readFrame()
	if err != nil {
		t.Fatalf("failed to read SSE frame: %v", err)
	}
	if frame.event != events.TypeTransferComplete {
		t.Errorf("expected resumed %s event, got %q", events.TypeTransferComplete, frame.event)
	}
}
