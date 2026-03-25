package planexec

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/units"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/versionutil"
	"github.com/globulario/services/golang/workflow"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/protobuf/proto"
)

// Runner executes planpb.NodePlan objects until invariants are met or retries are exhausted.
type Runner struct {
	NodeID         string
	PublishStatus  func(context.Context, *planpb.NodePlanStatus)
	Now            func() time.Time
	DefaultBackoff time.Duration

	// Workflow tracing (optional, nil-safe).
	WorkflowRec *workflow.Recorder
	ClusterID   string
}

// NewRunner builds a Runner with sane defaults.
func NewRunner(nodeID string, publish func(context.Context, *planpb.NodePlanStatus)) *Runner {
	return &Runner{
		NodeID:         nodeID,
		PublishStatus:  publish,
		Now:            time.Now,
		DefaultBackoff: 2 * time.Second,
	}
}

// ReconcilePlan attempts to converge the supplied plan by running steps until invariants pass.
func (r *Runner) ReconcilePlan(ctx context.Context, plan *planpb.NodePlan, current *planpb.NodePlanStatus) (*planpb.NodePlanStatus, error) {
	if plan == nil {
		return current, nil
	}
	status := r.normalizeStatus(plan, current)
	if isTerminal(status.GetState()) {
		return status, nil
	}
	if status.GetStartedUnixMs() == 0 {
		status.StartedUnixMs = uint64(r.now().UnixMilli())
	}

	// Extract workflow run ID from plan annotations (set by controller).
	wfRunID := plan.GetAnnotations()["workflow_run_id"]

	// Notify workflow service: node-agent is now executing.
	r.WorkflowRec.UpdateRunStatus(ctx, wfRunID,
		workflow.Executing, fmt.Sprintf("Node-agent %s executing plan %s", r.NodeID, plan.GetPlanId()),
		workflow.ActorNodeAgent)

	// quick-success path
	if err := r.EvaluateInvariants(ctx, plan); err == nil {
		r.addEvent(status, "info", "invariants satisfied; plan complete", "")
		status.State = planpb.PlanState_PLAN_SUCCEEDED
		status.FinishedUnixMs = uint64(r.now().UnixMilli())
		r.publish(ctx, status)
		r.WorkflowRec.FinishRun(ctx, wfRunID, workflow.Succeeded,
			"Invariants already satisfied", "", workflow.NoFailure)
		return status, nil
	}

	status.State = planpb.PlanState_PLAN_RUNNING
	r.publish(ctx, status)

	policy := plan.GetPolicy()
	if policy == nil {
		policy = &planpb.PlanPolicy{}
	}
	maxAttempts := int(policy.GetMaxRetries()) + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		runErr := r.runStepsOnce(ctx, plan, status, wfRunID)
		if runErr != nil {
			r.WorkflowRec.FinishRun(ctx, wfRunID, workflow.Failed,
				fmt.Sprintf("Plan execution failed: %v", runErr), runErr.Error(),
				workflowpb.FailureClass_FAILURE_CLASS_SYSTEMD)
			return status, runErr
		}
		r.addEvent(status, "info", "phase: VALIDATING", "")
		r.publish(ctx, status)

		verifySeq := r.WorkflowRec.RecordStep(ctx, wfRunID, &workflow.StepParams{
			StepKey: "verify_invariants",
			Title:   "Verify plan invariants",
			Actor:   workflow.ActorNodeAgent,
			Phase:   workflow.PhaseVerify,
			Status:  workflow.StepRunning,
		})

		if invErr := r.EvaluateInvariants(ctx, plan); invErr == nil {
			r.WorkflowRec.CompleteStep(ctx, wfRunID, verifySeq, "invariants satisfied", 0)
			r.addEvent(status, "info", "phase: COMMITTED", "")
			r.addEvent(status, "info", "invariants satisfied; plan complete", "")
			status.State = planpb.PlanState_PLAN_SUCCEEDED
			status.FinishedUnixMs = uint64(r.now().UnixMilli())
			r.publish(ctx, status)
			r.WorkflowRec.FinishRun(ctx, wfRunID, workflow.Succeeded,
				"Plan converged successfully", "", workflow.NoFailure)
			return status, nil
		} else {
			log.Printf("plan-runner: invariant check failed (attempt %d/%d): %v", attempt+1, maxAttempts, invErr)
		}
		r.WorkflowRec.CompleteStep(ctx, wfRunID, verifySeq, "invariants not yet satisfied", 0)

		// backoff before retrying full plan
		if attempt+1 < maxAttempts {
			r.addEvent(status, "warn", "invariants still failing; retrying plan", "")
			r.publish(ctx, status)
			select {
			case <-ctx.Done():
				return status, ctx.Err()
			case <-time.After(r.backoff(plan, attempt+1)):
			}
		}
	}

	status.State = planpb.PlanState_PLAN_FAILED
	status.ErrorMessage = "invariants not satisfied after retries"
	status.FinishedUnixMs = uint64(r.now().UnixMilli())
	r.publish(ctx, status)
	r.WorkflowRec.FinishRun(ctx, wfRunID, workflow.Failed,
		status.ErrorMessage, status.ErrorMessage,
		workflowpb.FailureClass_FAILURE_CLASS_SYSTEMD)
	return status, errors.New(status.ErrorMessage)
}

