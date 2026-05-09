package main

// awareness_failurelearning_cmds.go: globular awareness failure-learning <subcommand>
//
// Commands:
//
//	globular awareness failure-learning propose         --source-type incident --source-id INC-2026-0012 [--error ...] [--cause ...] ...
//	globular awareness failure-learning propose-incident --incident INC-2026-0012
//	globular awareness failure-learning list-pending    [--json]
//	globular awareness failure-learning show            --proposal FLP-... [--json]
//	globular awareness failure-learning approve         --proposal FLP-... --reviewer dave [--notes "..."]
//	globular awareness failure-learning reject          --proposal FLP-... --reviewer dave --reason "..."
//	globular awareness failure-learning defer           --proposal FLP-... --reviewer dave --reason "..."
//	globular awareness failure-learning apply           --proposal FLP-...
//	globular awareness failure-learning sync-seeds      [--dry-run]
//	globular awareness failure-learning check-closure   --closure CLOSE-... [--has-root-cause] [--has-resolution] [--has-proof]

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/awareness/failuregraph"
	"github.com/globulario/services/golang/awareness/failurelearning"
	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/incidentpattern"
	"github.com/spf13/cobra"
)

var failureLearningCmd = &cobra.Command{
	Use:   "failure-learning",
	Short: "Failure Graph Learning Loop: propose, review, apply, sync knowledge updates",
}

func init() {
	awarenessCmd.AddCommand(failureLearningCmd)
	failureLearningCmd.AddCommand(
		flProposeCmd,
		flProposeIncidentCmd,
		flListPendingCmd,
		flShowCmd,
		flApproveCmd,
		flRejectCmd,
		flDeferCmd,
		flApplyCmd,
		flSyncSeedsCmd,
		flCheckClosureCmd,
	)
}

var flCfg = struct {
	sourceType    string
	sourceID      string
	incidentID    string
	proposalID    string
	reviewer      string
	decision      string
	notes         string
	reason        string
	rawErrors     []string
	causes        []string
	resolutions   []string
	wrongFixes    []string
	tests         []string
	files         []string
	components    []string
	hasRootCause  bool
	hasResolution bool
	hasProof      bool
	closureID     string
	createdBy     string
	dryRun        bool
	jsonOutput    bool
}{}

func openLearningStore() (*graph.Graph, *failurelearning.Store, *failuregraph.Store, error) {
	const systemPath = "/var/lib/globular/awareness/graph.db"
	if _, err := os.Stat(systemPath); err != nil {
		return nil, nil, nil, fmt.Errorf("awareness graph not found — run 'globular awareness build' first")
	}
	g, err := graph.Open(systemPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open awareness graph: %w", err)
	}
	return g, failurelearning.New(g), failuregraph.New(g), nil
}

// docsDirFromPath walks up from the binary's location to find <repo>/docs/awareness/.
// Returns the full docs/awareness path, or "" if not found.
func docsDirFromPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	dir := filepath.Dir(exe)
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "docs", "awareness")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	return ""
}

// ── propose ──────────────────────────────────────────────────────────────────

var flProposeCmd = &cobra.Command{
	Use:   "propose",
	Short: "Propose a Failure Graph update from raw fields",
	Example: `  globular awareness failure-learning propose \
    --source-type incident --source-id INC-2026-0012 \
    --error "x509: certificate is valid for foo, not 10.0.0.100" \
    --cause "TLS identity drifted" \
    --resolution "Use canonical DNS endpoint" \
    --wrong-fix "Do not disable TLS" \
    --test "Advertised endpoint matches cert SAN"`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flCfg.sourceType == "" || flCfg.sourceID == "" {
			return fmt.Errorf("--source-type and --source-id are required")
		}
		g, ls, fg, err := openLearningStore()
		if err != nil {
			return err
		}
		defer g.Close()

		req := failurelearning.ProposeRequest{
			SourceType:   flCfg.sourceType,
			SourceID:     flCfg.sourceID,
			CreatedBy:    flCfg.createdBy,
			RawErrors:    flCfg.rawErrors,
			RootCauses:   flCfg.causes,
			Resolutions:  flCfg.resolutions,
			WrongFixes:   flCfg.wrongFixes,
			Tests:        flCfg.tests,
			Files:        flCfg.files,
			Components:   flCfg.components,
		}
		p, err := failurelearning.ProposeUpdate(context.Background(), req, ls, fg)
		if err != nil {
			return err
		}
		if flCfg.jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(p)
		}
		printProposal(p)
		return nil
	},
}

