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
	if cfg.Name != "discovery.PackageDiscovery" {
		t.Errorf("Name = %q, want %q", cfg.Name, "discovery.PackageDiscovery")
	}

	if cfg.Port != 10029 {
		t.Errorf("Port = %d, want 10029", cfg.Port)
	}

	if cfg.Proxy != 10030 {
		t.Errorf("Proxy = %d, want 10030", cfg.Proxy)
	}

	if cfg.Protocol != "grpc" {
		t.Errorf("Protocol = %q, want %q", cfg.Protocol, "grpc")
	}

	if cfg.Version != "0.0.1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "0.0.1")
	}

	if cfg.Description != "Service discovery client" {
		t.Errorf("Description = %q, want %q", cfg.Description, "Service discovery client")
	}

	// Dependencies
	expectedDeps := []string{"rbac.RbacService", "resource.ResourceService"}
	if len(cfg.Dependencies) != len(expectedDeps) {
		t.Errorf("Dependencies length = %d, want %d", len(cfg.Dependencies), len(expectedDeps))
	}

	// Keywords
	expectedKeywords := []string{"Discovery", "Package", "Service", "Application"}
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

	// Permissions
	if len(cfg.Permissions) != 2 {
		t.Errorf("Permissions length = %d, want 2 (PublishService, PublishApplication)", len(cfg.Permissions))
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
		{
			name: "missing dependencies",
			modify: func(c *Config) {
				c.Dependencies = []string{}
			},
			wantError: true,
		},
		{
			name: "missing permissions",
			modify: func(c *Config) {
				c.Permissions = []any{}
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
	cfg.ID = "test-id-123"
	cfg.Domain = "test.local"
	cfg.Port = 20029

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
}

// TestConfigClone tests deep copying
func TestConfigClone(t *testing.T) {
	original := DefaultConfig()
	original.ID = "original-id"
	original.Port = 20029
	original.Keywords = []string{"Test", "Clone"}
	original.Dependencies = []string{"test.Service"}

	clone := original.Clone()

	// Verify values match
	if clone.ID != original.ID {
		t.Errorf("clone.ID = %q, want %q", clone.ID, original.ID)
	}

	if clone.Port != original.Port {
		t.Errorf("clone.Port = %d, want %d", clone.Port, original.Port)
	}

	// Verify deep copy (modifying clone doesn't affect original)
	clone.ID = "modified-id"
	clone.Port = 30029
	clone.Keywords[0] = "Modified"
	clone.Dependencies[0] = "modified.Service"

	if original.ID == clone.ID {
		t.Error("Modifying clone.ID affected original")
	}

	if original.Port == clone.Port {
		t.Error("Modifying clone.Port affected original")
	}

	if original.Keywords[0] == clone.Keywords[0] {
		t.Error("Modifying clone.Keywords affected original")
	}

	if original.Dependencies[0] == clone.Dependencies[0] {
		t.Error("Modifying clone.Dependencies affected original")
	}
}

// TestConfigPermissionsStructure tests RBAC permissions in config
func TestConfigPermissionsStructure(t *testing.T) {
	cfg := DefaultConfig()

	if len(cfg.Permissions) != 2 {
		t.Fatalf("Permissions length = %d, want 2", len(cfg.Permissions))
	}

	// Verify PublishService permission
	publishService, ok := cfg.Permissions[0].(map[string]any)
	if !ok {
		t.Fatal("PublishService permission is not a map")
	}

	if publishService["action"] != "/discovery.PackageDiscovery/PublishService" {
		t.Errorf("PublishService action = %v, want /discovery.PackageDiscovery/PublishService",
			publishService["action"])
	}

	if publishService["permission"] != "write" {
		t.Errorf("PublishService permission = %v, want write", publishService["permission"])
	}

	// Verify resources structure
	resources, ok := publishService["resources"].([]any)
	if !ok {
		t.Fatal("PublishService resources is not an array")
	}

	if len(resources) != 2 {
		t.Errorf("PublishService resources length = %d, want 2", len(resources))
	}

	// Verify first resource (RepositoryId)
	res0, ok := resources[0].(map[string]any)
	if !ok {
		t.Fatal("First resource is not a map")
	}

	if res0["field"] != "RepositoryId" {
		t.Errorf("First resource field = %v, want RepositoryId", res0["field"])
	}

	// Verify PublishApplication permission
	publishApp, ok := cfg.Permissions[1].(map[string]any)
	if !ok {
		t.Fatal("PublishApplication permission is not a map")
	}

	if publishApp["action"] != "/discovery.PackageDiscovery/PublishApplication" {
		t.Errorf("PublishApplication action = %v, want /discovery.PackageDiscovery/PublishApplication",
			publishApp["action"])
	}
}

// TestPermissionsClone verifies deep copy of complex permission structures
func TestPermissionsClone(t *testing.T) {
	original := DefaultConfig()
	clone := original.Clone()

	// Verify permissions were cloned
	if len(clone.Permissions) != len(original.Permissions) {
		t.Errorf("clone.Permissions length = %d, want %d",
			len(clone.Permissions), len(original.Permissions))
	}

	// Modify clone permission
	if clonePerm, ok := clone.Permissions[0].(map[string]any); ok {
		clonePerm["action"] = "modified-action"

		// Verify original wasn't affected
		if origPerm, ok := original.Permissions[0].(map[string]any); ok {
			if origPerm["action"] == "modified-action" {
				t.Error("Modifying clone.Permissions affected original")
			}
		}
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
	t.Log("6. Permissions include PublishService and PublishApplication")
	t.Log("7. Dependencies: rbac.RbacService, resource.ResourceService")
	t.Log("")
	t.Log("Phase 1 Step 1: Config extracted for clean separation of concerns")
}
