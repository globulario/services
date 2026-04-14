package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
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
	"github.com/globulario/services/golang/cluster_controller/cluster_controller_server/projections"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/netutil"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/workflow"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
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
	repositoryServiceName    = "repository.PackageRepository"

	// Health monitoring constants
	healthCheckInterval     = 30 * time.Second // How often to check node health
	unhealthyThreshold      = 2 * time.Minute  // Time without contact before marking unhealthy
	recoveryAttemptInterval = 5 * time.Minute  // How often to attempt recovery
	maxRecoveryAttempts     = 3                // Max recovery attempts before giving up
)

const heartbeatStaleThreshold = 5 * time.Minute

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


type server struct {
	cluster_controllerpb.UnimplementedClusterControllerServiceServer

	cfg                  *clusterControllerConfig
	cfgPath              string
	statePath            string
	state                *controllerState
	mu                   sync.Mutex
	muHeldSince          atomic.Int64
	muHeldBy             atomic.Value
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
	leaderEpoch          atomic.Int64
	resignCh             chan struct{} // signal leader election to resign
	lastHeartbeatProcessed atomic.Int64 // UnixNano of last successful ReportNodeStatus
	reconcileRunning            atomic.Bool
	clusterReconcileRunning     atomic.Bool
	clusterReconcilePending     atomic.Bool
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
	// runClusterReconcileIfIdle starts a cluster.reconcile workflow if none
	// is active. If one is running, it marks pending for a follow-up pass.
	// Set by startControllerRuntime.
	runClusterReconcileIfIdle func(ctx context.Context, source string)

	// autoImportDone is set to true after the first successful auto-import
	// of installed services into desired state. Prevents repeated imports
	// on every ReportNodeStatus heartbeat.
	autoImportDone atomic.Bool

	// workflowSem limits concurrent release workflows to prevent systemd
	// overload on target nodes. Defaults to 3 concurrent workflows.
	workflowSem chan struct{}

	// resolveSem limits concurrent repository resolve calls during the
	// PENDING phase to avoid saturating the repository gRPC endpoint.
	resolveSem chan struct{}

	// lastInfraRetry tracks when retryFailedInfraReleases + enqueueInfraReleases
	// last ran, enforcing a 60-second cooldown.
	lastInfraRetry time.Time

	// inflightWorkflows tracks release IDs that have an active workflow
	// goroutine. Prevents reconcileResolved from dispatching duplicate
	// goroutines for the same release, which would cause a router
	// registration race (the second goroutine overwrites + deletes the
	// first's router on lease contention failure).
	inflightMu        sync.Mutex
	inflightWorkflows map[string]struct{}

	// workflowGate is a circuit breaker that prevents dispatching workflows
	// when the backend is unhealthy (repeated RPC failures).
	workflowGate *workflowHealthGate

	// reconcileBreaker opens when reconcile workflows fail repeatedly,
	// suspending periodic dispatch to prevent backlog buildup.
	reconcileBreaker *reconcileCircuitBreaker

	// dispatchReg provides cross-path deduplication for package dispatches
	// between the drift reconciler and the release pipeline.
	dispatchReg *dispatchRegistry

	// applyLoopDet detects and quarantines packages that are being applied
	// repeatedly without convergence.
	applyLoopDet *applyLoopDetector

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

	// workflowClient is used to delegate workflow execution to the
	// centralized WorkflowService. Lazily connected via the same
	// address resolver as workflowRec.
	workflowClient workflowpb.WorkflowServiceClient

	// actorServer handles workflow callbacks from the centralized
	// WorkflowService. Per-run Routers are registered before each
	// ExecuteWorkflow call.
	actorServer *ControllerActorServer

	// read-only projections over cluster-controller state. Best-effort:
	// failures never propagate to the request path; missing data falls back
	// to the in-memory srv.state. Governed by docs/architecture/projection-clauses.md.
	nodeIdentityProj *projections.NodeIdentityProjector

	// test seams
	testHasActivePlanWithLock func(context.Context, string, string) bool
	// Plan test seams removed.
}

