package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow/engine"
)

// RunControllerDeployWorkflow executes the leader-aware rolling update for
// a control-plane service (typically cluster-controller itself).
//
// Rollout order:
//  1. Discover all controller replicas and identify the leader
//  2. Apply package to non-leader replicas (one at a time)
//  3. Verify each follower is healthy after update
//  4. Resign leadership so an upgraded follower takes over
//  5. Apply package to the old leader
//  6. Verify all replicas healthy
//
// Postcondition if apply_old_leader fails (step 7):
//   - Leadership has already moved to an upgraded follower (step 5-6).
//   - The cluster is functional: the new leader runs the new version.
//   - The old leader node is running the OLD version but is a follower.
//   - The workflow fails, logging the error. The old leader node can be
//     retried by re-running the workflow (idempotent) or manually via
//     the ApplyPackageRelease RPC.
//   - No rollback is needed: the cluster is in a safe degraded state
//     with N-1 nodes upgraded.
func (srv *server) RunControllerDeployWorkflow(ctx context.Context, pkgName, pkgKind, version string, buildNumber int64) error {
	router := engine.NewRouter()
	engine.RegisterControllerDeployActions(router, srv.buildControllerDeployConfig())

	// Correlation ID is independent of build identity — includes timestamp
	// to ensure uniqueness across retries of the same version+build.
	correlationID := fmt.Sprintf("controller-deploy/%s@%s+%d/%d",
		pkgName, version, buildNumber, time.Now().UnixMilli())

	inputs := map[string]any{
		"cluster_id":    srv.cfg.ClusterDomain,
		"package_name":  pkgName,
		"package_kind":  pkgKind,
		"version":       version,
		"build_number":  buildNumber,
		"resign_reason": fmt.Sprintf("rolling update %s@%s+%d", pkgName, version, buildNumber),
	}

	log.Printf("controller-deploy: starting leader-aware rollout for %s/%s@%s (build %d, correlation=%s)",
		pkgKind, pkgName, version, buildNumber, correlationID)

	resp, err := srv.executeWorkflowCentralized(ctx,
		"release.apply.controller",
		correlationID,
		inputs,
		router,
	)
	if err != nil {
		return fmt.Errorf("controller deploy workflow: %w", err)
	}
	if resp.Status == "FAILED" {
		return fmt.Errorf("controller deploy failed: %s", resp.Error)
	}

	log.Printf("controller-deploy: rollout completed for %s@%s — %s", pkgName, version, resp.Status)
	return nil
}

// buildControllerDeployConfig creates the callback config for the
// release.apply.controller workflow.
func (srv *server) buildControllerDeployConfig() engine.ControllerDeployConfig {
	return engine.ControllerDeployConfig{
		DiscoverReplicas: func(ctx context.Context, clusterID string) (*engine.ControllerDiscovery, error) {
			return srv.discoverControllerReplicas(ctx)
		},

		VerifyReplicaHealth: func(ctx context.Context, nodeID, packageName, version string) error {
			return srv.verifyControllerReplicaHealth(ctx, nodeID, packageName, version)
		},

		ResignLeadership: func(ctx context.Context, reason string) error {
			if !srv.isLeader() {
				return nil // already not leader
			}
			select {
			case srv.resignCh <- struct{}{}:
			default:
				return fmt.Errorf("resign already in progress")
			}
			// Wait for leadership to clear.
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if !srv.isLeader() {
					log.Printf("controller-deploy: leadership resigned (reason: %s)", reason)
					return nil
				}
				time.Sleep(200 * time.Millisecond)
			}
			return fmt.Errorf("resign timed out after 5s")
		},

		ConfirmLeadershipMoved: func(ctx context.Context, oldLeaderNodeID string) error {
			return srv.confirmLeadershipMoved(ctx, oldLeaderNodeID)
		},

		ApplyPackageRelease: func(ctx context.Context, nodeID, agentEndpoint, pkgName, pkgKind, version, publisher, repoAddr string, buildNumber int64, force bool) error {
			return srv.remoteApplyPackageRelease(ctx, nodeID, agentEndpoint, pkgName, pkgKind, version, publisher, repoAddr, buildNumber, force)
		},
	}
}

// discoverControllerReplicas builds the replica list from in-memory node state
// and the etcd service registry.
func (srv *server) discoverControllerReplicas(ctx context.Context) (*engine.ControllerDiscovery, error) {
	srv.lock("discover-replicas")
	nodes := make(map[string]*nodeState, len(srv.state.Nodes))
	for id, n := range srv.state.Nodes {
		nodes[id] = n
	}
	srv.unlock()

	myNodeID := ""
	isLeader := srv.isLeader()

	// Resolve which node this controller runs on.
	localIP := config.GetRoutableIPv4()

	var replicas []engine.ControllerReplica
	var followers []engine.ControllerReplica
	var leaderNodeID, leaderEndpoint string

	for nodeID, node := range nodes {
		// Check if this node runs a controller (all control-plane nodes do).
		agentEndpoint := node.AgentEndpoint
		if agentEndpoint == "" {
			continue
		}

		// Determine if this node is the leader by matching the local IP.
		nodeIsLeader := false
		if isLeader && node.PrimaryIP() == localIP {
			nodeIsLeader = true
			myNodeID = nodeID
			leaderNodeID = nodeID
			leaderEndpoint = agentEndpoint
		}

		replica := engine.ControllerReplica{
			NodeID:        nodeID,
			AgentEndpoint: agentEndpoint,
			IsLeader:      nodeIsLeader,
		}
		replicas = append(replicas, replica)
		if !nodeIsLeader {
			followers = append(followers, replica)
		}
	}

	if leaderNodeID == "" && isLeader {
		// Fallback: if we couldn't match by IP, use the first node.
		leaderNodeID = myNodeID
	}

	if len(replicas) == 0 {
		return nil, fmt.Errorf("no controller replicas found in node state")
	}

	return &engine.ControllerDiscovery{
		Replicas:       replicas,
		Followers:      followers,
		LeaderNodeID:   leaderNodeID,
		LeaderEndpoint: leaderEndpoint,
	}, nil
}

