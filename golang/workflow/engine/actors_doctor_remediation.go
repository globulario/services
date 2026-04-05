package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// --------------------------------------------------------------------------
// Doctor remediation actions (remediate.doctor.finding workflow)
// --------------------------------------------------------------------------
//
// These handlers implement the pipeline:
//   resolve → assess → approve → execute → verify
//
// All doctor side-effects go through cluster_doctor.ExecuteRemediation —
// the workflow wraps that RPC, never bypasses it. See:
//   - services/docs/remediation_workflow.md (directive)
//   - services/docs/architecture/projection-clauses.md (Clause 8)
//   - services/golang/cluster_doctor/cluster_doctor_server/executor.go

// ResolvedFinding is what resolve_finding writes into run outputs.
// Shape is stable and consumed by assess_risk + execute_remediation +
// verify_convergence steps in the workflow YAML.
type ResolvedFinding struct {
	FindingID   string `json:"finding_id"`
	StepIndex   uint32 `json:"step_index"`
	NodeID      string `json:"node_id"`
	ActionType  string `json:"action_type"`
	Risk        string `json:"risk"`
	Idempotent  bool   `json:"idempotent"`
	Description string `json:"description"`
	HasAction   bool   `json:"has_action"`
}

// ExecutionResult mirrors cluster_doctorpb.ExecuteRemediationResponse in a
// map-friendly form for workflow output propagation.
type ExecutionResult struct {
	AuditID  string `json:"audit_id"`
	Status   string `json:"status"`
	Executed bool   `json:"executed"`
	Output   string `json:"output"`
	Reason   string `json:"reason"`
}

// RiskAssessment is the classification output. Mirrors the gate logic
// inside executor.requiresApproval — the workflow step reads it for the
// approval guard. The ExecuteRemediation RPC still re-gates on the
// server side; this step exists to make the pipeline observable.
type RiskAssessment struct {
	AutoExecutable   bool   `json:"auto_executable"`
	RequiresApproval bool   `json:"requires_approval"`
	Reason           string `json:"reason"`
}

// Verification is the output of verify_convergence. A finding is converged
// iff it is no longer present in the doctor's latest snapshot.
type Verification struct {
	Converged            bool `json:"converged"`
	FindingStillPresent  bool `json:"finding_still_present"`
	RemainingRelated     int  `json:"remaining_related"`
}

// DoctorRemediationConfig provides dependencies for doctor remediation
// orchestration. All fields are optional; nil fields use inert defaults
// so tests can run without a real doctor client.
type DoctorRemediationConfig struct {
	// ResolveFinding returns the shape cluster_doctor exposes via its
	// finding cache: the finding's structured RemediationAction.
	ResolveFinding func(ctx context.Context, findingID string, stepIndex uint32) (*ResolvedFinding, error)

	// ExecuteRemediation forwards to cluster_doctor.ExecuteRemediation.
	// The workflow never executes side-effects outside this call.
	ExecuteRemediation func(ctx context.Context, findingID string, stepIndex uint32, approvalToken string, dryRun bool) (*ExecutionResult, error)

	// VerifyConvergence re-runs doctor (GetNodeReport) and reports
	// whether the finding has cleared.
	VerifyConvergence func(ctx context.Context, findingID, nodeID string) (*Verification, error)

	// MarkFailed is called via onFailure hook when the workflow ends
	// in a non-terminal-success state.
	MarkFailed func(ctx context.Context, findingID string) error
}

