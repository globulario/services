package main

// awareness_learning_cmds.go: Awareness learning CLI commands.
//
// Commands that depend on the deleted learning/failurelearning API
// (incident-bundle, propose-from-incident, validate-proposal, promote-proposal,
// approve-proposal, proposal-context) are stubs that return a clear error.
//
// The following commands remain fully functional:
//
//	globular awareness list-proposals
//	globular awareness queue-triage
//	globular awareness queue-resolve-stale
//	globular awareness error-contract
//	globular awareness closure-ledger-check
//	globular awareness aliases --task "<task>"

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/learning"
)

var learningCfg = struct {
	incidentID      string
	proposalFile    string
	bundleDir       string // where to find/save incident bundle YAML files
	allowUnapproved bool   // promote-proposal --allow-unapproved
	staleAfter      time.Duration
	output          string
	apply           bool
	errorText       string
	reportFile      string
	statusClaim     string
}{
	bundleDir:  "docs/awareness/incidents",
	staleAfter: 24 * time.Hour,
	output:     "table",
}

// ── Minimal proposal struct for file scanning ─────────────────────────────────

// minimalProposal is just enough to read proposal status and identity from a
// YAML file. The full ProposalSpec was removed from the standalone module.
type minimalProposal struct {
	Proposal struct {
		ID             string `yaml:"id"`
		SourceIncident string `yaml:"source_incident"`
		Status         string `yaml:"status"`
		CreatedAt      string `yaml:"created_at"`
	} `yaml:"proposal"`
	FailureModes   []struct{} `yaml:"failure_modes"`
	Invariants     []struct{} `yaml:"invariants"`
	ForbiddenFixes []struct{} `yaml:"forbidden_fixes"`
}

func loadMinimalProposal(path string) (*minimalProposal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p minimalProposal
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// ── Stub helper ───────────────────────────────────────────────────────────────

func makeLearningStub(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short + " (not available — removed from standalone awareness module)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("%s is not available: the learning/failurelearning API was removed from the standalone awareness module — use the MCP tools awareness.failure_learning_* instead", strings.Fields(use)[0])
		},
	}
}

// ── Stubbed commands ──────────────────────────────────────────────────────────

var awarenessIncidentBundleCmd = makeLearningStub("incident-bundle", "Show a stored incident bundle")
var awarenessProposeFromIncidentCmd = makeLearningStub("propose-from-incident", "Generate a draft awareness proposal from an incident bundle")
var awarenessValidateProposalCmd = makeLearningStub("validate-proposal", "Validate an awareness proposal YAML against all admission rules")
var awarenessPromoteProposalCmd = makeLearningStub("promote-proposal", "Promote a validated and approved proposal")
var awarenessApproveProposalCmd = makeLearningStub("approve-proposal", "Approve an awareness proposal")
var awarenessProposalContextCmd = makeLearningStub("proposal-context", "Show architectural context for a proposal")

// ── list-proposals command ────────────────────────────────────────────────────

var awarenessListProposalsCmd = &cobra.Command{
	Use:   "list-proposals",
	Short: "List awareness proposals stored in the graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		proposals, err := g.AllProposals(ctx)
		if err != nil {
			return fmt.Errorf("list proposals: %w", err)
		}

		// Also scan docs/awareness/proposals/ for proposal YAML files not yet in graph.
		repoRoot, _ := resolveRepoRoot(awareCfg.repoPath)
		proposalsDir := filepath.Join(repoRoot, "docs", "awareness", "proposals")
		files, _ := os.ReadDir(proposalsDir)

		// Filter to only YAML files.
		var yamlFiles []os.DirEntry
		for _, f := range files {
			if !f.IsDir() && strings.HasSuffix(f.Name(), ".yaml") {
				yamlFiles = append(yamlFiles, f)
			}
		}

		if len(proposals) == 0 && len(yamlFiles) == 0 {
			fmt.Fprintf(os.Stdout, "No awareness proposals found.\n")
			fmt.Fprintf(os.Stdout, "Run 'globular awareness propose-from-incident --incident <id>' to create one.\n")
			return nil
		}

		// Print proposals from graph.
		if len(proposals) > 0 {
			fmt.Fprintf(os.Stdout, "%-50s %-12s %s\n", "ID", "STATUS", "INCIDENT")
			fmt.Fprintf(os.Stdout, "%s\n", strings.Repeat("-", 90))
			for _, p := range proposals {
				fmt.Fprintf(os.Stdout, "%-50s %-12s %s\n", p.ID, p.Status, p.IncidentID)
			}
			fmt.Fprintln(os.Stdout)
		}

		// Print proposal YAML files in proposals directory.
		if len(yamlFiles) > 0 {
			fmt.Fprintf(os.Stdout, "Proposal files in docs/awareness/proposals/:\n")
			for _, f := range yamlFiles {
				fmt.Fprintf(os.Stdout, "  %s\n", f.Name())
			}
		}

		return nil
	},
}

