package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/globulario/services/golang/config"
)

// collector gathers endpoint metrics from Prometheus for scoring.
type collector struct {
	promURL string // e.g. "http://<routable-ip>:9090"
	client  *http.Client
}

func newCollector() *collector {
	promHost := config.GetRoutableIPv4()
	return &collector{
		promURL: fmt.Sprintf("http://%s:9090", promHost),
		client:  &http.Client{Timeout: 2 * time.Second},
	}
}

// endpointMetrics holds all signals for one service instance.
type endpointMetrics struct {
	Service   string
	Instance  string // Prometheus instance label (ip:port)
	LatencyP99 float64 // seconds
	ErrorRate  float64 // 0.0-1.0
	RPS        float64 // requests per second
	Stale      bool
}

// nodeMetrics holds node-level signals.
type nodeMetrics struct {
	CPUUsage    float64 // 0.0-1.0
	MemoryUsage float64 // 0.0-1.0
	Stale       bool
}

// collectAll gathers metrics for all gRPC services in one batch.
func (c *collector) collectAll(ctx context.Context) (map[string]*endpointMetrics, *nodeMetrics, error) {
	endpoints := make(map[string]*endpointMetrics)

	// Query 1: p99 latency per service
	latencies, err := c.queryVector(ctx, `histogram_quantile(0.99, sum(rate(grpc_server_handling_seconds_bucket[5m])) by (grpc_service, instance, le))`)
	if err != nil {
		return nil, nil, fmt.Errorf("latency query: %w", err)
	}
	for _, sample := range latencies {
		svc := sample.metric["grpc_service"]
		inst := sample.metric["instance"]
		key := svc + "/" + inst
		ep := endpoints[key]
		if ep == nil {
			ep = &endpointMetrics{Service: svc, Instance: inst}
			endpoints[key] = ep
		}
		ep.LatencyP99 = sample.value
	}

	// Query 2: error rate per service
	errors_, err := c.queryVector(ctx, `sum(rate(grpc_server_handled_total{grpc_code!="OK"}[5m])) by (grpc_service, instance)`)
	if err != nil {
		return nil, nil, fmt.Errorf("error rate query: %w", err)
	}
	for _, sample := range errors_ {
		svc := sample.metric["grpc_service"]
		inst := sample.metric["instance"]
		key := svc + "/" + inst
		ep := endpoints[key]
		if ep == nil {
			ep = &endpointMetrics{Service: svc, Instance: inst}
			endpoints[key] = ep
		}
		ep.ErrorRate = sample.value
	}

	// Query 3: RPS per service (total rate including OK)
	rps, err := c.queryVector(ctx, `sum(rate(grpc_server_handled_total[5m])) by (grpc_service, instance)`)
	if err != nil {
		return nil, nil, fmt.Errorf("rps query: %w", err)
	}
	for _, sample := range rps {
		svc := sample.metric["grpc_service"]
		inst := sample.metric["instance"]
		key := svc + "/" + inst
		ep := endpoints[key]
		if ep == nil {
			ep = &endpointMetrics{Service: svc, Instance: inst}
			endpoints[key] = ep
		}
		ep.RPS = sample.value
		// Compute error rate as ratio (errors / total)
		if ep.RPS > 0 && ep.ErrorRate > 0 {
			ep.ErrorRate = ep.ErrorRate / ep.RPS
		} else {
			ep.ErrorRate = 0
		}
	}

	// Query 4: node CPU usage
	node := &nodeMetrics{}
	cpuResults, err := c.queryVector(ctx, `1 - avg(rate(node_cpu_seconds_total{mode="idle"}[1m]))`)
	if err != nil {
		node.Stale = true
	} else if len(cpuResults) > 0 {
		node.CPUUsage = cpuResults[0].value
	}

	// Query 5: node memory usage
	memResults, err := c.queryVector(ctx, `1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)`)
	if err != nil {
		node.Stale = true
	} else if len(memResults) > 0 {
		node.MemoryUsage = memResults[0].value
	}

	return endpoints, node, nil
}

// promSample represents one metric sample from Prometheus.
type promSample struct {
	metric map[string]string
	value  float64
}

// queryVector executes an instant PromQL query and returns vector results.
func (c *collector) queryVector(ctx context.Context, query string) ([]promSample, error) {
	reqURL := c.promURL + "/api/v1/query"
	form := url.Values{"query": {query}}

	req, err := http.NewRequestWithContext(ctx, "POST", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = io.NopCloser(nil)

	// Use POST with form body to avoid URL encoding issues with PromQL.
	resp, err := c.client.PostForm(reqURL, form)
	if err != nil {
		return nil, fmt.Errorf("prometheus query failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prometheus returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Value  [2]json.RawMessage `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if result.Status != "success" {
		return nil, fmt.Errorf("prometheus query status: %s", result.Status)
	}

	var samples []promSample
	for _, r := range result.Data.Result {
		var valStr string
		if err := json.Unmarshal(r.Value[1], &valStr); err != nil {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		// Skip NaN / Inf values.
		if val != val { // NaN check
			continue
		}
		samples = append(samples, promSample{metric: r.Metric, value: val})
	}
	return samples, nil
}
