package main

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfigValidates(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Name != "search.SearchService" {
		t.Fatalf("Name=%q want search.SearchService", cfg.Name)
	}
	if cfg.Port != defaultPort || cfg.Proxy != defaultProxy {
		t.Fatalf("default ports mismatch")
	}
	if cfg.Protocol != "grpc" {
		t.Fatalf("Protocol=%q want grpc", cfg.Protocol)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigFileRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ID = "search-1"
	cfg.Address = "example:1234"
	cfg.Domain = "example"
	cfg.Permissions = []any{"p1"}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := cfg.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}
	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if loaded.ID != cfg.ID || loaded.Address != cfg.Address {
		t.Fatalf("round-trip mismatch: %+v vs %+v", loaded, cfg)
	}
}

func TestCloneDeepCopy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Keywords = []string{"search"}
	cfg.Dependencies = []string{"dep"}
	cfg.Permissions = []any{"p"}

	clone := cfg.Clone()
	clone.Keywords[0] = "changed"
	clone.Dependencies[0] = "changed"
	clone.Permissions[0] = "p2"

	if cfg.Keywords[0] == clone.Keywords[0] ||
		cfg.Dependencies[0] == clone.Dependencies[0] ||
		cfg.Permissions[0] == clone.Permissions[0] {
		t.Fatal("clone mutation leaked to original")
	}
}
