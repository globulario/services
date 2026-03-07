package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/cluster_controller/cluster_controller_server/operator"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/netutil"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/store"
	"github.com/globulario/services/golang/plan/versionutil"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	agentIdleTimeoutDefault  = 5 * time.Minute
	agentCleanupInterval     = 1 * time.Minute
	operationCleanupInterval = 1 * time.Minute
	operationTimeout         = 10 * time.Minute
	joinRequestRetention     = 72 * time.Hour
	pendingJoinRetention     = 7 * 24 * time.Hour
	statePersistInterval     = 5 * time.Second
	statusGracePeriod        = 2 * time.Minute
	planPollInterval         = 3 * time.Second
	upgradePlanTTL           = 10 * time.Minute
	defaultProbePort         = 80
	defaultBinaryPath        = "/usr/local/bin/globular"
	defaultTargetPublisher   = "globular"
	defaultTargetName        = "globular"
	upgradeDiskMinBytes      = 1 << 30
	repositoryAddressEnv     = "REPOSITORY_ADDRESS"

	// Health monitoring constants
	healthCheckInterval     = 30 * time.Second // How often to check node health
	unhealthyThreshold      = 2 * time.Minute  // Time without contact before marking unhealthy
	recoveryAttemptInterval = 5 * time.Minute  // How often to attempt recovery
	maxRecoveryAttempts     = 3                // Max recovery attempts before giving up
)

func serviceUnitName(name string) string {
	if strings.HasSuffix(strings.ToLower(name), ".service") {
		return name
	}
	return fmt.Sprintf("%s.service", name)
}

// filterVersionsForNode returns all desired services with canonical names.
// It includes services whose unit does not yet exist on the node so that
// desired-hash computation and health counts reflect the full desired state.
func filterVersionsForNode(desired map[string]string, node *nodeState) map[string]string {
	out := make(map[string]string)
	if len(desired) == 0 || node == nil {
		return out
	}
	for svc, ver := range desired {
		norm := canonicalServiceName(svc)
		out[norm] = ver
	}
	return out
}

// computeServiceDelta returns desiredActionable (desired services that need
// install or upgrade — includes services whose unit does not yet exist on the
// node) and toRemove (services present on node but not desired).
func computeServiceDelta(desiredCanon map[string]string, units []unitStatusRecord) (map[string]string, []string) {
	desiredActionable := make(map[string]string)
	for svc, ver := range desiredCanon {
		desiredActionable[svc] = ver
	}
	toRemove := make([]string, 0)
	for _, u := range units {
		unitName := strings.ToLower(strings.TrimSpace(u.Name))
		canon := ""
		if strings.HasPrefix(unitName, "globular-") && strings.HasSuffix(unitName, ".service") {
			canon = canonicalServiceName(unitName)
		} else if unitName == "envoy.service" {
			canon = "envoy"
		}
		if canon == "" {
			continue
		}
		if _, ok := desiredCanon[canon]; !ok {
			toRemove = append(toRemove, canon)
		}
	}
	return desiredActionable, toRemove
}

func toWatchEvent(typ string, evt resourcestore.Event) *cluster_controllerpb.WatchEvent {
	we := &cluster_controllerpb.WatchEvent{
		EventType:       evt.Type,
		ResourceVersion: evt.ResourceVersion,
	}
	switch typ {
	case "ClusterNetwork":
		if obj, ok := evt.Object.(*cluster_controllerpb.ClusterNetwork); ok {
			we.ClusterNetwork = obj
		}
	case "ServiceDesiredVersion":
		if obj, ok := evt.Object.(*cluster_controllerpb.ServiceDesiredVersion); ok {
			we.ServiceDesiredVersion = obj
		}
	case "ServiceRelease":
		if obj, ok := evt.Object.(*cluster_controllerpb.ServiceRelease); ok {
			we.ServiceRelease = obj
		}
	}
	return we
}

type kvClient interface {
	Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error)
	Put(ctx context.Context, key, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error)
}

func extractKV(ps store.PlanStore) kvClient {
	if eps, ok := ps.(*store.EtcdPlanStore); ok && eps != nil {
		return eps.Client()
	}
	return nil
}

type server struct {
	cluster_controllerpb.UnimplementedClusterControllerServiceServer

	cfg                  *clusterControllerConfig
	cfgPath              string
	statePath            string
	state                *controllerState
	mu                   sync.Mutex
	muHeldSince          atomic.Int64
	muHeldBy             atomic.Value
	planStore            store.PlanStore
	kv                   kvClient
	agentMu              sync.Mutex
	agentClients         map[string]*agentClient
	agentInsecure        bool
	agentIdleTimeout     time.Duration
	agentCAPath          string
	lastStateSave        time.Time
	agentServerName      string
	opMu                 sync.Mutex
	operations           map[string]*operationState
	watchMu              sync.Mutex
	watchers             map[*operationWatcher]struct{}
	serviceBlock         map[string]time.Time
	enableServiceRemoval bool
	leader               atomic.Bool
	leaderID             atomic.Value
	leaderAddr           atomic.Value
	reconcileRunning     atomic.Bool
	resources            resourcestore.Store
	etcdClient           *clientv3.Client
	// releaseEnqueue is set by startControllerRuntime so that ReportNodeStatus can
	// trigger release re-evaluation when a node's AppliedServicesHash changes.
	releaseEnqueue func(releaseName string)
	// enqueueReconcile is set by startControllerRuntime so that SetNodeProfiles
	// can immediately trigger a reconcile cycle after saving profile changes.
	enqueueReconcile func()

	// event publishing (fire-and-forget, nil-safe)
	eventClient *event_client.Event_Client

	// test seams
	testHasActivePlanWithLock func(context.Context, string, string) bool
	testDispatchReleasePlan   func(context.Context, *cluster_controllerpb.ServiceRelease, string) (*planpb.NodePlan, error)
}

var testHookBeforeReportNodeStatusApply func()

func newServer(cfg *clusterControllerConfig, cfgPath, statePath string, state *controllerState, planStore store.PlanStore) *server {
	if state == nil {
		state = newControllerState()
	}
	if statePath == "" {
		statePath = defaultClusterStatePath
	}
	agentCAPath := strings.TrimSpace(os.Getenv("CLUSTER_AGENT_CA"))
	serverName := strings.TrimSpace(os.Getenv("CLUSTER_AGENT_SERVER_NAME"))
	srv := &server{
		cfg:              cfg,
		cfgPath:          cfgPath,
		statePath:        statePath,
		state:            state,
		planStore:        planStore,
		kv:               extractKV(planStore),
		agentClients:     make(map[string]*agentClient),
		serviceBlock:     make(map[string]time.Time),
		agentInsecure:    strings.EqualFold(os.Getenv("CLUSTER_INSECURE_AGENT_GRPC"), "true"),
		agentIdleTimeout: agentIdleTimeoutDefault,
		agentCAPath:      agentCAPath,
		agentServerName:  serverName,
		operations:       make(map[string]*operationState),
		watchers:         make(map[*operationWatcher]struct{}),
	}
	if strings.EqualFold(os.Getenv("ENABLE_SERVICE_REMOVAL"), "true") {
		srv.enableServiceRemoval = true
	}

	// Connect to EventService for reconciliation event publishing.
	eventAddr := strings.TrimSpace(os.Getenv("CLUSTER_EVENT_SERVICE_ADDR"))
	if eventAddr == "" {
		eventAddr = "localhost:10050"
	}
	if ec, err := event_client.NewEventService_Client(eventAddr, "event.EventService"); err == nil {
		srv.eventClient = ec
	} else {
		log.Printf("cluster-controller: event client unavailable: %v", err)
	}

	srv.setLeader(false, "", "")

	// Register built-in operators
	nodesFn := func() []string {
		srv.lock("nodes:snapshot")
		defer srv.unlock()
		ids := make([]string, 0, len(srv.state.Nodes))
		for id := range srv.state.Nodes {
			ids = append(ids, id)
		}
		return ids
	}
	operator.Register("etcd", operator.NewEtcdOperator(planStore, nodesFn))
	operator.Register("minio", operator.NewMinioOperator(planStore, nodesFn))
	operator.Register("scylla", operator.NewScyllaOperator(planStore, nodesFn))

	safeGo("mu-watchdog", func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			since := srv.muHeldSince.Load()
			if since == 0 {
				continue
			}
			heldFor := time.Since(time.Unix(0, since))
			if heldFor < 3*time.Second {
				continue
			}
			tag, _ := srv.muHeldBy.Load().(string)
			log.Printf("[WARN] srv.mu held for %s by %q; dumping goroutines", heldFor, tag)
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, true)
			log.Printf("[WARN] goroutine dump:\n%s", string(buf[:n]))
		}
	})

	return srv
}

// emitClusterEvent publishes an event to the EventService (fire-and-forget).
// Safe to call when eventClient is nil.
func (srv *server) emitClusterEvent(name string, payload map[string]interface{}) {
	if srv.eventClient == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	go func() {
		if err := srv.eventClient.Publish(name, data); err != nil {
			log.Printf("cluster-controller: publish %q failed: %v", name, err)
		}
	}()
}

func (srv *server) lock(tag string) {
	srv.mu.Lock()
	srv.muHeldBy.Store(tag)
	srv.muHeldSince.Store(time.Now().UnixNano())
}

func (srv *server) unlock() {
	srv.muHeldSince.Store(0)
	srv.muHeldBy.Store("")
	srv.mu.Unlock()
}

func safeGo(tag string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("panic in %s: %v\n%s", tag, r, debug.Stack())
			}
		}()
		fn()
	}()
}

func (srv *server) GetClusterInfo(ctx context.Context, req *timestamppb.Timestamp) (*cluster_controllerpb.ClusterInfo, error) {
	created := srv.state.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	domain := srv.cfg.ClusterDomain
	if domain == "" {
		domain = netutil.DefaultClusterDomain()
	}
	clusterID := srv.state.ClusterId
	if clusterID == "" {
		clusterID = domain
	}
	info := &cluster_controllerpb.ClusterInfo{
		ClusterDomain: domain,
		ClusterId:     clusterID,
		CreatedAt:     timestamppb.New(created),
	}
	return info, nil
}

func (srv *server) CreateJoinToken(ctx context.Context, req *cluster_controllerpb.CreateJoinTokenRequest) (*cluster_controllerpb.CreateJoinTokenResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	srv.lock("CreateJoinToken")
	defer srv.unlock()
	token := uuid.NewString()
	expiresAt := time.Now().Add(24 * time.Hour)
	if req != nil && req.ExpiresAt != nil {
		expiresAt = req.ExpiresAt.AsTime()
	}
	srv.state.JoinTokens[token] = &joinTokenRecord{
		Token:     token,
		ExpiresAt: expiresAt,
		MaxUses:   1,
	}
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist token: %v", err)
	}
	return &cluster_controllerpb.CreateJoinTokenResponse{
		JoinToken: token,
		ExpiresAt: timestamppb.New(expiresAt),
	}, nil
}

func (srv *server) RequestJoin(ctx context.Context, req *cluster_controllerpb.RequestJoinRequest) (*cluster_controllerpb.RequestJoinResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil || req.GetJoinToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "join_token is required")
	}
	token := strings.TrimSpace(req.GetJoinToken())
	srv.lock("unknown")
	defer srv.unlock()
	jt := srv.state.JoinTokens[token]
	if jt == nil {
		return nil, status.Error(codes.NotFound, "join token not found")
	}
	if time.Now().After(jt.ExpiresAt) {
		return nil, status.Error(codes.PermissionDenied, "token expired")
	}
	if jt.Uses >= jt.MaxUses {
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
		Capabilities:      capsToStored(caps),
		SuggestedProfiles: deduceProfiles(caps),
	}
	srv.state.JoinRequests[reqID] = jr
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist join request: %v", err)
	}
	return &cluster_controllerpb.RequestJoinResponse{
		RequestId: reqID,
		Status:    "pending",
		Message:   "pending approval",
	}, nil
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
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
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
	jr.Status = "approved"
	rawProfiles := req.GetProfiles()
	if len(rawProfiles) == 0 {
		rawProfiles = srv.cfg.DefaultProfiles
	}
	profiles := normalizeProfiles(rawProfiles)
	jr.Profiles = profiles
	nodeID := uuid.NewString()
	jr.AssignedNodeID = nodeID

	// Create new node with current network generation
	node := &nodeState{
		NodeID:                nodeID,
		Identity:              jr.Identity,
		Profiles:              profiles,
		LastSeen:              time.Now(),
		Status:                "converging",
		Metadata:              copyLabels(jr.Labels),
		LastAppliedGeneration: 0, // New node hasn't applied any generation yet
	}
	srv.state.Nodes[nodeID] = node

	if err := srv.persistStateLocked(true); err != nil {
		srv.unlock()
		return nil, status.Errorf(codes.Internal, "persist node state: %v", err)
	}

	// Immediately dispatch initial plan with network config if node has endpoint
	// Note: New nodes won't have endpoint yet, so reconciliation loop will pick this up
	// when the node first reports status with its agent endpoint
	srv.unlock()
	// NOTE: lock is released here; remaining code below does not need the lock.

	return &cluster_controllerpb.ApproveJoinResponse{
		NodeId:  nodeID,
		Message: "approved; node will receive configuration on first heartbeat",
	}, nil
}

func (srv *server) RejectJoin(ctx context.Context, req *cluster_controllerpb.RejectJoinRequest) (*cluster_controllerpb.RejectJoinResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
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
	jr.Reason = req.GetReason()
	if err := srv.persistStateLocked(true); err != nil {
		return nil, status.Errorf(codes.Internal, "persist join request: %v", err)
	}
	return &cluster_controllerpb.RejectJoinResponse{
		NodeId:  jr.AssignedNodeID,
		Message: "rejected",
	}, nil
}

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

// PreviewNodeProfiles computes what WOULD happen if the given profiles were assigned
// to the node, without mutating any state. Useful for dry-run before applying.
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

// buildConfigDiff produces a sorted list of ConfigFileDiff entries comparing old and new hashes.
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

// drainNode sends stop commands to the node agent to gracefully stop all services.
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

