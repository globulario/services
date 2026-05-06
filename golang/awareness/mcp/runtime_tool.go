package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/globulario/services/golang/awareness/runtime"
)

func registerRuntimeTool(s *Server) {
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

		bridge := runtime.NewBridge(s.cfg.NodeID, "")
		snap, err := bridge.Snapshot(ctx, window, s.g)
		if err != nil {
			return nil, fmt.Errorf("runtime snapshot: %w", err)
		}

		// Optionally write evidence to the graph (still read-only w.r.t. runtime state).
		if boolArg(args, "write_graph") && s.g != nil {
			if writeErr := bridge.WriteToGraph(ctx, snap, s.g); writeErr != nil {
				snap.Warnings = append(snap.Warnings, "write-graph: "+writeErr.Error())
			}
		}

		return snapshotToMap(snap), nil
	})
}

// snapshotToMap converts a RuntimeSnapshot to a JSON-serializable map.
func snapshotToMap(snap *runtime.RuntimeSnapshot) map[string]interface{} {
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

	return map[string]interface{}{
		"id":                   snap.ID,
		"captured_at":          snap.CapturedAt.Format(time.RFC3339),
		"node_id":              snap.NodeID,
		"cluster_id":           snap.ClusterID,
		"doctor_findings":      findings,
		"service_statuses":     services,
		"workflow_receipts":    workflows,
		"state_deltas":         deltas,
		"matched_invariants":   snap.MatchedInvariants,
		"matched_failure_modes": snap.MatchedFailureModes,
		"warnings":             snap.Warnings,
	}
}
