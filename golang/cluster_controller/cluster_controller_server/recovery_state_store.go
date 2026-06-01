package main

// recovery_state_store.go — etcd persistence for NodeRecoveryState and
// NodeRecoverySnapshot. Also provides the reconcile fencing query used by
// reconcileNodes().
//
// etcd key schema:
//
//   /globular/recovery/nodes/<node_id>/state
//         NodeRecoveryState (JSON)
//
//   /globular/recovery/nodes/<node_id>/snapshots/<snapshot_id>
//         NodeRecoverySnapshot (JSON)
//
//   /globular/recovery/nodes/<node_id>/artifacts/<name>
//         NodeRecoveryArtifactResult (JSON)

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

const (
	recoveryKeyPrefix = "/globular/recovery/nodes/"
)

// recoveryStateKey returns the etcd key for a node's recovery state.
func recoveryStateKey(nodeID string) string {
	return recoveryKeyPrefix + nodeID + "/state"
}

// recoverySnapshotKey returns the etcd key for a specific snapshot.
func recoverySnapshotKey(nodeID, snapshotID string) string {
	return recoveryKeyPrefix + nodeID + "/snapshots/" + snapshotID
}

// recoverySnapshotPrefix returns the etcd prefix for all snapshots of a node.
func recoverySnapshotPrefix(nodeID string) string {
	return recoveryKeyPrefix + nodeID + "/snapshots/"
}

// recoveryArtifactKey returns the etcd key for a per-artifact result.
// name is the artifact name (lowercased, canonical).
func recoveryArtifactKey(nodeID, name string) string {
	return recoveryKeyPrefix + nodeID + "/artifacts/" + strings.ToLower(name)
}

// recoveryArtifactPrefix returns the etcd prefix for all artifact results of a node.
func recoveryArtifactPrefix(nodeID string) string {
	return recoveryKeyPrefix + nodeID + "/artifacts/"
}

// ── NodeRecoveryState ─────────────────────────────────────────────────────────

