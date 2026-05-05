package main

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/installed_state"
)

func TestDriftSuppressedBlockedOutcomes(t *testing.T) {
	blocked := []installed_state.ConvergenceOutcome{
		installed_state.OutcomeBlockedMissingNativeDep,
		installed_state.OutcomeBlockedCriticalKeyMissing,
		installed_state.OutcomeBlockedNodeUnreachable,
		installed_state.OutcomeFailedPermanent,
	}
	for _, outcome := range blocked {
		r := &installed_state.ConvergenceResultV1{Outcome: outcome}
		conv := map[string]*installed_state.ConvergenceResultV1{"workflow": r}
		if !driftSuppressed(conv, "workflow", "node1", "n1") {
			t.Errorf("outcome %s should suppress drift", outcome)
		}
	}
}

func TestDriftSuppressedSuccessOutcomesNotSuppressed(t *testing.T) {
	passOutcomes := []installed_state.ConvergenceOutcome{
		installed_state.OutcomeSuccessCommitted,
		installed_state.OutcomeSuccessLocalPendingSync,
		installed_state.OutcomeStaleInstalledState,
	}
	for _, outcome := range passOutcomes {
		r := &installed_state.ConvergenceResultV1{Outcome: outcome}
		conv := map[string]*installed_state.ConvergenceResultV1{"workflow": r}
		if driftSuppressed(conv, "workflow", "node1", "n1") {
			t.Errorf("outcome %s should NOT suppress drift", outcome)
		}
	}
}

func TestDriftSuppressedMissingEntry(t *testing.T) {
	// No convergence result → fail-open (don't suppress).
	conv := map[string]*installed_state.ConvergenceResultV1{}
	if driftSuppressed(conv, "workflow", "node1", "n1") {
		t.Error("missing entry should not suppress drift")
	}
}

func TestDriftSuppressedTransientWithBackoff(t *testing.T) {
	// Transient failure that just happened → suppressed (within backoff window).
	r := &installed_state.ConvergenceResultV1{
		Outcome:       installed_state.OutcomeFailedTransient,
		AttemptCount:  1,
		LastAttemptAt: time.Now().Add(-30 * time.Second).Unix(), // 30s ago, backoff=2min
	}
	conv := map[string]*installed_state.ConvergenceResultV1{"workflow": r}
	if !driftSuppressed(conv, "workflow", "node1", "n1") {
		t.Error("transient failure within backoff window should suppress drift")
	}
}

func TestDriftSuppressedTransientBackoffExpired(t *testing.T) {
	// Transient failure from 10 minutes ago → backoff expired, allow re-dispatch.
	r := &installed_state.ConvergenceResultV1{
		Outcome:       installed_state.OutcomeFailedTransient,
		AttemptCount:  1,
		LastAttemptAt: time.Now().Add(-10 * time.Minute).Unix(), // 10min ago, backoff=2min
	}
	conv := map[string]*installed_state.ConvergenceResultV1{"workflow": r}
	if driftSuppressed(conv, "workflow", "node1", "n1") {
		t.Error("transient failure with expired backoff should allow re-dispatch")
	}
}

func TestConvergenceBackoffValues(t *testing.T) {
	cases := []struct {
		attempts int32
		minD     time.Duration
		maxD     time.Duration
	}{
		{0, 2 * time.Minute, 2*time.Minute + 1},
		{1, 2 * time.Minute, 2*time.Minute + 1},
		{2, 4 * time.Minute, 4*time.Minute + 1},
		{3, 8 * time.Minute, 8*time.Minute + 1},
		{4, 16 * time.Minute, 16*time.Minute + 1},
		{5, 30 * time.Minute, 30*time.Minute + 1}, // capped
		{10, 30 * time.Minute, 30*time.Minute + 1}, // capped
	}
	for _, tc := range cases {
		d := convergenceBackoff(tc.attempts)
		if d < tc.minD || d > tc.maxD {
			t.Errorf("convergenceBackoff(%d) = %v, want [%v, %v]",
				tc.attempts, d, tc.minD, tc.maxD)
		}
	}
}