// ── queue-triage command ──────────────────────────────────────────────────────

var awarenessQueueTriageCmd = &cobra.Command{
	Use:   "queue-triage",
	Short: "Summarize stale proposal queue and print direct next actions",
	Long: `Scans docs/awareness/proposals and classifies files by staleness and proposal status.

The output is action-oriented:
  - stale proposal files
  - one-command next action for triage
  - follow-up commands to validate/approve/promote where applicable`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}
		proposalsDir := filepath.Join(repoRoot, "docs", "awareness", "proposals")
		now := time.Now()
		stale, totalYAML, err := collectStaleProposalItems(proposalsDir, now, learningCfg.staleAfter)
		if err != nil {
			return err
		}
		if learningCfg.output == "json" {
			out := map[string]interface{}{
				"scanned_files":      totalYAML,
				"stale_threshold":    learningCfg.staleAfter.String(),
				"stale_count":        len(stale),
				"one_command_triage": "globular awareness list-proposals",
				"items":              stale,
			}
			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, string(data))
			return nil
		}

		fmt.Fprintf(os.Stdout, "Queue triage: %d proposal files scanned, %d stale (>%s)\n\n",
			totalYAML, len(stale), learningCfg.staleAfter.Round(time.Hour))
		if len(stale) == 0 {
			fmt.Fprintln(os.Stdout, "No stale proposal files detected.")
			return nil
		}

		fmt.Fprintf(os.Stdout, "%-44s %-14s %-8s %s\n", "FILE", "STATUS", "AGE", "SUGGESTED NEXT")
		fmt.Fprintf(os.Stdout, "%s\n", strings.Repeat("-", 96))
		for _, it := range stale {
			fmt.Fprintf(os.Stdout, "%-44s %-14s %-8s %s\n",
				it.File, it.Status, humanizeAge(it.Age), it.RecommendedAction)
		}
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "One-command triage:")
		fmt.Fprintln(os.Stdout, "  globular awareness list-proposals")
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, "Follow-up actions:")
		fmt.Fprintln(os.Stdout, "  globular awareness validate-proposal --file docs/awareness/proposals/<file>.yaml")
		fmt.Fprintln(os.Stdout, "  globular awareness approve-proposal --file docs/awareness/proposals/<file>.yaml")
		fmt.Fprintln(os.Stdout, "  globular awareness promote-proposal --file docs/awareness/proposals/<file>.yaml")

		return nil
	},
}

// ── queue-resolve-stale command ───────────────────────────────────────────────

