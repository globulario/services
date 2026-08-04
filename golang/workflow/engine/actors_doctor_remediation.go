// @awareness namespace=globular.platform
// @awareness component=platform_workflow.actors_doctor_remediation
// @awareness file_role=correlated_doctor_remediation_with_workflow_run_audit_trail
// @awareness implements=globular.platform:intent.workflow.source_of_operational_truth
// @awareness implements=globular.platform:intent.workflow.step_receipts_are_evidence
// @awareness risk=high
package engine

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/globulario/services/golang/remediation"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// withRemediationCorrelation stamps the doctor-facing context with the
// workflow run id and a correlation id derived from the run + step so
// every audit record the doctor writes joins back to the workflow run.
// The doctor reads these via remediation.CorrelationFromContext /
// WorkflowRunFromContext (incoming-context path). See
// docs/intent/audit.retention_and_correlation_policy.yaml.
func withRemediationCorrelation(ctx context.Context, req ActionRequest) context.Context {
	correlationID := req.RunID
	if req.StepID != "" {
		correlationID = req.RunID + "/" + req.StepID
	}
	return remediation.WithCorrelationAsIncoming(ctx, correlationID, req.RunID)
}

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

	// Identity of the thing being remediated. Carried so post-action
	// verification can be bound to the SAME subject rather than merely to a
	// finding id.
	//
	// EntityRef is NOT NodeID. They coincide for node-scoped findings and
	// diverge for service- or cluster-scoped ones, so deriving one from the
	// other would silently attribute a verification to the wrong subject.
	//
	// ClusterID matters beyond bookkeeping: downstream evidence is indexed by
	// cluster, so verification carrying an empty cluster would be written to
	// the cluster-less partition and become unfindable by any cluster-scoped
	// query — present in the store, invisible to the reader.
	ClusterID   string `json:"cluster_id"`
	InvariantID string `json:"invariant_id"`
	EntityRef   string `json:"entity_ref"`
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
	Converged           bool `json:"converged"`
	FindingStillPresent bool `json:"finding_still_present"`
	RemainingRelated    int  `json:"remaining_related"`
}

// DoctorRemediationConfig provides dependencies for doctor remediation
// orchestration. All fields are optional; nil fields use inert defaults
// so tests can run without a real doctor client.
type DoctorRemediationConfig struct {
	// ResolveFinding returns the shape cluster_doctor exposes via its
	// finding cache: the finding's structured RemediationAction.
	// runID is the workflow run this resolution belongs to. It is passed
	// explicitly rather than read from context because it is an identity input,
	// not ambient metadata: an autonomous caller binds the exact finding it
	// selected to this run id before starting it, and the resolver must be able
	// to demand THAT finding rather than re-reading a mutable cache that may
	// have changed between selection and dispatch.
	// bindingMode is the run's explicit resolution contract, carried as a
	// workflow input. FindingBindingAutonomousRequired means a run-scoped
	// binding MUST exist and match; absence or mismatch must fail closed.
	// FindingBindingOperatorCurrent means resolving the current cached finding
	// is correct. It is passed rather than inferred because binding absence has
	// many causes — premature cleanup, cancellation, an RPC timeout with
	// callbacks still in flight, a mismatched run id, a bug — and treating any
	// of them as "operator run" would silently substitute a different finding.
	ResolveFinding func(ctx context.Context, runID, findingID string, stepIndex uint32, bindingMode string) (*ResolvedFinding, error)

	// ExecuteRemediation forwards to cluster_doctor.ExecuteRemediation.
	// The workflow never executes side-effects outside this call.
	// runID and bindingMode travel with the execution, not just the
	// resolution. A run-scoped binding that guards resolve_finding but not
	// execute_remediation protects the wrong half: the executor can still
	// re-resolve the finding from a mutable cache and mutate a different
	// subject than the one the caller bound.
	ExecuteRemediation func(ctx context.Context, runID, findingID string, stepIndex uint32,
		approvalToken string, dryRun bool, bindingMode string) (*ExecutionResult, error)

	// VerifyConvergence re-runs doctor (GetNodeReport) and reports
	// whether the finding has cleared.
	// dispatchedAt is the instant the executor confirmed the action ran. The
	// verifier MUST prove its evidence was collected strictly after it — a
	// snapshot that predates the repair still shows the finding present, which
	// would record a successful repair as a failure and teach the learning loop
	// the opposite of what happened. Zero means dispatch never happened or was
	// never timestamped; a verifier that cannot place its evidence after the
	// action must refuse rather than guess.
	VerifyConvergence func(ctx context.Context, findingID, nodeID string, dispatchedAt time.Time) (*Verification, error)

	// MarkFailed is called via onFailure hook when the workflow ends
	// in a non-terminal-success state.
	MarkFailed func(ctx context.Context, findingID string) error

	// Now supplies the clock for dispatch timestamping. Injected rather than
	// calling time.Now directly so lineage tests can assert an exact
	// dispatched_at instead of a wall-clock approximation. Defaults to
	// time.Now when nil.
	Now func() time.Time

	// ObserveOutcome receives every assembled remediation.Outcome — successful
	// or not — so the owning service can record what the repair achieved.
	//
	// Deliberately shaped as a fire-and-forget sink over the CANONICAL Outcome:
	//
	//   - No error return. Learning is subordinate to remediation. A behavioral
	//     persistence failure must not turn a verified repair into a failed
	//     workflow step, which is exactly what an error here would do.
	//   - Outcome, not evidence. The engine stays a workflow engine; it does not
	//     import behavioral-memory types or decide what is worth recording.
	//     Whoever supplies the hook owns that policy.
	//   - Called for every outcome, including unsuccessful ones. Filtering here
	//     would hide failed repairs from the learning path, and a system that
	//     only remembers its successes learns the wrong lesson.
	//
	// Implementations MUST NOT block: this runs inside the verify step.
	ObserveOutcome func(ctx context.Context, o remediation.Outcome)

	// GateAction asks the behavioral governor whether this remediation may be
	// dispatched. Called immediately before the executor, so a refusal stops the
	// action rather than annotating one that already happened.
	//
	// Expressed in engine terms rather than behavioral-memory types for the same
	// reason as ObserveOutcome: the workflow engine coordinates, it does not
	// import the governor. Whoever supplies the hook owns that translation.
	//
	// An error means the gate could not decide. The handler treats that as a
	// refusal — see doctorExecuteRemediation. Nil means no governor is wired and
	// the existing safety gates decide alone.
	GateAction func(ctx context.Context, req GateRequest) (GateVerdict, error)
}

