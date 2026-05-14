package main

// awareness_decision_trace_cmd.go — Phase 10 CLI surface for the
// context-navigation effort. Two commands:
//
//   - globular awareness decision-trace
//     Runs a full preflight and returns ONLY the DecisionTraces slice.
//     Useful when an agent already has the trace it wants and doesn't
//     need the rest of the preflight output.
//
//   - globular awareness finding-context
//     Takes an explicit prefixed finding id (e.g.
//     `failure_mode:workflow.resume_poisoning`) and returns the single
//     DecisionTrace for that finding without running full preflight.
//     The fast path for "I know which failure mode I'm dealing with —
//     give me the pivots, owner, falsifiers, and next actions."
//
// Both commands accept --format=json|agent|markdown. JSON contains the
// full trace structure; agent format runs the trace through the same
// renderer preflight uses; markdown is the human-readable view.

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/analysis/contextnav"
	"github.com/globulario/services/golang/awareness/preflight"
)

var decisionTraceCfg = struct {
	task           string
	files          []string
	finding        string
	includeRuntime bool
	format         string
}{}

var awarenessDecisionTraceCmd = &cobra.Command{
	Use:   "decision-trace",
	Short: "Return only the per-finding decision traces from a preflight run",
	Long: `decision-trace runs the same preflight pipeline as 'preflight' but
returns ONLY the per-finding decision traces. Useful when the agent
already has the rest of the preflight output cached and just wants the
navigation layer (matched_by / pivots / next_actions / falsifiers).

Examples:

  globular awareness decision-trace --task "workflow retry loop" --format agent

  globular awareness decision-trace \
    --task "desired_hash mismatch after deploy" \
    --file golang/cluster_controller/convergence.go \
    --include-runtime \
    --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if decisionTraceCfg.task == "" {
			return fmt.Errorf("--task is required")
		}
		ctx := context.Background()

		repoRoot, err := resolveRepoRoot(awareCfg.repoPath)
		if err != nil {
			return err
		}
		docsDir := filepath.Join(repoRoot, "docs", "awareness")

		g, err := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if err != nil {
			return err
		}
		defer g.Close()

		opts := preflight.Options{
			Task:           decisionTraceCfg.task,
			Files:          decisionTraceCfg.files,
			DocsDir:        docsDir,
			RepoRoot:       repoRoot,
			IncludeRuntime: decisionTraceCfg.includeRuntime,
		}
		r, err := preflight.Run(ctx, opts, g)
		if err != nil {
			return fmt.Errorf("preflight run: %w", err)
		}
		return renderDecisionTraces(r.DecisionTraces, decisionTraceCfg.format)
	},
}

var awarenessFindingContextCmd = &cobra.Command{
	Use:   "finding-context",
	Short: "Return the decision trace for a single explicit finding id",
	Long: `finding-context returns the per-finding decision trace for an
explicit prefixed id (failure_mode:X, invariant:Y, forbidden_fix:Z).
Skips the full preflight pipeline — runs only the contextnav.Build path
on the supplied finding plus the graph walk for owner inference and
pivots.

Examples:

  globular awareness finding-context \
    --finding failure_mode:workflow.resume_poisoning \
    --format agent

  globular awareness finding-context \
    --finding invariant:service.restart_singleflight \
    --include-runtime \
    --format json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if decisionTraceCfg.finding == "" {
			return fmt.Errorf("--finding is required (form: failure_mode:X | invariant:Y | forbidden_fix:Z)")
		}
		ctx := context.Background()

		kind, id, err := contextnav.ParseFindingID(decisionTraceCfg.finding)
		if err != nil {
			return err
		}

		// Graph is optional — finding-context still produces a useful trace
		// (templated falsifiers + actions) without one.
		g, _ := openAwarenessGraph(awareCfg.dbPath, awareCfg.repoPath)
		if g != nil {
			defer g.Close()
		}

		tr, err := contextnav.BuildForFinding(ctx, contextnav.FindingContextOptions{
			Kind:           kind,
			ID:             id,
			Graph:          g,
			Task:           decisionTraceCfg.task,
			Files:          decisionTraceCfg.files,
			IncludeRuntime: decisionTraceCfg.includeRuntime,
		})
		if err != nil {
			return err
		}
		return renderDecisionTraces([]preflight.DecisionTrace{tr}, decisionTraceCfg.format)
	},
}

// renderDecisionTraces formats a slice of traces using the same logic the
// preflight `agent` / `json` renderers use. For agent format we wrap the
// traces in a minimal Report so the existing renderAgent decision-trace
// section runs unchanged.
func renderDecisionTraces(traces []preflight.DecisionTrace, format string) error {
	if format == "" {
		format = "json"
	}
	switch strings.ToLower(format) {
	case "json":
		b, err := json.MarshalIndent(traces, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	case "agent", "markdown":
		// Use the preflight renderer's "Decision traces" section. A
		// minimal Report avoids producing the rest of the agent banner.
		r := &preflight.Report{DecisionTraces: traces}
		out, err := preflight.Render(r, preflight.Format(format))
		if err != nil {
			return err
		}
		fmt.Print(out)
		return nil
	}
	return fmt.Errorf("unsupported format %q (want json|agent|markdown)", format)
}

func init() {
	awarenessDecisionTraceCmd.Flags().StringVar(&decisionTraceCfg.task, "task", "", "Task description (required)")
	awarenessDecisionTraceCmd.Flags().StringSliceVar(&decisionTraceCfg.files, "file", nil, "Files you plan to edit (repeatable)")
	awarenessDecisionTraceCmd.Flags().BoolVar(&decisionTraceCfg.includeRuntime, "include-runtime", false, "Collect runtime evidence")
	awarenessDecisionTraceCmd.Flags().StringVar(&decisionTraceCfg.format, "format", "agent", "Output format: agent | json | markdown")
	awarenessDecisionTraceCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db")
	awarenessDecisionTraceCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	awarenessFindingContextCmd.Flags().StringVar(&decisionTraceCfg.finding, "finding", "", "Prefixed finding id (failure_mode:X | invariant:Y | forbidden_fix:Z)")
	awarenessFindingContextCmd.Flags().StringVar(&decisionTraceCfg.task, "task", "", "Optional task description for owner inference and pivot context")
	awarenessFindingContextCmd.Flags().StringSliceVar(&decisionTraceCfg.files, "file", nil, "Optional files for owner inference (repeatable)")
	awarenessFindingContextCmd.Flags().BoolVar(&decisionTraceCfg.includeRuntime, "include-runtime", false, "Include runtime-flavoured pivots")
	awarenessFindingContextCmd.Flags().StringVar(&decisionTraceCfg.format, "format", "agent", "Output format: agent | json | markdown")
	awarenessFindingContextCmd.Flags().StringVar(&awareCfg.dbPath, "db", "", "Path to graph.db (optional)")
	awarenessFindingContextCmd.Flags().StringVar(&awareCfg.repoPath, "repo", "", "Repo root")

	awarenessCmd.AddCommand(awarenessDecisionTraceCmd)
	awarenessCmd.AddCommand(awarenessFindingContextCmd)
}
