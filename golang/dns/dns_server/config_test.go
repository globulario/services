package main

import (
	"path/filepath"
	"strconv"
	"testing"
)

func TestDefaultConfigValidates(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Name != "dns.DnsService" {
		t.Fatalf("Name = %q, want dns.DnsService", cfg.Name)
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
	if cfg.DnsPort != 53 {
		t.Fatalf("DnsPort = %d, want 53", cfg.DnsPort)
	}
	if cfg.Root == "" {
		t.Fatal("Root should be set by default")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigFileRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ID = "dns-1"
	cfg.Domain = "example.internal"
	cfg.Address = "10.0.0.2:" + strconv.Itoa(cfg.Port)
	cfg.Domains = []string{"example.internal."}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := cfg.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if loaded.ID != cfg.ID || loaded.Domain != cfg.Domain || loaded.Address != cfg.Address {
		t.Fatalf("loaded config mismatch: %+v vs %+v", loaded, cfg)
	}
	if len(loaded.Domains) != 1 || loaded.Domains[0] != "example.internal." {
		t.Fatalf("Domains round-trip mismatch: %v", loaded.Domains)
	}
}

func TestCloneDeepCopy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Keywords = []string{"DNS", "Resolver"}
	cfg.Dependencies = []string{"dep1"}
	cfg.Discoveries = []string{"disc1"}
	cfg.Domains = []string{"globular.internal."}
	cfg.Permissions = []any{"perm"}

	clone := cfg.Clone()

	clone.Keywords[0] = "Changed"
	clone.Dependencies[0] = "Changed"
	clone.Discoveries[0] = "Changed"
	clone.Domains[0] = "changed."
	clone.Permissions[0] = "other"

	if cfg.Keywords[0] == clone.Keywords[0] ||
		cfg.Dependencies[0] == clone.Dependencies[0] ||
		cfg.Discoveries[0] == clone.Discoveries[0] ||
		cfg.Domains[0] == clone.Domains[0] ||
		cfg.Permissions[0] == clone.Permissions[0] {
		t.Fatal("mutating clone affected original slices")
	}
}

func TestDnsSpecificValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DnsPort = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate should fail when DnsPort is zero")
	}
}