func (srv *server) GetClusterHealth(ctx context.Context, req *cluster_controllerpb.GetClusterHealthRequest) (*cluster_controllerpb.GetClusterHealthResponse, error) {
	srv.lock("cluster-health")
	defer srv.unlock()

	resp := &cluster_controllerpb.GetClusterHealthResponse{
		TotalNodes: int32(len(srv.state.Nodes)),
	}

	now := time.Now()
	healthyThreshold := 2 * time.Minute // Node is healthy if seen within this time

	for _, node := range srv.state.Nodes {
		nodeHealth := &cluster_controllerpb.NodeHealthStatus{
			NodeId:    node.NodeID,
			Hostname:  node.Identity.Hostname,
			LastError: node.LastError,
			LastSeen:  timestamppb.New(node.LastSeen),
		}

		// Determine node health status
		timeSinceSeen := now.Sub(node.LastSeen)
		isHealthy := (node.Status == "healthy" || node.Status == "ready" || node.Status == "converging")
		switch {
		case isHealthy && timeSinceSeen < healthyThreshold:
			nodeHealth.Status = "healthy"
			resp.HealthyNodes++
		case node.Status == "unhealthy" || node.Status == "degraded" || node.LastError != "":
			nodeHealth.Status = "unhealthy"
			nodeHealth.FailedChecks = 1
			if node.LastError != "" {
				nodeHealth.LastError = node.LastError
			}
			resp.UnhealthyNodes++
		case timeSinceSeen >= healthyThreshold:
			nodeHealth.Status = "unknown"
			nodeHealth.LastError = fmt.Sprintf("not seen for %v", timeSinceSeen.Round(time.Second))
			resp.UnknownNodes++
		default:
			nodeHealth.Status = "unknown"
			resp.UnknownNodes++
		}

		resp.NodeHealth = append(resp.NodeHealth, nodeHealth)
	}

	// Determine overall cluster status
	switch {
	case resp.TotalNodes == 0:
		resp.Status = "unhealthy"
	case resp.UnhealthyNodes == 0 && resp.UnknownNodes == 0:
		resp.Status = "healthy"
	case resp.HealthyNodes > 0:
		resp.Status = "degraded"
	default:
		resp.Status = "unhealthy"
	}

	return resp, nil
}

func (srv *server) GetNodePlan(ctx context.Context, req *cluster_controllerpb.GetNodePlanRequest) (*cluster_controllerpb.GetNodePlanResponse, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	srv.lock("unknown")
	defer srv.unlock()
	node := srv.state.Nodes[req.GetNodeId()]
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	plan, err := srv.computeNodePlan(node)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "compute plan: %v", err)
	}
	return &cluster_controllerpb.GetNodePlanResponse{
		Plan: plan,
	}, nil
}

func (srv *server) UpdateClusterNetwork(ctx context.Context, req *cluster_controllerpb.UpdateClusterNetworkRequest) (*cluster_controllerpb.UpdateClusterNetworkResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil || req.GetSpec() == nil {
		return nil, status.Error(codes.InvalidArgument, "spec is required")
	}
	spec := req.GetSpec()
	domain := strings.TrimSpace(spec.GetClusterDomain())
	if domain == "" {
		return nil, status.Error(codes.InvalidArgument, "cluster_domain is required")
	}
	spec.ClusterDomain = domain

	protocol := strings.ToLower(strings.TrimSpace(spec.GetProtocol()))
	if protocol == "" {
		protocol = "http"
	}
	if protocol != "http" && protocol != "https" {
		return nil, status.Error(codes.InvalidArgument, "protocol must be http or https")
	}
	spec.Protocol = protocol

	if protocol == "http" && spec.GetPortHttp() == 0 {
		spec.PortHttp = 80
	}
	if protocol == "https" && spec.GetPortHttps() == 0 {
		spec.PortHttps = 443
	}

	if spec.GetAcmeEnabled() && strings.TrimSpace(spec.GetAdminEmail()) == "" {
		return nil, status.Error(codes.InvalidArgument, "admin_email is required when acme_enabled is true")
	}

	spec.AdminEmail = strings.TrimSpace(spec.GetAdminEmail())
	spec.AlternateDomains = normalizeDomains(spec.GetAlternateDomains())

	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	applied, err := srv.resources.Apply(ctx, "ClusterNetwork", &cluster_controllerpb.ClusterNetwork{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "default"},
		Spec: spec,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "apply desired network: %v", err)
	}
	gen := uint64(0)
	if cn, ok := applied.(*cluster_controllerpb.ClusterNetwork); ok && cn.Meta != nil {
		gen = uint64(cn.Meta.Generation)
	}
	return &cluster_controllerpb.UpdateClusterNetworkResponse{
		Generation: gen,
	}, nil
}

func (srv *server) reconcileNetworkPlans(ctx context.Context, spec *cluster_controllerpb.ClusterNetworkSpec) {
	if spec == nil || srv.planStore == nil {
		return
	}
	srv.lock("reconcileNetworkPlans:snapshot")
	clusterID := srv.state.ClusterId
	nodes := make([]string, 0, len(srv.state.Nodes))
	for id := range srv.state.Nodes {
		nodes = append(nodes, id)
	}
	srv.unlock()

	desired := ClusterDesiredState{
		Network:         spec,
		ServiceVersions: map[string]string{},
	}

	for _, nodeID := range nodes {
		obsUnits := srv.observedUnitsForNode(nodeID)
		plan, err := BuildNetworkTransitionPlan(nodeID, desired, NodeObservedState{Units: obsUnits})
		if err != nil {
			log.Printf("reconcile network plan for node %s failed: %v", nodeID, err)
			continue
		}
		plan.PlanId = uuid.NewString()
		plan.ClusterId = clusterID
		plan.NodeId = nodeID
		plan.Generation = srv.nextPlanGeneration(ctx, nodeID)
		plan.IssuedBy = "cluster-controller"
		if plan.GetCreatedUnixMs() == 0 {
			plan.CreatedUnixMs = uint64(time.Now().UnixMilli())
		}
		if err := srv.planStore.PutCurrentPlan(ctx, nodeID, plan); err != nil {
			log.Printf("persist plan for node %s: %v", nodeID, err)
			continue
		}
		if appendable, ok := srv.planStore.(interface {
			AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
		}); ok {
			_ = appendable.AppendHistory(ctx, nodeID, plan)
		}
		log.Printf("reconcile: wrote network plan node=%s plan_id=%s gen=%d", nodeID, plan.GetPlanId(), plan.GetGeneration())
	}
}

func (srv *server) ApplyNodePlan(ctx context.Context, req *cluster_controllerpb.ApplyNodePlanRequest) (*cluster_controllerpb.ApplyNodePlanResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.GetNodeId()) == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())
	srv.lock("unknown")
	node := srv.state.Nodes[nodeID]
	srv.unlock()
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	if node.AgentEndpoint == "" {
		return nil, status.Error(codes.FailedPrecondition, "agent endpoint unknown")
	}
	plan, planErr := srv.computeNodePlan(node)
	if planErr != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "compute plan: %v", planErr)
	}
	if plan == nil {
		return nil, status.Error(codes.FailedPrecondition, "plan is empty")
	}
	if len(plan.GetUnitActions()) == 0 && len(plan.GetRenderedConfig()) == 0 {
		return nil, status.Error(codes.FailedPrecondition, "plan has no changes")
	}
	hash := planHash(plan)
	if hash == "" {
		return nil, status.Error(codes.FailedPrecondition, "plan has no changes")
	}

	opID := uuid.NewString()
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_QUEUED, "plan queued", 0, false, ""))
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "plan running", 5, false, ""))
	if err := srv.dispatchPlan(ctx, node, plan, opID); err != nil {
		log.Printf("node %s apply dispatch failed: %v", nodeID, err)
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "plan failed", 0, true, err.Error()))
		return nil, status.Errorf(codes.Internal, "dispatch plan: %v", err)
	}
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "plan dispatched to node-agent", 25, false, ""))
	// Phase 4b: store pending rendered config hashes on dispatch.
	// These will be promoted to RenderedConfigHashes only when the agent reports apply success.
	if len(plan.GetRenderedConfig()) > 0 {
		srv.lock("rendered-config-hashes")
		if n := srv.state.Nodes[nodeID]; n != nil {
			n.PendingRenderedConfigHashes = HashRenderedConfigs(plan.GetRenderedConfig())
		}
		srv.unlock()
	}
	if srv.recordPlanSent(nodeID, hash) {
		srv.lock("apply-node-plan:persist")
		func() {
			defer srv.unlock()
			if err := srv.persistStateLocked(true); err != nil {
				log.Printf("persist state after ApplyNodePlan: %v", err)
			}
		}()
	}

	return &cluster_controllerpb.ApplyNodePlanResponse{
		OperationId: opID,
	}, nil
}

// ApplyNodePlanV1 submits and executes an arbitrary V1 NodePlan.
func (srv *server) ApplyNodePlanV1(ctx context.Context, req *cluster_controllerpb.ApplyNodePlanV1Request) (*cluster_controllerpb.ApplyNodePlanV1Response, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}

	// Validation
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())
	if nodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	if req.GetPlan() == nil {
		return nil, status.Error(codes.InvalidArgument, "plan is required")
	}
	plan := req.GetPlan()

	// Validate plan node_id matches request node_id
	planNodeID := strings.TrimSpace(plan.GetNodeId())
	if planNodeID != "" && planNodeID != nodeID {
		return nil, status.Errorf(codes.InvalidArgument, "plan.node_id %q does not match request node_id %q", planNodeID, nodeID)
	}
	// If plan.node_id is empty, set it to request node_id
	if planNodeID == "" {
		plan.NodeId = nodeID
	}

	// Validate steps exist
	if plan.GetSpec() == nil || len(plan.GetSpec().GetSteps()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "plan must have at least one step")
	}

	// Verify node exists and has agent endpoint
	srv.lock("apply-plan-v1")
	node := srv.state.Nodes[nodeID]
	srv.unlock()
	if node == nil {
		return nil, status.Errorf(codes.NotFound, "node %q not found", nodeID)
	}
	if node.AgentEndpoint == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "node %q has no agent endpoint", nodeID)
	}

	// Create operation ID
	opID := uuid.NewString()

	// Persist plan to disk (optional but recommended)
	if err := srv.persistPlanV1(nodeID, opID, plan); err != nil {
		log.Printf("warning: failed to persist plan for node %s operation %s: %v", nodeID, opID, err)
	}

	// Broadcast initial operation events
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_QUEUED, "plan received and validated", 0, false, ""))
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "dispatching plan to node-agent", 5, false, ""))

	// Dispatch plan to node agent
	client, err := srv.getAgentClient(ctx, node.AgentEndpoint)
	if err != nil {
		log.Printf("node %s: failed to get agent client: %v", nodeID, err)
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "failed to connect to node-agent", 0, true, err.Error()))
		return nil, status.Errorf(codes.Internal, "get agent client: %v", err)
	}

	if err := client.ApplyPlanV1(ctx, plan, opID); err != nil {
		log.Printf("node %s: apply plan v1 dispatch failed: %v", nodeID, err)
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "plan dispatch failed", 0, true, err.Error()))
		return nil, status.Errorf(codes.Internal, "dispatch plan: %v", err)
	}

	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "plan dispatched to node-agent", 25, false, ""))

	return &cluster_controllerpb.ApplyNodePlanV1Response{
		OperationId: opID,
	}, nil
}

// persistPlanV1 writes the plan to /var/lib/globular/plans/<nodeID>/<operationID>.json
func (srv *server) persistPlanV1(nodeID, operationID string, plan *planpb.NodePlan) error {
	plansRoot := "/var/lib/globular/plans"
	nodeDir := filepath.Join(plansRoot, nodeID)

	// Create node directory with 0700 permissions
	if err := os.MkdirAll(nodeDir, 0700); err != nil {
		return fmt.Errorf("create plans directory: %w", err)
	}

	// Marshal plan to JSON
	planJSON, err := protojson.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}

	// Write to temp file and rename (atomic write)
	planFile := filepath.Join(nodeDir, operationID+".json")
	tempFile := planFile + ".tmp"

	if err := os.WriteFile(tempFile, planJSON, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tempFile, planFile); err != nil {
		os.Remove(tempFile) // Clean up temp file on error
		return fmt.Errorf("rename temp file: %w", err)
	}

	return nil
}

