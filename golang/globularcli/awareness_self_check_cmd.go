package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/globulario/awareness/selfcheck"
)

var selfCheckCfg = struct {
	format             string
	createIncident     bool
	strict             bool
	maxTopWarningGroup int
}{}

var awarenessSelfCheckCmd = &cobra.Command{
	Use:   "self-check",
	Short: "Run the awareness system against itself — detect false silences, noise, and MCP safety regressions",
	Long: `self-check evaluates the awareness system's own precision and safety:

  - Runs awareness build staleness check
  - Runs enforcement audit (contracts, annotations, required tests)
  - Runs annotation coverage
  - Runs graph drift detection
  - Runs 5 preflight smoke cases (false silence detection)
  - Runs node-context, semantic path, debug-session, check-edit, and preflight-audit smokes
  - Checks MCP tool discovery for promotion safety (awareness.mcp_must_not_expose_promotion)

Safety contract:
  - Never mutates approved awareness YAML (invariants.yaml, failure_modes.yaml, etc.)
  - Never generates a proposal automatically
  - Never approves or promotes anything
  - --create-incident writes evidence only; no proposal is generated

Examples:

  globular awareness self-check --format agent

  globular awareness self-check --format markdown --create-incident

  globular awareness self-check --strict`,

	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}
		docsDir := filepath.Join(repoRoot, "docs", "awareness")

		dbPath := awareCfg.dbPath
		if dbPath == "" {
			dbPath = resolveAwarenessDBPath(repoRoot)
		}

		// Open graph — non-fatal; self-check degrades gracefully.
		g, graphErr := openAwarenessGraph(dbPath, awareCfg.repoPath)
		if graphErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not open awareness graph (%v)\n", graphErr)
			fmt.Fprintf(os.Stderr, "  run 'globular awareness build' to build the graph\n")
		} else {
			defer g.Close()
		}

		opts := selfcheck.Options{
			RepoPath:           repoRoot,
			DocsDir:            docsDir,
			DBPath:             dbPath,
			Strict:             selfCheckCfg.strict,
			MaxTopWarningGroup: selfCheckCfg.maxTopWarningGroup,
		}

		r, err := selfcheck.Run(ctx, opts, g)
		if err != nil {
			return fmt.Errorf("self-check: %w", err)
		}

		format := selfcheck.Format(selfCheckCfg.format)
		if format == "" {
			format = selfcheck.FormatMarkdown
		}

		out, err := selfcheck.Render(r, format)
		if err != nil {
			return fmt.Errorf("render self-check: %w", err)
		}
		fmt.Fprint(os.Stdout, out)

		if selfCheckCfg.createIncident && r.ShouldCreateIncident {
			path, err := selfcheck.CreateIncidentBundle(r, docsDir)
			if err != nil {
				return fmt.Errorf("create incident: %w", err)
			}
			fmt.Fprintf(os.Stderr, "\nIncident bundle written: %s\n", path)
			fmt.Fprintf(os.Stderr, "Review findings, then run:\n")
			fmt.Fprintf(os.Stderr, "  globular awareness propose-from-incident %s\n",
				incidentIDFromPath(path))
		}

		if r.StrictFail {
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	awarenessSelfCheckCmd.Flags().StringVar(&selfCheckCfg.format, "format", "markdown",
		"Output format: markdown | json | agent")
	awarenessSelfCheckCmd.Flags().BoolVar(&selfCheckCfg.createIncident, "create-incident", false,
		"Write an incident bundle to docs/awareness/incidents/ when checks fail (no proposal generated)")
	awarenessSelfCheckCmd.Flags().BoolVar(&selfCheckCfg.strict, "strict", false,
		"Exit non-zero if any check fails")
	awarenessSelfCheckCmd.Flags().IntVar(&selfCheckCfg.maxTopWarningGroup, "max-top-warning-group", -1,
		"Fail strict self-check if the largest audit warning group exceeds this count (-1 disables)")
	awarenessSelfCheckCmd.Flags().StringVar(&awareCfg.dbPath, "db", "",
		"Path to graph.db (default: .globular/awareness/graph.db)")
	awarenessSelfCheckCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "",
		"Repo root (default: auto-detected from git)")

	awarenessCmd.AddCommand(awarenessSelfCheckCmd)
}

// incidentIDFromPath extracts the incident ID from a full file path.
func incidentIDFromPath(path string) string {
	base := filepath.Base(path)
	if len(base) > 5 {
		return base[:len(base)-5] // strip .yaml
	}
	return base
}
