package main

// awareness_live_cluster_cmds.go: globular awareness live <subcommand>
//
// Query live cluster signals (service health, convergence, incidents) that
// have been persisted by the live preflight system.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/globulario/services/golang/awareness/graph"
	"github.com/globulario/services/golang/awareness/livecluster"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	"github.com/spf13/cobra"
)

var liveClusterCmd = &cobra.Command{
	Use:   "live",
	Short: "Query live cluster signals (health, convergence, preflight)",
}

func init() {
	awarenessCmd.AddCommand(liveClusterCmd)
	liveClusterCmd.AddCommand(
		liveCollectCmd,
		livePreflightCmd,
		liveHealthCmd,
		liveConvergenceCmd,
		liveLatestCmd,
	)
}

func openLiveStore() (*graph.Graph, *livecluster.Store, error) {
	dbPath := livGraphPath()
	if dbPath == "" {
		return nil, nil, fmt.Errorf("awareness graph not found — run 'globular awareness build' first")
	}
	g, err := graph.Open(dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open awareness graph: %w", err)
	}
	return g, livecluster.NewStore(g), nil
}

func livGraphPath() string {
	const systemPath = "/var/lib/globular/awareness/graph.json"
	if _, err := os.Stat(systemPath); err == nil {
		return systemPath
	}
	return ""
}

// liveCollectors builds the doctor + controller collectors the CLI passes
// to livecluster.CollectClusterSignals / RunLivePreflight. Each factory
// dials a fresh gRPC connection (the CLI is short-lived) and registers
// the conn for close on collector release. Transport failures degrade
// the source to "unavailable"; they never error the snapshot.
func liveCollectors() []livecluster.SignalCollector {
	return []livecluster.SignalCollector{
		livecluster.NewDoctorCollector("doctor", func(ctx context.Context) (cluster_doctorpb.ClusterDoctorServiceClient, func(), error) {
			addr, err := resolveDoctorEndpoint("")
			if err != nil {
				return nil, nil, err
			}
			cc, err := dialGRPC(addr)
			if err != nil {
				return nil, nil, err
			}
			return cluster_doctorpb.NewClusterDoctorServiceClient(cc), func() { _ = cc.Close() }, nil
		}),
		livecluster.NewControllerCollector("controller", func(ctx context.Context) (cluster_controllerpb.ClusterControllerServiceClient, func(), error) {
			addr := config.ResolveServiceAddr("cluster_controller.ClusterControllerService", "")
			if addr == "" {
				return nil, nil, fmt.Errorf("cluster-controller endpoint not found in etcd")
			}
			cc, err := dialGRPC(addr)
			if err != nil {
				return nil, nil, err
			}
			return cluster_controllerpb.NewClusterControllerServiceClient(cc), func() { _ = cc.Close() }, nil
		}),
	}
}


// ── collect ───────────────────────────────────────────────────────────────────

var liveCollectCfg = struct {
	clusterID     string
	sessionID     string
	task          string
	services      []string
	lookbackHours int
	requireLive   bool
	jsonOut       bool
}{}

var liveCollectCmd = &cobra.Command{
	Use:   "collect",
	Short: "Collect live cluster signals and store a snapshot",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, st, err := openLiveStore()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		req := livecluster.CollectSignalsRequest{
			ClusterID:       liveCollectCfg.clusterID,
			SessionID:       liveCollectCfg.sessionID,
			Task:            liveCollectCfg.task,
			Services:        liveCollectCfg.services,
			LookbackHours:   liveCollectCfg.lookbackHours,
			RequireLiveData: liveCollectCfg.requireLive,
		}
		snap, err := livecluster.CollectClusterSignals(ctx, req, liveCollectors())
		if err != nil {
			return err
		}
		_ = st.StoreClusterSignalSnapshot(ctx, snap)

		if liveCollectCfg.jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(snap)
		}

		fmt.Printf("Snapshot:    %s\n", snap.ID)
		fmt.Printf("Status:      %s\n", snap.Status)
		fmt.Printf("Summary:     %s\n", snap.Summary)
		fmt.Printf("Collected:   %s\n", time.Unix(snap.CollectedAt, 0).UTC().Format(time.RFC3339))
		fmt.Printf("Services:    %d\n", len(snap.Services))
		fmt.Printf("Convergence: %d\n", len(snap.Convergence))
		fmt.Printf("Incidents:   %d\n", len(snap.Incidents))
		return nil
	},
}

func init() {
	liveCollectCmd.Flags().StringVar(&liveCollectCfg.clusterID, "cluster", "", "Cluster ID")
	liveCollectCmd.Flags().StringVar(&liveCollectCfg.sessionID, "session", "", "Session ID")
	liveCollectCmd.Flags().StringVar(&liveCollectCfg.task, "task", "", "Task description")
	liveCollectCmd.Flags().StringArrayVar(&liveCollectCfg.services, "service", nil, "Service names to scope")
	liveCollectCmd.Flags().IntVar(&liveCollectCfg.lookbackHours, "lookback", 24, "Error lookback window in hours")
	liveCollectCmd.Flags().BoolVar(&liveCollectCfg.requireLive, "require-live", false, "Block if no live data available")
	liveCollectCmd.Flags().BoolVar(&liveCollectCfg.jsonOut, "json", false, "Output as JSON")
}

