package main

// awareness_runtime_cmds.go: CLI commands for the runtime bridge (Task 6).
//
// Commands:
//
//	globular awareness runtime-snapshot [--format json] [--window 15m] [--write-graph]
//	globular awareness doctor-context --finding <finding>
//	globular awareness runtime-context --service <service>
//	globular awareness incident-from-runtime --task "<task>" [--propose]

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/analysis"
	"github.com/globulario/services/golang/awareness/learning"
	"github.com/globulario/services/golang/awareness/runtime"
)

var runtimeCfg = struct {
	format     string
	window     time.Duration
	writeGraph bool
	finding    string
	service    string
	task       string
	propose    bool
}{}

// ---- runtime-snapshot command ----

var awarenessRuntimeSnapshotCmd = &cobra.Command{
	Use:   "runtime-snapshot",
	Short: "Collect a read-only runtime snapshot from live cluster sources",
	Long: `Collects a point-in-time observation of cluster state using pluggable
read-only sources. In V1 all sources are noop (no cluster connection required);
real sources are attached by wiring in service implementations.

With --write-graph the snapshot is written as nodes/edges into the awareness graph.

This command is strictly read-only — it never mutates cluster state.`,
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

		// Open graph (non-fatal).
		g, graphErr := openAwarenessGraph(dbPath, awareCfg.repoPath)
		if graphErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not open awareness graph (%v)\n", graphErr)
		} else {
			defer g.Close()
		}

		bridge := runtime.NewBridge("", "")

		window := runtimeCfg.window
		if window <= 0 {
			window = 15 * time.Minute
		}

		snap, err := bridge.Snapshot(ctx, window, g)
		if err != nil {
			return fmt.Errorf("runtime-snapshot: %w", err)
		}

		if runtimeCfg.writeGraph && g != nil {
			if err := bridge.WriteToGraph(ctx, snap, g); err != nil {
				return fmt.Errorf("write-graph: %w", err)
			}
			fmt.Fprintf(os.Stdout, "snapshot written to graph: %s\n", snap.ID)
		}

		switch strings.ToLower(runtimeCfg.format) {
		case "json":
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(snap)
		default:
			printRuntimeSnapshotHuman(snap)
		}
		return nil
	},
}

func printRuntimeSnapshotHuman(snap *runtime.RuntimeSnapshot) {
	fmt.Fprintf(os.Stdout, "Runtime Snapshot: %s\n", snap.ID)
	fmt.Fprintf(os.Stdout, "  Captured:  %s\n", snap.CapturedAt.Format(time.RFC3339))
	fmt.Fprintf(os.Stdout, "  Node:      %s\n", snap.NodeID)
	fmt.Fprintf(os.Stdout, "  Cluster:   %s\n\n", snap.ClusterID)

	if len(snap.DoctorFindings) > 0 {
		fmt.Fprintf(os.Stdout, "Doctor Findings (%d):\n", len(snap.DoctorFindings))
		for _, f := range snap.DoctorFindings {
			suppressed := ""
			if f.Suppressed {
				suppressed = " [suppressed]"
			}
			fmt.Fprintf(os.Stdout, "  [%s] %s: %s%s\n", f.Severity, f.FindingID, f.Title, suppressed)
		}
		fmt.Fprintln(os.Stdout)
	}

	if len(snap.StateDelta) > 0 {
		fmt.Fprintf(os.Stdout, "State Deltas (%d):\n", len(snap.StateDelta))
		for _, d := range snap.StateDelta {
			fmt.Fprintf(os.Stdout, "  %s [%s] desired=%s installed=%s\n",
				d.ServiceID, d.DeltaType, d.DesiredVersion, d.InstalledVersion)
		}
		fmt.Fprintln(os.Stdout)
	}

	if len(snap.WorkflowReceipts) > 0 {
		fmt.Fprintf(os.Stdout, "Recent Workflows (%d):\n", len(snap.WorkflowReceipts))
		for _, w := range snap.WorkflowReceipts {
			fmt.Fprintf(os.Stdout, "  [%s] %s\n", w.Status, w.WorkflowType)
		}
		fmt.Fprintln(os.Stdout)
	}

	if len(snap.MatchedInvariants) > 0 {
		fmt.Fprintf(os.Stdout, "Matched Invariants:\n")
		for _, id := range snap.MatchedInvariants {
			fmt.Fprintf(os.Stdout, "  - %s\n", id)
		}
		fmt.Fprintln(os.Stdout)
	}

	if len(snap.MatchedFailureModes) > 0 {
		fmt.Fprintf(os.Stdout, "Matched Failure Modes:\n")
		for _, id := range snap.MatchedFailureModes {
			fmt.Fprintf(os.Stdout, "  - %s\n", id)
		}
		fmt.Fprintln(os.Stdout)
	}

	if len(snap.Warnings) > 0 {
		fmt.Fprintf(os.Stdout, "Warnings:\n")
		for _, w := range snap.Warnings {
			fmt.Fprintf(os.Stdout, "  ! %s\n", w)
		}
		fmt.Fprintln(os.Stdout)
	}
}

