// Package govops is the canonical model and validator for the Governed Operation
// Gateway (Slice 2). It defines the refusal rules that decide whether an
// OperationRequest may mutate owner-owned cluster state, and the adapters proving
// this model subsumes the existing MCP governor and the audittrail desired-write
// record rather than running as a second, divergent gate.
//
// The wire schema lives in proto/governed_operation.proto (package
// governed_operation); this package operates on those generated types and adds
// the behavior the schema cannot express.
package govops

import (
	"strings"

	pb "github.com/globulario/services/golang/govops/governed_operationpb"
)

// DecisionKind mirrors the `awg op-briefing` decision vocabulary.
type DecisionKind string

const (
	DecisionAllowed        DecisionKind = "allowed"
	DecisionRefused        DecisionKind = "refused"
	DecisionNeedsOwnerPath DecisionKind = "needs_owner_path"
	DecisionBreakGlassOnly DecisionKind = "break_glass_only"
)

// Violation is one reason an operation is not a clean governed mutation.
type Violation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Decision is the verdict for an OperationRequest. Violations are advisory on a
// DRY_RUN (they show what an APPLY would trip) and binding otherwise.
type Decision struct {
	Kind       DecisionKind `json:"decision"`
	Violations []Violation  `json:"violations,omitempty"`
}

// Refused reports whether the decision blocks a mutating apply.
func (d Decision) Refused() bool {
	return d.Kind == DecisionRefused || d.Kind == DecisionNeedsOwnerPath
}

func mutatesDesiredOrSpec(r *pb.OperationRequest) bool {
	e := r.GetExpectedEffect()
	return e.GetMutatesDesired() || e.GetMutatesSpec()
}

func mutatesOwnerState(r *pb.OperationRequest) bool {
	e := r.GetExpectedEffect()
	return e.GetMutatesDesired() || e.GetMutatesSpec() || e.GetMutatesStatus()
}

func ownerOwned(r *pb.OperationRequest) bool {
	return strings.TrimSpace(r.GetTarget().GetOwner()) != ""
}

// Validate applies the Governed Operation Gateway refusal rules to a request.
//
// The Gateway refuses execution when (per the gateway instruction set):
//   - the target has an owner but the operation is not routed through that owner;
//   - the operation mutates desired/spec/status by a raw storage write;
//   - generation should change but no generation bump is declared;
//   - reconcile is required but no reconcile trigger is declared;
//   - the derived projection is not refreshed;
//   - postcondition checks / rollback plan / ledger are missing;
//   - the before-snapshot is missing;
//   - the operation cannot explain its authority.
//
// DRY_RUN never mutates, so it is always allowed — but the same checks run and are
// returned as advisory violations so a preflight shows what an APPLY would trip.
// BREAK_GLASS is allowed only when it carries a before-snapshot, a rollback plan,
// a ledger requirement, and a post-reconcile check; otherwise it is refused.
func Validate(r *pb.OperationRequest) Decision {
	mode := r.GetExecution().GetMode()

	switch mode {
	case pb.ExecutionMode_DRY_RUN:
		// Plan only: never mutates. Surface would-be violations as advisory.
		d := evaluateApply(r)
		return Decision{Kind: DecisionAllowed, Violations: d.Violations}
	case pb.ExecutionMode_BREAK_GLASS:
		return evaluateBreakGlass(r)
	default: // APPLY (and UNSPECIFIED, treated as a mutating apply — fail closed).
		return evaluateApply(r)
	}
}

