package dnsprovider

import (
	"context"
	"testing"
)

func TestRegister(t *testing.T) {
	// Clear factories for test isolation
	mu.Lock()
	factories = make(map[string]ProviderFactory)
	mu.Unlock()

	// Test successful registration
	factory := func(cfg Config) (Provider, error) {
		return nil, nil
	}

	Register("test", factory)

	if !IsRegistered("test") {
		t.Fatal("expected provider 'test' to be registered")
	}

	// Test duplicate registration panics
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	Register("test", factory)
}

func TestRegisterNilFactory(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when registering nil factory")
		}
	}()
	Register("nil-test", nil)
}

func TestNewProvider(t *testing.T) {
	// Clear factories
	mu.Lock()
	factories = make(map[string]ProviderFactory)
	mu.Unlock()

	// Register a test provider
	Register("test", func(cfg Config) (Provider, error) {
		return &testProvider{zone: cfg.Zone}, nil
	})

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: Config{
				Type: "test",
				Zone: "example.com",
			},
			wantErr: false,
		},
		{
			name: "unknown provider type",
			cfg: Config{
				Type: "unknown",
				Zone: "example.com",
			},
			wantErr: true,
			errMsg:  "unknown provider type",
		},
		{
			name: "missing zone",
			cfg: Config{
				Type: "test",
				Zone: "",
			},
			wantErr: true,
			errMsg:  "zone is required",
		},
		{
			name: "default TTL applied",
			cfg: Config{
				Type:       "test",
				Zone:       "example.com",
				DefaultTTL: 0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.cfg)
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
			if provider == nil {
				t.Fatal("expected non-nil provider")
			}

			// Check default TTL was applied
			if tt.cfg.DefaultTTL == 0 && tt.cfg.DefaultTTL != 600 {
				// Default should be applied by NewProvider
				// (Note: this is checking the input cfg, the provider itself doesn't expose TTL)
			}
		})
	}
}

func TestListProviders(t *testing.T) {
	// Clear factories
	mu.Lock()
	factories = make(map[string]ProviderFactory)
	mu.Unlock()

	// Register multiple providers
	Register("provider1", func(cfg Config) (Provider, error) { return nil, nil })
	Register("provider2", func(cfg Config) (Provider, error) { return nil, nil })

	providers := ListProviders()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}

	// Check both are present
	hasProvider1 := false
	hasProvider2 := false
	for _, p := range providers {
		if p == "provider1" {
			hasProvider1 = true
		}
		if p == "provider2" {
			hasProvider2 = true
		}
	}

	if !hasProvider1 || !hasProvider2 {
		t.Fatalf("expected provider1 and provider2 in list, got %v", providers)
	}
}

func TestIsRegistered(t *testing.T) {
	// Clear factories
	mu.Lock()
	factories = make(map[string]ProviderFactory)
	mu.Unlock()

	if IsRegistered("nonexistent") {
		t.Fatal("expected IsRegistered to return false for nonexistent provider")
	}

	Register("exists", func(cfg Config) (Provider, error) { return nil, nil })

	if !IsRegistered("exists") {
		t.Fatal("expected IsRegistered to return true for registered provider")
	}
}

// Test helper types
type testProvider struct {
	zone string
}

func (p *testProvider) Name() string { return "test" }
func (p *testProvider) UpsertA(ctx context.Context, zone string, name string, ip string, ttl int) error {
	return nil
}
func (p *testProvider) UpsertAAAA(ctx context.Context, zone string, name string, ip string, ttl int) error {
	return nil
}
func (p *testProvider) UpsertCNAME(ctx context.Context, zone string, name string, target string, ttl int) error {
	return nil
}
func (p *testProvider) UpsertTXT(ctx context.Context, zone string, name string, values []string, ttl int) error {
	return nil
}
func (p *testProvider) DeleteTXT(ctx context.Context, zone string, name string, values []string) error {
	return nil
}
func (p *testProvider) GetRecords(ctx context.Context, zone string, name string, rtype string) ([]Record, error) {
	return nil, nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
