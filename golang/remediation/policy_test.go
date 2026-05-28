package remediation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestFailureRatePolicyBlocksAfterThresholdByActionClass — contract test.
// Different action classes have different thresholds. PACKAGE_REINSTALL is
// far more expensive than SYSTEMCTL_RESTART, so its breaker trips much
// sooner. A single global threshold would either over-retry destructive
// actions or under-retry cheap ones.
func TestFailureRatePolicyBlocksAfterThresholdByActionClass(t *testing.T) {
	p := DefaultFailureRatePolicy()

	restart := p.For("SYSTEMCTL_RESTART")
	reinstall := p.For("PACKAGE_REINSTALL")
	if restart.Threshold <= reinstall.Threshold {
		t.Fatalf("SYSTEMCTL_RESTART threshold (%d) must be higher than PACKAGE_REINSTALL (%d)",
			restart.Threshold, reinstall.Threshold)
	}

	// Below the per-class threshold: still allowed.
	if ok, _ := p.Allow("SYSTEMCTL_RESTART", restart.Threshold-1); !ok {
		t.Fatalf("SYSTEMCTL_RESTART must allow attempts below threshold")
	}
	// At the threshold: breaker opens.
	if ok, reason := p.Allow("SYSTEMCTL_RESTART", restart.Threshold); ok {
		t.Fatalf("SYSTEMCTL_RESTART must trip at threshold")
	} else if !strings.Contains(reason, "SYSTEMCTL_RESTART") {
		t.Fatalf("breaker reason must name the action class, got: %q", reason)
	}

	// PACKAGE_REINSTALL trips much sooner. Same failure count that
	// SYSTEMCTL_RESTART tolerates must be rejected for the destructive
	// class. The shared policy gives different answers — that's the
	// whole point of action-class awareness.
	mid := restart.Threshold - 1 // below restart's threshold
	if mid < reinstall.Threshold {
		t.Skipf("default thresholds adjusted: mid=%d < reinstall.Threshold=%d", mid, reinstall.Threshold)
	}
	if ok, _ := p.Allow("PACKAGE_REINSTALL", mid); ok {
		t.Fatalf("PACKAGE_REINSTALL must reject what SYSTEMCTL_RESTART tolerates")
	}

	// Unknown class falls back to Default — the policy is total.
	defThreshold := p.Default.Threshold
	if ok, _ := p.Allow("MADE_UP_ACTION", defThreshold); ok {
		t.Fatalf("unknown class must trip at default threshold")
	}
}

type stubGetter struct {
	value []byte
	err   error
}

func (s stubGetter) Get(_ context.Context, _ string) ([]byte, error) {
	return s.value, s.err
}

// TestFailureRatePolicyPersistsAcrossRestart — contract test. A future
// doctor process must inherit the same policy from etcd. Simulate restart
// by serializing the operator-published override and re-reading it through
// the LoadFromEtcd path. Defaults must still merge in for any class the
// override didn't name — silence is not "unlimited retries."
func TestFailureRatePolicyPersistsAcrossRestart(t *testing.T) {
	// Operator publishes a tighter PACKAGE_REPAIR budget and a custom default.
	original := &FailureRatePolicy{
		Default: ClassPolicy{Threshold: 7, Window: 45 * time.Minute},
		ClassPolicies: map[ActionClass]ClassPolicy{
			"PACKAGE_REPAIR": {Threshold: 1, Window: 6 * time.Hour},
		},
	}
	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// "Restart" — new process reads the persisted JSON via LoadFromEtcd.
	reloaded := LoadFromEtcd(context.Background(), stubGetter{value: raw})
	if got := reloaded.For("PACKAGE_REPAIR").Threshold; got != 1 {
		t.Fatalf("PACKAGE_REPAIR threshold after restart: got %d, want 1", got)
	}
	if got := reloaded.Default.Threshold; got != 7 {
		t.Fatalf("Default threshold after restart: got %d, want 7", got)
	}
	// Defaults must still apply to classes the override didn't name.
	// SYSTEMCTL_RESTART is in the built-in defaults — those must survive
	// the merge so an operator can't accidentally "delete" enforcement by
	// publishing a partial JSON.
	if got := reloaded.For("SYSTEMCTL_RESTART").Threshold; got <= 0 {
		t.Fatalf("SYSTEMCTL_RESTART threshold after partial override: got %d, want built-in default", got)
	}

	// etcd unreachable → defaults applied (never "no policy").
	fallback := LoadFromEtcd(context.Background(), stubGetter{err: errors.New("etcd unreachable")})
	if fallback == nil || fallback.Default.Threshold == 0 {
		t.Fatalf("etcd error must fall back to defaults, got %+v", fallback)
	}

	// Missing key → defaults.
	missing := LoadFromEtcd(context.Background(), stubGetter{})
	if missing == nil || missing.Default.Threshold == 0 {
		t.Fatalf("missing key must fall back to defaults, got %+v", missing)
	}
}

// TestRemediationCircuitBreakerUsesSharedPolicy — contract test. The
// failure-rate decision must be identical whether the question comes from
// the doctor handler (server-side) or from the workflow assess step
// (pipeline-side). They must consult the same policy object so an operator
// override propagates everywhere without per-surface re-implementation.
func TestRemediationCircuitBreakerUsesSharedPolicy(t *testing.T) {
	p := DefaultFailureRatePolicy()
	// "Server-side" caller (doctor):
	allowedDoctor, doctorReason := p.Allow("SYSTEMCTL_RESTART", 5)
	// "Pipeline-side" caller (workflow assess step), constructed via
	// NormalizeActionClass so the workflow can pass raw proto strings.
	allowedWorkflow, workflowReason := p.Allow(NormalizeActionClass("systemctl_restart"), 5)
	if allowedDoctor != allowedWorkflow {
		t.Fatalf("doctor (%v) and workflow (%v) must agree on the breaker", allowedDoctor, allowedWorkflow)
	}
	if doctorReason != workflowReason {
		t.Fatalf("doctor reason (%q) and workflow reason (%q) must match", doctorReason, workflowReason)
	}

	// Now flip the operator override and confirm both surfaces see it.
	override := &FailureRatePolicy{
		Default: ClassPolicy{Threshold: 1, Window: 5 * time.Minute},
		ClassPolicies: map[ActionClass]ClassPolicy{
			"SYSTEMCTL_RESTART": {Threshold: 1, Window: 5 * time.Minute},
		},
	}
	raw, _ := json.Marshal(override)
	shared := LoadFromEtcd(context.Background(), stubGetter{value: raw})
	allowedDoctor, _ = shared.Allow("SYSTEMCTL_RESTART", 1)
	allowedWorkflow, _ = shared.Allow(NormalizeActionClass("systemctl_restart"), 1)
	if allowedDoctor || allowedWorkflow {
		t.Fatal("override of SYSTEMCTL_RESTART to threshold=1 must trip both surfaces at 1 failure")
	}
}
