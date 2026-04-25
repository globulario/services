package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	configpkg "github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// objectStoreLockLeaseTTL is the etcd lease TTL for the topology lock.
// If the controller process dies while holding the lock, etcd automatically
// deletes it after this many seconds. Must be >> the expected workflow duration
// but short enough that a genuine crash doesn't block the cluster for hours.
const objectStoreLockLeaseTTL = 1800 // 30 minutes

// objectStoreLockStaleDur is the age threshold beyond which a lock written
// WITHOUT a lease (legacy or manually set) is considered stale and force-removed.
const objectStoreLockStaleDur = 30 * time.Minute

// RunObjectStoreTopologyWorkflow triggers the coordinated MinIO topology
// workflow for the given target generation. It wires all controller actor
// callbacks and calls executeWorkflowCentralized.
//
// Call this after publishObjectStoreDesiredStateLocked() when all pool nodes
// are at MinioJoinVerified and the applied_generation in etcd lags the desired
// generation — meaning the pool topology changed but MinIO has not yet been
// restarted in distributed mode.
//
// The workflow is idempotent: if a run for the same correlationID is already
// in progress the workflow service returns it immediately.
func (srv *server) RunObjectStoreTopologyWorkflow(ctx context.Context, targetGeneration int64) (*workflowpb.ExecuteWorkflowResponse, error) {
	// Snapshot pool nodes and their agent endpoints under lock.
	srv.lock("RunObjectStoreTopologyWorkflow:snapshot")
	poolNodeIDs := make([]string, 0)
	poolNodes := make([]any, 0)
	for _, node := range srv.state.Nodes {
		if !nodeHasMinioProfile(node) {
			continue
		}
		if nodeRoutableIP(node) == "" {
			continue
		}
		poolNodeIDs = append(poolNodeIDs, node.NodeID)
		poolNodes = append(poolNodes, map[string]any{
			"node_id":        node.NodeID,
			"agent_endpoint": node.AgentEndpoint,
		})
	}
	poolNodeIDsAny := make([]any, len(poolNodeIDs))
	for i, id := range poolNodeIDs {
		poolNodeIDsAny[i] = id
	}
	clusterID := srv.cfg.ClusterDomain
	srv.unlock()

	// Compute the expected state fingerprint and volumes_hash from the
	// currently-desired objectstore state so all render checks use the same
	// reference value that was published to etcd.
	desiredCtx, desiredCancel := context.WithTimeout(ctx, 10*time.Second)
	defer desiredCancel()
	desired, err := configpkg.LoadObjectStoreDesiredState(desiredCtx)
	if err != nil {
		return nil, fmt.Errorf("objectstore-workflow: load desired state: %w", err)
	}
	if desired == nil {
		return nil, fmt.Errorf("objectstore-workflow: no desired state in etcd (generation=%d)", targetGeneration)
	}
	if desired.Generation != targetGeneration {
		return nil, fmt.Errorf("objectstore-workflow: desired generation changed (%d != %d) — aborting", desired.Generation, targetGeneration)
	}
	expectedFingerprint := configpkg.RenderStateFingerprint(desired)
	expectedVolumesHash := desired.VolumesHash

	router := engine.NewRouter()

	engine.RegisterObjectStoreControllerActions(router, engine.ObjectStoreControllerConfig{
		CheckAllNodesRendered: func(ctx context.Context, gen int64, fingerprint string, nodeIDs []string) error {
			return srv.checkAllNodesRenderedGeneration(ctx, gen, fingerprint, nodeIDs)
		},
		AcquireTopologyLock: func(ctx context.Context) error {
			return srv.acquireObjectStoreTopologyLock(ctx)
		},
		ReleaseTopologyLock: func(ctx context.Context) error {
			return srv.releaseObjectStoreTopologyLock(ctx)
		},
		MarkRestartInProgress: func(ctx context.Context) error {
			return srv.setObjectStoreRestartInProgress(ctx, true)
		},
		ClearRestartInProgress: func(ctx context.Context) error {
			return srv.setObjectStoreRestartInProgress(ctx, false)
		},
		RecordAppliedGeneration: func(ctx context.Context, gen int64) error {
			return srv.recordObjectStoreAppliedGeneration(ctx, gen)
		},
		VerifyMinioClusterHealthy: func(ctx context.Context, gen int64, hash string, nodeIDs []string) error {
			return srv.verifyMinioClusterHealthy(ctx, gen, hash, nodeIDs)
		},
		FailureCleanup: func(ctx context.Context, gen int64, reason string) error {
			return srv.objectStoreFailureCleanup(ctx, gen, reason)
		},
	})

	engine.RegisterNodeDirectApplyActions(router, srv.buildObjectStoreNodeDirectApplyConfig())

	inputs := map[string]any{
		"cluster_id":               clusterID,
		"target_generation":        targetGeneration,
		"expected_state_fingerprint": expectedFingerprint,
		"expected_volumes_hash":    expectedVolumesHash,
		"pool_node_ids":            poolNodeIDsAny,
		"pool_nodes":               poolNodes,
	}

	correlationID := fmt.Sprintf("objectstore.topology:%d", targetGeneration)

	log.Printf("objectstore-workflow: starting topology workflow gen=%d nodes=%v fingerprint=%s",
		targetGeneration, poolNodeIDs, expectedFingerprint[:16])

	start := time.Now()
	resp, err := srv.executeWorkflowCentralized(ctx,
		"objectstore.minio.apply_topology_generation",
		correlationID, inputs, router)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("objectstore-workflow: FAILED after %s: %v", elapsed.Round(time.Millisecond), err)
		return nil, err
	}

	log.Printf("objectstore-workflow: %s in %s", resp.Status, elapsed.Round(time.Millisecond))
	return resp, nil
}