// getNodeRecoveryState reads the NodeRecoveryState from etcd for a node.
// Returns nil, nil when no recovery state exists.
func (srv *server) getNodeRecoveryState(ctx context.Context, nodeID string) (*cluster_controllerpb.NodeRecoveryState, error) {
	kv := srv.recoveryKV()
	if kv == nil {
		return nil, fmt.Errorf("etcd not available")
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := kv.Get(tctx, recoveryStateKey(nodeID))
	if err != nil {
		return nil, fmt.Errorf("etcd get recovery state for %s: %w", nodeID, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var st cluster_controllerpb.NodeRecoveryState
	if err := json.Unmarshal(resp.Kvs[0].Value, &st); err != nil {
		return nil, fmt.Errorf("decode recovery state for %s: %w", nodeID, err)
	}
	return &st, nil
}

// putNodeRecoveryState writes a NodeRecoveryState to etcd.
func (srv *server) putNodeRecoveryState(ctx context.Context, st *cluster_controllerpb.NodeRecoveryState) error {
	kv := srv.recoveryKV()
	if kv == nil {
		return fmt.Errorf("etcd not available")
	}
	st.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("encode recovery state: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = kv.Put(tctx, recoveryStateKey(st.NodeID), string(data))
	return err
}

// deleteNodeRecoveryState removes a node's recovery state from etcd.
// Only called on explicit cleanup — normally records stay for audit.
func (srv *server) deleteNodeRecoveryState(ctx context.Context, nodeID string) error {
	kv := srv.recoveryKV()
	if kv == nil {
		return fmt.Errorf("etcd not available")
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := kv.Delete(tctx, recoveryStateKey(nodeID))
	return err
}

// ── Fencing query (called by reconcileNodes) ──────────────────────────────────

// isNodeUnderRecovery returns true when the given node has an active
// (non-terminal) recovery workflow with reconciliation paused.
// A return value of true means the reconciler MUST skip that node.
//
// This is the implementation of Invariant 2:
//
//	"A node cannot be under both normal reconciliation and active full reseed
//	at the same time."
func (srv *server) isNodeUnderRecovery(ctx context.Context, nodeID string) bool {
	st, err := srv.getNodeRecoveryState(ctx, nodeID)
	if err != nil || st == nil {
		return false
	}
	if st.Phase.IsTerminal() {
		return false
	}
	return st.ReconciliationPaused
}

// ── NodeRecoverySnapshot ──────────────────────────────────────────────────────

// getNodeRecoverySnapshot reads a snapshot from etcd.
func (srv *server) getNodeRecoverySnapshot(ctx context.Context, nodeID, snapshotID string) (*cluster_controllerpb.NodeRecoverySnapshot, error) {
	kv := srv.recoveryKV()
	if kv == nil {
		return nil, fmt.Errorf("etcd not available")
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := kv.Get(tctx, recoverySnapshotKey(nodeID, snapshotID))
	if err != nil {
		return nil, fmt.Errorf("etcd get snapshot %s/%s: %w", nodeID, snapshotID, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var snap cluster_controllerpb.NodeRecoverySnapshot
	if err := json.Unmarshal(resp.Kvs[0].Value, &snap); err != nil {
		return nil, fmt.Errorf("decode snapshot %s/%s: %w", nodeID, snapshotID, err)
	}
	return &snap, nil
}

// putNodeRecoverySnapshot persists a snapshot to etcd.
func (srv *server) putNodeRecoverySnapshot(ctx context.Context, snap *cluster_controllerpb.NodeRecoverySnapshot) error {
	kv := srv.recoveryKV()
	if kv == nil {
		return fmt.Errorf("etcd not available")
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = kv.Put(tctx, recoverySnapshotKey(snap.NodeID, snap.SnapshotID), string(data))
	return err
}

// listNodeRecoverySnapshots returns all snapshots for a node (newest first).
func (srv *server) listNodeRecoverySnapshots(ctx context.Context, nodeID string) ([]*cluster_controllerpb.NodeRecoverySnapshot, error) {
	kv := srv.recoveryKV()
	if kv == nil {
		return nil, fmt.Errorf("etcd not available")
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := kv.Get(tctx, recoverySnapshotPrefix(nodeID), clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("list snapshots for %s: %w", nodeID, err)
	}
	snaps := make([]*cluster_controllerpb.NodeRecoverySnapshot, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var snap cluster_controllerpb.NodeRecoverySnapshot
		if err := json.Unmarshal(kv.Value, &snap); err != nil {
			continue
		}
		snaps = append(snaps, &snap)
	}
	// Sort newest first by CreatedAt.
	for i := 0; i < len(snaps)-1; i++ {
		for j := i + 1; j < len(snaps); j++ {
			if snaps[j].CreatedAt.After(snaps[i].CreatedAt) {
				snaps[i], snaps[j] = snaps[j], snaps[i]
			}
		}
	}
	return snaps, nil
}

// ── NodeRecoveryArtifactResult ────────────────────────────────────────────────

// getArtifactResult reads one artifact result (nil if not yet started).
func (srv *server) getArtifactResult(ctx context.Context, nodeID, name string) (*cluster_controllerpb.NodeRecoveryArtifactResult, error) {
	kv := srv.recoveryKV()
	if kv == nil {
		return nil, fmt.Errorf("etcd not available")
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := kv.Get(tctx, recoveryArtifactKey(nodeID, name))
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var r cluster_controllerpb.NodeRecoveryArtifactResult
	if err := json.Unmarshal(resp.Kvs[0].Value, &r); err != nil {
		return nil, fmt.Errorf("decode artifact result %s/%s: %w", nodeID, name, err)
	}
	return &r, nil
}

// putArtifactResult writes a per-artifact result.
func (srv *server) putArtifactResult(ctx context.Context, r *cluster_controllerpb.NodeRecoveryArtifactResult) error {
	kv := srv.recoveryKV()
	if kv == nil {
		return fmt.Errorf("etcd not available")
	}
	data, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode artifact result: %w", err)
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = kv.Put(tctx, recoveryArtifactKey(r.NodeID, r.Name), string(data))
	return err
}

// listArtifactResults returns all artifact results for a recovery workflow.
func (srv *server) listArtifactResults(ctx context.Context, nodeID string) ([]cluster_controllerpb.NodeRecoveryArtifactResult, error) {
	kv := srv.recoveryKV()
	if kv == nil {
		return nil, fmt.Errorf("etcd not available")
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := kv.Get(tctx, recoveryArtifactPrefix(nodeID), clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("list artifact results for %s: %w", nodeID, err)
	}
	out := make([]cluster_controllerpb.NodeRecoveryArtifactResult, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var r cluster_controllerpb.NodeRecoveryArtifactResult
		if err := json.Unmarshal(kv.Value, &r); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// deleteArtifactResults removes all artifact results for a node (cleanup).
func (srv *server) deleteArtifactResults(ctx context.Context, nodeID string) error {
	kv := srv.recoveryKV()
	if kv == nil {
		return fmt.Errorf("etcd not available")
	}
	tctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := kv.Delete(tctx, recoveryArtifactPrefix(nodeID), clientv3.WithPrefix())
	return err
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// recoveryKV returns a KV client for recovery operations.
// Uses the dedicated etcd client if available, otherwise falls back to the
// shared config client.
func (srv *server) recoveryKV() clientv3.KV {
	if srv.etcdClient != nil {
		return clientv3.NewKV(srv.etcdClient)
	}
	c, err := config.GetEtcdClient()
	if err == nil && c != nil {
		return clientv3.NewKV(c)
	}
	return nil
}
