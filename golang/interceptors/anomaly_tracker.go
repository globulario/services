package interceptors

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// anomalyTracker detects application-layer attacks by monitoring
// request durations and error rates per source IP.
//
// Detects:
// - Slowloris / slow-read attacks (abnormally long requests)
// - Error rate spikes (resource exhaustion, cascade failures)
// - Expensive query abuse (one IP causing high server load)
type anomalyTracker struct {
	mu sync.Mutex

	// Per-IP slow request counts (requests > slowThreshold in current window).
	slowCounts map[string]*atomic.Int64

	// Global error counter for the current window.
	errorCount atomic.Int64
	totalCount atomic.Int64

	// Thresholds.
	slowThreshold     time.Duration // request duration considered "slow"
	slowAlertCount    int64         // slow requests from one IP before alerting
	errorRatePercent  float64       // error percentage that triggers alert
	minRequestsForErr int64         // minimum requests before error rate is meaningful

	// Cooldown: don't re-alert for the same condition within a window.
	alerted map[string]time.Time
	window  time.Duration
}

var (
	anomalyInstance *anomalyTracker
	anomalyOnce    sync.Once
)

func getAnomalyTracker() *anomalyTracker {
	anomalyOnce.Do(func() {
		anomalyInstance = &anomalyTracker{
			slowCounts:        make(map[string]*atomic.Int64),
			slowThreshold:     30 * time.Second,  // 30s+ = abnormally slow
			slowAlertCount:    10,                 // 10+ slow requests from one IP
			errorRatePercent:  50.0,               // 50%+ error rate = something is wrong
			minRequestsForErr: 50,                 // need at least 50 requests to judge
			alerted:           make(map[string]time.Time),
			window:            60 * time.Second,
		}
		go anomalyInstance.sweepLoop()
	})
	return anomalyInstance
}

// record tracks a completed request for anomaly detection.
func (at *anomalyTracker) record(remoteAddr, method string, duration time.Duration, isError bool) {
	at.totalCount.Add(1)
	if isError {
		at.errorCount.Add(1)
	}

	// Strip port from IP.
	ip := stripPort(remoteAddr)
	if ip == "" || ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return
	}

	// Track slow requests per IP.
	if duration >= at.slowThreshold {
		at.mu.Lock()
		counter, exists := at.slowCounts[ip]
		if !exists {
			counter = &atomic.Int64{}
			at.slowCounts[ip] = counter
		}
		at.mu.Unlock()

		count := counter.Add(1)
		if count == at.slowAlertCount {
			at.fireAlert(ip, "slowloris_detected", fmt.Sprintf(
				"%d requests > %s from %s (last: %s on %s)",
				count, at.slowThreshold, ip, duration.Round(time.Millisecond), method,
			))
		}
	}

	// Check global error rate periodically (every 100 requests).
	total := at.totalCount.Load()
	if total > 0 && total%100 == 0 && total >= at.minRequestsForErr {
		errCount := at.errorCount.Load()
		rate := float64(errCount) / float64(total) * 100
		if rate >= at.errorRatePercent {
			at.fireAlert("global", "error_rate_spike", fmt.Sprintf(
				"%.1f%% error rate (%d/%d requests in window)",
				rate, errCount, total,
			))
		}
	}
}

// fireAlert publishes a security event if not in cooldown.
func (at *anomalyTracker) fireAlert(key, reason, detail string) {
	at.mu.Lock()
	defer at.mu.Unlock()

	if last, ok := at.alerted[key+":"+reason]; ok && time.Since(last) < at.window {
		return
	}
	at.alerted[key+":"+reason] = time.Now()

	if OnSecurityEvent != nil {
		OnSecurityEvent(&AuditDecision{
			Timestamp:     time.Now().UTC(),
			Subject:       "",
			PrincipalType: "unknown",
			AuthMethod:    "none",
			GRPCMethod:    detail,
			RemoteAddr:    key,
			Allowed:       false,
			Reason:        reason,
			CallSource:    "remote",
		})
	}
}

// sweepLoop resets counters and expires cooldowns.
func (at *anomalyTracker) sweepLoop() {
	ticker := time.NewTicker(at.window)
	defer ticker.Stop()
	for range ticker.C {
		at.mu.Lock()
		at.slowCounts = make(map[string]*atomic.Int64)
		now := time.Now()
		for k, t := range at.alerted {
			if now.Sub(t) > 2*at.window {
				delete(at.alerted, k)
			}
		}
		at.mu.Unlock()
		at.errorCount.Store(0)
		at.totalCount.Store(0)
	}
}

// stripPort extracts IP from "ip:port" or returns as-is.
func stripPort(addr string) string {
	if addr == "" || addr == "unknown" {
		return ""
	}
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
