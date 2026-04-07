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
		HTTPListenAddr:            ":10250",
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

		// Apply defaults for new tool groups that may be missing from
		// older config files. When a bool field is absent from the JSON
		// object, Go decodes it as false — so we check the raw JSON
		// and restore the default for any missing field.
		applyToolGroupDefaults(data, cfg)

		return cfg
	}

	log.Println("mcp: no config file found, writing defaults to " + defaultConfigPath)
	writeDefaultConfig(cfg)
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
		writeDefaultConfig(cfg)
	}
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
