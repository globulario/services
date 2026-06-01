package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	configpkg "github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

const (
	minioReconcileOutcomeKey        = "/globular/objectstore/reconcile/last"
	minioLegacyTopologyGeneration   = "/globular/objectstore/topology/generation"
	minioInactiveDriftAfter         = 10 * time.Minute
	minioTopologyReconcileInterval  = 5 * time.Minute
)

type minioReconcileOutcome struct {
	TimestampUnix int64  `json:"timestamp_unix"`
	Outcome       string `json:"outcome"`
	Reason        string `json:"reason"`
	DesiredGen    int64  `json:"desired_generation,omitempty"`
	AppliedGen    int64  `json:"applied_generation,omitempty"`
	StorageNodes  int    `json:"storage_nodes,omitempty"`
}

type minioNodeHealth struct {
	nodeID        string
	lastSeen      time.Time
	hasMinioUnit  bool
	minioActive   bool
	isStorageNode bool
}

// minioTopologyReconciler drives controller-side MinIO topology convergence.
// It compares desired generation against applied generation and dispatches the
// topology workflow when drift is detected. The quorum guard (storageCount <
// MinQuorumNodes) prevents workflow dispatch against a degraded storage layer.
//
type minioTopologyReconciler struct {
	srv      *server
	interval time.Duration

	mu          sync.Mutex
	inflight    bool
	lastAttempt time.Time
	attempts    int32

	now                  func() time.Time
	loadDesired          func(ctx context.Context) (*configpkg.ObjectStoreDesiredState, error)
	loadAppliedGen       func(ctx context.Context) (int64, error)
	snapshotStorageNodes func() []minioNodeHealth
	runTopologyWorkflow  func(ctx context.Context, targetGeneration int64) (*workflowpb.ExecuteWorkflowResponse, error)
	writeOutcome         func(ctx context.Context, out minioReconcileOutcome) error
}

func newMinioTopologyReconciler(srv *server) *minioTopologyReconciler {
	r := &minioTopologyReconciler{
		srv:      srv,
		interval: minioTopologyReconcileInterval,
		now:      time.Now,
	}
	r.loadDesired = func(ctx context.Context) (*configpkg.ObjectStoreDesiredState, error) {
		return configpkg.LoadObjectStoreDesiredState(ctx)
	}
	r.loadAppliedGen = r.defaultLoadAppliedGeneration
	r.snapshotStorageNodes = r.defaultSnapshotStorageNodes
	r.runTopologyWorkflow = func(ctx context.Context, targetGeneration int64) (*workflowpb.ExecuteWorkflowResponse, error) {
		return srv.RunObjectStoreTopologyWorkflow(ctx, targetGeneration)
	}
	r.writeOutcome = r.defaultWriteOutcome
	return r
}

func (r *minioTopologyReconciler) Start(ctx context.Context) {
	safeGo("minio-topology-reconciler", func() {
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !r.srv.isLeader() {
					continue
				}
				r.runOnce(ctx)
			}
		}
	})
}

