package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/govops"
	pb "github.com/globulario/services/golang/govops/governed_operationpb"
)

// writeReq writes an OperationRequest JSON file and returns its path.
func writeReq(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "op.json")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// allowedProductionReq is a clean, owner-routed desired write (scope=production).
const allowedProductionReq = `{
  "id":"op-ok","actor":"ACTOR_AGENT","action":"set_desired_version",
  "target":{"resourceType":"ServiceDesiredVersion","resourceId":"echo","owner":"cluster_controller"},
  "authority":{"requiredOwnerPath":"cluster_controller.UpsertDesiredService","callerIdentity":"sa:mcp"},
  "expectedEffect":{"mutatesDesired":true,"bumpsGeneration":true,"triggersReconcile":true,"refreshesProjection":true},
  "evidence":{"beforeSnapshot":"sha256:abc"},
  "execution":{"mode":"APPLY","approvedPath":"OWNER_RPC"},
  "postconditions":{"requiredChecks":["generation_advanced"],"rollbackPlan":"re-pin","ledgerRequired":true},
  "parameters":{"version":"1.2.3","build_number":"7"}
}`

// rawWriteReq is refused (raw write to owner-owned state).
const rawWriteReq = `{
  "id":"op-raw","actor":"ACTOR_AGENT","action":"etcdctl_put",
  "target":{"resourceId":"xds","owner":"cluster_controller"},
  "authority":{"requiredOwnerPath":"cluster_controller.UpsertDesiredService","callerIdentity":"sa:mcp"},
  "expectedEffect":{"mutatesDesired":true,"bumpsGeneration":true,"triggersReconcile":true,"refreshesProjection":true},
  "evidence":{"beforeSnapshot":"sha256:def"},
  "execution":{"mode":"APPLY","approvedPath":"DIAGNOSTIC_READONLY"},
  "postconditions":{"requiredChecks":["x"],"rollbackPlan":"y","ledgerRequired":true},
  "parameters":{"version":"1.2.3"}
}`

// stubApply swaps the dispatcher + ledger store for tests and restores them.
func stubApply(t *testing.T, dispatch func(context.Context, *pb.OperationRequest) (dispatchOutcome, error)) *govops.MemLedgerStore {
	t.Helper()
	mem := govops.NewMemLedgerStore()
	origDispatch, origStore := ownerDispatch, opsLedgerStore
	origYes, origDry := opsApplyYes, opsApplyDryRun
	ownerDispatch = dispatch
	opsLedgerStore = mem
	t.Cleanup(func() {
		ownerDispatch, opsLedgerStore = origDispatch, origStore
		opsApplyYes, opsApplyDryRun = origYes, origDry
	})
	return mem
}

func TestDispatchThroughOwnerPath_UnknownPath(t *testing.T) {
	req := &pb.OperationRequest{Authority: &pb.OperationAuthority{RequiredOwnerPath: "mystery.Service"}}
	_, err := dispatchThroughOwnerPath(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "no governed dispatcher") {
		t.Fatalf("unknown owner path: want 'no governed dispatcher' error, got %v", err)
	}
}

// Each known owner path routes to its handler, and each handler fails closed on
// missing typed parameters BEFORE any dispatch (no generic key/path/value hatch).
func TestDispatchRoutesKnownPathsAndFailsClosed(t *testing.T) {
	cases := []struct{ path, wantErr string }{
		{"cluster_controller.UpsertDesiredService", "resourceId"},
		{"cluster_controller.RemoveDesiredService", "resourceId"},
		{"cluster_controller.ApplyInfrastructureRelease", "is required"},
	}
	for _, tc := range cases {
		req := &pb.OperationRequest{Authority: &pb.OperationAuthority{RequiredOwnerPath: tc.path}}
		_, err := dispatchThroughOwnerPath(context.Background(), req)
		if err == nil {
			t.Errorf("%s: want a validation error for an empty request", tc.path)
			continue
		}
		if strings.Contains(err.Error(), "no governed dispatcher") {
			t.Errorf("%s: not routed — got unknown-path error", tc.path)
		}
		if !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: error = %v, want contains %q", tc.path, err, tc.wantErr)
		}
	}
}

