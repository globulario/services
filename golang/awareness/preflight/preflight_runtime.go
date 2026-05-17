package preflight

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/runtime"
)

// mergeRuntime collects a runtime snapshot and merges its findings into the report.
func mergeRuntime(ctx context.Context, opts Options, g *graph.Graph, r *Report) *Report {
	bridge := opts.Bridge
	if bridge == nil {
		bridge = runtime.NewBridge("", "")
	}
	window := opts.RuntimeWindow
	if window <= 0 {
		window = 15 * time.Minute
	}

	snap, err := bridge.Snapshot(ctx, window, g)
	if err != nil {
		r.Warnings = append(r.Warnings, "runtime snapshot failed: "+err.Error())
		return r
	}

	// Build compact runtime section.
	rs := &RuntimeSection{
		Included:            true,
		CapturedAt:          snap.CapturedAt.Format(time.RFC3339),
		MatchedInvariants:   snap.MatchedInvariants,
		MatchedFailureModes: snap.MatchedFailureModes,
		MetricWarnings:      metricWarnings(snap.Warnings),
		Warnings:            snap.Warnings,
		DoctorFindings:      make([]DoctorFindingSummary, 0, len(snap.DoctorFindings)),
		ServiceStatuses:     make([]ServiceStatusSummary, 0, len(snap.RuntimeServices)),
		WorkflowReceipts:    make([]WorkflowReceiptSummary, 0, len(snap.WorkflowReceipts)),
		StateDeltas:         make([]StateDeltaSummary, 0, len(snap.StateDelta)),
	}

	for _, f := range snap.DoctorFindings {
		if !f.Suppressed {
			rs.DoctorFindings = append(rs.DoctorFindings, DoctorFindingSummary{
				ID:       f.FindingID,
				Severity: f.Severity,
				Title:    f.Title,
			})
		}
	}
	for _, svc := range snap.RuntimeServices {
		rs.ServiceStatuses = append(rs.ServiceStatuses, ServiceStatusSummary{
			ServiceID: svc.ServiceID,
			State:     svc.State,
			NodeID:    svc.NodeID,
		})
	}
	for _, wf := range snap.WorkflowReceipts {
		rs.WorkflowReceipts = append(rs.WorkflowReceipts, WorkflowReceiptSummary{
			WorkflowType: wf.WorkflowType,
			Status:       wf.Status,
			ErrorMsg:     wf.ErrorMsg,
		})
	}
	for _, d := range snap.StateDelta {
		rs.StateDeltas = append(rs.StateDeltas, StateDeltaSummary{
			ServiceID: d.ServiceID,
			DeltaType: d.DeltaType,
			Desired:   d.DesiredVersion,
			Installed: d.InstalledVersion,
		})
	}

	// Attach live workflow runtime section from graph overlay.
	rs.WorkflowRuntime = buildWorkflowRuntimeSection(ctx, g)
	r.Runtime = rs

	// Merge matched invariants and failure modes (deduplicate).
	r.Invariants = unique(append(r.Invariants, snap.MatchedInvariants...))
	r.FailureModes = unique(append(r.FailureModes, snap.MatchedFailureModes...))
	r.Warnings = append(r.Warnings, snap.Warnings...)

	// Propagate workflow runtime stale/failed as a blind spot and lower confidence.
	if rs.WorkflowRuntime != nil {
		switch rs.WorkflowRuntime.Coverage {
		case "stale":
			r.BlindSpots = append(r.BlindSpots, "workflow_runtime_stale: live workflow overlay is expired — rebuild with --collect-workflow")
		case "failed":
			r.BlindSpots = append(r.BlindSpots, "workflow_runtime_failed: live workflow overlay collection failed — source may be unreachable")
		case "not_checked", "disabled":
			r.BlindSpots = append(r.BlindSpots, "workflow_runtime_not_checked: no live workflow overlay in graph — run awareness build --collect-workflow to enable")
		}
	}

	// Adjust classification based on runtime evidence.
	if len(snap.StateDelta) > 0 {
		r.Classification = appendClass(r.Classification, ClassStateMismatch)
		r.Classification = appendClass(r.Classification, ClassConvergenceRisk)
	}

	// Runtime warnings can promote static preflight into dynamic risk.
	for _, w := range snap.Warnings {
		lw := strings.ToLower(w)
		if strings.Contains(lw, "start-limit-hit") {
			r.Classification = appendClass(r.Classification, ClassRestartStorm)
		}
		if strings.Contains(lw, "metric saturation") || strings.Contains(lw, "metric error signal") {
			r.Classification = appendClass(r.Classification, ClassRuntimeIncident)
			r.Classification = appendClass(r.Classification, ClassConvergenceRisk)
		}
	}

	// Critical doctor findings → ClassArchitectureSensitive.
	for _, f := range snap.DoctorFindings {
		if !f.Suppressed && f.Severity == "critical" {
			r.Classification = appendClass(r.Classification, ClassArchitectureSensitive)
			break
		}
	}

	// Repository non-NORMAL mode → ClassArchitectureSensitive.
	for _, rs2 := range snap.RepositoryStatus {
		if rs2.Mode != "NORMAL" && rs2.Mode != "" {
			r.Classification = appendClass(r.Classification, ClassArchitectureSensitive)
			break
		}
	}

	return r
}

