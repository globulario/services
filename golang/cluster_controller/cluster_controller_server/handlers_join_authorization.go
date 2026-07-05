// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.handlers_join_authorization
// @awareness file_role=v2_join_signed_joinplan_issuer_with_founding_quorum_and_profile_policy
// @awareness implements=globular.platform:intent.controller.join_lifecycle_fsm_gates_cluster_decisions
// @awareness enforces=globular.platform:invariant.founding.quorum.three_nodes_required
// @awareness risk=critical
package main

// handlers_join_authorization.go — RequestJoinAuthorization is the
// v2 join entry point. The installer arrives with a join token + its
// stable identity; this handler:
//
//   - verifies the token
//   - enforces cluster policy (founding quorum: first 3 nodes MUST
//     carry core+control-plane+storage; profile assignment per
//     enforceFoundingProfiles())
//   - issues a signed JoinPlan (TTL 2 h)
//
// The installer MUST validate the plan signature before any
// cluster-affecting step. Bypassing signature validation collapses
// the join trust model into "whoever can reach the controller can
// join" — anchoring this handler to controller.join_lifecycle_fsm
// and the founding.quorum invariant makes that boundary explicit.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// joinPlanTTL is how long a newly issued JoinPlan remains valid.
const joinPlanTTL = 2 * time.Hour

// RequestJoinAuthorization is the v2 join path. The installer calls this RPC
// with a join token and its stable identity; the controller verifies the token,
// enforces cluster policy (founding quorum, profile assignment), and returns a
// signed JoinPlan. The installer must validate the plan before executing any
// cluster-affecting step.
//
// The gateway MUST NOT invent profiles, etcd intent, or release identity. It
// is only a courier — it forwards this request and returns the response.
func (srv *server) RequestJoinAuthorization(ctx context.Context, req *cluster_controllerpb.JoinAuthorizationRequest) (*cluster_controllerpb.JoinAuthorizationResponse, error) {
	if !srv.isLeader() {
		resp := &cluster_controllerpb.JoinAuthorizationResponse{}
		if err := srv.leaderForward(ctx, "/cluster_controller.ClusterControllerService/RequestJoinAuthorization", req, resp); err != nil {
			return nil, err
		}
		return resp, nil
	}
	if req == nil || strings.TrimSpace(req.GetJoinToken()) == "" {
		return nil, status.Error(codes.InvalidArgument, "join_token is required")
	}

	// Convert proto identity to internal Go type for core logic.
	goReq := protoToJoinAuthRequest(req)

	resp, err := srv.requestJoinAuthorizationCore(goReq)
	if err != nil {
		return nil, err
	}

	// Serialize plan to JSON for the proto response.
	planJSON := []byte{}
	if resp.Plan != nil {
		planJSON, err = json.Marshal(resp.Plan)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "marshal join plan: %v", err)
		}
	}

	pbResp := &cluster_controllerpb.JoinAuthorizationResponse{
		Allowed:              resp.Allowed,
		DeniedReason:         resp.DeniedReason,
		JoinId:               resp.JoinID,
		PlanJson:             planJSON,
		ControllerGeneration: resp.ControllerGeneration,
	}
	return pbResp, nil
}

