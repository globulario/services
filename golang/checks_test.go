// Package golang_test contains regression tests for the static security and
// correctness invariants enforced by `make check-services`.
//
// Running `go test ./...` from the golang/ directory will execute these
// alongside unit tests so that CI catches invariant regressions automatically.
package golang_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// repoRoot returns the root of the services repository (parent of golang/).
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file is …/golang/checks_test.go → parent is the repo root
	return filepath.Dir(filepath.Dir(file))
}

// runMake runs `make <target>` in the repo root and fails the test if it exits
// non-zero.
func runMake(t *testing.T, target string) {
	t.Helper()
	root := repoRoot(t)
	cmd := exec.Command("make", target)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("make %s output:\n%s", target, string(out))
		t.Fatalf("make %s failed: %v", target, err)
	}
}

// TestCheckTargetPathsExist ensures the Makefile paths for the security
// boundary checks point at real directories. A missing directory must be an
// explicit failure — it cannot be a silent pass.
func TestCheckTargetPathsExist(t *testing.T) {
	root := repoRoot(t)

	dirs := []string{
		filepath.Join(root, "golang", "cluster_controller", "cluster_controller_server"),
		filepath.Join(root, "golang", "node_agent", "node_agent_server"),
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); err != nil {
			t.Errorf("security check directory missing: %s — make check-services would silently skip it", d)
		}
	}
}

// TestControllerNoExec verifies that cluster_controller_server contains no
// forbidden exec primitives. This is the same check as `make check-controller-no-exec`
// but expressed as a Go test so CI picks it up with `go test ./...`.
func TestControllerNoExec(t *testing.T) {
	runMake(t, "check-controller-no-exec")
}

// TestNodeAgentExecBoundary verifies that exec usage in node_agent_server does
// not appear in generated protobuf / type files. This mirrors
// `make check-nodeagent-exec-boundary`.
func TestNodeAgentExecBoundary(t *testing.T) {
	runMake(t, "check-nodeagent-exec-boundary")
}

// TestProtoAuthzCoverage verifies that every gRPC RPC in every service proto
// has a (globular.auth.authz) annotation. This mirrors `make check-proto-authz`.
func TestProtoAuthzCoverage(t *testing.T) {
	if testing.Short() {
		t.Skip("requires slow bash script; run via 'make check-proto-authz'")
	}
	runMake(t, "check-proto-authz")
}
