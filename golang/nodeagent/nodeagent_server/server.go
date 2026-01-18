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
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/actions"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/apply"
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

func NewNodeAgentServer(statePath string, state *nodeAgentState) *NodeAgentServer {
	if state == nil {
		state = newNodeAgentState()
	}
	port := getEnv("NODE_AGENT_PORT", defaultPort)
	advertised := strings.TrimSpace(os.Getenv("NODE_AGENT_ADVERTISE_ADDR"))
	if advertised == "" {
		if ips := gatherIPs(); len(ips) > 0 {
			advertised = fmt.Sprintf("%s:%s", ips[0], port)
		} else {
			// Fallback to first non-loopback IP or localhost
			advertised = fmt.Sprintf("localhost:%s", port)
		}
	}
	useInsecure := strings.EqualFold(getEnv("NODE_AGENT_INSECURE", "false"), "true")
	controllerEndpoint := strings.TrimSpace(os.Getenv("NODE_AGENT_CONTROLLER_ENDPOINT"))
	if controllerEndpoint == "" {
		controllerEndpoint = state.ControllerEndpoint
	} else {
		state.ControllerEndpoint = controllerEndpoint
	}
	nodeID := state.NodeID
	if nodeID == "" {
		nodeID = strings.TrimSpace(os.Getenv("NODE_AGENT_NODE_ID"))
		state.NodeID = nodeID
	}
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
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.pollPlan(ctx)
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
	if plan == nil {
		return
	}
	if plan.GetNodeId() != "" && plan.GetNodeId() != srv.nodeID {
		return
	}
	if plan.GetGeneration() <= srv.lastPlanGeneration {
		return
	}
	now := time.Now().UnixMilli()
	if plan.GetExpiresUnixMs() > 0 && now > int64(plan.GetExpiresUnixMs()) {
		srv.markPlanExpired(pollCtx, plan)
		return
	}
	srv.runStoredPlan(ctx, plan)
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
		txn := client.Txn(ctx).If(clientv3.Compare(clientv3.CreateRevision(key), "=", 0)).
			Then(clientv3.OpPut(key, "", clientv3.WithLease(leaseResp.ID)))
		resp, err := txn.Commit()
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

func (srv *NodeAgentServer) runStoredPlan(ctx context.Context, plan *planpb.NodePlan) {
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
		status := srv.newPlanStatus(plan)
		status.State = planpb.PlanState_PLAN_FAILED
		status.ErrorMessage = msg
		srv.addPlanEvent(status, "error", msg, "")
		srv.publishPlanStatus(ctx, status)
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

	status := srv.newPlanStatus(plan)
	status.State = planpb.PlanState_PLAN_RUNNING
	status.StartedUnixMs = uint64(time.Now().UnixMilli())
	srv.publishPlanStatus(ctx, status)

	var steps []*planpb.PlanStep
	if spec := plan.GetSpec(); spec != nil {
		steps = spec.GetSteps()
	}
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
		srv.addPlanEvent(status, "info", fmt.Sprintf("step %s running", step.GetId()), step.GetId())
		srv.publishPlanStatus(ctx, status)

		percent := percentForStep(idx, total)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("step %s running", step.GetId()), percent, false, ""))

		if err := srv.executePlanStep(ctx, step); err != nil {
			stepStatus.State = planpb.StepState_STEP_FAILED
			stepStatus.FinishedUnixMs = uint64(time.Now().UnixMilli())
			status.State = planpb.PlanState_PLAN_FAILED
			status.ErrorMessage = err.Error()
			status.ErrorStepId = step.GetId()
			status.FinishedUnixMs = uint64(time.Now().UnixMilli())
			srv.addPlanEvent(status, "error", fmt.Sprintf("step %s failed: %v", step.GetId(), err), step.GetId())
			srv.publishPlanStatus(ctx, status)
			op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, err.Error(), percent, true, err.Error()))
			if plan.GetPolicy().GetFailureMode() == planpb.FailureMode_FAILURE_MODE_ROLLBACK {
				if rbErr := srv.performRollback(ctx, plan, status, op); rbErr != nil {
					log.Printf("rollback failed: %v", rbErr)
				}
			}
			return
		}

		stepStatus.State = planpb.StepState_STEP_OK
		stepStatus.FinishedUnixMs = uint64(time.Now().UnixMilli())
		srv.addPlanEvent(status, "info", fmt.Sprintf("step %s succeeded", step.GetId()), step.GetId())
		srv.publishPlanStatus(ctx, status)
	}

	status.State = planpb.PlanState_PLAN_SUCCEEDED
	status.FinishedUnixMs = uint64(time.Now().UnixMilli())
	status.CurrentStepId = ""
	srv.addPlanEvent(status, "info", "plan succeeded", "")
	srv.publishPlanStatus(ctx, status)
	srv.lastPlanGeneration = plan.GetGeneration()
	srv.state.LastPlanGeneration = plan.GetGeneration()
	if err := srv.state.save(srv.statePath); err != nil {
		log.Printf("save state: %v", err)
	}
	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_SUCCEEDED, "plan succeeded", 100, true, ""))
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
	if srv.controllerClient == nil {
		if err := srv.ensureControllerClient(ctx); err != nil {
			return err
		}
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
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := srv.controllerClient.ReportNodeStatus(ctx, &clustercontrollerpb.ReportNodeStatusRequest{
		Status: status,
	})
	return err
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
	conn, err := grpc.DialContext(dialCtx, srv.controllerEndpoint, opts...)
	if err != nil {
		return err
	}
	srv.controllerConn = conn
	srv.controllerClient = clustercontrollerpb.NewClusterControllerServiceClient(conn)
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
	netChanged = netChanged || (networkGen > 0 && networkGen != srv.lastNetworkGeneration)
	if netChanged {
		if err := srv.reconcileNetwork(plan, op, networkGen); err != nil {
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

	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_SUCCEEDED, "plan applied", 100, true, ""))
	srv.notifyControllerOperationResult(op.id, true, "plan applied", nil)
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

func (srv *NodeAgentServer) reconcileNetwork(plan *clustercontrollerpb.NodePlan, op *operation, generation uint64) error {
	if plan == nil {
		return nil
	}
	spec, err := specFromPlan(plan)
	if err != nil {
		return fmt.Errorf("parse desired spec: %w", err)
	}
	if err := srv.ensureNetworkCerts(spec); err != nil {
		return fmt.Errorf("ensure network certs: %w", err)
	}
	units := parseRestartUnits(plan)
	if len(units) > 0 {
		if err := srv.performRestartUnits(units, op); err != nil {
			return fmt.Errorf("restart units: %w", err)
		}
	}
	if generation > 0 && generation != srv.lastNetworkGeneration {
		if err := srv.syncDNS(spec); err != nil {
			return fmt.Errorf("sync dns: %w", err)
		}
		srv.lastNetworkGeneration = generation
		srv.state.NetworkGeneration = generation
		if err := srv.saveState(); err != nil {
			log.Printf("nodeagent: save state after dns sync: %v", err)
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

func (srv *NodeAgentServer) ensureNetworkCerts(spec *clustercontrollerpb.ClusterNetworkSpec) error {
	if spec == nil || strings.ToLower(spec.GetProtocol()) != "https" {
		return nil
	}
	domain := strings.TrimSpace(spec.GetClusterDomain())
	if domain == "" {
		return nil
	}
	if spec.GetAcmeEnabled() && strings.TrimSpace(spec.GetAdminEmail()) == "" {
		return errors.New("admin_email is required for ACME")
	}
	dns := append([]string{domain}, spec.GetAlternateDomains()...)
	dir := filepath.Join(config.GetRuntimeConfigDir(), "pki", domain)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create pki dir: %w", err)
	}
	opts := pki.Options{
		Storage: pki.FileStorage{},
		LocalCA: pki.LocalCAConfig{
			Enabled: true,
		},
	}
	if spec.GetAcmeEnabled() {
		opts.ACME = pki.ACMEConfig{
			Enabled: true,
			Email:   strings.TrimSpace(spec.GetAdminEmail()),
			Domain:  domain,
		}
	}
	manager := networkPKIManager(opts)
	if spec.GetAcmeEnabled() {
		subject := fmt.Sprintf("CN=%s", domain)
		keyFile, _, issuerFile, fullchainFile, err := manager.EnsurePublicACMECert(dir, domain, subject, dns, 90*24*time.Hour)
		if err != nil {
			return fmt.Errorf("issue ACME certs: %w", err)
		}
		if err := copyFilePerm(keyFile, filepath.Join(dir, "privkey.pem"), 0o600); err != nil {
			return err
		}
		if err := copyFilePerm(fullchainFile, filepath.Join(dir, "fullchain.pem"), 0o644); err != nil {
			return err
		}
		if err := copyFilePerm(issuerFile, filepath.Join(dir, "ca.pem"), 0o644); err != nil {
			return err
		}
		return nil
	}
	keyFile, leafFile, caFile, err := manager.EnsureServerCert(dir, domain, dns, 90*24*time.Hour)
	if err != nil {
		return fmt.Errorf("issue server certs: %w", err)
	}
	if err := copyFilePerm(keyFile, filepath.Join(dir, "privkey.pem"), 0o600); err != nil {
		return err
	}
	chainDst := filepath.Join(dir, "fullchain.pem")
	if err := concatFiles(chainDst, leafFile, caFile); err != nil {
		return fmt.Errorf("build fullchain: %w", err)
	}
	if caFile != "" {
		if err := copyFilePerm(caFile, filepath.Join(dir, "ca.pem"), 0o644); err != nil {
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
	for idx, unit := range units {
		unit = strings.TrimSpace(unit)
		if unit == "" {
			continue
		}
		percent := int32(30 + idx*10)
		if percent > 95 {
			percent = 95
		}
		if op != nil {
			op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("restart %s", unit), percent, false, ""))
		}
		if err := restartCommand(systemctl, unit); err != nil {
			log.Printf("nodeagent: %s reload/restart: %v", unit, err)
			errs = append(errs, fmt.Sprintf("%s: %v", unit, err))
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
		Protocol:      "http",  // Default to http for bootstrap
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
		"Domain":   spec.ClusterDomain,
		"Protocol": spec.Protocol,
		"PortHTTP": spec.PortHttp,
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