func (r *minioTopologyReconciler) runOnce(ctx context.Context) {
	now := r.now()
	desired, err := r.loadDesired(ctx)
	if err != nil {
		r.recordOutcome(ctx, minioReconcileOutcome{TimestampUnix: now.Unix(), Outcome: "ERROR", Reason: "desired_load_failed: " + err.Error()})
		return
	}
	if desired == nil {
		r.recordOutcome(ctx, minioReconcileOutcome{TimestampUnix: now.Unix(), Outcome: "SKIP", Reason: "desired_state_missing"})
		return
	}

	nodes := r.snapshotStorageNodes()
	storageCount := 0
	unhealthyForLong := false
	for _, n := range nodes {
		if !n.isStorageNode {
			continue
		}
		storageCount++
		if !n.minioActive {
			if n.lastSeen.IsZero() || now.Sub(n.lastSeen) >= minioInactiveDriftAfter {
				unhealthyForLong = true
			}
		}
	}
	if storageCount < MinQuorumNodes {
		r.recordOutcome(ctx, minioReconcileOutcome{
			TimestampUnix: now.Unix(),
			Outcome:       "SKIP_NO_QUORUM",
			Reason:        fmt.Sprintf("storage_nodes_below_quorum:%d", storageCount),
			DesiredGen:    desired.Generation,
			StorageNodes:  storageCount,
		})
		return
	}

	appliedGen, err := r.loadAppliedGen(ctx)
	if err != nil {
		r.recordOutcome(ctx, minioReconcileOutcome{
			TimestampUnix: now.Unix(),
			Outcome:       "ERROR",
			Reason:        "applied_generation_load_failed: " + err.Error(),
			DesiredGen:    desired.Generation,
			StorageNodes:  storageCount,
		})
		return
	}

	drift := desired.Generation > appliedGen || unhealthyForLong
	if !drift {
		r.recordOutcome(ctx, minioReconcileOutcome{
			TimestampUnix: now.Unix(),
			Outcome:       "OK",
			Reason:        "topology_current",
			DesiredGen:    desired.Generation,
			AppliedGen:    appliedGen,
			StorageNodes:  storageCount,
		})
		return
	}

	r.mu.Lock()
	if r.inflight {
		r.mu.Unlock()
		r.recordOutcome(ctx, minioReconcileOutcome{
			TimestampUnix: now.Unix(),
			Outcome:       "DEFERRED",
			Reason:        "workflow_inflight",
			DesiredGen:    desired.Generation,
			AppliedGen:    appliedGen,
			StorageNodes:  storageCount,
		})
		return
	}
	if !r.lastAttempt.IsZero() {
		backoff := convergenceBackoff(r.attempts)
		if now.Sub(r.lastAttempt) < backoff {
			wait := backoff - now.Sub(r.lastAttempt)
			r.mu.Unlock()
			r.recordOutcome(ctx, minioReconcileOutcome{
				TimestampUnix: now.Unix(),
				Outcome:       "BACKOFF",
				Reason:        fmt.Sprintf("retry_in_%s", wait.Round(time.Second)),
				DesiredGen:    desired.Generation,
				AppliedGen:    appliedGen,
				StorageNodes:  storageCount,
			})
			return
		}
	}
	r.inflight = true
	r.lastAttempt = now
	r.mu.Unlock()

	wctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()
	_, err = r.runTopologyWorkflow(wctx, desired.Generation)

	r.mu.Lock()
	r.inflight = false
	if err != nil {
		r.attempts++
	} else {
		r.attempts = 0
	}
	r.mu.Unlock()

	if err != nil {
		r.recordOutcome(ctx, minioReconcileOutcome{
			TimestampUnix: now.Unix(),
			Outcome:       "FAILED_TRANSIENT",
			Reason:        err.Error(),
			DesiredGen:    desired.Generation,
			AppliedGen:    appliedGen,
			StorageNodes:  storageCount,
		})
		return
	}
	r.recordOutcome(ctx, minioReconcileOutcome{
		TimestampUnix: now.Unix(),
		Outcome:       "DISPATCHED",
		Reason:        "objectstore.minio.apply_topology_generation",
		DesiredGen:    desired.Generation,
		AppliedGen:    appliedGen,
		StorageNodes:  storageCount,
	})
}

func (r *minioTopologyReconciler) defaultSnapshotStorageNodes() []minioNodeHealth {
	r.srv.lock("minio-topology-reconciler:snapshot")
	defer r.srv.unlock()
	out := make([]minioNodeHealth, 0, len(r.srv.state.Nodes))
	for id, n := range r.srv.state.Nodes {
		mh := minioNodeHealth{
			nodeID:        id,
			lastSeen:      n.LastSeen,
			isStorageNode: nodeHasMinioProfile(n),
		}
		for _, u := range n.Units {
			if u.Name == "globular-minio.service" {
				mh.hasMinioUnit = true
				if u.State == "active" {
					mh.minioActive = true
				}
				break
			}
		}
		out = append(out, mh)
	}
	return out
}

func (r *minioTopologyReconciler) defaultLoadAppliedGeneration(ctx context.Context) (int64, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return 0, err
	}
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Primary key.
	if resp, err := cli.Get(tctx, configpkg.EtcdKeyObjectStoreAppliedGeneration); err == nil && len(resp.Kvs) > 0 {
		var v int64
		if _, scanErr := fmt.Sscan(string(resp.Kvs[0].Value), &v); scanErr == nil {
			return v, nil
		}
	}
	// Legacy fallback key (used by older troubleshooting scripts).
	if resp, err := cli.Get(tctx, minioLegacyTopologyGeneration); err == nil && len(resp.Kvs) > 0 {
		var v int64
		if _, scanErr := fmt.Sscan(string(resp.Kvs[0].Value), &v); scanErr == nil {
			return v, nil
		}
	}
	return 0, nil
}

func (r *minioTopologyReconciler) defaultWriteOutcome(ctx context.Context, out minioReconcileOutcome) error {
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	return config.PutRuntimeWithClass(ctx, minioReconcileOutcomeKey, b, config.CriticalWrite)
}

func (r *minioTopologyReconciler) recordOutcome(ctx context.Context, out minioReconcileOutcome) {
	if err := r.writeOutcome(ctx, out); err != nil {
		log.Printf("minio-topology-reconciler: write outcome failed: %v", err)
	}
}
