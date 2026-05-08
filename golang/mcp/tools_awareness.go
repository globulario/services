package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/awareness/graph"
)

// awarenessState is captured in awareness tool handler closures.
// The graph may be nil when the DB is missing — all tools degrade gracefully.
type awarenessState struct {
	g        *graph.Graph
	docsDir  string
	repoRoot string
	nodeID   string
}

// registerAwarenessTools initialises the awareness state from the server config,
// opens the graph DB (non-fatal if absent), and registers all 12 awareness tools.
// promote-proposal is intentionally NOT registered.
func registerAwarenessTools(s *server) {
	cfg := &s.cfg.Awareness

	repoRoot := cfg.RepoPath
	if repoRoot == "" {
		repoRoot = awarGitRoot()
	}

	docsDir := cfg.DocsDir
	if docsDir == "" && repoRoot != "" {
		docsDir = filepath.Join(repoRoot, "docs", "awareness")
	}

	dbPath := cfg.DBPath
	if dbPath == "" {
		// Prefer system install path; fall back to repo-relative for dev mode.
		const systemDir = "/var/lib/globular/awareness"
		if _, err := os.Stat(systemDir); err == nil {
			dbPath = systemDir + "/graph.db"
		} else if repoRoot != "" {
			dbPath = filepath.Join(repoRoot, ".globular", "awareness", "graph.db")
		}
	}

	st := &awarenessState{
		docsDir:  docsDir,
		repoRoot: repoRoot,
		nodeID:   cfg.NodeID,
	}

	if dbPath != "" {
		if g, err := graph.Open(dbPath); err == nil {
			st.g = g
			log.Printf("mcp: awareness graph opened: %s", dbPath)
		} else {
			log.Printf("mcp: awareness graph unavailable (%s): %v — degraded mode", dbPath, err)
		}
	}

	registerAwarenessPreflightTools(s, st)
	registerAwarenessRuntimeTools(s, st)
	registerAwarenessFixledgerTools(s, st)
	registerAwarenessPackageTools(s, st)
	registerAwarenessLearningTools(s, st)
	registerAwarenessNodeContextTools(s, st)
	registerAwarenessSemanticTools(s, st)
	registerAwarenessDebugSessionTool(s, st)
	registerAwarenessIntegrityTools(s, st)
	registerAwarenessSessionTools(s, st)
	// New tools merged from golang/awareness/mcp.
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
	registerAwarenessDecisionTools(s, st)
}

// awarGitRoot returns the git repository root via git rev-parse.
// Falls back to the current working directory on failure.
func awarGitRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		cwd, _ := os.Getwd()
		return cwd
	}
	return strings.TrimSpace(string(out))
}

// strSliceArg extracts a []string from an MCP args map.
func strSliceArg(args map[string]interface{}, key string) []string {
	if v, ok := args[key]; ok {
		switch vv := v.(type) {
		case []interface{}:
			out := make([]string, 0, len(vv))
			for _, item := range vv {
				if s, ok := item.(string); ok {
					out = append(out, s)
				}
			}
			return out
		case []string:
			return vv
		}
	}
	return nil
}

// boolArg extracts a bool from an MCP args map (false if missing or wrong type).
func boolArg(args map[string]interface{}, key string) bool {
	if v, ok := args[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// awarOrEmpty filters empty strings from a slice, returning a non-nil slice.
func awarOrEmpty(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}
