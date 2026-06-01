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
	// threshold is the max requests per window before alerting (external IPs).
	threshold int64
	// loopbackThreshold is a higher limit for 127.0.0.1/::1/localhost.
	// Normal inter-service traffic on a 28-service node can easily reach
	// thousands of req/s over loopback, so the external threshold (500/30s)
	// would cause constant false alarms. 5000/30s catches genuine storms
	// while ignoring normal operation.
	loopbackThreshold int64
	// cooldown prevents repeated alerts for the same IP.
	alerted map[string]time.Time
}

func isLoopback(ip string) bool {
	return ip == "127.0.0.1" || ip == "::1" || ip == "localhost"
}

var (
	dosTracker *rateTracker
	dosOnce    sync.Once
)

// getDosTracker returns the singleton DoS rate tracker.
func getDosTracker() *rateTracker {
	dosOnce.Do(func() {
		dosTracker = &rateTracker{
			counters:          make(map[string]*atomic.Int64),
			window:            30 * time.Second,
			threshold:         500,  // 500 requests in 30s from one external IP = suspicious
			loopbackThreshold: 5000, // loopback: higher bar — normal inter-service traffic is ~1000-3000/30s
			alerted:           make(map[string]time.Time),
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
	// Loopback is NOT exempt: circular service-to-service calls on the same
	// node are the most dangerous amplification vector (they crashed the host
	// on 2026-03-24). Call-depth guards catch cycles, but rate tracking on
	// loopback provides defense-in-depth against local event storms.

	rt.mu.Lock()
	counter, exists := rt.counters[ip]
	if !exists {
		counter = &atomic.Int64{}
		rt.counters[ip] = counter
	}
	rt.mu.Unlock()

	count := counter.Add(1)
	limit := rt.threshold
	if isLoopback(ip) {
		limit = rt.loopbackThreshold
	}
	if count < limit {
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