func (srv *server) CompleteOperation(ctx context.Context, req *cluster_controllerpb.CompleteOperationRequest) (*cluster_controllerpb.CompleteOperationResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	opID := strings.TrimSpace(req.GetOperationId())
	if opID == "" {
		return nil, status.Error(codes.InvalidArgument, "operation_id is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())
	if nodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	phase := cluster_controllerpb.OperationPhase_OP_SUCCEEDED
	if !req.GetSuccess() {
		phase = cluster_controllerpb.OperationPhase_OP_FAILED
	}
	message := strings.TrimSpace(req.GetMessage())
	if message == "" {
		if phase == cluster_controllerpb.OperationPhase_OP_SUCCEEDED {
			message = "plan applied"
		} else {
			message = "plan failed"
		}
	}
	percent := req.GetPercent()
	if percent == 0 && phase == cluster_controllerpb.OperationPhase_OP_SUCCEEDED {
		percent = 100
	}
	errMsg := strings.TrimSpace(req.GetError())
	evt := srv.newOperationEvent(opID, nodeID, phase, message, percent, true, errMsg)
	srv.broadcastOperationEvent(evt)
	return &cluster_controllerpb.CompleteOperationResponse{
		Message: fmt.Sprintf("operation %s completion recorded", opID),
	}, nil
}

func (srv *server) UpgradeGlobular(ctx context.Context, req *cluster_controllerpb.UpgradeGlobularRequest) (*cluster_controllerpb.UpgradeGlobularResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if len(req.GetArtifact()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "artifact is required")
	}
	platform := strings.TrimSpace(req.GetPlatform())
	if platform == "" {
		return nil, status.Error(codes.InvalidArgument, "platform is required")
	}
	nodeID := strings.TrimSpace(req.GetNodeId())
	if nodeID == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	srv.lock("unknown")
	node := srv.state.Nodes[nodeID]
	srv.unlock()
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	if srv.planStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "plan store unavailable")
	}

	sha := strings.TrimSpace(req.GetSha256())
	if sha == "" {
		hash := sha256.Sum256(req.GetArtifact())
		sha = hex.EncodeToString(hash[:])
	} else {
		sha = strings.ToLower(sha)
	}

	planID := uuid.NewString()
	ref := &repositorypb.ArtifactRef{
		PublisherId: defaultTargetPublisher,
		Name:        defaultTargetName,
		Version:     planID,
		Platform:    platform,
		Kind:        repositorypb.ArtifactKind_SUBSYSTEM,
	}
	if err := uploadArtifact(ctx, ref, req.GetArtifact()); err != nil {
		return nil, status.Errorf(codes.Internal, "stage artifact: %v", err)
	}

	targetPath := strings.TrimSpace(req.GetTargetPath())
	if targetPath == "" {
		targetPath = os.Getenv("GLOBULAR_BINARY_PATH")
	}
	if targetPath == "" {
		targetPath = defaultBinaryPath
	}
	if targetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "target_path unavailable")
	}

	minPath := filepath.Dir(targetPath)
	fetchDest := filepath.Join(os.TempDir(), "globular-upgrade", planID, filepath.Base(targetPath))

	generation := srv.nextPlanGeneration(ctx, nodeID)
	expires := time.Now().Add(upgradePlanTTL)
	plan := buildUpgradePlan(planID, nodeID, srv.state.ClusterId, generation, expires, targetPath, fetchDest, ref, sha, req.GetProbePort(), minPath)

	if err := srv.planStore.PutCurrentPlan(ctx, nodeID, plan); err != nil {
		return nil, status.Errorf(codes.Internal, "persist plan: %v", err)
	}
	if appendable, ok := srv.planStore.(interface {
		AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
	}); ok {
		_ = appendable.AppendHistory(ctx, nodeID, plan)
	}

	opID := uuid.NewString()
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_QUEUED, "upgrade queued", 0, false, ""))
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "plan dispatched", 10, false, ""))

	status, err := srv.waitForPlanStatus(ctx, nodeID, planID, expires)
	if err != nil {
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "plan failed", 100, true, err.Error()))
		return nil, err
	}

	phase := cluster_controllerpb.OperationPhase_OP_SUCCEEDED
	msg := "plan succeeded"
	done := true
	errMsg := ""
	if status.GetState() != planpb.PlanState_PLAN_SUCCEEDED {
		phase = cluster_controllerpb.OperationPhase_OP_FAILED
		msg = "plan completed with error"
		errMsg = status.GetErrorMessage()
	}
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, nodeID, phase, msg, 100, done, errMsg))

	return &cluster_controllerpb.UpgradeGlobularResponse{
		PlanId:        planID,
		Generation:    generation,
		TerminalState: planStateName(status.GetState()),
		ErrorStepId:   status.GetErrorStepId(),
		ErrorMessage:  status.GetErrorMessage(),
	}, nil
}

func (srv *server) ReportNodeStatus(ctx context.Context, req *cluster_controllerpb.ReportNodeStatusRequest) (*cluster_controllerpb.ReportNodeStatusResponse, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil || req.GetStatus() == nil || req.GetStatus().GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "status.node_id is required")
	}
	nodeStatus := req.GetStatus()
	ns := nodeStatus
	nodeID := strings.TrimSpace(ns.GetNodeId())
	newIdentity := protoToStoredIdentity(ns.GetIdentity())
	newEndpoint := strings.TrimSpace(ns.GetAgentEndpoint())
	reportedAt := time.Now()
	if ts := ns.GetReportedAt(); ts != nil {
		reportedAt = ts.AsTime()
	}
	rawUnits := protoUnitsToStored(ns.GetUnits())
	units := normalizedUnits(rawUnits)
	lastError := ns.GetLastError()
	appliedSvcHash := strings.ToLower(strings.TrimSpace(ns.GetAppliedServicesHash()))
	installedVersions := ns.GetInstalledVersions()
	installedUnitFiles := ns.GetInstalledUnitFiles()
	inventoryComplete := ns.GetInventoryComplete()

	// Snapshot existing node for evaluation without holding the lock during compute.
	srv.lock("ReportNodeStatus:snapshot")
	node := srv.state.Nodes[nodeID]
	if node == nil {
		srv.unlock()
		return nil, status.Error(codes.NotFound, "node not found")
	}
	nodeSnapshot := *node
	srv.unlock()

	healthStatus, reason := srv.evaluateNodeStatus(&nodeSnapshot, units)
	if lastError == "" && reason != "" && healthStatus != "ready" {
		lastError = reason
	}

	if testHookBeforeReportNodeStatusApply != nil {
		testHookBeforeReportNodeStatusApply()
	}

	srv.lock("ReportNodeStatus:commit")
	defer srv.unlock()
	node = srv.state.Nodes[nodeID]
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	changed := false

	if !identitiesEqual(node.Identity, newIdentity) {
		changed = true
	}
	node.Identity = newIdentity

	oldEndpoint := node.AgentEndpoint
	node.AgentEndpoint = newEndpoint
	node.ReportedAt = reportedAt
	node.LastSeen = reportedAt

	if !unitsEqual(node.Units, units) {
		node.Units = units
		changed = true
	}
	if node.Status != healthStatus {
		node.Status = healthStatus
		changed = true
	}
	if node.LastError != lastError {
		node.LastError = lastError
		changed = true
	}
	hashChanged := node.AppliedServicesHash != appliedSvcHash
	if hashChanged {
		node.AppliedServicesHash = appliedSvcHash
		changed = true
	}
	// Persist the node-reported inventory hash under the observed key.
	// This is distinct from applied_hash_services which is only set by the
	// reconciler when convergence with the desired state is confirmed.
	if appliedSvcHash != "" && inventoryComplete {
		if err := srv.putNodeObservedServiceHash(ctx, nodeID, appliedSvcHash); err != nil {
			log.Printf("ReportNodeStatus: store observed service hash for %s: %v", nodeID, err)
		}
	}
	// Update installed versions when the node reports inventory, even if empty
	// (inventoryComplete=true means the node has finished scanning).
	if len(installedVersions) > 0 || inventoryComplete {
		if !mapsEqual(node.InstalledVersions, installedVersions) {
			node.InstalledVersions = installedVersions
			changed = true
		}
	}
	// Store hardware capabilities if reported.
	if caps := nodeStatus.GetCapabilities(); caps != nil {
		node.Capabilities = capsToStored(caps)
	}
	// Phase 3: store installed unit file inventory and inventory_complete flag.
	if inventoryComplete || len(installedUnitFiles) > 0 {
		// Merge the reported unit files into the node's unit list as "inactive" records
		// so that missingInstalledUnits can find them. Only add entries not already present.
		unitMap := make(map[string]string, len(node.Units))
		for _, u := range node.Units {
			unitMap[strings.ToLower(u.Name)] = u.State
		}
		for _, uf := range installedUnitFiles {
			name := strings.ToLower(strings.TrimSpace(uf))
			if name == "" {
				continue
			}
			if _, exists := unitMap[name]; !exists {
				node.Units = append(node.Units, unitStatusRecord{Name: uf, State: "inactive"})
				unitMap[name] = "inactive"
				changed = true
			}
		}
		if node.InventoryComplete != inventoryComplete {
			node.InventoryComplete = inventoryComplete
			changed = true
		}
	}
	if oldEndpoint != newEndpoint {
		changed = true
	}

	// Phase 4b: commit or discard pending rendered config hashes based on apply outcome.
	// A report received after the plan was dispatched is our confirmation signal.
	if len(node.PendingRenderedConfigHashes) > 0 && !node.LastPlanSentAt.IsZero() &&
		reportedAt.After(node.LastPlanSentAt) {
		if healthStatus == "ready" {
			// Agent is healthy after plan dispatch — config files are on disk.
			node.RenderedConfigHashes = node.PendingRenderedConfigHashes
			node.PendingRenderedConfigHashes = nil
			changed = true
		} else if healthStatus == "error" || healthStatus == "failed" {
			// Agent explicitly failed — clear pending so next cycle retries.
			node.PendingRenderedConfigHashes = nil
			changed = true
		}
		// For other states (converging, etc.) keep pending and wait.
	}
	endpointToClose := ""
	if oldEndpoint != "" && oldEndpoint != newEndpoint {
		endpointToClose = oldEndpoint
	}
	if changed {
		if err := srv.persistStateLocked(false); err != nil {
			return nil, status.Errorf(codes.Internal, "persist node status: %v", err)
		}
	}

	// When the applied services hash changes, re-enqueue any ServiceReleases that
	// include this node so drift detection can re-evaluate and potentially recover
	// DEGRADED releases without waiting for the next spec change.
	if hashChanged && srv.releaseEnqueue != nil && srv.resources != nil {
		enqueue := srv.releaseEnqueue
		resources := srv.resources
		nID := nodeID
		go func() {
			items, _, err := resources.List(context.Background(), "ServiceRelease", "")
			if err != nil {
				return
			}
			for _, obj := range items {
				rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
				if !ok || rel.Meta == nil {
					continue
				}
				if rel.Status == nil {
					continue
				}
				for _, nrs := range rel.Status.Nodes {
					if nrs != nil && nrs.NodeID == nID {
						enqueue(rel.Meta.Name)
						break
					}
				}
			}
		}()
	}

	if endpointToClose != "" {
		srv.closeAgentClient(endpointToClose)
	}
	return &cluster_controllerpb.ReportNodeStatusResponse{
		Message: "status recorded",
	}, nil
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
		Status:   jr.Status,
		NodeId:   jr.AssignedNodeID,
		Message:  jr.Reason,
		Profiles: append([]string(nil), jr.Profiles...),
	}, nil
}

func (srv *server) GetClusterHealthV1(ctx context.Context, _ *cluster_controllerpb.GetClusterHealthV1Request) (*cluster_controllerpb.GetClusterHealthV1Response, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	if srv.planStore == nil || srv.kv == nil {
		return nil, status.Error(codes.FailedPrecondition, "plan store or kv unavailable")
	}
	desiredNetObj, err := srv.loadDesiredNetwork(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load desired network: %v", err)
	}
	specHash := ""
	if desiredNetObj != nil && desiredNetObj.Spec != nil {
		hash, _ := hashDesiredNetwork(&cluster_controllerpb.DesiredNetwork{
			Domain:           desiredNetObj.Spec.GetClusterDomain(),
			Protocol:         desiredNetObj.Spec.GetProtocol(),
			PortHttp:         desiredNetObj.Spec.GetPortHttp(),
			PortHttps:        desiredNetObj.Spec.GetPortHttps(),
			AlternateDomains: append([]string(nil), desiredNetObj.Spec.GetAlternateDomains()...),
			AcmeEnabled:      desiredNetObj.Spec.GetAcmeEnabled(),
			AdminEmail:       desiredNetObj.Spec.GetAdminEmail(),
		})
		specHash = hash
	}
	desiredCanon, _, err := srv.loadDesiredServices(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load desired services: %v", err)
	}
	srv.lock("health:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, n := range srv.state.Nodes {
		nodes = append(nodes, n)
	}
	srv.unlock()

	var nodeHealths []*cluster_controllerpb.NodeHealth
	serviceCounts := make(map[string]int)
	serviceAtDesired := make(map[string]int)
	serviceUpgrading := make(map[string]int)

	for _, node := range nodes {
		if node == nil {
			continue
		}
		appliedNet, _ := srv.getNodeAppliedHash(ctx, node.NodeID)
		filtered := filterVersionsForNode(desiredCanon, node)
		desiredSvcHash := stableServiceDesiredHash(filtered)
		appliedSvcHash, _ := srv.getNodeAppliedServiceHash(ctx, node.NodeID)
		plan, _ := srv.planStore.GetCurrentPlan(ctx, node.NodeID)
		status, _ := srv.planStore.GetStatus(ctx, node.NodeID)
		phase := ""
		if status != nil {
			phase = status.GetState().String()
		}
		lastErr := ""
		if status != nil {
			lastErr = status.GetErrorMessage()
		}
		// Determine whether the node can perform privileged operations.
		canPriv := false
		if node.Capabilities != nil {
			canPriv = node.Capabilities.CanApplyPrivileged
		}

		// Only show PLAN_AWAITING_PRIVILEGED_APPLY when at least one desired
		// service is genuinely missing or at the wrong version AND the node
		// cannot self-apply. Previously this compared hashes alone, which
		// caused an impossible-to-resolve loop when the node had extra
		// unmanaged services (extras inflate the inventory hash but
		// apply-desired can never remove them when enableServiceRemoval=false).
		if desiredSvcHash != "" && !canPriv {
			hasMissing := false
			for svc, desiredVer := range filtered {
				installedVer := ""
				for k, v := range node.InstalledVersions {
					parts := strings.SplitN(k, "/", 2)
					candidate := k
					if len(parts) == 2 {
						candidate = parts[1]
					}
					if canonicalServiceName(candidate) == canonicalServiceName(svc) {
						installedVer = v
						break
					}
				}
				if installedVer != desiredVer {
					hasMissing = true
					break
				}
			}
			if hasMissing {
				isActive := status != nil &&
					(status.GetState() == planpb.PlanState_PLAN_RUNNING ||
						status.GetState() == planpb.PlanState_PLAN_ROLLING_BACK)
				if !isActive {
					phase = planpb.PlanState_PLAN_AWAITING_PRIVILEGED_APPLY.String()
				}
			}
		}

		nodeHealths = append(nodeHealths, &cluster_controllerpb.NodeHealth{
			NodeId:              node.NodeID,
			DesiredNetworkHash:  specHash,
			AppliedNetworkHash:  appliedNet,
			DesiredServicesHash: desiredSvcHash,
			AppliedServicesHash: appliedSvcHash,
			CurrentPlanId: func() string {
				if plan != nil {
					return plan.GetPlanId()
				} else {
					return ""
				}
			}(),
			CurrentPlanGeneration: func() uint64 {
				if plan != nil {
					return plan.GetGeneration()
				} else {
					return 0
				}
			}(),
			CurrentPlanPhase:    phase,
			LastError:           lastErr,
			CanApplyPrivileged:  canPriv,
			InstalledVersions:   node.InstalledVersions,
		})

		for svc, desiredVer := range filtered {
			serviceCounts[svc]++
			// Per-service convergence: compare installed version against
			// the desired version for THIS service, not the global hash.
			if installedVer, ok := node.InstalledVersions[svc]; ok && installedVer == desiredVer {
				serviceAtDesired[svc]++
			}
			if status != nil && plan != nil && plan.GetDesiredHash() != "" && plan.GetDesiredHash() == desiredSvcHash {
				if status.GetState() == planpb.PlanState_PLAN_RUNNING || status.GetState() == planpb.PlanState_PLAN_PENDING {
					serviceUpgrading[svc]++
				}
			}
		}
	}

	var summaries []*cluster_controllerpb.ServiceSummary
	for svc, ver := range desiredCanon {
		total := int32(serviceCounts[svc])
		at := int32(serviceAtDesired[svc])
		up := int32(serviceUpgrading[svc])
		summaries = append(summaries, &cluster_controllerpb.ServiceSummary{
			ServiceName:    svc,
			DesiredVersion: ver,
			NodesAtDesired: at,
			NodesTotal:     total,
			Upgrading:      up,
		})
	}

	return &cluster_controllerpb.GetClusterHealthV1Response{
		Nodes:    nodeHealths,
		Services: summaries,
	}, nil
}