// RegisterDoctorRemediationActions registers the controller-side handlers
// for the remediate.doctor.finding workflow. Call this alongside
// RegisterReconcileControllerActions at cluster-controller boot.
func RegisterDoctorRemediationActions(router *Router, cfg DoctorRemediationConfig) {
	router.Register(v1alpha1.ActorClusterController, "controller.doctor.resolve_finding", doctorResolveFinding(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.doctor.assess_risk", doctorAssessRisk())
	router.Register(v1alpha1.ActorClusterController, "controller.doctor.require_approval", doctorRequireApproval())
	router.Register(v1alpha1.ActorClusterController, "controller.doctor.execute_remediation", doctorExecuteRemediation(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.doctor.verify_convergence", doctorVerifyConvergence(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.doctor.mark_failed", doctorMarkFailed(cfg))
}

func doctorResolveFinding(cfg DoctorRemediationConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		findingID := toStr(req.With["finding_id"])
		if findingID == "" {
			return nil, fmt.Errorf("resolve_finding: finding_id is required")
		}
		var stepIndex uint32
		if v, ok := req.With["step_index"]; ok {
			stepIndex = toUint32(v)
		}
		if cfg.ResolveFinding == nil {
			return nil, fmt.Errorf("resolve_finding: no ResolveFinding handler configured")
		}
		rf, err := cfg.ResolveFinding(ctx, findingID, stepIndex)
		if err != nil {
			return nil, fmt.Errorf("resolve_finding: %w", err)
		}
		if !rf.HasAction {
			return nil, fmt.Errorf("resolve_finding: finding %s step %d has no structured action", findingID, stepIndex)
		}
		out := map[string]any{
			"finding_id":  rf.FindingID,
			"step_index":  rf.StepIndex,
			"node_id":     rf.NodeID,
			"action_type": rf.ActionType,
			"risk":        rf.Risk,
			"idempotent":  rf.Idempotent,
			"description": rf.Description,
			"has_action":  rf.HasAction,
		}
		req.Outputs["resolved_finding"] = out
		return &ActionResult{OK: true, Output: out}, nil
	}
}

func doctorAssessRisk() ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		rf, _ := req.With["resolved_finding"].(map[string]any)
		if rf == nil {
			if v, ok := req.Outputs["resolved_finding"].(map[string]any); ok {
				rf = v
			}
		}
		if rf == nil {
			return nil, fmt.Errorf("assess_risk: resolved_finding missing from inputs and outputs")
		}
		risk := fmt.Sprint(rf["risk"])
		actionType := fmt.Sprint(rf["action_type"])

		assessment := RiskAssessment{AutoExecutable: true}
		switch risk {
		case "RISK_HIGH":
			assessment.AutoExecutable = false
			assessment.RequiresApproval = true
			assessment.Reason = "RISK_HIGH actions require explicit operator approval"
		case "RISK_MEDIUM":
			assessment.AutoExecutable = false
			assessment.RequiresApproval = true
			assessment.Reason = "RISK_MEDIUM actions require operator approval"
		}
		// Type-specific escalations mirror the server-side executor.
		// The RPC is authoritative; this step is for pipeline visibility.
		switch actionType {
		case "SYSTEMCTL_STOP", "PACKAGE_REINSTALL":
			assessment.AutoExecutable = false
			assessment.RequiresApproval = true
			if assessment.Reason == "" {
				assessment.Reason = actionType + " requires approval by policy"
			}
		}

		out := map[string]any{
			"auto_executable":   assessment.AutoExecutable,
			"requires_approval": assessment.RequiresApproval,
			"reason":            assessment.Reason,
		}
		req.Outputs["risk_assessment"] = out
		return &ActionResult{OK: true, Output: out}, nil
	}
}

// doctorRequireApproval gates on the risk_assessment from the prior step.
// If the assessment says approval is required and no approval_token was
// supplied, the step fails and the workflow terminates. Otherwise it
// passes through. Gating lives here (not in a YAML `when`) because the
// engine's condition language does not support `&&` / dotted paths.
func doctorRequireApproval() ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		ra, _ := req.With["risk_assessment"].(map[string]any)
		if ra == nil {
			if v, ok := req.Outputs["risk_assessment"].(map[string]any); ok {
				ra = v
			}
		}
		if ra == nil {
			return nil, fmt.Errorf("require_approval: risk_assessment missing")
		}
		needs := toBool(ra["requires_approval"])
		if !needs {
			return &ActionResult{OK: true, Output: map[string]any{"gated": false}}, nil
		}
		token := toStr(req.With["approval_token"])
		if token != "" {
			return &ActionResult{OK: true, Output: map[string]any{"gated": true, "approved": true}}, nil
		}
		reason := fmt.Sprint(ra["reason"])
		if reason == "" {
			reason = "approval required"
		}
		return nil, fmt.Errorf("require_approval: %s — rerun with approval_token set", reason)
	}
}

