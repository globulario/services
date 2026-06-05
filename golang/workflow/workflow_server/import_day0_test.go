// @awareness namespace=globular.platform
// @awareness component=platform_workflow.import_day0
// @awareness file_role=regression_tests_for_day0_trace_import_silent_paths
// @awareness enforces=globular.platform:invariant.workflow.day0_trace_import_is_idempotent_and_best_effort
// @awareness risk=medium
package main

import (
	"os"
	"path/filepath"
	"testing"
)

// withDay0LogPath overrides the package-level day0LogPath for the lifetime
// of a test. Production callers leave it at the default.
func withDay0LogPath(t *testing.T, path string) {
	t.Helper()
	prev := day0LogPath
	day0LogPath = path
	t.Cleanup(func() { day0LogPath = prev })
}

// TestImportDay0Trace_MissingLogIsSilent pins rule 2 (BEST-EFFORT) of
// workflow.day0_trace_import_is_idempotent_and_best_effort: a missing log
// file silently returns. The function is called from workflow-server
// startup; promoting "missing log" to a fatal would couple startup to a
// file that only exists on the founding controller node.
func TestImportDay0Trace_MissingLogIsSilent(t *testing.T) {
	// Point day0LogPath at a path that definitely does not exist.
	withDay0LogPath(t, filepath.Join(t.TempDir(), "no-such-day0-log.jsonl"))

	srv := &server{}
	// Must return without panic. No Scylla session needed because the
	// open-file check short-circuits before getSession() is called.
	srv.importDay0Trace()
}

// TestImportDay0Trace_NilSessionIsSilent pins the second half of rule 2:
// even when the log file exists, a nil Scylla session must silently return
// (the workflow service is starting up; the trace import is best-effort).
func TestImportDay0Trace_NilSessionIsSilent(t *testing.T) {
	// Create a temp log file so the file-open succeeds.
	logPath := filepath.Join(t.TempDir(), "day0-install.jsonl")
	if err := os.WriteFile(logPath, []byte(`{"type":"run_start","ts":1,"hostname":"test"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture log: %v", err)
	}
	withDay0LogPath(t, logPath)

	// srv with no scylla session — getSession() returns nil.
	srv := &server{}
	// Must return without panic. The function checks sess == nil after
	// the file-open succeeds.
	srv.importDay0Trace()
}
