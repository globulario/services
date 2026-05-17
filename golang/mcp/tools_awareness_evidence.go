package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/awareness/bundlesync"
	"github.com/globulario/awareness/evidence"
)

// registerAwarenessEvidenceTools registers the runtime error intelligence MCP tools.
// These tools expose local node evidence collection, error normalization, Day-1 classification,
// and bundle status — all read-only.
//
// Core pipeline: collect → normalize → classify → expose.
func registerAwarenessEvidenceTools(s *server, st *awarenessState) {
	registerBundleStatusTool(s, st)
	registerRuntimeErrorsTool(s, st)
	registerNormalizeErrorsTool(s, st)
	registerDay1ClassifyNodeTool(s, st)
	registerExplainBlockerTool(s, st)
}

// ── awareness.bundle_status ──────────────────────────────────────────────────

func registerBundleStatusTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.bundle_status",
		Description: "Return the locally installed awareness bundle status on this node. " +
			"Reports presence, version/build_id, MCP load source, AND the bundlesync " +
			"freshness verdict (AWARENESS_READY / MISSING / STALE / MISMATCH / " +
			"SCHEMA_UNSUPPORTED / VERIFY_FAILED) by comparing the active manifest " +
			"against /var/lib/globular/release-index.json. Read-only.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		coll := evidence.NewCollector("", "", evidence.PhaseDAY1)
		bundleStatus := coll.Collect(ctx).AwarenessBundle

		graphSource := "unknown"
		if st.g != nil {
			// Determine which path MCP is actually loading from.
			const bundlePath = "/var/lib/globular/awareness/current/graph.json"
			const systemPath = "/var/lib/globular/awareness/graph.json"
			switch {
			case fileExists(bundlePath):
				graphSource = "bundle (" + bundlePath + ")"
			case fileExists(systemPath):
				graphSource = "system (" + systemPath + ")"
			default:
				graphSource = "dev-fallback"
			}
		} else {
			graphSource = "unavailable"
		}

		// Compose the bundlesync freshness verdict so callers can read one
		// tool to learn both "what's on disk" and "is it the right thing."
		// Best-effort: missing release-index or manifest yields a clean
		// state without erroring the whole tool.
		fresh := computeBundleFreshness()

		return map[string]interface{}{
			"bundle": map[string]interface{}{
				"present":  bundleStatus.Present,
				"version":  bundleStatus.Version,
				"build_id": bundleStatus.BuildID,
				"status":   bundleStatus.Status,
			},
			"graph_source": graphSource,
			"graph_loaded": st.g != nil,
			"docs_dir":     st.docsDir,
			"freshness":    fresh,
		}, nil
	})
}

// computeBundleFreshness reads the active manifest + release-index and runs
// bundlesync.CheckAwarenessFreshness, returning a compact verdict for the
// awareness.bundle_status response. Never errors — every failure mode maps to
// a state the caller can act on.
func computeBundleFreshness() map[string]interface{} {
	out := map[string]interface{}{
		"state":              string(bundlesync.StateAwarenessBundleMissing),
		"release_index_path": releaseIndexPath,
		"manifest_path":      filepath.Join(activeBundleDir, activeManifestFile),
	}

	ri, riErr := loadReleaseIndex(releaseIndexPath)
	if riErr != nil {
		out["state"] = string(bundlesync.StateAwarenessBundleVerifyFailed)
		out["reason"] = riErr.Error()
		return out
	}
	out["release_index"] = ri

	manifestPath := filepath.Join(activeBundleDir, activeManifestFile)
	if _, statErr := os.Stat(manifestPath); os.IsNotExist(statErr) {
		out["state"] = string(bundlesync.StateAwarenessBundleMissing)
		out["reason"] = "no manifest installed"
		return out
	}
	m, err := bundlesync.LoadManifest(manifestPath)
	if err != nil {
		out["state"] = string(bundlesync.StateAwarenessBundleVerifyFailed)
		out["reason"] = err.Error()
		return out
	}

	report := bundlesync.CheckAwarenessFreshness(m, ri, nil)
	out["state"] = string(report.State)
	out["reason"] = report.Reason
	out["ok"] = report.OK
	out["version_matches_release"] = report.VersionMatchesRelease
	out["build_id_matches_release"] = report.BuildIDMatchesRelease
	out["schema_supported"] = report.SchemaSupported
	return out
}

// ── awareness.runtime_errors ─────────────────────────────────────────────────

func registerRuntimeErrorsTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.runtime_errors",
		Description: "Return recent normalized runtime error facts from the local node. " +
			"Facts are structured observations (SERVICE_FAILED, SCYLLA_CQL_UNREACHABLE, etc.) " +
			"derived from systemd state and port checks. Read-only. " +
			"Returns at most the facts collected in the last snapshot cycle.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"severity": {
					Type:        "string",
					Description: "Filter by minimum severity: CRITICAL, HIGH, MEDIUM, LOW. Default: all.",
					Default:     "",
				},
				"kind": {
					Type:        "string",
					Description: "Filter by fact kind (e.g. SCYLLA_CQL_UNREACHABLE). Default: all.",
					Default:     "",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		// Load from disk first (most recent persisted snapshot).
		snap, err := evidence.LoadLatestSnapshot()
		if err != nil || snap == nil {
			// Fall back to a fresh collection.
			coll := evidence.NewCollector("", "", evidence.PhaseDAY1)
			snap = coll.Collect(ctx)
			norm := &evidence.Normalizer{}
			snap.Facts = norm.Normalize(snap)
		}

		filterSev := strArg(args, "severity")
		filterKind := strArg(args, "kind")

		var out []map[string]interface{}
		for _, f := range snap.Facts {
			if filterSev != "" && string(f.Severity) != filterSev {
				continue
			}
			if filterKind != "" && string(f.Kind) != filterKind {
				continue
			}
			out = append(out, factToMap(f))
		}

		return map[string]interface{}{
			"node_id":      snap.NodeID,
			"collected_at": snap.CollectedAt.Format(time.RFC3339),
			"phase":        string(snap.Phase),
			"fact_count":   len(out),
			"facts":        out,
		}, nil
	})
}

// ── awareness.normalize_errors ───────────────────────────────────────────────

func registerNormalizeErrorsTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.normalize_errors",
		Description: "Collect fresh local evidence and normalize it into structured facts. " +
			"This triggers a live collection from systemd and port checks, then runs the " +
			"normalizer to produce RuntimeFact structs. Returns the full normalized snapshot. " +
			"Optionally saves the result to /var/lib/globular/awareness/runtime/. Read-only.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"save": {
					Type:        "boolean",
					Description: "Save the snapshot to disk after collection (default false).",
					Default:     false,
				},
				"phase": {
					Type:        "string",
					Description: "Phase context: DAY0 or DAY1 (default: DAY1).",
					Default:     "DAY1",
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		phase := evidence.PhaseDAY1
		if strArg(args, "phase") == "DAY0" {
			phase = evidence.PhaseDAY0
		}

		coll := evidence.NewCollector(st.nodeID, "", phase)
		snap := coll.Collect(ctx)
		norm := &evidence.Normalizer{}
		snap.Facts = norm.Normalize(snap)

		if getBool(args, "save", false) {
			if err := evidence.SaveSnapshot(snap); err != nil {
				snap.RawEvidenceRefs = append(snap.RawEvidenceRefs,
					"warning: save failed: "+err.Error())
			}
		}

		return nodeSnapshotToMap(snap), nil
	})
}

// ── awareness.day1_classify_node ─────────────────────────────────────────────

func registerDay1ClassifyNodeTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.day1_classify_node",
		Description: "Classify the local node's Day-1 readiness using live runtime evidence. " +
			"Collects a fresh local snapshot, normalizes errors into facts, and runs the " +
			"Day-1 classifier to produce a verdict with readiness levels, classification, " +
			"primary blocker, and allowed/forbidden actions. " +
			"Use this before marking a node ready or dispatching workloads. Read-only.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"save": {
					Type:        "boolean",
					Description: "Save the snapshot to disk (default false).",
					Default:     false,
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		coll := evidence.NewCollector(st.nodeID, "", evidence.PhaseDAY1)
		snap := coll.Collect(ctx)
		norm := &evidence.Normalizer{}
		snap.Facts = norm.Normalize(snap)

		if getBool(args, "save", false) {
			if err := evidence.SaveSnapshot(snap); err != nil {
				snap.RawEvidenceRefs = append(snap.RawEvidenceRefs,
					"warning: save failed: "+err.Error())
			}
		}

		classifier := &evidence.Classifier{}
		verdict := classifier.Classify(snap)

		readiness := make(map[string]bool, len(evidence.Day1ReadinessLadder))
		for _, level := range evidence.Day1ReadinessLadder {
			readiness[string(level)] = verdict.Readiness[level]
		}

		return map[string]interface{}{
			"node_id":           verdict.NodeID,
			"phase":             string(verdict.Phase),
			"verdict":           verdict.Verdict,
			"classification":    string(verdict.Classification),
			"primary_blocker":   verdict.PrimaryBlocker,
			"highest_level":     string(verdict.HighestReachedLevel()),
			"readiness":         readiness,
			"blocked_services":  verdict.BlockedServices,
			"allowed_actions":   verdict.AllowedActions,
			"forbidden_actions": verdict.ForbiddenActions,
			"fact_count":        len(verdict.Evidence),
			"critical_facts":    criticalFactsFromSlice(verdict.Evidence),
			"bundle_status": map[string]interface{}{
				"present":  snap.AwarenessBundle.Present,
				"version":  snap.AwarenessBundle.Version,
				"build_id": snap.AwarenessBundle.BuildID,
				"status":   snap.AwarenessBundle.Status,
			},
		}, nil
	})
}

