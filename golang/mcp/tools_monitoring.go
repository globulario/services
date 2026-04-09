package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/monitoring/monitoringpb"
)

const defaultConnectionID = "local_prometheus"

func monitoringEndpoint() string {
	return config.ResolveServiceAddr("monitoring.MonitoringService", "")
}

// ensureConnection creates the default local Prometheus connection if it
// doesn't exist yet. The monitoring service persists connections, so this
// is effectively a no-op on subsequent calls.
func ensureConnection(ctx context.Context, s *server) error {
	conn, err := s.clients.get(ctx, monitoringEndpoint())
	if err != nil {
		return fmt.Errorf("dial monitoring: %w", err)
	}
	client := monitoringpb.NewMonitoringServiceClient(conn)

	callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
	defer cancel()

	// Try a simple query first — if it works, the connection already exists.
	_, err = client.Query(callCtx, &monitoringpb.QueryRequest{
		ConnectionId: defaultConnectionID,
		Query:        "up",
		Ts:           float64(time.Now().Unix()),
	})
	if err == nil {
		return nil
	}

	// Connection doesn't exist — create it.
	callCtx2, cancel2 := context.WithTimeout(authCtx(ctx), 10*time.Second)
	defer cancel2()

	promHost := config.GetRoutableIPv4()
	_, err = client.CreateConnection(callCtx2, &monitoringpb.CreateConnectionRqst{
		Connection: &monitoringpb.Connection{
			Id:    defaultConnectionID,
			Host:  promHost,
			Port:  9090,
			Store: monitoringpb.StoreType_PROMETHEUS,
		},
	})
	if err != nil {
		return fmt.Errorf("create default Prometheus connection: %w", err)
	}
	return nil
}

