package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// autoAdvanceDecision is the pure decision the reconcile-driven auto-advance uses
// (Slice 2). It advances ONLY when converged (laggardCount == 0), never forces,
// never regresses.

// Converged + a newer/first desired target → advance.
func TestAutoAdvance_ConvergedAdvances(t *testing.T) {
	// first activation (no current anchor)
	if a, _ := autoAdvanceDecision(nil, "v1.2.251", "1.2.251", 0); a != activationActivate {
		t.Fatalf("converged first-activation must advance, got %q", a)
	}
	// forward from an older active
	cur := anchor("v1.2.250", "1.2.250")
	if a, _ := autoAdvanceDecision(cur, "v1.2.251", "1.2.251", 0); a != activationActivate {
		t.Fatalf("converged forward must advance, got %q", a)
	}
}

// Not converged (laggards present) → never advances, regardless of desired.
func TestAutoAdvance_NotConverged_NoOp(t *testing.T) {
	cur := anchor("v1.2.250", "1.2.250")
	if a, _ := autoAdvanceDecision(cur, "v1.2.251", "1.2.251", 1); a != activationNoop {
		t.Fatalf("not-converged must be a no-op (no force), got %q", a)
	}
	// even many laggards on a first activation → no-op (never forces)
	if a, _ := autoAdvanceDecision(nil, "v1.2.251", "1.2.251", 5); a != activationNoop {
		t.Fatalf("not-converged first-activation must be a no-op, got %q", a)
	}
}

// Already active → idempotent no-op even when converged.
func TestAutoAdvance_AlreadyActive_NoOp(t *testing.T) {
	cur := anchor("v1.2.251", "1.2.251")
	if a, _ := autoAdvanceDecision(cur, "v1.2.251", "1.2.251", 0); a != activationNoop {
		t.Fatalf("already-active must be a no-op, got %q", a)
	}
}

// No-regression: the auto-advance never moves the pointer backward (it has no
// allow-regression escape hatch — converged or not, an older desired is refused).
func TestAutoAdvance_NoRegression_Refused(t *testing.T) {
	cur := anchor("v1.2.251", "1.2.251")
	if a, _ := autoAdvanceDecision(cur, "v1.2.250", "1.2.250", 0); a != activationRefuse {
		t.Fatalf("older desired must be refused (no auto-regression), got %q", a)
	}
}

// Native (non-SemVer) desired platform that cannot be ordered against the current
// active is refused (no silent guess), mirroring decideActivation.
func TestAutoAdvance_NativeUnorderable_Refused(t *testing.T) {
	cur := anchor("v1.2.251", "1.2.251")
	if a, _ := autoAdvanceDecision(cur, "nightly-2026", "nightly-2026", 0); a != activationRefuse {
		t.Fatalf("non-orderable native desired must be refused, got %q", a)
	}
}

// desired_release anchor JSON round-trips (the new Slice 2 etcd document).
func TestDesiredReleaseAnchor_JSONRoundTrip(t *testing.T) {
	in := desiredReleaseAnchor{
		ReleaseTag:      "v1.2.251",
		PlatformRelease: "1.2.251",
		SetAtUnix:       1782600000,
		SetBy:           "platform-upgrade",
	}
	b, err := json.Marshal(&in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out desiredReleaseAnchor
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Fatalf("round-trip mismatch: got %+v want %+v", out, in)
	}
	// field names are the etcd contract — assert them explicitly
	for _, want := range []string{`"release_tag"`, `"platform_release"`, `"set_at_unix"`, `"set_by"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("desired_release JSON missing %s: %s", want, b)
		}
	}
}
