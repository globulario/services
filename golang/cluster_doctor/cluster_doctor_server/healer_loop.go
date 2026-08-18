// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.healer
// @awareness file_role=auto_heal_loop_for_bounded_self_correction
// @awareness implements=globular.platform:intent.autonomy.remediation_is_bounded_and_escalates
// @awareness implements=globular.platform:intent.circuit_breakers_protect_convergence
// @awareness implements=globular.platform:intent.remediation.must_go_through_workflow
// @awareness risk=high
// @awareness failure_mode=doctor.healer_auto_remediation_on_reduced_harvest
package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/render"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// ──────────────────────────────────────────────────────────────────────────────
// Periodic healer loop (v3)
//
// Runs in the background on the leader only. Evaluates invariant findings
// against the auto-heal policy on a configurable interval, optionally
// executing safe auto-heal actions.
//
// Behavior by healer_mode:
//   observe  — classify findings, log summary, no mutation
//   dry_run  — classify + log intended actions, no mutation (default)
//   enforce  — execute auto-heal actions for HEAL_AUTO findings
//
// Safety rails:
//   - Only runs when srv.isAuthoritative is true (leader)
//   - Stops immediately when leadership is lost
//   - Rate-limited by healer_max_actions_per_cycle
//   - Circuit breaker: stops execution after 3 failures in a cycle
//   - Every action is logged with timestamp, finding, disposition, result
// ──────────────────────────────────────────────────────────────────────────────

// healerAuditRing is a bounded in-memory ring buffer of recent heal reports.
// Keeps the last N reports for inspection via GetClusterReport or logs.
type healerAuditRing struct {
	mu      sync.Mutex
	reports []rules.HealReport
	maxSize int
}

func newHealerAuditRing(size int) *healerAuditRing {
	if size <= 0 {
		size = 20
	}
	return &healerAuditRing{maxSize: size}
}

func (r *healerAuditRing) push(report rules.HealReport) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reports = append(r.reports, report)
	if len(r.reports) > r.maxSize {
		r.reports = r.reports[len(r.reports)-r.maxSize:]
	}
}

func (r *healerAuditRing) latest() *rules.HealReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.reports) == 0 {
		return nil
	}
	last := r.reports[len(r.reports)-1]
	return &last
}

type healerExecutionDecision struct {
	mode           string
	reducedHarvest bool
}

// decideHealerExecution preserves the operator-configured mode even when the
// cluster snapshot has unrelated collection failures. Finding-level harvest
// honesty is already enforced by rules.Registry: a finding whose own evidence
// source failed is downgraded to INVARIANT_UNKNOWN with CheckError, while a
// finding backed by available evidence remains conclusive and is merely marked
// [reduced-harvest]. rules.Healer applies the final evidence-closure refusal
// before dispatch.
func decideHealerExecution(mode string, dataIncomplete bool) healerExecutionDecision {
	return healerExecutionDecision{
		mode:           mode,
		reducedHarvest: dataIncomplete,
	}
}

// startHealerLoop launches the periodic healer as a background goroutine.
// Only runs when the server is the leader. Stops when ctx is cancelled.
func (s *ClusterDoctorServer) startHealerLoop(ctx context.Context) {
	if !s.cfg.HealerEnabled {
		logger.Info("healer: background loop disabled (healer_enabled=false)")
		return
	}

	interval := s.cfg.healerInterval()
	mode := s.cfg.HealerMode
	maxActions := s.cfg.HealerMaxActionsPerCycle

	logger.Info("healer: background loop starting",
		"mode", mode, "interval", interval, "max_actions", maxActions)

	s.auditRing = newHealerAuditRing(20)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Poll recorder delivery health BEFORE the leader gate.
				// Remediation is leader-only because only one actor may act;
				// observing your own recorder is not an action and needs no
				// election. Gating it would make a follower's failing recorder
				// permanently invisible — the silence #238 exists to break.
				s.projectRecorderHealth(time.Now())

				if !s.isAuthoritative.Load() {
					continue // only leader runs
				}
				s.runHealerCycle(ctx, mode, maxActions)
				// Read the accumulated outcomes back and queue any repeated
				// theme for human review. After the cycle, so this tick's own
				// outcomes are already recorded; inside the leader gate, so N
				// doctors cannot multiply one observation into N review items.
				//
				// Deliberately NOT gated on snapshot harvest, unlike dispatch
				// above (doctor.healer_auto_remediation_on_reduced_harvest).
				// That gate exists because acting on a partial snapshot can be
				// destructive. This call dispatches nothing: it queues a
				// question for a human, whose worst case is one misleading hint
				// that review discards. It also reads outcomes accumulated
				// across many cycles, so gating it on the freshness of a single
				// snapshot would suppress real patterns for an unrelated reason.
				s.synthesizePromotionCandidates(ctx)
			}
		}
	}()
}

