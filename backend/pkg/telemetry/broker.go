package telemetry

import "sync"

// EventBroker fans out TelemetryEvents to all registered SSE clients.
// It is safe for concurrent use.
type EventBroker struct {
	mu      sync.RWMutex
	clients map[chan TelemetryEvent]struct{}
}

// NewEventBroker creates a ready-to-use EventBroker.
func NewEventBroker() *EventBroker {
	return &EventBroker{
		clients: make(map[chan TelemetryEvent]struct{}),
	}
}

// Subscribe registers a new client and returns its dedicated channel (buffered 64).
func (b *EventBroker) Subscribe() chan TelemetryEvent {
	ch := make(chan TelemetryEvent, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes the client channel and closes it.
func (b *EventBroker) Unsubscribe(ch chan TelemetryEvent) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

// Broadcast sends event to every registered client.
// Sends are non-blocking: a slow/full client is skipped, never stalls the broadcaster.
func (b *EventBroker) Broadcast(event TelemetryEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- event:
		default:
			// client buffer full — drop for this client only
		}
	}
}

// ClientCount returns the number of active SSE subscribers.
func (b *EventBroker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
