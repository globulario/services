package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// auditWriter is the destination for audit log entries.
// Defaults to stderr; overridden by initAuditLog if a file path is configured.
var auditWriter io.Writer = os.Stderr

func initAuditLog(cfg *MCPConfig) {
	if cfg.AuditLogPath != "" && cfg.AuditLogPath != "stderr" {
		f, err := os.OpenFile(cfg.AuditLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
		if err != nil {
			log.Printf("mcp: failed to open audit log %s: %v (using stderr)", cfg.AuditLogPath, err)
			return
		}
		auditWriter = f
	}
}

type auditEntry struct {
	Timestamp  string `json:"ts"`
	Tool       string `json:"tool"`
	Group      string `json:"group"`
	Caller     string `json:"caller,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Success    bool   `json:"success"`
	ErrorClass string `json:"error_class,omitempty"`
	ArgSummary string `json:"args,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"`
}

// callerKey is a context key for the authenticated caller identity.
type callerKeyType struct{}

var callerKey = callerKeyType{}

func toolGroup(toolName string) string {
	parts := strings.SplitN(toolName, "_", 2)
	if len(parts) == 0 {
		return "unknown"
	}
	switch parts[0] {
	case "cluster":
		return "cluster"
	case "nodeagent":
		return "nodeagent"
	case "repository":
		return "repository"
	case "backup":
		return "backup"
	case "rbac":
		return "rbac"
	case "resource":
		return "resource"
	case "file":
		return "file"
	case "deploy":
		return "file"
	case "db":
		return "persistence"
	case "kv":
		return "storage"
	case "auth":
		return "auth"
	case "dns":
		return "dns"
	default:
		return "composed"
	}
}

func auditLog(ctx context.Context, toolName string, args map[string]interface{}, start time.Time, err error) {
	caller := "anonymous"
	if v, ok := ctx.Value(callerKey).(string); ok && v != "" {
		caller = v
	}
	entry := auditEntry{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Tool:       toolName,
		Group:      toolGroup(toolName),
		Caller:     caller,
		DurationMs: time.Since(start).Milliseconds(),
		Success:    err == nil,
	}

	if err != nil {
		entry.ErrorClass = classifyError(err)
	}

	// Redact sensitive args before logging.
	entry.ArgSummary = redactArgSummary(args)

	data, _ := json.Marshal(entry)
	data = append(data, '\n')
	auditWriter.Write(data)
}

func classifyError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "path_not_allowed"):
		return "path_not_allowed"
	case strings.Contains(msg, "collection_not_allowed"):
		return "collection_not_allowed"
	case strings.Contains(msg, "key_prefix_not_allowed"):
		return "key_prefix_not_allowed"
	case strings.Contains(msg, "invalid_input"):
		return "invalid_input"
	case strings.Contains(msg, "NotFound"):
		return "not_found"
	case strings.Contains(msg, "PermissionDenied"):
		return "access_denied"
	case strings.Contains(msg, "Unavailable"):
		return "backend_unavailable"
	case strings.Contains(msg, "DeadlineExceeded"):
		return "timeout"
	default:
		return "internal_error"
	}
}

func redactArgSummary(args map[string]interface{}) string {
	if len(args) == 0 {
		return "{}"
	}
	safe := make(map[string]interface{}, len(args))
	for k, v := range args {
		if sensitiveFields[k] || strings.Contains(strings.ToLower(k), "password") || strings.Contains(strings.ToLower(k), "token") || strings.Contains(strings.ToLower(k), "secret") {
			safe[k] = "***"
		} else if s, ok := v.(string); ok && len(s) > 100 {
			safe[k] = s[:100] + "..."
		} else {
			safe[k] = v
		}
	}
	data, _ := json.Marshal(safe)
	if len(data) > 500 {
		return string(data[:500]) + "..."
	}
	return string(data)
}