// buildWorkflowRuntimeSection reads workflow_run overlay nodes from the graph
// and summarises their freshness and coverage for the preflight report.
// Returns nil when the graph is nil or no workflow nodes exist.
func buildWorkflowRuntimeSection(ctx context.Context, g *graph.Graph) *WorkflowRuntimeSection {
	if g == nil {
		return nil
	}
	runs, err := g.FindNodesByType(ctx, graph.NodeTypeWorkflowRun)
	if err != nil || len(runs) == 0 {
		return &WorkflowRuntimeSection{
			Coverage:        "not_checked",
			Freshness:       "unknown",
			Source:          "none",
			CollectorStatus: "disabled",
		}
	}

	ws := &WorkflowRuntimeSection{
		Source:          "graph_cache",
		CollectorStatus: "ok",
	}
	now := time.Now()
	stale := false
	for _, n := range runs {
		ws.RunsSeen++
		if status, ok := n.Metadata["status"].(string); ok {
			if status == "failed" {
				ws.FailedRuns++
			}
			if status == "blocked" {
				ws.BlockedRuns++
			}
		}
		// Check TTL freshness.
		if expiresStr, ok := n.Metadata["expires_at"].(string); ok {
			exp, parseErr := time.Parse(time.RFC3339, expiresStr)
			if parseErr == nil && now.After(exp) {
				stale = true
			}
		}
		if collectedAt, ok := n.Metadata["collected_at"].(string); ok && ws.CollectedAt == "" {
			ws.CollectedAt = collectedAt
		}
		if ttl, ok := n.Metadata["ttl_seconds"].(int); ok && ws.TTLSeconds == 0 {
			ws.TTLSeconds = ttl
		}
	}

	if stale {
		ws.Freshness = "stale"
		ws.Coverage = "stale"
	} else {
		ws.Freshness = "fresh"
		if ws.FailedRuns > 0 || ws.BlockedRuns > 0 {
			ws.Coverage = "checked_with_matches"
		} else {
			ws.Coverage = "checked_clean"
		}
	}

	return ws
}

// ComputeLiveOverlayFreshness checks when the last live-snapshot was run
// and returns a freshness report. Returns status "absent" if never run.
// Exported so tests in other packages can call it directly.
func ComputeLiveOverlayFreshness(ctx context.Context, g *graph.Graph, now time.Time) *LiveOverlayFreshness {
	if now.IsZero() {
		now = time.Now()
	}
	rec, err := g.LatestLiveSnapshotRecord(ctx)
	if err != nil || rec == nil {
		return &LiveOverlayFreshness{Status: "absent"}
	}

	age := now.Unix() - rec.CreatedAt
	ageSeconds := float64(age)

	status := "fresh"
	if ageSeconds > float64(LiveOverlayStaleSeconds) {
		status = "absent"
	} else if ageSeconds > float64(LiveOverlayTTLSeconds) {
		status = "stale"
	}

	// Derive status from collector health if any collectors failed.
	okCount, failCount := 0, 0
	var collectors []CollectorHealthSummary
	for _, ch := range rec.CollectorHealth {
		c := CollectorHealthSummary{
			CollectorID:  ch.CollectorID,
			Status:       ch.Status,
			NodesEmitted: ch.NodesEmitted,
			Error:        ch.Error,
		}
		collectors = append(collectors, c)
		if ch.Status == "error" || ch.Status == "failed" {
			failCount++
		} else {
			okCount++
		}
	}
	if status == "fresh" && failCount > 0 && okCount == 0 {
		status = "failed"
	} else if status == "fresh" && failCount > 0 {
		status = "partial"
	}

	collectedAt := time.Unix(rec.CreatedAt, 0).UTC().Format(time.RFC3339)
	return &LiveOverlayFreshness{
		Status:      status,
		AgeSeconds:  ageSeconds,
		CollectedAt: collectedAt,
		Collectors:  collectors,
	}
}

