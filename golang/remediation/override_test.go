package remediation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestOverrideRequiresReasonScopeAndExpiry — contract test. Each missing
// or incomplete field rejects the override. The validator's job is to
// turn a "force" flag into a structured intent.
func TestOverrideRequiresReasonScopeAndExpiry(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	base := Override{
		Actor:         "operator@cluster",
		Reason:        "scylladb partition manually repaired; resume remediation pipeline",
		PolicyID:      "remediation.failure_rate_policy",
		Scope:         "finding:f-abc123",
		IssuedAt:      now,
		Expiry:        now.Add(15 * time.Minute),
		CorrelationID: "corr-1",
	}
	if err := base.Validate(now); err != nil {
		t.Fatalf("base override must validate, got: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*Override)
		want   string
	}{
		{"empty actor", func(o *Override) { o.Actor = "" }, "actor is required"},
		{"vague reason", func(o *Override) { o.Reason = "force" }, "reason must be at least"},
		{"no policy id", func(o *Override) { o.PolicyID = "" }, "policy_id is required"},
		{"no scope", func(o *Override) { o.Scope = "" }, "scope is required"},
		{"no correlation", func(o *Override) { o.CorrelationID = "" }, "correlation_id is required"},
		{"no expiry", func(o *Override) { o.Expiry = time.Time{} }, "expiry is required"},
		{"past expiry", func(o *Override) { o.Expiry = now.Add(-time.Minute) }, "not in the future"},
		{"too-long lifetime", func(o *Override) { o.Expiry = now.Add(2 * time.Hour) }, "exceeds max"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := base
			tc.mutate(&o)
			err := o.Validate(now)
			if err == nil {
				t.Fatalf("expected validation error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error must mention %q, got: %v", tc.want, err)
			}
		})
	}
}

// TestOverrideStillRequiresVerification — contract test. An override
// bypasses a policy GATE, not the verification step. The presence of an
// override must not make Outcome.IsSuccess() return true without
// FindingResolved.
func TestOverrideStillRequiresVerification(t *testing.T) {
	now := time.Now()
	override := Override{
		Actor:         "operator@cluster",
		Reason:        "scylladb partition manually repaired; resume remediation pipeline",
		PolicyID:      "remediation.failure_rate_policy",
		Scope:         "finding:f-abc",
		IssuedAt:      now,
		Expiry:        now.Add(15 * time.Minute),
		CorrelationID: "corr-2",
	}
	if !override.RequiresVerification() {
		t.Fatal("override.RequiresVerification() must always be true")
	}

	// Outcome that dispatched under an override but hasn't verified yet
	// must NOT be reported as success — the override changes how we
	// passed the gate, not what success means.
	out := Outcome{FindingID: "f-abc", Dispatched: true}
	if out.IsSuccess() {
		t.Fatal("override-dispatched outcome without verification must not be SUCCESS")
	}
	if out.Status() != StatusPending {
		t.Fatalf("override-dispatched outcome: got status %s, want PENDING_VERIFICATION", out.Status())
	}

	// Verified but invariant still present is still not success.
	out = Outcome{FindingID: "f-abc", Dispatched: true, Verified: true, FindingResolved: false, VerifiedAt: now}
	if out.IsSuccess() {
		t.Fatal("override + verified but invariant present must not be SUCCESS")
	}

	// Only verified + resolved is success — same rule as without override.
	out = Outcome{FindingID: "f-abc", Dispatched: true, Verified: true, FindingResolved: true, VerifiedAt: now}
	if !out.IsSuccess() {
		t.Fatal("verified + resolved must be SUCCESS regardless of override path")
	}
}

// TestOverrideAuditNamesBypassedPolicy — contract test. The audit entry
// derived from an override MUST name the bypassed policy and the actor.
// An auditor reading the record alone must be able to answer "what was
// bypassed, by whom, why, and was it verified?"
func TestOverrideAuditNamesBypassedPolicy(t *testing.T) {
	now := time.Now()
	override := Override{
		Actor:         "operator@cluster",
		Reason:        "scylladb partition manually repaired; resume remediation pipeline",
		PolicyID:      "remediation.failure_rate_policy",
		Scope:         "finding:f-abc",
		IssuedAt:      now,
		Expiry:        now.Add(15 * time.Minute),
		CorrelationID: "corr-3",
	}
	out := Outcome{
		FindingID:       "f-abc",
		Dispatched:      true,
		Verified:        true,
		FindingResolved: true,
		VerifiedAt:      now.Add(5 * time.Minute),
	}
	entry := override.NewAuditEntry(out)

	if entry.BypassedPolicy != override.PolicyID {
		t.Fatalf("audit must name bypassed policy: got %q, want %q",
			entry.BypassedPolicy, override.PolicyID)
	}
	if entry.Actor != override.Actor {
		t.Fatalf("audit must name the actor: got %q, want %q", entry.Actor, override.Actor)
	}
	if entry.OutcomeStatus != string(StatusSucceeded) {
		t.Fatalf("audit must record outcome status: got %q", entry.OutcomeStatus)
	}
	if !entry.FindingResolved {
		t.Fatal("audit must record FindingResolved for a successful override")
	}

	// JSON shape: the bypassed_policy field must be present (auditors
	// grep JSON, not Go structs).
	b, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(b), `"bypassed_policy":"remediation.failure_rate_policy"`) {
		t.Fatalf("JSON must contain bypassed_policy field, got: %s", string(b))
	}
	if !strings.Contains(string(b), `"reason"`) {
		t.Fatalf("JSON must contain reason, got: %s", string(b))
	}
}