// GateRequest is the real action context a governed check is made against. Every
// field is read from workflow state, never invented: a gate asked about the
// wrong subject returns a confident answer to a question nobody posed.
// Finding resolution contracts, carried as the finding_binding_mode workflow
// input. See remediate.doctor.finding.yaml.
const (
	// FindingBindingAutonomousRequired — the run was started for ONE exact
	// finding bound to its run id before the run began. Resolution must use
	// that binding and fail closed otherwise.
	FindingBindingAutonomousRequired = "autonomous_required"
	// FindingBindingOperatorCurrent — an operator named a finding id and
	// expects the current finding.
	FindingBindingOperatorCurrent = "operator_current"
)

// Canonical dispatch dispositions written to the run's dispatch_result output.
//
// These exist so a caller can classify a run from STRUCTURED fields that
// survive a failed step, rather than by matching error text. A refusal and an
// executor malfunction are different worlds — one is governance working, the
// other charges a circuit breaker — and a classifier that told them apart by
// string would change meaning the next time a message was reworded.
const (
	DispositionRefused         = "REFUSED"
	DispositionExecutionFailed = "EXECUTION_FAILED"
)

type GateRequest struct {
	FindingID     string
	ClusterID     string
	InvariantID   string
	EntityRef     string
	NodeID        string
	ActionKind    string
	WorkflowRunID string
	StepIndex     uint32
	// ApprovalToken is the doctor's existing operator approval. Passed through
	// so the governor can see approval was obtained; it does NOT replace the
	// executor's own approval gate, which still runs.
	ApprovalToken string
}

// GateVerdict is the governor's answer.
//
// Governed and Allowed are separate on purpose. An ungoverned allow ("no
// applicable principle") and a governed allow ("principles satisfied") are
// different facts, and collapsing them would make the gate's reach unmeasurable
// — every unreviewed action would look approved.
type GateVerdict struct {
	ActionCheckID string
	Governed      bool
	Allowed       bool
	Status        string // allowed|blocked|needs_evidence|needs_authority|needs_human_approval
	Reason        string
	PrincipleIDs  []string
}

