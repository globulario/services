package main

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/awareness/runtime"
)

func registerAwarenessRuntimeTools(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.runtime_snapshot",
		Description: "Collect a read-only runtime snapshot from the local Globular node. Returns current doctor findings, service statuses, desired/installed state deltas, and matched invariants. Never mutates runtime state.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"window": {
					Type:        "string",
					Description: "Lookback window for events and workflow receipts (e.g. 15m, 1h). Defaults to 15m.",
					Default:     "15m",
				},
				"write_graph": {
					Type:        "boolean",
					Description: "Write evidence nodes to the awareness graph (default false). Only evidence nodes are written — no runtime state is mutated.",
					Default:     false,
				},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		window := 15 * time.Minute
		if ws := strArg(args, "window"); ws != "" {
			if d, err := time.ParseDuration(ws); err == nil {
				window = d
			}
		}

		bridge := newLiveBridge(st)
		snap, err := bridge.Snapshot(ctx, window, st.g)
		if err != nil {
			return nil, fmt.Errorf("runtime snapshot: %w", err)
		}

		// Optionally write evidence to the graph (still read-only w.r.t. runtime state).
		if getBool(args, "write_graph", false) && st.g != nil {
			if writeErr := bridge.WriteToGraph(ctx, snap, st.g); writeErr != nil {
				snap.Warnings = append(snap.Warnings, "write-graph: "+writeErr.Error())
			}
		}

		return awarenessSnapshotToMap(snap), nil
	})
}

// awarenessSnapshotToMap converts a RuntimeSnapshot to a JSON-serializable map.
// source_health, confidence, coverage, and blind_spots are always present so
// callers can detect noop snapshots without guessing from empty result arrays.
func awarenessSnapshotToMap(snap *runtime.RuntimeSnapshot) map[string]interface{} {
	findings := make([]map[string]interface{}, 0, len(snap.DoctorFindings))
	for _, f := range snap.DoctorFindings {
		findings = append(findings, map[string]interface{}{
			"id": f.FindingID, "severity": f.Severity, "title": f.Title,
			"invariant_ref": f.InvariantRef, "suppressed": f.Suppressed,
		})
	}

	services := make([]map[string]interface{}, 0, len(snap.RuntimeServices))
	for _, svc := range snap.RuntimeServices {
		services = append(services, map[string]interface{}{
			"service_id": svc.ServiceID, "node_id": svc.NodeID,
			"state": svc.State, "version": svc.Version,
			"restart_count": svc.RestartCount,
		})
	}

	workflows := make([]map[string]interface{}, 0, len(snap.WorkflowReceipts))
	for _, w := range snap.WorkflowReceipts {
		entry := map[string]interface{}{
			"workflow_type": w.WorkflowType, "status": w.Status,
			"service_id": w.ServiceID,
		}
		if w.ErrorMsg != "" {
			entry["error_msg"] = w.ErrorMsg
		}
		workflows = append(workflows, entry)
	}

	deltas := make([]map[string]interface{}, 0, len(snap.StateDelta))
	for _, d := range snap.StateDelta {
		deltas = append(deltas, map[string]interface{}{
			"service_id": d.ServiceID, "node_id": d.NodeID,
			"delta_type":        d.DeltaType,
			"desired_version":   d.DesiredVersion,
			"installed_version": d.InstalledVersion,
		})
	}

	// Source health: expose every source so callers can distinguish noop from
	// genuinely empty (healthy cluster with no findings).
	sourceHealthList := make([]map[string]interface{}, 0, len(snap.SourceHealth))
	healthyCount, noopCount := 0, 0
	var coverage, blindSpots []string
	for _, sh := range snap.SourceHealth {
		entry := map[string]interface{}{
			"source":            string(sh.Source),
			"backend":           sh.Backend,
			"healthy":           sh.Healthy,
			"empty_due_to_noop": sh.EmptyDueToNoop,
			"collected_at":      sh.CollectedAt,
		}
		if sh.Transport != "" {
			entry["transport"] = sh.Transport
		}
		if sh.Auth != "" {
			entry["auth"] = sh.Auth
		}
		if sh.LastError != "" {
			entry["last_error"] = sh.LastError
		}
		if len(sh.Warnings) > 0 {
			entry["warnings"] = sh.Warnings
		}
		sourceHealthList = append(sourceHealthList, entry)

		if sh.EmptyDueToNoop {
			noopCount++
			blindSpots = append(blindSpots, string(sh.Source))
		} else if sh.Healthy {
			healthyCount++
			coverage = append(coverage, string(sh.Source))
		}
	}

	total := len(snap.SourceHealth)
	confidence := "unknown"
	switch {
	case total == 0:
		confidence = "unknown"
	case healthyCount == 0 && noopCount == total:
		confidence = "noop"
	case healthyCount == 0:
		confidence = "low"
	case healthyCount < total/2+1:
		confidence = "medium"
	case healthyCount < total:
		confidence = "high"
	default:
		confidence = "full"
	}

	return map[string]interface{}{
		"id":                    snap.ID,
		"captured_at":           snap.CapturedAt.Format(time.RFC3339),
		"node_id":               snap.NodeID,
		"cluster_id":            snap.ClusterID,
		"doctor_findings":       findings,
		"service_statuses":      services,
		"workflow_receipts":     workflows,
		"state_deltas":          deltas,
		"matched_invariants":    snap.MatchedInvariants,
		"matched_failure_modes": snap.MatchedFailureModes,
		"warnings":              snap.Warnings,
		"source_health":         sourceHealthList,
		"confidence":            confidence,
		"coverage":              coverage,
		"blind_spots":           blindSpots,
	}
}
