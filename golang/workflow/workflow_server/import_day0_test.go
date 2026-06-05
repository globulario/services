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

// withDay0Seams overrides the count + write test seams for the lifetime
// of a test. The override functions are returned so tests can read call
// counters via closure capture if they need to.
func withDay0Seams(
	t *testing.T,
	count func(srv *server, clusterID, corrID string) (int, bool),
	write func(srv *server, clusterID, corrID string, lines []day0LogLine),
) {
	t.Helper()
	prevCount := day0CountExistingFn
	prevWrite := day0WriteRunFn
	day0CountExistingFn = count
	day0WriteRunFn = write
	t.Cleanup(func() {
		day0CountExistingFn = prevCount
		day0WriteRunFn = prevWrite
	})
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

// TestImportDay0Trace_IdempotentAcrossRestarts pins rule 1 (IDEMPOTENT) of
// workflow.day0_trace_import_is_idempotent_and_best_effort: if the trace
// has already been imported (correlation_id "day0-install" has any rows
// in workflow_runs), a subsequent call must NOT write a duplicate run.
//
// Implementation: the count + write seams let the test simulate the
// "already imported" state without touching real Scylla. First call has
// count=0 → write seam fires; second call has count=1 → write seam must
// NOT fire.
func TestImportDay0Trace_IdempotentAcrossRestarts(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "day0-install.jsonl")
	body := `{"type":"run_start","ts":1,"hostname":"test"}` + "\n" +
		`{"type":"step","seq":1,"key":"k1","title":"t1","status":"ok","ts":2}` + "\n" +
		`{"type":"run_finish","ts":3,"status":"ok"}` + "\n"
	if err := os.WriteFile(logPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture log: %v", err)
	}
	withDay0LogPath(t, logPath)

	// existing simulates the row count in workflow_runs for correlation_id
	// "day0-install". First call: 0 (nothing imported yet). Second call:
	// 1 (the first call's hypothetical write landed).
	existing := 0
	writeCalls := 0
	withDay0Seams(t,
		func(srv *server, clusterID, corrID string) (int, bool) {
			if clusterID == "" || corrID != "day0-install" {
				t.Errorf("count seam called with unexpected (cluster=%q, corr=%q)", clusterID, corrID)
			}
			return existing, true
		},
		func(srv *server, clusterID, corrID string, lines []day0LogLine) {
			writeCalls++
			// Simulate the write landing — next call sees existing > 0.
			existing = 1
		},
	)

	srv := &server{}

	// First call — should write.
	srv.importDay0Trace()
	if writeCalls != 1 {
		t.Fatalf("first call: expected 1 write, got %d", writeCalls)
	}

	// Second call (simulating restart) — must NOT write.
	srv.importDay0Trace()
	if writeCalls != 1 {
		t.Fatalf("second call: expected write count to stay 1 (IDEMPOTENT), got %d", writeCalls)
	}
}
