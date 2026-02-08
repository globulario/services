package main

import (
	"fmt"

	"github.com/globulario/services/golang/globular_service"
)

// Config represents the Discovery service configuration.
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

	// RBAC Permissions
	// Discovery service has complex permission structures for PublishService and PublishApplication
	Permissions []any `json:"Permissions"`
}

// DefaultConfig returns a Config with Discovery service defaults.
func DefaultConfig() *Config {
	cfg := &Config{
		Name:        "discovery.PackageDiscovery",
		Port:        10029,
		Proxy:       10030,
		Protocol:    "grpc",
		Version:     "0.0.1",
		PublisherID: "localhost",
		Description: "Service discovery client",
		Keywords:    []string{"Discovery", "Package", "Service", "Application"},

		// Service discovery
		Repositories: []string{},
		Discoveries:  []string{},

		// Dependencies
		Dependencies: []string{"rbac.RbacService", "resource.ResourceService"},

		// Policy
		AllowAllOrigins: true,
		AllowedOrigins:  "",
		KeepUpToDate:    true,
		KeepAlive:       true,

		// Runtime
		Process:      -1,
		ProxyProcess: -1,

		// RBAC Permissions - PublishService and PublishApplication
		Permissions: []any{
			// PublishService permission
			map[string]any{
				"action":     "/discovery.PackageDiscovery/PublishService",
				"permission": "write",
				"resources": []any{
					map[string]any{"index": 0, "field": "RepositoryId", "permission": "write"},
					map[string]any{"index": 0, "field": "DiscoveryId", "permission": "write"},
				},
			},
			// PublishApplication permission
			map[string]any{
				"action":     "/discovery.PackageDiscovery/PublishApplication",
				"permission": "write",
				"resources": []any{
					map[string]any{"index": 0, "field": "Repository", "permission": "write"},
					map[string]any{"index": 0, "field": "Discovery", "permission": "write"},
				},
			},
		},
	}

	cfg.TLS.Enabled = false

	// Set default domain and address from environment or use localhost
	cfg.Domain, cfg.Address = globular_service.GetDefaultDomainAddress(cfg.Port)

	return cfg
}

// Validate checks that required configuration fields are set correctly.
func (c *Config) Validate() error {
	// Validate common fields
	if err := globular_service.ValidateCommonFields(c.Name, c.Port, c.Proxy, c.Protocol, c.Version); err != nil {
		return err
	}

	// Discovery-specific validation: validate dependencies are present
	if len(c.Dependencies) == 0 {
		return fmt.Errorf("dependencies are required (rbac.RbacService, resource.ResourceService)")
	}

	// Discovery-specific validation: validate permissions structure
	if len(c.Permissions) == 0 {
		return fmt.Errorf("RBAC permissions are required")
	}

	return nil
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
	}

	// Deep copy TLS
	clone.TLS.Enabled = c.TLS.Enabled
	clone.TLS.CertFile = c.TLS.CertFile
	clone.TLS.KeyFile = c.TLS.KeyFile
	clone.TLS.CertAuthorityTrust = c.TLS.CertAuthorityTrust

	// Deep copy permissions (complex nested structures)
	if c.Permissions != nil {
		clone.Permissions = make([]any, len(c.Permissions))
		for i, perm := range c.Permissions {
			if permMap, ok := perm.(map[string]any); ok {
				clonedPerm := make(map[string]any)
				for k, v := range permMap {
					// Handle nested resources array
					if k == "resources" {
						if resources, ok := v.([]any); ok {
							clonedResources := make([]any, len(resources))
							for j, res := range resources {
								if resMap, ok := res.(map[string]any); ok {
									clonedRes := make(map[string]any)
									for rk, rv := range resMap {
										clonedRes[rk] = rv
									}
									clonedResources[j] = clonedRes
								} else {
									clonedResources[j] = res
								}
							}
							clonedPerm[k] = clonedResources
						} else {
							clonedPerm[k] = v
						}
					} else {
						clonedPerm[k] = v
					}
				}
				clone.Permissions[i] = clonedPerm
			} else {
				clone.Permissions[i] = perm
			}
		}
	}

	return clone
}
