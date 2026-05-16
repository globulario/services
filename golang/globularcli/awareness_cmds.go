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
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"

	"github.com/globulario/awareness/analysis"
	"github.com/globulario/awareness/extractors/clusterspec"
	"github.com/globulario/awareness/extractors/clusterstate"
	"github.com/globulario/awareness/extractors/docs"
	"github.com/globulario/awareness/extractors/doctor"
	"github.com/globulario/awareness/extractors/goast"
	"github.com/globulario/awareness/extractors/manual"
	"github.com/globulario/awareness/extractors/metrics"
	"github.com/globulario/awareness/extractors/packages"
	"github.com/globulario/awareness/extractors/pki"
	"github.com/globulario/awareness/extractors/proto"
	"github.com/globulario/awareness/extractors/rbac"
	"github.com/globulario/awareness/extractors/scripts"
	"github.com/globulario/awareness/extractors/tests"
	"github.com/globulario/awareness/extractors/workflows"
	"github.com/globulario/awareness/extractors/workflowstate"
	"github.com/globulario/awareness/graph"
	"github.com/globulario/services/golang/config"
)

var awareCfg = struct {
	dbPath          string
	repoPath        string
	file            string
	task            string
	phase           string
	packagePath     string
	commit          bool
	explain         bool
	cleanBuild      bool
	packagesMetaDir string
	extraScriptRoots []string
	collectSystemd      bool
	collectVarLib       bool
	collectEtcd         bool
	collectConvergence  bool
	collectMetrics      bool
	collectPKI          bool
	collectRBAC         bool
	collectWorkflow     bool
	workflowAddr        string
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
			dbPath = resolveAwarenessDBPath(repoRoot)
		}

		fmt.Fprintf(os.Stdout, "Building awareness graph\n")
		fmt.Fprintf(os.Stdout, "  repo: %s\n", repoRoot)
		fmt.Fprintf(os.Stdout, "  db:   %s\n\n", dbPath)

		// --clean removes stale edges from incremental upsert builds.
		// Must remove main db AND the WAL/SHM files — SQLite will fail to open
		// a new db if orphaned WAL/SHM files remain from the previous session.
		if awareCfg.cleanBuild {
			for _, suffix := range []string{"", "-wal", "-shm"} {
				p := dbPath + suffix
				if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("clean graph: remove %s: %w", p, err)
				}
			}
			fmt.Fprintf(os.Stdout, "Cleaned previous graph.\n")
		}

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

		// Doctor rule extractor — surfaces every cluster_doctor rule as a
		// detector node, and joins them to failure_modes via
		// docs/awareness/detector_mapping.yaml so failure_modes can flip from
		// TESTED to DETECTED in the assurance coverage report.
		fmt.Fprintf(os.Stdout, "Extracting cluster_doctor rules ...\n")
		rulesDir := filepath.Join(repoRoot, "golang", "cluster_doctor", "cluster_doctor_server", "rules")
		mappingPath := filepath.Join(repoRoot, "docs", "awareness", "detector_mapping.yaml")
		if dr, err := doctor.Extract(ctx, g, rulesDir, repoRoot, mappingPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: doctor extractor: %v\n", err)
		} else {
			fmt.Fprintf(os.Stdout, "  doctor: extracted %d rules, applied %d mappings\n",
				len(dr.Rules), dr.MappingsApplied)
			for _, skipped := range dr.MappingsSkipped {
				fmt.Fprintf(os.Stderr, "warning: doctor mapping skipped: %s\n", skipped)
			}
		}

		// collectorHealthItems accumulates the outcome of every P0/P1 collector run.
		var collectorHealthItems []graph.CollectorHealthItem
		addHealth := func(id, sourceTier, status string, nodes int, errStr, priority string) {
			collectorHealthItems = append(collectorHealthItems, graph.CollectorHealthItem{
				CollectorID:  id,
				SourceTier:   sourceTier,
				Status:       status,
				NodesEmitted: nodes,
				Error:        errStr,
				Priority:     priority,
			})
		}

		// Package spec indexer — reads packages metadata repo (package.json + awareness.yaml).
		if awareCfg.packagesMetaDir != "" {
			fmt.Fprintf(os.Stdout, "Extracting package specs from %s ...\n", awareCfg.packagesMetaDir)
			h, err := clusterspec.Extract(ctx, g, awareCfg.packagesMetaDir)
			errStr := ""
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: clusterspec extractor: %v\n", err)
				errStr = err.Error()
			} else {
				fmt.Fprintf(os.Stdout, "  clusterspec: status=%s nodes=%d\n", h.Status, h.NodesEmitted)
			}
			addHealth("clusterspec", "package_spec", h.Status, h.NodesEmitted, errStr, "P0")
		}

		// Shell script crawler — indexes extra repos (installer scripts, Makefile, etc.).
		if len(awareCfg.extraScriptRoots) > 0 {
			fmt.Fprintf(os.Stdout, "Crawling extra script repos ...\n")
			var roots []scripts.RepoRoot
			for _, r := range awareCfg.extraScriptRoots {
				roots = append(roots, scripts.RepoRoot{Path: r, SourceTier: "installer_script"})
			}
			healths, err := scripts.Extract(ctx, g, roots)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: scripts extractor: %v\n", err)
			}
			for _, h := range healths {
				fmt.Fprintf(os.Stdout, "  scripts[%s]: status=%s nodes=%d\n", h.CollectorID, h.Status, h.NodesEmitted)
				addHealth(h.CollectorID, "installer_script", h.Status, h.NodesEmitted, h.Error, "P0")
			}
		}

		// systemd unit snapshot — reads /etc/systemd/system/globular-*.service.
		if awareCfg.collectSystemd {
			fmt.Fprintf(os.Stdout, "Collecting systemd unit state ...\n")
			h, err := clusterstate.CollectSystemd(ctx, g)
			errStr := ""
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: systemd collector: %v\n", err)
				errStr = err.Error()
			} else {
				fmt.Fprintf(os.Stdout, "  systemd: status=%s nodes=%d\n", h.Status, h.NodesEmitted)
			}
			addHealth("systemd", "systemd_runtime", h.Status, h.NodesEmitted, errStr, "P0")
		}

		// /var/lib/globular metadata scanner — certs, receipts, minio.env.
		if awareCfg.collectVarLib {
			fmt.Fprintf(os.Stdout, "Scanning /var/lib/globular metadata ...\n")
			h, err := clusterstate.CollectVarLib(ctx, g)
			errStr := ""
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: varlib collector: %v\n", err)
				errStr = err.Error()
			} else {
				fmt.Fprintf(os.Stdout, "  varlib: status=%s nodes=%d\n", h.Status, h.NodesEmitted)
			}
			addHealth("varlib", "installed_metadata", h.Status, h.NodesEmitted, errStr, "P0")
		}

		// etcd snapshot — desired/installed divergence detection.
		if awareCfg.collectEtcd {
			fmt.Fprintf(os.Stdout, "Collecting etcd desired-state snapshot ...\n")
			var etcdFactory clusterstate.EtcdClientFactory
			if client, err := getEtcdClientFactory(); err == nil {
				etcdFactory = client
			}
			h, err := clusterstate.CollectEtcd(ctx, g, etcdFactory)
			errStr := ""
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: etcd collector: %v\n", err)
				errStr = err.Error()
			} else {
				fmt.Fprintf(os.Stdout, "  etcd: status=%s nodes=%d\n", h.Status, h.NodesEmitted)
			}
			addHealth("etcd", "etcd_desired_state", h.Status, h.NodesEmitted, errStr, "P0")
		}

		// convergence records — Desired→Installed→Runtime delta extraction.
		if awareCfg.collectConvergence {
			fmt.Fprintf(os.Stdout, "Collecting convergence records from etcd ...\n")
			var etcdFactory clusterstate.EtcdClientFactory
			if client, err := getEtcdClientFactory(); err == nil {
				etcdFactory = client
			}
			h, err := clusterstate.CollectConvergence(ctx, g, etcdFactory)
			errStr := ""
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: convergence collector: %v\n", err)
				errStr = err.Error()
			} else {
				fmt.Fprintf(os.Stdout, "  convergence: status=%s nodes=%d\n", h.Status, h.NodesEmitted)
			}
			addHealth("convergence", "cluster_authority", h.Status, h.NodesEmitted, errStr, "P0")
		}

		// metrics knowledge indexer — loads metric_queries.yaml + metric_thresholds.yaml.
		if awareCfg.collectMetrics {
			fmt.Fprintf(os.Stdout, "Indexing metrics knowledge ...\n")
			docsDir := filepath.Join(repoRoot, "docs", "awareness")
			if err := metrics.Extract(ctx, g, docsDir); err != nil {
				fmt.Fprintf(os.Stderr, "warning: metrics extractor: %v\n", err)
				addHealth("metrics", "knowledge_base", "error", 0, err.Error(), "P0")
			} else {
				fmt.Fprintf(os.Stdout, "  metrics: indexed\n")
				addHealth("metrics", "knowledge_base", "ok", 0, "", "P0")
			}
		}

		// PKI certificate extractor — public cert metadata, SAN coverage, expiry.
		if awareCfg.collectPKI {
			fmt.Fprintf(os.Stdout, "Extracting PKI certificate metadata ...\n")
			h, err := pki.Extract(ctx, g, pki.DefaultPKIPaths[0])
			errStr := ""
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: pki extractor: %v\n", err)
				errStr = err.Error()
			} else {
				fmt.Fprintf(os.Stdout, "  pki: status=%s nodes=%d\n", h.Status, h.NodesEmitted)
			}
			addHealth("pki", "cluster_security", h.Status, h.NodesEmitted, errStr, "P1")
		}

		// RBAC policy extractor — roles, permissions, bindings.
		if awareCfg.collectRBAC {
			fmt.Fprintf(os.Stdout, "Extracting RBAC policy ...\n")
			h, err := rbac.Extract(ctx, g, rbac.DefaultPolicyDir)
			errStr := ""
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: rbac extractor: %v\n", err)
				errStr = err.Error()
			} else {
				fmt.Fprintf(os.Stdout, "  rbac: status=%s nodes=%d\n", h.Status, h.NodesEmitted)
			}
			addHealth("rbac", "cluster_security", h.Status, h.NodesEmitted, errStr, "P1")
		}

		// Workflow execution overlay — live run state from workflow service gRPC.
		if awareCfg.collectWorkflow {
			fmt.Fprintf(os.Stdout, "Collecting workflow execution state ...\n")
			var wfFactory workflowstate.GRPCConnFactory
			if addr := awareCfg.workflowAddr; addr != "" {
				wfFactory = func() (*grpc.ClientConn, error) {
					return dialGRPC(addr)
				}
			}
			docsDir := filepath.Join(repoRoot, "docs", "awareness")
			h, err := workflowstate.Collect(ctx, g, wfFactory, docsDir)
			errStr := ""
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: workflow state collector: %v\n", err)
				errStr = err.Error()
			} else {
				fmt.Fprintf(os.Stdout, "  workflow: status=%s coverage=%s runs_seen=%d failed=%d\n",
					h.Status, h.Coverage, h.RunsSeen, h.FailedRuns)
			}
			addHealth("workflow_execution", "live_runtime", h.Status, h.NodesEmitted, errStr, "P1")
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
		if len(collectorHealthItems) > 0 {
			if err := g.SetBuildCollectorHealth(ctx, buildID, collectorHealthItems); err != nil {
				fmt.Fprintf(os.Stderr, "warning: store collector health: %v\n", err)
			}
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

		if awareCfg.explain {
			return runImpactExplain(ctx, g, awareCfg.file)
		}

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

// runImpactExplain runs impact analysis with graph path traces printed inline.
func runImpactExplain(ctx context.Context, g *graph.Graph, filePath string) error {
	result, err := analysis.ExplainImpactByFile(ctx, g, filePath)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Impact analysis (explained): %s\n\n", filePath)

	if len(result.MissingLinks) > 0 {
		for _, m := range result.MissingLinks {
			fmt.Fprintf(os.Stdout, "  note: %s\n", m)
		}
		fmt.Fprintln(os.Stdout)
	}

	printExplainedSection := func(label string, findings []analysis.ExplainedFinding) {
		if len(findings) == 0 {
			return
		}
		fmt.Fprintf(os.Stdout, "%s:\n", label)
		for _, f := range findings {
			mandatoryTag := ""
			if f.Mandatory {
				mandatoryTag = "  [MANDATORY]"
			}
			fmt.Fprintf(os.Stdout, "  - %s%s\n", f.NodeName, mandatoryTag)
			for _, p := range f.EdgePath {
				fmt.Fprintf(os.Stdout, "    path: %s\n", p)
			}
			fmt.Fprintf(os.Stdout, "    confidence: %s\n", f.Confidence)
			if f.Source != "" {
				fmt.Fprintf(os.Stdout, "    source: %s\n", f.Source)
			}
		}
		fmt.Fprintln(os.Stdout)
	}

	printExplainedSection("Forbidden fixes", result.ForbiddenFixes)
	printExplainedSection("Required tests", result.RequiredTests)
	printExplainedSection("Impacted invariants", result.Invariants)
	printExplainedSection("Impacted failure modes", result.FailureModes)
	return nil
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

var awarenessReviewServiceCmd = &cobra.Command{
	Use:   "review-service <service-id>",
	Short: "Design-level review of a service in the awareness graph",
	Long: `Synthesises proto contract, RPC authz coverage, implementation links,
invariant attachments, dependencies, and runtime identity into a structured
design review for the named service.

<service-id> can be the service ID (e.g. "file-service"), the proto service
name (e.g. "file.FileService"), or the service display name.

Examples:
  globular awareness review-service file-service
  globular awareness review-service file.FileService`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		serviceID := args[0]

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		review, err := analysis.ReviewService(ctx, g, serviceID)
		if err != nil {
			return fmt.Errorf("review-service: %w", err)
		}

		fmt.Fprint(os.Stdout, renderServiceReview(review))
		return nil
	},
}

// renderServiceReview formats a ServiceDesignReview as human-readable text.
func renderServiceReview(r *analysis.ServiceDesignReview) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== Service Design Review: %s ===\n\n", r.ServiceID)

	// Identity
	fmt.Fprintf(&b, "## Identity\n")
	if r.ProtoService != "" {
		fmt.Fprintf(&b, "  proto_service : %s\n", r.ProtoService)
	}
	if r.ProtoFile != "" {
		fmt.Fprintf(&b, "  proto_file    : %s\n", r.ProtoFile)
	}
	if r.SystemdUnit != "" {
		fmt.Fprintf(&b, "  systemd_unit  : %s\n", r.SystemdUnit)
	}
	fmt.Fprintln(&b)

	// API contract
	if len(r.APIContract.RPCs) > 0 {
		fmt.Fprintf(&b, "## API Contract (%d RPCs)\n", len(r.APIContract.RPCs))
		for _, rpc := range r.APIContract.RPCs {
			mode := ""
			if rpc.StreamingMode != "" {
				mode = " [" + rpc.StreamingMode + "]"
			}
			authz := "NO AUTHZ"
			if rpc.HasAuthz {
				authz = fmt.Sprintf("authz: %s/%s", rpc.AuthzAction, rpc.AuthzResource)
			}
			fmt.Fprintf(&b, "  %-45s  %s%s\n", rpc.Name+mode, authz, gapsStr(rpc.Gaps))
		}
		fmt.Fprintln(&b)
	}

	// Dependencies
	if len(r.Dependencies) > 0 {
		fmt.Fprintf(&b, "## Dependencies (%d)\n", len(r.Dependencies))
		for _, dep := range r.Dependencies {
			req := "optional"
			if dep.Required {
				req = "required"
			}
			fmt.Fprintf(&b, "  %-35s  phase=%-20s  %s\n", dep.Service, dep.Phase, req)
		}
		fmt.Fprintln(&b)
	}

	// Invariants
	totalInvariants := len(r.Invariants.Critical) + len(r.Invariants.High) +
		len(r.Invariants.Medium) + len(r.Invariants.Low)
	if totalInvariants > 0 {
		fmt.Fprintf(&b, "## Invariants (%d)\n", totalInvariants)
		for _, id := range r.Invariants.Critical {
			fmt.Fprintf(&b, "  [CRITICAL] %s\n", id)
		}
		for _, id := range r.Invariants.High {
			fmt.Fprintf(&b, "  [HIGH]     %s\n", id)
		}
		for _, id := range r.Invariants.Medium {
			fmt.Fprintf(&b, "  [MEDIUM]   %s\n", id)
		}
		for _, id := range r.Invariants.Low {
			fmt.Fprintf(&b, "  [LOW]      %s\n", id)
		}
		fmt.Fprintln(&b)
	}

	// Forbidden fixes
	if len(r.ForbiddenFixes) > 0 {
		fmt.Fprintf(&b, "## Forbidden Fixes (%d)\n", len(r.ForbiddenFixes))
		for _, f := range r.ForbiddenFixes {
			fmt.Fprintf(&b, "  [%s] %s  (%s)\n", f.Severity, f.NodeName, f.NodeType)
			for _, p := range f.EdgePath {
				fmt.Fprintf(&b, "    path: %s\n", p)
			}
		}
		fmt.Fprintln(&b)
	}

	// Required tests
	if len(r.RequiredTests) > 0 {
		fmt.Fprintf(&b, "## Required Tests (%d)\n", len(r.RequiredTests))
		for _, f := range r.RequiredTests {
			fmt.Fprintf(&b, "  [%s] %s  (%s)\n", f.Severity, f.NodeName, f.NodeType)
		}
		fmt.Fprintln(&b)
	}

	// Missing links
	if len(r.MissingLinks) > 0 {
		fmt.Fprintf(&b, "## Missing Links\n")
		for _, m := range r.MissingLinks {
			fmt.Fprintf(&b, "  ! %s\n", m)
		}
		fmt.Fprintln(&b)
	}

	// Recommendations
	if len(r.Recommendations) > 0 {
		fmt.Fprintf(&b, "## Recommendations\n")
		for _, rec := range r.Recommendations {
			fmt.Fprintf(&b, "  [%s] %s\n", strings.ToUpper(rec.Priority), rec.Action)
			if rec.Evidence != "" {
				fmt.Fprintf(&b, "    evidence: %s\n", rec.Evidence)
			}
			if rec.GraphPath != "" {
				fmt.Fprintf(&b, "    graph:    %s\n", rec.GraphPath)
			}
		}
		fmt.Fprintln(&b)
	}

	if len(r.MissingLinks) == 0 && len(r.Recommendations) == 0 {
		fmt.Fprintf(&b, "No gaps or recommendations found.\n")
	}

	return b.String()
}

