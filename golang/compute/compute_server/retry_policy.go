// retry_policy.go implements bounded retry decisions for failed compute units.
//
// Retry decisions are based on:
//   - The definition's IdempotencyMode (SAFE_RETRY, RETRY_WITH_CLEANUP, NO_AUTOMATIC_RETRY)
//   - The definition's DeterminismLevel (DETERMINISTIC failures won't produce different results)
//   - The unit's FailureClass (some failures are retryable, others are terminal)
//   - The unit's attempt count vs maxAttempts
//
// No infinite loops — every unit has a hard max attempt limit.
package main

import (
	"log/slog"

	"github.com/globulario/services/golang/compute/computepb"
)

const defaultMaxAttempts = 3

// retryDecision describes whether a failed unit should be retried.
type retryDecision struct {
	ShouldRetry bool
	Reason      string
}

// shouldRetryUnit evaluates whether a failed unit should be retried based on
// the definition's policy and the failure classification.
func shouldRetryUnit(def *computepb.ComputeDefinition, unit *computepb.ComputeUnit) retryDecision {
	maxAttempts := defaultMaxAttempts

	// Check idempotency mode.
	switch def.GetIdempotencyMode() {
	case computepb.IdempotencyMode_NO_AUTOMATIC_RETRY:
		return retryDecision{false, "idempotency mode is NO_AUTOMATIC_RETRY"}
	case computepb.IdempotencyMode_RETRY_WITH_CLEANUP:
		// Allowed but would need cleanup — for now, treat as safe.
	case computepb.IdempotencyMode_SAFE_RETRY, computepb.IdempotencyMode_IDEMPOTENCY_MODE_UNSPECIFIED:
		// Safe to retry.
	}

	// Check attempt count.
	if int(unit.GetAttempt()) >= maxAttempts {
		return retryDecision{false, "max attempts reached"}
	}

	// Classify the failure.
	switch unit.GetFailureClass() {
	case computepb.FailureClass_DETERMINISTIC_REPEAT_FAILURE:
		// Deterministic failures produce the same result on retry — never retry.
		return retryDecision{false, "deterministic repeat failure"}

	case computepb.FailureClass_OUTPUT_VERIFICATION_FAILED:
		// If the definition is DETERMINISTIC, the same output will be produced.
		if def.GetDeterminismLevel() == computepb.DeterminismLevel_DETERMINISTIC {
			return retryDecision{false, "verification failed on deterministic definition — retry would produce same result"}
		}
		// Non-deterministic definitions might produce different output.
		return retryDecision{true, "verification failed on non-deterministic definition — retry may produce different output"}

	case computepb.FailureClass_POLICY_BLOCKED:
		// Policy blocks never resolve on retry.
		return retryDecision{false, "policy blocked — not retryable"}

	case computepb.FailureClass_AGGREGATION_BLOCKED:
		// Aggregation issues don't resolve by re-running a unit.
		return retryDecision{false, "aggregation blocked — not retryable"}

	case computepb.FailureClass_NODE_UNREACHABLE,
		computepb.FailureClass_LEASE_EXPIRED,
		computepb.FailureClass_RESOURCE_EXHAUSTED:
		// Transient infrastructure failures — always retry.
		return retryDecision{true, "transient infrastructure failure"}

	case computepb.FailureClass_ARTIFACT_FETCH_FAILED,
		computepb.FailureClass_INPUT_MISSING:
		// Input issues might be transient (MinIO momentarily unavailable).
		return retryDecision{true, "input/artifact fetch failure — may be transient"}

	case computepb.FailureClass_OUTPUT_UPLOAD_FAILED:
		// Upload failures are usually transient.
		return retryDecision{true, "output upload failure — may be transient"}

	case computepb.FailureClass_EXECUTION_NONZERO_EXIT:
		// Non-zero exit: retry only if non-deterministic.
		if def.GetDeterminismLevel() == computepb.DeterminismLevel_DETERMINISTIC {
			// Mark as deterministic repeat failure to prevent further retries.
			return retryDecision{false, "execution failed on deterministic definition — retry would produce same failure"}
		}
		return retryDecision{true, "execution failed on non-deterministic definition"}

	default:
		// Unclassified failures: retry if policy allows.
		return retryDecision{true, "unclassified failure — retry allowed by default"}
	}
}

// classifyStageFailure returns the appropriate FailureClass for a staging error.
func classifyStageFailure(err error) computepb.FailureClass {
	if err == nil {
		return computepb.FailureClass_FAILURE_CLASS_UNSPECIFIED
	}
	errStr := err.Error()
	switch {
	case contains(errStr, "not found", "NoSuchKey", "does not exist"):
		return computepb.FailureClass_INPUT_MISSING
	case contains(errStr, "dial", "connection refused", "unreachable"):
		return computepb.FailureClass_NODE_UNREACHABLE
	case contains(errStr, "checksum mismatch"):
		return computepb.FailureClass_ARTIFACT_FETCH_FAILED
	default:
		return computepb.FailureClass_ARTIFACT_FETCH_FAILED
	}
}

// classifyRunFailure returns the appropriate FailureClass for a run dispatch error.
func classifyRunFailure(err error) computepb.FailureClass {
	if err == nil {
		return computepb.FailureClass_FAILURE_CLASS_UNSPECIFIED
	}
	errStr := err.Error()
	switch {
	case contains(errStr, "dial", "connection refused", "unreachable"):
		return computepb.FailureClass_NODE_UNREACHABLE
	default:
		return computepb.FailureClass_EXECUTION_NONZERO_EXIT
	}
}

func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// logRetryDecision logs the retry decision for observability.
func logRetryDecision(unit *computepb.ComputeUnit, decision retryDecision) {
	if decision.ShouldRetry {
		slog.Info("compute retry: will retry unit",
			"unit_id", unit.UnitId, "attempt", unit.Attempt,
			"failure_class", unit.FailureClass.String(),
			"reason", decision.Reason)
	} else {
		slog.Info("compute retry: terminal failure — no retry",
			"unit_id", unit.UnitId, "attempt", unit.Attempt,
			"failure_class", unit.FailureClass.String(),
			"reason", decision.Reason)
	}
}