var awarenessQueueResolveStaleCmd = &cobra.Command{
	Use:   "queue-resolve-stale",
	Short: "Bulk-resolve stale terminal proposals by archiving files",
	Long: `Resolves stale queue items in bulk.

By default this is a dry run. With --apply, stale proposal files with terminal
statuses (PROMOTED, REJECTED, SUPERSEDED) are renamed to .resolved to remove
them from active backlog scans while keeping the content on disk.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}
		proposalsDir := filepath.Join(repoRoot, "docs", "awareness", "proposals")
		now := time.Now()
		stale, _, err := collectStaleProposalItems(proposalsDir, now, learningCfg.staleAfter)
		if err != nil {
			return err
		}

		eligible := []queueTriageItem{}
		for _, it := range stale {
			if it.Status == graph.ProposalStatusPromoted || it.Status == graph.ProposalStatusRejected || it.Status == graph.ProposalStatusSuperseded {
				eligible = append(eligible, it)
			}
		}

		if learningCfg.output == "json" {
			out := map[string]interface{}{
				"dry_run":        !learningCfg.apply,
				"eligible_count": len(eligible),
				"eligible_items": eligible,
			}
			data, err := json.MarshalIndent(out, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, string(data))
			if !learningCfg.apply {
				return nil
			}
		}

		if len(eligible) == 0 {
			fmt.Fprintln(os.Stdout, "No stale terminal proposals to resolve.")
			return nil
		}

		if !learningCfg.apply {
			fmt.Fprintf(os.Stdout, "Dry run: %d stale terminal proposal files can be archived.\n", len(eligible))
			for _, it := range eligible {
				fmt.Fprintf(os.Stdout, "  %s (%s, %s)\n", it.File, it.Status, humanizeAge(it.Age))
			}
			fmt.Fprintln(os.Stdout, "Re-run with --apply to archive them.")
			return nil
		}

		changed := 0
		for _, it := range eligible {
			src := filepath.Join(proposalsDir, it.File)
			dst := src + ".resolved"
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("archive %s: %w", it.File, err)
			}
			changed++
		}
		fmt.Fprintf(os.Stdout, "Archived %d stale terminal proposal files.\n", changed)
		return nil
	},
}

// ── error-contract command ────────────────────────────────────────────────────

var awarenessErrorContractCmd = &cobra.Command{
	Use:   "error-contract",
	Short: "Print diagnostic+fix contract scaffold for error-fix tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		errText := strings.TrimSpace(learningCfg.errorText)
		if errText == "" {
			errText = "<paste error message>"
		}
		fmt.Fprintf(os.Stdout, "Diagnostic contract:\n")
		fmt.Fprintf(os.Stdout, "- Error: %s\n", errText)
		fmt.Fprintf(os.Stdout, "- Goal: Identify the smallest root cause and affected layer.\n")
		fmt.Fprintf(os.Stdout, "- Suspected layer: unknown\n")
		fmt.Fprintf(os.Stdout, "- Allowed diagnostic actions: inspect logs/files, run awareness session/preflight/impact, run targeted tests.\n")
		fmt.Fprintf(os.Stdout, "- Forbidden actions: no broad refactor, no unrelated cleanup, no architecture change.\n")
		fmt.Fprintf(os.Stdout, "- Diagnostic stop condition: root cause bounded, affected files+invariants+tests identified.\n\n")

		fmt.Fprintf(os.Stdout, "Diagnosis:\n")
		fmt.Fprintf(os.Stdout, "- Root cause:\n")
		fmt.Fprintf(os.Stdout, "- Evidence:\n")
		fmt.Fprintf(os.Stdout, "- Affected files:\n")
		fmt.Fprintf(os.Stdout, "- Affected invariants:\n")
		fmt.Fprintf(os.Stdout, "- Required tests:\n\n")

		fmt.Fprintf(os.Stdout, "Fix contract:\n")
		fmt.Fprintf(os.Stdout, "- Allowed files:\n")
		fmt.Fprintf(os.Stdout, "- Allowed change types: logic fix, test update, fixture update, docs update.\n")
		fmt.Fprintf(os.Stdout, "- Forbidden scope: unrelated services/refactors/authority-boundary changes.\n")
		fmt.Fprintf(os.Stdout, "- Required proof: targeted tests, impact path, graph integrity, scan violations.\n")
		fmt.Fprintf(os.Stdout, "- Stop condition: proof complete, no new critical findings, no scope drift.\n\n")

		fmt.Fprintf(os.Stdout, "Closure ledger:\n")
		fmt.Fprintf(os.Stdout, "- reported error:\n")
		fmt.Fprintf(os.Stdout, "- root cause:\n")
		fmt.Fprintf(os.Stdout, "- affected layer:\n")
		fmt.Fprintf(os.Stdout, "- files changed:\n")
		fmt.Fprintf(os.Stdout, "- invariants touched:\n")
		fmt.Fprintf(os.Stdout, "- forbidden fixes checked:\n")
		fmt.Fprintf(os.Stdout, "- tests run:\n")
		fmt.Fprintf(os.Stdout, "- tests passed:\n")
		fmt.Fprintf(os.Stdout, "- tests skipped:\n")
		fmt.Fprintf(os.Stdout, "- graph integrity:\n")
		fmt.Fprintf(os.Stdout, "- scan violations:\n")
		fmt.Fprintf(os.Stdout, "- live/runtime evidence freshness:\n")
		fmt.Fprintf(os.Stdout, "- remaining blind spots:\n")
		fmt.Fprintf(os.Stdout, "- learned knowledge proposal needed:\n")
		fmt.Fprintf(os.Stdout, "- final status: fixed|likely_fixed_proof_incomplete|patch_prepared_not_verified|blocked\n")
		return nil
	},
}

// ── closure-ledger-check command ──────────────────────────────────────────────

var awarenessClosureLedgerCheckCmd = &cobra.Command{
	Use:   "closure-ledger-check",
	Short: "Validate closure-ledger presence before claiming a fix",
	RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(learningCfg.reportFile) == "" {
			return fmt.Errorf("--file is required")
		}
		b, err := os.ReadFile(learningCfg.reportFile)
		if err != nil {
			return err
		}
		s := string(b)
		required := []string{
			"Closure ledger:",
			"reported error:",
			"root cause:",
			"affected layer:",
			"files changed:",
			"invariants touched:",
			"forbidden fixes checked:",
			"tests run:",
			"tests passed:",
			"tests skipped:",
			"graph integrity:",
			"scan violations:",
			"live/runtime evidence freshness:",
			"remaining blind spots:",
			"learned knowledge proposal needed:",
			"final status:",
		}
		missing := []string{}
		for _, r := range required {
			if !strings.Contains(s, r) {
				missing = append(missing, r)
			}
		}
		claim := strings.TrimSpace(strings.ToLower(learningCfg.statusClaim))
		if claim == "fixed" && strings.Contains(s, "tests passed:") {
			if strings.Contains(strings.ToLower(s), "tests passed: 0") {
				missing = append(missing, "tests passed must be > 0 for status=fixed")
			}
			if strings.Contains(strings.ToLower(s), "graph integrity: skipped") {
				missing = append(missing, "graph integrity cannot be skipped for status=fixed")
			}
		}

		if len(missing) > 0 {
			fmt.Fprintf(os.Stdout, "closure-ledger check: FAIL\n")
			for _, m := range missing {
				fmt.Fprintf(os.Stdout, "  missing/invalid: %s\n", m)
			}
			return fmt.Errorf("closure-ledger requirements not satisfied")
		}
		fmt.Fprintf(os.Stdout, "closure-ledger check: PASS\n")
		return nil
	},
}

// ── aliases command ───────────────────────────────────────────────────────────

var awarenessAliasesCmd = &cobra.Command{
	Use:   "aliases",
	Short: "Show which graph nodes match a task via context aliases",
	Long: `Loads docs/awareness/context_aliases.yaml and shows which invariants,
failure modes, or forbidden fixes are surfaced by the given task language.

This is the same alias-matching that agent-context uses internally.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if awareCfg.task == "" {
			return fmt.Errorf("--task is required")
		}

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		aliasPath := filepath.Join(repoRoot, "docs", "awareness", "context_aliases.yaml")
		aliases, err := learning.LoadContextAliases(aliasPath)
		if err != nil {
			return fmt.Errorf("load aliases: %w", err)
		}
		if len(aliases) == 0 {
			fmt.Fprintf(os.Stdout, "No context aliases found at %s\n", aliasPath)
			return nil
		}

		matched := learning.MatchAliasTargets(awareCfg.task, aliases)

		fmt.Fprintf(os.Stdout, "# Context Alias Matches\n\n")
		fmt.Fprintf(os.Stdout, "**Task**: %s\n\n", awareCfg.task)

		if len(matched) == 0 {
			fmt.Fprintf(os.Stdout, "No alias matches found for this task.\n")
			fmt.Fprintf(os.Stdout, "Add aliases to docs/awareness/context_aliases.yaml or promote a proposal that includes them.\n")
			return nil
		}

		fmt.Fprintf(os.Stdout, "## Matched targets (%d)\n", len(matched))
		for _, target := range matched {
			fmt.Fprintf(os.Stdout, "- %s\n", target)
			// Show which aliases matched.
			for _, phrase := range aliases[target] {
				if strings.Contains(strings.ToLower(awareCfg.task), strings.ToLower(phrase)) {
					fmt.Fprintf(os.Stdout, "    matched phrase: %q\n", phrase)
				}
			}
		}
		fmt.Fprintln(os.Stdout)

		// Offer to also run full agent-context with aliases applied.
		fmt.Fprintf(os.Stdout, "Run 'globular awareness agent-context --task %q' for the full context.\n",
			awareCfg.task)

		return nil
	},
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// loadAliasesQuiet loads context_aliases.yaml silently — errors produce empty map.
func loadAliasesQuiet(repoRoot string) learning.ContextAliasMap {
	aliasPath := filepath.Join(repoRoot, "docs", "awareness", "context_aliases.yaml")
	aliases, _ := learning.LoadContextAliases(aliasPath)
	return aliases
}