// ApplyInfrastructureRelease requires every typed parameter — omitting any one
// fails closed before dispatch (the stricter infra-release contract).
func TestDispatchApplyInfra_RequiresEachTypedParam(t *testing.T) {
	full := map[string]string{
		"service_id": "xds", "publisher_id": "core@globular.io", "version": "1.2.235",
		"release_channel": "stable", "reason": "rollout",
	}
	for omit := range full {
		params := map[string]string{}
		for k, v := range full {
			if k != omit {
				params[k] = v
			}
		}
		req := &pb.OperationRequest{
			Authority:  &pb.OperationAuthority{RequiredOwnerPath: "cluster_controller.ApplyInfrastructureRelease"},
			Parameters: params,
		}
		_, err := dispatchThroughOwnerPath(context.Background(), req)
		if err == nil || !strings.Contains(err.Error(), "is required") {
			t.Errorf("omitting %q: want 'is required' error before dispatch, got %v", omit, err)
		}
	}
}

func TestOpsApply_RefusedNotDispatched(t *testing.T) {
	dispatched := false
	mem := stubApply(t, func(context.Context, *pb.OperationRequest) (dispatchOutcome, error) {
		dispatched = true
		return dispatchOutcome{}, nil
	})
	err := runOpsApply(nil, []string{writeReq(t, rawWriteReq)})
	if err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("refused op: want refusal error, got %v", err)
	}
	if dispatched {
		t.Error("a refused operation must NOT be dispatched")
	}
	// Refusal is still auditable.
	entries, _ := mem.List(context.Background())
	if len(entries) != 1 || entries[0].GetResult() != pb.OperationResult_REFUSED {
		t.Errorf("refused op must be ledgered as REFUSED, got %+v", entries)
	}
}

func TestOpsApply_ProductionScopeRequiresYes(t *testing.T) {
	dispatched := false
	stubApply(t, func(context.Context, *pb.OperationRequest) (dispatchOutcome, error) {
		dispatched = true
		return dispatchOutcome{Succeeded: true}, nil
	})
	opsApplyYes = false
	err := runOpsApply(nil, []string{writeReq(t, allowedProductionReq)})
	if err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("production scope without --yes: want scope error, got %v", err)
	}
	if dispatched {
		t.Error("must not dispatch a production op without --yes")
	}
}

func TestOpsApply_DispatchOnAllowed(t *testing.T) {
	dispatched := false
	mem := stubApply(t, func(_ context.Context, req *pb.OperationRequest) (dispatchOutcome, error) {
		dispatched = true
		if got := req.GetParameters()["version"]; got != "1.2.3" {
			t.Errorf("dispatcher received version %q, want 1.2.3", got)
		}
		return dispatchOutcome{Succeeded: true, GenerationAfter: "42", PostconditionsPassed: []string{"owner_rpc_accepted"}}, nil
	})
	opsApplyYes = true
	if err := runOpsApply(nil, []string{writeReq(t, allowedProductionReq)}); err != nil {
		t.Fatalf("allowed op with --yes: %v", err)
	}
	if !dispatched {
		t.Fatal("an allowed, confirmed operation must be dispatched")
	}
	entries, _ := mem.List(context.Background())
	if len(entries) != 1 || entries[0].GetResult() != pb.OperationResult_COMPLETED {
		t.Fatalf("dispatched op must be ledgered COMPLETED, got %+v", entries)
	}
	if entries[0].GetGenerationAfter() != "42" {
		t.Errorf("ledger must record generation_after from the owner RPC, got %q", entries[0].GetGenerationAfter())
	}
}

// --dry-run validates + ledgers ALLOWED without dispatching, even at production scope.
func TestOpsApply_DryRunDoesNotDispatch(t *testing.T) {
	dispatched := false
	mem := stubApply(t, func(context.Context, *pb.OperationRequest) (dispatchOutcome, error) {
		dispatched = true
		return dispatchOutcome{Succeeded: true}, nil
	})
	opsApplyDryRun = true
	if err := runOpsApply(nil, []string{writeReq(t, allowedProductionReq)}); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if dispatched {
		t.Error("--dry-run must not dispatch")
	}
	entries, _ := mem.List(context.Background())
	if len(entries) != 1 || entries[0].GetResult() != pb.OperationResult_ALLOWED {
		t.Errorf("dry-run must ledger ALLOWED, got %+v", entries)
	}
}