// buildObjectStoreNodeDirectApplyConfig wires only the stop and restart service
// actions needed by the topology workflow.
func (srv *server) buildObjectStoreNodeDirectApplyConfig() engine.NodeDirectApplyConfig {
	return engine.NodeDirectApplyConfig{
		StopPackageService: func(ctx context.Context, name string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}
			agent, err := srv.getAgentClient(ctx, endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			unit := "globular-" + name + ".service"
			if _, err := agent.ControlService(ctx, unit, "stop"); err != nil {
				return fmt.Errorf("stop %s on %s: %w", unit, nodeID, err)
			}
			log.Printf("objectstore-workflow: stopped %s on %s", unit, nodeID)
			return nil
		},
		RestartPackageService: func(ctx context.Context, name string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}
			agent, err := srv.getAgentClient(ctx, endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			unit := "globular-" + name + ".service"
			if _, err := agent.ControlService(ctx, unit, "start"); err != nil {
				return fmt.Errorf("start %s on %s: %w", unit, nodeID, err)
			}
			log.Printf("objectstore-workflow: started %s on %s", unit, nodeID)
			return nil
		},
	}
}

// ── controller action implementations ────────────────────────────────────────

// checkAllNodesRenderedGeneration returns nil when every node in nodeIDs has:
//   - written rendered_generation >= gen
//   - written rendered_state_fingerprint == expectedFingerprint (when non-empty)
//
// Returns a retriable error otherwise.
func (srv *server) checkAllNodesRenderedGeneration(ctx context.Context, gen int64, expectedFingerprint string, nodeIDs []string) error {
	cli, err := configpkg.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}

	var notReady []string
	for _, nodeID := range nodeIDs {
		genKey := configpkg.EtcdKeyNodeRenderedGeneration(nodeID)
		genResp, err := cli.Get(ctx, genKey)
		if err != nil {
			notReady = append(notReady, nodeID+":etcd_err")
			continue
		}
		if len(genResp.Kvs) == 0 {
			notReady = append(notReady, nodeID+":not_written")
			continue
		}
		rendered, err := strconv.ParseInt(string(genResp.Kvs[0].Value), 10, 64)
		if err != nil || rendered < gen {
			notReady = append(notReady, fmt.Sprintf("%s:rendered=%d", nodeID, rendered))
			continue
		}

		// Check fingerprint when the caller provides one.
		if expectedFingerprint != "" {
			fpKey := configpkg.EtcdKeyNodeRenderedStateFingerprint(nodeID)
			fpResp, err := cli.Get(ctx, fpKey)
			if err != nil {
				notReady = append(notReady, nodeID+":fp_etcd_err")
				continue
			}
			if len(fpResp.Kvs) == 0 {
				notReady = append(notReady, nodeID+":fp_not_written")
				continue
			}
			gotFP := string(fpResp.Kvs[0].Value)
			if gotFP != expectedFingerprint {
				notReady = append(notReady, fmt.Sprintf("%s:fp_mismatch(got=%s)", nodeID, gotFP[:8]))
			}
		}
	}

	if len(notReady) > 0 {
		return fmt.Errorf("nodes not yet at generation %d (fingerprint=%s): %v", gen, expectedFingerprint[:min8(expectedFingerprint)], notReady)
	}
	return nil
}

