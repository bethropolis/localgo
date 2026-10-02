package ipc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bethropolis/localgo/pkg/events"
)

// sseHeartbeatInterval keeps idle SSE connections alive through proxies and
// lets clients detect a dead peer.
const sseHeartbeatInterval = 15 * time.Second

// handleEvents streams broker events to the client as text/event-stream.
// Supports Last-Event-ID (or ?lastEventId=) to replay missed events from the
// broker's retained history.
func (s *IPCServer) handleEvents(broker *events.Broker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		// Replay only when the client asks to resume (Last-Event-ID header or
		// ?lastEventId=). A fresh connection must start live, otherwise it
		// would receive the whole retained history as if it were new.
		var lastID uint64
		resume := false
		if v := r.Header.Get("Last-Event-ID"); v != "" {
			if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
				lastID, resume = parsed, true
			}
		}
		if v := r.URL.Query().Get("lastEventId"); v != "" {
			if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
				lastID, resume = parsed, true
			}
		}

		// Subscribe before replaying so no event is lost in between.
		ch, unsubscribe := broker.Subscribe()
		defer unsubscribe()

		if resume {
			for _, evt := range broker.HistorySince(lastID) {
				if err := writeSSE(w, evt); err != nil {
					return
				}
			}
			flusher.Flush()
		}

		heartbeat := time.NewTicker(sseHeartbeatInterval)
		defer heartbeat.Stop()

		for {
			select {
			case evt, ok := <-ch:
				if !ok {
					return
				}
				if err := writeSSE(w, evt); err != nil {
					return
				}
				flusher.Flush()
			case <-heartbeat.C:
				// Comment line keeps the connection warm without an event.
				if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
					return
				}
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}

// writeSSE emits one event as an SSE frame.
func writeSSE(w http.ResponseWriter, evt events.Event) error {
	payload, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", evt.ID, evt.Type, payload)
	return err
}