// ---- doctor-context command ----

var awarenessDoctorContextCmd = &cobra.Command{
	Use:   "doctor-context",
	Short: "Show awareness graph context for a doctor finding keyword",
	Long: `Queries the awareness graph for invariants and failure modes that match
the given finding keyword. Useful for understanding what architectural
contracts a doctor finding relates to.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtimeCfg.finding == "" {
			return fmt.Errorf("--finding is required")
		}

		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		repoRoot, _ := resolveRepoRoot(awareCfg.repoPath)
		aliasMap := loadAliasesQuiet(repoRoot)

		md, _, err := analysis.GenerateAgentContext(ctx, g, runtimeCfg.finding,
			analysis.AgentContextHints{},
			analysis.AgentContextAliases(aliasMap))
		if err != nil {
			return fmt.Errorf("doctor-context: %w", err)
		}
		fmt.Fprint(os.Stdout, md)
		return nil
	},
}

// ---- runtime-context command ----

var awarenessRuntimeContextCmd = &cobra.Command{
	Use:   "runtime-context",
	Short: "Show awareness graph context for a specific service at runtime",
	Long: `Queries the awareness graph for invariants, failure modes, and forbidden
fixes that relate to the given service. Combines agent-context with impact
analysis for the service node.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtimeCfg.service == "" {
			return fmt.Errorf("--service is required")
		}

		ctx := context.Background()
		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		repoRoot, _ := resolveRepoRoot(awareCfg.repoPath)
		aliasMap := loadAliasesQuiet(repoRoot)

		task := fmt.Sprintf("runtime context for service: %s", runtimeCfg.service)
		hints := analysis.AgentContextHints{
			Services: []string{runtimeCfg.service},
		}

		md, _, err := analysis.GenerateAgentContext(ctx, g, task, hints,
			analysis.AgentContextAliases(aliasMap))
		if err != nil {
			return fmt.Errorf("runtime-context: %w", err)
		}
		fmt.Fprint(os.Stdout, md)
		return nil
	},
}

// ---- incident-from-runtime command ----

