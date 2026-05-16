package main

// awareness_context_cmds.go: Stale Context Detection CLI commands.
//
// Commands:
//
//	globular awareness context-read  --session <id> --file <path> [--reason <text>] [--tool <name>] [--turn <n>]
//	globular awareness context-check --session <id> --file <path> [--turn <n>]
//	globular awareness context-check --session <id> --all        [--turn <n>]
//	globular awareness context-stale --session <id>              [--turn <n>]

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/globulario/awareness/contextfreshness"
	"github.com/spf13/cobra"
)

var contextFreshCfg = struct {
	sessionID string
	filePath  string
	reason    string
	tool      string
	turnIndex int
	all       bool
	output    string
}{output: "table"}

// ---- context-read command ----

var awarenessContextReadCmd = &cobra.Command{
	Use:   "context-read",
	Short: "Record that you read a file — awareness tracks its fingerprint from this point",
	Long: `Record that the agent consumed a source file at its current fingerprint.

Awareness stores the sha256 of the file at the moment of the read.
Later, context-check can compare the current fingerprint and warn if it changed.

Example:
  globular awareness context-read \
    --session claude-run-123 \
    --file golang/cluster_controller/server.go \
    --reason "debug install retry loop"`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if contextFreshCfg.sessionID == "" {
			return fmt.Errorf("--session is required")
		}
		if contextFreshCfg.filePath == "" {
			return fmt.Errorf("--file is required")
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		tr := contextfreshness.New(g)
		cr, err := tr.RecordContextRead(
			context.Background(),
			contextFreshCfg.sessionID,
			contextFreshCfg.filePath,
			contextFreshCfg.reason,
			contextFreshCfg.tool,
			contextFreshCfg.turnIndex,
		)
		if err != nil {
			return fmt.Errorf("record context read: %w", err)
		}

		if contextFreshCfg.output == "json" {
			out, _ := json.MarshalIndent(map[string]interface{}{
				"path":        cr.Path,
				"fingerprint": cr.Fingerprint,
				"turn_index":  cr.TurnIndex,
				"status":      "recorded",
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		}

		fmt.Printf("recorded  %s\n", cr.Path)
		fmt.Printf("  fingerprint: %s\n", cr.Fingerprint)
		fmt.Printf("  turn:        %d\n", cr.TurnIndex)
		return nil
	},
}

// ---- context-check command ----

var awarenessContextCheckCmd = &cobra.Command{
	Use:   "context-check",
	Short: "Check whether a file (or all session reads) has changed since it was read",
	Long: `Compare the current file fingerprint against what the session recorded at read time.

Check a single file:
  globular awareness context-check --session claude-run-123 --file golang/cluster_controller/server.go

Check all files read in the session:
  globular awareness context-check --session claude-run-123 --all`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if contextFreshCfg.sessionID == "" {
			return fmt.Errorf("--session is required")
		}
		if !contextFreshCfg.all && contextFreshCfg.filePath == "" {
			return fmt.Errorf("--file or --all is required")
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := context.Background()
		tr := contextfreshness.New(g)

		var warnings []contextfreshness.StaleContextWarning
		if contextFreshCfg.all {
			warnings, err = tr.CheckAllSessionReads(ctx, contextFreshCfg.sessionID, contextFreshCfg.turnIndex)
		} else {
			warnings, err = tr.CheckStaleContext(
				ctx,
				contextFreshCfg.sessionID,
				[]string{contextFreshCfg.filePath},
				contextFreshCfg.turnIndex,
				contextfreshness.SeverityCritical,
			)
		}
		if err != nil {
			return fmt.Errorf("check stale: %w", err)
		}

		if contextFreshCfg.output == "json" {
			out, _ := json.MarshalIndent(map[string]interface{}{
				"stale":    len(warnings) > 0,
				"warnings": warnings,
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		}

		if len(warnings) == 0 {
			if contextFreshCfg.all {
				fmt.Println("All session reads are fresh.")
			} else {
				fmt.Printf("FRESH  %s\n", contextFreshCfg.filePath)
			}
			return nil
		}

		for _, w := range warnings {
			printStaleWarning(w)
		}
		return nil
	},
}

// ---- context-stale command ----

var awarenessContextStaleCmd = &cobra.Command{
	Use:   "context-stale",
	Short: "List all stale context warnings for a session (shorthand for context-check --all)",
	Long: `List all files the session read that have since changed.

Example:
  globular awareness context-stale --session claude-run-123`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if contextFreshCfg.sessionID == "" {
			return fmt.Errorf("--session is required")
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		tr := contextfreshness.New(g)
		warnings, err := tr.CheckAllSessionReads(
			context.Background(),
			contextFreshCfg.sessionID,
			contextFreshCfg.turnIndex,
		)
		if err != nil {
			return fmt.Errorf("check all: %w", err)
		}

		if contextFreshCfg.output == "json" {
			staleFiles := make([]string, 0, len(warnings))
			for _, w := range warnings {
				staleFiles = append(staleFiles, w.Path)
			}
			out, _ := json.MarshalIndent(map[string]interface{}{
				"stale":       len(warnings) > 0,
				"stale_files": staleFiles,
				"warnings":    warnings,
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		}

		if len(warnings) == 0 {
			fmt.Println("No stale context detected for this session.")
			return nil
		}

		fmt.Printf("STALE CONTEXT: %d file(s) changed since they were read\n\n", len(warnings))
		for _, w := range warnings {
			printStaleWarning(w)
		}
		return nil
	},
}

func printStaleWarning(w contextfreshness.StaleContextWarning) {
	sev := strings.ToUpper(w.Severity)
	fmt.Printf("[%s] STALE CONTEXT DETECTED\n\n", sev)
	fmt.Printf("  File:    %s\n", w.Path)
	fmt.Printf("  Read at: turn %d / %s\n", w.ReadTurnIndex, w.ReadFingerprint)
	fmt.Printf("  Current: turn %d / %s\n", w.CurrentTurnIndex, w.CurrentFingerprint)
	fmt.Printf("  Action:  Re-read this file before editing or reasoning from it.\n\n")
}

func init() {
	// context-read flags.
	awarenessContextReadCmd.Flags().StringVar(&contextFreshCfg.sessionID, "session", "", "Session or run ID")
	awarenessContextReadCmd.Flags().StringVar(&contextFreshCfg.filePath, "file", "", "Path to the file you just read")
	awarenessContextReadCmd.Flags().StringVar(&contextFreshCfg.reason, "reason", "", "Why you read the file")
	awarenessContextReadCmd.Flags().StringVar(&contextFreshCfg.tool, "tool", "Read", "Tool used to read the file")
	awarenessContextReadCmd.Flags().IntVar(&contextFreshCfg.turnIndex, "turn", 0, "Approximate turn number")
	awarenessContextReadCmd.Flags().StringVar(&contextFreshCfg.output, "output", "table", "Output format: table|json")
	awarenessContextReadCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessContextReadCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// context-check flags.
	awarenessContextCheckCmd.Flags().StringVar(&contextFreshCfg.sessionID, "session", "", "Session or run ID")
	awarenessContextCheckCmd.Flags().StringVar(&contextFreshCfg.filePath, "file", "", "Specific file to check")
	awarenessContextCheckCmd.Flags().BoolVar(&contextFreshCfg.all, "all", false, "Check all files read in the session")
	awarenessContextCheckCmd.Flags().IntVar(&contextFreshCfg.turnIndex, "turn", 0, "Current turn number")
	awarenessContextCheckCmd.Flags().StringVar(&contextFreshCfg.output, "output", "table", "Output format: table|json")
	awarenessContextCheckCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessContextCheckCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// context-stale flags.
	awarenessContextStaleCmd.Flags().StringVar(&contextFreshCfg.sessionID, "session", "", "Session or run ID")
	awarenessContextStaleCmd.Flags().IntVar(&contextFreshCfg.turnIndex, "turn", 0, "Current turn number")
	awarenessContextStaleCmd.Flags().StringVar(&contextFreshCfg.output, "output", "table", "Output format: table|json")
	awarenessContextStaleCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessContextStaleCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	awarenessCmd.AddCommand(awarenessContextReadCmd)
	awarenessCmd.AddCommand(awarenessContextCheckCmd)
	awarenessCmd.AddCommand(awarenessContextStaleCmd)
}
