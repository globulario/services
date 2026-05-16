package main

// awareness_learning_cmds.go: Awareness learning CLI commands (Task 3).
//
// Commands:
//
//	globular awareness incident-bundle --incident <id>
//	globular awareness propose-from-incident --incident <id>
//	globular awareness validate-proposal --file <proposal.yaml>
//	globular awareness promote-proposal --file <proposal.yaml>
//	globular awareness list-proposals
//	globular awareness queue-triage
//	globular awareness proposal-context --file <proposal.yaml>
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

	"github.com/globulario/awareness/analysis"
	"github.com/globulario/awareness/graph"
	"github.com/globulario/awareness/learning"
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
	bundleDir: "docs/awareness/incidents",
	staleAfter: 24 * time.Hour,
	output: "table",
}

// ---- incident-bundle command ----

var awarenessIncidentBundleCmd = &cobra.Command{
	Use:   "incident-bundle",
	Short: "Show a stored incident bundle (loads from docs/awareness/incidents/)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if learningCfg.incidentID == "" {
			return fmt.Errorf("--incident is required")
		}

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		bundlePath := filepath.Join(repoRoot, learningCfg.bundleDir,
			strings.ReplaceAll(learningCfg.incidentID, ".", "_")+".yaml")

		b, err := learning.LoadIncidentBundle(bundlePath)
		if err != nil {
			// Try exact filename.
			bundlePath = filepath.Join(repoRoot, learningCfg.bundleDir, learningCfg.incidentID+".yaml")
			b, err = learning.LoadIncidentBundle(bundlePath)
			if err != nil {
				return fmt.Errorf("incident bundle not found for %q (looked in %s): %w",
					learningCfg.incidentID, filepath.Join(repoRoot, learningCfg.bundleDir), err)
			}
		}

		fmt.Fprintf(os.Stdout, "# Incident Bundle\n\n")
		fmt.Fprintf(os.Stdout, "**ID**: %s\n", b.IncidentID)
		fmt.Fprintf(os.Stdout, "**Title**: %s\n", b.Title)
		fmt.Fprintf(os.Stdout, "**Severity**: %s\n", b.Severity)
		fmt.Fprintf(os.Stdout, "**Status**: %s\n\n", b.Status)

		if len(b.Symptoms) > 0 {
			fmt.Fprintf(os.Stdout, "## Symptoms\n")
			for _, s := range b.Symptoms {
				fmt.Fprintf(os.Stdout, "- %s\n", s)
			}
			fmt.Fprintln(os.Stdout)
		}

		if len(b.ObservedServices) > 0 {
			fmt.Fprintf(os.Stdout, "## Observed services\n")
			for _, s := range b.ObservedServices {
				fmt.Fprintf(os.Stdout, "- %s\n", s)
			}
			fmt.Fprintln(os.Stdout)
		}

		if b.SuspectedRootCause != "" {
			fmt.Fprintf(os.Stdout, "## Suspected root cause\n")
			fmt.Fprintf(os.Stdout, "%s\n\n", strings.TrimSpace(b.SuspectedRootCause))
		}

		if len(b.ManualRepairs) > 0 {
			fmt.Fprintf(os.Stdout, "## Manual repairs\n")
			for _, r := range b.ManualRepairs {
				fmt.Fprintf(os.Stdout, "- %s\n", r)
			}
			fmt.Fprintln(os.Stdout)
		}

		if b.Proposed != nil {
			fmt.Fprintf(os.Stdout, "Proposed awareness: %d failure modes, %d invariants, %d forbidden fixes\n",
				len(b.Proposed.FailureModes), len(b.Proposed.Invariants), len(b.Proposed.ForbiddenFixes))
		}

		return nil
	},
}

// ---- propose-from-incident command ----

