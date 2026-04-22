package main

// acc_cmds.go: CLI commands for tuning Adaptive Concurrency Control (ACC).
//
// Usage:
//
//	globular cluster acc get           — display the current ACC config stored in etcd
//	globular cluster acc set [flags]   — update one or more ACC parameters in etcd
//	globular cluster acc reset         — delete the etcd key (revert to compile-time defaults)
//
// The ACC config is stored as JSON at /globular/system/acc/config in etcd.
// Every gRPC interceptor picks up changes within seconds via a background watcher.

import (
	"context"
	"encoding/json"
	"fmt"
	"text/tabwriter"
	"os"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/config"
)

const accConfigEtcdKey = "/globular/system/acc/config"

// accConfig mirrors interceptors.ACCConfig — same JSON tags, same zero-means-ignore semantics.
// Defined locally to avoid importing the server-side interceptors package.
type accConfig struct {
	// P1 pool sizes.
	P1AuthzSize    int64 `json:"p1_authz_size,omitempty"`
	P1PeriodicSize int64 `json:"p1_periodic_size,omitempty"`
	P1ControlSize  int64 `json:"p1_control_size,omitempty"`

	// P2 AIMD window bounds.
	P2MinWindow int64 `json:"p2_min_window,omitempty"`
	P2MaxWindow int64 `json:"p2_max_window,omitempty"`

	// AIMD thresholds (multiples of baseline RTT).
	AIMDIncreaseThresholdMult float64 `json:"aimd_increase_threshold_mult,omitempty"`
	AIMDDecreaseThresholdMult float64 `json:"aimd_decrease_threshold_mult,omitempty"`
	AIMDDecreaseRate          float64 `json:"aimd_decrease_rate,omitempty"`

	// Recalibration.
	RecalibIntervalSec int64   `json:"recalib_interval_sec,omitempty"`
	RecalibAlpha       float64 `json:"recalib_alpha,omitempty"`
	RecalibMaxIncrease float64 `json:"recalib_max_increase,omitempty"`
	RecalibLoadGate    float64 `json:"recalib_load_gate,omitempty"`
}

// ── flags ─────────────────────────────────────────────────────────────────────

var (
	accSetP1AuthzSize    int64
	accSetP1PeriodicSize int64
	accSetP1ControlSize  int64
	accSetP2MinWindow    int64
	accSetP2MaxWindow    int64

	accSetAIMDIncreaseMult float64
	accSetAIMDDecreaseMult float64
	accSetAIMDDecreaseRate float64

	accSetRecalibIntervalSec int64
	accSetRecalibAlpha       float64
	accSetRecalibMaxIncrease float64
	accSetRecalibLoadGate    float64
)

// ── command tree ──────────────────────────────────────────────────────────────

var accCmd = &cobra.Command{
	Use:   "acc",
	Short: "Manage Adaptive Concurrency Control (ACC) configuration",
	Long: `Manage the Adaptive Concurrency Control (ACC) parameters stored in etcd.

ACC classifies every incoming gRPC call into a priority lane:

  P0              Health probes — always admitted, never throttled
  P1-authz (200)  ValidateAction — bounded to prevent OOM under cold-start storms
  P1-periodic (50) ReportNodeStatus, EmitWorkflowEvent — liveness heartbeats
  P1-control (10) CompleteOperation, ExecuteWorkflow — workflow lifecycle
  P2 (AIMD)       Everything else — window grows/shrinks via additive-increase /
                  multiplicative-decrease based on observed RTT

Configuration changes written to etcd take effect within seconds without a
service restart.`,
}

var accGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Display the current ACC configuration from etcd",
	Long: `Read the ACC configuration stored at /globular/system/acc/config in etcd
and display it.  If no key is set, compile-time defaults are in effect.`,
	RunE: runAccGet,
}

var accSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Update ACC parameters in etcd",
	Long: `Update one or more ACC parameters. Only the flags you pass are changed;
omitted flags leave their current values unchanged.

The change takes effect within seconds — the ACC background watcher in every
interceptor polls etcd continuously.`,
	RunE: runAccSet,
}

var accResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Delete the ACC config key (revert to compile-time defaults)",
	Long: `Remove /globular/system/acc/config from etcd.  Every interceptor reverts
to its compile-time defaults (P1-authz=200, P1-periodic=50, P1-control=10,
P2 window 10–2000, AIMD 1.5× increase / 2.0× decrease / 0.9 rate,
recalibration every 5 min with alpha=0.05).`,
	RunE: runAccReset,
}

func init() {
	// Flags for 'acc set'
	accSetCmd.Flags().Int64Var(&accSetP1AuthzSize, "p1-authz-size", 0,
		"ValidateAction pool size (default 200)")
	accSetCmd.Flags().Int64Var(&accSetP1PeriodicSize, "p1-periodic-size", 0,
		"Heartbeat pool size for ReportNodeStatus/EmitWorkflowEvent (default 50)")
	accSetCmd.Flags().Int64Var(&accSetP1ControlSize, "p1-control-size", 0,
		"Workflow lifecycle pool size for CompleteOperation/ExecuteWorkflow (default 10)")

	accSetCmd.Flags().Int64Var(&accSetP2MinWindow, "p2-min-window", 0,
		"Minimum AIMD window size (default 10)")
	accSetCmd.Flags().Int64Var(&accSetP2MaxWindow, "p2-max-window", 0,
		"Maximum AIMD window size (default 2000)")

	accSetCmd.Flags().Float64Var(&accSetAIMDIncreaseMult, "aimd-increase-mult", 0,
		"RTT < baseline×mult triggers window increase (default 1.5)")
	accSetCmd.Flags().Float64Var(&accSetAIMDDecreaseMult, "aimd-decrease-mult", 0,
		"RTT > baseline×mult triggers window decrease (default 2.0)")
	accSetCmd.Flags().Float64Var(&accSetAIMDDecreaseRate, "aimd-decrease-rate", 0,
		"Multiplicative decrease factor, must be 0<x<1 (default 0.9)")

	accSetCmd.Flags().Int64Var(&accSetRecalibIntervalSec, "recalib-interval-sec", 0,
		"Baseline recalibration interval in seconds (default 300)")
	accSetCmd.Flags().Float64Var(&accSetRecalibAlpha, "recalib-alpha", 0,
		"EMA smoothing factor for baseline updates, must be 0<x<1 (default 0.05)")
	accSetCmd.Flags().Float64Var(&accSetRecalibMaxIncrease, "recalib-max-increase", 0,
		"Sanity cap: reject recalib candidate if > N× current baseline (default 1.25)")
	accSetCmd.Flags().Float64Var(&accSetRecalibLoadGate, "recalib-load-gate", 0,
		"Skip recalibration if inflight > gate×window, must be 0<x<1 (default 0.60)")

	accCmd.AddCommand(accGetCmd, accSetCmd, accResetCmd)
	clusterCmd.AddCommand(accCmd)
}

// ── handlers ──────────────────────────────────────────────────────────────────

