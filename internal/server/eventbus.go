package server

import (
	"context"
	"sync"

	"bootforge/internal/domain"
)

// EventBus is an in-process publish/subscribe system for domain events.
type EventBus struct {
	mu          sync.RWMutex
	subscribers []chan domain.Event
	bufferSize  int
}

// NewEventBus creates a new event bus with the given channel buffer size.
func NewEventBus(bufferSize int) *EventBus {
	if bufferSize < 1 {
		bufferSize = 64
	}
	return &EventBus{
		bufferSize: bufferSize,
	}
}

// Publish sends an event to all subscribers. Non-blocking: if a subscriber's
// channel is full, the event is dropped for that subscriber.
func (eb *EventBus) Publish(ctx context.Context, event domain.Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for _, ch := range eb.subscribers {
		select {
		case ch <- event:
		default:
			// Drop event if subscriber can't keep up.
		}
	}
}

// Subscribe returns a channel that receives events. The channel is closed
// when the context is cancelled. Callers must consume events promptly
// to avoid dropped events.
func (eb *EventBus) Subscribe(ctx context.Context) <-chan domain.Event {
	ch := make(chan domain.Event, eb.bufferSize)

	eb.mu.Lock()
	eb.subscribers = append(eb.subscribers, ch)
	eb.mu.Unlock()

	go func() {
		<-ctx.Done()
		eb.mu.Lock()
		defer eb.mu.Unlock()

		for i, sub := range eb.subscribers {
			if sub == ch {
				eb.subscribers = append(eb.subscribers[:i], eb.subscribers[i+1:]...)
				break
			}
		}
		close(ch)
	}()

	return ch
}
