package domain

import (
	"testing"
)

// TestDomainSpecKey verifies the spec key formatting.
func TestDomainSpecKey(t *testing.T) {
	tests := []struct {
		fqdn     string
		expected string
	}{
		{"test.example.com", "/globular/domains/v1/test.example.com"},
		{"api.example.com", "/globular/domains/v1/api.example.com"},
	}

	for _, tt := range tests {
		got := DomainSpecKey(tt.fqdn)
		if got != tt.expected {
			t.Errorf("DomainSpecKey(%q) = %q, want %q", tt.fqdn, got, tt.expected)
		}
	}
}

// TestDomainStatusKey verifies the status key formatting.
func TestDomainStatusKey(t *testing.T) {
	tests := []struct {
		fqdn     string
		expected string
	}{
		{"test.example.com", "/globular/domains/v1/test.example.com/status"},
		{"api.example.com", "/globular/domains/v1/api.example.com/status"},
	}

	for _, tt := range tests {
		got := DomainStatusKey(tt.fqdn)
		if got != tt.expected {
			t.Errorf("DomainStatusKey(%q) = %q, want %q", tt.fqdn, got, tt.expected)
		}
	}
}

// TestProviderConfigKey verifies the provider config key formatting.
func TestProviderConfigKey(t *testing.T) {
	tests := []struct {
		ref      string
		expected string
	}{
		{"godaddy-prod", "/globular/dns/providers/godaddy-prod"},
		{"route53-dev", "/globular/dns/providers/route53-dev"},
	}

	for _, tt := range tests {
		got := ProviderConfigKey(tt.ref)
		if got != tt.expected {
			t.Errorf("ProviderConfigKey(%q) = %q, want %q", tt.ref, got, tt.expected)
		}
	}
}

// TestDomainStore_KeySeparation verifies that spec and status keys are distinct.
func TestDomainStore_KeySeparation(t *testing.T) {
	fqdn := "test.example.com"

	specKey := DomainSpecKey(fqdn)
	statusKey := DomainStatusKey(fqdn)

	if specKey == statusKey {
		t.Errorf("spec and status keys must be different: %q", specKey)
	}

	if statusKey != specKey+"/status" {
		t.Errorf("status key must be spec key + /status, got %q", statusKey)
	}
}

// NOTE: Integration tests that require a running etcd instance should be written separately
// and run with a build tag (e.g., //go:build integration).
//
// Example integration test structure:
//
// //go:build integration
// func TestDomainStore_SpecStatusSeparation_Integration(t *testing.T) {
//     // Setup etcd client (requires ETCD_ENDPOINTS env var)
//     // Test that spec and status are stored separately
//     // Test that concurrent updates don't overwrite each other
//     // Test CAS operations
// }
//
// Run integration tests with: go test -tags=integration
