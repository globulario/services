package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPermissions_FromFile(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	// Write a valid permissions.json to the package root.
	svcDir := filepath.Join(PackageRoot, "services", "file")
	if err := os.MkdirAll(svcDir, 0755); err != nil {
		t.Fatal(err)
	}
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
	if m["action"] != "/file.FileService/ReadDir" {
		t.Errorf("unexpected action: %v", m["action"])
	}
	if m["permission"] != "read" {
		t.Errorf("unexpected permission: %v", m["permission"])
	}
}

func TestLoadPermissions_AdminOverridesPackage(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	// Write package default
	pkgDir := filepath.Join(PackageRoot, "services", "file")
	os.MkdirAll(pkgDir, 0755)
	os.WriteFile(filepath.Join(pkgDir, "permissions.json"), []byte(`{
		"version": "1.0", "service": "file.FileService",
		"permissions": [{"action": "/file.FileService/ReadDir", "permission": "read", "resources": []}]
	}`), 0644)

	// Write admin override with different permission
	admDir := filepath.Join(AdminRoot, "services", "file")
	os.MkdirAll(admDir, 0755)
	os.WriteFile(filepath.Join(admDir, "permissions.json"), []byte(`{
		"version": "1.0", "service": "file.FileService",
		"permissions": [{"action": "/file.FileService/ReadDir", "permission": "admin", "resources": []}]
	}`), 0644)

	perms, fromFile, _ := LoadPermissions("file")
	if !fromFile {
		t.Fatal("expected fromFile=true")
	}
	m := perms[0].(map[string]interface{})
	if m["permission"] != "admin" {
		t.Errorf("expected admin override, got %v", m["permission"])
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

func TestLoadPermissions_ValidationFailure(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	svcDir := filepath.Join(PackageRoot, "services", "file")
	os.MkdirAll(svcDir, 0755)
	// Missing version, invalid action (no leading /), invalid verb
	os.WriteFile(filepath.Join(svcDir, "permissions.json"), []byte(`{
		"service": "file.FileService",
		"permissions": [{"action": "BadAction", "permission": "nope", "resources": []}]
	}`), 0644)

	perms, fromFile, _ := LoadPermissions("file")
	if fromFile {
		t.Fatal("expected fromFile=false for validation failure")
	}
	if perms != nil {
		t.Fatal("expected nil perms for validation failure")
	}
}

func TestLoadClusterRoles_FromFile(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	rbacDir := filepath.Join(PackageRoot, "rbac")
	os.MkdirAll(rbacDir, 0755)
	os.WriteFile(filepath.Join(rbacDir, "cluster-roles.json"), []byte(`{
		"version": "1.0",
		"roles": {
			"globular-admin": ["/*"],
			"file.viewer": ["/file.FileService/ReadDir"]
		}
	}`), 0644)

	roles, fromFile, err := LoadClusterRoles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fromFile {
		t.Fatal("expected fromFile=true")
	}
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(roles))
	}
	if roles["globular-admin"][0] != "/*" {
		t.Errorf("unexpected admin perm: %v", roles["globular-admin"])
	}
}

func TestLoadClusterRoles_NoFile(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	roles, fromFile, _ := LoadClusterRoles()
	if fromFile {
		t.Fatal("expected fromFile=false")
	}
	if roles != nil {
		t.Fatal("expected nil roles")
	}
}

func TestValidatePermissions(t *testing.T) {
	// Valid file should produce no errors
	pf := &PermissionsFile{
		Version: "1.0",
		Service: "file.FileService",
		Permissions: []Permission{
			{
				Action:     "/file.FileService/ReadDir",
				Permission: "read",
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

func TestValidateClusterRoles(t *testing.T) {
	// Valid file
	crf := &ClusterRolesFile{
		Version: "1.0",
		Roles: map[string][]string{
			"globular-admin": {"/*"},
			"file.viewer":    {"/file.FileService/ReadDir"},
		},
	}
	if errs := validateClusterRoles(crf); len(errs) > 0 {
		t.Errorf("expected no errors, got: %v", errs)
	}

	// Invalid method path
	crf2 := &ClusterRolesFile{
		Version: "1.0",
		Roles: map[string][]string{
			"bad-role": {"no-slash-method"},
		},
	}
	if errs := validateClusterRoles(crf2); len(errs) == 0 {
		t.Error("expected validation errors for bad method path")
	}
}
