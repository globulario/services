package main

// awareness_cmds.go: CLI commands for the Globular awareness graph.
//
// Usage:
//
//	globular awareness build [--repo <path>] [--db <path>]
//	globular awareness stats [--db <path>]
//	globular awareness impact --file <path> [--db <path>]
//	globular awareness agent-context --task "<task>" [--db <path>]
//	globular awareness cycles [--phase <phase>] [--db <path>]
//
// The awareness graph is purely local — no cluster connection required.
// DB default: .globular/awareness/graph.db (relative to repo root).

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/analysis"
	"github.com/globulario/services/golang/awareness/extractors/docs"
	"github.com/globulario/services/golang/awareness/extractors/goast"
	"github.com/globulario/services/golang/awareness/extractors/manual"
	"github.com/globulario/services/golang/awareness/extractors/packages"
	"github.com/globulario/services/golang/awareness/extractors/proto"
	"github.com/globulario/services/golang/awareness/extractors/tests"
	"github.com/globulario/services/golang/awareness/extractors/workflows"
	"github.com/globulario/services/golang/awareness/graph"
)

var awareCfg = struct {
	dbPath      string
	repoPath    string
	file        string
	task        string
	phase       string
	packagePath string
	commit      bool
}{}

var awarenessCmd = &cobra.Command{
	Use:   "awareness",
	Short: "Awareness graph — architectural context for AI agents",
	Long: `The awareness graph connects source code, invariants, failure modes, and
services into a queryable SQLite graph that gives AI agents architectural
context before they edit code or suggest fixes.

It is a local, offline tool. No cluster connection is required.`,
}

var awarenessBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build (or rebuild) the awareness graph from source and YAML truth files",
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

		fmt.Fprintf(os.Stdout, "Building awareness graph\n")
		fmt.Fprintf(os.Stdout, "  repo: %s\n", repoRoot)
		fmt.Fprintf(os.Stdout, "  db:   %s\n\n", dbPath)

		g, err := graph.Open(dbPath)
		if err != nil {
			return fmt.Errorf("open graph: %w", err)
		}
		defer g.Close()

		docsDir := filepath.Join(repoRoot, "docs", "awareness")

		// Load manual truth files.
		fmt.Fprintf(os.Stdout, "Loading manual truth files from %s ...\n", docsDir)
		if err := manual.LoadAll(ctx, g, docsDir); err != nil {
			return fmt.Errorf("manual loader: %w", err)
		}

		golangDir := filepath.Join(repoRoot, "golang")

		// Go AST extractor — paths stored relative to repoRoot.
		fmt.Fprintf(os.Stdout, "Extracting Go source ...\n")
		if err := goast.Extract(ctx, g, golangDir, repoRoot); err != nil {
			fmt.Fprintf(os.Stderr, "warning: Go extractor: %v\n", err)
		}

		// Test extractor — paths stored relative to repoRoot.
		fmt.Fprintf(os.Stdout, "Extracting Go tests ...\n")
		if err := tests.Extract(ctx, g, golangDir, repoRoot); err != nil {
			fmt.Fprintf(os.Stderr, "warning: test extractor: %v\n", err)
		}

		// Proto extractor — paths stored relative to repoRoot.
		fmt.Fprintf(os.Stdout, "Extracting proto files ...\n")
		protoDir := filepath.Join(repoRoot, "proto")
		if err := proto.Extract(ctx, g, protoDir, repoRoot); err != nil {
			fmt.Fprintf(os.Stderr, "warning: proto extractor: %v\n", err)
		}

		// Workflow extractor.
		fmt.Fprintf(os.Stdout, "Extracting workflow definitions ...\n")
		if err := workflows.Extract(ctx, g, repoRoot); err != nil {
			fmt.Fprintf(os.Stderr, "warning: workflow extractor: %v\n", err)
		}

		// Package extractor.
		fmt.Fprintf(os.Stdout, "Extracting package manifests ...\n")
		if err := packages.Extract(ctx, g, repoRoot); err != nil {
			fmt.Fprintf(os.Stderr, "warning: package extractor: %v\n", err)
		}

		// Docs / design decision extractor.
		fmt.Fprintf(os.Stdout, "Extracting documentation and design decisions ...\n")
		if warnings, err := docs.Extract(ctx, g, repoRoot); err != nil {
			fmt.Fprintf(os.Stderr, "warning: docs extractor: %v\n", err)
		} else {
			for _, w := range warnings {
				fmt.Fprintf(os.Stderr, "warning: docs extractor: %s\n", w)
			}
		}

		// Prune stale source_file nodes (files that no longer exist on disk).
		if pruned, pruneErr := g.PruneStaleSourceFileNodes(ctx, repoRoot); pruneErr != nil {
			fmt.Fprintf(os.Stderr, "warning: prune stale nodes: %v\n", pruneErr)
		} else if pruned > 0 {
			fmt.Fprintf(os.Stdout, "Pruned %d stale source_file nodes\n", pruned)
		}

		// Record the build.
		gitCommit := gitHead(repoRoot)
		stats, err := g.Stats(ctx)
		if err != nil {
			return err
		}
		buildID := fmt.Sprintf("build-%d", time.Now().Unix())
		if err := g.UpsertBuildRecord(ctx, buildID, repoRoot, gitCommit, "", stats); err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "\nBuild complete:\n")
		fmt.Fprintf(os.Stdout, "  nodes:        %d\n", stats.Nodes)
		fmt.Fprintf(os.Stdout, "  edges:        %d\n", stats.Edges)
		fmt.Fprintf(os.Stdout, "  invariants:   %d\n", stats.Invariants)
		fmt.Fprintf(os.Stdout, "  failure modes: %d\n", stats.FailureModes)
		return nil
	},
}

var awarenessStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Print graph statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		stats, err := g.Stats(ctx)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "nodes:         %d\n", stats.Nodes)
		fmt.Fprintf(os.Stdout, "edges:         %d\n", stats.Edges)
		fmt.Fprintf(os.Stdout, "invariants:    %d\n", stats.Invariants)
		fmt.Fprintf(os.Stdout, "failure modes: %d\n", stats.FailureModes)
		return nil
	},
}

var awarenessImpactCmd = &cobra.Command{
	Use:   "impact",
	Short: "Show what is impacted by changes to a file",
	RunE: func(cmd *cobra.Command, args []string) error {
		if awareCfg.file == "" {
			return fmt.Errorf("--file is required")
		}
		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		result, err := analysis.ImpactByFile(ctx, g, awareCfg.file)
		if err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "Impact analysis: %s\n\n", awareCfg.file)

		if result.SourceFile == nil {
			fmt.Fprintf(os.Stdout, "No source_file node found for this path.\n")
			fmt.Fprintf(os.Stdout, "Run 'globular awareness build' first.\n")
			return nil
		}

		printSection := func(label string, nodes []*graph.Node) {
			if len(nodes) == 0 {
				return
			}
			fmt.Fprintf(os.Stdout, "%s:\n", label)
			for _, n := range nodes {
				fmt.Fprintf(os.Stdout, "  - %s\n", n.Name)
			}
			fmt.Fprintln(os.Stdout)
		}

		printSection("Impacted services", result.Services)
		printSection("Impacted invariants", result.Invariants)
		printSection("Impacted failure modes", result.FailureModes)
		printSection("Forbidden fixes", result.ForbiddenFixes)
		printSection("Required tests", result.Tests)
		printSection("Other nodes", result.Other)
		return nil
	},
}

var awarenessAgentContextCmd = &cobra.Command{
	Use:   "agent-context",
	Short: "Generate architectural context for an AI agent task",
	RunE: func(cmd *cobra.Command, args []string) error {
		if awareCfg.task == "" {
			return fmt.Errorf("--task is required")
		}
		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		// Load context aliases to enrich matching with natural-language phrases.
		repoRoot, _ := resolveRepoRoot(awareCfg.repoPath)
		aliasMap := loadAliasesQuiet(repoRoot)

		md, _, err := analysis.GenerateAgentContext(ctx, g, awareCfg.task, analysis.AgentContextHints{},
			analysis.AgentContextAliases(aliasMap))
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, md)
		return nil
	},
}

