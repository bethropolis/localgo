// Package events provides a small in-process publish/subscribe broker used
// to stream LocalGo activity (pending transfers, transfer progress,
// completed transfers, discovered devices) to IPC/SSE consumers.
//
// It lives in its own package so low-level packages (server handlers) can
// publish without importing the IPC layer, which depends on them.
package events

import (
	"sync"
	"sync/atomic"
	"time"
)

// Event types published by the broker.
const (
	TypeTransferPending  = "transfer_pending"
	TypeTransferProgress = "transfer_progress"
	TypeTransferComplete = "transfer_complete"
	TypeTransferRejected = "transfer_rejected"
	TypeDeviceDiscovered = "device_discovered"
)

// Event is a single broker message.
type Event struct {
	// ID is a monotonically increasing sequence number, used for SSE
	// Last-Event-ID resumption.
	ID uint64 `json:"id"`
	// Time is the Unix-millisecond publish time.
	Time int64       `json:"time"`
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

const (
	// defaultHistory is how many recent events are retained for replay.
	defaultHistory = 64
	// defaultSubscriberBuffer is the per-subscriber queue depth. Slow
	// consumers miss events rather than stalling the publisher.
	defaultSubscriberBuffer = 256
)

// Broker fans events out to subscribers. The zero value is not usable; call
// NewBroker.
type Broker struct {
	mu      sync.RWMutex
	subs    map[chan Event]struct{}
	seq     atomic.Uint64
	history []Event
	limit   int
	buffer  int
}

// NewBroker creates a broker retaining the default replay history.
func NewBroker() *Broker {
	return &Broker{
		subs:    make(map[chan Event]struct{}),
		history: make([]Event, 0, defaultHistory),
		limit:   defaultHistory,
		buffer:  defaultSubscriberBuffer,
	}
}

// Publish delivers an event to all current subscribers and records it in the
// replay history. It never blocks: a subscriber whose queue is full misses
// the event.
func (b *Broker) Publish(eventType string, data interface{}) {
	if b == nil {
		return
	}
	evt := Event{
		ID:   b.seq.Add(1),
		Time: time.Now().UnixMilli(),
		Type: eventType,
		Data: data,
	}

	b.mu.Lock()
	b.history = append(b.history, evt)
	if len(b.history) > b.limit {
		b.history = b.history[len(b.history)-b.limit:]
	}
	subs := make([]chan Event, 0, len(b.subs))
	for ch := range b.subs {
		subs = append(subs, ch)
	}
	b.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- evt:
		default: // slow consumer: drop rather than block the publisher
		}
	}
}

// Subscribe returns a channel of future events and a function to unsubscribe.
// The channel is closed by the unsubscribe function.
func (b *Broker) Subscribe() (<-chan Event, func()) {
	if b == nil {
		ch := make(chan Event)
		close(ch)
		return ch, func() {}
	}
	ch := make(chan Event, b.buffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			if _, ok := b.subs[ch]; ok {
				delete(b.subs, ch)
				close(ch)
			}
			b.mu.Unlock()
		})
	}
}

// SubscriberCount reports how many live subscribers the broker has. It is
// primarily useful for tests and diagnostics.
func (b *Broker) SubscriberCount() int {
	if b == nil {
		return 0
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}

// HistorySince returns retained events with an ID greater than afterID, for
// SSE Last-Event-ID resumption. Pass 0 to replay the whole retained history.
func (b *Broker) HistorySince(afterID uint64) []Event {
	if b == nil {
		return nil
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Event, 0, len(b.history))
	for _, evt := range b.history {
		if evt.ID > afterID {
			out = append(out, evt)
		}
	}
	return out
}
