package mcp

import (
	"context"
	"fmt"
	"path/filepath"
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

		bridge := buildBridge(s)
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

// buildBridge constructs a RuntimeBridge with real gRPC sources where configured.
func buildBridge(s *Server) *runtime.RuntimeBridge {
	b := runtime.NewBridge(s.cfg.NodeID, "")

	if s.cfg.DoctorAddr != "" {
		if src, err := runtime.NewGrpcDoctorSource(s.cfg.DoctorAddr); err == nil {
			b.Doctor = src
		}
	}
	if s.cfg.ControllerAddr != "" {
		if src, err := runtime.NewGrpcStateSource(s.cfg.ControllerAddr); err == nil {
			b.State = src
		}
		if src, err := runtime.NewGrpcServiceStatusSource(s.cfg.ControllerAddr); err == nil {
			b.Services = src
		}
	}
	if s.cfg.WorkflowAddr != "" {
		if src, err := runtime.NewGrpcWorkflowSource(s.cfg.WorkflowAddr); err == nil {
			b.Workflows = src
		}
	}
	if s.cfg.PrometheusAddr != "" {
		queriesFile := ""
		if s.cfg.DocsDir != "" {
			queriesFile = filepath.Join(s.cfg.DocsDir, "knowledge", "metric_queries.yaml")
		}
		if src, err := runtime.NewPrometheusMetricsSource(s.cfg.PrometheusAddr, queriesFile); err == nil {
			b.Metrics = src
		}
	}
	return b
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

	metrics := make([]map[string]interface{}, 0, len(snap.Metrics))
	for _, m := range snap.Metrics {
		metrics = append(metrics, map[string]interface{}{
			"name": m.Name, "node_id": m.NodeID, "service_id": m.ServiceID,
			"value": m.Value, "unit": m.Unit, "labels": m.Labels,
		})
	}

	sourceHealth := make([]map[string]interface{}, 0, len(snap.SourceHealth))
	for _, sh := range snap.SourceHealth {
		sourceHealth = append(sourceHealth, map[string]interface{}{
			"source":            sh.Source,
			"backend":           sh.Backend,
			"healthy":           sh.Healthy,
			"empty_due_to_noop": sh.EmptyDueToNoop,
			"last_error":        sh.LastError,
			"collected_at":      sh.CollectedAt,
		})
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
		"metrics":               metrics,
		"matched_invariants":    snap.MatchedInvariants,
		"matched_failure_modes": snap.MatchedFailureModes,
		"warnings":              snap.Warnings,
		"source_health":         sourceHealth,
	}
}
