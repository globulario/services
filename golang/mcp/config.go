package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/config"
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
	HTTPListenAddr    string   `json:"http_listen_addr"`    // e.g. ":10260", empty = disabled
	HTTPReadTimeout   Duration `json:"http_read_timeout"`   // default 30s
	HTTPWriteTimeout  Duration `json:"http_write_timeout"`  // default 60s
	HTTPUseTLS        bool     `json:"http_use_tls"`        // serve HTTPS if true
	HTTPTLSCertFile   string   `json:"http_tls_cert_file"`  // path to TLS cert (PEM)
	HTTPTLSKeyFile    string   `json:"http_tls_key_file"`   // path to TLS key (PEM)
	HTTPAdvertiseHost string   `json:"http_advertise_host"` // optional host to publish in .mcp.json

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
	CLI         bool `json:"cli"`         // default true
	Governor    bool `json:"governor"`    // default true
	Memory      bool `json:"memory"`      // default true (AI memory service)
	Skills      bool `json:"skills"`      // default true (operational skill playbooks)
	Workflow    bool `json:"workflow"`    // default true (reconciliation workflow tracing)
	Etcd        bool `json:"etcd"`        // default true (direct etcd access)
	Title       bool `json:"title"`       // default true (search index tools)
	Frontend    bool `json:"frontend"`    // default true (gRPC service map, web probe)
	Proto       bool `json:"proto"`       // default true (gRPC reflection describe)
	HTTPDiag    bool `json:"http_diag"`   // default true (HTTP latency diagnostics)
	Monitoring  bool `json:"monitoring"`  // default true (Prometheus metrics)
	Browser     bool `json:"browser"`     // default true (Chrome DevTools Protocol bridge)
	AIExecutor  bool `json:"ai_executor"` // default true (AI executor peer collaboration)
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
var (
	serviceCertPath = "/var/lib/globular/pki/issued/services/service.crt"
	serviceKeyPath  = "/var/lib/globular/pki/issued/services/service.key"
)

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
			CLI:         true,
			Governor:    true,
			Memory:      true,
			Skills:      true,
			Workflow:    true,
			File:        true,
			Persistence: false, // requires explicit allowlist
			Storage:     false, // requires explicit allowlist
			Etcd:        true,
			Title:       true,
			Frontend:    true,
			Proto:       true,
			HTTPDiag:    true,
			Monitoring:  true,
			Browser:     true,
			AIExecutor:  true,
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
		HTTPListenAddr:            "", // resolved from etcd at startup
		HTTPReadTimeout:           Duration{30 * time.Second},
		HTTPWriteTimeout:          Duration{60 * time.Second},
		HTTPUseTLS:                true,
		HTTPTLSCertFile:           serviceCertPath,
		HTTPTLSKeyFile:            serviceKeyPath,
		HTTPAdvertiseHost:         "",
		AuditLog:                  true,
		AuditLogPath:              "", // stderr
	}
}

// loadConfig loads config from the canonical path. If missing, it writes
// defaults. If the file exists, it is never overwritten (no auto-rewrite).
// The GLOBULAR_MCP_CONFIG env var overrides the default path, which is
// useful for running the MCP server in stdio mode from a dev checkout.
func loadConfig() *MCPConfig {
	cfg := defaultConfig()

	configPath := defaultConfigPath
	if override := os.Getenv("GLOBULAR_MCP_CONFIG"); override != "" {
		configPath = override
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		log.Println("mcp: no config file found, writing defaults to " + configPath)
		// Enable TLS by default when certs are present.
		_ = maybeEnableTLSFromServiceCert(cfg)
		writeConfig(configPath, cfg)
		return cfg
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		log.Printf("mcp: warning: failed to parse config %s: %v", configPath, err)
		return cfg
	}

	log.Printf("mcp: loaded config from %s", configPath)

	// Apply defaults for new tool groups that may be missing from
	// older config files. When a bool field is absent from the JSON
	// object, Go decodes it as false — so we check the raw JSON
	// and restore the default for any missing field. Do NOT rewrite
	// the file; keep runtime-only changes in memory.
	applyToolGroupDefaults(data, cfg)

	// Enforce TLS at runtime if certs are present, but do not rewrite config.
	_ = maybeEnableTLSFromServiceCert(cfg)

	return cfg
}

// applyToolGroupDefaults re-applies default=true for tool group fields that
// are missing from the on-disk config (e.g. added in a newer version).
func applyToolGroupDefaults(rawJSON []byte, cfg *MCPConfig) {
	// Parse just the tool_groups section to see which keys are present.
	var raw struct {
		ToolGroups map[string]json.RawMessage `json:"tool_groups"`
	}
	if err := json.Unmarshal(rawJSON, &raw); err != nil || raw.ToolGroups == nil {
		return
	}

	// These tool groups default to true when absent from config.
	defaultTrue := map[string]*bool{
		"cli":        &cfg.ToolGroups.CLI,
		"governor":   &cfg.ToolGroups.Governor,
		"memory":     &cfg.ToolGroups.Memory,
		"skills":     &cfg.ToolGroups.Skills,
		"workflow":   &cfg.ToolGroups.Workflow,
		"etcd":       &cfg.ToolGroups.Etcd,
		"monitoring": &cfg.ToolGroups.Monitoring,
	}

	updated := false
	for key, field := range defaultTrue {
		if _, present := raw.ToolGroups[key]; !present {
			*field = true
			updated = true
			log.Printf("mcp: config missing tool_groups.%s — defaulting to true", key)
		}
	}

	// Re-write config with the new defaults so next restart picks them up.
	if updated {
		writeConfig(defaultConfigPath, cfg)
	}
}

func writeConfig(path string, cfg *MCPConfig) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		log.Printf("mcp: cannot create config dir %s: %v", dir, err)
		return
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		log.Printf("mcp: cannot marshal config: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0640); err != nil {
		log.Printf("mcp: cannot write config to %s: %v", path, err)
	}
}

// maybeEnableTLSFromServiceCert enables HTTPS automatically when the standard
// service certificate exists in /var/lib/globular/pki/issued/services.
// It mutates cfg in-place and returns true if any field changed.
func maybeEnableTLSFromServiceCert(cfg *MCPConfig) bool {
	if _, err := os.Stat(serviceCertPath); err != nil {
		return false
	}
	if _, err := os.Stat(serviceKeyPath); err != nil {
		return false
	}

	changed := false

	if !cfg.HTTPUseTLS {
		cfg.HTTPUseTLS = true
		changed = true
	}
	if cfg.HTTPTLSCertFile != serviceCertPath {
		cfg.HTTPTLSCertFile = serviceCertPath
		changed = true
	}
	if cfg.HTTPTLSKeyFile != serviceKeyPath {
		cfg.HTTPTLSKeyFile = serviceKeyPath
		changed = true
	}
	if cfg.HTTPAdvertiseHost == "" {
		cfg.HTTPAdvertiseHost = config.GetRoutableIPv4()
		changed = true
	}

	return changed
}
