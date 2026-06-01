// persist.go wires the workflow engine to the existing Workflow gRPC service
// for ScyllaDB-backed persistence. Each engine run/step maps to the
// workflow service's StartRun/RecordStep/FinishRun RPCs.
package engine

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// WorkflowPersister writes engine events to the Workflow gRPC service
// (ScyllaDB-backed). Wire it to Engine.OnStepDone and call
// PersistStartRun/PersistFinishRun around Execute.
type WorkflowPersister struct {
	Client    workflowpb.WorkflowServiceClient
	ClusterID string
}

// PersistStartRun creates a workflow run record in ScyllaDB.
func (p *WorkflowPersister) PersistStartRun(ctx context.Context, run *Run, workflowName string) {
	if p.Client == nil {
		return
	}
	nodeID := fmt.Sprint(run.Inputs["node_id"])
	hostname := fmt.Sprint(run.Inputs["node_hostname"])

	wfRun := &workflowpb.WorkflowRun{
		Id:            run.ID,
		CorrelationId: run.ID,
		Context: &workflowpb.WorkflowContext{
			ClusterId:     p.ClusterID,
			NodeId:        nodeID,
			NodeHostname:  hostname,
			ComponentName: workflowName,
		},
		TriggerReason: workflowpb.TriggerReason_TRIGGER_REASON_BOOTSTRAP,
		Status:        workflowpb.RunStatus_RUN_STATUS_EXECUTING,
		Summary:       fmt.Sprintf("workflow %s started", workflowName),
		StartedAt:     timestamppb.Now(),
		WorkflowName:  workflowName,
	}

	_, err := p.Client.StartRun(ctx, &workflowpb.StartRunRequest{Run: wfRun})
	if err != nil {
		log.Printf("persist: StartRun failed: %v", err)
	}
}

// OnStepDone records each step completion in ScyllaDB.
// Wire this to Engine.OnStepDone.
func (p *WorkflowPersister) OnStepDone(run *Run, step *StepState) {
	if p.Client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status := workflowpb.StepStatus_STEP_STATUS_RUNNING
	switch step.Status {
	case StepSucceeded:
		status = workflowpb.StepStatus_STEP_STATUS_SUCCEEDED
	case StepFailed:
		status = workflowpb.StepStatus_STEP_STATUS_FAILED
	case StepSkipped:
		status = workflowpb.StepStatus_STEP_STATUS_SKIPPED
	}

	var startedAt, finishedAt *timestamppb.Timestamp
	if !step.StartedAt.IsZero() {
		startedAt = timestamppb.New(step.StartedAt)
	}
	if !step.FinishedAt.IsZero() {
		finishedAt = timestamppb.New(step.FinishedAt)
	}

	durationMs := int64(0)
	if !step.StartedAt.IsZero() && !step.FinishedAt.IsZero() {
		durationMs = step.FinishedAt.Sub(step.StartedAt).Milliseconds()
	}

	wfStep := &workflowpb.WorkflowStep{
		RunId:      run.ID,
		StepKey:    step.ID,
		Title:      step.ID,
		Status:     status,
		Attempt:    int32(step.Attempt),
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		DurationMs: durationMs,
		Message:    step.Error,
	}

	_, err := p.Client.RecordStep(ctx, &workflowpb.RecordStepRequest{
		ClusterId: p.ClusterID,
		Step:      wfStep,
	})
	if err != nil {
		log.Printf("persist: RecordStep %s failed: %v", step.ID, err)
	}
}

// PersistFinishRun marks the run as completed in ScyllaDB.
func (p *WorkflowPersister) PersistFinishRun(ctx context.Context, run *Run) {
	if p.Client == nil {
		return
	}

	status := workflowpb.RunStatus_RUN_STATUS_SUCCEEDED
	if run.Status == RunFailed {
		status = workflowpb.RunStatus_RUN_STATUS_FAILED
	}

	_, err := p.Client.FinishRun(ctx, &workflowpb.FinishRunRequest{
		Id:           run.ID,
		ClusterId:    p.ClusterID,
		Status:       status,
		Summary:      fmt.Sprintf("workflow %s: %s", run.Definition, run.Status),
		ErrorMessage: run.Error,
	})
	if err != nil {
		log.Printf("persist: FinishRun failed: %v", err)
	}
}
