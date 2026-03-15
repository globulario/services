package main

import (
	"testing"
)

func TestFilePathValidation(t *testing.T) {
	cfg := defaultConfig()
	cfg.FileAllowedRoots = []string{"/var/lib/globular/webroot", "/var/lib/globular/config"}

	tests := []struct {
		path    string
		wantErr bool
		desc    string
	}{
		{"/var/lib/globular/webroot/index.html", false, "allowed root exact file"},
		{"/var/lib/globular/webroot/admin/app.js", false, "allowed root subdirectory"},
		{"/var/lib/globular/config/etcd.yaml", false, "second allowed root"},
		{"/etc/passwd", true, "outside all roots"},
		{"/var/lib/globular/keys/private.pem", true, "sensitive path not in roots"},
		{"/var/lib/globular/webroot/../keys/private.pem", true, "traversal attack"},
		{"/var/lib/globular/webroot/../../etc/passwd", true, "deep traversal"},
		{"relative/path", true, "relative path rejected"},
		{"", true, "empty path rejected"},
	}

	for _, tt := range tests {
		_, err := cfg.validateFilePath(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: path=%q wantErr=%v got err=%v", tt.desc, tt.path, tt.wantErr, err)
		}
	}
}

func TestFilePathValidationEmptyRoots(t *testing.T) {
	cfg := defaultConfig()
	cfg.FileAllowedRoots = []string{}

	_, err := cfg.validateFilePath("/var/lib/globular/webroot/index.html")
	if err == nil {
		t.Error("expected error when no roots configured")
	}
}

func TestPersistenceAllowlist(t *testing.T) {
	cfg := defaultConfig()
	cfg.PersistenceAllowedConns = []string{"local_resource"}
	cfg.PersistenceAllowedDBs = []string{"local_resource", "test_db"}
	cfg.PersistenceAllowedColls = []string{"accounts", "roles"}

	tests := []struct {
		conn, db, coll string
		wantErr        bool
		desc           string
	}{
		{"local_resource", "local_resource", "accounts", false, "all allowed"},
		{"local_resource", "test_db", "roles", false, "second db allowed"},
		{"foreign_conn", "local_resource", "accounts", true, "connection not allowed"},
		{"local_resource", "secret_db", "accounts", true, "database not allowed"},
		{"local_resource", "local_resource", "sessions", true, "collection not allowed"},
	}

	for _, tt := range tests {
		err := cfg.validatePersistenceAccess(tt.conn, tt.db, tt.coll)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: wantErr=%v got err=%v", tt.desc, tt.wantErr, err)
		}
	}
}

func TestStorageAllowlist(t *testing.T) {
	cfg := defaultConfig()
	cfg.StorageAllowedConns = []string{"default"}
	cfg.StorageAllowedKeyPrefixes = []string{"/globular/nodes/", "/globular/services/"}

	tests := []struct {
		conn, key string
		wantErr   bool
		desc      string
	}{
		{"default", "/globular/nodes/abc/packages", false, "allowed prefix"},
		{"default", "/globular/services/auth/config", false, "second prefix"},
		{"other", "/globular/nodes/abc", true, "connection not allowed"},
		{"default", "/secrets/private-key", true, "prefix not allowed"},
		{"default", "/globular/tokens/sa", true, "similar but not matching prefix"},
	}

	for _, tt := range tests {
		err := cfg.validateStorageAccess(tt.conn, tt.key)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s: wantErr=%v got err=%v", tt.desc, tt.wantErr, err)
		}
	}
}

func TestToolGroupGating(t *testing.T) {
	cfg := defaultConfig()
	cfg.ToolGroups.File = false
	cfg.ToolGroups.Persistence = false
	cfg.ToolGroups.Storage = false

	srv := newServer(cfg)
	registerAllTools(srv)

	// File tools should NOT be registered
	if _, ok := srv.tools["file_get_info"]; ok {
		t.Error("file_get_info should not be registered when file group disabled")
	}
	if _, ok := srv.tools["deploy_get_webroot_snapshot"]; ok {
		t.Error("deploy tool should not be registered when file group disabled")
	}

	// Cluster tools SHOULD be registered
	if _, ok := srv.tools["cluster_get_health"]; !ok {
		t.Error("cluster_get_health should be registered")
	}

	// Composed tools SHOULD be registered
	if _, ok := srv.tools["cluster_get_operational_snapshot"]; !ok {
		t.Error("composed tool should be registered")
	}
}

func TestToolGroupGatingAllDisabled(t *testing.T) {
	cfg := defaultConfig()
	cfg.ToolGroups = ToolGroupConfig{} // all false

	srv := newServer(cfg)
	registerAllTools(srv)

	if len(srv.tools) != 0 {
		t.Errorf("expected 0 tools when all groups disabled, got %d", len(srv.tools))
	}
}

func TestRedactSensitive(t *testing.T) {
	input := map[string]interface{}{
		"name":         "admin",
		"password":     "secret123",
		"email":        "admin@example.com",
		"refreshToken": "tok_abc",
		"nested": map[string]interface{}{
			"api_key": "key_xyz",
			"value":   "safe",
		},
	}

	result := redactSensitive(input)

	if result["name"] != "admin" {
		t.Error("name should not be redacted")
	}
	if result["email"] != "admin@example.com" {
		t.Error("email should not be redacted")
	}
	if result["password"] != "***REDACTED***" {
		t.Error("password should be redacted")
	}
	if result["refreshToken"] != "***REDACTED***" {
		t.Error("refreshToken should be redacted")
	}

	nested := result["nested"].(map[string]interface{})
	if nested["api_key"] != "***REDACTED***" {
		t.Error("nested api_key should be redacted")
	}
	if nested["value"] != "safe" {
		t.Error("nested value should not be redacted")
	}
}

func TestDefaultConfigSafe(t *testing.T) {
	cfg := defaultConfig()

	if !cfg.ReadOnly {
		t.Error("default must be read-only")
	}
	if !cfg.AuditLog {
		t.Error("default must have audit logging")
	}
	if !cfg.ToolGroups.File {
		t.Error("file tools must be enabled by default")
	}
	if cfg.ToolGroups.Persistence {
		t.Error("persistence tools must be disabled by default")
	}
	if cfg.ToolGroups.Storage {
		t.Error("storage tools must be disabled by default")
	}
	if !cfg.ToolGroups.Cluster {
		t.Error("cluster tools must be enabled by default")
	}
	if !cfg.ToolGroups.Backup {
		t.Error("backup tools must be enabled by default")
	}
	if len(cfg.FileAllowedRoots) == 0 {
		t.Error("file roots must include default paths")
	}
	if len(cfg.PersistenceAllowedConns) != 0 {
		t.Error("persistence conns must be empty by default")
	}
}
