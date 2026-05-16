package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/globulario/awareness/checkedit"
)

var checkEditCfg = struct {
	file   string
	format string
}{}

var awarenessCheckEditCmd = &cobra.Command{
	Use:   "check-edit",
	Short: "Post-edit awareness check: surfaces forbidden fixes and code smells for an edited file",
	Long: `check-edit runs a post-edit awareness check after you modify a file.

It looks up the file in the awareness graph and surfaces:
  - Forbidden fixes that apply to this file (via invariant links)
  - Code smells from architectural anti-patterns linked to those invariants

Examples:

  globular awareness check-edit --file golang/cluster_controller/convergence.go

  globular awareness check-edit \
    --file golang/cluster_controller/convergence.go \
    --format agent

  # Use as a PostToolUse hook in Claude Code .claude/settings.json:
  # "command": "globular awareness check-edit --file $FILE --format agent"`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if checkEditCfg.file == "" {
			fmt.Fprintln(os.Stderr, "check-edit: no --file provided (hook fired without file path) — skipping")
			return nil
		}

		ctx := context.Background()

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		// Normalize path to repo-relative.
		file := checkEditCfg.file
		if filepath.IsAbs(file) {
			rel, err := filepath.Rel(repoRoot, file)
			if err == nil {
				file = rel
			}
		}

		dbPath := awareCfg.dbPath
		if dbPath == "" {
			dbPath = resolveAwarenessDBPath(repoRoot)
		}

		g, graphErr := openAwarenessGraph(dbPath, awareCfg.repoPath)
		if graphErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not open awareness graph (%v)\n", graphErr)
			fmt.Fprintf(os.Stderr, "  run 'globular awareness build' to build the graph\n")
		} else {
			defer g.Close()
		}

		r, err := checkedit.Run(ctx, g, checkedit.Options{File: file})
		if err != nil {
			return fmt.Errorf("check-edit: %w", err)
		}

		fmt.Fprint(os.Stdout, checkedit.RenderCheckEdit(r, checkEditCfg.format))

		if r.HasIssues {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	awarenessCheckEditCmd.Flags().StringVar(&checkEditCfg.file, "file", "", "Repo-relative path of the edited file (required)")
	awarenessCheckEditCmd.Flags().StringVar(&checkEditCfg.format, "format", "agent", "Output format: agent | markdown | json")
	awarenessCheckEditCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db (default: .globular/awareness/graph.db)")
	awarenessCheckEditCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root (default: auto-detected from git)")

	awarenessCmd.AddCommand(awarenessCheckEditCmd)
}