// ── preflight ─────────────────────────────────────────────────────────────────

var livePreflightCfg = struct {
	task          string
	files         []string
	components    []string
	services      []string
	sessionID     string
	staticID      string
	lookbackHours int
	requireLive   bool
	jsonOut       bool
}{}

var livePreflightCmd = &cobra.Command{
	Use:   "preflight",
	Short: "Run live preflight check before editing code",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, st, err := openLiveStore()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		req := livecluster.LivePreflightRequest{
			SessionID:       livePreflightCfg.sessionID,
			Task:            livePreflightCfg.task,
			Files:           livePreflightCfg.files,
			Components:      livePreflightCfg.components,
			Services:        livePreflightCfg.services,
			StaticResultID:  livePreflightCfg.staticID,
			LookbackHours:   livePreflightCfg.lookbackHours,
			RequireLiveData: livePreflightCfg.requireLive,
		}
		r, err := livecluster.RunLivePreflight(ctx, g, st, liveCollectors(), req)
		if err != nil {
			return err
		}

		if livePreflightCfg.jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(r)
		}

		fmt.Print(livecluster.FormatLiveSection(r))
		return nil
	},
}

func init() {
	livePreflightCmd.Flags().StringVar(&livePreflightCfg.task, "task", "", "Task description")
	livePreflightCmd.Flags().StringArrayVar(&livePreflightCfg.files, "file", nil, "Files being changed")
	livePreflightCmd.Flags().StringArrayVar(&livePreflightCfg.components, "component", nil, "Component names to check")
	livePreflightCmd.Flags().StringArrayVar(&livePreflightCfg.services, "service", nil, "Service names to check")
	livePreflightCmd.Flags().StringVar(&livePreflightCfg.sessionID, "session", "", "Session ID")
	livePreflightCmd.Flags().StringVar(&livePreflightCfg.staticID, "static-result", "", "Static preflight result ID to associate")
	livePreflightCmd.Flags().IntVar(&livePreflightCfg.lookbackHours, "lookback", 24, "Error lookback window in hours")
	livePreflightCmd.Flags().BoolVar(&livePreflightCfg.requireLive, "require-live", false, "Block if no live data available")
	livePreflightCmd.Flags().BoolVar(&livePreflightCfg.jsonOut, "json", false, "Output as JSON")
	_ = livePreflightCmd.MarkFlagRequired("task")
}

// ── health ────────────────────────────────────────────────────────────────────

var liveHealthCfg = struct {
	clusterID string
	services  []string
	jsonOut   bool
}{}

var liveHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Show service health from the latest stored snapshot",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, st, err := openLiveStore()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		snap, err := st.GetLatestClusterSignalSnapshot(ctx, liveHealthCfg.clusterID)
		if err != nil {
			return fmt.Errorf("no snapshot available (%w) — run 'globular awareness live collect' first", err)
		}

		services := snap.Services
		if len(liveHealthCfg.services) > 0 {
			filterSet := map[string]bool{}
			for _, s := range liveHealthCfg.services {
				filterSet[s] = true
			}
			var filtered []livecluster.ServiceLiveState
			for _, svc := range services {
				if filterSet[svc.ServiceName] || filterSet[svc.Component] {
					filtered = append(filtered, svc)
				}
			}
			services = filtered
		}

		if liveHealthCfg.jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(services)
		}

		fmt.Printf("Snapshot: %s  Status: %s\n", snap.ID, snap.Status)
		fmt.Printf("Collected: %s\n\n", time.Unix(snap.CollectedAt, 0).UTC().Format(time.RFC3339))
		if len(services) == 0 {
			fmt.Println("No service health data in snapshot.")
			return nil
		}
		for _, svc := range services {
			flag := "  "
			if svc.Health == "unhealthy" || svc.Health == "unreachable" {
				flag = "! "
			} else if svc.Health == "degraded" {
				flag = "~ "
			}
			fmt.Printf("%s%-30s  health=%-12s  status=%-12s  readiness=%s\n",
				flag, svc.ServiceName, svc.Health, svc.Status, svc.Readiness)
			if svc.LastError != "" {
				fmt.Printf("    error: %s\n", svc.LastError)
			}
		}
		return nil
	},
}

func init() {
	liveHealthCmd.Flags().StringVar(&liveHealthCfg.clusterID, "cluster", "", "Cluster ID")
	liveHealthCmd.Flags().StringArrayVar(&liveHealthCfg.services, "service", nil, "Filter to these service names")
	liveHealthCmd.Flags().BoolVar(&liveHealthCfg.jsonOut, "json", false, "Output as JSON")
}

// ── convergence ───────────────────────────────────────────────────────────────

