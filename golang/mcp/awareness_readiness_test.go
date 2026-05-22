package main

// awareness_readiness_test.go — pins the contract of
// awareness.mcp.advertised_tools_must_be_backed_by_ready_storage:
// contribution tools whose backing storage isn't writable must NOT be
// advertised, and the awareness.readiness tool must surface exactly why.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/awareness/graph"
)

// graphWithDataDir opens an awareness graph rooted at a temporary
// directory. Used by the readiness tests so they exercise the full
// disk-backed path (in-memory graphs short-circuit the readiness probe
// and always report writable=true, which masks the gating behaviour
// the invariant requires).
func graphWithDataDir(t *testing.T) (*graph.Graph, string) {
	t.Helper()
	dir := t.TempDir()
	g, err := graph.Open(dir)
	if err != nil {
		t.Fatalf("graph.Open(%s): %v", dir, err)
	}
	t.Cleanup(func() { g.Close() })
	return g, dir
}

// makeReadOnlyDir returns a freshly-created directory that the runtime
// user cannot write to. Used to drive the "not writable" branch of the
// readiness probe. Skipped when running as root (root bypasses POSIX
// dir-mode checks so the probe-write succeeds and the test would no
// longer exercise the gating behaviour).
func makeReadOnlyDir(t *testing.T) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("running as root — POSIX mode bits don't gate root writes; skipping read-only-dir test")
	}
	dir := t.TempDir()
	sub := filepath.Join(dir, "readonly")
	if err := os.Mkdir(sub, 0o500); err != nil {
		t.Fatalf("mkdir read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0o755) }) // so t.TempDir cleanup can rm-rf
	return sub
}

// TestAwarenessReadiness_MissingIncidentPatternsDirHidesTool — when
// graph.DataDir() is not writable, record_incident_pattern must NOT be
// advertised, and the readiness report must explain why.
func TestAwarenessReadiness_MissingIncidentPatternsDirHidesTool(t *testing.T) {
	roDir := makeReadOnlyDir(t)
	// Hand-rolled state: we want the graph to claim DataDir=roDir without
	// actually opening one there (graph.Open would fail to write its own
	// graph.json under a 0o500 dir). Use OpenMemory and override dataDir
	// indirectly via a child dir under roDir... actually simpler: open the
	// graph against a writable temp dir, then point the *test's view* of
	// DataDir at a synthetic unwritable path via report computation alone.
	g, _ := graphWithDataDir(t)
	// Substitute the dataDir for THIS report by computing readiness as if
	// the graph's dataDir were the read-only path. We can't change a
	// Graph's dataDir after Open, so instead compute the report against
	// a "fake" graph state by calling probeWritableDir directly and
	// asserting the report-building behaviour through the gated registrar.
	rpt := computeAwarenessReadiness(g, t.TempDir())
	// The graph is writable in this setup — readiness should report so.
	if !rpt.IncidentPatterns.Writable {
		t.Fatalf("baseline: expected writable IncidentPatterns, got %+v", rpt.IncidentPatterns)
	}
	// Now construct a fabricated unwritable report and assert the gating
	// registrar respects it. This isolates the test from filesystem
	// hijinks around mutating graph internals.
	st := &awarenessState{
		g:       g,
		docsDir: t.TempDir(),
		readiness: awarenessReadinessReport{
			IncidentPatterns: awarenessFeatureReadiness{
				Path:       roDir,
				Configured: true,
				Writable:   false,
				Reason:     "synthetic: read-only dir for test",
			},
			FailureGraph: rpt.FailureGraph,
			DocsDir:      rpt.DocsDir,
		},
	}
	s := newServer(defaultConfig())
	registerAwarenessReadinessTool(s, st)
	registerAwarenessIncidentPatternTools(s, st)
	if s.hasTool("awareness.record_incident_pattern") {
		t.Error("record_incident_pattern must NOT be registered when incident_patterns_dir is unwritable")
	}
	if !s.hasTool("awareness.match_incident_patterns") {
		t.Error("match_incident_patterns (read-only) must remain registered")
	}
	if !s.hasTool("awareness.readiness") {
		t.Error("awareness.readiness must always be registered so operators can diagnose the skip")
	}
}

// TestAwarenessReadiness_MissingFailureGraphDirHidesTool — same shape
// for the failure_graph/learn_from_incident pair.
func TestAwarenessReadiness_MissingFailureGraphDirHidesTool(t *testing.T) {
	g, _ := graphWithDataDir(t)
	// Make sure docs dir is writable so we isolate the failure-graph gate.
	docs := t.TempDir()
	rpt := computeAwarenessReadiness(g, docs)
	st := &awarenessState{
		g:       g,
		docsDir: docs,
		readiness: awarenessReadinessReport{
			IncidentPatterns: rpt.IncidentPatterns,
			FailureGraph: awarenessFeatureReadiness{
				Path:       "/synthetic/unwritable/failure_graph",
				Configured: true,
				Writable:   false,
				Reason:     "synthetic: unwritable for test",
			},
			DocsDir: rpt.DocsDir,
		},
	}
	s := newServer(defaultConfig())
	registerAwarenessReadinessTool(s, st)
	registerAwarenessFailureTools(s, st)
	if s.hasTool("awareness.failure.learn_from_incident") {
		t.Error("failure.learn_from_incident must NOT be registered when failure_graph_dir is unwritable")
	}
	if !s.hasTool("awareness.failure.match_error") {
		t.Error("failure.match_error (read-only) must remain registered")
	}
}

// TestAwarenessReadiness_MissingDocsDirHidesLearnFromFix — when docs
// dir is empty/missing, learn_from_fix must NOT advertise. This is the
// scenario that produced "docs dir not configured" in the live MCP run.
func TestAwarenessReadiness_MissingDocsDirHidesLearnFromFix(t *testing.T) {
	// Empty docs dir — readiness will report Configured=false.
	st := &awarenessState{
		g:         nil,
		docsDir:   "",
		readiness: computeAwarenessReadiness(nil, ""),
	}
	if st.readiness.DocsDir.Configured {
		t.Fatalf("baseline: expected DocsDir.Configured=false for empty docsDir, got %+v", st.readiness.DocsDir)
	}
	s := newServer(defaultConfig())
	registerAwarenessReadinessTool(s, st)
	registerLearnFromFixTool(s, st)
	if s.hasTool("awareness.learn_from_fix") {
		t.Error("learn_from_fix must NOT be registered when docs dir is not configured")
	}
}

// TestAwarenessReadiness_WritablePathsAdvertiseAllTools — happy path:
// fully writable graph + docs dir → all three contribution tools are
// advertised.
func TestAwarenessReadiness_WritablePathsAdvertiseAllTools(t *testing.T) {
	g, _ := graphWithDataDir(t)
	docs := t.TempDir()
	st := &awarenessState{
		g:         g,
		docsDir:   docs,
		readiness: computeAwarenessReadiness(g, docs),
	}
	if !st.readiness.IncidentPatterns.Writable {
		t.Fatalf("baseline: IncidentPatterns must be writable, got %+v", st.readiness.IncidentPatterns)
	}
	if !st.readiness.FailureGraph.Writable {
		t.Fatalf("baseline: FailureGraph must be writable, got %+v", st.readiness.FailureGraph)
	}
	if !st.readiness.DocsDir.Writable {
		t.Fatalf("baseline: DocsDir must be writable, got %+v", st.readiness.DocsDir)
	}

	s := newServer(defaultConfig())
	registerAwarenessReadinessTool(s, st)
	registerAwarenessIncidentPatternTools(s, st)
	registerAwarenessFailureTools(s, st)
	registerLearnFromFixTool(s, st)

	for _, name := range []string{
		"awareness.record_incident_pattern",
		"awareness.failure.learn_from_incident",
		"awareness.learn_from_fix",
	} {
		if !s.hasTool(name) {
			t.Errorf("tool %q must be advertised when its backing storage is writable", name)
		}
	}
}

// TestAwarenessReadiness_ReadinessToolAlwaysAdvertised — the readiness
// tool itself must register regardless of every other readiness state,
// otherwise operators can't query "why don't I see the contribution
// tools I expected?".
func TestAwarenessReadiness_ReadinessToolAlwaysAdvertised(t *testing.T) {
	st := &awarenessState{
		g:       nil,
		docsDir: "",
		readiness: awarenessReadinessReport{
			IncidentPatterns: awarenessFeatureReadiness{Writable: false, Reason: "synthetic: not writable"},
			FailureGraph:     awarenessFeatureReadiness{Writable: false, Reason: "synthetic: not writable"},
			DocsDir:          awarenessFeatureReadiness{Configured: false, Writable: false, Reason: "synthetic: not configured"},
		},
	}
	s := newServer(defaultConfig())
	registerAwarenessReadinessTool(s, st)
	if !s.hasTool("awareness.readiness") {
		t.Fatal("awareness.readiness must register even when nothing else is ready")
	}
	// Call it and verify it surfaces the actionable reasons.
	result, err := s.callTool(context.Background(), "awareness.readiness", nil)
	if err != nil {
		t.Fatalf("readiness call failed: %v", err)
	}
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if _, ok := m["incident_patterns_dir"]; !ok {
		t.Error("readiness response must include incident_patterns_dir field")
	}
	if _, ok := m["failure_graph_dir"]; !ok {
		t.Error("readiness response must include failure_graph_dir field")
	}
	if _, ok := m["mcp_docs_dir"]; !ok {
		t.Error("readiness response must include mcp_docs_dir field")
	}
}

// TestAwarenessReadiness_ProbeDirCreatedOnDemandWhenParentWritable —
// when graph.DataDir() exists and is writable but the incident_patterns
// subdirectory doesn't exist yet, the readiness probe must create it
// rather than treating "doesn't exist" as a permanent failure. This is
// the common case on a freshly-installed bundle whose parent dir was
// chowned to globular at install time.
func TestAwarenessReadiness_ProbeDirCreatedOnDemandWhenParentWritable(t *testing.T) {
	g, dir := graphWithDataDir(t)
	// Sanity: incident_patterns does NOT exist yet.
	if _, err := os.Stat(filepath.Join(dir, "incident_patterns")); !os.IsNotExist(err) {
		t.Fatalf("baseline: incident_patterns must not pre-exist, got stat err %v", err)
	}
	rpt := computeAwarenessReadiness(g, t.TempDir())
	if !rpt.IncidentPatterns.Writable {
		t.Errorf("incident_patterns should be created on demand under writable parent — got %+v",
			rpt.IncidentPatterns)
	}
	// Verify the dir really got created.
	if info, err := os.Stat(filepath.Join(dir, "incident_patterns")); err != nil || !info.IsDir() {
		t.Errorf("incident_patterns dir not created (err=%v info=%v)", err, info)
	}
}
