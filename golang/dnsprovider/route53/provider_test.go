package route53

import (
	"testing"

	"github.com/globulario/services/golang/dnsprovider"
)

func TestRoute53Provider_Name(t *testing.T) {
	provider := &Route53Provider{}
	if provider.Name() != "route53" {
		t.Errorf("expected Name() to return 'route53', got %q", provider.Name())
	}
}

func TestRoute53Provider_FQDN(t *testing.T) {
	provider := &Route53Provider{zone: "example.com"}

	tests := []struct {
		name     string
		expected string
	}{
		{"test", "test.example.com."},
		{"", "example.com."},
		{"@", "example.com."},
		{"test.example.com", "test.example.com."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fqdn := provider.fqdn(tt.name)
			if fqdn != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, fqdn)
			}
		})
	}
}

func TestRoute53Provider_ExtractRelativeName(t *testing.T) {
	provider := &Route53Provider{zone: "example.com"}

	tests := []struct {
		fqdn     string
		expected string
	}{
		{"test.example.com.", "test"},
		{"test.example.com", "test"},
		{"sub.test.example.com.", "sub.test"},
		{"example.com.", "@"},
		{"example.com", "@"},
	}

	for _, tt := range tests {
		t.Run(tt.fqdn, func(t *testing.T) {
			result := provider.extractRelativeName(tt.fqdn)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestRoute53Provider_ResolveTTL(t *testing.T) {
	tests := []struct {
		name         string
		provider     *Route53Provider
		input        int
		expected     int64
	}{
		{
			name:     "explicit TTL",
			provider: &Route53Provider{ttl: 600},
			input:    300,
			expected: 300,
		},
		{
			name:     "provider default TTL",
			provider: &Route53Provider{ttl: 600},
			input:    0,
			expected: 600,
		},
		{
			name:     "system default TTL",
			provider: &Route53Provider{ttl: 0},
			input:    0,
			expected: 300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.provider.resolveTTL(tt.input)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestRoute53Provider_ValidateZone(t *testing.T) {
	provider := &Route53Provider{zone: "example.com"}

	// Valid zone
	if err := provider.validateZone("example.com"); err != nil {
		t.Errorf("unexpected error for valid zone: %v", err)
	}

	// Invalid zone
	err := provider.validateZone("wrong.com")
	if err == nil {
		t.Fatal("expected error for zone mismatch")
	}

	if !contains(err.Error(), "zone mismatch") {
		t.Errorf("expected zone mismatch error, got: %v", err)
	}
}

func TestNewRoute53Provider_RequiresHostedZone(t *testing.T) {
	// Skip if AWS credentials not available
	t.Skip("Requires AWS credentials and real hosted zone - run manually")

	cfg := dnsprovider.Config{
		Type:       "route53",
		Zone:       "example.com",
		DefaultTTL: 300,
	}

	_, err := NewRoute53Provider(cfg)
	if err != nil {
		// Expected if hosted zone doesn't exist or no AWS credentials
		t.Logf("Expected failure without real hosted zone: %v", err)
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

// Integration test example (requires real AWS account)
func TestRoute53Provider_Integration(t *testing.T) {
	t.Skip("Integration test - requires AWS credentials and hosted zone")

	/*
	   // Prerequisites:
	   // 1. AWS credentials configured (env vars, ~/.aws/credentials, or IAM role)
	   // 2. Hosted zone "example.com" exists in Route53
	   // 3. Permissions: route53:ChangeResourceRecordSets, route53:ListHostedZones

	   cfg := dnsprovider.Config{
	       Type:       "route53",
	       Zone:       "example.com",
	       DefaultTTL: 300,
	   }

	   provider, err := NewRoute53Provider(cfg)
	   if err != nil {
	       t.Fatalf("failed to create provider: %v", err)
	   }

	   ctx := context.Background()

	   // Test UpsertA
	   err = provider.UpsertA(ctx, "example.com", "test", "192.0.2.1", 300)
	   if err != nil {
	       t.Fatalf("UpsertA failed: %v", err)
	   }

	   // Test GetRecords
	   records, err := provider.GetRecords(ctx, "example.com", "test", "A")
	   if err != nil {
	       t.Fatalf("GetRecords failed: %v", err)
	   }

	   if len(records) == 0 {
	       t.Fatal("expected at least one record")
	   }

	   // Test UpsertTXT (ACME challenge)
	   err = provider.UpsertTXT(ctx, "example.com", "_acme-challenge.test", []string{"test-token"}, 60)
	   if err != nil {
	       t.Fatalf("UpsertTXT failed: %v", err)
	   }

	   // Test DeleteTXT
	   err = provider.DeleteTXT(ctx, "example.com", "_acme-challenge.test", []string{"test-token"})
	   if err != nil {
	       t.Fatalf("DeleteTXT failed: %v", err)
	   }

	   // Cleanup
	   provider.deleteRecord(ctx, "test.example.com.", "A")
	*/
}
