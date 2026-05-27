package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/intentaudit"
)

var intentAuditCfg = struct {
	intentDir      string
	srcDir         string
	scope          string
	format         string
	failOn         string
	history        string
	runtime        bool
	runtimeTimeout string
}{}

var (
	newRuntimeEvidenceProvider = func() (intentaudit.RuntimeEvidenceProvider, error) {
		return intentaudit.NewEtcdProvider()
	}
	evaluateDesiredBuildImmutability = intentaudit.EvaluateDesiredBuildImmutability
)

var intentAuditCmd = &cobra.Command{
	Use:   "intent-audit",
	Short: "Audit intent nodes against source code for violations and test coverage",
	Long: `Runs the intent audit engine against docs/intent/*.yaml, checking violation
patterns and required tests. Outputs a human-readable summary (--format text)
or structured JSON (--format json).

Exit code is non-zero based on --fail-on:
  none          always exit 0
  violation     exit 1 if any CANDIDATE_VIOLATION found
  missing-test  exit 1 if any MISSING_REQUIRED_TEST found
  gap           exit 1 if any TEST_COVERAGE_GAP found`,
	RunE: runIntentAudit,
}

func runIntentAudit(cmd *cobra.Command, args []string) error {
	intentDir := intentAuditCfg.intentDir
	srcDir := intentAuditCfg.srcDir

	// Auto-detect from git root if not explicitly provided.
	if intentDir == "" || srcDir == "" {
		root, err := detectGitRoot()
		if err != nil {
			return fmt.Errorf("cannot detect git root (use --intent-dir and --src-dir explicitly): %w", err)
		}
		if intentDir == "" {
			intentDir = filepath.Join(root, "docs", "intent")
		}
		if srcDir == "" {
			srcDir = filepath.Join(root, "golang")
		}
	}

	// Parse scope IDs.
	var scopeIDs []string
	if intentAuditCfg.scope != "" {
		for _, id := range strings.Split(intentAuditCfg.scope, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				scopeIDs = append(scopeIDs, id)
			}
		}
	}

	report, err := intentaudit.RunAudit(intentaudit.AuditOptions{
		IntentDir: intentDir,
		SrcDir:    srcDir,
		ScopeIDs:  scopeIDs,
	})
	if err != nil {
		return fmt.Errorf("intent audit: %w", err)
	}

	// Runtime evidence (optional — requires cluster/etcd access).
	if intentAuditCfg.runtime {
		runRuntimeEvidence(report, intentAuditCfg.runtimeTimeout)
	}

	// Output.
	switch intentAuditCfg.format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return fmt.Errorf("encode JSON: %w", err)
		}
	default:
		printIntentAuditText(report)
	}

	// Append history if requested.
	if intentAuditCfg.history != "" {
		if err := intentaudit.AppendHistory(intentAuditCfg.history, report, scopeIDs); err != nil {
			fmt.Fprintf(os.Stderr, "warning: append history: %v\n", err)
		}
	}

	// Exit code based on --fail-on.
	return checkFailOn(report, intentAuditCfg.failOn)
}

func printIntentAuditText(report *intentaudit.AuditReport) {
	fmt.Fprintf(os.Stdout, "Intent Audit Report: %s\n", report.RunID)
	fmt.Fprintf(os.Stdout, "  timestamp:  %s\n", report.Timestamp.Format("2006-01-02T15:04:05Z"))
	fmt.Fprintf(os.Stdout, "  intents:    %d\n\n", report.Summary.Total)

	// Print non-pass results.
	for _, r := range report.Results {
		if r.Status == intentaudit.StatusPass {
			continue
		}
		fmt.Fprintf(os.Stdout, "  [%s] %s — %s\n", r.Status, r.IntentID, r.Title)
		for _, f := range r.Findings {
			if f.Status == intentaudit.StatusPass {
				continue
			}
			if f.File != "" {
				fmt.Fprintf(os.Stdout, "    %s:%d  %s\n", f.File, f.Line, f.Detail)
			} else {
				fmt.Fprintf(os.Stdout, "    %s\n", f.Detail)
			}
		}
	}

	fmt.Fprintf(os.Stdout, "\nSummary:\n")
	fmt.Fprintf(os.Stdout, "  pass:               %d\n", report.Summary.Pass)
	fmt.Fprintf(os.Stdout, "  candidate_violation: %d\n", report.Summary.CandidateViolation)
	fmt.Fprintf(os.Stdout, "  accepted_exception:  %d\n", report.Summary.AcceptedException)
	fmt.Fprintf(os.Stdout, "  test_coverage_gap:   %d\n", report.Summary.TestCoverageGap)
	fmt.Fprintf(os.Stdout, "  missing_test:        %d\n", report.Summary.MissingTest)

	// Runtime evidence section.
	if len(report.RuntimeEvidence) > 0 {
		fmt.Fprintln(os.Stdout, "\nRuntime Evidence:")
		for _, re := range report.RuntimeEvidence {
			fmt.Fprintf(os.Stdout, "  [%s] %s\n", strings.ToUpper(re.Status), re.Detail)
		}
	}

	// Agent Action Summary — concise actionable output for AI agents.
	printAgentActionSummary(report)
}