// gapsStr formats RPC gaps as a short inline suffix.
func gapsStr(gaps []string) string {
	if len(gaps) == 0 {
		return ""
	}
	return "  GAPS: [" + strings.Join(gaps, ", ") + "]"
}

func init() {
	// Build command flags.
	awarenessBuildCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db (default: .globular/awareness/graph.db in repo root)")
	awarenessBuildCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root (default: auto-detected from git)")
	awarenessBuildCmd.Flags().BoolVar(&awareCfg.cleanBuild, "clean", false, "Remove existing graph.db before building (required for edge correctness after YAML edits)")
	awarenessBuildCmd.Flags().StringVar(&awareCfg.packagesMetaDir, "packages-meta", "", "Path to packages metadata repo (e.g. /path/to/packages/metadata)")
	awarenessBuildCmd.Flags().StringArrayVar(&awareCfg.extraScriptRoots, "extra-scripts", nil, "Extra repo roots to crawl for shell scripts and Makefiles (repeatable)")
	awarenessBuildCmd.Flags().BoolVar(&awareCfg.collectSystemd, "collect-systemd", false, "Collect systemd unit state from /etc/systemd/system/globular-*.service")
	awarenessBuildCmd.Flags().BoolVar(&awareCfg.collectVarLib, "collect-var-lib", false, "Scan /var/lib/globular for PKI certs, receipts, and minio.env (never reads private keys)")
	awarenessBuildCmd.Flags().BoolVar(&awareCfg.collectEtcd, "collect-etcd", false, "Collect desired/installed state from etcd (requires cluster connectivity and TLS certs)")
	awarenessBuildCmd.Flags().BoolVar(&awareCfg.collectConvergence, "collect-convergence", false, "Collect Desired→Installed→Runtime convergence deltas from etcd (requires cluster connectivity)")
	awarenessBuildCmd.Flags().BoolVar(&awareCfg.collectMetrics, "collect-metrics", true, "Index metric_queries.yaml and metric_thresholds.yaml into the graph (default: on)")
	awarenessBuildCmd.Flags().BoolVar(&awareCfg.collectPKI, "collect-pki", false, "Extract public certificate metadata from /var/lib/globular/pki (never reads private keys)")
	awarenessBuildCmd.Flags().BoolVar(&awareCfg.collectRBAC, "collect-rbac", false, "Extract RBAC roles and permissions from /var/lib/globular/policy/rbac")
	awarenessBuildCmd.Flags().BoolVar(&awareCfg.collectWorkflow, "collect-workflow", false, "Collect live workflow execution state from workflow service gRPC (requires --workflow-addr)")
	awarenessBuildCmd.Flags().StringVar(&awareCfg.workflowAddr, "workflow-addr", "", "Workflow service gRPC address for live execution collection (e.g. workflow.globular.internal:10004)")

	// Stats flags.
	awarenessStatsCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessStatsCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// Impact flags.
	awarenessImpactCmd.Flags().StringVar(&awareCfg.file, "file", "", "File path to analyse (relative to repo root)")
	awarenessImpactCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessImpactCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")
	awarenessImpactCmd.Flags().BoolVar(&awareCfg.explain, "explain", false, "Show graph path for each finding")

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

	// review-service flags.
	awarenessReviewServiceCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessReviewServiceCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// Register subcommands.
	awarenessCmd.AddCommand(awarenessBuildCmd)
	awarenessCmd.AddCommand(awarenessStatsCmd)
	awarenessCmd.AddCommand(awarenessImpactCmd)
	awarenessCmd.AddCommand(awarenessAgentContextCmd)
	awarenessCmd.AddCommand(awarenessCyclesCmd)
	awarenessCmd.AddCommand(awarenessValidatePackageCmd)
	awarenessCmd.AddCommand(awarenessPackageContextCmd)
	awarenessCmd.AddCommand(awarenessAdmitPackageCmd)
	awarenessCmd.AddCommand(awarenessReviewServiceCmd)
	awarenessCmd.AddCommand(awarenessLiveSnapshotCmd)

	// Register top-level command.
	rootCmd.AddCommand(awarenessCmd)
}

