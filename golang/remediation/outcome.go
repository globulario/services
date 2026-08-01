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

	// Subject identity, carried so a verification can be bound to the thing it
	// verified rather than to a finding id alone.
	//
	// EntityRef is NOT NodeID: they coincide for node-scoped findings and
	// diverge for service- or cluster-scoped ones. ClusterID is load-bearing
	// downstream — evidence is indexed by cluster, so an empty ClusterID would
	// place a verification in the cluster-less partition where no
	// cluster-scoped reader will ever find it.
	ClusterID   string
	InvariantID string
	EntityRef   string
	NodeID      string

	// DispatchedAt is when the executor accepted the request. Zero when
	// nothing was dispatched. Never derived from VerifiedAt — verification
	// that cannot be placed after the action proves nothing about it.
	DispatchedAt time.Time
}

// LineageDefect names a way an outcome's provenance is incomplete or
// self-contradictory. Stable constants: a caller must be able to distinguish
// these without parsing prose, and an operator must see WHICH binding failed.
type LineageDefect string

const (
	LineageMissingCluster     LineageDefect = "missing_cluster_id"
	LineageMissingInvariant   LineageDefect = "missing_invariant_id"
	LineageMissingEntity      LineageDefect = "missing_entity_ref"
	LineageMissingWorkflowRun LineageDefect = "missing_workflow_run_id"
	LineageMissingFinding     LineageDefect = "missing_finding_id"
	LineageMissingDispatchAt  LineageDefect = "missing_dispatched_at_after_dispatch"
	LineageVerifiedBefore     LineageDefect = "verified_at_before_dispatched_at"
)

// LineageDefects reports every provenance problem, in stable order.
//
// This does NOT change Status() or IsSuccess(). Remediation status answers "did
// the repair work"; lineage answers "can this outcome be trusted as evidence
// about a specific subject". A repair can genuinely succeed while its
// provenance is too thin to cite — conflating the two would either suppress a
// real success or launder an unattributable one into authoritative evidence.
//
// A verification whose timestamp precedes its dispatch is reported rather than
// silently accepted: it cannot be evidence that the action worked, whatever the
// booleans say.
func (o Outcome) LineageDefects() []LineageDefect {
	var out []LineageDefect
	if strings.TrimSpace(o.FindingID) == "" {
		out = append(out, LineageMissingFinding)
	}
	if strings.TrimSpace(o.ClusterID) == "" {
		out = append(out, LineageMissingCluster)
	}
	if strings.TrimSpace(o.InvariantID) == "" {
		out = append(out, LineageMissingInvariant)
	}
	if strings.TrimSpace(o.EntityRef) == "" {
		out = append(out, LineageMissingEntity)
	}
	if strings.TrimSpace(o.WorkflowRunID) == "" {
		out = append(out, LineageMissingWorkflowRun)
	}
	if o.Dispatched && o.DispatchedAt.IsZero() {
		out = append(out, LineageMissingDispatchAt)
	}
	if !o.DispatchedAt.IsZero() && !o.VerifiedAt.IsZero() && o.VerifiedAt.Before(o.DispatchedAt) {
		out = append(out, LineageVerifiedBefore)
	}
	return out
}

// LineageComplete reports whether this outcome carries provenance sufficient to
// be cited as evidence about a specific subject in a specific cluster.
//
// Deliberately separate from IsSuccess. A future evidence producer must require
// BOTH: a successful repair with unattributable provenance is not verification
// evidence, and complete provenance around a failed repair is not success.
func (o Outcome) LineageComplete() bool { return len(o.LineageDefects()) == 0 }

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
