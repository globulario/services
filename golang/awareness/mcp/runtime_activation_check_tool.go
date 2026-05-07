package mcp

import (
	"context"
	"fmt"
	"net"
	"os"
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
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func registerRuntimeActivationCheckTool(s *Server) {
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

		cfg := s.cfg

		// Determine transport.
		transport := "mtls"
		if cfg.Insecure {
			transport = "insecure"
		}

		// Check credentials readability.
		credentialsPresent := false
		if cfg.Insecure {
			credentialsPresent = true // insecure mode doesn't need certs
		} else if cfg.CACert != "" {
			caOK := fileReadable(cfg.CACert)
			if cfg.ClientCert != "" && cfg.ClientKey != "" {
				credentialsPresent = caOK && fileReadable(cfg.ClientCert) && fileReadable(cfg.ClientKey)
			} else {
				credentialsPresent = caOK
			}
		}

		// Define sources.
		type sourceDef struct {
			name string
			addr string
		}
		sourceDefs := []sourceDef{
			{"controller", cfg.ControllerAddr},
			{"doctor", cfg.DoctorAddr},
			{"workflow", cfg.WorkflowAddr},
			{"prometheus", cfg.PrometheusAddr},
		}

		var sources []sourceActivationStatus
		var missingConfig []string
		configuredCount := 0

		for _, src := range sourceDefs {
			configured := src.addr != ""
			if configured {
				configuredCount++
			} else {
				missingConfig = append(missingConfig, addrFieldName(src.name))
			}

			status := sourceActivationStatus{
				Source:             src.name,
				Configured:         configured,
				Connectivity:       "not_checked",
				CredentialsPresent: credentialsPresent || !configured,
			}
			if configured {
				status.Address = src.addr
				status.Transport = transport
				if !cfg.Insecure && checkCreds && !credentialsPresent {
					status.CredentialsPresent = false
					status.LastError = "mTLS credentials missing or unreadable"
				}
				if checkConn && status.CredentialsPresent {
					connStatus, connErr := dialCheck(src.addr, 2*time.Second)
					status.Connectivity = connStatus
					status.LastError = connErr
				}
			}
			sources = append(sources, status)
		}

		// Also flag missing TLS files explicitly.
		if !cfg.Insecure {
			if cfg.CACert == "" {
				missingConfig = append(missingConfig, "CACert")
			} else if !fileReadable(cfg.CACert) {
				missingConfig = append(missingConfig, fmt.Sprintf("CACert (unreadable: %s)", cfg.CACert))
			}
		}

		// Compute overall status.
		overallStatus := computeRuntimeStatus(configuredCount, len(sourceDefs), missingConfig, cfg.Insecure)

		// Build recommended config hint.
		recommended := buildRecommendedConfig(cfg)

		// Confidence.
		confidence := "high"
		if configuredCount == 0 {
			confidence = "low"
		} else if configuredCount < len(sourceDefs) {
			confidence = "medium"
		}

		return &runtimeActivationResult{
			RuntimeAwarenessStatus: overallStatus,
			Sources:                sources,
			MissingConfig:          missingConfig,
			RecommendedConfig:      recommended,
			Confidence:             confidence,
		}, nil
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func fileReadable(path string) bool {
	if path == "" {
		return false
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

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
	if configured == 0 {
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
	if configured < total {
		return "partial"
	}
	return "live"
}

func buildRecommendedConfig(cfg Config) map[string]string {
	hint := map[string]string{
		"ControllerAddr": "globular.internal:12000",
		"DoctorAddr":     "globular.internal:12005",
		"WorkflowAddr":   "globular.internal:10004",
		"PrometheusAddr": "http://globular.internal:9090",
	}
	// Replace hints with current values where configured.
	if cfg.ControllerAddr != "" {
		hint["ControllerAddr"] = cfg.ControllerAddr
	}
	if cfg.DoctorAddr != "" {
		hint["DoctorAddr"] = cfg.DoctorAddr
	}
	if cfg.WorkflowAddr != "" {
		hint["WorkflowAddr"] = cfg.WorkflowAddr
	}
	if cfg.PrometheusAddr != "" {
		hint["PrometheusAddr"] = cfg.PrometheusAddr
	}
	if !cfg.Insecure {
		caHint := cfg.CACert
		if caHint == "" {
			caHint = "/var/lib/globular/pki/ca.crt"
		}
		hint["CACert"] = caHint
		if cfg.ClientCert != "" {
			hint["ClientCert"] = cfg.ClientCert
		}
		if cfg.ClientKey != "" {
			hint["ClientKey"] = cfg.ClientKey
		}
	} else {
		hint["Insecure"] = "true (dev/test only)"
	}
	return hint
}
