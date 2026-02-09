package main

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfigValidates(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Name != "log.LogService" {
		t.Fatalf("Name=%q want log.LogService", cfg.Name)
	}
	if cfg.Port != defaultPort || cfg.Proxy != defaultProxy {
		t.Fatalf("default ports mismatch")
	}
	if cfg.Protocol != "grpc" {
		t.Fatalf("Protocol=%q want grpc", cfg.Protocol)
	}
	if cfg.MonitoringPort <= 0 {
		t.Fatalf("MonitoringPort should be set")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigFileRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ID = "log-1"
	cfg.Domain = "example"
	cfg.Address = "example:1234"
	cfg.MonitoringPort = 9100
	cfg.RetentionHours = 10

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := cfg.SaveToFile(path); err != nil {
		t.Fatalf("SaveToFile: %v", err)
	}
	loaded, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile: %v", err)
	}
	if loaded.ID != cfg.ID || loaded.Address != cfg.Address || loaded.MonitoringPort != cfg.MonitoringPort {
		t.Fatalf("round-trip mismatch: %+v vs %+v", loaded, cfg)
	}
}

func TestCloneDeepCopy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Keywords = []string{"log"}
	cfg.Dependencies = []string{"dep"}

	clone := cfg.Clone()
	clone.Keywords[0] = "changed"
	clone.Dependencies[0] = "changed"

	if cfg.Keywords[0] == clone.Keywords[0] || cfg.Dependencies[0] == clone.Dependencies[0] {
		t.Fatal("clone mutation leaked to original")
	}
}

func TestLogSpecificValidation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MonitoringPort = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for MonitoringPort=0")
	}
}
