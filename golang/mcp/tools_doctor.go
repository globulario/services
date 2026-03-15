package main

import (
	"context"
	"fmt"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// severityStr converts a doctor Severity enum to a human-readable string.
func severityStr(s cluster_doctorpb.Severity) string {
	switch s {
	case cluster_doctorpb.Severity_SEVERITY_INFO:
		return "info"
	case cluster_doctorpb.Severity_SEVERITY_WARN:
		return "warn"
	case cluster_doctorpb.Severity_SEVERITY_ERROR:
		return "error"
	case cluster_doctorpb.Severity_SEVERITY_CRITICAL:
		return "critical"
	default:
		return "unknown"
	}
}

// clusterStatusStr converts a doctor ClusterStatus enum to a human-readable string.
func clusterStatusStr(s cluster_doctorpb.ClusterStatus) string {
	switch s {
	case cluster_doctorpb.ClusterStatus_CLUSTER_HEALTHY:
		return "healthy"
	case cluster_doctorpb.ClusterStatus_CLUSTER_DEGRADED:
		return "degraded"
	case cluster_doctorpb.ClusterStatus_CLUSTER_CRITICAL:
		return "critical"
	default:
		return "unknown"
	}
}

// planRiskStr converts a doctor PlanRisk enum to a human-readable string.
func planRiskStr(r cluster_doctorpb.PlanRisk) string {
	switch r {
	case cluster_doctorpb.PlanRisk_PLAN_RISK_SAFE:
		return "safe"
	case cluster_doctorpb.PlanRisk_PLAN_RISK_MODERATE:
		return "moderate"
	case cluster_doctorpb.PlanRisk_PLAN_RISK_DANGEROUS:
		return "dangerous"
	default:
		return "unknown"
	}
}

// driftCategoryStr converts a DriftCategory enum to a human-readable string.
func driftCategoryStr(c cluster_doctorpb.DriftCategory) string {
	switch c {
	case cluster_doctorpb.DriftCategory_MISSING_UNIT_FILE:
		return "missing_unit_file"
	case cluster_doctorpb.DriftCategory_UNIT_STOPPED:
		return "unit_stopped"
	case cluster_doctorpb.DriftCategory_UNIT_DISABLED:
		return "unit_disabled"
	case cluster_doctorpb.DriftCategory_VERSION_MISMATCH:
		return "version_mismatch"
	case cluster_doctorpb.DriftCategory_STATE_HASH_MISMATCH:
		return "state_hash_mismatch"
	case cluster_doctorpb.DriftCategory_ENDPOINT_MISSING:
		return "endpoint_missing"
	case cluster_doctorpb.DriftCategory_INVENTORY_INCOMPLETE:
		return "inventory_incomplete"
	default:
		return "unknown"
	}
}

func registerDoctorTools(s *server) {

	// ── cluster_get_doctor_report ───────────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_get_doctor_report",
		Description: "Runs a full cluster health analysis and returns findings with severity, category, and remediation steps. Use this as the primary diagnostic tool to identify issues across the cluster. Shows top issues first for triage.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, doctorEndpoint())
		if err != nil {
			return nil, err
		}
		client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		report, err := client.GetClusterReport(callCtx, &cluster_doctorpb.ClusterReportRequest{})
		if err != nil {
			return nil, fmt.Errorf("GetClusterReport: %w", err)
		}

		findings := make([]map[string]interface{}, 0, len(report.GetFindings()))
		for _, f := range report.GetFindings() {
			remediation := make([]map[string]interface{}, 0, len(f.GetRemediation()))
			for _, r := range f.GetRemediation() {
				step := map[string]interface{}{
					"order":       r.GetOrder(),
					"description": r.GetDescription(),
				}
				if cmd := r.GetCliCommand(); cmd != "" {
					step["cli_command"] = cmd
				}
				remediation = append(remediation, step)
			}

			findings = append(findings, map[string]interface{}{
				"finding_id":  f.GetFindingId(),
				"severity":    severityStr(f.GetSeverity()),
				"category":    f.GetCategory(),
				"summary":     f.GetSummary(),
				"remediation": remediation,
			})
		}

		return map[string]interface{}{
			"overall_status": clusterStatusStr(report.GetOverallStatus()),
			"finding_count":  len(report.GetFindings()),
			"findings":       findings,
			"top_issues":     report.GetTopIssueIds(),
		}, nil
	})

	// ── cluster_get_drift_report ────────────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_get_drift_report",
		Description: "Returns all configuration drift items: differences between desired state and actual state on nodes. Optionally filter by node_id. Each item shows the entity, drift category, desired value, and actual value.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id": {Type: "string", Description: "Optional node ID to filter drift items for a specific node"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, doctorEndpoint())
		if err != nil {
			return nil, err
		}
		client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		req := &cluster_doctorpb.DriftReportRequest{
			NodeId: getStr(args, "node_id"),
		}

		report, err := client.GetDriftReport(callCtx, req)
		if err != nil {
			return nil, fmt.Errorf("GetDriftReport: %w", err)
		}

		items := make([]map[string]interface{}, 0, len(report.GetItems()))
		for _, item := range report.GetItems() {
			items = append(items, map[string]interface{}{
				"node_id":  item.GetNodeId(),
				"entity":   item.GetEntityRef(),
				"category": driftCategoryStr(item.GetCategory()),
				"desired":  item.GetDesired(),
				"actual":   item.GetActual(),
			})
		}

		return map[string]interface{}{
			"total_drift_count": report.GetTotalDriftCount(),
			"items":             items,
		}, nil
	})

	// ── cluster_explain_finding ─────────────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_explain_finding",
		Description: "Provides a deep explanation for a specific finding from the doctor report: why it failed, evidence collected, remediation steps with CLI commands, and risk assessment if a plan is generated. Use this after cluster_get_doctor_report to understand and fix specific issues.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"finding_id": {Type: "string", Description: "The finding ID from cluster_get_doctor_report to explain"},
			},
			Required: []string{"finding_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		findingID := getStr(args, "finding_id")
		if findingID == "" {
			return nil, fmt.Errorf("finding_id is required")
		}

		conn, err := s.clients.get(ctx, doctorEndpoint())
		if err != nil {
			return nil, err
		}
		client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		expl, err := client.ExplainFinding(callCtx, &cluster_doctorpb.ExplainFindingRequest{
			FindingId: findingID,
		})
		if err != nil {
			return nil, fmt.Errorf("ExplainFinding: %w", err)
		}

		remediation := make([]map[string]interface{}, 0, len(expl.GetRemediation()))
		for _, r := range expl.GetRemediation() {
			step := map[string]interface{}{
				"order":       r.GetOrder(),
				"description": r.GetDescription(),
			}
			if cmd := r.GetCliCommand(); cmd != "" {
				step["cli_command"] = cmd
			}
			remediation = append(remediation, step)
		}

		evidence := make([]map[string]interface{}, 0, len(expl.GetEvidence()))
		for _, e := range expl.GetEvidence() {
			ev := map[string]interface{}{
				"source_service": e.GetSourceService(),
				"source_rpc":     e.GetSourceRpc(),
				"key_values":     e.GetKeyValues(),
			}
			if e.GetTimestamp() != nil {
				ev["timestamp"] = fmtTimestamp(e.GetTimestamp().GetSeconds(), e.GetTimestamp().GetNanos())
			}
			evidence = append(evidence, ev)
		}

		return map[string]interface{}{
			"finding_id":  expl.GetFindingId(),
			"why_failed":  expl.GetWhyFailed(),
			"remediation": remediation,
			"evidence":    evidence,
			"plan_risk":   planRiskStr(expl.GetPlanRisk()),
		}, nil
	})
}
