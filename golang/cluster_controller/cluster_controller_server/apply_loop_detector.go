package main

import (
	"log"
	"sync"
	"time"
)

// applyLoopDetector tracks rolling apply counts per package/node and
// quarantines targets that are being applied repeatedly without convergence.
// Quarantine is scoped — only the specific package/node is blocked; the rest
// of the cluster continues normally.
//
// Key format: "nodeID/KIND/pkgName" (same as drift reconciler inflight keys).
type applyLoopDetector struct {
	mu         sync.Mutex
	applies    map[string][]time.Time // rolling apply timestamps
	quarantine map[string]time.Time   // key → quarantine expiry

	// Config.
	windowSize       time.Duration // rolling window for counting applies
	applyThreshold   int           // max applies in window before quarantine
	quarantinePeriod time.Duration // how long to quarantine
}

func newApplyLoopDetector() *applyLoopDetector {
	return &applyLoopDetector{
		applies:          make(map[string][]time.Time),
		quarantine:       make(map[string]time.Time),
		windowSize:       10 * time.Minute,
		applyThreshold:   5,
		quarantinePeriod: 5 * time.Minute,
	}
}

// IsQuarantined returns true if the given key is currently quarantined.
func (d *applyLoopDetector) IsQuarantined(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	expiry, ok := d.quarantine[key]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		// Quarantine expired — auto-clear.
		delete(d.quarantine, key)
		log.Printf("apply-loop: quarantine expired for %s", key)
		return false
	}
	return true
}

// RecordApply records an apply dispatch and checks if the threshold is
// exceeded. Returns true if the target was just quarantined.
func (d *applyLoopDetector) RecordApply(key string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()

	// Prune old timestamps.
	cutoff := now.Add(-d.windowSize)
	timestamps := d.applies[key]
	i := 0
	for i < len(timestamps) && timestamps[i].Before(cutoff) {
		i++
	}
	timestamps = append(timestamps[i:], now)
	d.applies[key] = timestamps

	if len(timestamps) >= d.applyThreshold {
		// Check if already quarantined (avoid re-logging).
		if _, ok := d.quarantine[key]; ok {
			return false
		}
		d.quarantine[key] = now.Add(d.quarantinePeriod)
		applyLoopDetectedTotal.Inc()
		log.Printf("apply-loop: QUARANTINED %s — %d applies in %s without convergence, blocked for %s",
			key, len(timestamps), d.windowSize, d.quarantinePeriod)
		return true
	}
	return false
}

// ClearQuarantine removes a quarantine (operator override).
func (d *applyLoopDetector) ClearQuarantine(key string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.quarantine, key)
	delete(d.applies, key)
}

// QuarantinedKeys returns all currently quarantined keys (for doctor findings).
func (d *applyLoopDetector) QuarantinedKeys() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now()
	var keys []string
	for key, expiry := range d.quarantine {
		if now.Before(expiry) {
			keys = append(keys, key)
		} else {
			delete(d.quarantine, key)
		}
	}
	return keys
}
