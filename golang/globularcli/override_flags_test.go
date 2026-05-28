package main

import (
	"strings"
	"testing"
	"time"
)

// TestBuildOverride_RejectsIncompleteFlags — wiring contract test for
// operator.override_intent. A bare --force without the structured
// override flags must be refused at the CLI before any gate-bypassing
// RPC is sent. The error must name the missing fields so the operator
// knows exactly what to supply.
func TestBuildOverride_RejectsIncompleteFlags(t *testing.T) {
	cases := []struct {
		name string
		f    OverrideFlags
		want string
	}{
		{
			name: "empty",
			f:    OverrideFlags{Lifetime: 5 * time.Minute},
			want: "actor is required",
		},
		{
			name: "missing reason",
			f:    OverrideFlags{Actor: "alice@cluster", PolicyID: "p", Scope: "s", Lifetime: 5 * time.Minute},
			want: "reason must be at least",
		},
		{
			name: "missing policy",
			f:    OverrideFlags{Actor: "alice", Reason: "this is a long reason for override", Scope: "s", Lifetime: 5 * time.Minute},
			want: "policy_id is required",
		},
		{
			name: "missing scope",
			f:    OverrideFlags{Actor: "alice", Reason: "this is a long reason for override", PolicyID: "p", Lifetime: 5 * time.Minute},
			want: "scope is required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := BuildOverride(tc.f)
			if err == nil {
				t.Fatalf("expected error mentioning %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error must mention %q, got: %v", tc.want, err)
			}
			// Error must also hint at which CLI flags to set.
			if !strings.Contains(err.Error(), "--override-") {
				t.Fatalf("error must hint at --override-* flags, got: %v", err)
			}
		})
	}
}

// TestBuildOverride_AcceptsCompleteFlags — happy path. Complete flag set
// builds a validated Override with a derived correlation id.
func TestBuildOverride_AcceptsCompleteFlags(t *testing.T) {
	f := OverrideFlags{
		Actor:    "alice@cluster",
		Reason:   "scylladb manually repaired; resume node recovery pipeline",
		PolicyID: "node.recovery.cluster_safety_checks",
		Scope:    "node:globule-dell",
		Lifetime: 15 * time.Minute,
	}
	o, err := BuildOverride(f)
	if err != nil {
		t.Fatalf("complete flag set must build: %v", err)
	}
	if o.Actor != f.Actor || o.Reason != f.Reason {
		t.Fatalf("flag fields not copied: %+v", o)
	}
	if o.CorrelationID == "" {
		t.Fatal("override must carry a derived correlation id")
	}
	if !strings.HasPrefix(o.CorrelationID, "override-") {
		t.Fatalf("correlation id format: got %q", o.CorrelationID)
	}
	if o.Expiry.Sub(o.IssuedAt) != f.Lifetime {
		t.Fatalf("expiry math: got %s, want %s", o.Expiry.Sub(o.IssuedAt), f.Lifetime)
	}
}

// TestBuildOverride_RejectsLifetimeOverMax — Override.Validate enforces
// a 1-hour ceiling. Confirm the CLI helper surfaces that rejection
// rather than silently allowing a long-lived bypass.
func TestBuildOverride_RejectsLifetimeOverMax(t *testing.T) {
	f := OverrideFlags{
		Actor:    "alice@cluster",
		Reason:   "scylladb manually repaired; resume node recovery pipeline",
		PolicyID: "node.recovery.cluster_safety_checks",
		Scope:    "node:globule-dell",
		Lifetime: 2 * time.Hour, // ← above 1h ceiling
	}
	if _, err := BuildOverride(f); err == nil {
		t.Fatal("lifetime > 1h must be rejected")
	} else if !strings.Contains(err.Error(), "exceeds max") {
		t.Fatalf("error must mention max lifetime, got: %v", err)
	}
}
