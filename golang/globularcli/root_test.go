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

func TestPersistentPreRunTokenFileIsExplicitOptIn(t *testing.T) {
	home := t.TempDir()
	tokenPath := filepath.Join(home, "token.txt")
	if err := os.WriteFile(tokenPath, []byte("abc123"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}

	oldToken := rootCfg.token
	oldEnv := os.Getenv("GLOBULAR_TOKEN_FILE")
	defer func() {
		rootCfg.token = oldToken
		_ = os.Setenv("GLOBULAR_TOKEN_FILE", oldEnv)
	}()

	rootCfg.token = ""
	_ = os.Unsetenv("GLOBULAR_TOKEN_FILE")
	if err := rootCmd.PersistentPreRunE(nil, nil); err != nil {
		t.Fatalf("PreRunE error: %v", err)
	}
	if rootCfg.token != "" {
		t.Fatalf("token should stay empty without opt-in env; got %q", rootCfg.token)
	}

	rootCfg.token = ""
	_ = os.Setenv("GLOBULAR_TOKEN_FILE", tokenPath)
	if err := rootCmd.PersistentPreRunE(nil, nil); err != nil {
		t.Fatalf("PreRunE error (opt-in): %v", err)
	}
	if rootCfg.token != "abc123" {
		t.Fatalf("expected token from opt-in file, got %q", rootCfg.token)
	}
}