// requestJoinAuthorizationCore implements the authorization logic without
// gRPC transport concerns. Separated for unit testing.
func (srv *server) requestJoinAuthorizationCore(req *JoinAuthorizationRequest) (*JoinAuthorizationResponse, error) {
	token := strings.TrimSpace(req.JoinToken)

	srv.lock("RequestJoinAuthorization")
	jt := srv.state.JoinTokens[token]
	if jt == nil {
		srv.unlock()
		return nil, status.Error(codes.NotFound, "join token not found")
	}
	if time.Now().After(jt.ExpiresAt) {
		srv.unlock()
		return nil, status.Error(codes.PermissionDenied, "join token expired")
	}
	if jt.Uses >= jt.MaxUses {
		srv.unlock()
		return nil, status.Error(codes.PermissionDenied, "join token uses exhausted")
	}

	// Cluster identity gate: if the installer knows the cluster_id, it must match.
	//
	// Still namespace-based (state.ClusterId) BY DESIGN until A6: the installer
	// learns the membership UUID only FROM the signed JoinPlan (issued after this
	// authorization), so it cannot present the UUID here yet. A5 distributes the
	// UUID via the JoinPlan; A6 then flips this gate to state.ClusterUID. The
	// primary membership enforcement (the gRPC interceptor) is already UUID-only.
	if callerClusterID := strings.TrimSpace(req.ClusterID); callerClusterID != "" {
		if srv.state.ClusterId != "" && callerClusterID != srv.state.ClusterId {
			srv.unlock()
			return &JoinAuthorizationResponse{
				Allowed:      false,
				DeniedReason: fmt.Sprintf("cluster_id mismatch: request=%q cluster=%q", callerClusterID, srv.state.ClusterId),
			}, nil
		}
	}

	// Membership identity gate (A6-final): the cluster identity is the minted UUID
	// (state.ClusterUID), never the domain. Once the cluster is INITIALIZED
	// (ClusterUID set), EVERY join token must be bound to it:
	//   - unbound token (jt.ClusterUID == "") is REJECTED — no legitimate token is
	//     unbound after init. The config token is bound at seed/reconcile and
	//     backfilled atomically at mint (ensureClusterMembershipID), so an unbound
	//     token here is stale or forged.
	//   - a token bound to a DIFFERENT cluster is REJECTED (jt.ClusterUID is
	//     server-side / unspoofable).
	//   - a FORWARDED value that doesn't match (e.g. the domain) is REJECTED.
	// BEFORE init (ClusterUID == "") the token secret + the gRPC interceptor are
	// the gates and unbound is expected — the Day-0 bootstrap window is not
	// steady-state auth (intent:bootstrap.window_is_not_steady_state_auth). This
	// does not weaken founding-quorum join: the mint backfill binds the founding
	// config token atomically with ClusterUID, so nodes 2/3 present a bound token.
	if srv.state.ClusterUID != "" {
		if strings.TrimSpace(jt.ClusterUID) == "" {
			srv.unlock()
			return &JoinAuthorizationResponse{
				Allowed:      false,
				DeniedReason: "join token is not bound to the cluster membership identity",
			}, nil
		}
		if jt.ClusterUID != srv.state.ClusterUID {
			srv.unlock()
			return &JoinAuthorizationResponse{
				Allowed:      false,
				DeniedReason: fmt.Sprintf("join token bound to a different cluster: token=%q cluster=%q", jt.ClusterUID, srv.state.ClusterUID),
			}, nil
		}
		if callerUID := strings.TrimSpace(req.ClusterUID); callerUID != "" && callerUID != srv.state.ClusterUID {
			srv.unlock()
			return &JoinAuthorizationResponse{
				Allowed:      false,
				DeniedReason: fmt.Sprintf("cluster_uid mismatch: request=%q cluster=%q", callerUID, srv.state.ClusterUID),
			}, nil
		}
	}

	jt.Uses++
	joinID := uuid.NewString()

	identity := storedIdentity{
		Hostname: strings.TrimSpace(req.Identity.Hostname),
		Ips:      append([]string(nil), req.Identity.IPs...),
	}

	caps := &cluster_controllerpb.NodeCapabilities{
		CpuCount:  req.CPUCount,
		RamBytes:  req.RAMBytes,
		DiskBytes: req.DiskBytes,
	}

	jr := &joinRequestRecord{
		RequestID:         joinID,
		Token:             token,
		Identity:          identity,
		Labels:            copyLabels(req.Labels),
		RequestedAt:       time.Now(),
		Status:            "pending",
		LifecyclePhase:    JoinPhaseRequested,
		SuggestedProfiles: deduceProfiles(caps, countNodesWithProfile(srv.state.Nodes, "storage")),
	}
	srv.state.JoinRequests[joinID] = jr

	// Preflight: hostname/IP stability, no conflicts.
	if ok, reason := srv.evaluateJoinPreflightLocked(jr); !ok {
		jr.LifecyclePhase = JoinPhaseBlocked
		srv.unlock()
		_ = srv.persistStateLocked(false)
		return &JoinAuthorizationResponse{
			Allowed:      false,
			DeniedReason: reason,
			JoinID:       joinID,
		}, nil
	}

	// Approve: assign profiles (enforcing founding quorum), assign node_id.
	// Operator-requested profiles (join --profiles) take precedence over the
	// suggested/inherited default; the controller validates them to catalog
	// profiles here (the gateway is a courier and never assigns). Unknown labels
	// are dropped, degrading to defaults. enforceFoundingProfiles in the approve
	// path still guarantees core+control-plane+storage on the first quorum nodes.
	srv.approveJoinRecordLocked(jr, filterCatalogProfiles(req.RequestedProfiles))

	// Build and sign the JoinPlan now that profiles and node_id are determined.
	plan, err := srv.buildJoinPlan(jr)
	if err != nil {
		srv.unlock()
		return nil, status.Errorf(codes.Internal, "build join plan: %v", err)
	}

	// Signed JoinPlan issued: the installer is authorized to attempt bootstrap.
	// This is NOT admission — the node does not become RF/topology eligible here.
	jr.LifecyclePhase = JoinPhaseAuthorized

	// Store the signed plan JSON so GetJoinRequestStatus can return it.
	if planJSON, merr := json.Marshal(plan); merr == nil {
		jr.JoinPlanJSON = planJSON
	}

	if err := srv.persistStateLocked(true); err != nil {
		srv.unlock()
		return nil, status.Errorf(codes.Internal, "persist join authorization: %v", err)
	}
	srv.unlock()

	// Async side-effects (RBAC binding, bootstrap workflow trigger).
	srv.postApproveJoinAsync(jr)

	return &JoinAuthorizationResponse{
		Allowed:              true,
		JoinID:               joinID,
		Plan:                 plan,
		ExpiresAt:            plan.ExpiresAt,
		ControllerGeneration: plan.ControllerGeneration,
	}, nil
}

