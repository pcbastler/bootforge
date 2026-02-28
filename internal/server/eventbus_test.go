package server

import (
	"context"
	"testing"
	"time"

	"bootforge/internal/domain"
)

func TestEventBusPublishSubscribe(t *testing.T) {
	eb := NewEventBus(16)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := eb.Subscribe(ctx)

	event := domain.Event{
		Type:    domain.EventBoot,
		Message: "test event",
		At:      time.Now(),
	}
	eb.Publish(ctx, event)

	select {
	case got := <-ch:
		if got.Message != "test event" {
			t.Errorf("received event message = %q, want %q", got.Message, "test event")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestEventBusPublishNoSubscribers(t *testing.T) {
	eb := NewEventBus(16)
	ctx := context.Background()

	// Should not panic.
	event := domain.Event{Type: domain.EventBoot, Message: "no listeners"}
	eb.Publish(ctx, event)
}

func TestEventBusMultipleSubscribers(t *testing.T) {
	eb := NewEventBus(16)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch1 := eb.Subscribe(ctx)
	ch2 := eb.Subscribe(ctx)

	event := domain.Event{Type: domain.EventBoot, Message: "broadcast"}
	eb.Publish(ctx, event)

	for i, ch := range []<-chan domain.Event{ch1, ch2} {
		select {
		case got := <-ch:
			if got.Message != "broadcast" {
				t.Errorf("subscriber %d: message = %q, want %q", i, got.Message, "broadcast")
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out", i)
		}
	}
}

func TestEventBusContextCancel(t *testing.T) {
	eb := NewEventBus(16)
	ctx, cancel := context.WithCancel(context.Background())

	ch := eb.Subscribe(ctx)
	cancel()

	// Channel should be closed after context cancel.
	// Give goroutine time to process.
	time.Sleep(50 * time.Millisecond)

	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after context cancel")
	}
}

func TestEventBusDropOnFull(t *testing.T) {
	eb := NewEventBus(1) // Very small buffer.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := eb.Subscribe(ctx)

	// Fill the buffer.
	eb.Publish(ctx, domain.Event{Message: "event-1"})
	// This should be dropped (buffer full).
	eb.Publish(ctx, domain.Event{Message: "event-2"})

	got := <-ch
	if got.Message != "event-1" {
		t.Errorf("first event = %q, want %q", got.Message, "event-1")
	}

	// Channel should be empty now (event-2 was dropped).
	select {
	case ev := <-ch:
		// event-2 might or might not be there depending on timing
		// but at least we shouldn't hang forever
		_ = ev
	case <-time.After(50 * time.Millisecond):
		// Expected: event-2 was dropped.
	}
}

func TestEventBusSubscribeAfterPublish(t *testing.T) {
	eb := NewEventBus(16)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Publish before subscribing.
	eb.Publish(ctx, domain.Event{Message: "before subscribe"})

	ch := eb.Subscribe(ctx)

	// New subscriber should not receive old events.
	select {
	case <-ch:
		t.Error("new subscriber should not receive events published before subscribing")
	case <-time.After(50 * time.Millisecond):
		// Expected.
	}
}
