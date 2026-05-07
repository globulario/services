package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/preflight"
	"github.com/globulario/services/golang/awareness/runtime"
)

var preflightCfg = struct {
	task           string
	files          []string
	packagePath    string
	phase          string
	format         string
	includeRuntime bool
	runtimeWindow  time.Duration
	writeAudit     bool
	gitSHA         string
}{}

var awarenessPreflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Run a full architecture preflight before editing Globular code",
	Long: `preflight is the front door for AI agents before editing Globular code.

It composes all awareness capabilities — agent context, impact analysis,
fix-ledger, package admission, cycle detection — into a single deterministic
report with explicit instruction.

Examples:

  globular awareness preflight --task "desired_hash mismatch after deploy" --format agent

  globular awareness preflight \
    --task "envoy restart storm" \
    --file golang/cluster_controller/convergence.go \
    --phase recovery \
    --format markdown

  globular awareness preflight \
    --task "add new package" \
    --package /path/to/package \
    --format json`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if preflightCfg.task == "" {
			return fmt.Errorf("--task is required")
		}

		ctx := context.Background()

		// Resolve repo root and docs dir.
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}
		docsDir := filepath.Join(repoRoot, "docs", "awareness")

		// Open graph — non-fatal if missing (preflight degrades gracefully).
		dbPath := awareCfg.dbPath
		if dbPath == "" {
			dbPath = filepath.Join(repoRoot, ".globular", "awareness", "graph.db")
		}
		g, graphErr := openAwarenessGraph(dbPath, awareCfg.repoPath)
		if graphErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not open awareness graph (%v)\n", graphErr)
			fmt.Fprintf(os.Stderr, "  run 'globular awareness build' to build the graph\n")
			// g remains nil — preflight handles nil graph gracefully
		} else {
			defer g.Close()
		}

		opts := preflight.Options{
			Task:           preflightCfg.task,
			Files:          preflightCfg.files,
			PackagePath:    preflightCfg.packagePath,
			Phase:          preflightCfg.phase,
			DocsDir:        docsDir,
			IncludeRuntime: preflightCfg.includeRuntime,
			RuntimeWindow:  preflightCfg.runtimeWindow,
			WriteAudit:     preflightCfg.writeAudit,
			GitSHA:         preflightCfg.gitSHA,
		}

		if preflightCfg.includeRuntime {
			opts.Bridge = runtime.NewBridge("", "")
		}

		r, err := preflight.Run(ctx, opts, g)
		if err != nil {
			return fmt.Errorf("preflight: %w", err)
		}

		format := preflight.Format(preflightCfg.format)
		if format == "" {
			format = preflight.FormatMarkdown
		}

		out, err := preflight.Render(r, format)
		if err != nil {
			return fmt.Errorf("render preflight: %w", err)
		}

		fmt.Fprint(os.Stdout, out)
		return nil
	},
}

func init() {
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.task, "task", "", "Task description (required)")
	awarenessPreflightCmd.Flags().StringArrayVar(&preflightCfg.files, "file", nil, "File(s) to run impact analysis on (repeatable)")
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.packagePath, "package", "", "Path to package directory with awareness.yaml")
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.phase, "phase", "", "Dependency phase for cycle detection (e.g. recovery, bootstrap, package_install)")
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.format, "format", "markdown", "Output format: markdown | json | agent")
	awarenessPreflightCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db (default: .globular/awareness/graph.db)")
	awarenessPreflightCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root (default: auto-detected from git)")
	awarenessPreflightCmd.Flags().BoolVar(&preflightCfg.includeRuntime, "include-runtime", false, "Collect live runtime snapshot and merge into preflight report")
	awarenessPreflightCmd.Flags().DurationVar(&preflightCfg.runtimeWindow, "runtime-window", 15*time.Minute, "Lookback window for runtime events/workflows")
	awarenessPreflightCmd.Flags().BoolVar(&preflightCfg.writeAudit, "write-audit", false, "Persist a preflight audit record to the graph DB after the run")
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.gitSHA, "git-sha", "", "Current git SHA for the audit record (used with --write-audit)")

	awarenessCmd.AddCommand(awarenessPreflightCmd)
}