func printAgentActionSummary(report *intentaudit.AuditReport) {
	var mustFix []string
	var exceptions []string
	var missingProtection []string
	var runtimeIssues []string
	var violatedIDs []string

	for _, r := range report.Results {
		switch r.Status {
		case intentaudit.StatusCandidateViolation:
			for _, f := range r.Findings {
				if f.Status == intentaudit.StatusCandidateViolation {
					mustFix = append(mustFix, fmt.Sprintf("%s: %s (%s)", f.IntentID, f.File, f.Pattern))
				}
			}
			violatedIDs = append(violatedIDs, r.IntentID)
		case intentaudit.StatusAcceptedException:
			for _, f := range r.Findings {
				if f.Status == intentaudit.StatusAcceptedException && f.ExceptionID != "" {
					exceptions = append(exceptions, fmt.Sprintf("%s: %s", f.ExceptionID, f.Detail))
				}
			}
		case intentaudit.StatusTestCoverageGap:
			missingProtection = append(missingProtection, fmt.Sprintf("%s lacks required_tests", r.IntentID))
		}
	}
	for _, re := range report.RuntimeEvidence {
		switch strings.ToLower(re.Status) {
		case "fail", "unknown":
			runtimeIssues = append(runtimeIssues, fmt.Sprintf("[%s] %s", strings.ToUpper(re.Status), re.Detail))
		}
	}

	// Only print the section if there is anything actionable.
	if len(mustFix) == 0 && len(exceptions) == 0 && len(missingProtection) == 0 && len(runtimeIssues) == 0 {
		return
	}

	fmt.Fprintf(os.Stdout, "\nAgent Next Actions:\n")
	if len(mustFix) > 0 {
		fmt.Fprintf(os.Stdout, "  Must fix: %d\n", len(mustFix))
		for _, line := range mustFix {
			fmt.Fprintf(os.Stdout, "    - %s\n", line)
		}
	}
	if len(runtimeIssues) > 0 {
		fmt.Fprintf(os.Stdout, "  Runtime evidence: %d\n", len(runtimeIssues))
		for _, line := range runtimeIssues {
			fmt.Fprintf(os.Stdout, "    - %s\n", line)
		}
	}
	if len(exceptions) > 0 {
		fmt.Fprintf(os.Stdout, "  Accepted exceptions: %d\n", len(exceptions))
		for _, line := range exceptions {
			fmt.Fprintf(os.Stdout, "    - %s\n", line)
		}
	}
	if len(missingProtection) > 0 {
		fmt.Fprintf(os.Stdout, "  Coverage gaps: %d\n", len(missingProtection))
		limit := len(missingProtection)
		if limit > 5 {
			limit = 5
		}
		for _, line := range missingProtection[:limit] {
			fmt.Fprintf(os.Stdout, "    - %s\n", line)
		}
		if len(missingProtection) > limit {
			fmt.Fprintf(os.Stdout, "    - ... (%d more)\n", len(missingProtection)-limit)
		}
	}
	if len(violatedIDs) > 0 {
		fmt.Fprintf(os.Stdout, "  Suggested command: go run ./globularcli awareness intent-audit --scope %s --format text\n", strings.Join(violatedIDs, ","))
	} else if len(missingProtection) > 0 || len(runtimeIssues) > 0 {
		fmt.Fprintf(os.Stdout, "  Suggested command: go run ./globularcli awareness intent-audit --format text --fail-on none\n")
	}
}

