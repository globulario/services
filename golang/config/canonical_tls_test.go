package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalTLSPathsAreDomainAgnostic(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("GLOBULAR_STATE_DIR", tmp)

	tlsDir, fullchain, privkey, ca := CanonicalTLSPaths(GetRuntimeConfigDir())

	// INV-PKI-1: Verify paths use canonical PKI structure
	if !strings.Contains(tlsDir, filepath.Join("pki", "issued", "services")) {
		t.Fatalf("tlsDir should contain /pki/issued/services/, got %s", tlsDir)
	}
	if !strings.Contains(fullchain, "service.crt") {
		t.Fatalf("fullchain should contain service.crt, got %s", fullchain)
	}
	if !strings.Contains(privkey, "service.key") {
		t.Fatalf("privkey should contain service.key, got %s", privkey)
	}
	if !strings.Contains(ca, filepath.Join("pki", "ca.pem")) {
		t.Fatalf("ca should contain /pki/ca.pem, got %s", ca)
	}

	// Verify paths are domain-agnostic (no domain-specific subdirectories)
	for name, p := range map[string]string{
		"tlsDir":    tlsDir,
		"fullchain": fullchain,
		"privkey":   privkey,
		"ca":        ca,
	} {
		if strings.Contains(p, "example.com") {
			t.Fatalf("%s should not contain domain-scoped path: %s", name, p)
		}
	}
}
