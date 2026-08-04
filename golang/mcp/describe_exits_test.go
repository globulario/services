package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestDescribeExitsWithoutServing builds this binary and runs it with
// --describe, asserting it exits promptly instead of entering the serving
// path.
//
// This is the mcp-side half of failure_mode
// install.describe_preflight_unbounded_hangs_join. During Day-1 SERVICE
// install, node-agent execs "<binary> --describe" as a best-effort port
// preflight. mcp had no flag handling at all, fell through to http serve,
// bound its port and blocked forever — consuming the entire 8-minute
// per-package install budget, failing the install, and looping the join
// every ~30s (2026-07, nuc).
//
// The node-agent side is independently bounded (runDescribe, 5s, proven by
// serviceports/describe_bound_test.go:TestRunDescribeBoundedWhenBinaryHangs).
// Both halves are required: the node-agent must defend regardless of binary
// behaviour, and the binary must not need defending.
//
// SCOPE: this asserts --describe only. It deliberately does NOT assert that
// an arbitrary unrecognized flag exits — main() matches --describe,
// --version and --help/-h and lets anything else fall through to server
// startup. That broader rule is not the contract this incident proved; see
// the scope_note on the failure mode.
func TestDescribeExitsWithoutServing(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "mcp_server_test_binary")

	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build mcp binary: %v\n%s", err, out)
	}

	// Generous relative to an immediate exit, tight relative to the 5s
	// runDescribe bound this must stay well inside.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	start := time.Now()
	out, err := exec.CommandContext(ctx, bin, "--describe").CombinedOutput()
	elapsed := time.Since(start)

	if ctx.Err() != nil {
		t.Fatalf("mcp_server --describe did not exit within %v — it entered the serving path, "+
			"which is the exact shape that hung the join in 2026-07. output:\n%s", elapsed, out)
	}
	if err != nil {
		t.Fatalf("mcp_server --describe exited with error %v (must answer and exit, or fail fast).\noutput:\n%s", err, out)
	}
	if elapsed > 5*time.Second {
		t.Errorf("mcp_server --describe took %v; it must exit well inside the 5s runDescribe bound", elapsed)
	}
}
