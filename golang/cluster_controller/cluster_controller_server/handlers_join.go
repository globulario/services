// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.handlers_join
// @awareness file_role=cluster_join_rpc_handlers_token_create_and_legacy_v1_join
// @awareness implements=globular.platform:intent.controller.join_lifecycle_fsm_gates_cluster_decisions
// @awareness implements=globular.platform:intent.controller.leader_election_gates_all_writes
// @awareness risk=high
package main

// handlers_join.go — gRPC handlers for the v1 join surface.
// CreateJoinToken forwards to the leader if called on a follower;
// follower-issued tokens would race against the leader's admission
// decisions. The v2 join (handlers_join_authorization.go) supersedes
// the imperative path here for newly-joined nodes; both must remain
// leader-gated.

import (
	"context"
	"log"
	"net"
	"sort"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/security"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (srv *server) CreateJoinToken(ctx context.Context, req *cluster_controllerpb.CreateJoinTokenRequest) (*cluster_controllerpb.CreateJoinTokenResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.CreateJoinTokenResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/CreateJoinToken", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	srv.lock("CreateJoinToken")
	defer srv.unlock()
	token := uuid.NewString()
	expiresAt := time.Now().Add(24 * time.Hour)
	if req != nil && req.ExpiresAt != nil {
		expiresAt = req.ExpiresAt.AsTime()
	}
	srv.state.JoinTokens[token] = &joinTokenRecord{
		Token:      token,
		ExpiresAt:  expiresAt,
		MaxUses:    1,
		ClusterUID: srv.state.ClusterUID, // bind the token to the cluster membership identity
	}
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist token: %v", err)
	}
	return &cluster_controllerpb.CreateJoinTokenResponse{
		JoinToken:  token,
		ExpiresAt:  timestamppb.New(expiresAt),
		ClusterUid: srv.state.ClusterUID, // token-bound cluster identity the installer forwards
	}, nil
}

func (srv *server) RequestJoin(ctx context.Context, req *cluster_controllerpb.RequestJoinRequest) (*cluster_controllerpb.RequestJoinResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.RequestJoinResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/RequestJoin", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if req == nil || req.GetJoinToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "join_token is required")
	}
	token := strings.TrimSpace(req.GetJoinToken())
	srv.lock("RequestJoin")
	jt := srv.state.JoinTokens[token]
	if jt == nil {
		srv.unlock()
		return nil, status.Error(codes.NotFound, "join token not found")
	}
	if time.Now().After(jt.ExpiresAt) {
		srv.unlock()
		return nil, status.Error(codes.PermissionDenied, "token expired")
	}
	if jt.Uses >= jt.MaxUses {
		srv.unlock()
		return nil, status.Error(codes.PermissionDenied, "token uses exhausted")
	}
	jt.Uses++
	reqID := uuid.NewString()
	caps := req.GetCapabilities()
	jr := &joinRequestRecord{
		RequestID:         reqID,
		Token:             token,
		Identity:          protoToStoredIdentity(req.GetIdentity()),
		Labels:            copyLabels(req.GetLabels()),
		RequestedAt:       time.Now(),
		Status:            "pending",
		LifecyclePhase:    JoinPhaseRequested,
		Capabilities:      capsToStored(caps),
		SuggestedProfiles: deduceProfiles(caps, countNodesWithProfile(srv.state.Nodes, "storage")),
	}
	srv.state.JoinRequests[reqID] = jr

	// A valid token authorizes a join ATTEMPT only. Admission into active node
	// membership requires preflight checks to pass first.
	preflightOK, preflightReason := srv.evaluateJoinPreflightLocked(jr)
	if preflightOK {
		// Auto-approve after preflight passes.
		srv.approveJoinRecordLocked(jr, nil)
	} else {
		jr.Status = "blocked"
		jr.LifecyclePhase = JoinPhaseBlocked
		jr.Reason = preflightReason
	}

	if err := srv.persistStateLocked(true); err != nil {
		srv.unlock()
		return nil, status.Errorf(codes.Internal, "persist join request: %v", err)
	}
	srv.unlock()

	if !preflightOK {
		return nil, status.Errorf(codes.FailedPrecondition, "join preflight blocked: %s", preflightReason)
	}

	// Async side-effects (RBAC binding + bootstrap workflow trigger) must run
	// outside the lock because they re-acquire it internally.
	srv.postApproveJoinAsync(jr)

	return &cluster_controllerpb.RequestJoinResponse{
		RequestId: reqID,
		Status:    jr.Status,
		Message:   jr.statusMessage(),
	}, nil
}

