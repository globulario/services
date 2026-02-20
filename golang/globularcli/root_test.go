package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistentPreRunSetsCACertEnv(t *testing.T) {
	caDir := t.TempDir()
	caPath := filepath.Join(caDir, "ca.crt")
	if err := os.WriteFile(caPath, []byte("ca"), 0644); err != nil {
		t.Fatalf("write ca: %v", err)
	}
	oldEnv := os.Getenv("GLOBULAR_CA_CERT")
	defer os.Setenv("GLOBULAR_CA_CERT", oldEnv)

	oldCA := rootCfg.caFile
	oldToken := rootCfg.token
	rootCfg.caFile = caPath
	rootCfg.token = ""
	defer func() {
		rootCfg.caFile = oldCA
		rootCfg.token = oldToken
	}()

	if err := rootCmd.PersistentPreRunE(nil, nil); err != nil {
		t.Fatalf("PreRunE error: %v", err)
	}
	if got := os.Getenv("GLOBULAR_CA_CERT"); got != caPath {
		t.Fatalf("expected env %s, got %s", caPath, got)
	}
}

