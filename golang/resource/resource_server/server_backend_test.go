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
	if s.Address == "" {
		t.Fatalf("Address should be defaulted, got empty")
	}
	if s.Backend_address == "" {
		t.Fatalf("Backend_address should not be empty when scylla detected")
	}
	if s.Backend_address != "localhost" {
		t.Fatalf("expected Backend_address localhost, got %s", s.Backend_address)
	}
}
