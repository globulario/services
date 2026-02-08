package globular_service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigValidator defines the interface for validatable configurations.
type ConfigValidator interface {
	Validate() error
}

// ConfigPersistence provides common configuration persistence methods.
//
// Phase 2 Step 3: Extracted common config operations from Echo, Discovery, and Repository.
//
// Services can use these helper functions to reduce duplication in their config.go files.

// SaveConfigToFile writes a configuration to a local JSON file.
// This is a helper that services can use in their SaveToFile() methods.
func SaveConfigToFile(cfg any, path string) error {
	if path == "" {
		return fmt.Errorf("config path is required")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadConfigFromFile reads a configuration from a local JSON file.
// The cfg parameter should be a pointer to the config struct to unmarshal into.
func LoadConfigFromFile(path string, cfg any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	return nil
}

// ValidateCommonFields validates common configuration fields that all services share.
// Services should call this from their Validate() method, then add service-specific validation.
func ValidateCommonFields(name string, port, proxy int, protocol, version string) error {
	if name == "" {
		return fmt.Errorf("service name is required")
	}

	if port <= 0 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}

	if proxy <= 0 || proxy > 65535 {
		return fmt.Errorf("proxy port must be between 1 and 65535, got %d", proxy)
	}

	if protocol == "" {
		return fmt.Errorf("protocol is required")
	}

	if version == "" {
		return fmt.Errorf("version is required")
	}

	return nil
}

// CloneStringSlice creates a deep copy of a string slice.
// Services use this in their Clone() methods to ensure proper deep copying.
func CloneStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	return append([]string(nil), src...)
}

// GetDefaultDomainAddress returns default domain and address values from environment.
// Services use this in their DefaultConfig() functions.
func GetDefaultDomainAddress(port int) (domain string, address string) {
	if v := os.Getenv("GLOBULAR_DOMAIN"); v != "" {
		domain = v
	} else {
		domain = "localhost"
	}

	if v := os.Getenv("GLOBULAR_ADDRESS"); v != "" {
		address = v
	} else {
		address = fmt.Sprintf("localhost:%d", port)
	}

	return domain, address
}