func (c DoctorRemediationConfig) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// RegisterDoctorRemediationActions registers the controller-side handlers
// for the remediate.doctor.finding workflow. Call this alongside
// RegisterReconcileControllerActions at cluster-controller boot.
func RegisterDoctorRemediationActions(router *Router, cfg DoctorRemediationConfig) {
	router.Register(v1alpha1.ActorClusterDoctor, "doctor.resolve_finding", doctorResolveFinding(cfg))
	router.Register(v1alpha1.ActorClusterDoctor, "doctor.assess_risk", doctorAssessRisk())
	router.Register(v1alpha1.ActorClusterDoctor, "doctor.require_approval", doctorRequireApproval())
	router.Register(v1alpha1.ActorClusterDoctor, "doctor.execute_remediation", doctorExecuteRemediation(cfg))
	router.Register(v1alpha1.ActorClusterDoctor, "doctor.verify_convergence", doctorVerifyConvergence(cfg))
	router.Register(v1alpha1.ActorClusterDoctor, "doctor.mark_failed", doctorMarkFailed(cfg))
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
		ctx = withRemediationCorrelation(ctx, req)
		// Default is the operator contract, matching the definition's default.
		// An autonomous caller must say so explicitly.
		bindingMode := toStr(req.With["finding_binding_mode"])
		if bindingMode == "" {
			bindingMode = FindingBindingOperatorCurrent
		}
		rf, err := cfg.ResolveFinding(ctx, req.RunID, findingID, stepIndex, bindingMode)
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
			// Subject identity, exported so the verify step can bind the
			// verification to the same subject the action was resolved against.
			// entity_ref is its own key and is never folded into node_id.
			"cluster_id":   rf.ClusterID,
			"invariant_id": rf.InvariantID,
			"entity_ref":   rf.EntityRef,
		}
		// Named delta, not a bare field map. The engine merges result.Output at
		// the run-output ROOT, so returning bare fields would publish
		// finding_id/node_id/... individually and leave $.resolved_finding
		// undefined for the next step — which is exactly what happens across the
		// actor RPC, where the handler's req.Outputs write is a dead local copy.
		// One shape for both topologies.
		req.Outputs["resolved_finding"] = out
		return &ActionResult{OK: true, Output: map[string]any{"resolved_finding": out}}, nil
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
		return &ActionResult{OK: true, Output: map[string]any{"risk_assessment": out}}, nil
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
		ctx = withRemediationCorrelation(ctx, req)

		// ── the governed gate ───────────────────────────────────────────────
		//
		// Placed HERE, before the executor call, because a governor consulted
		// after dispatch annotates history instead of deciding. Everything the
		// check is about is read from the resolved finding this run already
		// produced — the gate must judge the action actually about to happen.
		verdict, gateErr := runRemediationGate(ctx, cfg, req, findingID, stepIndex, approvalToken)
		if gateErr != nil {
			// Do NOT fabricate an allow. A governor that could not decide has
			// not decided in favour; treating an unreachable gate as consent
			// would make governance strongest exactly when it is working and
			// absent exactly when it is not.
			// Governance UNAVAILABLE is not governance REFUSED. Both stop the
			// action, but only the second is a decision; the first is an
			// infrastructure failure and must charge the circuit breaker.
			dr := map[string]any{
				"disposition":            DispositionExecutionFailed,
				"governance_unavailable": true,
				"executed":               false,
				"verified":               false,
				"converged":              false,
			}
			req.Outputs["dispatch_result"] = dr
			// Returned as an explicit delta, NOT only as a req.Outputs mutation:
			// across the actor RPC that map is a deserialized local copy that
			// dies with the call, so the receipt must ride the response.
			return &ActionResult{OK: false, Output: map[string]any{"dispatch_result": dr},
					Message: "governance check unavailable"},
				fmt.Errorf("execute_remediation: governance check unavailable, refusing to dispatch: %w", gateErr)
		}
		if verdict != nil {
			// Recorded whatever the verdict, so a blocked action leaves the same
			// audit trail as an allowed one.
			req.Outputs["governance"] = gateVerdictAsMap(*verdict)
			if !verdict.Allowed {
				// The refusal receipt is RETURNED as an explicit output delta,
				// not merely written into req.Outputs.
				//
				// In-process, req.Outputs is the run's live map and a write
				// survives. Across the actor RPC it is a deserialized local copy
				// that dies with the call, and ExecuteAction previously dropped
				// OutputJson entirely on failure — so the receipt vanished, the
				// refusal classified as EXECUTION_FAILED, and a correct
				// governance decision charged the healer's circuit breaker.
				// Returning the delta makes both topologies produce the same
				// run-output shape.
				//
				// Nothing executed, so this claims no dispatch and no
				// verification, and no remediation outcome is emitted.
				dr := map[string]any{
					"disposition":     DispositionRefused,
					"action_check_id": verdict.ActionCheckID,
					"status":          verdict.Status,
					"executed":        false,
					"verified":        false,
					"converged":       false,
				}
				req.Outputs["dispatch_result"] = dr
				return &ActionResult{OK: false,
						Output:  map[string]any{"dispatch_result": dr, "governance": gateVerdictAsMap(*verdict)},
						Message: fmt.Sprintf("blocked by governance (status=%s check=%s)", verdict.Status, verdict.ActionCheckID)},
					fmt.Errorf("execute_remediation: blocked by governance (status=%s check=%s): %s",
						verdict.Status, verdict.ActionCheckID, verdict.Reason)
			}
		}

		res, err := cfg.ExecuteRemediation(ctx, req.RunID, findingID, stepIndex, approvalToken, dryRun,
			execBindingMode(req))
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
		// DispatchedAt is stamped HERE — the instant the executor accepted the
		// governed request — and nowhere else. It must not be approval time,
		// workflow start, or anything derived from verification: post-action
		// verification is only meaningful relative to when the action actually
		// went out.
		//
		// Absent when dispatch did not occur. A missing timestamp is the honest
		// record of "nothing was dispatched"; synthesising one would let a
		// never-dispatched remediation later look verified.
		if res.Executed {
			out["dispatched_at"] = cfg.now().UTC().Format(time.RFC3339Nano)
		}
		// The check that authorized this dispatch travels with it, so the later
		// verification can be tied back to the decision rather than merely
		// co-occurring with it.
		if verdict != nil && verdict.ActionCheckID != "" {
			out["action_check_id"] = verdict.ActionCheckID
		}
		req.Outputs["execution_result"] = out
		// Rejections from the RPC are reflected as step failure so the
		// workflow terminates and onFailure runs. The receipt still travels:
		// a failed step that carries no record of WHY is unclassifiable, which
		// is how a governed refusal became an executor failure.
		if !res.Executed && !dryRun {
			return &ActionResult{OK: false, Output: map[string]any{"execution_result": out},
					Message: fmt.Sprintf("execute_remediation rejected: status=%s reason=%s", res.Status, res.Reason)},
				fmt.Errorf("execute_remediation rejected: status=%s reason=%s", res.Status, res.Reason)
		}
		return &ActionResult{OK: true, Output: map[string]any{"execution_result": out}}, nil
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
		ctx = withRemediationCorrelation(ctx, req)

		// The verifier receives the instant the executor accepted the action, so
		// it can prove it read POST-action state rather than assume it. Read from
		// the same execute_remediation output buildRemediationOutcome parses, so
		// the timestamp the verification is judged against and the timestamp the
		// outcome records are one value, never two that can drift.
		//
		// Zero when dispatch did not happen or the step wrote no timestamp. That
		// is not "verify anyway with no constraint": a verifier that cannot place
		// its snapshot after the action must refuse, because a snapshot that
		// predates the repair reports the finding still present and would record
		// a successful repair as failed.
		dispatchedAt := dispatchedAtFromOutputs(req)
		v, err := cfg.VerifyConvergence(ctx, findingID, nodeID, dispatchedAt)
		if err != nil {
			return nil, fmt.Errorf("verify_convergence: %w", err)
		}
		out := map[string]any{
			"converged":             v.Converged,
			"finding_still_present": v.FindingStillPresent,
			"remaining_related":     v.RemainingRelated,
		}
		req.Outputs["verification"] = out

		// Assemble the structured remediation.Outcome from this workflow
		// run's accumulated state. The Outcome encodes the truth-
		// consistency contract: SUCCEEDED iff dispatched + verified +
		// resolved. Emitted as a workflow output so callers (CLI, MCP,
		// dashboards) can read the verdict without re-deriving it from
		// step-level errors. See
		// docs/intent/workflow.remediation_truth_consistency.yaml.
		outcome := buildRemediationOutcome(req, findingID, v, cfg.now)
		req.Outputs["remediation_outcome"] = outcomeAsMap(outcome)
		verifyDelta := map[string]any{
			"verification":        out,
			"remediation_outcome": outcomeAsMap(outcome),
		}

		// Offer the outcome for learning BEFORE the convergence branch below,
		// so a remediation that did not converge is recorded too. Placing this
		// after the early return would mean the behavioral record only ever
		// contained successes.
		if cfg.ObserveOutcome != nil {
			cfg.ObserveOutcome(ctx, outcome)
		}

		if !v.Converged {
			// "Verified but invariant present" — the workflow MUST NOT
			// report success. Failing the step here propagates to the
			// run-level status so a green workflow status cannot hide an
			// unresolved doctor finding. The verification and outcome still
			// travel: this is the case a promotion decision most needs to read.
			return &ActionResult{OK: false, Output: verifyDelta,
					Message: fmt.Sprintf("finding %s still present after remediation (status=%s)",
						findingID, outcome.Status())},
				fmt.Errorf("verify_convergence: finding %s still present after remediation (status=%s)",
					findingID, outcome.Status())
		}
		return &ActionResult{OK: true, Output: verifyDelta}, nil
	}
}

