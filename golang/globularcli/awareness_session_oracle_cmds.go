package main

// awareness_session_oracle_cmds.go: globular awareness session <subcommand>
//
// Manages the Session Resumption Oracle — a structured causal memory that lets
// the next agent session resume safely without relying on fragile chat history.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/globulario/awareness/graph"
	"github.com/globulario/awareness/sessionoracle"
	"github.com/spf13/cobra"
)

var sessionOracleCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage session resumption oracle (start, touch, decision, unfinished, test, close, resume)",
}

func init() {
	awarenessCmd.AddCommand(sessionOracleCmd)
	sessionOracleCmd.AddCommand(
		oracleStartCmd,
		oracleTouchCmd,
		oracleDecisionCmd,
		oracleAssumptionCmd,
		oracleUnfinishedCmd,
		oracleTestCmd,
		oracleCloseCmd,
		oracleResumeCmd,
	)
}

func openOracleGraph() (*graph.Graph, *sessionoracle.Oracle, error) {
	dbPath := oracleGraphPath()
	if dbPath == "" {
		return nil, nil, fmt.Errorf("awareness graph not found — run 'globular awareness build' first")
	}
	g, err := graph.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open awareness graph: %w", err)
	}
	return g, sessionoracle.New(g), nil
}

func oracleGraphPath() string {
	const systemPath = "/var/lib/globular/awareness/graph.db"
	if _, err := os.Stat(systemPath); err == nil {
		return systemPath
	}
	return ""
}

// ── session start ─────────────────────────────────────────────────────────────

var oracleStartCfg = struct {
	id        string
	title     string
	objective string
	actor     string
	repoRoot  string
	parentID  string
}{}

var oracleStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new oracle session",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, o, err := openOracleGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		sess, err := o.StartSession(ctx, sessionoracle.StartSessionRequest{
			ID:              oracleStartCfg.id,
			Title:           oracleStartCfg.title,
			Objective:       oracleStartCfg.objective,
			Actor:           oracleStartCfg.actor,
			RepoRoot:        oracleStartCfg.repoRoot,
			ParentSessionID: oracleStartCfg.parentID,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Session started: %s\n", sess.ID)
		if sess.Branch != "" {
			fmt.Printf("Branch: %s  Commit: %s\n", sess.Branch, sess.GitCommitStart)
		}
		return nil
	},
}

func init() {
	oracleStartCmd.Flags().StringVar(&oracleStartCfg.id, "id", "", "Optional explicit session ID")
	oracleStartCmd.Flags().StringVar(&oracleStartCfg.title, "title", "", "Session title")
	oracleStartCmd.Flags().StringVar(&oracleStartCfg.objective, "objective", "", "Session objective")
	oracleStartCmd.Flags().StringVar(&oracleStartCfg.actor, "actor", "claude", "Actor (claude, gpt, human)")
	oracleStartCmd.Flags().StringVar(&oracleStartCfg.repoRoot, "repo-root", "", "Repository root path")
	oracleStartCmd.Flags().StringVar(&oracleStartCfg.parentID, "parent", "", "Parent session ID")
}

// ── session touch ─────────────────────────────────────────────────────────────

var oracleTouchCfg = struct {
	session string
	file    string
	action  string
	reason  string
}{}

var oracleTouchCmd = &cobra.Command{
	Use:   "touch",
	Short: "Record a file touch (read, edit, create, etc.) in the session",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, o, err := openOracleGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		ft, err := o.RecordFileTouch(ctx, oracleTouchCfg.session, oracleTouchCfg.file,
			oracleTouchCfg.action, oracleTouchCfg.reason, 0)
		if err != nil {
			return err
		}
		fmt.Printf("Recorded: %s  seq=%d  %s\n", ft.Path, ft.Sequence, ft.Action)
		return nil
	},
}

