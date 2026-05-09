package main

// awareness_coordination_cmds.go: CLI commands for Agent Coordination Memory.
//
// Usage:
//
//	globular awareness coordination start   --title "..." --objective "..."
//	globular awareness coordination join    --run <id> --agent <name>
//	globular awareness coordination snapshot --run <id> [--agent <id>] [--file path] [--json]
//	globular awareness coordination claim-file --run <id> --agent <id> --file <path> --kind <kind>
//	globular awareness coordination lock-file  --run <id> --agent <id> --file <path> --kind <kind>
//	globular awareness coordination release-lock --run <id> --agent <id> --lock <id>
//	globular awareness coordination decision --run <id> --agent <id> --title "..." --decision "..." --rationale "..." --scope <scope>
//	globular awareness coordination conflicts --run <id> [--json]
//	globular awareness coordination close --run <id> [--json]

import (
	"context"
	"fmt"
	"os"

	"github.com/globulario/services/golang/awareness/coordination"
	"github.com/spf13/cobra"
)

// coordinationCfg holds flag values for coordination subcommands.
var coordinationCfg = struct {
	runID       string
	id          string
	title       string
	objective   string
	repo        string
	branch      string
	agentID     string
	agentName   string
	agentKind   string
	sessionID   string
	role        string
	filePath    string
	files       []string
	claimKind   string
	lockKind    string
	lockID      string
	reason      string
	ttl         int64
	decision    string
	rationale   string
	scope       string
	components  []string
	invariants  []string
	incidents   []string
	binding     bool
	decisionID  string
	evidence    string
	toAgent     string
	workItemID  string
	body        string
	handoffID   string
	json        bool
}{
	agentKind: "claude",
}

// coordinationCmd is the parent command for all coordination subcommands.
var coordinationCmd = &cobra.Command{
	Use:   "coordination",
	Short: "Agent Coordination Memory — multi-agent collaboration with file locks and shared state",
	Long: `Agent Coordination Memory lets multiple Claude (or other AI) agents share state
during parallel work on the same codebase. Each agent records its file claims,
locks, decisions, assumptions, warnings, and handoff notes into the shared graph.

Before editing a file, check for active locks. Before starting, join the run.

Example workflow:

  # Agent 1 starts a run
  globular awareness coordination start --title "Fix session bug" --objective "Fix INC-2026-0007"

  # Each agent joins
  globular awareness coordination join --run RUN-abc123 --agent claude-1 --kind claude

  # Before editing, get the snapshot
  globular awareness coordination snapshot --run RUN-abc123 --agent AGENT-xyz

  # Lock a file before editing
  globular awareness coordination lock-file --run RUN-abc123 --agent AGENT-xyz \
    --file golang/awareness/session.go --kind edit --reason "fixing nil panic"

  # Record a key decision
  globular awareness coordination decision --run RUN-abc123 --agent AGENT-xyz \
    --title "No env vars" --decision "use etcd only" --rationale "arch rule" --scope global

  # Release the lock after editing
  globular awareness coordination release-lock --run RUN-abc123 --agent AGENT-xyz \
    --lock LOCK-abc123

  # Close when done
  globular awareness coordination close --run RUN-abc123`,
}

// ── start ────────────────────────────────────────────────────────────────────

var coordinationStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a new multi-agent coordination run",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		store := coordination.New(g)
		run, err := store.StartCoordinationRun(context.Background(), coordination.StartCoordinationRunRequest{
			ID:           coordinationCfg.id,
			Title:        coordinationCfg.title,
			Objective:    coordinationCfg.objective,
			OwnerAgentID: coordinationCfg.agentID,
			RepoRoot:     coordinationCfg.repo,
			Branch:       coordinationCfg.branch,
		})
		if err != nil {
			return err
		}
		if coordinationCfg.json {
			return printJSON(run)
		}
		fmt.Fprintf(os.Stdout, "Run started: %s\n", run.ID)
		fmt.Fprintf(os.Stdout, "  title:     %s\n", run.Title)
		fmt.Fprintf(os.Stdout, "  objective: %s\n", run.Objective)
		if run.Branch != "" {
			fmt.Fprintf(os.Stdout, "  branch:    %s\n", run.Branch)
		}
		return nil
	},
}

// ── join ─────────────────────────────────────────────────────────────────────

var coordinationJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join a coordination run as an agent participant",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		store := coordination.New(g)
		a, err := store.JoinCoordinationRun(context.Background(), coordination.JoinCoordinationRunRequest{
			RunID:     coordinationCfg.runID,
			AgentName: coordinationCfg.agentName,
			AgentKind: coordinationCfg.agentKind,
			SessionID: coordinationCfg.sessionID,
			Role:      coordinationCfg.role,
		})
		if err != nil {
			return err
		}
		if coordinationCfg.json {
			return printJSON(a)
		}
		fmt.Fprintf(os.Stdout, "Joined run %s as agent %s\n", a.RunID, a.ID)
		fmt.Fprintf(os.Stdout, "  name: %s  kind: %s  role: %s\n", a.AgentName, a.AgentKind, a.Role)
		return nil
	},
}

// ── snapshot ─────────────────────────────────────────────────────────────────

var coordinationSnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Get a full coordination run snapshot",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		store := coordination.New(g)
		snap, err := store.GetCoordinationSnapshot(context.Background(),
			coordinationCfg.runID,
			coordinationCfg.agentID,
			coordinationCfg.files,
		)
		if err != nil {
			return err
		}
		if coordinationCfg.json {
			return printJSON(snap)
		}
		printCoordinationSnapshot(snap)
		return nil
	},
}

// printCoordinationSnapshot prints a human-readable summary of the snapshot.
func printCoordinationSnapshot(snap *coordination.CoordinationSnapshot) {
	fmt.Fprintf(os.Stdout, "Coordination Run: %s (%s)\n", snap.Run.ID, snap.Run.Status)
	fmt.Fprintf(os.Stdout, "  title:     %s\n", snap.Run.Title)
	fmt.Fprintf(os.Stdout, "  objective: %s\n\n", snap.Run.Objective)

	fmt.Fprintf(os.Stdout, "Agents (%d):\n", len(snap.Agents))
	for _, a := range snap.Agents {
		fmt.Fprintf(os.Stdout, "  [%s] %s (%s) status=%s\n", a.ID, a.AgentName, a.AgentKind, a.Status)
	}

	fmt.Fprintf(os.Stdout, "\nWork Items (%d):\n", len(snap.WorkItems))
	for _, wi := range snap.WorkItems {
		fmt.Fprintf(os.Stdout, "  [%s] %s — %s (priority=%s)\n", wi.ID, wi.Title, wi.Status, wi.Priority)
	}

	fmt.Fprintf(os.Stdout, "\nActive File Claims (%d):\n", len(snap.ActiveClaims))
	for _, c := range snap.ActiveClaims {
		fmt.Fprintf(os.Stdout, "  [%s] %s %s (agent=%s)\n", c.ID, c.ClaimKind, c.Path, c.AgentID)
	}

	fmt.Fprintf(os.Stdout, "\nActive File Locks (%d):\n", len(snap.ActiveLocks))
	for _, lk := range snap.ActiveLocks {
		fmt.Fprintf(os.Stdout, "  [%s] %s %s (agent=%s)\n", lk.ID, lk.LockKind, lk.Path, lk.AgentID)
	}

	fmt.Fprintf(os.Stdout, "\nDecisions (%d):\n", len(snap.Decisions))
	for _, d := range snap.Decisions {
		binding := ""
		if d.Binding {
			binding = " [BINDING]"
		}
		fmt.Fprintf(os.Stdout, "  [%s]%s %s\n", d.ID, binding, d.Title)
	}

	fmt.Fprintf(os.Stdout, "\nWarnings (%d):\n", len(snap.Warnings))
	for _, w := range snap.Warnings {
		fmt.Fprintf(os.Stdout, "  [%s] %s: %s\n", w.Severity, w.WarningType, w.Message)
	}

	fmt.Fprintf(os.Stdout, "\nOpen Conflicts (%d):\n", len(snap.OpenConflicts))
	for _, c := range snap.OpenConflicts {
		fmt.Fprintf(os.Stdout, "  [%s] %s (severity=%s): %s\n", c.ID, c.ConflictType, c.Severity, c.Message)
	}

	fmt.Fprintf(os.Stdout, "\nUnread Handoffs (%d):\n", len(snap.HandoffNotes))
	for _, h := range snap.HandoffNotes {
		fmt.Fprintf(os.Stdout, "  [%s] from=%s: %s\n", h.ID, h.FromAgentID, h.Title)
	}

	if len(snap.RecommendedRules) > 0 {
		fmt.Fprintf(os.Stdout, "\nRecommended Rules:\n")
		for _, r := range snap.RecommendedRules {
			fmt.Fprintf(os.Stdout, "  ! %s\n", r)
		}
	}
}

// ── claim-file ────────────────────────────────────────────────────────────────

