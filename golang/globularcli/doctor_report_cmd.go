// doctor_report_cmd.go: Cluster-doctor read surfaces (cluster / node /
// drift reports) with freshness contract surfaced to operators.
//
//	globular doctor report cluster [--fresh] [--json]
//	globular doctor report node <node-id> [--fresh] [--json]
//	globular doctor report drift [--node <id>] [--fresh] [--json]
//
// Every printed report header shows source, observed_at, age, cache
// status and mode so the operator can reason about staleness without
// having to understand the doctor service's TTL config.

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	doctorReportEndpoint string
	doctorReportFresh    bool
	doctorReportJSON     bool
	doctorReportNodeID   string

	doctorReportCmd = &cobra.Command{
		Use:   "report",
		Short: "Fetch cluster-doctor reports (cluster / node / drift)",
		Long: `Read-only doctor surfaces with an explicit freshness contract.

By default, reports are served from cluster-doctor's short-TTL snapshot
cache. Pass --fresh to force a new upstream scan (useful right after a
remediation, or when opening an incident). Every response shows:

  source              who produced the report ("cluster-doctor")
  observed_at         when the snapshot was taken (server clock)
  age                 snapshot age at response time (server-computed)
  cache_hit           whether the cache was used
  cache_ttl           max staleness on a cached read
  freshness_mode      mode honoured for this response

These are the single source of truth for "how fresh is this data?" —
do not compute age from observed_at on the client (clock skew).`,
	}

	doctorReportClusterCmd = &cobra.Command{
		Use:   "cluster",
		Short: "Get the full cluster report",
		RunE:  runDoctorReportCluster,
	}

	doctorReportNodeCmd = &cobra.Command{
		Use:   "node <node-id>",
		Short: "Get a single-node report",
		Args:  cobra.ExactArgs(1),
		RunE:  runDoctorReportNode,
	}

	doctorReportDriftCmd = &cobra.Command{
		Use:   "drift",
		Short: "Get the desired/actual drift report",
		RunE:  runDoctorReportDrift,
	}
)

func init() {
	doctorCmd.AddCommand(doctorReportCmd)
	doctorReportCmd.AddCommand(doctorReportClusterCmd)
	doctorReportCmd.AddCommand(doctorReportNodeCmd)
	doctorReportCmd.AddCommand(doctorReportDriftCmd)

	for _, c := range []*cobra.Command{doctorReportClusterCmd, doctorReportNodeCmd, doctorReportDriftCmd} {
		c.Flags().StringVar(&doctorReportEndpoint, "endpoint", "", "cluster-doctor gRPC endpoint (default localhost:10080)")
		c.Flags().BoolVar(&doctorReportFresh, "fresh", false, "Force a fresh snapshot (bypass cache)")
		c.Flags().BoolVar(&doctorReportJSON, "json", false, "Output as JSON")
	}
	doctorReportDriftCmd.Flags().StringVar(&doctorReportNodeID, "node", "", "Filter drift items to a single node_id")
}

// dialDoctor opens a TLS connection to cluster-doctor. Mirrors the
// dialer used in doctor_remediate_cmd.go so the two share the same
// trust policy.
func dialDoctor() (*grpc.ClientConn, error) {
	endpoint := doctorReportEndpoint
	if endpoint == "" {
		endpoint = "localhost:10080"
	}
	return grpc.NewClient(
		endpoint,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{InsecureSkipVerify: true})),
	)
}

// freshnessMode returns the FreshnessMode to send based on --fresh.
func freshnessMode() cluster_doctorpb.FreshnessMode {
	if doctorReportFresh {
		return cluster_doctorpb.FreshnessMode_FRESHNESS_FRESH
	}
	return cluster_doctorpb.FreshnessMode_FRESHNESS_CACHED
}

func runDoctorReportCluster(cmd *cobra.Command, _ []string) error {
	conn, err := dialDoctor()
	if err != nil {
		return err
	}
	defer conn.Close()
	client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()
	rsp, err := client.GetClusterReport(ctx, &cluster_doctorpb.ClusterReportRequest{
		Freshness: freshnessMode(),
	})
	if err != nil {
		return fmt.Errorf("GetClusterReport: %w", err)
	}
	if doctorReportJSON {
		return writeJSON(rsp)
	}
	printHeader(rsp.GetHeader())
	fmt.Printf("overall_status: %s\n", rsp.GetOverallStatus())
	fmt.Printf("findings:       %d\n", len(rsp.GetFindings()))
	for cat, n := range rsp.GetCountsByCategory() {
		fmt.Printf("  %-16s %d\n", cat+":", n)
	}
	if len(rsp.GetTopIssueIds()) > 0 {
		fmt.Printf("top_issues: %s\n", strings.Join(rsp.GetTopIssueIds(), ", "))
	}
	return nil
}

func runDoctorReportNode(cmd *cobra.Command, args []string) error {
	nodeID := strings.TrimSpace(args[0])
	conn, err := dialDoctor()
	if err != nil {
		return err
	}
	defer conn.Close()
	client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()
	rsp, err := client.GetNodeReport(ctx, &cluster_doctorpb.NodeReportRequest{
		NodeId:    nodeID,
		Freshness: freshnessMode(),
	})
	if err != nil {
		return fmt.Errorf("GetNodeReport: %w", err)
	}
	if doctorReportJSON {
		return writeJSON(rsp)
	}
	printHeader(rsp.GetHeader())
	fmt.Printf("node:           %s\n", rsp.GetNodeId())
	fmt.Printf("reachable:      %v\n", rsp.GetReachable())
	fmt.Printf("heartbeat_age:  %ds\n", rsp.GetHeartbeatAgeSeconds())
	fmt.Printf("findings:       %d\n", len(rsp.GetFindings()))
	return nil
}

func runDoctorReportDrift(cmd *cobra.Command, _ []string) error {
	conn, err := dialDoctor()
	if err != nil {
		return err
	}
	defer conn.Close()
	client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()
	rsp, err := client.GetDriftReport(ctx, &cluster_doctorpb.DriftReportRequest{
		NodeId:    doctorReportNodeID,
		Freshness: freshnessMode(),
	})
	if err != nil {
		return fmt.Errorf("GetDriftReport: %w", err)
	}
	if doctorReportJSON {
		return writeJSON(rsp)
	}
	printHeader(rsp.GetHeader())
	fmt.Printf("drift_items:    %d\n", rsp.GetTotalDriftCount())
	return nil
}

// printHeader renders the freshness contract in a consistent shape.
// Keep this function in one place: every report surface must show the
// same six fields so operators build a stable mental model.
func printHeader(h *cluster_doctorpb.ReportHeader) {
	if h == nil {
		fmt.Println("(header missing)")
		return
	}
	fmt.Println("── report header ──────────────────────────────────────────")
	fmt.Printf("source:         %s\n", h.GetSource())
	if obs := h.GetObservedAt(); obs != nil {
		fmt.Printf("observed_at:    %s\n", obs.AsTime().Format(time.RFC3339))
	}
	fmt.Printf("age:            %ds\n", h.GetSnapshotAgeSeconds())
	fmt.Printf("cache_hit:      %v\n", h.GetCacheHit())
	fmt.Printf("cache_ttl:      %ds\n", h.GetCacheTtlSeconds())
	fmt.Printf("freshness_mode: %s\n", h.GetFreshnessMode())
	fmt.Printf("snapshot_id:    %s\n", h.GetSnapshotId())
	if h.GetDataIncomplete() {
		fmt.Printf("data_incomplete: true (%d errors)\n", len(h.GetDataErrors()))
	}
	fmt.Println("────────────────────────────────────────────────────────────")
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
