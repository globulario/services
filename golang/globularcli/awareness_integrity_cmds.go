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
	Short: "Validate the awareness knowledge graph (not available — integrity.Check removed)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("graph-integrity-check is not available: integrity.Check/Options/IntegrityResult were removed from standalone awareness module")
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
