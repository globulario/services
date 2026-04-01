package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/workflow"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (srv *server) ListNodes(ctx context.Context, req *cluster_controllerpb.ListNodesRequest) (*cluster_controllerpb.ListNodesResponse, error) {
	srv.lock("unknown")
	defer srv.unlock()
	resp := &cluster_controllerpb.ListNodesResponse{}
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, node := range srv.state.Nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].NodeID < nodes[j].NodeID
	})
	for _, node := range nodes {
		meta := copyLabels(node.Metadata)
		if meta == nil {
			meta = make(map[string]string)
		}
		if node.LastError != "" {
			meta["last_error"] = node.LastError
		}
		if node.BootstrapPhase != "" {
			meta["bootstrap_phase"] = string(node.BootstrapPhase)
		}
		if node.BootstrapError != "" {
			meta["bootstrap_error"] = node.BootstrapError
		}
		if node.EtcdJoinPhase != "" {
			meta["etcd_join_phase"] = string(node.EtcdJoinPhase)
		}
		if node.ScyllaJoinPhase != "" {
			meta["scylla_join_phase"] = string(node.ScyllaJoinPhase)
		}
		if node.MinioJoinPhase != "" {
			meta["minio_join_phase"] = string(node.MinioJoinPhase)
		}
		if node.Day1Phase != "" {
			meta["day1_phase"] = string(node.Day1Phase)
		}
		if node.Day1PhaseReason != "" {
			meta["day1_phase_reason"] = node.Day1PhaseReason
		}
		if node.ResolvedIntent != nil {
			meta["desired_infra"] = strings.Join(node.ResolvedIntent.DesiredInfraNames, ",")
			meta["desired_workloads"] = strings.Join(node.ResolvedIntent.DesiredWorkloadNames, ",")
			if len(node.ResolvedIntent.BlockedWorkloads) > 0 {
				bw := make([]string, len(node.ResolvedIntent.BlockedWorkloads))
				for i, b := range node.ResolvedIntent.BlockedWorkloads {
					bw[i] = b.Name + ":" + b.Reason
				}
				meta["blocked_workloads"] = strings.Join(bw, "; ")
			}
			if len(node.ResolvedIntent.MaterializedDesired) > 0 {
				md := make([]string, len(node.ResolvedIntent.MaterializedDesired))
				for i, m := range node.ResolvedIntent.MaterializedDesired {
					md[i] = m.Component + "@" + m.Version
				}
				meta["materialized_infra_desired"] = strings.Join(md, ",")
			}
		}
		resp.Nodes = append(resp.Nodes, &cluster_controllerpb.NodeRecord{
			NodeId:        node.NodeID,
			Identity:      storedIdentityToProto(node.Identity),
			LastSeen:      timestamppb.New(node.LastSeen),
			Status:        node.Status,
			Profiles:      append([]string(nil), node.Profiles...),
			Metadata:      meta,
			AgentEndpoint: node.AgentEndpoint,
			Capabilities:  storedToProtoCapabilities(node.Capabilities),
		})
	}
	return resp, nil
}

func (srv *server) SetNodeProfiles(ctx context.Context, req *cluster_controllerpb.SetNodeProfilesRequest) (*cluster_controllerpb.SetNodeProfilesResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil || req.GetNodeId() == "" || len(req.GetProfiles()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "--profile is required")
	}
	normalized := normalizeProfiles(req.GetProfiles())
	srv.lock("unknown")
	defer srv.unlock()
	node := srv.state.Nodes[req.GetNodeId()]
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	node.Profiles = normalized
	node.LastSeen = time.Now()
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist node profiles: %v", err)
	}
	if srv.enqueueReconcile != nil {
		srv.enqueueReconcile()
	}
	return &cluster_controllerpb.SetNodeProfilesResponse{
		OperationId: uuid.NewString(),
	}, nil
}

