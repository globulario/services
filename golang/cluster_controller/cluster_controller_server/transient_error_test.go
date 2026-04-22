package main

import (
	"strings"
	"testing"
)

// isTransientWorkflowError mirrors the strings.Contains classification in
// release_pipeline.go (lines 375-381).  Engine-level errors (preflight
// failures, missing handlers, circuit breakers, unavailable backends) are
// transient — the release stays RESOLVED so the drift reconciler retries.
// Only real execution errors (checksum mismatch, version mismatch, item
// failures) should transition to FAILED.
func isTransientWorkflowError(errMsg string) bool {
	return strings.Contains(errMsg, "preflight") ||
		strings.Contains(errMsg, "no registered handler") ||
		strings.Contains(errMsg, "handler not found") ||
		strings.Contains(errMsg, "Unavailable") ||
		strings.Contains(errMsg, "circuit breaker") ||
		strings.Contains(errMsg, "DeadlineExceeded") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "posture gate")
}

func TestTransientErrorClassification(t *testing.T) {
	tests := []struct {
		name      string
		errMsg    string
		transient bool // true = stays RESOLVED (transient), false = becomes FAILED (permanent)
	}{
		// ── Transient errors: should stay RESOLVED for retry ──
		{
			name:      "preflight with unregistered handlers",
			errMsg:    "preflight release.apply.package: 16 action(s) have no registered handler",
			transient: true,
		},
		{
			name:      "handler not found",
			errMsg:    "handler not found for action controller.release.mark_resolved",
			transient: true,
		},
		{
			name:      "gRPC Unavailable",
			errMsg:    "Unavailable: connection refused",
			transient: true,
		},
		{
			name:      "circuit breaker open",
			errMsg:    "workflow circuit breaker open: 13 failures",
			transient: true,
		},
		{
			name:      "deadline exceeded",
			errMsg:    "DeadlineExceeded: context deadline exceeded",
			transient: true,
		},
		{
			name:      "plain connection refused",
			errMsg:    "connection refused",
			transient: true,
		},

		{
			name:      "posture gate suppression",
			errMsg:    "posture gate: cluster in RECOVERY_ONLY — release.apply.package dispatch suppressed (will retry when posture clears)",
			transient: true,
		},

		// ── Permanent errors: should become FAILED ──
		{
			name:      "per-node item failures",
			errMsg:    "step apply_per_node: 1/2 items failed",
			transient: false,
		},
		{
			name:      "checksum mismatch",
			errMsg:    "install_package: checksum mismatch",
			transient: false,
		},
		{
			name:      "version mismatch",
			errMsg:    "verify_installed: version mismatch",
			transient: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isTransientWorkflowError(tc.errMsg)
			if got != tc.transient {
				verb := "FAILED (permanent)"
				if tc.transient {
					verb = "RESOLVED (transient)"
				}
				t.Errorf("isTransientWorkflowError(%q) = %v, want %v (%s)",
					tc.errMsg, got, tc.transient, verb)
			}
		})
	}
}
