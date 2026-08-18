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
	"github.com/globulario/services/golang/component_catalog"
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
	// A use is one node admission, not one attempt — see priorJoinAdmissionLocked.
	// A node already admitted under this token is retrying, so it is not charged
	// again and is not blocked by an exhausted budget it is already inside.
	identity := protoToStoredIdentity(req.GetIdentity())
	priorAdmission := srv.priorJoinAdmissionLocked(token, identity)
	if priorAdmission == nil && jt.Uses >= jt.MaxUses {
		srv.unlock()
		return nil, status.Error(codes.PermissionDenied, "token uses exhausted")
	}
	reqID := uuid.NewString()
	caps := req.GetCapabilities()

	// Declared placement intent. Validated here, at admission, so a typo is a
	// loud rejection rather than a node silently placed by hardware thresholds
	// instead of by what the operator asked for. Mirrors the v2 signed-JoinPlan
	// path (handlers_join_authorization.go) so both paths reject the same inputs.
	requestedProfiles := component_catalog.NormalizeProfiles(req.GetRequestedProfiles())
	if len(req.GetRequestedProfiles()) > 0 {
		if unknown := component_catalog.UnknownProfiles(req.GetRequestedProfiles()); len(unknown) > 0 {
			srv.unlock()
			return nil, status.Errorf(codes.InvalidArgument,
				"unknown requested_profiles %v (known profiles: %v)", unknown, component_catalog.ProfileNames())
		}
		if len(requestedProfiles) == 0 {
			srv.unlock()
			return nil, status.Error(codes.InvalidArgument,
				"requested_profiles resolved to no installable profiles")
		}
	}

	jr := &joinRequestRecord{
		RequestID:         reqID,
		Token:             token,
		Identity:          identity,
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
		// Auto-approve after preflight passes. Pass the node's DECLARED profiles:
		// nil here would discard them and fall through to hardware deduction, which
		// is how a node that asked for core,compute ended up placed as
		// control-plane,core,gateway,storage. Empty stays nil, so a node that
		// declares nothing still gets the deduced/default chain unchanged.
		srv.approveJoinRecordLocked(jr, requestedProfiles)
		// Charge the token only now, and only for a node this token has not
		// already admitted. A blocked preflight costs nothing: it admitted
		// nobody.
		if priorAdmission == nil {
			jt.Uses++
		}
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

// joinAdmissionKey is the canonical identity of a node ATTEMPTING to join:
// hostname plus its advertised IPs. It is deliberately the same shape the join
// preflight uses to detect conflicts, so "is this the same node coming back"
// and "does this node collide with an existing one" cannot drift apart.
//
// Returns "" when there is nothing stable to key on. An unkeyable request is
// treated as a brand-new admission — never as a retry — so an identity-less
// caller can never ride in on someone else's token use.
func joinAdmissionKey(id storedIdentity) string {
	host := strings.ToLower(strings.TrimSpace(id.Hostname))
	ips := make([]string, 0, len(id.Ips))
	seen := make(map[string]bool, len(id.Ips))
	for _, raw := range id.Ips {
		ip := strings.TrimSpace(raw)
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	if host == "" && len(ips) == 0 {
		return ""
	}
	return host + "|" + strings.Join(ips, ",")
}

// sameMachineIdentity reports whether two identities describe the same physical
// node: same hostname, and at least one routable IP in common.
//
// Requiring a shared IP is what keeps this from being a rename loophole — a
// second machine that merely claims an existing hostname shares no address with
// it and is still rejected.
func sameMachineIdentity(a, b storedIdentity) bool {
	ha := strings.ToLower(strings.TrimSpace(a.Hostname))
	hb := strings.ToLower(strings.TrimSpace(b.Hostname))
	if ha == "" || ha != hb {
		return false
	}
	for _, rawA := range a.Ips {
		ipA := strings.TrimSpace(rawA)
		if ipA == "" {
			continue
		}
		parsed := net.ParseIP(ipA)
		if parsed == nil || parsed.IsLoopback() {
			continue
		}
		for _, rawB := range b.Ips {
			if strings.TrimSpace(rawB) == ipA {
				return true
			}
		}
	}
	return false
}

// joinAdmissionStillLive reports whether a join request record still represents
// an admission the node may be retrying. Terminal outcomes (rejected, removed,
// or a node that already made it to active) do not.
func joinAdmissionStillLive(jr *joinRequestRecord) bool {
	phase := effectiveLifecyclePhase(jr)
	if phase.Terminal() {
		return false
	}
	// Removing is not terminal — the removal is still in flight — but a node on
	// its way out is not retrying a join, so it holds no admission open.
	return phase != JoinPhaseRemoving
}

// priorJoinAdmissionLocked finds an in-flight admission this token already
// granted to this same node, or nil.
//
// WHY THIS EXISTS — a token use is one NODE ADMISSION, not one HTTP request.
//
// Both join handlers used to charge a use the moment a token validated, before
// preflight and long before the node finished bootstrapping. A join that died
// partway — and phase [2.3] "Generating service certificate" dies whenever
// sign_ca_certificate answers non-2xx — had already spent it. The installer
// then retried, spent another, and a MaxUses=1 token was gone after the first
// failed attempt. cleanupJoinStateLocked then DELETED the exhausted token, so
// the next attempt did not even get "uses exhausted", it got "join token not
// found", and the node was permanently unjoinable — a full state wipe did not
// help, because the exhaustion lived on the controller.
//
// Charging on retry also breaks the security property it looks like it is
// protecting: MaxUses is meant to bound how many DISTINCT nodes a token admits.
// Counting attempts instead of nodes makes the bound depend on how flaky the
// network was, which is not a security boundary at all.
//
// So: the same node retrying continues the admission it already paid for, and a
// DIFFERENT node still cannot get in once the budget is spent.
//
// Must be called with the server lock held.
func (srv *server) priorJoinAdmissionLocked(token string, identity storedIdentity) *joinRequestRecord {
	key := joinAdmissionKey(identity)
	if key == "" {
		return nil
	}
	for _, jr := range srv.state.JoinRequests {
		if jr == nil || jr.Token != token {
			continue
		}
		if !joinAdmissionStillLive(jr) {
			continue
		}
		if joinAdmissionKey(jr.Identity) == key {
			return jr
		}
	}
	return nil
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
		// A node mid-join is not in conflict with ITSELF.
		//
		// This scan asks "does a DIFFERENT machine already hold this identity",
		// but it used to ask "does this hostname exist anywhere" — and once the
		// first authorization registers the node, the answer for that node's own
		// retry is yes. A join that died partway (the certificate phase, a
		// dropped connection, a reboot) could then never be retried: every
		// attempt was refused as a conflict with the record its own previous
		// attempt had just created.
		//
		// The exemption is deliberately narrow — it covers ONLY a node still
		// bootstrapping. An already-active member re-requesting a join is still
		// refused, because approveJoinRecordLocked would otherwise overwrite its
		// nodeState with a fresh converging/BootstrapAdmitted record and throw
		// away the placement generation and runtime progress it has earned. That
		// refusal is now harmless: a blocked preflight no longer charges a token
		// use, so a node looping against this costs nothing.
		//
		// Same hostname AND a shared routable IP is the same machine coming
		// back. Same hostname with NO shared IP is a genuine collision — two
		// machines claiming one name — and still fails, as does a shared IP
		// under a different hostname.
		if n.JoinLifecyclePhase == JoinPhaseBootstrapping && sameMachineIdentity(n.Identity, jr.Identity) {
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
	profileSource := nodeProfileSourceRequested
	if len(profiles) == 0 {
		profiles = jr.SuggestedProfiles
		profileSource = nodeProfileSourceDeduced
	}
	if len(profiles) == 0 {
		profiles = srv.cfg.DefaultProfiles
		profileSource = nodeProfileSourceDefault
	}
	profiles = normalizeProfiles(profiles)

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
		PlacementGeneration:   1, // D1c 1a: new node — established placement (bumped on later changes)
		LastSeen:              time.Now(),
		Status:                "converging",
		Metadata:              profileSourceMetadata(jr.Labels, profileSource),
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
		365*24*60, // 1 year TTL
		nodeID,    // audience = node ID
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
		if node.AgentEndpoint == "" {
			// Agent hasn't heartbeated yet — no address to dial. The first
			// heartbeat trigger in ReportNodeStatus fires once the endpoint
			// is reported; dialing "" only burns a trigger on a guaranteed
			// "passthrough: received empty target" failure.
			log.Printf("ApproveJoin: node %s approved but agent endpoint not yet reported — deferring join trigger to first heartbeat", nodeID)
		} else {
			node.BootstrapWorkflowActive = true
			agentEndpoint := node.AgentEndpoint
			log.Printf("ApproveJoin: triggering join workflow for %s at %s", nodeID, agentEndpoint)
			go srv.triggerJoinWorkflow(nodeID, agentEndpoint)
		}
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

// tokenHasLiveAdmissionLocked reports whether any node is still mid-join under
// this token. Must be called with the server lock held.
func (srv *server) tokenHasLiveAdmissionLocked(token string) bool {
	for _, jr := range srv.state.JoinRequests {
		if jr != nil && jr.Token == token && joinAdmissionStillLive(jr) {
			return true
		}
	}
	return false
}

func (srv *server) cleanupJoinStateLocked(now time.Time) bool {
	dirty := false
	for token, jt := range srv.state.JoinTokens {
		if jt.MaxUses > 0 && jt.Uses >= jt.MaxUses {
			// An exhausted token is dropped only once nothing is still joining
			// under it. Deleting it while a node is mid-join turns every retry
			// into "join token not found", which is worse than "uses
			// exhausted": it reads as operator error and hides that the node
			// already holds an admission. The token still expires on its own
			// clock below, so this delays reclamation, never prevents it.
			if srv.tokenHasLiveAdmissionLocked(token) {
				continue
			}
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
