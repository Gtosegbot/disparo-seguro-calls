// Package events defines the internal event bus for the AI voice layer.
// Consumers (analytics, CRM, billing) subscribe without touching the core.
package events

import (
	"sync"
	"time"
)

// Kind enumerates all AI-layer events.
type Kind string

const (
	KindCallCreated    Kind = "call.created"
	KindCallRinging    Kind = "call.ringing"
	KindCallConnected  Kind = "call.connected"
	KindAIStarted      Kind = "ai.started"
	KindAIListening    Kind = "ai.listening"
	KindAIThinking     Kind = "ai.thinking"
	KindAISpeaking     Kind = "ai.speaking"
	KindAIInterrupted  Kind = "ai.interrupted"
	KindAIEnded        Kind = "ai.ended"
	KindAIError        Kind = "ai.error"
)

// Event is a single occurrence on the bus.
type Event struct {
	Kind      Kind           `json:"kind"`
	SessionID string         `json:"session_id"`
	TenantID  string         `json:"tenant_id"`
	CallID    string         `json:"call_id"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload,omitempty"`
}

// Handler is a function that receives events. Must not block.
type Handler func(Event)

// Bus is a simple in-process pub/sub event bus.
type Bus struct {
	mu       sync.RWMutex
	handlers []Handler
}

// NewBus returns a ready Bus.
func NewBus() *Bus { return &Bus{} }

// Subscribe registers a handler. All handlers receive every event.
func (b *Bus) Subscribe(h Handler) {
	b.mu.Lock()
	b.handlers = append(b.handlers, h)
	b.mu.Unlock()
}

// Publish sends an event to all registered handlers (non-blocking goroutines).
func (b *Bus) Publish(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	b.mu.RLock()
	hs := make([]Handler, len(b.handlers))
	copy(hs, b.handlers)
	b.mu.RUnlock()
	for _, h := range hs {
		h := h
		go h(e)
	}
}
