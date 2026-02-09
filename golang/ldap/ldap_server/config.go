package main

import (
	"encoding/json"
	"fmt"

	globular "github.com/globulario/services/golang/globular_service"
)

// Config captures LDAP service configuration.
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

	// LDAP specific
	LdapListenAddr  string                 `json:"LdapListenAddr"`
	LdapsListenAddr string                 `json:"LdapsListenAddr"`
	DisableLDAPS    bool                   `json:"DisableLDAPS"`
	Connections     map[string]connection  `json:"Connections"`
	LdapSyncInfos   map[string]interface{} `json:"LdapSyncInfos"`

	Permissions []any `json:"Permissions"`
}

// DefaultConfig matches historical defaults.
func DefaultConfig() *Config {
	cfg := &Config{
		Name:            "ldap.LdapService",
		Port:            defaultPort,
		Proxy:           defaultProxy,
		Protocol:        "grpc",
		Version:         "0.0.1",
		PublisherID:     "localhost",
		Description:     "LDAP service",
		Keywords:        []string{"LDAP", "Directory"},
		Repositories:    []string{},
		Discoveries:     []string{},
		Dependencies:    []string{"rbac.RbacService"},
		AllowAllOrigins: allowAllOrigins,
		AllowedOrigins:  allowedOriginsStr,
		KeepUpToDate:    true,
		KeepAlive:       true,
		Process:         -1,
		ProxyProcess:    -1,
		LdapListenAddr:  "0.0.0.0:389",
		LdapsListenAddr: "0.0.0.0:636",
		DisableLDAPS:    false,
		Connections:     map[string]connection{},
		LdapSyncInfos:   map[string]interface{}{},
		Permissions:     nil,
	}
	cfg.Domain, cfg.Address = globular.GetDefaultDomainAddress(cfg.Port)
	return cfg
}

func (c *Config) Validate() error {
	if err := globular.ValidateCommonFields(c.Name, c.Port, c.Proxy, c.Protocol, c.Version); err != nil {
		return err
	}
	if c.LdapListenAddr == "" {
		return fmt.Errorf("LdapListenAddr is required")
	}
	if c.LdapsListenAddr == "" {
		return fmt.Errorf("LdapsListenAddr is required")
	}
	return nil
}

func (c *Config) SaveToFile(path string) error { return globular.SaveConfigToFile(c, path) }

func LoadFromFile(path string) (*Config, error) {
	cfg := &Config{}
	if err := globular.LoadConfigFromFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Clone() *Config {
	clone := *c
	clone.Keywords = globular.CloneStringSlice(c.Keywords)
	clone.Repositories = globular.CloneStringSlice(c.Repositories)
	clone.Discoveries = globular.CloneStringSlice(c.Discoveries)
	clone.Dependencies = globular.CloneStringSlice(c.Dependencies)

	if c.Connections != nil {
		clone.Connections = make(map[string]connection, len(c.Connections))
		for k, v := range c.Connections {
			// copy without live ldap.Conn pointer
			v.conn = nil
			clone.Connections[k] = v
		}
	}

	if c.LdapSyncInfos != nil {
		var buf []byte
		buf, _ = json.Marshal(c.LdapSyncInfos)
		tmp := map[string]interface{}{}
		_ = json.Unmarshal(buf, &tmp)
		clone.LdapSyncInfos = tmp
	}

	if c.Permissions != nil {
		clone.Permissions = make([]any, len(c.Permissions))
		copy(clone.Permissions, c.Permissions)
	}
	return &clone
}
