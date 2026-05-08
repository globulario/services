package main

import (
	"testing"
)

// TestRuntimeActivationCheck_AllNoop verifies that a Config with no addresses
// produces "noop" status for all sources and overall.
func TestRuntimeActivationCheck_AllNoop(t *testing.T) {
	cfg := Config{Insecure: true} // no addresses set
	status := computeRuntimeStatus(0, 4, nil, cfg.Insecure)
	if status != "noop" {
		t.Errorf("expected noop, got %q", status)
	}
}

// TestRuntimeActivationCheck_PartialConfig verifies that a Config with some
// but not all addresses produces "partial" status.
func TestRuntimeActivationCheck_PartialConfig(t *testing.T) {
	cfg := Config{
		ControllerAddr: "globular.internal:12000",
		Insecure:       true,
	}
	status := computeRuntimeStatus(1, 4, nil, cfg.Insecure)
	if status != "partial" {
		t.Errorf("expected partial, got %q", status)
	}
}

// TestRuntimeActivationCheck_MissingTLSFiles verifies that a Config with
// addresses but missing TLS credentials produces "misconfigured".
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
	cfg := Config{
		ControllerAddr: "globular.internal:12000",
		DoctorAddr:     "globular.internal:12005",
		WorkflowAddr:   "globular.internal:10004",
		PrometheusAddr: "http://globular.internal:9090",
		Insecure:       true,
	}
	status := computeRuntimeStatus(4, 4, nil, cfg.Insecure)
	if status != "live" {
		t.Errorf("expected live, got %q", status)
	}
}
