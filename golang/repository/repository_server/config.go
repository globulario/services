package main

import (
	"github.com/globulario/services/golang/globular_service"
)

// Config represents the Repository service configuration.
// Phase 1 Step 1: Extracted from server struct for clean separation of concerns.
type Config struct {
	// Core service identity
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Domain      string `json:"Domain"`
	Address     string `json:"Address"`
	Port        int    `json:"Port"`
	Proxy       int    `json:"Proxy"`
	Protocol    string `json:"Protocol"`
	Version     string `json:"Version"`
	PublisherID string `json:"PublisherId"`
	Description string `json:"Description"`
	Keywords    []string `json:"Keywords"`

	// Service discovery
	Repositories []string `json:"Repositories"`
	Discoveries  []string `json:"Discoveries"`

	// Dependencies
	Dependencies []string `json:"Dependencies"`

	// Policy & Operations
	AllowAllOrigins bool   `json:"AllowAllOrigins"`
	AllowedOrigins  string `json:"AllowedOrigins"`
	KeepUpToDate    bool   `json:"KeepUpToDate"`
	KeepAlive       bool   `json:"KeepAlive"`

	// Platform metadata
	Platform    string `json:"Platform"`
	Checksum    string `json:"Checksum"`
	Path        string `json:"Path"`
	Proto       string `json:"Proto"`
	Mac         string `json:"Mac"`

	// Runtime state
	Process      int    `json:"Process"`
	ProxyProcess int    `json:"ProxyProcess"`
	State        string `json:"State"`
	LastError    string `json:"LastError"`
	ModTime      int64  `json:"ModTime"`

	// TLS configuration
	TLS struct {
		Enabled            bool   `json:"TLS"`
		CertFile           string `json:"CertFile"`
		KeyFile            string `json:"KeyFile"`
		CertAuthorityTrust string `json:"CertAuthorityTrust"`
	} `json:"TLS"`

	// Permissions
	Permissions []any `json:"Permissions"`

	// Repository-specific
	Root string `json:"Root"` // Base data directory for package storage
}

// DefaultConfig returns a Config with Repository service defaults.
func DefaultConfig() *Config {
	cfg := &Config{
		Name:        "repository.PackageRepository",
		Port:        10000,
		Proxy:       10001,
		Protocol:    "grpc",
		Version:     "0.0.1",
		PublisherID: "localhost",
		Description: "Package repository for distributing services and applications",
		Keywords:    []string{"Package", "Repository"},

		// Service discovery
		Repositories: []string{},
		Discoveries:  []string{},

		// Dependencies
		Dependencies: []string{},

		// Policy
		AllowAllOrigins: true,
		AllowedOrigins:  "",
		KeepUpToDate:    true,
		KeepAlive:       true,

		// Runtime
		Process:      -1,
		ProxyProcess: -1,

		// Permissions
		Permissions: []any{},

		// Repository-specific
		Root: "", // Will be set during initialization
	}

	cfg.TLS.Enabled = false

	// Set default domain and address from environment or use localhost
	cfg.Domain, cfg.Address = globular_service.GetDefaultDomainAddress(cfg.Port)

	return cfg
}

// Validate checks that required configuration fields are set correctly.
func (c *Config) Validate() error {
	return globular_service.ValidateCommonFields(c.Name, c.Port, c.Proxy, c.Protocol, c.Version)
}

// SaveToFile writes the configuration to a JSON file.
func (c *Config) SaveToFile(path string) error {
	return globular_service.SaveConfigToFile(c, path)
}

// LoadFromFile reads configuration from a JSON file.
func LoadFromFile(path string) (*Config, error) {
	cfg := &Config{}
	if err := globular_service.LoadConfigFromFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Clone creates a deep copy of the configuration.
func (c *Config) Clone() *Config {
	clone := &Config{
		ID:              c.ID,
		Name:            c.Name,
		Domain:          c.Domain,
		Address:         c.Address,
		Port:            c.Port,
		Proxy:           c.Proxy,
		Protocol:        c.Protocol,
		Version:         c.Version,
		PublisherID:     c.PublisherID,
		Description:     c.Description,
		Keywords:        globular_service.CloneStringSlice(c.Keywords),
		Repositories:    globular_service.CloneStringSlice(c.Repositories),
		Discoveries:     globular_service.CloneStringSlice(c.Discoveries),
		Dependencies:    globular_service.CloneStringSlice(c.Dependencies),
		AllowAllOrigins: c.AllowAllOrigins,
		AllowedOrigins:  c.AllowedOrigins,
		KeepUpToDate:    c.KeepUpToDate,
		KeepAlive:       c.KeepAlive,
		Platform:        c.Platform,
		Checksum:        c.Checksum,
		Path:            c.Path,
		Proto:           c.Proto,
		Mac:             c.Mac,
		Process:         c.Process,
		ProxyProcess:    c.ProxyProcess,
		State:           c.State,
		LastError:       c.LastError,
		ModTime:         c.ModTime,
		Root:            c.Root,
	}

	// Deep copy TLS
	clone.TLS.Enabled = c.TLS.Enabled
	clone.TLS.CertFile = c.TLS.CertFile
	clone.TLS.KeyFile = c.TLS.KeyFile
	clone.TLS.CertAuthorityTrust = c.TLS.CertAuthorityTrust

	// Deep copy permissions
	if c.Permissions != nil {
		clone.Permissions = make([]any, len(c.Permissions))
		copy(clone.Permissions, c.Permissions)
	}

	return clone
}
