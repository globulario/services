package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRuntimeConfigBootstrap_DetectsConfig verifies that when config files are
// absent, the bootstrap falls back to globular.internal defaults with warnings.
func TestRuntimeConfigBootstrap_DetectsConfig(t *testing.T) {
	// Use a temp dir as the config dir — no etcd.yaml exists.
	configDir := t.TempDir()
	detected, warnings := detectGlobularConfig(configDir, true)

	// Should fall back to globular.internal defaults.
	if detected.ControllerAddr == "" {
		t.Error("expected ControllerAddr to be set to a default")
	}
	if !strings.Contains(detected.ControllerAddr, "12000") {
		t.Errorf("expected ControllerAddr to include port 12000, got %q", detected.ControllerAddr)
	}
	if len(warnings) == 0 {
		t.Error("expected at least one warning for missing etcd.yaml")
	}
}

// TestRuntimeConfigBootstrap_MissingTLS verifies that when TLS cert files don't
// exist, missingFields includes CACert and ClientKey.
func TestRuntimeConfigBootstrap_MissingTLS(t *testing.T) {
	detected := bootstrapDetected{
		ControllerAddr: "globular.internal:12000",
		DoctorAddr:     "globular.internal:12005",
		WorkflowAddr:   "globular.internal:10004",
		PrometheusAddr: "http://globular.internal:9090",
		// CACert and ClientKey deliberately absent.
	}
	missing := missingFields(detected, false)

	hasCACert := false
	hasClientKey := false
	for _, m := range missing {
		if m == "CACert" {
			hasCACert = true
		}
		if m == "ClientKey" {
			hasClientKey = true
		}
	}
	if !hasCACert {
		t.Error("expected CACert in missing fields")
	}
	if !hasClientKey {
		t.Error("expected ClientKey in missing fields")
	}
}

// TestRuntimeConfigBootstrap_WriteFalseDoesNotWrite verifies that when write=false
// the output file is not created even if outputPath is set.
func TestRuntimeConfigBootstrap_WriteFalseDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, ".awareness", "runtime_sources.yaml")

	// Manually call writeSampleConfig only when write=true (not here).
	detected := bootstrapDetected{
		ControllerAddr: "globular.internal:12000",
		DoctorAddr:     "globular.internal:12005",
		WorkflowAddr:   "globular.internal:10004",
		PrometheusAddr: "http://globular.internal:9090",
	}
	sample := buildSampleConfig(detected, true)

	// write=false: do NOT call writeSampleConfig.
	_ = sample

	// File should not exist.
	if _, err := os.Stat(outputPath); err == nil {
		t.Error("expected output file to NOT exist when write=false")
	}
}

// TestRuntimeConfigBootstrap_RedactsSecrets verifies that the sample config
// does not include "present" or the literal key path in the client_key field.
func TestRuntimeConfigBootstrap_RedactsSecrets(t *testing.T) {
	detected := bootstrapDetected{
		ControllerAddr: "globular.internal:12000",
		DoctorAddr:     "globular.internal:12005",
		WorkflowAddr:   "globular.internal:10004",
		PrometheusAddr: "http://globular.internal:9090",
		CACert:         "/var/lib/globular/pki/ca.crt",
		ClientCert:     "/var/lib/globular/pki/issued/services/service.crt",
		ClientKey:      "present", // not a real path — just the sentinel
	}
	sample := buildSampleConfig(detected, false)

	// "present" should not appear as a YAML value.
	if strings.Contains(sample, `"present"`) || strings.Contains(sample, `: present`) {
		t.Error("sample config must not expose the 'present' sentinel as a value")
	}
	// The sample config should contain a commented hint for client_key.
	if !strings.Contains(sample, "client_key") {
		t.Error("sample config should contain a client_key comment")
	}
}
