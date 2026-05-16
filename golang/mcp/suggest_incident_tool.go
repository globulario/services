package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/awareness/runtime"
)

func registerSuggestIncidentTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.suggest_incident",
		Description: "Runs a live runtime snapshot (or loads a stored one by ID), matches it against known critical failure modes, and suggests incident drafts for any unrecognised critical patterns. Does not auto-open incidents — requires operator approval.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"snapshot_id":  {Type: "string", Description: "ID of a previously stored snapshot to analyse. If omitted and live=true, a fresh snapshot is collected."},
				"live":         {Type: "boolean", Description: "Collect a fresh snapshot (default true). Set false to use snapshot_id."},
				"min_severity": {Type: "string", Description: "Minimum severity to consider (default: critical).", Enum: []string{"critical", "high", "medium"}},
				"window":       {Type: "string", Description: "Lookback window for workflow/event data (e.g. 15m, 1h). Default 15m."},
				"include_yaml": {Type: "boolean", Description: "Include a YAML incident draft for each candidate (default false)."},
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
		snapshotID := strArg(args, "snapshot_id")
		live := true
		if v, ok := args["live"]; ok {
			if b, ok := v.(bool); ok {
				live = b
			}
		}
		includeYAML := boolArg(args, "include_yaml")

		var snap *runtime.RuntimeSnapshot

		if snapshotID != "" {
			// Load from stored snapshot.
			if st.g == nil {
				return nil, fmt.Errorf("snapshot_id provided but no graph DB available — run 'globular awareness build' first")
			}
			data, err := st.g.GetRuntimeSnapshotByID(ctx, snapshotID)
			if err != nil {
				return nil, fmt.Errorf("load snapshot %s: %w", snapshotID, err)
			}
			if data == nil {
				return nil, fmt.Errorf("snapshot %q not found — use awareness.runtime_snapshot with write_graph=true to store a snapshot first", snapshotID)
			}
			var loaded runtime.RuntimeSnapshot
			if err := json.Unmarshal(data, &loaded); err != nil {
				return nil, fmt.Errorf("parse snapshot %s: %w", snapshotID, err)
			}
			snap = &loaded
		} else if live {
			// Collect live snapshot.
			bridge := newLiveBridge(st)
			var err error
			snap, err = bridge.Snapshot(ctx, window, st.g)
			if err != nil {
				return nil, fmt.Errorf("runtime snapshot: %w", err)
			}
			// Store it so it can be referenced later.
			if st.g != nil {
				_ = bridge.WriteToGraph(ctx, snap, st.g)
			}
		} else {
			return nil, fmt.Errorf("snapshot_id is required when live=false")
		}

		// Load existing open incidents from proposals dir to avoid duplicates.
		openIncidents := loadOpenIncidentIDs(st.docsDir)

		// Find candidate incidents from snapshot evidence.
		candidates := suggestFromSnapshot(snap, minSeverity, openIncidents, includeYAML)

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
		var blindSpots []string
		if noopCount > 3 {
			confidence = "low"
			blindSpots = append(blindSpots, fmt.Sprintf("%d of %d sources are noop — evidence is incomplete", noopCount, len(snap.SourceHealth)))
		} else if noopCount > 1 {
			confidence = "medium"
			blindSpots = append(blindSpots, fmt.Sprintf("%d sources are noop — some evidence may be missing", noopCount))
		}

		return map[string]interface{}{
			"candidate_incidents": candidates,
			"snapshot_id":         snap.ID,
			"snapshot_at":         snap.CapturedAt.Format(time.RFC3339),
			"source_health":       sourceHealthOut,
			"confidence":          confidence,
			"blind_spots":         blindSpots,
			"note":                "Candidates require operator review. Use awareness.propose_from_incident to create a formal proposal.",
		}, nil
	})
}

type incidentCandidate struct {
	FailureModeID   string   `json:"failure_mode_id"`
	FindingID       string   `json:"finding_id,omitempty"`
	Severity        string   `json:"severity"`
	Evidence        []string `json:"evidence"`
	MatchedFMs      []string `json:"matched_failure_modes,omitempty"`
	MatchedInvs     []string `json:"matched_invariants,omitempty"`
	DuplicateOf     string   `json:"duplicate_of,omitempty"`
	AlreadyOpen     bool     `json:"already_open"`
	SuggestedAction string   `json:"suggested_action"`
	YAMLDraft       string   `json:"yaml_draft,omitempty"`
}