func (s *ClusterDoctorServer) runHealerCycle(ctx context.Context, mode string, maxActions int) {
	// Take a fresh snapshot.
	snap, fresh, err := s.takeSnapshot(ctx, cluster_doctorpb.FreshnessMode_FRESHNESS_FRESH)
	if err != nil && snap == nil {
		log.Printf("healer: cycle skipped — snapshot failed: %v", err)
		return
	}

	// Evaluate invariants. The registry applies finding-scoped reduced-harvest
	// policy before any finding reaches the healer: only findings whose own
	// evidence source failed are downgraded to UNKNOWN.
	findings := s.registry.EvaluateAll(snap)

	// Publish cluster-wide findings to the last-snapshot cache so the
	// gated Dispatcher can resolve finding_ids back to Finding objects
	// when ExecuteRemediation looks them up. Cluster-wide=true is correct
	// because EvaluateAll evaluates every registered invariant.
	s.cacheFindings(findings, true)

	// Offer these findings to behavioral memory BEFORE anything is dispatched.
	//
	// The governed gate (behavioral_governance.go) authorizes a remediation only
	// when qualifying evidence exists for the finding — sat.doctor.finding_observed
	// requires an evidence row of kind cluster_doctor_evidence naming the same
	// invariant and entity. The only producer of that row was
	// emitBehavioralClusterReport, called from the GetClusterReport RPC. The
	// consumer is this loop. So an autonomous healer gated on evidence it never
	// emitted: the check returned needs_evidence and the repair never happened,
	// while an operator who happened to pull a report first made the same action
	// succeed. Producer and consumer must live on the same path.
	//
	// Reduced-harvest snapshots are deliberately EXCLUDED, and this survives the
	// move to finding-scoped closure below. Dispatch granularity and evidence
	// minting are separate decisions: the registry downgrades a compromised
	// finding to UNKNOWN so it cannot be ACTED on, but observation.FromDoctorFinding
	// mints a BindSatisfies-bound cluster_doctor_evidence row from a finding's
	// evidence without consulting its verdict. An UNKNOWN finding would therefore
	// still mint standing authorization naming that invariant and entity. If a
	// snapshot is not trusted enough to act on, it is not trusted enough to mint
	// the evidence that authorizes acting later — recording it would launder a
	// possibly-false finding into a future cycle's permission.
	//
	// The cost is bounded and in the safe direction: a finding newly observed
	// during a reduced harvest waits for a clean cycle to mint its evidence.
	// Findings already carrying evidence from earlier clean cycles stay
	// actionable, which is what preserving enforce mode below is for.
	//
	// Delivery stays best-effort: Enqueue is non-blocking by contract, so this
	// cannot delay or fail the cycle. The consequence is that evidence for a
	// newly-observed finding may land after this cycle's gate check, and the
	// repair happens on a subsequent cycle. Emitting here rather than after
	// Evaluate gives the recorder the whole dispatch window to drain.
	if !snap.DataIncomplete {
		s.emitBehavioralClusterReport(render.ClusterReport(snap, findings, s.version, fresh))
	}

	decision := decideHealerExecution(mode, snap.DataIncomplete)
	if decision.reducedHarvest && decision.mode == "enforce" {
		log.Printf("healer: reduced-harvest snapshot (DataIncomplete=true) — preserving enforce mode; findings with compromised evidence are refused individually while unrelated, conclusive findings remain eligible")
	}

	// Determine healer mode. Reduced harvest is not itself execution authority;
	// rules.Healer refuses each auto-action unless its finding remains a
	// conclusive FAIL with no CheckError after registry harvest policy.
	dryRun := decision.mode != "enforce"
	healer := &rules.Healer{
		DryRun:      dryRun,
		Dispatcher:  s.gatedDispatcher(),
		MaxActions:  maxActions,
		MaxFailures: 3,
	}

	report := healer.Evaluate(ctx, findings)

	// Store in audit ring + persistent file.
	if s.auditRing != nil {
		s.auditRing.push(report)
	}
	if s.auditStore != nil {
		s.auditStore.AppendReport(report)
	}

	// Log summary.
	modeLabel := "observe"
	if !dryRun {
		modeLabel = "enforce"
	} else if mode == "dry_run" {
		modeLabel = "dry_run"
	}
	autoCount := 0
	for _, r := range report.Results {
		if r.Executed {
			autoCount++
		}
	}

	// Only log if there's something to report (avoid spamming every 60s).
	if report.AutoFixed > 0 || report.Errors > 0 || report.Proposed > 0 {
		log.Printf("healer: cycle complete mode=%s findings=%d auto=%d executed=%d proposed=%d errors=%d",
			modeLabel, len(findings), report.AutoFixed, autoCount, report.Proposed, report.Errors)

		// Log each executed action as a structured audit record.
		for _, r := range report.Results {
			if r.Executed || r.Error != "" {
				b, _ := json.Marshal(map[string]interface{}{
					"ts":          r.Timestamp.Format(time.RFC3339),
					"invariant":   r.InvariantID,
					"entity":      r.EntityRef,
					"disposition": string(r.Disposition),
					"executed":    r.Executed,
					"verified":    r.Verified,
					"error":       r.Error,
				})
				log.Printf("healer: audit %s", string(b))
			}
		}
	}
}
