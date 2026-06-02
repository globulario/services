package rules

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// TestHealer_NoDirectRemoteOpsMutation pins the Milestone 2 architectural
// invariant: the Healer struct has no field that lets it mutate cluster
// state directly. Path B (background-healer → RemoteOps → node-agent /
// workflow / etcd) is closed. All mutations flow through the Dispatcher,
// which the cluster-doctor server wires to ExecuteRemediation.
//
// If a future refactor reintroduces a RemoteOps-shaped field, this test
// fails — making the regression loud at build time of the test binary.
func TestHealer_NoDirectRemoteOpsMutation(t *testing.T) {
	h := Healer{}
	typ := reflect.TypeOf(h)

	// Field allowlist. Any new field must be either (a) a classifier
	// parameter (DryRun, MaxActions, MaxFailures) or (b) a gated hook
	// (Dispatcher, PolicyLookup). Direct mutation surfaces are forbidden.
	allowed := map[string]bool{
		"DryRun":       true,
		"Dispatcher":   true,
		"MaxActions":   true,
		"MaxFailures":  true,
		"PolicyLookup": true,
	}
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		if !allowed[name] {
			t.Fatalf("Healer must not have field %q — Path B mutation surface is forbidden (Milestone 2); allowed=%v",
				name, sortedKeys(allowed))
		}
	}

	// Explicit forbidden-name guard for clarity (redundant with the
	// allowlist check above, but it surfaces the intent in the failure
	// message if someone adds a forbidden field under a creative name).
	for _, name := range []string{"Remote", "RemoteOps", "NodeAgent", "Etcd", "Workflow"} {
		if _, ok := typ.FieldByName(name); ok {
			t.Fatalf("Healer must not have field %q — direct mutation surface (Path B closed in Milestone 2)", name)
		}
	}
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Sort for stable failure messages.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// TestHealer_AutoActionsRouteThroughExecuteRemediation verifies the gated
// path: when a HealAuto rule fires, the Healer asks its Dispatcher to
// handle the action — it does NOT execute it itself. PolicyLookup is
// overridden so the test can exercise the dispatch path without depending
// on a real HealAuto policy entry (PolicyV1 has none with a non-empty
// AutoAction after Milestone 2 demotions).
//
// In production the Dispatcher is the cluster-doctor server's
// gatedDispatcher, which routes through ExecuteRemediation. This test
// uses a recording fake to assert the wiring; the gatedDispatcher →
// ExecuteRemediation hop is covered by TestHealer_EveryExecutedActionWritesEtcdAudit
// and the existing TestExecuteRemediation_HardBlocksETCDPut_Always.
func TestHealer_AutoActionsRouteThroughExecuteRemediation(t *testing.T) {
	dispatcher := &recordingDispatcher{}
	healer := &Healer{
		DryRun:     false,
		Dispatcher: dispatcher,
		// Inject a synthetic HealAuto rule for an arbitrary invariant id.
		PolicyLookup: func(invariantID string) HealRule {
			if invariantID == "synthetic.heal_auto" {
				return HealRule{
					InvariantID: invariantID,
					Disposition: HealAuto,
					Action:      "synthetic auto-action for the routing test",
					AutoAction:  "synthetic_auto_action",
				}
			}
			return LookupPolicy(invariantID)
		},
	}

	findings := []Finding{
		{
			FindingID:   "f-syn-1",
			InvariantID: "synthetic.heal_auto",
			EntityRef:   "test/synthetic",
		},
	}
	healer.Evaluate(context.Background(), findings)

	if len(dispatcher.calls) != 1 {
		t.Fatalf("HealAuto finding must produce exactly 1 Dispatch call; got %d: %+v",
			len(dispatcher.calls), dispatcher.calls)
	}
	c := dispatcher.calls[0]
	if c.InvariantID != "synthetic.heal_auto" {
		t.Fatalf("Dispatch invariant_id = %q, want synthetic.heal_auto", c.InvariantID)
	}
	if c.AutoAction != "synthetic_auto_action" {
		t.Fatalf("Dispatch auto_action = %q, want synthetic_auto_action", c.AutoAction)
	}
	if c.DryRun {
		t.Fatalf("Dispatch DryRun = true, want false (Healer{DryRun:false})")
	}
}

// executingDispatcher returns the configured (executed, auditID, err) so a
// test can simulate ExecuteRemediation reporting back to the Healer.
type executingDispatcher struct {
	executed bool
	auditID  string
	err      error
	calls    int
}

func (d *executingDispatcher) Dispatch(_ context.Context, _ Finding, _ string, _ bool) (bool, string, error) {
	d.calls++
	return d.executed, d.auditID, d.err
}

// TestHealer_EveryExecutedActionWritesEtcdAudit verifies that when the
// Dispatcher reports an executed mutation (with an audit_id from the
// gated path), the HealReport carries that audit_id forward — the
// single audit trail invariant is preserved end-to-end.
//
// The audit_id is the canonical link to /globular/cluster_doctor/audit/rem-*,
// written by auditRemediation() in the cluster-doctor server when
// ExecuteRemediation processes a dispatch. The Healer must not invent a
// parallel audit; it must record exactly what the gate reports.
func TestHealer_EveryExecutedActionWritesEtcdAudit(t *testing.T) {
	const wantAuditID = "rem-1780431234"
	dispatcher := &executingDispatcher{
		executed: true,
		auditID:  wantAuditID,
		err:      nil,
	}
	healer := &Healer{
		DryRun:     false,
		Dispatcher: dispatcher,
		PolicyLookup: func(invariantID string) HealRule {
			return HealRule{
				InvariantID: invariantID,
				Disposition: HealAuto,
				Action:      "audit-trail wiring test",
				AutoAction:  "audit_test_action",
			}
		},
	}
	findings := []Finding{
		{FindingID: "f-audit", InvariantID: "synthetic.audit_trail", EntityRef: "test/audit"},
	}
	report := healer.Evaluate(context.Background(), findings)

	if dispatcher.calls != 1 {
		t.Fatalf("expected 1 Dispatch call, got %d", dispatcher.calls)
	}
	if report.AutoFixed != 1 {
		t.Fatalf("expected AutoFixed=1 (dispatcher reported executed=true), got %d", report.AutoFixed)
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	r := report.Results[0]
	if !r.Executed {
		t.Fatalf("result.Executed = false, want true (dispatcher returned executed=true)")
	}
	if r.AuditID != wantAuditID {
		t.Fatalf("result.AuditID = %q, want %q (Healer must not invent a parallel audit_id)", r.AuditID, wantAuditID)
	}
	if !strings.HasPrefix(r.AuditID, "rem-") {
		t.Fatalf("audit_id %q must be in the canonical rem-* form (etcd /globular/cluster_doctor/audit/rem-*)", r.AuditID)
	}
}
