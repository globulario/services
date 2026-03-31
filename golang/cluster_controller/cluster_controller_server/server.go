package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/globulario/services/golang/cluster_controller/cluster_controller_server/operator"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/netutil"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/workflow"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/store"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	// infraReleaseEnqueue re-triggers infra release reconciliation (used when
	// new nodes join and need infra dispatched).
	infraReleaseEnqueue func(releaseName string)
	// enqueueReconcile is set by startControllerRuntime so that SetNodeProfiles
	// can immediately trigger a reconcile cycle after saving profile changes.
	enqueueReconcile func()

	// autoImportDone is set to true after the first successful auto-import
	// of installed services into desired state. Prevents repeated imports
	// on every ReportNodeStatus heartbeat.
	autoImportDone atomic.Bool

	// etcd cluster membership manager (for multi-node expansion)
	etcdMembers *etcdMemberManager

	// ScyllaDB cluster join manager (gossip-based expansion)
	scyllaMembers *scyllaClusterManager

	// MinIO pool expansion manager (erasure-coded pools)
	minioPoolMgr *minioPoolManager

	// event publishing (fire-and-forget, nil-safe)
	eventClient *event_client.Event_Client

	// plan signing (Ed25519)
	planSignerState *planSigner

	// workflow trace recorder (fire-and-forget, nil-safe if unavailable)
	workflowRec *workflow.Recorder

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
	if agentCAPath == "" {
		// Default to the standard cluster CA path.
		agentCAPath = config.GetTLSFile("", "", "ca.crt")
	}
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
	// Route through the Envoy gateway so it works on any node.
	eventAddr := strings.TrimSpace(os.Getenv("CLUSTER_EVENT_SERVICE_ADDR"))
	if eventAddr == "" {
		if addr, err := config.GetMeshAddress(); err == nil {
			eventAddr = addr // routes through Envoy service mesh (:443)
		} else {
			eventAddr = "localhost:10050"
		}
	}
	if ec, err := event_client.NewEventService_Client(eventAddr, "event.EventService"); err == nil {
		srv.eventClient = ec
	} else {
		log.Printf("cluster-controller: event client unavailable: %v", err)
	}

	// Connect to WorkflowService for reconciliation workflow tracing.
	// Route through the Envoy gateway so it works on any node.
	clusterID := strings.TrimSpace(os.Getenv("CLUSTER_ID"))
	if clusterID == "" {
		clusterID = "globular.internal"
	}
	srv.workflowRec = workflow.NewRecorderWithResolver(func() string {
		if env := strings.TrimSpace(os.Getenv("CLUSTER_WORKFLOW_SERVICE_ADDR")); env != "" {
			return env
		}
		if addr, err := config.GetMeshAddress(); err == nil {
			return addr // routes through Envoy service mesh (:443)
		}
		return ""
	}, clusterID)

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

func (srv *server) getWorkflowRecorder() *workflow.Recorder {
	return srv.workflowRec
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

func (srv *server) observedUnitsForNode(nodeID string) []unitStatusRecord {
	srv.lock("observedUnitsForNode")
	defer srv.unlock()
	node := srv.state.Nodes[nodeID]
	if node == nil {
		return nil
	}
	return append([]unitStatusRecord(nil), node.Units...)
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

// globularNodeIDNamespace is a fixed UUID v5 namespace for Globular node IDs.
// Must match the same constant in identity/validation.go.
var globularNodeIDNamespace = uuid.MustParse("a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d")

// deterministicNodeID generates a stable UUID v5 from the node's identity.
// Prefers MAC address (label "node.mac") for hardware-stable identity;
// falls back to hostname + sorted IPs.
func deterministicNodeID(identity storedIdentity, labels map[string]string) string {
	// Prefer MAC address if provided (most stable across restores).
	if mac := labels["node.mac"]; mac != "" {
		return uuid.NewSHA1(globularNodeIDNamespace, []byte("mac:"+mac)).String()
	}

	// Fallback: hostname + sorted IPs.
	parts := []string{identity.Hostname}
	ips := make([]string, len(identity.Ips))
	copy(ips, identity.Ips)
	sort.Strings(ips)
	parts = append(parts, ips...)
	key := strings.Join(parts, "|")
	return uuid.NewSHA1(globularNodeIDNamespace, []byte("host:"+key)).String()
}

// removeStaleNodesLocked removes nodes that share the same hostname or IP as
// the newly registered node but have a different ID and are unreachable/unhealthy.
// This handles post-restore scenarios where the same physical machine gets a
// new node ID and the old entry lingers as "unreachable" in the UI.
// MUST be called while srv.mu is held.
func (srv *server) removeStaleNodesLocked(newNodeID string, newIdentity storedIdentity, newEndpoint string) {
	if srv.state.Nodes == nil {
		return
	}
	var toRemove []string
	for id, existing := range srv.state.Nodes {
		if id == newNodeID {
			continue
		}
		// Only remove nodes that are not healthy.
		switch existing.Status {
		case "ready", "converging":
			continue
		}
		// Match by hostname or by overlapping IPs.
		match := false
		if newIdentity.Hostname != "" && existing.Identity.Hostname == newIdentity.Hostname {
			match = true
		}
		if !match && newEndpoint != "" && existing.AgentEndpoint == newEndpoint {
			match = true
		}
		if !match {
			for _, newIP := range newIdentity.Ips {
				for _, existIP := range existing.Identity.Ips {
					if newIP != "" && newIP == existIP {
						match = true
						break
					}
				}
				if match {
					break
				}
			}
		}
		if match {
			toRemove = append(toRemove, id)
		}
	}
	for _, id := range toRemove {
		log.Printf("removeStaleNodesLocked: removing stale node %s (same host as %s)", id, newNodeID)
		delete(srv.state.Nodes, id)
	}
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
