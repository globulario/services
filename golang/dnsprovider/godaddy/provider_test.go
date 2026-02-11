package godaddy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/globulario/services/golang/dnsprovider"
)

func TestNewGoDaddyProvider(t *testing.T) {
	tests := []struct {
		name    string
		cfg     dnsprovider.Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: dnsprovider.Config{
				Type: "godaddy",
				Zone: "example.com",
				Credentials: map[string]string{
					"api_key":    "test-key",
					"api_secret": "test-secret",
				},
				DefaultTTL: 600,
			},
			wantErr: false,
		},
		{
			name: "missing api_key",
			cfg: dnsprovider.Config{
				Type: "godaddy",
				Zone: "example.com",
				Credentials: map[string]string{
					"api_secret": "test-secret",
				},
			},
			wantErr: true,
			errMsg:  "api_key is required",
		},
		{
			name: "missing api_secret",
			cfg: dnsprovider.Config{
				Type: "godaddy",
				Zone: "example.com",
				Credentials: map[string]string{
					"api_key": "test-key",
				},
			},
			wantErr: true,
			errMsg:  "api_secret is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewGoDaddyProvider(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errMsg)
				}
				if !contains(err.Error(), tt.errMsg) {
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
		})
	}
}

func TestGoDaddyProvider_UpsertA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header
		auth := r.Header.Get("Authorization")
		if auth != "sso-key test-key:test-secret" {
			t.Errorf("invalid auth header: %s", auth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify request
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/v1/domains/example.com/records/A/test" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &GoDaddyProvider{
		apiKey:    "test-key",
		apiSecret: "test-secret",
		zone:      "example.com",
		baseURL:   server.URL,
		client:    server.Client(),
		ttl:       600,
	}

	err := provider.UpsertA(context.Background(), "example.com", "test", "192.0.2.1", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGoDaddyProvider_UpsertTXT(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/v1/domains/example.com/records/TXT/_acme-challenge.test" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	provider := &GoDaddyProvider{
		apiKey:    "test-key",
		apiSecret: "test-secret",
		zone:      "example.com",
		baseURL:   server.URL,
		client:    server.Client(),
		ttl:       60,
	}

	err := provider.UpsertTXT(context.Background(), "example.com", "_acme-challenge.test", []string{"token123"}, 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGoDaddyProvider_ZoneMismatch(t *testing.T) {
	provider := &GoDaddyProvider{
		zone: "example.com",
	}

	err := provider.UpsertA(context.Background(), "wrong.com", "test", "192.0.2.1", 600)
	if err == nil {
		t.Fatal("expected error for zone mismatch")
	}

	if !contains(err.Error(), "zone mismatch") {
		t.Errorf("expected zone mismatch error, got: %v", err)
	}
}

func TestGoDaddyProvider_Name(t *testing.T) {
	provider := &GoDaddyProvider{}
	if provider.Name() != "godaddy" {
		t.Errorf("expected Name() to return 'godaddy', got %q", provider.Name())
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
