package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// remediationWorkflowName is the canonical definition name stored in MinIO
// under workflows/remediate.doctor.finding.yaml.
const remediationWorkflowName = "remediate.doctor.finding"

// RunRemediationWorkflow delegates execution to the centralized
// WorkflowService. The workflow service loads the definition from MinIO,
// runs the engine, and dispatches steps back to this doctor via the
// WorkflowActorService.ExecuteAction callback.
//
// All side-effects still go through the existing ExecuteRemediation
// handler (called back via the actor service) — behavioral semantics
// are unchanged.
func (s *ClusterDoctorServer) RunRemediationWorkflow(
	ctx context.Context,
	findingID string,
	stepIndex uint32,
	approvalToken string,
	dryRun bool,
) (*workflowpb.ExecuteWorkflowResponse, error) {
	if findingID == "" {
		return nil, fmt.Errorf("finding_id is required")
	}
	if s.workflowClient == nil {
		return nil, fmt.Errorf("workflow service not configured (workflow_endpoint not set)")
	}

	inputs := map[string]any{
		"finding_id":     findingID,
		"step_index":     stepIndex,
		"approval_token": approvalToken,
		"dry_run":        dryRun,
	}
	inputsJSON, err := json.Marshal(inputs)
	if err != nil {
		return nil, fmt.Errorf("marshal inputs: %w", err)
	}

	// The doctor's own gRPC address is the callback endpoint for the
	// workflow service to dispatch doctor actions back to.
	doctorEndpoint := fmt.Sprintf("localhost:%d", s.cfg.Port)

	resp, err := s.workflowClient.ExecuteWorkflow(ctx, &workflowpb.ExecuteWorkflowRequest{
		ClusterId:    s.clusterID,
		WorkflowName: remediationWorkflowName,
		InputsJson:   string(inputsJSON),
		ActorEndpoints: map[string]string{
			"cluster-doctor": doctorEndpoint,
		},
		CorrelationId: fmt.Sprintf("remediation/%s/%d", findingID, stepIndex),
	})
	if err != nil {
		return nil, fmt.Errorf("execute workflow via WorkflowService: %w", err)
	}

	return resp, nil
}

// buildDoctorActorRouter creates a Router with the doctor's remediation
// action handlers wired to local state. This Router is used by the
// DoctorActorServer to handle callbacks from the workflow service.
func (s *ClusterDoctorServer) buildDoctorActorRouter() *engine.Router {
	router := engine.NewRouter()
	engine.RegisterDoctorRemediationActions(router, s.buildDoctorRemediationConfig())
	return router
}

// buildDoctorRemediationConfig wires the five pipeline callbacks the
// workflow engine invokes via actor callbacks. All callbacks access
// in-process state (finding cache, ExecuteRemediation, collector).
func (s *ClusterDoctorServer) buildDoctorRemediationConfig() engine.DoctorRemediationConfig {
	return engine.DoctorRemediationConfig{
		ResolveFinding: func(ctx context.Context, findingID string, stepIndex uint32) (*engine.ResolvedFinding, error) {
			s.lastFindingsMu.RLock()
			cached := make([]rules.Finding, len(s.lastFindings))
			copy(cached, s.lastFindings)
			s.lastFindingsMu.RUnlock()
			f, ok := rules.FindByID(cached, findingID)
			if !ok {
				return nil, fmt.Errorf("finding %s not in last snapshot — call GetClusterReport first", findingID)
			}
			steps := f.Remediation
			if int(stepIndex) >= len(steps) {
				return nil, fmt.Errorf("step_index %d out of range (finding has %d steps)", stepIndex, len(steps))
			}
			step := steps[stepIndex]
			action := step.GetAction()
			if action == nil {
				return &engine.ResolvedFinding{
					FindingID: findingID, StepIndex: stepIndex,
					HasAction:   false,
					Description: step.GetDescription(),
				}, nil
			}
			params := action.GetParams()
			return &engine.ResolvedFinding{
				FindingID:   findingID,
				StepIndex:   stepIndex,
				NodeID:      params["node_id"],
				ActionType:  action.GetActionType().String(),
				Risk:        action.GetRisk().String(),
				Idempotent:  action.GetIdempotent(),
				Description: action.GetDescription(),
				HasAction:   true,
			}, nil
		},

		ExecuteRemediation: func(ctx context.Context, findingID string, stepIndex uint32, approvalToken string, dryRun bool) (*engine.ExecutionResult, error) {
			resp, err := s.ExecuteRemediation(ctx, &cluster_doctorpb.ExecuteRemediationRequest{
				FindingId:     findingID,
				StepIndex:     stepIndex,
				ApprovalToken: approvalToken,
				DryRun:        dryRun,
			})
			if err != nil {
				return nil, err
			}
			return &engine.ExecutionResult{
				AuditID:  resp.GetAuditId(),
				Status:   resp.GetStatus(),
				Executed: resp.GetExecuted(),
				Output:   resp.GetOutput(),
				Reason:   resp.GetReason(),
			}, nil
		},

		VerifyConvergence: func(ctx context.Context, findingID, nodeID string) (*engine.Verification, error) {
			snap, err := s.collector.GetSnapshot(ctx)
			if err != nil && snap == nil {
				return nil, fmt.Errorf("verify snapshot fetch: %w", err)
			}
			var findings []rules.Finding
			if nodeID != "" {
				findings = s.registry.EvaluateForNode(snap, nodeID)
			} else {
				findings = s.registry.EvaluateAll(snap)
			}
			s.cacheFindings(findings)

			stillPresent := false
			for _, f := range findings {
				if f.FindingID == findingID {
					stillPresent = true
					break
				}
			}
			return &engine.Verification{
				Converged:           !stillPresent,
				FindingStillPresent: stillPresent,
				RemainingRelated:    0,
			}, nil
		},

		MarkFailed: func(ctx context.Context, findingID string) error {
			slog.Warn("remediate.doctor.finding workflow failed",
				"finding_id", findingID,
			)
			return nil
		},
	}
}
