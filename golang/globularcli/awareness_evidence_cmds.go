package main

// awareness_evidence_cmds.go: CLI commands for the runtime evidence pipeline.
//
// Commands:
//
//	globular awareness evidence collect   [--node <id>] [--phase DAY1] [--save] [--format json]
//	globular awareness evidence classify  [--node <id>] [--save] [--format json]
//	globular awareness evidence bundle-status [--format json]
//	globular awareness evidence recent-facts [--severity CRITICAL] [--kind SCYLLA_CQL_UNREACHABLE]

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/evidence"
)

var evidenceCfg = struct {
	nodeID   string
	phase    string
	save     bool
	format   string
	severity string
	kind     string
}{}

// ---- root evidence command ----

var awarenessEvidenceCmd = &cobra.Command{
	Use:   "evidence",
	Short: "Runtime evidence collection and Day-1 classification",
	Long: `The evidence pipeline converts raw runtime errors into structured facts and
classifies a node's Day-1 readiness:

  collect       — gather local node evidence (systemd, ports, bundle)
  classify      — produce a Day-1 verdict from live evidence
  bundle-status — show awareness bundle installation status
  recent-facts  — show recently stored runtime facts`,
}

// ---- evidence collect ----

var awarenessEvidenceCollectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Collect and normalize local runtime evidence",
	Long: `Reads systemd unit states and port listener state, normalizes the raw
observations into structured RuntimeFacts, and prints the result.

Use --save to persist the snapshot to /var/lib/globular/awareness/runtime/.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		phase := evidence.PhaseDAY1
		if strings.ToUpper(evidenceCfg.phase) == "DAY0" {
			phase = evidence.PhaseDAY0
		}

		coll := evidence.NewCollector(evidenceCfg.nodeID, "", phase)
		snap := coll.Collect(ctx)
		norm := &evidence.Normalizer{}
		snap.Facts = norm.Normalize(snap)

		if evidenceCfg.save {
			if err := evidence.SaveSnapshot(snap); err != nil {
				fmt.Fprintf(os.Stderr, "warning: save failed: %v\n", err)
			} else {
				fmt.Fprintf(os.Stderr, "snapshot saved to /var/lib/globular/awareness/runtime/\n")
			}
		}

		return printEvidence(snap, evidenceCfg.format)
	},
}

// ---- evidence classify ----

var awarenessEvidenceClassifyCmd = &cobra.Command{
	Use:   "classify",
	Short: "Classify local node Day-1 readiness from live evidence",
	Long: `Collects a fresh local snapshot, normalizes it, and runs the Day-1 classifier.
Outputs a verdict (PASS/BLOCK/UNKNOWN) with the readiness ladder, primary blocker,
and allowed/forbidden actions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		coll := evidence.NewCollector(evidenceCfg.nodeID, "", evidence.PhaseDAY1)
		snap := coll.Collect(ctx)
		norm := &evidence.Normalizer{}
		snap.Facts = norm.Normalize(snap)

		if evidenceCfg.save {
			if err := evidence.SaveSnapshot(snap); err != nil {
				fmt.Fprintf(os.Stderr, "warning: save failed: %v\n", err)
			}
		}

		classifier := &evidence.Classifier{}
		verdict := classifier.Classify(snap)

		switch strings.ToLower(evidenceCfg.format) {
		case "json":
			return printJSON(verdict)
		default:
			printVerdictHuman(verdict)
		}
		return nil
	},
}

// ---- evidence bundle-status ----