// computeGoFileCoverage walks repoRoot to count eligible Go files and compares
// them against source_file nodes indexed in g. Duplicates the core walk from
// enforce.GoFileCoverage to avoid the circular import (enforce → preflight).
//
// Mirrors enforce.GoFileCoverage's empty-repoRoot guard: when no source is
// available to walk, returns with ConfidenceImpact="unknown" and a blind
// spot — never zero counts that downstream consumers can collapse into a
// "0% coverage critical" finding. See
// awareness.source_scan_requires_verified_repo_root.
func computeGoFileCoverage(ctx context.Context, g *graph.Graph, repoRoot string) *GoFileCoverageReport {
	res := &GoFileCoverageReport{}

	if repoRoot == "" {
		res.ConfidenceImpact = "unknown"
		res.BlindSpots = []string{"repo root not provided — cannot measure Go file coverage"}
		return res
	}

	excludedDirs := map[string]bool{
		"vendor": true, ".git": true, "node_modules": true,
		"dist": true, "build": true, ".cache": true,
	}
	isExcluded := func(rel string) bool {
		parts := strings.SplitN(rel, string(os.PathSeparator), 2)
		return excludedDirs[parts[0]]
	}
	isGeneratedProto := func(rel string) bool {
		return strings.HasSuffix(rel, ".pb.go") || strings.HasSuffix(rel, ".pb.gw.go")
	}

	eligibleSet := map[string]bool{}
	_ = filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(repoRoot, path)
		if info.IsDir() {
			if isExcluded(rel) {
				return filepath.SkipDir
			}
			return nil
		}
		if isExcluded(rel) || !strings.HasSuffix(rel, ".go") || isGeneratedProto(rel) {
			return nil
		}
		eligibleSet[rel] = true
		res.EligibleGoFilesTotal++
		if !strings.HasSuffix(rel, "_test.go") {
			res.EligibleNonTestGoFiles++
		}
		return nil
	})

	if g == nil {
		res.ConfidenceImpact = "low"
		res.BlindSpots = []string{fmt.Sprintf("%d eligible Go files cannot be checked — graph not loaded", res.EligibleGoFilesTotal)}
		return res
	}

	nodes, err := g.FindNodesByType(ctx, graph.NodeTypeSourceFile)
	if err != nil {
		res.ConfidenceImpact = "low"
		res.BlindSpots = []string{"graph source_file query failed: " + err.Error()}
		return res
	}

	indexedSet := map[string]bool{}
	for _, n := range nodes {
		if n.Path == "" {
			continue
		}
		p := filepath.ToSlash(n.Path)
		indexedSet[p] = true
		if strings.HasSuffix(p, ".go") {
			res.IndexedGoFilesTotal++
			if !strings.HasSuffix(p, "_test.go") {
				res.IndexedNonTestGoFiles++
			}
		}
	}

	for rel := range eligibleSet {
		if !indexedSet[filepath.ToSlash(rel)] {
			res.MissingFiles = append(res.MissingFiles, rel)
		}
	}

	if res.EligibleGoFilesTotal > 0 {
		res.CoveragePercentGoFiles = float64(res.IndexedGoFilesTotal) / float64(res.EligibleGoFilesTotal) * 100
	}
	if res.EligibleNonTestGoFiles > 0 {
		// stored in struct but not used in confidence path here; kept for completeness
		_ = float64(res.IndexedNonTestGoFiles) / float64(res.EligibleNonTestGoFiles) * 100
	}

	missing := len(res.MissingFiles)
	switch {
	case res.CoveragePercentGoFiles < 70.0:
		res.ConfidenceImpact = "high"
		res.BlindSpots = append(res.BlindSpots,
			fmt.Sprintf("%d eligible Go files are not represented in the graph (coverage %.1f%% < 70%%)", missing, res.CoveragePercentGoFiles))
	case res.CoveragePercentGoFiles < 85.0:
		res.ConfidenceImpact = "medium"
		res.BlindSpots = append(res.BlindSpots,
			fmt.Sprintf("%d eligible Go files are not represented in the graph (coverage %.1f%% < 85%%)", missing, res.CoveragePercentGoFiles))
	default:
		res.ConfidenceImpact = "none"
	}
	return res
}

func metricWarnings(warnings []string) []string {
	var out []string
	for _, w := range warnings {
		lw := strings.ToLower(w)
		if strings.Contains(lw, "metric ") || strings.Contains(lw, "saturation") {
			out = append(out, w)
		}
	}
	return out
}
