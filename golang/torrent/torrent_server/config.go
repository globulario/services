package main

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
)

// Config captures torrent service configuration.
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
	TLS                bool     `json:"TLS"`
	CertFile           string   `json:"CertFile"`
	KeyFile            string   `json:"KeyFile"`
	CertAuthorityTrust string   `json:"CertAuthorityTrust"`

	// Torrent specific
	DownloadDir    string `json:"DownloadDir"`
	Seed           bool   `json:"Seed"`
	UseMinio       bool   `json:"UseMinio"`
	MinioEndpoint  string `json:"MinioEndpoint"`
	MinioAccessKey string `json:"MinioAccessKey"`
	MinioSecretKey string `json:"MinioSecretKey"`
	MinioBucket    string `json:"MinioBucket"`
	MinioPrefix    string `json:"MinioPrefix"`
	MinioUseSSL    bool   `json:"MinioUseSSL"`
	Permissions    []any  `json:"Permissions"`
}

func DefaultConfig() *Config {
	cfg := &Config{
		Name:            "torrent.TorrentService",
		Port:            defaultPort,
		Proxy:           defaultProxy,
		Protocol:        "grpc",
		Version:         "0.0.1",
		PublisherID:     "localhost",
		Description:     "Torrent gRPC service for Globular.",
		Keywords:        []string{"Torrent", "Download", "P2P", "Service"},
		Repositories:    []string{},
		Discoveries:     []string{},
		Dependencies:    []string{},
		AllowAllOrigins: allowAllOrigins,
		AllowedOrigins:  allowedOriginsStr,
		KeepUpToDate:    true,
		KeepAlive:       true,
		Process:         -1,
		ProxyProcess:    -1,
		DownloadDir:     config.GetDataDir() + "/torrents",
		Seed:            false,
		UseMinio:        false,
		MinioPrefix:     "/users",
		MinioUseSSL:     false,
		Permissions:     loadDefaultPermissions(),
	}
	cfg.Domain, cfg.Address = globular.GetDefaultDomainAddress(cfg.Port)
	return cfg
}

func (c *Config) Validate() error {
	if err := globular.ValidateCommonFields(c.Name, c.Port, c.Proxy, c.Protocol, c.Version); err != nil {
		return err
	}
	if strings.TrimSpace(c.DownloadDir) == "" {
		return fmt.Errorf("DownloadDir is required")
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
	if c.Permissions != nil {
		clone.Permissions = make([]any, len(c.Permissions))
		copy(clone.Permissions, c.Permissions)
	}
	return &clone
}
