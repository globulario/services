package interceptors

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/globulario/services/golang/policy"
	"github.com/globulario/services/golang/security"
)

// TestMain loads cluster-roles.json and any generated permission manifests
// so that security.IsRoleBasedMethod, HasRolePermission, and the GlobalResolver
// have correct data during acceptance tests.
func TestMain(m *testing.M) {
	// Load cluster-roles.json so RolePermissions (and derived methodSet/methodPrefix)
	// are populated before any test runs.
	security.ReloadClusterRoles()

	// Also register generated action-key mappings if they exist on disk.
	// On CI the generated/policy directory may not exist — that's OK.
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
		generatedDir := filepath.Join(repoRoot, "generated", "policy")
		policy.RegisterAllFromDirectory(generatedDir)
	}
	os.Exit(m.Run())
}
