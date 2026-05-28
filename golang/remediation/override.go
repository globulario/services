package remediation

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Override is the structured intent an operator presents when they need
// to force a remediation despite a policy gate. It is NOT a magic flag —
// every field is required, and the audit must record which policy was
// bypassed so the override is explainable later. See
// docs/intent/operator.override_intent.yaml.
type Override struct {
	// Actor identifies the operator (principal_id) who issued the override.
	Actor string

	// Reason is the human-readable justification. Vague reasons like "fix"
	// or "force" are rejected by Validate — operators must say why.
	Reason string

	// PolicyID names the gate the override is bypassing
	// (e.g. "remediation.failure_rate_policy", "evidence.must_carry_provenance_and_trust_level").
	// Required so the audit names what was bypassed.
	PolicyID string

	// Scope narrows the override's reach: typically a finding id, an
	// invariant id, a node id, or "cluster". Required so the override
	// cannot be construed as "all remediation everywhere".
	Scope string

	// IssuedAt is when the override was created.
	IssuedAt time.Time

	// Expiry is when the override stops being honored. Bounded — see
	// Validate; an override without an expiry is rejected.
	Expiry time.Time

	// CorrelationID joins the override to the doctor finding, the
	// workflow run, the audit record, and the verification result.
	CorrelationID string
}

// Validate enforces the override contract. Returns nil only when every
// field is non-empty, the reason is descriptive (≥10 chars), the expiry
// is in the future, and the override has a non-zero CorrelationID.
func (o Override) Validate(now time.Time) error {
	if strings.TrimSpace(o.Actor) == "" {
		return errors.New("override: actor is required")
	}
	if len(strings.TrimSpace(o.Reason)) < 10 {
		return errors.New("override: reason must be at least 10 characters — say why the gate is being bypassed")
	}
	if strings.TrimSpace(o.PolicyID) == "" {
		return errors.New("override: policy_id is required — name the gate being bypassed")
	}
	if strings.TrimSpace(o.Scope) == "" {
		return errors.New("override: scope is required — narrow the override to a finding/invariant/node")
	}
	if strings.TrimSpace(o.CorrelationID) == "" {
		return errors.New("override: correlation_id is required — overrides must join doctor/workflow/audit")
	}
	if o.IssuedAt.IsZero() {
		return errors.New("override: issued_at is required")
	}
	if o.Expiry.IsZero() {
		return errors.New("override: expiry is required — overrides must have a time bound")
	}
	if !o.Expiry.After(now) {
		return fmt.Errorf("override: expiry %s is not in the future (now=%s)",
			o.Expiry.Format(time.RFC3339), now.Format(time.RFC3339))
	}
	if maxLifetime := 1 * time.Hour; o.Expiry.Sub(o.IssuedAt) > maxLifetime {
		return fmt.Errorf("override: lifetime %s exceeds max %s",
			o.Expiry.Sub(o.IssuedAt), maxLifetime)
	}
	return nil
}

// RequiresVerification is unconditionally true. An override bypasses a
// policy gate; it does NOT waive the verification step. The presence of
// an override changes only the audit shape, not the success criteria —
// see Outcome.IsSuccess() which still demands FindingResolved.
func (o Override) RequiresVerification() bool {
	return true
}

// OverrideAuditEntry is the audit shape produced when an override is used.
// It MUST name the bypassed policy and the actor so compliance reviews
// can answer "who bypassed what, when, why, and was it verified?"
type OverrideAuditEntry struct {
	CorrelationID   string    `json:"correlation_id"`
	Actor           string    `json:"actor"`
	BypassedPolicy  string    `json:"bypassed_policy"`
	Reason          string    `json:"reason"`
	Scope           string    `json:"scope"`
	IssuedAt        time.Time `json:"issued_at"`
	Expiry          time.Time `json:"expiry"`
	OutcomeStatus   string    `json:"outcome_status"`             // STATUS_SUCCEEDED / DEGRADED / etc.
	FindingResolved bool      `json:"finding_resolved"`           // true only when verify confirms repair
	VerifiedAt      time.Time `json:"verified_at,omitempty"`
}

// NewAuditEntry derives an audit entry from an override and the eventual
// remediation outcome. The fields are deliberately verbose: an auditor
// must be able to answer the override questions from this record alone,
// without joining back to the workflow run.
func (o Override) NewAuditEntry(out Outcome) OverrideAuditEntry {
	return OverrideAuditEntry{
		CorrelationID:   o.CorrelationID,
		Actor:           o.Actor,
		BypassedPolicy:  o.PolicyID,
		Reason:          o.Reason,
		Scope:           o.Scope,
		IssuedAt:        o.IssuedAt,
		Expiry:          o.Expiry,
		OutcomeStatus:   string(out.Status()),
		FindingResolved: out.FindingResolved,
		VerifiedAt:      out.VerifiedAt,
	}
}
