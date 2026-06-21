package main

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// registerInfraTools registers the infrastructure truth-plane MCP tools. They are
// read-only (enforces awareness.mcp_bridge_exposes_safe_tools_only) and consume
// the node-agent GetInfraProbe RPC. Covers ScyllaDB, etcd, MinIO, and Envoy. infra_explain_stall
// is the operator-facing causal tool — it does NOT just dump the probe; it
// answers "why is this stuck and what is the safe next step".
func registerInfraTools(s *server) {
	// ── infra_probe_component ────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "infra_probe_component",
		Description: "Probe one infrastructure component's truth plane (scylladb, etcd, minio, envoy): desired state, rendered/attested config, native-API runtime truth, lifecycle FSM state, and violations. Read-only.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id":      {Type: "string", Description: "Target node ID. Omit for the local node."},
				"component":    {Type: "string", Description: "Component to probe.", Enum: []string{"scylladb", "etcd", "minio", "envoy"}, Default: "scylladb"},
				"bypass_cache": {Type: "boolean", Description: "Force a fresh probe instead of serving the node's probe cache.", Default: false},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		component := getStr(args, "component")
		if component == "" {
			component = "scylladb"
		}
		probe, err := s.fetchInfraProbe(ctx, getStr(args, "node_id"), component, getBool(args, "bypass_cache", false))
		if err != nil {
			return nil, err
		}
		return infraProbeToMap(probe), nil
	})

	// ── infra_probe_all ──────────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "infra_probe_all",
		Description: "Probe every infrastructure component's truth plane on a node (scylladb, etcd, minio, envoy). Read-only.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id":      {Type: "string", Description: "Target node ID. Omit for the local node."},
				"bypass_cache": {Type: "boolean", Description: "Force a fresh probe instead of serving the node's probe cache.", Default: false},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		results, err := s.fetchInfraProbes(ctx, getStr(args, "node_id"), "all", getBool(args, "bypass_cache", false))
		if err != nil {
			return nil, err
		}
		out := make([]interface{}, 0, len(results))
		for _, r := range results {
			out = append(out, infraProbeToMap(r))
		}
		return map[string]interface{}{"results": out, "count": len(out)}, nil
	})

	// ── infra_explain_stall ──────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "infra_explain_stall",
		Description: "Explain WHY an infrastructure component is not a healthy cluster member: expected vs actual lifecycle state, what blocked it, the last successful stage, evidence, the owner to repair (never a manual file edit), and safe read-only next commands. Causal diagnosis, not a raw probe dump.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id":   {Type: "string", Description: "Target node ID. Omit for the local node."},
				"component": {Type: "string", Description: "Component to explain.", Enum: []string{"scylladb", "etcd", "minio", "envoy"}, Default: "scylladb"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		component := getStr(args, "component")
		if component == "" {
			component = "scylladb"
		}
		nodeID := getStr(args, "node_id")
		probe, err := s.fetchInfraProbe(ctx, nodeID, component, true) // fresh probe for a diagnosis
		if err != nil {
			return nil, err
		}
		return explainInfraStall(nodeID, probe), nil
	})

	// ── infra_diff ───────────────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "infra_diff",
		Description: "Show the desired vs rendered vs runtime truth for an infrastructure component side by side (scylladb, etcd, minio, envoy), highlighting fields that disagree. Read-only.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id":   {Type: "string", Description: "Target node ID. Omit for the local node."},
				"component": {Type: "string", Description: "Component to diff.", Enum: []string{"scylladb", "etcd", "minio", "envoy"}, Default: "scylladb"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		component := getStr(args, "component")
		if component == "" {
			component = "scylladb"
		}
		probe, err := s.fetchInfraProbe(ctx, getStr(args, "node_id"), component, true)
		if err != nil {
			return nil, err
		}
		return infraDiff(probe), nil
	})
}

// fetchInfraProbes calls GetInfraProbe on the target node and returns all results.
func (s *server) fetchInfraProbes(ctx context.Context, nodeID, component string, bypass bool) ([]*cluster_controllerpb.InfraProbeResult, error) {
	endpoint, err := s.resolveNodeAgentEndpoint(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("resolve node agent: %w", err)
	}
	conn, err := s.clients.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	client := node_agentpb.NewNodeAgentServiceClient(conn)

	callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
	defer cancel()

	resp, err := client.GetInfraProbe(callCtx, &node_agentpb.GetInfraProbeRequest{
		NodeId:      nodeID,
		Component:   component,
		BypassCache: bypass,
	})
	if err != nil {
		return nil, fmt.Errorf("GetInfraProbe: %w", err)
	}
	return resp.GetResults(), nil
}

