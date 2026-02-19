package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigValidates(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Name != "event.EventService" {
		t.Fatalf("Name = %q, want event.EventService", cfg.Name)
	}
	if cfg.Port != defaultPort {
		t.Fatalf("Port = %d, want %d", cfg.Port, defaultPort)
	}
	if cfg.Proxy != defaultProxy {
		t.Fatalf("Proxy = %d, want %d", cfg.Proxy, defaultProxy)
	}
	if cfg.Protocol != "grpc" {
		t.Fatalf("Protocol = %q, want grpc", cfg.Protocol)
	}
	if cfg.Version != "0.0.1" {
		t.Fatalf("Version = %q, want 0.0.1", cfg.Version)
	}
	if !cfg.AllowAllOrigins {
		t.Fatal("AllowAllOrigins should default to true")
	}
	if !cfg.KeepAlive {
		t.Fatal("KeepAlive should default to true")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigFileRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ID = "ev-1"
	cfg.Domain = "example.local"
	cfg.Address = "example.local:1234"
	cfg.Port = 1234

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := cfg.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file missing: %v", err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if loaded.ID != cfg.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, cfg.ID)
	}
	if loaded.Domain != cfg.Domain {
		t.Errorf("Domain = %q, want %q", loaded.Domain, cfg.Domain)
	}
	if loaded.Address != cfg.Address {
		t.Errorf("Address = %q, want %q", loaded.Address, cfg.Address)
	}
	if loaded.Port != cfg.Port {
		t.Errorf("Port = %d, want %d", loaded.Port, cfg.Port)
	}
}

func TestCloneDeepCopy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Keywords = []string{"Event", "Service"}
	cfg.Dependencies = []string{"dep1"}
	cfg.Permissions = []any{"p1"}
	cfg.TLS.Enabled = true
	cfg.TLS.CertFile = "cert.pem"

	clone := cfg.Clone()

	clone.Keywords[0] = "Changed"
	clone.Dependencies[0] = "Changed"
	clone.Permissions[0] = "p2"
	clone.TLS.CertFile = "other.pem"

	if cfg.Keywords[0] == clone.Keywords[0] {
		t.Error("mutating clone.Keywords affected original")
	}
	if cfg.Dependencies[0] == clone.Dependencies[0] {
		t.Error("mutating clone.Dependencies affected original")
	}
	if cfg.Permissions[0] == clone.Permissions[0] {
		t.Error("mutating clone.Permissions affected original")
	}
	if cfg.TLS.CertFile == clone.TLS.CertFile {
		t.Error("mutating clone.TLS affected original")
	}
}

func TestEventSpecificValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Port = 0
	if err := cfg.Validate(); err == nil {
		t.Error("Validate should fail when Port is zero")
	}
	cfg = DefaultConfig()
	cfg.Protocol = ""
	if err := cfg.Validate(); err == nil {
		t.Error("Validate should fail when Protocol is empty")
	}
}
