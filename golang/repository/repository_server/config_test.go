package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaultConfig verifies the default configuration values
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Core identity
	if cfg.Name != "repository.PackageRepository" {
		t.Errorf("Name = %q, want %q", cfg.Name, "repository.PackageRepository")
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

	if cfg.Description != "Package repository for distributing services and applications" {
		t.Errorf("Description = %q, want repository description", cfg.Description)
	}

	// Keywords
	expectedKeywords := []string{"Package", "Repository"}
	if len(cfg.Keywords) != len(expectedKeywords) {
		t.Errorf("Keywords length = %d, want %d", len(cfg.Keywords), len(expectedKeywords))
	}

	// Policy
	if !cfg.AllowAllOrigins {
		t.Error("AllowAllOrigins should default to true")
	}

	if !cfg.KeepAlive {
		t.Error("KeepAlive should default to true")
	}

	// Runtime
	if cfg.Process != -1 {
		t.Errorf("Process = %d, want -1", cfg.Process)
	}

	if cfg.ProxyProcess != -1 {
		t.Errorf("ProxyProcess = %d, want -1", cfg.ProxyProcess)
	}

	// TLS
	if cfg.TLS.Enabled {
		t.Error("TLS should default to disabled")
	}

	// Repository-specific
	if cfg.Root != "" {
		t.Errorf("Root = %q, want empty string (set during init)", cfg.Root)
	}
}

// TestConfigValidation tests the Validate method
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(*Config)
		wantError bool
	}{
		{
			name:      "valid default config",
			modify:    func(c *Config) {},
			wantError: false,
		},
		{
			name: "missing name",
			modify: func(c *Config) {
				c.Name = ""
			},
			wantError: true,
		},
		{
			name: "invalid port (zero)",
			modify: func(c *Config) {
				c.Port = 0
			},
			wantError: true,
		},
		{
			name: "invalid port (too high)",
			modify: func(c *Config) {
				c.Port = 99999
			},
			wantError: true,
		},
		{
			name: "missing protocol",
			modify: func(c *Config) {
				c.Protocol = ""
			},
			wantError: true,
		},
		{
			name: "missing version",
			modify: func(c *Config) {
				c.Version = ""
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)

			err := cfg.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestConfigFileOperations tests saving and loading configuration
func TestConfigFileOperations(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create config with custom values
	cfg := DefaultConfig()
	cfg.ID = "test-id-456"
	cfg.Domain = "test.local"
	cfg.Port = 20000
	cfg.Root = "/var/lib/globular/repository-test"

	// Save to file
	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load from file
	loaded, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	// Verify loaded config matches original
	if loaded.ID != cfg.ID {
		t.Errorf("loaded.ID = %q, want %q", loaded.ID, cfg.ID)
	}

	if loaded.Domain != cfg.Domain {
		t.Errorf("loaded.Domain = %q, want %q", loaded.Domain, cfg.Domain)
	}

	if loaded.Port != cfg.Port {
		t.Errorf("loaded.Port = %d, want %d", loaded.Port, cfg.Port)
	}

	if loaded.Name != cfg.Name {
		t.Errorf("loaded.Name = %q, want %q", loaded.Name, cfg.Name)
	}

	if loaded.Root != cfg.Root {
		t.Errorf("loaded.Root = %q, want %q", loaded.Root, cfg.Root)
	}
}

// TestConfigClone tests deep copying
func TestConfigClone(t *testing.T) {
	original := DefaultConfig()
	original.ID = "original-id"
	original.Port = 20000
	original.Keywords = []string{"Test", "Clone"}
	original.Root = "/original/path"

	clone := original.Clone()

	// Verify values match
	if clone.ID != original.ID {
		t.Errorf("clone.ID = %q, want %q", clone.ID, original.ID)
	}

	if clone.Port != original.Port {
		t.Errorf("clone.Port = %d, want %d", clone.Port, original.Port)
	}

	if clone.Root != original.Root {
		t.Errorf("clone.Root = %q, want %q", clone.Root, original.Root)
	}

	// Verify deep copy (modifying clone doesn't affect original)
	clone.ID = "modified-id"
	clone.Port = 30000
	clone.Keywords[0] = "Modified"
	clone.Root = "/modified/path"

	if original.ID == clone.ID {
		t.Error("Modifying clone.ID affected original")
	}

	if original.Port == clone.Port {
		t.Error("Modifying clone.Port affected original")
	}

	if original.Keywords[0] == clone.Keywords[0] {
		t.Error("Modifying clone.Keywords affected original")
	}

	if original.Root == clone.Root {
		t.Error("Modifying clone.Root affected original")
	}
}

// TestRepositorySpecificFields tests Repository-specific config fields
func TestRepositorySpecificFields(t *testing.T) {
	cfg := DefaultConfig()

	// Root field should be configurable
	cfg.Root = "/var/lib/globular/repository"

	if cfg.Root != "/var/lib/globular/repository" {
		t.Errorf("Root = %q, want %q", cfg.Root, "/var/lib/globular/repository")
	}

	// Clone should preserve Root
	clone := cfg.Clone()
	if clone.Root != cfg.Root {
		t.Errorf("clone.Root = %q, want %q", clone.Root, cfg.Root)
	}

	// Modifying clone.Root should not affect original
	clone.Root = "/different/path"
	if cfg.Root == clone.Root {
		t.Error("Modifying clone.Root affected original")
	}
}

// TestConfigInvariant documents the Config component contract
func TestConfigInvariant(t *testing.T) {
	t.Log("Config Component Contract:")
	t.Log("1. DefaultConfig() returns valid, usable defaults")
	t.Log("2. Validate() enforces required fields")
	t.Log("3. SaveToFile() persists config as JSON")
	t.Log("4. LoadFromFile() restores config from JSON")
	t.Log("5. Clone() creates independent deep copy")
	t.Log("6. Root field specifies package storage directory")
	t.Log("")
	t.Log("Phase 1 Step 1: Config extracted for clean separation of concerns")
}
