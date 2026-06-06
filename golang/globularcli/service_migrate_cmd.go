// @awareness namespace=globular.platform
// @awareness component=platform_globularcli.service_migrate
// @awareness file_role=cli_command_invokes_service_mobility_orchestrator
// @awareness risk=medium
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	mobility "github.com/globulario/services/golang/services_mobility"
	"github.com/spf13/cobra"
)

var (
	serviceMigrateTarget     string
	serviceMigrateReadyWait  time.Duration
	serviceMigrateDrainWait  time.Duration
	serviceMigratePollEvery  time.Duration
	serviceMigrateJSONOutput bool
	serviceMigrateDryRun     bool
)

// serviceCmd is the parent for service-management subcommands. We
// register it independently from the `generate` group's nested
// `generate service` to avoid naming collision (full paths are
// distinct).
var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage Globular services (migrate, list, ...)",
}

var serviceMigrateCmd = &cobra.Command{
	Use:   "migrate <service-name>",
	Short: "Move a single-instance service to a target node (rebind, not reinstall)",
	Long: `Move a single-instance Globular service from its current node to a
target node. Implements the mobility primitive named in
meta.mobility_is_stronger_recovery_than_replication: start the binary
on the target, wait until it is serving, drain the source, stop the
source. Persistent state (Scylla, MinIO, etcd) stays put — only the
process moves.

This is the operator-triggered form. Automatic mobility on node-health
events is a follow-up that will invoke this same orchestration.

Prerequisites:
  - The service is currently running on exactly one node.
  - The target node is reachable and has the service binary installed
    (the release pipeline must have run on the target).
  - The service is Scylla-backed (process state is acceptable to lose;
    persistent state is preserved by the underlying store).

Failure modes:
  - Target unreachable, binary not installed, target fails to become
    healthy: refuses with a step-named error; source remains running.
  - Stop-source fails after target is serving: error surfaced so an
    operator can resolve the two-instances-running state manually.

Examples:
  globular service migrate ai-memory --to <node-id>
  globular service migrate ai_memory.AiMemoryService --to <node-id> --json
  globular service migrate ai-memory --to <node-id> --dry-run
`,
	Args: cobra.ExactArgs(1),
	RunE: runServiceMigrate,
}

func init() {
	serviceCmd.AddCommand(serviceMigrateCmd)
	rootCmd.AddCommand(serviceCmd)

	serviceMigrateCmd.Flags().StringVar(&serviceMigrateTarget, "to", "", "Target node ID")
	_ = serviceMigrateCmd.MarkFlagRequired("to")
	serviceMigrateCmd.Flags().DurationVar(&serviceMigrateReadyWait, "ready-timeout", 90*time.Second, "How long to wait for the target instance to become healthy")
	serviceMigrateCmd.Flags().DurationVar(&serviceMigrateDrainWait, "drain-grace", 10*time.Second, "Grace period between target-ready and source-stop (absorbs xDS propagation)")
	serviceMigrateCmd.Flags().DurationVar(&serviceMigratePollEvery, "poll-interval", 2*time.Second, "How often to poll the target health probe")
	serviceMigrateCmd.Flags().BoolVar(&serviceMigrateJSONOutput, "json", false, "Print the outcome as JSON")
	serviceMigrateCmd.Flags().BoolVar(&serviceMigrateDryRun, "dry-run", false, "Resolve and validate the plan; do not start/stop services")
}