func init() {
	oracleTouchCmd.Flags().StringVar(&oracleTouchCfg.session, "session", "", "Session ID")
	oracleTouchCmd.Flags().StringVar(&oracleTouchCfg.file, "file", "", "File path")
	oracleTouchCmd.Flags().StringVar(&oracleTouchCfg.action, "action", "read", "read|edit|create|delete|rename|test|inspect")
	oracleTouchCmd.Flags().StringVar(&oracleTouchCfg.reason, "reason", "", "Reason for the access")
	_ = oracleTouchCmd.MarkFlagRequired("session")
	_ = oracleTouchCmd.MarkFlagRequired("file")
}

// ── session decision ──────────────────────────────────────────────────────────

var oracleDecisionCfg = struct {
	session      string
	title        string
	decision     string
	rationale    string
	alternatives string
	files        string
	invariants   string
	incidents    string
	confidence   string
}{}

var oracleDecisionCmd = &cobra.Command{
	Use:   "decision",
	Short: "Record an architectural decision made during the session",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, o, err := openOracleGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		d, err := o.RecordDecision(ctx, sessionoracle.RecordDecisionRequest{
			SessionID:              oracleDecisionCfg.session,
			Title:                  oracleDecisionCfg.title,
			Decision:               oracleDecisionCfg.decision,
			Rationale:              oracleDecisionCfg.rationale,
			AlternativesConsidered: splitOracleCSV(oracleDecisionCfg.alternatives),
			RelatedFiles:           splitOracleCSV(oracleDecisionCfg.files),
			RelatedInvariants:      splitOracleCSV(oracleDecisionCfg.invariants),
			RelatedIncidents:       splitOracleCSV(oracleDecisionCfg.incidents),
			Confidence:             oracleDecisionCfg.confidence,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Decision recorded: %s\n", d.ID)
		return nil
	},
}

func init() {
	oracleDecisionCmd.Flags().StringVar(&oracleDecisionCfg.session, "session", "", "Session ID")
	oracleDecisionCmd.Flags().StringVar(&oracleDecisionCfg.title, "title", "", "Decision title")
	oracleDecisionCmd.Flags().StringVar(&oracleDecisionCfg.decision, "decision", "", "The decision made")
	oracleDecisionCmd.Flags().StringVar(&oracleDecisionCfg.rationale, "rationale", "", "Why this decision was made")
	oracleDecisionCmd.Flags().StringVar(&oracleDecisionCfg.alternatives, "alternatives", "", "Comma-separated alternatives considered")
	oracleDecisionCmd.Flags().StringVar(&oracleDecisionCfg.files, "files", "", "Comma-separated related files")
	oracleDecisionCmd.Flags().StringVar(&oracleDecisionCfg.invariants, "invariants", "", "Comma-separated related invariant IDs")
	oracleDecisionCmd.Flags().StringVar(&oracleDecisionCfg.incidents, "incidents", "", "Comma-separated related incident IDs")
	oracleDecisionCmd.Flags().StringVar(&oracleDecisionCfg.confidence, "confidence", "medium", "high|medium|low")
	_ = oracleDecisionCmd.MarkFlagRequired("session")
	_ = oracleDecisionCmd.MarkFlagRequired("title")
	_ = oracleDecisionCmd.MarkFlagRequired("decision")
	_ = oracleDecisionCmd.MarkFlagRequired("rationale")
}

// ── session assumption ────────────────────────────────────────────────────────

var oracleAssumptionCfg = struct {
	session        string
	assumption     string
	basis          string
	validationPlan string
	relatedFiles   string
}{}

var oracleAssumptionCmd = &cobra.Command{
	Use:   "assumption",
	Short: "Record an unverified assumption made during the session",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, o, err := openOracleGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		a, err := o.RecordAssumption(ctx, sessionoracle.RecordAssumptionRequest{
			SessionID:      oracleAssumptionCfg.session,
			Assumption:     oracleAssumptionCfg.assumption,
			Basis:          oracleAssumptionCfg.basis,
			ValidationPlan: oracleAssumptionCfg.validationPlan,
			RelatedFiles:   oracleAssumptionCfg.relatedFiles,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Assumption recorded: %s\n", a.ID)
		return nil
	},
}

func init() {
	oracleAssumptionCmd.Flags().StringVar(&oracleAssumptionCfg.session, "session", "", "Session ID")
	oracleAssumptionCmd.Flags().StringVar(&oracleAssumptionCfg.assumption, "assumption", "", "The assumption being made")
	oracleAssumptionCmd.Flags().StringVar(&oracleAssumptionCfg.basis, "basis", "", "Evidence or reasoning")
	oracleAssumptionCmd.Flags().StringVar(&oracleAssumptionCfg.validationPlan, "validation-plan", "", "How to verify this assumption")
	oracleAssumptionCmd.Flags().StringVar(&oracleAssumptionCfg.relatedFiles, "files", "", "Related files")
	_ = oracleAssumptionCmd.MarkFlagRequired("session")
	_ = oracleAssumptionCmd.MarkFlagRequired("assumption")
}

// ── session unfinished ────────────────────────────────────────────────────────

var oracleUnfinishedCfg = struct {
	session     string
	title       string
	description string
	priority    string
	reason      string
	next        string
	files       string
	tests       string
	incidents   string
}{}

var oracleUnfinishedCmd = &cobra.Command{
	Use:   "unfinished",
	Short: "Record unfinished work that must be picked up in the next session",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, o, err := openOracleGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		w, err := o.RecordUnfinishedWork(ctx, sessionoracle.RecordUnfinishedWorkRequest{
			SessionID:        oracleUnfinishedCfg.session,
			Title:            oracleUnfinishedCfg.title,
			Description:      oracleUnfinishedCfg.description,
			Priority:         oracleUnfinishedCfg.priority,
			ReasonUnfinished: oracleUnfinishedCfg.reason,
			NextAction:       oracleUnfinishedCfg.next,
			RelatedFiles:     splitOracleCSV(oracleUnfinishedCfg.files),
			RelatedTests:     splitOracleCSV(oracleUnfinishedCfg.tests),
			RelatedIncidents: splitOracleCSV(oracleUnfinishedCfg.incidents),
		})
		if err != nil {
			return err
		}
		fmt.Printf("Unfinished work recorded: %s\n", w.ID)
		return nil
	},
}

func init() {
	oracleUnfinishedCmd.Flags().StringVar(&oracleUnfinishedCfg.session, "session", "", "Session ID")
	oracleUnfinishedCmd.Flags().StringVar(&oracleUnfinishedCfg.title, "title", "", "Task title")
	oracleUnfinishedCmd.Flags().StringVar(&oracleUnfinishedCfg.description, "description", "", "What needs to be done")
	oracleUnfinishedCmd.Flags().StringVar(&oracleUnfinishedCfg.priority, "priority", "medium", "critical|high|medium|low")
	oracleUnfinishedCmd.Flags().StringVar(&oracleUnfinishedCfg.reason, "reason", "", "Why not completed now")
	oracleUnfinishedCmd.Flags().StringVar(&oracleUnfinishedCfg.next, "next", "", "Specific first step for next session")
	oracleUnfinishedCmd.Flags().StringVar(&oracleUnfinishedCfg.files, "files", "", "Comma-separated related files")
	oracleUnfinishedCmd.Flags().StringVar(&oracleUnfinishedCfg.tests, "tests", "", "Comma-separated related test targets")
	oracleUnfinishedCmd.Flags().StringVar(&oracleUnfinishedCfg.incidents, "incidents", "", "Comma-separated related incident IDs")
	_ = oracleUnfinishedCmd.MarkFlagRequired("session")
	_ = oracleUnfinishedCmd.MarkFlagRequired("title")
}

// ── session test ──────────────────────────────────────────────────────────────

var oracleTestCfg = struct {
	session string
	cmd     string
	status  string
	summary string
	files   string
}{}

var oracleTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Record a test run result during the session",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, o, err := openOracleGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		r, err := o.RecordTestResult(ctx, sessionoracle.RecordTestResultRequest{
			SessionID:    oracleTestCfg.session,
			Command:      oracleTestCfg.cmd,
			Status:       oracleTestCfg.status,
			Summary:      oracleTestCfg.summary,
			RelatedFiles: splitOracleCSV(oracleTestCfg.files),
		})
		if err != nil {
			return err
		}
		fmt.Printf("Test result recorded: %s  status=%s\n", r.ID, r.Status)
		return nil
	},
}