// buildJoinPlan constructs and signs a JoinPlan from an approved joinRequestRecord.
// Must be called with the server lock held (reads srv.state).
func (srv *server) buildJoinPlan(jr *joinRequestRecord) (*JoinPlan, error) {
	clusterID := srv.state.ClusterId
	generation := int64(srv.state.NetworkingGeneration)

	bootstrapEndpoints := []string{}
	if srv.cfg != nil && srv.cfg.ClusterDomain != "" {
		bootstrapEndpoints = append(bootstrapEndpoints, srv.cfg.ClusterDomain+":12000")
	}

	plan := &JoinPlan{
		JoinID:               jr.RequestID,
		ClusterID:            clusterID,           // namespace (domain)
		ClusterUID:           srv.state.ClusterUID, // membership identity (signed)
		ControllerGeneration: generation,
		IssuedAt:             time.Now(),
		ExpiresAt:            time.Now().Add(joinPlanTTL),
		AssignedProfiles:     append([]string(nil), jr.Profiles...),
		AssignedNodeID:       jr.AssignedNodeID,
		NodePrincipal:        jr.NodePrincipal,
		ExpectedNodeIdentity: storedToNodePlanIdentity(jr.Identity),
		BootstrapEndpoints:   bootstrapEndpoints,
	}

	// Sign the plan with the controller's Ed25519 key.
	if err := SignJoinPlan(plan); err != nil {
		// Signing failure is non-fatal in degraded mode (key not yet generated).
		// Log and continue: the installer will reject unsigned plans.
		log.Printf("WARN: join_authorization: failed to sign JoinPlan for %s: %v", jr.RequestID, err)
	}

	return plan, nil
}

// storedToNodePlanIdentity converts a joinRequestRecord's identity to the
// compact NodePlanIdentity embedded in a JoinPlan.
func storedToNodePlanIdentity(id storedIdentity) NodePlanIdentity {
	return NodePlanIdentity{
		Hostname: id.Hostname,
		IPs:      append([]string(nil), id.Ips...),
	}
}

// protoToJoinAuthRequest converts a proto JoinAuthorizationRequest to the
// internal Go type consumed by requestJoinAuthorizationCore.
func protoToJoinAuthRequest(req *cluster_controllerpb.JoinAuthorizationRequest) *JoinAuthorizationRequest {
	if req == nil {
		return &JoinAuthorizationRequest{}
	}
	id := NodePlanIdentity{}
	if pi := req.GetIdentity(); pi != nil {
		id.Hostname = strings.TrimSpace(pi.GetHostname())
		id.IPs = append([]string(nil), pi.GetIps()...)
	}
	caps := req.GetCapabilities()
	r := &JoinAuthorizationRequest{
		JoinToken:         strings.TrimSpace(req.GetJoinToken()),
		Identity:          id,
		Labels:            copyLabels(req.GetLabels()),
		Nonce:             req.GetNonce(),
		InstallerVersion:  req.GetInstallerVersion(),
		ClusterID:         strings.TrimSpace(req.GetClusterId()),
		ClusterUID:        strings.TrimSpace(req.GetClusterUid()),
		RequestedProfiles: append([]string(nil), req.GetRequestedProfiles()...),
	}
	if caps != nil {
		r.CPUCount = caps.GetCpuCount()
		r.RAMBytes = caps.GetRamBytes()
		r.DiskBytes = caps.GetDiskBytes()
	}
	return r
}
