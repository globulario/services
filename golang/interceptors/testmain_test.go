package interceptors

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/globulario/services/golang/policy"
)

// TestMain loads generated permission files from the repo so that the
// GlobalResolver has semantic action-key mappings available during tests.
// On CI the generated/policy directory may not exist — that's OK; tests
// that require semantic resolution will skip or use legacy fallbacks.
func TestMain(m *testing.M) {
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
		generatedDir := filepath.Join(repoRoot, "generated", "policy")
		policy.RegisterAllFromDirectory(generatedDir)
	}
	os.Exit(m.Run())
}
