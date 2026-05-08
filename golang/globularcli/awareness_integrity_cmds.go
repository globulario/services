package main

// awareness_integrity_cmds.go: CLI commands for graph integrity and impact path (Phase 11).
//
// Commands:
//
//	globular awareness graph-integrity-check [--repo <path>] [--db <path>] [--docs <path>]
//	    [--test-results <file>] [--strict] [--json]
//	globular awareness impact-path --files <f1,f2,...> [--db <path>] [--max-depth <n>] [--json]

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/integrity"
)

var integrityCfg = struct {
	docsDir     string
	testResults string
	strict      bool
	jsonOutput  bool
	files       []string
	maxDepth    int
}{
	maxDepth: 6,
}

// ── graph-integrity-check ─────────────────────────────────────────────────────

var awarenessGraphIntegrityCheckCmd = &cobra.Command{
	Use:   "graph-integrity-check",
	Short: "Validate the awareness knowledge graph for stale references, shape violations, and contradictions",
	Long: `Runs four categories of checks:

  1. Shape validation — DONE fix cases with missing tests, invalid failure mode
     references, missing safe_alternative on forbidden fixes, malformed causal rules
  2. Contradiction detection — causal rules that recommend forbidden operations
     (e.g. etcd alarm disarm before compact)
  3. Test reference integrity — required tests that are missing from disk or failed in CI
  4. Graph-dependent checks — stale edges, missing edge provenance, orphan nodes
     (requires an indexed awareness graph)

Exit codes: 0=healthy, 1=warning, 2=critical.
Use --strict to treat warnings as critical.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		docsDir := integrityCfg.docsDir
		if docsDir == "" {
			docsDir = filepath.Join(repoRoot, "docs", "awareness")
		}

		g, _ := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if g != nil {
			defer g.Close()
		}

		opts := integrity.Options{
			DocsDir:         docsDir,
			RepoRoot:        repoRoot,
			Strict:          integrityCfg.strict,
			TestResultsFile: integrityCfg.testResults,
		}

		result, err := integrity.Check(ctx, opts, g)
		if err != nil {
			return fmt.Errorf("integrity check: %w", err)
		}

		if integrityCfg.jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(result)
		} else {
			printIntegrityResult(result)
		}

		os.Exit(result.ExitCode)
		return nil
	},
}

func printIntegrityResult(r *integrity.IntegrityResult) {
	statusIcon := map[string]string{
		"healthy":  "✓",
		"warning":  "⚠",
		"critical": "✗",
	}[r.Status]
	if statusIcon == "" {
		statusIcon = "?"
	}

	fmt.Fprintf(os.Stdout, "\n## Awareness Graph Integrity — %s %s\n\n", statusIcon, strings.ToUpper(r.Status))
	fmt.Fprintf(os.Stdout, "  nodes: %d  edges: %d\n", r.Summary.Nodes, r.Summary.Edges)
	fmt.Fprintf(os.Stdout, "  shape violations: %d  missing tests: %d  contradictions: %d\n",
		r.Summary.InvalidShapes, r.Summary.MissingTests, r.Summary.Contradictions)
	if r.Summary.StaleEdges > 0 || r.Summary.OrphanNodes > 0 || r.Summary.EdgesWithoutProvenance > 0 {
		fmt.Fprintf(os.Stdout, "  stale edges: %d  orphan nodes: %d  missing provenance: %d\n",
			r.Summary.StaleEdges, r.Summary.OrphanNodes, r.Summary.EdgesWithoutProvenance)
	}
	fmt.Fprintln(os.Stdout)

	if len(r.InvalidShapes) > 0 {
		fmt.Fprintln(os.Stdout, "### Shape Violations")
		for _, v := range r.InvalidShapes {
			fmt.Fprintf(os.Stdout, "  [%s] %s.%s — %s\n", strings.ToUpper(v.Severity), v.NodeID, v.Field, v.Message)
		}
		fmt.Fprintln(os.Stdout)
	}

	if len(r.Contradictions) > 0 {
		fmt.Fprintln(os.Stdout, "### Contradictions")
		for _, c := range r.Contradictions {
			fmt.Fprintf(os.Stdout, "  [CRITICAL] rule:%s step:%s — %s\n", c.CausalRuleID, c.Step, c.Reason)
			if c.ForbiddenFixID != "" {
				fmt.Fprintf(os.Stdout, "    forbidden fix: %s\n", c.ForbiddenFixID)
			}
		}
		fmt.Fprintln(os.Stdout)
	}

	if len(r.MissingTests) > 0 {
		fmt.Fprintln(os.Stdout, "### Missing Tests")
		for _, ti := range r.MissingTests {
			fmt.Fprintf(os.Stdout, "  [%s] %s — %s (%s)\n",
				strings.ToUpper(ti.Severity), ti.FixCaseID, ti.TestName, ti.Issue)
		}
		fmt.Fprintln(os.Stdout)
	}

	if len(r.StaleEdges) > 0 {
		fmt.Fprintln(os.Stdout, "### Stale Edges")
		for _, e := range r.StaleEdges {
			fmt.Fprintf(os.Stdout, "  %s -[%s]-> %s: %s\n", e.Src, e.Kind, e.Dst, e.Reason)
		}
		fmt.Fprintln(os.Stdout)
	}

	if len(r.OrphanNodes) > 0 {
		fmt.Fprintln(os.Stdout, "### Orphan Nodes")
		for _, id := range r.OrphanNodes {
			fmt.Fprintf(os.Stdout, "  %s\n", id)
		}
		fmt.Fprintln(os.Stdout)
	}

	if len(r.RecommendedActions) > 0 {
		fmt.Fprintln(os.Stdout, "### Recommended Actions")
		for _, a := range r.RecommendedActions {
			fmt.Fprintf(os.Stdout, "  • %s\n", a)
		}
		fmt.Fprintln(os.Stdout)
	}
}

// ── impact-path ───────────────────────────────────────────────────────────────

var awarenessImpactPathCmd = &cobra.Command{
	Use:   "impact-path",
	Short: "Traverse the awareness graph from changed files to impacted invariants, tests, and failure modes",
	Long: `Performs a typed BFS from each changed file through the awareness graph,
returning chains of edges that lead to invariants, tests, and failure modes.

Each step carries a trust level (verified, declared, inferred) and paths through
inferred edges are labelled low-confidence.

Requires an indexed awareness graph (run 'globular awareness build' first).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		if len(integrityCfg.files) == 0 {
			return fmt.Errorf("--files is required")
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		q := integrity.ImpactPathQuery{
			ChangedFiles: integrityCfg.files,
			MaxDepth:     integrityCfg.maxDepth,
		}

		paths, err := integrity.TraverseImpactPaths(ctx, g, q)
		if err != nil {
			return fmt.Errorf("impact path: %w", err)
		}

		if integrityCfg.jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]interface{}{
				"paths": paths,
				"count": len(paths),
			})
			return nil
		}

		printImpactPaths(paths)
		return nil
	},
}