var testHookBeforeReportNodeStatusApply func()

// buildControllerClientTLSCreds loads the cluster CA and the node's service
// certificate for mTLS, then returns gRPC transport credentials for outgoing
// client connections (workflow service, etc). Uses config.ResolveDialTarget-
// provided serverName for TLS verification.
func buildControllerClientTLSCreds(serverName string) credentials.TransportCredentials {
	tlsCfg := &tls.Config{ServerName: serverName}
	caFile := config.GetLocalCACertificate()
	if caFile != "" {
		if caData, err := os.ReadFile(caFile); err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(caData) {
				tlsCfg.RootCAs = pool
			}
		} else {
			log.Printf("cluster-controller: WARN failed to read CA %s: %v", caFile, err)
		}
	} else {
		log.Printf("cluster-controller: WARN no CA certificate path configured")
	}
	// Load client certificate for mTLS (same certs the workflow Recorder uses).
	certFile := "/var/lib/globular/pki/issued/services/service.crt"
	keyFile := "/var/lib/globular/pki/issued/services/service.key"
	if cert, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil {
		tlsCfg.Certificates = []tls.Certificate{cert}
	} else {
		log.Printf("cluster-controller: WARN failed to load client cert %s: %v", certFile, err)
	}
	return credentials.NewTLS(tlsCfg)
}

// loadControllerToken reads or generates a node identity token for outgoing gRPC calls.
// Tries, in order: explicit node_token file, any *_token file in the tokens dir,
// cached MAC-based token, and finally generates a fresh "sa" token.
// cachedToken holds a recently generated token to avoid regeneration on
// every RPC call. The interceptor calls loadControllerToken() per-request;
// without caching, every workflow dispatch would hit the keystore.
var (
	cachedToken       string
	cachedTokenExpiry time.Time
	cachedTokenMu     sync.Mutex
)

func loadControllerToken() string {
	// Fast path: return cached token if still valid (>20% TTL remaining).
	cachedTokenMu.Lock()
	if cachedToken != "" && time.Now().Before(cachedTokenExpiry) {
		t := cachedToken
		cachedTokenMu.Unlock()
		return t
	}
	cachedTokenMu.Unlock()

	token := loadControllerTokenUncached()

	// Cache the token with 80% of its 300s TTL = 240s.
	if token != "" {
		cachedTokenMu.Lock()
		cachedToken = token
		cachedTokenExpiry = time.Now().Add(240 * time.Second)
		cachedTokenMu.Unlock()
	}
	return token
}

func loadControllerTokenUncached() string {
	// 1. Explicit node token file.
	tokenFile := "/var/lib/globular/tokens/node_token"
	if data, err := os.ReadFile(tokenFile); err == nil {
		if t := strings.TrimSpace(string(data)); t != "" {
			if _, err := security.ValidateToken(t); err == nil {
				return t
			}
			log.Printf("cluster-controller: node_token exists but is invalid/expired, trying fallbacks")
		}
	}
	// 2. Scan tokens directory for any valid token.
	dir := "/var/lib/globular/tokens"
	_ = os.MkdirAll(dir, 0750) // ensure directory exists for later writes
	if entries, err := os.ReadDir(dir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), "_token") {
				continue
			}
			if data, err := os.ReadFile(dir + "/" + e.Name()); err == nil {
				if t := strings.TrimSpace(string(data)); t != "" {
					if _, err := security.ValidateToken(t); err == nil {
						return t
					}
				}
			}
		}
	}
	// 3. Cached MAC-based token (may refresh if recently expired).
	if mac, err := config.GetMacAddress(); err == nil && mac != "" {
		if token, err := security.GetLocalToken(mac); err == nil && token != "" {
			log.Printf("cluster-controller: using cached MAC token for auth")
			return token
		}
	}
	// 4. Generate a fresh service-account token. This works on any node
	//    that has access to the signing keystore (all cluster members).
	//    TTL=300s: the interceptor calls loadControllerToken() per-RPC,
	//    so we regenerate often. Longer TTL reduces generation overhead.
	mac, _ := config.GetMacAddress()
	if token, err := security.GenerateToken(300, mac, "sa", "sa", ""); err == nil {
		return token
	} else {
		log.Printf("cluster-controller: WARN token generation failed: %v", err)
	}
	return ""
}