var liveConvergenceCfg = struct {
	clusterID    string
	components   []string
	statusFilter string
	jsonOut      bool
}{}

var liveConvergenceCmd = &cobra.Command{
	Use:   "convergence",
	Short: "Show convergence states from the latest stored snapshot",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, st, err := openLiveStore()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		snap, err := st.GetLatestClusterSignalSnapshot(ctx, liveConvergenceCfg.clusterID)
		if err != nil {
			return fmt.Errorf("no snapshot available (%w) — run 'globular awareness live collect' first", err)
		}

		convergence := snap.Convergence
		if len(liveConvergenceCfg.components) > 0 {
			filterSet := map[string]bool{}
			for _, c := range liveConvergenceCfg.components {
				filterSet[c] = true
			}
			var filtered []livecluster.RuntimeConvergenceState
			for _, c := range convergence {
				if filterSet[c.Component] {
					filtered = append(filtered, c)
				}
			}
			convergence = filtered
		}
		if liveConvergenceCfg.statusFilter != "" {
			var filtered []livecluster.RuntimeConvergenceState
			for _, c := range convergence {
				if c.ConvergenceStatus == liveConvergenceCfg.statusFilter {
					filtered = append(filtered, c)
				}
			}
			convergence = filtered
		}

		if liveConvergenceCfg.jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(convergence)
		}

		fmt.Printf("Snapshot: %s\n\n", snap.ID)
		if len(convergence) == 0 {
			fmt.Println("No convergence data matching filters.")
			return nil
		}
		for _, c := range convergence {
			flag := "  "
			switch c.ConvergenceStatus {
			case "stuck", "diverged", "blocked":
				flag = "! "
			case "pending", "in_progress":
				flag = "~ "
			}
			fmt.Printf("%s%-30s  status=%-14s  retries=%d  age=%ds\n",
				flag, c.Component, c.ConvergenceStatus, c.RetryCount, c.AgeSeconds)
			if c.BlockedReason != "" {
				fmt.Printf("    reason: %s\n", c.BlockedReason)
			}
		}
		return nil
	},
}

func init() {
	liveConvergenceCmd.Flags().StringVar(&liveConvergenceCfg.clusterID, "cluster", "", "Cluster ID")
	liveConvergenceCmd.Flags().StringArrayVar(&liveConvergenceCfg.components, "component", nil, "Filter to these components")
	liveConvergenceCmd.Flags().StringVar(&liveConvergenceCfg.statusFilter, "status", "", "Filter by convergence status (e.g. stuck)")
	liveConvergenceCmd.Flags().BoolVar(&liveConvergenceCfg.jsonOut, "json", false, "Output as JSON")
}

// ── latest ────────────────────────────────────────────────────────────────────

var liveLatestCfg = struct {
	clusterID string
	jsonOut   bool
}{}

var liveLatestCmd = &cobra.Command{
	Use:   "latest",
	Short: "Show the latest stored cluster signal snapshot",
	RunE: func(cmd *cobra.Command, _ []string) error {
		g, st, err := openLiveStore()
		if err != nil {
			return err
		}
		defer g.Close()

		ctx := cmd.Context()
		snap, err := st.GetLatestClusterSignalSnapshot(ctx, liveLatestCfg.clusterID)
		if err != nil {
			return fmt.Errorf("no snapshot available (%w) — run 'globular awareness live collect' first", err)
		}

		if liveLatestCfg.jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(snap)
		}

		fmt.Printf("Snapshot ID: %s\n", snap.ID)
		fmt.Printf("Cluster:     %s\n", snap.ClusterID)
		fmt.Printf("Status:      %s\n", snap.Status)
		fmt.Printf("Summary:     %s\n", snap.Summary)
		fmt.Printf("Collected:   %s\n", time.Unix(snap.CollectedAt, 0).UTC().Format(time.RFC3339))
		fmt.Printf("\nServices (%d):\n", len(snap.Services))
		for _, svc := range snap.Services {
			fmt.Printf("  %-28s  %s / %s\n", svc.ServiceName, svc.Health, svc.Status)
		}
		fmt.Printf("\nConvergence (%d):\n", len(snap.Convergence))
		for _, c := range snap.Convergence {
			fmt.Printf("  %-28s  %s\n", c.Component, c.ConvergenceStatus)
		}
		fmt.Printf("\nIncidents (%d):\n", len(snap.Incidents))
		for _, inc := range snap.Incidents {
			fmt.Printf("  [%s] %s (%s)\n", inc.Severity, inc.Title, inc.Status)
		}
		fmt.Printf("\nErrors (%d):\n", len(snap.Errors))
		for _, e := range snap.Errors {
			fmt.Printf("  [%s] count=%d  %s\n", e.Severity, e.Count, e.Signature)
		}
		return nil
	},
}

func init() {
	liveLatestCmd.Flags().StringVar(&liveLatestCfg.clusterID, "cluster", "", "Cluster ID")
	liveLatestCmd.Flags().BoolVar(&liveLatestCfg.jsonOut, "json", false, "Output as JSON")
}
