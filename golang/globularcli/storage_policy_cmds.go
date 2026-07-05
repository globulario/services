package main

import (
	"fmt"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/spf13/cobra"
)

// storage_policy_cmds.go — operator declaration of the cluster storage-durability
// policy. `set` writes through the controller's owner RPC (never a raw etcd
// write); degraded requires the explicit --allow-degraded opt-in.

var (
	storagePolicyProfile string
	storagePolicyAllow   bool
	storagePolicyReason  string
	storagePolicyBy      string
)

var storageCmd = &cobra.Command{
	Use:   "storage",
	Short: "Cluster storage durability policy",
	Long: "Declare and inspect the cluster storage-durability policy.\n\n" +
		"By default a cluster is DURABLE and requires 3 storage nodes for ScyllaDB/\n" +
		"MinIO. A degraded profile lets a 1- or 2-node cluster converge with reduced\n" +
		"redundancy — it is NOT highly available and must be declared explicitly.",
}

var storagePolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Declare or show the storage durability policy",
}

var storagePolicyShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current cluster storage policy",
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)

		ctx, cancel := ctxWithCLITimeout(cmd.Context())
		defer cancel()

		resp, err := client.GetStoragePolicy(ctx, &cluster_controllerpb.GetStoragePolicyRequest{})
		if err != nil {
			return err
		}
		fmt.Printf("Cluster Storage Policy:\n")
		fmt.Printf("  Profile:            %s\n", resp.GetProfile())
		fmt.Printf("  Allow degraded:     %t\n", resp.GetAllowDegraded())
		fmt.Printf("  Degraded:           %t\n", resp.GetIsDegraded())
		fmt.Printf("  Min storage nodes:  %d\n", resp.GetMinStorageNodes())
		fmt.Printf("  Generation:         %d\n", resp.GetGeneration())
		if resp.GetDeclaredBy() != "" {
			fmt.Printf("  Declared by:        %s\n", resp.GetDeclaredBy())
		}
		if resp.GetReason() != "" {
			fmt.Printf("  Reason:             %s\n", resp.GetReason())
		}
		if resp.GetIsDegraded() {
			fmt.Printf("\n  ⚠ DEGRADED storage: reduced redundancy, NOT highly available.\n")
		}
		return nil
	},
}

var storagePolicySetCmd = &cobra.Command{
	Use:   "set",
	Short: "Declare the cluster storage policy",
	Long: "Declare the cluster storage-durability policy.\n\n" +
		"Profiles:\n" +
		"  durable            full redundancy, requires 3 storage nodes (default)\n" +
		"  two_node_degraded  2 storage nodes, RF=2, standalone MinIO — NOT HA\n" +
		"  single_node        1 storage node, RF=1, standalone MinIO — zero redundancy\n\n" +
		"A degraded profile requires --allow-degraded (explicit opt-in). Example:\n" +
		"  globular cluster storage policy set --profile two_node_degraded --allow-degraded --reason \"2-node lab\"",
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer cc.Close()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)

		ctx, cancel := ctxWithCLITimeout(cmd.Context())
		defer cancel()

		resp, err := client.SetStoragePolicy(ctx, &cluster_controllerpb.SetStoragePolicyRequest{
			Profile:       storagePolicyProfile,
			AllowDegraded: storagePolicyAllow,
			Reason:        storagePolicyReason,
			DeclaredBy:    storagePolicyBy,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Storage policy declared:\n")
		fmt.Printf("  Profile:            %s\n", resp.GetProfile())
		fmt.Printf("  Degraded:           %t\n", resp.GetIsDegraded())
		fmt.Printf("  Min storage nodes:  %d\n", resp.GetMinStorageNodes())
		fmt.Printf("  Generation:         %d\n", resp.GetGeneration())
		if resp.GetIsDegraded() {
			fmt.Printf("\n  ⚠ DEGRADED storage selected: reduced redundancy, NOT highly available.\n")
		}
		return nil
	},
}

func init() {
	storagePolicySetCmd.Flags().StringVar(&storagePolicyProfile, "profile", "durable", "durable | two_node_degraded | single_node")
	storagePolicySetCmd.Flags().BoolVar(&storagePolicyAllow, "allow-degraded", false, "Explicit opt-in required for a degraded profile")
	storagePolicySetCmd.Flags().StringVar(&storagePolicyReason, "reason", "", "Human note explaining the declaration")
	storagePolicySetCmd.Flags().StringVar(&storagePolicyBy, "declared-by", "", "Operator/actor making the declaration")

	storagePolicyCmd.AddCommand(storagePolicySetCmd, storagePolicyShowCmd)
	storageCmd.AddCommand(storagePolicyCmd)
	clusterCmd.AddCommand(storageCmd)
}
