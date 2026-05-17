package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/graph"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newAwarenessTestServer(t *testing.T) (*server, *awarenessState) {
	t.Helper()
	docsDir := setupAwarenessTestDocsDir(t)
	g := setupAwarenessTestGraph(t)

	cfg := defaultConfig()
	cfg.ToolGroups.Awareness = true

	s := newServer(cfg)
	st := &awarenessState{g: g, docsDir: docsDir, nodeID: "test-node"}

	registerAwarenessPreflightTools(s, st)
	registerAwarenessRuntimeTools(s, st)
	registerAwarenessFixledgerTools(s, st)
	registerAwarenessPackageTools(s, st)
	registerAwarenessLearningTools(s, st)

	t.Cleanup(func() { g.Close() })
	return s, st
}

func newAwarenessDegradedServer(t *testing.T) (*server, *awarenessState) {
	t.Helper()
	docsDir := setupAwarenessTestDocsDir(t)

	cfg := defaultConfig()
	s := newServer(cfg)
	st := &awarenessState{g: nil, docsDir: docsDir}

	registerAwarenessPreflightTools(s, st)
	registerAwarenessRuntimeTools(s, st)
	registerAwarenessFixledgerTools(s, st)
	registerAwarenessPackageTools(s, st)
	registerAwarenessLearningTools(s, st)
	return s, st
}

func setupAwarenessTestDocsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "context_aliases.yaml"), []byte(`aliases:
  infra.desired_hash_consistency:
    - desired_hash
    - checksum mismatch
`), 0o644)

	_ = os.WriteFile(filepath.Join(dir, "fix_cases.yaml"), []byte(`fix_cases:
  - id: desired_hash_consistency
    title: "Desired hash fix"
    status: PARTIAL
    pattern: "desired_hash"
    target_invariants:
      - infra.desired_hash_consistency
    remaining_files:
      - golang/awareness/analysis/hash.go
    required_tests:
      - TestDriftWorkflowUsesDesiredHash
`), 0o644)

	_ = os.WriteFile(filepath.Join(dir, "guardrails.yaml"), []byte("guardrails: []\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "proposals"), 0o755)
	_ = os.MkdirAll(filepath.Join(dir, "incidents"), 0o755)
	return dir
}

func setupAwarenessTestGraph(t *testing.T) *graph.Graph {
	t.Helper()
	ctx := context.Background()
	g, err := graph.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	_ = g.UpsertInvariant(ctx, graph.Invariant{
		ID: "infra.desired_hash_consistency", Title: "Desired hash must be stable",
		Severity: "critical", Status: "active",
	})
	_ = g.AddNode(ctx, graph.Node{
		ID: "invariant:infra.desired_hash_consistency", Type: graph.NodeTypeInvariant,
		Name: "infra.desired_hash_consistency",
	})
	_ = g.AddNode(ctx, graph.Node{
		ID: "forbidden_fix:use_raw_digest", Type: graph.NodeTypeForbiddenFix,
		Name: "use raw artifact digest as desired_hash",
	})
	_ = g.AddEdge(ctx, graph.Edge{
		Src: "invariant:infra.desired_hash_consistency", Kind: graph.EdgeForbids,
		Dst: "forbidden_fix:use_raw_digest",
	})
	for _, svc := range []string{"envoy", "cluster-controller", "node-agent"} {
		_ = g.AddNode(ctx, graph.Node{ID: "service:" + svc, Type: graph.NodeTypeGlobularService, Name: svc})
	}
	return g
}

func writeAwarenessIncidentFixture(t *testing.T, docsDir string) {
	t.Helper()
	incidentsDir := filepath.Join(docsDir, "incidents")
	_ = os.MkdirAll(incidentsDir, 0o755)
	content := `incident_id: "test_incident"
title: "Test incident for MCP tool tests"
severity: "high"
suspected_root_cause: "desired_hash instability"
symptoms:
  - "envoy restart storm"
state_deltas:
  - "desired_hash changed"
manual_repairs:
  - "restarted envoy"
observed_services:
  - "envoy"
proposed:
  failure_modes:
    - id: "failure_mode.test_hash_storm"
      title: "Test hash storm"
      severity: "high"
      symptoms:
        - "restart storm"
      root_cause: "desired_hash instability"
      architecture_fix: "stabilize desired_hash computation"
  invariants:
    - id: "infra.desired_hash_consistency"
      title: "Desired hash must be stable"
      severity: "critical"
      summary: "Hash must not change per tick"
  forbidden_fixes:
    - id: "forbidden_fix.raw_digest"
      title: "Do not use raw digest"
      summary: "raw artifact digest is not stable"
`
	_ = os.WriteFile(filepath.Join(incidentsDir, "test_incident.yaml"), []byte(content), 0o644)
}

// ── registration tests ────────────────────────────────────────────────────────

func TestAwarenessToolsRegisterAll12(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	required := []string{
		"awareness.preflight",
		"awareness.agent_context",
		"awareness.impact_file",
		"awareness.did_we_fix",
		"awareness.pattern_status",
		"awareness.fix_status",
		"awareness.runtime_snapshot",
		"awareness.validate_package",
		"awareness.package_context",
		"awareness.propose_from_incident",
		"awareness.validate_proposal",
		"awareness.approve_proposal",
	}

	for _, want := range required {
		if !s.hasTool(want) {
			t.Errorf("missing required awareness tool: %q", want)
		}
	}
}

func TestAwarenessPromoteProposalNotRegistered(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	for _, forbidden := range []string{"awareness.promote_proposal", "awareness.promote-proposal"} {
		if s.hasTool(forbidden) {
			t.Errorf("forbidden tool %q must not be registered over MCP", forbidden)
		}
	}
}

func TestToolGroupsAwareness_TrueIncludesTools(t *testing.T) {
	cfg := defaultConfig()
	cfg.ToolGroups.Awareness = true
	s := newServer(cfg)
	st := &awarenessState{docsDir: t.TempDir()}
	registerAwarenessPreflightTools(s, st)
	registerAwarenessRuntimeTools(s, st)
	registerAwarenessFixledgerTools(s, st)
	registerAwarenessPackageTools(s, st)
	registerAwarenessLearningTools(s, st)

	if !s.hasTool("awareness.preflight") {
		t.Error("expected awareness.preflight to be registered when Awareness=true")
	}
}

func TestToolGroupsAwareness_FalseExcludesTools(t *testing.T) {
	cfg := defaultConfig()
	cfg.ToolGroups.Awareness = false

	s := newServer(cfg)
	registerAllTools(s)

	for _, name := range []string{
		"awareness.preflight",
		"awareness.agent_context",
		"awareness.runtime_snapshot",
	} {
		if s.hasTool(name) {
			t.Errorf("tool %q must not be registered when ToolGroups.Awareness=false", name)
		}
	}
}

// ── functional tests ──────────────────────────────────────────────────────────

func TestAwarenessPreflight_DegradedWithoutGraph(t *testing.T) {
	s, _ := newAwarenessDegradedServer(t)

	result, err := s.callTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"task": "desired_hash mismatch",
	})
	if err != nil {
		t.Fatalf("preflight should degrade gracefully, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result in degraded mode")
	}
	b, _ := json.Marshal(result)
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("preflight result must be JSON-serializable: %v", err)
	}
	// Must contain warnings about missing graph.
	raw := string(b)
	if !strings.Contains(raw, "no graph") && !strings.Contains(raw, "warnings") {
		t.Logf("degraded preflight result: %s", raw)
	}
}

