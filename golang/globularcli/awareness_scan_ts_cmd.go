package main

// awareness_scan_ts_cmd.go: offline TypeScript invariant-violation scanner.
//
// Usage:
//
//	globular awareness scan-ts [--dir <path>] [--repo <path>] [--format text|json]
//
// Walks .ts/.tsx files and reports pattern-based invariant violations without
// opening the graph database. Useful as a fast pre-commit check.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/extractors/typescript"
)

var scanTsCfg = struct {
	dir    string
	format string // "text" | "json"
}{}

var awarenessScanTsCmd = &cobra.Command{
	Use:   "scan-ts",
	Short: "Scan TypeScript files for invariant violations (no graph required)",
	Long: `Walk .ts/.tsx source files under --dir (default: repo root) and report
any lines that match the built-in forbidden-pattern rules:

  ui.token_storage_sessionStorage_only        localStorage token storage
  ui.no_hardcoded_backend_addresses           hardcoded http://host:port strings
  ui.grpc_web_errors_must_surface_to_operator empty catch blocks
  ui.unknown_state_must_not_appear_healthy     default branch maps unknown → "healthy"

Exits with code 0 when clean, 1 when violations are found.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := scanTsCfg.dir
		if dir == "" {
			repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
			if err != nil {
				return err
			}
			dir = repoRoot
		}

		absDir, err := filepath.Abs(dir)
		if err != nil {
			return fmt.Errorf("resolve dir: %w", err)
		}

		violations, err := typescript.Scan(absDir)
		if err != nil {
			return fmt.Errorf("scan: %w", err)
		}

		if scanTsCfg.format == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			type jsonViolation struct {
				File        string `json:"file"`
				Line        int    `json:"line"`
				InvariantID string `json:"invariant_id"`
				Detail      string `json:"detail"`
			}
			out := make([]jsonViolation, 0, len(violations))
			for _, v := range violations {
				rel, _ := filepath.Rel(absDir, v.File)
				out = append(out, jsonViolation{
					File:        rel,
					Line:        v.Line,
					InvariantID: v.InvariantID,
					Detail:      v.Detail,
				})
			}
			return enc.Encode(out)
		}

		// text format
		if len(violations) == 0 {
			fmt.Fprintln(os.Stdout, "scan-ts: no violations found ✓")
			return nil
		}
		fmt.Fprintf(os.Stderr, "scan-ts: %d violation(s) found\n\n", len(violations))
		for _, v := range violations {
			rel, _ := filepath.Rel(absDir, v.File)
			fmt.Fprintf(os.Stderr, "  %s:%d  [%s]  %s\n", rel, v.Line, v.InvariantID, v.Detail)
		}
		fmt.Fprintln(os.Stderr)
		os.Exit(1)
		return nil
	},
}

func init() {
	awarenessScanTsCmd.Flags().StringVar(&scanTsCfg.dir, "dir", "", "Directory to scan (default: repo root)")
	awarenessScanTsCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root (used when --dir is not set)")
	awarenessScanTsCmd.Flags().StringVar(&scanTsCfg.format, "format", "text", "Output format: text or json")

	awarenessCmd.AddCommand(awarenessScanTsCmd)
}
