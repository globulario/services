package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

var (
	pkgInfoJSON      bool
	pkgInfoPublisher string
)

var pkgInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show kind, desired version, and per-node install status for a package",
	Long: `Aggregates the repository catalog, cluster desired-state, and per-node
installed state into a single answer — "what is this package, where is it,
is anything broken?"

Examples:
  globular pkg info cluster-controller
  globular pkg info claude
  globular pkg info envoy --publisher core@globular.io
  globular pkg info minio --json
`,
	Args: cobra.ExactArgs(1),
	RunE: runPkgInfo,
}

func init() {
	pkgCmd.AddCommand(pkgInfoCmd)
	pkgInfoCmd.Flags().BoolVar(&pkgInfoJSON, "json", false, "Output as JSON")
	pkgInfoCmd.Flags().StringVar(&pkgInfoPublisher, "publisher", "", "Filter by publisher ID")
}

func runPkgInfo(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(args[0])
	if name == "" {
		return fmt.Errorf("package name required")
	}

	resolveRepositoryAddr(cmd)
	conn, err := dialRepository()
	if err != nil {
		return fmt.Errorf("connect to repository: %w", err)
	}
	defer conn.Close()

	client := repopb.NewPackageRepositoryClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), rootCfg.timeout)
	defer cancel()

	rsp, err := client.DescribePackage(ctx, &repopb.DescribePackageRequest{
		Name:        name,
		PublisherId: pkgInfoPublisher,
	})
	if err != nil {
		return fmt.Errorf("DescribePackage: %w", err)
	}
	info := rsp.GetInfo()
	if info == nil {
		return fmt.Errorf("no info returned")
	}

	if pkgInfoJSON {
		out := infoToJSON(info)
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	printPkgInfo(info)
	return nil
}

func infoToJSON(info *repopb.PackageInfo) map[string]interface{} {
	desired := map[string]interface{}{"present": false}
	if d := info.GetDesired(); d != nil && d.GetPresent() {
		desired = map[string]interface{}{
			"present":    true,
			"version":    d.GetVersion(),
			"generation": d.GetGeneration(),
			"publisher":  d.GetPublisher(),
		}
	}
	installed := make([]map[string]interface{}, 0, len(info.GetInstalledOn()))
	for _, n := range info.GetInstalledOn() {
		installed = append(installed, map[string]interface{}{
			"node_id":      n.GetNodeId(),
			"version":      n.GetVersion(),
			"status":       n.GetStatus(),
			"installed_at": n.GetInstalledAt(),
		})
	}
	failing := make([]map[string]interface{}, 0, len(info.GetFailingOn()))
	for _, n := range info.GetFailingOn() {
		failing = append(failing, map[string]interface{}{
			"node_id": n.GetNodeId(),
			"version": n.GetVersion(),
			"status":  n.GetStatus(),
		})
	}
	return map[string]interface{}{
		"name":           info.GetName(),
		"kind":           info.GetKind().String(),
		"publisher":      info.GetPublisher(),
		"versions":       info.GetVersions(),
		"latest_version": info.GetLatestVersion(),
		"desired":        desired,
		"installed_on":   installed,
		"failing_on":     failing,
		"source":         info.GetSource(),
		"observed_at":    info.GetObservedAt(),
	}
}

func printPkgInfo(info *repopb.PackageInfo) {
	fmt.Printf("name:            %s\n", info.GetName())
	fmt.Printf("kind:            %s\n", info.GetKind().String())
	fmt.Printf("publisher:       %s\n", info.GetPublisher())
	fmt.Printf("versions (repo): %s\n", strings.Join(info.GetVersions(), ", "))
	if info.GetLatestVersion() != "" {
		fmt.Printf("latest:          %s\n", info.GetLatestVersion())
	}

	d := info.GetDesired()
	if d != nil && d.GetPresent() {
		fmt.Printf("desired:         %s  (gen %d)\n", d.GetVersion(), d.GetGeneration())
	} else {
		fmt.Printf("desired:         <none>\n")
	}

	if installed := info.GetInstalledOn(); len(installed) > 0 {
		fmt.Printf("installed_on:    %d node(s)\n", len(installed))
		for _, n := range installed {
			fmt.Printf("  • %s  %s  %s\n", shortNode(n.GetNodeId()), n.GetVersion(), n.GetStatus())
		}
	} else {
		fmt.Printf("installed_on:    none\n")
	}
	if failing := info.GetFailingOn(); len(failing) > 0 {
		fmt.Printf("failing_on:      %d node(s)\n", len(failing))
		for _, n := range failing {
			fmt.Printf("  ✕ %s  %s  %s\n", shortNode(n.GetNodeId()), n.GetVersion(), n.GetStatus())
		}
	}

	// Drift / alignment summary.
	if d != nil && d.GetPresent() {
		drift := []string{}
		for _, n := range info.GetInstalledOn() {
			if n.GetVersion() != d.GetVersion() {
				drift = append(drift, fmt.Sprintf("%s@%s", shortNode(n.GetNodeId()), n.GetVersion()))
			}
		}
		if len(drift) > 0 {
			fmt.Printf("⚠ drift:         %d node(s) not at desired: %s\n", len(drift), strings.Join(drift, ", "))
		} else if len(info.GetInstalledOn()) > 0 {
			fmt.Printf("✓ aligned:       all %d node(s) at desired\n", len(info.GetInstalledOn()))
		}
	}

	fmt.Printf("source:          %s\n", info.GetSource())
}

// shortNode abbreviates a UUID-ish node_id to its first 8 chars. Node IDs
// aren't human-friendly; callers chain into node_resolve for hostname.
func shortNode(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
