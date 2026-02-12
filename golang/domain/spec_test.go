package domain

import (
	"encoding/json"
	"testing"
)

func TestExternalDomainSpec_Validate(t *testing.T) {
	tests := []struct {
		name    string
		spec    *ExternalDomainSpec
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid spec",
			spec: &ExternalDomainSpec{
				FQDN:        "test.example.com",
				Zone:        "example.com",
				NodeID:      "node-1",
				TargetIP:    "192.0.2.1",
				ProviderRef: "godaddy",
				TTL:         600,
			},
			wantErr: false,
		},
		{
			name: "valid with auto target IP",
			spec: &ExternalDomainSpec{
				FQDN:        "test.example.com",
				Zone:        "example.com",
				NodeID:      "node-1",
				TargetIP:    "auto",
				ProviderRef: "godaddy",
			},
			wantErr: false,
		},
		{
			name: "missing FQDN",
			spec: &ExternalDomainSpec{
				Zone:        "example.com",
				NodeID:      "node-1",
				TargetIP:    "auto",
				ProviderRef: "godaddy",
			},
			wantErr: true,
			errMsg:  "fqdn is required",
		},
		{
			name: "missing zone",
			spec: &ExternalDomainSpec{
				FQDN:        "test.example.com",
				NodeID:      "node-1",
				TargetIP:    "auto",
				ProviderRef: "godaddy",
			},
			wantErr: true,
			errMsg:  "zone is required",
		},
		{
			name: "FQDN not subdomain of zone",
			spec: &ExternalDomainSpec{
				FQDN:        "test.other.com",
				Zone:        "example.com",
				NodeID:      "node-1",
				TargetIP:    "auto",
				ProviderRef: "godaddy",
			},
			wantErr: true,
			errMsg:  "not a subdomain of zone",
		},
		{
			name: "missing node ID",
			spec: &ExternalDomainSpec{
				FQDN:        "test.example.com",
				Zone:        "example.com",
				TargetIP:    "auto",
				ProviderRef: "godaddy",
			},
			wantErr: true,
			errMsg:  "node_id is required",
		},
		{
			name: "missing target IP",
			spec: &ExternalDomainSpec{
				FQDN:        "test.example.com",
				Zone:        "example.com",
				NodeID:      "node-1",
				ProviderRef: "godaddy",
			},
			wantErr: true,
			errMsg:  "target_ip is required",
		},
		{
			name: "missing provider ref",
			spec: &ExternalDomainSpec{
				FQDN:     "test.example.com",
				Zone:     "example.com",
				NodeID:   "node-1",
				TargetIP: "auto",
			},
			wantErr: true,
			errMsg:  "provider_ref is required",
		},
		{
			name: "ACME enabled without email",
			spec: &ExternalDomainSpec{
				FQDN:        "test.example.com",
				Zone:        "example.com",
				NodeID:      "node-1",
				TargetIP:    "auto",
				ProviderRef: "godaddy",
				ACME: ACMEConfig{
					Enabled: true,
				},
			},
			wantErr: true,
			errMsg:  "acme.email is required",
		},
		{
			name: "ACME with invalid challenge type",
			spec: &ExternalDomainSpec{
				FQDN:        "test.example.com",
				Zone:        "example.com",
				NodeID:      "node-1",
				TargetIP:    "auto",
				ProviderRef: "godaddy",
				ACME: ACMEConfig{
					Enabled:       true,
					Email:         "admin@example.com",
					ChallengeType: "http-01",
				},
			},
			wantErr: true,
			errMsg:  "challenge_type must be \"dns-01\"",
		},
		{
			name: "ACME with valid config",
			spec: &ExternalDomainSpec{
				FQDN:        "test.example.com",
				Zone:        "example.com",
				NodeID:      "node-1",
				TargetIP:    "auto",
				ProviderRef: "godaddy",
				ACME: ACMEConfig{
					Enabled:       true,
					Email:         "admin@example.com",
					ChallengeType: "dns-01",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errMsg)
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Fatalf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check defaults were applied
			if tt.spec.TTL == 0 {
				t.Error("expected default TTL to be applied")
			}
			if tt.spec.ACME.Enabled && tt.spec.ACME.ChallengeType == "" {
				t.Error("expected default challenge type to be applied")
			}
		})
	}
}

