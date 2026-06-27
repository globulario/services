package main

import (
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/spf13/cobra"
)

var (
	platformActivateAllowRegression bool
	platformActivateForce           bool
)

var platformCmd = &cobra.Command{
	Use:   "platform",
	Short: "Platform release management",
}

var platformActivateCmd = &cobra.Command{
	Use:   "activate <release-tag>",
	Short: "Advance the cluster's active platform release pointer (etcd) after a convergence gate",
	Long: `Activates a platform release: sets the authoritative active-release pointer
in etcd (/globular/platform/active_release) to <release-tag>, after verifying
every installed package across the cluster already runs the release's BOM
version (native-version safe).

The active pointer means "the cluster is operating against this release" — it is
convergence-gated, so it never advances ahead of what nodes actually run. etcd is
the authority; /var/lib/globular/release-index.json is a node-local projection.
See docs/design/platform-release-pointer-advance.md.

Refuses if any node has not converged (use --force to override) or if the
platform_release is older than the current active one (use --allow-regression,
audited). Idempotent: re-activating the current tag is a no-op.

Typical workflow:
  globular repo sync --source globulario-github --tag v1.2.250
  globular platform-upgrade v1.2.250    # converge nodes to the BOM
  globular platform activate v1.2.250   # advance the active pointer once converged`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cc, err := controllerClient()
		if err != nil {
			return err
		}
		defer func() { _ = cc.Close() }()
		client := cluster_controllerpb.NewClusterControllerServiceClient(cc)
		resp, err := client.ActivatePlatformRelease(ctxWithTimeout(), &cluster_controllerpb.ActivatePlatformReleaseRequest{
			ReleaseTag:      args[0],
			AllowRegression: platformActivateAllowRegression,
			Force:           platformActivateForce,
		})
		if err != nil {
			return err
		}
		printProto(resp)
		return nil
	},
}

func init() {
	platformActivateCmd.Flags().BoolVar(&platformActivateAllowRegression, "allow-regression", false,
		"Permit activating an older platform_release than the current active (audited)")
	platformActivateCmd.Flags().BoolVar(&platformActivateForce, "force", false,
		"Bypass the convergence gate (activate despite nodes not yet at the BOM)")
	platformCmd.AddCommand(platformActivateCmd)
	rootCmd.AddCommand(platformCmd)
}
