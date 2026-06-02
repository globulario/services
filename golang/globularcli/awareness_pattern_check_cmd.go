package main

// awareness_pattern_check_cmd.go — Phase D validator.
//
// `globular awareness pattern-check <file>...` is a lightweight text scanner
// that checks each file against ImplementationPattern recipes returned by
// the awareness-graph briefing. Read-only: no etcd writes, no AST, no new
// gRPC RPC, no new schema.
//
// Behaviour:
//   1. For each file, call awareness.Briefing(file, task=<derived>) so the
//      graph's existing matcher decides which patterns apply.
//   2. For each matched pattern, scan the file content via strings.Contains
//      for each pattern.required_calls and pattern.forbidden_calls.
//   3. Report missing required calls and present forbidden calls per file.
//   4. Exit non-zero when at least one violation is found (CI-friendly).
//
// What this does NOT do:
//   - Parse Go AST. v1 is pure text scan; the brief is explicit.
//   - Recommend or apply fixes. Reference files are echoed for guidance.
//   - Fetch every ImplementationPattern proactively. The briefing already
//     filters to patterns whose triggers / file shape match — we trust
//     that gating so the CLI doesn't have to reimplement it.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
)

var (
	patternCheckOutput     string // "table" (default) | "json"
	patternCheckForceID    string // optional --pattern <bare_id> override (deferred to v2; flag accepted but unused for now)
	patternCheckExitOnFail bool   // exit non-zero on violation (default true)
)

