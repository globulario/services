package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/healthchecks"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/identity"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/actions"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/apply"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/certs"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/planexec"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/planner"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/supervisor"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/units"
	nodeagentpb "github.com/globulario/services/golang/nodeagent/nodeagentpb"
	"github.com/globulario/services/golang/pki"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/store"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var defaultPort = "11000"

const defaultPlanPollInterval = 5 * time.Second
const planLockTTL = 30

var (
	restartCommand    = restartUnit
	systemctlLookPath = exec.LookPath
	networkPKIManager = func(opts pki.Options) pki.Manager {
		return pki.NewFileManager(opts)
	}
)

// NodeAgentServer implements the simplified node executor API.
type NodeAgentServer struct {
	nodeagentpb.UnimplementedNodeAgentServiceServer

	mu                       sync.Mutex
	stateMu                  sync.Mutex
	controllerConnMu         sync.Mutex
	operations               map[string]*operation
	joinToken                string
	bootstrapToken           string
	controllerEndpoint       string
	agentVersion             string
	bootstrapPlan            []string
	nodeID                   string
	controllerConn           *grpc.ClientConn
	controllerClient         clustercontrollerpb.ClusterControllerServiceClient
	statePath                string
	state                    *nodeAgentState
	joinRequestID            string
	advertisedAddr           string
	useInsecure              bool
	joinPollCancel           context.CancelFunc
	joinPollMu               sync.Mutex
	etcdMode                 string
	controllerCAPath         string
	controllerSNI            string
	controllerUseSystemRoots bool
	lastNetworkGeneration    uint64
	planStore                store.PlanStore
	planPollInterval         time.Duration
	lastPlanGeneration       uint64
	planRunnerCtx            context.Context
	planRunnerOnce           sync.Once
	controllerDialer         func(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error)
	controllerClientFactory  func(conn grpc.ClientConnInterface) clustercontrollerpb.ClusterControllerServiceClient
	controllerClientOverride func(addr string) clustercontrollerpb.ClusterControllerServiceClient

	// test hooks
	syncDNSHook           func(*clustercontrollerpb.ClusterNetworkSpec) error
	waitDNSHook           func(context.Context, *clustercontrollerpb.ClusterNetworkSpec) error
	ensureCertsHook       func(*clustercontrollerpb.ClusterNetworkSpec) error
	restartHook           func([]string, *operation) error
	objectstoreLayoutHook func(context.Context, string) error
	healthCheckHook       func(context.Context, *clustercontrollerpb.ClusterNetworkSpec) error

	certKV certs.KV

	lastCertRestart time.Time
	lastSpec        *clustercontrollerpb.ClusterNetworkSpec
}

type lockablePlanStore interface {
	store.PlanStore
	Client() *clientv3.Client
}

type planLockGuard struct {
	client   *clientv3.Client
	leaseID  clientv3.LeaseID
	cancel   context.CancelFunc
	nodeID   string
	lockKeys []string
}

func (g *planLockGuard) release(ctx context.Context) {
	if g == nil {
		return
	}
	if g.cancel != nil {
		g.cancel()
	}
	if g.client != nil && g.leaseID != 0 {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		g.client.Revoke(ctx, g.leaseID)
	}
}

func (g *planLockGuard) keepAliveLoop(ch <-chan *clientv3.LeaseKeepAliveResponse) {
	if ch == nil {
		return
	}
	for range ch {
	}
}

func planLockKey(nodeID, lock string) string {
	return fmt.Sprintf("%s/%s/%s", store.PlanLockBaseKey, nodeID, lock)
}

func isTerminalState(state planpb.PlanState) bool {
	switch state {
	case planpb.PlanState_PLAN_SUCCEEDED, planpb.PlanState_PLAN_FAILED, planpb.PlanState_PLAN_ROLLED_BACK, planpb.PlanState_PLAN_EXPIRED:
		return true
	default:
		return false
	}
}

func NewNodeAgentServer(statePath string, state *nodeAgentState) *NodeAgentServer {
	if state == nil {
		state = newNodeAgentState()
	}
	port := getEnv("NODE_AGENT_PORT", defaultPort)
	advertised := strings.TrimSpace(os.Getenv("NODE_AGENT_ADVERTISE_ADDR"))
	clusterMode := getEnv("NODE_AGENT_CLUSTER_MODE", "true") != "false"

	if advertised == "" {
		// Determine advertise IP using validated selection
		advertiseIP, err := identity.SelectAdvertiseIP(os.Getenv("NODE_AGENT_ADVERTISE_IP"))
		if err != nil {
			if clusterMode {
				// In cluster mode, FAIL FAST if no valid IP
				log.Fatalf("node-agent: cannot determine advertise IP in cluster mode: %v", err)
			}
			// Development/single-node mode: allow localhost
			log.Printf("node-agent: warning: no advertise IP, using localhost (development mode)")
			advertiseIP = "127.0.0.1"
		}
		advertised = fmt.Sprintf("%s:%s", advertiseIP, port)
	}

	// Validate advertise endpoint
	if err := identity.ValidateAdvertiseEndpoint(advertised, clusterMode); err != nil {
		log.Fatalf("node-agent: invalid advertise endpoint: %v", err)
	}
	useInsecure := strings.EqualFold(getEnv("NODE_AGENT_INSECURE", "false"), "true")

	// Determine cluster domain early for controller discovery (PR3)
	clusterDomain := getEnv("CLUSTER_DOMAIN", "")

	// Controller endpoint discovery (PR3: prefer DNS in cluster mode)
	controllerEndpoint := strings.TrimSpace(os.Getenv("NODE_AGENT_CONTROLLER_ENDPOINT"))
	if controllerEndpoint == "" && clusterDomain != "" && clusterMode {
		// In cluster mode with domain configured, use DNS-based discovery
		controllerPort := getEnv("CLUSTER_CONTROLLER_PORT", "12000")
		controllerEndpoint = fmt.Sprintf("controller.%s:%s", clusterDomain, controllerPort)
		log.Printf("node-agent: using DNS-based controller discovery: %s", controllerEndpoint)
	}
	if controllerEndpoint == "" {
		controllerEndpoint = state.ControllerEndpoint
	} else {
		state.ControllerEndpoint = controllerEndpoint
	}

	// Validate controller endpoint in cluster mode (PR3)
	if controllerEndpoint != "" && clusterMode {
		if err := identity.ValidateAdvertiseEndpoint(controllerEndpoint, clusterMode); err != nil {
			log.Printf("node-agent: WARNING - controller endpoint uses localhost in cluster mode: %s (this may prevent multi-node operation)", controllerEndpoint)
		}
	}

	nodeID := state.NodeID
	if nodeID == "" {
		nodeID = strings.TrimSpace(os.Getenv("NODE_AGENT_NODE_ID"))
		state.NodeID = nodeID
	}

	// Node name selection (PR1)
	nodeName := getEnv("NODE_AGENT_NODE_NAME", "")
	if nodeName == "" {
		hostname, _ := os.Hostname()
		if hostname != "" {
			nodeName = identity.SanitizeNodeName(hostname)
		} else {
			nodeName = "node"
		}
	}
	state.NodeName = nodeName

	// Compute advertise FQDN (clusterDomain already defined above for controller discovery)
	if clusterDomain != "" {
		state.AdvertiseFQDN = fmt.Sprintf("%s.%s", nodeName, clusterDomain)
		state.ClusterDomain = clusterDomain
	}
	state.AdvertiseIP = strings.Split(advertised, ":")[0]

	return &NodeAgentServer{
		operations:               make(map[string]*operation),
		joinToken:                strings.TrimSpace(os.Getenv("NODE_AGENT_JOIN_TOKEN")),
		bootstrapToken:           strings.TrimSpace(os.Getenv("NODE_AGENT_BOOTSTRAP_TOKEN")),
		controllerEndpoint:       controllerEndpoint,
		agentVersion:             getEnv("NODE_AGENT_VERSION", "v0.1.0"),
		bootstrapPlan:            nil,
		nodeID:                   nodeID,
		statePath:                statePath,
		state:                    state,
		joinRequestID:            state.RequestID,
		advertisedAddr:           advertised,
		useInsecure:              useInsecure,
		etcdMode:                 "managed",
		controllerCAPath:         strings.TrimSpace(os.Getenv("NODE_AGENT_CONTROLLER_CA")),
		controllerSNI:            strings.TrimSpace(os.Getenv("NODE_AGENT_CONTROLLER_SNI")),
		controllerUseSystemRoots: strings.EqualFold(os.Getenv("NODE_AGENT_CONTROLLER_USE_SYSTEM_ROOTS"), "true"),
		planPollInterval:         defaultPlanPollInterval,
		lastPlanGeneration:       state.LastPlanGeneration,
		lastNetworkGeneration:    state.NetworkGeneration,
		controllerDialer:         grpc.DialContext,
		controllerClientFactory:  clustercontrollerpb.NewClusterControllerServiceClient,
	}
}

func (srv *NodeAgentServer) SetEtcdMode(mode string) {
	if mode == "" {
		return
	}
	srv.etcdMode = strings.ToLower(strings.TrimSpace(mode))
}

func (srv *NodeAgentServer) SetPlanStore(ps store.PlanStore) {
	srv.planStore = ps
	if srv.planPollInterval <= 0 {
		srv.planPollInterval = defaultPlanPollInterval
	}
}

func (srv *NodeAgentServer) isEtcdManaged() bool {
	return strings.EqualFold(srv.etcdMode, "managed")
}

func (srv *NodeAgentServer) EnsureEtcd(ctx context.Context) error {
	if !srv.isEtcdManaged() {
		return nil
	}
	unit := units.UnitForService("etcd")
	if unit == "" {
		unit = "globular-etcd.service"
	}
	log.Printf("etcd bootstrap skipped; %s should be managed by systemd", unit)
	return nil
}

func (srv *NodeAgentServer) SetBootstrapPlan(plan []string) {
	srv.bootstrapPlan = append([]string(nil), plan...)
}

func (srv *NodeAgentServer) BootstrapIfNeeded(ctx context.Context) error {
	if len(srv.bootstrapPlan) == 0 {
		return nil
	}
	var reachable bool
	if srv.controllerEndpoint != "" {
		timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := srv.ensureControllerClient(timeoutCtx); err == nil {
			reachable = true
		}
	}
	if reachable {
		return nil
	}
	plan := buildBootstrapPlan(srv.bootstrapPlan)
	if len(plan.GetUnitActions()) == 0 {
		return nil
	}
	op := srv.registerOperation("bootstrap plan", srv.bootstrapPlan)
	go srv.runPlan(ctx, op, plan)
	return nil
}

func (srv *NodeAgentServer) StartHeartbeat(ctx context.Context) {
	go srv.heartbeatLoop(ctx)
}