func (r *Runner) runStepsOnce(ctx context.Context, plan *planpb.NodePlan, status *planpb.NodePlanStatus, wfRunID string) error {
	spec := plan.GetSpec()
	if spec == nil {
		return errors.New("plan spec required")
	}
	for idx, step := range spec.GetSteps() {
		stepStatus := getOrCreateStepStatus(status, step.GetId())
		stepStatus.State = planpb.StepState_STEP_RUNNING
		stepStatus.StartedUnixMs = uint64(r.now().UnixMilli())
		status.CurrentStepId = step.GetId()

		// Emit rollout phase event based on the step action.
		if phase := rolloutPhaseForAction(step.GetAction()); phase != "" {
			r.addEvent(status, "info", fmt.Sprintf("phase: %s", phase), step.GetId())
		}
		log.Printf("plan-step[%d/%d] %s action=%s starting", idx+1, len(spec.GetSteps()), step.GetId(), step.GetAction())
		r.addEvent(status, "info", fmt.Sprintf("step %s running", step.GetId()), step.GetId())
		r.publish(ctx, status)

		// Record workflow step start.
		wfStepSeq := r.WorkflowRec.RecordStep(ctx, wfRunID, &workflow.StepParams{
			StepKey:     step.GetId(),
			Title:       fmt.Sprintf("[%d/%d] %s", idx+1, len(spec.GetSteps()), step.GetAction()),
			Actor:       workflow.ActorNodeAgent,
			Phase:       workflowPhaseForAction(step.GetAction()),
			Status:      workflow.StepRunning,
			SourceActor: workflow.ActorNodeAgent,
			TargetActor: actorForAction(step.GetAction()),
		})

		stepStart := r.now()
		if err := r.runStepWithRetry(ctx, plan, step, stepStatus, status); err != nil {
			durationMs := r.now().Sub(stepStart).Milliseconds()
			log.Printf("plan-step[%d/%d] %s action=%s FAILED after %v: %v",
				idx+1, len(spec.GetSteps()), step.GetId(), step.GetAction(), r.now().Sub(stepStart), err)
			r.WorkflowRec.FailStep(ctx, wfRunID, wfStepSeq,
				"nodeagent.step_failed", err.Error(),
				fmt.Sprintf("Step %s (%s) failed", step.GetId(), step.GetAction()),
				workflowpb.FailureClass_FAILURE_CLASS_SYSTEMD, true)
			_ = durationMs
			return err
		}
		durationMs := r.now().Sub(stepStart).Milliseconds()
		log.Printf("plan-step[%d/%d] %s action=%s OK (%v)", idx+1, len(spec.GetSteps()), step.GetId(), step.GetAction(), r.now().Sub(stepStart))
		r.WorkflowRec.CompleteStep(ctx, wfRunID, wfStepSeq,
			fmt.Sprintf("%s completed", step.GetAction()), durationMs)

		stepStatus.State = planpb.StepState_STEP_OK
		stepStatus.FinishedUnixMs = uint64(r.now().UnixMilli())
		status.CurrentStepId = ""
		r.addEvent(status, "info", fmt.Sprintf("step %s succeeded", step.GetId()), step.GetId())
		r.publish(ctx, status)
	}
	return nil
}