type queueTriageItem struct {
	File              string        `json:"file"`
	ID                string        `json:"id"`
	Status            string        `json:"status"`
	AgeHours          int64         `json:"age_hours"`
	Age               time.Duration `json:"-"`
	RecommendedAction string        `json:"recommended_action"`
}

func collectStaleProposalItems(proposalsDir string, now time.Time, staleAfter time.Duration) ([]queueTriageItem, int, error) {
	entries, err := os.ReadDir(proposalsDir)
	if err != nil {
		return nil, 0, fmt.Errorf("read proposals dir: %w", err)
	}
	var stale []queueTriageItem
	totalYAML := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		totalYAML++
		info, err := e.Info()
		if err != nil {
			continue
		}
		age := now.Sub(info.ModTime())
		if age <= staleAfter {
			continue
		}
		path := filepath.Join(proposalsDir, e.Name())
		id := strings.TrimSuffix(e.Name(), ".yaml")
		status := "UNKNOWN"
		if p, perr := loadMinimalProposal(path); perr == nil {
			if strings.TrimSpace(p.Proposal.ID) != "" {
				id = p.Proposal.ID
			}
			if strings.TrimSpace(p.Proposal.Status) != "" {
				status = strings.ToUpper(strings.TrimSpace(p.Proposal.Status))
			}
		}
		item := queueTriageItem{
			File:              e.Name(),
			ID:                id,
			Status:            status,
			Age:               age,
			AgeHours:          int64(age.Hours()),
			RecommendedAction: queueRecommendationForStatus(status),
		}
		stale = append(stale, item)
	}
	return stale, totalYAML, nil
}

