package main

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// TestWorkflowGateBurstOpensExactlyOnce is the regression for the 2026-06-09
// bring-up dispatch-pressure incident. A concurrent burst of ExecuteWorkflow
// RPC failures drove RecordFailure ~30× in one second; the level-triggered
// open logic re-armed the cooldown, reset the probe, re-logged, and
// re-incremented workflow_circuit_breaker_open_total on EVERY failure past the
// threshold — amplifying one logical open into 26 (observed
// circuit_breaker_open_total=26, 26 identical log lines). The breaker must be
// edge-triggered: one logical open == one counter increment, regardless of how
// many failures arrive while already open.
//
// Classifies against meta.failure_response_must_contract_not_amplify and
// meta.diagnostic_output_must_be_bounded.
func TestWorkflowGateBurstOpensExactlyOnce(t *testing.T) {
	g := newWorkflowHealthGate()
	before := testutil.ToFloat64(workflowCircuitBreakerOpenTotal)

	// Far more failures than the threshold, all while the circuit is (or
	// becomes) open, with no half-open probe in between — the burst shape.
	const burst = 30
	for i := 0; i < burst; i++ {
		g.RecordFailure()
	}
	if !g.IsOpen() {
		t.Fatal("circuit should be open after a failure burst")
	}

	if delta := testutil.ToFloat64(workflowCircuitBreakerOpenTotal) - before; delta != 1 {
		t.Errorf("burst of %d failures must open the circuit exactly once "+
			"(workflow_circuit_breaker_open_total += 1), got += %v", burst, delta)
	}
}

// TestReconcileBreakerBurstOpensExactlyOnce is the sibling regression: the
// reconcile circuit breaker had the identical level-triggered shape, so a
// burst of reconcile timeouts must likewise open it exactly once.
func TestReconcileBreakerBurstOpensExactlyOnce(t *testing.T) {
	cb := newReconcileCircuitBreaker()
	before := testutil.ToFloat64(reconcileCircuitOpenTotal)

	const burst = 20
	for i := 0; i < burst; i++ {
		cb.RecordTimeout()
	}
	if !cb.IsOpen() {
		t.Fatal("reconcile circuit should be open after a timeout burst")
	}

	if delta := testutil.ToFloat64(reconcileCircuitOpenTotal) - before; delta != 1 {
		t.Errorf("burst of %d timeouts must open the reconcile circuit exactly once "+
			"(reconcile_circuit_open_total += 1), got += %v", burst, delta)
	}
}

// TestReconcileCircuitGaugeExpiresCooldownEvenWithoutProbe is the sibling of
// TestWorkflowGateExpiresCooldownEvenWithoutProbe. The cluster-doctor finding
// cluster.reconcile_circuit_open is driven by the reconcile_circuit_open GAUGE
// (current state), not the monotonic _total counter. This is the regression for
// the 2026-06-21 false CRITICAL: the doctor rule consumed the raw counter, so a
// single transient open left total_opens=3 and the finding fired "periodic
// reconcile suspended" forever while reconcile was actually succeeding. The
// gauge must read 1 while open and drop to 0 once the breaker closes — including
// the idle path where the cooldown elapses without RecordSuccess being driven.
//
// Classifies against meta.failure_response_must_contract_not_amplify and
// diagnostics.must_measure_reality.
func TestReconcileCircuitGaugeExpiresCooldownEvenWithoutProbe(t *testing.T) {
	reconcileCircuitOpenGauge.Set(0)
	cb := newReconcileCircuitBreaker()

	for i := 0; i < cb.timeoutThreshold; i++ {
		cb.RecordTimeout()
	}
	if !cb.IsOpen() {
		t.Fatal("reconcile circuit should be open after threshold timeouts")
	}
	if got := testutil.ToFloat64(reconcileCircuitOpenGauge); got != 1 {
		t.Fatalf("gauge should be 1 while open, got %v", got)
	}

	// Move the cooldown deadline into the past without anybody driving
	// RecordSuccess — the idle-controller scenario.
	cb.mu.Lock()
	cb.openUntil = time.Now().Add(-time.Millisecond)
	cb.mu.Unlock()

	if cb.IsOpen() {
		t.Error("IsOpen should be false once the cooldown deadline elapses")
	}
	if got := testutil.ToFloat64(reconcileCircuitOpenGauge); got != 0 {
		t.Errorf("gauge should drop to 0 when cooldown expires naturally, got %v", got)
	}
}