// fetchInfraProbe returns the single result for a named component.
func (s *server) fetchInfraProbe(ctx context.Context, nodeID, component string, bypass bool) (*cluster_controllerpb.InfraProbeResult, error) {
	results, err := s.fetchInfraProbes(ctx, nodeID, component, bypass)
	if err != nil {
		return nil, err
	}
	for _, r := range results {
		if r.GetComponent() == component {
			return r, nil
		}
	}
	if len(results) > 0 {
		return results[0], nil // "all" resolved to a single component in Phase 1
	}
	return nil, fmt.Errorf("no infra probe result for component %q", component)
}

func infraProbeToMap(r *cluster_controllerpb.InfraProbeResult) map[string]interface{} {
	if r == nil {
		return map[string]interface{}{}
	}
	violations := make([]map[string]interface{}, 0, len(r.GetViolations()))
	for _, v := range r.GetViolations() {
		violations = append(violations, map[string]interface{}{
			"id":          v.GetId(),
			"severity":    v.GetSeverity(),
			"message":     v.GetMessage(),
			"evidence":    v.GetEvidence(),
			"remediation": v.GetRemediation(),
		})
	}
	out := map[string]interface{}{
		"component":         r.GetComponent(),
		"node_id":           r.GetNodeId(),
		"installed":         r.GetInstalled(),
		"daemon_active":     r.GetDaemonActive(),
		"healthy":           r.GetHealthy(),
		"config_valid":      r.GetConfigValid(),
		"summary":           r.GetSummary(),
		"expected_peers":    r.GetExpectedPeers(),
		"observed_peers":    r.GetObservedPeers(),
		"peers_match":       r.GetPeersMatch(),
		"desired":           r.GetDesired(),
		"rendered":          r.GetRendered(),
		"runtime":           r.GetRuntime(),
		"violations":        violations,
		"probe_stale":       r.GetProbeStale(),
		"probe_age_seconds": r.GetProbeAgeSeconds(),
		"probe_duration_ms": r.GetProbeDurationMs(),
		"errors":            r.GetErrors(),
	}
	if lc := r.GetLifecycle(); lc != nil {
		out["lifecycle"] = map[string]interface{}{
			"state":           lc.GetStateLabel(),
			"blocking_reason": lc.GetBlockingReason(),
			"state_age_secs":  lc.GetStateAgeSeconds(),
		}
	}
	return out
}

// explainInfraStall builds a causal explanation from the probe — the operator's
// "why is this stuck" answer, not a raw dump.
func explainInfraStall(nodeID string, r *cluster_controllerpb.InfraProbeResult) map[string]interface{} {
	lc := r.GetLifecycle()
	actual := lc.GetStateLabel()

	stalled := actual == "stalled" || actual == "degraded" || (!r.GetHealthy() && r.GetInstalled())

	// Highest-severity violation drives the recommended repair target.
	repairTarget := "No violation detected. If still unhealthy, inspect runtime evidence and logs."
	var blocking []map[string]interface{}
	for _, sev := range []string{"CRITICAL", "ERROR", "WARN", "INFO"} {
		for _, v := range r.GetViolations() {
			if v.GetSeverity() != sev {
				continue
			}
			blocking = append(blocking, map[string]interface{}{
				"id": v.GetId(), "severity": v.GetSeverity(), "message": v.GetMessage(), "evidence": v.GetEvidence(),
			})
			if repairTargetFromViolation(v) != "" && repairTarget == "No violation detected. If still unhealthy, inspect runtime evidence and logs." {
				repairTarget = repairTargetFromViolation(v)
			}
		}
	}

	expected := "member_ready"
	if !r.GetInstalled() {
		expected = "not_present"
	}

	return map[string]interface{}{
		"component":                 r.GetComponent(),
		"node_id":                   nodeID,
		"installed":                 r.GetInstalled(),
		"stalled":                   stalled,
		"expected_state":            expected,
		"actual_state":              actual,
		"blocking_reason":           lc.GetBlockingReason(),
		"last_successful_stage":     lastSuccessfulStage(r),
		"blocking_violations":       blocking,
		"evidence":                  map[string]interface{}{"rendered": r.GetRendered(), "runtime": r.GetRuntime(), "summary": r.GetSummary(), "probe_errors": r.GetErrors()},
		"recommended_repair_target": repairTarget,
		"safe_next_commands": []string{
			fmt.Sprintf("infra_diff(node_id=%q, component=%q)", nodeID, r.GetComponent()),
			fmt.Sprintf("nodeagent_get_service_logs(node_id=%q, unit=%q)", nodeID, infraLogUnit(r.GetComponent())),
			"cluster_get_doctor_report(freshness=\"fresh\")",
		},
		"note": "Repair the config OWNER, never hand-edit the rendered config file — a render overwrites it.",
	}
}

