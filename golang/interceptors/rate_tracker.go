package interceptors

import (
	"sync"
	"sync/atomic"
	"time"
)

// rateTracker detects request floods from individual source IPs.
// Lightweight: one atomic counter per active IP, swept every window.
type rateTracker struct {
	mu       sync.Mutex
	counters map[string]*atomic.Int64
	window   time.Duration
	// threshold is the max requests per window before alerting.
	threshold int64
	// cooldown prevents repeated alerts for the same IP.
	alerted map[string]time.Time
}

var (
	dosTracker *rateTracker
	dosOnce    sync.Once
)

// getDosTracker returns the singleton DoS rate tracker.
func getDosTracker() *rateTracker {
	dosOnce.Do(func() {
		dosTracker = &rateTracker{
			counters:  make(map[string]*atomic.Int64),
			window:    30 * time.Second,
			threshold: 500, // 500 requests in 30s from one IP = suspicious
			alerted:   make(map[string]time.Time),
		}
		go dosTracker.sweepLoop()
	})
	return dosTracker
}

// track increments the counter for a source IP and returns true if threshold exceeded.
func (rt *rateTracker) track(remoteAddr string) bool {
	if remoteAddr == "" || remoteAddr == "unknown" {
		return false
	}

	// Strip port — we care about IP, not ephemeral port.
	ip := remoteAddr
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == ':' {
			ip = ip[:i]
			break
		}
	}
	// Skip loopback — local services talk to each other frequently.
	if ip == "127.0.0.1" || ip == "::1" || ip == "localhost" {
		return false
	}

	rt.mu.Lock()
	counter, exists := rt.counters[ip]
	if !exists {
		counter = &atomic.Int64{}
		rt.counters[ip] = counter
	}
	rt.mu.Unlock()

	count := counter.Add(1)
	if count < rt.threshold {
		return false
	}

	// Check cooldown — don't alert for the same IP within one window.
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if last, ok := rt.alerted[ip]; ok && time.Since(last) < rt.window {
		return false
	}
	rt.alerted[ip] = time.Now()
	return true
}

// sweepLoop resets counters every window.
func (rt *rateTracker) sweepLoop() {
	ticker := time.NewTicker(rt.window)
	defer ticker.Stop()
	for range ticker.C {
		rt.mu.Lock()
		rt.counters = make(map[string]*atomic.Int64)
		// Expire old alerts.
		now := time.Now()
		for ip, t := range rt.alerted {
			if now.Sub(t) > 2*rt.window {
				delete(rt.alerted, ip)
			}
		}
		rt.mu.Unlock()
	}
}
