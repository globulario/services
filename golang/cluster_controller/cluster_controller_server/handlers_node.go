// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.handlers_node
// @awareness file_role=node_lifecycle_rpc_handlers_list_get_remove_update_profiles
// @awareness enforces=globular.platform:invariant.destructive_actions.require_explicit_guard
// @awareness implements=globular.platform:intent.controller.leader_election_gates_all_writes
// @awareness risk=critical
package main

// handlers_node.go — gRPC handlers for node lifecycle (list/get/
// remove/update profiles). RemoveNode is destructive — it deletes
// the node's state from etcd and triggers downstream cleanup. The
// handler MUST be leader-only and the request MUST carry the
// explicit-removal contract (matching node_removal_requests.go).
//
import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
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
		// Phase B.2: admission proof observability.
		for k, v := range admissionStatusMetadata(node) {
			meta[k] = v
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
	if !srv.isLeader() {
		resp := &cluster_controllerpb.SetNodeProfilesResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/SetNodeProfiles", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if req == nil || req.GetNodeId() == "" || len(req.GetProfiles()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "--profile is required")
	}
	normalized := normalizeProfiles(req.GetProfiles())
	srv.lock("SetNodeProfiles")
	defer srv.unlock()
	node := srv.state.Nodes[req.GetNodeId()]
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}

	// D1c 1a: route the profile write through the single owner mutation so a
	// real (set-level) change atomically bumps PlacementGeneration; an idempotent
	// re-apply leaves both profiles and the generation untouched.
	applyNodePlacementProfilesLocked(node, normalized)
	node.LastSeen = time.Now()
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist node profiles: %v", err)
	}
	// Update the node_identity projection so labels reflect the new profiles.
	// Best-effort — must not fail the handler (Clause 3).
	if srv.nodeIdentityProj != nil {
		id := nodeToIdentity(node)
		go func() {
			bg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := srv.nodeIdentityProj.Upsert(bg, *id); err != nil {
				log.Printf("node_identity: upsert %s after SetNodeProfiles failed: %v", id.NodeID, err)
			}
		}()
	}

	if srv.enqueueReconcile != nil {
		srv.enqueueReconcile()
	}
	return &cluster_controllerpb.SetNodeProfilesResponse{
		OperationId: uuid.NewString(),
	}, nil
}

// SetNodeBootstrapPhase is the workflow-driven RPC handler. Node-agent calls
// this from its workflow steps to advance its own bootstrap phase. The
// controller updates in-memory node state and emits a lifecycle event.
func (srv *server) SetNodeBootstrapPhase(ctx context.Context, req *cluster_controllerpb.SetNodeBootstrapPhaseRequest) (*cluster_controllerpb.SetNodeBootstrapPhaseResponse, error) {
	if req == nil || req.GetNodeId() == "" || req.GetPhase() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id and phase are required")
	}
	if reason := strings.TrimSpace(req.GetReason()); reason != "" {
		// Persist the reason on the node record before the phase transition.
		srv.lock("SetNodeBootstrapPhase:reason")
		if node := srv.state.Nodes[req.GetNodeId()]; node != nil {
			node.BootstrapError = reason
		}
		srv.unlock()
	}
	if err := srv.setBootstrapPhase(req.GetNodeId(), req.GetPhase()); err != nil {
		return nil, status.Errorf(codes.Internal, "set bootstrap phase: %v", err)
	}
	return &cluster_controllerpb.SetNodeBootstrapPhaseResponse{Accepted: true}, nil
}