func (srv *server) GetNodeHealthDetailV1(ctx context.Context, req *cluster_controllerpb.GetNodeHealthDetailV1Request) (*cluster_controllerpb.GetNodeHealthDetailV1Response, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	nodeID := req.GetNodeId()

	srv.lock("health-detail:snapshot")
	node := srv.state.Nodes[nodeID]
	srv.unlock()

	if node == nil {
		return nil, status.Errorf(codes.NotFound, "node %q not found", nodeID)
	}

	var checks []*cluster_controllerpb.NodeHealthCheck

	// 1. Heartbeat check
	heartbeatOK := !node.LastSeen.IsZero() && time.Since(node.LastSeen) < unhealthyThreshold
	hbReason := ""
	if !heartbeatOK {
		if node.LastSeen.IsZero() {
			hbReason = "never seen"
		} else {
			hbReason = fmt.Sprintf("last seen %s ago", time.Since(node.LastSeen).Truncate(time.Second))
		}
	}
	checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
		Subsystem: "heartbeat",
		Ok:        heartbeatOK,
		Reason:    hbReason,
	})

	// 2. Unit checks — compare required units from plan vs reported unit states
	plan, _ := srv.computeNodePlan(node)
	required := requiredUnitsFromPlan(plan)
	unitStates := make(map[string]string, len(node.Units))
	for _, u := range node.Units {
		if u.Name != "" {
			unitStates[strings.ToLower(u.Name)] = strings.ToLower(u.State)
		}
	}
	for unit := range required {
		unitOK := false
		reason := ""
		st, found := unitStates[strings.ToLower(unit)]
		if !found {
			reason = "unit not reported by node"
		} else if st != "active" {
			reason = fmt.Sprintf("state is %q", st)
		} else {
			unitOK = true
		}
		checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
			Subsystem: "unit:" + unit,
			Ok:        unitOK,
			Reason:    reason,
		})
	}

	// 3. Inventory check
	checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
		Subsystem: "inventory",
		Ok:        node.InventoryComplete,
		Reason: func() string {
			if !node.InventoryComplete {
				return "inventory scan not yet complete"
			}
			return ""
		}(),
	})

	// 4. Version checks — compare installed vs desired
	desiredCanon, _, _ := srv.loadDesiredServices(ctx)
	filtered := filterVersionsForNode(desiredCanon, node)
	for svc, desiredVer := range filtered {
		installedVer, found := node.InstalledVersions[svc]
		ok := found && installedVer == desiredVer
		reason := ""
		if !found {
			reason = fmt.Sprintf("not installed (desired %s)", desiredVer)
		} else if installedVer != desiredVer {
			reason = fmt.Sprintf("installed %s, desired %s", installedVer, desiredVer)
		}
		checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
			Subsystem: "version:" + svc,
			Ok:        ok,
			Reason:    reason,
		})
	}

	// Overall status from existing evaluator, overridden to unhealthy if heartbeat fails.
	overallStatus, _ := srv.evaluateNodeStatus(node, node.Units)
	if !heartbeatOK {
		overallStatus = "unhealthy"
	}
	allOK := true
	for _, c := range checks {
		if !c.Ok {
			allOK = false
			break
		}
	}

	canPriv := false
	if node.Capabilities != nil {
		canPriv = node.Capabilities.CanApplyPrivileged
	}

	return &cluster_controllerpb.GetNodeHealthDetailV1Response{
		NodeId:             nodeID,
		OverallStatus:      overallStatus,
		Healthy:            allOK,
		Checks:             checks,
		LastError:          node.LastError,
		CanApplyPrivileged: canPriv,
		InventoryComplete:  node.InventoryComplete,
		LastSeen:           timestamppb.New(node.LastSeen),
	}, nil
}

// ResourcesService implementation
func (srv *server) ApplyClusterNetwork(ctx context.Context, req *cluster_controllerpb.ApplyClusterNetworkRequest) (*cluster_controllerpb.ClusterNetwork, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if req == nil || req.Object == nil || req.Object.Spec == nil || strings.TrimSpace(req.Object.Spec.ClusterDomain) == "" {
		return nil, status.Error(codes.InvalidArgument, "cluster_network.spec.cluster_domain is required")
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	obj := req.Object
	if obj.Meta == nil {
		obj.Meta = &cluster_controllerpb.ObjectMeta{}
	}
	obj.Meta.Name = "default"
	applied, err := srv.resources.Apply(ctx, "ClusterNetwork", obj)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "apply cluster network: %v", err)
	}
	return applied.(*cluster_controllerpb.ClusterNetwork), nil
}

func (srv *server) GetClusterNetwork(ctx context.Context, _ *cluster_controllerpb.GetClusterNetworkRequest) (*cluster_controllerpb.ClusterNetwork, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	obj, _, err := srv.resources.Get(ctx, "ClusterNetwork", "default")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get cluster network: %v", err)
	}
	if obj == nil {
		return nil, status.Error(codes.NotFound, "cluster network not found")
	}
	return obj.(*cluster_controllerpb.ClusterNetwork), nil
}

func (srv *server) ApplyServiceDesiredVersion(ctx context.Context, req *cluster_controllerpb.ApplyServiceDesiredVersionRequest) (*cluster_controllerpb.ServiceDesiredVersion, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	if req == nil || req.Object == nil || req.Object.Spec == nil || strings.TrimSpace(req.Object.Spec.ServiceName) == "" || strings.TrimSpace(req.Object.Spec.Version) == "" {
		return nil, status.Error(codes.InvalidArgument, "service_name and version are required")
	}
	canon := canonicalServiceName(req.Object.Spec.ServiceName)
	if canon == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid service_name")
	}
	obj := req.Object
	if obj.Meta == nil {
		obj.Meta = &cluster_controllerpb.ObjectMeta{}
	}
	obj.Meta.Name = canon
	obj.Spec.ServiceName = canon
	applied, err := srv.resources.Apply(ctx, "ServiceDesiredVersion", obj)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "apply service desired version: %v", err)
	}
	return applied.(*cluster_controllerpb.ServiceDesiredVersion), nil
}

func (srv *server) DeleteServiceDesiredVersion(ctx context.Context, req *cluster_controllerpb.DeleteServiceDesiredVersionRequest) (*emptypb.Empty, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	name := canonicalServiceName(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if err := srv.resources.Delete(ctx, "ServiceDesiredVersion", name); err != nil {
		return nil, status.Errorf(codes.Internal, "delete service desired version: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (srv *server) ListServiceDesiredVersions(ctx context.Context, _ *cluster_controllerpb.ListServiceDesiredVersionsRequest) (*cluster_controllerpb.ListServiceDesiredVersionsResponse, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list service desired versions: %v", err)
	}
	out := &cluster_controllerpb.ListServiceDesiredVersionsResponse{}
	for _, obj := range items {
		out.Items = append(out.Items, obj.(*cluster_controllerpb.ServiceDesiredVersion))
	}
	return out, nil
}

func (srv *server) ApplyServiceRelease(ctx context.Context, req *cluster_controllerpb.ApplyServiceReleaseRequest) (*cluster_controllerpb.ServiceRelease, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	obj := req.GetObject()
	if obj == nil || obj.Spec == nil {
		return nil, status.Error(codes.InvalidArgument, "object and spec are required")
	}
	if strings.TrimSpace(obj.Spec.PublisherID) == "" || strings.TrimSpace(obj.Spec.ServiceName) == "" {
		return nil, status.Error(codes.InvalidArgument, "spec.publisher_id and spec.service_name are required")
	}
	if obj.Meta == nil {
		obj.Meta = &cluster_controllerpb.ObjectMeta{}
	}
	// Canonical name: publisher/service to keep it unique across publishers.
	if obj.Meta.Name == "" {
		obj.Meta.Name = obj.Spec.PublisherID + "/" + canonicalServiceName(obj.Spec.ServiceName)
	}
	applied, err := srv.resources.Apply(ctx, "ServiceRelease", obj)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "apply service release: %v", err)
	}
	return applied.(*cluster_controllerpb.ServiceRelease), nil
}

func (srv *server) GetServiceRelease(ctx context.Context, req *cluster_controllerpb.GetServiceReleaseRequest) (*cluster_controllerpb.ServiceRelease, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	obj, _, err := srv.resources.Get(ctx, "ServiceRelease", name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get service release: %v", err)
	}
	if obj == nil {
		return nil, status.Errorf(codes.NotFound, "service release %q not found", name)
	}
	return obj.(*cluster_controllerpb.ServiceRelease), nil
}

func (srv *server) ListServiceReleases(ctx context.Context, _ *cluster_controllerpb.ListServiceReleasesRequest) (*cluster_controllerpb.ListServiceReleasesResponse, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	items, _, err := srv.resources.List(ctx, "ServiceRelease", "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list service releases: %v", err)
	}
	out := &cluster_controllerpb.ListServiceReleasesResponse{}
	for _, obj := range items {
		out.Items = append(out.Items, obj.(*cluster_controllerpb.ServiceRelease))
	}
	return out, nil
}

func (srv *server) DeleteServiceRelease(ctx context.Context, req *cluster_controllerpb.DeleteServiceReleaseRequest) (*emptypb.Empty, error) {
	if err := srv.requireLeader(ctx); err != nil {
		return nil, err
	}
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if err := srv.resources.Delete(ctx, "ServiceRelease", name); err != nil {
		return nil, status.Errorf(codes.Internal, "delete service release: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (srv *server) Watch(req *cluster_controllerpb.WatchRequest, stream cluster_controllerpb.ResourcesService_WatchServer) error {
	if srv.resources == nil {
		return status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	if req == nil {
		return status.Error(codes.InvalidArgument, "request required")
	}
	ch, err := srv.resources.Watch(stream.Context(), req.GetType(), req.GetPrefix(), req.GetFromResourceVersion())
	if err != nil {
		return status.Errorf(codes.Internal, "watch: %v", err)
	}
	if req.GetIncludeExisting() {
		items, rv, err := srv.resources.List(stream.Context(), req.GetType(), req.GetPrefix())
		if err == nil {
			for _, obj := range items {
				evt := resourcestore.Event{Type: resourcestore.EventAdded, ResourceVersion: rv, Object: obj}
				if err := stream.Send(toWatchEvent(req.GetType(), evt)); err != nil {
					return err
				}
			}
		}
	}
	for evt := range ch {
		if err := stream.Send(toWatchEvent(req.GetType(), evt)); err != nil {
			return err
		}
	}
	return nil
}
func (srv *server) startReconcileLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	safeGo("reconcile-loop", func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.isLeader() {
					continue
				}
				srv.reconcileNodes(ctx)
			}
		}
	})
}

func (srv *server) startAgentCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(agentCleanupInterval)
	safeGo("agent-cleanup", func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.isLeader() {
					continue
				}
				srv.cleanupAgentClients()
			}
		}
	})
}

func (srv *server) startOperationCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(operationCleanupInterval)
	safeGo("operation-cleanup", func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.isLeader() {
					continue
				}
				srv.cleanupTimedOutOperations()
			}
		}
	})
}

// startHealthMonitorLoop runs periodic health checks and attempts recovery for unhealthy nodes.
func (srv *server) startHealthMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(healthCheckInterval)
	safeGo("health-monitor", func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.isLeader() {
					continue
				}
				srv.monitorNodeHealth(ctx)
			}
		}
	})
}

func (srv *server) isLeader() bool {
	return srv.leader.Load()
}

func (srv *server) setLeader(isLeader bool, id, addr string) {
	srv.leader.Store(isLeader)
	srv.leaderID.Store(id)
	srv.leaderAddr.Store(addr)
}

func (srv *server) requireLeader(ctx context.Context) error {
	if srv.isLeader() {
		return nil
	}
	addr, _ := srv.leaderAddr.Load().(string)
	if addr == "" && srv.kv != nil {
		if resp, err := srv.kv.Get(ctx, leaderElectionPrefix+"/addr"); err == nil && resp != nil && len(resp.Kvs) > 0 {
			addr = string(resp.Kvs[0].Value)
			srv.leaderAddr.Store(addr)
		}
	}
	return status.Errorf(codes.FailedPrecondition, "not leader (leader_addr=%s)", addr)
}