// ── propose-incident ─────────────────────────────────────────────────────────

var flProposeIncidentCmd = &cobra.Command{
	Use:   "propose-incident",
	Short: "Propose a Failure Graph update by reading an existing incident pattern",
	Example: `  globular awareness failure-learning propose-incident --incident INC-2026-0012`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flCfg.incidentID == "" {
			return fmt.Errorf("--incident is required")
		}
		g, ls, fg, err := openLearningStore()
		if err != nil {
			return err
		}
		defer g.Close()

		ip := incidentpattern.NewStore(g)
		extract, err := failurelearning.ExtractFromIncident(context.Background(), flCfg.incidentID, ip, fg)
		if err != nil {
			return err
		}
		if extract == nil {
			return fmt.Errorf("incident not found: %s", flCfg.incidentID)
		}

		req := failurelearning.ProposeRequest{
			SourceType:   failurelearning.SourceIncident,
			SourceID:     flCfg.incidentID,
			CreatedBy:    flCfg.createdBy,
			RawErrors:    extract.RawErrors,
			Symptoms:     extract.Symptoms,
			RootCauses:   extract.RootCauses,
			Resolutions:  extract.Resolutions,
			WrongFixes:   extract.WrongFixes,
			Tests:        extract.RegressionTests,
			Files:        extract.RelatedFiles,
		}
		p, err := failurelearning.ProposeUpdate(context.Background(), req, ls, fg)
		if err != nil {
			return err
		}
		if flCfg.jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(p)
		}
		printProposal(p)
		return nil
	},
}

// ── list-pending ─────────────────────────────────────────────────────────────

var flListPendingCmd = &cobra.Command{
	Use:   "list-pending",
	Short: "List pending Failure Graph learning proposals",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, ls, _, err := openLearningStore()
		if err != nil {
			return err
		}
		defer g.Close()

		proposals, err := ls.ListPending(context.Background())
		if err != nil {
			return err
		}
		if flCfg.jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(proposals)
		}
		if len(proposals) == 0 {
			fmt.Fprintln(os.Stdout, "No pending failure learning proposals.")
			return nil
		}
		fmt.Fprintf(os.Stdout, "PENDING FAILURE LEARNING PROPOSALS (%d)\n\n", len(proposals))
		for _, p := range proposals {
			fmt.Fprintf(os.Stdout, "  %-30s  %-22s  %s\n", p.ID, p.ProposalKind, p.TargetCategoryID)
			fmt.Fprintf(os.Stdout, "  %s\n\n", wrapAt(p.Summary, 72))
		}
		return nil
	},
}

// ── show ─────────────────────────────────────────────────────────────────────

var flShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show a failure learning proposal in full",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flCfg.proposalID == "" {
			return fmt.Errorf("--proposal is required")
		}
		g, ls, _, err := openLearningStore()
		if err != nil {
			return err
		}
		defer g.Close()

		p, err := ls.GetProposal(context.Background(), flCfg.proposalID)
		if err != nil {
			return err
		}
		if flCfg.jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(p)
		}
		printProposalFull(p)
		return nil
	},
}

// ── approve ───────────────────────────────────────────────────────────────────

var flApproveCmd = &cobra.Command{
	Use:   "approve",
	Short: "Approve a failure learning proposal (does not apply — use 'apply' next)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flCfg.proposalID == "" || flCfg.reviewer == "" {
			return fmt.Errorf("--proposal and --reviewer are required")
		}
		g, ls, _, err := openLearningStore()
		if err != nil {
			return err
		}
		defer g.Close()

		p, err := failurelearning.ReviewProposal(context.Background(),
			flCfg.proposalID, flCfg.reviewer, failurelearning.DecisionApprove, flCfg.notes, nil, ls)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Approved: %s\n", p.ID)
		fmt.Fprintf(os.Stdout, "Run: globular awareness failure-learning apply --proposal %s\n", p.ID)
		return nil
	},
}

