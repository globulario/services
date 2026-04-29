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
updates the cluster's desired state so every package version matches
the BOM — both services and infrastructure.

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
	rc := cluster_controllerpb.NewResourcesServiceClient(conn)

	var svcUpdated, infraUpdated, failed int

	for _, pkg := range idx.Packages {
		if pkg.Name == "" || pkg.Version == "" {
			continue
		}

		kind := strings.ToLower(pkg.Kind)
		label := "service"

		if kind == "infrastructure" || kind == "command" {
			label = kind
			if platformUpgradeDryRun {
				fmt.Printf("  would   %-25s -> v%-25s (%s)\n", pkg.Name, pkg.Version, label)
				infraUpdated++
				continue
			}

			// Update InfrastructureRelease via ResourcesService.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, err := rc.ApplyInfrastructureRelease(ctx,
				&cluster_controllerpb.ApplyInfrastructureReleaseRequest{
					Object: &cluster_controllerpb.InfrastructureRelease{
						Meta: &cluster_controllerpb.ObjectMeta{
							Name: "core@globular.io/" + pkg.Name,
						},
						Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
							PublisherID: "core@globular.io",
							Component:   pkg.Name,
							Version:     pkg.Version,
						},
					},
				})
			cancel()
			if err != nil {
				fmt.Printf("  FAIL   %-25s v%-25s (%s: %v)\n", pkg.Name, pkg.Version, label, err)
				failed++
				continue
			}

			fmt.Printf("  update  %-25s -> v%-25s (%s)\n", pkg.Name, pkg.Version, label)
			infraUpdated++
		} else {
			// Service packages — update via UpsertDesiredService.
			if platformUpgradeDryRun {
				fmt.Printf("  would   %-25s -> v%-25s (%s)\n", pkg.Name, pkg.Version, label)
				svcUpdated++
				continue
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, err := cc.UpsertDesiredService(ctx,
				&cluster_controllerpb.UpsertDesiredServiceRequest{
					Service: &cluster_controllerpb.DesiredService{
						ServiceId: pkg.Name,
						Version:   pkg.Version,
					},
				})
			cancel()
			if err != nil {
				fmt.Printf("  FAIL   %-25s v%-25s (%s: %v)\n", pkg.Name, pkg.Version, label, err)
				failed++
				continue
			}

			fmt.Printf("  update  %-25s -> v%-25s (%s)\n", pkg.Name, pkg.Version, label)
			svcUpdated++
		}
	}

	fmt.Printf("\n%s: %d services + %d infra/command updated, %d failed\n",
		idx.ReleaseTag, svcUpdated, infraUpdated, failed)

	if platformUpgradeDryRun {
		fmt.Println("(dry-run — no changes applied)")
	}

	if failed > 0 {
		return fmt.Errorf("%d package(s) failed to update", failed)
	}
	return nil
}

// loadBOMIndex loads the release-index.json for a given tag.
func loadBOMIndex(tag string) (*bomIndex, error) {
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