func (srv *server) PreviewNodeProfiles(ctx context.Context, req *cluster_controllerpb.PreviewNodeProfilesRequest) (*cluster_controllerpb.PreviewNodeProfilesResponse, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())
	normalized := normalizeProfiles(req.GetProfiles())

	// Snapshot entire cluster state (read-only, no mutation).
	srv.lock("preview-profiles:snapshot")
	realNode := srv.state.Nodes[nodeID]
	if realNode == nil {
		srv.unlock()
		return nil, status.Errorf(codes.NotFound, "node %q not found", nodeID)
	}
	// Shallow-copy all nodes so we can build a hypothetical membership.
	previewNode := *realNode
	otherNodes := make([]*nodeState, 0, len(srv.state.Nodes)-1)
	for id, n := range srv.state.Nodes {
		if id == nodeID || n == nil {
			continue
		}
		cp := *n
		otherNodes = append(otherNodes, &cp)
	}
	clusterID := srv.state.ClusterId
	srv.unlock()
	previewNode.Profiles = normalized

	// Compute unit actions for the proposed profiles.
	actions, err := buildPlanActions(normalized)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid profiles: %v", err)
	}

	// Build hypothetical membership with the target node's new profiles.
	hypoMembership := &clusterMembership{
		ClusterID: clusterID,
		Nodes:     make([]memberNode, 0, len(otherNodes)+1),
	}
	var ip string
	if len(previewNode.Identity.Ips) > 0 {
		ip = previewNode.Identity.Ips[0]
	}
	hypoMembership.Nodes = append(hypoMembership.Nodes, memberNode{
		NodeID:   previewNode.NodeID,
		Hostname: previewNode.Identity.Hostname,
		IP:       ip,
		Profiles: normalized,
	})
	for _, n := range otherNodes {
		var nip string
		if len(n.Identity.Ips) > 0 {
			nip = n.Identity.Ips[0]
		}
		hypoMembership.Nodes = append(hypoMembership.Nodes, memberNode{
			NodeID:   n.NodeID,
			Hostname: n.Identity.Hostname,
			IP:       nip,
			Profiles: append([]string(nil), n.Profiles...),
		})
	}
	sort.Slice(hypoMembership.Nodes, func(i, j int) bool {
		return hypoMembership.Nodes[i].NodeID < hypoMembership.Nodes[j].NodeID
	})

	// Render target node's configs using hypothetical membership.
	rendered := srv.renderServiceConfigsForNodeInMembership(&previewNode, hypoMembership)

	// Compute config diffs for target node.
	newHashes := HashRenderedConfigs(rendered)
	oldHashes := realNode.RenderedConfigHashes
	configDiff := buildConfigDiff(oldHashes, newHashes)

	// Compute restart units for target node.
	restartActions := restartActionsForChangedConfigs(oldHashes, rendered)
	restartUnits := make([]string, 0, len(restartActions))
	for _, a := range restartActions {
		restartUnits = append(restartUnits, a.GetUnitName())
	}

	// Compute affected other nodes: re-render with hypothetical membership and compare.
	var affectedNodes []*cluster_controllerpb.AffectedNodeDiff
	for _, n := range otherNodes {
		hypoRendered := srv.renderServiceConfigsForNodeInMembership(n, hypoMembership)
		hypoNewHashes := HashRenderedConfigs(hypoRendered)
		diff := buildConfigDiff(n.RenderedConfigHashes, hypoNewHashes)
		// Only include nodes that actually have config changes.
		hasChange := false
		for _, d := range diff {
			if d.GetChanged() {
				hasChange = true
				break
			}
		}
		if hasChange {
			affectedNodes = append(affectedNodes, &cluster_controllerpb.AffectedNodeDiff{
				NodeId:     n.NodeID,
				ConfigDiff: diff,
			})
		}
	}
	// Sort for deterministic output.
	sort.Slice(affectedNodes, func(i, j int) bool {
		return affectedNodes[i].GetNodeId() < affectedNodes[j].GetNodeId()
	})

	return &cluster_controllerpb.PreviewNodeProfilesResponse{
		NormalizedProfiles: normalized,
		UnitDiff:           actions,
		ConfigDiff:         configDiff,
		RestartUnits:       restartUnits,
		AffectedNodes:      affectedNodes,
	}, nil
}

func buildConfigDiff(oldHashes, newHashes map[string]string) []*cluster_controllerpb.ConfigFileDiff {
	pathSet := make(map[string]struct{})
	for p := range newHashes {
		pathSet[p] = struct{}{}
	}
	for p := range oldHashes {
		pathSet[p] = struct{}{}
	}
	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	diff := make([]*cluster_controllerpb.ConfigFileDiff, 0, len(paths))
	for _, p := range paths {
		newH := newHashes[p]
		oldH := oldHashes[p]
		diff = append(diff, &cluster_controllerpb.ConfigFileDiff{
			Path:    p,
			OldHash: oldH,
			NewHash: newH,
			Changed: newH != oldH,
		})
	}
	return diff
}

