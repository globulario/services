package main

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// workflowHealthGate is a circuit breaker that prevents dispatching workflows
// when the workflow backend is unhealthy (repeated RPC failures). It opens
// after a threshold of failures within a rolling window, and auto-closes
// after a cooldown period via a half-open probe.
type workflowHealthGate struct {
	mu               sync.Mutex
	failures         []time.Time
	windowSize       time.Duration
	failureThreshold int
	circuitOpenUntil time.Time
	cooldownPeriod   time.Duration
	halfOpenProbe    atomic.Bool
}

func newWorkflowHealthGate() *workflowHealthGate {
	return &workflowHealthGate{
		windowSize:       5 * time.Minute,
		failureThreshold: 5,
		cooldownPeriod:   30 * time.Second,
	}
}

// Check returns nil if dispatch is allowed, or an error if the circuit is open.
// In half-open state, exactly one probe request is allowed through.
func (g *workflowHealthGate) Check() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pruneOld()

	if time.Now().Before(g.circuitOpenUntil) {
		// Half-open: allow exactly one probe.
		if g.halfOpenProbe.CompareAndSwap(false, true) {
			return nil
		}
		workflowDispatchRejectedTotal.Inc()
		return fmt.Errorf("workflow circuit breaker open: %d failures in %s, retry after %s",
			len(g.failures), g.windowSize, time.Until(g.circuitOpenUntil).Round(time.Second))
	}
	return nil
}

// RecordFailure records an RPC-level failure. Only transport/infrastructure
// errors should trigger this — not business-level workflow failures.
func (g *workflowHealthGate) RecordFailure() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.failures = append(g.failures, time.Now())
	g.pruneOld()

	if len(g.failures) >= g.failureThreshold {
		g.circuitOpenUntil = time.Now().Add(g.cooldownPeriod)
		g.halfOpenProbe.Store(false)
		workflowCircuitBreakerOpenTotal.Inc()
		log.Printf("workflow health gate: circuit OPEN — %d failures in %s, cooldown %s",
			len(g.failures), g.windowSize, g.cooldownPeriod)
	}
}

// RecordSuccess closes the circuit and resets the failure window.
func (g *workflowHealthGate) RecordSuccess() {
	g.mu.Lock()
	defer g.mu.Unlock()
	wasOpen := !g.circuitOpenUntil.IsZero() && time.Now().Before(g.circuitOpenUntil)
	g.circuitOpenUntil = time.Time{}
	g.halfOpenProbe.Store(false)
	g.failures = nil
	if wasOpen {
		log.Printf("workflow health gate: circuit CLOSED — probe succeeded")
	}
}

// IsOpen returns true if the circuit breaker is currently open (dispatch blocked).
// This is a read-only probe — it does NOT advance the half-open state.
func (g *workflowHealthGate) IsOpen() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pruneOld()
	return time.Now().Before(g.circuitOpenUntil)
}

func (g *workflowHealthGate) pruneOld() {
	cutoff := time.Now().Add(-g.windowSize)
	i := 0
	for i < len(g.failures) && g.failures[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		g.failures = g.failures[i:]
	}
}

// dispatchRegistry provides cross-path deduplication for package dispatches.
// Both the drift reconciler and the release pipeline register their in-flight
// work here so they don't collide.
//
// Key format: "nodeID/KIND/pkgName"
type dispatchRegistry struct {
	mu       sync.Mutex
	inflight map[string]dispatchRecord
	ttl      time.Duration
}

type dispatchRecord struct {
	source    string
	startedAt time.Time
}

func newDispatchRegistry() *dispatchRegistry {
	return &dispatchRegistry{
		inflight: make(map[string]dispatchRecord),
		ttl:      15 * time.Minute,
	}
}

