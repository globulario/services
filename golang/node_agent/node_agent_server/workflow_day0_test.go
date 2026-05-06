package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/config"
)

func TestPurgeDay0PKIMaterial_RemovesExistingArtifacts(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GLOBULAR_STATE_DIR", root)

	pkiDir := config.GetCanonicalPKIDir()
	targets := []string{
		filepath.Join(pkiDir, "ca.key"),
		filepath.Join(pkiDir, "ca.crt"),
		filepath.Join(pkiDir, "ca.pem"),
		filepath.Join(pkiDir, "issued", "services", "service.key"),
		filepath.Join(pkiDir, "issued", "services", "service.crt"),
	}
	for _, p := range targets {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
		if err := os.WriteFile(p, []byte("stale"), 0o600); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}

	srv := &NodeAgentServer{}
	if err := srv.purgeDay0PKIMaterial(); err != nil {
		t.Fatalf("purgeDay0PKIMaterial: %v", err)
	}

	for _, p := range targets {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, err=%v", p, err)
		}
	}
}