// infraLogUnit maps a component to the systemd unit whose logs an operator would
// read when diagnosing it.
func infraLogUnit(component string) string {
	switch component {
	case "etcd":
		return "globular-etcd"
	case "minio":
		return "globular-minio"
	case "envoy":
		return "globular-envoy"
	default:
		return "scylla-server"
	}
}

func repairTargetFromViolation(v *cluster_controllerpb.InfraViolation) string {
	if r := strings.TrimSpace(v.GetRemediation()); r != "" {
		return r
	}
	return ""
}

// lastSuccessfulStage infers the furthest stage the component actually reached,
// from the runtime booleans — independent of the (possibly stalled) FSM state.
// The runtime key names differ per component (Scylla: cql_ready/rest_api_ready;
// etcd: local_reachable), so this checks the union.
func lastSuccessfulStage(r *cluster_controllerpb.InfraProbeResult) string {
	rt := r.GetRuntime()
	switch {
	case rt["cql_ready"] == "true" || rt["write_quorum"] == "true" || rt["active_listeners"] != "" && rt["active_listeners"] != "0":
		return "cql_ready"
	case rt["rest_api_ready"] == "true" || rt["local_reachable"] == "true" || rt["live"] == "true" || rt["admin_reachable"] == "true":
		return "local_api_ready"
	case r.GetDaemonActive():
		return "daemon_starting"
	case r.GetConfigValid():
		return "config_attested"
	case len(r.GetRendered()) > 0 && r.GetRendered()["present"] == "true":
		return "config_rendered"
	case r.GetInstalled():
		return "package_installed"
	default:
		return "not_present"
	}
}

// infraDiff lays desired/rendered/runtime side by side and flags disagreements
// on the key cluster-facing fields. The comparable fields differ per component:
// Scylla compares cluster_name/listen_address/seeds; etcd compares the cluster
// token and the advertised client host.
func infraDiff(r *cluster_controllerpb.InfraProbeResult) map[string]interface{} {
	desired, rendered, runtime := r.GetDesired(), r.GetRendered(), r.GetRuntime()

	// Compare the fields where desired and rendered share a meaning.
	type row struct{ field, desired, rendered string }
	var checks []row
	switch r.GetComponent() {
	case "etcd":
		checks = []row{
			{"cluster_token", desired["cluster_name"], rendered["initial_cluster_token"]},
			{"listen_address", firstCSV(desired["expected_listen"]), hostOfFirstURL(rendered["advertise_client_urls"])},
		}
	case "minio":
		checks = []row{
			{"mode", desired["mode"], rendered["mode"]},
			{"volume_count", desired["expected_volume_count"], rendered["volume_count"]},
		}
	case "envoy":
		// Envoy is per-node: the meaningful comparison is the node id and whether
		// the ADS cluster the bootstrap targets matches the expected name.
		checks = []row{
			{"node_id", desired["node_id"], rendered["node_id"]},
			{"ads_cluster", desired["cluster_name"], rendered["ads_cluster"]},
		}
	default:
		checks = []row{
			{"cluster_name", desired["cluster_name"], rendered["cluster_name"]},
			{"listen_address", firstCSV(desired["expected_listen"]), rendered["listen_address"]},
			{"seeds", desired["expected_seeds"], rendered["seeds"]},
		}
	}
	var mismatches []map[string]string
	for _, c := range checks {
		if c.desired != "" && c.rendered != "" && c.desired != c.rendered {
			mismatches = append(mismatches, map[string]string{"field": c.field, "desired": c.desired, "rendered": c.rendered})
		}
	}

	return map[string]interface{}{
		"component":  r.GetComponent(),
		"node_id":    r.GetNodeId(),
		"desired":    desired,
		"rendered":   rendered,
		"runtime":    runtime,
		"mismatches": mismatches,
		"keys":       sortedKeys(desired, rendered, runtime),
	}
}

func firstCSV(s string) string {
	if i := strings.IndexByte(s, ','); i >= 0 {
		return s[:i]
	}
	return s
}

// hostOfFirstURL returns the bare host of the first URL in a comma-separated
// list like "https://10.0.0.63:2379,https://...". Used to compare an etcd
// advertised client URL against the desired listen address.
func hostOfFirstURL(s string) string {
	first := firstCSV(s)
	if u, err := url.Parse(strings.TrimSpace(first)); err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return strings.TrimSpace(first)
}

func sortedKeys(maps ...map[string]string) []string {
	seen := map[string]bool{}
	for _, m := range maps {
		for k := range m {
			seen[k] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