func (srv *NodeAgentServer) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		if err := srv.reportStatus(ctx); err != nil {
			log.Printf("node heartbeat failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// StartACMERenewal starts the background ACME certificate renewal loop
func (srv *NodeAgentServer) StartACMERenewal(ctx context.Context) {
	go srv.acmeRenewalLoop(ctx)
}

func (srv *NodeAgentServer) acmeRenewalLoop(ctx context.Context) {
	// Check every 12 hours
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	// Run immediately on startup, then every 12h
	srv.checkAndRenewCertificate(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.checkAndRenewCertificate(ctx)
		}
	}
}

func (srv *NodeAgentServer) checkAndRenewCertificate(ctx context.Context) {
	srv.mu.Lock()
	spec := srv.lastSpec
	srv.mu.Unlock()

	if spec == nil {
		return
	}

	// Only renew if https and ACME enabled
	if !strings.EqualFold(spec.GetProtocol(), "https") || !spec.GetAcmeEnabled() {
		return
	}

	log.Printf("ACME renewal check: domain=%s", spec.GetClusterDomain())

	// Run tls.acme.ensure action
	handler := actions.Get("tls.acme.ensure")
	if handler == nil {
		log.Printf("ACME renewal: action not registered")
		return
	}

	args := map[string]interface{}{
		"domain":       spec.GetClusterDomain(),
		"admin_email":  spec.GetAdminEmail(),
		"acme_enabled": spec.GetAcmeEnabled(),
		"dns_addr":     "localhost:10033",
	}

	argsStruct, err := structpb.NewStruct(args)
	if err != nil {
		log.Printf("ACME renewal: failed to create args: %v", err)
		return
	}

	if err := handler.Validate(argsStruct); err != nil {
		log.Printf("ACME renewal: validation failed: %v", err)
		return
	}

	result, err := handler.Apply(ctx, argsStruct)
	if err != nil {
		log.Printf("ACME renewal failed: %v", err)
		return
	}

	log.Printf("ACME renewal result: %s", result)

	// If certificate changed, restart services
	if strings.Contains(result, "issued") || strings.Contains(result, "renewed") {
		log.Printf("Certificate changed, restarting gateway/xds/envoy")
		srv.restartServicesAfterCertChange(ctx)
	}
}

func (srv *NodeAgentServer) restartServicesAfterCertChange(ctx context.Context) {
	// Restart gateway, xds, and envoy if present
	servicesToRestart := []string{"gateway", "xds", "envoy"}

	if srv.restartHook != nil {
		if err := srv.restartHook(servicesToRestart, nil); err != nil {
			log.Printf("restart services after cert change failed: %v", err)
		}
	}
}

func (srv *NodeAgentServer) StartPlanRunner(ctx context.Context) {
	if srv.planStore == nil {
		return
	}
	srv.planRunnerCtx = ctx
	srv.startPlanRunnerLoop()
}

func (srv *NodeAgentServer) startPlanRunnerLoop() {
	if srv.planRunnerCtx == nil || srv.planStore == nil {
		return
	}
	srv.planRunnerOnce.Do(func() {
		go srv.planLoop(srv.planRunnerCtx)
	})
}

func (srv *NodeAgentServer) planLoop(ctx context.Context) {
	interval := srv.planPollInterval
	if interval <= 0 {
		interval = defaultPlanPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	srv.pollPlan(ctx)
	srv.pollCertGeneration(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.pollPlan(ctx)
			srv.pollCertGeneration(ctx)
		}
	}
}

func (srv *NodeAgentServer) pollPlan(ctx context.Context) {
	if srv.planStore == nil || srv.nodeID == "" {
		return
	}
	pollCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	plan, err := srv.planStore.GetCurrentPlan(pollCtx, srv.nodeID)
	if err != nil {
		log.Printf("unable to read current plan: %v", err)
		return
	}
	status, _ := srv.planStore.GetStatus(pollCtx, srv.nodeID)
	if plan == nil {
		return
	}
	if plan.GetNodeId() != "" && plan.GetNodeId() != srv.nodeID {
		return
	}
	if status != nil && status.GetGeneration() == plan.GetGeneration() && isTerminalState(status.GetState()) {
		return
	}
	now := time.Now().UnixMilli()
	if plan.GetExpiresUnixMs() > 0 && now > int64(plan.GetExpiresUnixMs()) {
		srv.markPlanExpired(pollCtx, plan)
		return
	}
	srv.runStoredPlan(ctx, plan, status)
}

func (srv *NodeAgentServer) acquirePlanLocks(ctx context.Context, plan *planpb.NodePlan) (*planLockGuard, error) {
	if plan == nil || len(plan.GetLocks()) == 0 || plan.GetNodeId() == "" || srv.planStore == nil {
		return nil, nil
	}
	st, ok := srv.planStore.(lockablePlanStore)
	if !ok {
		return nil, fmt.Errorf("plan store does not support locks")
	}
	client := st.Client()
	if client == nil {
		return nil, fmt.Errorf("etcd client unavailable")
	}
	locks := append([]string(nil), plan.GetLocks()...)
	sort.Strings(locks)
	leaseCtx, leaseCancel := context.WithTimeout(ctx, 5*time.Second)
	defer leaseCancel()
	leaseResp, err := client.Grant(leaseCtx, planLockTTL)
	if err != nil {
		return nil, fmt.Errorf("lease grant: %w", err)
	}
	guard := &planLockGuard{
		client:  client,
		leaseID: leaseResp.ID,
		nodeID:  plan.GetNodeId(),
	}
	keepCtx, keepCancel := context.WithCancel(ctx)
	guard.cancel = keepCancel
	ch, err := client.KeepAlive(keepCtx, leaseResp.ID)
	if err != nil {
		guard.release(ctx)
		return nil, fmt.Errorf("keepalive: %w", err)
	}
	go guard.keepAliveLoop(ch)
	for _, lock := range locks {
		key := planLockKey(plan.GetNodeId(), lock)
		txnCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		txn := client.Txn(txnCtx).If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
			Then(clientv3.OpPut(key, "", clientv3.WithLease(leaseResp.ID)))
		resp, err := txn.Commit()
		cancel()
		if err != nil {
			guard.release(ctx)
			return nil, fmt.Errorf("lock %s: %w", lock, err)
		}
		if !resp.Succeeded {
			guard.release(ctx)
			return nil, fmt.Errorf("lock %s busy", lock)
		}
		guard.lockKeys = append(guard.lockKeys, key)
	}
	return guard, nil
}

func (srv *NodeAgentServer) markPlanExpired(ctx context.Context, plan *planpb.NodePlan) {
	opID := plan.GetPlanId()
	if opID == "" {
		opID = uuid.NewString()
	}
	op := srv.registerOperationWithID("plan-expired", opID, nil)
	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, "plan expired", 0, true, "plan expired before execution"))

	status := srv.newPlanStatus(plan)
	status.State = planpb.PlanState_PLAN_EXPIRED
	status.FinishedUnixMs = uint64(time.Now().UnixMilli())
	status.ErrorMessage = "plan expired"
	srv.addPlanEvent(status, "warn", "plan expired before execution", "")
	srv.publishPlanStatus(ctx, status)
}

func (srv *NodeAgentServer) runStoredPlan(ctx context.Context, plan *planpb.NodePlan, status *planpb.NodePlanStatus) {
	if plan == nil {
		return
	}
	guard, err := srv.acquirePlanLocks(ctx, plan)
	if err != nil {
		msg := fmt.Sprintf("lock acquisition failed: %v", err)
		opID := plan.GetPlanId()
		if opID == "" {
			opID = uuid.NewString()
		}
		op := srv.registerOperationWithID("plan-runner", opID, nil)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, msg, 0, true, msg))
		st := srv.newPlanStatus(plan)
		st.State = planpb.PlanState_PLAN_PENDING
		st.ErrorMessage = msg
		srv.addPlanEvent(st, "error", msg, "")
		srv.publishPlanStatus(ctx, st)
		return
	}
	defer guard.release(ctx)
	opID := plan.GetPlanId()
	if opID == "" {
		opID = uuid.NewString()
	}
	op := srv.registerOperationWithID("plan-runner", opID, nil)
	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_QUEUED, "plan queued", 0, false, ""))
	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, "plan running", 5, false, ""))

	runner := planexec.NewRunner(srv.nodeID, srv.publishPlanStatus)
	updated, recErr := runner.ReconcilePlan(ctx, plan, status)
	if recErr != nil {
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, recErr.Error(), 90, true, recErr.Error()))
		return
	}
	if updated != nil && isTerminalState(updated.GetState()) {
		srv.lastPlanGeneration = plan.GetGeneration()
		srv.state.LastPlanGeneration = plan.GetGeneration()
		if err := srv.state.save(srv.statePath); err != nil {
			log.Printf("save state: %v", err)
		}
	}
	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_SUCCEEDED, "plan reconciled", 100, true, ""))
}

func (srv *NodeAgentServer) performRollback(ctx context.Context, plan *planpb.NodePlan, status *planpb.NodePlanStatus, op *operation) error {
	spec := plan.GetSpec()
	if spec == nil || len(spec.GetRollback()) == 0 {
		err := errors.New("rollback steps not configured")
		status.ErrorMessage = err.Error()
		srv.publishPlanStatus(ctx, status)
		return err
	}
	status.State = planpb.PlanState_PLAN_ROLLING_BACK
	srv.addPlanEvent(status, "warn", "plan rolling back", "")
	srv.publishPlanStatus(ctx, status)

	steps := spec.GetRollback()
	total := len(steps)
	for idx, step := range steps {
		stepStatus := &planpb.StepStatus{
			Id:            step.GetId(),
			State:         planpb.StepState_STEP_RUNNING,
			Attempt:       1,
			StartedUnixMs: uint64(time.Now().UnixMilli()),
		}
		status.Steps = append(status.Steps, stepStatus)
		status.CurrentStepId = step.GetId()
		srv.addPlanEvent(status, "info", fmt.Sprintf("rollback step %s running", step.GetId()), step.GetId())
		srv.publishPlanStatus(ctx, status)

		percent := percentForStep(idx, total)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("rollback %s running", step.GetId()), percent, false, ""))

		if err := srv.executePlanStep(ctx, step); err != nil {
			stepStatus.State = planpb.StepState_STEP_FAILED
			stepStatus.FinishedUnixMs = uint64(time.Now().UnixMilli())
			status.State = planpb.PlanState_PLAN_FAILED
			status.ErrorMessage = fmt.Sprintf("rollback failed: %v", err)
			status.ErrorStepId = step.GetId()
			status.FinishedUnixMs = uint64(time.Now().UnixMilli())
			srv.addPlanEvent(status, "error", fmt.Sprintf("rollback step %s failed: %v", step.GetId(), err), step.GetId())
			srv.publishPlanStatus(ctx, status)
			op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, fmt.Sprintf("rollback %s failed", step.GetId()), percent, true, err.Error()))
			return err
		}

		stepStatus.State = planpb.StepState_STEP_OK
		stepStatus.FinishedUnixMs = uint64(time.Now().UnixMilli())
		srv.addPlanEvent(status, "info", fmt.Sprintf("rollback step %s succeeded", step.GetId()), step.GetId())
		srv.publishPlanStatus(ctx, status)
	}

	status.State = planpb.PlanState_PLAN_ROLLED_BACK
	status.FinishedUnixMs = uint64(time.Now().UnixMilli())
	status.CurrentStepId = ""
	srv.addPlanEvent(status, "info", "rollback succeeded", "")
	srv.publishPlanStatus(ctx, status)
	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, "rollback succeeded", 100, true, ""))
	return nil
}