var awarenessCyclesCmd = &cobra.Command{
	Use:   "cycles",
	Short: "Detect dependency cycles in the graph",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		phase := awareCfg.phase
		cycles, err := analysis.FindCycles(ctx, g, phase)
		if err != nil {
			return err
		}

		if len(cycles) == 0 {
			if phase != "" {
				fmt.Fprintf(os.Stdout, "No dependency cycles found for phase %q.\n", phase)
			} else {
				fmt.Fprintf(os.Stdout, "No dependency cycles found.\n")
			}
			return nil
		}

		fmt.Fprintf(os.Stdout, "Dependency cycles found: %d\n\n", len(cycles))
		for i, c := range cycles {
			fmt.Fprintf(os.Stdout, "--- Cycle %d [%s] ---\n", i+1, c.Classification)
			fmt.Fprintf(os.Stdout, "  Phase:    %s\n", c.Phase)
			fmt.Fprintf(os.Stdout, "  Required: %v\n", c.AllRequired)
			fmt.Fprintf(os.Stdout, "  Path:     %s\n", strings.Join(c.Path, " → "))
			fmt.Fprintf(os.Stdout, "  Reason:   %s\n\n", c.Reason)
		}
		return nil
	},
}

var awarenessValidatePackageCmd = &cobra.Command{
	Use:   "validate-package",
	Short: "Validate a package's awareness.yaml against admission rules",
	Long: `Validates a package's awareness contract against the current awareness graph.
Prints an ADMIT / WARN / BLOCK result with full rule-by-rule reasoning.

This command is read-only — it never modifies the graph.
Use 'admit-package --commit' to write the contract into the graph.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if awareCfg.packagePath == "" {
			return fmt.Errorf("--path is required")
		}
		ctx := context.Background()

		contract, err := packages.LoadAwarenessContract(awareCfg.packagePath)
		if err != nil {
			return fmt.Errorf("load awareness contract: %w", err)
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		packageKind := ""
		if contract != nil {
			packageKind = contract.PackageKind
		}

		result, err := analysis.ValidatePackage(ctx, contract, packageKind, g)
		if err != nil {
			return fmt.Errorf("validate: %w", err)
		}

		fmt.Fprint(os.Stdout, analysis.RenderAdmissionMarkdown(contract, result))

		if result.Status == analysis.AdmissionBlock {
			os.Exit(1)
		}
		return nil
	},
}

var awarenessPackageContextCmd = &cobra.Command{
	Use:   "package-context",
	Short: "Generate architectural context for a package from its awareness contract",
	Long: `Loads a package's awareness.yaml and queries the awareness graph for all
invariants, failure modes, and forbidden fixes that relate to the package's
declared services, etcd keys, and dependencies.

Output is Markdown suitable for pasting into an AI agent prompt.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if awareCfg.packagePath == "" {
			return fmt.Errorf("--path is required")
		}
		ctx := context.Background()

		contract, err := packages.LoadAwarenessContract(awareCfg.packagePath)
		if err != nil {
			return fmt.Errorf("load awareness contract: %w", err)
		}
		if contract == nil {
			return fmt.Errorf("no awareness.yaml found in %s", awareCfg.packagePath)
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		// Build a task description from the contract identity.
		task := fmt.Sprintf("package %s service %s kind %s: %s", contract.Package, contract.Service, contract.PackageKind, contract.Summary)

		// Populate hints from the contract.
		hints := analysis.AgentContextHints{
			Services: []string{contract.Service},
		}
		for _, dep := range contract.DependsOn {
			hints.Services = append(hints.Services, dep.Service)
		}

		md, _, err := analysis.GenerateAgentContext(ctx, g, task, hints)
		if err != nil {
			return err
		}
		fmt.Fprint(os.Stdout, md)
		return nil
	},
}