// min8 returns the shorter of len(s) and 8, for safe prefix logging.
func min8(s string) int {
	if len(s) < 8 {
		return len(s)
	}
	return 8
}

// acquireObjectStoreTopologyLock acquires the distributed topology lock using
// an etcd lease. The lease TTL is objectStoreLockLeaseTTL seconds — if the
// controller crashes while holding the lock, etcd deletes it automatically
// after the TTL expires.
//
// Stale lock recovery: if a lock key exists but was written WITHOUT a lease
// (no associated lease ID in etcd, or the stored timestamp is > objectStoreLockStaleDur old),
// the lock is force-deleted and re-acquired.
func (srv *server) acquireObjectStoreTopologyLock(ctx context.Context) error {
	cli, err := configpkg.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}

	lockKey := configpkg.EtcdKeyObjectStoreTopologyLock

	// Check for a stale lock and recover it if necessary.
	if err := srv.maybeRecoverStaleLock(ctx, cli, lockKey); err != nil {
		log.Printf("objectstore-topology: stale lock recovery failed: %v", err)
		// Non-fatal: try to acquire normally; will fail if lock still held.
	}

	// Grant a lease so the lock auto-expires if we crash.
	lease, err := cli.Grant(ctx, objectStoreLockLeaseTTL)
	if err != nil {
		return fmt.Errorf("grant etcd lease: %w", err)
	}

	lockVal := fmt.Sprintf("%s|lease=%d", time.Now().Format(time.RFC3339), lease.ID)

	txnResp, err := cli.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(lockKey), "=", 0)).
		Then(clientv3.OpPut(lockKey, lockVal, clientv3.WithLease(lease.ID))).
		Commit()
	if err != nil {
		// Revoke the lease — we don't need it if we failed to acquire the lock.
		rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = cli.Revoke(rctx, lease.ID)
		cancel()
		return fmt.Errorf("etcd txn: %w", err)
	}
	if !txnResp.Succeeded {
		rctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = cli.Revoke(rctx, lease.ID)
		cancel()
		return fmt.Errorf("topology lock already held (key %s)", lockKey)
	}
	return nil
}

// maybeRecoverStaleLock force-deletes the topology lock key if it appears stale.
// A lock is stale when:
//   - Its value contains a timestamp older than objectStoreLockStaleDur, AND
//   - The key has no live lease (lease ID not found or expired).
func (srv *server) maybeRecoverStaleLock(ctx context.Context, cli *clientv3.Client, lockKey string) error {
	resp, err := cli.Get(ctx, lockKey)
	if err != nil || len(resp.Kvs) == 0 {
		return nil // no lock or read error — nothing to recover
	}

	kv := resp.Kvs[0]

	// If the key has an active lease, it is NOT stale — the holder is alive.
	if kv.Lease != 0 {
		leaseResp, err := cli.TimeToLive(ctx, clientv3.LeaseID(kv.Lease))
		if err == nil && leaseResp.TTL > 0 {
			return nil // lease is alive — lock is legitimately held
		}
		// Lease expired or TTL check failed — proceed to force-delete.
	}

	// No lease or expired lease: check the timestamp embedded in the lock value.
	// Format: "2006-01-02T15:04:05Z07:00|lease=..." or just the RFC3339 string.
	val := string(kv.Value)
	tsStr := val
	if idx := len(val); idx > 25 {
		tsStr = val[:25] // trim to RFC3339 length
	}
	if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
		if time.Since(t) < objectStoreLockStaleDur {
			return nil // lock is recent — don't steal it
		}
	}

	log.Printf("objectstore-topology: recovering stale lock (age check passed, no live lease): %s", val)
	if _, err := cli.Delete(ctx, lockKey); err != nil {
		return fmt.Errorf("delete stale lock: %w", err)
	}
	return nil
}

// releaseObjectStoreTopologyLock deletes the topology lock key.
func (srv *server) releaseObjectStoreTopologyLock(ctx context.Context) error {
	cli, err := configpkg.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}
	_, err = cli.Delete(ctx, configpkg.EtcdKeyObjectStoreTopologyLock)
	return err
}