// ── awareness.explain_blocker ────────────────────────────────────────────────

func registerExplainBlockerTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name: "awareness.explain_blocker",
		Description: "Explain what is blocking Day-1 completion on the local node. " +
			"Returns the primary blocker, the classification, what contracts are violated, " +
			"and the safe next actions. Combines runtime evidence with awareness bundle " +
			"contracts (if the graph is loaded). Read-only.",
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		coll := evidence.NewCollector(st.nodeID, "", evidence.PhaseDAY1)
		snap := coll.Collect(ctx)
		norm := &evidence.Normalizer{}
		snap.Facts = norm.Normalize(snap)

		classifier := &evidence.Classifier{}
		verdict := classifier.Classify(snap)

		if verdict.Verdict == "PASS" {
			return map[string]interface{}{
				"verdict":        "PASS",
				"classification": "HEALTHY",
				"message":        "No blockers detected. Node appears Day-1 complete.",
			}, nil
		}

		explanation := fmt.Sprintf(
			"Node is blocked from Day-1 completion.\n\n"+
				"Classification: %s\n"+
				"Primary blocker: %s\n\n"+
				"Highest readiness level reached: %s\n\n"+
				"Critical facts:\n",
			verdict.Classification, verdict.PrimaryBlocker,
			verdict.HighestReachedLevel(),
		)
		for _, f := range verdict.Evidence {
			if f.Severity == evidence.SeverityCritical || f.Severity == evidence.SeverityHigh {
				explanation += fmt.Sprintf("  - [%s] %s: %s\n",
					f.Severity, f.Kind, f.Detail)
			}
		}

		return map[string]interface{}{
			"verdict":           verdict.Verdict,
			"classification":    string(verdict.Classification),
			"primary_blocker":   verdict.PrimaryBlocker,
			"explanation":       explanation,
			"allowed_actions":   verdict.AllowedActions,
			"forbidden_actions": verdict.ForbiddenActions,
			"blocked_services":  verdict.BlockedServices,
			"highest_level":     string(verdict.HighestReachedLevel()),
		}, nil
	})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func factToMap(f evidence.RuntimeFact) map[string]interface{} {
	m := map[string]interface{}{
		"kind":        string(f.Kind),
		"node_id":     f.NodeID,
		"phase":       string(f.Phase),
		"severity":    string(f.Severity),
		"confidence":  f.Confidence,
		"timestamp":   f.Timestamp.Format(time.RFC3339),
	}
	if f.Service != "" {
		m["service"] = f.Service
	}
	if f.Port != 0 {
		m["port"] = f.Port
	}
	if len(f.Blocks) > 0 {
		m["blocks"] = f.Blocks
	}
	if f.EvidenceRef != "" {
		m["evidence_ref"] = f.EvidenceRef
	}
	if f.Detail != "" {
		m["detail"] = f.Detail
	}
	return m
}

func nodeSnapshotToMap(snap *evidence.NodeRuntimeSnapshot) map[string]interface{} {
	services := make([]map[string]interface{}, 0, len(snap.Services))
	for _, svc := range snap.Services {
		services = append(services, map[string]interface{}{
			"name":         svc.Name,
			"unit_name":    svc.UnitName,
			"active_state": svc.ActiveState,
			"sub_state":    svc.SubState,
			"exit_code":    svc.ExitCode,
		})
	}

	ports := make([]map[string]interface{}, 0, len(snap.Ports))
	for _, p := range snap.Ports {
		ports = append(ports, map[string]interface{}{
			"port":      p.Port,
			"protocol":  p.Protocol,
			"listening": p.Listening,
		})
	}

	facts := make([]map[string]interface{}, 0, len(snap.Facts))
	for _, f := range snap.Facts {
		facts = append(facts, factToMap(f))
	}

	return map[string]interface{}{
		"node_id":      snap.NodeID,
		"address":      snap.Address,
		"phase":        string(snap.Phase),
		"collected_at": snap.CollectedAt.Format(time.RFC3339),
		"release": map[string]interface{}{
			"version":  snap.Release.Version,
			"build_id": snap.Release.BuildID,
		},
		"awareness_bundle": map[string]interface{}{
			"present":  snap.AwarenessBundle.Present,
			"version":  snap.AwarenessBundle.Version,
			"build_id": snap.AwarenessBundle.BuildID,
			"status":   snap.AwarenessBundle.Status,
		},
		"services":          services,
		"ports":             ports,
		"facts":             facts,
		"raw_evidence_refs": snap.RawEvidenceRefs,
	}
}

func criticalFactsFromSlice(facts []evidence.RuntimeFact) []map[string]interface{} {
	var out []map[string]interface{}
	for _, f := range facts {
		if f.Severity == evidence.SeverityCritical || f.Severity == evidence.SeverityHigh {
			out = append(out, factToMap(f))
		}
	}
	return out
}

// fileExists is a helper; note dirExists is already defined in tools_awareness.go.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