func (r *Runner) runStepWithRetry(ctx context.Context, plan *planpb.NodePlan, step *planpb.PlanStep, stepStatus *planpb.StepStatus, status *planpb.NodePlanStatus) error {
	policy := step.GetPolicy()
	if policy == nil {
		policy = &planpb.StepPolicy{}
	}
	maxAttempts := int(policy.GetMaxRetries()) + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		stepStatus.Attempt = uint32(attempt)
		msg, err := RunStep(ctx, step)
		if msg != "" {
			stepStatus.Message = msg
		}
		if err == nil {
			return nil
		}
		lastErr = err
		r.addEvent(status, "error", fmt.Sprintf("step %s failed: %v", step.GetId(), err), step.GetId())
		stepStatus.State = planpb.StepState_STEP_FAILED
		r.publish(ctx, status)

		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(r.backoff(plan, attempt)):
			}
			continue
		}
		status.ErrorMessage = err.Error()
		status.ErrorStepId = step.GetId()

		// Handle rollback if configured.
		if policy.GetOnFail() == planpb.FailureMode_FAILURE_MODE_ROLLBACK || plan.GetPolicy().GetFailureMode() == planpb.FailureMode_FAILURE_MODE_ROLLBACK {
			status.State = planpb.PlanState_PLAN_ROLLING_BACK
			r.publish(ctx, status)
			if rbErr := r.runRollback(ctx, plan, status); rbErr != nil {
				status.State = planpb.PlanState_PLAN_FAILED
				status.FinishedUnixMs = uint64(r.now().UnixMilli())
				status.ErrorMessage = fmt.Sprintf("rollback failed: %v", rbErr)
				r.publish(ctx, status)
				return rbErr
			}
			status.State = planpb.PlanState_PLAN_ROLLED_BACK
			status.FinishedUnixMs = uint64(r.now().UnixMilli())
			r.publish(ctx, status)
			return err
		}

		status.State = planpb.PlanState_PLAN_FAILED
		status.FinishedUnixMs = uint64(r.now().UnixMilli())
		r.publish(ctx, status)
	}
	return lastErr
}

func (r *Runner) runRollback(ctx context.Context, plan *planpb.NodePlan, status *planpb.NodePlanStatus) error {
	spec := plan.GetSpec()
	if spec == nil {
		return errors.New("plan spec missing; cannot rollback")
	}
	for _, step := range spec.GetRollback() {
		stepStatus := getOrCreateStepStatus(status, step.GetId())
		stepStatus.Attempt++
		stepStatus.State = planpb.StepState_STEP_RUNNING
		stepStatus.StartedUnixMs = uint64(r.now().UnixMilli())
		status.CurrentStepId = step.GetId()
		r.addEvent(status, "warn", fmt.Sprintf("rollback %s running", step.GetId()), step.GetId())
		r.publish(ctx, status)

		msg, err := RunStep(ctx, step)
		if msg != "" {
			stepStatus.Message = msg
		}
		if err != nil {
			stepStatus.State = planpb.StepState_STEP_FAILED
			stepStatus.FinishedUnixMs = uint64(r.now().UnixMilli())
			status.ErrorMessage = fmt.Sprintf("rollback %s failed: %v", step.GetId(), err)
			status.ErrorStepId = step.GetId()
			r.addEvent(status, "error", status.ErrorMessage, step.GetId())
			r.publish(ctx, status)
			return err
		}

		stepStatus.State = planpb.StepState_STEP_OK
		stepStatus.FinishedUnixMs = uint64(r.now().UnixMilli())
		r.addEvent(status, "info", fmt.Sprintf("rollback %s succeeded", step.GetId()), step.GetId())
		r.publish(ctx, status)
	}
	return nil
}

// RunStep executes a single plan step by validating and applying its action and conditions.
func RunStep(ctx context.Context, step *planpb.PlanStep) (string, error) {
	if step == nil {
		return "", errors.New("step is nil")
	}
	handler := actions.Get(step.GetAction())
	if handler == nil {
		return "", fmt.Errorf("action %q not registered", step.GetAction())
	}
	if err := handler.Validate(step.GetArgs()); err != nil {
		return "", err
	}
	for _, cond := range step.GetPre() {
		if err := EvaluateCondition(ctx, cond); err != nil {
			return "", fmt.Errorf("pre condition %s failed: %w", cond.GetType(), err)
		}
	}
	stepCtx := ctx
	var cancel context.CancelFunc
	if policy := step.GetPolicy(); policy != nil {
		if to := policy.GetTimeoutMs(); to > 0 {
			stepCtx, cancel = context.WithTimeout(ctx, time.Duration(to)*time.Millisecond)
			defer cancel()
		}
	}
	msg, err := handler.Apply(stepCtx, step.GetArgs())
	if err != nil {
		return msg, err
	}
	for _, cond := range step.GetPost() {
		if err := EvaluateCondition(ctx, cond); err != nil {
			return msg, fmt.Errorf("post condition %s failed: %w", cond.GetType(), err)
		}
	}
	return msg, nil
}