// TryAcquire attempts to register a dispatch. Returns true if acquired,
// false if already held by another source.
func (r *dispatchRegistry) TryAcquire(key, source string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expireLocked()

	if existing, ok := r.inflight[key]; ok {
		dispatchDedupSuppressedTotal.WithLabelValues(source, existing.source).Inc()
		log.Printf("dispatch dedup: %s blocked — already held by %s (since %s)",
			key, existing.source, time.Since(existing.startedAt).Round(time.Second))
		return false
	}
	r.inflight[key] = dispatchRecord{source: source, startedAt: time.Now()}
	return true
}

// Release removes a dispatch registration.
func (r *dispatchRegistry) Release(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.inflight, key)
}

func (r *dispatchRegistry) expireLocked() {
	cutoff := time.Now().Add(-r.ttl)
	for key, rec := range r.inflight {
		if rec.startedAt.Before(cutoff) {
			delete(r.inflight, key)
		}
	}
}

// reconcileCircuitBreaker tracks reconcile outcomes and opens when the
// reconcile loop is failing consistently, preventing useless dispatches
// that create backlog in the workflow service.
type reconcileCircuitBreaker struct {
	mu               sync.Mutex
	timeouts         []time.Time   // timestamps of recent reconcile timeouts
	windowSize       time.Duration // rolling window
	timeoutThreshold int           // consecutive timeouts to trip
	openUntil        time.Time     // zero = closed
	cooldownPeriod   time.Duration
	halfOpenProbe    atomic.Bool
}

func newReconcileCircuitBreaker() *reconcileCircuitBreaker {
	return &reconcileCircuitBreaker{
		windowSize:       10 * time.Minute,
		timeoutThreshold: 3,
		cooldownPeriod:   2 * time.Minute,
	}
}

// Allow returns nil if reconcile dispatch is allowed, or an error if the
// circuit is open.
func (cb *reconcileCircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.pruneOld()

	if time.Now().Before(cb.openUntil) {
		// Half-open: allow one probe to see if things recovered.
		if cb.halfOpenProbe.CompareAndSwap(false, true) {
			log.Printf("reconcile circuit breaker: half-open probe allowed")
			return nil
		}
		reconcileCircuitOpenTotal.Inc()
		return fmt.Errorf("reconcile circuit breaker open: %d timeouts in %s, retry after %s",
			len(cb.timeouts), cb.windowSize, time.Until(cb.openUntil).Round(time.Second))
	}
	return nil
}

// RecordTimeout records a reconcile timeout/failure.
func (cb *reconcileCircuitBreaker) RecordTimeout() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.timeouts = append(cb.timeouts, time.Now())
	cb.pruneOld()

	if len(cb.timeouts) >= cb.timeoutThreshold {
		cb.openUntil = time.Now().Add(cb.cooldownPeriod)
		cb.halfOpenProbe.Store(false)
		reconcileCircuitOpenTotal.Inc()
		log.Printf("reconcile circuit breaker: OPEN — %d timeouts in %s, cooldown %s",
			len(cb.timeouts), cb.windowSize, cb.cooldownPeriod)
	}
}

// RecordSuccess closes the circuit and resets the timeout window.
func (cb *reconcileCircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	wasOpen := !cb.openUntil.IsZero() && time.Now().Before(cb.openUntil)
	cb.openUntil = time.Time{}
	cb.halfOpenProbe.Store(false)
	cb.timeouts = nil
	if wasOpen {
		log.Printf("reconcile circuit breaker: CLOSED — reconcile succeeded")
	}
}

// IsOpen returns true if the reconcile circuit breaker is currently open.
// Read-only probe — does NOT advance the half-open state.
func (cb *reconcileCircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.pruneOld()
	return time.Now().Before(cb.openUntil)
}

func (cb *reconcileCircuitBreaker) pruneOld() {
	cutoff := time.Now().Add(-cb.windowSize)
	i := 0
	for i < len(cb.timeouts) && cb.timeouts[i].Before(cutoff) {
		i++
	}
	if i > 0 {
		cb.timeouts = cb.timeouts[i:]
	}
}
