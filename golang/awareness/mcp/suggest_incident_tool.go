package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/awareness/runtime"
)

func registerSuggestIncidentTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.suggest_incident",
		Description: "Runs a live runtime snapshot, matches it against known critical failure modes, and suggests incident drafts for any unrecognised critical patterns. Does not auto-open incidents — requires operator approval.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"min_severity": {Type: "string", Description: "Minimum severity to consider (default: critical).", Enum: []string{"critical", "high", "medium"}},
				"live":         {Type: "boolean", Description: "Collect a fresh snapshot (default true). Set false to use last stored snapshot."},
				"window":       {Type: "string", Description: "Lookback window for workflow/event data (e.g. 15m, 1h). Default 15m."},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		minSeverity := strArg(args, "min_severity")
		if minSeverity == "" {
			minSeverity = "critical"
		}
		windowStr := strArg(args, "window")
		window := 15 * time.Minute
		if windowStr != "" {
			if d, err := time.ParseDuration(windowStr); err == nil {
				window = d
			}
		}

		// Collect live snapshot.
		bridge := buildBridge(s)
		snap, err := bridge.Snapshot(ctx, window, s.g)
		if err != nil {
			return nil, fmt.Errorf("runtime snapshot: %w", err)
		}

		// Load existing open incidents from proposals dir to avoid duplicates.
		openIncidents := loadOpenIncidentIDs(s.resolvedDocsDir())

		// Find candidate incidents from snapshot evidence.
		candidates := suggestFromSnapshot(snap, minSeverity, openIncidents)

		sourceHealthOut := make([]map[string]interface{}, 0, len(snap.SourceHealth))
		for _, sh := range snap.SourceHealth {
			sourceHealthOut = append(sourceHealthOut, map[string]interface{}{
				"source":  sh.Source,
				"healthy": sh.Healthy,
				"noop":    sh.EmptyDueToNoop,
			})
		}

		noopCount := 0
		for _, sh := range snap.SourceHealth {
			if sh.EmptyDueToNoop {
				noopCount++
			}
		}

		confidence := "high"
		if noopCount > 3 {
			confidence = "low"
		} else if noopCount > 1 {
			confidence = "medium"
		}

		return map[string]interface{}{
			"candidates":        candidates,
			"snapshot_at":       snap.CapturedAt.Format(time.RFC3339),
			"source_health":     sourceHealthOut,
			"confidence":        confidence,
			"confidence_reason": fmt.Sprintf("%d of %d sources were noop", noopCount, len(snap.SourceHealth)),
			"note":              "Candidates require operator review. Use awareness.propose_from_incident to create a formal proposal.",
		}, nil
	})
}

func suggestFromSnapshot(snap *runtime.RuntimeSnapshot, minSeverity string, openIDs map[string]bool) []map[string]interface{} {
	var candidates []map[string]interface{}

	// Critical doctor findings → candidate incidents.
	for _, f := range snap.DoctorFindings {
		if !severityAtLeast(f.Severity, minSeverity) {
			continue
		}
		alreadyOpen := false
		if f.InvariantRef != "" {
			alreadyOpen = openIDs[f.InvariantRef]
		}
		candidates = append(candidates, map[string]interface{}{
			"failure_mode_id":  f.InvariantRef,
			"finding_id":       f.FindingID,
			"severity":         f.Severity,
			"evidence":         f.Title + ": " + f.Description,
			"already_open":     alreadyOpen,
			"suggested_action": suggestAction(alreadyOpen, f.InvariantRef),
		})
	}

	// Matched failure modes from snapshot.
	for _, fmID := range snap.MatchedFailureModes {
		if openIDs[fmID] {
			continue // skip already-open
		}
		candidates = append(candidates, map[string]interface{}{
			"failure_mode_id":  fmID,
			"severity":         "high",
			"evidence":         "matched by runtime snapshot pattern analysis",
			"already_open":     false,
			"suggested_action": "Review failure mode " + fmID + " and propose incident if confirmed.",
		})
	}

	// State deltas → convergence incident candidate.
	if len(snap.StateDelta) > 0 {
		services := make([]string, 0, len(snap.StateDelta))
		for _, d := range snap.StateDelta {
			services = append(services, d.ServiceID+":"+d.DeltaType)
		}
		candidates = append(candidates, map[string]interface{}{
			"failure_mode_id":  "desired.bootstrap_premature_convergence",
			"severity":         "high",
			"evidence":         "state drift detected: " + strings.Join(services, ", "),
			"already_open":     openIDs["desired.bootstrap_premature_convergence"],
			"suggested_action": "Run awareness.propose_from_incident with this drift as evidence.",
		})
	}

	return candidates
}

func suggestAction(alreadyOpen bool, ref string) string {
	if alreadyOpen {
		return "Incident already open for " + ref + " — check proposals dir."
	}
	return "No open incident found. Consider running propose_from_incident."
}

func loadOpenIncidentIDs(docsDir string) map[string]bool {
	out := make(map[string]bool)
	if docsDir == "" {
		return out
	}
	incidentsDir := filepath.Join(docsDir, "incidents")
	entries, err := os.ReadDir(incidentsDir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yaml") {
			continue // fixed: was "return out" (bug), now "continue"
		}
		// Extract invariant/failure-mode IDs from incident file content (keyword scan).
		data, err := os.ReadFile(filepath.Join(incidentsDir, e.Name()))
		if err != nil {
			continue
		}
		// Very lightweight: look for "failure_mode_id:" or "invariant_id:" lines.
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, "failure_mode_id:") || strings.Contains(line, "invariant_id:") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					id := strings.TrimSpace(parts[1])
					if id != "" {
						out[id] = true
					}
				}
			}
		}
	}
	return out
}
