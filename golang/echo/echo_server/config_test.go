package main

import (
	"os"
	"testing"
)

// TestDefaultConfig verifies the default configuration
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Verify service identity
	if cfg.Name != "echo.EchoService" {
		t.Errorf("Name = %q, want %q", cfg.Name, "echo.EchoService")
	}

	// Verify network defaults
	if cfg.Port != 10000 {
		t.Errorf("Port = %d, want 10000", cfg.Port)
	}

	if cfg.Proxy != 10001 {
		t.Errorf("Proxy = %d, want 10001", cfg.Proxy)
	}

	if cfg.Protocol != "grpc" {
		t.Errorf("Protocol = %q, want %q", cfg.Protocol, "grpc")
	}

	// Verify metadata
	if cfg.Version != "0.0.1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "0.0.1")
	}

	if cfg.Description == "" {
		t.Error("Description should not be empty")
	}

	if len(cfg.Keywords) == 0 {
		t.Error("Keywords should not be empty")
	}

	// Verify operational flags
	if !cfg.KeepAlive {
		t.Error("KeepAlive should default to true")
	}

	if !cfg.KeepUpToDate {
		t.Error("KeepUpToDate should default to true")
	}

	// Verify CORS policy
	if !cfg.AllowAllOrigins {
		t.Error("AllowAllOrigins should default to true")
	}

	// Verify slices are initialized (not nil)
	if cfg.Repositories == nil {
		t.Error("Repositories should be initialized")
	}

	if cfg.Discoveries == nil {
		t.Error("Discoveries should be initialized")
	}

	if cfg.Dependencies == nil {
		t.Error("Dependencies should be initialized")
	}
}

// TestConfigValidation verifies config validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
	}{
		{
			name:    "valid default config",
			cfg:     DefaultConfig(),
			wantErr: false,
		},
		{
			name: "missing name",
			cfg: &Config{
				Port:     10000,
				Proxy:    10001,
				Protocol: "grpc",
				Version:  "0.0.1",
			},
			wantErr: true,
		},
		{
			name: "invalid port (zero)",
			cfg: &Config{
				Name:     "test.Service",
				Port:     0,
				Proxy:    10001,
				Protocol: "grpc",
				Version:  "0.0.1",
			},
			wantErr: true,
		},
		{
			name: "invalid port (too high)",
			cfg: &Config{
				Name:     "test.Service",
				Port:     70000,
				Proxy:    10001,
				Protocol: "grpc",
				Version:  "0.0.1",
			},
			wantErr: true,
		},
		{
			name: "missing protocol",
			cfg: &Config{
				Name:    "test.Service",
				Port:    10000,
				Proxy:   10001,
				Version: "0.0.1",
			},
			wantErr: true,
		},
		{
			name: "missing version",
			cfg: &Config{
				Name:     "test.Service",
				Port:     10000,
				Proxy:    10001,
				Protocol: "grpc",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestConfigFileOperations verifies file-based config save/load
func TestConfigFileOperations(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	configPath := tmpDir + "/echo-config.json"

	// Create config
	cfg := DefaultConfig()
	cfg.ID = "test-echo-123"
	cfg.ConfigPath = configPath

	// Save to file
	if err := cfg.SaveToFile(configPath); err != nil {
		t.Fatalf("SaveToFile() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load from file
	loaded, err := LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("LoadFromFile() failed: %v", err)
	}

	// Verify loaded config matches original
	if loaded.ID != cfg.ID {
		t.Errorf("Loaded ID = %q, want %q", loaded.ID, cfg.ID)
	}

	if loaded.Name != cfg.Name {
		t.Errorf("Loaded Name = %q, want %q", loaded.Name, cfg.Name)
	}

	if loaded.Port != cfg.Port {
		t.Errorf("Loaded Port = %d, want %d", loaded.Port, cfg.Port)
	}
}

// TestConfigClone verifies deep copy functionality
func TestConfigClone(t *testing.T) {
	original := DefaultConfig()
	original.Keywords = []string{"Test", "Echo"}
	original.Repositories = []string{"repo1"}
	original.Discoveries = []string{"disc1"}
	original.Dependencies = []string{"dep1"}

	// Clone
	clone := original.Clone()

	// Modify original slices
	original.Keywords[0] = "Modified"
	original.Repositories = append(original.Repositories, "repo2")
	original.Discoveries = append(original.Discoveries, "disc2")
	original.Dependencies = append(original.Dependencies, "dep2")

	// Verify clone is independent
	if clone.Keywords[0] == "Modified" {
		t.Error("Clone Keywords were modified (not a deep copy)")
	}

	if len(clone.Repositories) != 1 {
		t.Errorf("Clone Repositories length = %d, want 1", len(clone.Repositories))
	}

	if len(clone.Discoveries) != 1 {
		t.Errorf("Clone Discoveries length = %d, want 1", len(clone.Discoveries))
	}

	if len(clone.Dependencies) != 1 {
		t.Errorf("Clone Dependencies length = %d, want 1", len(clone.Dependencies))
	}
}

// TestConfigEnvironmentVariables verifies env var handling
func TestConfigEnvironmentVariables(t *testing.T) {
	// Set test environment variables
	os.Setenv("GLOBULAR_DOMAIN", "test.globular.local")
	os.Setenv("GLOBULAR_ADDRESS", "test.globular.local:8080")
	defer func() {
		os.Unsetenv("GLOBULAR_DOMAIN")
		os.Unsetenv("GLOBULAR_ADDRESS")
	}()

	cfg := DefaultConfig()

	if cfg.Domain != "test.globular.local" {
		t.Errorf("Domain = %q, want %q (from env)", cfg.Domain, "test.globular.local")
	}

	if cfg.Address != "test.globular.local:8080" {
		t.Errorf("Address = %q, want %q (from env)", cfg.Address, "test.globular.local:8080")
	}
}
