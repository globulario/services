// circuit_breaker.go — shared stream circuit breaker for gRPC streaming clients.
//
// Prevents reconnection storms when a streaming endpoint is persistently
// down. Consumers call RecordFailure / RecordSuccess around stream
// reconnection attempts; the breaker transitions:
//
//   CLOSED → (failThreshold consecutive failures) → OPEN
//   OPEN   → (cooldown elapses)                   → HALF_OPEN
//   HALF_OPEN → success                            → CLOSED
//   HALF_OPEN → failure                            → OPEN
//
// Usage:
//
//	cb := NewStreamCircuitBreaker(3, 30*time.Second)
//	if !cb.Allow() {
//	    // circuit is open, back off
//	}
//	err := reconnect()
//	if err != nil {
//	    cb.RecordFailure()
//	} else {
//	    cb.RecordSuccess()
//	}
package globular_client

import (
	"sync"
	"time"
)

// CircuitState represents the three states of a circuit breaker.
type CircuitState int

const (
	CircuitClosed   CircuitState = iota // Normal — allow attempts.
	CircuitOpen                         // Too many failures — reject attempts.
	CircuitHalfOpen                     // Cooldown elapsed — allow one probe.
)

// StreamCircuitBreaker protects a streaming reconnection loop from
// hammering a dead endpoint.
type StreamCircuitBreaker struct {
	mu             sync.Mutex
	state          CircuitState
	failures       int
	failThreshold  int
	cooldown       time.Duration
	openSince      time.Time
}

// NewStreamCircuitBreaker creates a breaker that opens after failThreshold
// consecutive failures and stays open for cooldown before allowing a probe.
func NewStreamCircuitBreaker(failThreshold int, cooldown time.Duration) *StreamCircuitBreaker {
	return &StreamCircuitBreaker{
		failThreshold: failThreshold,
		cooldown:      cooldown,
	}
}

// Allow returns true if a reconnection attempt is permitted.
// CLOSED and HALF_OPEN allow attempts; OPEN blocks until cooldown elapses.
func (cb *StreamCircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitHalfOpen:
		return true
	case CircuitOpen:
		if time.Since(cb.openSince) >= cb.cooldown {
			cb.state = CircuitHalfOpen
			return true
		}
		return false
	}
	return true
}

// RecordFailure records a failed reconnection attempt.
func (cb *StreamCircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	if cb.state == CircuitHalfOpen || cb.failures >= cb.failThreshold {
		cb.state = CircuitOpen
		cb.openSince = time.Now()
	}
}

// RecordSuccess records a successful reconnection. Resets the breaker to CLOSED.
func (cb *StreamCircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.failures = 0
}

// State returns the current circuit state (for diagnostics).
func (cb *StreamCircuitBreaker) State() CircuitState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	// Check for transition on read.
	if cb.state == CircuitOpen && time.Since(cb.openSince) >= cb.cooldown {
		cb.state = CircuitHalfOpen
	}
	return cb.state
}

// TimeUntilHalfOpen returns how long until the circuit transitions from OPEN
// to HALF_OPEN. Returns 0 if not in OPEN state.
func (cb *StreamCircuitBreaker) TimeUntilHalfOpen() time.Duration {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state != CircuitOpen {
		return 0
	}
	remaining := cb.cooldown - time.Since(cb.openSince)
	if remaining < 0 {
		return 0
	}
	return remaining
}