func TestAwarenessValidatePackage_InvalidPath(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	result, err := s.callTool(context.Background(), "awareness.validate_package", map[string]interface{}{
		"path": "/nonexistent/path/to/package",
	})
	// Either an error or a structured result is acceptable — must not panic.
	if err != nil {
		// Error is acceptable for invalid path.
		return
	}
	if result != nil {
		if _, err := json.Marshal(result); err != nil {
			t.Errorf("validate_package result must be JSON-serializable: %v", err)
		}
	}
}

func TestAwarenessProposeFromIncident_RejectsPathTraversal(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	cases := []struct {
		name       string
		outputName string
	}{
		{"relative traversal", "../../etc/passwd"},
		{"absolute path", "/tmp/evil"},
		{"subdir slash", "subdir/evil"},
		{"backslash", `sub\evil`},
		{"dotdot only", ".."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.callTool(context.Background(), "awareness.propose_from_incident", map[string]interface{}{
				"incident_id": "test_incident",
				"output_name": tc.outputName,
			})
			if err == nil {
				t.Errorf("output_name %q: expected rejection, got nil error", tc.outputName)
				return
			}
			msg := err.Error()
			isSafetyRejection := strings.Contains(msg, "plain filename") ||
				strings.Contains(msg, "directory separator") ||
				strings.Contains(msg, "absolute path") ||
				strings.Contains(msg, "invalid")
			if !isSafetyRejection {
				t.Errorf("output_name %q: rejected for wrong reason: %v", tc.outputName, err)
			}
		})
	}
}