// runRemediationGate consults the governor about the action about to be
// dispatched. Returns (nil, nil) when no governor is wired.
//
// The action context is read from the RESOLVED FINDING this run already
// produced. Nothing here is defaulted or reconstructed: a gate asked about a
// subject the workflow did not resolve would answer a different question, and
// answer it confidently.
func runRemediationGate(ctx context.Context, cfg DoctorRemediationConfig, req ActionRequest,
	findingID string, stepIndex uint32, approvalToken string) (*GateVerdict, error) {

	if cfg.GateAction == nil {
		return nil, nil
	}
	resolved, _ := req.Outputs["resolved_finding"].(map[string]any)
	if resolved == nil {
		// The gate cannot be asked truthfully without the subject. Refusing is
		// the only safe answer: inventing a scope would produce a verdict about
		// nothing, and skipping the gate would dispatch ungoverned while the
		// operator believes governance is on.
		return nil, fmt.Errorf("no resolved_finding in run outputs — cannot build a truthful action context")
	}

	v, err := cfg.GateAction(ctx, GateRequest{
		FindingID:   findingID,
		ClusterID:   toStr(resolved["cluster_id"]),
		InvariantID: toStr(resolved["invariant_id"]),
		// entity_ref is read from its own key and never defaulted to node_id:
		// they coincide for node-scoped findings and diverge for service- and
		// cluster-scoped ones.
		EntityRef:     toStr(resolved["entity_ref"]),
		NodeID:        toStr(resolved["node_id"]),
		ActionKind:    toStr(resolved["action_type"]),
		WorkflowRunID: req.RunID,
		StepIndex:     stepIndex,
		ApprovalToken: approvalToken,
	})
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// gateVerdictAsMap projects the verdict into the run's receipt shape. Recorded
// for blocked and allowed alike — a governed refusal is as much a fact about the
// run as a governed dispatch.
func gateVerdictAsMap(v GateVerdict) map[string]any {
	m := map[string]any{
		"action_check_id": v.ActionCheckID,
		"governed":        v.Governed,
		"allowed":         v.Allowed,
		"status":          v.Status,
	}
	if v.Reason != "" {
		m["reason"] = v.Reason
	}
	if len(v.PrincipleIDs) > 0 {
		m["principle_ids"] = v.PrincipleIDs
	}
	return m
}

// outcomeAsMap projects a remediation.Outcome into the workflow's
// generic step-output shape. Tests should assert against the canonical
// Outcome methods (Status / IsSuccess / Reason); this map is for
// out-of-process consumers (CLI, MCP).
//
// 4C-pre added subject identity and dispatch time to the Outcome but did not
// surface them here, so the receipt described a verdict whose subject could not
// be recovered by anyone reading the run. Per
// docs/intent/workflow.step_receipts_are_evidence, a receipt must carry the
// identity metadata needed to reconstruct what happened — a status without a
// subject is not reconstructable.
//
// Key names are the ones the workflow already established (resolved_finding's
// cluster_id/invariant_id/entity_ref, execution_result's dispatched_at). A
// second name for a field that already has one would make two spellings of the
// same fact, and out-of-process readers would have to know which to trust.
func outcomeAsMap(o remediation.Outcome) map[string]any {
	m := map[string]any{
		"finding_id":       o.FindingID,
		"workflow_run_id":  o.WorkflowRunID,
		"dispatched":       o.Dispatched,
		"verified":         o.Verified,
		"finding_resolved": o.FindingResolved,
		"dispatch_error":   o.DispatchError,
		"status":           string(o.Status()),
		"reason":           o.Reason(),
		"is_success":       o.IsSuccess(),

		// Subject identity. entity_ref is its own key and is never folded into
		// node_id: they coincide for node-scoped findings and diverge for
		// service- and cluster-scoped ones.
		"cluster_id":   o.ClusterID,
		"invariant_id": o.InvariantID,
		"entity_ref":   o.EntityRef,
		"node_id":      o.NodeID,

		"lineage_complete": o.LineageComplete(),
		// Empty when the action was ungoverned — a real state, and one an
		// out-of-process reader must be able to tell from "governed".
		"action_check_id": o.ActionCheckID,
	}
	// Timestamps are RFC3339Nano, matching what the execute step stamps, and
	// are OMITTED when zero rather than emitted as a zero-value date. A
	// remediation that never dispatched has no dispatch time, and
	// "0001-01-01T00:00:00Z" would read as one.
	if !o.DispatchedAt.IsZero() {
		m["dispatched_at"] = o.DispatchedAt.UTC().Format(time.RFC3339Nano)
	}
	if !o.VerifiedAt.IsZero() {
		m["verified_at"] = o.VerifiedAt.UTC().Format(time.RFC3339Nano)
	}
	// Named defects, so a consumer can tell an unattributable success from a
	// failed repair without re-deriving the rule.
	if defects := o.LineageDefects(); len(defects) > 0 {
		names := make([]string, 0, len(defects))
		for _, d := range defects {
			names = append(names, string(d))
		}
		m["lineage_defects"] = names
	}
	return m
}

// buildRemediationOutcome assembles the verdict from the verify step's
// view of run state. It reads back the execute_remediation output that
// the prior step wrote into req.Outputs so the verdict reflects the full
// resolve → execute → verify chain in one place.
// execBindingMode reads the run's declared resolution contract for the execute
// step. Defaults to the operator contract, matching the definition's default;
// an autonomous caller must say so explicitly.
func execBindingMode(req ActionRequest) string {
	if m := toStr(req.With["finding_binding_mode"]); m != "" {
		return m
	}
	return FindingBindingOperatorCurrent
}

// dispatchedAtFromOutputs reads the executor-accepted instant the
// execute_remediation step recorded, and is the single reader of that value.
//
// Both the post-action verification constraint and the recorded outcome derive
// from this one function on purpose: if they parsed it separately they could
// disagree, and a verification judged against one timestamp while the outcome
// reports another is exactly the kind of split that makes an incorrect record
// look self-consistent.
//
// Returns the zero time when dispatch did not occur or nothing was recorded.
// Never synthesised from verification or wall-clock time.
func dispatchedAtFromOutputs(req ActionRequest) time.Time {
	exec, ok := req.Outputs["execution_result"].(map[string]any)
	if !ok {
		return time.Time{}
	}
	ts, ok := exec["dispatched_at"].(string)
	if !ok || ts == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func buildRemediationOutcome(req ActionRequest, findingID string, v *Verification, now func() time.Time) remediation.Outcome {
	out := remediation.Outcome{
		FindingID:     findingID,
		WorkflowRunID: req.RunID,
		Verified:      true,
		VerifiedAt:    now(),
		// Subject identity travels through the verify step's `with:` block,
		// sourced from the resolved finding. Read here rather than looked up
		// later so the outcome reflects the identity the workflow actually
		// acted on, not whatever the doctor cache holds at report time.
		//
		// EntityRef is read from its own key. It is NOT defaulted to NodeID:
		// they diverge for service- and cluster-scoped findings, and silently
		// substituting one would attribute a verification to the wrong subject.
		ClusterID:   toStr(req.With["cluster_id"]),
		InvariantID: toStr(req.With["invariant_id"]),
		EntityRef:   toStr(req.With["entity_ref"]),
		NodeID:      toStr(req.With["node_id"]),
	}
	if v != nil {
		out.FindingResolved = v.Converged
	}
	if exec, ok := req.Outputs["execution_result"].(map[string]any); ok {
		if executed, ok := exec["executed"].(bool); ok {
			out.Dispatched = executed
		}
		if reason, ok := exec["reason"].(string); ok && reason != "" {
			out.DispatchError = reason
		}
		// Parsed, never synthesised. An unparseable or absent value leaves
		// DispatchedAt zero, which LineageDefects reports as
		// missing_dispatched_at_after_dispatch when dispatch was claimed —
		// visible rather than papered over with VerifiedAt.
		out.DispatchedAt = dispatchedAtFromOutputs(req)
		// The governance decision that authorized this dispatch, carried from
		// the execute step so the later verification ties back to the decision
		// rather than merely following it in time.
		if id, ok := exec["action_check_id"].(string); ok {
			out.ActionCheckID = id
		}
	}
	return out
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