// monitorNodeHealth checks all nodes and attempts recovery for unhealthy ones.
func (srv *server) monitorNodeHealth(ctx context.Context) {
	now := time.Now()

	srv.lock("health-monitor:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, node := range srv.state.Nodes {
		nodes = append(nodes, node)
	}
	srv.unlock()

	var stateDirty bool

	for _, node := range nodes {
		timeSinceSeen := now.Sub(node.LastSeen)

		srv.lock("health-monitor:check")
		currentNode := srv.state.Nodes[node.NodeID]
		if currentNode == nil {
			srv.unlock()
			continue
		}

		previousStatus := currentNode.Status

		// Check if node is unhealthy
		if timeSinceSeen > unhealthyThreshold {
			currentNode.FailedHealthChecks++

			if currentNode.Status != "unhealthy" {
				currentNode.Status = "unhealthy"
				currentNode.MarkedUnhealthySince = now
				currentNode.LastError = fmt.Sprintf("no contact for %v", timeSinceSeen.Round(time.Second))
				log.Printf("node %s marked unhealthy: %s", node.NodeID, currentNode.LastError)
				stateDirty = true
			}

			// Attempt recovery if needed
			shouldRecover := currentNode.RecoveryAttempts < maxRecoveryAttempts &&
				(currentNode.LastRecoveryAttempt.IsZero() || now.Sub(currentNode.LastRecoveryAttempt) > recoveryAttemptInterval)

			if shouldRecover && node.AgentEndpoint != "" {
				currentNode.LastRecoveryAttempt = now
				currentNode.RecoveryAttempts++
				stateDirty = true
				log.Printf("attempting recovery for node %s (attempt %d/%d)", node.NodeID, currentNode.RecoveryAttempts, maxRecoveryAttempts)
				srv.unlock()

				// Attempt to reconnect and redispatch plan
				if err := srv.attemptNodeRecovery(ctx, node); err != nil {
					log.Printf("recovery attempt for node %s failed: %v", node.NodeID, err)
					srv.lock("health-monitor:recovery-failed")
					if n := srv.state.Nodes[node.NodeID]; n != nil {
						n.LastError = fmt.Sprintf("recovery failed: %v", err)
					}
					srv.unlock()
				} else {
					log.Printf("recovery attempt for node %s initiated successfully", node.NodeID)
				}
				continue
			}
		} else if currentNode.Status == "unhealthy" && previousStatus == "unhealthy" {
			// Node came back online - reset recovery counters
			currentNode.Status = "healthy"
			currentNode.FailedHealthChecks = 0
			currentNode.RecoveryAttempts = 0
			currentNode.MarkedUnhealthySince = time.Time{}
			currentNode.LastError = ""
			log.Printf("node %s recovered and marked healthy", node.NodeID)
			stateDirty = true
		}
		srv.unlock()
	}

	if stateDirty {
		srv.lock("health-monitor:persist")
		func() {
			defer srv.unlock()
			if err := srv.persistStateLocked(true); err != nil {
				log.Printf("health monitor: persist state: %v", err)
			}
		}()
	}
}

// attemptNodeRecovery tries to reconnect to a node and redispatch its plan.
func (srv *server) attemptNodeRecovery(ctx context.Context, node *nodeState) error {
	if node.AgentEndpoint == "" {
		return fmt.Errorf("no agent endpoint for node %s", node.NodeID)
	}

	// Close any existing connection to force reconnection
	srv.closeAgentClient(node.AgentEndpoint)

	// Get fresh agent client
	client, err := srv.getAgentClient(ctx, node.AgentEndpoint)
	if err != nil {
		return fmt.Errorf("connect to agent: %w", err)
	}

	// Try to get inventory to verify connectivity
	_, err = client.GetInventory(ctx)
	if err != nil {
		return fmt.Errorf("get inventory: %w", err)
	}

	// If we can connect, dispatch the current plan
	plan, planErr := srv.computeNodePlan(node)
	if planErr != nil {
		return fmt.Errorf("compute plan: %w", planErr)
	}
	if plan == nil || (len(plan.GetUnitActions()) == 0 && len(plan.GetRenderedConfig()) == 0) {
		// No plan needed, just mark as recovered
		return nil
	}

	opID := uuid.NewString()
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_QUEUED, "recovery: plan queued", 0, false, ""))

	if err := srv.dispatchPlan(ctx, node, plan, opID); err != nil {
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "recovery: plan failed", 0, true, err.Error()))
		return fmt.Errorf("dispatch plan: %w", err)
	}

	// Phase 4b: store pending rendered config hashes on recovery dispatch.
	// Promoted to RenderedConfigHashes only after agent reports apply success.
	if len(plan.GetRenderedConfig()) > 0 {
		srv.lock("recovery-rendered-config-hashes")
		node.PendingRenderedConfigHashes = HashRenderedConfigs(plan.GetRenderedConfig())
		srv.unlock()
	}
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "recovery: plan dispatched", 25, false, ""))
	return nil
}

func (srv *server) observedUnitsForNode(nodeID string) []unitStatusRecord {
	srv.lock("observedUnitsForNode")
	defer srv.unlock()
	node := srv.state.Nodes[nodeID]
	if node == nil {
		return nil
	}
	return append([]unitStatusRecord(nil), node.Units...)
}

