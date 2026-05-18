package telemetry

import (
	"testing"
	"time"
)

func TestAnomalyThresholdDetection(t *testing.T) {
	ad := NewAnomalyDetector()
	now := time.Now()

	// First event — no prior record, must not be anomalous.
	e1 := &TelemetryEvent{ID: "entity-1", Timestamp: now, Properties: map[string]interface{}{}}
	ad.Detect(e1)
	if e1.Properties["anomaly"].(bool) {
		t.Error("first event should not be anomalous")
	}

	// Within threshold (15 s < 30 s) — not anomalous.
	e2 := &TelemetryEvent{ID: "entity-1", Timestamp: now.Add(15 * time.Second), Properties: map[string]interface{}{}}
	ad.Detect(e2)
	if e2.Properties["anomaly"].(bool) {
		t.Error("event within threshold should not be anomalous")
	}

	// Exceeds threshold (45 s > 30 s) — anomalous.
	e3 := &TelemetryEvent{ID: "entity-1", Timestamp: now.Add(60 * time.Second), Properties: map[string]interface{}{}}
	ad.Detect(e3)
	if !e3.Properties["anomaly"].(bool) {
		t.Error("event exceeding threshold should be anomalous")
	}

	// New entity — first event, not anomalous even if timestamp is far ahead.
	e4 := &TelemetryEvent{ID: "entity-2", Timestamp: now.Add(1 * time.Hour), Properties: map[string]interface{}{}}
	ad.Detect(e4)
	if e4.Properties["anomaly"].(bool) {
		t.Error("first event for a new entity should not be anomalous")
	}
}

func TestAnomalyMultipleEntities(t *testing.T) {
	ad := NewAnomalyDetector()
	now := time.Now()

	eA := &TelemetryEvent{ID: "entity-A", Timestamp: now, Properties: map[string]interface{}{}}
	eB := &TelemetryEvent{ID: "entity-B", Timestamp: now, Properties: map[string]interface{}{}}
	ad.Detect(eA)
	ad.Detect(eB)

	// Both first events — not anomalous.
	if eA.Properties["anomaly"].(bool) || eB.Properties["anomaly"].(bool) {
		t.Error("first events for both entities should not be anomalous")
	}

	// Advance entity-A by 60 s (> threshold).
	eA2 := &TelemetryEvent{ID: "entity-A", Timestamp: now.Add(60 * time.Second), Properties: map[string]interface{}{}}
	ad.Detect(eA2)
	if !eA2.Properties["anomaly"].(bool) {
		t.Error("entity-A second event should be anomalous")
	}

	// Advance entity-B by only 5 s (< threshold).
	eB2 := &TelemetryEvent{ID: "entity-B", Timestamp: now.Add(5 * time.Second), Properties: map[string]interface{}{}}
	ad.Detect(eB2)
	if eB2.Properties["anomaly"].(bool) {
		t.Error("entity-B second event should not be anomalous (tracked independently)")
	}
}