func registerMonitoringTools(s *server) {

	// ── metrics_query ──────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "metrics_query",
		Description: `Execute an instant PromQL query against Prometheus.

Returns the current value of a metric expression. Use this for point-in-time checks.

Examples:
  query: "up"                                          → which targets are up
  query: "rate(grpc_server_handled_total[5m])"         → request rate per service
  query: "rate(grpc_server_handled_total{grpc_code!=\"OK\"}[5m])"  → error rate
  query: "histogram_quantile(0.99, sum(rate(grpc_server_handling_seconds_bucket[5m])) by (grpc_service, le))"  → p99 latency
  query: "node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes"  → memory available %
  query: "minio_disk_storage_used_bytes"               → MinIO storage usage
  query: "scylla_database_total_writes"                → ScyllaDB write throughput
  query: "envoy_http_downstream_rq_total"              → Envoy request count`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"query":         {Type: "string", Description: "PromQL expression to evaluate"},
				"connection_id": {Type: "string", Description: "Prometheus connection ID (default: local_prometheus)"},
				"timeout":       {Type: "number", Description: "Request timeout in seconds (default: 30)"},
			},
			Required: []string{"query"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		query := getStr(args, "query")
		if query == "" {
			return nil, fmt.Errorf("query is required")
		}
		connID := getStr(args, "connection_id")
		if connID == "" {
			connID = defaultConnectionID
		}
		timeout := getInt(args, "timeout", 30)

		if err := ensureConnection(ctx, s); err != nil {
			return nil, err
		}

		conn, err := s.clients.get(ctx, monitoringEndpoint())
		if err != nil {
			return nil, err
		}
		client := monitoringpb.NewMonitoringServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), time.Duration(timeout)*time.Second)
		defer cancel()

		resp, err := client.Query(callCtx, &monitoringpb.QueryRequest{
			ConnectionId: connID,
			Query:        query,
			Ts:           float64(time.Now().Unix()),
		})
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(monitoringEndpoint())
			}
			return nil, fmt.Errorf("Query: %w", err)
		}

		// Try to parse the value as JSON for structured output.
		var parsed interface{}
		if err := json.Unmarshal([]byte(resp.GetValue()), &parsed); err != nil {
			parsed = resp.GetValue()
		}

		result := map[string]interface{}{
			"query":  query,
			"result": parsed,
		}
		if w := resp.GetWarnings(); w != "" {
			result["warnings"] = w
		}
		return result, nil
	})

	// ── metrics_query_range ────────────────────────────────────────────────
	s.register(toolDef{
		Name: "metrics_query_range",
		Description: `Execute a PromQL range query over a time window.

Returns time-series data points. Use this for trends, charts, and historical analysis.

The range parameter accepts Go duration strings: "5m", "1h", "24h", "7d" (converted to hours).

Examples:
  query: "rate(grpc_server_handled_total[5m])", range: "1h", step: 60     → request rate over last hour
  query: "node_memory_MemAvailable_bytes", range: "24h", step: 300        → memory trend over 24h
  query: "rate(scylla_database_total_writes[5m])", range: "6h", step: 120 → ScyllaDB write rate over 6h`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"query":         {Type: "string", Description: "PromQL expression to evaluate"},
				"range":         {Type: "string", Description: "Lookback window as duration string, e.g. '1h', '30m', '24h' (default: 1h)"},
				"step":          {Type: "number", Description: "Step between data points in seconds (default: 60)"},
				"connection_id": {Type: "string", Description: "Prometheus connection ID (default: local_prometheus)"},
				"timeout":       {Type: "number", Description: "Request timeout in seconds (default: 60)"},
			},
			Required: []string{"query"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		query := getStr(args, "query")
		if query == "" {
			return nil, fmt.Errorf("query is required")
		}
		connID := getStr(args, "connection_id")
		if connID == "" {
			connID = defaultConnectionID
		}
		rangeStr := getStr(args, "range")
		if rangeStr == "" {
			rangeStr = "1h"
		}
		step := getInt(args, "step", 60)
		timeout := getInt(args, "timeout", 60)

		rangeDur, err := time.ParseDuration(rangeStr)
		if err != nil {
			return nil, fmt.Errorf("invalid range %q: %w", rangeStr, err)
		}

		if err := ensureConnection(ctx, s); err != nil {
			return nil, err
		}

		conn, err := s.clients.get(ctx, monitoringEndpoint())
		if err != nil {
			return nil, err
		}
		client := monitoringpb.NewMonitoringServiceClient(conn)

		now := time.Now()
		callCtx, cancel := context.WithTimeout(authCtx(ctx), time.Duration(timeout)*time.Second)
		defer cancel()

		stream, err := client.QueryRange(callCtx, &monitoringpb.QueryRangeRequest{
			ConnectionId: connID,
			Query:        query,
			StartTime:    float64(now.Add(-rangeDur).Unix()),
			EndTime:      float64(now.Unix()),
			Step:         float64(step),
		})
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(monitoringEndpoint())
			}
			return nil, fmt.Errorf("QueryRange: %w", err)
		}

		// Collect streamed responses.
		var value string
		var warnings string
		for i := 0; i < 200; i++ {
			msg, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("QueryRange stream: %w", err)
			}
			value += msg.GetValue()
			if w := msg.GetWarnings(); w != "" {
				warnings = w
			}
		}

		// Try to parse as JSON.
		var parsed interface{}
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			parsed = value
		}

		result := map[string]interface{}{
			"query": query,
			"range": rangeStr,
			"step":  step,
			"result": parsed,
		}
		if warnings != "" {
			result["warnings"] = warnings
		}
		return result, nil
	})

	// ── metrics_targets ────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "metrics_targets",
		Description: `List all Prometheus scrape targets and their status.

Returns which targets are up/down, their labels, last scrape time, and errors.
Use this to check if MinIO, ScyllaDB, Envoy, or any Globular service is being scraped.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"connection_id": {Type: "string", Description: "Prometheus connection ID (default: local_prometheus)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		connID := getStr(args, "connection_id")
		if connID == "" {
			connID = defaultConnectionID
		}

		if err := ensureConnection(ctx, s); err != nil {
			return nil, err
		}

		conn, err := s.clients.get(ctx, monitoringEndpoint())
		if err != nil {
			return nil, err
		}
		client := monitoringpb.NewMonitoringServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		resp, err := client.Targets(callCtx, &monitoringpb.TargetsRequest{
			ConnectionId: connID,
		})
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(monitoringEndpoint())
			}
			return nil, fmt.Errorf("Targets: %w", err)
		}

		var parsed interface{}
		if err := json.Unmarshal([]byte(resp.GetResult()), &parsed); err != nil {
			parsed = resp.GetResult()
		}

		return map[string]interface{}{
			"targets": parsed,
		}, nil
	})

	// ── metrics_alerts ─────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "metrics_alerts",
		Description: `List all active Prometheus alerts.

