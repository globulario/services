// @awareness namespace=globular.platform
// @awareness component=platform_cli
// @awareness file_role=package_config_commands
// @awareness risk=medium
package main

// pkg_config_cmds.go — Phase CLI-D config ownership commands (read-only).
//
//   globular pkg config list   <name>
//   globular pkg config verify <name>
//
// These commands read the package's repository manifest and surface the
// declared configs[] entries with resolved defaults (DefaultMergeStrategy,
// preserve_on_upgrade for OPERATOR_OVERRIDE, etc.). The mutation surface
// (--accept-new, --keep-local, --restore --snapshot, conflicts resolution)
// requires the node-agent's config receipts plumbing and is wired in a
// follow-up pass — this iteration ships the read-only manifest layer so
// package authors can declare config ownership and verify-on-publish works.

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

var (
	cfgPublisher string
	cfgKind      string
	cfgPlatform  string
	cfgVersion   string
	cfgNode      string
	cfgJSON      bool
)

var pkgConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Inspect package config ownership declarations",
}

var pkgConfigListCmd = &cobra.Command{
	Use:   "list <name>",
	Short: "List config files declared by a package's manifest",
	Args:  cobra.ExactArgs(1),
	RunE:  runPkgConfigList,
}

var pkgConfigVerifyCmd = &cobra.Command{
	Use:   "verify <name>",
	Short: "Validate config declarations: every entry has a resolvable merge strategy and kind",
	Args:  cobra.ExactArgs(1),
	RunE:  runPkgConfigVerify,
}

func init() {
	defaultPlatform := fmt.Sprintf("%s_%s", runtime.GOOS, runtime.GOARCH)
	for _, c := range []*cobra.Command{pkgConfigListCmd, pkgConfigVerifyCmd} {
		c.Flags().StringVar(&cfgPublisher, "publisher", "core@globular.io", "Publisher namespace")
		c.Flags().StringVar(&cfgKind, "kind", "service", "Artifact kind")
		c.Flags().StringVar(&cfgPlatform, "platform", defaultPlatform, "Target platform")
		c.Flags().StringVar(&cfgVersion, "version", "", "Specific version (default: latest PUBLISHED)")
		c.Flags().StringVar(&cfgNode, "node", "", "Limit to one node (placeholder; node-agent integration pending)")
		c.Flags().BoolVar(&cfgJSON, "json", false, "Emit JSON output")
	}
	pkgConfigConflictsCmd.Flags().StringVar(&cfgPublisher, "publisher", "core@globular.io", "Publisher namespace")
	pkgConfigConflictsCmd.Flags().StringVar(&cfgKind, "kind", "service", "Artifact kind")
	pkgConfigConflictsCmd.Flags().StringVar(&cfgPlatform, "platform", defaultPlatform, "Target platform")
	pkgConfigConflictsCmd.Flags().StringVar(&cfgNode, "node", "", "Limit to one node")
	pkgConfigConflictsCmd.Flags().BoolVar(&cfgJSON, "json", false, "Emit JSON output")

	pkgConfigCmd.AddCommand(pkgConfigListCmd, pkgConfigVerifyCmd, pkgConfigConflictsCmd)
	pkgCmd.AddCommand(pkgConfigCmd)
}

var pkgConfigConflictsCmd = &cobra.Command{
	Use:   "conflicts <name>",
	Short: "List config-file conflicts reported by node-agents (CONFLICT receipts)",
	Args:  cobra.ExactArgs(1),
	RunE:  runPkgConfigConflicts,
}

func runPkgConfigConflicts(cmd *cobra.Command, args []string) error {
	client, err := newRepoClient()
	if err != nil {
		return err
	}
	defer client.Close()
	resp, err := client.ListConfigReceipts(&repopb.ListConfigReceiptsRequest{
		PublisherId:  cfgPublisher,
		Name:         args[0],
		Platform:     cfgPlatform,
		NodeId:       cfgNode,
		ActionFilter: repopb.ConfigReceiptAction_CONFIG_RECEIPT_CONFLICT,
		Limit:        100,
	})
	if err != nil {
		return err
	}
	if cfgJSON {
		emitJSON(resp.GetReceipts())
		if len(resp.GetReceipts()) > 0 {
			cmd.SilenceUsage = true
			return fmt.Errorf("%d unresolved config conflict(s)", len(resp.GetReceipts()))
		}
		return nil
	}
	if len(resp.GetReceipts()) == 0 {
		fmt.Printf("(no conflicts on %s/%s [%s])\n", cfgPublisher, args[0], cfgPlatform)
		return nil
	}
	fmt.Printf("%-16s %-48s %-22s %-12s %s\n",
		"NODE", "PATH", "MERGE", "TIMESTAMP", "REASON")
	for _, r := range resp.GetReceipts() {
		fmt.Printf("%-16s %-48s %-22s %-12d %s\n",
			truncStrDup(r.GetNodeId(), 16),
			truncStrDup(r.GetPath(), 48),
			truncStrDup(r.GetMergeStrategy().String(), 22),
			r.GetTimestampUnix(), r.GetReason())
	}
	cmd.SilenceUsage = true
	return fmt.Errorf("%d unresolved config conflict(s)", len(resp.GetReceipts()))
}