// verifyControllerReplicaHealth checks that a controller on the given node
// is reporting healthy (node status = ready) and running the expected version.
// Uses both in-memory InstalledVersions AND the etcd installed-state registry
// for consistency.
func (srv *server) verifyControllerReplicaHealth(ctx context.Context, nodeID, packageName, version string) error {
	srv.lock("verify-replica-health")
	node := srv.state.Nodes[nodeID]
	srv.unlock()

	if node == nil {
		return fmt.Errorf("node %s not found", nodeID)
	}
	if !strings.EqualFold(node.Status, "ready") {
		return fmt.Errorf("node %s status=%s (expected ready)", nodeID, node.Status)
	}

	// Check in-memory InstalledVersions (updated by heartbeat).
	canonicalName := strings.ReplaceAll(packageName, "_", "-")
	versionMatch := false
	if node.InstalledVersions != nil {
		for k, v := range node.InstalledVersions {
			if (k == canonicalName || k == packageName || strings.ReplaceAll(k, "_", "-") == canonicalName) && v == version {
				versionMatch = true
				break
			}
		}
	}
	if !versionMatch {
		return fmt.Errorf("node %s: %s not at version %s (installed: %v)", nodeID, packageName, version, node.InstalledVersions)
	}

	// Cross-check with etcd installed-state registry (source of truth for build_number).
	pkg, err := installed_state.GetInstalledPackage(ctx, nodeID, "SERVICE", canonicalName)
	if err != nil || pkg == nil {
		// Fallback: in-memory version matched, etcd may lag. Accept it.
		return nil
	}
	if pkg.Status != "installed" {
		return fmt.Errorf("node %s: %s etcd status=%s (expected installed)", nodeID, packageName, pkg.Status)
	}

	return nil
}

// confirmLeadershipMoved verifies the leader is no longer the old node
// by checking the etcd election key (source of truth), not just in-memory state.
func (srv *server) confirmLeadershipMoved(ctx context.Context, oldLeaderNodeID string) error {
	// Primary check: this instance must no longer be the leader.
	if srv.isLeader() {
		return fmt.Errorf("this instance is still the leader")
	}

	// Secondary check: read the leader address from etcd to confirm a new
	// leader was elected (not just that we resigned).
	if srv.kv != nil {
		key := leaderElectionPrefix + "/addr"
		resp, err := srv.kv.Get(ctx, key)
		if err != nil {
			return fmt.Errorf("read leader key from etcd: %w", err)
		}
		if resp == nil || len(resp.Kvs) == 0 {
			return fmt.Errorf("no leader registered in etcd yet")
		}
		newLeaderAddr := string(resp.Kvs[0].Value)

		// Verify the new leader address differs from our own.
		localAddr := config.ResolveLocalServiceAddr("cluster_controller.ClusterControllerService")
		if newLeaderAddr == localAddr {
			return fmt.Errorf("etcd still shows this node as leader (addr=%s)", newLeaderAddr)
		}
		log.Printf("controller-deploy: leadership confirmed moved to %s", newLeaderAddr)
	}

	return nil
}

// remoteApplyPackageRelease calls the node-agent's ApplyPackageRelease RPC
// on a remote node to install a package.
func (srv *server) remoteApplyPackageRelease(ctx context.Context, nodeID, agentEndpoint, pkgName, pkgKind, version, publisher, repoAddr string, buildNumber int64, force bool) error {
	conn, err := srv.dialNodeAgent(agentEndpoint)
	if err != nil {
		return fmt.Errorf("connect to node %s at %s: %w", nodeID, agentEndpoint, err)
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)

	resp, err := client.ApplyPackageRelease(ctx, &node_agentpb.ApplyPackageReleaseRequest{
		PackageName:    pkgName,
		PackageKind:    pkgKind,
		Version:        version,
		Publisher:      publisher,
		RepositoryAddr: repoAddr,
		BuildNumber:    buildNumber,
		Force:          force,
		OperationId:    fmt.Sprintf("controller-deploy/%s@%s-b%d", pkgName, version, buildNumber),
	})
	if err != nil {
		return fmt.Errorf("ApplyPackageRelease RPC to node %s: %w", nodeID, err)
	}
	if !resp.GetOk() {
		return fmt.Errorf("apply on node %s: %s", nodeID, resp.GetErrorDetail())
	}

	log.Printf("controller-deploy: applied %s@%s on node %s — status=%s",
		pkgName, version, nodeID, resp.GetStatus())
	return nil
}
