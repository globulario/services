package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// mockRoleStore simulates RBAC persistence for testing.
type mockRoleStore struct {
	existing map[string]bool
	created  map[string][]string
	failOn   string // role name that should fail CreateRole
}

func newMockStore(existingRoles ...string) *mockRoleStore {
	m := &mockRoleStore{
		existing: make(map[string]bool),
		created:  make(map[string][]string),
	}
	for _, r := range existingRoles {
		m.existing[r] = true
	}
	return m
}

func (m *mockRoleStore) RoleExists(_ context.Context, roleName string) (bool, error) {
	return m.existing[roleName], nil
}

func (m *mockRoleStore) CreateRole(_ context.Context, roleName string, actions []string, _ map[string]string) error {
	if m.failOn == roleName {
		return fmt.Errorf("simulated failure for %s", roleName)
	}
	m.created[roleName] = actions
	m.existing[roleName] = true
	return nil
}

func TestSeedServiceRoles_SeedsMissingRoles(t *testing.T) {
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
			{"name": "role:catalog.editor", "inherits": ["role:catalog.viewer"], "actions": ["catalog.item.write"]},
			{"name": "role:catalog.admin", "inherits": ["role:catalog.editor"], "actions": ["catalog.item.delete"]}
		]
	}`), 0644)

	store := newMockStore() // no existing roles
	result, err := SeedServiceRoles(context.Background(), "catalog", store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Seeded != 3 {
		t.Errorf("expected 3 seeded, got %d", result.Seeded)
	}
	if result.Skipped != 0 {
		t.Errorf("expected 0 skipped, got %d", result.Skipped)
	}
	if len(store.created) != 3 {
		t.Errorf("expected 3 created roles, got %d", len(store.created))
	}
	if len(store.created["role:catalog.viewer"]) != 2 {
		t.Errorf("expected 2 actions for viewer, got %v", store.created["role:catalog.viewer"])
	}
}

func TestSeedServiceRoles_PreservesExistingRoles(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	svcDir := filepath.Join(PackageRoot, "services", "catalog")
	os.MkdirAll(svcDir, 0755)
	os.WriteFile(filepath.Join(svcDir, "roles.generated.json"), []byte(`{
		"schema_version": "2",
		"service": "catalog.CatalogService",
		"roles": [
			{"name": "role:catalog.viewer", "actions": ["catalog.item.read"]},
			{"name": "role:catalog.editor", "actions": ["catalog.item.write"]},
			{"name": "role:catalog.admin", "actions": ["catalog.item.delete"]}
		]
	}`), 0644)

	// viewer already exists (maybe admin-edited) — must NOT be overwritten
	store := newMockStore("role:catalog.viewer")
	result, err := SeedServiceRoles(context.Background(), "catalog", store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Seeded != 2 {
		t.Errorf("expected 2 seeded, got %d", result.Seeded)
	}
	if result.Skipped != 1 {
		t.Errorf("expected 1 skipped (preserved), got %d", result.Skipped)
	}
	// Verify viewer was NOT recreated
	if _, created := store.created["role:catalog.viewer"]; created {
		t.Error("existing role:catalog.viewer should NOT have been recreated")
	}
}

func TestSeedServiceRoles_NoFileReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	store := newMockStore()
	result, err := SeedServiceRoles(context.Background(), "nonexistent", store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Seeded != 0 || result.Skipped != 0 || result.Failed != 0 {
		t.Errorf("expected all zeros for missing file, got %+v", result)
	}
}

func TestSeedServiceRoles_HandlesCreateFailure(t *testing.T) {
	dir := t.TempDir()
	AdminRoot = filepath.Join(dir, "etc")
	PackageRoot = filepath.Join(dir, "var")

	svcDir := filepath.Join(PackageRoot, "services", "catalog")
	os.MkdirAll(svcDir, 0755)
	os.WriteFile(filepath.Join(svcDir, "roles.generated.json"), []byte(`{
		"schema_version": "2",
		"service": "catalog.CatalogService",
		"roles": [
			{"name": "role:catalog.viewer", "actions": ["catalog.item.read"]},
			{"name": "role:catalog.editor", "actions": ["catalog.item.write"]}
		]
	}`), 0644)

	store := newMockStore()
	store.failOn = "role:catalog.editor" // simulate RBAC failure
	result, err := SeedServiceRoles(context.Background(), "catalog", store)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Seeded != 1 {
		t.Errorf("expected 1 seeded, got %d", result.Seeded)
	}
	if result.Failed != 1 {
		t.Errorf("expected 1 failed, got %d", result.Failed)
	}
}