func (srv *server) evaluateJoinPreflightLocked(jr *joinRequestRecord) (bool, string) {
	if jr == nil {
		return false, "empty join request"
	}
	hostname := strings.TrimSpace(jr.Identity.Hostname)
	if hostname == "" {
		return false, "missing stable node identity: hostname is required"
	}

	primaryIP := ""
	for _, raw := range jr.Identity.Ips {
		ip := strings.TrimSpace(raw)
		if ip == "" {
			continue
		}
		parsed := net.ParseIP(ip)
		if parsed == nil || parsed.IsLoopback() {
			continue
		}
		primaryIP = ip
		break
	}
	if primaryIP == "" {
		return false, "missing stable node identity: routable non-loopback IP is required"
	}

	for _, n := range srv.state.Nodes {
		if n == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(n.Identity.Hostname), hostname) {
			return false, "node identity conflict: hostname already present"
		}
		for _, existingIP := range n.Identity.Ips {
			if strings.TrimSpace(existingIP) == primaryIP {
				return false, "node identity conflict: IP already present"
			}
		}
	}

	// TODO(day1): add stronger preflight checks before admission:
	// - repository active release index/build-id resolvable
	// - etcd endpoint reachability with approved endpoint set
	// - CA fingerprint match against cluster CA metadata
	return true, ""
}

func (srv *server) ListJoinRequests(ctx context.Context, req *cluster_controllerpb.ListJoinRequestsRequest) (*cluster_controllerpb.ListJoinRequestsResponse, error) {
	srv.lock("unknown")
	defer srv.unlock()
	resp := &cluster_controllerpb.ListJoinRequestsResponse{}
	pending := make([]*joinRequestRecord, 0, len(srv.state.JoinRequests))
	for _, jr := range srv.state.JoinRequests {
		if jr.Status != "pending" {
			continue
		}
		pending = append(pending, jr)
	}
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].RequestedAt.Before(pending[j].RequestedAt)
	})
	for _, jr := range pending {
		resp.Pending = append(resp.Pending, &cluster_controllerpb.JoinRequestRecord{
			RequestId:         jr.RequestID,
			Identity:          storedIdentityToProto(jr.Identity),
			Status:            jr.Status,
			Profiles:          append([]string(nil), jr.Profiles...),
			Metadata:          copyLabels(jr.Labels),
			Capabilities:      storedToProtoCapabilities(jr.Capabilities),
			SuggestedProfiles: append([]string(nil), jr.SuggestedProfiles...),
		})
	}
	return resp, nil
}

func (srv *server) ApproveJoin(ctx context.Context, req *cluster_controllerpb.ApproveJoinRequest) (*cluster_controllerpb.ApproveJoinResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.ApproveJoinResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/ApproveJoin", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	reqID := strings.TrimSpace(req.GetRequestId())
	if reqID == "" {
		reqID = strings.TrimSpace(req.GetNodeId())
	}
	if reqID == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}
	srv.lock("ApproveJoin")
	jr := srv.state.JoinRequests[reqID]
	if jr == nil {
		srv.unlock()
		return nil, status.Error(codes.NotFound, "join request not found")
	}
	if jr.Status != "pending" {
		srv.unlock()
		return nil, status.Error(codes.FailedPrecondition, "request not pending")
	}

	srv.approveJoinRecordLocked(jr, req.GetProfiles())

	if err := srv.persistStateLocked(true); err != nil {
		srv.unlock()
		return nil, status.Errorf(codes.Internal, "persist node state: %v", err)
	}
	srv.unlock()

	srv.postApproveJoinAsync(jr)

	return &cluster_controllerpb.ApproveJoinResponse{
		NodeId:        jr.AssignedNodeID,
		Message:       "approved; node will receive configuration on first heartbeat",
		NodeToken:     jr.NodeToken,
		NodePrincipal: jr.NodePrincipal,
	}, nil
}