var awarenessAdmitPackageCmd = &cobra.Command{
	Use:   "admit-package",
	Short: "Validate and optionally commit a package's awareness contract to the graph",
	Long: `Runs the full admission ruleset against a package's awareness.yaml.
Prints an ADMIT / WARN / BLOCK result.

With --commit: if the result is ADMIT or WARN, the contract's nodes and edges
are written into the main awareness graph so they participate in future
cycle detection, impact analysis, and agent context generation.

BLOCK always exits with code 1. WARN exits with code 0 but prints warnings.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if awareCfg.packagePath == "" {
			return fmt.Errorf("--path is required")
		}
		ctx := context.Background()

		contract, err := packages.LoadAwarenessContract(awareCfg.packagePath)
		if err != nil {
			return fmt.Errorf("load awareness contract: %w", err)
		}

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		packageKind := ""
		if contract != nil {
			packageKind = contract.PackageKind
		}

		result, err := analysis.ValidatePackage(ctx, contract, packageKind, g)
		if err != nil {
			return fmt.Errorf("validate: %w", err)
		}

		fmt.Fprint(os.Stdout, analysis.RenderAdmissionMarkdown(contract, result))

		if result.Status == analysis.AdmissionBlock {
			fmt.Fprintf(os.Stderr, "\nBLOCKED — contract not committed.\n")
			os.Exit(1)
		}

		if awareCfg.commit && contract != nil {
			if err := packages.AddContractToGraph(ctx, g, contract); err != nil {
				return fmt.Errorf("commit contract to graph: %w", err)
			}
			fmt.Fprintf(os.Stdout, "\nContract committed to graph (%d nodes, %d edges).\n",
				len(result.GraphNodesAddedPreview), len(result.GraphEdgesAddedPreview))
		} else if awareCfg.commit && contract == nil {
			fmt.Fprintf(os.Stdout, "\nNo contract to commit (awareness.yaml not found).\n")
		}

		return nil
	},
}

func init() {
	// Build command flags.
	awarenessBuildCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db (default: .globular/awareness/graph.db in repo root)")
	awarenessBuildCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root (default: auto-detected from git)")

	// Stats flags.
	awarenessStatsCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessStatsCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// Impact flags.
	awarenessImpactCmd.Flags().StringVar(&awareCfg.file, "file", "", "File path to analyse (relative to repo root)")
	awarenessImpactCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessImpactCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// Agent-context flags.
	awarenessAgentContextCmd.Flags().StringVar(&awareCfg.task, "task", "", "Task description for the AI agent")
	awarenessAgentContextCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessAgentContextCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// Cycles flags.
	awarenessCyclesCmd.Flags().StringVar(&awareCfg.phase, "phase", "", "Filter by dependency phase (e.g. recovery, bootstrap, package_install)")
	awarenessCyclesCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessCyclesCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// validate-package flags.
	awarenessValidatePackageCmd.Flags().StringVar(&awareCfg.packagePath, "path", "", "Path to the package directory containing awareness.yaml")
	awarenessValidatePackageCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessValidatePackageCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// package-context flags.
	awarenessPackageContextCmd.Flags().StringVar(&awareCfg.packagePath, "path", "", "Path to the package directory containing awareness.yaml")
	awarenessPackageContextCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessPackageContextCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// admit-package flags.
	awarenessAdmitPackageCmd.Flags().StringVar(&awareCfg.packagePath, "path", "", "Path to the package directory containing awareness.yaml")
	awarenessAdmitPackageCmd.Flags().BoolVar(&awareCfg.commit, "commit", false, "Commit the contract to the main graph if ADMIT or WARN")
	awarenessAdmitPackageCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessAdmitPackageCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// Register subcommands.
	awarenessCmd.AddCommand(awarenessBuildCmd)
	awarenessCmd.AddCommand(awarenessStatsCmd)
	awarenessCmd.AddCommand(awarenessImpactCmd)
	awarenessCmd.AddCommand(awarenessAgentContextCmd)
	awarenessCmd.AddCommand(awarenessCyclesCmd)
	awarenessCmd.AddCommand(awarenessValidatePackageCmd)
	awarenessCmd.AddCommand(awarenessPackageContextCmd)
	awarenessCmd.AddCommand(awarenessAdmitPackageCmd)

	// Register top-level command.
	rootCmd.AddCommand(awarenessCmd)
}

// openAwarenessGraph opens the graph DB using the given path or the default location.
func openAwarenessGraph(dbPath, repoPath string) (*graph.Graph, error) {
	if dbPath == "" {
		repoRoot, err := resolveRepoRoot(repoPath)
		if err != nil {
			return nil, err
		}
		dbPath = filepath.Join(repoRoot, ".globular", "awareness", "graph.db")
	}
	g, err := graph.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open graph %s: %w", dbPath, err)
	}
	return g, nil
}

// resolveRepoRoot returns the given path or auto-detects the git root.
func resolveRepoRoot(given string) (string, error) {
	if given != "" {
		return given, nil
	}
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		// Fall back to current directory.
		return os.Getwd()
	}
	return strings.TrimSpace(string(out)), nil
}

// gitHead returns the current HEAD commit hash, or "" on error.
func gitHead(repoRoot string) string {
	cmd := exec.Command("git", "-C", repoRoot, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