var awarenessPatternCheckCmd = &cobra.Command{
	Use:   "pattern-check <file>...",
	Short: "Check files against ImplementationPattern recipes (required/forbidden calls)",
	Long: `Text-scans each file against ImplementationPattern recipes returned
by the awareness-graph briefing. Reports:

  - missing required calls (e.g. globular.InitClient)
  - present forbidden calls (e.g. grpc.Dial)
  - reference files to consult for the canonical recipe

The validator is intentionally lightweight: it uses strings.Contains, not
a Go AST parser. False positives are possible when a forbidden symbol
appears in a comment or string literal — fix them anyway, or rephrase
the comment. The intent is to catch service clients that bypass the
shared Globular helpers, not to be a perfect linter.

Exit code: 0 when all scanned files satisfy every matched pattern, 1
when at least one violation is found.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runAwarenessPatternCheck,
}

func runAwarenessPatternCheck(cmd *cobra.Command, args []string) error {
	cli, err := awarenessDialClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results := make([]patternCheckFileResult, 0, len(args))
	totalViolations := 0
	for _, file := range args {
		fr := checkOneFile(ctx, cli.Briefing, file)
		totalViolations += fr.violationCount()
		results = append(results, fr)
	}

	switch patternCheckOutput {
	case "json":
		printPatternCheckJSON(results)
	default:
		printPatternCheckTable(results)
	}

	if patternCheckExitOnFail && totalViolations > 0 {
		// cobra prints "Error:" before the message; use SilenceUsage to keep
		// the usage block out of the output for a clean CI tail.
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		return fmt.Errorf("pattern-check: %d violation(s)", totalViolations)
	}
	return nil
}

// patternCheckFileResult is the per-file structured output. One entry per
// matched pattern, plus optional read/briefing errors.
type patternCheckFileResult struct {
	File           string                   `json:"file"`
	Error          string                   `json:"error,omitempty"`
	PatternResults []patternCheckOneResult  `json:"patterns,omitempty"`
}

type patternCheckOneResult struct {
	PatternID       string   `json:"pattern_id"`
	PatternLabel    string   `json:"pattern_label,omitempty"`
	MatchStrength   string   `json:"match_strength,omitempty"`
	MissingRequired []string `json:"missing_required,omitempty"`
	ForbiddenFound  []string `json:"forbidden_found,omitempty"`
	ReferenceFiles  []string `json:"reference_files,omitempty"`
	Status          string   `json:"status"` // "pass" | "violation"
}

func (r patternCheckFileResult) violationCount() int {
	n := 0
	for _, p := range r.PatternResults {
		if p.Status == "violation" {
			n++
		}
	}
	return n
}

// briefingFn is the slice of the awareness client surface the validator
// needs. Extracted so unit tests can inject a fake without standing up a
// gRPC server.
type briefingFn func(ctx context.Context, file, task, depth string) (*awarenesspb.BriefingResponse, error)

// checkOneFile is the core validation logic for one file. Separated from
// runAwarenessPatternCheck so it's unit-testable with a fake briefingFn.
func checkOneFile(ctx context.Context, briefing briefingFn, file string) patternCheckFileResult {
	out := patternCheckFileResult{File: file}

	content, err := os.ReadFile(file)
	if err != nil {
		out.Error = "read: " + err.Error()
		return out
	}

	// Call briefing with a synthesized task derived from the filename so the
	// graph's narrow-file-shape rule has a keyword to bind to. The exact
	// task text doesn't matter beyond providing keywords — the matcher
	// will pick patterns whose triggers overlap.
	task := derivePatternCheckTask(file)
	resp, err := briefing(ctx, file, task, "compact")
	if err != nil {
		out.Error = "briefing: " + err.Error()
		return out
	}

	patterns := resp.GetImplementationPatterns()
	if len(patterns) == 0 {
		// No patterns matched — nothing to scan against. Not a violation.
		return out
	}

	contentStr := string(content)
	for _, p := range patterns {
		one := patternCheckOneResult{
			PatternID:      trimPatternIDPrefix(p.GetId()),
			PatternLabel:   p.GetLabel(),
			MatchStrength:  p.GetMatchStrength(),
			ReferenceFiles: p.GetReferenceFiles(),
			Status:         "pass",
		}
		for _, req := range p.GetRequiredCalls() {
			if req != "" && !strings.Contains(contentStr, req) {
				one.MissingRequired = append(one.MissingRequired, req)
			}
		}
		for _, forb := range p.GetForbiddenCalls() {
			if forb != "" && strings.Contains(contentStr, forb) {
				one.ForbiddenFound = append(one.ForbiddenFound, forb)
			}
		}
		if len(one.MissingRequired) > 0 || len(one.ForbiddenFound) > 0 {
			one.Status = "violation"
		}
		out.PatternResults = append(out.PatternResults, one)
	}
	return out
}

// derivePatternCheckTask turns a file path into a task string with high
// keyword overlap for common pattern triggers. We don't need to be precise
// — anything that ensures the briefing's narrow-file-shape rule has ≥1
// keyword to bind to is enough.
//
// Strategy: take the filename without extension, replace underscores with
// spaces (so "echo_client.go" → "echo client"), then prepend the literal
// "service" so the most common Globular client trigger always overlaps.
func derivePatternCheckTask(file string) string {
	base := filepath.Base(file)
	if dot := strings.LastIndexByte(base, '.'); dot > 0 {
		base = base[:dot]
	}
	return "service " + strings.ReplaceAll(base, "_", " ")
}

// trimPatternIDPrefix strips the class-qualified prefix so reports show
// the bare id (e.g. "globular.pattern.grpc_client_standard").
func trimPatternIDPrefix(id string) string {
	const p = "implementation_pattern:"
	if strings.HasPrefix(id, p) {
		return id[len(p):]
	}
	return id
}

// ─── output ───────────────────────────────────────────────────────────────

func printPatternCheckTable(results []patternCheckFileResult) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer tw.Flush()
	fmt.Fprintln(tw, "FILE\tPATTERN\tSTATUS\tDETAIL")

	any := false
	for _, fr := range results {
		if fr.Error != "" {
			fmt.Fprintf(tw, "%s\t-\tERROR\t%s\n", fr.File, fr.Error)
			any = true
			continue
		}
		if len(fr.PatternResults) == 0 {
			fmt.Fprintf(tw, "%s\t-\tno_pattern\t(no implementation pattern matched this file)\n", fr.File)
			any = true
			continue
		}
		for _, p := range fr.PatternResults {
			detail := ""
			switch p.Status {
			case "violation":
				parts := []string{}
				if len(p.MissingRequired) > 0 {
					parts = append(parts, "missing: "+strings.Join(p.MissingRequired, ","))
				}
				if len(p.ForbiddenFound) > 0 {
					parts = append(parts, "forbidden: "+strings.Join(p.ForbiddenFound, ","))
				}
				detail = strings.Join(parts, "; ")
			case "pass":
				detail = "ok"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", fr.File, p.PatternID, strings.ToUpper(p.Status), detail)
			any = true
		}
	}
	if !any {
		fmt.Fprintln(tw, "(no files scanned)")
	}

	// Surface reference files separately so violations carry actionable hints.
	for _, fr := range results {
		for _, p := range fr.PatternResults {
			if p.Status == "violation" && len(p.ReferenceFiles) > 0 {
				fmt.Fprintln(os.Stdout)
				fmt.Fprintf(os.Stdout, "%s — pattern %s recommends consulting:\n", fr.File, p.PatternID)
				for _, ref := range p.ReferenceFiles {
					fmt.Fprintf(os.Stdout, "  - %s\n", ref)
				}
			}
		}
	}
}

func printPatternCheckJSON(results []patternCheckFileResult) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]interface{}{
		"results": results,
	})
}

// ─── registration ─────────────────────────────────────────────────────────

func init() {
	awarenessPatternCheckCmd.Flags().StringVar(&patternCheckOutput, "format", "table",
		"output format: table | json")
	awarenessPatternCheckCmd.Flags().BoolVar(&patternCheckExitOnFail, "fail-on-violation", true,
		"exit non-zero when at least one violation is found (CI-friendly)")
	awarenessPatternCheckCmd.Flags().StringVar(&patternCheckForceID, "pattern", "",
		"reserved — force-check against a specific pattern id (not implemented in v1)")
	_ = patternCheckForceID // silence unused for v1 — flag accepted to keep the surface stable

	awarenessCmd.AddCommand(awarenessPatternCheckCmd)
}
