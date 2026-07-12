package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// defaultPromEndpoint returns the Prometheus HTTP query endpoint.
//
// Prometheus runs on every node at :9090 and is NOT registered in the
// etcd service registry. DNS-based access ("prometheus.globular.internal")
// resolves through the Envoy gateway on port 443, which requires HTTPS
// and sometimes doesn't have a route configured — causing persistent
// data_errors in doctor. Direct loopback avoids all those issues: the
// doctor process always lives on the same host as a Prometheus instance,
// no TLS is needed for loopback HTTP, and no gateway route is required.
//
// Override with PROMETHEUS_ENDPOINT env var for non-standard setups.
func defaultPromEndpoint() string {
	if v := os.Getenv("PROMETHEUS_ENDPOINT"); v != "" {
		return v
	}
	return "http://127.0.0.1:9090"
}

// fetchPrometheus executes a handful of instant queries to enrich the snapshot.
// Best-effort: errors mark DataIncomplete but don't fail the doctor run.
func (c *Collector) fetchPrometheus(ctx context.Context, snap *Snapshot) {
	endpoint := strings.TrimSpace(c.promEndpoint)
	if endpoint == "" {
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}

	queries := map[string]string{
		// The `> 0` filter excludes series whose heartbeat timestamp is still 0
		// (a node/controller that has been scraped but has not yet recorded a
		// successful heartbeat — e.g. a just-joined node). Without it,
		// `time() - 0` yields the full Unix epoch (~1.78e9s ≈ 56 years) as a
		// bogus "age", firing a false CRITICAL on every join. An unset heartbeat
		// is "not yet reporting" (unknown), NOT "stale"; if every series is 0 the
		// result is empty and the metric is simply absent (no false finding).
		// See failure_mode controller.non_leader_heartbeat_false_positive and
		// repair.runtime_evidence_stale_or_conflicting.
		"controller_loop_heartbeat_age": "time() - max(globular_controller_loop_heartbeat_unix > 0)",
		"workflow_oldest_active_age":    "globular_workflow_oldest_active_age_seconds",
		"workflow_active_runs":          "globular_workflow_active_runs",
		"node_heartbeat_age_max":        "max(time() - (globular_node_agent_heartbeat_success_unix > 0))",
		"etcd_has_leader":               "max(etcd_server_has_leader)",
		// etcd_server_quorum_size doesn't exist in etcd 3.5.x.
		// Use count(etcd_server_id) which counts how many etcd members
		// this Prometheus can see. On a single-node scrape it returns 1;
		// with federation it returns the full membership.
		"etcd_quorum_size": "count(etcd_server_id)",
		// Non-leader indicator: sum > 0 means the scraped instance(s) have
		// explicitly dropped reconcile ticks because they are not the leader.
		// Used by prometheus_runtime.go to suppress false-positive stall findings
		// when Prometheus only scrapes a non-leader controller instance.
		"reconcile_dropped_not_leader": "sum(globular_controller_reconcile_dropped_not_leader_total)",
		// Storm protection signals (Phase A-D).
		"apply_loop_detected":        "globular_controller_apply_loop_detected_total",
		"drift_kind_mismatch":        "globular_controller_drift_kind_mismatch_total",
		// Rate over a 15-minute window — used by the kind_mismatch rule.
		// Auto-clears when no new mismatches have been detected in the window.
		// The raw counter above is kept for audit but must NOT be consumed by
		// rules directly (a counter only ever grows → finding fires forever).
		"drift_kind_mismatch_rate_15m": "sum(increase(globular_controller_drift_kind_mismatch_total[15m]))",
		// Current-state gauge (1 = breaker open now, 0 = closed). The raw
		// globular_controller_reconcile_circuit_open_total counter must NOT be
		// consumed here: a counter only ever grows, so the finding would fire
		// CRITICAL forever after a single transient open and never auto-clear.
		"reconcile_circuit_open":     "globular_controller_reconcile_circuit_open",
		// Raw counter — kept for backwards compatibility / audit. The rule
		// itself should NOT consume this directly: a counter only ever
		// grows, so the finding would fire forever after a single
		// transient blip and never auto-clear.
		"workflow_dispatch_rejected": "globular_controller_workflow_dispatch_rejected_total",
		// Rate over a 5-minute window — used by the backend_pressure
		// rule to distinguish transient flap (one-time rejection during
		// a restart) from sustained pressure (rejections still
		// happening). Auto-clears when the window has no rejections.
		"workflow_dispatch_rejected_rate_5m": "sum(increase(globular_controller_workflow_dispatch_rejected_total[5m]))",
		// Same idea over a longer window, so the rule can elevate
		// severity when pressure persists across multiple sweeps.
		"workflow_dispatch_rejected_rate_15m": "sum(increase(globular_controller_workflow_dispatch_rejected_total[15m]))",
		// Day-1 resilience signals (Phase 2-4).
		"workflow_circuit_open":        "globular_controller_workflow_circuit_open",
		"release_transient_blocked":    "globular_controller_release_transient_blocked",
		"controller_leader_outdated":   "globular_controller_leader_outdated",
		"controller_no_safe_successor": "globular_controller_no_safe_successor",
		// Dependency cache watch health (Phase A-C depcache).
		"depcache_watch_inactive":  "sum(globular_depcache_watch_active == 0)",
		"depcache_watch_errors_5m": "sum(increase(globular_depcache_watch_errors_total[5m]))",
		// xDS config generation tracking (Phase F).
		"xds_config_events_total":  "globular_controller_xds_config_events_total",
		"xds_config_applied_total": "globular_controller_xds_config_applied_total",
		"xds_last_applied_unix":    "max(globular_controller_xds_last_applied_unix)",
		// Reconcile lane isolation health.
		"reconcile_lane_timeouts_cluster":       `sum(globular_controller_reconcile_lane_timeouts_total{lane="cluster_reconcile"})`,
		"reconcile_lane_timeouts_projections":   `sum(globular_controller_reconcile_lane_timeouts_total{lane="projections"})`,
		"reconcile_lane_blocked_cluster":        `max(globular_controller_reconcile_blocked_phase{phase="cluster_reconcile"})`,
		"reconcile_lane_blocked_projections":    `max(globular_controller_reconcile_blocked_phase{phase="projections"})`,
		"reconcile_lane_blocked_release_bridge": `max(globular_controller_reconcile_blocked_phase{phase="release_bridge"})`,
		"reconcile_lane_blocked_drift":          `max(globular_controller_reconcile_blocked_phase{phase="drift_reconcile"})`,
		// Envoy data-plane handshake signals. Consumed by envoyLDSWedge
		// (rules/envoy_lds_wedge.go) which pins the invariant
		// envoy.lds_progress_required_for_http_mesh_readiness and detects
		// the failure_mode envoy.lds_update_attempt_zero_despite_cds_progress.
		//
		// Envoy publishes one counter per xDS type. CDS updates are received
		// FIRST during init (before LDS), so if CDS counts > 0 but LDS attempt
		// stays at 0, the mesh handshake is wedged before it can produce
		// listeners — port 443 stays unbound and the HTTP mesh is down even
		// though `systemctl is-active globular-envoy.service` reports active.
		// max() collapses any per-pod label dimensions to a single scalar.
		"envoy_cds_update_success":   "max(envoy_cluster_manager_cds_update_success)",
		"envoy_lds_update_attempt":   "max(envoy_listener_manager_lds_update_attempt)",
		"envoy_lds_update_success":   "max(envoy_listener_manager_lds_update_success)",
		"envoy_lds_update_rejected":  "max(envoy_listener_manager_lds_update_rejected)",
	}

	results := make(map[string]float64)

	for key, q := range queries {
		val, err := c.promQuery(ctx, client, endpoint, q)
		if err != nil {
			// "no data" from an optional enrichment query is not a hard
			// error — the metric may not exist in this version or may
			// have no current samples. Only surface real HTTP/parse
			// failures. This prevents non-existent optional metrics from
			// keeping data_incomplete permanently set.
			if !strings.Contains(err.Error(), "prometheus no data") {
				snap.addError("prometheus", key, err)
			}
			continue
		}
		results[key] = val
	}

	if len(results) > 0 {
		snap.PromMetrics = results
		snap.PromTS = time.Now()
		snap.addSource("prometheus")
	}
}

func (c *Collector) promQuery(ctx context.Context, client *http.Client, endpoint, query string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/query", endpoint), nil)
	if err != nil {
		return 0, err
	}
	q := req.URL.Query()
	q.Set("query", query)
	req.URL.RawQuery = q.Encode()

	if c.promTokenFile != "" {
		if b, err := os.ReadFile(c.promTokenFile); err == nil {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(b)))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus query status %d: %s", resp.StatusCode, string(b))
	}

	var out struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Value [2]any `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return 0, err
	}
	if out.Status != "success" || len(out.Data.Result) == 0 {
		return 0, fmt.Errorf("prometheus no data")
	}
	// Value[1] should be a string numeric.
	if s, ok := out.Data.Result[0].Value[1].(string); ok {
		var f float64
		_, err := fmt.Sscan(s, &f)
		return f, err
	}
	return 0, fmt.Errorf("unexpected value type")
}