func (srv *NodeAgentServer) executePlanStep(ctx context.Context, step *planpb.PlanStep) error {
	if step == nil {
		return errors.New("plan step is nil")
	}
	handler := actions.Get(step.GetAction())
	if handler == nil {
		return fmt.Errorf("unsupported action %q", step.GetAction())
	}
	if err := handler.Validate(step.GetArgs()); err != nil {
		return err
	}
	timeout := stepTimeout(step)
	stepCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		stepCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	msg, err := handler.Apply(stepCtx, step.GetArgs())
	if err != nil {
		return err
	}
	if msg != "" {
		log.Printf("plan step %s: %s", step.GetId(), msg)
	}
	return nil
}

func stepTimeout(step *planpb.PlanStep) time.Duration {
	if step == nil {
		return 0
	}
	if policy := step.GetPolicy(); policy != nil {
		if policy.GetTimeoutMs() > 0 {
			return time.Duration(policy.GetTimeoutMs()) * time.Millisecond
		}
	}
	return 0
}

func (srv *NodeAgentServer) newPlanStatus(plan *planpb.NodePlan) *planpb.NodePlanStatus {
	now := uint64(time.Now().UnixMilli())
	return &planpb.NodePlanStatus{
		PlanId:        plan.GetPlanId(),
		NodeId:        srv.nodeID,
		Generation:    plan.GetGeneration(),
		StartedUnixMs: now,
	}
}

func (srv *NodeAgentServer) addPlanEvent(status *planpb.NodePlanStatus, level, msg, stepID string) {
	if status == nil {
		return
	}
	status.Events = append(status.Events, &planpb.PlanEvent{
		TsUnixMs: uint64(time.Now().UnixMilli()),
		Level:    level,
		Msg:      msg,
		StepId:   stepID,
	})
}

func (srv *NodeAgentServer) publishPlanStatus(ctx context.Context, status *planpb.NodePlanStatus) {
	if srv.planStore == nil || status == nil {
		return
	}
	if err := srv.planStore.PutStatus(ctx, srv.nodeID, status); err != nil {
		log.Printf("failed to publish plan status: %v", err)
	}
}

func (srv *NodeAgentServer) reportStatus(ctx context.Context) error {
	if srv.controllerEndpoint == "" {
		return nil
	}
	if srv.nodeID == "" {
		return nil
	}
	identity := buildNodeIdentity()
	status := &clustercontrollerpb.NodeStatus{
		NodeId:        srv.nodeID,
		Identity:      identity,
		Ips:           append([]string(nil), identity.GetIps()...),
		Units:         convertNodeAgentUnits(detectUnits(ctx)),
		LastError:     "",
		ReportedAt:    timestamppb.Now(),
		AgentEndpoint: srv.advertisedAddr,
	}
	return srv.sendStatusWithRetry(ctx, status)
}

func leaderAddrFromError(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return ""
	}
	if st.Code() != codes.FailedPrecondition {
		return ""
	}
	msg := st.Message()
	const marker = "leader_addr="
	idx := strings.Index(msg, marker)
	if idx < 0 {
		return ""
	}
	addr := strings.TrimSpace(msg[idx+len(marker):])
	addr = strings.Trim(addr, ")")
	return addr
}

func (srv *NodeAgentServer) resetControllerClient() {
	srv.controllerConnMu.Lock()
	defer srv.controllerConnMu.Unlock()
	if srv.controllerConn != nil {
		_ = srv.controllerConn.Close()
		srv.controllerConn = nil
	}
	srv.controllerClient = nil
}

func (srv *NodeAgentServer) sendStatusWithRetry(ctx context.Context, statusReq *clustercontrollerpb.NodeStatus) error {
	if statusReq == nil {
		return errors.New("status request is nil")
	}
	if err := srv.ensureControllerClient(ctx); err != nil {
		return err
	}
	send := func() error {
		sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, err := srv.controllerClient.ReportNodeStatus(sendCtx, &clustercontrollerpb.ReportNodeStatusRequest{
			Status: statusReq,
		})
		return err
	}
	if err := send(); err != nil {
		addr := leaderAddrFromError(err)
		if addr == "" {
			return err
		}
		// Switch to leader and retry once.
		srv.controllerEndpoint = addr
		if srv.controllerClientOverride != nil {
			srv.controllerClient = srv.controllerClientOverride(addr)
		} else {
			srv.resetControllerClient()
			if errEnsure := srv.ensureControllerClient(ctx); errEnsure != nil {
				return err
			}
		}
		return send()
	}
	return nil
}

func (srv *NodeAgentServer) ensureControllerClient(ctx context.Context) error {
	if srv.controllerEndpoint == "" {
		return errors.New("controller endpoint is not configured")
	}
	opts, err := srv.controllerDialOptions()
	if err != nil {
		return err
	}
	srv.controllerConnMu.Lock()
	defer srv.controllerConnMu.Unlock()
	if srv.controllerClient != nil {
		return nil
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	dialer := srv.controllerDialer
	if dialer == nil {
		dialer = grpc.DialContext
	}
	conn, err := dialer(dialCtx, srv.controllerEndpoint, opts...)
	if err != nil {
		return err
	}
	srv.controllerConn = conn
	factory := srv.controllerClientFactory
	if factory == nil {
		factory = clustercontrollerpb.NewClusterControllerServiceClient
	}
	srv.controllerClient = factory(conn)
	return nil
}

func (srv *NodeAgentServer) controllerDialOptions() ([]grpc.DialOption, error) {
	if srv.controllerEndpoint == "" {
		return nil, errors.New("controller endpoint is not configured")
	}
	opts := []grpc.DialOption{grpc.WithBlock()}
	if srv.useInsecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		return opts, nil
	}
	if srv.controllerCAPath == "" && !srv.controllerUseSystemRoots {
		return nil, errors.New("NODE_AGENT_CONTROLLER_CA is required unless NODE_AGENT_INSECURE=true or NODE_AGENT_CONTROLLER_USE_SYSTEM_ROOTS=true")
	}
	var tlsConfig tls.Config
	if srv.controllerCAPath != "" {
		data, err := os.ReadFile(srv.controllerCAPath)
		if err != nil {
			return nil, fmt.Errorf("read controller ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("failed to parse controller ca")
		}
		tlsConfig.RootCAs = pool
	}
	serverName := srv.controllerSNI
	if serverName == "" {
		if host, _, err := net.SplitHostPort(srv.controllerEndpoint); err == nil {
			serverName = host
		} else {
			serverName = srv.controllerEndpoint
		}
	}
	if serverName != "" {
		tlsConfig.ServerName = serverName
	}
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tlsConfig)))
	return opts, nil
}

func (srv *NodeAgentServer) JoinCluster(ctx context.Context, req *nodeagentpb.JoinClusterRequest) (*nodeagentpb.JoinClusterResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	token := strings.TrimSpace(req.GetJoinToken())
	if token == "" {
		return nil, status.Error(codes.InvalidArgument, "join_token is required")
	}
	if srv.joinToken != "" && token != srv.joinToken {
		return nil, status.Error(codes.PermissionDenied, "join token mismatch")
	}
	controllerEndpoint := strings.TrimSpace(req.GetControllerEndpoint())
	if controllerEndpoint == "" {
		return nil, status.Error(codes.InvalidArgument, "controller_endpoint is required")
	}
	srv.controllerEndpoint = controllerEndpoint
	srv.state.ControllerEndpoint = controllerEndpoint
	if err := srv.saveState(); err != nil {
		log.Printf("warn: persist controller endpoint: %v", err)
	}

	if err := srv.ensureControllerClient(ctx); err != nil {
		return nil, status.Errorf(codes.Unavailable, "controller unavailable: %v", err)
	}

	resp, err := srv.controllerClient.RequestJoin(ctx, &clustercontrollerpb.RequestJoinRequest{
		JoinToken: token,
		Identity:  buildNodeIdentity(),
		Labels:    parseNodeAgentLabels(),
	})
	if err != nil {
		return nil, err
	}
	srv.joinRequestID = resp.GetRequestId()
	srv.state.RequestID = srv.joinRequestID
	srv.state.NodeID = ""
	srv.nodeID = ""
	if err := srv.saveState(); err != nil {
		log.Printf("warn: persist join request: %v", err)
	}

	srv.startJoinApprovalWatcher(context.Background(), srv.joinRequestID)

	return &nodeagentpb.JoinClusterResponse{
		RequestId: resp.GetRequestId(),
		Status:    resp.GetStatus(),
		Message:   resp.GetMessage(),
	}, nil
}

func (srv *NodeAgentServer) GetInventory(ctx context.Context, _ *nodeagentpb.GetInventoryRequest) (*nodeagentpb.GetInventoryResponse, error) {
	resp := &nodeagentpb.GetInventoryResponse{
		Inventory: &nodeagentpb.Inventory{
			Identity:   buildNodeIdentity(),
			UnixTime:   timestamppb.Now(),
			Components: nil,
			Units:      detectUnits(ctx),
		},
	}
	return resp, nil
}

func (srv *NodeAgentServer) ApplyPlan(ctx context.Context, req *nodeagentpb.ApplyPlanRequest) (*nodeagentpb.ApplyPlanResponse, error) {
	if req == nil || req.GetPlan() == nil {
		return nil, status.Error(codes.InvalidArgument, "plan is required")
	}

	opID := strings.TrimSpace(req.GetOperationId())
	if opID == "" {
		opID = uuid.NewString()
	}
	op := srv.registerOperationWithID("apply plan", opID, req.GetPlan().GetProfiles())
	go srv.runPlan(ctx, op, req.GetPlan())
	return &nodeagentpb.ApplyPlanResponse{OperationId: op.id}, nil
}

func (srv *NodeAgentServer) ApplyPlanV1(ctx context.Context, req *nodeagentpb.ApplyPlanV1Request) (*nodeagentpb.ApplyPlanV1Response, error) {
	if req == nil || req.GetPlan() == nil {
		return nil, status.Error(codes.InvalidArgument, "plan is required")
	}
	if srv.planStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "plan store unavailable")
	}
	plan := proto.Clone(req.GetPlan()).(*planpb.NodePlan)
	if plan.GetNodeId() == "" {
		plan.NodeId = srv.nodeID
	}
	if srv.nodeID != "" && plan.GetNodeId() != "" && plan.GetNodeId() != srv.nodeID {
		return nil, status.Error(codes.InvalidArgument, "plan node_id does not match this agent")
	}
	planID := strings.TrimSpace(plan.GetPlanId())
	if planID == "" {
		planID = uuid.NewString()
		plan.PlanId = planID
	}
	opID := strings.TrimSpace(req.GetOperationId())
	if opID == "" {
		opID = planID
	}
	generation := plan.GetGeneration()
	if generation == 0 {
		generation = srv.lastPlanGeneration + 1
		if statusValue, err := srv.planStore.GetStatus(ctx, srv.nodeID); err == nil && statusValue != nil && statusValue.GetGeneration() >= generation {
			generation = statusValue.GetGeneration() + 1
		}
		if currentPlan, err := srv.planStore.GetCurrentPlan(ctx, srv.nodeID); err == nil && currentPlan != nil && currentPlan.GetGeneration() >= generation {
			generation = currentPlan.GetGeneration() + 1
		}
		plan.Generation = generation
	}
	if plan.GetCreatedUnixMs() == 0 {
		plan.CreatedUnixMs = uint64(time.Now().UnixMilli())
	}
	if err := srv.planStore.PutCurrentPlan(ctx, srv.nodeID, plan); err != nil {
		return nil, status.Errorf(codes.Internal, "persist plan: %v", err)
	}
	log.Printf("nodeagent: ApplyPlanV1 stored plan node=%s plan_id=%s gen=%d op_id=%s", plan.GetNodeId(), planID, plan.GetGeneration(), opID)
	return &nodeagentpb.ApplyPlanV1Response{
		OperationId:    opID,
		PlanId:         planID,
		PlanGeneration: plan.GetGeneration(),
	}, nil
}

