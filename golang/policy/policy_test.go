package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPermissions_V2Format(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	svcDir := filepath.Join(PackageRoot, "services", "file")
	if err := os.MkdirAll(svcDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := `{
		"version": "1.0",
		"service": "file.FileService",
		"permissions": [
			{
				"method": "/file.FileService/ReadDir",
				"action": "file.list",
				"resources": [
					{"index": 0, "field": "Path", "permission": "read"}
				]
			}
		]
	}`
	if err := os.WriteFile(filepath.Join(svcDir, "permissions.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	perms, fromFile, err := LoadPermissions("file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fromFile {
		t.Fatal("expected fromFile=true")
	}
	if len(perms) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(perms))
	}
	m, ok := perms[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected map[string]interface{}")
	}
	// In v2 output, "action" is the stable action key
	if m["action"] != "file.list" {
		t.Errorf("expected action=file.list, got %v", m["action"])
	}
	// "method" is the gRPC transport path
	if m["method"] != "/file.FileService/ReadDir" {
		t.Errorf("expected method=/file.FileService/ReadDir, got %v", m["method"])
	}
}

func TestLoadPermissions_V1Format(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	svcDir := filepath.Join(PackageRoot, "services", "file")
	os.MkdirAll(svcDir, 0755)
	// v1 format: action is the method path, no method field
	content := `{
		"version": "1.0",
		"service": "file.FileService",
		"permissions": [
			{
				"action": "/file.FileService/ReadDir",
				"permission": "read",
				"resources": [
					{"index": 0, "field": "Path", "permission": "read"}
				]
			}
		]
	}`
	os.WriteFile(filepath.Join(svcDir, "permissions.json"), []byte(content), 0644)

	perms, fromFile, _ := LoadPermissions("file")
	if !fromFile {
		t.Fatal("expected fromFile=true")
	}
	m := perms[0].(map[string]interface{})
	if m["action"] != "/file.FileService/ReadDir" {
		t.Errorf("expected v1 action, got %v", m["action"])
	}
}

func TestLoadPermissions_AdminOverridesPackage(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	pkgDir := filepath.Join(PackageRoot, "services", "file")
	os.MkdirAll(pkgDir, 0755)
	os.WriteFile(filepath.Join(pkgDir, "permissions.json"), []byte(`{
		"version": "1.0", "service": "file.FileService",
		"permissions": [{"method": "/file.FileService/ReadDir", "action": "file.list", "resources": []}]
	}`), 0644)

	admDir := filepath.Join(AdminRoot, "services", "file")
	os.MkdirAll(admDir, 0755)
	os.WriteFile(filepath.Join(admDir, "permissions.json"), []byte(`{
		"version": "1.0", "service": "file.FileService",
		"permissions": [{"method": "/file.FileService/ReadDir", "action": "file.read", "resources": []}]
	}`), 0644)

	perms, fromFile, _ := LoadPermissions("file")
	if !fromFile {
		t.Fatal("expected fromFile=true")
	}
	m := perms[0].(map[string]interface{})
	if m["action"] != "file.read" {
		t.Errorf("expected admin override action=file.read, got %v", m["action"])
	}
}

func TestLoadPermissions_NoFile(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	perms, fromFile, err := LoadPermissions("file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fromFile {
		t.Fatal("expected fromFile=false when no file exists")
	}
	if perms != nil {
		t.Fatal("expected nil perms when no file exists")
	}
}

func TestLoadPermissions_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	svcDir := filepath.Join(PackageRoot, "services", "file")
	os.MkdirAll(svcDir, 0755)
	os.WriteFile(filepath.Join(svcDir, "permissions.json"), []byte(`{invalid`), 0644)

	perms, fromFile, _ := LoadPermissions("file")
	if fromFile {
		t.Fatal("expected fromFile=false for malformed JSON")
	}
	if perms != nil {
		t.Fatal("expected nil perms for malformed JSON")
	}
}

func TestActionResolver(t *testing.T) {
	r := NewResolver()
	r.Register([]Permission{
		{Method: "/file.FileService/ReadDir", Action: "file.list"},
		{Method: "/file.FileService/ReadFile", Action: "file.read"},
		{Method: "/file.FileService/GetFileInfo", Action: "file.list"}, // multiple methods → same action
	})

	if got := r.Resolve("/file.FileService/ReadDir"); got != "file.list" {
		t.Errorf("expected file.list, got %s", got)
	}
	if got := r.Resolve("/file.FileService/ReadFile"); got != "file.read" {
		t.Errorf("expected file.read, got %s", got)
	}
	if got := r.Resolve("/file.FileService/GetFileInfo"); got != "file.list" {
		t.Errorf("expected file.list, got %s", got)
	}
	// Unknown method falls back to method path
	if got := r.Resolve("/file.FileService/Unknown"); got != "/file.FileService/Unknown" {
		t.Errorf("expected fallback to method path, got %s", got)
	}
	if !r.HasMapping("/file.FileService/ReadDir") {
		t.Error("expected HasMapping=true")
	}
	// LegacyMethods: reverse lookup for compatibility shim
	methods := r.LegacyMethods("file.list")
	if len(methods) != 2 {
		t.Fatalf("expected 2 legacy methods for file.list, got %d", len(methods))
	}
	methods = r.LegacyMethods("file.read")
	if len(methods) != 1 || methods[0] != "/file.FileService/ReadFile" {
		t.Errorf("unexpected legacy methods for file.read: %v", methods)
	}
	if methods := r.LegacyMethods("file.unknown"); methods != nil {
		t.Errorf("expected nil for unknown action, got %v", methods)
	}
	if r.HasMapping("/file.FileService/Unknown") {
		t.Error("expected HasMapping=false for unknown")
	}
}