// controllerTokenInterceptor returns a gRPC unary interceptor that attaches
// a fresh token and cluster_id as metadata on every outgoing call.
//
// IMPORTANT: The token is resolved lazily on every call via loadControllerToken(),
// NOT captured once at startup. Static token capture was the root cause of
// recurring Unauthenticated errors — GenerateToken produces 60-second TTL tokens
// that expire long before the controller process restarts.
func controllerTokenInterceptor(clusterID string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{},
		cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		token := loadControllerToken()
		if token == "" {
			// Proceed without token — let the server reject if it requires auth.
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}
		md.Set("token", token)
		if clusterID != "" {
			md.Set("cluster_id", clusterID)
		}
		return invoker(metadata.NewOutgoingContext(ctx, md), method, req, reply, cc, opts...)
	}
}

func newServer(cfg *clusterControllerConfig, cfgPath, statePath string, state *controllerState, kv kvClient) *server {
	if state == nil {
		state = newControllerState()
	}
	if statePath == "" {
		statePath = defaultClusterStatePath
	}
	// TLS paths come from the config layer, not env vars.
	agentCAPath := config.GetTLSFile("", "", "ca.crt")
	serverName := ""
	srv := &server{
		cfg:              cfg,
		cfgPath:          cfgPath,
		statePath:        statePath,
		state:            state,
		kv:               kv,
		agentClients:     make(map[string]*agentClient),
		serviceBlock:     make(map[string]time.Time),
		agentInsecure:    false, // always mTLS in production
		agentIdleTimeout: agentIdleTimeoutDefault,
		agentCAPath:      agentCAPath,
		agentServerName:  serverName,
		operations:       make(map[string]*operationState),
		watchers:         make(map[*operationWatcher]struct{}),
		workflowSem:       make(chan struct{}, 3), // max 3 concurrent release workflows
		resolveSem:        make(chan struct{}, 2), // max 2 concurrent repository resolve calls
		inflightWorkflows: make(map[string]struct{}),
		resignCh:          make(chan struct{}, 1),
		actorServer:       NewControllerActorServer(),
		workflowGate:      newWorkflowHealthGate(),
		reconcileBreaker:  newReconcileCircuitBreaker(),
		dispatchReg:       newDispatchRegistry(),
		applyLoopDet:      newApplyLoopDetector(),
	}
	// Service removal is controlled by the config file, not env vars.
	// Disabled by default for safety.

	// Connect to EventService for reconciliation event publishing.
	// On cold boot the event service may not be registered yet —
	// retry in background so we don't permanently miss it.
	go func() {
		for attempt := 0; attempt < 60; attempt++ {
			eventAddr := config.ResolveServiceAddr("event.EventService", "")
			if eventAddr == "" {
				if addr, err := config.GetMeshAddress(); err == nil {
					eventAddr = addr
				}
			}
			if eventAddr != "" {
				if ec, err := event_client.NewEventService_Client(eventAddr, "event.EventService"); err == nil {
					srv.eventClient = ec
					log.Printf("cluster-controller: event client connected (attempt %d)", attempt+1)
					return
				}
			}
			if attempt == 0 {
				log.Printf("cluster-controller: event service not yet available — will retry")
			}
			time.Sleep(5 * time.Second)
		}
		log.Printf("cluster-controller: WARNING — event client not available after 60 attempts")
	}()

	// Connect to WorkflowService for reconciliation workflow tracing.
	// Prefer the LOCAL workflow service (same as workflowClient) so the
	// call carries mTLS + token auth directly, without Envoy stripping it.
	clusterID := cfg.ClusterDomain
	if clusterID == "" {
		clusterID = "globular.internal"
	}
	wfAddrResolver := func() string {
		// Resolve local workflow service from etcd — runs on every node.
		if addr := config.ResolveLocalServiceAddr("workflow.WorkflowService"); addr != "" {
			return addr
		}
		// Fallback: mesh address (gateway) if local not yet registered.
		if addr, err := config.GetMeshAddress(); err == nil {
			return addr
		}
		return ""
	}
	srv.workflowRec = workflow.NewRecorderWithResolver(wfAddrResolver, clusterID)

	// Create a WorkflowService client for centralized execution.
	// Resolve the LOCAL workflow service from etcd registry (source of truth).
	// The workflow service runs on every node — we always use the local
	// instance so execution stays on this node. HA durability is handled by
	// executor leases and orphan recovery, not by routing to remote instances.
	//
	// On cold boot (e.g. Docker quickstart), the workflow service may not be
	// registered in etcd yet when the controller starts. Resolve lazily in
	// a background goroutine so we don't permanently miss it.
	go func() {
		for attempt := 0; attempt < 60; attempt++ {
			// Try local workflow service first (same node, avoids mesh routing).
			wfAddr := config.ResolveLocalServiceAddr("workflow.WorkflowService")
			if wfAddr == "" {
				// Fallback: any workflow service in the cluster. This handles
				// cold boot where the shared etcd client may not yet see the
				// local instance due to initialization ordering.
				wfAddr = config.ResolveServiceAddr("workflow.WorkflowService", "")
			}
			if wfAddr != "" {
				dt := config.ResolveDialTarget(wfAddr)
				log.Printf("cluster-controller: workflow client dialing %s (resolved from %s)", dt.Address, wfAddr)
				dialOpts := []grpc.DialOption{
					grpc.WithTransportCredentials(buildControllerClientTLSCreds(dt.ServerName)),
					grpc.WithUnaryInterceptor(controllerTokenInterceptor(clusterID)),
				}
				if wfConn, err := grpc.NewClient(dt.Address, dialOpts...); err == nil {
					srv.workflowClient = workflowpb.NewWorkflowServiceClient(wfConn)
					log.Printf("cluster-controller: workflow client connected (attempt %d)", attempt+1)
					return
				} else {
					log.Printf("cluster-controller: workflow client dial failed: %v", err)
				}
			}
			if attempt == 0 {
				log.Printf("cluster-controller: workflow service not yet in registry — will retry")
			}
			time.Sleep(5 * time.Second)
		}
		log.Printf("cluster-controller: WARNING — workflow client not available after 60 attempts")
	}()

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
	operator.Register("etcd", operator.NewEtcdOperator(nodesFn))
	operator.Register("minio", operator.NewMinioOperator(nodesFn))
	operator.Register("scylla", operator.NewScyllaOperator(nodesFn))

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

// eventCircuitBreaker prevents flooding the EventService when it is down.
// When a publish fails, the circuit opens for a cooldown period during which
// events are silently dropped. This avoids burning CPU on auth evaluation +
// error logging for dozens of RPCs per second that will all fail.
var eventCircuitBreaker struct {
	openUntil atomic.Int64 // UnixNano; 0 = closed (healthy)
	dropped   atomic.Int64 // events dropped during current open window
}

const eventCircuitCooldown = 30 * time.Second

// emitClusterEvent publishes an event to the EventService (fire-and-forget).
// Safe to call when eventClient is nil. Uses a circuit breaker to avoid
// flooding the event service when it is down.
func (srv *server) emitClusterEvent(name string, payload map[string]interface{}) {
	if srv.eventClient == nil {
		return
	}

	// Circuit breaker: if open, silently drop until cooldown expires.
	if openUntil := eventCircuitBreaker.openUntil.Load(); openUntil > 0 {
		if time.Now().UnixNano() < openUntil {
			eventCircuitBreaker.dropped.Add(1)
			return
		}
		// Cooldown expired — close circuit and log how many were dropped.
		dropped := eventCircuitBreaker.dropped.Swap(0)
		eventCircuitBreaker.openUntil.Store(0)
		if dropped > 0 {
			log.Printf("cluster-controller: event circuit breaker closed (dropped %d events during cooldown)", dropped)
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	go func() {
		if err := srv.eventClient.Publish(name, data); err != nil {
			// Open the circuit breaker on failure.
			eventCircuitBreaker.openUntil.Store(time.Now().Add(eventCircuitCooldown).UnixNano())
			log.Printf("cluster-controller: publish %q failed, circuit breaker open for %s: %v", name, eventCircuitCooldown, err)
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
	if !isLeader {
		srv.leaderEpoch.Store(0) // clear epoch on demotion
	}
	// When gaining leadership, update the service registry so Envoy routes
	// to this node's controller (the leader). Without this, the registry
	// points to whichever node last started — not necessarily the leader.
	if isLeader {
		if err := config.SaveServiceConfiguration(map[string]interface{}{
			"Id":       "cluster_controller.ClusterControllerService",
			"Name":     "cluster_controller.ClusterControllerService",
			"Address":  config.GetRoutableIPv4(),
			"Port":     srv.cfg.Port,
			"Protocol": "grpc",
			"TLS":      true,
			"State":    "running",
			"Process":  os.Getpid(),
			"Version":  Version,
		}); err != nil {
			log.Printf("leader: failed to update service registry: %v", err)
		} else {
			log.Printf("leader: updated service registry to %s:%d", config.GetRoutableIPv4(), srv.cfg.Port)
		}

		// Signal routing refresh: write a generation key to etcd so xDS
		// (and any other routing-aware component) can detect leader changes
		// and rebuild routing tables immediately.
		//
		// Mechanism (Phase 1): controller writes to this well-known key on
		//   every leadership acquisition. xDS discovers the change on its
		//   next 5-second poll cycle (max 5s latency).
		//
		// Mechanism (Phase 2 — future xDS work): xDS adds an etcd Watch on
		//   /globular/routing/refresh-generation to get immediate push
		//   notification, eliminating the polling latency entirely.
		srv.writeRoutingRefresh()
	}
}

// routingRefreshKey is the well-known etcd key that signals routing-aware
// components (xDS, Envoy, gateway) to rebuild their routing tables.
// Written by the controller on every leadership acquisition.
//
// Contract:
//   - Value: JSON {"epoch": N, "leader_addr": "host:port", "timestamp": "RFC3339"}
//   - Writers: cluster-controller (on leader change)
//   - Readers: xDS watcher (poll or watch), gateway, admin UI
const routingRefreshKey = "/globular/routing/refresh-generation"

// writeRoutingRefresh writes a routing refresh signal to etcd so that
// xDS and other routing-aware components can detect leader changes.
func (srv *server) writeRoutingRefresh() {
	if srv.etcdClient == nil {
		return
	}
	epoch := srv.leaderEpoch.Load()
	addr := fmt.Sprintf("%s:%d", config.GetRoutableIPv4(), srv.cfg.Port)
	value := fmt.Sprintf(`{"epoch":%d,"leader_addr":"%s","timestamp":"%s"}`,
		epoch, addr, time.Now().UTC().Format(time.RFC3339))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := srv.etcdClient.Put(ctx, routingRefreshKey, value); err != nil {
		log.Printf("leader: failed to write routing refresh: %v", err)
	} else {
		log.Printf("leader: wrote routing refresh (epoch=%d)", epoch)
	}
}

// requireLeader returns nil if this instance is the active leader.
// Returns FailedPrecondition with leader_addr metadata if not leader,
// so callers can redirect to the actual leader.
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
	return status.Errorf(codes.FailedPrecondition,
		"not leader (leader_addr=%s, epoch=%d)", addr, srv.leaderEpoch.Load())
}

// requireLeaderEpoch checks that this instance is the leader AND the
// fencing epoch matches. Used for state-mutating operations to prevent
// stale-leader writes after lease loss.
func (srv *server) requireLeaderEpoch(ctx context.Context) error {
	if err := srv.requireLeader(ctx); err != nil {
		return err
	}
	if srv.etcdClient == nil {
		return nil // no etcd = single-node mode, no fencing needed
	}
	currentEpoch := readEpoch(ctx, srv.etcdClient)
	myEpoch := srv.leaderEpoch.Load()
	if currentEpoch != 0 && myEpoch != 0 && currentEpoch != myEpoch {
		// Another leader has incremented the epoch — we're stale.
		srv.setLeader(false, "", "")
		return status.Errorf(codes.FailedPrecondition,
			"stale leader: my_epoch=%d, current_epoch=%d — re-campaigning", myEpoch, currentEpoch)
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

// reloadStateFromEtcd loads the authoritative controller state from etcd.
// Called when this instance gains leadership so it picks up state written by
// the previous leader (which may have been on a different node).
func (srv *server) reloadStateFromEtcd() {
	if srv.etcdClient == nil {
		return
	}
	etcdState, err := loadFromEtcd(srv.etcdClient)
	if err != nil {
		log.Printf("leader: failed to load state from etcd: %v — keeping local state", err)
		return
	}
	if etcdState == nil {
		log.Printf("leader: no state in etcd — seeding from local state")
		// First leader ever: push local state to etcd so other nodes can pick it up.
		if err := srv.state.saveToEtcd(srv.etcdClient); err != nil {
			log.Printf("leader: failed to seed state to etcd: %v", err)
		}
		return
	}
	srv.lock("leader:reload-state")
	defer srv.unlock()
	// Preserve in-memory-only fields that don't serialize to etcd.
	for nodeID, oldNode := range srv.state.Nodes {
		if oldNode == nil {
			continue
		}
		if newNode, ok := etcdState.Nodes[nodeID]; ok && newNode != nil {
			newNode.BootstrapWorkflowActive = oldNode.BootstrapWorkflowActive
			newNode.RestartAttempts = oldNode.RestartAttempts
		}
	}
	srv.state = etcdState
	// Also update the local disk backup.
	if err := srv.state.save(srv.statePath); err != nil {
		log.Printf("leader: failed to save etcd state to disk: %v", err)
	}
	log.Printf("leader: reloaded state from etcd (%d nodes, cluster=%s)", len(srv.state.Nodes), srv.state.ClusterId)
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
	// Primary: persist to etcd (authoritative for leadership transfers).
	if err := srv.state.saveToEtcd(srv.etcdClient); err != nil {
		log.Printf("persist state to etcd failed: %v", err)
		// Continue to save to disk even if etcd write fails.
	}
	// Publish MinIO connection info to the well-known etcd key so that all
	// cluster services can read it. Endpoint is a DNS name (minio.<domain>)
	// served by the DNS reconciler — no IPs are baked in.
	srv.publishMinioConfigLocked()
	// Backup: persist to local disk.
	if err := srv.state.save(srv.statePath); err != nil {
		return err
	}
	srv.lastStateSave = time.Now()
	return nil
}

// publishMinioConfigLocked writes /globular/cluster/minio/config to etcd.
// Must be called with srv.mu held. Safe to call on every state persist —
// the write is tiny and idempotent, so we always keep etcd in sync with
// generated credentials and the current cluster domain.
func (srv *server) publishMinioConfigLocked() {
	if srv.state == nil || srv.state.MinioCredentials == nil {
		return
	}
	// The endpoint is a DNS name served by the cluster DNS (see DNSReconciler
	// collectPoolMemberships → minio.<domain> multi-A records).
	domain := ""
	if srv.state.ClusterNetworkSpec != nil {
		domain = srv.state.ClusterNetworkSpec.ClusterDomain
	}
	if domain == "" {
		return
	}
	cfg := config.MinIOConfig{
		Endpoint:  fmt.Sprintf("minio.%s:9000", domain),
		AccessKey: srv.state.MinioCredentials.RootUser,
		SecretKey: srv.state.MinioCredentials.RootPassword,
		Secure:    true,
		Bucket:    "globular",
		Prefix:    domain,
	}
	if err := config.SaveMinIOConfig(cfg); err != nil {
		log.Printf("publish minio config to etcd failed: %v", err)
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

func requiredUnitsFromPlan(plan *NodeUnitPlan) map[string]struct{} {
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