func (srv *NodeAgentServer) GetPlanStatusV1(ctx context.Context, req *nodeagentpb.GetPlanStatusV1Request) (*nodeagentpb.GetPlanStatusV1Response, error) {
	if srv.planStore == nil {
		return nil, status.Error(codes.FailedPrecondition, "plan store unavailable")
	}
	targetNode := srv.nodeID
	if req != nil && strings.TrimSpace(req.GetNodeId()) != "" {
		if srv.nodeID != "" && req.GetNodeId() != srv.nodeID {
			return nil, status.Error(codes.InvalidArgument, "node_id does not match this agent")
		}
		targetNode = strings.TrimSpace(req.GetNodeId())
	}
	statusMsg, err := srv.planStore.GetStatus(ctx, targetNode)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fetch plan status: %v", err)
	}
	return &nodeagentpb.GetPlanStatusV1Response{Status: statusMsg}, nil
}

func (srv *NodeAgentServer) WatchPlanStatusV1(req *nodeagentpb.WatchPlanStatusV1Request, stream nodeagentpb.NodeAgentService_WatchPlanStatusV1Server) error {
	if srv.planStore == nil {
		return status.Error(codes.FailedPrecondition, "plan store unavailable")
	}
	targetNode := srv.nodeID
	if req != nil && strings.TrimSpace(req.GetNodeId()) != "" {
		if srv.nodeID != "" && req.GetNodeId() != srv.nodeID {
			return status.Error(codes.InvalidArgument, "node_id does not match this agent")
		}
		targetNode = strings.TrimSpace(req.GetNodeId())
	}
	var lastPayload []byte
	sendStatus := func(ctx context.Context) error {
		statusMsg, err := srv.planStore.GetStatus(ctx, targetNode)
		if err != nil {
			return status.Errorf(codes.Internal, "fetch plan status: %v", err)
		}
		if statusMsg == nil {
			return nil
		}
		current, err := proto.Marshal(statusMsg)
		if err != nil {
			return status.Errorf(codes.Internal, "encode status: %v", err)
		}
		if string(current) == string(lastPayload) {
			return nil
		}
		lastPayload = current
		return stream.Send(statusMsg)
	}

	if err := sendStatus(stream.Context()); err != nil {
		return err
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case <-ticker.C:
			if err := sendStatus(stream.Context()); err != nil {
				return err
			}
		}
	}
}

func (srv *NodeAgentServer) runPlan(ctx context.Context, op *operation, plan *clustercontrollerpb.NodePlan) {
	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_QUEUED, "plan queued", 0, false, ""))
	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, "plan running", 5, false, ""))

	netChanged, err := srv.applyRenderedConfig(plan)
	if err != nil {
		msg := err.Error()
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, msg, 10, true, msg))
		srv.notifyControllerOperationResult(op.id, false, msg, err)
		return
	}
	networkGen := networkGenerationFromPlan(plan)
	if networkGen == 0 {
		msg := "network generation missing from plan"
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, msg, 15, true, msg))
		srv.notifyControllerOperationResult(op.id, false, msg, errors.New(msg))
		return
	}
	log.Printf("nodeagent: network generation desired=%d current=%d", networkGen, srv.lastNetworkGeneration)
	spec, specErr := specFromPlan(plan)
	if specErr != nil {
		msg := fmt.Sprintf("parse desired spec: %v", specErr)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, msg, 18, true, msg))
		srv.notifyControllerOperationResult(op.id, false, msg, specErr)
		return
	}
	var desiredDomain string
	if spec != nil {
		desiredDomain = strings.TrimSpace(spec.GetClusterDomain())
	}
	if desiredDomain == "" {
		hasSpec := plan != nil && plan.GetRenderedConfig()["cluster.network.spec.json"] != ""
		msg := fmt.Sprintf("objectstore layout enforcement requires cluster domain, but none was found (spec_present=%t)", hasSpec)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, msg, 19, true, msg))
		srv.notifyControllerOperationResult(op.id, false, msg, errors.New(msg))
		return
	}
	netChanged = netChanged || (networkGen > 0 && networkGen != srv.lastNetworkGeneration)
	if netChanged {
		if err := srv.reconcileNetwork(ctx, plan, op, networkGen, desiredDomain, netChanged); err != nil {
			msg := err.Error()
			op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, msg, 20, true, msg))
			srv.notifyControllerOperationResult(op.id, false, msg, err)
			return
		}
	}

	actions := planner.ComputeActions(plan)
	total := len(actions)
	current := 0
	var lastPercent int32
	err = apply.ApplyActions(ctx, actions, func(action planner.Action) {
		lastPercent = percentForStep(current, total)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("%s %s", action.Op, action.Unit), lastPercent, false, ""))
		current++
	})
	if err != nil {
		msg := err.Error()
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, msg, lastPercent, true, msg))
		srv.notifyControllerOperationResult(op.id, false, msg, err)
		return
	}

	// Ensure objectstore layout (bucket + sentinels) is present; must succeed.
	layoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	log.Printf("Invoking ensure_objectstore_layout (domain=%s)", desiredDomain)
	if err := srv.ensureObjectstoreLayout(layoutCtx, desiredDomain); err != nil {
		msg := fmt.Sprintf("ensure objectstore layout: %v", err)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, msg, lastPercent, true, msg))
		srv.notifyControllerOperationResult(op.id, false, msg, err)
		return
	}

	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_SUCCEEDED, "plan applied", 100, true, ""))
	srv.lastNetworkGeneration = networkGen
	srv.state.NetworkGeneration = networkGen
	_ = srv.saveState()
	srv.notifyControllerOperationResult(op.id, true, "plan applied", nil)
}

func (srv *NodeAgentServer) ensureObjectstoreLayout(ctx context.Context, domain string) error {
	log.Printf("==== ensureObjectstoreLayout CALLED ====")
	log.Printf("  domain passed: %q", domain)

	if strings.TrimSpace(domain) == "" {
		return fmt.Errorf("objectstore layout enforcement requires cluster domain, but none was provided")
	}

	handler := actions.Get("ensure_objectstore_layout")
	if handler == nil {
		log.Printf("  ERROR: ensure_objectstore_layout handler not registered")
		return errors.New("ensure_objectstore_layout handler not registered")
	}

	contractPath := strings.TrimSpace(os.Getenv("GLOBULAR_MINIO_CONTRACT_PATH"))
	envOverride := false
	if contractPath == "" {
		contractPath = strings.TrimSpace(os.Getenv("NODE_AGENT_MINIO_CONTRACT"))
		envOverride = contractPath != ""
	} else {
		envOverride = true
	}
	if contractPath == "" {
		contractPath = "/var/lib/globular/objectstore/minio.json"
	}
	log.Printf("  contract_path: %s (env override: %t)", contractPath, envOverride)

	retry := 30
	if v := strings.TrimSpace(os.Getenv("OBJECTSTORE_RETRY")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retry = n
		}
	}
	retryDelay := 1000
	if v := strings.TrimSpace(os.Getenv("OBJECTSTORE_RETRY_DELAY_MS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			retryDelay = n
		}
	}

	if cfg := parseContractForLog(contractPath); cfg != nil {
		log.Printf("  minio endpoint=%s bucket=%s secure=%t", cfg.Endpoint, cfg.Bucket, cfg.Secure)
	}

	args, err := buildObjectstoreArgs(contractPath, domain, retry, retryDelay, true)
	if err != nil {
		log.Printf("  ERROR building args: %v", err)
		return fmt.Errorf("build args: %w", err)
	}
	if err := handler.Validate(args); err != nil {
		log.Printf("  ERROR validating: %v", err)
		return fmt.Errorf("validate ensure_objectstore_layout: %w", err)
	}
	msg, err := handler.Apply(ctx, args)
	if err != nil {
		log.Printf("  ERROR applying ensure_objectstore_layout: %v", err)
		return fmt.Errorf("apply ensure_objectstore_layout: %w", err)
	}
	log.Printf("  SUCCESS: %s", msg)
	log.Printf("==== ensureObjectstoreLayout COMPLETED ====")
	return nil
}

func buildObjectstoreArgs(contractPath, domain string, retry int, retryDelayMs int, strict bool) (*structpb.Struct, error) {
	fields := map[string]interface{}{
		"contract_path":    contractPath,
		"domain":           domain,
		"create_sentinels": true,
		"sentinel_name":    ".keep",
		"retry":            int64(retry),
		"retry_delay_ms":   int64(retryDelayMs),
		"strict_contract":  strict,
	}
	return structpb.NewStruct(fields)
}

type minioContractLog struct {
	Endpoint string
	Bucket   string
	Secure   bool
}

func parseContractForLog(path string) *minioContractLog {
	f, err := os.Open(path)
	if err != nil {
		log.Printf("  WARN: cannot read contract %s: %v", path, err)
		return nil
	}
	defer f.Close()
	var cfg config.MinioProxyConfig
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		log.Printf("  WARN: cannot parse contract %s: %v", path, err)
		return nil
	}
	return &minioContractLog{
		Endpoint: cfg.Endpoint,
		Bucket:   cfg.Bucket,
		Secure:   cfg.Secure,
	}
}

func (srv *NodeAgentServer) applyRenderedConfig(plan *clustercontrollerpb.NodePlan) (bool, error) {
	if plan == nil {
		return false, nil
	}
	rendered := plan.GetRenderedConfig()
	if len(rendered) == 0 {
		return false, nil
	}
	networkChanged := false
	for target, value := range rendered {
		if target == "" {
			continue
		}
		switch target {
		case "cluster.network.spec.json":
			if err := srv.writeNetworkSpecSnapshot(value); err != nil {
				return false, fmt.Errorf("write network spec snapshot: %w", err)
			}
			networkChanged = true
		case "/var/lib/globular/network.json":
			if err := srv.applyNetworkOverlay(target, value); err != nil {
				return false, fmt.Errorf("apply network overlay: %w", err)
			}
			networkChanged = true
		default:
			if !isAllowedRenderTarget(target) {
				log.Printf("nodeagent: render target %s not allowed; skipping", target)
				continue
			}
			if err := writeAtomicFile(target, []byte(value), 0o644); err != nil {
				return false, fmt.Errorf("write rendered config %s: %w", target, err)
			}
		}
	}
	return networkChanged, nil
}

func isAllowedRenderTarget(target string) bool {
	if target == "" {
		return false
	}
	if !filepath.IsAbs(target) {
		return false
	}
	clean := filepath.Clean(target)
	if strings.Contains(clean, "..") {
		return false
	}
	allowed := []string{
		"/var/lib/globular/",
		"/run/globular/",
		"/etc/globular/",
		"/etc/systemd/system/",
	}
	for _, prefix := range allowed {
		if clean == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(clean, prefix) {
			return true
		}
	}
	return false
}

func (srv *NodeAgentServer) writeNetworkSpecSnapshot(data string) error {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	path := filepath.Join(config.GetRuntimeConfigDir(), "cluster_network_spec.json")
	return writeAtomicFile(path, []byte(data), 0o600)
}

