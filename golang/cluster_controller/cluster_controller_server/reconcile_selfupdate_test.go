package main

import (
	"context"
	"testing"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestReconcileControllerSelfUpdate_NoSafeSuccessorWritesPendingRecord(t *testing.T) {
	srv := newTestServer(t, &controllerState{})
	srv.etcdClient = &clientv3.Client{}

	origRead := readControllerTargetBuildFn
	origFindSelf := findSelfNodeIDFn
	origEval := evaluateControllerFollowersFn
	origWrite := writeLeaderPendingUpdate
	origClear := clearLeaderPendingUpdate
	t.Cleanup(func() {
		readControllerTargetBuildFn = origRead
		findSelfNodeIDFn = origFindSelf
		evaluateControllerFollowersFn = origEval
		writeLeaderPendingUpdate = origWrite
		clearLeaderPendingUpdate = origClear
		leaderStuckSince.Store(0)
	})

	readControllerTargetBuildFn = func(_ *server, _ context.Context) (*controllerTargetBuild, error) {
		return &controllerTargetBuild{Version: "1.0.99", BuildNumber: 7}, nil
	}
	findSelfNodeIDFn = func(_ *server) string { return "node-leader" }
	evaluateControllerFollowersFn = func(_ *server, _ context.Context, _ string, _ *controllerTargetBuild) (int, int, map[string]string) {
		return 0, 2, map[string]string{"node-f1": "installed 1.0.98+1, target 1.0.99+7"}
	}

	var wrote bool
	writeLeaderPendingUpdate = func(_ context.Context, rec LeaderPendingUpdateRecord) {
		wrote = true
		if rec.LeaderNodeID != "node-leader" {
			t.Fatalf("LeaderNodeID=%q want node-leader", rec.LeaderNodeID)
		}
	}
	clearLeaderPendingUpdate = func(_ context.Context) {
		t.Fatal("clearLeaderPendingUpdate must not be called when no safe successor exists")
	}

	srv.reconcileControllerSelfUpdate(context.Background())

	if !wrote {
		t.Fatal("expected writeLeaderPendingUpdate call when safeSuccessors==0")
	}
	select {
	case <-srv.resignCh:
		t.Fatal("unexpected resignation signal when no safe successor exists")
	default:
	}
}

func TestReconcileControllerSelfUpdate_SafeSuccessorClearsPendingAndResigns(t *testing.T) {
	srv := newTestServer(t, &controllerState{})
	srv.etcdClient = &clientv3.Client{}

	origRead := readControllerTargetBuildFn
	origFindSelf := findSelfNodeIDFn
	origEval := evaluateControllerFollowersFn
	origWrite := writeLeaderPendingUpdate
	origClear := clearLeaderPendingUpdate
	t.Cleanup(func() {
		readControllerTargetBuildFn = origRead
		findSelfNodeIDFn = origFindSelf
		evaluateControllerFollowersFn = origEval
		writeLeaderPendingUpdate = origWrite
		clearLeaderPendingUpdate = origClear
		leaderStuckSince.Store(0)
	})

	readControllerTargetBuildFn = func(_ *server, _ context.Context) (*controllerTargetBuild, error) {
		return &controllerTargetBuild{Version: "1.0.99", BuildNumber: 7}, nil
	}
	findSelfNodeIDFn = func(_ *server) string { return "node-leader" }
	evaluateControllerFollowersFn = func(_ *server, _ context.Context, _ string, _ *controllerTargetBuild) (int, int, map[string]string) {
		return 1, 2, map[string]string{}
	}

	writeLeaderPendingUpdate = func(_ context.Context, _ LeaderPendingUpdateRecord) {
		t.Fatal("writeLeaderPendingUpdate must not be called when a safe successor exists")
	}
	var cleared bool
	clearLeaderPendingUpdate = func(_ context.Context) {
		cleared = true
	}

	srv.reconcileControllerSelfUpdate(context.Background())

	if !cleared {
		t.Fatal("expected clearLeaderPendingUpdate call when safe successor exists")
	}
	select {
	case <-srv.resignCh:
	default:
		t.Fatal("expected resignation signal when safe successor exists")
	}
}
