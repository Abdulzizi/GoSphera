package telemetry

import (
	"testing"
	"time"
)

func TestBrokerSubscribeUnsubscribe(t *testing.T) {
	b := NewEventBroker()

	if got := b.ClientCount(); got != 0 {
		t.Fatalf("expected 0 clients before subscribe, got %d", got)
	}

	ch := b.Subscribe()
	if got := b.ClientCount(); got != 1 {
		t.Fatalf("expected 1 client after subscribe, got %d", got)
	}

	b.Unsubscribe(ch)
	if got := b.ClientCount(); got != 0 {
		t.Fatalf("expected 0 clients after unsubscribe, got %d", got)
	}

	// channel must be closed after Unsubscribe
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("channel should be closed after Unsubscribe")
		}
	default:
		t.Fatal("channel was not closed after Unsubscribe")
	}
}

func TestBrokerBroadcast(t *testing.T) {
	b := NewEventBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	want := TelemetryEvent{ID: "sensor-1", Lat: 1.23, Lon: 4.56}
	b.Broadcast(want)

	select {
	case got := <-ch:
		if got.ID != want.ID || got.Lat != want.Lat || got.Lon != want.Lon {
			t.Fatalf("broadcast event mismatch: got %+v, want %+v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast event")
	}
}

func TestBrokerSlowClientDropped(t *testing.T) {
	b := NewEventBroker()
	ch := b.Subscribe() // buffer size 64
	defer b.Unsubscribe(ch)

	// Fill the buffer completely (64 events).
	filler := TelemetryEvent{ID: "fill"}
	for i := 0; i < 64; i++ {
		b.Broadcast(filler)
	}

	// One more broadcast must not block even though buffer is full.
	done := make(chan struct{})
	go func() {
		b.Broadcast(TelemetryEvent{ID: "overflow"})
		close(done)
	}()

	select {
	case <-done:
		// good — Broadcast returned without blocking
	case <-time.After(time.Second):
		t.Fatal("Broadcast blocked on a full client channel")
	}
}
