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

// HealResult records the outcome of one classification (and, for HealAuto
// proposals, the outcome of the gated dispatch).
type HealResult struct {
	InvariantID string          `json:"invariant_id"`
	EntityRef   string          `json:"entity_ref"`
	Disposition HealDisposition `json:"disposition"`
	Action      string          `json:"action"`
	Executed    bool            `json:"executed"`
	Verified    bool            `json:"verified"` // true if Dispatcher reports the gate executed the action
	AuditID     string          `json:"audit_id,omitempty"`
	Error       string          `json:"error,omitempty"`
	Timestamp   time.Time       `json:"timestamp"`
}

// HealReport is the structured output of one healer pass.
type HealReport struct {
	Timestamp time.Time    `json:"timestamp"`
	Results   []HealResult `json:"results"`
	AutoFixed int          `json:"auto_fixed"` // dispatches the Dispatcher reported as executed
	Proposed  int          `json:"proposed"`
	Observed  int          `json:"observed"`
	Errors    int          `json:"errors"`
}

// Dispatcher routes a HealAuto finding's auto-action through the gated
// remediation path. The cluster-doctor server provides the implementation;
// rules.Healer never touches cluster state, RemoteOps, or etcd directly.
//
// Contract:
//   - Dispatch MUST flow through ExecuteRemediation (or an equivalent path
//     that applies the same gates: leader, evidence trust, hard-blocklist,
//     approval/cooldown/failure-rate, etcd audit).
//   - Returning (false, "", nil) is a valid "no-op" outcome — e.g. the auto-
//     action has no RemediationAction representation today, so the gate
//     rejected nothing because nothing was attempted. The healer records
//     this as a proposal, not a failure.
//   - Returning a non-nil error counts toward the MaxFailures circuit
//     breaker; auditID may be empty.
//   - Dry-run dispatches are still expected to call ExecuteRemediation
//     with DryRun=true so the gate's audit trail includes the rehearsal.
type Dispatcher interface {
	Dispatch(ctx context.Context, f Finding, autoAction string, dryRun bool) (executed bool, auditID string, err error)
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
				executed, auditID, err := h.Dispatcher.Dispatch(ctx, f, rule.AutoAction, h.DryRun)
				result.AuditID = auditID
				if err != nil {
					result.Error = err.Error()
					report.Errors++
					failureCount++
					log.Printf("healer: dispatch %s FAILED for %s: %v",
						rule.AutoAction, f.EntityRef, err)
				} else if executed {
					result.Executed = true
					result.Verified = true
					report.AutoFixed++
					log.Printf("healer: dispatch %s EXECUTED for %s (audit=%s)",
						rule.AutoAction, f.EntityRef, auditID)
				} else {
					// Gate accepted but did not execute (dry-run, no
					// RemediationAction representation, cooldown, etc.).
					// Recorded as a proposal — the gate did its job.
					report.Proposed++
					log.Printf("healer: dispatch %s PROPOSED for %s (audit=%s, no execution)",
						rule.AutoAction, f.EntityRef, auditID)
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