func TestActionResolver_V1SkipsNonActionKeys(t *testing.T) {
	r := NewResolver()
	// v1 format: Method empty, Action is a method path → should not create mapping
	r.Register([]Permission{
		{Action: "/file.FileService/ReadDir"},
	})
	if r.HasMapping("/file.FileService/ReadDir") {
		t.Error("v1 format should not create method→action mapping")
	}
}

func TestIsActionKey(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"file.read", true},
		{"file.list", true},
		{"resource.account.create", true},
		{"dns.zone.write", true},
		{"/file.FileService/ReadDir", false},
		{"", false},
		{"file", false},              // needs at least one dot
		{"File.Read", false},         // uppercase
		{"*", false},                 // wildcard
		{"file.read.v2.beta", true},  // multiple dots ok
	}
	for _, tt := range tests {
		if got := IsActionKey(tt.input); got != tt.want {
			t.Errorf("IsActionKey(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLoadClusterRoles_WithActionKeys(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	rbacDir := filepath.Join(PackageRoot, "rbac")
	os.MkdirAll(rbacDir, 0755)
	os.WriteFile(filepath.Join(rbacDir, "cluster-roles.json"), []byte(`{
		"version": "1.0",
		"roles": {
			"globular-admin": ["*"],
			"file.viewer": ["file.list", "file.read"],
			"file.editor": ["file.list", "file.read", "file.write"]
		}
	}`), 0644)

	roles, fromFile, err := LoadClusterRoles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fromFile {
		t.Fatal("expected fromFile=true")
	}
	if len(roles) != 3 {
		t.Fatalf("expected 3 roles, got %d", len(roles))
	}
	if roles["file.viewer"][0] != "file.list" {
		t.Errorf("unexpected viewer grant: %v", roles["file.viewer"])
	}
}

func TestLoadClusterRoles_MixedGrants(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	rbacDir := filepath.Join(PackageRoot, "rbac")
	os.MkdirAll(rbacDir, 0755)
	// Mixed: action keys + legacy method paths (migration period)
	os.WriteFile(filepath.Join(rbacDir, "cluster-roles.json"), []byte(`{
		"version": "1.0",
		"roles": {
			"globular-operator": [
				"file.list",
				"/dns.DnsService/*"
			]
		}
	}`), 0644)

	roles, fromFile, _ := LoadClusterRoles()
	if !fromFile {
		t.Fatal("expected fromFile=true")
	}
	if len(roles["globular-operator"]) != 2 {
		t.Fatalf("expected 2 grants, got %d", len(roles["globular-operator"]))
	}
}

func TestValidatePermissions_V2(t *testing.T) {
	pf := &PermissionsFile{
		Version: "1.0",
		Service: "file.FileService",
		Permissions: []Permission{
			{
				Method: "/file.FileService/ReadDir",
				Action: "file.list",
				Resources: []Resource{
					{Index: 0, Field: "Path", Permission: "read"},
				},
			},
		},
	}
	if errs := validatePermissions(pf); len(errs) > 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}
}

func TestValidatePermissions_InvalidActionKey(t *testing.T) {
	pf := &PermissionsFile{
		Version: "1.0",
		Service: "file.FileService",
		Permissions: []Permission{
			{Method: "/file.FileService/ReadDir", Action: "BAD-KEY"},
		},
	}
	errs := validatePermissions(pf)
	if len(errs) == 0 {
		t.Error("expected validation error for bad action key")
	}
}

func TestValidatePermissions_DuplicateMethod(t *testing.T) {
	pf := &PermissionsFile{
		Version: "1.0",
		Service: "file.FileService",
		Permissions: []Permission{
			{Method: "/file.FileService/ReadDir", Action: "file.list"},
			{Method: "/file.FileService/ReadDir", Action: "file.read"}, // duplicate method
		},
	}
	errs := validatePermissions(pf)
	if len(errs) == 0 {
		t.Error("expected validation error for duplicate method")
	}
}

// ── Phase 3b: .generated.json precedence tests ──────────────────────────────

func TestLoadPermissions_GeneratedFileUsedWhenNoOverride(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	// Only write a .generated.json (no override)
	svcDir := filepath.Join(PackageRoot, "services", "catalog")
	os.MkdirAll(svcDir, 0755)
	os.WriteFile(filepath.Join(svcDir, "permissions.generated.json"), []byte(`{
		"schema_version": "2",
		"generator_version": "authzgen/0.1.0",
		"service": "catalog.CatalogService",
		"permissions": [
			{"method": "/catalog.CatalogService/GetItem", "action": "catalog.item.read", "permission": "read", "resources": []}
		]
	}`), 0644)

	perms, fromFile, _ := LoadPermissions("catalog")
	if !fromFile {
		t.Fatal("expected fromFile=true from .generated.json")
	}
	if len(perms) != 1 {
		t.Fatalf("expected 1 permission, got %d", len(perms))
	}
	m := perms[0].(map[string]interface{})
	if m["action"] != "catalog.item.read" {
		t.Errorf("expected action=catalog.item.read, got %v", m["action"])
	}
}

func TestLoadPermissions_OverrideBeatsGenerated(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	// Write generated file
	svcDir := filepath.Join(PackageRoot, "services", "catalog")
	os.MkdirAll(svcDir, 0755)
	os.WriteFile(filepath.Join(svcDir, "permissions.generated.json"), []byte(`{
		"version": "1.0", "service": "catalog.CatalogService",
		"permissions": [{"method": "/catalog.CatalogService/GetItem", "action": "catalog.item.read", "resources": []}]
	}`), 0644)

	// Write admin override (takes precedence)
	admDir := filepath.Join(AdminRoot, "services", "catalog")
	os.MkdirAll(admDir, 0755)
	os.WriteFile(filepath.Join(admDir, "permissions.json"), []byte(`{
		"version": "1.0", "service": "catalog.CatalogService",
		"permissions": [{"method": "/catalog.CatalogService/GetItem", "action": "catalog.item.view", "resources": []}]
	}`), 0644)

	perms, fromFile, _ := LoadPermissions("catalog")
	if !fromFile {
		t.Fatal("expected fromFile=true")
	}
	m := perms[0].(map[string]interface{})
	if m["action"] != "catalog.item.view" {
		t.Errorf("expected override action=catalog.item.view, got %v", m["action"])
	}
}

func TestLoadPermissions_OldServiceFallsBackToCompiled(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	// No files at all — simulates old unannotated service
	perms, fromFile, _ := LoadPermissions("old_service")
	if fromFile {
		t.Fatal("expected fromFile=false for unannotated service")
	}
	if perms != nil {
		t.Fatal("expected nil perms — caller should use compiled fallback")
	}
}

func TestLoadServiceRoles_FromGeneratedFile(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	svcDir := filepath.Join(PackageRoot, "services", "catalog")
	os.MkdirAll(svcDir, 0755)
	os.WriteFile(filepath.Join(svcDir, "roles.generated.json"), []byte(`{
		"schema_version": "2",
		"service": "catalog.CatalogService",
		"roles": [
			{"name": "role:catalog.viewer", "actions": ["catalog.item.read", "catalog.item.list"]},
			{"name": "role:catalog.editor", "inherits": ["role:catalog.viewer"], "actions": ["catalog.item.write"]}
		]
	}`), 0644)

	roles, fromFile, _ := LoadServiceRoles("catalog")
	if !fromFile {
		t.Fatal("expected fromFile=true")
	}
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(roles))
	}
	if roles[0].Name != "role:catalog.viewer" {
		t.Errorf("unexpected first role: %s", roles[0].Name)
	}
	if len(roles[1].Inherits) != 1 || roles[1].Inherits[0] != "role:catalog.viewer" {
		t.Errorf("expected editor to inherit viewer, got %v", roles[1].Inherits)
	}
}

func TestLoadServiceRoles_OverrideBeatsGenerated(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	// Generated
	svcDir := filepath.Join(PackageRoot, "services", "catalog")
	os.MkdirAll(svcDir, 0755)
	os.WriteFile(filepath.Join(svcDir, "roles.generated.json"), []byte(`{
		"version": "1.0", "service": "catalog",
		"roles": [{"name": "role:catalog.viewer", "actions": ["catalog.item.read"]}]
	}`), 0644)

	// Override
	admDir := filepath.Join(AdminRoot, "services", "catalog")
	os.MkdirAll(admDir, 0755)
	os.WriteFile(filepath.Join(admDir, "roles.json"), []byte(`{
		"version": "1.0", "service": "catalog",
		"roles": [{"name": "role:catalog.custom", "actions": ["catalog.everything"]}]
	}`), 0644)

	roles, fromFile, _ := LoadServiceRoles("catalog")
	if !fromFile {
		t.Fatal("expected fromFile=true")
	}
	if roles[0].Name != "role:catalog.custom" {
		t.Errorf("expected override role, got %s", roles[0].Name)
	}
}
