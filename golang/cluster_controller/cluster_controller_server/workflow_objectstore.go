package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	configpkg "github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

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

	router := engine.NewRouter()

	engine.RegisterObjectStoreControllerActions(router, engine.ObjectStoreControllerConfig{
		CheckAllNodesRendered: func(ctx context.Context, gen int64, nodeIDs []string) error {
			return srv.checkAllNodesRenderedGeneration(ctx, gen, nodeIDs)
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
	})

	engine.RegisterNodeDirectApplyActions(router, srv.buildObjectStoreNodeDirectApplyConfig())

	inputs := map[string]any{
		"cluster_id":        clusterID,
		"target_generation": targetGeneration,
		"pool_node_ids":     poolNodeIDsAny,
		"pool_nodes":        poolNodes,
	}

	correlationID := fmt.Sprintf("objectstore.topology:%d", targetGeneration)

	log.Printf("objectstore-workflow: starting topology workflow gen=%d nodes=%v",
		targetGeneration, poolNodeIDs)

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

// checkAllNodesRenderedGeneration returns nil when every node in nodeIDs has
// written rendered_generation >= gen to etcd. Returns a retriable error otherwise.
func (srv *server) checkAllNodesRenderedGeneration(ctx context.Context, gen int64, nodeIDs []string) error {
	cli, err := configpkg.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}

	var notReady []string
	for _, nodeID := range nodeIDs {
		key := configpkg.EtcdKeyNodeRenderedGeneration(nodeID)
		resp, err := cli.Get(ctx, key)
		if err != nil {
			notReady = append(notReady, nodeID+":etcd_err")
			continue
		}
		if len(resp.Kvs) == 0 {
			notReady = append(notReady, nodeID+":not_written")
			continue
		}
		rendered, err := strconv.ParseInt(string(resp.Kvs[0].Value), 10, 64)
		if err != nil || rendered < gen {
			notReady = append(notReady, fmt.Sprintf("%s:rendered=%d", nodeID, rendered))
		}
	}

	if len(notReady) > 0 {
		return fmt.Errorf("nodes not yet at generation %d: %v", gen, notReady)
	}
	return nil
}

// acquireObjectStoreTopologyLock acquires the distributed topology lock via an
// etcd compare-and-set transaction. Returns an error if the lock is already held.
func (srv *server) acquireObjectStoreTopologyLock(ctx context.Context) error {
	cli, err := configpkg.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}

	lockKey := configpkg.EtcdKeyObjectStoreTopologyLock
	lockVal := time.Now().Format(time.RFC3339)

	txnResp, err := cli.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(lockKey), "=", 0)).
		Then(clientv3.OpPut(lockKey, lockVal)).
		Commit()
	if err != nil {
		return fmt.Errorf("etcd txn: %w", err)
	}
	if !txnResp.Succeeded {
		return fmt.Errorf("topology lock already held (key %s)", lockKey)
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
// summary to etcd.
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