func init() {
	oracleTestCmd.Flags().StringVar(&oracleTestCfg.session, "session", "", "Session ID")
	oracleTestCmd.Flags().StringVar(&oracleTestCfg.cmd, "cmd", "", "Test command that was run")
	oracleTestCmd.Flags().StringVar(&oracleTestCfg.status, "status", "", "passed|failed|skipped|error")
	oracleTestCmd.Flags().StringVar(&oracleTestCfg.summary, "summary", "", "Brief summary of results")
	oracleTestCmd.Flags().StringVar(&oracleTestCfg.files, "files", "", "Comma-separated related files")
	_ = oracleTestCmd.MarkFlagRequired("session")
	_ = oracleTestCmd.MarkFlagRequired("cmd")
	_ = oracleTestCmd.MarkFlagRequired("status")
}

// ── session close ─────────────────────────────────────────────────────────────

var oracleCloseCfg = struct {
	session     string
	pushAIMem   bool
}{}

var oracleCloseCmd = &cobra.Command{
	Use:   "close",
	Short: "Close a session, build resume snapshot, optionally push to AI Memory",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, o, err := openOracleGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		var bridge sessionoracle.AIMemoryBridge
		if oracleCloseCfg.pushAIMem {
			bridge = sessionoracle.NoopBridge()
		}
		snap, err := o.CloseSession(ctx, oracleCloseCfg.session, oracleCloseCfg.pushAIMem, bridge)
		if err != nil {
			return err
		}

		fmt.Printf("Session closed: %s\n", snap.SessionID)
		fmt.Printf("Snapshot ID:    %s\n", snap.ID)
		fmt.Printf("Summary:        %s\n", snap.Summary)
		fmt.Printf("Next action:    %s\n", snap.RecommendedNextAction)

		open := 0
		for _, w := range snap.Unfinished {
			if w.Status == "open" || w.Status == "in_progress" {
				open++
			}
		}
		if open > 0 {
			fmt.Printf("\n%d open unfinished item(s):\n", open)
			for _, w := range snap.Unfinished {
				if w.Status == "open" || w.Status == "in_progress" {
					fmt.Printf("  [%s] %s — %s\n", w.Priority, w.Title, w.NextAction)
				}
			}
		}
		return nil
	},
}

