package main

// awareness_incident_pattern_cmds.go: Incident Pattern Matching CLI commands.
//
// Commands:
//
//	globular awareness incident-pattern record --incident INC-2026-0001 --file pattern.json
//	globular awareness incident-pattern match  --task "..." [--file path ...]
//	globular awareness incident-pattern list
//	globular awareness incident-pattern show   INC-2026-0001
//	globular awareness incident-pattern ack    --session <id> --incident <id> --reason "..."

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/globulario/services/golang/awareness/incidentpattern"
	"github.com/spf13/cobra"
)

var incidentPatternCfg = struct {
	incidentID string
	patternFile string
	task        string
	intent      string
	files       []string
	symbols     []string
	components  []string
	invariants  []string
	shapes      []string
	sessionID   string
	reason      string
	output      string
}{output: "table"}

// ---- parent command: incident-pattern ----

var awarenessIncidentPatternCmd = &cobra.Command{
	Use:   "incident-pattern",
	Short: "Manage and query incident patterns (scar-tissue oracle)",
	Long: `Incident patterns capture reusable failure signatures from real incidents.

Before editing code, awareness can warn you when the current task resembles a past
incident, failed proposal, reverted fix, or known architectural trap.

Example workflow:
  # Record a pattern after closing an incident
  globular awareness incident-pattern record --incident INC-2026-0001 --file pattern.json

  # Check before editing
  globular awareness incident-pattern match \
    --task "Fix install retry loop" \
    --file golang/cluster_controller/reconcile.go

  # After reading the incident, acknowledge and continue
  globular awareness incident-pattern ack \
    --session claude-run-123 \
    --incident INC-2026-0001 \
    --reason "Changed approach to use atomic etcd transaction"`,
}

// ---- record command ----

var incidentPatternRecordCmd = &cobra.Command{
	Use:   "record",
	Short: "Record a reusable incident pattern from a JSON definition file",
	Long: `Reads a pattern definition JSON and stores it in the awareness graph.

Example JSON structure (pattern.json):
  {
    "incident_id": "INC-2026-0001",
    "title": "etcd cascade after partial install result promotion",
    "severity": "critical",
    "summary": "...",
    "failure_mode": "partial_authoritative_state_commit",
    "root_cause": "Install result promotion was split across multiple etcd writes.",
    "lesson": "Installed-state, result promotion, and action cleanup must commit atomically.",
    "files": [{"path": "golang/cluster_controller/reconcile.go", "role": "dispatch authority"}],
    "edit_shapes": [{"shape_kind": "split_authoritative_state_transition", "description": "...", "dangerous": true}],
    "failed_fixes": [{"description": "...", "reverted": true, "revert_reason": "..."}]
  }`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if incidentPatternCfg.patternFile == "" && incidentPatternCfg.incidentID == "" {
			return fmt.Errorf("--file or --incident is required")
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		var p incidentpattern.IncidentPattern
		if incidentPatternCfg.patternFile != "" {
			data, err := os.ReadFile(incidentPatternCfg.patternFile)
			if err != nil {
				return fmt.Errorf("read pattern file: %w", err)
			}
			if err := json.Unmarshal(data, &p); err != nil {
				return fmt.Errorf("parse pattern JSON: %w", err)
			}
		} else {
			p.IncidentID = incidentPatternCfg.incidentID
		}

		store := incidentpattern.NewStore(g)
		stored, err := store.RecordPattern(context.Background(), p)
		if err != nil {
			return fmt.Errorf("record pattern: %w", err)
		}

		if incidentPatternCfg.output == "json" {
			out, _ := json.MarshalIndent(map[string]interface{}{
				"status": "recorded", "pattern_id": stored.ID, "incident_id": stored.IncidentID,
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		}
		fmt.Printf("recorded  pattern_id=%s  incident=%s\n", stored.ID, stored.IncidentID)
		return nil
	},
}

// ---- match command ----

var incidentPatternMatchCmd = &cobra.Command{
	Use:   "match",
	Short: "Check whether the current task resembles a known past incident",
	Long: `Scores the current task against all stored incident patterns.

Example:
  globular awareness incident-pattern match \
    --task "Fix install retry loop after leader failover" \
    --file golang/cluster_controller/reconcile.go \
    --file golang/node_agent/apply.go \
    --shape split_authoritative_state_transition`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if incidentPatternCfg.task == "" {
			return fmt.Errorf("--task is required")
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		req := incidentpattern.IncidentMatchRequest{
			Task:          incidentPatternCfg.task,
			Intent:        incidentPatternCfg.intent,
			Files:         incidentPatternCfg.files,
			Symbols:       incidentPatternCfg.symbols,
			Components:    incidentPatternCfg.components,
			Invariants:    incidentPatternCfg.invariants,
			ProposedShape: incidentPatternCfg.shapes,
		}

		matches, err := incidentpattern.Match(context.Background(), g, req)
		if err != nil {
			return fmt.Errorf("match: %w", err)
		}

		if incidentPatternCfg.output == "json" {
			out, _ := json.MarshalIndent(map[string]interface{}{
				"has_warning": len(matches) > 0,
				"matches":     matches,
			}, "", "  ")
			fmt.Println(string(out))
			return nil
		}

		if len(matches) == 0 {
			fmt.Println("No incident pattern matches found.")
			return nil
		}

		for _, m := range matches {
			printPatternMatch(m)
		}
		return nil
	},
}

// ---- list command ----

var incidentPatternListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all active incident patterns",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		store := incidentpattern.NewStore(g)
		patterns, err := store.ListPatterns(context.Background())
		if err != nil {
			return fmt.Errorf("list: %w", err)
		}

		if len(patterns) == 0 {
			fmt.Println("No incident patterns recorded.")
			fmt.Println("Run 'globular awareness incident-pattern record' to add one.")
			return nil
		}

		if incidentPatternCfg.output == "json" {
			out, _ := json.MarshalIndent(patterns, "", "  ")
			fmt.Println(string(out))
			return nil
		}

		fmt.Printf("%-12s %-10s %-10s %s\n", "ID", "INCIDENT", "SEVERITY", "TITLE")
		fmt.Println(strings.Repeat("-", 80))
		for _, p := range patterns {
			fmt.Printf("%-12s %-10s %-10s %s\n", p.ID, p.IncidentID, p.Severity, p.Title)
		}
		return nil
	},
}

