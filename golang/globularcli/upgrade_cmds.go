package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
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
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Version     string `json:"version"`
	BuildNumber int64  `json:"build_number,omitempty"`
	BuildID     string `json:"build_id,omitempty"`
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

			// Update InfrastructureRelease directly in etcd.
			// The ResourcesService RPC is not exposed through the mesh,
			// so we write the spec.version update directly.
			err := updateInfraReleaseVersion("core@globular.io", pkg.Name, pkg.Version)
			if err != nil {
				fmt.Printf("  FAIL   %-25s v%-25s (%s: %v)\n", pkg.Name, pkg.Version, label, err)
				failed++
				continue
			}

			fmt.Printf("  update  %-25s -> v%-25s (%s)\n", pkg.Name, pkg.Version, label)
			infraUpdated++
		} else {
			// Service packages — write ServiceDesiredVersion directly to etcd.
			// We bypass the gRPC path intentionally: platform-upgrade is run during
			// early bootstrap when the mesh may not yet be routing, and infra packages
			// already use direct etcd writes for the same reason. The controller reads
			// ServiceDesiredVersion records from etcd on every reconcile tick.
			if platformUpgradeDryRun {
				fmt.Printf("  would   %-25s -> v%-25s (%s)\n", pkg.Name, pkg.Version, label)
				svcUpdated++
				continue
			}

			err := upsertServiceDesiredVersion(pkg.Name, pkg.Version, pkg.BuildNumber, pkg.BuildID)
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

// upsertServiceDesiredVersion writes a ServiceDesiredVersion record directly to etcd.
// This mirrors updateInfraReleaseVersion: both bypass gRPC so platform-upgrade works
// during early bootstrap before the mesh is routing.
func upsertServiceDesiredVersion(serviceName, version string, buildNumber int64, buildID string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}

	key := "/globular/resources/ServiceDesiredVersion/" + serviceName
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := cli.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("etcd get %s: %w", key, err)
	}

	var rec map[string]interface{}
	generation := float64(1)
	if len(resp.Kvs) > 0 {
		if err := json.Unmarshal(resp.Kvs[0].Value, &rec); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
		if m, ok := rec["meta"].(map[string]interface{}); ok {
			if g, ok := m["generation"].(float64); ok {
				generation = g + 1
			}
		}
	} else {
		rec = map[string]interface{}{
			"meta":   map[string]interface{}{},
			"spec":   map[string]interface{}{},
			"status": map[string]interface{}{},
		}
	}

	rec["meta"] = map[string]interface{}{
		"name":       serviceName,
		"generation": generation,
	}
	spec := map[string]interface{}{
		"service_name": serviceName,
		"version":      version,
	}
	if buildNumber > 0 {
		spec["build_number"] = buildNumber
	}
	if buildID != "" {
		spec["build_id"] = buildID
	}
	rec["spec"] = spec

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	_, err = cli.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("etcd put %s: %w", key, err)
	}
	return nil
}

// updateInfraReleaseVersion updates the spec.version of an InfrastructureRelease
// record in etcd. This is used instead of the gRPC RPC because the
// ResourcesService ApplyInfrastructureRelease is not exposed through the mesh.
func updateInfraReleaseVersion(publisher, component, version string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}

	key := "/globular/resources/InfrastructureRelease/" + publisher + "/" + component
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := cli.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("etcd get %s: %w", key, err)
	}

	var rel map[string]interface{}
	if len(resp.Kvs) > 0 {
		if err := json.Unmarshal(resp.Kvs[0].Value, &rel); err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
	} else {
		// Create new record.
		rel = map[string]interface{}{
			"meta": map[string]interface{}{
				"name":       publisher + "/" + component,
				"generation": float64(1),
			},
			"spec": map[string]interface{}{
				"publisher_id": publisher,
				"component":    component,
				"version":      version,
			},
			"status": map[string]interface{}{},
		}
	}

	// Update spec.version.
	spec, ok := rel["spec"].(map[string]interface{})
	if !ok {
		spec = map[string]interface{}{}
		rel["spec"] = spec
	}
	spec["version"] = version

	data, err := json.Marshal(rel)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	_, err = cli.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("etcd put %s: %w", key, err)
	}
	return nil
}
