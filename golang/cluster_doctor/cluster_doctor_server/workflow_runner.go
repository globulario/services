package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// remediateDoctorFindingYAML is the embedded workflow definition. Shipping
// it inside the binary avoids a separate distribution step; the workflow
// runs from whatever doctor binary you deploy.
//
// Kept in sync manually with services/golang/workflow/definitions/
// remediate.doctor.finding.yaml. A compile-time test asserts the file
// loads cleanly on every build.
//
//go:embed workflow_remediate_doctor_finding.yaml
var remediateDoctorFindingYAML []byte

// RunRemediationWorkflow executes the remediate.doctor.finding workflow
// in-process. All side-effects go through the existing ExecuteRemediation
// handler — the workflow wraps it, never bypasses it.
//
// Returns the engine.Run so callers can inspect outputs per step
// (resolved_finding, risk_assessment, execution_result, verification).
func (s *ClusterDoctorServer) RunRemediationWorkflow(
	ctx context.Context,
	findingID string,
	stepIndex uint32,
	approvalToken string,
	dryRun bool,
) (*engine.Run, error) {
	if findingID == "" {
		return nil, fmt.Errorf("finding_id is required")
	}

	loader := v1alpha1.NewLoader()
	def, err := loader.LoadBytes(remediateDoctorFindingYAML)
	if err != nil {
		return nil, fmt.Errorf("load embedded workflow: %w", err)
	}

	router := engine.NewRouter()
	engine.RegisterDoctorRemediationActions(router, s.buildDoctorRemediationConfig())
	eng := &engine.Engine{Router: router}

	inputs := map[string]any{
		"finding_id":     findingID,
		"step_index":     stepIndex,
		"approval_token": approvalToken,
		"dry_run":        dryRun,
	}
	run, _ := eng.Execute(ctx, def, inputs)
	if run == nil {
		return nil, fmt.Errorf("workflow engine returned nil run")
	}
	return run, nil
}

// buildDoctorRemediationConfig wires the five pipeline callbacks the
// workflow engine invokes. All callbacks stay in-process — no extra gRPC
// hops. This keeps the wrapped ExecuteRemediation semantics identical to
// the direct RPC path.
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
					HasAction: false,
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
			// Re-run a targeted scan. If the finding clears, converged.
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
			// Refresh cache so a follow-up ExecuteRemediation call still
			// finds its anchor even if the verify ran a node-only scan.
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
