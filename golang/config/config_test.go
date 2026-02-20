package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetCACertificatePathEnvOverride(t *testing.T) {
	tmp := t.TempDir()
	ca := filepath.Join(tmp, "ca.crt")
	if err := os.WriteFile(ca, []byte("test"), 0644); err != nil {
		t.Fatalf("write ca: %v", err)
	}
	old := os.Getenv("GLOBULAR_CA_CERT")
	defer os.Setenv("GLOBULAR_CA_CERT", old)
	os.Setenv("GLOBULAR_CA_CERT", ca)
	got := GetCACertificatePath()
	if got != ca {
		t.Fatalf("expected %s, got %s", ca, got)
	}
}

