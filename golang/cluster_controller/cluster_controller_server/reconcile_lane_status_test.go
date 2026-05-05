package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
	"strings"

	"github.com/globulario/services/golang/cluster_controller/cluster_controller_server/projections"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type fakeKV struct {
	mu sync.Mutex
	m  map[string]string
}

type blockingProjector struct{}

func (blockingProjector) Reconcile(ctx context.Context, _ []projections.NodeIdentity) error {
	<-ctx.Done()
	return ctx.Err()
}

func newFakeKV() *fakeKV {
	return &fakeKV{m: map[string]string{}}
}

func (f *fakeKV) Get(_ context.Context, key string, _ ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	resp := &clientv3.GetResponse{}
	if strings.HasSuffix(key, "/") {
		for k, v := range f.m {
			if strings.HasPrefix(k, key) {
				resp.Kvs = append(resp.Kvs, &mvccpb.KeyValue{Key: []byte(k), Value: []byte(v)})
			}
		}
		return resp, nil
	}
	if v, ok := f.m[key]; ok {
		resp.Kvs = []*mvccpb.KeyValue{{Key: []byte(key), Value: []byte(v)}}
	}
	return resp, nil
}

func (f *fakeKV) Put(_ context.Context, key, val string, _ ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[key] = val
	return &clientv3.PutResponse{}, nil
}

func (f *fakeKV) Delete(_ context.Context, key string, _ ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.m[key]; ok {
		delete(f.m, key)
		return &clientv3.DeleteResponse{Deleted: 1}, nil
	}
	return &clientv3.DeleteResponse{Deleted: 0}, nil
}

func TestPublishReconcileLaneStatus_WritesKV(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	srv.publishReconcileLaneStatus(context.Background(), "cluster_reconcile", reconcileLaneStatus{
		Phase:   "BLOCKED",
		Running: true,
	})

	resp, err := kv.Get(context.Background(), "/globular/controller/reconcile/lanes/cluster_reconcile")
	if err != nil {
		t.Fatalf("kv get: %v", err)
	}
	if len(resp.Kvs) == 0 {
		t.Fatal("expected lane status key to be written")
	}
	var st reconcileLaneStatus
	if err := json.Unmarshal(resp.Kvs[0].Value, &st); err != nil {
		t.Fatalf("unmarshal lane status: %v", err)
	}
	if st.Lane != "cluster_reconcile" || st.Phase != "BLOCKED" || !st.Running {
		t.Fatalf("unexpected lane status: %+v", st)
	}
}

func TestConsumeManualRequests_DeletesKeys(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	_, _ = kv.Put(context.Background(), scyllaSchemaGuardEnforceRequestKey, "1")
	if !srv.consumeScyllaSchemaGuardEnforceRequest(context.Background()) {
		t.Fatal("expected consumeScyllaSchemaGuardEnforceRequest=true")
	}
	resp, _ := kv.Get(context.Background(), scyllaSchemaGuardEnforceRequestKey)
	if len(resp.Kvs) != 0 {
		t.Fatal("expected enforce_request key to be deleted")
	}

	_, _ = kv.Put(context.Background(), ingressRepublishRequestKey, "1")
	if !srv.consumeIngressRepublishRequest(context.Background()) {
		t.Fatal("expected consumeIngressRepublishRequest=true")
	}
	resp, _ = kv.Get(context.Background(), ingressRepublishRequestKey)
	if len(resp.Kvs) != 0 {
		t.Fatal("expected republish_request key to be deleted")
	}
}

func TestClusterReconcileOverlapPublishesBlockedLaneStatus(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv
	srv.setLeader(true, "test", "127.0.0.1:1234")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.startControllerRuntime(ctx, 1)

	// Wait for runtime wiring to install runClusterReconcileIfIdle callback.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if srv.runClusterReconcileIfIdle != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if srv.runClusterReconcileIfIdle == nil {
		t.Fatal("runClusterReconcileIfIdle not initialized")
	}

	// Force overlap path.
	srv.clusterReconcileRunning.Store(true)
	srv.runClusterReconcileIfIdle(context.Background(), "test")

	resp, err := kv.Get(context.Background(), "/globular/controller/reconcile/lanes/cluster_reconcile")
	if err != nil {
		t.Fatalf("kv get lane status: %v", err)
	}
	if len(resp.Kvs) == 0 {
		t.Fatal("expected cluster_reconcile lane status key")
	}
	var st reconcileLaneStatus
	if err := json.Unmarshal(resp.Kvs[0].Value, &st); err != nil {
		t.Fatalf("unmarshal lane status: %v", err)
	}
	if st.Phase != "BLOCKED" || !st.PreviousRunAlive {
		t.Fatalf("expected BLOCKED previous-run-active status, got %+v", st)
	}
}