var awarenessIncidentFromRuntimeCmd = &cobra.Command{
	Use:   "incident-from-runtime",
	Short: "Create an incident bundle from a live runtime snapshot",
	Long: `Runs a runtime snapshot (with noop sources in V1) and creates an
incident bundle YAML from the evidence. The bundle is written to
docs/awareness/incidents/<timestamp>_<task-slug>.yaml.

With --propose: a draft proposal is also generated alongside the bundle.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if runtimeCfg.task == "" {
			return fmt.Errorf("--task is required")
		}

		ctx := context.Background()

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}

		dbPath := awareCfg.dbPath
		if dbPath == "" {
			dbPath = filepath.Join(repoRoot, ".globular", "awareness", "graph.db")
		}

		g, graphErr := openAwarenessGraph(dbPath, awareCfg.repoPath)
		if graphErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not open awareness graph (%v)\n", graphErr)
		} else {
			defer g.Close()
		}

		bridge := runtime.NewBridge("", "")
		window := runtimeCfg.window
		if window <= 0 {
			window = 15 * time.Minute
		}

		snap, err := bridge.Snapshot(ctx, window, g)
		if err != nil {
			return fmt.Errorf("runtime snapshot: %w", err)
		}

		// Build incident bundle from snapshot evidence.
		now := time.Now().UTC()
		incidentID := fmt.Sprintf("runtime.%d", now.Unix())
		bundle := &learning.IncidentBundle{
			IncidentID: incidentID,
			Title:      runtimeCfg.task,
			Status:     "OPEN",
			Severity:   "medium",
			TimeRange: learning.TimeRange{
				From: snap.CapturedAt.Format(time.RFC3339),
				To:   snap.CapturedAt.Format(time.RFC3339),
			},
		}

		// Populate from snapshot evidence.
		for _, f := range snap.DoctorFindings {
			if !f.Suppressed {
				bundle.DoctorFindings = append(bundle.DoctorFindings, f.Title)
				if f.Severity == "critical" || f.Severity == "high" {
					bundle.Severity = f.Severity
				}
			}
		}
		for _, d := range snap.StateDelta {
			bundle.StateDeltas = append(bundle.StateDeltas,
				fmt.Sprintf("%s: %s (desired=%s installed=%s)",
					d.ServiceID, d.DeltaType, d.DesiredVersion, d.InstalledVersion))
		}
		for _, w := range snap.WorkflowReceipts {
			if w.Status == "FAILED" || w.Status == "TIMED_OUT" {
				bundle.WorkflowReceipts = append(bundle.WorkflowReceipts,
					fmt.Sprintf("%s [%s]: %s", w.WorkflowType, w.Status, w.ErrorMsg))
			}
		}
		for _, svc := range snap.RuntimeServices {
			bundle.ObservedServices = append(bundle.ObservedServices, svc.ServiceID)
		}
		bundle.Symptoms = snap.Warnings
		if len(snap.MatchedInvariants)+len(snap.MatchedFailureModes) > 0 {
			bundle.SuspectedRootCause = fmt.Sprintf(
				"matched invariants: %s; failure modes: %s",
				strings.Join(snap.MatchedInvariants, ", "),
				strings.Join(snap.MatchedFailureModes, ", "))
		}

		// Write bundle YAML.
		incidentsDir := filepath.Join(repoRoot, "docs", "awareness", "incidents")
		if err := os.MkdirAll(incidentsDir, 0o755); err != nil {
			return fmt.Errorf("mkdir incidents: %w", err)
		}

		slug := slugify(runtimeCfg.task)
		bundlePath := filepath.Join(incidentsDir,
			fmt.Sprintf("%d_%s.yaml", now.Unix(), slug))

		if err := learning.SaveIncidentBundle(bundlePath, bundle); err != nil {
			return fmt.Errorf("save bundle: %w", err)
		}
		fmt.Fprintf(os.Stdout, "incident bundle written: %s\n", bundlePath)

		if runtimeCfg.propose {
			proposal := learning.GenerateProposalFromBundle(bundle)
			proposalPath := filepath.Join(incidentsDir,
				fmt.Sprintf("%d_%s_proposal.yaml", now.Unix(), slug))
			if err := learning.SaveProposal(proposalPath, proposal); err != nil {
				return fmt.Errorf("save proposal: %w", err)
			}
			fmt.Fprintf(os.Stdout, "proposal written: %s\n", proposalPath)
		}

		return nil
	},
}

// slugify converts a task string to a filename-safe slug.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastWas := true // start with no dash
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastWas = false
		} else if !lastWas {
			b.WriteRune('_')
			lastWas = true
		}
	}
	result := strings.Trim(b.String(), "_")
	if len(result) > 40 {
		result = result[:40]
	}
	return result
}

func init() {
	// runtime-snapshot flags.
	awarenessRuntimeSnapshotCmd.Flags().StringVar(&runtimeCfg.format, "format", "human", "Output format: human | json")
	awarenessRuntimeSnapshotCmd.Flags().DurationVar(&runtimeCfg.window, "window", 15*time.Minute, "Lookback window for events and workflows")
	awarenessRuntimeSnapshotCmd.Flags().BoolVar(&runtimeCfg.writeGraph, "write-graph", false, "Write snapshot as nodes/edges into the awareness graph")
	awarenessRuntimeSnapshotCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessRuntimeSnapshotCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// doctor-context flags.
	awarenessDoctorContextCmd.Flags().StringVar(&runtimeCfg.finding, "finding", "", "Doctor finding keyword (required)")
	awarenessDoctorContextCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessDoctorContextCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// runtime-context flags.
	awarenessRuntimeContextCmd.Flags().StringVar(&runtimeCfg.service, "service", "", "Service ID (required)")
	awarenessRuntimeContextCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessRuntimeContextCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	// incident-from-runtime flags.
	awarenessIncidentFromRuntimeCmd.Flags().StringVar(&runtimeCfg.task, "task", "", "Task or incident description (required)")
	awarenessIncidentFromRuntimeCmd.Flags().BoolVar(&runtimeCfg.propose, "propose", false, "Also generate a draft proposal from the bundle")
	awarenessIncidentFromRuntimeCmd.Flags().DurationVar(&runtimeCfg.window, "window", 15*time.Minute, "Lookback window for runtime snapshot")
	awarenessIncidentFromRuntimeCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessIncidentFromRuntimeCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	awarenessCmd.AddCommand(awarenessRuntimeSnapshotCmd)
	awarenessCmd.AddCommand(awarenessDoctorContextCmd)
	awarenessCmd.AddCommand(awarenessRuntimeContextCmd)
	awarenessCmd.AddCommand(awarenessIncidentFromRuntimeCmd)
}
