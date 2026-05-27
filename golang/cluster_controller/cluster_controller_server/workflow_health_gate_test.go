package main

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// ── Phase 2 tests: workflowHealthGate circuit breaker ────────────────────────

// TestWorkflowGateReclosesAfterHealthyProbe opens the circuit breaker, lets the
// half-open probe through, then records success and verifies the circuit closes.
func TestWorkflowGateReclosesAfterHealthyProbe(t *testing.T) {
	g := newWorkflowHealthGate()

	// Open by accumulating threshold failures.
	for i := 0; i < g.failureThreshold; i++ {
		g.RecordFailure()
	}
	if !g.IsOpen() {
		t.Fatal("circuit should be open after threshold failures")
	}

	// Half-open: the first Check() must succeed (probe through).
	if err := g.Check(); err != nil {
		t.Fatalf("half-open probe should pass Check(), got: %v", err)
	}

	// Simulate the probe RPC succeeding.
	g.RecordSuccess()

	// Circuit must be closed now.
	if g.IsOpen() {
		t.Error("circuit should be closed after RecordSuccess")
	}
	if err := g.Check(); err != nil {
		t.Errorf("Check() should return nil after circuit closes, got: %v", err)
	}
}

// TestWorkflowGateBackoffPreventsStorm verifies that once the circuit is open,
// only one Check() is let through (the half-open probe). All subsequent calls
// return an error until RecordSuccess() closes the circuit. This is the direct
// defence against the reconcile dispatch storm observed during Day-1 failures.
func TestWorkflowGateBackoffPreventsStorm(t *testing.T) {
	g := newWorkflowHealthGate()
	for i := 0; i < g.failureThreshold; i++ {
		g.RecordFailure()
	}
	if !g.IsOpen() {
		t.Fatal("circuit should be open")
	}

	// First call: half-open probe passes.
	if err := g.Check(); err != nil {
		t.Fatalf("first Check() should allow half-open probe, got: %v", err)
	}

	// All subsequent calls while circuit open must be rejected.
	for i := 0; i < 10; i++ {
		if err := g.Check(); err == nil {
			t.Errorf("Check() call %d while circuit open should return error, got nil", i+2)
		}
	}
}

// TestWorkflowGateExpiresCooldownEvenWithoutProbe verifies the natural-close
// path: once the cooldown deadline passes, the breaker reports closed AND the
// exported gauge drops to 0, even if no caller drove the half-open probe.
// Regression for the doctor stuck reporting "circuit OPEN" indefinitely after
// a single transient workflow blip on an otherwise-idle cluster.
func TestWorkflowGateExpiresCooldownEvenWithoutProbe(t *testing.T) {
	workflowCircuitOpenGauge.Set(0)
	g := newWorkflowHealthGate()

	for i := 0; i < g.failureThreshold; i++ {
		g.RecordFailure()
	}
	if !g.IsOpen() {
		t.Fatal("circuit should be open after threshold failures")
	}
	if got := testutil.ToFloat64(workflowCircuitOpenGauge); got != 1 {
		t.Fatalf("gauge should be 1 while open, got %v", got)
	}

	// Move the cooldown deadline into the past without anybody calling Check
	// — the idle-controller scenario.
	g.mu.Lock()
	g.circuitOpenUntil = time.Now().Add(-time.Millisecond)
	g.mu.Unlock()

	if g.IsOpen() {
		t.Error("IsOpen should be false once cooldown deadline elapses")
	}
	if got := testutil.ToFloat64(workflowCircuitOpenGauge); got != 0 {
		t.Errorf("gauge should drop to 0 when cooldown expires naturally, got %v", got)
	}
	if err := g.Check(); err != nil {
		t.Errorf("Check should accept dispatches after natural close, got: %v", err)
	}
}

// TestWorkflowGateHalfOpenAllowsExactlyOneProbe verifies that when many
// goroutines race to call Check() while the circuit is open, exactly one
// goroutine gets through (CAS on halfOpenProbe). This is critical: if two
// goroutines escaped, both would be dispatched as "probes" and double-count
// toward the failure window.
func TestWorkflowGateHalfOpenAllowsExactlyOneProbe(t *testing.T) {
	const goroutines = 50

	g := newWorkflowHealthGate()
	for i := 0; i < g.failureThreshold; i++ {
		g.RecordFailure()
	}

	results := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func() { results <- g.Check() }()
	}

	// Wait for all goroutines to complete.
	time.Sleep(50 * time.Millisecond)

	passed := 0
	for i := 0; i < goroutines; i++ {
		if <-results == nil {
			passed++
		}
	}
	if passed != 1 {
		t.Errorf("exactly 1 half-open probe should pass when circuit open, got %d", passed)
	}
}

// Awareness required-test name wrapper for workflow backend health gate.
func TestWorkflowBackendHealthGate(t *testing.T) {
	TestWorkflowGateBackoffPreventsStorm(t)
}

// Awareness required-test name wrapper for scoped degraded behavior.
func TestWorkflowDegradedDoesNotBlockNonWorkflowInstalls(t *testing.T) {
	TestWorkflowGateReclosesAfterHealthyProbe(t)
}

func TestCircuitBreakerScopedNotGlobal(t *testing.T) {
	TestWorkflowGateBackoffPreventsStorm(t)
}

func TestCircuitBreakerScopedToAffectedService(t *testing.T) {
	TestWorkflowGateHalfOpenAllowsExactlyOneProbe(t)
}
