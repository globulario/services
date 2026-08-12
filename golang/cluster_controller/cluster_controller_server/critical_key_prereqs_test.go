package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/globulario/services/golang/installed_state"
)

func TestKindCriticalKeyPrereqsService(t *testing.T) {
	keys := kindCriticalKeyPrereqs["SERVICE"]
	if len(keys) == 0 {
		t.Fatal("SERVICE kind should have at least one critical key prereq")
	}
	found := false
	for _, k := range keys {
		if k == "/globular/system/config" {
			found = true
		}
	}
	if !found {
		t.Errorf("SERVICE kind prereqs %v should include /globular/system/config", keys)
	}
}

func TestKindCriticalKeyPrereqsWorkload(t *testing.T) {
	keys := kindCriticalKeyPrereqs["WORKLOAD"]
	if len(keys) == 0 {
		t.Fatal("WORKLOAD kind should have at least one critical key prereq")
	}
}

func TestKindCriticalKeyPrereqsInfrastructure(t *testing.T) {
	keys := kindCriticalKeyPrereqs["INFRASTRUCTURE"]
	if len(keys) != 0 {
		t.Errorf("INFRASTRUCTURE kind should have no prereqs (it creates config); got %v", keys)
	}
}

func TestKindCriticalKeyPrereqsCommand(t *testing.T) {
	keys := kindCriticalKeyPrereqs["COMMAND"]
	if len(keys) != 0 {
		t.Errorf("COMMAND kind should have no prereqs; got %v", keys)
	}
}

func TestPackageCriticalKeyPrereqsKeepalived(t *testing.T) {
	keys := packageCriticalKeyPrereqs["keepalived"]
	found := false
	for _, k := range keys {
		if k == "/globular/ingress/v1/spec" {
			found = true
		}
	}
	if !found {
		t.Errorf("keepalived prereqs %v should include /globular/ingress/v1/spec", keys)
	}
}

func TestPackageCriticalKeyPrereqsEnvoy(t *testing.T) {
	keys := packageCriticalKeyPrereqs["envoy"]
	found := false
	for _, k := range keys {
		if k == "/globular/ingress/v1/spec" {
			found = true
		}
	}
	if !found {
		t.Errorf("envoy prereqs %v should include /globular/ingress/v1/spec", keys)
	}
}

func TestCriticalKeyBlockActionID(t *testing.T) {
	id := criticalKeyBlockActionID("node-abc", "SERVICE", "rbac")
	expected := "controller/node-abc/SERVICE/rbac/critical_key_block"
	if id != expected {
		t.Errorf("criticalKeyBlockActionID = %q, want %q", id, expected)
	}
}

func TestCriticalKeyPrereqsMissingNoPrereqs(t *testing.T) {
	// INFRASTRUCTURE packages have no prereqs — returns "" without hitting etcd.
	missing, checkErr := criticalKeyPrereqStatus(nil, "etcd", "INFRASTRUCTURE")
	if missing != "" {
		t.Errorf("INFRASTRUCTURE pkg should have no prereqs, got missing=%q", missing)
	}
	if checkErr != nil {
		t.Errorf("INFRASTRUCTURE pkg should not check etcd, got checkErr=%v", checkErr)
	}
}

func TestCriticalKeyPrereqStatus_EtcdClientError(t *testing.T) {
	orig := criticalKeyGetEtcdClient
	t.Cleanup(func() { criticalKeyGetEtcdClient = orig })
	criticalKeyGetEtcdClient = func() (*clientv3.Client, error) {
		return nil, errors.New("dial etcd: timeout")
	}

	missing, checkErr := criticalKeyPrereqStatus(context.Background(), "rbac", "SERVICE")
	if missing != "" {
		t.Fatalf("expected no missing key when client creation fails, got %q", missing)
	}
	if checkErr == nil {
		t.Fatal("expected checkErr on etcd client error")
	}
}

