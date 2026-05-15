package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/preflight"
	"github.com/globulario/services/golang/awareness/runtime"
	"github.com/globulario/services/golang/awareness/semanticdiff"
)

var preflightCfg = struct {
	task                 string
	files                []string
	packagePath          string
	phase                string
	format               string
	verbosity            string
	budget               string
	includeRuntime       bool
	runtimeWindow        time.Duration
	writeAudit           bool
	gitSHA               string
	semanticDiff         bool
	diffFile             string
	requireSemanticClean bool
	bundleManifestPath   string
	selfHeal             bool
}{}

var awarenessPreflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Run a full architecture preflight before editing Globular code",
	Long: `preflight is the front door for AI agents before editing Globular code.

It composes all awareness capabilities — agent context, impact analysis,
fix-ledger, package admission, cycle detection — into a single deterministic
report with explicit instruction.

Examples:

  globular awareness preflight --task "desired_hash mismatch after deploy" --format agent

  globular awareness preflight \
    --task "envoy restart storm" \
    --file golang/cluster_controller/convergence.go \
    --phase recovery \
    --format markdown

  globular awareness preflight \
    --task "add new package" \
    --package /path/to/package \
    --format json`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if preflightCfg.task == "" {
			return fmt.Errorf("--task is required")
		}

		ctx := context.Background()

		// Resolve repo root and docs dir.
		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}
		docsDir := filepath.Join(repoRoot, "docs", "awareness")

		// Open graph — non-fatal if missing (preflight degrades gracefully).
		dbPath := awareCfg.dbPath
		if dbPath == "" {
			dbPath = filepath.Join(repoRoot, ".globular", "awareness", "graph.db")
		}
		opts := preflight.Options{
			Task:               preflightCfg.task,
			Files:              preflightCfg.files,
			PackagePath:        preflightCfg.packagePath,
			Phase:              preflightCfg.phase,
			DocsDir:            docsDir,
			IncludeRuntime:     preflightCfg.includeRuntime,
			RuntimeWindow:      preflightCfg.runtimeWindow,
			WriteAudit:         preflightCfg.writeAudit,
			GitSHA:             preflightCfg.gitSHA,
			BundleManifestPath: preflightCfg.bundleManifestPath,
		}

		if preflightCfg.includeRuntime {
			opts.Bridge = runtime.NewBridge("", "")
		}

		runOnce := func() (*preflight.Report, error) {
			g, graphErr := openAwarenessGraph(dbPath, awareCfg.repoPath)
			if graphErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not open awareness graph (%v)\n", graphErr)
				fmt.Fprintf(os.Stderr, "  run 'globular awareness build' to build the graph\n")
			} else {
				defer g.Close()
			}
			r, err := preflight.Run(ctx, opts, g)
			if err != nil {
				return nil, fmt.Errorf("preflight: %w", err)
			}
			return r, nil
		}

		r, err := runOnce()
		if err != nil {
			return err
		}

		if preflightCfg.selfHeal && preflightNeedsSelfHeal(r) {
			fmt.Fprintln(os.Stderr, "awareness preflight self-heal: stale/unknown safety detected; rebuilding graph and retrying preflight")
			if healErr := runAwarenessSelfHealBuild(ctx, repoRoot, dbPath); healErr != nil {
				fmt.Fprintf(os.Stderr, "warning: awareness preflight self-heal failed: %v\n", healErr)
			} else {
				r2, rerunErr := runOnce()
				if rerunErr != nil {
					fmt.Fprintf(os.Stderr, "warning: awareness preflight rerun after self-heal failed: %v\n", rerunErr)
				} else {
					r = r2
				}
			}
		}

		format := preflight.Format(preflightCfg.format)
		if format == "" {
			format = preflight.FormatMarkdown
		}

		out, err := preflight.RenderWithOptions(r, format, preflight.RenderOptions{
			Verbosity: preflight.Verbosity(preflightCfg.verbosity),
			Budget:    preflight.Budget(preflightCfg.budget),
		})
		if err != nil {
			return fmt.Errorf("render preflight: %w", err)
		}

		fmt.Fprint(os.Stdout, out)

		// Semantic diff overlay — appended after static preflight output.
		if preflightCfg.semanticDiff || preflightCfg.diffFile != "" {
			diffText, diffErr := preflightLoadDiff(preflightCfg.diffFile)
			if diffErr != nil {
				fmt.Fprintf(os.Stderr, "warning: semantic diff skipped (%v)\n", diffErr)
			} else if diffText != "" {
				sdReq := semanticdiff.SemanticDiffRequest{
					Task:         preflightCfg.task,
					DiffText:     diffText,
					DiffSource:   "preflight",
					RequireClean: preflightCfg.requireSemanticClean,
				}
				sdReport, sdErr := semanticdiff.InterpretSemanticDiff(ctx, sdReq)
				if sdErr != nil {
					fmt.Fprintf(os.Stderr, "warning: semantic diff error: %v\n", sdErr)
				} else {
					fmt.Fprintln(os.Stdout)
					fmt.Fprint(os.Stdout, semanticdiff.FormatReport(sdReport))
					if preflightCfg.requireSemanticClean && sdReport.Verdict == semanticdiff.VerdictBlock {
						return fmt.Errorf("semantic diff blocked: %s", sdReport.Summary)
					}
				}
			}
		}

		return nil
	},
}