func (srv *server) reconcileNodes(ctx context.Context) {
	if !srv.reconcileRunning.CompareAndSwap(false, true) {
		return
	}
	defer srv.reconcileRunning.Store(false)
	if srv.planStore == nil || srv.kv == nil {
		return
	}
	desiredNetworkObj, err := srv.loadDesiredNetwork(ctx)
	if err != nil {
		log.Printf("reconcile: load desired network failed: %v", err)
	}
	var desiredNet *cluster_controllerpb.DesiredNetwork
	specHash := ""
	if desiredNetworkObj != nil {
		desiredNet = &cluster_controllerpb.DesiredNetwork{
			Domain:           desiredNetworkObj.Spec.GetClusterDomain(),
			Protocol:         desiredNetworkObj.Spec.GetProtocol(),
			PortHttp:         desiredNetworkObj.Spec.GetPortHttp(),
			PortHttps:        desiredNetworkObj.Spec.GetPortHttps(),
			AlternateDomains: append([]string(nil), desiredNetworkObj.Spec.GetAlternateDomains()...),
			AcmeEnabled:      desiredNetworkObj.Spec.GetAcmeEnabled(),
			AdminEmail:       desiredNetworkObj.Spec.GetAdminEmail(),
		}
		if h, herr := hashDesiredNetwork(desiredNet); herr == nil {
			specHash = h
		} else {
			log.Printf("reconcile: hash desired network: %v", herr)
		}
	}
	srv.lock("reconcile:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, node := range srv.state.Nodes {
		nodes = append(nodes, node)
	}
	stateDirty := srv.cleanupJoinStateLocked(time.Now())
	srv.unlock()
	now := time.Now()

	// (RECONCILE_CYCLE removed — not in required event set)

	for _, node := range nodes {
		if node == nil || node.NodeID == "" {
			continue
		}
		// Validate profiles before any dispatch — unknown profiles block the node.
		actions, profileErr := buildPlanActions(node.Profiles)
		if profileErr != nil {
			node.Status = "blocked"
			node.LastPlanError = profileErr.Error()
			node.BlockedReason = "unknown_profile"
			node.BlockedDetails = profileErr.Error()
			stateDirty = true
			log.Printf("reconcile: node %s blocked: %v", node.NodeID, profileErr)
			srv.emitClusterEvent("plan_blocked", map[string]interface{}{
				"severity":       "WARN",
				"node_id":        node.NodeID,
				"hostname":       node.Identity.Hostname,
				"message":        fmt.Sprintf("Node %s blocked: unknown profile", node.Identity.Hostname),
				"correlation_id": fmt.Sprintf("plan:%s:gen:0", node.NodeID),
			})
			continue
		}
		// Clear stale unknown_profile block now that profiles are valid.
		if node.BlockedReason == "unknown_profile" {
			node.BlockedReason = ""
			node.BlockedDetails = ""
			node.LastPlanError = ""
			if node.Status == "blocked" {
				node.Status = "converging"
			}
			stateDirty = true
		}

		// Phase 3: Capability gating — desired units must be installed on the node.
		// Hard-gate when inventory_complete=true; soft-gate (warn only) otherwise.
		if len(node.Units) > 0 {
			desiredUnitNames := desiredUnitsFromActions(actions)
			if missing := missingInstalledUnits(desiredUnitNames, node.Units); len(missing) > 0 {
				if node.InventoryComplete {
					// Full inventory reported — hard block.
					node.Status = "blocked"
					node.LastPlanError = fmt.Sprintf("missing unit files: %v", missing)
					node.BlockedReason = "missing_units"
					node.BlockedDetails = fmt.Sprintf("missing: %s", strings.Join(missing, ", "))
					stateDirty = true
					log.Printf("reconcile: node %s blocked (hard): missing units: %v", node.NodeID, missing)
					srv.emitClusterEvent("plan_blocked", map[string]interface{}{
						"severity":       "WARN",
						"node_id":        node.NodeID,
						"hostname":       node.Identity.Hostname,
						"message":        fmt.Sprintf("Node %s blocked: missing unit files %v", node.Identity.Hostname, missing),
						"correlation_id": fmt.Sprintf("plan:%s:gen:0", node.NodeID),
					})
					continue
				}
				// Inventory not complete — soft mode: warn but allow reconcile to proceed.
				log.Printf("reconcile: node %s soft-warn: possibly missing units (inventory incomplete): %v", node.NodeID, missing)
			} else if node.InventoryComplete {
				// Full inventory present and all units confirmed — clear stale missing_units block.
				if node.BlockedReason == "missing_units" {
					node.BlockedReason = ""
					node.BlockedDetails = ""
					node.LastPlanError = ""
					if node.Status == "blocked" {
						node.Status = "converging"
					}
					stateDirty = true
				}
			}
		}

		// Phase 4: Privileged-apply gating — when the node lacks privilege to
		// write systemd units, skip plan dispatch and record the state so the
		// UI can show "Awaiting privileged apply".
		canPriv := node.Capabilities != nil && node.Capabilities.CanApplyPrivileged
		if !canPriv {
			existingStatus, _ := srv.planStore.GetStatus(ctx, node.NodeID)
			alreadyAwaiting := existingStatus != nil &&
				existingStatus.GetState() == planpb.PlanState_PLAN_AWAITING_PRIVILEGED_APPLY
			if !alreadyAwaiting {
				log.Printf("reconcile: node %s lacks privileged-apply capability, skipping plan dispatch", node.NodeID)
			}
		}

		appliedHash, err := srv.getNodeAppliedHash(ctx, node.NodeID)
		if err != nil {
			log.Printf("reconcile: read applied hash for %s: %v", node.NodeID, err)
			continue
		}
		status, _ := srv.planStore.GetStatus(ctx, node.NodeID)
		currentPlan, _ := srv.planStore.GetCurrentPlan(ctx, node.NodeID)
		meta, _ := srv.getNodePlanMeta(ctx, node.NodeID)
		planHash := ""
		lastEmitMs := int64(0)
		if currentPlan != nil {
			planHash = currentPlan.GetDesiredHash()
			if currentPlan.GetCreatedUnixMs() > 0 {
				lastEmitMs = int64(currentPlan.GetCreatedUnixMs())
			}
		}
		if planHash == "" && meta != nil {
			planHash = meta.DesiredHash
		}
		if lastEmitMs == 0 && meta != nil {
			lastEmitMs = meta.LastEmit
		}
		if specHash != "" && appliedHash != specHash {
			if status != nil && (status.GetState() == planpb.PlanState_PLAN_RUNNING || status.GetState() == planpb.PlanState_PLAN_PENDING) {
				if planHash == specHash && currentPlan != nil && status.GetPlanId() == currentPlan.GetPlanId() && status.GetGeneration() == currentPlan.GetGeneration() {
					if !isPlanStuck(status, lastEmitMs, now) {
						continue
					}
				}
			}
			if status != nil && status.GetState() == planpb.PlanState_PLAN_SUCCEEDED {
				if planHash == specHash && currentPlan != nil && status.GetPlanId() == currentPlan.GetPlanId() && status.GetGeneration() == currentPlan.GetGeneration() {
					if err := srv.putNodeAppliedHash(ctx, node.NodeID, specHash); err != nil {
						log.Printf("reconcile: store applied hash for %s: %v", node.NodeID, err)
					}
					if desiredNetworkObj != nil && desiredNetworkObj.Meta != nil && srv.resources != nil {
						_, _ = srv.resources.UpdateStatus(ctx, "ClusterNetwork", "default", &cluster_controllerpb.ObjectStatus{
							ObservedGeneration: desiredNetworkObj.Meta.Generation,
						})
					}
					_ = srv.putNodeFailureCount(ctx, node.NodeID, 0)
					srv.emitClusterEvent("plan_apply_succeeded", map[string]interface{}{
						"severity":       "INFO",
						"node_id":        node.NodeID,
						"hostname":       node.Identity.Hostname,
						"message":        fmt.Sprintf("Network plan succeeded for %s", node.Identity.Hostname),
						"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, currentPlan.GetGeneration()),
					})
					continue
				}
			}
			fails, _ := srv.getNodeFailureCount(ctx, node.NodeID)
			if status != nil && planHash == specHash && (status.GetState() == planpb.PlanState_PLAN_FAILED || status.GetState() == planpb.PlanState_PLAN_ROLLED_BACK || status.GetState() == planpb.PlanState_PLAN_EXPIRED) {
				srv.emitClusterEvent("plan_apply_failed", map[string]interface{}{
					"severity":       "ERROR",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"message":        fmt.Sprintf("Network plan failed for %s (state=%s)", node.Identity.Hostname, status.GetState()),
					"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, status.GetGeneration()),
				})
				delay := backoffDuration(fails)
				if lastEmitMs > 0 && now.Sub(time.UnixMilli(lastEmitMs)) < delay {
					continue
				}
			}

			spec := desiredNetworkToSpec(desiredNet)
			if spec == nil {
				continue
			}
			plan, err := BuildNetworkTransitionPlan(node.NodeID, ClusterDesiredState{
				Network: spec,
			}, NodeObservedState{Units: node.Units})
			if err != nil {
				log.Printf("reconcile: build plan for %s failed: %v", node.NodeID, err)
				continue
			}
			plan.PlanId = uuid.NewString()
			plan.ClusterId = srv.state.ClusterId
			plan.NodeId = node.NodeID
			plan.Generation = srv.nextPlanGeneration(ctx, node.NodeID)
			plan.DesiredHash = specHash
			if plan.GetCreatedUnixMs() == 0 {
				plan.CreatedUnixMs = uint64(now.UnixMilli())
			}
			plan.IssuedBy = "cluster-controller"

			// Skip dispatch if node lacks privileged-apply capability.
			if !canPriv {
				log.Printf("reconcile: node %s needs privileged apply for network plan (plan_id=%s)", node.NodeID, plan.GetPlanId())
				srv.emitClusterEvent("plan_blocked_privileged", map[string]interface{}{
					"severity":       "WARN",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"message":        fmt.Sprintf("Node %s cannot apply privileged operations. Run: globular services apply-desired", node.Identity.Hostname),
					"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, plan.GetGeneration()),
				})
				continue
			}

			if err := srv.planStore.PutCurrentPlan(ctx, node.NodeID, plan); err != nil {
				log.Printf("reconcile: persist plan for %s: %v", node.NodeID, err)
				continue
			}
			if appendable, ok := srv.planStore.(interface {
				AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
			}); ok {
				_ = appendable.AppendHistory(ctx, node.NodeID, plan)
			}
			newMeta := &planMeta{PlanId: plan.GetPlanId(), Generation: plan.GetGeneration(), DesiredHash: specHash, LastEmit: now.UnixMilli()}
			_ = srv.putNodePlanMeta(ctx, node.NodeID, newMeta)
			if status != nil && (status.GetState() == planpb.PlanState_PLAN_FAILED || status.GetState() == planpb.PlanState_PLAN_ROLLED_BACK || status.GetState() == planpb.PlanState_PLAN_EXPIRED) {
				_ = srv.putNodeFailureCount(ctx, node.NodeID, fails+1)
			}
			log.Printf("reconcile: wrote network plan node=%s plan_id=%s gen=%d", node.NodeID, plan.GetPlanId(), plan.GetGeneration())
			srv.emitClusterEvent("plan_generated", map[string]interface{}{
				"severity":       "INFO",
				"node_id":        node.NodeID,
				"hostname":       node.Identity.Hostname,
				"message":        fmt.Sprintf("Network plan generated for %s", node.Identity.Hostname),
				"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, plan.GetGeneration()),
			})
			srv.emitClusterEvent("plan_apply_started", map[string]interface{}{
				"severity":       "INFO",
				"node_id":        node.NodeID,
				"hostname":       node.Identity.Hostname,
				"message":        fmt.Sprintf("Network plan dispatched for %s", node.Identity.Hostname),
				"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, plan.GetGeneration()),
			})
			continue
		}

		// Services reconciliation
		desiredCanon, desiredObjs, err := srv.loadDesiredServices(ctx)
		if err != nil {
			log.Printf("reconcile: load desired services failed: %v", err)
			desiredCanon = map[string]string{}
		}
		filtered, toRemove := computeServiceDelta(desiredCanon, node.Units)
		svcHash := stableServiceDesiredHash(filtered)
		if srv.enableServiceRemoval && len(toRemove) > 0 {
			sort.Strings(toRemove)
			remSvc := toRemove[0]
			if status != nil && (status.GetState() == planpb.PlanState_PLAN_RUNNING || status.GetState() == planpb.PlanState_PLAN_PENDING) && planHash == svcHash {
				continue
			}
			if status != nil && status.GetState() == planpb.PlanState_PLAN_SUCCEEDED && planHash == svcHash {
				if err := srv.putNodeAppliedServiceHash(ctx, node.NodeID, svcHash); err != nil {
					log.Printf("reconcile: store applied service hash for %s: %v", node.NodeID, err)
				}
				if srv.resources != nil {
					if obj, ok := desiredObjs[remSvc]; ok && obj != nil && obj.Meta != nil {
						_, _ = srv.resources.UpdateStatus(ctx, "ServiceDesiredVersion", obj.Meta.Name, &cluster_controllerpb.ObjectStatus{
							ObservedGeneration: obj.Meta.Generation,
						})
					}
				}
				srv.emitClusterEvent("service_apply_succeeded", map[string]interface{}{
					"severity":       "INFO",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"service":        remSvc,
					"message":        fmt.Sprintf("Service removal succeeded for %s on %s", remSvc, node.Identity.Hostname),
					"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, status.GetGeneration()),
				})
				continue
			}
			rmPlan := BuildServiceRemovePlan(node.NodeID, remSvc, svcHash)
			rmPlan.PlanId = uuid.NewString()
			rmPlan.ClusterId = srv.state.ClusterId
			rmPlan.NodeId = node.NodeID
			rmPlan.Generation = srv.nextPlanGeneration(ctx, node.NodeID)
			rmPlan.DesiredHash = svcHash
			if rmPlan.GetCreatedUnixMs() == 0 {
				rmPlan.CreatedUnixMs = uint64(now.UnixMilli())
			}
			rmPlan.IssuedBy = "cluster-controller"
			if err := srv.planStore.PutCurrentPlan(ctx, node.NodeID, rmPlan); err == nil {
				if appendable, ok := srv.planStore.(interface {
					AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
				}); ok {
					_ = appendable.AppendHistory(ctx, node.NodeID, rmPlan)
				}
				log.Printf("reconcile: wrote service removal plan node=%s service=%s plan_id=%s gen=%d", node.NodeID, remSvc, rmPlan.GetPlanId(), rmPlan.GetGeneration())
				srv.emitClusterEvent("service_apply_started", map[string]interface{}{
					"severity":       "INFO",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"service":        remSvc,
					"message":        fmt.Sprintf("Service removal plan dispatched for %s on %s", remSvc, node.Identity.Hostname),
					"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, rmPlan.GetGeneration()),
				})
				continue
			}
		}
		if svcHash == "" {
			continue
		}
		appliedSvcHash, err := srv.getNodeAppliedServiceHash(ctx, node.NodeID)
		if err != nil {
			log.Printf("reconcile: read applied service hash for %s: %v", node.NodeID, err)
			continue
		}
		if len(filtered) == 0 {
			if status != nil && status.GetState() == planpb.PlanState_PLAN_SUCCEEDED && planHash == svcHash && currentPlan != nil && status.GetPlanId() == currentPlan.GetPlanId() && status.GetGeneration() == currentPlan.GetGeneration() {
				if err := srv.putNodeAppliedServiceHash(ctx, node.NodeID, svcHash); err != nil {
					log.Printf("reconcile: store applied service hash for %s: %v", node.NodeID, err)
				}
				if srv.resources != nil {
					for _, obj := range desiredObjs {
						if obj != nil && obj.Meta != nil {
							_, _ = srv.resources.UpdateStatus(ctx, "ServiceDesiredVersion", obj.Meta.Name, &cluster_controllerpb.ObjectStatus{
								ObservedGeneration: obj.Meta.Generation,
							})
						}
					}
				}
				srv.emitClusterEvent("plan_apply_succeeded", map[string]interface{}{
					"severity":       "INFO",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"message":        fmt.Sprintf("All services at desired state for %s", node.Identity.Hostname),
					"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, currentPlan.GetGeneration()),
				})
			}
			continue
		}
		if svcHash == appliedSvcHash {
			continue
		}
		// External install detection: if all desired services are reported as
		// installed at the correct version (e.g. via CLI), update the applied
		// hash without requiring a plan to succeed. This handles the case where
		// services were installed outside the plan system.
		if len(node.InstalledVersions) > 0 && len(filtered) > 0 {
			allMatch := true
			for svc, ver := range filtered {
				installedVer := ""
				// InstalledVersions keys are "publisher/service" or just "service"
				for k, v := range node.InstalledVersions {
					parts := strings.SplitN(k, "/", 2)
					candidate := k
					if len(parts) == 2 {
						candidate = parts[1]
					}
					if canonicalServiceName(candidate) == canonicalServiceName(svc) {
						installedVer = v
						break
					}
				}
				if installedVer != ver {
					allMatch = false
					break
				}
			}
			if allMatch {
				log.Printf("reconcile: external install detected node=%s — all %d desired services match installed versions, updating applied hash", node.NodeID, len(filtered))
				if err := srv.putNodeAppliedServiceHash(ctx, node.NodeID, svcHash); err != nil {
					log.Printf("reconcile: store applied service hash for %s: %v", node.NodeID, err)
				}
				_ = srv.putNodeFailureCountServices(ctx, node.NodeID, 0)
				// (EXTERNAL_INSTALL_DETECTED removed — not in required event set)
				continue
			}
		}
		if status != nil && (status.GetState() == planpb.PlanState_PLAN_RUNNING || status.GetState() == planpb.PlanState_PLAN_PENDING) {
			if planHash == svcHash && currentPlan != nil && status.GetPlanId() == currentPlan.GetPlanId() && status.GetGeneration() == currentPlan.GetGeneration() {
				if !isPlanStuck(status, lastEmitMs, now) {
					continue
				}
			} else {
				continue
			}
		}
		if status != nil && status.GetState() == planpb.PlanState_PLAN_SUCCEEDED {
			if planHash == svcHash && currentPlan != nil && status.GetPlanId() == currentPlan.GetPlanId() && status.GetGeneration() == currentPlan.GetGeneration() {
				if err := srv.putNodeAppliedServiceHash(ctx, node.NodeID, svcHash); err != nil {
					log.Printf("reconcile: store applied service hash for %s: %v", node.NodeID, err)
				}
				_ = srv.putNodeFailureCountServices(ctx, node.NodeID, 0)
				srv.emitClusterEvent("service_apply_succeeded", map[string]interface{}{
					"severity":       "INFO",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"message":        fmt.Sprintf("Service plan succeeded for %s", node.Identity.Hostname),
					"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, currentPlan.GetGeneration()),
				})
				continue
			}
		}
		failsSvc, _ := srv.getNodeFailureCountServices(ctx, node.NodeID)
		if status != nil && planHash == svcHash && (status.GetState() == planpb.PlanState_PLAN_FAILED || status.GetState() == planpb.PlanState_PLAN_ROLLED_BACK || status.GetState() == planpb.PlanState_PLAN_EXPIRED) {
			srv.emitClusterEvent("service_apply_failed", map[string]interface{}{
				"severity":       "ERROR",
				"node_id":        node.NodeID,
				"hostname":       node.Identity.Hostname,
				"message":        fmt.Sprintf("Service plan failed for %s (state=%s)", node.Identity.Hostname, status.GetState()),
				"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, status.GetGeneration()),
			})
			delay := backoffDuration(failsSvc)
			if lastEmitMs > 0 && now.Sub(time.UnixMilli(lastEmitMs)) < delay {
				continue
			}
		}

		// pick deterministic service to update this round, rotating on failure
		// so that one unavailable artifact doesn't block all other services.
		svcNames := make([]string, 0, len(filtered))
		for name := range filtered {
			svcNames = append(svcNames, name)
		}
		sort.Strings(svcNames)
		svcName := svcNames[int(failsSvc)%len(svcNames)]
		version := filtered[svcName]
		if blockUntil, ok := srv.serviceBlock[svcName]; ok && now.Before(blockUntil) {
			continue
		}
		op := operator.Get(canonicalServiceName(svcName))
		decision, err := op.AdmitPlan(ctx, operator.AdmitRequest{
			Service:        canonicalServiceName(svcName),
			NodeID:         node.NodeID,
			DesiredVersion: version,
			DesiredHash:    svcHash,
		})
		if err != nil {
			log.Printf("reconcile: operator admit %s on %s failed: %v", svcName, node.NodeID, err)
			continue
		}
		if !decision.Allowed {
			if decision.RequeueAfterSeconds > 0 {
				srv.serviceBlock[svcName] = now.Add(time.Duration(decision.RequeueAfterSeconds) * time.Second)
			}
			continue
		}
		plan := BuildServiceUpgradePlan(node.NodeID, canonicalServiceName(svcName), version, svcHash)
		if plan != nil {
			mutated, err := op.MutatePlan(ctx, operator.MutateRequest{Service: canonicalServiceName(svcName), NodeID: node.NodeID, Plan: plan, DesiredDomain: desiredNet.GetDomain(), DesiredProtocol: desiredNet.GetProtocol(), ClusterID: srv.state.ClusterId})
			if err != nil {
				log.Printf("reconcile: operator mutate %s on %s failed: %v", svcName, node.NodeID, err)
				continue
			}
			if mutated != nil {
				plan = mutated
			}
		}
		plan.PlanId = uuid.NewString()
		plan.ClusterId = srv.state.ClusterId
		plan.NodeId = node.NodeID
		plan.Generation = srv.nextPlanGeneration(ctx, node.NodeID)
		plan.DesiredHash = svcHash
		if plan.GetCreatedUnixMs() == 0 {
			plan.CreatedUnixMs = uint64(now.UnixMilli())
		}
		plan.IssuedBy = "cluster-controller"
		if err := srv.planStore.PutCurrentPlan(ctx, node.NodeID, plan); err != nil {
			log.Printf("reconcile: persist service plan for %s: %v", node.NodeID, err)
			continue
		}
		if appendable, ok := srv.planStore.(interface {
			AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
		}); ok {
			_ = appendable.AppendHistory(ctx, node.NodeID, plan)
		}
		newMeta := &planMeta{PlanId: plan.GetPlanId(), Generation: plan.GetGeneration(), DesiredHash: svcHash, LastEmit: now.UnixMilli()}
		_ = srv.putNodePlanMeta(ctx, node.NodeID, newMeta)
		if status != nil && planHash == svcHash && (status.GetState() == planpb.PlanState_PLAN_FAILED || status.GetState() == planpb.PlanState_PLAN_ROLLED_BACK || status.GetState() == planpb.PlanState_PLAN_EXPIRED) {
			_ = srv.putNodeFailureCountServices(ctx, node.NodeID, failsSvc+1)
		}
		log.Printf("reconcile: wrote service plan node=%s service=%s plan_id=%s gen=%d", node.NodeID, svcName, plan.GetPlanId(), plan.GetGeneration())
		srv.emitClusterEvent("service_apply_started", map[string]interface{}{
			"severity":       "INFO",
			"node_id":        node.NodeID,
			"hostname":       node.Identity.Hostname,
			"service":        svcName,
			"message":        fmt.Sprintf("Service upgrade plan dispatched for %s on %s", svcName, node.Identity.Hostname),
			"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, plan.GetGeneration()),
		})
	}
	if stateDirty {
		srv.lock("reconcile:persist")
		func() {
			defer srv.unlock()
			if err := srv.persistStateLocked(true); err != nil {
				log.Printf("persist state: %v", err)
			}
		}()
	}
}

func backoffDuration(fails int) time.Duration {
	switch {
	case fails <= 0:
		return 0
	case fails == 1:
		return 5 * time.Second
	case fails == 2:
		return 15 * time.Second
	case fails == 3:
		return 30 * time.Second
	default:
		return 60 * time.Second
	}
}

func isPlanStuck(status *planpb.NodePlanStatus, lastEmitMs int64, now time.Time) bool {
	if status == nil {
		return false
	}
	last := status.GetFinishedUnixMs()
	if last == 0 {
		last = status.GetStartedUnixMs()
	}
	if last == 0 && lastEmitMs > 0 {
		last = uint64(lastEmitMs)
	}
	if last == 0 {
		return false
	}
	return now.Sub(time.UnixMilli(int64(last))) > 10*time.Minute
}

