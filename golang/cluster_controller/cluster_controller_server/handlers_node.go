package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/google/uuid"
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
		if node.LastError != "" {
			if meta == nil {
				meta = make(map[string]string)
			}
			meta["last_error"] = node.LastError
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

func (srv *server) drainNode(ctx context.Context, node *nodeState, opID string) error {
	if node.AgentEndpoint == "" {
		return fmt.Errorf("node %s has no agent endpoint", node.NodeID)
	}

	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "draining node services", 10, false, ""))

	// Build a plan with stop actions for all services
	plan := &cluster_controllerpb.NodePlan{
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

	client, err := srv.getAgentClient(ctx, node.AgentEndpoint)
	if err != nil {
		return fmt.Errorf("connect to agent: %w", err)
	}

	if err := client.ApplyPlan(ctx, plan, opID); err != nil {
		return fmt.Errorf("apply drain plan: %w", err)
	}

	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "drain plan sent", 50, false, ""))

	return nil
}