// preflightLoadDiff returns the diff text from a file path, stdin (-), or git diff HEAD.
func preflightLoadDiff(diffFile string) (string, error) {
	if diffFile == "-" {
		raw, err := io.ReadAll(os.Stdin)
		return strings.TrimSpace(string(raw)), err
	}
	if diffFile != "" {
		raw, err := os.ReadFile(diffFile)
		return strings.TrimSpace(string(raw)), err
	}
	// No file specified — run git diff HEAD.
	out, err := exec.Command("git", "diff", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("git diff HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func init() {
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.task, "task", "", "Task description (required)")
	awarenessPreflightCmd.Flags().StringArrayVar(&preflightCfg.files, "file", nil, "File(s) to run impact analysis on (repeatable)")
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.packagePath, "package", "", "Path to package directory with awareness.yaml")
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.phase, "phase", "", "Dependency phase for cycle detection (e.g. recovery, bootstrap, package_install)")
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.format, "format", "markdown", "Output format: markdown | json | agent")
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.verbosity, "verbosity", "standard", "Agent output verbosity: compact | standard | full (overridden by --budget)")
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.budget, "budget", "", "Token budget: compact | standard | deep | forensic (overrides --verbosity)")
	awarenessPreflightCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db (default: .globular/awareness/graph.db)")
	awarenessPreflightCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root (default: auto-detected from git)")
	awarenessPreflightCmd.Flags().BoolVar(&preflightCfg.includeRuntime, "include-runtime", false, "Collect live runtime snapshot and merge into preflight report")
	awarenessPreflightCmd.Flags().DurationVar(&preflightCfg.runtimeWindow, "runtime-window", 15*time.Minute, "Lookback window for runtime events/workflows")
	awarenessPreflightCmd.Flags().BoolVar(&preflightCfg.writeAudit, "write-audit", false, "Persist a preflight audit record to the graph DB after the run")
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.gitSHA, "git-sha", "", "Current git SHA for the audit record (used with --write-audit)")
	awarenessPreflightCmd.Flags().BoolVar(&preflightCfg.semanticDiff, "semantic-diff", false, "Run semantic diff on 'git diff HEAD' and append to preflight output")
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.diffFile, "diff-file", "", "Path to unified diff file for semantic interpretation (or - for stdin)")
	awarenessPreflightCmd.Flags().BoolVar(&preflightCfg.requireSemanticClean, "require-semantic-clean", false, "Exit non-zero if semantic diff verdict is block")
	awarenessPreflightCmd.Flags().StringVar(&preflightCfg.bundleManifestPath, "bundle-manifest", "", "Path to the installed awareness-bundle manifest.json (default: /var/lib/globular/awareness/current/manifest.json if it exists)")
	awarenessPreflightCmd.Flags().BoolVar(&preflightCfg.selfHeal, "self-heal", true, "When preflight is UNKNOWN_NOT_SAFE or stale, run awareness build --clean and retry once")

	awarenessCmd.AddCommand(awarenessPreflightCmd)
}
