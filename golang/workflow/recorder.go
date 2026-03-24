// Package workflow provides a client-side recorder for emitting workflow
// runs, steps, artifacts, and events to the WorkflowService.
//
// Usage:
//
//	rec := workflow.NewRecorder("localhost:10220", "cluster-id")
//	defer rec.Close()
//
//	run, _ := rec.StartRun(ctx, &workflow.RunParams{...})
//	rec.RecordStep(ctx, run.Id, 1, &workflow.StepParams{...})
//	rec.FinishRun(ctx, run.Id, workflow.Succeeded, "all good", "", workflow.NoFailure)
package workflow

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Convenience aliases for enum values.
var (
	Succeeded  = workflowpb.RunStatus_RUN_STATUS_SUCCEEDED
	Failed     = workflowpb.RunStatus_RUN_STATUS_FAILED
	Executing  = workflowpb.RunStatus_RUN_STATUS_EXECUTING
	Planning   = workflowpb.RunStatus_RUN_STATUS_PLANNING
	Dispatched = workflowpb.RunStatus_RUN_STATUS_DISPATCHED
	Pending    = workflowpb.RunStatus_RUN_STATUS_PENDING
	Blocked    = workflowpb.RunStatus_RUN_STATUS_BLOCKED
	RolledBack = workflowpb.RunStatus_RUN_STATUS_ROLLED_BACK

	StepRunning   = workflowpb.StepStatus_STEP_STATUS_RUNNING
	StepSucceeded = workflowpb.StepStatus_STEP_STATUS_SUCCEEDED
	StepFailed    = workflowpb.StepStatus_STEP_STATUS_FAILED
	StepSkipped   = workflowpb.StepStatus_STEP_STATUS_SKIPPED
	StepBlocked   = workflowpb.StepStatus_STEP_STATUS_BLOCKED

	NoFailure = workflowpb.FailureClass_FAILURE_CLASS_UNKNOWN

	ActorController = workflowpb.WorkflowActor_ACTOR_CLUSTER_CONTROLLER
	ActorNodeAgent  = workflowpb.WorkflowActor_ACTOR_NODE_AGENT
	ActorInstaller  = workflowpb.WorkflowActor_ACTOR_INSTALLER
	ActorRuntime    = workflowpb.WorkflowActor_ACTOR_RUNTIME
	ActorRepository = workflowpb.WorkflowActor_ACTOR_REPOSITORY

	PhaseDecision  = workflowpb.WorkflowPhaseKind_PHASE_DECISION
	PhasePlan      = workflowpb.WorkflowPhaseKind_PHASE_PLAN
	PhaseDispatch  = workflowpb.WorkflowPhaseKind_PHASE_DISPATCH
	PhaseFetch     = workflowpb.WorkflowPhaseKind_PHASE_FETCH
	PhaseInstall   = workflowpb.WorkflowPhaseKind_PHASE_INSTALL
	PhaseConfigure = workflowpb.WorkflowPhaseKind_PHASE_CONFIGURE
	PhaseStart     = workflowpb.WorkflowPhaseKind_PHASE_START
	PhaseVerify    = workflowpb.WorkflowPhaseKind_PHASE_VERIFY
	PhasePublish   = workflowpb.WorkflowPhaseKind_PHASE_PUBLISH

	KindInfra   = workflowpb.ComponentKind_COMPONENT_KIND_INFRASTRUCTURE
	KindService = workflowpb.ComponentKind_COMPONENT_KIND_SERVICE
)

// RunParams holds the parameters for starting a workflow run.
type RunParams struct {
	NodeID           string
	NodeHostname     string
	ComponentName    string
	ComponentKind    workflowpb.ComponentKind
	ComponentVersion string
	ReleaseKind      string
	ReleaseObjectID  string
	PlanID           string
	PlanGeneration   int32
	TriggerReason    workflowpb.TriggerReason
	CorrelationID    string
}