func (srv *NodeAgentServer) reconcileNetwork(ctx context.Context, plan *clustercontrollerpb.NodePlan, op *operation, generation uint64, desiredDomain string, networkChanged bool) error {
	if plan == nil {
		return nil
	}
	spec, err := specFromPlan(plan)
	if err != nil {
		return fmt.Errorf("parse desired spec: %w", err)
	}
	if desiredDomain == "" && spec != nil {
		desiredDomain = strings.TrimSpace(spec.GetClusterDomain())
	}
	if desiredDomain == "" {
		return fmt.Errorf("objectstore layout enforcement requires cluster domain, but none was found in reconcile")
	}
	if spec != nil && strings.EqualFold(spec.GetProtocol(), "https") && strings.TrimSpace(spec.GetClusterDomain()) == "" {
		return fmt.Errorf("cluster_domain is required when protocol=https")
	}

	if spec != nil && strings.TrimSpace(spec.GetClusterDomain()) == "" {
		return fmt.Errorf("cluster domain required for reconcile")
	}

	shouldSyncDNS := networkChanged
	if spec != nil {
		if strings.TrimSpace(spec.GetClusterDomain()) != strings.TrimSpace(srv.state.ClusterDomain) {
			shouldSyncDNS = true
		}
		if strings.ToLower(strings.TrimSpace(spec.GetProtocol())) != strings.ToLower(strings.TrimSpace(srv.state.Protocol)) {
			shouldSyncDNS = true
		}
	}

	if shouldSyncDNS {
		syncFn := srv.syncDNS
		if srv.syncDNSHook != nil {
			syncFn = srv.syncDNSHook
		}
		if err := syncFn(spec); err != nil {
			return fmt.Errorf("sync dns: %w", err)
		}
		waitFn := srv.waitForDNSAuthoritative
		if srv.waitDNSHook != nil {
			waitFn = srv.waitDNSHook
		}
		if err := waitFn(ctx, spec); err != nil {
			return fmt.Errorf("dns readiness: %w", err)
		}
		srv.state.ClusterDomain = spec.GetClusterDomain()
		srv.state.Protocol = spec.GetProtocol()
		if generation != 0 {
			srv.lastNetworkGeneration = generation
			srv.state.NetworkGeneration = generation
		}
		if err := srv.saveState(); err != nil {
			log.Printf("nodeagent: save state after dns sync: %v", err)
		}
	}

	if spec != nil && strings.EqualFold(spec.GetProtocol(), "https") {
		if spec.GetAcmeEnabled() && strings.TrimSpace(spec.GetAdminEmail()) == "" {
			return fmt.Errorf("admin_email is required for ACME")
		}
		if spec.GetAcmeEnabled() {
			if err := srv.acmeDNSPreflight(ctx, spec); err != nil {
				return fmt.Errorf("acme preflight: %w", err)
			}
		}
		certFn := srv.ensureNetworkCerts
		if srv.ensureCertsHook != nil {
			certFn = srv.ensureCertsHook
		}
		if err := certFn(spec); err != nil {
			return fmt.Errorf("ensure network certs: %w", err)
		}
	}

	units := orderRestartUnits(parseRestartUnits(plan))
	if len(units) > 0 {
		restartFn := srv.performRestartUnits
		if srv.restartHook != nil {
			restartFn = srv.restartHook
		}
		if err := restartFn(units, op); err != nil {
			return fmt.Errorf("restart units: %w", err)
		}
	}

	if spec != nil {
		checkFn := runConvergenceChecks
		if srv.healthCheckHook != nil {
			checkFn = srv.healthCheckHook
		}
		if err := checkFn(ctx, spec); err != nil {
			return fmt.Errorf("convergence checks failed: %w", err)
		}
	}

	if spec != nil && strings.TrimSpace(spec.ClusterDomain) != "" {
		srv.lastSpec = proto.Clone(spec).(*clustercontrollerpb.ClusterNetworkSpec)
		if strings.TrimSpace(spec.GetClusterDomain()) != "" {
			srv.state.ClusterDomain = spec.GetClusterDomain()
		}
		if strings.TrimSpace(spec.GetProtocol()) != "" {
			srv.state.Protocol = spec.GetProtocol()
		}
		layoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
		log.Printf("Invoking ensure_objectstore_layout (network reconcile, domain=%s)", spec.ClusterDomain)
		ensureFn := srv.ensureObjectstoreLayout
		if srv.objectstoreLayoutHook != nil {
			ensureFn = func(c context.Context, d string) error {
				return srv.objectstoreLayoutHook(c, d)
			}
		}
		if err := ensureFn(layoutCtx, spec.ClusterDomain); err != nil {
			return fmt.Errorf("ensure objectstore layout: %w", err)
		}
		if err := srv.saveState(); err != nil {
			log.Printf("nodeagent: save state after reconcile: %v", err)
		}
	}
	return nil
}

func specFromPlan(plan *clustercontrollerpb.NodePlan) (*clustercontrollerpb.ClusterNetworkSpec, error) {
	if plan == nil {
		return nil, nil
	}
	data := strings.TrimSpace(plan.GetRenderedConfig()["cluster.network.spec.json"])
	if data == "" {
		return nil, nil
	}
	spec := &clustercontrollerpb.ClusterNetworkSpec{}
	if err := protojson.Unmarshal([]byte(data), spec); err != nil {
		return nil, err
	}
	return spec, nil
}

func parseRestartUnits(plan *clustercontrollerpb.NodePlan) []string {
	if plan == nil {
		return nil
	}
	data := strings.TrimSpace(plan.GetRenderedConfig()["reconcile.restart_units"])
	if data == "" {
		return nil
	}
	var units []string
	if err := json.Unmarshal([]byte(data), &units); err != nil {
		log.Printf("nodeagent: invalid restart unit list: %v", err)
		return nil
	}
	return units
}

func orderRestartUnits(units []string) []string {
	priority := map[string]int{
		"globular-etcd.service":      1,
		"globular-minio.service":     2,
		"scylladb.service":           3,
		"globular-dns.service":       4,
		"globular-discovery.service": 5,
		"globular-xds.service":       6,
		"globular-envoy.service":     7,
		"globular-gateway.service":   8,
		"globular-storage.service":   9,
	}
	seen := map[string]struct{}{}
	type pair struct {
		unit string
		p    int
	}
	var ordered []pair
	for _, u := range units {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; ok {
			continue
		}
		seen[u] = struct{}{}
		p := 100
		if v, ok := priority[strings.ToLower(u)]; ok {
			p = v
		}
		ordered = append(ordered, pair{unit: u, p: p})
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].p == ordered[j].p {
			return ordered[i].unit < ordered[j].unit
		}
		return ordered[i].p < ordered[j].p
	})
	out := make([]string, 0, len(ordered))
	for _, p := range ordered {
		out = append(out, p.unit)
	}
	return out
}

func resolveUnits(units []string, exists func(string) bool) []string {
	aliasMap := map[string][]string{
		"globular-envoy.service":     {"envoy.service", "globular-envoy.service"},
		"globular-gateway.service":   {"gateway.service", "globular-gateway.service"},
		"globular-xds.service":       {"xds.service", "globular-xds.service"},
		"globular-etcd.service":      {"etcd.service", "globular-etcd.service"},
		"globular-minio.service":     {"minio.service", "globular-minio.service"},
		"globular-dns.service":       {"dns.service", "globular-dns.service"},
		"globular-discovery.service": {"discovery.service", "globular-discovery.service"},
		"globular-storage.service":   {"storage.service", "globular-storage.service"},
	}
	resolved := []string{}
	seen := map[string]struct{}{}
	for _, u := range units {
		original := strings.TrimSpace(u)
		if original == "" {
			continue
		}
		effective := original
		for canon, aliases := range aliasMap {
			match := strings.EqualFold(canon, original)
			if !match {
				for _, a := range aliases {
					if strings.EqualFold(a, original) {
						match = true
						break
					}
				}
			}
			if match {
				for _, cand := range append([]string{canon}, aliases...) {
					if exists != nil && exists(cand) {
						effective = cand
						break
					}
				}
				break
			}
		}
		if _, ok := seen[effective]; ok {
			continue
		}
		seen[effective] = struct{}{}
		if effective != original {
			log.Printf("nodeagent: resolved unit %s -> %s", original, effective)
		}
		resolved = append(resolved, effective)
	}
	return orderRestartUnits(resolved)
}

func networkGenerationFromPlan(plan *clustercontrollerpb.NodePlan) uint64 {
	if plan == nil {
		return 0
	}
	data := strings.TrimSpace(plan.GetRenderedConfig()["cluster.network.generation"])
	if data == "" {
		return 0
	}
	gen, err := strconv.ParseUint(data, 10, 64)
	if err != nil {
		return 0
	}
	return gen
}

func (srv *NodeAgentServer) acmeDNSPreflight(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
	if spec == nil || !strings.EqualFold(spec.GetProtocol(), "https") || !spec.GetAcmeEnabled() {
		return nil
	}
	if os.Getenv("GLOBULAR_ACME_PUBLIC_DNS_PREFLIGHT") != "1" {
		return nil
	}
	resolver := &net.Resolver{}
	if override := strings.TrimSpace(os.Getenv("GLOBULAR_DNS_RESOLVER")); override != "" {
		dialer := func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{}
			target := override
			if !strings.Contains(target, ":") {
				target = net.JoinHostPort(target, "53")
			}
			return d.DialContext(ctx, "udp", target)
		}
		resolver = &net.Resolver{
			PreferGo: true,
			Dial:     dialer,
		}
	}
	domains := []string{strings.TrimSpace(spec.GetClusterDomain())}
	for _, alt := range spec.GetAlternateDomains() {
		alt = strings.TrimSpace(alt)
		if alt != "" {
			domains = append(domains, alt)
		}
	}
	waitSeconds := 0
	if v := strings.TrimSpace(os.Getenv("ACME_DNS_WAIT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			waitSeconds = n
		}
	}
	deadline := time.Now().Add(time.Duration(waitSeconds) * time.Second)
	missing := []string{}
	for _, d := range domains {
		if d == "" {
			continue
		}
		name := "_acme-challenge." + d
		ok := false
		for {
			txt, err := resolver.LookupTXT(ctx, name)
			if err == nil && len(txt) > 0 {
				ok = true
			}
			if ok || waitSeconds == 0 || time.Now().After(deadline) {
				break
			}
			time.Sleep(time.Second)
		}
		if !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("ACME preflight failed: missing public DNS TXT record(s): %s. Create these _acme-challenge TXT records at your DNS provider and retry.", strings.Join(missing, ", "))
	}
	return nil
}

func (srv *NodeAgentServer) waitForDNSAuthoritative(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
	if spec == nil || strings.TrimSpace(spec.GetClusterDomain()) == "" {
		return fmt.Errorf("cluster domain required for dns readiness check")
	}
	domain := strings.TrimSpace(spec.GetClusterDomain())
	target := "gateway." + domain
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(c context.Context, network, address string) (net.Conn, error) {
			udpAddr := strings.TrimSpace(os.Getenv("GLOBULAR_DNS_UDP_ADDR"))
			if udpAddr == "" {
				udpAddr = "127.0.0.1:53"
			}
			d := net.Dialer{}
			return d.DialContext(c, "udp", udpAddr)
		},
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		_, err := resolver.LookupHost(ctx, target)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("dns not authoritative for %s: %v", target, err)
		}
		time.Sleep(time.Second)
	}
}