// ---- show command ----

var incidentPatternShowCmd = &cobra.Command{
	Use:   "show <incident-id>",
	Short: "Show full details of an incident pattern",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		incidentID := args[0]

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		store := incidentpattern.NewStore(g)
		p, err := store.LoadPatternByIncident(context.Background(), incidentID)
		if err != nil {
			return fmt.Errorf("show %s: %w", incidentID, err)
		}

		if incidentPatternCfg.output == "json" {
			out, _ := json.MarshalIndent(p, "", "  ")
			fmt.Println(string(out))
			return nil
		}

		printPatternDetail(*p)
		return nil
	},
}

// ---- ack command ----

var incidentPatternAckCmd = &cobra.Command{
	Use:   "ack",
	Short: "Acknowledge that you read an incident and adjusted your plan",
	Long: `Records an acknowledgement for this session. After acknowledging, the matcher
will not re-block for this session + incident pair.

Example:
  globular awareness incident-pattern ack \
    --session claude-run-123 \
    --incident INC-2026-0001 \
    --reason "Changed approach to use single atomic etcd transaction"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if incidentPatternCfg.sessionID == "" {
			return fmt.Errorf("--session is required")
		}
		if incidentPatternCfg.incidentID == "" {
			return fmt.Errorf("--incident is required")
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		ack := incidentpattern.NewAcknowledgementStore(g)
		if err := ack.AcknowledgeIncident(
			context.Background(),
			incidentPatternCfg.sessionID,
			incidentPatternCfg.incidentID,
			incidentPatternCfg.reason,
		); err != nil {
			return fmt.Errorf("acknowledge: %w", err)
		}

		fmt.Printf("acknowledged  incident=%s  session=%s\n",
			incidentPatternCfg.incidentID, incidentPatternCfg.sessionID)
		return nil
	},
}

// ── print helpers ─────────────────────────────────────────────────────────────

func printPatternMatch(m incidentpattern.IncidentPatternMatch) {
	blockLabel := ""
	if m.Block {
		blockLabel = " [BLOCKING]"
	}
	fmt.Printf("INCIDENT PATTERN MATCH%s\n\n", blockLabel)
	fmt.Printf("  Incident:   %s — %s\n", m.IncidentID, m.Title)
	fmt.Printf("  Confidence: %s, score %.2f\n", m.Confidence, m.Score)
	if len(m.MatchedSignals) > 0 {
		fmt.Println("\n  Why it matched:")
		for _, sig := range m.MatchedSignals {
			fmt.Printf("    - %s: %s\n", sig.Kind, sig.Explanation)
		}
	}
	for _, ff := range m.FailedFixes {
		if ff.Reverted {
			fmt.Printf("\n  Past failed fix (REVERTED):\n    %s\n", ff.Description)
			fmt.Printf("    Reverted because: %s\n", ff.RevertReason)
		}
	}
	if m.Lesson != "" {
		fmt.Printf("\n  Lesson:\n    %s\n", m.Lesson)
	}
	if m.Block {
		fmt.Println("\n  Action: STOP — Read the incident before editing.")
	}
	fmt.Println()
}

func printPatternDetail(p incidentpattern.IncidentPattern) {
	fmt.Printf("# Incident Pattern: %s\n\n", p.IncidentID)
	fmt.Printf("  ID:           %s\n", p.ID)
	fmt.Printf("  Title:        %s\n", p.Title)
	fmt.Printf("  Severity:     %s\n", p.Severity)
	fmt.Printf("  Failure mode: %s\n", p.FailureMode)
	fmt.Printf("  Root cause:   %s\n", p.RootCause)
	fmt.Printf("  Lesson:       %s\n", p.Lesson)
	if len(p.Files) > 0 {
		fmt.Println("\n  Files:")
		for _, f := range p.Files {
			fmt.Printf("    %s (%s)\n", f.Path, f.Role)
		}
	}
	if len(p.EditShapes) > 0 {
		fmt.Println("\n  Dangerous edit shapes:")
		for _, es := range p.EditShapes {
			danger := ""
			if es.Dangerous {
				danger = " [DANGEROUS]"
			}
			fmt.Printf("    %s%s — %s\n", es.ShapeKind, danger, es.Description)
		}
	}
	if len(p.FailedFixes) > 0 {
		fmt.Println("\n  Failed fixes:")
		for _, ff := range p.FailedFixes {
			revertLabel := ""
			if ff.Reverted {
				revertLabel = " [REVERTED]"
			}
			fmt.Printf("    %s%s\n", ff.Description, revertLabel)
			if ff.RevertReason != "" {
				fmt.Printf("      Reason: %s\n", ff.RevertReason)
			}
		}
	}
	if len(p.Proposals) > 0 {
		fmt.Println("\n  Related proposals:")
		for _, pp := range p.Proposals {
			fmt.Printf("    %s (%s) — %s\n", pp.ProposalID, pp.Relationship, pp.Reason)
		}
	}
}

func init() {
	// record flags
	incidentPatternRecordCmd.Flags().StringVar(&incidentPatternCfg.incidentID, "incident", "", "Incident ID")
	incidentPatternRecordCmd.Flags().StringVar(&incidentPatternCfg.patternFile, "file", "", "Path to pattern JSON file")
	incidentPatternRecordCmd.Flags().StringVar(&incidentPatternCfg.output, "output", "table", "Output: table|json")
	incidentPatternRecordCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.json")
	incidentPatternRecordCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// match flags
	incidentPatternMatchCmd.Flags().StringVar(&incidentPatternCfg.task, "task", "", "Task description")
	incidentPatternMatchCmd.Flags().StringVar(&incidentPatternCfg.intent, "intent", "edit", "Intent: edit|review|diagnose")
	incidentPatternMatchCmd.Flags().StringArrayVar(&incidentPatternCfg.files, "file", nil, "File to check (repeatable)")
	incidentPatternMatchCmd.Flags().StringArrayVar(&incidentPatternCfg.symbols, "symbol", nil, "Symbol to check (repeatable)")
	incidentPatternMatchCmd.Flags().StringArrayVar(&incidentPatternCfg.components, "component", nil, "Component to check (repeatable)")
	incidentPatternMatchCmd.Flags().StringArrayVar(&incidentPatternCfg.invariants, "invariant", nil, "Invariant ID to check (repeatable)")
	incidentPatternMatchCmd.Flags().StringArrayVar(&incidentPatternCfg.shapes, "shape", nil, "Proposed edit shape (repeatable)")
	incidentPatternMatchCmd.Flags().StringVar(&incidentPatternCfg.output, "output", "table", "Output: table|json")
	incidentPatternMatchCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.json")
	incidentPatternMatchCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// list flags
	incidentPatternListCmd.Flags().StringVar(&incidentPatternCfg.output, "output", "table", "Output: table|json")
	incidentPatternListCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.json")
	incidentPatternListCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// show flags
	incidentPatternShowCmd.Flags().StringVar(&incidentPatternCfg.output, "output", "table", "Output: table|json")
	incidentPatternShowCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.json")
	incidentPatternShowCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// ack flags
	incidentPatternAckCmd.Flags().StringVar(&incidentPatternCfg.sessionID, "session", "", "Session ID")
	incidentPatternAckCmd.Flags().StringVar(&incidentPatternCfg.incidentID, "incident", "", "Incident ID")
	incidentPatternAckCmd.Flags().StringVar(&incidentPatternCfg.reason, "reason", "", "Why you are proceeding")
	incidentPatternAckCmd.Flags().StringVar(&incidentPatternCfg.output, "output", "table", "Output: table|json")
	incidentPatternAckCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.json")
	incidentPatternAckCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	awarenessIncidentPatternCmd.AddCommand(incidentPatternRecordCmd)
	awarenessIncidentPatternCmd.AddCommand(incidentPatternMatchCmd)
	awarenessIncidentPatternCmd.AddCommand(incidentPatternListCmd)
	awarenessIncidentPatternCmd.AddCommand(incidentPatternShowCmd)
	awarenessIncidentPatternCmd.AddCommand(incidentPatternAckCmd)

	awarenessCmd.AddCommand(awarenessIncidentPatternCmd)
}
