package main

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/awareness/extractors/manual"
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
		// Resolution order (most authoritative first):
		// 1. /var/lib/globular/awareness/current/graph.json  — active release bundle (symlink)
		// 2. /var/lib/globular/awareness/graph.json           — legacy system build
		// 3. <repoRoot>/.globular/awareness/graph.json        — dev-machine fallback
		const bundlePath = "/var/lib/globular/awareness/current/graph.json"
		const systemPath = "/var/lib/globular/awareness/graph.json"
		if _, err := os.Stat(bundlePath); err == nil {
			dbPath = bundlePath
		} else if _, err := os.Stat(systemPath); err == nil {
			dbPath = systemPath
		} else if repoRoot != "" {
			dbPath = filepath.Join(repoRoot, ".globular", "awareness", "graph.json")
		}
	}

	// Prefer docs dir from the installed bundle, then from the repo checkout.
	if docsDir == "" || !dirExists(docsDir) {
		const bundleDocsDir = "/var/lib/globular/awareness/current/docs"
		if dirExists(bundleDocsDir) {
			docsDir = bundleDocsDir
		}
	}

	st := &awarenessState{
		docsDir:  docsDir,
		repoRoot: repoRoot,
		nodeID:   cfg.NodeID,
	}

	if dbPath != "" {
		var g *graph.Graph
		var err error
		g, err = graph.Open(dbPath)
		if err == nil {
			log.Printf("mcp: awareness graph opened: %s", dbPath)
			// YAML-only fallback: if the graph has no nodes (graph.json absent or
			// empty) but a docs/ sibling directory exists, load manual knowledge
			// from YAML files. Keeps the MCP server partially operational when a
			// bundle ships docs/ but no pre-built graph.json yet.
			if stats, statsErr := g.Stats(context.Background()); statsErr == nil && stats.Nodes == 0 {
				var yamlDocsDir string
				// Try bundle docs dir first, then configured docsDir.
				const bundleDocsDir = "/var/lib/globular/awareness/current/docs"
				if dirExists(bundleDocsDir) {
					yamlDocsDir = bundleDocsDir
				} else if docsDir != "" && dirExists(docsDir) {
					yamlDocsDir = docsDir
				}
				if yamlDocsDir != "" {
					log.Printf("mcp: awareness graph empty — loading YAML knowledge from %s", yamlDocsDir)
					if loadErr := manual.LoadAll(context.Background(), g, yamlDocsDir); loadErr != nil {
						log.Printf("mcp: YAML docs load warning: %v", loadErr)
					}
				}
			}
		}
		if err == nil {
			st.g = g
			// Best-effort background cleanup of records older than 30 days.
			go func() {
				if cleanErr := g.Cleanup(30 * 24 * time.Hour); cleanErr != nil {
					log.Printf("mcp: awareness graph cleanup: %v", cleanErr)
				}
			}()
		} else {
			log.Printf("mcp: awareness graph unavailable (%s): %v — degraded mode", dbPath, err)
		}
	}

	registerAwarenessPreflightTools(s, st)
	registerAwarenessRuntimeTools(s, st)
	registerAwarenessPackageTools(s, st)
	registerAwarenessLearningTools(s, st)
	registerAwarenessNodeContextTools(s, st)
	registerAwarenessContextNavTools(s, st)
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
	registerAwarenessInvariantTools(s, st)
	registerAwarenessFreshnessTools(s, st)
	registerAwarenessIncidentPatternTools(s, st)
	registerAwarenessSessionOracleTools(s, st)
	registerAwarenessLiveClusterTools(s, st)
	registerAwarenessFailureTools(s, st)
	registerAwarenessFailureLearningTools(s, st)
	registerAwarenessEvidenceTools(s, st)
	registerAwarenessExperienceTools(s, st)
	// Phase B serve tools: independent of the awareness graph state, they
	// only read /var/lib/globular/awareness/current. Registered here so
	// they ship with the rest of the awareness tool group.
	registerAwarenessBundleServeTools(s)
	// Lean knowledge tools: assurance and selfcheck backed by the standalone
	// github.com/globulario/awareness module (missing-pieces merge).
	registerAwarenessKnowledgeTools(s, st)
}

// dirExists returns true if path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// awarGitRoot returns the git repository root, or "" if not inside a git checkout.
func awarGitRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
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
