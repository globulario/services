package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

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
				if !s.isAuthoritative.Load() {
					continue // only leader runs
				}
				s.runHealerCycle(ctx, mode, maxActions)
			}
		}
	}()
}

func (s *ClusterDoctorServer) runHealerCycle(ctx context.Context, mode string, maxActions int) {
	// Take a fresh snapshot.
	snap, _, err := s.takeSnapshot(ctx, cluster_doctorpb.FreshnessMode_FRESHNESS_FRESH)
	if err != nil && snap == nil {
		log.Printf("healer: cycle skipped — snapshot failed: %v", err)
		return
	}

	// Evaluate invariants.
	findings := s.registry.EvaluateAll(snap)

	// Determine healer mode.
	dryRun := mode != "enforce"
	healer := &rules.Healer{
		DryRun:      dryRun,
		Remote:      s.healerRemoteOps(),
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
