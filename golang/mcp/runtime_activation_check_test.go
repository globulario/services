package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRuntimeActivationCheck_AllNoop verifies that zero configured addresses
// produces "noop" status.
func TestRuntimeActivationCheck_AllNoop(t *testing.T) {
	status := computeRuntimeStatus(0, 4, nil, true)
	if status != "noop" {
		t.Errorf("expected noop, got %q", status)
	}
}

// TestRuntimeActivationCheck_PartialConfig verifies that some but not all
// addresses configured produces "partial" status.
func TestRuntimeActivationCheck_PartialConfig(t *testing.T) {
	status := computeRuntimeStatus(1, 4, nil, true)
	if status != "partial" {
		t.Errorf("expected partial, got %q", status)
	}
}

// TestRuntimeActivationCheck_MissingTLSFiles verifies that missing TLS files
// produces "misconfigured" status.
func TestRuntimeActivationCheck_MissingTLSFiles(t *testing.T) {
	missingConfig := []string{"CACert (unreadable: /nonexistent/ca.crt)"}
	status := computeRuntimeStatus(4, 4, missingConfig, false)
	if status != "misconfigured" {
		t.Errorf("expected misconfigured, got %q", status)
	}
}

// TestRuntimeActivationCheck_InsecureDevModeExplicit verifies that insecure
// mode with all addresses configured produces "live" status.
func TestRuntimeActivationCheck_InsecureDevModeExplicit(t *testing.T) {
	status := computeRuntimeStatus(4, 4, nil, true)
	if status != "live" {
		t.Errorf("expected live, got %q", status)
	}
}

// TestLoadRuntimeSourcesConfig_MissingFile returns empty config when the file
// does not exist — noop, never panics.
func TestLoadRuntimeSourcesConfig_MissingFile(t *testing.T) {
	cfg := loadRuntimeSourcesConfig(t.TempDir())
	if cfg.ControllerAddr != "" || cfg.DoctorAddr != "" {
		t.Error("expected empty config when file is missing")
	}
}

// TestLoadRuntimeSourcesConfig_ParsesAddresses verifies that addresses written
// to runtime_sources.yaml are read back correctly.
func TestLoadRuntimeSourcesConfig_ParsesAddresses(t *testing.T) {
	dir := t.TempDir()
	awarenessDir := filepath.Join(dir, ".awareness")
	if err := os.MkdirAll(awarenessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := `controller_addr: "globular.internal:12000"
doctor_addr: "globular.internal:12005"
workflow_addr: "globular.internal:10004"
prometheus_addr: "http://globular.internal:9090"
insecure: true
`
	if err := os.WriteFile(filepath.Join(awarenessDir, "runtime_sources.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := loadRuntimeSourcesConfig(dir)
	if cfg.ControllerAddr != "globular.internal:12000" {
		t.Errorf("ControllerAddr: got %q", cfg.ControllerAddr)
	}
	if cfg.DoctorAddr != "globular.internal:12005" {
		t.Errorf("DoctorAddr: got %q", cfg.DoctorAddr)
	}
	if !cfg.Insecure {
		t.Error("expected Insecure=true")
	}
}

// TestEvaluateRuntimeActivation_AllAddressesInsecure verifies that a fully
// configured insecure config produces "live" status with all sources configured.
func TestEvaluateRuntimeActivation_AllAddressesInsecure(t *testing.T) {
	cfg := &runtimeSourcesConfig{
		ControllerAddr: "globular.internal:12000",
		DoctorAddr:     "globular.internal:12005",
		WorkflowAddr:   "globular.internal:10004",
		PrometheusAddr: "http://globular.internal:9090",
		Insecure:       true,
	}
	result := evaluateRuntimeActivation(cfg, false, false)
	if result.RuntimeAwarenessStatus != "live" {
		t.Errorf("expected live, got %q", result.RuntimeAwarenessStatus)
	}
	if result.Confidence != "high" {
		t.Errorf("expected high confidence, got %q", result.Confidence)
	}
	configured := 0
	for _, src := range result.Sources {
		if src.Configured {
			configured++
		}
	}
	if configured != 4 {
		t.Errorf("expected 4 configured sources, got %d", configured)
	}
}

// TestEvaluateRuntimeActivation_Noop verifies empty config → noop, never panics.
func TestEvaluateRuntimeActivation_Noop(t *testing.T) {
	cfg := &runtimeSourcesConfig{}
	result := evaluateRuntimeActivation(cfg, false, false)
	if result.RuntimeAwarenessStatus != "noop" {
		t.Errorf("expected noop, got %q", result.RuntimeAwarenessStatus)
	}
	if result.Confidence != "low" {
		t.Errorf("expected low confidence for noop, got %q", result.Confidence)
	}
}

// TestBuildRuntimeSection_ReadsConfig verifies that buildRuntimeSection reads
// .awareness/runtime_sources.yaml rather than hardcoding configured=0.
func TestBuildRuntimeSection_ReadsConfig(t *testing.T) {
	dir := t.TempDir()
	awarenessDir := filepath.Join(dir, ".awareness")
	if err := os.MkdirAll(awarenessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := `controller_addr: "globular.internal:12000"
doctor_addr: "globular.internal:12005"
workflow_addr: "globular.internal:10004"
prometheus_addr: "http://globular.internal:9090"
insecure: true
`
	if err := os.WriteFile(filepath.Join(awarenessDir, "runtime_sources.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	section, _ := buildRuntimeSection(dir)
	if section.ConfiguredSources != 4 {
		t.Errorf("expected ConfiguredSources=4 from config file, got %d", section.ConfiguredSources)
	}
	if section.RuntimeAwarenessStatus != "live" {
		t.Errorf("expected live, got %q", section.RuntimeAwarenessStatus)
	}
}

// TestBuildRuntimeSection_NoConfigIsNoop verifies that missing config → noop warning.
func TestBuildRuntimeSection_NoConfigIsNoop(t *testing.T) {
	section, alerts := buildRuntimeSection(t.TempDir())
	if section.RuntimeAwarenessStatus != "noop" {
		t.Errorf("expected noop without config, got %q", section.RuntimeAwarenessStatus)
	}
	found := false
	for _, a := range alerts {
		if a.ID == "runtime.noop" {
			found = true
		}
	}
	if !found {
		t.Error("expected runtime.noop alert when no config present")
	}
}