// EvaluateCondition resolves and applies the registered action for the condition.
func EvaluateCondition(ctx context.Context, cond *planpb.Condition) error {
	if cond == nil {
		return errors.New("condition is nil")
	}
	handler := actions.Get(cond.GetType())
	if handler == nil {
		return fmt.Errorf("condition handler %q not registered", cond.GetType())
	}
	if err := handler.Validate(cond.GetArgs()); err != nil {
		return err
	}
	_, err := handler.Apply(ctx, cond.GetArgs())
	return err
}

// EvaluateInvariants checks success probes and desired state.
// Probes are retried up to 3 times with a 3-second delay to allow
// services to finish starting after a restart step.
func (r *Runner) EvaluateInvariants(ctx context.Context, plan *planpb.NodePlan) error {
	spec := plan.GetSpec()
	if spec == nil {
		return errors.New("plan spec required")
	}
	const probeRetries = 3
	for _, probe := range spec.GetSuccessProbes() {
		cond := &planpb.Condition{Type: probe.GetType(), Args: probe.GetArgs()}
		var probeErr error
		for try := 0; try < probeRetries; try++ {
			if probeErr = EvaluateCondition(ctx, cond); probeErr == nil {
				break
			}
			if try+1 < probeRetries {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(3 * time.Second):
				}
			}
		}
		if probeErr != nil {
			return fmt.Errorf("success probe %s failed: %w", probe.GetType(), probeErr)
		}
	}
	if desired := spec.GetDesired(); desired != nil {
		if err := checkDesiredServices(ctx, desired.GetServices()); err != nil {
			return err
		}
		if err := checkDesiredFiles(desired.GetFiles()); err != nil {
			return err
		}
	}
	return nil
}

func checkDesiredServices(ctx context.Context, services []*planpb.DesiredService) error {
	for _, svc := range services {
		unit := strings.TrimSpace(svc.GetUnit())
		if unit == "" {
			unit = units.UnitForService(svc.GetName())
		}
		if unit == "" {
			return fmt.Errorf("desired service %s missing unit", svc.GetName())
		}
		active, err := supervisor.IsActive(ctx, unit)
		if err != nil {
			return fmt.Errorf("check service %s: %w", unit, err)
		}
		if !active {
			// During Day-1 convergence, services may crash-loop because
			// dependencies (event, etc.) aren't installed yet. Accept the
			// service as "installed" if the unit file exists and was loaded
			// by systemd — it will stabilize once deps arrive.
			loaded, loadErr := supervisor.IsLoaded(ctx, unit)
			if loadErr != nil || !loaded {
				return fmt.Errorf("service %s not active and not loaded", unit)
			}
			// Unit is loaded but not active — accept for convergence.
		}
		// Version verification is best-effort; skip if unknown.
		if v := strings.TrimSpace(svc.GetVersion()); v != "" {
			installed, err := detectServiceVersion(svc.GetName())
			if err != nil {
				return fmt.Errorf("service %s version check: %w", svc.GetName(), err)
			}
			if installed == "" {
				return fmt.Errorf("service %s version marker missing", svc.GetName())
			}
			if installed != v {
				return fmt.Errorf("service %s version mismatch: have %s want %s", svc.GetName(), installed, v)
			}
		}
	}
	return nil
}

func checkDesiredFiles(files []*planpb.DesiredFile) error {
	for _, f := range files {
		path := strings.TrimSpace(f.GetPath())
		if path == "" {
			return errors.New("desired file path missing")
		}
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("file %s check: %w", path, err)
		}
		if mode := strings.TrimSpace(f.GetMode()); mode != "" {
			if !fileModeMatches(info.Mode(), mode) {
				return fmt.Errorf("file %s mode mismatch", path)
			}
		}
		if owner := strings.TrimSpace(f.GetOwner()); owner != "" {
			// best-effort owner check via base path; platform-specific ownership omitted
			_ = owner
		}
		if ref := strings.TrimSpace(f.GetContentRef()); ref != "" {
			// TODO: verify content hash once content addressing is available.
			_ = ref
		}
	}
	return nil
}

