package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/planexec"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/google/uuid"
	clientv3 "go.etcd.io/etcd/client/v3"
)

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
		log.Printf("poll-plan: unable to read current plan for node %s: %v", srv.nodeID, err)
		return
	}
	status, _ := srv.planStore.GetStatus(pollCtx, srv.nodeID)
	if plan == nil {
		return
	}
	statusState := "nil"
	statusGen := uint64(0)
	if status != nil {
		statusState = status.GetState().String()
		statusGen = status.GetGeneration()
	}
	log.Printf("poll-plan: found plan %s gen=%d for node %s (reason=%s) status=%s status_gen=%d", plan.GetPlanId(), plan.GetGeneration(), srv.nodeID, plan.GetReason(), statusState, statusGen)
	if plan.GetNodeId() != "" && plan.GetNodeId() != srv.nodeID {
		log.Printf("poll-plan: SKIP node id mismatch plan=%s srv=%s", plan.GetNodeId(), srv.nodeID)
		return
	}
	if status != nil && status.GetGeneration() == plan.GetGeneration() && isTerminalState(status.GetState()) {
		log.Printf("poll-plan: SKIP terminal status=%s gen=%d", status.GetState().String(), status.GetGeneration())
		return
	}
	now := time.Now().UnixMilli()
	if plan.GetExpiresUnixMs() > 0 && now > int64(plan.GetExpiresUnixMs()) {
		log.Printf("poll-plan: SKIP expired")
		srv.markPlanExpired(pollCtx, plan)
		return
	}
	// Plan verification (Phase 1B): new plan_id clears quarantine
	planID := plan.GetPlanId()
	if planID != srv.lastSeenPlanID {
		srv.rejectionTracker.clearAll()
		srv.lastSeenPlanID = planID
	}
	if srv.rejectionTracker.isQuarantined(planID) {
		log.Printf("poll-plan: SKIP quarantined plan_id=%s", planID)
		return
	}
	if err := srv.verifyPlan(plan); err != nil {
		srv.reportPlanRejection(plan, err)
		return
	}

	log.Printf("poll-plan: executing plan %s gen=%d", plan.GetPlanId(), plan.GetGeneration())
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
	op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_FAILED, "plan expired", 0, true, "plan expired before execution"))

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
	acquire := srv.acquirePlanLocks
	if srv.lockAcquirer != nil {
		acquire = srv.lockAcquirer
	}
	guard, err := acquire(ctx, plan)
	if err != nil {
		msg := fmt.Sprintf("lock acquisition failed: %v", err)
		opID := plan.GetPlanId()
		if opID == "" {
			opID = uuid.NewString()
		}
		op := srv.registerOperationWithID("plan-runner", opID, nil)
		op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_FAILED, msg, 0, true, msg))
		st := srv.newPlanStatus(plan)
		st.State = planpb.PlanState_PLAN_FAILED
		st.ErrorMessage = lockConflictMessage(err)
		st.FinishedUnixMs = uint64(time.Now().UnixMilli())
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
	op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_QUEUED, "plan queued", 0, false, ""))
	op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_RUNNING, "plan running", 5, false, ""))

	runner := planexec.NewRunner(srv.nodeID, srv.publishPlanStatus)
	runner.WorkflowRec = srv.workflowRec
	runner.ClusterID = srv.clusterID
	updated, recErr := runner.ReconcilePlan(ctx, plan, status)
	if recErr != nil {
		log.Printf("plan-runner: plan %s gen=%d FAILED: %v", plan.GetPlanId(), plan.GetGeneration(), recErr)
		op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_FAILED, recErr.Error(), 90, true, recErr.Error()))
		return
	}
	if updated != nil {
		log.Printf("plan-runner: plan %s gen=%d finished state=%s err=%s", plan.GetPlanId(), plan.GetGeneration(), updated.GetState().String(), updated.GetErrorMessage())
	}
	if updated != nil && isTerminalState(updated.GetState()) {
		srv.lastPlanGeneration = plan.GetGeneration()
		srv.state.LastPlanGeneration = plan.GetGeneration()
		if err := srv.state.save(srv.statePath); err != nil {
			log.Printf("save state: %v", err)
		}
		// Persist generation to file for replay protection (only on success).
		if updated.GetState() == planpb.PlanState_PLAN_SUCCEEDED && plan.GetGeneration() > 0 {
			if err := saveLastAppliedGeneration(plan.GetGeneration()); err != nil {
				log.Printf("WARN: failed to persist generation %d: %v", plan.GetGeneration(), err)
			}
		}
	}
	op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_SUCCEEDED, "plan reconciled", 100, true, ""))
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
		op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("rollback %s running", step.GetId()), percent, false, ""))

		if err := srv.executePlanStep(ctx, step); err != nil {
			stepStatus.State = planpb.StepState_STEP_FAILED
			stepStatus.FinishedUnixMs = uint64(time.Now().UnixMilli())
			status.State = planpb.PlanState_PLAN_FAILED
			status.ErrorMessage = fmt.Sprintf("rollback failed: %v", err)
			status.ErrorStepId = step.GetId()
			status.FinishedUnixMs = uint64(time.Now().UnixMilli())
			srv.addPlanEvent(status, "error", fmt.Sprintf("rollback step %s failed: %v", step.GetId(), err), step.GetId())
			srv.publishPlanStatus(ctx, status)
			op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_FAILED, fmt.Sprintf("rollback %s failed", step.GetId()), percent, true, err.Error()))
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
	op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_FAILED, "rollback succeeded", 100, true, ""))
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

func lockConflictMessage(err error) string {
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "LOCK_CONFLICT"
	}
	return "LOCK_CONFLICT: " + msg
}

func (srv *NodeAgentServer) publishPlanStatus(ctx context.Context, status *planpb.NodePlanStatus) {
	if srv.planStore == nil || status == nil {
		return
	}
	if err := srv.planStore.PutStatus(ctx, srv.nodeID, status); err != nil {
		log.Printf("failed to publish plan status: %v", err)
	}
}