var awarenessEvidenceBundleStatusCmd = &cobra.Command{
	Use:   "bundle-status",
	Short: "Show awareness bundle installation status on this node",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		coll := evidence.NewCollector("", "", evidence.PhaseDAY1)
		snap := coll.Collect(ctx)
		b := snap.AwarenessBundle

		switch strings.ToLower(evidenceCfg.format) {
		case "json":
			return printJSON(b)
		default:
			fmt.Fprintf(os.Stdout, "Awareness Bundle Status\n")
			fmt.Fprintf(os.Stdout, "  Present:  %v\n", b.Present)
			fmt.Fprintf(os.Stdout, "  Status:   %s\n", b.Status)
			if b.Version != "" {
				fmt.Fprintf(os.Stdout, "  Version:  %s\n", b.Version)
			}
			if b.BuildID != "" {
				fmt.Fprintf(os.Stdout, "  Build ID: %s\n", b.BuildID)
			}
		}
		return nil
	},
}

// ---- evidence recent-facts ----

var awarenessEvidenceRecentFactsCmd = &cobra.Command{
	Use:   "recent-facts",
	Short: "Show recently stored runtime facts from disk",
	Long: `Reads /var/lib/globular/awareness/runtime/facts.jsonl and prints facts
from the last 24 hours. Use --severity and --kind to filter.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cutoff := time.Now().Add(-24 * time.Hour)
		facts, err := evidence.LoadRecentFacts(cutoff)
		if err != nil {
			return fmt.Errorf("load facts: %w", err)
		}

		// Filter.
		var out []evidence.RuntimeFact
		for _, f := range facts {
			if evidenceCfg.severity != "" && string(f.Severity) != evidenceCfg.severity {
				continue
			}
			if evidenceCfg.kind != "" && string(f.Kind) != evidenceCfg.kind {
				continue
			}
			out = append(out, f)
		}

		switch strings.ToLower(evidenceCfg.format) {
		case "json":
			return printJSON(out)
		default:
			if len(out) == 0 {
				fmt.Fprintf(os.Stdout, "no facts found (last 24h)\n")
				return nil
			}
			fmt.Fprintf(os.Stdout, "Runtime Facts (last 24h, %d total):\n", len(out))
			for _, f := range out {
				fmt.Fprintf(os.Stdout, "  [%s] %s service=%s detail=%s\n",
					f.Severity, f.Kind, f.Service, f.Detail)
			}
		}
		return nil
	},
}

// ---- print helpers ----

func printEvidence(snap *evidence.NodeRuntimeSnapshot, format string) error {
	switch strings.ToLower(format) {
	case "json":
		return printJSON(snap)
	default:
		fmt.Fprintf(os.Stdout, "Evidence Snapshot\n")
		fmt.Fprintf(os.Stdout, "  Node:        %s\n", snap.NodeID)
		fmt.Fprintf(os.Stdout, "  Phase:       %s\n", snap.Phase)
		fmt.Fprintf(os.Stdout, "  Collected:   %s\n", snap.CollectedAt.Format(time.RFC3339))
		fmt.Fprintf(os.Stdout, "  Bundle:      %s (v%s build_id=%s)\n",
			snap.AwarenessBundle.Status, snap.AwarenessBundle.Version, snap.AwarenessBundle.BuildID)
		fmt.Fprintf(os.Stdout, "  Release:     v%s build_id=%s\n",
			snap.Release.Version, snap.Release.BuildID)
		fmt.Fprintf(os.Stdout, "\nServices (%d observed):\n", len(snap.Services))
		for _, svc := range snap.Services {
			if svc.ActiveState != "active" {
				fmt.Fprintf(os.Stdout, "  [%s/%s] %s\n",
					svc.ActiveState, svc.SubState, svc.UnitName)
			}
		}
		fmt.Fprintf(os.Stdout, "\nFacts (%d normalized):\n", len(snap.Facts))
		for _, f := range snap.Facts {
			fmt.Fprintf(os.Stdout, "  [%s] %s service=%s — %s\n",
				f.Severity, f.Kind, f.Service, f.Detail)
		}
	}
	return nil
}

func printVerdictHuman(v *evidence.Day1Verdict) {
	fmt.Fprintf(os.Stdout, "Day-1 Verdict: %s\n", v.Verdict)
	fmt.Fprintf(os.Stdout, "  Node:           %s\n", v.NodeID)
	fmt.Fprintf(os.Stdout, "  Classification: %s\n", v.Classification)
	if v.PrimaryBlocker != "" {
		fmt.Fprintf(os.Stdout, "  Primary Blocker: %s\n", v.PrimaryBlocker)
	}
	fmt.Fprintf(os.Stdout, "  Highest Level:  %s\n", v.HighestReachedLevel())

	fmt.Fprintf(os.Stdout, "\nReadiness Ladder:\n")
	for _, level := range evidence.Day1ReadinessLadder {
		mark := "✗"
		if v.Readiness[level] {
			mark = "✓"
		}
		fmt.Fprintf(os.Stdout, "  [%s] %s\n", mark, level)
	}

	if len(v.AllowedActions) > 0 {
		fmt.Fprintf(os.Stdout, "\nAllowed actions:\n")
		for _, a := range v.AllowedActions {
			fmt.Fprintf(os.Stdout, "  + %s\n", a)
		}
	}
	if len(v.ForbiddenActions) > 0 {
		fmt.Fprintf(os.Stdout, "\nForbidden actions:\n")
		for _, a := range v.ForbiddenActions {
			fmt.Fprintf(os.Stdout, "  ✗ %s\n", a)
		}
	}
	if len(v.BlockedServices) > 0 {
		fmt.Fprintf(os.Stdout, "\nBlocked services: %s\n",
			strings.Join(v.BlockedServices, ", "))
	}
}

func init() {
	// evidence collect
	awarenessEvidenceCollectCmd.Flags().StringVar(&evidenceCfg.nodeID, "node", "", "Node ID (default: empty)")
	awarenessEvidenceCollectCmd.Flags().StringVar(&evidenceCfg.phase, "phase", "DAY1", "Phase context: DAY0 or DAY1")
	awarenessEvidenceCollectCmd.Flags().BoolVar(&evidenceCfg.save, "save", false, "Save snapshot to /var/lib/globular/awareness/runtime/")
	awarenessEvidenceCollectCmd.Flags().StringVar(&evidenceCfg.format, "format", "human", "Output format: human | json")

	// evidence classify
	awarenessEvidenceClassifyCmd.Flags().StringVar(&evidenceCfg.nodeID, "node", "", "Node ID (default: empty)")
	awarenessEvidenceClassifyCmd.Flags().BoolVar(&evidenceCfg.save, "save", false, "Save snapshot to /var/lib/globular/awareness/runtime/")
	awarenessEvidenceClassifyCmd.Flags().StringVar(&evidenceCfg.format, "format", "human", "Output format: human | json")

	// evidence bundle-status
	awarenessEvidenceBundleStatusCmd.Flags().StringVar(&evidenceCfg.format, "format", "human", "Output format: human | json")

	// evidence recent-facts
	awarenessEvidenceRecentFactsCmd.Flags().StringVar(&evidenceCfg.severity, "severity", "", "Filter by severity: CRITICAL, HIGH, MEDIUM, LOW")
	awarenessEvidenceRecentFactsCmd.Flags().StringVar(&evidenceCfg.kind, "kind", "", "Filter by fact kind (e.g. SCYLLA_CQL_UNREACHABLE)")
	awarenessEvidenceRecentFactsCmd.Flags().StringVar(&evidenceCfg.format, "format", "human", "Output format: human | json")

	awarenessEvidenceCmd.AddCommand(awarenessEvidenceCollectCmd)
	awarenessEvidenceCmd.AddCommand(awarenessEvidenceClassifyCmd)
	awarenessEvidenceCmd.AddCommand(awarenessEvidenceBundleStatusCmd)
	awarenessEvidenceCmd.AddCommand(awarenessEvidenceRecentFactsCmd)

	awarenessCmd.AddCommand(awarenessEvidenceCmd)
}
