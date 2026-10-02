package handlers

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/bethropolis/localgo/pkg/events"
	"github.com/bethropolis/localgo/pkg/model"
	"github.com/google/uuid"
)

// PendingFile is the IPC-visible summary of one file in a pending transfer.
type PendingFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
	Type string `json:"type"`
}

// PendingTransfer is the IPC-visible view of a transfer awaiting a decision.
type PendingTransfer struct {
	ID          string        `json:"pendingId"`
	SenderAlias string        `json:"senderAlias"`
	SenderIP    string        `json:"senderIp"`
	Clipboard   bool          `json:"clipboard"`
	Files       []PendingFile `json:"files"`
	CreatedAt   int64         `json:"createdAt"`
}

type pendingEntry struct {
	transfer PendingTransfer
	decide   chan bool
	decided  bool
}

// PublishBroker is implemented by the events broker; the receive handler uses
// it to stream activity to IPC/SSE consumers. It is satisfied by
// *events.Broker and kept as an interface to avoid a package cycle.
type PublishBroker interface {
	Publish(eventType string, data interface{})
}

// PendingRegistry tracks transfers awaiting an external (IPC) decision.
// Every entry is removed by its awaiting handler — including on timeout and
// shutdown — so Decide on an unknown ID simply reports false instead of
// blocking or leaking.
type PendingRegistry struct {
	mu     sync.Mutex
	items  map[string]*pendingEntry
	broker PublishBroker
}

// NewPendingRegistry creates an empty pending-transfer registry. broker may
// be nil, in which case no events are published.
func NewPendingRegistry(broker PublishBroker) *PendingRegistry {
	return &PendingRegistry{items: make(map[string]*pendingEntry), broker: broker}
}

// Add registers a transfer and returns its ID plus the decision channel.
// The caller must Remove the ID when done (defer).
func (r *PendingRegistry) Add(t PendingTransfer) (string, <-chan bool) {
	id := uuid.NewString()
	t.ID = id
	t.CreatedAt = time.Now().UnixMilli()
	ch := make(chan bool, 1)
	r.mu.Lock()
	r.items[id] = &pendingEntry{transfer: t, decide: ch}
	r.mu.Unlock()
	r.publish(events.TypeTransferPending, t)
	return id, ch
}

// publish forwards an event to the broker, tolerating a nil broker.
func (r *PendingRegistry) publish(eventType string, data interface{}) {
	if r == nil || r.broker == nil {
		return
	}
	r.broker.Publish(eventType, data)
}

// publishEvent forwards an event from the handler, tolerating a nil broker.
func (h *ReceiveHandler) publishEvent(eventType string, data interface{}) {
	if h == nil || h.broker == nil {
		return
	}
	h.broker.Publish(eventType, data)
}

// Remove drops a pending transfer. Safe to call for unknown IDs.
func (r *PendingRegistry) Remove(id string) {
	r.mu.Lock()
	delete(r.items, id)
	r.mu.Unlock()
}

// Decide records an accept (true) or reject (false) for a pending transfer.
// It reports whether the transfer still existed. Each transfer accepts
// exactly one decision; repeats and unknown IDs report false.
func (r *PendingRegistry) Decide(id string, accept bool) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.items[id]
	if !ok || e.decided {
		return false
	}
	e.decided = true
	// Buffered(1) and first send: never blocks. If the handler already timed
	// out, the value is simply dropped with the entry on Remove.
	e.decide <- accept
	return true
}

// List snapshots all transfers currently awaiting a decision.
func (r *PendingRegistry) List() []PendingTransfer {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]PendingTransfer, 0, len(r.items))
	for _, e := range r.items {
		out = append(out, e.transfer)
	}
	return out
}

// pendingFilesFromDTO converts prepare-upload files to IPC-visible summaries.
func pendingFilesFromDTO(files map[string]model.FileDto) []PendingFile {
	out := make([]PendingFile, 0, len(files))
	for id, f := range files {
		out = append(out, PendingFile{ID: id, Name: f.FileName, Size: f.Size, Type: f.FileType})
	}
	return out
}

// awaitIPCDecision registers a pending transfer and waits for an external
// decision, the timeout, or server shutdown — whichever comes first.
// It always fails closed (false). Callers pass already-sanitized display
// values (see sanitizeName / cli.Sanitize at the prepare boundary).
func (h *ReceiveHandler) awaitIPCDecision(t PendingTransfer, timeout time.Duration) bool {
	if h.pending == nil {
		return false
	}
	id, ch := h.pending.Add(t)
	defer h.pending.Remove(id)
	h.logger.Infof("Transfer awaiting external decision (pendingId=%s)", id)
	fmt.Fprintf(os.Stderr, "\n━━ Incoming transfer from %s awaits an external decision (pendingId=%s, default: reject after %ds) ━━\n",
		t.SenderAlias, id, int(timeout.Seconds()))
	os.Stderr.Sync()
	select {
	case accept := <-ch:
		if !accept {
			h.publishEvent(events.TypeTransferRejected, map[string]string{"pendingId": id})
		}
		return accept
	case <-time.After(timeout):
		h.logger.Warnf("External decision timeout for %s, rejecting", id)
		h.publishEvent(events.TypeTransferRejected, map[string]string{"pendingId": id, "reason": "timeout"})
		return false
	case <-h.shutdownCtx.Done():
		return false
	}
}
