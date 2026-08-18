package rules

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Temporary diagnostic: CI checks out Sensei beside services. Print the exact
// ownership-aware seed diff so the cross-repository classifier can be repaired
// from real evidence, then remove this test before PR completion.
func TestDiagnosticCombinedSeedOwnership(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Skipf("getwd: %v", err)
	}
	servicesRoot := filepath.Clean(filepath.Join(cwd, "../../../.."))
	senseiRoot := filepath.Join(filepath.Dir(servicesRoot), "awareness-graph")
	if _, err := os.Stat(filepath.Join(senseiRoot, "cmd", "awg")); err != nil {
		t.Skip("Sensei sibling checkout unavailable outside cross-repository CI")
	}
	cmd := exec.Command("go", "run", "./cmd/awg", "audit", "-verbose", "-services-repo", servicesRoot, "-ag-repo", senseiRoot)
	cmd.Dir = senseiRoot
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		t.Fatalf("combined Sensei audit diagnostic failed as expected; exact output follows:\n%s", out)
	}
	t.Logf("combined Sensei audit unexpectedly clean:\n%s", out)
}