func TestReleaseBridgeOverlapPublishesBlockedLaneStatus(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv
	srv.setLeader(true, "test", "127.0.0.1:1234")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.startControllerRuntime(ctx, 1)

	// Force overlap path for release bridge lane.
	srv.releaseBridgeRunning.Store(true)
	srv.publishReconcileLaneStatus(context.Background(), "release_bridge", reconcileLaneStatus{
		Phase:            "BLOCKED",
		Running:          true,
		PreviousRunAlive: true,
		LastError:        "previous run still active",
	})

	resp, err := kv.Get(context.Background(), "/globular/controller/reconcile/lanes/release_bridge")
	if err != nil {
		t.Fatalf("kv get lane status: %v", err)
	}
	if len(resp.Kvs) == 0 {
		t.Fatal("expected release_bridge lane status key")
	}
	var st reconcileLaneStatus
	if err := json.Unmarshal(resp.Kvs[0].Value, &st); err != nil {
		t.Fatalf("unmarshal lane status: %v", err)
	}
	if st.Phase != "BLOCKED" || !st.PreviousRunAlive {
		t.Fatalf("expected BLOCKED previous-run-active status, got %+v", st)
	}
}

func TestDriftReconcileOverlapPublishesBlockedLaneStatus(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv
	srv.setLeader(true, "test", "127.0.0.1:1234")

	// Force overlap path for drift lane directly.
	srv.driftReconcileRunning.Store(true)
	srv.publishReconcileLaneStatus(context.Background(), "drift_reconcile", reconcileLaneStatus{
		Phase:            "BLOCKED",
		Running:          true,
		PreviousRunAlive: true,
		LastError:        "previous run still active",
	})

	resp, err := kv.Get(context.Background(), "/globular/controller/reconcile/lanes/drift_reconcile")
	if err != nil {
		t.Fatalf("kv get lane status: %v", err)
	}
	if len(resp.Kvs) == 0 {
		t.Fatal("expected drift_reconcile lane status key")
	}
	var st reconcileLaneStatus
	if err := json.Unmarshal(resp.Kvs[0].Value, &st); err != nil {
		t.Fatalf("unmarshal lane status: %v", err)
	}
	if st.Phase != "BLOCKED" || !st.PreviousRunAlive {
		t.Fatalf("expected BLOCKED previous-run-active status, got %+v", st)
	}
}

func TestRecordClusterReconcileOutcome_TimeoutPublishesTimeoutStatus(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	srv.recordClusterReconcileOutcome(context.Background(), errors.New("workflow timeout"), context.DeadlineExceeded)

	resp, err := kv.Get(context.Background(), "/globular/controller/reconcile/lanes/cluster_reconcile")
	if err != nil {
		t.Fatalf("kv get lane status: %v", err)
	}
	if len(resp.Kvs) == 0 {
		t.Fatal("expected cluster_reconcile lane status key")
	}
	var st reconcileLaneStatus
	if err := json.Unmarshal(resp.Kvs[0].Value, &st); err != nil {
		t.Fatalf("unmarshal lane status: %v", err)
	}
	if st.Phase != "TIMEOUT" {
		t.Fatalf("expected TIMEOUT phase, got %+v", st)
	}
}

func TestProjectionLaneTimeoutPublishesTimeoutStatus(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv
	srv.setLeader(true, "test", "127.0.0.1:1234")

	oldTimeout := projectionLaneTimeout
	oldDelay := projectionLaneInitialDelay
	projectionLaneTimeout = 50 * time.Millisecond
	projectionLaneInitialDelay = 1 * time.Millisecond
	defer func() {
		projectionLaneTimeout = oldTimeout
		projectionLaneInitialDelay = oldDelay
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go srv.startProjectionReconcileLane(ctx, blockingProjector{}, 200*time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		resp, _ := kv.Get(context.Background(), "/globular/controller/reconcile/lanes/projections")
		if len(resp.Kvs) == 0 {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		var st reconcileLaneStatus
		if json.Unmarshal(resp.Kvs[0].Value, &st) == nil && st.Phase == "TIMEOUT" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected projections lane TIMEOUT status")
}

func TestRecordReleaseBridgeOutcome_TimeoutPublishesTimeoutStatus(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	srv.recordReleaseBridgeOutcome(context.Background(), context.DeadlineExceeded)

	resp, err := kv.Get(context.Background(), "/globular/controller/reconcile/lanes/release_bridge")
	if err != nil {
		t.Fatalf("kv get lane status: %v", err)
	}
	if len(resp.Kvs) == 0 {
		t.Fatal("expected release_bridge lane status key")
	}
	var st reconcileLaneStatus
	if err := json.Unmarshal(resp.Kvs[0].Value, &st); err != nil {
		t.Fatalf("unmarshal lane status: %v", err)
	}
	if st.Phase != "TIMEOUT" {
		t.Fatalf("expected TIMEOUT phase, got %+v", st)
	}
}

func TestRecordDriftReconcileOutcome_TimeoutPublishesTimeoutStatus(t *testing.T) {
	kv := newFakeKV()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.kv = kv

	srv.recordDriftReconcileOutcome(context.Background(), context.DeadlineExceeded)

	resp, err := kv.Get(context.Background(), "/globular/controller/reconcile/lanes/drift_reconcile")
	if err != nil {
		t.Fatalf("kv get lane status: %v", err)
	}
	if len(resp.Kvs) == 0 {
		t.Fatal("expected drift_reconcile lane status key")
	}
	var st reconcileLaneStatus
	if err := json.Unmarshal(resp.Kvs[0].Value, &st); err != nil {
		t.Fatalf("unmarshal lane status: %v", err)
	}
	if st.Phase != "TIMEOUT" {
		t.Fatalf("expected TIMEOUT phase, got %+v", st)
	}
}
