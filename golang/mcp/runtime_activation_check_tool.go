package main

import (
	"context"
	"net"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Output types
// ---------------------------------------------------------------------------

type sourceActivationStatus struct {
	Source             string `json:"source"`
	Configured         bool   `json:"configured"`
	Address            string `json:"address,omitempty"`
	Transport          string `json:"transport,omitempty"`
	CredentialsPresent bool   `json:"credentials_present"`
	Connectivity       string `json:"connectivity"` // ok | failed | not_checked
	LastError          string `json:"last_error,omitempty"`
}

type runtimeActivationResult struct {
	RuntimeAwarenessStatus string                   `json:"runtime_awareness_status"` // live | partial | noop | misconfigured
	Sources                []sourceActivationStatus `json:"sources"`
	MissingConfig          []string                 `json:"missing_config"`
	RecommendedConfig      map[string]string        `json:"recommended_config"`
	Confidence             string                   `json:"confidence"`
	// EtcdResolution indicates that unconfigured gRPC sources (controller/doctor/workflow)
	// are resolved at runtime from etcd — no static address required.
	EtcdResolution string `json:"etcd_resolution"` // "active" | "not_available"
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func registerRuntimeActivationCheckTool(s *server, st *awarenessState) {
	s.register(toolDef{
		Name:        "awareness.runtime_activation_check",
		Description: "Check whether runtime awareness is actually live on a real cluster, or whether sources are noop because config is missing. Reports each source (controller, doctor, workflow, prometheus) with configured/transport/connectivity status and exact missing fields. Noop is never silent — this tool makes it explicit.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"check_connectivity": {
					Type:        "boolean",
					Description: "If true, attempt TCP dial to each configured address (2s timeout). Default: false.",
					Default:     false,
				},
				"check_credentials": {
					Type:        "boolean",
					Description: "If true, verify that configured TLS cert/key files are readable. Default: true.",
					Default:     true,
				},
				"check_source_health": {
					Type:        "boolean",
					Description: "If true, report which sources have non-noop health based on current config. Default: true.",
					Default:     true,
				},
			},
			Required: []string{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx

		checkConn := boolArg(args, "check_connectivity")
		checkCreds := true
		if v, ok := args["check_credentials"].(bool); ok {
			checkCreds = v
		}

		cfg := loadRuntimeSourcesConfig(st.repoRoot)
		return evaluateRuntimeActivation(cfg, checkCreds, checkConn), nil
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func addrFieldName(source string) string {
	switch source {
	case "controller":
		return "ControllerAddr"
	case "doctor":
		return "DoctorAddr"
	case "workflow":
		return "WorkflowAddr"
	case "prometheus":
		return "PrometheusAddr"
	}
	return strings.Title(source) + "Addr" //nolint:staticcheck
}

func dialCheck(addr string, timeout time.Duration) (status string, errStr string) {
	// Strip scheme for Prometheus-style "http://..." addresses.
	dialAddr := addr
	if strings.HasPrefix(dialAddr, "http://") {
		dialAddr = strings.TrimPrefix(dialAddr, "http://")
	} else if strings.HasPrefix(dialAddr, "https://") {
		dialAddr = strings.TrimPrefix(dialAddr, "https://")
	}
	// Ensure host:port format.
	if !strings.Contains(dialAddr, ":") {
		dialAddr += ":80"
	}
	conn, err := net.DialTimeout("tcp", dialAddr, timeout)
	if err != nil {
		return "failed", err.Error()
	}
	conn.Close()
	return "ok", ""
}

func computeRuntimeStatus(configured, total int, missingConfig []string, insecure bool) string {
	if configured == 0 && len(missingConfig) > 0 {
		// No statically configured sources AND at least one source truly missing
		// (etcd-resolvable sources are never added to missingConfig, so a non-empty
		// missingConfig means something like PrometheusAddr is absent).
		return "noop"
	}
	hasMissingCreds := false
	for _, m := range missingConfig {
		if strings.Contains(m, "Cert") || strings.Contains(m, "Key") || strings.Contains(m, "CA") {
			hasMissingCreds = true
			break
		}
	}
	if hasMissingCreds && !insecure {
		return "misconfigured"
	}
	// Use missingConfig rather than configured<total: etcd-resolvable sources
	// (controller, doctor, workflow) are intentionally absent from missingConfig
	// and should not trigger "partial" — they resolve addresses at query time.
	if len(missingConfig) > 0 {
		return "partial"
	}
	return "live"
}

func buildRecommendedConfig() map[string]string {
	return map[string]string{
		"note":           "In the Globular MCP server, cluster addresses are resolved from etcd — no static runtime config needed.",
		"ControllerAddr": "globular.internal:12000",
		"DoctorAddr":     "globular.internal:12005",
		"WorkflowAddr":   "globular.internal:10004",
		"PrometheusAddr": "http://globular.internal:9090",
		"CACert":         "/var/lib/globular/pki/ca.crt",
	}
}
