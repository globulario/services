package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/graph"
)

var preflightAuditCfg = struct {
	since  time.Duration
	gitSHA string
	format string
}{}

var awarenessPreflightAuditCmd = &cobra.Command{
	Use:   "preflight-audit",
	Short: "Query the durable preflight audit log",
	Long: `preflight-audit queries the preflight_audits table in the awareness graph.

Each record captures what invariants, forbidden fixes, and code smells were
surfaced to the agent for a given task and git SHA. This provides an audit trail
of awareness checks performed before code edits.

Examples:

  globular awareness preflight-audit

  globular awareness preflight-audit --since 24h

  globular awareness preflight-audit --git-sha abc123

  globular awareness preflight-audit --format json`,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		dbPath := awareCfg.dbPath
		if dbPath == "" {
			dbPath = filepath.Join(repoRoot, ".globular", "awareness", "graph.db")
		}

		g, err := openAwarenessGraph(dbPath, awareCfg.repoPath)
		if err != nil {
			return fmt.Errorf("open awareness graph: %w", err)
		}
		defer g.Close()

		var since int64
		if preflightAuditCfg.since > 0 {
			since = time.Now().Add(-preflightAuditCfg.since).Unix()
		}

		records, err := g.QueryPreflightAudits(ctx, since, preflightAuditCfg.gitSHA)
		if err != nil {
			return fmt.Errorf("query preflight audits: %w", err)
		}

		if len(records) == 0 {
			fmt.Fprintln(os.Stdout, "No preflight audit records found.")
			return nil
		}

		switch strings.ToLower(preflightAuditCfg.format) {
		case "json":
			return renderAuditJSON(records)
		default:
			return renderAuditMarkdown(records)
		}
	},
}

func renderAuditMarkdown(records []*graph.PreflightAuditRecord) error {
	fmt.Fprintf(os.Stdout, "# Preflight Audit Log (%d records)\n\n", len(records))
	for _, r := range records {
		ts := time.Unix(r.Timestamp, 0).Format("2006-01-02 15:04:05")
		fmt.Fprintf(os.Stdout, "## %s\n\n", r.ID)
		fmt.Fprintf(os.Stdout, "- **Task:** %s\n", r.Task)
		fmt.Fprintf(os.Stdout, "- **Time:** %s\n", ts)
		if r.GitSHA != "" {
			fmt.Fprintf(os.Stdout, "- **Git SHA:** %s\n", r.GitSHA)
		}
		if len(r.Files) > 0 {
			fmt.Fprintf(os.Stdout, "- **Files:** %s\n", strings.Join(r.Files, ", "))
		}
		if len(r.Invariants) > 0 {
			fmt.Fprintf(os.Stdout, "- **Invariants:** %s\n", strings.Join(r.Invariants, ", "))
		}
		if len(r.ForbiddenFixes) > 0 {
			fmt.Fprintf(os.Stdout, "- **Forbidden fixes:** %s\n", strings.Join(r.ForbiddenFixes, ", "))
		}
		if len(r.CodeSmells) > 0 {
			fmt.Fprintf(os.Stdout, "- **Code smells:** %s\n", strings.Join(r.CodeSmells, ", "))
		}
		fmt.Fprintln(os.Stdout)
	}
	return nil
}

func renderAuditJSON(records []*graph.PreflightAuditRecord) error {
	b, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal audit records: %w", err)
	}
	fmt.Fprintln(os.Stdout, string(b))
	return nil
}

func init() {
	awarenessPreflightAuditCmd.Flags().DurationVar(&preflightAuditCfg.since, "since", 0, "Only return records newer than this duration (e.g. 24h, 7d)")
	awarenessPreflightAuditCmd.Flags().StringVar(&preflightAuditCfg.gitSHA, "git-sha", "", "Filter records by git SHA")
	awarenessPreflightAuditCmd.Flags().StringVar(&preflightAuditCfg.format, "format", "markdown", "Output format: markdown | json")
	awarenessPreflightAuditCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db (default: .globular/awareness/graph.db)")
	awarenessPreflightAuditCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root (default: auto-detected from git)")

	awarenessCmd.AddCommand(awarenessPreflightAuditCmd)
}