func (srv *server) RemoveNode(ctx context.Context, req *cluster_controllerpb.RemoveNodeRequest) (*cluster_controllerpb.RemoveNodeResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())

	srv.lock("remove-node")
	node := srv.state.Nodes[nodeID]
	if node == nil {
		srv.unlock()
		return nil, status.Error(codes.NotFound, "node not found")
	}

	agentEndpoint := node.AgentEndpoint
	hostname := node.Identity.Hostname
	srv.unlock()

	opID := uuid.NewString()
	var drainErr error

	// If drain requested and node has an agent endpoint, try to stop services gracefully
	if req.GetDrain() && agentEndpoint != "" {
		drainErr = srv.drainNode(ctx, node, opID)
		if drainErr != nil && !req.GetForce() {
			return nil, status.Errorf(codes.FailedPrecondition, "drain failed (use force=true to override): %v", drainErr)
		}
	}

	// Remove from state
	srv.lock("remove-node")
	delete(srv.state.Nodes, nodeID)
	persistErr := srv.persistStateLocked(true)
	srv.unlock()
	if persistErr != nil {
		return nil, status.Errorf(codes.Internal, "persist node removal: %v", persistErr)
	}

	// ── REPAIR workflow run ─────────────────────────────────────────────
	// Clean ALL etcd state for this node inside a visible REPAIR run.
	// Every state-changing action must belong to exactly one workflow run.
	repairRunID := srv.workflowRec.StartRun(ctx, &workflow.RunParams{
		NodeID:        nodeID,
		NodeHostname:  hostname,
		ReleaseKind:   "NodeRemoval",
		TriggerReason: workflow.TriggerRepair,
		CorrelationID: fmt.Sprintf("repair/node-removal/%s", nodeID),
	})

	prefixes := []struct {
		key     string
		stepKey string
		title   string
	}{
		{fmt.Sprintf("/globular/nodes/%s/", nodeID), "repair_package_state_deleted", "Delete installed package state"},
		{fmt.Sprintf("/globular/ingress/v1/status/%s", nodeID), "repair_ingress_status_deleted", "Delete ingress status"},
		{fmt.Sprintf("globular/plans/v1/nodes/%s/", nodeID), "repair_plan_keys_deleted", "Delete plan history and current plan"},
		{fmt.Sprintf("globular/cluster/v1/observed_hash_services/%s", nodeID), "repair_observed_hash_deleted", "Delete observed service hash"},
		{fmt.Sprintf("globular/cluster/v1/applied_hash_services/%s", nodeID), "repair_applied_hash_deleted", "Delete applied service hash"},
		{fmt.Sprintf("globular/cluster/v1/fail_count_services/%s", nodeID), "repair_fail_count_deleted", "Delete fail count"},
	}

	var totalDeleted int64
	for _, p := range prefixes {
		stepSeq := srv.workflowRec.RecordStep(ctx, repairRunID, &workflow.StepParams{
			StepKey: p.stepKey,
			Title:   p.title,
			Actor:   workflow.ActorController,
			Phase:   workflow.PhasePublish,
			Status:  workflow.StepRunning,
			Message: fmt.Sprintf("prefix=%s", p.key),
		})
		deleted := srv.cleanNodeEtcdPrefix(ctx, nodeID, p.key)
		totalDeleted += deleted
		srv.workflowRec.CompleteStep(ctx, repairRunID, stepSeq,
			fmt.Sprintf("deleted %d keys under %s", deleted, p.key), 0)
	}

	// Purge node from release status objects so the controller doesn't
	// think packages are "already installed" on a re-added node.
	releasesCleaned := srv.cleanNodeFromReleases(ctx, nodeID)
	if releasesCleaned > 0 {
		stepSeq := srv.workflowRec.RecordStep(ctx, repairRunID, &workflow.StepParams{
			StepKey: "repair_release_status_purged",
			Title:   "Purge node from release status objects",
			Actor:   workflow.ActorController,
			Phase:   workflow.PhasePublish,
			Status:  workflow.StepRunning,
			Message: fmt.Sprintf("node_id=%s", nodeID),
		})
		srv.workflowRec.CompleteStep(ctx, repairRunID, stepSeq,
			fmt.Sprintf("purged node from %d release objects", releasesCleaned), 0)
	}

	srv.workflowRec.FinishRun(ctx, repairRunID, workflow.Succeeded,
		fmt.Sprintf("node %s removed: %d etcd keys cleaned, %d releases purged", nodeID, totalDeleted, releasesCleaned),
		"", workflow.NoFailure)

	// Close agent client if we have one
	if agentEndpoint != "" {
		srv.closeAgentClient(agentEndpoint)
	}

	message := fmt.Sprintf("node %s removed from cluster", nodeID)
	if drainErr != nil {
		message = fmt.Sprintf("node %s removed (drain failed: %v)", nodeID, drainErr)
	}

	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_SUCCEEDED, message, 100, true, ""))

	return &cluster_controllerpb.RemoveNodeResponse{
		OperationId: opID,
		Message:     message,
	}, nil
}

