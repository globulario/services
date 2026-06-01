// @awareness namespace=globular.platform
// @awareness component=platform_remediation
// @awareness file_role=remediation_outcome_recording
// @awareness implements=globular.platform:intent.remediation.failure_rate_policy
// @awareness risk=high
package remediation

import (
	"strings"
	"sync"
	"time"
)

// RemediationStatus is the public state of a remediation attempt. A
// dispatched action that has not yet been verified is PENDING — never
// SUCCEEDED. See docs/intent/workflow.remediation_truth_consistency.yaml.
type RemediationStatus string

const (
	// StatusPending — the action was dispatched but the verification
	// step has not yet confirmed invariant resolution.
	StatusPending RemediationStatus = "PENDING_VERIFICATION"

	// StatusSucceeded — verification confirmed the underlying invariant
	// cleared. Only this status counts as success.
	StatusSucceeded RemediationStatus = "SUCCEEDED"

	// StatusDegraded — verification ran but the invariant is still
	// present (partial fix, related findings remain, etc.).
	StatusDegraded RemediationStatus = "DEGRADED"

	// StatusFailed — the action failed to dispatch or the verification
	// step returned a clear negative.
	StatusFailed RemediationStatus = "FAILED"
)

// Outcome is the contract for what a remediation actually achieved. The
// workflow engine MUST populate Dispatched, Verified, and FindingResolved
// before reporting a terminal status. A workflow that sets terminal status
// without these fields violates the truth-consistency invariant.
type Outcome struct {
	FindingID       string
	WorkflowRunID   string
	Dispatched      bool      // executor accepted and ran the action
	Verified        bool      // verify_convergence step completed
	FindingResolved bool      // verify_convergence found that the original finding cleared
	DispatchError   string    // non-empty when Dispatched == false
	VerifiedAt      time.Time // when Verified was confirmed
}

// IsSuccess reports whether this remediation may be reported as terminal
// success to operators. Dispatch alone is never success — verification
// must have run AND the underlying finding must have cleared.
func (o Outcome) IsSuccess() bool {
	return o.Dispatched && o.Verified && o.FindingResolved
}

// Status returns the workflow-engine-facing status. The mapping is total:
// every possible combination of flags returns one of the constants.
func (o Outcome) Status() RemediationStatus {
	switch {
	case !o.Dispatched:
		return StatusFailed
	case !o.Verified:
		return StatusPending
	case o.Verified && o.FindingResolved:
		return StatusSucceeded
	default:
		return StatusDegraded
	}
}

// Reason returns a one-line explanation suitable for workflow run output
// or operator dashboards. Never returns "" — every status has an answer.
func (o Outcome) Reason() string {
	switch o.Status() {
	case StatusSucceeded:
		return "remediation verified: " + o.FindingID + " cleared"
	case StatusPending:
		return "dispatched; awaiting verification of " + o.FindingID
	case StatusDegraded:
		return "verified but " + o.FindingID + " still present — partial resolution"
	case StatusFailed:
		if strings.TrimSpace(o.DispatchError) != "" {
			return "dispatch failed: " + o.DispatchError
		}
		return "dispatch failed for " + o.FindingID
	}
	return "unknown remediation outcome for " + o.FindingID
}

// ─────────────────────────────────────────────────────────────────────
// ActiveFindings — a process-local registry that keeps findings in the
// "active" set until a remediation Outcome with IsSuccess()==true is
// recorded against them. Production wiring will replace this with the
// doctor's finding cache; the in-memory implementation lets tests and
// callers reason about the contract without that dependency.
// ─────────────────────────────────────────────────────────────────────

type ActiveFindings struct {
	mu     sync.Mutex
	active map[string]struct{}
}

func NewActiveFindings(seed ...string) *ActiveFindings {
	a := &ActiveFindings{active: make(map[string]struct{})}
	for _, id := range seed {
		a.active[id] = struct{}{}
	}
	return a
}

// IsActive reports whether the finding is still in the active set.
func (a *ActiveFindings) IsActive(findingID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.active[findingID]
	return ok
}

// Record marks the outcome of a remediation attempt. The finding is
// removed from the active set only when the outcome reports IsSuccess.
// Pending / Degraded / Failed all keep the finding active so operators
// see it on the next doctor sweep.
func (a *ActiveFindings) Record(o Outcome) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if o.IsSuccess() {
		delete(a.active, o.FindingID)
	}
}
