package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/globulario/services/golang/awareness/graph"
)

// runGit runs a git command and returns trimmed stdout, or an error.
func runGit(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// newMCPWithDocsDir creates a test server with all awareness tools registered.
// Replaces the old awareness/mcp.NewWithGraph(Config{DocsDir: docsDir}, g) pattern.
// graph is optional — pass nil for degraded/offline tests.
func newMCPWithDocsDir(t *testing.T, docsDir string) *server {
	t.Helper()
	return newMCPWithGraph(t, docsDir, nil)
}

func newMCPWithGraph(t *testing.T, docsDir string, g *graph.Graph) *server {
	t.Helper()
	cfg := defaultConfig()
	// Do NOT enable Awareness in ToolGroups — that would cause newServer to call
	// registerAwarenessTools() which opens the disk graph and leaves it unclosed,
	// racing with the test's own registrations. We register awareness tools manually below.
	s := newServer(cfg)
	repoRoot := awarGitRoot()
	st := &awarenessState{g: g, docsDir: docsDir, repoRoot: repoRoot}
	registerAwarenessPreflightTools(s, st)
	registerAwarenessRuntimeTools(s, st)
	registerAwarenessFixledgerTools(s, st)
	registerAwarenessPackageTools(s, st)
	registerAwarenessLearningTools(s, st)
	registerAwarenessNodeContextTools(s, st)
	registerAwarenessSemanticTools(s, st)
	registerAwarenessDebugSessionTool(s, st)
	registerAwarenessIntegrityTools(s, st)
	registerPendingProposalsTool(s, st)
	registerExplainSymptomTool(s, st)
	registerScanViolationsTool(s, st)
	registerSuggestIncidentTool(s, st)
	registerLearnFromFixTool(s, st)
	registerOfflineDiagnoseTool(s, st)
	registerCausalChainTool(s, st)
	registerSelfReviewTools(s, st)
	registerCoverageReportTool(s, st)
	registerRuntimeActivationCheckTool(s, st)
	registerProposalQueueHealthTool(s, st)
	registerSuggestCausalRuleTool(s, st)
	registerHealthPulseTool(s, st)
	registerRuntimeConfigBootstrapTool(s, st)
	registerProposalDrainTools(s, st)
	if g != nil {
		t.Cleanup(func() { g.Close() })
	}
	return s
}
