package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/globulario/services/golang/interceptors"
)

func registerLogRingTools(s *server) {

	// ── set_log_verbosity ───────────────────────────────────────────────────
	s.register(toolDef{
		Name: "set_log_verbosity",
		Description: `Change the interceptor log verbosity at runtime without restarting services.

Levels (from most to least verbose):
- TRACE: every request entering/exiting (method, duration, caller)
- DEBUG: auth decisions, routing, token resolution
- INFO: errors, slow requests, state changes (default)
- WARN: denials, anomalies only
- ERROR: handler panics and auth failures only

Changes take effect immediately on this MCP server's in-process interceptor ring buffer. To change verbosity on service processes, use nodeagent_control_service to restart them with DEBUG=1 env var.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"level": {Type: "string", Description: "Log level to set", Enum: []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR"}},
			},
			Required: []string{"level"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		level := strings.ToUpper(getStr(args, "level"))
		if level == "" {
			return nil, fmt.Errorf("level is required")
		}
		if err := interceptors.SetInterceptorLogLevel(level); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"ok":            true,
			"level":         level,
			"message":       fmt.Sprintf("interceptor log level set to %s", level),
			"ring_capacity": interceptors.GetLogRing().Count(),
		}, nil
	})

	// ── get_log_verbosity ───────────────────────────────────────────────────
	s.register(toolDef{
		Name: "get_log_verbosity",
		Description: `Returns the current interceptor log verbosity level and ring buffer stats.`,
		InputSchema: inputSchema{
			Type:       "object",
			Properties: map[string]propSchema{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		return map[string]interface{}{
			"level":      interceptors.GetInterceptorLogLevel(),
			"ring_count": interceptors.GetLogRing().Count(),
		}, nil
	})

	// ── query_log_ring ──────────────────────────────────────────────────────
	s.register(toolDef{
		Name: "query_log_ring",
		Description: `Query the in-memory interceptor log ring buffer. Returns structured log entries from gRPC request processing — faster than journalctl and with richer fields.

The ring holds up to 10,000 entries. Entries are returned newest-first.

Filter by service, method, pattern, severity, subject, or time range. All filters are optional — omit for latest entries.`,
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"level":   {Type: "string", Description: "Minimum log level filter", Enum: []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR"}},
				"service": {Type: "string", Description: "Service name substring (e.g. 'dns', 'rbac', 'gateway')"},
				"method":  {Type: "string", Description: "Method substring (e.g. 'CreateZone', 'GetAccount')"},
				"pattern": {Type: "string", Description: "Message substring (e.g. 'denied', 'timeout', 'tls')"},
				"subject": {Type: "string", Description: "Exact caller identity match (e.g. 'sa', 'dave')"},
				"since":   {Type: "string", Description: "Start of time range (e.g. '5m', '1h', '30s')"},
				"limit":   {Type: "number", Description: "Max results (default 50, max 500)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		filter := interceptors.LogFilter{
			Level:   strings.ToUpper(getStr(args, "level")),
			Service: getStr(args, "service"),
			Method:  getStr(args, "method"),
			Pattern: getStr(args, "pattern"),
			Subject: getStr(args, "subject"),
			Limit:   getInt(args, "limit", 50),
		}

		if filter.Limit > 500 {
			filter.Limit = 500
		}

		// Parse since as a duration
		if since := getStr(args, "since"); since != "" {
			if d, err := time.ParseDuration(since); err == nil {
				filter.Since = time.Now().UTC().Add(-d)
			}
		}

		entries := interceptors.GetLogRing().Query(filter)

		results := make([]map[string]interface{}, 0, len(entries))
		for _, e := range entries {
			entry := map[string]interface{}{
				"timestamp":   e.Timestamp.Format(time.RFC3339Nano),
				"level":       e.Level,
				"service":     e.Service,
				"method":      e.Method,
				"subject":     e.Subject,
				"remote_addr": e.RemoteAddr,
				"duration_ms": e.DurationMs,
				"status_code": e.StatusCode,
				"message":     e.Message,
			}
			if len(e.Fields) > 0 {
				entry["fields"] = e.Fields
			}
			results = append(results, entry)
		}

		return map[string]interface{}{
			"count":      len(results),
			"ring_total": interceptors.GetLogRing().Count(),
			"level":      interceptors.GetInterceptorLogLevel(),
			"entries":    results,
		}, nil
	})
}
