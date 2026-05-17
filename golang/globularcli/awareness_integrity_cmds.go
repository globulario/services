package main

// awareness_integrity_cmds.go: CLI commands for graph integrity and impact path.
//
// Commands:
//
//	globular awareness graph-integrity-check [--repo <path>] [--db <path>] [--docs <path>]
//	    [--test-results <file>] [--strict] [--json]
//	globular awareness impact-path --files <f1,f2,...> [--db <path>] [--max-depth <n>] [--json]
//
// Exit codes for graph-integrity-check:
//
//	0 — no errors, all checks passed
//	1 — errors found (CI fails)
//	2 — errors found AND --strict mode (critical; also exits 2 when strict+warnings)

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/enforce"
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
	Short: "Validate the awareness knowledge graph with CI-grade exit codes",
	Long: `Runs GraphIntegrityCICheck and exits with:
  0 — no errors (pass)
  1 — errors found (CI fails)
  2 — errors found in --strict mode (or any finding when strict is set)

In --strict mode, warnings are also treated as failures (exit 2).
Designed for CI pipeline wiring: "globular awareness graph-integrity-check --strict"`,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			// No graph is a non-fatal warning — CI should still report the check state.
			fmt.Fprintf(os.Stderr, "warning: could not open graph: %v\n", err)
		}
		if g != nil {
			defer g.Close()
		}

		repoRoot, _ := resolveRepoRoot(awareCfg.repoPath)
		docsDir := integrityCfg.docsDir
		if docsDir == "" && repoRoot != "" {
			docsDir = filepath.Join(repoRoot, "docs", "awareness")
		}

		ciOpts := enforce.CICheckOptions{
			RepoRoot:              repoRoot,
			DocsDir:               docsDir,
			MaxScaffoldSkips:      0,
			MaxRequiredTestNoPath: 0,
		}

		res := enforce.GraphIntegrityCICheck(ctx, g, ciOpts)

		if integrityCfg.jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]interface{}{
				"pass":            res.Pass,
				"error_count":     res.ErrorCount,
				"warning_count":   res.WarningCount,
				"failure_reasons": res.FailureReasons,
				"findings":        res.Findings,
			})
		} else {
			if res.Pass && res.WarningCount == 0 {
				fmt.Fprintln(os.Stdout, "graph-integrity: PASS — no issues found")
			} else {
				for _, f := range res.Findings {
					fmt.Fprintf(os.Stdout, "[%s] %s: %s\n", f.Severity, f.Code, f.Message)
				}
				if !res.Pass {
					fmt.Fprintf(os.Stderr, "\nFAIL: %d error(s), %d warning(s)\n", res.ErrorCount, res.WarningCount)
					for _, r := range res.FailureReasons {
						fmt.Fprintf(os.Stderr, "  • %s\n", r)
					}
				}
			}
		}

		// Exit code logic:
		//   0 — clean pass
		//   1 — errors (default mode)
		//   2 — errors in strict mode, OR any warning/error in strict mode
		if !res.Pass {
			if integrityCfg.strict {
				os.Exit(2)
			}
			os.Exit(1)
		}
		if integrityCfg.strict && res.WarningCount > 0 {
			os.Exit(2)
		}
		return nil
	},
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
	awarenessGraphIntegrityCheckCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.json")
	awarenessGraphIntegrityCheckCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessGraphIntegrityCheckCmd.Flags().StringVar(&integrityCfg.docsDir, "docs", "", "Path to docs/awareness (default: <repo>/docs/awareness)")
	awarenessGraphIntegrityCheckCmd.Flags().StringVar(&integrityCfg.testResults, "test-results", "", "Path to CI test results JSON file")
	awarenessGraphIntegrityCheckCmd.Flags().BoolVar(&integrityCfg.strict, "strict", false, "Treat warnings as critical (exit code 2)")
	awarenessGraphIntegrityCheckCmd.Flags().BoolVar(&integrityCfg.jsonOutput, "json", false, "Output JSON")

	// impact-path
	awarenessImpactPathCmd.Flags().StringSliceVar(&integrityCfg.files, "files", nil, "Comma-separated list of changed files (repo-relative)")
	awarenessImpactPathCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.json")
	awarenessImpactPathCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessImpactPathCmd.Flags().IntVar(&integrityCfg.maxDepth, "max-depth", 6, "Maximum edge hops to traverse")
	awarenessImpactPathCmd.Flags().BoolVar(&integrityCfg.jsonOutput, "json", false, "Output JSON")

	awarenessCmd.AddCommand(awarenessGraphIntegrityCheckCmd)
	awarenessCmd.AddCommand(awarenessImpactPathCmd)
}