// ── reject ────────────────────────────────────────────────────────────────────

var flRejectCmd = &cobra.Command{
	Use:   "reject",
	Short: "Reject a failure learning proposal",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flCfg.proposalID == "" || flCfg.reviewer == "" {
			return fmt.Errorf("--proposal and --reviewer are required")
		}
		g, ls, _, err := openLearningStore()
		if err != nil {
			return err
		}
		defer g.Close()

		if err := failurelearning.RejectProposal(context.Background(),
			flCfg.proposalID, flCfg.reviewer, flCfg.reason, ls); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Rejected: %s\n", flCfg.proposalID)
		return nil
	},
}

// ── defer ─────────────────────────────────────────────────────────────────────

var flDeferCmd = &cobra.Command{
	Use:   "defer",
	Short: "Defer a failure learning proposal",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flCfg.proposalID == "" || flCfg.reviewer == "" {
			return fmt.Errorf("--proposal and --reviewer are required")
		}
		g, ls, _, err := openLearningStore()
		if err != nil {
			return err
		}
		defer g.Close()

		if err := failurelearning.DeferProposal(context.Background(),
			flCfg.proposalID, flCfg.reviewer, flCfg.reason, ls); err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Deferred: %s\n", flCfg.proposalID)
		return nil
	},
}

// ── apply ─────────────────────────────────────────────────────────────────────

var flApplyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply an approved proposal: patches SQLite graph and writes YAML seed",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flCfg.proposalID == "" {
			return fmt.Errorf("--proposal is required")
		}
		g, ls, fg, err := openLearningStore()
		if err != nil {
			return err
		}
		defer g.Close()

		result, err := failurelearning.ApplyProposal(context.Background(),
			flCfg.proposalID, ls, fg, docsDirFromPath())
		if err != nil {
			return err
		}
		if flCfg.jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(result)
		}
		fmt.Fprintf(os.Stdout, "Applied: %s\n", result.ProposalID)
		fmt.Fprintf(os.Stdout, "  Created nodes: %d\n", result.CreatedNodes)
		fmt.Fprintf(os.Stdout, "  Created edges: %d\n", result.CreatedEdges)
		if result.SeedPath != "" {
			fmt.Fprintf(os.Stdout, "  Seed YAML:     %s\n", result.SeedPath)
		}
		return nil
	},
}

// ── sync-seeds ────────────────────────────────────────────────────────────────

var flSyncSeedsCmd = &cobra.Command{
	Use:   "sync-seeds",
	Short: "Export all Failure Graph categories to YAML seed files",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, _, fg, err := openLearningStore()
		if err != nil {
			return err
		}
		defer g.Close()

		docsDir := docsDirFromPath()
		if docsDir == "" {
			return fmt.Errorf("could not locate docs/awareness directory (run from within the services repo)")
		}
		if flCfg.dryRun {
			fmt.Fprintf(os.Stdout, "Dry run: would export seeds to %s/failuregraph_seeds/\n", docsDir)
			return nil
		}
		n, err := failurelearning.ExportSeeds(context.Background(), docsDir, fg)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Exported %d seed files to %s/failuregraph_seeds/\n", n, docsDir)
		return nil
	},
}

// ── check-closure ─────────────────────────────────────────────────────────────

