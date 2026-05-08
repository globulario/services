package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRuntimeActivationCheck_AllNoop verifies that zero configured addresses
// AND a truly-missing source (non-etcd-resolvable with no addr) produces "noop".
// etcd-resolvable sources are never added to missingConfig, so noop requires a
// genuinely absent non-etcd-resolvable source (e.g. PrometheusAddr).
func TestRuntimeActivationCheck_AllNoop(t *testing.T) {
	status := computeRuntimeStatus(0, 4, []string{"PrometheusAddr"}, false)
	if status != "noop" {
		t.Errorf("expected noop, got %q", status)
	}
}

// TestRuntimeActivationCheck_EtcdResolvedWithStaticPrometheusIsLive verifies
// that 1 statically configured source (prometheus) + 3 etcd-resolved sources
// (controller/doctor/workflow) with no missing config → "live", not "partial".
// This is the standard production scenario for a Globular cluster.
func TestRuntimeActivationCheck_EtcdResolvedWithStaticPrometheusIsLive(t *testing.T) {
	// missingConfig is nil: etcd-resolvable sources are never added to it.
	status := computeRuntimeStatus(1, 4, nil, true)
	if status != "live" {
		t.Errorf("expected live (etcd-resolved sources are functional), got %q", status)
	}
}

// TestRuntimeActivationCheck_PartialConfig verifies that a genuinely missing
// non-etcd-resolvable source alongside a configured source → "partial".
func TestRuntimeActivationCheck_PartialConfig(t *testing.T) {
	// configured=1, but missingConfig has a non-cred entry → partial.
	status := computeRuntimeStatus(1, 5, []string{"OtherMetricsAddr"}, false)
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

// TestEvaluateRuntimeActivation_EtcdResolvedSources verifies that controller/doctor/workflow
// show as etcd_resolved in the source list when their addr is empty — they are not missing,
// just not statically configured. Prometheus with empty addr remains unconfigured.
func TestEvaluateRuntimeActivation_EtcdResolvedSources(t *testing.T) {
	cfg := &runtimeSourcesConfig{
		PrometheusAddr: "http://globular.internal:9090",
		Insecure:       true,
	}
	result := evaluateRuntimeActivation(cfg, false, false)

	for _, src := range result.Sources {
		switch src.Source {
		case "controller", "doctor", "workflow":
			if src.Transport != "etcd_resolved" {
				t.Errorf("source %q: expected transport=etcd_resolved, got %q", src.Source, src.Transport)
			}
			if src.Address != "etcd" {
				t.Errorf("source %q: expected address=etcd, got %q", src.Source, src.Address)
			}
		case "prometheus":
			if src.Transport == "etcd_resolved" {
				t.Errorf("prometheus should not be etcd_resolved")
			}
		}
	}
}

// TestBuildRuntimeSection_EtcdResolvedIsLiveNotPartial verifies that the standard
// production configuration — prometheus statically configured, controller/doctor/workflow
// resolved from etcd — produces "live" status with no runtime.partial alert.
// This was the false-positive this fix addresses.
func TestBuildRuntimeSection_EtcdResolvedIsLiveNotPartial(t *testing.T) {
	dir := t.TempDir()
	awarenessDir := filepath.Join(dir, ".awareness")
	if err := os.MkdirAll(awarenessDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := `prometheus_addr: "http://globular.internal:9090"
insecure: true
`
	if err := os.WriteFile(filepath.Join(awarenessDir, "runtime_sources.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	section, alerts := buildRuntimeSection(dir)
	if section.RuntimeAwarenessStatus != "live" {
		t.Errorf("expected live (etcd-resolved sources are functional), got %q", section.RuntimeAwarenessStatus)
	}
	for _, a := range alerts {
		if a.ID == "runtime.partial" {
			t.Errorf("expected no runtime.partial alert, got: %s", a.Message)
		}
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
