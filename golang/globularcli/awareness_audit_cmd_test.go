package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCollectFilePathsFromNT verifies that the NT scanner extracts
// repo-relative file paths from sourceFile IRIs.
func TestCollectFilePathsFromNT(t *testing.T) {
	nt := strings.Join([]string{
		`<https://globular.io/awareness#sourceFile/golang%2Fserver%2Fbriefing.go> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <https://globular.io/awareness#SourceFile> .`,
		`<https://globular.io/awareness#sourceFile/golang%2Fnode_agent%2Fnode_agent_server%2Fheartbeat.go> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <https://globular.io/awareness#SourceFile> .`,
		`<https://globular.io/awareness#invariant/test_inv> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <https://globular.io/awareness#Invariant> .`,
	}, "\n")
	paths := collectFilePathsFromNT([]byte(nt))
	if !paths["golang/server/briefing.go"] {
		t.Errorf("expected golang/server/briefing.go in paths; got %v", paths)
	}
	if !paths["golang/node_agent/node_agent_server/heartbeat.go"] {
		t.Errorf("expected heartbeat.go in paths; got %v", paths)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 paths; got %d: %v", len(paths), paths)
	}
}

// TestAuditStaleFileRefs_DualRepoCheck verifies that paths existing in
// either the services repo OR the awareness-graph repo are not flagged
// as stale. Only paths missing from both repos are stale.
func TestAuditStaleFileRefs_DualRepoCheck(t *testing.T) {
	svcDir := t.TempDir()
	agDir := t.TempDir()

	// Create a file in each repo.
	os.MkdirAll(filepath.Join(svcDir, "golang", "node_agent"), 0o755)
	os.WriteFile(filepath.Join(svcDir, "golang", "node_agent", "heartbeat.go"), []byte("package node_agent"), 0o644)

	os.MkdirAll(filepath.Join(agDir, "golang", "server"), 0o755)
	os.WriteFile(filepath.Join(agDir, "golang", "server", "briefing.go"), []byte("package server"), 0o644)

	// Build NT with 3 sourceFile refs: one in each repo, one in neither.
	nt := strings.Join([]string{
		`<https://globular.io/awareness#sourceFile/golang%2Fnode_agent%2Fheartbeat.go> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <https://globular.io/awareness#SourceFile> .`,
		`<https://globular.io/awareness#sourceFile/golang%2Fserver%2Fbriefing.go> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <https://globular.io/awareness#SourceFile> .`,
		`<https://globular.io/awareness#sourceFile/golang%2Fmissing%2Fdeleted.go> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <https://globular.io/awareness#SourceFile> .`,
	}, "\n")

	result := auditStaleFileRefs(svcDir, agDir, []byte(nt))

	if result.result != checkWARN {
		t.Fatalf("expected WARN (1 stale ref); got %s: %s", result.result, result.summary)
	}
	if len(result.details) != 1 {
		t.Fatalf("expected 1 stale detail; got %d: %v", len(result.details), result.details)
	}
	if result.details[0] != "golang/missing/deleted.go" {
		t.Errorf("expected stale ref 'golang/missing/deleted.go'; got %q", result.details[0])
	}
}

// TestAuditStaleFileRefs_GlobPatternsSkipped verifies that paths
// containing glob characters (*, ?, [) are skipped, not checked.
func TestAuditStaleFileRefs_GlobPatternsSkipped(t *testing.T) {
	svcDir := t.TempDir()
	agDir := t.TempDir()

	nt := `<https://globular.io/awareness#sourceFile/golang%2F*%2F*_server%2Fzz_version_generated.go> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <https://globular.io/awareness#SourceFile> .`

	result := auditStaleFileRefs(svcDir, agDir, []byte(nt))

	// Glob path skipped → 0 checked → PASS.
	if result.result != checkPASS {
		t.Fatalf("expected PASS (glob skipped); got %s: %s", result.result, result.summary)
	}
	if !strings.Contains(result.summary, "glob patterns skipped") {
		t.Errorf("expected summary to mention glob patterns; got %q", result.summary)
	}
}

// TestAuditStaleFileRefs_AllExist verifies PASS when all paths exist.
func TestAuditStaleFileRefs_AllExist(t *testing.T) {
	svcDir := t.TempDir()
	agDir := t.TempDir()

	os.MkdirAll(filepath.Join(svcDir, "golang", "echo"), 0o755)
	os.WriteFile(filepath.Join(svcDir, "golang", "echo", "main.go"), []byte("package main"), 0o644)

	nt := `<https://globular.io/awareness#sourceFile/golang%2Fecho%2Fmain.go> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <https://globular.io/awareness#SourceFile> .`

	result := auditStaleFileRefs(svcDir, agDir, []byte(nt))

	if result.result != checkPASS {
		t.Fatalf("expected PASS; got %s: %s", result.result, result.summary)
	}
}

// TestAuditStaleFileRefs_FixOnlyTouchesServicesRepo confirms that the
// removeStaleFileRefsFromYAML function only modifies files in the
// services repo, never the AG repo.
func TestAuditStaleFileRefs_FixOnlyTouchesServicesRepo(t *testing.T) {
	svcDir := t.TempDir()
	agDir := t.TempDir()

	// Create a minimal invariants.yaml in the services repo with a stale ref.
	awarenessDir := filepath.Join(svcDir, "docs", "awareness")
	os.MkdirAll(awarenessDir, 0o755)
	yamlContent := `invariants:
  - id: test_inv
    severity: high
    protects:
      files:
        - golang/existing/file.go
        - golang/missing/deleted.go
`
	os.WriteFile(filepath.Join(awarenessDir, "invariants.yaml"), []byte(yamlContent), 0o644)

	// Create a dummy file in AG repo awareness dir — should NOT be touched.
	agAwareness := filepath.Join(agDir, "docs", "awareness")
	os.MkdirAll(agAwareness, 0o755)
	agYAML := "invariants:\n  - id: ag_inv\n"
	os.WriteFile(filepath.Join(agAwareness, "invariants.yaml"), []byte(agYAML), 0o644)

	// Run the fix.
	staleFiles := []string{"golang/missing/deleted.go"}
	removed := removeStaleFileRefsFromYAML(svcDir, agDir, staleFiles)

	if removed != 1 {
		t.Errorf("expected 1 removal; got %d", removed)
	}

	// Verify services YAML was modified.
	svcData, _ := os.ReadFile(filepath.Join(awarenessDir, "invariants.yaml"))
	if strings.Contains(string(svcData), "deleted.go") {
		t.Error("services invariants.yaml still contains deleted.go after fix")
	}

	// Verify AG YAML was NOT modified.
	agData, _ := os.ReadFile(filepath.Join(agAwareness, "invariants.yaml"))
	if string(agData) != agYAML {
		t.Error("AG invariants.yaml was modified — fix must not touch AG repo")
	}
}