// setObjectStoreRestartInProgress sets or clears the restart_in_progress flag.
func (srv *server) setObjectStoreRestartInProgress(ctx context.Context, inProgress bool) error {
	cli, err := configpkg.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}
	if !inProgress {
		_, err = cli.Delete(ctx, configpkg.EtcdKeyObjectStoreRestartInProgress)
		return err
	}
	_, err = cli.Put(ctx, configpkg.EtcdKeyObjectStoreRestartInProgress, time.Now().Format(time.RFC3339))
	return err
}

// recordObjectStoreAppliedGeneration writes the applied generation and a JSON
// summary to etcd with status=succeeded.
func (srv *server) recordObjectStoreAppliedGeneration(ctx context.Context, gen int64) error {
	cli, err := configpkg.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}

	summary, _ := json.Marshal(map[string]any{
		"generation": gen,
		"applied_at": time.Now().Format(time.RFC3339),
		"status":     "succeeded",
	})

	if _, err := cli.Put(ctx, configpkg.EtcdKeyObjectStoreAppliedGeneration, strconv.FormatInt(gen, 10)); err != nil {
		return err
	}
	_, err = cli.Put(ctx, configpkg.EtcdKeyObjectStoreLastRestartResult, string(summary))
	return err
}

// verifyMinioClusterHealthy checks that all pool nodes are running MinIO in the
// expected topology. Called by the workflow step before recording applied_generation.
//
// Checks:
//  1. All pool nodes have globular-minio.service active (via node agent ControlService status).
//  2. The MinIO health endpoint responds (TCP probe to cluster endpoint port 9000).
//  3. The desired generation in etcd still equals targetGeneration (guards against
//     a concurrent topology change while restart was in progress).
//  4. The desired volumes_hash still equals expectedVolumesHash (same guard for
//     pool membership changes).
//
// Returns a retriable error so the workflow can retry health checks.
func (srv *server) verifyMinioClusterHealthy(ctx context.Context, targetGeneration int64, expectedVolumesHash string, nodeIDs []string) error {
	// ── 1. Desired state consistency check ───────────────────────────────────
	stateCtx, stateCancel := context.WithTimeout(ctx, 10*time.Second)
	defer stateCancel()
	desired, err := configpkg.LoadObjectStoreDesiredState(stateCtx)
	if err != nil {
		return fmt.Errorf("load desired state: %w", err)
	}
	if desired == nil {
		return fmt.Errorf("desired state missing from etcd")
	}
	if desired.Generation != targetGeneration {
		return fmt.Errorf("desired generation changed mid-workflow: want=%d got=%d", targetGeneration, desired.Generation)
	}
	if expectedVolumesHash != "" && desired.VolumesHash != expectedVolumesHash {
		return fmt.Errorf("desired volumes_hash changed mid-workflow: want=%s got=%s", expectedVolumesHash, desired.VolumesHash)
	}

	// ── 2. Per-node service active check ─────────────────────────────────────
	srv.lock("verifyMinioClusterHealthy:snapshot")
	nodeEndpoints := make(map[string]string, len(srv.state.Nodes))
	for _, n := range srv.state.Nodes {
		nodeEndpoints[n.NodeID] = n.AgentEndpoint
	}
	srv.unlock()

	var unhealthy []string
	for _, nodeID := range nodeIDs {
		ep := nodeEndpoints[nodeID]
		if ep == "" {
			unhealthy = append(unhealthy, nodeID+":no_endpoint")
			continue
		}
		agentCtx, agentCancel := context.WithTimeout(ctx, 10*time.Second)
		agent, err := srv.getAgentClient(agentCtx, ep)
		agentCancel()
		if err != nil {
			unhealthy = append(unhealthy, nodeID+":dial_err")
			continue
		}
		statusCtx, statusCancel := context.WithTimeout(ctx, 10*time.Second)
		statusResp, err := agent.ControlService(statusCtx, "globular-minio.service", "status")
		statusCancel()
		if err != nil {
			unhealthy = append(unhealthy, fmt.Sprintf("%s:status_err(%v)", nodeID, err))
			continue
		}
		if statusResp.GetState() != "active" {
			unhealthy = append(unhealthy, fmt.Sprintf("%s:not_active(%s)", nodeID, statusResp.GetState()))
		}
	}
	if len(unhealthy) > 0 {
		return fmt.Errorf("minio service not active on nodes: %v", unhealthy)
	}

	// ── 3. MinIO health endpoint TCP probe ────────────────────────────────────
	if desired.Endpoint != "" {
		host, port, err := net.SplitHostPort(desired.Endpoint)
		if err != nil {
			// Endpoint has no port — assume 9000.
			host = desired.Endpoint
			port = "9000"
		}
		healthURL := fmt.Sprintf("http://%s:%s/minio/health/live", host, port)
		httpCtx, httpCancel := context.WithTimeout(ctx, 15*time.Second)
		defer httpCancel()
		req, _ := http.NewRequestWithContext(httpCtx, http.MethodGet, healthURL, nil)
		httpClient := &http.Client{Timeout: 15 * time.Second}
		resp, err := httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("minio health endpoint %s unreachable: %w", healthURL, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("minio health endpoint returned %d (want 200)", resp.StatusCode)
		}
	}

	return nil
}