func doctorExecuteRemediation(cfg DoctorRemediationConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		findingID := toStr(req.With["finding_id"])
		if findingID == "" {
			return nil, fmt.Errorf("execute_remediation: finding_id is required")
		}
		stepIndex := toUint32(req.With["step_index"])
		approvalToken := toStr(req.With["approval_token"])
		dryRun := toBool(req.With["dry_run"])

		if cfg.ExecuteRemediation == nil {
			return nil, fmt.Errorf("execute_remediation: no ExecuteRemediation handler configured")
		}
		res, err := cfg.ExecuteRemediation(ctx, findingID, stepIndex, approvalToken, dryRun)
		if err != nil {
			return nil, fmt.Errorf("execute_remediation: %w", err)
		}
		out := map[string]any{
			"audit_id": res.AuditID,
			"status":   res.Status,
			"executed": res.Executed,
			"output":   res.Output,
			"reason":   res.Reason,
		}
		req.Outputs["execution_result"] = out
		// Rejections from the RPC are reflected as step failure so the
		// workflow terminates and onFailure runs.
		if !res.Executed && !dryRun {
			return nil, fmt.Errorf("execute_remediation rejected: status=%s reason=%s", res.Status, res.Reason)
		}
		return &ActionResult{OK: true, Output: out}, nil
	}
}

func doctorVerifyConvergence(cfg DoctorRemediationConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		findingID := toStr(req.With["finding_id"])
		nodeID := toStr(req.With["node_id"])
		if findingID == "" {
			return nil, fmt.Errorf("verify_convergence: finding_id is required")
		}
		if cfg.VerifyConvergence == nil {
			log.Printf("actor[controller]: verify_convergence skipped — no verifier configured")
			out := map[string]any{
				"converged":             true,
				"finding_still_present": false,
				"remaining_related":     0,
			}
			req.Outputs["verification"] = out
			return &ActionResult{OK: true, Output: out}, nil
		}
		v, err := cfg.VerifyConvergence(ctx, findingID, nodeID)
		if err != nil {
			return nil, fmt.Errorf("verify_convergence: %w", err)
		}
		out := map[string]any{
			"converged":             v.Converged,
			"finding_still_present": v.FindingStillPresent,
			"remaining_related":     v.RemainingRelated,
		}
		req.Outputs["verification"] = out
		if !v.Converged {
			return nil, fmt.Errorf("verify_convergence: finding %s still present after remediation", findingID)
		}
		return &ActionResult{OK: true, Output: out}, nil
	}
}

func doctorMarkFailed(cfg DoctorRemediationConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		findingID := toStr(req.With["finding_id"])
		log.Printf("actor[controller]: doctor remediation FAILED for finding %s", findingID)
		if cfg.MarkFailed != nil {
			if err := cfg.MarkFailed(ctx, findingID); err != nil {
				return nil, fmt.Errorf("mark doctor remediation failed: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

// ── Small coercion helpers ───────────────────────────────────────────────────

// toStr returns "" for nil values. fmt.Sprint(nil) yields "<nil>" which
// is dangerous for required-string checks.
func toStr(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func toUint32(v any) uint32 {
	switch x := v.(type) {
	case uint32:
		return x
	case int:
		if x < 0 {
			return 0
		}
		return uint32(x)
	case int64:
		if x < 0 {
			return 0
		}
		return uint32(x)
	case float64:
		if x < 0 {
			return 0
		}
		return uint32(x)
	case string:
		var u uint32
		fmt.Sscanf(x, "%d", &u)
		return u
	}
	return 0
}

func toBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x == "true" || x == "1"
	}
	return false
}
