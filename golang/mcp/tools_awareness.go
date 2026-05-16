package main

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/globulario/awareness/graph"
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
		// 1. /var/lib/globular/awareness/current/graph.db  — active release bundle (symlink)
		// 2. /var/lib/globular/awareness/graph.db           — legacy system build
		// 3. <repoRoot>/.globular/awareness/graph.db        — dev-machine fallback
		const bundlePath = "/var/lib/globular/awareness/current/graph.db"
		const systemPath = "/var/lib/globular/awareness/graph.db"
		if _, err := os.Stat(bundlePath); err == nil {
			dbPath = bundlePath
		} else if _, err := os.Stat(systemPath); err == nil {
			dbPath = systemPath
		} else if repoRoot != "" {
			dbPath = filepath.Join(repoRoot, ".globular", "awareness", "graph.db")
		}
	}

	// Bundle paths are signed, content-addressed, root-owned, and immutable
	// post-install. graph.Open's migrate() would try to write DDL and fail
	// when the service user can't open the file read-write. OpenComposite
	// is the correct verb: the bundle is ATTACHed read-only and a writable
	// runtime database — runtime.db, sibling of the bundle — holds session,
	// coordination, experience, learning, and other mutable awareness data.
	// Non-bundle paths (dev checkouts, the legacy system db) still use Open
	// so behaviour for dev/CI runs is unchanged.
	useComposite := isAwarenessBundlePath(dbPath)

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
		if useComposite {
			runtimePath := awarenessRuntimeDBPath(dbPath)
			g, err = graph.OpenComposite(dbPath, runtimePath)
			if err == nil {
				log.Printf("mcp: awareness graph opened (composite): bundle=%s runtime=%s", dbPath, runtimePath)
			}
		} else {
			g, err = graph.Open(dbPath)
			if err == nil {
				log.Printf("mcp: awareness graph opened (read-write): %s", dbPath)
			}
		}
		if err == nil {
			st.g = g
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
	registerAwarenessAgentUsageTools(s, st)
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

// awarenessBundleRoot is the install root for content-addressed awareness
// bundles. Anything under it (including the /current symlink which points
// to /installed/<version>/<uuid>/) is treated as immutable signed content
// and opened read-only.
const awarenessBundleRoot = "/var/lib/globular/awareness/"

// awarenessRuntimeDBPath returns the writable runtime database path that
// pairs with the given bundle path. It lives inside the existing
// /var/lib/globular/awareness/runtime/ directory, which was created by
// the bundle installer with globular:globular ownership so the service
// user can create files there. The awareness root itself is root-owned
// (the bundle is installed by root and must stay so), so a sibling at
// the root level would fail to open for the service user.
//
// Path is stable regardless of which installed/<version>/<uuid>/ the
// /current symlink points to — bundle reinstalls swap the symlink, the
// runtime database stays put. The historical workaround under this
// directory ("graph.db", a writable copy of the bundle) is unreferenced
// by the new composite path and can be removed in a later cycle.
func awarenessRuntimeDBPath(bundlePath string) string {
	return filepath.Join(awarenessBundleRoot, "runtime", "runtime.db")
}

// isAwarenessBundlePath reports whether path lives inside a signed bundle.
// We resolve symlinks so /var/lib/globular/awareness/current/graph.db (a
// symlink into installed/<version>/<uuid>/) is correctly classified.
// Falls back to the lexical prefix when the link can't be resolved (e.g.,
// during dev with a broken symlink) — better to err on the side of
// composite-mode than to attempt a migrate against a root-owned file.
func isAwarenessBundlePath(path string) bool {
	if path == "" {
		return false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = path
	}
	const installedPrefix = awarenessBundleRoot + "installed/"
	const currentPrefix = awarenessBundleRoot + "current"
	return strings.HasPrefix(resolved, installedPrefix) ||
		strings.HasPrefix(path, currentPrefix)
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
