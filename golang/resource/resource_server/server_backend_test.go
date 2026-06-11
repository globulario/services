package main

import (
	"testing"
)

func TestComputeBackendConfigScyllaWithEmptyAddress(t *testing.T) {
	s := &server{Port: 12345, TLS: false}

	computeBackendConfig(s, true)

	if s.Backend_type != "SCYLLA" {
		t.Fatalf("expected Backend_type SCYLLA, got %s", s.Backend_type)
	}
	// Address is intentionally left empty when not configured — the
	// service must not fall back to "localhost" (HARD RULE). The empty
	// address will fail explicitly at connection time.
	if s.Address != "" {
		t.Fatalf("Address should remain empty (no localhost fallback), got %s", s.Address)
	}
	if s.Backend_address == "" {
		t.Fatalf("Backend_address should not be empty when scylla detected")
	}
}
