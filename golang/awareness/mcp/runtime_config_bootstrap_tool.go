package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Output types
// ---------------------------------------------------------------------------

type bootstrapDetected struct {
	ControllerAddr string `json:"controller_addr,omitempty"`
	DoctorAddr     string `json:"doctor_addr,omitempty"`
	WorkflowAddr   string `json:"workflow_addr,omitempty"`
	PrometheusAddr string `json:"prometheus_addr,omitempty"`
	CACert         string `json:"ca_cert,omitempty"`
	ClientCert     string `json:"client_cert,omitempty"`
	ClientKey      string `json:"client_key,omitempty"` // "present" or "missing" — never printed
}

type bootstrapResult struct {
	CanBootstrap bool              `json:"can_bootstrap"`
	Detected     bootstrapDetected `json:"detected"`
	Missing      []string          `json:"missing"`
	Warnings     []string          `json:"warnings"`
	SampleConfig string            `json:"sample_config,omitempty"`
	WrittenTo    string            `json:"written_to,omitempty"`
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

func registerRuntimeConfigBootstrapTool(s *Server) {
	s.register(toolDef{
		Name:        "awareness.runtime_config_bootstrap",
		Description: "Detect existing Globular config and generate a safe sample awareness runtime config. Discovers ControllerAddr, DoctorAddr, WorkflowAddr, PrometheusAddr and TLS material from the Globular config directory. Never prints private key contents. Writes sample config only when write=true is explicitly set.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"globular_config_dir": {
					Type:        "string",
					Description: "Path to the Globular config directory (default: /var/lib/globular/config). Used to auto-detect service addresses.",
				},
				"output_config_path": {
					Type:        "string",
					Description: "Path to write the sample config (default: .awareness/runtime_sources.yaml). Only written if write=true.",
				},
				"write": {
					Type:        "boolean",
					Description: "If true, write the sample config to output_config_path. Default: false (dry-run only).",
					Default:     false,
				},
				"insecure": {
					Type:        "boolean",
					Description: "If true, generate a sample config without TLS (dev/test only). Default: false.",
					Default:     false,
				},
			},
			Required: []string{},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		_ = ctx

		globularConfigDir := strArg(args, "globular_config_dir")
		if globularConfigDir == "" {
			globularConfigDir = "/var/lib/globular/config"
		}
		outputPath := strArg(args, "output_config_path")
		if outputPath == "" {
			outputPath = ".awareness/runtime_sources.yaml"
		}
		write := boolArg(args, "write")
		insecure := boolArg(args, "insecure")

		detected, warnings := detectGlobularConfig(globularConfigDir, insecure)
		missing := missingFields(detected, insecure)
		canBootstrap := len(missing) == 0

		sample := buildSampleConfig(detected, insecure)

		writtenTo := ""
		if write {
			if err := writeSampleConfig(outputPath, sample); err != nil {
				warnings = append(warnings, fmt.Sprintf("Failed to write config: %v", err))
			} else {
				writtenTo = outputPath
			}
		}

		return &bootstrapResult{
			CanBootstrap: canBootstrap,
			Detected:     detected,
			Missing:      missing,
			Warnings:     warnings,
			SampleConfig: sample,
			WrittenTo:    writtenTo,
		}, nil
	})
}

// ---------------------------------------------------------------------------
// Detection helpers
// ---------------------------------------------------------------------------

// etcdServiceEntry is a minimal representation of a Globular service record in etcd config YAML.
type etcdServiceEntry struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