// cleanNodeEtcdPrefix deletes all keys under a given prefix and returns
// the number of keys deleted. Returns 0 on error or no-op.
func (srv *server) cleanNodeEtcdPrefix(ctx context.Context, nodeID, prefix string) int64 {
	if srv.etcdClient == nil {
		return 0
	}
	delCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := srv.etcdClient.Delete(delCtx, prefix, clientv3.WithPrefix())
	if err != nil {
		log.Printf("remove-node: failed to clean etcd prefix %s: %v", prefix, err)
		return 0
	}
	if resp.Deleted > 0 {
		log.Printf("remove-node: cleaned %d etcd keys under %s", resp.Deleted, prefix)
	}
	return resp.Deleted
}

// cleanNodeFromReleases removes a node's status entries from all
// ServiceRelease and InfrastructureRelease objects in etcd. Without this,
// a re-added node is seen as "already installed" for packages that were
// never actually installed on the new disk.
func (srv *server) cleanNodeFromReleases(ctx context.Context, nodeID string) int {
	if srv.etcdClient == nil {
		return 0
	}
	getCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Scan all release objects under /globular/resources/
	resp, err := srv.etcdClient.Get(getCtx, "/globular/resources/", clientv3.WithPrefix())
	if err != nil {
		log.Printf("remove-node: failed to list releases: %v", err)
		return 0
	}

	cleaned := 0
	for _, kv := range resp.Kvs {
		// Quick check: skip keys that don't mention this node.
		if !strings.Contains(string(kv.Value), nodeID) {
			continue
		}

		var release map[string]interface{}
		if err := json.Unmarshal(kv.Value, &release); err != nil {
			continue
		}

		statusObj, ok := release["status"].(map[string]interface{})
		if !ok {
			continue
		}
		nodes, ok := statusObj["nodes"].([]interface{})
		if !ok {
			continue
		}

		// Filter out the removed node.
		filtered := make([]interface{}, 0, len(nodes))
		for _, n := range nodes {
			nMap, ok := n.(map[string]interface{})
			if !ok {
				filtered = append(filtered, n)
				continue
			}
			if nMap["node_id"] == nodeID {
				continue // drop this node's entry
			}
			filtered = append(filtered, n)
		}

		if len(filtered) == len(nodes) {
			continue // nothing changed
		}

		// Reset phase to PENDING so controller re-evaluates.
		statusObj["nodes"] = filtered
		statusObj["phase"] = "PENDING"
		release["status"] = statusObj

		newVal, err := json.Marshal(release)
		if err != nil {
			continue
		}

		putCtx, putCancel := context.WithTimeout(ctx, 5*time.Second)
		if _, err := srv.etcdClient.Put(putCtx, string(kv.Key), string(newVal)); err != nil {
			log.Printf("remove-node: failed to update release %s: %v", string(kv.Key), err)
		} else {
			cleaned++
			log.Printf("remove-node: purged node %s from release %s", nodeID, string(kv.Key))
		}
		putCancel()
	}

	return cleaned
}

func (srv *server) drainNode(ctx context.Context, node *nodeState, opID string) error {
	if node.AgentEndpoint == "" {
		return fmt.Errorf("node %s has no agent endpoint", node.NodeID)
	}

	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "draining node services", 10, false, ""))

	// Build a plan with stop actions for all services
	plan := &NodeUnitPlan{
		NodeId:   node.NodeID,
		Profiles: node.Profiles,
	}

	// Add stop actions for known service units based on profiles
	unitStops := []string{}
	for _, profile := range node.Profiles {
		switch profile {
		case "core":
			unitStops = append(unitStops, "globular-etcd.service", "globular-minio.service", "globular-xds.service", "globular-dns.service")
		case "compute":
			unitStops = append(unitStops, "globular-etcd.service", "globular-minio.service", "globular-xds.service")
		case "control-plane":
			unitStops = append(unitStops, "globular-etcd.service", "globular-xds.service")
		case "storage":
			unitStops = append(unitStops, "globular-minio.service")
		case "dns":
			unitStops = append(unitStops, "globular-dns.service")
		case "gateway":
			unitStops = append(unitStops, "globular-xds.service")
		}
	}

	// Dedupe and add stop actions
	seen := make(map[string]bool)
	for _, unit := range unitStops {
		if !seen[unit] {
			seen[unit] = true
			plan.UnitActions = append(plan.UnitActions, &cluster_controllerpb.UnitAction{
				UnitName: unit,
				Action:   "stop",
			})
		}
	}

	if len(plan.UnitActions) == 0 {
		return nil // Nothing to drain
	}

	// Plan dispatch removed — drain should use workflow-native path.
	log.Printf("drain: plan dispatch skipped for node %s (plan system removed)", node.NodeID)
	_ = plan
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "drain skipped (plan removed)", 50, false, ""))
	return nil
}
