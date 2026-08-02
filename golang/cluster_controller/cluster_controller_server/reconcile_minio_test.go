package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	configpkg "github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

func newTestMinioReconciler(now time.Time) *minioTopologyReconciler {
	r := &minioTopologyReconciler{
		interval: minioTopologyReconcileInterval,
		now:      func() time.Time { return now },
	}
	r.writeOutcome = func(ctx context.Context, out minioReconcileOutcome) error { return nil }
	return r
}

func TestMinioTopologyDriftDispatchesWorkflow(t *testing.T) {
	now := time.Now()
	r := newTestMinioReconciler(now)
	r.loadDesired = func(ctx context.Context) (*configpkg.ObjectStoreDesiredState, error) {
		return &configpkg.ObjectStoreDesiredState{Generation: 7}, nil
	}
	r.loadAppliedGen = func(ctx context.Context) (int64, error) { return 6, nil }
	r.snapshotStorageNodes = func() []minioNodeHealth {
		return []minioNodeHealth{
			{nodeID: "n1", isStorageNode: true, minioActive: true, lastSeen: now},
			{nodeID: "n2", isStorageNode: true, minioActive: true, lastSeen: now},
			{nodeID: "n3", isStorageNode: true, minioActive: true, lastSeen: now},
		}
	}
	dispatched := 0
	r.runTopologyWorkflow = func(ctx context.Context, targetGeneration int64) (*workflowpb.ExecuteWorkflowResponse, error) {
		dispatched++
		if targetGeneration != 7 {
			t.Fatalf("target_generation=%d want 7", targetGeneration)
		}
		return &workflowpb.ExecuteWorkflowResponse{Status: "SUCCEEDED"}, nil
	}

	r.runOnce(context.Background())
	if dispatched != 1 {
		t.Fatalf("expected 1 dispatch, got %d", dispatched)
	}
}

func TestMinioNoDispatchWhenTopologyCurrent(t *testing.T) {
	now := time.Now()
	r := newTestMinioReconciler(now)
	r.loadDesired = func(ctx context.Context) (*configpkg.ObjectStoreDesiredState, error) {
		return &configpkg.ObjectStoreDesiredState{Generation: 10}, nil
	}
	r.loadAppliedGen = func(ctx context.Context) (int64, error) { return 10, nil }
	r.snapshotStorageNodes = func() []minioNodeHealth {
		return []minioNodeHealth{
			{nodeID: "n1", isStorageNode: true, minioActive: true, lastSeen: now},
			{nodeID: "n2", isStorageNode: true, minioActive: true, lastSeen: now},
			{nodeID: "n3", isStorageNode: true, minioActive: true, lastSeen: now},
		}
	}
	dispatched := 0
	r.runTopologyWorkflow = func(ctx context.Context, targetGeneration int64) (*workflowpb.ExecuteWorkflowResponse, error) {
		dispatched++
		return &workflowpb.ExecuteWorkflowResponse{Status: "SUCCEEDED"}, nil
	}

	r.runOnce(context.Background())
	if dispatched != 0 {
		t.Fatalf("expected 0 dispatches, got %d", dispatched)
	}
}

func TestMinioStorageCapacityDoesNotBlockDriftDispatch(t *testing.T) {
	now := time.Now()
	r := newTestMinioReconciler(now)
	var outcomes []minioReconcileOutcome
	r.writeOutcome = func(ctx context.Context, out minioReconcileOutcome) error {
		outcomes = append(outcomes, out)
		return nil
	}
	r.loadDesired = func(ctx context.Context) (*configpkg.ObjectStoreDesiredState, error) {
		return &configpkg.ObjectStoreDesiredState{Generation: 3}, nil
	}
	r.loadAppliedGen = func(ctx context.Context) (int64, error) { return 1, nil }
	r.snapshotStorageNodes = func() []minioNodeHealth {
		return []minioNodeHealth{
			{nodeID: "n1", isStorageNode: true, minioActive: false, lastSeen: now.Add(-20 * time.Minute)},
			{nodeID: "n2", isStorageNode: true, minioActive: false, lastSeen: now.Add(-20 * time.Minute)},
		}
	}
	dispatched := 0
	r.runTopologyWorkflow = func(ctx context.Context, targetGeneration int64) (*workflowpb.ExecuteWorkflowResponse, error) {
		dispatched++
		return &workflowpb.ExecuteWorkflowResponse{Status: "SUCCEEDED"}, nil
	}

	r.runOnce(context.Background())
	if dispatched != 1 {
		t.Fatalf("expected storage capacity to be reported but not block dispatch, got %d dispatches", dispatched)
	}
	if len(outcomes) == 0 {
		t.Fatal("expected reconcile outcome to be recorded")
	}
	last := outcomes[len(outcomes)-1]
	if last.StorageNodes != 2 {
		t.Fatalf("expected storage_nodes=2 in outcome, got %+v", last)
	}
	if last.Outcome == "SKIP_NO_QUORUM" {
		t.Fatalf("storage capacity must not produce SKIP_NO_QUORUM: %+v", last)
	}
}

func TestMinioBackoffAfterTransientFailure(t *testing.T) {
	now := time.Now()
	r := newTestMinioReconciler(now)
	r.loadDesired = func(ctx context.Context) (*configpkg.ObjectStoreDesiredState, error) {
		return &configpkg.ObjectStoreDesiredState{Generation: 4}, nil
	}
	r.loadAppliedGen = func(ctx context.Context) (int64, error) { return 1, nil }
	r.snapshotStorageNodes = func() []minioNodeHealth {
		return []minioNodeHealth{
			{nodeID: "n1", isStorageNode: true, minioActive: true, lastSeen: now},
			{nodeID: "n2", isStorageNode: true, minioActive: true, lastSeen: now},
			{nodeID: "n3", isStorageNode: true, minioActive: true, lastSeen: now},
		}
	}

	dispatched := 0
	r.runTopologyWorkflow = func(ctx context.Context, targetGeneration int64) (*workflowpb.ExecuteWorkflowResponse, error) {
		dispatched++
		if dispatched == 1 {
			return nil, fmt.Errorf("transient workflow error")
		}
		return &workflowpb.ExecuteWorkflowResponse{Status: "SUCCEEDED"}, nil
	}

	r.runOnce(context.Background()) // first attempt fails
	r.runOnce(context.Background()) // immediate retry should back off

	if dispatched != 1 {
		t.Fatalf("expected backoff to suppress second dispatch, got %d dispatches", dispatched)
	}
}
