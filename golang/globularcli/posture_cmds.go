package main

// posture_cmds.go: CLI commands for observing cluster posture.
//
//	globular cluster posture       — show current posture + signals (table)
//	globular cluster posture --output json — raw JSON from etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
)

const clusterPostureEtcdKey = "/globular/system/posture"

// postureSnapshot mirrors the JSON written by the controller's posture loop.
// Defined locally to avoid importing the controller binary.
type postureSnapshot struct {
	PostureStr  string          `json:"posture"`
	Reason      string          `json:"reason"`
	Signals     postureSignals  `json:"signals"`
	EvaluatedAt time.Time       `json:"evaluated_at"`
	StableTicks int             `json:"stable_ticks"`
}

type postureSignals struct {
	WorkflowCBOpen         bool    `json:"workflow_cb_open"`
	ReconcileCBOpen        bool    `json:"reconcile_cb_open"`
	LeaderLivenessDegraded bool    `json:"leader_liveness_degraded"`
	KnownNodes             int     `json:"known_nodes"`
	UnreachableNodes       int     `json:"unreachable_nodes"`
	UnreachableFraction    float64 `json:"unreachable_fraction,omitempty"`
}

// ── command ───────────────────────────────────────────────────────────────────

var postureCmd = &cobra.Command{
	Use:   "posture",
	Short: "Show the current cluster posture",
	Long: `Read the cluster posture snapshot from etcd and display it.

The cluster posture is computed every 30 seconds by the leader controller and
written to /globular/system/posture.  It reflects the current health of the
cluster's control-plane circuits:

  NORMAL        All signals healthy.  Full operation.
  DEGRADED      At least one circuit breaker open or leader liveness degraded.
                Rollout and background work would be paused in enforcement mode.
  RECOVERY_ONLY Both circuit breakers open, or workflow CB + liveness degraded.
                Only liveness and targeted repair would be allowed in enforcement mode.

Phase 1 (current): posture is computed and visible but does NOT gate dispatch.
Enforcement gates are enabled in Phase 2 after live signal validation.`,
	RunE: runPosture,
}

func init() {
	clusterCmd.AddCommand(postureCmd)
}

func runPosture(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	defer cli.Close()

	resp, err := cli.Get(ctx, clusterPostureEtcdKey)
	if err != nil {
		return fmt.Errorf("etcd get: %w", err)
	}

	if len(resp.Kvs) == 0 {
		fmt.Println("No posture snapshot in etcd.")
		fmt.Println("The posture loop runs on the leader controller every 30 seconds.")
		fmt.Println("If the cluster just started, wait up to 30 seconds and retry.")
		return nil
	}

	var snap postureSnapshot
	if err := json.Unmarshal(resp.Kvs[0].Value, &snap); err != nil {
		return fmt.Errorf("parse snapshot: %w", err)
	}

	if rootCfg.output == "json" {
		pretty, _ := json.MarshalIndent(snap, "", "  ")
		fmt.Println(string(pretty))
		return nil
	}

	age := time.Since(snap.EvaluatedAt).Truncate(time.Second)
	stale := age > 10*time.Minute

	// ── Posture header ────────────────────────────────────────────────────────
	fmt.Printf("Posture:  %s\n", snap.PostureStr)
	if snap.Reason != "" {
		fmt.Printf("Reason:   %s\n", snap.Reason)
	}
	fmt.Printf("Age:      %s ago", age)
	if stale {
		fmt.Print("  ⚠  STALE — leader may have crashed or restarted")
	}
	fmt.Println()
	fmt.Printf("Ticks:    %d consecutive at current posture\n", snap.StableTicks)
	fmt.Println()

	// ── Signals ───────────────────────────────────────────────────────────────
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SIGNAL\tVALUE\tNOTE")
	printSignalRow(w, "workflow_cb_open", snap.Signals.WorkflowCBOpen,
		"trusted trigger — workflow dispatch circuit breaker")
	printSignalRow(w, "reconcile_cb_open", snap.Signals.ReconcileCBOpen,
		"trusted trigger — reconcile circuit breaker")
	printSignalRow(w, "leader_liveness_degraded", snap.Signals.LeaderLivenessDegraded,
		"trusted trigger — no heartbeat from cluster nodes")
	fmt.Fprintf(w, "known_nodes\t%d\tobservational\n", snap.Signals.KnownNodes)
	fmt.Fprintf(w, "unreachable_nodes\t%d\tobservational\n", snap.Signals.UnreachableNodes)
	if snap.Signals.KnownNodes >= 3 {
		fmt.Fprintf(w, "unreachable_fraction\t%.1f%%\tobservational — not yet an enforcement trigger\n",
			snap.Signals.UnreachableFraction*100)
	} else {
		fmt.Fprintf(w, "unreachable_fraction\t—\tobservational — needs ≥3 known nodes\n")
	}
	w.Flush()

	// ── Phase annotation ──────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("Phase 1 (signal-only): posture is observed but enforcement gates are not active.")

	return nil
}

func printSignalRow(w *tabwriter.Writer, name string, active bool, note string) {
	value := "false"
	if active {
		value = "TRUE"
	}
	fmt.Fprintf(w, "%s\t%s\t%s\n", name, value, note)
}
