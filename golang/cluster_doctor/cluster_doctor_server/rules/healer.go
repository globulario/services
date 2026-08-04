// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.healer_dispatcher_classifier
// @awareness file_role=classify_only_healer_dispatcher_routes_to_execute_remediation
// @awareness implements=globular.platform:intent.remediation.must_go_through_workflow
// @awareness implements=globular.platform:intent.autonomy.remediation_is_bounded_and_escalates
// @awareness implements=globular.platform:intent.circuit_breakers_protect_convergence
// @awareness risk=high
package rules

import (
	"context"
	"log"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// Healer — classifier-only for PolicyV1 (Milestone 2 of the path-unification
// patch set; see docs/design/auto-healing-path-unification-patch-c.md).
//
// The healer reads invariant findings, classifies each against PolicyV1, and
// for HealAuto findings asks the injected Dispatcher to route the action
// through the gated ExecuteRemediation handler. The healer does NOT call any
// node-agent / workflow / ai-memory / etcd RPC directly — that surface was
// removed when Path B (background-healer mutation) was merged into Path A
// (operator-driven mutation) under one execution gate.
//
// Rate limits and the circuit breaker stay in the healer (they're properties
// of a healer cycle, not of a single dispatch). MaxActions caps how many
// dispatches fire per Evaluate call; MaxFailures stops execution after that
// many Dispatcher errors.
//
// Today's PolicyV1 demotes every HealAuto rule with a non-empty AutoAction
// (delete_stale_cache, seed_ops_knowledge, clear_resolved_drift,
// patch_release_available) to HealPropose. The Dispatcher hook is still
// wired so Milestone 3 can re-promote one rule by changing the policy file
// alone — no infrastructure work needed.
// ──────────────────────────────────────────────────────────────────────────────

// DispatchDisposition is the terminal classification of one dispatch attempt.
//
// EXECUTION IS AN EVENT. CONVERGENCE IS THE SUCCESSFUL OUTCOME.
//
// These are deliberately separate values rather than a bool, because the states
// between them are the ones that used to be reported as success. A systemctl
// restart that completed while the invariant stayed violated is not a fix, and
// counting it as one recreates the operator-truth problem — a report that says
// "auto-fixed" about a cluster that is still broken.
type DispatchDisposition string

const (
	// DispatchRefused — governance declined to authorize the action. No side
	// effect occurred. This is a governed decision working correctly, NOT an
	// executor malfunction: it must stay visible and must never count against
	// the executor failure budget.
	DispatchRefused DispatchDisposition = "REFUSED"

	// DispatchProposed — nothing was attempted. Dry-run rehearsal, an action
	// with no structured representation, or a cooldown. The gate did its job.
	DispatchProposed DispatchDisposition = "PROPOSED"

	// DispatchExecutionFailed — the run could not start, or the executor
	// rejected or failed the action. Counts toward the circuit breaker.
	DispatchExecutionFailed DispatchDisposition = "EXECUTION_FAILED"

	// DispatchExecutedUnverified — the action ran, but no post-action evidence
	// could be obtained, so convergence is UNKNOWN. Deliberately not folded
	// into either success or failure: claiming either would be a guess about a
	// mutation that really happened.
	DispatchExecutedUnverified DispatchDisposition = "EXECUTED_UNVERIFIED"

	// DispatchExecutedNotConverged — the action ran, post-action evidence was
	// obtained, and the finding is STILL PRESENT. A failed repair, and the
	// single most valuable outcome to record honestly: it is what a future
	// promotion decision most needs to know.
	DispatchExecutedNotConverged DispatchDisposition = "EXECUTED_NOT_CONVERGED"

	// DispatchConverged — the action ran, post-action evidence was obtained,
	// and the finding cleared. The ONLY disposition that is an auto-fix.
	DispatchConverged DispatchDisposition = "CONVERGED"
)

// DispatchResult is the structured outcome of one dispatch attempt.
//
// Replaces the (executed, auditID, err) tuple. The tuple could not express the
// difference between "refused by governance", "executed but unverifiable", and
// "executed but still broken" — all three collapsed into executed=false or
// executed=true, and the healer then had to infer intent from error strings.
type DispatchResult struct {
	Disposition DispatchDisposition `json:"disposition"`

	// WorkflowRunID is the real, durably committed Workflow Service run that
	// performed this remediation. Empty only when no run was started (refused
	// before start, or the service was unreachable).
	WorkflowRunID string `json:"workflow_run_id,omitempty"`
	AuditID       string `json:"audit_id,omitempty"`
	// ActionCheckID is the governance decision for this attempt. Populated even
	// on a refusal: a refusal an operator cannot trace back to its decision is
	// not accountable.
	ActionCheckID string `json:"action_check_id,omitempty"`

	// Executed / Verified / Converged are the three independent facts the
	// disposition is derived from, kept so a reader can see WHY without
	// re-parsing the disposition. Verified means post-action evidence was
	// obtained at all — not that it was favourable.
	Executed  bool `json:"executed"`
	Verified  bool `json:"verified"`
	Converged bool `json:"converged"`

	Err error `json:"-"`
}

// HealResult records the outcome of one classification (and, for HealAuto
// proposals, the outcome of the gated dispatch).
type HealResult struct {
	InvariantID string          `json:"invariant_id"`
	EntityRef   string          `json:"entity_ref"`
	Disposition HealDisposition `json:"disposition"`
	Action      string          `json:"action"`

	// DispatchDisposition is the terminal classification of the dispatch, and
	// the field a reader should trust. Executed/Verified/Converged remain as
	// the underlying facts.
	DispatchDisposition DispatchDisposition `json:"dispatch_disposition,omitempty"`
	WorkflowRunID       string              `json:"workflow_run_id,omitempty"`

	Executed bool `json:"executed"`
	// Verified means post-action evidence was obtained. It does NOT mean the
	// repair worked — see Converged. Previously this was set true whenever a
	// dispatch executed, which made "verified" indistinguishable from
	// "acknowledged".
	Verified  bool      `json:"verified"`
	Converged bool      `json:"converged"`
	AuditID   string    `json:"audit_id,omitempty"`
	Error     string    `json:"error,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// HealReport is the structured output of one healer pass.
//
// The counters changed meaning in the commit that made Workflow Service the
// canonical autonomous path. AutoFixed previously counted dispatches the
// Dispatcher reported as EXECUTED; it now counts only findings that were
// verified to have CLEARED. The old meaning let a report claim an auto-fix for
// a cluster that was still broken.
type HealReport struct {
	Timestamp time.Time    `json:"timestamp"`
	Results   []HealResult `json:"results"`

	// AutoFixed counts CONVERGED only — the action ran AND post-action
	// evidence proved the finding cleared.
	AutoFixed int `json:"auto_fixed"`
	Proposed  int `json:"proposed"`
	Observed  int `json:"observed"`

	// Errors is the total of failed mutations (ExecutionFailed +
	// ExecutedNotConverged), retained so existing readers keep working. It
	// deliberately excludes Refused and ExecutedUnverified: a governed refusal
	// is not a malfunction, and an unverifiable execution is not known to have
	// failed.
	Errors int `json:"errors"`

	// The honest breakdown.
	ExecutionFailed      int `json:"execution_failed"`
	ExecutedNotConverged int `json:"executed_not_converged"`
	ExecutedUnverified   int `json:"executed_unverified"`
	Refused              int `json:"refused"`
}

// Dispatcher routes a HealAuto finding's auto-action through the canonical
// remediation path. The cluster-doctor server provides the implementation;
// rules.Healer never touches cluster state, RemoteOps, or etcd directly.
//
// Contract:
//   - Dispatch MUST start a real Workflow Service run of
//     remediate.doctor.finding. Governance, execution, post-action evidence,
//     convergence and outcome recording all belong to that run — the healer
//     neither gates nor records, so exactly one action check and exactly one
//     remediation outcome exist per dispatch.
//   - The returned DispatchResult MUST be derived from the workflow's own
//     outputs, never inferred from error text.
//   - When Workflow Service is unavailable or refuses to start the run, return
//     DispatchExecutionFailed. There is no direct-execution fallback: a repair
//     that cannot be governed and recorded must not happen.
//   - Dry-run dispatches still start a real run with dry_run=true so the audit
//     trail includes the rehearsal, and return DispatchProposed.
type Dispatcher interface {
	Dispatch(ctx context.Context, f Finding, autoAction string, dryRun bool) DispatchResult
}

// Healer evaluates findings against the policy and dispatches HealAuto
// proposals through the Dispatcher. Without a Dispatcher (nil), the healer
// is fail-closed: HealAuto findings are recorded as proposals but never
// dispatched. This is the safe default for Milestone 2.
type Healer struct {
	// DryRun is forwarded to the Dispatcher; no mutation should occur but
	// the gated path is exercised so the audit trail records the rehearsal.
	DryRun bool

	// Dispatcher is the gated dispatch hook the cluster-doctor server wires
	// to ExecuteRemediation. nil means HealAuto findings are recorded but
	// never dispatched — fail-closed behaviour required by Milestone 2's
	// "no direct mutation from rules.Healer" invariant.
	Dispatcher Dispatcher

	// MaxActions caps the number of dispatches per Evaluate call.
	// 0 = unlimited.
	MaxActions int

	// MaxFailures stops further dispatches in a cycle after this many
	// Dispatcher errors. 0 = unlimited.
	MaxFailures int

	// PolicyLookup overrides the default LookupPolicy. Production wiring
	// leaves this nil (LookupPolicy is the source of truth). Tests inject
	// synthetic HealAuto rules here without mutating PolicyV1.
	PolicyLookup func(invariantID string) HealRule
}

// Evaluate classifies findings against PolicyV1 and routes HealAuto
// proposals through the Dispatcher.
//
// Rate limiting: if MaxActions > 0, execution stops after that many
// dispatches (remaining findings are classified but not dispatched). If
// MaxFailures > 0, execution stops after that many failures.
func (h *Healer) Evaluate(ctx context.Context, findings []Finding) HealReport {
	report := HealReport{Timestamp: time.Now()}
	dispatchCount := 0
	failureCount := 0
	rateLimited := false

	lookup := h.PolicyLookup
	if lookup == nil {
		lookup = LookupPolicy
	}

	for _, f := range findings {
		rule := lookup(f.InvariantID)
		result := HealResult{
			InvariantID: f.InvariantID,
			EntityRef:   f.EntityRef,
			Disposition: rule.Disposition,
			Action:      rule.AutoAction,
			Timestamp:   time.Now(),
		}

		switch rule.Disposition {
		case HealAuto:
			if rule.AutoAction == "" {
				// HealAuto with no programmatic action — informational
				// no-op (e.g. cache_missing). Classify as observed and
				// auto-verified.
				result.Verified = true
				report.Observed++
			} else if h.Dispatcher == nil {
				// Fail-closed: no dispatcher means the gated path isn't
				// wired. The healer never mutates directly.
				log.Printf("healer: [no-dispatch] HealAuto finding %s on %s — no Dispatcher wired (Milestone 2 fail-closed)",
					f.InvariantID, f.EntityRef)
				report.Proposed++
			} else if rateLimited {
				result.Error = "rate-limited (max actions or max failures reached)"
				report.Observed++
			} else {
				dr := h.Dispatcher.Dispatch(ctx, f, rule.AutoAction, h.DryRun)
				result.AuditID = dr.AuditID
				result.DispatchDisposition = dr.Disposition
				result.WorkflowRunID = dr.WorkflowRunID
				result.Executed = dr.Executed
				result.Verified = dr.Verified
				result.Converged = dr.Converged
				if dr.Err != nil {
					result.Error = dr.Err.Error()
				}

				// Classification is driven by the structured disposition, never
				// by error text. Only CONVERGED is an auto-fix: the action ran
				// AND post-action evidence proved the finding cleared.
				switch dr.Disposition {
				case DispatchConverged:
					report.AutoFixed++
					log.Printf("healer: dispatch %s CONVERGED for %s (run=%s audit=%s)",
						rule.AutoAction, f.EntityRef, dr.WorkflowRunID, dr.AuditID)

				case DispatchExecutedNotConverged:
					// The action ran and the finding is still present. A failed
					// repair, counted as one — this is the outcome a future
					// promotion decision most needs to be true.
					report.ExecutedNotConverged++
					report.Errors++
					failureCount++
					log.Printf("healer: dispatch %s EXECUTED BUT NOT CONVERGED for %s (run=%s audit=%s)",
						rule.AutoAction, f.EntityRef, dr.WorkflowRunID, dr.AuditID)

				case DispatchExecutedUnverified:
					// A real mutation happened but convergence is UNKNOWN.
					// Visible on its own counter rather than folded into either
					// success or failure, both of which would be a guess.
					report.ExecutedUnverified++
					log.Printf("healer: dispatch %s EXECUTED BUT UNVERIFIED for %s (run=%s audit=%s): %v",
						rule.AutoAction, f.EntityRef, dr.WorkflowRunID, dr.AuditID, dr.Err)

				case DispatchExecutionFailed:
					report.ExecutionFailed++
					report.Errors++
					failureCount++
					log.Printf("healer: dispatch %s EXECUTION FAILED for %s: %v",
						rule.AutoAction, f.EntityRef, dr.Err)

				case DispatchRefused:
					// Governance declined. No side effect, no malfunction, and
					// deliberately NOT charged to the executor failure budget:
					// a refusal that tripped the circuit breaker would let
					// correct governance disable the healer.
					report.Refused++
					log.Printf("healer: dispatch %s REFUSED by governance for %s",
						rule.AutoAction, f.EntityRef)

				default: // DispatchProposed
					report.Proposed++
					log.Printf("healer: dispatch %s PROPOSED for %s (audit=%s, no execution)",
						rule.AutoAction, f.EntityRef, dr.AuditID)
				}
				dispatchCount++
				if h.MaxActions > 0 && dispatchCount >= h.MaxActions {
					rateLimited = true
					log.Printf("healer: rate limit reached (%d dispatches), skipping remaining", h.MaxActions)
				}
				if h.MaxFailures > 0 && failureCount >= h.MaxFailures {
					rateLimited = true
					log.Printf("healer: failure threshold reached (%d failures), stopping execution", h.MaxFailures)
				}
			}
		case HealPropose:
			report.Proposed++
		case HealObserve:
			report.Observed++
		}

		report.Results = append(report.Results, result)
	}

	return report
}
