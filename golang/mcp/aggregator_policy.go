// @awareness namespace=globular.platform
// @awareness component=platform_mcp.aggregator
// @awareness file_role=mcp_aggregator_routing_policy
// @awareness risk=high
package main

// allowedRemoteTools is the phase-1 allowlist for tools callable via mcp.remote_call.
// All entries are read-only local tools that do not mutate state.
// Mutating tools (etcd.put/delete, workflow.execute, nodeagent.apply, etc.) are
// forbidden by default and must never be added here without controller/workflow policy.
var allowedRemoteTools = map[string]bool{
	// Awareness evidence pipeline
	"awareness.bundle_status":     true,
	"awareness.runtime_errors":    true,
	"awareness.normalize_errors":  true,
	"awareness.day1_classify_node": true,
	"awareness.explain_blocker":   true,

	// Awareness bundle serve (Phase B) — read-only, returns manifest/metadata only.
	// Bundle bytes themselves are streamed via HTTPS at /awareness/bundle, not
	// returned in the JSON response, so these are safe to expose remotely.
	"mcp.awareness_bundle_manifest":    true,
	"mcp.awareness_bundle_stream":      true,
	"mcp.awareness_freshness_status":   true,

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
