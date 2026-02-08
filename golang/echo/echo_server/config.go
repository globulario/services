package main

import (
	"github.com/globulario/services/golang/globular_service"
)

// Config holds the Echo service configuration.
// It separates declarative configuration from runtime state.
type Config struct {
	// Service identity
	ID          string   `json:"Id"`
	Name        string   `json:"Name"`
	Domain      string   `json:"Domain"`
	Address     string   `json:"Address"`
	Description string   `json:"Description"`
	Version     string   `json:"Version"`
	PublisherID string   `json:"PublisherId"`
	Keywords    []string `json:"Keywords"`

	// Network configuration
	Port     int    `json:"Port"`
	Proxy    int    `json:"Proxy"`
	Protocol string `json:"Protocol"`

	// Service discovery and dependencies
	Repositories []string `json:"Repositories"`
	Discoveries  []string `json:"Discoveries"`
	Dependencies []string `json:"Dependencies"`

	// CORS policy
	AllowAllOrigins bool   `json:"AllowAllOrigins"`
	AllowedOrigins  string `json:"AllowedOrigins"`

	// Operational flags
	KeepAlive    bool `json:"KeepAlive"`
	KeepUpToDate bool `json:"KeepUpToDate"`

	// TLS configuration
	TLS struct {
		Enabled            bool   `json:"TLS"`
		CertFile           string `json:"CertFile"`
		KeyFile            string `json:"KeyFile"`
		CertAuthorityTrust string `json:"CertAuthorityTrust"`
	} `json:"TLS"`

	// Configuration file path
	ConfigPath string `json:"ConfigPath"`

	// Legacy fields (for compatibility)
	Plaform string `json:"Plaform"` // typo preserved for compatibility
}

// DefaultConfig returns a Config with sensible defaults for the Echo service.
func DefaultConfig() *Config {
	cfg := &Config{
		Name:        "echo.EchoService",
		Port:        defaultPort,
		Proxy:       defaultProxy,
		Protocol:    "grpc",
		Version:     "0.0.1",
		PublisherID: "localhost",
		Description: "The Hello World of gRPC services.",
		Keywords:    []string{"Example", "Echo", "Test", "Service"},

		Repositories: make([]string, 0),
		Discoveries:  make([]string, 0),
		Dependencies: make([]string, 0),

		AllowAllOrigins: allowAllOrigins,
		AllowedOrigins:  allowedOriginsStr,
		KeepAlive:       true,
		KeepUpToDate:    true,
	}

	// Set default domain and address from environment or use localhost
	cfg.Domain, cfg.Address = globular_service.GetDefaultDomainAddress(cfg.Port)

	return cfg
}

// LoadConfig reads configuration from the Globular config backend (etcd).
// TODO: This will be implemented when we integrate with the full config system.
// For now, it's a placeholder that returns nil (uses defaults).
func LoadConfig(cfg *Config) error {
	// Placeholder: In the full implementation, this would load from etcd
	// For Phase 1, we're focusing on structure separation, not changing persistence
	return nil
}

// Save persists the configuration.
// TODO: This should eventually save to etcd via globular.SaveService()
// For Phase 1, this is a placeholder to maintain API compatibility.
func (c *Config) Save() error {
	// Placeholder: Actual persistence is handled by server.Save() for now
	// This will be properly implemented after refactoring is complete
	return nil
}

// SaveToFile writes the configuration to a local JSON file.
// This is a fallback for when etcd is unavailable.
func (c *Config) SaveToFile(path string) error {
	return globular_service.SaveConfigToFile(c, path)
}

// LoadFromFile reads configuration from a local JSON file.
func LoadFromFile(path string) (*Config, error) {
	cfg := &Config{}
	if err := globular_service.LoadConfigFromFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Validate checks that the configuration is valid and complete.
func (c *Config) Validate() error {
	return globular_service.ValidateCommonFields(c.Name, c.Port, c.Proxy, c.Protocol, c.Version)
}

// Clone creates a deep copy of the configuration.
func (c *Config) Clone() *Config {
	clone := *c
	clone.Keywords = globular_service.CloneStringSlice(c.Keywords)
	clone.Repositories = globular_service.CloneStringSlice(c.Repositories)
	clone.Discoveries = globular_service.CloneStringSlice(c.Discoveries)
	clone.Dependencies = globular_service.CloneStringSlice(c.Dependencies)
	return &clone
}