func suggestFromSnapshot(snap *runtime.RuntimeSnapshot, minSeverity string, openIDs map[string]bool, includeYAML bool) []incidentCandidate {
	var candidates []incidentCandidate
	seen := make(map[string]bool)

	addCandidate := func(c incidentCandidate) {
		key := c.FailureModeID
		if key == "" {
			key = c.FindingID
		}
		if seen[key] {
			return
		}
		seen[key] = true
		if includeYAML {
			c.YAMLDraft = buildYAMLDraft(c, snap)
		}
		candidates = append(candidates, c)
	}

	// Critical doctor findings → candidate incidents.
	for _, f := range snap.DoctorFindings {
		if !severityAtLeast(f.Severity, minSeverity) {
			continue
		}
		alreadyOpen := false
		duplicateOf := ""
		if f.InvariantRef != "" && openIDs[f.InvariantRef] {
			alreadyOpen = true
			duplicateOf = f.InvariantRef
		}
		var evidence []string
		if f.Title != "" {
			evidence = append(evidence, "doctor finding: "+f.Title)
		}
		if f.Description != "" {
			evidence = append(evidence, "detail: "+f.Description)
		}
		addCandidate(incidentCandidate{
			FailureModeID:   f.InvariantRef,
			FindingID:       f.FindingID,
			Severity:        f.Severity,
			Evidence:        evidence,
			MatchedInvs:     snap.MatchedInvariants,
			DuplicateOf:     duplicateOf,
			AlreadyOpen:     alreadyOpen,
			SuggestedAction: suggestAction(alreadyOpen, f.InvariantRef),
		})
	}

	// Matched failure modes from snapshot.
	for _, fmID := range snap.MatchedFailureModes {
		alreadyOpen := openIDs[fmID]
		duplicateOf := ""
		if alreadyOpen {
			duplicateOf = fmID
		}
		addCandidate(incidentCandidate{
			FailureModeID:   fmID,
			Severity:        "high",
			Evidence:        []string{"matched by runtime snapshot pattern analysis"},
			MatchedFMs:      snap.MatchedFailureModes,
			DuplicateOf:     duplicateOf,
			AlreadyOpen:     alreadyOpen,
			SuggestedAction: suggestAction(alreadyOpen, fmID),
		})
	}

	// State deltas → convergence incident candidate.
	if len(snap.StateDelta) > 0 {
		services := make([]string, 0, len(snap.StateDelta))
		var evidence []string
		for _, d := range snap.StateDelta {
			services = append(services, d.ServiceID+":"+d.DeltaType)
			evidence = append(evidence, fmt.Sprintf("state delta %s: desired=%s installed=%s type=%s",
				d.ServiceID, d.DesiredVersion, d.InstalledVersion, d.DeltaType))
		}
		fmID := "desired.bootstrap_premature_convergence"
		alreadyOpen := openIDs[fmID]
		duplicateOf := ""
		if alreadyOpen {
			duplicateOf = fmID
		}
		addCandidate(incidentCandidate{
			FailureModeID:   fmID,
			Severity:        "high",
			Evidence:        append([]string{"state drift detected: " + strings.Join(services, ", ")}, evidence...),
			MatchedFMs:      snap.MatchedFailureModes,
			MatchedInvs:     snap.MatchedInvariants,
			DuplicateOf:     duplicateOf,
			AlreadyOpen:     alreadyOpen,
			SuggestedAction: "Run awareness.propose_from_incident with this drift as evidence.",
		})
	}

	// Metric warnings → resource saturation candidates.
	for _, w := range snap.Warnings {
		if strings.Contains(w, "metric critical:") || strings.Contains(w, "metric warning:") {
			if !severityAtLeast("high", minSeverity) {
				continue
			}
			fmID := "resource.metric_saturation"
			if !seen[fmID] {
				alreadyOpen := openIDs[fmID]
				addCandidate(incidentCandidate{
					FailureModeID:   fmID,
					Severity:        "high",
					Evidence:        []string{w},
					AlreadyOpen:     alreadyOpen,
					SuggestedAction: "Review metric saturation and propose incident if confirmed.",
				})
			}
		}
	}

	return candidates
}

func buildYAMLDraft(c incidentCandidate, snap *runtime.RuntimeSnapshot) string {
	now := snap.CapturedAt.Format("2006-01-02")
	fmID := c.FailureModeID
	if fmID == "" {
		fmID = c.FindingID
	}
	id := "auto-" + now + "-" + fmID

	var sb strings.Builder
	sb.WriteString("id: \"" + id + "\"\n")
	sb.WriteString("status: OPEN\n")
	sb.WriteString("severity: " + c.Severity + "\n")
	sb.WriteString("headline: \"" + describeCandidate(c) + "\"\n")
	sb.WriteString("failure_mode_id: " + fmID + "\n")
	sb.WriteString("evidence:\n")
	for _, e := range c.Evidence {
		sb.WriteString("  - " + e + "\n")
	}
	return sb.String()
}

func describeCandidate(c incidentCandidate) string {
	if c.FindingID != "" && len(c.Evidence) > 0 {
		return c.Evidence[0]
	}
	if c.FailureModeID != "" {
		return "Failure mode detected: " + c.FailureModeID
	}
	return "Runtime anomaly detected"
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
			continue
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
