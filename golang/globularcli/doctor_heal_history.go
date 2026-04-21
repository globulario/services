package main

import (
	"context"
	"fmt"
	"os"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	"github.com/spf13/cobra"
)

var (
	histNode      string
	histPackage   string
	histInvariant string
	histExecuted  bool
	histFailures  bool
	histLimit     int32
	histJSON      bool
)

var doctorHealHistoryCmd = &cobra.Command{
	Use:   "heal-history",
	Short: "Show recent auto-heal action history",
	Long: `Retrieves the persistent heal audit trail from the cluster doctor.

Shows what the healer did (or would have done), when, and whether
the action succeeded. Records persist across doctor restarts.

Examples:
  globular doctor heal-history                   # recent 50 records
  globular doctor heal-history --failures        # only failed actions
  globular doctor heal-history --package rbac    # filter by package
  globular doctor heal-history --executed        # only executed actions
  globular doctor heal-history --limit 10 --json # JSON output
`,
	RunE: runDoctorHealHistory,
}

func runDoctorHealHistory(cmd *cobra.Command, args []string) error {
	// Prefer the local instance; fall back to any reachable instance from etcd.
	// Port comes from etcd — never hardcoded. (CLAUDE.md rule 1 & 4)
	doctorAddr := config.ResolveLocalServiceAddr("cluster_doctor.ClusterDoctorService")
	if doctorAddr == "" {
		doctorAddr = config.ResolveServiceAddr("cluster_doctor.ClusterDoctorService", "")
	}
	if doctorAddr == "" {
		return fmt.Errorf("cluster-doctor not found in etcd service registry — is it running?")
	}
	cc, err := dialGRPC(doctorAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial doctor %s: %v\n", doctorAddr, err)
		os.Exit(2)
	}
	defer cc.Close()

	client := cluster_doctorpb.NewClusterDoctorServiceClient(cc)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := client.GetHealHistory(ctx, &cluster_doctorpb.GetHealHistoryRequest{
		Node:         histNode,
		PackageName:  histPackage,
		InvariantId:  histInvariant,
		ExecutedOnly: histExecuted,
		FailuresOnly: histFailures,
		Limit:        histLimit,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetHealHistory: %v\n", err)
		os.Exit(2)
	}

	if histJSON {
		for _, r := range resp.GetRecords() {
			fmt.Printf("{\"ts\":%q,\"cycle\":%q,\"invariant\":%q,\"entity\":%q,\"node\":%q,\"package\":%q,\"disposition\":%q,\"executed\":%v,\"verified\":%v,\"error\":%q}\n",
				r.GetTs(), r.GetCycleId(), r.GetInvariantId(), r.GetEntityRef(),
				r.GetNode(), r.GetPackageName(), r.GetDisposition(),
				r.GetExecuted(), r.GetVerified(), r.GetError())
		}
		return nil
	}

	fmt.Printf("Heal History — %d records\n\n", resp.GetTotal())
	if len(resp.GetRecords()) == 0 {
		fmt.Println("  (no heal actions recorded yet)")
		return nil
	}
	for _, r := range resp.GetRecords() {
		status := "skipped"
		if r.GetExecuted() && r.GetVerified() {
			status = "executed+verified"
		} else if r.GetExecuted() {
			status = "executed"
		}
		errStr := ""
		if r.GetError() != "" {
			errStr = fmt.Sprintf(" ERROR: %s", truncate(r.GetError(), 60))
		}
		fmt.Printf("  %s [%s] %s %s/%s — %s%s\n",
			r.GetTs()[:19], r.GetDisposition(),
			r.GetInvariantId(),
			truncate(r.GetNode(), 8), r.GetPackageName(),
			status, errStr)
	}
	return nil
}

func init() {
	doctorHealHistoryCmd.Flags().StringVar(&histNode, "node", "", "Filter by node ID prefix")
	doctorHealHistoryCmd.Flags().StringVar(&histPackage, "package", "", "Filter by package name")
	doctorHealHistoryCmd.Flags().StringVar(&histInvariant, "invariant", "", "Filter by invariant ID")
	doctorHealHistoryCmd.Flags().BoolVar(&histExecuted, "executed", false, "Only show executed actions")
	doctorHealHistoryCmd.Flags().BoolVar(&histFailures, "failures", false, "Only show failed actions")
	doctorHealHistoryCmd.Flags().Int32Var(&histLimit, "limit", 50, "Max records to return")
	doctorHealHistoryCmd.Flags().BoolVar(&histJSON, "json", false, "Output as JSONL")
	doctorCmd.AddCommand(doctorHealHistoryCmd)
}
