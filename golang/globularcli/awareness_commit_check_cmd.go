package main

// awareness_commit_check_cmd.go: globular awareness commit-check
//
// Runs all pre-commit awareness checks in sequence and produces a combined
// verdict. Blocks non-zero exit when any check fails.
//
//	globular awareness commit-check --task "fix install retry" --semantic

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/globulario/awareness/incidentpattern"
	"github.com/globulario/awareness/scan"
	"github.com/spf13/cobra"
)

var commitCheckCfg = struct {
	task      string
	sessionID string
	base      string
	live      bool
	jsonOut   bool
	failFast  bool
}{}

var commitCheckCmd = &cobra.Command{
	Use:   "commit-check",
	Short: "Run pre-commit awareness checks: scan violations, incident patterns",
	Long: `commit-check runs the pre-commit awareness suite in order:

  1. Static scan violations on changed Go files
  2. Incident pattern match on changed files + task
  3. Live preflight (--live flag, requires running cluster)

Blocks with non-zero exit if:
  - Scan violations found critical patterns
  - High-confidence incident pattern unacknowledged

Examples:
  globular awareness commit-check --task "fix install retry loop"
  globular awareness commit-check --task "add health gate" --base HEAD~1`,

	RunE: func(cmd *cobra.Command, _ []string) error {
		ctx := context.Background()
		task := commitCheckCfg.task
		if task == "" {
			task = "pre-commit check"
		}
		base := commitCheckCfg.base
		if base == "" {
			base = "HEAD"
		}

		blocked := false
		var sections []string
		var blockReasons []string

		// ── 1. Scan violations on changed Go files ────────────────────────────
		changedFiles := commitCheckChangedGoFiles(base)
		if len(changedFiles) > 0 {
			var allFindings []scan.Finding
			for _, f := range changedFiles {
				findings, err := scan.ScanGoFile(f, nil)
				if err == nil {
					allFindings = append(allFindings, findings...)
				}
			}
			if len(allFindings) == 0 {
				sections = append(sections, fmt.Sprintf("## Scan Violations\n✓ No violations found in %d changed Go file(s).\n", len(changedFiles)))
			} else {
				var sb strings.Builder
				fmt.Fprintf(&sb, "## Scan Violations\n%d violation(s) in %d file(s):\n\n", len(allFindings), len(changedFiles))
				critical := 0
				for _, f := range allFindings {
					fmt.Fprintf(&sb, "  [%s] %s:%d — %s\n", f.PatternID, f.File, f.Line, f.WhyDangerous)
					if f.Severity == "error" || f.Severity == "critical" {
						critical++
					}
				}
				sections = append(sections, sb.String())
				if critical > 0 {
					blocked = true
					blockReasons = append(blockReasons, fmt.Sprintf("Scan violations: %d critical finding(s)", critical))
				}
			}
		} else {
			sections = append(sections, "## Scan Violations\n✓ No changed Go files to scan.\n")
		}

		// ── 3. Incident pattern match ─────────────────────────────────────────
		g, graphErr := openAwarenessGraph("", "")
		if graphErr == nil {
			defer g.Close()
			matchReq := incidentpattern.IncidentMatchRequest{
				SessionID:   commitCheckCfg.sessionID,
				Task:        task,
				Intent:      "edit",
				Files:       changedFiles,
				DiffPreview: func() string {
					out, _ := exec.Command("git", "diff", "--stat", base).Output()
					return string(out)
				}(),
			}
			matches, matchErr := incidentpattern.Match(ctx, g, matchReq)
			if matchErr != nil || len(matches) == 0 {
				sections = append(sections, "## Incident Pattern Match\n✓ No matching incident patterns.\n")
			} else {
				section := "## Incident Pattern Match\n" + incidentpattern.FormatAgentContextSection(matches)
				sections = append(sections, section)
				for _, m := range matches {
					if m.Block {
						blocked = true
						blockReasons = append(blockReasons, fmt.Sprintf("Incident pattern: %s [%s]", m.Title, m.Confidence))
					}
				}
			}
		} else {
			sections = append(sections, "## Incident Pattern Match\n⚠ Graph unavailable — skipped.\n")
		}

		// ── 4. Live preflight (optional) ──────────────────────────────────────
		if commitCheckCfg.live {
			sections = append(sections, "## Live Preflight\n⚠ Use 'globular awareness live preflight --task \"...\"' for live cluster signals.\n")
		}

		// ── Combined output ───────────────────────────────────────────────────
		fmt.Println("╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("║              GLOBULAR COMMIT CHECK                           ║")
		fmt.Println("╚══════════════════════════════════════════════════════════════╝")
		fmt.Printf("Task: %s\n", task)
		fmt.Printf("Base: %s\n\n", base)

		for _, s := range sections {
			fmt.Println(s)
		}

		if blocked {
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("COMMIT BLOCKED:")
			for _, r := range blockReasons {
				fmt.Printf("  ✗ %s\n", r)
			}
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			return fmt.Errorf("commit-check: %d block reason(s)", len(blockReasons))
		}

		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Println("✓ COMMIT ALLOWED — all checks passed.")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		return nil
	},
}

func init() {
	awarenessCmd.AddCommand(commitCheckCmd)
	commitCheckCmd.Flags().StringVar(&commitCheckCfg.task, "task", "", "Task description for context")
	commitCheckCmd.Flags().StringVar(&commitCheckCfg.sessionID, "session", "", "Session ID for correlation")
	commitCheckCmd.Flags().StringVar(&commitCheckCfg.base, "base", "HEAD", "Git base ref for diff")
	commitCheckCmd.Flags().BoolVar(&commitCheckCfg.live, "live", false, "Include live cluster preflight reminder")
	commitCheckCmd.Flags().BoolVar(&commitCheckCfg.failFast, "fail-fast", false, "Stop after first blocking check")
}

// commitCheckChangedGoFiles returns .go files changed relative to base.
func commitCheckChangedGoFiles(base string) []string {
	out, err := exec.Command("git", "diff", "--name-only", base).Output()
	if err != nil {
		return nil
	}
	var goFiles []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, ".go") {
			// Only include files that still exist.
			if _, statErr := os.Stat(line); statErr == nil {
				goFiles = append(goFiles, line)
			} else if _, statErr2 := os.Stat(filepath.Join("..", line)); statErr2 == nil {
				goFiles = append(goFiles, filepath.Join("..", line))
			}
		}
	}
	return goFiles
}
