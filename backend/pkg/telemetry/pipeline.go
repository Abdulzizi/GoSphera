package telemetry

import (
	"context"
	"sync"
	"time"
)

// TelemetryEvent is a single geospatial event fed into the pipeline.
type TelemetryEvent struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	Lat        float64                `json:"lat"`
	Lon        float64                `json:"lon"`
	Properties map[string]interface{} `json:"properties"`
}

// Pipeline is a concurrent, channel-driven event processing pipeline.
type Pipeline struct {
	ingest    chan TelemetryEvent
	processed chan TelemetryEvent
	workers   int
	detector  *AnomalyDetector
	mu        sync.RWMutex
	running   bool
}

// NewPipeline creates a Pipeline with the given number of worker goroutines.
// workerCount is clamped to a minimum of 1.
func NewPipeline(workerCount int) *Pipeline {
	if workerCount < 1 {
		workerCount = 1
	}
	return &Pipeline{
		ingest:    make(chan TelemetryEvent, 100),
		processed: make(chan TelemetryEvent, 100),
		workers:   workerCount,
		detector:  NewAnomalyDetector(),
	}
}

// Start spawns worker goroutines that process events until ctx is cancelled.
func (p *Pipeline) Start(ctx context.Context) {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	for i := 0; i < p.workers; i++ {
		go p.worker(ctx)
	}
}

func (p *Pipeline) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-p.ingest:
			if !ok {
				return
			}
			p.detector.Detect(&event)
			select {
			case p.processed <- event:
			case <-ctx.Done():
				return
			default:
				// processed buffer full — drop rather than block
			}
		}
	}
}

// Ingest sends an event to the pipeline without blocking.
// Events are silently dropped if the ingest buffer is full or pipeline is stopped.
func (p *Pipeline) Ingest(event TelemetryEvent) {
	p.mu.RLock()
	running := p.running
	p.mu.RUnlock()
	if !running {
		return
	}
	select {
	case p.ingest <- event:
	default:
	}
}

// Stop closes the ingest channel and marks the pipeline as stopped.
func (p *Pipeline) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		close(p.ingest)
		p.running = false
	}
}

// GetProcessedChannel exposes the processed-events channel for consumers.
func (p *Pipeline) GetProcessedChannel() <-chan TelemetryEvent {
	return p.processed
}