func init() {
	oracleCloseCmd.Flags().StringVar(&oracleCloseCfg.session, "session", "", "Session ID to close")
	oracleCloseCmd.Flags().BoolVar(&oracleCloseCfg.pushAIMem, "push-ai-memory", false, "Push compact summary to AI Memory service")
	_ = oracleCloseCmd.MarkFlagRequired("session")
}

// ── session resume ────────────────────────────────────────────────────────────

var oracleResumeCfg = struct {
	session  string
	latest   bool
	repoRoot string
	jsonOut  bool
}{}

var oracleResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume a session — shows oracle snapshot with decisions, unfinished work, stale files, and next action",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, o, err := openOracleGraph()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		var snap *sessionoracle.SessionResumeSnapshot

		if oracleResumeCfg.latest {
			repoRoot := oracleResumeCfg.repoRoot
			if repoRoot == "" {
				repoRoot, _ = os.Getwd()
			}
			snap, err = o.ResumeLatestOpenSession(ctx, repoRoot)
		} else if oracleResumeCfg.session != "" {
			snap, err = o.ResumeSession(ctx, oracleResumeCfg.session)
		} else {
			return fmt.Errorf("provide --session <id> or --latest")
		}
		if err != nil {
			return err
		}

		if oracleResumeCfg.jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(snap)
		}

		printOracleSnapshot(snap)
		return nil
	},
}

