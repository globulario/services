package main

import (
    globular "github.com/globulario/services/golang/globular_service"
)

// Config captures the blog service configuration (Phase 2 layout).
type Config struct {
    // Service identity
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

    // Dependencies
    Dependencies []string `json:"Dependencies"`

    // Policy & operations
    AllowAllOrigins bool   `json:"AllowAllOrigins"`
    AllowedOrigins  string `json:"AllowedOrigins"`
    KeepUpToDate    bool   `json:"KeepUpToDate"`
    KeepAlive       bool   `json:"KeepAlive"`

    // Platform metadata & runtime state
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

    // TLS configuration
    TLS struct {
        Enabled            bool   `json:"TLS"`
        CertFile           string `json:"CertFile"`
        KeyFile            string `json:"KeyFile"`
        CertAuthorityTrust string `json:"CertAuthorityTrust"`
    } `json:"TLS"`

    // Permissions metadata
    Permissions []any `json:"Permissions"`

    // Blog-specific fields
    Root string `json:"Root"`
}

// DefaultConfig returns defaults matching the legacy blog server behavior.
func DefaultConfig() *Config {
    cfg := &Config{
        Name:        "blog.BlogService",
        Port:        defaultPort,
        Proxy:       defaultProxy,
        Protocol:    "grpc",
        Version:     "0.0.1",
        PublisherID: "localhost",
        Description: "Blog service",
        Keywords:    []string{"Example", "Blog", "Post", "Service"},

        Repositories: []string{},
        Discoveries:  []string{},
        Dependencies: []string{"event.EventService", "rbac.RbacService", "log.LogService"},

        AllowAllOrigins: allowAllOrigins,
        AllowedOrigins:  allowedOriginsStr,
        KeepUpToDate:    true,
        KeepAlive:       true,

        Process:      -1,
        ProxyProcess: -1,

        Permissions: []any{},
        Root:        "",
    }

    cfg.TLS.Enabled = false

    // Set default domain and address using shared helper.
    cfg.Domain, cfg.Address = globular.GetDefaultDomainAddress(cfg.Port)

    return cfg
}

// Validate ensures the configuration has valid common fields.
func (c *Config) Validate() error {
    return globular.ValidateCommonFields(c.Name, c.Port, c.Proxy, c.Protocol, c.Version)
}

// SaveToFile writes the configuration to a JSON file.
func (c *Config) SaveToFile(path string) error {
    return globular.SaveConfigToFile(c, path)
}

// LoadFromFile reads the configuration from a JSON file.
func LoadFromFile(path string) (*Config, error) {
    cfg := &Config{}
    if err := globular.LoadConfigFromFile(path, cfg); err != nil {
        return nil, err
    }
    return cfg, nil
}

// Clone produces a deep copy of the configuration.
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
        Root:            c.Root,
    }

    // Deep copy TLS block
    clone.TLS.Enabled = c.TLS.Enabled
    clone.TLS.CertFile = c.TLS.CertFile
    clone.TLS.KeyFile = c.TLS.KeyFile
    clone.TLS.CertAuthorityTrust = c.TLS.CertAuthorityTrust

    if c.Permissions != nil {
        clone.Permissions = make([]any, len(c.Permissions))
        copy(clone.Permissions, c.Permissions)
    }

    return clone
}
