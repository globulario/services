package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigValidates(t *testing.T) {
	cfg := DefaultConfig()

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if cfg.Name != "authentication.AuthenticationService" {
		t.Errorf("Name = %q, want %q", cfg.Name, "authentication.AuthenticationService")
	}
	if cfg.Port != 10000 {
		t.Errorf("Port = %d, want 10000", cfg.Port)
	}
	if cfg.Proxy != 10001 {
		t.Errorf("Proxy = %d, want 10001", cfg.Proxy)
	}
	if cfg.Protocol != "grpc" {
		t.Errorf("Protocol = %q, want %q", cfg.Protocol, "grpc")
	}
	if cfg.Version != "0.0.1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "0.0.1")
	}
	if cfg.SessionTimeout != 15 {
		t.Errorf("SessionTimeout = %d, want 15", cfg.SessionTimeout)
	}
	if cfg.WatchSessionsDelay != 60 {
		t.Errorf("WatchSessionsDelay = %d, want 60", cfg.WatchSessionsDelay)
	}
	if cfg.RootPassword != "adminadmin" {
		t.Errorf("RootPassword = %q, want %q", cfg.RootPassword, "adminadmin")
	}
	if cfg.AdminEmail == "" {
		t.Error("AdminEmail should not be empty")
	}
	if len(cfg.Permissions) != 4 {
		t.Errorf("Permissions length = %d, want 4", len(cfg.Permissions))
	}
}

func TestConfigFileRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ID = "auth-1"
	cfg.ConfigPath = "custom/path.json"
	cfg.AdminEmail = "root@example.com"

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "auth-config.json")

	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	loaded, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if loaded.ID != cfg.ID {
		t.Errorf("ID mismatch: got %q, want %q", loaded.ID, cfg.ID)
	}
	if loaded.AdminEmail != cfg.AdminEmail {
		t.Errorf("AdminEmail mismatch: got %q, want %q", loaded.AdminEmail, cfg.AdminEmail)
	}
	if loaded.ConfigPath != cfg.ConfigPath {
		t.Errorf("ConfigPath mismatch: got %q, want %q", loaded.ConfigPath, cfg.ConfigPath)
	}
}

func TestCloneDeepCopy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Keywords = []string{"Authentication", "Security"}
	cfg.Repositories = []string{"repo1"}
	cfg.Discoveries = []string{"disc1"}
	cfg.Dependencies = []string{"dep1"}
	cfg.Permissions = []any{"perm1", "perm2"}
	cfg.TLS.CertFile = "/tmp/cert"
	cfg.TLS.KeyFile = "/tmp/key"
	cfg.TLS.CertAuthorityTrust = "/tmp/ca"
	cfg.AdminEmail = "root@example.com"
	cfg.RootPassword = "secret"

	clone := cfg.Clone()

	// Modify clone slices
	clone.Keywords[0] = "Modified"
	clone.Repositories = append(clone.Repositories, "repo2")
	clone.Dependencies[0] = "dep2"
	clone.Permissions[0] = "changed"
	clone.TLS.CertFile = "/other/cert"

	if cfg.Keywords[0] == clone.Keywords[0] {
		t.Error("Keywords slice was not deep copied")
	}
	if len(cfg.Repositories) == len(clone.Repositories) {
		t.Error("Repositories slice was not deep copied")
	}
	if cfg.Dependencies[0] == clone.Dependencies[0] {
		t.Error("Dependencies slice was not deep copied")
	}
	if cfg.Permissions[0] == clone.Permissions[0] {
		t.Error("Permissions slice was not deep copied")
	}
	if cfg.TLS.CertFile == clone.TLS.CertFile {
		t.Error("TLS struct was not deep copied")
	}
}

func TestValidateRejectsInvalidPort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Port = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected error for port 0, got nil")
	}
}

// Ensure temp files are cleaned up when SaveToFile creates directories.
func TestSaveCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nested := filepath.Join(tmpDir, "nested", "config.json")

	cfg := DefaultConfig()
	if err := cfg.SaveToFile(nested); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}
	if _, err := os.Stat(nested); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
}