type certWatcherDeps struct {
	kv                   certs.KV
	writeTLS             func(certs.CertBundle) error
	restartUnits         func() error
	runConvergenceChecks func(context.Context) error
	now                  func() time.Time
	debounce             time.Duration
}

func runCertWatcherOnce(ctx context.Context, domain string, stateGen uint64, lastRestart time.Time, deps certWatcherDeps) (uint64, time.Time, bool, error) {
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.debounce == 0 {
		deps.debounce = 10 * time.Second
	}
	if deps.kv == nil || strings.TrimSpace(domain) == "" {
		return stateGen, lastRestart, false, nil
	}

	gen, err := deps.kv.GetBundleGeneration(ctx, domain)
	if err != nil || gen == 0 || gen <= stateGen {
		return stateGen, lastRestart, false, err
	}
	bundle, err := deps.kv.GetBundle(ctx, domain)
	if err != nil {
		return stateGen, lastRestart, false, err
	}
	if deps.writeTLS != nil {
		if err := deps.writeTLS(bundle); err != nil {
			return stateGen, lastRestart, false, err
		}
	}

	restarted := false
	if deps.restartUnits != nil && (lastRestart.IsZero() || deps.now().Sub(lastRestart) >= deps.debounce) {
		if err := deps.restartUnits(); err != nil {
			return gen, lastRestart, false, err
		}
		lastRestart = deps.now()
		restarted = true
	}

	if restarted && deps.runConvergenceChecks != nil {
		if err := deps.runConvergenceChecks(ctx); err != nil {
			return gen, lastRestart, restarted, err
		}
	}

	return gen, lastRestart, restarted, nil
}

func (srv *NodeAgentServer) pollCertGeneration(ctx context.Context) {
	if srv == nil || srv.state == nil {
		return
	}
	if strings.ToLower(strings.TrimSpace(srv.state.Protocol)) != "https" {
		return
	}
	domain := strings.TrimSpace(srv.state.ClusterDomain)
	if domain == "" {
		return
	}
	kv := srv.getCertKV()
	if kv == nil {
		return
	}
	tlsDir, fullchainDst, keyDst, caDst := config.CanonicalTLSPaths(config.GetRuntimeConfigDir())
	deps := certWatcherDeps{
		kv: kv,
		writeTLS: func(bundle certs.CertBundle) error {
			if err := os.MkdirAll(tlsDir, 0o755); err != nil {
				return err
			}
			return writeCertBundleFiles(bundle, keyDst, fullchainDst, caDst)
		},
		restartUnits: func() error {
			units := orderRestartUnits([]string{"globular-xds.service", "globular-envoy.service", "globular-gateway.service"})
			restartFn := srv.performRestartUnits
			if srv.restartHook != nil {
				restartFn = srv.restartHook
			}
			return restartFn(units, nil)
		},
		now:      time.Now,
		debounce: 10 * time.Second,
	}
	if spec := srv.lastSpec; spec != nil {
		deps.runConvergenceChecks = func(c context.Context) error {
			checkFn := runConvergenceChecks
			if srv.healthCheckHook != nil {
				checkFn = srv.healthCheckHook
			}
			if err := checkFn(c, spec); err != nil {
				return err
			}
			log.Printf("cert watcher: convergence checks passed")
			return nil
		}
	}

	newGen, newRestart, _, err := runCertWatcherOnce(ctx, domain, srv.state.CertGeneration, srv.lastCertRestart, deps)
	if err != nil {
		log.Printf("cert watcher: %v", err)
		return
	}
	if newGen > srv.state.CertGeneration {
		srv.state.CertGeneration = newGen
		if err := srv.saveState(); err != nil {
			log.Printf("cert watcher: save state: %v", err)
		}
	}
	srv.lastCertRestart = newRestart
}

func (srv *NodeAgentServer) ensureNetworkCerts(spec *clustercontrollerpb.ClusterNetworkSpec) error {
	if spec == nil || strings.ToLower(spec.GetProtocol()) != "https" {
		return nil
	}
	domain := strings.TrimSpace(spec.GetClusterDomain())
	if domain == "" {
		return errors.New("cluster_domain is required when protocol=https")
	}
	if spec.GetAcmeEnabled() && strings.TrimSpace(spec.GetAdminEmail()) == "" {
		return errors.New("admin_email is required for ACME")
	}
	dns := append([]string{domain}, spec.GetAlternateDomains()...)
	tlsDir, fullchainDst, keyDst, caDst := config.CanonicalTLSPaths(config.GetRuntimeConfigDir())
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		return fmt.Errorf("create tls dir: %w", err)
	}
	kv := srv.getCertKV()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	isLeader := true
	var release func()
	if kv != nil {
		lockLeader, unlock, err := kv.AcquireCertIssuerLock(ctx, domain, srv.nodeID, 30*time.Second)
		if err != nil {
			return fmt.Errorf("acquire cert issuer lock: %w", err)
		}
		isLeader = lockLeader
		release = unlock
	}
	if release != nil {
		defer release()
	}

	waitTimeout := 60 * time.Second
	if v := strings.TrimSpace(os.Getenv("CERT_WAIT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			waitTimeout = time.Duration(n) * time.Second
		}
	}

	if kv == nil && !srv.isIssuerNode() {
		if err := waitForFiles([]string{keyDst, fullchainDst, caDst}, waitTimeout); err != nil {
			return fmt.Errorf("wait for tls files: %w", err)
		}
		return nil
	}

	if kv != nil && !isLeader {
		bundle, err := kv.WaitForBundle(ctx, domain, waitTimeout)
		if err != nil {
			return fmt.Errorf("wait for tls bundle: %w", err)
		}
		if err := writeCertBundleFiles(bundle, keyDst, fullchainDst, caDst); err != nil {
			return fmt.Errorf("write cert bundle: %w", err)
		}
		if srv.state != nil {
			srv.state.CertGeneration = bundle.Generation
			_ = srv.saveState()
		}
		log.Printf("nodeagent: fetched cert bundle for %s generation %d", domain, bundle.Generation)
		return nil
	}

	opts := pki.Options{
		Storage: pki.FileStorage{},
		LocalCA: pki.LocalCAConfig{
			Enabled: true,
		},
	}
	if spec.GetAcmeEnabled() {
		opts.ACME = pki.ACMEConfig{
			Enabled:  true,
			Email:    strings.TrimSpace(spec.GetAdminEmail()),
			Domain:   domain,
			Provider: "globular",
			DNS:      strings.TrimSpace(os.Getenv("GLOBULAR_DNS_ADDR")),
		}
		if opts.ACME.DNS == "" {
			opts.ACME.DNS = "127.0.0.1:10033"
		}
	}
	workDir := filepath.Join(tlsDir, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("create tls work dir: %w", err)
	}

	manager := networkPKIManager(opts)
	var bundle certs.CertBundle
	if spec.GetAcmeEnabled() {
		subject := fmt.Sprintf("CN=%s", domain)
		keyFile, _, issuerFile, fullchainFile, err := manager.EnsurePublicACMECert(workDir, domain, subject, dns, 90*24*time.Hour)
		if err != nil {
			return fmt.Errorf("issue ACME certs: %w", err)
		}
		if err := copyFilePerm(keyFile, keyDst, 0o600); err != nil {
			return err
		}
		if err := copyFilePerm(fullchainFile, fullchainDst, 0o644); err != nil {
			return err
		}
		if err := copyFilePerm(issuerFile, caDst, 0o644); err != nil {
			return err
		}
	} else {
		keyFile, leafFile, caFile, err := manager.EnsureServerCert(workDir, domain, dns, 90*24*time.Hour)
		if err != nil {
			return fmt.Errorf("issue server certs: %w", err)
		}
		if err := copyFilePerm(keyFile, keyDst, 0o600); err != nil {
			return err
		}
		if err := concatFiles(fullchainDst, leafFile, caFile); err != nil {
			return fmt.Errorf("build fullchain: %w", err)
		}
		if caFile != "" {
			if err := copyFilePerm(caFile, caDst, 0o644); err != nil {
				return err
			}
		}
	}

	keyBytes, err := os.ReadFile(keyDst)
	if err != nil {
		return fmt.Errorf("read key for publish: %w", err)
	}
	fullchainBytes, err := os.ReadFile(fullchainDst)
	if err != nil {
		return fmt.Errorf("read fullchain for publish: %w", err)
	}
	caBytes, _ := os.ReadFile(caDst)
	bundle = certs.CertBundle{
		Key:        keyBytes,
		Fullchain:  fullchainBytes,
		CA:         caBytes,
		Generation: uint64(time.Now().UnixNano()),
		UpdatedMS:  time.Now().UnixMilli(),
	}

	if kv != nil {
		if err := kv.PutBundle(ctx, domain, bundle); err != nil {
			log.Printf("nodeagent: failed to publish cert bundle: %v", err)
		} else {
			log.Printf("nodeagent: published cert bundle for %s generation %d", domain, bundle.Generation)
			srv.state.CertGeneration = bundle.Generation
			_ = srv.saveState()
		}
	}
	return nil
}

func (srv *NodeAgentServer) isIssuerNode() bool {
	issuer := strings.TrimSpace(os.Getenv("GLOBULAR_CERT_ISSUER_NODE"))
	if issuer == "" {
		issuer = "node-0"
	}
	if srv == nil || strings.TrimSpace(srv.nodeID) == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(srv.nodeID), issuer)
}

func waitForFiles(paths []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		missing := []string{}
		for _, p := range paths {
			info, err := os.Stat(p)
			if err != nil || info.Size() == 0 {
				missing = append(missing, p)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("missing files after wait: %s", strings.Join(missing, ", "))
		}
		time.Sleep(time.Second)
	}
}

func (srv *NodeAgentServer) getCertKV() certs.KV {
	if srv.certKV != nil {
		return srv.certKV
	}
	ps, ok := srv.planStore.(lockablePlanStore)
	if !ok || ps.Client() == nil {
		return nil
	}
	srv.certKV = certs.NewEtcdKV(ps.Client())
	return srv.certKV
}

