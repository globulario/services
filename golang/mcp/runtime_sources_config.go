package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// runtimeSourcesConfig holds static addresses for the Globular runtime bridge sources.
// Loaded from .awareness/runtime_sources.yaml under the repo root.
// When all addresses are empty, all sources are noop — never silently healthy.
type runtimeSourcesConfig struct {
	ControllerAddr string `yaml:"controller_addr"`
	DoctorAddr     string `yaml:"doctor_addr"`
	WorkflowAddr   string `yaml:"workflow_addr"`
	PrometheusAddr string `yaml:"prometheus_addr"`
	CACert         string `yaml:"ca_cert"`
	ClientCert     string `yaml:"client_cert"`
	ClientKey      string `yaml:"client_key"`
	// Insecure allows plaintext gRPC transport. Dev/test only — never set in production.
	Insecure bool `yaml:"insecure"`
}

// loadRuntimeSourcesConfig reads .awareness/runtime_sources.yaml under repoRoot.
// Returns an empty config (all noop) if the file does not exist or cannot be parsed.
// Noop is explicit: callers must treat zero configured sources as "no runtime awareness."
func loadRuntimeSourcesConfig(repoRoot string) *runtimeSourcesConfig {
	if repoRoot == "" {
		return &runtimeSourcesConfig{}
	}
	path := filepath.Join(repoRoot, ".awareness", "runtime_sources.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return &runtimeSourcesConfig{}
	}
	var cfg runtimeSourcesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &runtimeSourcesConfig{}
	}
	return &cfg
}

// evaluateRuntimeActivation is the single source of truth for runtime activation status.
// It is used by both runtime_activation_check and health_pulse so they never diverge.
func evaluateRuntimeActivation(cfg *runtimeSourcesConfig, checkCreds, checkConn bool) *runtimeActivationResult {
	transport := "mtls"
	if cfg.Insecure {
		transport = "insecure"
	}

	// Determine credential presence. Use the configured CA path, or the standard PKI path.
	caPath := cfg.CACert
	if caPath == "" {
		caPath = "/var/lib/globular/pki/ca.crt"
	}
	credentialsPresent := cfg.Insecure || fileReadable(caPath)

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
			CredentialsPresent: cfg.Insecure || !configured || credentialsPresent,
		}
		if configured {
			status.Address = src.addr
			status.Transport = transport
			if checkCreds && !cfg.Insecure && !credentialsPresent {
				status.CredentialsPresent = false
				status.LastError = "mTLS credentials missing or unreadable"
			}
			if checkConn && (cfg.Insecure || credentialsPresent) {
				connStatus, connErr := dialCheck(src.addr, 2*time.Second)
				status.Connectivity = connStatus
				if connErr != "" {
					status.LastError = connErr
				}
			}
		}
		sources = append(sources, status)
	}

	if !cfg.Insecure && !fileReadable(caPath) {
		missingConfig = append(missingConfig, fmt.Sprintf("CACert (not found: %s)", caPath))
	}

	overallStatus := computeRuntimeStatus(configuredCount, len(sourceDefs), missingConfig, cfg.Insecure)

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
		RecommendedConfig:      buildRecommendedConfig(),
		Confidence:             confidence,
	}
}
