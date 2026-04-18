package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	monitoringpb "github.com/globulario/services/golang/monitoring/monitoringpb"
)

var monitorAddr string

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Query the monitoring service (Prometheus)",
	Long: `Interact with the Globular monitoring service to view alerts,
query metrics, and inspect targets.

Examples:
  globular monitor alerts
  globular monitor query 'up'
  globular monitor targets
`,
}

// --- alerts ---

var monitorAlertsCmd = &cobra.Command{
	Use:   "alerts",
	Short: "List active alerts",
	RunE:  runMonitorAlerts,
}

func runMonitorAlerts(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(pick(monitorAddr, rootCfg.controllerAddr))
	if err != nil {
		return fmt.Errorf("connect to monitoring service: %w", err)
	}
	defer cc.Close()

	client := monitoringpb.NewMonitoringServiceClient(cc)
	resp, err := client.Alerts(ctxWithTimeout(), &monitoringpb.AlertsRequest{})
	if err != nil {
		return fmt.Errorf("alerts: %w", err)
	}

	if rootCfg.output == "json" {
		return printJSON(resp)
	}

	result := resp.GetResults()
	if result == "" {
		fmt.Println("No active alerts.")
		return nil
	}
	fmt.Println(result)
	return nil
}

// --- query ---

var (
	monitorQueryExpr string
	monitorQueryTime string
)

var monitorQueryCmd = &cobra.Command{
	Use:   "query <expr>",
	Short: "Execute an instant PromQL query",
	Args:  cobra.ExactArgs(1),
	RunE:  runMonitorQuery,
}

func runMonitorQuery(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(pick(monitorAddr, rootCfg.controllerAddr))
	if err != nil {
		return fmt.Errorf("connect to monitoring service: %w", err)
	}
	defer cc.Close()

	client := monitoringpb.NewMonitoringServiceClient(cc)

	req := &monitoringpb.QueryRequest{Query: args[0]}
	if monitorQueryTime != "" {
		// Parse RFC3339 time string to Unix timestamp
		if t, err := time.Parse(time.RFC3339, monitorQueryTime); err == nil {
			req.Ts = float64(t.Unix())
		}
	}

	resp, err := client.Query(ctxWithTimeout(), req)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}

	if rootCfg.output == "json" {
		return printJSON(resp)
	}

	if resp.GetWarnings() != "" {
		fmt.Printf("Warnings: %s\n\n", resp.GetWarnings())
	}
	fmt.Println(resp.GetValue())
	return nil
}

// --- targets ---

var monitorTargetsCmd = &cobra.Command{
	Use:   "targets",
	Short: "List scrape targets and their health",
	RunE:  runMonitorTargets,
}

func runMonitorTargets(cmd *cobra.Command, args []string) error {
	cc, err := dialGRPC(pick(monitorAddr, rootCfg.controllerAddr))
	if err != nil {
		return fmt.Errorf("connect to monitoring service: %w", err)
	}
	defer cc.Close()

	client := monitoringpb.NewMonitoringServiceClient(cc)
	resp, err := client.Targets(ctxWithTimeout(), &monitoringpb.TargetsRequest{})
	if err != nil {
		return fmt.Errorf("targets: %w", err)
	}

	if rootCfg.output == "json" {
		return printJSON(resp)
	}

	fmt.Println(resp.GetResult())
	return nil
}

func init() {
	monitorCmd.PersistentFlags().StringVar(&monitorAddr, "monitor", "globular.internal", "Monitoring service address")

	monitorQueryCmd.Flags().StringVar(&monitorQueryTime, "time", "", "Evaluation timestamp (RFC3339, default: now)")

	monitorCmd.AddCommand(monitorAlertsCmd)
	monitorCmd.AddCommand(monitorQueryCmd)
	monitorCmd.AddCommand(monitorTargetsCmd)

	rootCmd.AddCommand(monitorCmd)
}
