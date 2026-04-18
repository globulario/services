package security

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/globulario/services/golang/policy"
)

// TestMain loads generated permissions into the GlobalResolver so that tests
// that assert permission via raw gRPC method paths work correctly even in
// development environments where the runtime policy paths are not populated.
func TestMain(m *testing.M) {
	// Find the generated/policy directory relative to this file's location.
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		// thisFile = .../golang/security/resolver_test.go
		// generatedDir = .../generated/policy
		repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile))) // up 3 levels
		generatedDir := filepath.Join(repoRoot, "generated", "policy")
		policy.RegisterAllFromDirectory(generatedDir)
	}
	os.Exit(m.Run())
}
