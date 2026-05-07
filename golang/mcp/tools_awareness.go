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
	g       *graph.Graph
	docsDir string
	nodeID  string
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
	if dbPath == "" && repoRoot != "" {
		dbPath = filepath.Join(repoRoot, ".globular", "awareness", "graph.db")
	}

	st := &awarenessState{
		docsDir: docsDir,
		nodeID:  cfg.NodeID,
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