var flCheckClosureCmd = &cobra.Command{
	Use:   "check-closure",
	Short: "Check whether a closure requires a failure learning proposal",
	Example: `  globular awareness failure-learning check-closure \
    --closure INC-2026-0012 --has-root-cause --has-resolution`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if flCfg.closureID == "" {
			return fmt.Errorf("--closure is required")
		}
		g, ls, fg, err := openLearningStore()
		if err != nil {
			return err
		}
		defer g.Close()

		info := failurelearning.ClosureInfo{
			ClosureID:     flCfg.closureID,
			SourceType:    flCfg.sourceType,
			HasRootCause:  flCfg.hasRootCause,
			HasResolution: flCfg.hasResolution,
			HasProof:      flCfg.hasProof,
		}
		result, err := failurelearning.CheckClosure(context.Background(), info, ls, fg)
		if err != nil {
			return err
		}
		if flCfg.jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(result)
		}
		fmt.Fprintf(os.Stdout, "Closure status: %s\n", result.Status)
		if result.RequiresLearning {
			fmt.Fprintf(os.Stdout, "Learning required: %s\n", result.Reason)
			fmt.Fprintf(os.Stdout, "Run: globular awareness failure-learning propose --source-type closure --source-id %s\n", flCfg.closureID)
		}
		if result.ExistingProposalID != "" {
			fmt.Fprintf(os.Stdout, "Existing proposal: %s\n", result.ExistingProposalID)
		}
		return nil
	},
}

// ── flag registration ─────────────────────────────────────────────────────────

func init() {
	// propose
	flProposeCmd.Flags().StringVar(&flCfg.sourceType, "source-type", "", "Source type: incident | session | closure")
	flProposeCmd.Flags().StringVar(&flCfg.sourceID, "source-id", "", "Source ID")
	flProposeCmd.Flags().StringVar(&flCfg.createdBy, "created-by", "", "Proposer identity")
	flProposeCmd.Flags().StringArrayVar(&flCfg.rawErrors, "error", nil, "Raw error string (repeatable)")
	flProposeCmd.Flags().StringArrayVar(&flCfg.causes, "cause", nil, "Root cause (repeatable)")
	flProposeCmd.Flags().StringArrayVar(&flCfg.resolutions, "resolution", nil, "Resolution (repeatable)")
	flProposeCmd.Flags().StringArrayVar(&flCfg.wrongFixes, "wrong-fix", nil, "Wrong fix (repeatable)")
	flProposeCmd.Flags().StringArrayVar(&flCfg.tests, "test", nil, "Regression test (repeatable)")
	flProposeCmd.Flags().StringArrayVar(&flCfg.files, "file", nil, "Related file (repeatable)")
	flProposeCmd.Flags().StringArrayVar(&flCfg.components, "component", nil, "Related component (repeatable)")
	flProposeCmd.Flags().BoolVar(&flCfg.jsonOutput, "json", false, "JSON output")

	// propose-incident
	flProposeIncidentCmd.Flags().StringVar(&flCfg.incidentID, "incident", "", "Incident ID")
	flProposeIncidentCmd.Flags().StringVar(&flCfg.createdBy, "created-by", "", "Proposer identity")
	flProposeIncidentCmd.Flags().BoolVar(&flCfg.jsonOutput, "json", false, "JSON output")

	// list-pending
	flListPendingCmd.Flags().BoolVar(&flCfg.jsonOutput, "json", false, "JSON output")

	// show
	flShowCmd.Flags().StringVar(&flCfg.proposalID, "proposal", "", "Proposal ID (FLP-...)")
	flShowCmd.Flags().BoolVar(&flCfg.jsonOutput, "json", false, "JSON output")

	// approve
	flApproveCmd.Flags().StringVar(&flCfg.proposalID, "proposal", "", "Proposal ID")
	flApproveCmd.Flags().StringVar(&flCfg.reviewer, "reviewer", "", "Reviewer identity")
	flApproveCmd.Flags().StringVar(&flCfg.notes, "notes", "", "Review notes")

	// reject
	flRejectCmd.Flags().StringVar(&flCfg.proposalID, "proposal", "", "Proposal ID")
	flRejectCmd.Flags().StringVar(&flCfg.reviewer, "reviewer", "", "Reviewer identity")
	flRejectCmd.Flags().StringVar(&flCfg.reason, "reason", "", "Rejection reason")

	// defer
	flDeferCmd.Flags().StringVar(&flCfg.proposalID, "proposal", "", "Proposal ID")
	flDeferCmd.Flags().StringVar(&flCfg.reviewer, "reviewer", "", "Reviewer identity")
	flDeferCmd.Flags().StringVar(&flCfg.reason, "reason", "", "Deferral reason")

	// apply
	flApplyCmd.Flags().StringVar(&flCfg.proposalID, "proposal", "", "Proposal ID")
	flApplyCmd.Flags().BoolVar(&flCfg.jsonOutput, "json", false, "JSON output")

	// sync-seeds
	flSyncSeedsCmd.Flags().BoolVar(&flCfg.dryRun, "dry-run", false, "Show what would be exported without writing")

	// check-closure
	flCheckClosureCmd.Flags().StringVar(&flCfg.closureID, "closure", "", "Closure or incident ID")
	flCheckClosureCmd.Flags().StringVar(&flCfg.sourceType, "source-type", "runtime_bug", "Closure source type")
	flCheckClosureCmd.Flags().BoolVar(&flCfg.hasRootCause, "has-root-cause", false, "Root cause was identified")
	flCheckClosureCmd.Flags().BoolVar(&flCfg.hasResolution, "has-resolution", false, "Resolution was applied")
	flCheckClosureCmd.Flags().BoolVar(&flCfg.hasProof, "has-proof", false, "Proof/test was recorded")
	flCheckClosureCmd.Flags().BoolVar(&flCfg.jsonOutput, "json", false, "JSON output")
}