// StepParams holds the parameters for recording a workflow step.
type StepParams struct {
	StepKey     string
	Title       string
	Actor       workflowpb.WorkflowActor
	Phase       workflowpb.WorkflowPhaseKind
	Status      workflowpb.StepStatus
	SourceActor workflowpb.WorkflowActor
	TargetActor workflowpb.WorkflowActor
	Message     string
	DetailsJSON string
}

// Recorder is a fire-and-forget client for the WorkflowService.
// All methods log errors but never return them — the workflow trace
// must never block the reconciliation pipeline.
type Recorder struct {
	clusterID string
	client    workflowpb.WorkflowServiceClient
	conn      *grpc.ClientConn
	mu        sync.Mutex
	seqMap    map[string]int32 // run_id → next seq number
}

// NewRecorder connects to the workflow service and returns a recorder.
// If the connection fails, it returns a no-op recorder that silently drops events.
func NewRecorder(addr, clusterID string) *Recorder {
	r := &Recorder{
		clusterID: clusterID,
		seqMap:    make(map[string]int32),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		log.Printf("workflow recorder: connect to %s failed (recording disabled): %v", addr, err)
		return r // no-op mode
	}
	r.conn = conn
	r.client = workflowpb.NewWorkflowServiceClient(conn)
	log.Printf("workflow recorder: connected to %s", addr)
	return r
}

// Close releases the gRPC connection.
func (r *Recorder) Close() {
	if r != nil && r.conn != nil {
		r.conn.Close()
	}
}

// Available returns true if the recorder has a live connection.
func (r *Recorder) Available() bool {
	return r != nil && r.client != nil
}

// StartRun begins a new workflow run. Returns the run ID (even on failure, returns "").
func (r *Recorder) StartRun(ctx context.Context, p *RunParams) string {
	if r == nil || r.client == nil {
		return ""
	}

	now := timestamppb.Now()
	run := &workflowpb.WorkflowRun{
		CorrelationId: p.CorrelationID,
		Context: &workflowpb.WorkflowContext{
			ClusterId:        r.clusterID,
			NodeId:           p.NodeID,
			NodeHostname:     p.NodeHostname,
			ComponentName:    p.ComponentName,
			ComponentKind:    p.ComponentKind,
			ComponentVersion: p.ComponentVersion,
			ReleaseKind:      p.ReleaseKind,
			ReleaseObjectId:  p.ReleaseObjectID,
			PlanId:           p.PlanID,
			PlanGeneration:   p.PlanGeneration,
		},
		TriggerReason: p.TriggerReason,
		Status:        workflowpb.RunStatus_RUN_STATUS_PENDING,
		CurrentActor:  ActorController,
		StartedAt:     now,
	}

	resp, err := r.client.StartRun(ctx, &workflowpb.StartRunRequest{Run: run})
	if err != nil {
		log.Printf("workflow recorder: StartRun failed: %v", err)
		return ""
	}
	return resp.GetId()
}

// RecordStep records a step in a workflow run. Returns the step seq.
func (r *Recorder) RecordStep(ctx context.Context, runID string, p *StepParams) int32 {
	if r == nil || r.client == nil || runID == "" {
		return 0
	}

	r.mu.Lock()
	seq := r.seqMap[runID] + 1
	r.seqMap[runID] = seq
	r.mu.Unlock()

	now := timestamppb.Now()
	step := &workflowpb.WorkflowStep{
		RunId:       runID,
		Seq:         seq,
		StepKey:     p.StepKey,
		Title:       p.Title,
		Actor:       p.Actor,
		Phase:       p.Phase,
		Status:      p.Status,
		SourceActor: p.SourceActor,
		TargetActor: p.TargetActor,
		CreatedAt:   now,
		StartedAt:   now,
		Message:     p.Message,
		DetailsJson: p.DetailsJSON,
	}

	if _, err := r.client.RecordStep(ctx, &workflowpb.RecordStepRequest{
		ClusterId: r.clusterID,
		Step:      step,
	}); err != nil {
		log.Printf("workflow recorder: RecordStep failed: %v", err)
	}
	return seq
}