const systemAwarenessDir = "/var/lib/globular/awareness"

// resolveAwarenessDBPath returns the canonical graph.db path for the
// current process. Resolution order:
//  1. /var/lib/globular/awareness/graph.db when accessible — the system
//     install location used by services and root-equivalent operators.
//  2. $HOME/.globular/awareness/graph.db when (1) is present but not
//     readable/writable by the current user. Surfaces with a stderr
//     warning so the fallback isn't silent.
//  3. repoRoot/.globular/awareness/graph.db when no system install exists
//     (developer working in a fresh checkout).
//
// The user fallback is for local CLI ergonomics only. Cluster service
// behaviour is unchanged: services run as root or globular and always hit
// path (1).
func resolveAwarenessDBPath(repoRoot string) string {
	home, _ := os.UserHomeDir()
	return resolveAwarenessDBPathFor(systemAwarenessDir, home, repoRoot, defaultAwarenessFallbackWarner)
}

// defaultAwarenessFallbackWarner prints a single line to stderr explaining
// why the user DB is being used. We don't dedupe across CLI invocations —
// each invocation that fell back gets the warning, so the friction stays
// visible until the perms are fixed.
func defaultAwarenessFallbackWarner(msg string) {
	fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
}

// resolveAwarenessDBPathFor is the testable form of resolveAwarenessDBPath.
// systemDir, homeDir, and repoRoot are injected so tests can simulate any
// of the three resolution branches without touching the real filesystem.
// warn is called once if the user fallback is selected; pass nil to suppress.
func resolveAwarenessDBPathFor(systemDir, homeDir, repoRoot string, warn func(string)) string {
	sysPath := filepath.Join(systemDir, "graph.db")
	if isUsableAwarenessDB(sysPath) {
		return sysPath
	}
	// System path is present-but-inaccessible OR not present at all.
	// Prefer the user fallback when we have a home directory, since it's
	// stable across repos and works even outside a git checkout.
	if homeDir != "" {
		userDir := filepath.Join(homeDir, ".globular", "awareness")
		if err := os.MkdirAll(userDir, 0o755); err == nil {
			userPath := filepath.Join(userDir, "graph.db")
			if warn != nil {
				warn(fmt.Sprintf(
					"Default awareness DB %s is not accessible; using user DB at %s",
					sysPath, userPath))
			}
			return userPath
		}
	}
	// No home or couldn't mkdir — fall through to repo-local.
	return filepath.Join(repoRoot, ".globular", "awareness", "graph.db")
}

