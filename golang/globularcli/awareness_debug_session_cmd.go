package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/debugsession"
)

var dbsCfg = struct {
	task           string
	files          []string
	packagePath    string
	phase          string
	format         string
	includeRuntime bool
	runtimeWindow  string
}{}

var awarenessDebugSessionCmd = &cobra.Command{
	Use:   "debug-session",
	Short: "Produce a guided debugging plan for an AI agent",
	Long: `Composes preflight, semantic navigation, runtime evidence, fix-ledger, and node context
into a ranked, explainable debugging plan.

This command is read-only: it never edits code, mutates runtime state, or promotes proposals.
It tells the agent where to start, what root-cause paths are likely, what files to inspect,
what fixes are forbidden, and what tests are required.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if dbsCfg.task == "" {
			return fmt.Errorf("--task is required")
		}

		ctx := context.Background()

		repoRoot, _ := resolveRepoRoot(awareCfg.repoPath)
		docsDir := filepath.Join(repoRoot, "docs", "awareness")

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		window := 15 * time.Minute
		if dbsCfg.runtimeWindow != "" {
			d, err := time.ParseDuration(dbsCfg.runtimeWindow)
			if err != nil {
				return fmt.Errorf("invalid --runtime-window %q: %w", dbsCfg.runtimeWindow, err)
			}
			window = d
		}

		opts := debugsession.Options{
			Task:           dbsCfg.task,
			Files:          dbsCfg.files,
			PackagePath:    dbsCfg.packagePath,
			Phase:          dbsCfg.phase,
			DocsDir:        docsDir,
			IncludeRuntime: dbsCfg.includeRuntime,
			RuntimeWindow:  window,
		}

		report, err := debugsession.Run(ctx, opts, g)
		if err != nil {
			return fmt.Errorf("debug-session: %w", err)
		}

		fmt.Fprint(os.Stdout, debugsession.FormatReport(report, dbsCfg.format))
		return nil
	},
}

func init() {
	awarenessDebugSessionCmd.Flags().StringVar(&dbsCfg.task, "task", "", "Task description (required)")
	awarenessDebugSessionCmd.Flags().StringArrayVar(&dbsCfg.files, "file", nil, "File path to include in impact analysis (repeatable)")
	awarenessDebugSessionCmd.Flags().StringVar(&dbsCfg.packagePath, "package", "", "Path to package dir with awareness.yaml")
	awarenessDebugSessionCmd.Flags().StringVar(&dbsCfg.phase, "phase", "", "Dependency phase for cycle detection")
	awarenessDebugSessionCmd.Flags().BoolVar(&dbsCfg.includeRuntime, "include-runtime", false, "Include live runtime snapshot")
	awarenessDebugSessionCmd.Flags().StringVar(&dbsCfg.runtimeWindow, "runtime-window", "15m", "Lookback window for runtime evidence")
	awarenessDebugSessionCmd.Flags().StringVar(&dbsCfg.format, "format", "agent", "Output format: agent, markdown, json")
	awarenessDebugSessionCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessDebugSessionCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	awarenessCmd.AddCommand(awarenessDebugSessionCmd)
}