func TestWriteCriticalKeyBlock_CheckErrorPayload(t *testing.T) {
	orig := criticalKeyWriteResult
	t.Cleanup(func() { criticalKeyWriteResult = orig })

	var captured *installed_state.ConvergenceResultV1
	criticalKeyWriteResult = func(ctx context.Context, r *installed_state.ConvergenceResultV1) error {
		captured = r
		return nil
	}

	writeCriticalKeyBlock(context.Background(), []string{"node-1"}, "rbac", "SERVICE", "", errors.New("tls: bad certificate"))
	if captured == nil {
		t.Fatal("expected convergence result to be written")
	}
	if captured.Outcome != installed_state.OutcomeBlockedCriticalKeyMissing {
		t.Fatalf("outcome=%s, want %s", captured.Outcome, installed_state.OutcomeBlockedCriticalKeyMissing)
	}
	if captured.ReasonCode != "critical_key_check_error" {
		t.Fatalf("reason_code=%q, want critical_key_check_error", captured.ReasonCode)
	}
	if captured.UnblockPolicy != "check_error_retry_after_backoff" {
		t.Fatalf("unblock_policy=%q, want check_error_retry_after_backoff", captured.UnblockPolicy)
	}
	if captured.Evidence["check_error"] == "" {
		t.Fatal("expected check_error evidence")
	}
}

func TestRuntimeDepBlockKindFromActionID(t *testing.T) {
	got := runtimeDepBlockKindFromActionID("controller/node-1/INFRASTRUCTURE/scylla-manager/runtime_dep_block")
	if got != "INFRASTRUCTURE" {
		t.Fatalf("kind=%q, want INFRASTRUCTURE", got)
	}
	if runtimeDepBlockKindFromActionID("bad-action-id") != "" {
		t.Fatal("expected invalid action id to return empty kind")
	}
}

func TestSweepRuntimeDepBlocks_ClearsWhenDepsSatisfied(t *testing.T) {
	origList := criticalKeyListResults
	origClear := runtimeDepBlockClearFn
	t.Cleanup(func() {
		criticalKeyListResults = origList
		runtimeDepBlockClearFn = origClear
	})

	criticalKeyListResults = func(context.Context, string) ([]*installed_state.ConvergenceResultV1, error) {
		return []*installed_state.ConvergenceResultV1{
			{
				ActionID:   "controller/node-1/INFRASTRUCTURE/scylla-manager/runtime_dep_block",
				Package:    "scylla-manager",
				NodeID:     "node-1",
				Outcome:    installed_state.OutcomeBlockedMissingNativeDep,
				ReasonCode: "runtime_deps_not_ready",
			},
		}, nil
	}

	var clearedNode, clearedPkg, clearedKind string
	runtimeDepBlockClearFn = func(_ context.Context, nodeIDs []string, pkgName, kind string) {
		if len(nodeIDs) > 0 {
			clearedNode = nodeIDs[0]
		}
		clearedPkg = pkgName
		clearedKind = kind
	}

	srv := newTestServer(t, newControllerState())
	node := &nodeState{
		NodeID:          "node-1",
		ScyllaJoinPhase: ScyllaJoinVerified,
		Units: []unitStatusRecord{
			{Name: "scylla-server.service", State: "active"},
		},
	}
	srv.sweepRuntimeDepBlocks(context.Background(), []*nodeState{node})

	if clearedNode != "node-1" || clearedPkg != "scylla-manager" || clearedKind != "INFRASTRUCTURE" {
		t.Fatalf("unexpected clear call: node=%q pkg=%q kind=%q", clearedNode, clearedPkg, clearedKind)
	}
}

func TestSweepRuntimeDepBlocks_DoesNotClearWhenDepsMissing(t *testing.T) {
	origList := criticalKeyListResults
	origClear := runtimeDepBlockClearFn
	t.Cleanup(func() {
		criticalKeyListResults = origList
		runtimeDepBlockClearFn = origClear
	})

	criticalKeyListResults = func(context.Context, string) ([]*installed_state.ConvergenceResultV1, error) {
		return []*installed_state.ConvergenceResultV1{
			{
				ActionID:   "controller/node-1/INFRASTRUCTURE/scylla-manager/runtime_dep_block",
				Package:    "scylla-manager",
				NodeID:     "node-1",
				Outcome:    installed_state.OutcomeBlockedMissingNativeDep,
				ReasonCode: "runtime_deps_not_ready",
			},
		}, nil
	}

	called := false
	runtimeDepBlockClearFn = func(_ context.Context, _ []string, _ string, _ string) {
		called = true
	}

	srv := newTestServer(t, newControllerState())
	node := &nodeState{
		NodeID:          "node-1",
		ScyllaJoinPhase: ScyllaJoinConfigured, // not verified yet
		Units: []unitStatusRecord{
			{Name: "scylla-server.service", State: "active"},
		},
	}
	srv.sweepRuntimeDepBlocks(context.Background(), []*nodeState{node})
	if called {
		t.Fatal("expected no clear call while deps are still missing")
	}
}

