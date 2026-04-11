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

// defaultPromEndpoint returns the cluster-local Prometheus endpoint. Envoy
// usually fronts Prometheus, so prefer the DNS name; fall back to loopback.
func defaultPromEndpoint() string {
	if v := os.Getenv("PROMETHEUS_ENDPOINT"); v != "" {
		return v
	}
	return "http://prometheus.globular.internal" // routed via Envoy
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
		"controller_loop_heartbeat_age": "time() - globular_controller_loop_heartbeat_unix",
		"workflow_oldest_active_age":    "globular_workflow_oldest_active_age_seconds",
		"workflow_active_runs":          "globular_workflow_active_runs",
		"node_heartbeat_age_max":        "max(time() - globular_node_agent_heartbeat_success_unix)",
		"etcd_has_leader":               "max(etcd_server_has_leader)",
		"etcd_quorum_size":              "max(etcd_server_quorum_size)",
	}

	results := make(map[string]float64)

	for key, q := range queries {
		val, err := c.promQuery(ctx, client, endpoint, q)
		if err != nil {
			snap.addError("prometheus", key, err)
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
