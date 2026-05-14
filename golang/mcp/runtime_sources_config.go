package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/fsutil"
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
	// Split exists vs readable so a permissions/ownership problem (running
	// the check as a non-globular user against a 0640 globular:globular CA)
	// produces a remediation that says "fix permissions / run as service
	// user" instead of "reissue the CA". See composed-path failure log,
	// 2026-05-14 — PKI fileReadable conflates missing with not-readable.
	caExists, caReadable := fsutil.ObserveFile(caPath)
	credentialsPresent := cfg.Insecure || caReadable

	type sourceDef struct {
		name           string
		addr           string
		etcdResolvable bool // true if this source can be resolved from etcd when addr is empty
	}
	sourceDefs := []sourceDef{
		{"controller", cfg.ControllerAddr, true},
		{"doctor", cfg.DoctorAddr, true},
		{"workflow", cfg.WorkflowAddr, true},
		{"prometheus", cfg.PrometheusAddr, false},
	}

	var sources []sourceActivationStatus
	var missingConfig []string
	configuredCount := 0

	for _, src := range sourceDefs {
		configured := src.addr != ""
		if configured {
			configuredCount++
		} else if !src.etcdResolvable {
			// Only count truly unconfigured sources as missing.
			// etcd-resolvable sources (controller/doctor/workflow) are resolved at
			// bridge construction time — they are not missing, just not static.
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
				status.LastError = mtlsCredentialError(caPath, caExists)
			}
			if checkConn && (cfg.Insecure || credentialsPresent) {
				connStatus, connErr := dialCheck(src.addr, 2*time.Second)
				status.Connectivity = connStatus
				if connErr != "" {
					status.LastError = connErr
				}
			}
		} else if src.etcdResolvable {
			// Show the operator that this source is reached via etcd, not missing.
			status.Address = "etcd"
			status.Transport = "etcd_resolved"
		}
		sources = append(sources, status)
	}

	if !cfg.Insecure && !caReadable {
		missingConfig = append(missingConfig, mtlsMissingConfigEntry(caPath, caExists))
	}

	overallStatus := computeRuntimeStatus(configuredCount, len(sourceDefs), missingConfig, cfg.Insecure)

	confidence := "high"
	if configuredCount == 0 {
		confidence = "low"
	} else if configuredCount < len(sourceDefs) {
		confidence = "medium"
	}

	// gRPC sources without static addresses are resolved from etcd at bridge construction time.
	etcdResolution := "not_needed"
	if cfg.ControllerAddr == "" || cfg.DoctorAddr == "" || cfg.WorkflowAddr == "" {
		etcdResolution = "active"
	}

	return &runtimeActivationResult{
		RuntimeAwarenessStatus: overallStatus,
		Sources:                sources,
		MissingConfig:          missingConfig,
		RecommendedConfig:      buildRecommendedConfig(),
		Confidence:             confidence,
		EtcdResolution:         etcdResolution,
	}
}

// mtlsCredentialError formats the per-source LastError for a CA that the
// current process can't read. "missing" means reissuance; "unreadable"
// means ownership/permissions or running-as-wrong-user — the remediations
// are different, and a single conflated message has historically caused
// the wrong remediation to be applied.
func mtlsCredentialError(caPath string, exists bool) string {
	if exists {
		return fmt.Sprintf("mTLS CA at %s exists but is not readable by this process — check ownership/permissions or run as the service user", caPath)
	}
	return fmt.Sprintf("mTLS CA not found at %s — reissue from the cluster authority", caPath)
}

// mtlsMissingConfigEntry formats the entry appended to MissingConfig.
// Same exists-vs-readable split as mtlsCredentialError.
func mtlsMissingConfigEntry(caPath string, exists bool) string {
	if exists {
		return fmt.Sprintf("CACert (present at %s but not readable by this process — check ownership/permissions)", caPath)
	}
	return fmt.Sprintf("CACert (not found: %s)", caPath)
}
