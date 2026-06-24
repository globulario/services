package govops

import (
	"strings"

	"github.com/globulario/services/golang/audittrail"
	pb "github.com/globulario/services/golang/govops/governed_operationpb"
)

// This file demonstrates that the canonical OperationRequest / OperationLedgerEntry
// SUBSUME the existing MCP governor approval model and the audittrail desired-write
// record, so the governor can become the CLI projection of this model rather than a
// second, divergent gate (the "extend the governor" decision).

// Governor approval scopes, mirrored from golang/mcp/workflow_engine.go
// (ApprovalPolicy.Scope). The governor's scope for a command is derivable from an
// OperationRequest's target + expected effect, so the gateway can reproduce the
// governor's approval verdict without a separate policy table.
const (
	ScopeNone        = ""
	ScopeProduction  = "production"
	ScopeDestructive = "destructive"
	ScopePublish     = "publish"
)

// ApprovalScope returns the governor approval scope an OperationRequest implies.
// Precedence: destructive (irreversible / removal / break-glass) > publish
// (repository visibility) > production (desired/spec mutation) > none.
func ApprovalScope(r *pb.OperationRequest) string {
	if r.GetExecution().GetMode() == pb.ExecutionMode_BREAK_GLASS || isDestructive(r) {
		return ScopeDestructive
	}
	if isPublish(r) {
		return ScopePublish
	}
	if mutatesOwnerState(r) {
		return ScopeProduction
	}
	return ScopeNone
}

func isDestructive(r *pb.OperationRequest) bool {
	a := strings.ToLower(r.GetAction())
	// repair / bootstrap / remove / uninstall are the governor's destructive patterns.
	for _, k := range []string{"repair", "bootstrap", "remove", "uninstall", "delete", "destroy"} {
		if strings.Contains(a, k) {
			return true
		}
	}
	return false
}

func isPublish(r *pb.OperationRequest) bool {
	a := strings.ToLower(r.GetAction())
	if strings.Contains(a, "publish") {
		return true
	}
	sub := strings.ToLower(r.GetTarget().GetSubsystem())
	return sub == "repository" || sub == "publish"
}

// ProjectDesiredWrite projects a completed OperationLedgerEntry onto the existing
// audittrail.DesiredWriteRecord. The ledger carries strictly more than the
// desired-write record, so the existing audit envelope is a lossless-enough
// projection of the ledger — proving the two need not be separate writers.
//
// The returned record satisfies audittrail's required-field contract (service,
// actor, source, action, reason all non-empty) whenever the ledger entry is
// well-formed; Reason is derived from the governing invariants (falling back to the
// command) since the ledger records the contract, not free-text intent.
func ProjectDesiredWrite(e *pb.OperationLedgerEntry) audittrail.DesiredWriteRecord {
	reason := strings.Join(e.GetAwgInvariants(), ", ")
	if strings.TrimSpace(reason) == "" {
		reason = e.GetCommand()
	}
	return audittrail.DesiredWriteRecord{
		Service:   e.GetTargetResource(),
		Actor:     e.GetActor(),
		Source:    e.GetTool(),
		Action:    e.GetCommand(),
		Reason:    reason,
		Timestamp: e.GetTimestamp(),
	}
}