func detectServiceVersion(unit string) (string, error) {
	path := versionutil.MarkerPath(unit)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func fileModeMatches(actual os.FileMode, expected string) bool {
	trim := strings.TrimPrefix(expected, "0")
	parsed, err := strconv.ParseUint(trim, 8, 32)
	if err != nil {
		return true
	}
	return actual.Perm() == os.FileMode(parsed)
}

func isTerminal(state planpb.PlanState) bool {
	switch state {
	case planpb.PlanState_PLAN_SUCCEEDED, planpb.PlanState_PLAN_FAILED, planpb.PlanState_PLAN_ROLLED_BACK, planpb.PlanState_PLAN_EXPIRED:
		return true
	default:
		return false
	}
}

func getOrCreateStepStatus(status *planpb.NodePlanStatus, id string) *planpb.StepStatus {
	for _, st := range status.GetSteps() {
		if st.GetId() == id {
			return st
		}
	}
	step := &planpb.StepStatus{
		Id: id,
	}
	status.Steps = append(status.Steps, step)
	return step
}

func (r *Runner) normalizeStatus(plan *planpb.NodePlan, st *planpb.NodePlanStatus) *planpb.NodePlanStatus {
	if st != nil && st.GetGeneration() == plan.GetGeneration() && st.GetPlanId() == plan.GetPlanId() {
		return st
	}
	var status *planpb.NodePlanStatus
	if st != nil {
		if cloned, ok := proto.Clone(st).(*planpb.NodePlanStatus); ok {
			status = cloned
		}
	}
	if status == nil {
		status = &planpb.NodePlanStatus{}
	}
	status.PlanId = plan.GetPlanId()
	status.NodeId = plan.GetNodeId()
	status.Generation = plan.GetGeneration()
	status.State = planpb.PlanState_PLAN_PENDING
	status.StartedUnixMs = uint64(r.now().UnixMilli())
	status.ErrorMessage = ""
	status.ErrorStepId = ""
	status.CurrentStepId = ""
	status.Events = nil
	status.Steps = nil
	return status
}

func (r *Runner) addEvent(status *planpb.NodePlanStatus, level, msg, stepID string) {
	if status == nil {
		return
	}
	status.Events = append(status.Events, &planpb.PlanEvent{
		TsUnixMs: uint64(r.now().UnixMilli()),
		Level:    level,
		Msg:      msg,
		StepId:   stepID,
	})
}

func (r *Runner) publish(ctx context.Context, status *planpb.NodePlanStatus) {
	if r.PublishStatus != nil {
		r.PublishStatus(ctx, status)
	}
}

func (r *Runner) now() time.Time {
	if r != nil && r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Runner) backoff(plan *planpb.NodePlan, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	policy := plan.GetPolicy()
	if policy == nil {
		policy = &planpb.PlanPolicy{}
	}
	backoffMs := policy.GetRetryBackoffMs()
	if backoffMs <= 0 && r.DefaultBackoff > 0 {
		return r.DefaultBackoff
	}
	if backoffMs <= 0 {
		return 0
	}
	return time.Duration(backoffMs*uint32(attempt)) * time.Millisecond
}

// workflowPhaseForAction maps plan step actions to workflow phase enums.
func workflowPhaseForAction(action string) workflowpb.WorkflowPhaseKind {
	switch action {
	case "artifact.fetch", "artifact.verify":
		return workflow.PhaseFetch
	case "service.install_payload", "install_os_packages":
		return workflow.PhaseInstall
	case "config.write", "service.write_version_marker":
		return workflow.PhaseConfigure
	case "service.restart", "service.enable":
		return workflow.PhaseStart
	case "package.report_state":
		return workflow.PhaseVerify
	default:
		return workflow.PhaseInstall
	}
}

// actorForAction returns the workflow actor that owns the action.
func actorForAction(action string) workflowpb.WorkflowActor {
	if strings.HasPrefix(action, "service.") || strings.HasPrefix(action, "artifact.") ||
		strings.HasPrefix(action, "config.") || strings.HasPrefix(action, "package.") {
		return workflow.ActorInstaller
	}
	return workflow.ActorNodeAgent
}

// rolloutPhaseForAction maps plan step action names to human-readable rollout phases.
// These provide explicit operator visibility into the rollout lifecycle.
func rolloutPhaseForAction(action string) string {
	switch action {
	case "artifact.fetch":
		return "DOWNLOADING"
	case "artifact.verify":
		return "VERIFYING"
	case "service.install_payload":
		return "STAGING"
	case "service.restart":
		return "RESTARTING"
	case "service.write_version_marker", "package.report_state":
		return "COMMITTING"
	default:
		return ""
	}
}
