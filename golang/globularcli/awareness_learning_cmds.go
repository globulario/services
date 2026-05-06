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
//	globular awareness proposal-context --file <proposal.yaml>
//	globular awareness aliases --task "<task>"

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/analysis"
	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/learning"
)

var learningCfg = struct {
	incidentID      string
	proposalFile    string
	bundleDir       string // where to find/save incident bundle YAML files
	allowUnapproved bool   // promote-proposal --allow-unapproved
}{
	bundleDir: "docs/awareness/incidents",
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
	awarenessCmd.AddCommand(awarenessProposalContextCmd)
	awarenessCmd.AddCommand(awarenessAliasesCmd)
}

// ensure graph package import is used.
var _ = graph.ProposalStatusPromoted