Returns firing and pending alerts with their labels, annotations, and state.
Use this to quickly check if anything is wrong in the cluster.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"connection_id": {Type: "string", Description: "Prometheus connection ID (default: local_prometheus)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		connID := getStr(args, "connection_id")
		if connID == "" {
			connID = defaultConnectionID
		}

		if err := ensureConnection(ctx, s); err != nil {
			return nil, err
		}

		conn, err := s.clients.get(ctx, monitoringEndpoint())
		if err != nil {
			return nil, err
		}
		client := monitoringpb.NewMonitoringServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		resp, err := client.Alerts(callCtx, &monitoringpb.AlertsRequest{
			ConnectionId: connID,
		})
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(monitoringEndpoint())
			}
			return nil, fmt.Errorf("Alerts: %w", err)
		}

		var parsed interface{}
		if err := json.Unmarshal([]byte(resp.GetResults()), &parsed); err != nil {
			parsed = resp.GetResults()
		}

		return map[string]interface{}{
			"alerts": parsed,
		}, nil
	})

	// ── metrics_rules ──────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "metrics_rules",
		Description: `List all Prometheus alerting and recording rules.

Returns rule groups, their evaluation intervals, and current state.
Use this to understand what alerting is configured.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"connection_id": {Type: "string", Description: "Prometheus connection ID (default: local_prometheus)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		connID := getStr(args, "connection_id")
		if connID == "" {
			connID = defaultConnectionID
		}

		if err := ensureConnection(ctx, s); err != nil {
			return nil, err
		}

		conn, err := s.clients.get(ctx, monitoringEndpoint())
		if err != nil {
			return nil, err
		}
		client := monitoringpb.NewMonitoringServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		resp, err := client.Rules(callCtx, &monitoringpb.RulesRequest{
			ConnectionId: connID,
		})
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(monitoringEndpoint())
			}
			return nil, fmt.Errorf("Rules: %w", err)
		}

		var parsed interface{}
		if err := json.Unmarshal([]byte(resp.GetResult()), &parsed); err != nil {
			parsed = resp.GetResult()
		}

		return map[string]interface{}{
			"rules": parsed,
		}, nil
	})

	// ── metrics_label_values ───────────────────────────────────────────────
	s.register(toolDef{
		Name: "metrics_label_values",
		Description: `Get all values for a Prometheus label.

Use this to discover what services, instances, or jobs exist.

Examples:
  label: "job"           → list all scrape job names (envoy, scylla, minio, globular, ...)
  label: "grpc_service"  → list all gRPC service names
  label: "instance"      → list all scrape target addresses
  label: "grpc_code"     → list all gRPC status codes seen`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"label":         {Type: "string", Description: "Label name to get values for"},
				"connection_id": {Type: "string", Description: "Prometheus connection ID (default: local_prometheus)"},
			},
			Required: []string{"label"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		label := getStr(args, "label")
		if label == "" {
			return nil, fmt.Errorf("label is required")
		}
		connID := getStr(args, "connection_id")
		if connID == "" {
			connID = defaultConnectionID
		}

		if err := ensureConnection(ctx, s); err != nil {
			return nil, err
		}

		conn, err := s.clients.get(ctx, monitoringEndpoint())
		if err != nil {
			return nil, err
		}
		client := monitoringpb.NewMonitoringServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 15*time.Second)
		defer cancel()

		resp, err := client.LabelValues(callCtx, &monitoringpb.LabelValuesRequest{
			ConnectionId: connID,
			Label:        label,
		})
		if err != nil {
			if isConnError(err) {
				s.clients.invalidate(monitoringEndpoint())
			}
			return nil, fmt.Errorf("LabelValues: %w", err)
		}

		var parsed interface{}
		raw := resp.GetLabelValues()
		if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
			parsed = raw
		}

		result := map[string]interface{}{
			"label":  label,
			"values": parsed,
		}
		if w := resp.GetWarnings(); w != "" {
			result["warnings"] = w
		}
		return result, nil
	})
}