var awarenessProposeFromIncidentCmd = &cobra.Command{
	Use:   "propose-from-incident",
	Short: "Generate a draft awareness proposal from an incident bundle",
	Long: `Loads an incident bundle from docs/awareness/incidents/ and generates
a draft awareness proposal YAML in docs/awareness/proposals/.

The proposal must be reviewed and validated before promotion.
AI may propose awareness — humans must approve it.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if learningCfg.incidentID == "" {
			return fmt.Errorf("--incident is required")
		}

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		// Find the incident bundle.
		incDir := filepath.Join(repoRoot, learningCfg.bundleDir)
		candidates := []string{
			filepath.Join(incDir, strings.ReplaceAll(learningCfg.incidentID, ".", "_")+".yaml"),
			filepath.Join(incDir, learningCfg.incidentID+".yaml"),
		}
		var b *learning.IncidentBundle
		for _, path := range candidates {
			b, err = learning.LoadIncidentBundle(path)
			if err == nil {
				break
			}
		}
		if b == nil {
			return fmt.Errorf("incident bundle not found for %q in %s", learningCfg.incidentID, incDir)
		}

		// Generate the proposal.
		p := learning.GenerateProposalFromBundle(b)

		// Write proposal to docs/awareness/proposals/.
		proposalsDir := filepath.Join(repoRoot, "docs", "awareness", "proposals")
		if err := os.MkdirAll(proposalsDir, 0o755); err != nil {
			return fmt.Errorf("create proposals dir: %w", err)
		}

		date := time.Now().UTC().Format("2006-01-02")
		filename := date + "-" + strings.ReplaceAll(learningCfg.incidentID, ".", "-") + ".yaml"
		outPath := filepath.Join(proposalsDir, filename)

		// Safety check: output path must remain within proposals directory.
		absProposals, err := filepath.Abs(proposalsDir)
		if err != nil {
			return fmt.Errorf("resolve proposals dir: %w", err)
		}
		absOut, err := filepath.Abs(outPath)
		if err != nil {
			return fmt.Errorf("resolve output path: %w", err)
		}
		if !strings.HasPrefix(absOut, absProposals+string(filepath.Separator)) {
			return fmt.Errorf("output path %q is outside the proposals directory %q", outPath, proposalsDir)
		}

		if err := learning.SaveProposal(outPath, p); err != nil {
			return fmt.Errorf("save proposal: %w", err)
		}

		fmt.Fprintf(os.Stdout, "Draft proposal written to: %s\n\n", outPath)
		fmt.Fprintf(os.Stdout, "  failure modes:  %d\n", len(p.FailureModes))
		fmt.Fprintf(os.Stdout, "  invariants:     %d\n", len(p.Invariants))
		fmt.Fprintf(os.Stdout, "  forbidden fixes: %d\n", len(p.ForbiddenFixes))
		fmt.Fprintf(os.Stdout, "  context aliases: %d groups\n", len(p.ContextAliases))
		fmt.Fprintf(os.Stdout, "\nNext: globular awareness validate-proposal --file %s\n", outPath)

		return nil
	},
}

// ---- validate-proposal command ----

var awarenessValidateProposalCmd = &cobra.Command{
	Use:   "validate-proposal",
	Short: "Validate an awareness proposal YAML against all admission rules",
	Long: `Validates a proposal file against 12 rules including severity-lowering checks,
forbidden-fix removal checks, cycle detection, and evidence link verification.

Exits with code 1 if validation fails.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if learningCfg.proposalFile == "" {
			return fmt.Errorf("--file is required")
		}

		ctx := context.Background()

		p, err := learning.LoadProposalFromFile(learningCfg.proposalFile)
		if err != nil {
			return fmt.Errorf("load proposal: %w", err)
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		result, err := learning.ValidateProposal(ctx, p, g)
		if err != nil {
			return fmt.Errorf("validate: %w", err)
		}

		fmt.Fprint(os.Stdout, learning.RenderValidationMarkdown(p, result))

		if result.Status == learning.ValidationFail {
			os.Exit(1)
		}
		return nil
	},
}

// ---- promote-proposal command ----

