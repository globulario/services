package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/spf13/cobra"
)

var (
	platformUpgradeDryRun bool
)

var platformUpgradeCmd = &cobra.Command{
	Use:   "platform-upgrade <release-tag>",
	Short: "Apply a platform release BOM to update desired state",
	Long: `Reads the release-index.json for the given platform release tag and
updates the cluster's desired state so every service version matches
the BOM.

This bridges Layer 1 (repository) and Layer 2 (desired state). The
reconciler handles Layer 3 (install) and Layer 4 (runtime) automatically.

Typical workflow:
  globular repo sync --source globulario-github --tag v1.0.87
  globular platform-upgrade v1.0.87

The sync imports packages. The platform-upgrade sets desired versions.`,
	Args: cobra.ExactArgs(1),
	RunE: runPlatformUpgrade,
}

func init() {
	platformUpgradeCmd.Flags().BoolVar(&platformUpgradeDryRun, "dry-run", false, "Preview changes without applying")
	rootCmd.AddCommand(platformUpgradeCmd)
}

// bomEntry matches a v2 release-index.json package entry.
type bomEntry struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Version string `json:"version"`
}

type bomIndex struct {
	SchemaVersion   string     `json:"schema_version"`
	PlatformRelease string     `json:"platform_release"`
	ReleaseTag      string     `json:"release_tag"`
	Packages        []bomEntry `json:"packages"`
}

func runPlatformUpgrade(cmd *cobra.Command, args []string) error {
	tag := args[0]
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	autoDiscoverController(cmd)

	// Load the release-index.json from the local file written at Day-0,
	// or fall back to the synced copy in etcd.
	idx, err := loadBOMIndex(tag)
	if err != nil {
		return err
	}

	fmt.Printf("Platform upgrade to %s (%s)\n", idx.PlatformRelease, idx.ReleaseTag)
	fmt.Printf("BOM packages: %d\n\n", len(idx.Packages))

	conn, err := controllerClient()
	if err != nil {
		return fmt.Errorf("connect to controller: %w", err)
	}
	defer conn.Close()

	cc := cluster_controllerpb.NewClusterControllerServiceClient(conn)

	var updated, skipped, failed int

	for _, pkg := range idx.Packages {
		if pkg.Name == "" || pkg.Version == "" {
			continue
		}

		kind := strings.ToLower(pkg.Kind)

		// Infrastructure and command packages are managed by
		// InfrastructureRelease resources. UpsertDesiredService only
		// handles SERVICE kind. Skip infra — their versions are set
		// by the release pipeline reconciler from the repository catalog.
		if kind == "infrastructure" || kind == "command" {
			skipped++
			continue
		}

		if platformUpgradeDryRun {
			fmt.Printf("  would   %-25s -> v%s\n", pkg.Name, pkg.Version)
			updated++
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := cc.UpsertDesiredService(ctx, &cluster_controllerpb.UpsertDesiredServiceRequest{
			Service: &cluster_controllerpb.DesiredService{
				ServiceId: pkg.Name,
				Version:   pkg.Version,
			},
		})
		cancel()
		if err != nil {
			fmt.Printf("  FAIL   %-25s v%s  (%v)\n", pkg.Name, pkg.Version, err)
			failed++
			continue
		}

		fmt.Printf("  update  %-25s -> v%s\n", pkg.Name, pkg.Version)
		updated++
	}

	fmt.Printf("\n%s: %d services updated, %d infra/command skipped, %d failed\n",
		idx.ReleaseTag, updated, skipped, failed)

	if platformUpgradeDryRun {
		fmt.Println("(dry-run — no changes applied)")
	}

	if failed > 0 {
		return fmt.Errorf("%d service(s) failed to update", failed)
	}
	return nil
}

// loadBOMIndex loads the release-index.json for a given tag.
// Tries the local file first (written by Day-0), then falls back to
// reading from etcd where the repository sync stores the last synced index.
func loadBOMIndex(tag string) (*bomIndex, error) {
	// Try local file (written by Day-0 installer or offline tarball)
	for _, path := range []string{
		"/var/lib/globular/release-index.json",
		"release-index.json",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var idx bomIndex
		if err := json.Unmarshal(data, &idx); err != nil {
			continue
		}
		if idx.ReleaseTag == tag {
			fmt.Printf("Using release-index.json from %s\n", path)
			return &idx, nil
		}
	}

	return nil, fmt.Errorf(
		"release-index.json for %s not found locally.\n"+
			"Download it first:\n"+
			"  curl -LO https://github.com/globulario/services/releases/download/%s/release-index.json\n"+
			"  globular platform-upgrade %s", tag, tag, tag)
}