func (srv *server) computeNodePlan(node *nodeState) (*cluster_controllerpb.NodePlan, error) {
	if node == nil {
		return nil, nil
	}
	actionList, err := buildPlanActions(node.Profiles)
	if err != nil {
		return nil, err
	}
	plan := &cluster_controllerpb.NodePlan{
		NodeId:   node.NodeID,
		Profiles: append([]string(nil), node.Profiles...),
	}
	if len(actionList) > 0 {
		plan.UnitActions = actionList
	}
	if rendered := srv.renderedConfigForNode(node); len(rendered) > 0 {
		plan.RenderedConfig = rendered
		// Phase 4b: inject restart actions for renderers whose output has changed.
		// If a plan is already in flight (PendingRenderedConfigHashes is set), compare
		// against pending so we don't re-dispatch the same restart actions every cycle.
		compareHashes := node.RenderedConfigHashes
		if len(node.PendingRenderedConfigHashes) > 0 {
			compareHashes = node.PendingRenderedConfigHashes
		}
		if restarts := restartActionsForChangedConfigs(compareHashes, rendered); len(restarts) > 0 {
			plan.UnitActions = append(plan.UnitActions, restarts...)
		}
	}
	return plan, nil
}

func planHash(plan *cluster_controllerpb.NodePlan) string {
	if plan == nil {
		return ""
	}
	actions := plan.GetUnitActions()
	rendered := plan.GetRenderedConfig()
	if len(actions) == 0 && len(rendered) == 0 {
		return ""
	}
	h := sha256.New()
	sortedActions := append([]*cluster_controllerpb.UnitAction(nil), actions...)
	sort.Slice(sortedActions, func(i, j int) bool {
		a := sortedActions[i]
		b := sortedActions[j]
		if a == nil && b == nil {
			return false
		}
		if a == nil {
			return true
		}
		if b == nil {
			return false
		}
		if a.GetUnitName() != b.GetUnitName() {
			return a.GetUnitName() < b.GetUnitName()
		}
		return a.GetAction() < b.GetAction()
	})
	for _, action := range sortedActions {
		if action == nil {
			continue
		}
		h.Write([]byte(action.GetUnitName()))
		h.Write([]byte{0})
		h.Write([]byte(action.GetAction()))
		h.Write([]byte{0})
	}
	if len(rendered) > 0 {
		keys := make([]string, 0, len(rendered))
		for key := range rendered {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			h.Write([]byte(key))
			h.Write([]byte{0})
			h.Write([]byte(rendered[key]))
			h.Write([]byte{0})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (srv *server) clusterNetworkSpec() *cluster_controllerpb.ClusterNetworkSpec {
	srv.lock("unknown")
	spec := srv.state.ClusterNetworkSpec
	srv.unlock()
	if spec == nil {
		return nil
	}
	if clone, ok := proto.Clone(spec).(*cluster_controllerpb.ClusterNetworkSpec); ok {
		return clone
	}
	return nil
}

func (srv *server) renderedConfigForSpec() map[string]string {
	spec := srv.clusterNetworkSpec()
	if spec == nil {
		return nil
	}
	out := make(map[string]string, 4)
	if specJSON, err := protojson.Marshal(spec); err == nil {
		out["cluster.network.spec.json"] = string(specJSON)
	}
	configPayload := map[string]interface{}{
		"Domain":           spec.GetClusterDomain(),
		"Protocol":         spec.GetProtocol(),
		"PortHTTP":         spec.GetPortHttp(),
		"PortHTTPS":        spec.GetPortHttps(),
		"AlternateDomains": spec.GetAlternateDomains(),
		"ACMEEnabled":      spec.GetAcmeEnabled(),
		"AdminEmail":       spec.GetAdminEmail(),
		"ACMEChallenge":    "dns-01",
		"ACMEDNSPreflight": true,
	}
	if cfgJSON, err := json.MarshalIndent(configPayload, "", "  "); err == nil {
		out["/var/lib/globular/network.json"] = string(cfgJSON)
	}
	if gen := srv.networkingGeneration(); gen > 0 {
		out["cluster.network.generation"] = fmt.Sprintf("%d", gen)
	}
	if units := restartUnitsForSpec(spec); len(units) > 0 {
		if b, err := json.Marshal(units); err == nil {
			out["reconcile.restart_units"] = string(b)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// snapshotClusterMembership creates a snapshot of all cluster nodes for config rendering.
// This captures the current state without holding the lock during config generation.
func (srv *server) snapshotClusterMembership() *clusterMembership {
	srv.lock("snapshot-membership")
	defer srv.unlock()

	membership := &clusterMembership{
		ClusterID: srv.state.ClusterId,
		Nodes:     make([]memberNode, 0, len(srv.state.Nodes)),
	}

	for _, node := range srv.state.Nodes {
		if node == nil {
			continue
		}
		// Use the first IP address if available
		var ip string
		if len(node.Identity.Ips) > 0 {
			ip = node.Identity.Ips[0]
		}
		membership.Nodes = append(membership.Nodes, memberNode{
			NodeID:   node.NodeID,
			Hostname: node.Identity.Hostname,
			IP:       ip,
			Profiles: append([]string(nil), node.Profiles...),
		})
	}

	// Sort nodes by ID for deterministic output
	sort.Slice(membership.Nodes, func(i, j int) bool {
		return membership.Nodes[i].NodeID < membership.Nodes[j].NodeID
	})

	return membership
}

// renderedConfigForNode combines network config with service-specific configs for a node.
func (srv *server) renderedConfigForNode(node *nodeState) map[string]string {
	// Start with the network config
	out := srv.renderedConfigForSpec()
	if out == nil {
		out = make(map[string]string)
	}

	// Get cluster membership snapshot for service config rendering
	membership := srv.snapshotClusterMembership()

	// Find the current node in the membership
	var currentMember *memberNode
	for i := range membership.Nodes {
		if membership.Nodes[i].NodeID == node.NodeID {
			currentMember = &membership.Nodes[i]
			break
		}
	}

	// If node not found in membership (shouldn't happen), create a temporary entry
	if currentMember == nil {
		var ip string
		if len(node.Identity.Ips) > 0 {
			ip = node.Identity.Ips[0]
		}
		currentMember = &memberNode{
			NodeID:   node.NodeID,
			Hostname: node.Identity.Hostname,
			IP:       ip,
			Profiles: node.Profiles,
		}
	}

	// Get the cluster domain and external domain from network spec
	domain := ""
	externalDomain := ""
	if spec := srv.clusterNetworkSpec(); spec != nil {
		domain = spec.GetClusterDomain()
		if extDNS := spec.GetExternalDns(); extDNS != nil {
			externalDomain = extDNS.GetDomain()
		}
	}

	// Create config context
	ctx := &serviceConfigContext{
		Membership:     membership,
		CurrentNode:    currentMember,
		ClusterID:      membership.ClusterID,
		Domain:         domain,
		ExternalDomain: externalDomain,
	}

	// Render service-specific configs
	serviceConfigs := renderServiceConfigs(ctx)
	for path, content := range serviceConfigs {
		out[path] = content
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

// renderServiceConfigsForNodeInMembership renders service-specific configs for a
// node using an explicitly provided membership snapshot (instead of reading live state).
// Used by preview to compute hypothetical renders without mutating state.
func (srv *server) renderServiceConfigsForNodeInMembership(node *nodeState, membership *clusterMembership) map[string]string {
	if node == nil || membership == nil {
		return nil
	}
	// Find the node in the provided membership.
	var currentMember *memberNode
	for i := range membership.Nodes {
		if membership.Nodes[i].NodeID == node.NodeID {
			currentMember = &membership.Nodes[i]
			break
		}
	}
	if currentMember == nil {
		return nil
	}
	domain := ""
	externalDomain := ""
	if spec := srv.clusterNetworkSpec(); spec != nil {
		domain = spec.GetClusterDomain()
		if extDNS := spec.GetExternalDns(); extDNS != nil {
			externalDomain = extDNS.GetDomain()
		}
	}
	ctx := &serviceConfigContext{
		Membership:     membership,
		CurrentNode:    currentMember,
		ClusterID:      membership.ClusterID,
		Domain:         domain,
		ExternalDomain: externalDomain,
	}
	return renderServiceConfigs(ctx)
}

func (srv *server) networkingGeneration() uint64 {
	srv.lock("state:network-gen")
	gen := srv.state.NetworkingGeneration
	srv.unlock()
	return gen
}

func restartUnitsForSpec(spec *cluster_controllerpb.ClusterNetworkSpec) []string {
	if spec == nil {
		return nil
	}
	units := []string{
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
		"globular-xds.service",
		"globular-envoy.service",
		"globular-gateway.service",
		"globular-minio.service",
		"scylladb.service",
	}
	if spec.GetProtocol() == "https" {
		units = append(units, "globular-storage.service")
	}
	return units
}

func computeNetworkGeneration(spec *cluster_controllerpb.ClusterNetworkSpec) uint64 {
	if spec == nil {
		return 0
	}
	domain := strings.ToLower(strings.TrimSpace(spec.GetClusterDomain()))
	protoStr := strings.ToLower(strings.TrimSpace(spec.GetProtocol()))
	alts := normalizeDomains(spec.GetAlternateDomains())
	sort.Strings(alts)
	builder := strings.Builder{}
	builder.WriteString(domain)
	builder.WriteString("|")
	builder.WriteString(protoStr)
	builder.WriteString("|")
	builder.WriteString(fmt.Sprintf("%d|%d|", spec.GetPortHttp(), spec.GetPortHttps()))
	builder.WriteString(fmt.Sprintf("%t|", spec.GetAcmeEnabled()))
	builder.WriteString(strings.ToLower(strings.TrimSpace(spec.GetAdminEmail())))
	builder.WriteString("|")
	for _, a := range alts {
		builder.WriteString(a)
		builder.WriteString(",")
	}
	sum := sha256.Sum256([]byte(builder.String()))
	var gen uint64
	for i := 0; i < 8; i++ {
		gen = (gen << 8) | uint64(sum[i])
	}
	if gen == 0 {
		gen = 1
	}
	return gen
}

func normalizeDomains(domains []string) []string {
	if len(domains) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(domains))
	for _, v := range domains {
		if v == "" {
			continue
		}
		trimmed := strings.TrimSpace(strings.ToLower(v))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func (srv *server) shouldDispatch(node *nodeState, hash string) bool {
	if node == nil {
		return false
	}
	if node.AgentEndpoint == "" {
		return false
	}
	if hash == "" {
		return false
	}
	if node.LastPlanHash != hash {
		return true
	}
	if node.Status != "ready" {
		return true
	}
	if node.LastPlanError != "" {
		return true
	}
	return false
}

func (srv *server) dispatchPlan(ctx context.Context, node *nodeState, plan *cluster_controllerpb.NodePlan, operationID string) error {
	if plan == nil {
		return fmt.Errorf("node %s plan is empty", node.NodeID)
	}
	client, err := srv.getAgentClient(ctx, node.AgentEndpoint)
	if err != nil {
		return fmt.Errorf("node %s: %w", node.NodeID, err)
	}
	if err := client.ApplyPlan(ctx, plan, operationID); err != nil {
		return fmt.Errorf("node %s apply plan: %w", node.NodeID, err)
	}
	return nil
}

func (srv *server) getAgentClient(ctx context.Context, endpoint string) (*agentClient, error) {
	srv.agentMu.Lock()
	client := srv.agentClients[endpoint]
	srv.agentMu.Unlock()
	if client != nil {
		return client, nil
	}
	newClient, err := newAgentClient(ctx, endpoint, srv.agentInsecure, srv.agentCAPath, srv.agentServerName)
	if err != nil {
		return nil, err
	}
	srv.agentMu.Lock()
	srv.agentClients[endpoint] = newClient
	srv.agentMu.Unlock()
	return newClient, nil
}

func (srv *server) WatchOperations(req *cluster_controllerpb.WatchOperationsRequest, stream cluster_controllerpb.ClusterControllerService_WatchOperationsServer) error {
	if req == nil {
		return status.Error(codes.InvalidArgument, "request is required")
	}
	ctx := stream.Context()
	w := &operationWatcher{
		nodeID: req.GetNodeId(),
		opID:   req.GetOperationId(),
		ch:     make(chan *cluster_controllerpb.OperationEvent, 8),
	}
	srv.addWatcher(w)
	defer func() {
		srv.removeWatcher(w)
		close(w.ch)
	}()
	srv.opMu.Lock()
	for _, op := range srv.operations {
		op.mu.Lock()
		last := op.last
		op.mu.Unlock()
		if last != nil && w.matches(last) {
			select {
			case w.ch <- last:
			default:
			}
		}
	}
	srv.opMu.Unlock()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case evt := <-w.ch:
			if evt == nil {
				continue
			}
			if err := stream.Send(evt); err != nil {
				return err
			}
			if evt.GetDone() {
				return nil
			}
		}
	}
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

func (srv *server) updateNodeState(nodeID, status, lastError string) bool {
	srv.lock("unknown")
	defer srv.unlock()
	node := srv.state.Nodes[nodeID]
	if node == nil {
		return false
	}
	changed := false
	if node.Status != status {
		node.Status = status
		changed = true
	}
	if node.LastError != lastError {
		node.LastError = lastError
		changed = true
	}
	if changed {
		node.LastSeen = time.Now()
	}
	return changed
}

func (srv *server) recordPlanSent(nodeID, planHash string) bool {
	srv.lock("plan:record-sent")
	defer srv.unlock()
	node := srv.state.Nodes[nodeID]
	if node == nil {
		return false
	}
	node.LastPlanSentAt = time.Now()
	if node.LastPlanError != "" {
		node.LastPlanError = ""
	}
	if planHash != "" {
		node.LastPlanHash = planHash
	}
	node.LastAppliedGeneration = srv.state.NetworkingGeneration
	return true
}

func (srv *server) recordPlanError(nodeID, errMsg string) bool {
	srv.lock("plan:record-error")
	defer srv.unlock()
	node := srv.state.Nodes[nodeID]
	if node == nil {
		return false
	}
	if node.LastPlanError == errMsg {
		return false
	}
	node.LastPlanError = errMsg
	return true
}

func (srv *server) closeAgentClient(endpoint string) {
	if endpoint == "" {
		return
	}
	srv.agentMu.Lock()
	defer srv.agentMu.Unlock()
	if client, ok := srv.agentClients[endpoint]; ok {
		client.Close()
		delete(srv.agentClients, endpoint)
	}
}

func (srv *server) cleanupAgentClients() {
	srv.agentMu.Lock()
	defer srv.agentMu.Unlock()
	for endpoint, client := range srv.agentClients {
		if client.idleDuration() > srv.agentIdleTimeout {
			client.Close()
			delete(srv.agentClients, endpoint)
			log.Printf("closed idle agent client %s", endpoint)
		}
	}
}

func (srv *server) cleanupTimedOutOperations() {
	now := time.Now()
	var expired []struct {
		id     string
		nodeID string
	}
	srv.opMu.Lock()
	for id, op := range srv.operations {
		op.mu.Lock()
		done := op.done
		created := op.created
		nodeID := op.nodeID
		op.mu.Unlock()
		if done || created.IsZero() || nodeID == "" {
			continue
		}
		if now.Sub(created) > operationTimeout {
			expired = append(expired, struct {
				id     string
				nodeID string
			}{id: id, nodeID: nodeID})
		}
	}
	srv.opMu.Unlock()
	for _, entry := range expired {
		evt := srv.newOperationEvent(entry.id, entry.nodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "operation timed out", 0, true, "operation timed out")
		srv.broadcastOperationEvent(evt)
	}
}

func protoToStoredIdentity(pi *cluster_controllerpb.NodeIdentity) storedIdentity {
	if pi == nil {
		return storedIdentity{}
	}
	return storedIdentity{
		Hostname:     pi.GetHostname(),
		Domain:       pi.GetDomain(),
		Ips:          append([]string(nil), pi.GetIps()...),
		Os:           pi.GetOs(),
		Arch:         pi.GetArch(),
		AgentVersion: pi.GetAgentVersion(),
	}
}

func storedIdentityToProto(si storedIdentity) *cluster_controllerpb.NodeIdentity {
	return &cluster_controllerpb.NodeIdentity{
		Hostname:     si.Hostname,
		Domain:       si.Domain,
		Ips:          append([]string(nil), si.Ips...),
		Os:           si.Os,
		Arch:         si.Arch,
		AgentVersion: si.AgentVersion,
	}
}

func identitiesEqual(a, b storedIdentity) bool {
	if a.Hostname != b.Hostname || a.Domain != b.Domain || a.Os != b.Os || a.Arch != b.Arch || a.AgentVersion != b.AgentVersion {
		return false
	}
	if len(a.Ips) != len(b.Ips) {
		return false
	}
	for i := range a.Ips {
		if a.Ips[i] != b.Ips[i] {
			return false
		}
	}
	return true
}

func copyLabels(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func protoUnitsToStored(in []*cluster_controllerpb.NodeUnitStatus) []unitStatusRecord {
	if len(in) == 0 {
		return nil
	}
	out := make([]unitStatusRecord, 0, len(in))
	for _, u := range in {
		if u == nil {
			continue
		}
		out = append(out, unitStatusRecord{
			Name:    u.GetName(),
			State:   u.GetState(),
			Details: u.GetDetails(),
		})
	}
	return out
}

func storedUnitsToProto(in []unitStatusRecord) []*cluster_controllerpb.NodeUnitStatus {
	if len(in) == 0 {
		return nil
	}
	out := make([]*cluster_controllerpb.NodeUnitStatus, 0, len(in))
	for _, u := range in {
		out = append(out, &cluster_controllerpb.NodeUnitStatus{
			Name:    u.Name,
			State:   u.State,
			Details: u.Details,
		})
	}
	return out
}

func normalizedUnits(units []unitStatusRecord) []unitStatusRecord {
	if len(units) == 0 {
		return nil
	}
	out := append([]unitStatusRecord(nil), units...)
	sort.Slice(out, func(i, j int) bool {
		ni := strings.ToLower(out[i].Name)
		nj := strings.ToLower(out[j].Name)
		if ni != nj {
			return ni < nj
		}
		si := strings.ToLower(out[i].State)
		sj := strings.ToLower(out[j].State)
		if si != sj {
			return si < sj
		}
		return strings.ToLower(out[i].Details) < strings.ToLower(out[j].Details)
	})
	return out
}

type operationState struct {
	mu      sync.Mutex
	last    *cluster_controllerpb.OperationEvent
	created time.Time
	done    bool
	nodeID  string
}

type operationWatcher struct {
	nodeID string
	opID   string
	ch     chan *cluster_controllerpb.OperationEvent
}

func (w *operationWatcher) matches(evt *cluster_controllerpb.OperationEvent) bool {
	if w == nil || evt == nil {
		return false
	}
	if w.nodeID != "" && w.nodeID != evt.GetNodeId() {
		return false
	}
	if w.opID != "" && w.opID != evt.GetOperationId() {
		return false
	}
	return true
}

func (srv *server) getOperationState(id string) *operationState {
	srv.opMu.Lock()
	defer srv.opMu.Unlock()
	op, ok := srv.operations[id]
	if !ok {
		op = &operationState{}
		srv.operations[id] = op
	}
	return op
}

func (srv *server) broadcastOperationEvent(evt *cluster_controllerpb.OperationEvent) {
	if evt == nil {
		return
	}
	op := srv.getOperationState(evt.GetOperationId())
	op.mu.Lock()
	if op.created.IsZero() {
		op.created = time.Now()
	}
	if op.nodeID == "" && evt.GetNodeId() != "" {
		op.nodeID = evt.GetNodeId()
	}
	op.last = evt
	if evt.GetDone() {
		op.done = true
	}
	op.mu.Unlock()
	srv.watchMu.Lock()
	for w := range srv.watchers {
		if w.matches(evt) {
			select {
			case w.ch <- evt:
			default:
			}
		}
	}
	srv.watchMu.Unlock()
}

func (srv *server) newOperationEvent(opID, nodeID string, phase cluster_controllerpb.OperationPhase, message string, percent int32, done bool, errMsg string) *cluster_controllerpb.OperationEvent {
	return &cluster_controllerpb.OperationEvent{
		OperationId: opID,
		NodeId:      nodeID,
		Phase:       phase,
		Message:     message,
		Percent:     percent,
		Done:        done,
		Error:       errMsg,
		Ts:          timestamppb.Now(),
	}
}

func (srv *server) addWatcher(w *operationWatcher) {
	srv.watchMu.Lock()
	srv.watchers[w] = struct{}{}
	srv.watchMu.Unlock()
}

func (srv *server) removeWatcher(w *operationWatcher) {
	srv.watchMu.Lock()
	if _, ok := srv.watchers[w]; ok {
		delete(srv.watchers, w)
	}
	srv.watchMu.Unlock()
}
func (srv *server) evaluateNodeStatus(node *nodeState, units []unitStatusRecord) (string, string) {
	if node == nil {
		return "degraded", "missing node record"
	}
	plan, _ := srv.computeNodePlan(node)
	required := requiredUnitsFromPlan(plan)
	if len(required) == 0 {
		return "ready", ""
	}
	unitStates := make(map[string]string, len(units))
	for _, u := range units {
		if u.Name == "" {
			continue
		}
		unitStates[strings.ToLower(u.Name)] = strings.ToLower(u.State)
	}
	var missing []string
	var notActive []string
	for unit := range required {
		state, ok := unitStates[strings.ToLower(unit)]
		if !ok {
			missing = append(missing, fmt.Sprintf("%s missing", unit))
			continue
		}
		if state != "active" {
			if state == "" {
				state = "unknown"
			}
			notActive = append(notActive, fmt.Sprintf("%s is %s", unit, state))
		}
	}
	if len(missing) > 0 || len(notActive) > 0 {
		reason := strings.Join(append(missing, notActive...), "; ")
		if node.ReportedAt.IsZero() || time.Since(node.ReportedAt) < statusGracePeriod {
			return "converging", reason
		}
		return "degraded", reason
	}
	return "ready", ""
}

func requiredUnitsFromPlan(plan *cluster_controllerpb.NodePlan) map[string]struct{} {
	req := make(map[string]struct{})
	if plan == nil {
		return req
	}
	for _, action := range plan.GetUnitActions() {
		if action == nil {
			continue
		}
		switch strings.ToLower(action.GetAction()) {
		case "start", "restart":
			req[action.GetUnitName()] = struct{}{}
		}
	}
	return req
}

func unitsEqual(a, b []unitStatusRecord) bool {
	an := normalizedUnits(a)
	bn := normalizedUnits(b)
	if len(an) != len(bn) {
		return false
	}
	for i := range an {
		if an[i].Name != bn[i].Name || an[i].State != bn[i].State || an[i].Details != bn[i].Details {
			return false
		}
	}
	return true
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func buildUpgradePlan(planID, nodeID, clusterID string, generation uint64, expires time.Time, targetPath, fetchDest string, ref *repositorypb.ArtifactRef, sha string, probePort uint32, diskPath string) *planpb.NodePlan {
	if probePort == 0 {
		probePort = defaultProbePort
	}
	steps := []*planpb.PlanStep{
		planStep("check.disk_free", map[string]interface{}{
			"path":      diskPath,
			"min_bytes": float64(upgradeDiskMinBytes),
		}),
		planStep("artifact.fetch", map[string]interface{}{
			"publisher": ref.GetPublisherId(),
			"name":      ref.GetName(),
			"version":   ref.GetVersion(),
			"platform":  ref.GetPlatform(),
			"dest":      fetchDest,
		}),
		planStep("artifact.verify", map[string]interface{}{
			"path":   fetchDest,
			"sha256": sha,
		}),
		planStep("service.stop", map[string]interface{}{
			"unit": "globular.service",
		}),
		planStep("file.backup", map[string]interface{}{
			"path": targetPath,
		}),
		planStep("file.write_atomic", map[string]interface{}{
			"path": targetPath,
			"src":  fetchDest,
		}),
		planStep("file.write_atomic", map[string]interface{}{
			"path":    versionutil.MarkerPath(defaultTargetName),
			"content": ref.GetVersion(),
		}),
		planStep("service.start", map[string]interface{}{
			"unit": "globular.service",
		}),
		planStep("probe.http", map[string]interface{}{
			"url": fmt.Sprintf("http://127.0.0.1:%d/checksum", probePort),
		}),
	}
	rollback := []*planpb.PlanStep{
		planStep("file.restore_backup", map[string]interface{}{
			"path": targetPath,
		}),
		planStep("service.start", map[string]interface{}{
			"unit": "globular.service",
		}),
	}
	policy := &planpb.PlanPolicy{
		MaxRetries:       3,
		RetryBackoffMs:   2000,
		FailureMode:      planpb.FailureMode_FAILURE_MODE_ROLLBACK,
		DryRun:           false,
		MaxParallelSteps: 1,
	}
	desired := &planpb.DesiredState{
		Services: []*planpb.DesiredService{
			{
				Name:    defaultTargetName,
				Version: ref.GetVersion(),
				Unit:    "globular.service",
			},
		},
	}
	return &planpb.NodePlan{
		ApiVersion:    "globular.io/plan/v1",
		Kind:          "NodePlan",
		ClusterId:     clusterID,
		NodeId:        nodeID,
		PlanId:        planID,
		Generation:    generation,
		CreatedUnixMs: uint64(time.Now().UnixMilli()),
		ExpiresUnixMs: uint64(expires.UnixMilli()),
		IssuedBy:      "cluster-controller",
		Reason:        "update_globular",
		Locks:         []string{"node-upgrade", "service:Globular"},
		Policy:        policy,
		Spec: &planpb.PlanSpec{
			Steps:    steps,
			Rollback: rollback,
			Desired:  desired,
		},
	}
}

func planStep(action string, args map[string]interface{}) *planpb.PlanStep {
	return &planpb.PlanStep{
		Id:     fmt.Sprintf("step-%s", strings.ReplaceAll(action, ".", "-")),
		Action: action,
		Args:   structFromMap(args),
	}
}

func structFromMap(fields map[string]interface{}) *structpb.Struct {
	if len(fields) == 0 {
		return nil
	}
	s, _ := structpb.NewStruct(fields)
	return s
}

func (srv *server) nextPlanGeneration(ctx context.Context, nodeID string) uint64 {
	var last uint64
	if plan, err := srv.planStore.GetCurrentPlan(ctx, nodeID); err == nil && plan != nil {
		last = plan.GetGeneration()
	}
	if status, err := srv.planStore.GetStatus(ctx, nodeID); err == nil && status != nil {
		if status.GetGeneration() > last {
			last = status.GetGeneration()
		}
	}
	return last + 1
}

func (srv *server) waitForPlanStatus(ctx context.Context, nodeID, planID string, expires time.Time) (*planpb.NodePlanStatus, error) {
	ticker := time.NewTicker(planPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil, status.Errorf(codes.Canceled, "context canceled")
		case <-ticker.C:
			statusValue, err := srv.planStore.GetStatus(ctx, nodeID)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "fetch plan status: %v", err)
			}
			if statusValue == nil {
				if !expires.IsZero() && time.Now().After(expires) {
					return nil, status.Error(codes.DeadlineExceeded, "plan expired before execution")
				}
				continue
			}
			if statusValue.GetPlanId() != planID {
				continue
			}
			if isTerminalPlanState(statusValue.GetState()) {
				return statusValue, nil
			}
			if !expires.IsZero() && time.Now().After(expires) {
				return nil, status.Error(codes.DeadlineExceeded, "plan expired before completion")
			}
		}
	}
}

func planStateName(state planpb.PlanState) string {
	if name, ok := planpb.PlanState_name[int32(state)]; ok {
		return name
	}
	return fmt.Sprintf("PLAN_STATE_%d", state)
}

func isTerminalPlanState(state planpb.PlanState) bool {
	switch state {
	case planpb.PlanState_PLAN_SUCCEEDED, planpb.PlanState_PLAN_FAILED, planpb.PlanState_PLAN_ROLLED_BACK, planpb.PlanState_PLAN_EXPIRED:
		return true
	default:
		return false
	}
}

func uploadArtifact(ctx context.Context, ref *repositorypb.ArtifactRef, data []byte) error {
	addr := strings.TrimSpace(os.Getenv(repositoryAddressEnv))
	if addr == "" {
		addr = "localhost:10101"
	}
	client, err := repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
	if err != nil {
		return err
	}
	defer client.Close()
	return client.UploadArtifact(ref, data)
}

func (srv *server) persistStateLocked(force bool) error {
	if !force && time.Since(srv.lastStateSave) < statePersistInterval {
		return nil
	}
	if err := srv.state.save(srv.statePath); err != nil {
		return err
	}
	srv.lastStateSave = time.Now()
	return nil
}