// checkFailOn returns a non-nil error (causing non-zero exit) based on the
// --fail-on threshold.
func checkFailOn(report *intentaudit.AuditReport, failOn string) error {
	s := report.Summary
	switch failOn {
	case "none":
		return nil
	case "violation":
		if s.CandidateViolation > 0 {
			return fmt.Errorf("audit failed: %d violation(s) found", s.CandidateViolation)
		}
	case "missing-test":
		if s.CandidateViolation > 0 || s.MissingTest > 0 {
			return fmt.Errorf("audit failed: %d violation(s), %d missing test(s)",
				s.CandidateViolation, s.MissingTest)
		}
	case "gap":
		if s.CandidateViolation > 0 || s.MissingTest > 0 || s.TestCoverageGap > 0 {
			return fmt.Errorf("audit failed: %d violation(s), %d missing test(s), %d gap(s)",
				s.CandidateViolation, s.MissingTest, s.TestCoverageGap)
		}
	default:
		// Default to "violation" semantics.
		if s.CandidateViolation > 0 {
			return fmt.Errorf("audit failed: %d violation(s) found", s.CandidateViolation)
		}
	}
	return nil
}

// detectGitRoot returns the repository root via git rev-parse.
func detectGitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// runRuntimeEvidence runs optional runtime evidence checks and adds results
// to the report. Safe when etcd is unavailable — reports UNKNOWN.
func runRuntimeEvidence(report *intentaudit.AuditReport, timeoutFlag string) {
	timeout := 10 * time.Second
	if d, err := time.ParseDuration(timeoutFlag); err == nil && d > 0 {
		timeout = d
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Try to create an etcd provider.
	provider, err := newRuntimeEvidenceProvider()
	if err != nil {
		// etcd unavailable — add UNKNOWN result and continue.
		report.RuntimeEvidence = append(report.RuntimeEvidence, intentaudit.RuntimeResult{
			Status: "unknown",
			Detail: fmt.Sprintf("etcd unavailable: %v", err),
		})
		return
	}

	// Run the desired build immutability check.
	result := evaluateDesiredBuildImmutability(ctx, provider)
	report.RuntimeEvidence = append(report.RuntimeEvidence, result)

	// Validate installed-state evidence shape/ownership.
	report.RuntimeEvidence = append(report.RuntimeEvidence,
		intentaudit.EvaluateInstalledStateOwnership(ctx, provider))

	// Ensure runtime observation paths are not desired-state authorities.
	report.RuntimeEvidence = append(report.RuntimeEvidence,
		intentaudit.EvaluateRuntimeObservationDoesNotMutateDesired(ctx, provider))
}

func init() {
	intentAuditCmd.Flags().StringVar(&intentAuditCfg.intentDir, "intent-dir", "",
		"Path to intent YAML directory (default: auto-detect from git root + /docs/intent)")
	intentAuditCmd.Flags().StringVar(&intentAuditCfg.srcDir, "src-dir", "",
		"Path to Go source root for scanning (default: auto-detect from git root + /golang)")
	intentAuditCmd.Flags().StringVar(&intentAuditCfg.scope, "scope", "",
		"Comma-separated intent IDs to audit (default: all)")
	intentAuditCmd.Flags().StringVar(&intentAuditCfg.format, "format", "text",
		"Output format: text or json")
	intentAuditCmd.Flags().StringVar(&intentAuditCfg.failOn, "fail-on", "violation",
		"Exit non-zero threshold: none, violation, missing-test, or gap")
	intentAuditCmd.Flags().StringVar(&intentAuditCfg.history, "history", "",
		"Path to JSONL audit history file (appends one record per run)")
	intentAuditCmd.Flags().BoolVar(&intentAuditCfg.runtime, "runtime", false,
		"Run runtime evidence checks against live cluster (requires etcd)")
	intentAuditCmd.Flags().StringVar(&intentAuditCfg.runtimeTimeout, "runtime-timeout", "10s",
		"Timeout for runtime evidence checks")

	awarenessCmd.AddCommand(intentAuditCmd)
}