// CompleteStep marks a step as succeeded.
func (r *Recorder) CompleteStep(ctx context.Context, runID string, seq int32, msg string, durationMs int64) {
	if r == nil || r.client == nil || runID == "" {
		return
	}
	if _, err := r.client.UpdateStep(ctx, &workflowpb.UpdateStepRequest{
		ClusterId:  r.clusterID,
		RunId:      runID,
		Seq:        seq,
		Status:     StepSucceeded,
		Message:    msg,
		DurationMs: durationMs,
	}); err != nil {
		log.Printf("workflow recorder: CompleteStep failed: %v", err)
	}
}

// FailStep marks a step as failed with classification.
func (r *Recorder) FailStep(ctx context.Context, runID string, seq int32, errorCode, errorMsg, actionHint string, failClass workflowpb.FailureClass, retryable bool) {
	if r == nil || r.client == nil || runID == "" {
		return
	}
	if _, err := r.client.FailStep(ctx, &workflowpb.FailStepRequest{
		ClusterId:               r.clusterID,
		RunId:                   runID,
		Seq:                     seq,
		ErrorCode:               errorCode,
		ErrorMessage:            errorMsg,
		ActionHint:              actionHint,
		FailureClass:            failClass,
		Retryable:               retryable,
		OperatorActionRequired:  !retryable,
	}); err != nil {
		log.Printf("workflow recorder: FailStep failed: %v", err)
	}
}

// UpdateRunStatus updates the run status and summary.
func (r *Recorder) UpdateRunStatus(ctx context.Context, runID string, status workflowpb.RunStatus, summary string, actor workflowpb.WorkflowActor) {
	if r == nil || r.client == nil || runID == "" {
		return
	}
	if _, err := r.client.UpdateRun(ctx, &workflowpb.UpdateRunRequest{
		Id:           runID,
		ClusterId:    r.clusterID,
		Status:       status,
		Summary:      summary,
		CurrentActor: actor,
	}); err != nil {
		log.Printf("workflow recorder: UpdateRunStatus failed: %v", err)
	}
}

// FinishRun completes a workflow run.
func (r *Recorder) FinishRun(ctx context.Context, runID string, status workflowpb.RunStatus, summary, errorMsg string, failClass workflowpb.FailureClass) {
	if r == nil || r.client == nil || runID == "" {
		return
	}
	if _, err := r.client.FinishRun(ctx, &workflowpb.FinishRunRequest{
		Id:           runID,
		ClusterId:    r.clusterID,
		Status:       status,
		Summary:      summary,
		ErrorMessage: errorMsg,
		FailureClass: failClass,
	}); err != nil {
		log.Printf("workflow recorder: FinishRun failed: %v", err)
	}

	// Clean up seq counter.
	r.mu.Lock()
	delete(r.seqMap, runID)
	r.mu.Unlock()
}

// AddArtifact attaches an artifact reference to a run/step.
func (r *Recorder) AddArtifact(ctx context.Context, runID string, stepSeq int32, kind workflowpb.ArtifactKind, name, version, path string) {
	if r == nil || r.client == nil || runID == "" {
		return
	}
	if _, err := r.client.AddArtifactRef(ctx, &workflowpb.AddArtifactRefRequest{
		ClusterId: r.clusterID,
		Artifact: &workflowpb.WorkflowArtifactRef{
			RunId:   runID,
			StepSeq: stepSeq,
			Kind:    kind,
			Name:    name,
			Version: version,
			Path:    path,
		},
	}); err != nil {
		log.Printf("workflow recorder: AddArtifact failed: %v", err)
	}
}

// CorrelationID builds a stable correlation ID for a reconciliation lineage.
func CorrelationID(releaseKind, component, nodeID string) string {
	return fmt.Sprintf("%s/%s/%s", releaseKind, component, nodeID)
}