func printImpactPaths(paths []integrity.ImpactPath) {
	if len(paths) == 0 {
		fmt.Fprintln(os.Stdout, "No impact paths found. Run 'globular awareness build' to index the graph.")
		return
	}

	fmt.Fprintf(os.Stdout, "\n## Impact Paths (%d)\n\n", len(paths))

	lastFile := ""
	for _, p := range paths {
		if p.ChangedFile != lastFile {
			fmt.Fprintf(os.Stdout, "### %s\n\n", p.ChangedFile)
			lastFile = p.ChangedFile
		}

		if p.Note != "" {
			fmt.Fprintf(os.Stdout, "  note: %s\n\n", p.Note)
			continue
		}

		confIcon := map[string]string{
			"high":   "◉",
			"medium": "◎",
			"low":    "○",
		}[p.Confidence]
		if confIcon == "" {
			confIcon = "○"
		}

		fmt.Fprintf(os.Stdout, "  %s [%s confidence]\n", confIcon, p.Confidence)
		for _, step := range p.Steps {
			fmt.Fprintf(os.Stdout, "    → %s (%s) via %s [%s]\n",
				step.NodeName, step.NodeType, step.Predicate, step.Trust)
		}
		fmt.Fprintln(os.Stdout)
	}
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// graph-integrity-check
	awarenessGraphIntegrityCheckCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessGraphIntegrityCheckCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessGraphIntegrityCheckCmd.Flags().StringVar(&integrityCfg.docsDir, "docs", "", "Path to docs/awareness (default: <repo>/docs/awareness)")
	awarenessGraphIntegrityCheckCmd.Flags().StringVar(&integrityCfg.testResults, "test-results", "", "Path to CI test results JSON file")
	awarenessGraphIntegrityCheckCmd.Flags().BoolVar(&integrityCfg.strict, "strict", false, "Treat warnings as critical (exit code 2)")
	awarenessGraphIntegrityCheckCmd.Flags().BoolVar(&integrityCfg.jsonOutput, "json", false, "Output JSON")

	// impact-path
	awarenessImpactPathCmd.Flags().StringSliceVar(&integrityCfg.files, "files", nil, "Comma-separated list of changed files (repo-relative)")
	awarenessImpactPathCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessImpactPathCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessImpactPathCmd.Flags().IntVar(&integrityCfg.maxDepth, "max-depth", 6, "Maximum edge hops to traverse")
	awarenessImpactPathCmd.Flags().BoolVar(&integrityCfg.jsonOutput, "json", false, "Output JSON")

	awarenessCmd.AddCommand(awarenessGraphIntegrityCheckCmd)
	awarenessCmd.AddCommand(awarenessImpactPathCmd)
}