// isUsableAwarenessDB returns true when the current process can read AND
// write the graph.db at path (or create one in its parent directory if the
// file doesn't exist yet). The check is conservative: if any test fails,
// we treat the path as unusable and fall back. The classic dev-machine
// failure mode is "/var/lib/globular/awareness/graph.db owned by root,
// mode 0644" — readable but not writable to the dev user.
func isUsableAwarenessDB(path string) bool {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		f, openErr := os.OpenFile(path, os.O_RDWR, 0)
		if openErr != nil {
			return false
		}
		_ = f.Close()
		return true
	}
	// File doesn't exist — check whether we could create it. The probe
	// avoids partial writes by using O_EXCL and removes the probe on success.
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil || !info.IsDir() {
		return false
	}
	probe := filepath.Join(parent, ".globular-awareness-probe")
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(probe)
	return true
}

// resolveAwarenessTrendPath returns the canonical audit-trend.jsonl path,
// using the same system-first, repo-fallback priority as resolveAwarenessDBPath.
func resolveAwarenessTrendPath(repoRoot string) string {
	if _, err := os.Stat(systemAwarenessDir); err == nil {
		return filepath.Join(systemAwarenessDir, "audit-trend.jsonl")
	}
	return filepath.Join(repoRoot, ".globular", "awareness", "audit-trend.jsonl")
}

// openAwarenessGraph opens the graph DB using the given path or the default location.
// Resolution order:
//  1. Explicit --db flag
//  2. /var/lib/globular/awareness/graph.db  (system install — preferred)
//  3. repoRoot/.globular/awareness/graph.db (dev fallback)
func openAwarenessGraph(dbPath, repoPath string) (*graph.Graph, error) {
	if dbPath == "" {
		repoRoot, _ := resolveRepoRoot(repoPath)
		dbPath = resolveAwarenessDBPath(repoRoot)
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

// getEtcdClientFactory returns an EtcdClientFactory that uses config.GetEtcdClient.
// Returns an error if the config package is not available (e.g. running off-cluster).
func getEtcdClientFactory() (clusterstate.EtcdClientFactory, error) {
	// Probe once to see if etcd is reachable; avoid importing the factory if not.
	if _, err := config.GetEtcdClient(); err != nil {
		return nil, err
	}
	return func() (*clientv3.Client, error) {
		return config.GetEtcdClient()
	}, nil
}