// EmitWorkflowEvent is called by node-agent workflow steps to publish
// cluster-wide events (e.g. node.bootstrap.ready) via the controller.
func (srv *server) EmitWorkflowEvent(ctx context.Context, req *cluster_controllerpb.EmitWorkflowEventRequest) (*cluster_controllerpb.EmitWorkflowEventResponse, error) {
	if req == nil || req.GetEventType() == "" {
		return nil, status.Error(codes.InvalidArgument, "event_type is required")
	}
	payload := make(map[string]interface{}, len(req.GetData()))
	for k, v := range req.GetData() {
		payload[k] = v
	}
	srv.emitClusterEvent(req.GetEventType(), payload)
	return &cluster_controllerpb.EmitWorkflowEventResponse{Published: true}, nil
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

// RemoveNode dispatches the node.remove workflow. The previously-inline
// orchestration (preflight + etcd-membership-remove + drain + state-delete
// + scylla-publish + etcd-prefix-cleanup + release-purge + scylla-ring-remove)
// is now declared step-by-step in golang/workflow/definitions/node.remove.yaml.
// This handler captures the node's mutable state under srv.lock(), passes
// it as workflow inputs, and waits for terminal status. See failure_mode
// hidden_workflow.controller_remove_node_inline_preflight_and_drain.
func (srv *server) RemoveNode(ctx context.Context, req *cluster_controllerpb.RemoveNodeRequest) (*cluster_controllerpb.RemoveNodeResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.RemoveNodeResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/RemoveNode", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())

	// Capture mutable state under lock so the workflow steps operate on a
	// consistent snapshot. The workflow handler functions also acquire the
	// lock individually for state-mutating steps (delete_state in
	// particular) — capture-then-dispatch avoids holding the lock across
	// the workflow's wall-clock.
	srv.lock("remove-node-capture")
	node := srv.state.Nodes[nodeID]
	if node == nil {
		srv.unlock()
		return nil, status.Error(codes.NotFound, "node not found")
	}
	captured := nodeRemoveInputs{
		NodeID:        nodeID,
		Hostname:      node.Identity.Hostname,
		NodeIPs:       append([]string(nil), node.Identity.Ips...),
		ScyllaHostID:  node.ScyllaHostID,
		AgentEndpoint: node.AgentEndpoint,
		Force:         req.GetForce(),
		Drain:         req.GetDrain(),
		OpID:          uuid.NewString(),
	}
	srv.unlock()

	_, wfStatus, wfErr, derr := srv.dispatchNodeRemove(ctx, captured)
	if derr != nil {
		return nil, status.Errorf(codes.Internal, "dispatch node.remove workflow: %v", derr)
	}
	if wfStatus == "FAILED" {
		// Surface the workflow's error to the operator. Topology-violation
		// failures from the preflight step return as workflow errors; the
		// caller still gets FailedPrecondition for those.
		log.Printf("remove-node: workflow FAILED node=%s err=%s", nodeID, wfErr)
		return nil, status.Errorf(codes.FailedPrecondition, "node.remove workflow failed: %s", wfErr)
	}

	// Close agent client if we had one. (Workflow doesn't touch RPC
	// clients; the controller's connection pool cleanup is still done
	// here at the handler boundary.)
	if captured.AgentEndpoint != "" {
		srv.closeAgentClient(captured.AgentEndpoint)
	}

	message := fmt.Sprintf("node %s removed from cluster", nodeID)
	srv.broadcastOperationEvent(srv.newOperationEvent(captured.OpID, nodeID, cluster_controllerpb.OperationPhase_OP_SUCCEEDED, message, 100, true, ""))

	return &cluster_controllerpb.RemoveNodeResponse{
		OperationId: captured.OpID,
		Message:     message,
	}, nil
}

func (srv *server) removeNodeEtcdMembership(ctx context.Context, removedNodeID string) error {
	if srv == nil || srv.etcdMembers == nil || removedNodeID == "" {
		return nil
	}

	membership := srv.snapshotClusterMembership()
	if membership == nil {
		return nil
	}

	filtered := make([]memberNode, 0, len(membership.Nodes))
	removedWasEtcd := false
	for _, node := range membership.Nodes {
		if node.NodeID == removedNodeID {
			removedWasEtcd = nodeHasProfile(&node, profilesForEtcd)
			continue
		}
		filtered = append(filtered, node)
	}
	if !removedWasEtcd {
		return nil
	}

	desired := filterNodesByProfile(&clusterMembership{
		ClusterID: membership.ClusterID,
		Nodes:     filtered,
	}, profilesForEtcd)
	if len(desired) == 0 {
		return nil
	}

	pruneCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := srv.etcdMembers.removeStaleMembers(pruneCtx, desired); err != nil {
		return err
	}
	return nil
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

// cleanNodeFromReleases removes a node's status entries from every
// ServiceRelease, ApplicationRelease, and InfrastructureRelease in the
// controller's resource store. Without this, a re-added node is seen as
// "already installed" for packages that were never actually installed
// on the new disk.
//
// Authority boundary: the cluster_controller OWNS these release prefixes
// (Status field is controller-written). The function routes through
// srv.resources — the controller's own typed abstraction — instead of
// raw etcd Get/Put. This preserves the type, version, and audit
// contracts that the resource store applies on Apply, and keeps the
// function aligned with invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage
// (the controller reads its own owned data through its own typed API,
// not through generic etcd primitives — even when "I am the owner"
// might tempt a shortcut).
//
// Returns the number of release objects mutated.
func (srv *server) cleanNodeFromReleases(ctx context.Context, nodeID string) int {
	if srv == nil || srv.resources == nil {
		return 0
	}
	cleaned := 0
	cleaned += srv.cleanNodeFromServiceReleases(ctx, nodeID)
	cleaned += srv.cleanNodeFromApplicationReleases(ctx, nodeID)
	cleaned += srv.cleanNodeFromInfrastructureReleases(ctx, nodeID)
	return cleaned
}

// filterNodeStatuses returns a new slice with any entry matching nodeID
// removed. The second return is true iff the input contained nodeID.
func filterNodeStatuses(nodes []*cluster_controllerpb.NodeReleaseStatus, nodeID string) ([]*cluster_controllerpb.NodeReleaseStatus, bool) {
	out := make([]*cluster_controllerpb.NodeReleaseStatus, 0, len(nodes))
	dropped := false
	for _, n := range nodes {
		if n != nil && n.NodeID == nodeID {
			dropped = true
			continue
		}
		out = append(out, n)
	}
	return out, dropped
}

func (srv *server) cleanNodeFromServiceReleases(ctx context.Context, nodeID string) int {
	items, _, err := srv.resources.List(ctx, "ServiceRelease", "")
	if err != nil {
		log.Printf("remove-node: list ServiceRelease via resources: %v", err)
		return 0
	}
	cleaned := 0
	for _, obj := range items {
		rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
		if !ok || rel == nil || rel.Meta == nil || rel.Status == nil {
			continue
		}
		filtered, dropped := filterNodeStatuses(rel.Status.Nodes, nodeID)
		if !dropped {
			continue
		}
		rel.Status.Nodes = filtered
		rel.Status.Phase = cluster_controllerpb.ReleasePhasePending
		if _, err := srv.applyServiceRelease(ctx, rel); err != nil {
			log.Printf("remove-node: apply ServiceRelease %s after purge: %v", rel.Meta.Name, err)
			continue
		}
		cleaned++
		log.Printf("remove-node: purged node %s from ServiceRelease %s", nodeID, rel.Meta.Name)
	}
	return cleaned
}

func (srv *server) cleanNodeFromApplicationReleases(ctx context.Context, nodeID string) int {
	items, _, err := srv.resources.List(ctx, "ApplicationRelease", "")
	if err != nil {
		log.Printf("remove-node: list ApplicationRelease via resources: %v", err)
		return 0
	}
	cleaned := 0
	for _, obj := range items {
		rel, ok := obj.(*cluster_controllerpb.ApplicationRelease)
		if !ok || rel == nil || rel.Meta == nil || rel.Status == nil {
			continue
		}
		filtered, dropped := filterNodeStatuses(rel.Status.Nodes, nodeID)
		if !dropped {
			continue
		}
		rel.Status.Nodes = filtered
		rel.Status.Phase = cluster_controllerpb.ReleasePhasePending
		if _, err := srv.resources.Apply(ctx, "ApplicationRelease", rel); err != nil {
			log.Printf("remove-node: apply ApplicationRelease %s after purge: %v", rel.Meta.Name, err)
			continue
		}
		cleaned++
		log.Printf("remove-node: purged node %s from ApplicationRelease %s", nodeID, rel.Meta.Name)
	}
	return cleaned
}

func (srv *server) cleanNodeFromInfrastructureReleases(ctx context.Context, nodeID string) int {
	items, _, err := srv.resources.List(ctx, "InfrastructureRelease", "")
	if err != nil {
		log.Printf("remove-node: list InfrastructureRelease via resources: %v", err)
		return 0
	}
	cleaned := 0
	for _, obj := range items {
		rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
		if !ok || rel == nil || rel.Meta == nil || rel.Status == nil {
			continue
		}
		filtered, dropped := filterNodeStatuses(rel.Status.Nodes, nodeID)
		if !dropped {
			continue
		}
		rel.Status.Nodes = filtered
		rel.Status.Phase = cluster_controllerpb.ReleasePhasePending
		if _, err := srv.resources.Apply(ctx, "InfrastructureRelease", rel); err != nil {
			log.Printf("remove-node: apply InfrastructureRelease %s after purge: %v", rel.Meta.Name, err)
			continue
		}
		cleaned++
		log.Printf("remove-node: purged node %s from InfrastructureRelease %s", nodeID, rel.Meta.Name)
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

	// Stop each service via the node-agent ControlService RPC.
	agent, agentErr := srv.getAgentClient(ctx, node.AgentEndpoint)
	if agentErr != nil {
		return fmt.Errorf("drain: cannot reach node-agent at %s: %w", node.AgentEndpoint, agentErr)
	}

	var stopErrors []string
	for _, ua := range plan.UnitActions {
		resp, err := agent.ControlService(ctx, ua.UnitName, "stop")
		if err != nil {
			stopErrors = append(stopErrors, fmt.Sprintf("%s: %v", ua.UnitName, err))
			log.Printf("drain: stop %s on %s failed: %v", ua.UnitName, node.NodeID, err)
			continue
		}
		log.Printf("drain: stop %s on %s → %s (%s)", ua.UnitName, node.NodeID, resp.GetState(), resp.GetMessage())
	}

	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_RUNNING,
		fmt.Sprintf("drained %d/%d services", len(plan.UnitActions)-len(stopErrors), len(plan.UnitActions)), 50, false, ""))

	if len(stopErrors) > 0 {
		return fmt.Errorf("drain: %d/%d services failed to stop: %v", len(stopErrors), len(plan.UnitActions), stopErrors)
	}
	return nil
}
