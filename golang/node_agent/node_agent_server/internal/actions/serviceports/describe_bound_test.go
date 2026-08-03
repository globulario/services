package serviceports

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRunDescribeBoundedWhenBinaryHangs is a regression guard for the 2026-07
// Day-1 mcp join loop: mcp_server ignored --describe and started its HTTP
// server, blocking forever. runDescribe execed it under the caller's full
// 8-minute per-package install context, so the describe burned the entire
// budget, post-install failed on the dead context, and the join looped.
//
// runDescribe MUST bound the exec itself so a binary that never answers
// --describe is killed in seconds, independent of the (possibly very long)
// caller context. This test hands it a binary that blocks for an hour under a
// 60s parent context and asserts runDescribe returns (nil,nil) quickly.
func TestRunDescribeBoundedWhenBinaryHangs(t *testing.T) {
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "hang_server")
	// Ignores --describe and blocks forever, exactly as a service binary that
	// starts serving instead of honoring the describe protocol. `exec` so the
	// process IS the sleep (single process, like a real Go service binary):
	// SIGKILL from the bound closes its stdout pipe and Output() returns. A
	// bare `sleep` would fork and orphan, holding the pipe open — a shell
	// artifact that does not represent the mcp_server case.
	script := "#!/bin/sh\nexec sleep 3600\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	// Parent context deadline is far longer than runDescribe's internal bound;
	// if the internal bound is what returns us, we come back in ~5s, not 60s.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	start := time.Now()
	payload, err := runDescribe(ctx, binPath)
	elapsed := time.Since(start)

	if payload != nil || err != nil {
		t.Fatalf("expected (nil,nil) for a hanging binary, got payload=%v err=%v", payload, err)
	}
	if elapsed > 15*time.Second {
		t.Fatalf("runDescribe blocked %v — internal describe timeout not enforced (a hanging binary must not consume the caller's install budget)", elapsed)
	}
}
