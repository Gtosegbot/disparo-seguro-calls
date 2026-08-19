// Package events - tests for the event bus.
package events_test

import (
	"sync"
	"testing"
	"time"

	"wacalls/internal/ai/events"
)

func TestBus_Publish_ReceivesEvent(t *testing.T) {
	bus := events.NewBus()
	var (
		mu  sync.Mutex
		got []events.Event
	)
	bus.Subscribe(func(e events.Event) {
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
	})
	bus.Publish(events.Event{Kind: events.KindAIStarted, SessionID: "s1", TenantID: "t1"})
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0].Kind != events.KindAIStarted {
		t.Errorf("expected KindAIStarted, got %v", got)
	}
}

func TestBus_Publish_TimestampAutoSet(t *testing.T) {
	bus := events.NewBus()
	var mu sync.Mutex
	var recv events.Event
	bus.Subscribe(func(e events.Event) {
		mu.Lock()
		recv = e
		mu.Unlock()
	})
	bus.Publish(events.Event{Kind: events.KindCallCreated})
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if recv.Timestamp.IsZero() {
		t.Error("expected timestamp to be set automatically")
	}
}

func TestBus_MultipleSubscribers(t *testing.T) {
	bus := events.NewBus()
	var count int32
	var mu sync.Mutex
	for i := 0; i < 3; i++ {
		bus.Subscribe(func(e events.Event) {
			mu.Lock()
			count++
			mu.Unlock()
		})
	}
	bus.Publish(events.Event{Kind: events.KindAIEnded})
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if count != 3 {
		t.Errorf("expected 3 handler invocations, got %d", count)
	}
}

func TestBus_AllKindsExist(t *testing.T) {
	kinds := []events.Kind{
		events.KindCallCreated,
		events.KindCallRinging,
		events.KindCallConnected,
		events.KindAIStarted,
		events.KindAIListening,
		events.KindAIThinking,
		events.KindAISpeaking,
		events.KindAIInterrupted,
		events.KindAIEnded,
		events.KindAIError,
	}
	if len(kinds) != 10 {
		t.Errorf("expected 10 kinds, got %d", len(kinds))
	}
}
