package main

import (
	"fmt"

	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/globular_service"
)

// Config represents the Authentication service configuration.
// It mirrors the server fields used by Globular and adds authentication-specific settings.
type Config struct {
	// Core identity
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

	// Service discovery
	Repositories []string `json:"Repositories"`
	Discoveries  []string `json:"Discoveries"`
	Dependencies []string `json:"Dependencies"`

	// Policy & operations
	AllowAllOrigins bool   `json:"AllowAllOrigins"`
	AllowedOrigins  string `json:"AllowedOrigins"`
	KeepUpToDate    bool   `json:"KeepUpToDate"`
	KeepAlive       bool   `json:"KeepAlive"`

	// Platform metadata
	Platform string `json:"Platform"`
	Checksum string `json:"Checksum"`
	Path     string `json:"Path"`
	Proto    string `json:"Proto"`
	Mac      string `json:"Mac"`

	// Configuration file path
	ConfigPath string `json:"ConfigPath"`
	ConfigPort int    `json:"ConfigPort"`

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

	// Authentication-specific fields
	WatchSessionsDelay int    `json:"WatchSessionsDelay"`
	SessionTimeout     int    `json:"SessionTimeout"`
	LdapConnectionId   string `json:"LdapConnectionId"`
	AdminEmail         string `json:"AdminEmail"`
	RootPassword       string `json:"RootPassword"`
}

// DefaultConfig returns a Config populated with the authentication service defaults.
func DefaultConfig() *Config {
	cfg := &Config{
		Name:        string(authenticationpb.File_authentication_proto.Services().Get(0).FullName()),
		Port:        defaultPort,
		Proxy:       defaultProxy,
		Protocol:    "grpc",
		Version:     "0.0.1",
		PublisherID: "localhost",
		Description: "Authentication service",
		Keywords:    []string{"Authentication"},

		Repositories: []string{},
		Discoveries:  []string{},
		Dependencies: []string{"event.EventService"},

		AllowAllOrigins: allowAllOrigins,
		AllowedOrigins:  allowedOriginsStr,
		KeepUpToDate:    true,
		KeepAlive:       true,

		Process:      -1,
		ProxyProcess: -1,

		Permissions: []any{
			map[string]any{
				"action": "/authentication.AuthenticationService/SetPassword",
				"resources": []any{
					map[string]any{"index": 0, "permission": "write"},
				},
			},
			map[string]any{
				"action":     "/authentication.AuthenticationService/SetRootPassword",
				"permission": "owner",
			},
			map[string]any{
				"action":     "/authentication.AuthenticationService/SetRootEmail",
				"permission": "owner",
			},
			map[string]any{
				"action": "/authentication.AuthenticationService/GeneratePeerToken",
				"resources": []any{
					map[string]any{"index": 0, "permission": "write"},
				},
			},
		},

		WatchSessionsDelay: 60,
		SessionTimeout:     15,
		LdapConnectionId:   "",
		RootPassword:       "adminadmin",
	}

	// Set default domain/address from environment or localhost.
	cfg.Domain, cfg.Address = globular_service.GetDefaultDomainAddress(cfg.Port)
	cfg.AdminEmail = fmt.Sprintf("sa@%s", cfg.Domain)

	// TLS defaults
	cfg.TLS.Enabled = false

	return cfg
}

// Validate checks common configuration fields.
func (c *Config) Validate() error {
	return globular_service.ValidateCommonFields(c.Name, c.Port, c.Proxy, c.Protocol, c.Version)
}

// SaveToFile writes the configuration to disk.
func (c *Config) SaveToFile(path string) error {
	return globular_service.SaveConfigToFile(c, path)
}

// LoadFromFile reads configuration from disk.
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
		ID:                 c.ID,
		Name:               c.Name,
		Domain:             c.Domain,
		Address:            c.Address,
		Port:               c.Port,
		Proxy:              c.Proxy,
		Protocol:           c.Protocol,
		Version:            c.Version,
		PublisherID:        c.PublisherID,
		Description:        c.Description,
		Keywords:           globular_service.CloneStringSlice(c.Keywords),
		Repositories:       globular_service.CloneStringSlice(c.Repositories),
		Discoveries:        globular_service.CloneStringSlice(c.Discoveries),
		Dependencies:       globular_service.CloneStringSlice(c.Dependencies),
		AllowAllOrigins:    c.AllowAllOrigins,
		AllowedOrigins:     c.AllowedOrigins,
		KeepUpToDate:       c.KeepUpToDate,
		KeepAlive:          c.KeepAlive,
		Platform:           c.Platform,
		Checksum:           c.Checksum,
		Path:               c.Path,
		Proto:              c.Proto,
		Mac:                c.Mac,
		ConfigPath:         c.ConfigPath,
		ConfigPort:         c.ConfigPort,
		Process:            c.Process,
		ProxyProcess:       c.ProxyProcess,
		State:              c.State,
		LastError:          c.LastError,
		ModTime:            c.ModTime,
		WatchSessionsDelay: c.WatchSessionsDelay,
		SessionTimeout:     c.SessionTimeout,
		LdapConnectionId:   c.LdapConnectionId,
		AdminEmail:         c.AdminEmail,
		RootPassword:       c.RootPassword,
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