func detectGlobularConfig(configDir string, insecure bool) (bootstrapDetected, []string) {
	var detected bootstrapDetected
	var warnings []string

	// Try to read etcd.yaml for service addresses.
	etcdYAML := filepath.Join(configDir, "etcd.yaml")
	if data, err := os.ReadFile(etcdYAML); err == nil {
		var etcdCfg map[string]interface{}
		if err := yaml.Unmarshal(data, &etcdCfg); err == nil {
			// etcd.yaml may contain endpoints list.
			if eps, ok := etcdCfg["initial-advertise-peer-urls"].(string); ok && eps != "" {
				// Derive globular.internal host from peer URL.
				host := extractHost(eps)
				if host != "" {
					detected.ControllerAddr = host + ":12000"
					detected.DoctorAddr = host + ":12005"
					detected.WorkflowAddr = host + ":10004"
					detected.PrometheusAddr = "http://" + host + ":9090"
				}
			}
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("etcd.yaml not found at %s — using globular.internal defaults", etcdYAML))
		detected.ControllerAddr = "globular.internal:12000"
		detected.DoctorAddr = "globular.internal:12005"
		detected.WorkflowAddr = "globular.internal:10004"
		detected.PrometheusAddr = "http://globular.internal:9090"
	}

	if !insecure {
		// Standard PKI paths.
		pkiDir := "/var/lib/globular/pki"
		caCert := filepath.Join(pkiDir, "ca.crt")
		clientCert := filepath.Join(pkiDir, "issued", "services", "service.crt")
		clientKey := filepath.Join(pkiDir, "issued", "services", "service.key")

		if fileReadable(caCert) {
			detected.CACert = caCert
		} else {
			warnings = append(warnings, fmt.Sprintf("CA cert not found at %s", caCert))
		}
		if fileReadable(clientCert) {
			detected.ClientCert = clientCert
		} else {
			warnings = append(warnings, fmt.Sprintf("Client cert not found at %s", clientCert))
		}
		if fileReadable(clientKey) {
			detected.ClientKey = "present" // never expose the path to the key in output
		} else {
			warnings = append(warnings, fmt.Sprintf("Client key not found at %s", clientKey))
			detected.ClientKey = "missing"
		}
	}

	return detected, warnings
}

func missingFields(d bootstrapDetected, insecure bool) []string {
	var missing []string
	if d.ControllerAddr == "" {
		missing = append(missing, "ControllerAddr")
	}
	if d.DoctorAddr == "" {
		missing = append(missing, "DoctorAddr")
	}
	if d.WorkflowAddr == "" {
		missing = append(missing, "WorkflowAddr")
	}
	if d.PrometheusAddr == "" {
		missing = append(missing, "PrometheusAddr")
	}
	if !insecure {
		if d.CACert == "" {
			missing = append(missing, "CACert")
		}
		if d.ClientKey == "" || d.ClientKey == "missing" {
			missing = append(missing, "ClientKey")
		}
	}
	return missing
}

func buildSampleConfig(d bootstrapDetected, insecure bool) string {
	var sb strings.Builder
	sb.WriteString("# Awareness runtime sources config — generated by awareness.runtime_config_bootstrap\n")
	sb.WriteString("# Review and adjust addresses before use.\n")
	sb.WriteString("# Never commit credentials to source control.\n")
	sb.WriteString("awareness:\n")
	sb.WriteString("  runtime_sources:\n")
	writeField := func(key, value string) {
		if value != "" && value != "present" && value != "missing" {
			sb.WriteString(fmt.Sprintf("    %s: %q\n", key, value))
		}
	}
	writeField("controller_addr", d.ControllerAddr)
	writeField("doctor_addr", d.DoctorAddr)
	writeField("workflow_addr", d.WorkflowAddr)
	writeField("prometheus_addr", d.PrometheusAddr)
	if insecure {
		sb.WriteString("    insecure: true  # dev/test only\n")
	} else {
		writeField("ca_cert", d.CACert)
		writeField("client_cert", d.ClientCert)
		sb.WriteString("    # client_key: /path/to/service.key  # set this manually\n")
	}
	return sb.String()
}

func writeSampleConfig(path, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// extractHost pulls the host from a URL like "https://10.0.0.63:2380".
func extractHost(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	if idx := strings.LastIndex(url, ":"); idx > 0 {
		return url[:idx]
	}
	return url
}