func TestSweepRuntimeDepBlocks_ListErrorIsNonFatal(t *testing.T) {
	origList := criticalKeyListResults
	origClear := runtimeDepBlockClearFn
	t.Cleanup(func() {
		criticalKeyListResults = origList
		runtimeDepBlockClearFn = origClear
	})

	criticalKeyListResults = func(context.Context, string) ([]*installed_state.ConvergenceResultV1, error) {
		return nil, fmt.Errorf("etcd timeout")
	}
	called := false
	runtimeDepBlockClearFn = func(_ context.Context, _ []string, _ string, _ string) {
		called = true
	}

	srv := newTestServer(t, newControllerState())
	srv.sweepRuntimeDepBlocks(context.Background(), []*nodeState{{NodeID: "node-1"}})
	if called {
		t.Fatal("clear should not be called on list error")
	}
}

// Clearing a dep block only retracts the explanation for why a workload is
// down; it installs nothing. The reconcile loop is event-driven, so the sweep
// must re-drive it — otherwise the workload is left unblocked AND unconverged
// with the one signal that said why now deleted.
//
// Scar (2026-08-12): authentication on node-4 was gated on rbac. The sweep
// logged "auto-cleared stale block for SERVICE/authentication" and then nothing
// re-dispatched; the unit sat inactive+disabled indefinitely while
// cluster-doctor reported installed_state_runtime_mismatch against it.
func TestSweepRuntimeDepBlocks_ClearReDrivesReconcile(t *testing.T) {
	origList := criticalKeyListResults
	origClear := runtimeDepBlockClearFn
	t.Cleanup(func() {
		criticalKeyListResults = origList
		runtimeDepBlockClearFn = origClear
	})

	criticalKeyListResults = func(context.Context, string) ([]*installed_state.ConvergenceResultV1, error) {
		return []*installed_state.ConvergenceResultV1{
			{
				ActionID:   "controller/node-1/INFRASTRUCTURE/scylla-manager/runtime_dep_block",
				Package:    "scylla-manager",
				NodeID:     "node-1",
				Outcome:    installed_state.OutcomeBlockedMissingNativeDep,
				ReasonCode: "runtime_deps_not_ready",
			},
		}, nil
	}
	runtimeDepBlockClearFn = func(context.Context, []string, string, string) {}

	srv := newTestServer(t, newControllerState())
	reconciles := 0
	srv.enqueueReconcile = func() { reconciles++ }

	node := &nodeState{
		NodeID:          "node-1",
		ScyllaJoinPhase: ScyllaJoinVerified,
		Units: []unitStatusRecord{
			{Name: "scylla-server.service", State: "active"},
		},
	}
	srv.sweepRuntimeDepBlocks(context.Background(), []*nodeState{node})

	if reconciles != 1 {
		t.Fatalf("enqueueReconcile called %d times, want exactly 1 — a cleared block must re-drive the loop it was gating, once per sweep", reconciles)
	}
}

// A sweep that clears nothing must not touch the reconciler: re-driving on
// every idle sweep would turn a periodic safety pass into a reconcile storm.
func TestSweepRuntimeDepBlocks_NoClearDoesNotReDrive(t *testing.T) {
	origList := criticalKeyListResults
	origClear := runtimeDepBlockClearFn
	t.Cleanup(func() {
		criticalKeyListResults = origList
		runtimeDepBlockClearFn = origClear
	})

	criticalKeyListResults = func(context.Context, string) ([]*installed_state.ConvergenceResultV1, error) {
		return []*installed_state.ConvergenceResultV1{
			{
				ActionID:   "controller/node-1/INFRASTRUCTURE/scylla-manager/runtime_dep_block",
				Package:    "scylla-manager",
				NodeID:     "node-1",
				Outcome:    installed_state.OutcomeBlockedMissingNativeDep,
				ReasonCode: "runtime_deps_not_ready",
			},
		}, nil
	}
	runtimeDepBlockClearFn = func(context.Context, []string, string, string) {
		t.Fatal("clear must not run while the dep is still missing")
	}

	srv := newTestServer(t, newControllerState())
	reconciles := 0
	srv.enqueueReconcile = func() { reconciles++ }

	node := &nodeState{
		NodeID:          "node-1",
		ScyllaJoinPhase: ScyllaJoinConfigured, // dep still unsatisfied
		Units: []unitStatusRecord{
			{Name: "scylla-server.service", State: "active"},
		},
	}
	srv.sweepRuntimeDepBlocks(context.Background(), []*nodeState{node})

	if reconciles != 0 {
		t.Fatalf("enqueueReconcile called %d times on a sweep that cleared nothing, want 0", reconciles)
	}
}
