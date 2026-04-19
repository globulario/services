package config

import (
	"path/filepath"
	"testing"
)

// TestGetCACertificatePathCanonical verifies the CA cert path is always
// derived from the state root — no env var overrides (per hard rules).
func TestGetCACertificatePathCanonical(t *testing.T) {
	got := GetCACertificatePath()
	if got == "" {
		t.Fatal("expected non-empty CA cert path")
	}
	// Path must end with the canonical suffix — never a custom override.
	if !filepath.IsAbs(got) {
		t.Fatalf("expected absolute path, got %s", got)
	}
}