var coordinationClaimFileCmd = &cobra.Command{
	Use:   "claim-file",
	Short: "Declare intent to read or edit a file",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		store := coordination.New(g)
		c, err := store.ClaimFile(context.Background(), coordination.ClaimFileRequest{
			RunID:     coordinationCfg.runID,
			AgentID:   coordinationCfg.agentID,
			Path:      coordinationCfg.filePath,
			ClaimKind: coordinationCfg.claimKind,
			Reason:    coordinationCfg.reason,
			TTL:       coordinationCfg.ttl,
		})
		if err != nil {
			return err
		}
		if coordinationCfg.json {
			return printJSON(c)
		}
		fmt.Fprintf(os.Stdout, "Claimed %s: %s (kind=%s)\n", c.ID, c.Path, c.ClaimKind)
		return nil
	},
}

// ── lock-file ────────────────────────────────────────────────────────────────

var coordinationLockFileCmd = &cobra.Command{
	Use:   "lock-file",
	Short: "Acquire an exclusive file lock before making edits",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		store := coordination.New(g)
		lk, conflict, err := store.AcquireFileLock(context.Background(), coordination.AcquireFileLockRequest{
			RunID:    coordinationCfg.runID,
			AgentID:  coordinationCfg.agentID,
			Path:     coordinationCfg.filePath,
			LockKind: coordinationCfg.lockKind,
			Reason:   coordinationCfg.reason,
			TTL:      coordinationCfg.ttl,
		})
		if err != nil {
			return err
		}
		if conflict != nil {
			if coordinationCfg.json {
				return printJSON(map[string]interface{}{"status": "blocked", "conflict": conflict})
			}
			fmt.Fprintf(os.Stderr, "BLOCKED: %s\n", conflict.Message)
			fmt.Fprintf(os.Stderr, "  type:  %s\n", conflict.Type)
			fmt.Fprintf(os.Stderr, "  owner: %s\n", conflict.OwnerAgentID)
			return fmt.Errorf("cannot acquire lock — conflict: %s", conflict.Type)
		}
		if coordinationCfg.json {
			return printJSON(map[string]interface{}{"status": "locked", "lock_id": lk.ID, "path": lk.Path})
		}
		fmt.Fprintf(os.Stdout, "Locked: %s — %s (kind=%s)\n", lk.ID, lk.Path, lk.LockKind)
		return nil
	},
}

// ── release-lock ─────────────────────────────────────────────────────────────

var coordinationReleaseLockCmd = &cobra.Command{
	Use:   "release-lock",
	Short: "Release a file lock after edits are complete",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		store := coordination.New(g)
		if err := store.ReleaseFileLock(context.Background(),
			coordinationCfg.runID,
			coordinationCfg.lockID,
			coordinationCfg.agentID,
		); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Lock %s released\n", coordinationCfg.lockID)
		return nil
	},
}

// ── decision ─────────────────────────────────────────────────────────────────

var coordinationDecisionCmd = &cobra.Command{
	Use:   "decision",
	Short: "Record a coordination decision for other agents to respect",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		store := coordination.New(g)
		d, err := store.RecordCoordinationDecision(context.Background(), coordination.RecordDecisionRequest{
			RunID:             coordinationCfg.runID,
			AgentID:           coordinationCfg.agentID,
			Title:             coordinationCfg.title,
			Decision:          coordinationCfg.decision,
			Rationale:         coordinationCfg.rationale,
			Scope:             coordinationCfg.scope,
			RelatedFiles:      coordinationCfg.files,
			RelatedComponents: coordinationCfg.components,
			RelatedInvariants: coordinationCfg.invariants,
			RelatedIncidents:  coordinationCfg.incidents,
			Binding:           coordinationCfg.binding,
		})
		if err != nil {
			return err
		}
		if coordinationCfg.json {
			return printJSON(d)
		}
		fmt.Fprintf(os.Stdout, "Decision recorded: %s\n", d.ID)
		fmt.Fprintf(os.Stdout, "  title:   %s\n", d.Title)
		fmt.Fprintf(os.Stdout, "  binding: %v\n", d.Binding)
		return nil
	},
}

// ── conflicts ────────────────────────────────────────────────────────────────

var coordinationConflictsCmd = &cobra.Command{
	Use:   "conflicts",
	Short: "Detect and list all conflicts in a coordination run",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		store := coordination.New(g)
		conflicts, err := store.DetectCoordinationConflicts(context.Background(), coordinationCfg.runID)
		if err != nil {
			return err
		}
		if coordinationCfg.json {
			return printJSON(conflicts)
		}
		if len(conflicts) == 0 {
			fmt.Fprintln(os.Stdout, "No conflicts detected.")
			return nil
		}
		fmt.Fprintf(os.Stdout, "Conflicts (%d):\n", len(conflicts))
		for _, c := range conflicts {
			fmt.Fprintf(os.Stdout, "  [%s] %s (severity=%s status=%s): %s\n",
				c.ID, c.ConflictType, c.Severity, c.Status, c.Message)
		}
		return nil
	},
}

