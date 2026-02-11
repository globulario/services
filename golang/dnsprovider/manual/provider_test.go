package manual

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/globulario/services/golang/dnsprovider"
)

func TestManualProvider_UpsertA(t *testing.T) {
	var buf bytes.Buffer
	provider := &ManualProvider{
		zone:   "example.com",
		output: &buf,
	}

	err := provider.UpsertA(context.Background(), "example.com", "test", "192.0.2.1", 600)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "MANUAL DNS OPERATION REQUIRED") {
		t.Error("expected output to contain operation header")
	}
	if !strings.Contains(output, "CREATE or UPDATE A record") {
		t.Error("expected output to indicate A record operation")
	}
	if !strings.Contains(output, "192.0.2.1") {
		t.Error("expected output to contain IP address")
	}
	if !strings.Contains(output, "test.example.com.") {
		t.Error("expected output to contain FQDN")
	}
}

func TestManualProvider_UpsertTXT(t *testing.T) {
	var buf bytes.Buffer
	provider := &ManualProvider{
		zone:   "example.com",
		output: &buf,
	}

	err := provider.UpsertTXT(context.Background(), "example.com", "_acme-challenge.test", []string{"token123"}, 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ACME DNS-01 Challenge") {
		t.Error("expected output to indicate ACME challenge")
	}
	if !strings.Contains(output, "_acme-challenge.test.example.com.") {
		t.Error("expected output to contain ACME challenge FQDN")
	}
	if !strings.Contains(output, "token123") {
		t.Error("expected output to contain TXT value")
	}
}

func TestManualProvider_DeleteTXT(t *testing.T) {
	var buf bytes.Buffer
	provider := &ManualProvider{
		zone:   "example.com",
		output: &buf,
	}

	err := provider.DeleteTXT(context.Background(), "example.com", "_acme-challenge.test", []string{"token123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "DELETE TXT record") {
		t.Error("expected output to indicate DELETE operation")
	}
	if !strings.Contains(output, "ACME challenge record can now be safely deleted") {
		t.Error("expected output to indicate ACME cleanup")
	}
}

func TestManualProvider_ZoneMismatch(t *testing.T) {
	provider := &ManualProvider{
		zone: "example.com",
	}

	err := provider.UpsertA(context.Background(), "wrong.com", "test", "192.0.2.1", 600)
	if err == nil {
		t.Fatal("expected error for zone mismatch")
	}

	var providerErr *dnsprovider.ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T", err)
	}

	if providerErr.Provider != "manual" {
		t.Errorf("expected provider 'manual', got %q", providerErr.Provider)
	}
}

func TestManualProvider_FQDN(t *testing.T) {
	provider := &ManualProvider{zone: "example.com"}

	tests := []struct {
		name     string
		zone     string
		expected string
	}{
		{"test", "example.com", "test.example.com."},
		{"", "example.com", "example.com."},
		{"@", "example.com", "example.com."},
		{"test.example.com", "example.com", "test.example.com."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fqdn := provider.fqdn(tt.name, tt.zone)
			if fqdn != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, fqdn)
			}
		})
	}
}

func TestManualProvider_Name(t *testing.T) {
	provider := &ManualProvider{}
	if provider.Name() != "manual" {
		t.Errorf("expected Name() to return 'manual', got %q", provider.Name())
	}
}

func TestNewManualProvider(t *testing.T) {
	cfg := dnsprovider.Config{
		Type: "manual",
		Zone: "example.com",
	}

	provider, err := NewManualProvider(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if provider == nil {
		t.Fatal("expected non-nil provider")
	}

	if provider.Name() != "manual" {
		t.Errorf("expected provider name 'manual', got %q", provider.Name())
	}
}
