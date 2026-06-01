// Package evidence defines the cluster's shared vocabulary for observed
// facts — where they came from, who wrote them, when they were observed,
// and how much an operator/agent should trust them. See
// docs/intent/evidence.provenance_trust_levels.yaml.
//
// The package is provider-neutral: it knows nothing about doctor findings,
// workflow runs, or awareness graphs. Consumers wrap their own observations
// in Provenance and call Classify to get a TrustLevel.
// @awareness namespace=globular.platform
// @awareness component=platform_evidence
// @awareness file_role=evidence_trust_level_definitions
// @awareness implements=globular.platform:intent.evidence.provenance_trust_levels
// @awareness risk=high
package evidence

import (
	"strings"
	"time"
)

// TrustLevel grades an observation. Order from most to least trustworthy:
// Authoritative > Degraded > Stale > Untrusted. Callers MUST treat Stale
// and Untrusted as non-actionable for privileged decisions.
type TrustLevel string

const (
	// Authoritative — recent observation from a high-rank source with a
	// known writer. Safe to act on.
	TrustAuthoritative TrustLevel = "AUTHORITATIVE"

	// Degraded — observation is from a trusted source but is older than
	// its freshness window. Operators may proceed with caution; agents
	// should pivot to a fresher source if available.
	TrustDegraded TrustLevel = "DEGRADED"

	// Stale — observation is older than twice its freshness window.
	// Privileged actions must refuse.
	TrustStale TrustLevel = "STALE"

	// Untrusted — observation is missing provenance metadata (source,
	// writer, or timestamp) or originates from an unknown source.
	// Privileged actions must refuse; operators should re-query the
	// authoritative source.
	TrustUntrusted TrustLevel = "UNTRUSTED"
)

// Source names the read path that produced an observation. The classifier
// uses Source to pick a freshness window and to compute base trust. New
// sources can be added at any time; unknown sources classify as Untrusted.
type Source string

const (
	SourceEtcdContract        Source = "etcd_contract"
	SourceWorkflowReceipt     Source = "workflow_receipt"
	SourceVerifierAttestation Source = "verifier_attestation"
	SourceControllerSnapshot  Source = "controller_snapshot"
	SourceServiceLog          Source = "service_log"
	SourceTelemetry           Source = "telemetry"
	SourceOperatorInput       Source = "operator_input"
	SourceInferred            Source = "inferred"
)

// Provenance records where an observation came from, who wrote it, when it
// was observed, and what call chain it belongs to. Every field except
// CorrelationID is required for the observation to be classifiable as
// anything other than Untrusted.
type Provenance struct {
	Source        Source
	WriterID      string // service identity, node id, or operator subject
	ObservedAt    time.Time
	CorrelationID string // optional — links related observations
}

// freshnessWindow returns how recent an observation from src must be to
// still count as Authoritative. Observations older than this drop to
// Degraded; older than 2× this drop to Stale.
func freshnessWindow(src Source) time.Duration {
	switch src {
	case SourceEtcdContract:
		return 5 * time.Minute
	case SourceWorkflowReceipt:
		return 10 * time.Minute
	case SourceVerifierAttestation:
		return 5 * time.Minute
	case SourceControllerSnapshot:
		return 2 * time.Minute
	case SourceServiceLog:
		return 1 * time.Minute
	case SourceTelemetry:
		return 90 * time.Second
	case SourceOperatorInput:
		return 5 * time.Minute
	case SourceInferred:
		return 30 * time.Second
	default:
		return 0
	}
}

// Classify returns the trust level of an observation as of now. Pure
// function — does not consult external state. Callers MUST pass a non-zero
// now (use time.Now()).
func Classify(p Provenance, now time.Time) TrustLevel {
	if p.ObservedAt.IsZero() || strings.TrimSpace(p.WriterID) == "" {
		return TrustUntrusted
	}
	window := freshnessWindow(p.Source)
	if window == 0 {
		// Unknown / unset source: provenance metadata is incomplete.
		return TrustUntrusted
	}
	age := now.Sub(p.ObservedAt)
	switch {
	case age < 0:
		// Clock skew: caller's now is older than the observation. Don't
		// silently treat this as fresh — treat as Degraded so the
		// operator notices.
		return TrustDegraded
	case age <= window:
		return TrustAuthoritative
	case age <= 2*window:
		return TrustDegraded
	default:
		return TrustStale
	}
}

// Worst returns the lowest-trust level across a list. Useful when a
// decision spans multiple observations: the bound is set by the weakest.
// An empty list returns Untrusted — silence is not freshness.
func Worst(levels ...TrustLevel) TrustLevel {
	if len(levels) == 0 {
		return TrustUntrusted
	}
	rank := map[TrustLevel]int{
		TrustAuthoritative: 0,
		TrustDegraded:      1,
		TrustStale:         2,
		TrustUntrusted:     3,
	}
	worst := TrustAuthoritative
	worstRank := -1
	for _, l := range levels {
		r, ok := rank[l]
		if !ok {
			// Unknown level: treat as Untrusted, the worst-case bound.
			return TrustUntrusted
		}
		if r > worstRank {
			worstRank = r
			worst = l
		}
	}
	return worst
}

// PreflightVerdict translates a TrustLevel into a short verdict string for
// agent preflight and operator reports. The verdict vocabulary matches
// the awareness preflight surface: "ok" means proceed, "uncertain" means
// proceed with care, "reject" means do not act.
func PreflightVerdict(l TrustLevel) string {
	switch l {
	case TrustAuthoritative:
		return "ok"
	case TrustDegraded:
		return "uncertain"
	case TrustStale, TrustUntrusted:
		return "reject"
	default:
		return "reject"
	}
}

// AuthorizesRemediation reports whether a privileged remediation action
// may proceed at this trust level. Stale and Untrusted block; Degraded
// is allowed but the caller should record the downgrade in audit so the
// decision is explainable later.
func AuthorizesRemediation(l TrustLevel) bool {
	switch l {
	case TrustAuthoritative, TrustDegraded:
		return true
	default:
		return false
	}
}
