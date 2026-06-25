package govops

import (
	"sort"
	"strings"
	"testing"

	cluster_operator "github.com/globulario/services/golang/ai_memory/domains/cluster_operator"
	pb "github.com/globulario/services/golang/govops/governed_operationpb"
)

func applyReq(action string) *pb.OperationRequest {
	return &pb.OperationRequest{
		Id:        "op-1",
		Action:    action,
		Execution: &pb.OperationExecution{Mode: pb.ExecutionMode_APPLY},
		Target:    &pb.OperationTarget{Owner: "cluster_controller", ResourceId: "xds"},
	}
}

// TestEvaluateApply_BehavioralForbiddenMove_RefusesRawOwnerWrite is the core
// enforcement: a raw owner-owned-state write named by any authored alias is REFUSED
// under APPLY, with the forbidden-move rule id carried on the violation.
func TestEvaluateApply_BehavioralForbiddenMove_RefusesRawOwnerWrite(t *testing.T) {
	for _, action := range []string{
		"etcdctl_put", "raw_etcd_write", "mcp_raw_write",
		"write_desired_state_directly", "services_desired_set_force_cross_kind",
		"set_infra_version_raw", "nodeagent_installed_set_raw",
	} {
		d := Validate(applyReq(action))
		if d.Kind != DecisionRefused {
			t.Errorf("action %q: kind=%q, want refused", action, d.Kind)
			continue
		}
		var found bool
		for _, v := range d.Violations {
			if v.Code == "behavioral_forbidden_move" && v.Rule == rawOwnerWriteForbiddenID {
				found = true
			}
		}
		if !found {
			t.Errorf("action %q: no behavioral_forbidden_move violation carrying %q; got %+v",
				action, rawOwnerWriteForbiddenID, d.Violations)
		}
	}
}

// TestEvaluateApply_BehavioralMove_DoesNotOverBlock: the sentinel must not refuse an
// unrelated, owner-routed action (mirrors the behavioral kernel's no-over-block
// ratchet).
func TestEvaluateApply_BehavioralMove_DoesNotOverBlock(t *testing.T) {
	if rule := matchRawOwnerWrite("upsert_desired_via_owner_rpc"); rule != "" {
		t.Errorf("owner-routed action wrongly matched forbidden move %q", rule)
	}
}

// TestEvaluateApply_BehavioralMove_FiresEvenWhenEffectUnderdeclared proves the match
// is name-based, not effect-based: a request that declares no owner/effect (which
// would otherwise skip the structural owner-path gate) is still refused.
func TestEvaluateApply_BehavioralMove_FiresEvenWhenEffectUnderdeclared(t *testing.T) {
	r := &pb.OperationRequest{
		Id:        "op-2",
		Action:    "etcdctl_put",
		Execution: &pb.OperationExecution{Mode: pb.ExecutionMode_APPLY},
		// no Target.Owner, no ExpectedEffect.
	}
	if d := Validate(r); d.Kind != DecisionRefused {
		t.Errorf("under-declared raw write: kind=%q, want refused", d.Kind)
	}
}

// TestExecute_OwnerPathNotInvokedOnBehavioralRefusal proves the owner path is NEVER
// invoked on a behavioral refusal — enforcement is a hard stop, not advice.
func TestExecute_OwnerPathNotInvokedOnBehavioralRefusal(t *testing.T) {
	d := Validate(applyReq("services_desired_set_force_cross_kind"))
	called := false
	res, err := Execute(d, func() error { called = true; return nil })
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if called {
		t.Error("owner path was invoked despite a behavioral refusal")
	}
	if res != pb.OperationResult_REFUSED {
		t.Errorf("result=%v, want REFUSED", res)
	}
}

// TestLedgerEntryFor_RecordsBehavioralRuleOnRefusal proves the refusal is recorded
// against the rule it enforced (OperationLedgerEntry.behavioral_rules).
func TestLedgerEntryFor_RecordsBehavioralRuleOnRefusal(t *testing.T) {
	r := applyReq("etcdctl_put")
	e := LedgerEntryFor(r, Validate(r))
	if e.GetResult() != pb.OperationResult_REFUSED {
		t.Errorf("result=%v, want REFUSED", e.GetResult())
	}
	if !containsStr(e.GetBehavioralRules(), rawOwnerWriteForbiddenID) {
		t.Errorf("ledger behavioral_rules=%v, want to contain %q", e.GetBehavioralRules(), rawOwnerWriteForbiddenID)
	}
}

// TestBreakGlass_NotBlockedByBehavioralMove: a well-formed break-glass naming a
// raw-write action is the SANCTIONED path (human-confirmed, post-reconciled); the
// behavioral forbidden-move check must not override it.
func TestBreakGlass_NotBlockedByBehavioralMove(t *testing.T) {
	r := &pb.OperationRequest{
		Id:        "op-bg",
		Action:    "raw_etcd_write",
		Execution: &pb.OperationExecution{Mode: pb.ExecutionMode_BREAK_GLASS},
		Evidence:  &pb.OperationEvidence{BeforeSnapshot: "snap-1"},
		Postconditions: &pb.OperationPostconditions{
			RequiredChecks: []string{"post_reconcile_ok"},
			RollbackPlan:   "restore snap-1",
			LedgerRequired: true,
		},
	}
	if d := Validate(r); d.Kind != DecisionBreakGlassOnly {
		t.Errorf("well-formed break-glass raw write: kind=%q, want break_glass_only", d.Kind)
	}
}

// TestGovopsBehavioralRefusalUsesCompiledSeedNotLiveRPC is the boundary guard. It
// proves govops's alias set IS the COMPILED cluster_operator seed pack's aliases —
// the gate reads the real authored artifact, in sync, with no live ai-memory RPC and
// no hand-copied shadow. If a future change swaps the compiled match for a live
// CheckAction call or lets a divergent copy creep in, this fails. The test name is
// the carved boundary: the forbidden spell is in the gate, not a phone call to the
// oracle.
func TestGovopsBehavioralRefusalUsesCompiledSeedNotLiveRPC(t *testing.T) {
	var seedAliases []string
	for _, fm := range cluster_operator.MustNew().ForbiddenMoves() {
		if fm.ID != rawOwnerWriteForbiddenID {
			continue
		}
		for _, a := range strings.Split(fm.Fields["action_aliases"], ",") {
			if n := normalizeAction(a); n != "" {
				seedAliases = append(seedAliases, n)
			}
		}
	}
	if len(seedAliases) == 0 {
		t.Fatalf("compiled seed pack has no action_aliases for %q", rawOwnerWriteForbiddenID)
	}
	sort.Strings(seedAliases)

	got := rawOwnerWriteAliasList()
	if strings.Join(got, ",") != strings.Join(seedAliases, ",") {
		t.Errorf("govops alias set drifted from the compiled seed pack:\n got:  %v\n seed: %v", got, seedAliases)
	}
	// And every seed alias is actually enforced by the gate.
	for _, a := range seedAliases {
		if matchRawOwnerWrite(a) == "" {
			t.Errorf("seed alias %q is authored but not enforced by govops", a)
		}
	}
}