// approveJoinRecordLocked performs the in-memory approval of a pending join
// request. It MUST be called with srv.lock held. It does NOT persist state or
// launch async tasks — callers are responsible for that.
//
// profiles may be nil/empty; in that case the suggested or default profiles are used.
func (srv *server) approveJoinRecordLocked(jr *joinRequestRecord, profiles []string) {
	explicitProfiles := len(profiles) > 0
	if len(profiles) == 0 {
		profiles = jr.SuggestedProfiles
	}
	if len(profiles) == 0 {
		profiles = srv.cfg.DefaultProfiles
	}
	// When the operator did not request an explicit profile set, inherit the
	// cluster's existing assignable (catalog) profiles so joining nodes come out
	// identical to the founder (e.g. dns, compute). Hardware-gated profiles
	// (control-plane, storage, gateway) stay governed per-node by deduceProfiles
	// and enforceFoundingProfiles; opt-in workloads (media-server) and derived
	// non-catalog labels (e.g. "ai") are not inherited. See inheritableClusterProfiles.
	if !explicitProfiles {
		profiles = append(append([]string(nil), profiles...), inheritableClusterProfiles(srv.state.Nodes)...)
	}
	profiles = normalizeProfiles(profiles)

	// INVARIANT: The first 3 nodes MUST have foundational profiles
	// (core, control-plane, storage) to establish quorum for etcd,
	// ScyllaDB, and MinIO. Without 3 storage nodes, there is no
	// redundancy — MinIO becomes a single point of failure that
	// cascades into workflow execution and artifact publishing.
	// Count only VERIFIED storage members toward founding quorum — a node that
	// carries the label but has not verified in the ring/pool is not capacity
	// (forbidden_fix:profile_label_counts_as_storage_capacity). Under-counting is
	// the safe direction here: it forces storage onto the joiner rather than
	// assuming quorum is already met.
	storageCount := countVerifiedNodesWithProfile(srv.state.Nodes, "storage")
	profiles = enforceFoundingProfiles(profiles, storageCount)
	jr.Profiles = profiles

	nodeID := deterministicNodeID(jr.Identity, jr.Labels)
	jr.AssignedNodeID = nodeID
	jr.Status = "approved"
	// TODO(v2-join): legacy RequestJoin still creates node state during approval.
	// New signed JoinPlan flow should advance to admitted only after node-agent proof.
	// For now set LifecyclePhase = join_authorized to match the v2 path; the node
	// itself gets bootstrapping so RF eligibility gates correctly.
	if jr.LifecyclePhase != JoinPhaseAuthorized {
		jr.LifecyclePhase = JoinPhaseAuthorized
	}

	// Compute the node's advertised FQDN for DNS registration.
	// Format: <hostname>.<cluster-domain> (e.g. globule-dell.globular.internal)
	advertiseFqdn := ""
	if hostname := strings.TrimSpace(jr.Identity.Hostname); hostname != "" {
		domain := ""
		if srv.state.ClusterNetworkSpec != nil {
			domain = strings.TrimSuffix(strings.TrimSpace(srv.state.ClusterNetworkSpec.GetClusterDomain()), ".")
		}
		if domain != "" {
			advertiseFqdn = hostname + "." + domain
		}
	}

	node := &nodeState{
		NodeID:                nodeID,
		Identity:              jr.Identity,
		Profiles:              profiles,
		LastSeen:              time.Now(),
		Status:                "converging",
		Metadata:              copyLabels(jr.Labels),
		LastAppliedGeneration: 0,
		BootstrapPhase:        BootstrapAdmitted,
		BootstrapStartedAt:    time.Now(),
		AdvertiseFqdn:         advertiseFqdn,
		// TODO(v2-join): advance to admitted only after node-agent proof (Phase F).
		// Set bootstrapping so RF eligibility gates correctly: the node is not yet
		// eligible until node-agent registers and admission is confirmed.
		JoinLifecyclePhase: JoinPhaseBootstrapping,
		// Infrastructure intents: profiles are capability labels; intents are
		// controller-authorized membership. RFEligible starts false for all new
		// nodes — runtime proof required before the controller sets it true.
		EtcdMemberIntent:  initialEtcdIntentForProfiles(profiles),
		ScyllaIntent:      initialScyllaIntentForProfiles(profiles),
		ObjectStoreIntent: initialObjectStoreIntentForProfiles(profiles),
	}
	srv.state.Nodes[nodeID] = node
	srv.removeStaleNodesLocked(nodeID, jr.Identity, "")

	nodePrincipal := "node_" + nodeID
	nodeToken, err := security.GenerateToken(
		365*24*60,    // 1 year TTL
		nodeID,       // audience = node ID
		nodePrincipal,
		"node-agent",
		"",
	)
	if err != nil {
		log.Printf("WARN: failed to generate node token for %s: %v", nodeID, err)
	} else {
		jr.NodeToken = nodeToken
		jr.NodePrincipal = nodePrincipal
	}
}