var awarenessPromoteProposalCmd = &cobra.Command{
	Use:   "promote-proposal",
	Short: "Promote a validated and approved proposal into the approved docs/awareness files",
	Long: `Validates the proposal, then merges its invariants, failure modes, forbidden fixes,
and context aliases into the approved docs/awareness YAML files.

This creates normal git-visible diffs. No hidden mutation occurs.
Promotion is gated by successful validation AND APPROVED status.

Use --allow-unapproved to bypass the APPROVED requirement (developer/test mode only).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if learningCfg.proposalFile == "" {
			return fmt.Errorf("--file is required")
		}

		ctx := context.Background()

		p, err := learning.LoadProposalFromFile(learningCfg.proposalFile)
		if err != nil {
			return fmt.Errorf("load proposal: %w", err)
		}

		// Graph is mandatory in the CLI path (graph update must happen).
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		vr, err := learning.ValidateProposal(ctx, p, g)
		if err != nil {
			return fmt.Errorf("validate: %w", err)
		}

		if vr.Status == learning.ValidationFail {
			fmt.Fprint(os.Stdout, learning.RenderValidationMarkdown(p, vr))
			fmt.Fprintf(os.Stderr, "\nBLOCKED — proposal did not pass validation. Fix issues and re-run.\n")
			os.Exit(1)
		}

		if learningCfg.allowUnapproved {
			fmt.Fprintf(os.Stderr, "WARNING: --allow-unapproved bypasses the APPROVED status requirement. Use only in developer mode.\n")
		}

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}
		docsAwarenessDir := filepath.Join(repoRoot, "docs", "awareness")

		opts := learning.PromoteOptions{AllowUnapproved: learningCfg.allowUnapproved}
		promotion, err := learning.PromoteProposal(ctx, p, vr, docsAwarenessDir, g, opts)
		if err != nil {
			return fmt.Errorf("promote: %w", err)
		}

		fmt.Fprintf(os.Stdout, "# Proposal Promoted\n\n")
		fmt.Fprintf(os.Stdout, "**Proposal**: %s\n", p.Proposal.ID)
		fmt.Fprintf(os.Stdout, "**Source incident**: %s\n\n", p.Proposal.SourceIncident)

		if len(promotion.InvariantsAdded) > 0 {
			fmt.Fprintf(os.Stdout, "## Invariants added (%d)\n", len(promotion.InvariantsAdded))
			for _, id := range promotion.InvariantsAdded {
				fmt.Fprintf(os.Stdout, "- %s\n", id)
			}
			fmt.Fprintln(os.Stdout)
		}
		if len(promotion.FailureModesAdded) > 0 {
			fmt.Fprintf(os.Stdout, "## Failure modes added (%d)\n", len(promotion.FailureModesAdded))
			for _, id := range promotion.FailureModesAdded {
				fmt.Fprintf(os.Stdout, "- %s\n", id)
			}
			fmt.Fprintln(os.Stdout)
		}
		if len(promotion.ForbiddenFixesAdded) > 0 {
			fmt.Fprintf(os.Stdout, "## Forbidden fixes added (%d)\n", len(promotion.ForbiddenFixesAdded))
			for _, id := range promotion.ForbiddenFixesAdded {
				fmt.Fprintf(os.Stdout, "- %s\n", id)
			}
			fmt.Fprintln(os.Stdout)
		}
		if promotion.AliasesAdded > 0 {
			fmt.Fprintf(os.Stdout, "## Context aliases added: %d\n\n", promotion.AliasesAdded)
		}

		fmt.Fprintf(os.Stdout, "Graph rebuild required: %v\n", promotion.GraphRebuildNeeded)
		fmt.Fprintf(os.Stdout, "\nNext: globular awareness build  # to rebuild the graph with new scars\n")

		return nil
	},
}

// ---- approve-proposal command ----

var awarenessApproveProposalCmd = &cobra.Command{
	Use:   "approve-proposal",
	Short: "Approve an awareness proposal (sets status to APPROVED)",
	Long: `Loads a proposal YAML, runs validation, and if it passes sets
the proposal status to APPROVED. The updated YAML is saved back to disk.
If a graph DB is available the proposal status is also updated there.

A proposal must be APPROVED before it can be promoted.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if learningCfg.proposalFile == "" {
			return fmt.Errorf("--file is required")
		}

		ctx := context.Background()

		p, err := learning.LoadProposalFromFile(learningCfg.proposalFile)
		if err != nil {
			return fmt.Errorf("load proposal: %w", err)
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		result, err := learning.ValidateProposal(ctx, p, g)
		if err != nil {
			return fmt.Errorf("validate: %w", err)
		}

		if result.Status == learning.ValidationFail {
			fmt.Fprint(os.Stdout, learning.RenderValidationMarkdown(p, result))
			fmt.Fprintf(os.Stderr, "\nBLOCKED — proposal did not pass validation. Fix issues before approving.\n")
			os.Exit(1)
		}

		// Change status to APPROVED.
		learning.ApproveProposal(p)

		// Save the updated proposal back to disk.
		if err := learning.SaveProposal(learningCfg.proposalFile, p); err != nil {
			return fmt.Errorf("save proposal: %w", err)
		}

		// Update graph if available.
		if g != nil {
			_ = g.UpdateProposalStatus(ctx, p.Proposal.ID, "APPROVED")
		}

		fmt.Fprintf(os.Stdout, "Proposal %q approved.\n", p.Proposal.ID)
		fmt.Fprintf(os.Stdout, "Status updated to APPROVED in %s\n", learningCfg.proposalFile)
		fmt.Fprintf(os.Stdout, "\nNext: globular awareness promote-proposal --file %s\n", learningCfg.proposalFile)

		return nil
	},
}