func writeCertBundleFiles(bundle certs.CertBundle, keyDst, fullchainDst, caDst string) error {
	if err := writeAtomicFile(keyDst, bundle.Key, 0o600); err != nil {
		return err
	}
	if err := writeAtomicFile(fullchainDst, bundle.Fullchain, 0o644); err != nil {
		return err
	}
	if len(bundle.CA) > 0 {
		if err := writeAtomicFile(caDst, bundle.CA, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (srv *NodeAgentServer) performRestartUnits(units []string, op *operation) error {
	if len(units) == 0 {
		return nil
	}
	systemctl, err := systemctlLookPath("systemctl")
	if err != nil {
		return fmt.Errorf("systemctl lookup: %w", err)
	}
	var errs []string
	resolved := resolveUnits(units, func(u string) bool {
		return systemdUnitExists(systemctl, u) == nil
	})
	for idx, unit := range resolved {
		percent := int32(30 + idx*10)
		if percent > 95 {
			percent = 95
		}
		if op != nil {
			op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("restart %s", unit), percent, false, ""))
		}
		if err := restartCommand(systemctl, unit); err != nil {
			log.Printf("nodeagent: %s reload/restart: %v", unit, err)
			var details string
			journal, jerr := exec.Command(systemctl, "status", unit, "--no-pager", "-n", "50").CombinedOutput()
			if jerr == nil {
				details = string(journal)
			}
			errs = append(errs, fmt.Sprintf("%s: %v %s", unit, err, details))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("restart failures: %s", strings.Join(errs, "; "))
	}
	return nil
}

func restartUnit(systemctl, unit string) error {
	if err := systemdUnitExists(systemctl, unit); err != nil {
		return err
	}
	if err := runSystemctl(systemctl, "reload", unit); err != nil {
		if err := runSystemctl(systemctl, "restart", unit); err != nil {
			return err
		}
	}
	return nil
}

func (srv *NodeAgentServer) applyNetworkOverlay(target, data string) error {
	if strings.TrimSpace(data) == "" {
		return nil
	}
	if err := writeAtomicFile(target, []byte(data), 0o644); err != nil {
		return fmt.Errorf("write network overlay %s: %w", target, err)
	}
	if err := mergeNetworkIntoConfig(config.GetAdminConfigPath(), data); err != nil {
		return fmt.Errorf("merge network overlay: %w", err)
	}
	return nil
}

func mergeNetworkIntoConfig(basePath, overlay string) error {
	if strings.TrimSpace(overlay) == "" {
		return nil
	}
	var overlayData map[string]interface{}
	if err := json.Unmarshal([]byte(overlay), &overlayData); err != nil {
		return fmt.Errorf("parse overlay: %w", err)
	}
	if len(overlayData) == 0 {
		return nil
	}
	base := make(map[string]interface{})
	data, err := os.ReadFile(basePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read base config: %w", err)
		}
	} else if len(data) > 0 {
		if err := json.Unmarshal(data, &base); err != nil {
			return fmt.Errorf("parse base config: %w", err)
		}
	}
	if base == nil {
		base = make(map[string]interface{})
	}
	allowed := map[string]struct{}{
		"Domain":           {},
		"Protocol":         {},
		"PortHTTP":         {},
		"PortHTTPS":        {},
		"ACMEEnabled":      {},
		"AdminEmail":       {},
		"AlternateDomains": {},
	}
	for key, value := range overlayData {
		if _, ok := allowed[key]; ok {
			base[key] = value
		}
	}
	merged, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal merged config: %w", err)
	}
	if err := writeAtomicFile(basePath, merged, 0o644); err != nil {
		return fmt.Errorf("write merged config: %w", err)
	}
	return nil
}

func (srv *NodeAgentServer) notifyControllerOperationResult(operationID string, success bool, message string, opErr error) {
	if srv.controllerEndpoint == "" || operationID == "" || srv.nodeID == "" {
		return
	}
	if srv.controllerClient == nil {
		if err := srv.ensureControllerClient(context.Background()); err != nil {
			log.Printf("controller client unavailable: %v", err)
			return
		}
	}
	payload := &clustercontrollerpb.CompleteOperationRequest{
		OperationId: operationID,
		NodeId:      srv.nodeID,
		Success:     success,
		Message:     message,
	}
	if opErr != nil {
		payload.Error = opErr.Error()
		if payload.Message == "" {
			payload.Message = "plan failed"
		}
	}
	if success && payload.Percent == 0 {
		payload.Percent = 100
		if payload.Message == "" {
			payload.Message = "plan applied"
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := srv.controllerClient.CompleteOperation(ctx, payload); err != nil {
		log.Printf("notify controller operation %s completion: %v", operationID, err)
	}
}

func buildHealthChecks(spec *clustercontrollerpb.ClusterNetworkSpec) []healthchecks.Check {
	if spec == nil {
		return nil
	}
	httpPort := spec.GetPortHttp()
	httpsPort := spec.GetPortHttps()
	domain := strings.TrimSpace(spec.GetClusterDomain())

	checks := []healthchecks.Check{
		{
			Name:           "minio",
			URL:            firstNonEmpty(strings.TrimSpace(os.Getenv("GLOBULAR_HEALTH_MINIO_URL")), "http://127.0.0.1:9000/minio/health/ready"),
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
		},
		{
			Name:           "envoy-admin",
			URL:            firstNonEmpty(strings.TrimSpace(os.Getenv("GLOBULAR_HEALTH_ENVOY_URL")), "http://127.0.0.1:9901/ready"),
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
		},
	}
	if strings.EqualFold(spec.GetProtocol(), "https") {
		checks = append(checks, healthchecks.Check{
			Name:           "gateway-https",
			URL:            firstNonEmpty(strings.TrimSpace(os.Getenv("GLOBULAR_HEALTH_GATEWAY_URL")), fmt.Sprintf("https://127.0.0.1:%d/health", httpsPort)),
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
			InsecureTLS:    true,
			HostHeader:     domain,
		})
	} else {
		checks = append(checks, healthchecks.Check{
			Name:           "gateway-http",
			URL:            firstNonEmpty(strings.TrimSpace(os.Getenv("GLOBULAR_HEALTH_GATEWAY_URL")), fmt.Sprintf("http://127.0.0.1:%d/health", httpPort)),
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
			HostHeader:     domain,
		})
	}
	return checks
}

func runConvergenceChecks(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
	if spec == nil {
		return nil
	}
	if err := healthchecks.RunChecks(ctx, buildHealthChecks(spec)); err != nil {
		return err
	}
	if err := runSupplementalChecks(ctx, spec); err != nil {
		return err
	}
	return nil
}

var dnsLookupHost = func(ctx context.Context, resolver *net.Resolver, host string) ([]string, error) {
	return resolver.LookupHost(ctx, host)
}

func runSupplementalChecks(ctx context.Context, spec *clustercontrollerpb.ClusterNetworkSpec) error {
	var errs []string
	domain := strings.TrimSpace(spec.GetClusterDomain())
	if domain == "" {
		errs = append(errs, "dns: empty domain")
	} else {
		dnsAddr := strings.TrimSpace(os.Getenv("GLOBULAR_DNS_UDP_ADDR"))
		if dnsAddr == "" {
			dnsAddr = "127.0.0.1:53"
		}
		resolver := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return net.DialTimeout("udp", dnsAddr, 3*time.Second)
			},
		}
		target := fmt.Sprintf("gateway.%s", domain)
		if _, err := dnsLookupHost(ctx, resolver, target); err != nil {
			errs = append(errs, fmt.Sprintf("dns lookup %s failed: %v", target, err))
		}
	}

	addrs := []struct {
		name string
		addr string
	}{
		{"etcd", firstNonEmpty(os.Getenv("GLOBULAR_ETCD_ADDR"), "127.0.0.1:2379")},
		{"minio-tcp", firstNonEmpty(os.Getenv("GLOBULAR_MINIO_ADDR"), "127.0.0.1:9000")},
		{"scylla", firstNonEmpty(os.Getenv("GLOBULAR_SCYLLA_ADDR"), "127.0.0.1:9042")},
	}
	for _, a := range addrs {
		d := net.Dialer{Timeout: 3 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", a.addr)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s dial %s: %v", a.name, a.addr, err))
			continue
		}
		conn.Close()
	}

	if len(errs) > 0 {
		return fmt.Errorf("supplemental health failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func writeAtomicFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		tmp.Close()
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}
	defer cleanup()
	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if err := os.Chmod(path, perm); err != nil {
		return err
	}
	tmpName = ""
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirFile.Close()
	if err := dirFile.Sync(); err != nil {
		return err
	}
	return nil
}

func copyFilePerm(src, dst string, perm os.FileMode) error {
	if src == "" {
		return fmt.Errorf("source file is empty")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := writeAtomicFile(dst, data, perm); err != nil {
		return err
	}
	return os.Chmod(dst, perm)
}

func concatFiles(dst string, parts ...string) error {
	if len(parts) == 0 {
		return fmt.Errorf("no parts to concat")
	}
	var out []byte
	for _, part := range parts {
		if part == "" {
			continue
		}
		data, err := os.ReadFile(part)
		if err != nil {
			return err
		}
		out = append(out, data...)
	}
	if len(out) == 0 {
		return fmt.Errorf("no content to write")
	}
	return writeAtomicFile(dst, out, 0o644)
}

func systemdUnitExists(systemctl, unit string) error {
	cmd := exec.Command(systemctl, "show", "--property=LoadState", unit)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
		}
		return err
	}
	return nil
}

func runSystemctl(systemctl, action, unit string) error {
	cmd := exec.Command(systemctl, action, unit)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return fmt.Errorf("%w: %s", err, trimmed)
		}
		return err
	}
	return nil
}

func (srv *NodeAgentServer) WatchOperation(req *nodeagentpb.WatchOperationRequest, stream nodeagentpb.NodeAgentService_WatchOperationServer) error {
	if req == nil || strings.TrimSpace(req.GetOperationId()) == "" {
		return status.Error(codes.InvalidArgument, "operation_id is required")
	}
	op := srv.getOperation(req.GetOperationId())
	if op == nil {
		return status.Error(codes.NotFound, "operation not found")
	}

	ch, last := op.subscribe()
	defer op.unsubscribe(ch)

	if last != nil && last.Done {
		if err := stream.Send(last); err != nil {
			return err
		}
		return nil
	}

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case evt := <-ch:
			if evt == nil {
				continue
			}
			if err := stream.Send(evt); err != nil {
				return err
			}
			if evt.Done {
				return nil
			}
		}
	}
}

func (srv *NodeAgentServer) BootstrapFirstNode(ctx context.Context, req *nodeagentpb.BootstrapFirstNodeRequest) (*nodeagentpb.BootstrapFirstNodeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	profiles := append([]string(nil), req.GetProfiles()...)
	if len(profiles) == 0 {
		profiles = []string{"control-plane", "gateway"}
	}

	// Store controller endpoint if provided
	controllerEndpoint := strings.TrimSpace(req.GetControllerBind())
	if controllerEndpoint != "" {
		srv.controllerEndpoint = controllerEndpoint
		srv.state.ControllerEndpoint = controllerEndpoint
		if err := srv.saveState(); err != nil {
			log.Printf("warn: persist controller endpoint: %v", err)
		}
	}

	// Create a plan with both unit actions and network configuration
	plan := srv.buildBootstrapPlanWithNetwork(profiles, req.GetClusterDomain())

	op := srv.registerOperation("bootstrap node", profiles)
	go srv.runPlan(ctx, op, plan)

	return &nodeagentpb.BootstrapFirstNodeResponse{
		OperationId: op.id,
		JoinToken:   srv.joinToken,
		Message:     "bootstrap initiated with network configuration",
	}, nil
}

func (srv *NodeAgentServer) registerOperation(kind string, profiles []string) *operation {
	return srv.registerOperationWithID(kind, uuid.NewString(), profiles)
}

func (srv *NodeAgentServer) registerOperationWithID(kind, id string, profiles []string) *operation {
	op := &operation{
		id:          id,
		kind:        kind,
		profiles:    append([]string(nil), profiles...),
		subscribers: make(map[chan *nodeagentpb.OperationEvent]struct{}),
	}
	srv.mu.Lock()
	srv.operations[id] = op
	srv.mu.Unlock()
	return op
}

func (srv *NodeAgentServer) getOperation(id string) *operation {
	srv.mu.Lock()
	defer srv.mu.Unlock()
	return srv.operations[id]
}