func init() {
	oracleResumeCmd.Flags().StringVar(&oracleResumeCfg.session, "session", "", "Session ID to resume")
	oracleResumeCmd.Flags().BoolVar(&oracleResumeCfg.latest, "latest", false, "Resume the most recent open session")
	oracleResumeCmd.Flags().StringVar(&oracleResumeCfg.repoRoot, "repo-root", "", "Repo root (used with --latest)")
	oracleResumeCmd.Flags().BoolVar(&oracleResumeCfg.jsonOut, "json", false, "Output as JSON")
}

// ── display ───────────────────────────────────────────────────────────────────

func printOracleSnapshot(snap *sessionoracle.SessionResumeSnapshot) {
	fmt.Printf("\nSESSION RESUMPTION ORACLE\n")
	fmt.Printf("Session:   %s\n", snap.SessionID)
	fmt.Printf("Objective: %s\n", snap.Objective)
	fmt.Printf("Summary:   %s\n\n", snap.Summary)

	if len(snap.FilesTouched) > 0 {
		fmt.Printf("Files touched (%d):\n", len(snap.FilesTouched))
		for i, ft := range snap.FilesTouched {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(snap.FilesTouched)-10)
				break
			}
			fmt.Printf("  %d. [%s] %s\n", ft.Sequence, ft.Action, ft.Path)
		}
		fmt.Println()
	}

	if len(snap.Decisions) > 0 {
		fmt.Printf("Decisions made (%d):\n", len(snap.Decisions))
		for _, d := range snap.Decisions {
			fmt.Printf("  - %s [%s]\n", d.Title, d.Confidence)
			fmt.Printf("    %s\n", d.Decision)
		}
		fmt.Println()
	}

	if len(snap.Assumptions) > 0 {
		unverified := 0
		for _, a := range snap.Assumptions {
			if a.Status == "unverified" {
				unverified++
			}
		}
		if unverified > 0 {
			fmt.Printf("Unverified assumptions (%d):\n", unverified)
			for _, a := range snap.Assumptions {
				if a.Status == "unverified" {
					fmt.Printf("  - %s\n", a.Assumption)
				}
			}
			fmt.Println()
		}
	}

	open := 0
	for _, w := range snap.Unfinished {
		if w.Status == "open" || w.Status == "in_progress" {
			open++
		}
	}
	if open > 0 {
		fmt.Printf("Unfinished (%d):\n", open)
		for _, w := range snap.Unfinished {
			if w.Status == "open" || w.Status == "in_progress" {
				fmt.Printf("  [%s] %s\n", w.Priority, w.Title)
				if w.NextAction != "" {
					fmt.Printf("    Next: %s\n", w.NextAction)
				}
			}
		}
		fmt.Println()
	}

	activeWarnings := 0
	for _, w := range snap.Warnings {
		if !w.Acknowledged {
			activeWarnings++
		}
	}
	if activeWarnings > 0 {
		fmt.Printf("Active warnings (%d):\n", activeWarnings)
		for _, w := range snap.Warnings {
			if !w.Acknowledged {
				fmt.Printf("  [%s/%s] %s\n", w.WarningType, w.Severity, w.Message)
			}
		}
		fmt.Println()
	}

	if len(snap.Tests) > 0 {
		fmt.Printf("Test results (%d):\n", len(snap.Tests))
		for _, t := range snap.Tests {
			fmt.Printf("  [%s] %s\n", t.Status, t.Command)
		}
		fmt.Println()
	}

	fmt.Printf("Resume recommendation:\n  %s\n", snap.RecommendedNextAction)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func splitOracleCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