func loadManifestForConfig(name string) (*repopb.ArtifactManifest, error) {
	client, err := newRepoClient()
	if err != nil {
		return nil, err
	}
	defer client.Close()
	m, err := client.GetArtifactManifest(&repopb.ArtifactRef{
		PublisherId: cfgPublisher, Name: name, Version: cfgVersion,
		Platform: cfgPlatform, Kind: resolveArtifactKind(cfgKind),
	}, 0)
	if err != nil {
		return nil, fmt.Errorf("get manifest: %w", err)
	}
	return m, nil
}

func runPkgConfigList(cmd *cobra.Command, args []string) error {
	m, err := loadManifestForConfig(args[0])
	if err != nil {
		return err
	}
	if cfgJSON {
		emitJSON(m.GetConfigs())
		return nil
	}
	configs := m.GetConfigs()
	if len(configs) == 0 {
		fmt.Printf("(no config declarations on %s/%s)\n", cfgPublisher, args[0])
		return nil
	}
	fmt.Printf("%-48s %-22s %-22s %-9s %s\n",
		"PATH", "KIND", "MERGE", "SENSITIVE", "PRESERVE_ON_UPGRADE")
	for _, c := range configs {
		// Cannot resolve defaults in the CLI without depending on
		// repository_server internals. Mirror the same defaults:
		// OPERATOR_OVERRIDE → PRESERVE, SECRET → SECRET_EXTERNAL, etc.
		merge := c.GetMergeStrategy()
		if merge == repopb.MergeStrategy_MERGE_STRATEGY_UNSPECIFIED {
			merge = defaultMergeForKindCLI(c.GetConfigKind())
		}
		sensitive := c.GetSensitive() || c.GetConfigKind() == repopb.ConfigKind_CONFIG_SECRET
		preserve := c.GetPreserveOnUpgrade() ||
			c.GetConfigKind() == repopb.ConfigKind_CONFIG_OPERATOR_OVERRIDE
		path := c.GetPath()
		if sensitive {
			path = "[REDACTED]"
		}
		fmt.Printf("%-48s %-22s %-22s %-9s %v\n",
			truncStrDup(path, 48),
			strings.TrimPrefix(c.GetConfigKind().String(), "CONFIG_"),
			strings.TrimPrefix(merge.String(), "MERGE_"),
			yesNo(sensitive), preserve)
	}
	return nil
}

func runPkgConfigVerify(cmd *cobra.Command, args []string) error {
	m, err := loadManifestForConfig(args[0])
	if err != nil {
		return err
	}
	configs := m.GetConfigs()
	if len(configs) == 0 {
		fmt.Printf("(no config declarations to verify on %s/%s)\n", cfgPublisher, args[0])
		return nil
	}
	bad := 0
	for _, c := range configs {
		path := c.GetPath()
		if path == "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "config: missing path\n")
			bad++
			continue
		}
		if c.GetConfigKind() == repopb.ConfigKind_CONFIG_KIND_UNSPECIFIED {
			fmt.Fprintf(cmd.ErrOrStderr(), "config %s: kind unspecified — set explicit ConfigKind\n", path)
			bad++
		}
		if !strings.HasPrefix(path, "/") {
			fmt.Fprintf(cmd.ErrOrStderr(), "config %s: path is not absolute\n", path)
			bad++
		}
	}
	if bad > 0 {
		return fmt.Errorf("%d config declaration(s) failed validation", bad)
	}
	fmt.Printf("ok: %d config declaration(s) validated\n", len(configs))
	return nil
}

// defaultMergeForKindCLI mirrors repository_server.DefaultMergeStrategy for
// CLI rendering. Kept as a small duplicate to avoid the CLI package needing
// to import the server package.
func defaultMergeForKindCLI(k repopb.ConfigKind) repopb.MergeStrategy {
	switch k {
	case repopb.ConfigKind_CONFIG_DEFAULT:
		return repopb.MergeStrategy_MERGE_REPLACE
	case repopb.ConfigKind_CONFIG_OPERATOR_OVERRIDE:
		return repopb.MergeStrategy_MERGE_PRESERVE
	case repopb.ConfigKind_CONFIG_GENERATED:
		return repopb.MergeStrategy_MERGE_TEMPLATE_RENDER
	case repopb.ConfigKind_CONFIG_SECRET:
		return repopb.MergeStrategy_MERGE_SECRET_EXTERNAL
	case repopb.ConfigKind_CONFIG_RUNTIME_STATE:
		return repopb.MergeStrategy_MERGE_APPEND_ONLY
	}
	return repopb.MergeStrategy_MERGE_REPLACE
}
