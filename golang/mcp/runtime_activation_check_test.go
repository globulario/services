package main

import (
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