// ── close ────────────────────────────────────────────────────────────────────

var coordinationCloseCmd = &cobra.Command{
	Use:   "close",
	Short: "Close a coordination run",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		store := coordination.New(g)
		snap, err := store.CloseCoordinationRun(context.Background(), coordinationCfg.runID)
		if err != nil {
			return err
		}
		if coordinationCfg.json {
			return printJSON(snap)
		}
		fmt.Fprintf(os.Stdout, "Run %s closed.\n", snap.Run.ID)
		if len(snap.RecommendedRules) > 0 {
			fmt.Fprintf(os.Stdout, "\nNotes at close:\n")
			for _, r := range snap.RecommendedRules {
				fmt.Fprintf(os.Stdout, "  ! %s\n", r)
			}
		}
		return nil
	},
}

func init() {
	initCoordinationCmds()
}

// initCoordinationCmds registers all coordination subcommands.
func initCoordinationCmds() {
	// start flags
	coordinationStartCmd.Flags().StringVar(&coordinationCfg.id, "id", "", "Optional explicit run ID")
	coordinationStartCmd.Flags().StringVar(&coordinationCfg.title, "title", "", "Run title")
	coordinationStartCmd.Flags().StringVar(&coordinationCfg.objective, "objective", "", "Run objective")
	coordinationStartCmd.Flags().StringVar(&coordinationCfg.agentID, "owner", "", "Owner agent ID")
	coordinationStartCmd.Flags().StringVar(&coordinationCfg.repo, "repo", "", "Repository root path")
	coordinationStartCmd.Flags().StringVar(&coordinationCfg.branch, "branch", "", "Git branch")
	coordinationStartCmd.Flags().BoolVar(&coordinationCfg.json, "json", false, "Output JSON")
	_ = coordinationStartCmd.MarkFlagRequired("title")
	_ = coordinationStartCmd.MarkFlagRequired("objective")

	// join flags
	coordinationJoinCmd.Flags().StringVar(&coordinationCfg.runID, "run", "", "Coordination run ID")
	coordinationJoinCmd.Flags().StringVar(&coordinationCfg.agentName, "agent", "", "Agent name")
	coordinationJoinCmd.Flags().StringVar(&coordinationCfg.agentKind, "kind", "claude", "Agent kind: claude | gpt | human | ci")
	coordinationJoinCmd.Flags().StringVar(&coordinationCfg.sessionID, "session", "", "Session ID from session.start")
	coordinationJoinCmd.Flags().StringVar(&coordinationCfg.role, "role", "", "Role: coder | reviewer | planner | executor")
	coordinationJoinCmd.Flags().BoolVar(&coordinationCfg.json, "json", false, "Output JSON")
	_ = coordinationJoinCmd.MarkFlagRequired("run")
	_ = coordinationJoinCmd.MarkFlagRequired("agent")

	// snapshot flags
	coordinationSnapshotCmd.Flags().StringVar(&coordinationCfg.runID, "run", "", "Coordination run ID")
	coordinationSnapshotCmd.Flags().StringVar(&coordinationCfg.agentID, "agent", "", "Your agent ID")
	coordinationSnapshotCmd.Flags().StringArrayVar(&coordinationCfg.files, "file", nil, "Files to filter decisions (repeatable)")
	coordinationSnapshotCmd.Flags().BoolVar(&coordinationCfg.json, "json", false, "Output JSON")
	_ = coordinationSnapshotCmd.MarkFlagRequired("run")

	// claim-file flags
	coordinationClaimFileCmd.Flags().StringVar(&coordinationCfg.runID, "run", "", "Coordination run ID")
	coordinationClaimFileCmd.Flags().StringVar(&coordinationCfg.agentID, "agent", "", "Your agent ID")
	coordinationClaimFileCmd.Flags().StringVar(&coordinationCfg.filePath, "file", "", "File path to claim")
	coordinationClaimFileCmd.Flags().StringVar(&coordinationCfg.claimKind, "kind", "", "read | investigate | likely_edit | do_not_touch")
	coordinationClaimFileCmd.Flags().StringVar(&coordinationCfg.reason, "reason", "", "Why you are claiming this file")
	coordinationClaimFileCmd.Flags().Int64Var(&coordinationCfg.ttl, "ttl", 0, "TTL in seconds (0 = default)")
	coordinationClaimFileCmd.Flags().BoolVar(&coordinationCfg.json, "json", false, "Output JSON")

	// lock-file flags
	coordinationLockFileCmd.Flags().StringVar(&coordinationCfg.runID, "run", "", "Coordination run ID")
	coordinationLockFileCmd.Flags().StringVar(&coordinationCfg.agentID, "agent", "", "Your agent ID")
	coordinationLockFileCmd.Flags().StringVar(&coordinationCfg.filePath, "file", "", "File path to lock")
	coordinationLockFileCmd.Flags().StringVar(&coordinationCfg.lockKind, "kind", "", "edit | rename | delete | semantic_boundary | do_not_touch")
	coordinationLockFileCmd.Flags().StringVar(&coordinationCfg.reason, "reason", "", "Why you need this lock")
	coordinationLockFileCmd.Flags().Int64Var(&coordinationCfg.ttl, "ttl", 0, "TTL in seconds (0 = default)")
	coordinationLockFileCmd.Flags().BoolVar(&coordinationCfg.json, "json", false, "Output JSON")

	// release-lock flags
	coordinationReleaseLockCmd.Flags().StringVar(&coordinationCfg.runID, "run", "", "Coordination run ID")
	coordinationReleaseLockCmd.Flags().StringVar(&coordinationCfg.agentID, "agent", "", "Your agent ID")
	coordinationReleaseLockCmd.Flags().StringVar(&coordinationCfg.lockID, "lock", "", "Lock ID to release")

	// decision flags
	coordinationDecisionCmd.Flags().StringVar(&coordinationCfg.runID, "run", "", "Coordination run ID")
	coordinationDecisionCmd.Flags().StringVar(&coordinationCfg.agentID, "agent", "", "Your agent ID")
	coordinationDecisionCmd.Flags().StringVar(&coordinationCfg.title, "title", "", "Short decision title")
	coordinationDecisionCmd.Flags().StringVar(&coordinationCfg.decision, "decision", "", "The decision made")
	coordinationDecisionCmd.Flags().StringVar(&coordinationCfg.rationale, "rationale", "", "Why this decision was made")
	coordinationDecisionCmd.Flags().StringVar(&coordinationCfg.scope, "scope", "global", "global | file | component | service")
	coordinationDecisionCmd.Flags().StringArrayVar(&coordinationCfg.files, "file", nil, "Files covered by this decision (repeatable)")
	coordinationDecisionCmd.Flags().StringArrayVar(&coordinationCfg.components, "component", nil, "Components covered (repeatable)")
	coordinationDecisionCmd.Flags().StringArrayVar(&coordinationCfg.invariants, "invariant", nil, "Invariant IDs (repeatable)")
	coordinationDecisionCmd.Flags().StringArrayVar(&coordinationCfg.incidents, "incident", nil, "Incident IDs (repeatable)")
	coordinationDecisionCmd.Flags().BoolVar(&coordinationCfg.binding, "binding", false, "If true, decision blocks conflicting locks")
	coordinationDecisionCmd.Flags().BoolVar(&coordinationCfg.json, "json", false, "Output JSON")

	// conflicts flags
	coordinationConflictsCmd.Flags().StringVar(&coordinationCfg.runID, "run", "", "Coordination run ID")
	coordinationConflictsCmd.Flags().BoolVar(&coordinationCfg.json, "json", false, "Output JSON")
	_ = coordinationConflictsCmd.MarkFlagRequired("run")

	// close flags
	coordinationCloseCmd.Flags().StringVar(&coordinationCfg.runID, "run", "", "Coordination run ID")
	coordinationCloseCmd.Flags().BoolVar(&coordinationCfg.json, "json", false, "Output JSON")
	_ = coordinationCloseCmd.MarkFlagRequired("run")

	// Register subcommands.
	coordinationCmd.AddCommand(coordinationStartCmd)
	coordinationCmd.AddCommand(coordinationJoinCmd)
	coordinationCmd.AddCommand(coordinationSnapshotCmd)
	coordinationCmd.AddCommand(coordinationClaimFileCmd)
	coordinationCmd.AddCommand(coordinationLockFileCmd)
	coordinationCmd.AddCommand(coordinationReleaseLockCmd)
	coordinationCmd.AddCommand(coordinationDecisionCmd)
	coordinationCmd.AddCommand(coordinationConflictsCmd)
	coordinationCmd.AddCommand(coordinationCloseCmd)

	awarenessCmd.AddCommand(coordinationCmd)
}
