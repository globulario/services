package domain

import (
	"testing"
)

// TestExtractRelativeName_ApexDomain verifies that apex domains return empty string, not "@"
// This ensures ACME challenge records are created correctly without "@" in the name.
//
// Requirement INV-DNS-EXT-1: DNS-01 challenge for apex domain must create:
//   _acme-challenge.example.com
// NOT:
//   _acme-challenge.@.example.com
func TestExtractRelativeName_ApexDomain(t *testing.T) {
	tests := []struct {
		name     string
		zone     string
		domain   string
		expected string
	}{
		{
			name:     "apex domain returns empty string",
			zone:     "globular.cloud",
			domain:   "globular.cloud",
			expected: "", // Empty string, NOT "@"
		},
		{
			name:     "wildcard domain (as passed by lego) also treated as apex",
			zone:     "globular.cloud",
			domain:   "globular.cloud", // lego strips the * before calling Present()
			expected: "",
		},
		{
			name:     "subdomain returns relative name",
			zone:     "globular.cloud",
			domain:   "api.globular.cloud",
			expected: "api",
		},
		{
			name:     "nested subdomain returns relative name",
			zone:     "example.com",
			domain:   "api.staging.example.com",
			expected: "api.staging",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a minimal solver just to test extractRelativeName
			solver := &DNS01Solver{zone: tt.zone}

			result := solver.extractRelativeName(tt.domain)

			if result != tt.expected {
				t.Errorf("extractRelativeName(%q) in zone %q = %q, want %q",
					tt.domain, tt.zone, result, tt.expected)
			}
		})
	}
}

// TestACMEChallengeRecordName verifies the complete challenge record name generation
// This is the critical test that proves no "@" appears in the final record name.
func TestACMEChallengeRecordName(t *testing.T) {
	tests := []struct {
		name             string
		zone             string
		domain           string
		expectedRecord   string
		expectedFQDN     string
	}{
		{
			name:           "apex domain challenge",
			zone:           "globular.cloud",
			domain:         "globular.cloud",
			expectedRecord: "_acme-challenge", // Relative name for provider
			expectedFQDN:   "_acme-challenge.globular.cloud",
		},
		{
			name:           "subdomain challenge",
			zone:           "example.com",
			domain:         "api.example.com",
			expectedRecord: "_acme-challenge.api",
			expectedFQDN:   "_acme-challenge.api.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			solver := &DNS01Solver{zone: tt.zone}

			// Simulate what Present() does
			relativeName := solver.extractRelativeName(tt.domain)
			challengeName := "_acme-challenge"
			if relativeName != "" {
				challengeName = "_acme-challenge." + relativeName
			}

			if challengeName != tt.expectedRecord {
				t.Errorf("Challenge record name = %q, want %q", challengeName, tt.expectedRecord)
			}

			// Verify no "@" anywhere in the record name
			if containsAt(challengeName) {
				t.Errorf("Challenge record name %q contains '@' - this will be rejected by Cloudflare!", challengeName)
			}

			// Build FQDN for verification
			fqdn := challengeName + "." + tt.zone
			if fqdn != tt.expectedFQDN {
				t.Errorf("Challenge FQDN = %q, want %q", fqdn, tt.expectedFQDN)
			}
		})
	}
}

// containsAt checks if a string contains the "@" character
func containsAt(s string) bool {
	for _, r := range s {
		if r == '@' {
			return true
		}
	}
	return false
}
