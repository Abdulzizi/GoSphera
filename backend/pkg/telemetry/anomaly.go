package telemetry

import (
	"sync"
	"time"
)

// AnomalyThreshold is the Δt above which an entity is flagged anomalous.
const AnomalyThreshold = 30 * time.Second

// AnomalyDetector flags events whose inter-arrival time exceeds AnomalyThreshold.
// It is safe for concurrent use.
type AnomalyDetector struct {
	lastSeen sync.Map // map[string]time.Time  (entity ID → last event time)
}

// NewAnomalyDetector creates a ready-to-use AnomalyDetector.
func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{}
}

// Detect evaluates event against the stored last-seen time for its ID.
// It mutates event.Properties["anomaly"] (bool) and updates the stored timestamp.
func (ad *AnomalyDetector) Detect(event *TelemetryEvent) {
	if event.Properties == nil {
		event.Properties = make(map[string]interface{})
	}

	var anomalous bool

	if val, ok := ad.lastSeen.Load(event.ID); ok {
		lastTime := val.(time.Time)
		delta := event.Timestamp.Sub(lastTime)
		anomalous = delta > AnomalyThreshold
	}
	// First-ever event for this ID → anomalous stays false.

	event.Properties["anomaly"] = anomalous
	ad.lastSeen.Store(event.ID, event.Timestamp)
}
