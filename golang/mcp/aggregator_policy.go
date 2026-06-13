// @awareness namespace=globular.platform
// @awareness component=platform_mcp.aggregator_policy
// @awareness file_role=phase1_allowlist_of_read_only_tools_callable_via_mcp_remote_call
// @awareness implements=globular.platform:intent.mcp.aggregator_routes_via_etcd_discovery
// @awareness implements=globular.platform:intent.awareness.mcp_bridge_exposes_safe_tools_only
// @awareness enforces=globular.platform:invariant.mcp.etcd.tools_must_not_join_remote_allowlist
// @awareness risk=critical
package main

// aggregator_policy.go — the explicit allowlist of tools that
// other nodes' MCP servers may invoke on this one via
// mcp.remote_call. Every entry MUST be read-only and pure —
// adding a mutating tool here (etcd.put/delete,
// workflow.execute, nodeagent.apply) opens cross-node mutation
// without controller workflow oversight.
//
// New entries require explicit operator review. There is no
// "auto-allow if not destructive" branch — destructiveness is
// not always knowable from the tool name alone.

// allowedRemoteTools is the phase-1 allowlist for tools callable via mcp.remote_call.
// All entries are read-only local tools that do not mutate state.
// Mutating tools (etcd.put/delete, workflow.execute, nodeagent.apply, etc.) are
// forbidden by default and must never be added here without controller/workflow policy.
var allowedRemoteTools = map[string]bool{
	// Node agent read-only tools
	"nodeagent_get_inventory":          true,
	"nodeagent_list_installed_packages": true,
	"nodeagent_get_installed_package":   true,
	"nodeagent_get_certificate_status":  true,
	"nodeagent_get_service_logs":        true,
	"nodeagent_search_logs":             true,

	// Monitoring / metrics (Prometheus-backed read-only queries)
	"metrics_query":        true,
	"metrics_query_range":  true,
	"metrics_targets":      true,
	"metrics_alerts":       true,
	"metrics_rules":        true,
	"metrics_label_values": true,

	// Doctor findings (read-only diagnosis)
	"cluster_get_doctor_report": true,

	// Infrastructure truth plane (read-only diagnosis; Phase 1: scylladb).
	// These observe a node's infra via GetInfraProbe and never mutate state,
	// so they belong with the other read-only remote tools.
	"infra_probe_component": true,
	"infra_probe_all":       true,
	"infra_explain_stall":   true,
	"infra_diff":            true,
}

// forbiddenRemoteTools lists tools that must never be called remotely.
// Defined explicitly so violations can be classified precisely.
var forbiddenRemoteTools = map[string]bool{
	"etcd_put":              true,
	"etcd_delete":           true,
	"nodeagent_control_service": true,
	"nodeagent_installed_set":   true,
	"repository_publish":        true,
	"backup_restore":            true,
	"file_write":                true,
	"file_delete":               true,
}

// IsRemoteToolAllowed returns true when the tool may be called through the aggregator.
func IsRemoteToolAllowed(tool string) bool {
	return allowedRemoteTools[tool]
}

// ClassifyRemoteToolSafety returns a safety label for a given tool name.
func ClassifyRemoteToolSafety(tool string) string {
	if allowedRemoteTools[tool] {
		return "READ_ONLY"
	}
	if forbiddenRemoteTools[tool] {
		return "FORBIDDEN"
	}
	return "NOT_ALLOWLISTED"
}