func queueRecommendationForStatus(status string) string {
	switch status {
	case graph.ProposalStatusDraft:
		return "validate"
	case graph.ProposalStatusValidated:
		return "approve"
	case graph.ProposalStatusApproved:
		return "promote"
	case graph.ProposalStatusPromoted, graph.ProposalStatusRejected, graph.ProposalStatusSuperseded:
		return "archive/cleanup"
	default:
		return "review"
	}
}

func humanizeAge(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// incident-bundle flags (stub — kept for flag compatibility).
	awarenessIncidentBundleCmd.Flags().StringVar(&learningCfg.incidentID, "incident", "", "Incident ID")
	awarenessIncidentBundleCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// propose-from-incident flags (stub).
	awarenessProposeFromIncidentCmd.Flags().StringVar(&learningCfg.incidentID, "incident", "", "Incident ID to generate proposal from")
	awarenessProposeFromIncidentCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// validate-proposal flags (stub).
	awarenessValidateProposalCmd.Flags().StringVar(&learningCfg.proposalFile, "file", "", "Path to proposal YAML file")
	awarenessValidateProposalCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessValidateProposalCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// approve-proposal flags (stub).
	awarenessApproveProposalCmd.Flags().StringVar(&learningCfg.proposalFile, "file", "", "Path to proposal YAML file")
	awarenessApproveProposalCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessApproveProposalCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// promote-proposal flags (stub).
	awarenessPromoteProposalCmd.Flags().StringVar(&learningCfg.proposalFile, "file", "", "Path to proposal YAML file")
	awarenessPromoteProposalCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessPromoteProposalCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessPromoteProposalCmd.Flags().BoolVar(&learningCfg.allowUnapproved, "allow-unapproved", false, "Allow promotion of proposals without APPROVED status (developer mode only)")

	// list-proposals flags.
	awarenessListProposalsCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessListProposalsCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// queue-triage flags.
	awarenessQueueTriageCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessQueueTriageCmd.Flags().DurationVar(&learningCfg.staleAfter, "stale-after", 24*time.Hour, "Mark proposals older than this as stale")
	awarenessQueueTriageCmd.Flags().StringVar(&learningCfg.output, "output", "table", "Output format: table|json")

	// queue-resolve-stale flags.
	awarenessQueueResolveStaleCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessQueueResolveStaleCmd.Flags().DurationVar(&learningCfg.staleAfter, "stale-after", 24*time.Hour, "Mark proposals older than this as stale")
	awarenessQueueResolveStaleCmd.Flags().StringVar(&learningCfg.output, "output", "table", "Output format: table|json")
	awarenessQueueResolveStaleCmd.Flags().BoolVar(&learningCfg.apply, "apply", false, "Apply changes (default is dry-run)")

	// error-contract flags.
	awarenessErrorContractCmd.Flags().StringVar(&learningCfg.errorText, "error", "", "Reported error message")

	// closure-ledger-check flags.
	awarenessClosureLedgerCheckCmd.Flags().StringVar(&learningCfg.reportFile, "file", "", "Path to report/notes file containing closure ledger")
	awarenessClosureLedgerCheckCmd.Flags().StringVar(&learningCfg.statusClaim, "status-claim", "", "Optional claimed status to validate (e.g. fixed)")

	// proposal-context flags (stub).
	awarenessProposalContextCmd.Flags().StringVar(&learningCfg.proposalFile, "file", "", "Path to proposal YAML file")
	awarenessProposalContextCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessProposalContextCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// aliases flags.
	awarenessAliasesCmd.Flags().StringVar(&awareCfg.task, "task", "", "Task description to match against aliases")
	awarenessAliasesCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// Register all commands under awarenessCmd.
	awarenessCmd.AddCommand(awarenessIncidentBundleCmd)
	awarenessCmd.AddCommand(awarenessProposeFromIncidentCmd)
	awarenessCmd.AddCommand(awarenessValidateProposalCmd)
	awarenessCmd.AddCommand(awarenessApproveProposalCmd)
	awarenessCmd.AddCommand(awarenessPromoteProposalCmd)
	awarenessCmd.AddCommand(awarenessListProposalsCmd)
	awarenessCmd.AddCommand(awarenessQueueTriageCmd)
	awarenessCmd.AddCommand(awarenessQueueResolveStaleCmd)
	awarenessCmd.AddCommand(awarenessErrorContractCmd)
	awarenessCmd.AddCommand(awarenessClosureLedgerCheckCmd)
	awarenessCmd.AddCommand(awarenessProposalContextCmd)
	awarenessCmd.AddCommand(awarenessAliasesCmd)
}

// ensure graph package import is used.
var _ = graph.ProposalStatusPromoted