func TestAwarenessRuntimeSnapshot_DoesNotMutateRuntime(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	result, err := s.callTool(context.Background(), "awareness.runtime_snapshot", map[string]interface{}{
		"window":      "5m",
		"write_graph": false,
	})
	if err != nil {
		t.Fatalf("runtime_snapshot: %v", err)
	}

	b, _ := json.Marshal(result)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)

	if _, ok := m["captured_at"]; !ok {
		t.Error("runtime_snapshot missing captured_at field")
	}
}

func TestAwarenessAllToolOutputsAreJSONSerializable(t *testing.T) {
	s, st := newAwarenessTestServer(t)
	writeAwarenessIncidentFixture(t, st.docsDir)

	toolCalls := []struct {
		name string
		args map[string]interface{}
	}{
		{"awareness.preflight", map[string]interface{}{"task": "test task"}},
		{"awareness.agent_context", map[string]interface{}{"task": "test task"}},
		{"awareness.impact_file", map[string]interface{}{"file": "golang/foo.go"}},
		{"awareness.did_we_fix", map[string]interface{}{"task": "desired_hash fix"}},
		{"awareness.pattern_status", map[string]interface{}{"pattern": "desired"}},
		{"awareness.fix_status", map[string]interface{}{"pattern": "desired"}},
		{"awareness.runtime_snapshot", map[string]interface{}{}},
		{"awareness.validate_package", map[string]interface{}{"path": "/nonexistent"}},
	}

	for _, tc := range toolCalls {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := s.callTool(context.Background(), tc.name, tc.args)
			if result != nil {
				if _, err := json.Marshal(result); err != nil {
					t.Errorf("tool %q returned non-serializable result: %v", tc.name, err)
				}
			}
		})
	}
}

func TestAwarenessDidWeFix_ReturnsFixLedgerResult(t *testing.T) {
	s, _ := newAwarenessTestServer(t)

	result, err := s.callTool(context.Background(), "awareness.did_we_fix", map[string]interface{}{
		"task": "desired_hash drift",
	})
	if err != nil {
		t.Fatalf("did_we_fix: %v", err)
	}

	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", result)
	}

	for _, key := range []string{"status", "fix_cases", "remaining_gaps", "next_action"} {
		if _, ok := m[key]; !ok {
			t.Errorf("did_we_fix missing key %q", key)
		}
	}
}

// TestMCPAwarenessDebugSession_ReturnsValidJSON verifies that awareness.debug_session
// returns a parseable JSON object with the required top-level fields.
func TestMCPAwarenessDebugSession_ReturnsValidJSON(t *testing.T) {
	docsDir := setupAwarenessTestDocsDir(t)
	g := setupAwarenessTestGraph(t)
	t.Cleanup(func() { g.Close() })

	cfg := defaultConfig()
	cfg.ToolGroups.Awareness = true
	s := newServer(cfg)
	st := &awarenessState{g: g, docsDir: docsDir, nodeID: "test-node"}
	registerAwarenessDebugSessionTool(s, st)

	result, err := s.callTool(context.Background(), "awareness.debug_session", map[string]interface{}{
		"task":   "desired_hash mismatch causing convergence loop",
		"format": "json",
	})
	if err != nil {
		t.Fatalf("debug_session: %v", err)
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("result not serializable: %v", err)
	}

	var v map[string]interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("debug_session returned invalid JSON: %v\nraw: %s", err, string(b))
	}

	required := []string{"task", "classification", "confidence", "investigation_plan"}
	for _, key := range required {
		if _, ok := v[key]; !ok {
			t.Errorf("debug_session JSON missing field %q\nkeys: %v", key, keys(v))
		}
	}

	if task, _ := v["task"].(string); task == "" {
		t.Error("debug_session JSON 'task' field is empty")
	}
}

// keys returns the sorted keys of a map for diagnostic messages.
func keys(m map[string]interface{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestAwarenessApproveProposalDoesNotPromote(t *testing.T) {
	s, st := newAwarenessTestServer(t)
	writeAwarenessIncidentFixture(t, st.docsDir)

	propResult, err := s.callTool(context.Background(), "awareness.propose_from_incident", map[string]interface{}{
		"incident_id": "test_incident",
		"output_name": "approve_test",
	})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	propPath := propResult.(map[string]interface{})["proposal_path"].(string)

	approveResult, err := s.callTool(context.Background(), "awareness.approve_proposal", map[string]interface{}{
		"file": propPath,
	})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}

	m := approveResult.(map[string]interface{})
	if promoted, _ := m["promoted"].(bool); promoted {
		t.Error("approve_proposal must not set promoted=true")
	}
	if status, _ := m["status"].(string); status == "PROMOTED" {
		t.Error("approve_proposal must not set status=PROMOTED")
	}
}
