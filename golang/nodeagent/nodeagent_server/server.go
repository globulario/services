package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/actions"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/apply"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/planner"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/supervisor"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/units"
	nodeagentpb "github.com/globulario/services/golang/nodeagent/nodeagentpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/store"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var defaultPort = "11000"

const defaultPlanPollInterval = 5 * time.Second
const planLockTTL = 30

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
	planStore                store.PlanStore
	planPollInterval         time.Duration
	lastPlanGeneration       uint64
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
	if err := supervisor.EnableNow(ctx, unit); err != nil {
		return fmt.Errorf("enable %s: %w", unit, err)
	}
	if err := supervisor.WaitActive(ctx, unit, 60*time.Second); err != nil {
		return fmt.Errorf("wait active %s: %w", unit, err)
	}
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
	if srv.planStore == nil || srv.nodeID == "" {
		return
	}
	go srv.planLoop(ctx)
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

	op := srv.registerOperation("apply plan", req.GetPlan().GetProfiles())
	go srv.runPlan(ctx, op, req.GetPlan())
	return &nodeagentpb.ApplyPlanResponse{OperationId: op.id}, nil
}

func (srv *NodeAgentServer) runPlan(ctx context.Context, op *operation, plan *clustercontrollerpb.NodePlan) {
	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_QUEUED, "plan queued", 0, false, ""))
	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, "plan running", 5, false, ""))

	actions := planner.ComputeActions(plan)
	total := len(actions)
	current := 0
	var lastPercent int32
	err := apply.ApplyActions(ctx, actions, func(action planner.Action) {
		lastPercent = percentForStep(current, total)
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("%s %s", action.Op, action.Unit), lastPercent, false, ""))
		current++
	})
	if err != nil {
		msg := err.Error()
		op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_FAILED, msg, lastPercent, true, msg))
		return
	}

	op.broadcast(op.newEvent(clustercontrollerpb.OperationPhase_OP_SUCCEEDED, "plan applied", 100, true, ""))
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

	op := srv.registerOperation("bootstrap node", profiles)
	srv.startOperation(op, "bootstrapping first node")

	return &nodeagentpb.BootstrapFirstNodeResponse{
		OperationId: op.id,
		JoinToken:   srv.joinToken,
		Message:     "bootstrap initiated",
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
	return ips
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
