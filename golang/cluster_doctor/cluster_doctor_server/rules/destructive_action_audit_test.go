package rules

// globular:tested_by destructive_action_guards

import (
	"encoding/json"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// specJSON serialises an ingressSpecDisableGuard to a JSON string suitable for
// loading into Snapshot.IngressSpecRaw — mirrors how the collector stores the
// raw etcd value.
func specJSON(t *testing.T, spec ingressSpecDisableGuard) string {
	t.Helper()
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("specJSON marshal: %v", err)
	}
	return string(b)
}

// TestMalformedDisableDoesNotStopRuntime verifies that when the ingress desired
// spec carries mode=disabled but the explicit-disable guard is incomplete, the
// doctor emits a CRITICAL finding. The rule must not perform any runtime
// stop or remediation — the finding is observe-only.
//
// Invariant: destructive_actions.require_explicit_guard
func TestMalformedDisableDoesNotStopRuntime(t *testing.T) {
	inv := ingressUnguardedDisableIntent{}

	cases := []struct {
		name string
		spec ingressSpecDisableGuard
	}{
		{
			name: "no_explicit_disabled_flag",
			spec: ingressSpecDisableGuard{
				Mode:             "disabled",
				ExplicitDisabled: false,
				Reason:           "operator requested disable",
				Generation:       5,
			},
		},
		{
			name: "empty_reason",
			spec: ingressSpecDisableGuard{
				Mode:             "disabled",
				ExplicitDisabled: true,
				Reason:           "",
				Generation:       5,
			},
		},
		{
			name: "zero_generation",
			spec: ingressSpecDisableGuard{
				Mode:             "disabled",
				ExplicitDisabled: true,
				Reason:           "operator requested disable",
				Generation:       0,
			},
		},
		{
			name: "all_guard_fields_missing",
			spec: ingressSpecDisableGuard{
				Mode: "disabled",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := &collector.Snapshot{
				IngressSpecPresent: true,
				IngressSpecRaw:     specJSON(t, tc.spec),
			}

			findings := inv.Evaluate(snap, Config{})
			if len(findings) != 1 {
				t.Fatalf("expected 1 CRITICAL finding for %q, got %d", tc.name, len(findings))
			}
			f := findings[0]
			if f.Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
				t.Errorf("expected SEVERITY_CRITICAL, got %v", f.Severity)
			}
			if f.InvariantID != "ingress.unguarded_disable_intent" {
				t.Errorf("unexpected invariant ID: %q", f.InvariantID)
			}
			if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
				t.Errorf("expected INVARIANT_FAIL, got %v", f.InvariantStatus)
			}
			// Rule must not contain any remediation that stops runtime —
			// all remediation steps should be operator-guidance only (no
			// SYSTEMCTL_STOP or destructive actions).
			for _, step := range f.Remediation {
				if step.Action != nil &&
					step.Action.ActionType == cluster_doctorpb.ActionType_SYSTEMCTL_RESTART {
					t.Errorf("rule must not include auto-restart remediation: %v", step)
				}
			}
		})
	}
}

// TestExplicitValidDisableStopsRuntimeAndAuditable verifies that when the
// ingress desired spec carries mode=disabled WITH a complete explicit-disable
// guard, the doctor emits no CRITICAL violation. The transition is intentional
// and auditable — the rule should be silent.
//
// Invariant: destructive_actions.require_explicit_guard
func TestExplicitValidDisableStopsRuntimeAndAuditable(t *testing.T) {
	inv := ingressUnguardedDisableIntent{}

	validSpec := ingressSpecDisableGuard{
		Mode:             "disabled",
		ExplicitDisabled: true,
		Reason:           "scheduled maintenance window, operator: alice",
		Generation:       42,
		WriterLeaderID:   "leader-node-7f3a",
		Source:           "cluster-controller",
		Authoritative:    true,
	}

	snap := &collector.Snapshot{
		IngressSpecPresent: true,
		IngressSpecRaw:     specJSON(t, validSpec),
	}

	findings := inv.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected no findings for valid explicit disable, got %d: %+v", len(findings), findings)
	}
}

// TestActiveSpecProducesNoFinding verifies that a healthy active spec
// (mode=vip_failover) never triggers the destructive_action_audit invariant,
// even if explicit_disabled happens to be false (as it always is for active specs).
func TestActiveSpecProducesNoFinding(t *testing.T) {
	inv := ingressUnguardedDisableIntent{}

	activeSpec := ingressSpecDisableGuard{
		Mode:             "vip_failover",
		ExplicitDisabled: false,
		Reason:           "",
		Generation:       10,
	}

	snap := &collector.Snapshot{
		IngressSpecPresent: true,
		IngressSpecRaw:     specJSON(t, activeSpec),
	}

	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Fatalf("expected no findings for active spec, got %d", len(got))
	}
}

// TestMissingSpecProducesNoFinding verifies the rule is silent when
// IngressSpecPresent=false (etcd key absent). That condition is surfaced by
// ingressSpecMissing, not this rule.
func TestMissingSpecProducesNoFinding(t *testing.T) {
	inv := ingressUnguardedDisableIntent{}

	snap := &collector.Snapshot{
		IngressSpecPresent: false,
		IngressSpecRaw:     "",
	}

	if got := inv.Evaluate(snap, Config{}); len(got) != 0 {
		t.Fatalf("expected no findings for absent spec, got %d", len(got))
	}
}