func runServiceMigrate(cmd *cobra.Command, args []string) error {
	serviceName := strings.TrimSpace(args[0])
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}
	targetNodeID := strings.TrimSpace(serviceMigrateTarget)
	if targetNodeID == "" {
		return fmt.Errorf("--to is required")
	}

	ctx, cancel := context.WithTimeout(cmd.Context(), serviceMigrateReadyWait+serviceMigrateDrainWait+30*time.Second)
	defer cancel()

	// 1. Etcd client — standard config helper.
	etcd, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}

	// 2. Build the node-ID → IP and node-ID → AgentEndpoint maps by
	//    scanning etcd. The cluster_controller writes node records
	//    under /globular/nodes/<uuid>/...; we read them directly here
	//    rather than calling cluster_controller.ListNodes, so the CLI
	//    can run during controller unavailability windows.
	nodeIPToID, nodeRecords, err := loadClusterNodesEtcd(ctx, etcd)
	if err != nil {
		return fmt.Errorf("load cluster nodes: %w", err)
	}
	if _, ok := findNode(nodeRecords, targetNodeID); !ok {
		return fmt.Errorf("target node %q not found in cluster", targetNodeID)
	}

	// 3. Real ServiceRegistry against etcd.
	registry := mobility.NewEtcdServiceRegistry(etcd)
	registry.NodeIPToID = nodeIPToID

	// 4. Real NodeAgentController against the cluster's PKI material.
	caPath, certPath, keyPath := pkiPaths()
	nac, err := mobility.NewNodeAgentControllerImpl(nodeRecords, caPath, certPath, keyPath)
	if err != nil {
		return fmt.Errorf("node-agent controller: %w", err)
	}
	defer nac.Close()

	// 5. Dry run = resolve only.
	if serviceMigrateDryRun {
		instances, lerr := registry.InstancesOf(ctx, serviceName)
		if lerr != nil {
			return fmt.Errorf("dry-run resolve: %w", lerr)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "service: %s\n", serviceName)
		fmt.Fprintf(cmd.OutOrStdout(), "currently registered on: %v\n", instances)
		fmt.Fprintf(cmd.OutOrStdout(), "target: %s\n", targetNodeID)
		if len(instances) == 1 && instances[0] == targetNodeID {
			fmt.Fprintln(cmd.OutOrStdout(), "would be a no-op (already on target)")
		} else if len(instances) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "would refuse: service not registered anywhere")
		} else if len(instances) > 1 {
			fmt.Fprintf(cmd.OutOrStdout(), "would refuse: %d instances (multi-instance out of scope)\n", len(instances))
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "would migrate from %s to %s\n", instances[0], targetNodeID)
		}
		return nil
	}

	// 6. Orchestrate.
	o := mobility.New(nac, registry)
	o.Options = mobility.MigrateOptions{
		ReadyTimeout:     serviceMigrateReadyWait,
		DrainGracePeriod: serviceMigrateDrainWait,
		PollInterval:     serviceMigratePollEvery,
	}
	outcome := o.Migrate(ctx, serviceName, targetNodeID)

	// 7. Print outcome.
	if serviceMigrateJSONOutput {
		payload := struct {
			ServiceName  string   `json:"service_name"`
			SourceNodeID string   `json:"source_node_id"`
			TargetNodeID string   `json:"target_node_id"`
			Steps        []string `json:"steps"`
			StartedAt    string   `json:"started_at"`
			FinishedAt   string   `json:"finished_at"`
			Error        string   `json:"error,omitempty"`
		}{
			ServiceName:  outcome.ServiceName,
			SourceNodeID: outcome.SourceNodeID,
			TargetNodeID: outcome.TargetNodeID,
			Steps:        outcome.Steps,
			StartedAt:    outcome.StartedAt.UTC().Format(time.RFC3339),
			FinishedAt:   outcome.FinishedAt.UTC().Format(time.RFC3339),
		}
		if outcome.Err != nil {
			payload.Error = outcome.Err.Error()
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		fmt.Fprintln(cmd.OutOrStdout(), string(b))
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "service: %s\n", outcome.ServiceName)
		fmt.Fprintf(cmd.OutOrStdout(), "source:  %s\n", outcome.SourceNodeID)
		fmt.Fprintf(cmd.OutOrStdout(), "target:  %s\n", outcome.TargetNodeID)
		fmt.Fprintln(cmd.OutOrStdout(), "steps:")
		for _, s := range outcome.Steps {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", s)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "elapsed: %s\n", outcome.FinishedAt.Sub(outcome.StartedAt).Round(time.Millisecond))
		if outcome.Err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "ERROR: %s\n", outcome.Err)
		}
	}

	if outcome.Err != nil {
		return outcome.Err
	}
	return nil
}

// pkiPaths returns the standard service-cert paths Globular installs
// at /var/lib/globular/pki/. The CLI runs with these credentials when
// invoked on a cluster node by an operator who has the service-token
// in their environment.
func pkiPaths() (caPath, certPath, keyPath string) {
	caPath = "/var/lib/globular/pki/ca.crt"
	certPath = "/var/lib/globular/pki/issued/services/service.crt"
	keyPath = "/var/lib/globular/pki/issued/services/service.key"
	// Allow override for non-default installs.
	if v := os.Getenv("GLOBULAR_PKI_CA"); v != "" {
		caPath = v
	}
	if v := os.Getenv("GLOBULAR_PKI_CERT"); v != "" {
		certPath = v
	}
	if v := os.Getenv("GLOBULAR_PKI_KEY"); v != "" {
		keyPath = v
	}
	return
}

func findNode(nodes []mobility.NodeRecord, id string) (mobility.NodeRecord, bool) {
	for _, n := range nodes {
		if n.NodeID == id {
			return n, true
		}
	}
	return mobility.NodeRecord{}, false
}

