package cli

import (
	"encoding/json"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// jsonMode, when set, silences human-readable stdout output (Print*) so
// stdout carries pure NDJSON events for third-party consumers.
// Progress bars and logs already render on stderr and are unaffected.
var jsonMode atomic.Bool

// emitMu serializes concurrent event writes (uploads run in parallel).
var emitMu sync.Mutex

// SetJSONMode enables or disables machine-readable NDJSON output mode.
func SetJSONMode(enabled bool) { jsonMode.Store(enabled) }

// JSONMode reports whether NDJSON output mode is active.
func JSONMode() bool { return jsonMode.Load() }

// EventType identifies an IPC event.
type EventType string

const (
	EventTransferStart EventType = "transfer_start"
	EventProgress      EventType = "progress"
	EventFileComplete  EventType = "file_complete"
	EventSuccess       EventType = "success"
	EventError         EventType = "error"
)

// IPCEvent is a single newline-delimited JSON event on stdout.
type IPCEvent struct {
	Timestamp int64     `json:"timestamp"`
	Type      EventType `json:"type"`
	File      string    `json:"file,omitempty"`
	Bytes     int64     `json:"bytes,omitempty"`
	Total     int64     `json:"total,omitempty"`
	Percent   float64   `json:"percent,omitempty"`
	Error     string    `json:"error,omitempty"`
	Data      any       `json:"data,omitempty"`
}

// EmitEvent writes one NDJSON event to stdout. It is a no-op unless JSON
// mode is on, so library call sites stay unconditional.
func EmitEvent(evt IPCEvent) {
	if !jsonMode.Load() {
		return
	}
	evt.Timestamp = time.Now().UnixMilli()
	emitMu.Lock()
	defer emitMu.Unlock()
	_ = json.NewEncoder(os.Stdout).Encode(evt)
}

// EmitProgressTracker wraps a byte-count progress callback with an NDJSON
// progress event emitter. next may be nil.
func EmitProgressTracker(file string, total int64, next func(int64)) func(int64) {
	return func(done int64) {
		if next != nil {
			next(done)
		}
		pct := 0.0
		if total > 0 {
			pct = float64(done) / float64(total) * 100
			if pct > 100 {
				pct = 100
			}
		}
		EmitEvent(IPCEvent{Type: EventProgress, File: file, Bytes: done, Total: total, Percent: pct})
	}
}
