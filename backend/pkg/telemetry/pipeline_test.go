package telemetry

import (
	"context"
	"testing"
	"time"
)

func TestPipelineIngest(t *testing.T) {
	p := NewPipeline(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p.Start(ctx)

	event := TelemetryEvent{
		ID:         "test-1",
		Timestamp:  time.Now(),
		Lat:        25.5,
		Lon:        -80.2,
		Properties: map[string]interface{}{"source": "test"},
	}
	p.Ingest(event)

	// Give worker time to process.
	time.Sleep(100 * time.Millisecond)

	select {
	case processed := <-p.GetProcessedChannel():
		if processed.ID != event.ID {
			t.Errorf("expected ID %s, got %s", event.ID, processed.ID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timed out waiting for processed event")
	}

	p.Stop()
}

func TestPipelineWorkerCount(t *testing.T) {
	tests := []struct {
		input       int
		expectedMin int
	}{
		{3, 3},
		{0, 1},
		{-1, 1},
	}
	for _, tt := range tests {
		p := NewPipeline(tt.input)
		if p.workers < tt.expectedMin {
			t.Errorf("input %d: expected workers >= %d, got %d", tt.input, tt.expectedMin, p.workers)
		}
	}
}