// objectStoreFailureCleanup is the onFailure handler. It:
//  1. Releases the topology lock (idempotent delete).
//  2. Clears restart_in_progress.
//  3. Writes a last_restart_result record with status=failed.
//
// All three steps are attempted; partial failures are logged but not fatal
// so that all cleanup actions run even when one fails.
func (srv *server) objectStoreFailureCleanup(ctx context.Context, generation int64, reason string) error {
	cli, err := configpkg.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}

	var errs []string

	// Release lock.
	if _, err := cli.Delete(ctx, configpkg.EtcdKeyObjectStoreTopologyLock); err != nil {
		errs = append(errs, fmt.Sprintf("release lock: %v", err))
	}

	// Clear restart_in_progress.
	if _, err := cli.Delete(ctx, configpkg.EtcdKeyObjectStoreRestartInProgress); err != nil {
		errs = append(errs, fmt.Sprintf("clear restart_in_progress: %v", err))
	}

	// Write failure result.
	result, _ := json.Marshal(map[string]any{
		"status":      "failed",
		"generation":  generation,
		"reason":      reason,
		"failed_at":   time.Now().Format(time.RFC3339),
	})
	if _, err := cli.Put(ctx, configpkg.EtcdKeyObjectStoreLastRestartResult, string(result)); err != nil {
		errs = append(errs, fmt.Sprintf("write last_restart_result: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("failure cleanup partial errors: %v", errs)
	}
	return nil
}

// maybeRunObjectStoreTopologyWorkflow checks whether the objectstore topology
// workflow needs to run and launches it asynchronously if so. Called from
// reconcileMinioJoinPhases (minio_pools.go) after all pool nodes reach
// MinioJoinVerified.
//
// Conditions to run:
//  1. All MinIO pool nodes are at MinioJoinVerified.
//  2. The current ObjectStoreGeneration > applied_generation in etcd.
//  3. No restart_in_progress flag is set (guards against duplicate launches).
func (srv *server) maybeRunObjectStoreTopologyWorkflow(ctx context.Context) {
	srv.lock("maybeRunObjectStoreTopologyWorkflow:snapshot")

	allVerified := true
	for _, node := range srv.state.Nodes {
		if !nodeHasMinioProfile(node) {
			continue
		}
		if node.MinioJoinPhase != MinioJoinVerified {
			allVerified = false
			break
		}
	}
	targetGen := srv.state.ObjectStoreGeneration
	srv.unlock()

	if !allVerified || targetGen == 0 {
		return
	}

	cli, err := configpkg.GetEtcdClient()
	if err != nil {
		return
	}

	// Skip if generation is already applied.
	if resp, err := cli.Get(ctx, configpkg.EtcdKeyObjectStoreAppliedGeneration); err == nil && len(resp.Kvs) > 0 {
		if applied, err := strconv.ParseInt(string(resp.Kvs[0].Value), 10, 64); err == nil && applied >= targetGen {
			return
		}
	}

	// Skip if a restart is already in progress.
	if resp, err := cli.Get(ctx, configpkg.EtcdKeyObjectStoreRestartInProgress); err == nil && len(resp.Kvs) > 0 {
		log.Printf("objectstore-topology: restart already in progress — skipping trigger")
		return
	}

	log.Printf("objectstore-topology: triggering topology workflow gen=%d", targetGen)
	capturedGen := targetGen
	go func() {
		wctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		if _, err := srv.RunObjectStoreTopologyWorkflow(wctx, capturedGen); err != nil {
			log.Printf("objectstore-topology: workflow failed: %v", err)
		}
	}()
}
