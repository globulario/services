package main

import (
	"encoding/json"

	globular "github.com/globulario/services/golang/globular_service"
)

// Config captures catalog service configuration.
type Config struct {
	ID          string   `json:"Id"`
	Name        string   `json:"Name"`
	Domain      string   `json:"Domain"`
	Address     string   `json:"Address"`
	Port        int      `json:"Port"`
	Proxy       int      `json:"Proxy"`
	Protocol    string   `json:"Protocol"`
	Version     string   `json:"Version"`
	PublisherID string   `json:"PublisherId"`
	Description string   `json:"Description"`
	Keywords    []string `json:"Keywords"`

	Repositories []string `json:"Repositories"`
	Discoveries  []string `json:"Discoveries"`
	Dependencies []string `json:"Dependencies"`

	AllowAllOrigins bool   `json:"AllowAllOrigins"`
	AllowedOrigins  string `json:"AllowedOrigins"`
	KeepUpToDate    bool   `json:"KeepUpToDate"`
	KeepAlive       bool   `json:"KeepAlive"`

	Platform     string `json:"Platform"`
	Checksum     string `json:"Checksum"`
	Path         string `json:"Path"`
	Proto        string `json:"Proto"`
	Mac          string `json:"Mac"`
	Process      int    `json:"Process"`
	ProxyProcess int    `json:"ProxyProcess"`
	ConfigPath   string `json:"ConfigPath"`
	LastError    string `json:"LastError"`
	ModTime      int64  `json:"ModTime"`
	State        string `json:"State"`

	TLS struct {
		Enabled            bool   `json:"TLS"`
		CertFile           string `json:"CertFile"`
		KeyFile            string `json:"KeyFile"`
		CertAuthorityTrust string `json:"CertAuthorityTrust"`
	} `json:"TLS"`

	Permissions []any                  `json:"Permissions"`
	Services    map[string]interface{} `json:"Services"`
}

// DefaultConfig returns defaults matching the legacy catalog server.
func DefaultConfig() *Config {
	cfg := &Config{
		Name:        "catalog.CatalogService",
		Port:        defaultPort,
		Proxy:       defaultProxy,
		Protocol:    "grpc",
		Version:     "0.0.1",
		PublisherID: "localhost",
		Description: "Catalog service",

		Repositories: []string{},
		Discoveries:  []string{},
		Dependencies: []string{},

		AllowAllOrigins: allowAllOrigins,
		AllowedOrigins:  allowedOriginsStr,
		KeepUpToDate:    true,
		KeepAlive:       true,

		Process:      -1,
		ProxyProcess: -1,

		Permissions: []any{},
		Services:    map[string]interface{}{},
	}

	cfg.TLS.Enabled = false
	cfg.Domain, cfg.Address = globular.GetDefaultDomainAddress(cfg.Port)

	return cfg
}

// Validate checks common fields.
func (c *Config) Validate() error {
	return globular.ValidateCommonFields(c.Name, c.Port, c.Proxy, c.Protocol, c.Version)
}

// SaveToFile writes config to JSON file.
func (c *Config) SaveToFile(path string) error {
	return globular.SaveConfigToFile(c, path)
}

// LoadFromFile reads config from JSON file.
func LoadFromFile(path string) (*Config, error) {
	cfg := &Config{}
	if err := globular.LoadConfigFromFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Clone creates a deep copy.
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
		Keywords:        globular.CloneStringSlice(c.Keywords),
		Repositories:    globular.CloneStringSlice(c.Repositories),
		Discoveries:     globular.CloneStringSlice(c.Discoveries),
		Dependencies:    globular.CloneStringSlice(c.Dependencies),
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
		ConfigPath:      c.ConfigPath,
		LastError:       c.LastError,
		ModTime:         c.ModTime,
		State:           c.State,
		Permissions:     nil,
		Services:        nil,
	}

	clone.TLS.Enabled = c.TLS.Enabled
	clone.TLS.CertFile = c.TLS.CertFile
	clone.TLS.KeyFile = c.TLS.KeyFile
	clone.TLS.CertAuthorityTrust = c.TLS.CertAuthorityTrust

	if c.Permissions != nil {
		clone.Permissions = make([]any, len(c.Permissions))
		copy(clone.Permissions, c.Permissions)
	}

	if c.Services != nil {
		raw, _ := json.Marshal(c.Services)
		_ = json.Unmarshal(raw, &clone.Services)
	}

	return clone
}