// ---- list-proposals command ----

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

// ---- queue-triage command ----

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
				"scanned_files":       totalYAML,
				"stale_threshold":     learningCfg.staleAfter.String(),
				"stale_count":         len(stale),
				"one_command_triage":  "globular awareness list-proposals",
				"items":               stale,
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

// ---- queue-resolve-stale command ----

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

// ---- error-contract command ----

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

// ---- closure-ledger-check command ----

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

type queueTriageItem struct {
	File              string `json:"file"`
	ID                string `json:"id"`
	Status            string `json:"status"`
	AgeHours          int64  `json:"age_hours"`
	Age               time.Duration `json:"-"`
	RecommendedAction string `json:"recommended_action"`
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
		if p, perr := learning.LoadProposalFromFile(path); perr == nil {
			if strings.TrimSpace(p.Proposal.ID) != "" {
				id = p.Proposal.ID
			}
			if strings.TrimSpace(p.Proposal.Status) != "" {
				status = strings.ToUpper(strings.TrimSpace(p.Proposal.Status))
			}
		}
		item := queueTriageItem{
			File:   e.Name(),
			ID:     id,
			Status: status,
			Age:    age,
			AgeHours: int64(age.Hours()),
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

// ---- proposal-context command ----

var awarenessProposalContextCmd = &cobra.Command{
	Use:   "proposal-context",
	Short: "Show architectural context for a proposal (what it affects in the graph)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if learningCfg.proposalFile == "" {
			return fmt.Errorf("--file is required")
		}

		ctx := context.Background()

		p, err := learning.LoadProposalFromFile(learningCfg.proposalFile)
		if err != nil {
			return fmt.Errorf("load proposal: %w", err)
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		// Build task from proposal identity.
		task := fmt.Sprintf("proposal %s from incident %s: %d failure modes %d invariants",
			p.Proposal.ID, p.Proposal.SourceIncident,
			len(p.FailureModes), len(p.Invariants))

		// Collect service hints from the proposal.
		hints := analysis.AgentContextHints{
			Services: p.AllProposedServiceIDs(),
		}

		// Load aliases for richer context.
		repoRoot, _ := resolveRepoRoot(awareCfg.repoPath)
		aliasMap := loadAliasesQuiet(repoRoot)

		md, _, err := analysis.GenerateAgentContext(ctx, g, task, hints, analysis.AgentContextAliases(aliasMap))
		if err != nil {
			return err
		}

		// Prepend proposal summary.
		fmt.Fprintf(os.Stdout, "# Proposal Context: %s\n\n", p.Proposal.ID)
		fmt.Fprintf(os.Stdout, "**Source incident**: %s\n\n", p.Proposal.SourceIncident)
		if len(p.FailureModes) > 0 {
			fmt.Fprintf(os.Stdout, "## Proposed failure modes\n")
			for _, fm := range p.FailureModes {
				fmt.Fprintf(os.Stdout, "- %s: %s\n", fm.ID, fm.Title)
			}
			fmt.Fprintln(os.Stdout)
		}
		if len(p.Invariants) > 0 {
			fmt.Fprintf(os.Stdout, "## Proposed invariants\n")
			for _, inv := range p.Invariants {
				fmt.Fprintf(os.Stdout, "- %s [%s]: %s\n", inv.ID, inv.Severity, inv.Title)
			}
			fmt.Fprintln(os.Stdout)
		}

		fmt.Fprint(os.Stdout, md)
		return nil
	},
}

// ---- aliases command ----

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

// loadAliasesQuiet loads context_aliases.yaml silently — errors produce empty map.
func loadAliasesQuiet(repoRoot string) learning.ContextAliasMap {
	aliasPath := filepath.Join(repoRoot, "docs", "awareness", "context_aliases.yaml")
	aliases, _ := learning.LoadContextAliases(aliasPath)
	return aliases
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

func init() {
	// incident-bundle flags.
	awarenessIncidentBundleCmd.Flags().StringVar(&learningCfg.incidentID, "incident", "", "Incident ID")
	awarenessIncidentBundleCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// propose-from-incident flags.
	awarenessProposeFromIncidentCmd.Flags().StringVar(&learningCfg.incidentID, "incident", "", "Incident ID to generate proposal from")
	awarenessProposeFromIncidentCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// validate-proposal flags.
	awarenessValidateProposalCmd.Flags().StringVar(&learningCfg.proposalFile, "file", "", "Path to proposal YAML file")
	awarenessValidateProposalCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessValidateProposalCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// approve-proposal flags.
	awarenessApproveProposalCmd.Flags().StringVar(&learningCfg.proposalFile, "file", "", "Path to proposal YAML file")
	awarenessApproveProposalCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessApproveProposalCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// promote-proposal flags.
	awarenessPromoteProposalCmd.Flags().StringVar(&learningCfg.proposalFile, "file", "", "Path to proposal YAML file")
	awarenessPromoteProposalCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessPromoteProposalCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessPromoteProposalCmd.Flags().BoolVar(&learningCfg.allowUnapproved, "allow-unapproved", false, "Allow promotion of proposals without APPROVED status (developer mode only)")

	// list-proposals flags.
	awarenessListProposalsCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessListProposalsCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessQueueTriageCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessQueueTriageCmd.Flags().DurationVar(&learningCfg.staleAfter, "stale-after", 24*time.Hour, "Mark proposals older than this as stale")
	awarenessQueueTriageCmd.Flags().StringVar(&learningCfg.output, "output", "table", "Output format: table|json")
	awarenessQueueResolveStaleCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessQueueResolveStaleCmd.Flags().DurationVar(&learningCfg.staleAfter, "stale-after", 24*time.Hour, "Mark proposals older than this as stale")
	awarenessQueueResolveStaleCmd.Flags().StringVar(&learningCfg.output, "output", "table", "Output format: table|json")
	awarenessQueueResolveStaleCmd.Flags().BoolVar(&learningCfg.apply, "apply", false, "Apply changes (default is dry-run)")
	awarenessErrorContractCmd.Flags().StringVar(&learningCfg.errorText, "error", "", "Reported error message")
	awarenessClosureLedgerCheckCmd.Flags().StringVar(&learningCfg.reportFile, "file", "", "Path to report/notes file containing closure ledger")
	awarenessClosureLedgerCheckCmd.Flags().StringVar(&learningCfg.statusClaim, "status-claim", "", "Optional claimed status to validate (e.g. fixed)")

	// proposal-context flags.
	awarenessProposalContextCmd.Flags().StringVar(&learningCfg.proposalFile, "file", "", "Path to proposal YAML file")
	awarenessProposalContextCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessProposalContextCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// aliases flags.
	awarenessAliasesCmd.Flags().StringVar(&awareCfg.task, "task", "", "Task description to match against aliases")
	awarenessAliasesCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// Register all new commands under awarenessCmd.
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