func TestExternalDomainSpec_RelativeName(t *testing.T) {
	tests := []struct {
		spec     ExternalDomainSpec
		expected string
	}{
		{
			spec:     ExternalDomainSpec{FQDN: "test.example.com", Zone: "example.com"},
			expected: "test",
		},
		{
			spec:     ExternalDomainSpec{FQDN: "sub.test.example.com", Zone: "example.com"},
			expected: "sub.test",
		},
		{
			spec:     ExternalDomainSpec{FQDN: "example.com", Zone: "example.com"},
			expected: "@",
		},
	}

	for _, tt := range tests {
		t.Run(tt.spec.FQDN, func(t *testing.T) {
			result := tt.spec.RelativeName()
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestExternalDomainSpec_JSON(t *testing.T) {
	spec := &ExternalDomainSpec{
		FQDN:        "test.example.com",
		Zone:        "example.com",
		NodeID:      "node-1",
		TargetIP:    "192.0.2.1",
		ProviderRef: "godaddy",
		TTL:         600,
		ACME: ACMEConfig{
			Enabled:       true,
			Email:         "admin@example.com",
			ChallengeType: "dns-01",
		},
		Ingress: IngressConfig{
			Enabled: true,
			Service: "gateway",
			Port:    443,
		},
	}

	// Test serialization
	data, err := spec.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize spec: %v", err)
	}

	// Test deserialization
	parsed, err := FromJSON(data)
	if err != nil {
		t.Fatalf("failed to deserialize spec: %v", err)
	}

	// Verify fields
	if parsed.FQDN != spec.FQDN {
		t.Errorf("FQDN mismatch: expected %q, got %q", spec.FQDN, parsed.FQDN)
	}
	if parsed.Zone != spec.Zone {
		t.Errorf("Zone mismatch: expected %q, got %q", spec.Zone, parsed.Zone)
	}
	if parsed.ACME.Enabled != spec.ACME.Enabled {
		t.Error("ACME.Enabled mismatch")
	}
	if parsed.Ingress.Port != spec.Ingress.Port {
		t.Errorf("Ingress.Port mismatch: expected %d, got %d", spec.Ingress.Port, parsed.Ingress.Port)
	}
}

func TestDomainKey(t *testing.T) {
	fqdn := "test.example.com"
	expected := "/globular/domains/v1/test.example.com"

	result := DomainKey(fqdn)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestProviderKey(t *testing.T) {
	name := "my-godaddy"
	expected := "/globular/providers/v1/my-godaddy"

	result := ProviderKey(name)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestCondition_JSON(t *testing.T) {
	cond := Condition{
		Type:    "DNSRecordCreated",
		Status:  "True",
		Reason:  "RecordCreated",
		Message: "DNS A record created successfully",
	}

	data, err := json.Marshal(cond)
	if err != nil {
		t.Fatalf("failed to marshal condition: %v", err)
	}

	var parsed Condition
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal condition: %v", err)
	}

	if parsed.Type != cond.Type {
		t.Errorf("Type mismatch: expected %q, got %q", cond.Type, parsed.Type)
	}
	if parsed.Status != cond.Status {
		t.Errorf("Status mismatch: expected %q, got %q", cond.Status, parsed.Status)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestExternalDomainSpec_TargetIsAuto tests auto IP detection.
func TestExternalDomainSpec_TargetIsAuto(t *testing.T) {
	tests := []struct {
		targetIP string
		want     bool
	}{
		{"auto", true},
		{"Auto", true},
		{"AUTO", true},
		{"192.0.2.1", false},
		{"2001:db8::1", false},
		{"", false},
	}

	for _, tt := range tests {
		spec := ExternalDomainSpec{TargetIP: tt.targetIP}
		got := spec.TargetIsAuto()
		if got != tt.want {
			t.Errorf("TargetIsAuto(%q) = %v, want %v", tt.targetIP, got, tt.want)
		}
	}
}

// TestExternalDomainSpec_ParsedTargetIP tests IPv4/IPv6 parsing.
func TestExternalDomainSpec_ParsedTargetIP(t *testing.T) {
	tests := []struct {
		name      string
		targetIP  string
		wantIP    string
		wantIsV6  bool
		wantError bool
	}{
		{"auto", "auto", "", false, false},
		{"IPv4 valid", "192.0.2.1", "192.0.2.1", false, false},
		{"IPv4 localhost", "127.0.0.1", "127.0.0.1", false, false},
		{"IPv4 invalid (out of range)", "192.0.2.256", "", false, true},
		{"IPv4 invalid (too few octets)", "192.0.2", "", false, true},
		{"IPv6 valid", "2001:db8::1", "2001:db8::1", true, false},
		{"IPv6 compressed", "::1", "::1", true, false},
		{"IPv6 full", "2001:0db8:0000:0000:0000:0000:0000:0001", "2001:0db8:0000:0000:0000:0000:0000:0001", true, false},
		{"invalid (not IP)", "not-an-ip", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := ExternalDomainSpec{TargetIP: tt.targetIP}
			ip, isV6, err := spec.ParsedTargetIP()

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if ip != tt.wantIP {
				t.Errorf("IP = %q, want %q", ip, tt.wantIP)
			}
			if isV6 != tt.wantIsV6 {
				t.Errorf("isV6 = %v, want %v", isV6, tt.wantIsV6)
			}
		})
	}
}

// TestExternalDomainSpec_Validate_IPv6 tests validation with IPv6 addresses.
func TestExternalDomainSpec_Validate_IPv6(t *testing.T) {
	tests := []struct {
		name      string
		targetIP  string
		wantError bool
	}{
		{"valid IPv4", "192.0.2.1", false},
		{"valid IPv6", "2001:db8::1", false},
		{"valid auto", "auto", false},
		{"invalid IPv4", "192.0.2.256", true},
		{"invalid IPv6 (too short)", ":", true},
		{"invalid (not IP)", "not-an-ip", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := ExternalDomainSpec{
				FQDN:        "test.example.com",
				Zone:        "example.com",
				NodeID:      "test-node",
				TargetIP:    tt.targetIP,
				ProviderRef: "test-provider",
			}

			err := spec.Validate()

			if tt.wantError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}