// postApproveJoinAsync launches the async side-effects after an approval:
// RBAC binding and bootstrap workflow trigger. Must be called WITHOUT the lock.
func (srv *server) postApproveJoinAsync(jr *joinRequestRecord) {
	nodeID := jr.AssignedNodeID
	nodePrincipal := jr.NodePrincipal

	if nodePrincipal != "" {
		go srv.ensureNodeExecutorBinding(nodePrincipal)
	}

	// Trigger the join workflow immediately if the node-agent is already
	// reachable. The "first heartbeat" trigger in ReportNodeStatus may miss
	// if the agent started heartbeating before approval (race condition).
	srv.lock("postApproveJoinAsync:triggerWorkflow")
	node := srv.state.Nodes[nodeID]
	if node != nil && !node.BootstrapWorkflowActive {
		node.BootstrapWorkflowActive = true
		agentEndpoint := node.AgentEndpoint
		log.Printf("ApproveJoin: triggering join workflow for %s at %s", nodeID, agentEndpoint)
		go srv.triggerJoinWorkflow(nodeID, agentEndpoint)
	}
	srv.unlock()
}

func (srv *server) RejectJoin(ctx context.Context, req *cluster_controllerpb.RejectJoinRequest) (*cluster_controllerpb.RejectJoinResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.RejectJoinResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/RejectJoin", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	reqID := strings.TrimSpace(req.GetRequestId())
	if reqID == "" {
		reqID = strings.TrimSpace(req.GetNodeId())
	}
	if reqID == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}
	srv.lock("unknown")
	defer srv.unlock()
	jr := srv.state.JoinRequests[reqID]
	if jr == nil {
		return nil, status.Error(codes.NotFound, "join request not found")
	}
	if jr.Status != "pending" {
		return nil, status.Error(codes.FailedPrecondition, "request not pending")
	}
	jr.Status = "rejected"
	jr.LifecyclePhase = JoinPhaseRejected
	jr.Reason = req.GetReason()
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist join request: %v", err)
	}
	return &cluster_controllerpb.RejectJoinResponse{
		NodeId:  jr.AssignedNodeID,
		Message: "rejected",
	}, nil
}

func (srv *server) cleanupJoinStateLocked(now time.Time) bool {
	dirty := false
	for token, jt := range srv.state.JoinTokens {
		if jt.MaxUses > 0 && jt.Uses >= jt.MaxUses {
			delete(srv.state.JoinTokens, token)
			dirty = true
			continue
		}
		if !jt.ExpiresAt.IsZero() && now.After(jt.ExpiresAt) {
			delete(srv.state.JoinTokens, token)
			dirty = true
		}
	}
	for reqID, jr := range srv.state.JoinRequests {
		if jr.Status == "pending" {
			if now.Sub(jr.RequestedAt) > pendingJoinRetention {
				delete(srv.state.JoinRequests, reqID)
				dirty = true
			}
			continue
		}
		if now.Sub(jr.RequestedAt) > joinRequestRetention {
			delete(srv.state.JoinRequests, reqID)
			dirty = true
		}
	}
	return dirty
}

func (srv *server) GetJoinRequestStatus(ctx context.Context, req *cluster_controllerpb.GetJoinRequestStatusRequest) (*cluster_controllerpb.GetJoinRequestStatusResponse, error) {
	if req == nil || req.GetRequestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id is required")
	}
	srv.lock("unknown")
	defer srv.unlock()
	jr := srv.state.JoinRequests[req.GetRequestId()]
	if jr == nil {
		return nil, status.Error(codes.NotFound, "join request not found")
	}
	return &cluster_controllerpb.GetJoinRequestStatusResponse{
		Status:        jr.Status,
		NodeId:        jr.AssignedNodeID,
		Message:       jr.Reason,
		Profiles:      append([]string(nil), jr.Profiles...),
		NodeToken:     jr.NodeToken,
		NodePrincipal: jr.NodePrincipal,
		PlanJson:      append([]byte(nil), jr.JoinPlanJSON...),
	}, nil
}
