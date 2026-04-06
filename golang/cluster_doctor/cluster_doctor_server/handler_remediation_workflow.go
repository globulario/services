package main

import (
	"context"
	"encoding/json"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StartRemediationWorkflow delegates execution to the centralized
// WorkflowService and translates the response into the doctor's proto.
//
// The workflow service drives the engine and dispatches steps back to
// this doctor via WorkflowActorService.ExecuteAction callbacks.
// Behavioral semantics are identical to the former in-process execution.
func (s *ClusterDoctorServer) StartRemediationWorkflow(
	ctx context.Context,
	req *cluster_doctorpb.StartRemediationWorkflowRequest,
) (*cluster_doctorpb.StartRemediationWorkflowResponse, error) {
	if req.GetFindingId() == "" {
		return nil, status.Error(codes.InvalidArgument, "finding_id is required")
	}

	wfResp, err := s.RunRemediationWorkflow(ctx,
		req.GetFindingId(),
		req.GetStepIndex(),
		req.GetApprovalToken(),
		req.GetDryRun(),
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "run workflow: %v", err)
	}

	resp := &cluster_doctorpb.StartRemediationWorkflowResponse{
		RunId:     wfResp.RunId,
		RunStatus: wfResp.Status,
		RunError:  wfResp.Error,
	}

	// Parse the accumulated outputs from the workflow to populate the
	// response fields that callers (AI agents, CLI) depend on.
	if wfResp.OutputsJson != "" {
		var outputs map[string]any
		if err := json.Unmarshal([]byte(wfResp.OutputsJson), &outputs); err == nil {
			if rf, ok := outputs["resolved_finding"].(map[string]any); ok {
				resp.ResolvedNodeId = asString(rf["node_id"])
				resp.ResolvedActionType = asString(rf["action_type"])
				resp.Risk = asString(rf["risk"])
			}
			if ra, ok := outputs["risk_assessment"].(map[string]any); ok {
				resp.AutoExecutable = asBool(ra["auto_executable"])
				resp.RequiresApproval = asBool(ra["requires_approval"])
			}
			if er, ok := outputs["execution_result"].(map[string]any); ok {
				resp.Executed = asBool(er["executed"])
				resp.ExecuteStatus = asString(er["status"])
				resp.ExecuteOutput = asString(er["output"])
				resp.AuditId = asString(er["audit_id"])
			}
			if v, ok := outputs["verification"].(map[string]any); ok {
				resp.Converged = asBool(v["converged"])
				resp.FindingStillPresent = asBool(v["finding_still_present"])
			}
		}
	}

	return resp, nil
}

// ── tiny any→T coercions specific to this handler ───────────────────────────

func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func asBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}
