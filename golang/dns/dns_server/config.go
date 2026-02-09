package main

import (
	"fmt"
	"strconv"

	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
)

// Config captures DNS service configuration persisted to disk.
type Config struct {
	ID                 string   `json:"Id"`
	Name               string   `json:"Name"`
	Domain             string   `json:"Domain"`
	Address            string   `json:"Address"`
	Port               int      `json:"Port"`
	Proxy              int      `json:"Proxy"`
	Protocol           string   `json:"Protocol"`
	Version            string   `json:"Version"`
	PublisherID        string   `json:"PublisherId"`
	Description        string   `json:"Description"`
	Keywords           []string `json:"Keywords"`
	Repositories       []string `json:"Repositories"`
	Discoveries        []string `json:"Discoveries"`
	Dependencies       []string `json:"Dependencies"`
	AllowAllOrigins    bool     `json:"AllowAllOrigins"`
	AllowedOrigins     string   `json:"AllowedOrigins"`
	KeepUpToDate       bool     `json:"KeepUpToDate"`
	KeepAlive          bool     `json:"KeepAlive"`
	Platform           string   `json:"Platform"`
	Checksum           string   `json:"Checksum"`
	Path               string   `json:"Path"`
	Proto              string   `json:"Proto"`
	Mac                string   `json:"Mac"`
	Process            int      `json:"Process"`
	ProxyProcess       int      `json:"ProxyProcess"`
	ConfigPath         string   `json:"ConfigPath"`
	LastError          string   `json:"LastError"`
	ModTime            int64    `json:"ModTime"`
	State              string   `json:"State"`
	CertFile           string   `json:"CertFile"`
	KeyFile            string   `json:"KeyFile"`
	CertAuthorityTrust string   `json:"CertAuthorityTrust"`
	TLS                bool     `json:"TLS"`

	// DNS-specific
	DnsPort           int      `json:"DnsPort"`
	Domains           []string `json:"Domains"`
	ReplicationFactor int      `json:"ReplicationFactor"`
	Root              string   `json:"Root"`

	Permissions []any `json:"Permissions"`
}

// DefaultConfig returns the default DNS configuration (matches historical defaults).
func DefaultConfig() *Config {
	cfg := &Config{
		Name:              "dns.DnsService",
		Domain:            "globular.internal",
		Address:           "127.0.0.1:10006",
		Port:              defaultPort,
		Proxy:             defaultProxy,
		Protocol:          "grpc",
		Version:           "0.0.1",
		PublisherID:       "globular.internal",
		Description:       "DNS service",
		Keywords:          []string{"DNS", "Records", "Resolver"},
		Repositories:      []string{},
		Discoveries:       []string{"log.LogService", "rbac.RbacService"},
		Dependencies:      []string{},
		AllowAllOrigins:   allowAllOrigins,
		AllowedOrigins:    allowedOriginsStr,
		KeepUpToDate:      true,
		KeepAlive:         true,
		Process:           -1,
		ProxyProcess:      -1,
		DnsPort:           53,
		Domains:           []string{},
		ReplicationFactor: 0,
		Root:              "",
		Permissions:       nil,
	}

	cfg.Root = config.GetDataDir()
	// fill Address using actual port in case defaults are tweaked
	cfg.Address = "127.0.0.1:" + strconv.Itoa(cfg.Port)

	return cfg
}

// Validate performs common and DNS-specific validation.
func (c *Config) Validate() error {
	if err := globular.ValidateCommonFields(c.Name, c.Port, c.Proxy, c.Protocol, c.Version); err != nil {
		return err
	}
	if c.DnsPort <= 0 || c.DnsPort > 65535 {
		return fmt.Errorf("dns port must be between 1 and 65535, got %d", c.DnsPort)
	}
	if c.Root == "" {
		return fmt.Errorf("storage root is required")
	}
	return nil
}

// SaveToFile persists the configuration to disk.
func (c *Config) SaveToFile(path string) error { return globular.SaveConfigToFile(c, path) }

// LoadFromFile loads configuration from disk.
func LoadFromFile(path string) (*Config, error) {
	cfg := &Config{}
	if err := globular.LoadConfigFromFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Clone performs a deep copy.
func (c *Config) Clone() *Config {
	clone := *c
	clone.Keywords = globular.CloneStringSlice(c.Keywords)
	clone.Repositories = globular.CloneStringSlice(c.Repositories)
	clone.Discoveries = globular.CloneStringSlice(c.Discoveries)
	clone.Dependencies = globular.CloneStringSlice(c.Dependencies)
	clone.Domains = globular.CloneStringSlice(c.Domains)

	if c.Permissions != nil {
		clone.Permissions = make([]any, len(c.Permissions))
		copy(clone.Permissions, c.Permissions)
	}
	return &clone
}
