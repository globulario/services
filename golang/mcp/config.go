package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"
)

// MCPConfig is the explicit configuration for the Globular MCP server.
type MCPConfig struct {
	Enabled        bool     `json:"enabled"`
	Transport      string   `json:"transport"`       // "http" (default) or "stdio"
	ReadOnly       bool     `json:"read_only"`       // true by default, blocks mutating tools
	DefaultTimeout Duration `json:"default_timeout"` // default 10s

	// Tool group toggles — each can be enabled/disabled independently.
	ToolGroups ToolGroupConfig `json:"tool_groups"`

	// Safety: allowlists for risky data surfaces.
	// If a surface is enabled but its allowlist is empty, access is denied.
	FileAllowedRoots          []string `json:"file_allowed_roots"`
	PersistenceAllowedConns   []string `json:"persistence_allowed_connections"`
	PersistenceAllowedDBs     []string `json:"persistence_allowed_databases"`
	PersistenceAllowedColls   []string `json:"persistence_allowed_collections"`
	StorageAllowedConns       []string `json:"storage_allowed_connections"`
	StorageAllowedKeyPrefixes []string `json:"storage_allowed_key_prefixes"`

	// Limits
	MaxResponseSize  int `json:"max_response_size"`   // bytes, default 1MB
	MaxResultCount   int `json:"max_result_count"`    // default 100
	MaxFileReadBytes int `json:"max_file_read_bytes"` // default 32768
	ConcurrencyLimit int `json:"concurrency_limit"`   // default 10

	// Redaction
	RedactFields []string `json:"redact_fields"` // additional sensitive field names

	// HTTP transport (cluster-facing mode)
	HTTPListenAddr   string   `json:"http_listen_addr"`   // e.g. ":10050", empty = disabled
	HTTPReadTimeout  Duration `json:"http_read_timeout"`  // default 30s
	HTTPWriteTimeout Duration `json:"http_write_timeout"` // default 60s

	// Audit
	AuditLog     bool   `json:"audit_log"`      // true by default
	AuditLogPath string `json:"audit_log_path"` // default stderr
}

// ToolGroupConfig controls which tool groups are registered.
type ToolGroupConfig struct {
	Cluster     bool `json:"cluster"`     // default true
	Doctor      bool `json:"doctor"`      // default true
	NodeAgent   bool `json:"nodeagent"`   // default true
	Repository  bool `json:"repository"`  // default true
	Backup      bool `json:"backup"`      // default true
	RBAC        bool `json:"rbac"`        // default true
	Resource    bool `json:"resource"`    // default true
	File        bool `json:"file"`        // default true (scoped to allowed roots)
	Persistence bool `json:"persistence"` // default false (requires allowlist)
	Storage     bool `json:"storage"`     // default false (requires allowlist)
	Composed    bool `json:"composed"`    // default true
	Auth        bool `json:"auth"`        // default false (deferred)
	DNS         bool `json:"dns"`         // default false (deferred)
}

// Duration wraps time.Duration for JSON marshal/unmarshal.
type Duration struct{ time.Duration }

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		// Try as number (seconds)
		var secs float64
		if err2 := json.Unmarshal(b, &secs); err2 == nil {
			d.Duration = time.Duration(secs * float64(time.Second))
			return nil
		}
		return err
	}
	var err error
	d.Duration, err = time.ParseDuration(s)
	return err
}

// defaultConfigPath is the canonical location for the MCP config file.
const defaultConfigPath = "/var/lib/globular/mcp/config.json"

func defaultConfig() *MCPConfig {
	return &MCPConfig{
		Enabled:        true,
		Transport:      "http",
		ReadOnly:       true,
		DefaultTimeout: Duration{10 * time.Second},
		ToolGroups: ToolGroupConfig{
			Cluster:     true,
			Doctor:      true,
			NodeAgent:   true,
			Repository:  true,
			Backup:      true,
			RBAC:        true,
			Resource:    true,
			Composed:    true,
			File:        true,
			Persistence: false, // requires explicit allowlist
			Storage:     false, // requires explicit allowlist
			Auth:        false, // deferred
			DNS:         false, // deferred
		},
		FileAllowedRoots:          []string{"/users", "/applications", "/var/lib/globular/webroot", "/var/lib/globular/data/files"},
		PersistenceAllowedConns:   []string{},
		PersistenceAllowedDBs:     []string{},
		PersistenceAllowedColls:   []string{},
		StorageAllowedConns:       []string{},
		StorageAllowedKeyPrefixes: []string{},
		MaxResponseSize:           1 << 20, // 1MB
		MaxResultCount:            100,
		MaxFileReadBytes:          32768,
		ConcurrencyLimit:          10,
		RedactFields:              nil, // uses built-in defaults
		HTTPListenAddr:            "127.0.0.1:10050",
		HTTPReadTimeout:           Duration{30 * time.Second},
		HTTPWriteTimeout:          Duration{60 * time.Second},
		AuditLog:                  true,
		AuditLogPath:              "", // stderr
	}
}

// loadConfig loads config from file, env var, or defaults.
// Search order: $GLOBULAR_MCP_CONFIG, /var/lib/globular/mcp/config.json, ~/.config/globular/mcp.json, defaults.
func loadConfig() *MCPConfig {
	cfg := defaultConfig()

	paths := []string{
		os.Getenv("GLOBULAR_MCP_CONFIG"),
		"/var/lib/globular/mcp/config.json",
		filepath.Join(os.Getenv("HOME"), ".config/globular/mcp.json"),
	}

	for _, p := range paths {
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(data, cfg); err != nil {
			log.Printf("mcp: warning: failed to parse config %s: %v", p, err)
			continue
		}
		log.Printf("mcp: loaded config from %s", p)
		return cfg
	}

	log.Println("mcp: no config file found, writing defaults to " + defaultConfigPath)
	writeDefaultConfig(cfg)
	return cfg
}

// writeDefaultConfig persists the default config so operators can discover and
// edit it. Errors are non-fatal — the server runs fine with in-memory defaults.
func writeDefaultConfig(cfg *MCPConfig) {
	dir := filepath.Dir(defaultConfigPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		log.Printf("mcp: cannot create config dir %s: %v", dir, err)
		return
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("mcp: cannot marshal default config: %v", err)
		return
	}
	if err := os.WriteFile(defaultConfigPath, data, 0640); err != nil {
		log.Printf("mcp: cannot write default config to %s: %v (running with in-memory defaults)", defaultConfigPath, err)
	}
}