func (srv *NodeAgentServer) startOperation(op *operation, message string) {
	go func() {
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_QUEUED, fmt.Sprintf("%s queued", message), 0, false, ""))
		time.Sleep(100 * time.Millisecond)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("%s started", message), 5, false, ""))

		total := len(op.profiles)
		for idx, profile := range op.profiles {
			time.Sleep(250 * time.Millisecond)
			percent := percentForStep(idx, total)
			op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("profile %s applied", profile), percent, false, ""))
		}

		time.Sleep(200 * time.Millisecond)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_SUCCEEDED, fmt.Sprintf("%s complete", message), 100, true, ""))
	}()
}

func percentForStep(idx, total int) int32 {
	if total <= 0 {
		return 50
	}
	base := int32(20)
	step := int32(60 / total)
	res := base + step*int32(idx+1)
	if res > 95 {
		return 95
	}
	return res
}

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func gatherIPs() []string {
	var ips []string
	seen := make(map[string]struct{})
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		// Skip down or loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			// Skip nil, loopback, or IPv6 addresses
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			text := ip.String()
			if _, ok := seen[text]; ok {
				continue
			}
			seen[text] = struct{}{}
			ips = append(ips, text)
		}
	}

	// Sort IPs: prefer private network addresses (10.x, 172.16-31.x, 192.168.x) first
	sort.Slice(ips, func(i, j int) bool {
		ipI := net.ParseIP(ips[i])
		ipJ := net.ParseIP(ips[j])
		if ipI == nil || ipJ == nil {
			return ips[i] < ips[j]
		}

		privateI := isPrivateIP(ipI)
		privateJ := isPrivateIP(ipJ)

		// Private IPs come first
		if privateI != privateJ {
			return privateI
		}

		// Otherwise, sort by IP string
		return ips[i] < ips[j]
	})

	return ips
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	ip = ip.To4()
	if ip == nil {
		return false
	}

	// 10.0.0.0/8
	if ip[0] == 10 {
		return true
	}
	// 172.16.0.0/12
	if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
		return true
	}
	// 192.168.0.0/16
	if ip[0] == 192 && ip[1] == 168 {
		return true
	}
	return false
}

func buildNodeIdentity() *clustercontrollerpb.NodeIdentity {
	hostname, _ := os.Hostname()
	return &clustercontrollerpb.NodeIdentity{
		Hostname:     hostname,
		Domain:       os.Getenv("NODE_AGENT_DOMAIN"),
		Ips:          gatherIPs(),
		Os:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		AgentVersion: getEnv("NODE_AGENT_VERSION", "v0.1.0"),
	}
}

func detectUnits(ctx context.Context) []*nodeagentpb.UnitStatus {
	if ctx == nil {
		ctx = context.Background()
	}
	known := []string{
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
		"globular-event.service",
		"globular-rbac.service",
		"globular-file.service",
		"globular-minio.service",
		"globular-gateway.service",
		"globular-xds.service",
		"envoy.service",
	}
	statuses := make([]*nodeagentpb.UnitStatus, 0, len(known))
	for _, unit := range known {
		state := "unknown"
		details := ""
		unitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		active, err := supervisor.IsActive(unitCtx, unit)
		cancel()
		if err != nil {
			details = err.Error()
		} else {
			if active {
				state = "active"
			} else {
				state = "inactive"
			}
			statusCtx, statusCancel := context.WithTimeout(ctx, 2*time.Second)
			if out, err := supervisor.Status(statusCtx, unit); err == nil {
				details = out
			} else if details == "" {
				details = err.Error()
			}
			statusCancel()
		}
		statuses = append(statuses, &nodeagentpb.UnitStatus{
			Name:    unit,
			State:   state,
			Details: details,
		})
	}
	return statuses
}

func buildBootstrapPlan(services []string) *clustercontrollerpb.NodePlan {
	actions := make([]*clustercontrollerpb.UnitAction, 0, len(services))
	for _, svc := range services {
		unit := units.UnitForService(svc)
		if unit == "" {
			continue
		}
		actions = append(actions, &clustercontrollerpb.UnitAction{
			UnitName: unit,
			Action:   "start",
		})
	}
	if len(actions) == 0 {
		return &clustercontrollerpb.NodePlan{
			Profiles: []string{"bootstrap"},
		}
	}
	return &clustercontrollerpb.NodePlan{
		Profiles:    []string{"bootstrap"},
		UnitActions: actions,
	}
}

func (srv *NodeAgentServer) buildBootstrapPlanWithNetwork(profiles []string, clusterDomain string) *clustercontrollerpb.NodePlan {
	// Build unit actions from profiles
	plan := buildBootstrapPlan(profiles)
	plan.Profiles = append([]string(nil), profiles...)

	// Add network configuration if domain is provided
	domain := strings.TrimSpace(clusterDomain)
	if domain == "" {
		return plan
	}

	// Create default network spec for bootstrap
	spec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: domain,
		Protocol:      "http", // Default to http for bootstrap
		PortHttp:      8080,
		PortHttps:     8443,
		AcmeEnabled:   false,
		AdminEmail:    "",
	}

	// Build rendered config
	rendered := make(map[string]string)

	// Add network spec snapshot
	if specJSON, err := protojson.Marshal(spec); err == nil {
		rendered["cluster.network.spec.json"] = string(specJSON)
	}

	// Add network overlay
	configPayload := map[string]interface{}{
		"Domain":    spec.ClusterDomain,
		"Protocol":  spec.Protocol,
		"PortHTTP":  spec.PortHttp,
		"PortHTTPS": spec.PortHttps,
	}
	if cfgJSON, err := json.MarshalIndent(configPayload, "", "  "); err == nil {
		rendered["/var/lib/globular/network.json"] = string(cfgJSON)
	}

	// Add network generation (bootstrap starts at 1)
	rendered["cluster.network.generation"] = "1"

	// Add restart units for network config
	restartUnits := []string{
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
		"globular-xds.service",
		"globular-envoy.service",
		"globular-gateway.service",
		"globular-minio.service",
	}
	if unitsJSON, err := json.Marshal(restartUnits); err == nil {
		rendered["reconcile.restart_units"] = string(unitsJSON)
	}

	plan.RenderedConfig = rendered
	return plan
}

func (srv *NodeAgentServer) saveState() error {
	if srv.statePath == "" {
		return nil
	}
	srv.stateMu.Lock()
	defer srv.stateMu.Unlock()
	if srv.state == nil {
		srv.state = newNodeAgentState()
	}
	srv.state.ControllerEndpoint = srv.controllerEndpoint
	srv.state.RequestID = srv.joinRequestID
	srv.state.NodeID = srv.nodeID
	srv.state.LastPlanGeneration = srv.lastPlanGeneration
	srv.state.NetworkGeneration = srv.lastNetworkGeneration
	return srv.state.save(srv.statePath)
}

func (srv *NodeAgentServer) startJoinApprovalWatcher(ctx context.Context, requestID string) {
	if requestID == "" {
		return
	}
	srv.joinPollMu.Lock()
	if srv.joinPollCancel != nil {
		srv.joinPollCancel()
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	srv.joinPollCancel = cancel
	srv.joinPollMu.Unlock()
	go srv.watchJoinStatus(ctx, requestID)
}

func (srv *NodeAgentServer) watchJoinStatus(ctx context.Context, requestID string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := srv.ensureControllerClient(ctx); err != nil {
			log.Printf("join status: controller unreachable: %v", err)
			if !waitOrDone(ctx, ticker) {
				return
			}
			continue
		}
		resp, err := srv.controllerClient.GetJoinRequestStatus(ctx, &clustercontrollerpb.GetJoinRequestStatusRequest{
			RequestId: requestID,
		})
		if err != nil {
			log.Printf("join status poll error: %v", err)
			if !waitOrDone(ctx, ticker) {
				return
			}
			continue
		}
		switch strings.ToLower(resp.GetStatus()) {
		case "approved":
			if nodeID := resp.GetNodeId(); nodeID != "" {
				srv.applyApprovedNodeID(nodeID)
				log.Printf("join request %s approved (node %s)", requestID, nodeID)
			}
			return
		case "rejected":
			log.Printf("join request %s rejected: %s", requestID, resp.GetMessage())
			return
		}
		if !waitOrDone(ctx, ticker) {
			return
		}
	}
}

func waitOrDone(ctx context.Context, ticker *time.Ticker) bool {
	select {
	case <-ctx.Done():
		return false
	case <-ticker.C:
		return true
	}
}

func (srv *NodeAgentServer) applyApprovedNodeID(nodeID string) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return
	}
	srv.stateMu.Lock()
	srv.nodeID = nodeID
	srv.state.NodeID = nodeID
	srv.state.RequestID = ""
	srv.joinRequestID = ""
	srv.stateMu.Unlock()
	if err := srv.saveState(); err != nil {
		log.Printf("warn: persist approved node id: %v", err)
	}
	srv.startPlanRunnerLoop()
}

func parseNodeAgentLabels() map[string]string {
	raw := strings.TrimSpace(os.Getenv("NODE_AGENT_LABELS"))
	if raw == "" {
		return nil
	}
	labels := make(map[string]string)
	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		if pair = strings.TrimSpace(pair); pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			continue
		}
		labels[key] = value
	}
	if len(labels) == 0 {
		return nil
	}
	return labels
}

func convertNodeAgentUnits(units []*nodeagentpb.UnitStatus) []*clustercontrollerpb.NodeUnitStatus {
	if len(units) == 0 {
		return nil
	}
	out := make([]*clustercontrollerpb.NodeUnitStatus, 0, len(units))
	for _, unit := range units {
		if unit == nil {
			continue
		}
		out = append(out, &clustercontrollerpb.NodeUnitStatus{
			Name:    unit.GetName(),
			State:   unit.GetState(),
			Details: unit.GetDetails(),
		})
	}
	return out
}

type operation struct {
	id       string
	kind     string
	profiles []string

	mu          sync.Mutex
	subscribers map[chan *nodeagentpb.OperationEvent]struct{}
	lastEvent   *nodeagentpb.OperationEvent
}

func (op *operation) subscribe() (chan *nodeagentpb.OperationEvent, *nodeagentpb.OperationEvent) {
	ch := make(chan *nodeagentpb.OperationEvent, 4)
	op.mu.Lock()
	op.subscribers[ch] = struct{}{}
	last := op.lastEvent
	op.mu.Unlock()
	return ch, last
}

func (op *operation) unsubscribe(ch chan *nodeagentpb.OperationEvent) {
	op.mu.Lock()
	delete(op.subscribers, ch)
	op.mu.Unlock()
}

func (op *operation) broadcast(evt *nodeagentpb.OperationEvent) {
	op.mu.Lock()
	op.lastEvent = evt
	subs := make([]chan *nodeagentpb.OperationEvent, 0, len(op.subscribers))
	for ch := range op.subscribers {
		subs = append(subs, ch)
	}
	op.mu.Unlock()

	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (op *operation) newEvent(phase clustercontrollerpb.OperationPhase, message string, percent int32, done bool, errStr string) *nodeagentpb.OperationEvent {
	return &nodeagentpb.OperationEvent{
		OperationId: op.id,
		Phase:       phase,
		Message:     message,
		Percent:     percent,
		Done:        done,
		Error:       errStr,
	}
}