func runAccGet(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	defer cli.Close()

	resp, err := cli.Get(ctx, accConfigEtcdKey)
	if err != nil {
		return fmt.Errorf("etcd get: %w", err)
	}

	if len(resp.Kvs) == 0 {
		if rootCfg.output == "json" {
			fmt.Println("{}")
		} else {
			fmt.Println("No ACC config in etcd — compile-time defaults are active.")
			fmt.Println()
			printAccDefaults()
		}
		return nil
	}

	var cfg accConfig
	if err := json.Unmarshal(resp.Kvs[0].Value, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	if rootCfg.output == "json" {
		pretty, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(pretty))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PARAMETER\tVALUE\tDEFAULT")
	printAccRow(w, "p1_authz_size", cfg.P1AuthzSize, 200)
	printAccRow(w, "p1_periodic_size", cfg.P1PeriodicSize, 50)
	printAccRow(w, "p1_control_size", cfg.P1ControlSize, 10)
	printAccRow(w, "p2_min_window", cfg.P2MinWindow, 10)
	printAccRow(w, "p2_max_window", cfg.P2MaxWindow, 2000)
	printAccRowF(w, "aimd_increase_threshold_mult", cfg.AIMDIncreaseThresholdMult, 1.5)
	printAccRowF(w, "aimd_decrease_threshold_mult", cfg.AIMDDecreaseThresholdMult, 2.0)
	printAccRowF(w, "aimd_decrease_rate", cfg.AIMDDecreaseRate, 0.9)
	printAccRow(w, "recalib_interval_sec", cfg.RecalibIntervalSec, 300)
	printAccRowF(w, "recalib_alpha", cfg.RecalibAlpha, 0.05)
	printAccRowF(w, "recalib_max_increase", cfg.RecalibMaxIncrease, 1.25)
	printAccRowF(w, "recalib_load_gate", cfg.RecalibLoadGate, 0.60)
	w.Flush()
	return nil
}

func runAccSet(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	// Validate flag combinations before touching etcd.
	if accSetAIMDDecreaseRate != 0 && (accSetAIMDDecreaseRate <= 0 || accSetAIMDDecreaseRate >= 1) {
		return fmt.Errorf("--aimd-decrease-rate must be between 0 and 1 exclusive, got %.4f", accSetAIMDDecreaseRate)
	}
	if accSetRecalibAlpha != 0 && (accSetRecalibAlpha <= 0 || accSetRecalibAlpha >= 1) {
		return fmt.Errorf("--recalib-alpha must be between 0 and 1 exclusive, got %.4f", accSetRecalibAlpha)
	}
	if accSetRecalibLoadGate != 0 && (accSetRecalibLoadGate <= 0 || accSetRecalibLoadGate >= 1) {
		return fmt.Errorf("--recalib-load-gate must be between 0 and 1 exclusive, got %.4f", accSetRecalibLoadGate)
	}
	if accSetRecalibMaxIncrease != 0 && accSetRecalibMaxIncrease <= 1 {
		return fmt.Errorf("--recalib-max-increase must be > 1, got %.4f", accSetRecalibMaxIncrease)
	}

	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	defer cli.Close()

	// Read existing config so we can merge (only non-zero flags override).
	var cfg accConfig
	resp, err := cli.Get(ctx, accConfigEtcdKey)
	if err != nil {
		return fmt.Errorf("etcd get: %w", err)
	}
	if len(resp.Kvs) > 0 {
		if err := json.Unmarshal(resp.Kvs[0].Value, &cfg); err != nil {
			return fmt.Errorf("parse existing config: %w", err)
		}
	}

	// Merge: only apply flags explicitly set by the operator.
	changed := false
	if accSetP1AuthzSize > 0 {
		cfg.P1AuthzSize = accSetP1AuthzSize
		changed = true
	}
	if accSetP1PeriodicSize > 0 {
		cfg.P1PeriodicSize = accSetP1PeriodicSize
		changed = true
	}
	if accSetP1ControlSize > 0 {
		cfg.P1ControlSize = accSetP1ControlSize
		changed = true
	}
	if accSetP2MinWindow > 0 {
		cfg.P2MinWindow = accSetP2MinWindow
		changed = true
	}
	if accSetP2MaxWindow > 0 {
		cfg.P2MaxWindow = accSetP2MaxWindow
		changed = true
	}
	if accSetAIMDIncreaseMult > 0 {
		cfg.AIMDIncreaseThresholdMult = accSetAIMDIncreaseMult
		changed = true
	}
	if accSetAIMDDecreaseMult > 0 {
		cfg.AIMDDecreaseThresholdMult = accSetAIMDDecreaseMult
		changed = true
	}
	if accSetAIMDDecreaseRate > 0 {
		cfg.AIMDDecreaseRate = accSetAIMDDecreaseRate
		changed = true
	}
	if accSetRecalibIntervalSec > 0 {
		cfg.RecalibIntervalSec = accSetRecalibIntervalSec
		changed = true
	}
	if accSetRecalibAlpha > 0 {
		cfg.RecalibAlpha = accSetRecalibAlpha
		changed = true
	}
	if accSetRecalibMaxIncrease > 0 {
		cfg.RecalibMaxIncrease = accSetRecalibMaxIncrease
		changed = true
	}
	if accSetRecalibLoadGate > 0 {
		cfg.RecalibLoadGate = accSetRecalibLoadGate
		changed = true
	}

	if !changed {
		return fmt.Errorf("no flags provided — pass at least one parameter to update (see --help)")
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if _, err := cli.Put(ctx, accConfigEtcdKey, string(data)); err != nil {
		return fmt.Errorf("etcd put: %w", err)
	}

	if rootCfg.output == "json" {
		pretty, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(pretty))
		return nil
	}

	fmt.Printf("ACC config updated — interceptors will pick up changes within seconds.\n\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PARAMETER\tVALUE\tDEFAULT")
	printAccRow(w, "p1_authz_size", cfg.P1AuthzSize, 200)
	printAccRow(w, "p1_periodic_size", cfg.P1PeriodicSize, 50)
	printAccRow(w, "p1_control_size", cfg.P1ControlSize, 10)
	printAccRow(w, "p2_min_window", cfg.P2MinWindow, 10)
	printAccRow(w, "p2_max_window", cfg.P2MaxWindow, 2000)
	printAccRowF(w, "aimd_increase_threshold_mult", cfg.AIMDIncreaseThresholdMult, 1.5)
	printAccRowF(w, "aimd_decrease_threshold_mult", cfg.AIMDDecreaseThresholdMult, 2.0)
	printAccRowF(w, "aimd_decrease_rate", cfg.AIMDDecreaseRate, 0.9)
	printAccRow(w, "recalib_interval_sec", cfg.RecalibIntervalSec, 300)
	printAccRowF(w, "recalib_alpha", cfg.RecalibAlpha, 0.05)
	printAccRowF(w, "recalib_max_increase", cfg.RecalibMaxIncrease, 1.25)
	printAccRowF(w, "recalib_load_gate", cfg.RecalibLoadGate, 0.60)
	w.Flush()
	return nil
}

func runAccReset(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	defer cli.Close()

	dresp, err := cli.Delete(ctx, accConfigEtcdKey)
	if err != nil {
		return fmt.Errorf("etcd delete: %w", err)
	}

	if dresp.Deleted == 0 {
		fmt.Println("No ACC config was stored in etcd — compile-time defaults were already active.")
		return nil
	}

	fmt.Println("ACC config deleted. Interceptors will revert to compile-time defaults within seconds.")
	return nil
}

// ── output helpers ────────────────────────────────────────────────────────────

func printAccRow(w *tabwriter.Writer, name string, val, def int64) {
	v := fmt.Sprintf("%d", val)
	if val == 0 {
		v = fmt.Sprintf("(default: %d)", def)
	}
	fmt.Fprintf(w, "%s\t%s\t%d\n", name, v, def)
}

func printAccRowF(w *tabwriter.Writer, name string, val, def float64) {
	v := fmt.Sprintf("%.4g", val)
	if val == 0 {
		v = fmt.Sprintf("(default: %.4g)", def)
	}
	fmt.Fprintf(w, "%s\t%s\t%.4g\n", name, v, def)
}

func printAccDefaults() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PARAMETER\tDEFAULT")
	fmt.Fprintln(w, "p1_authz_size\t200")
	fmt.Fprintln(w, "p1_periodic_size\t50")
	fmt.Fprintln(w, "p1_control_size\t10")
	fmt.Fprintln(w, "p2_min_window\t10")
	fmt.Fprintln(w, "p2_max_window\t2000")
	fmt.Fprintln(w, "aimd_increase_threshold_mult\t1.5")
	fmt.Fprintln(w, "aimd_decrease_threshold_mult\t2")
	fmt.Fprintln(w, "aimd_decrease_rate\t0.9")
	fmt.Fprintln(w, "recalib_interval_sec\t300")
	fmt.Fprintln(w, "recalib_alpha\t0.05")
	fmt.Fprintln(w, "recalib_max_increase\t1.25")
	fmt.Fprintln(w, "recalib_load_gate\t0.6")
	w.Flush()
}
