package main

// awareness_semantic_diff_cmds.go: globular awareness semantic-diff <subcommand>
//
// Interpret unified diffs against Globular's 4-layer state model.
//
//	globular awareness semantic-diff interpret --diff-file <path> --task "<task>"
//	globular awareness semantic-diff git --base HEAD --task "<task>"
//	globular awareness semantic-diff show --report <id>

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/semanticdiff"
	"github.com/spf13/cobra"
)

var semanticDiffCmd = &cobra.Command{
	Use:   "semantic-diff",
	Short: "Interpret diffs against Globular's 4-layer state model",
}

func init() {
	awarenessCmd.AddCommand(semanticDiffCmd)
	semanticDiffCmd.AddCommand(
		sdInterpretCmd,
		sdGitCmd,
		sdShowCmd,
	)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func openSemDiffStore() (*graph.Graph, *semanticdiff.Store, error) {
	const systemPath = "/var/lib/globular/awareness/graph.db"
	if _, err := os.Stat(systemPath); err == nil {
		g, err := graph.Open(systemPath)
		if err != nil {
			return nil, nil, fmt.Errorf("open awareness graph: %w", err)
		}
		return g, semanticdiff.NewStore(g), nil
	}
	return nil, nil, fmt.Errorf("awareness graph not found — run 'globular awareness build' first")
}

func printSDReport(r *semanticdiff.SemanticDiffReport, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	fmt.Print(semanticdiff.FormatReport(r))
	return nil
}

// ── interpret ─────────────────────────────────────────────────────────────────

var sdInterpretCfg = struct {
	diffFile     string
	task         string
	sessionID    string
	requireClean bool
	jsonOut      bool
}{}

var sdInterpretCmd = &cobra.Command{
	Use:   "interpret",
	Short: "Interpret a unified diff against the 4-layer state model",
	RunE: func(cmd *cobra.Command, _ []string) error {
		var diffText string
		if sdInterpretCfg.diffFile == "-" || sdInterpretCfg.diffFile == "" {
			raw, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			diffText = string(raw)
		} else {
			raw, err := os.ReadFile(sdInterpretCfg.diffFile)
			if err != nil {
				return fmt.Errorf("read diff file: %w", err)
			}
			diffText = string(raw)
		}

		req := semanticdiff.SemanticDiffRequest{
			SessionID:    sdInterpretCfg.sessionID,
			Task:         sdInterpretCfg.task,
			DiffText:     strings.TrimSpace(diffText),
			DiffSource:   "file",
			RequireClean: sdInterpretCfg.requireClean,
		}

		ctx := context.Background()
		report, err := semanticdiff.InterpretSemanticDiff(ctx, req)
		if err != nil {
			return err
		}

		// Persist if store is available (best effort).
		if g, st, err := openSemDiffStore(); err == nil {
			defer g.Close()
			_ = st.StoreReport(ctx, report)
		}

		if sdInterpretCfg.requireClean && report.Verdict == semanticdiff.VerdictBlock {
			_ = printSDReport(report, sdInterpretCfg.jsonOut)
			return fmt.Errorf("semantic diff blocked: %s", report.Summary)
		}

		return printSDReport(report, sdInterpretCfg.jsonOut)
	},
}

func init() {
	sdInterpretCmd.Flags().StringVar(&sdInterpretCfg.diffFile, "diff-file", "-", "Path to unified diff file (or - for stdin)")
	sdInterpretCmd.Flags().StringVar(&sdInterpretCfg.task, "task", "", "Task description")
	sdInterpretCmd.Flags().StringVar(&sdInterpretCfg.sessionID, "session", "", "Session ID")
	sdInterpretCmd.Flags().BoolVar(&sdInterpretCfg.requireClean, "require-clean", false, "Exit non-zero if verdict is block")
	sdInterpretCmd.Flags().BoolVar(&sdInterpretCfg.jsonOut, "json", false, "Output as JSON")
}

// ── git ───────────────────────────────────────────────────────────────────────

var sdGitCfg = struct {
	base                   string
	head                   string
	task                   string
	sessionID              string
	requireClean           bool
	gateOnAuthorityChange  bool
	jsonOut                bool
}{}

// authorityGateFailure returns a non-empty reason when the gate
// (RequiresReview && Trust.Verdict != trusted) trips on the given
// report. Returns "" when the gate doesn't apply or passes.
//
// The gate exists to enforce the P1-6 contract: an authority-moving
// patch (Repository → Desired → Installed → Runtime crossings) must
// only land when awareness has STRONG coverage for it. "Trusted"
// verdict is the only assurance state strong enough to clear the
// authority gate; everything below (usable/limited/stale/unknown/unsafe)
// must trip the gate so a human reviews the layer crossing.
func authorityGateFailure(r *semanticdiff.SemanticDiffReport) string {
	if r == nil || r.AuthorityChange == nil || !r.AuthorityChange.RequiresReview {
		return ""
	}
	verdict := ""
	if r.Trust != nil {
		verdict = string(r.Trust.Verdict)
	}
	if verdict == "trusted" {
		return ""
	}
	from := r.AuthorityChange.FromLayer
	to := r.AuthorityChange.ToLayer
	return fmt.Sprintf("authority-change gate: %s → %s requires a 'trusted' awareness verdict; got %q. Re-run awareness build, collect runtime evidence, and improve coverage before merging.",
		from, to, verdict)
}

var sdGitCmd = &cobra.Command{
	Use:   "git",
	Short: "Interpret 'git diff' output against the 4-layer state model",
	RunE: func(cmd *cobra.Command, _ []string) error {
		base := sdGitCfg.base
		if base == "" {
			base = "HEAD"
		}
		head := sdGitCfg.head
		if head == "" {
			head = "working-tree"
		}

		var rawDiff []byte
		var cmdErr error
		if head == "working-tree" {
			rawDiff, cmdErr = exec.Command("git", "diff", base).Output()
		} else {
			rawDiff, cmdErr = exec.Command("git", "diff", base, head).Output()
		}
		if cmdErr != nil {
			return fmt.Errorf("git diff failed: %w", cmdErr)
		}

		diffText := strings.TrimSpace(string(rawDiff))
		if diffText == "" {
			fmt.Println("ALLOW: No changes found in git diff — nothing to interpret.")
			return nil
		}

		req := semanticdiff.SemanticDiffRequest{
			SessionID:    sdGitCfg.sessionID,
			Task:         sdGitCfg.task,
			DiffText:     diffText,
			DiffSource:   "git",
			GitBase:      base,
			GitHead:      head,
			RequireClean: sdGitCfg.requireClean,
		}

		ctx := context.Background()
		report, err := semanticdiff.InterpretSemanticDiff(ctx, req)
		if err != nil {
			return err
		}

		// Persist if store is available (best effort).
		if g, st, err := openSemDiffStore(); err == nil {
			defer g.Close()
			_ = st.StoreReport(ctx, report)
		}

		if sdGitCfg.requireClean && report.Verdict == semanticdiff.VerdictBlock {
			_ = printSDReport(report, sdGitCfg.jsonOut)
			return fmt.Errorf("semantic diff blocked: %s", report.Summary)
		}

		// P1-6: authority-change gate. Fails when the diff crosses an
		// authority layer that requires review AND the awareness trust
		// verdict is below 'trusted'. Independent of --require-clean so a
		// non-block diff can still trip the gate.
		if sdGitCfg.gateOnAuthorityChange {
			if reason := authorityGateFailure(report); reason != "" {
				_ = printSDReport(report, sdGitCfg.jsonOut)
				return fmt.Errorf("%s", reason)
			}
		}

		return printSDReport(report, sdGitCfg.jsonOut)
	},
}

func init() {
	sdGitCmd.Flags().StringVar(&sdGitCfg.base, "base", "HEAD", "Git base ref")
	sdGitCmd.Flags().StringVar(&sdGitCfg.head, "head", "working-tree", "Git head ref (or 'working-tree' for unstaged changes)")
	sdGitCmd.Flags().StringVar(&sdGitCfg.task, "task", "", "Task description")
	sdGitCmd.Flags().StringVar(&sdGitCfg.sessionID, "session", "", "Session ID")
	sdGitCmd.Flags().BoolVar(&sdGitCfg.requireClean, "require-clean", false, "Exit non-zero if verdict is block")
	sdGitCmd.Flags().BoolVar(&sdGitCfg.jsonOut, "json", false, "Output as JSON")
	sdGitCmd.Flags().BoolVar(&sdGitCfg.gateOnAuthorityChange, "gate-on-authority-change", false,
		"Exit non-zero when AuthorityChange.RequiresReview=true and Trust.Verdict != trusted (P1-6 gate). Use in CI on PR diffs.")
}

// ── show ──────────────────────────────────────────────────────────────────────

var sdShowCfg = struct {
	reportID string
	jsonOut  bool
}{}

var sdShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show a stored semantic diff report by ID",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if sdShowCfg.reportID == "" {
			return fmt.Errorf("--report is required")
		}
		g, st, err := openSemDiffStore()
		if err != nil {
			return err
		}
		defer g.Close()

		report, err := st.GetReport(context.Background(), sdShowCfg.reportID)
		if err != nil {
			return err
		}
		return printSDReport(report, sdShowCfg.jsonOut)
	},
}

func init() {
	sdShowCmd.Flags().StringVar(&sdShowCfg.reportID, "report", "", "Report ID to show")
	sdShowCmd.Flags().BoolVar(&sdShowCfg.jsonOut, "json", false, "Output as JSON")
	_ = sdShowCmd.MarkFlagRequired("report")
}