// ── output helpers ────────────────────────────────────────────────────────────

func printProposal(p *failurelearning.FailureLearningProposal) {
	fmt.Fprintf(os.Stdout, "FAILURE LEARNING PROPOSAL\n\n")
	fmt.Fprintf(os.Stdout, "Proposal:   %s\n", p.ID)
	fmt.Fprintf(os.Stdout, "Kind:       %s\n", p.ProposalKind)
	fmt.Fprintf(os.Stdout, "Status:     %s\n", p.Status)
	fmt.Fprintf(os.Stdout, "Confidence: %s\n\n", p.Confidence)
	if p.TargetCategoryID != "" {
		fmt.Fprintf(os.Stdout, "Target category: %s\n", p.TargetCategoryID)
	}
	if p.Title != "" {
		fmt.Fprintf(os.Stdout, "Title:  %s\n", p.Title)
	}
	if p.Summary != "" {
		fmt.Fprintf(os.Stdout, "Summary:\n  %s\n\n", p.Summary)
	}
	fmt.Fprintf(os.Stdout, "Review:\n  globular awareness failure-learning approve --proposal %s --reviewer <you>\n", p.ID)
	fmt.Fprintf(os.Stdout, "  globular awareness failure-learning reject  --proposal %s --reviewer <you> --reason \"...\"\n", p.ID)
}

func printProposalFull(p *failurelearning.FailureLearningProposal) {
	printProposal(p)

	ext := p.Extracted
	if len(ext.RawErrors) > 0 {
		fmt.Fprintln(os.Stdout, "\nObserved errors:")
		for _, e := range ext.RawErrors {
			fmt.Fprintf(os.Stdout, "  - %s\n", e)
		}
	}
	if len(ext.RootCauses) > 0 {
		fmt.Fprintln(os.Stdout, "\nRoot causes:")
		for _, c := range ext.RootCauses {
			fmt.Fprintf(os.Stdout, "  - %s\n", c)
		}
	}
	if len(ext.Resolutions) > 0 {
		fmt.Fprintln(os.Stdout, "\nResolutions:")
		for _, r := range ext.Resolutions {
			fmt.Fprintf(os.Stdout, "  - %s\n", r)
		}
	}
	if len(ext.WrongFixes) > 0 {
		fmt.Fprintln(os.Stdout, "\nWrong fixes:")
		for _, wf := range ext.WrongFixes {
			fmt.Fprintf(os.Stdout, "  - %s\n", wf)
		}
	}
	if len(ext.RegressionTests) > 0 {
		fmt.Fprintln(os.Stdout, "\nRegression tests:")
		for _, t := range ext.RegressionTests {
			fmt.Fprintf(os.Stdout, "  - %s\n", t)
		}
	}

	if p.Patch.SeedYAML != "" {
		fmt.Fprintf(os.Stdout, "\nSeed YAML preview:\n%s\n",
			indent(strings.Join(strings.Split(p.Patch.SeedYAML, "\n")[:min(20, len(strings.Split(p.Patch.SeedYAML, "\n")))], "\n"), "  "))
	}
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