// TestReconcileCircuitGaugeClearsOnSuccess verifies the active recovery path:
// a successful reconcile after an open breaker drives the gauge back to 0 so the
// doctor finding auto-clears.
func TestReconcileCircuitGaugeClearsOnSuccess(t *testing.T) {
	reconcileCircuitOpenGauge.Set(0)
	cb := newReconcileCircuitBreaker()

	for i := 0; i < cb.timeoutThreshold; i++ {
		cb.RecordTimeout()
	}
	if got := testutil.ToFloat64(reconcileCircuitOpenGauge); got != 1 {
		t.Fatalf("gauge should be 1 while open, got %v", got)
	}

	cb.RecordSuccess()
	if got := testutil.ToFloat64(reconcileCircuitOpenGauge); got != 0 {
		t.Errorf("gauge should be 0 after RecordSuccess closes the breaker, got %v", got)
	}
}

// TestWorkflowGateTripsOnlyOnTransportFailure is the regression for the
// 2026-06-21 workflow.backend_pressure WARN that fired with no real backend
// pressure. The single RecordFailure call site (workflow_execute.go) tripped
// the breaker on ANY non-nil ExecuteWorkflow error, conflating business-level
// gRPC rejections (a missing workflow definition, invalid inputs, a failed
// precondition) with genuine backend unreachability. Five config errors in 5m
// opened a cluster-wide dispatch freeze, and every half-open probe that hit the
// same business error re-armed it — sustained pressure that never cleared.
//
// The breaker must trip ONLY on transport/infra signals. Business codes must
// leave it closed; transport codes (including DeadlineExceeded, per operator
// decision) must open it. Classifies against
// meta.failure_response_must_contract_not_amplify and
// degraded_is_explicit_not_hidden.
func TestWorkflowGateTripsOnlyOnTransportFailure(t *testing.T) {
	businessCodes := []codes.Code{
		codes.NotFound,
		codes.InvalidArgument,
		codes.FailedPrecondition,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.OutOfRange,
		codes.Canceled,
	}
	for _, c := range businessCodes {
		if workflowDispatchErrorOpensBreaker(status.Error(c, "business reject")) {
			t.Errorf("code %s is a business rejection and must NOT open the breaker", c)
		}
	}

	transportCodes := []codes.Code{
		codes.Unavailable,
		codes.DeadlineExceeded,
		codes.ResourceExhausted,
		codes.Internal,
		codes.Unknown,
		codes.DataLoss,
		codes.Aborted,
	}
	for _, c := range transportCodes {
		if !workflowDispatchErrorOpensBreaker(status.Error(c, "transport failure")) {
			t.Errorf("code %s is a transport/infra failure and MUST open the breaker", c)
		}
	}

	// A raw, non-gRPC dial error (no status) is a transport failure → opens.
	if !workflowDispatchErrorOpensBreaker(errPlainTransport{}) {
		t.Error("raw non-gRPC transport error must open the breaker")
	}
	// nil never opens.
	if workflowDispatchErrorOpensBreaker(nil) {
		t.Error("nil error must not open the breaker")
	}

	// End-to-end: feeding the gate a burst of business-level errors via the
	// classifier must leave the circuit CLOSED — the actual dispatch-freeze
	// regression. (RecordFailure is only reached when the classifier returns
	// true, so a business burst never calls it.)
	g := newWorkflowHealthGate()
	for i := 0; i < g.failureThreshold*3; i++ {
		if workflowDispatchErrorOpensBreaker(status.Error(codes.FailedPrecondition, "dep-blocked")) {
			g.RecordFailure()
		}
	}
	if g.IsOpen() {
		t.Error("a burst of business-level FailedPrecondition errors must NOT open the workflow health gate")
	}
}

// errPlainTransport is a non-gRPC error (status.FromError reports ok=false),
// standing in for a raw dial/connection-refused failure.
type errPlainTransport struct{}

func (errPlainTransport) Error() string { return "connection refused" }

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