func evaluateApply(r *pb.OperationRequest) Decision {
	if !mutatesOwnerState(r) {
		// A read or an unowned/no-effect operation is not gated.
		return Decision{Kind: DecisionAllowed}
	}

	var vs []Violation
	wrongOwnerPath := false
	path := r.GetExecution().GetApprovedPath()
	e := r.GetExpectedEffect()

	// 1 / 2. Owner-owned state must be routed through the owner path; a raw write
	// (read-only/forbidden/unspecified path) to owner-owned state is the core refusal.
	if ownerOwned(r) {
		switch path {
		case pb.ApprovedPath_OWNER_RPC, pb.ApprovedPath_CONTROLLER_COMMAND:
			// routed correctly.
		case pb.ApprovedPath_FORBIDDEN:
			vs = append(vs, Violation{"forbidden_path", "the graph/behavioral-memory rules this mutation path out"})
		default: // DIAGNOSTIC_READONLY or UNSPECIFIED applied to a mutating op = raw write.
			wrongOwnerPath = true
			vs = append(vs, Violation{"owner_path_required", "owner-owned state must be mutated through the owner RPC / controller path, not a raw write"})
		}
	}

	// 3. Generation bump required for desired/spec mutation.
	if mutatesDesiredOrSpec(r) && !e.GetBumpsGeneration() {
		vs = append(vs, Violation{"generation_bump_required", "desired/spec mutation must bump the owner generation"})
	}
	// 4. Reconcile trigger required for desired/spec mutation.
	if mutatesDesiredOrSpec(r) && !e.GetTriggersReconcile() {
		vs = append(vs, Violation{"reconcile_required", "desired/spec mutation must trigger reconcile"})
	}
	// 5. Derived projection must be refreshed for desired/spec mutation.
	if mutatesDesiredOrSpec(r) && !e.GetRefreshesProjection() {
		vs = append(vs, Violation{"projection_refresh_required", "desired/spec mutation must refresh the derived projection"})
	}

	// 6. Before-snapshot required.
	if strings.TrimSpace(r.GetEvidence().GetBeforeSnapshot()) == "" {
		vs = append(vs, Violation{"before_snapshot_required", "a before-snapshot must be captured before mutation"})
	}

	// 7. Authority must be explained.
	auth := r.GetAuthority()
	if strings.TrimSpace(auth.GetRequiredOwnerPath()) == "" || strings.TrimSpace(auth.GetCallerIdentity()) == "" {
		vs = append(vs, Violation{"authority_unexplained", "the operation must declare its required owner path and caller identity"})
	}

	// 8. Postconditions: checks, rollback plan, ledger.
	pc := r.GetPostconditions()
	if len(pc.GetRequiredChecks()) == 0 {
		vs = append(vs, Violation{"postcondition_checks_missing", "at least one postcondition check is required"})
	}
	if strings.TrimSpace(pc.GetRollbackPlan()) == "" {
		vs = append(vs, Violation{"rollback_plan_missing", "a rollback plan is required"})
	}
	if !pc.GetLedgerRequired() {
		vs = append(vs, Violation{"ledger_required", "the operation must produce a ledger entry"})
	}

	if len(vs) == 0 {
		return Decision{Kind: DecisionAllowed}
	}
	// If the ONLY problem is the owner path, the actionable verdict is needs_owner_path.
	if wrongOwnerPath && len(vs) == 1 {
		return Decision{Kind: DecisionNeedsOwnerPath, Violations: vs}
	}
	return Decision{Kind: DecisionRefused, Violations: vs}
}

func evaluateBreakGlass(r *pb.OperationRequest) Decision {
	var vs []Violation
	if strings.TrimSpace(r.GetEvidence().GetBeforeSnapshot()) == "" {
		vs = append(vs, Violation{"before_snapshot_required", "break-glass requires a before-snapshot"})
	}
	pc := r.GetPostconditions()
	if strings.TrimSpace(pc.GetRollbackPlan()) == "" {
		vs = append(vs, Violation{"rollback_plan_missing", "break-glass requires a rollback plan"})
	}
	if !pc.GetLedgerRequired() {
		vs = append(vs, Violation{"ledger_required", "break-glass requires a ledger entry"})
	}
	if !hasReconcileCheck(pc.GetRequiredChecks()) {
		vs = append(vs, Violation{"post_reconcile_required", "break-glass requires a post-recovery reconcile check"})
	}
	if len(vs) > 0 {
		return Decision{Kind: DecisionRefused, Violations: vs}
	}
	return Decision{Kind: DecisionBreakGlassOnly}
}

func hasReconcileCheck(checks []string) bool {
	for _, c := range checks {
		if strings.Contains(strings.ToLower(c), "reconcile") {
			return true
		}
	}
	return false
}
