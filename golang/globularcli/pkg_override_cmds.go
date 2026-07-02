package main

// pkg_override_cmds.go — Local package override lifecycle commands.
//
// Identity lane model:
//   official BOM  = release-index.json (never mutated)
//   effective BOM = ServiceDesiredVersion records in etcd (can be overridden)
//   override      = /globular/releases/local_overrides/{name} (stores override + snapshot)
//
// Workflow:
//   pkg override <name>        — find local artifact in repo, save snapshot, update
//                                ServiceDesiredVersion to use local publisher+version
//   pkg override remove <name> — restore ServiceDesiredVersion from snapshot, delete override
//   pkg override list          — show all active overrides

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	pkgOverrideCmd = &cobra.Command{
		Use:   "override <service-name>",
		Short: "Activate a local package override for a service",
		Long: `Activate a local package override for a service, replacing the official
desired build with a locally-published artifact.

The official release-index (BOM) is never mutated. The override stores a
snapshot of the current desired state so 'pkg override remove' can restore it.

The controller will install the local build on all eligible nodes. Doctor will
report the override as active. platform-upgrade will warn before replacing it.

Examples:
  globular pkg override storage \
    --build-id abc123... \
    --reason "test retry fix during repository bootstrap"

  globular pkg override storage \
    --build-id abc123... --publisher local@ryzen \
    --version 1.2.43+local.ryzen.1 \
    --reason "fix retry loop"
`,
		Args: cobra.ExactArgs(1),
		RunE: runPkgOverride,
	}

	pkgOverrideRemoveCmd = &cobra.Command{
		Use:   "remove <service-name>",
		Short: "Remove a local override and restore the official desired build",
		Long: `Remove an active local package override and restore the service to
its official desired state (version and publisher).

The snapshot saved by 'pkg override' is used to restore the exact
ServiceDesiredVersion that was active before the override.

Examples:
  globular pkg override remove storage
`,
		Args: cobra.ExactArgs(1),
		RunE: runPkgOverrideRemove,
	}

	pkgOverrideListCmd = &cobra.Command{
		Use:   "list",
		Short: "List active local package overrides",
		RunE:  runPkgOverrideList,
	}
)

var (
	pkgOverrideBuildID    string
	pkgOverridePublisher  string
	pkgOverrideVersion    string
	pkgOverrideReason     string
	pkgOverrideBasedOn    string
	pkgOverrideRepository string
	pkgOverrideForce      bool
)

func init() {
	pkgCmd.AddCommand(pkgOverrideCmd)
	pkgOverrideCmd.AddCommand(pkgOverrideRemoveCmd)
	pkgOverrideCmd.AddCommand(pkgOverrideListCmd)

	pkgOverrideCmd.Flags().StringVar(&pkgOverrideBuildID, "build-id", "", "build_id of the local artifact to activate (required)")
	pkgOverrideCmd.Flags().StringVar(&pkgOverridePublisher, "publisher", "", "publisher of the local artifact (auto-detected from repository if omitted)")
	pkgOverrideCmd.Flags().StringVar(&pkgOverrideVersion, "version", "", "version of the local artifact (auto-detected from repository if omitted)")
	pkgOverrideCmd.Flags().StringVar(&pkgOverrideReason, "reason", "", "human-readable reason for the override (required)")
	pkgOverrideCmd.Flags().StringVar(&pkgOverrideBasedOn, "based-on", "", "official version this is derived from (e.g. 1.2.43)")
	pkgOverrideCmd.Flags().StringVar(&pkgOverrideRepository, "repository", "", "repository service address (auto-discovered if omitted)")
	pkgOverrideCmd.Flags().BoolVar(&pkgOverrideForce, "force", false, "skip based_on compatibility check (use when BOM has moved since the local build)")
	_ = pkgOverrideCmd.MarkFlagRequired("build-id")
	_ = pkgOverrideCmd.MarkFlagRequired("reason")
}

// ── override activate ─────────────────────────────────────────────────────────

func runPkgOverride(cmd *cobra.Command, args []string) error {
	serviceName := strings.TrimSpace(args[0])

	publisher := pkgOverridePublisher
	version := pkgOverrideVersion
	buildNumber := int64(0)

	// Auto-detect publisher + version from repository if not provided.
	if publisher == "" || version == "" {
		repoAddr := pkgOverrideRepository
		if repoAddr == "" {
			repoAddr = config.ResolveServiceAddr("repository.PackageRepository", "")
		}
		if repoAddr == "" {
			return fmt.Errorf("--repository is required when publisher/version cannot be auto-detected")
		}
		p, v, bn, err := findArtifactByBuildID(repoAddr, serviceName, pkgOverrideBuildID)
		if err != nil {
			return fmt.Errorf("repository lookup for build_id %s: %w", pkgOverrideBuildID[:min8(len(pkgOverrideBuildID))], err)
		}
		if publisher == "" {
			publisher = p
		}
		if version == "" {
			version = v
		}
		buildNumber = bn
	}

	if publisher == "" {
		return fmt.Errorf("--publisher is required: could not auto-detect publisher for build_id %s", pkgOverrideBuildID)
	}
	if version == "" {
		return fmt.Errorf("--version is required: could not auto-detect version for build_id %s", pkgOverrideBuildID)
	}

	// Validate local identity. Overrides must target a repository-allocated
	// build identity; version text remains the platform version.
	if buildNumber <= 0 {
		return fmt.Errorf("local override for %s@%s must use a repository-allocated build_number; "+
			"publish locally through AllocateUpload and apply the override by build_id",
			serviceName, version)
	}

	// Read current ServiceDesiredVersion for snapshot.
	snap, err := readServiceDesiredVersionSnapshot(serviceName)
	if err != nil {
		return fmt.Errorf("read current desired state for %s: %w", serviceName, err)
	}

	hostname, _ := os.Hostname()

	basedOnVersion := pkgOverrideBasedOn
	if basedOnVersion == "" && snap != nil {
		basedOnVersion = snap.Version
	}

	// based_on is required — it documents what official build this fix patches.
	if basedOnVersion == "" {
		return fmt.Errorf("cannot determine based_on version: provide --based-on <official-version>")
	}

	// based_on compatibility check: the local build should be derived from the
	// current official desired state. If the BOM has moved, the fix may not apply
	// cleanly — use --force to override.
	if !pkgOverrideForce && snap != nil && snap.Version != "" && snap.Version != basedOnVersion {
		return fmt.Errorf(
			"compatibility mismatch: the current official desired version for %s is %s,\n"+
				"but this override is based on %s.\n\n"+
				"If you rebuilt the fix against the current official build, update --based-on to %s.\n"+
				"If you intentionally want to apply an older fix, use --force to skip this check.",
			serviceName, snap.Version, basedOnVersion, snap.Version)
	}

	ov := &cluster_controllerpb.LocalOverride{
		ServiceName:      serviceName,
		PublisherID:      publisher,
		Version:          version,
		BuildID:          pkgOverrideBuildID,
		BuildNumber:      buildNumber,
		BasedOnVersion:   basedOnVersion,
		PatchReason:      pkgOverrideReason,
		CreatedBy:        hostname,
		CreatedAtUnixS:   time.Now().Unix(),
		OfficialSnapshot: snap,
	}

	if err := writeLocalOverride(serviceName, ov); err != nil {
		return fmt.Errorf("write override record: %w", err)
	}

	if err := upsertServiceDesiredVersion(serviceName, version, buildNumber, pkgOverrideBuildID); err != nil {
		// Undo the override record if the desired state write fails.
		_ = deleteLocalOverride(serviceName)
		return fmt.Errorf("update desired state: %w", err)
	}

	fmt.Printf("override active:\n")
	fmt.Printf("  service:    %s\n", serviceName)
	fmt.Printf("  publisher:  %s\n", publisher)
	fmt.Printf("  version:    %s\n", version)
	fmt.Printf("  build_id:   %s\n", pkgOverrideBuildID)
	if basedOnVersion != "" {
		fmt.Printf("  based_on:   %s\n", basedOnVersion)
	}
	fmt.Printf("  reason:     %s\n", pkgOverrideReason)
	fmt.Printf("\nThe controller will install the local build on eligible nodes.\n")
	fmt.Printf("Run 'globular repository explain-package %s' to monitor progress.\n", serviceName)
	fmt.Printf("Run 'globular pkg override remove %s' to restore the official build.\n", serviceName)
	return nil
}

// ── override remove ───────────────────────────────────────────────────────────

func runPkgOverrideRemove(cmd *cobra.Command, args []string) error {
	serviceName := strings.TrimSpace(args[0])

	ov, err := readLocalOverride(serviceName)
	if err != nil {
		return fmt.Errorf("read override record: %w", err)
	}
	if ov == nil {
		return fmt.Errorf("no active override found for %s", serviceName)
	}

	snap := ov.OfficialSnapshot
	if snap == nil {
		return fmt.Errorf("override record for %s has no official snapshot — cannot safely restore; delete the override key manually if needed", serviceName)
	}

	staleMetadataOnly := false
	if err := upsertServiceDesiredVersion(snap.ServiceName, snap.Version, snap.BuildNumber, snap.BuildID); err != nil {
		current, readErr := readServiceDesiredVersionSnapshot(serviceName)
		if readErr != nil {
			return fmt.Errorf("restore desired state: %w; additionally failed to read current desired state before deciding whether stale override metadata can be cleared: %v", err, readErr)
		}
		if desiredSnapshotMatches(current, snap) {
			fmt.Fprintf(os.Stderr, "WARN: restore RPC failed, but current desired state already matches the saved official snapshot; clearing stale override metadata: %v\n", err)
		} else if currentDesiredIsOutsideOverride(current, ov) {
			fmt.Fprintf(os.Stderr, "WARN: restore RPC failed and saved snapshot is not restorable, but current desired state is already outside the local override identity; clearing stale override metadata: %v\n", err)
			staleMetadataOnly = true
		} else {
			return fmt.Errorf("restore desired state: %w", err)
		}
	}

	if err := deleteLocalOverride(serviceName); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: desired state restored but override record delete failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "      Delete manually: etcdctl del %s%s\n", cluster_controllerpb.LocalOverrideKeyPrefix, serviceName)
	}

	fmt.Printf("override removed:\n")
	fmt.Printf("  service:  %s\n", serviceName)
	if staleMetadataOnly {
		current, err := readServiceDesiredVersionSnapshot(serviceName)
		if err == nil && current != nil {
			fmt.Printf("  action:   cleared stale override metadata; desired already version=%s build=%d build_id=%s\n",
				current.Version, current.BuildNumber, current.BuildID)
		} else {
			fmt.Printf("  action:   cleared stale override metadata\n")
		}
	} else {
		fmt.Printf("  restored: publisher=%s  version=%s\n",
			func() string {
				if snap.PublisherID == "" {
					return "core@globular.io"
				}
				return snap.PublisherID
			}(),
			snap.Version,
		)
		fmt.Printf("\nThe controller will reinstall the official build on eligible nodes.\n")
	}
	return nil
}

// ── override list ─────────────────────────────────────────────────────────────

func runPkgOverrideList(_ *cobra.Command, _ []string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := cli.Get(ctx, cluster_controllerpb.LocalOverrideKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("etcd get overrides: %w", err)
	}
	if len(resp.Kvs) == 0 {
		fmt.Println("no active local overrides")
		return nil
	}

	fmt.Printf("%-20s  %-20s  %-35s  %s\n", "SERVICE", "PUBLISHER", "VERSION", "REASON")
	fmt.Printf("%-20s  %-20s  %-35s  %s\n",
		strings.Repeat("-", 20), strings.Repeat("-", 20), strings.Repeat("-", 35), strings.Repeat("-", 30))

	for _, kv := range resp.Kvs {
		var ov cluster_controllerpb.LocalOverride
		if err := json.Unmarshal(kv.Value, &ov); err != nil {
			continue
		}
		reason := ov.PatchReason
		if len(reason) > 30 {
			reason = reason[:27] + "..."
		}
		fmt.Printf("%-20s  %-20s  %-35s  %s\n",
			ov.ServiceName, ov.PublisherID, ov.Version, reason)
	}
	return nil
}

// ── etcd helpers ─────────────────────────────────────────────────────────────

func writeLocalOverride(serviceName string, ov *cluster_controllerpb.LocalOverride) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	data, err := json.Marshal(ov)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	key := cluster_controllerpb.LocalOverrideKeyPrefix + serviceName
	if _, err := cli.Put(ctx, key, string(data)); err != nil {
		return fmt.Errorf("etcd put %s: %w", key, err)
	}
	return nil
}

func readLocalOverride(serviceName string) (*cluster_controllerpb.LocalOverride, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	key := cluster_controllerpb.LocalOverrideKeyPrefix + serviceName
	resp, err := cli.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("etcd get %s: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var ov cluster_controllerpb.LocalOverride
	if err := json.Unmarshal(resp.Kvs[0].Value, &ov); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &ov, nil
}

func deleteLocalOverride(serviceName string) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	key := cluster_controllerpb.LocalOverrideKeyPrefix + serviceName
	_, err = cli.Delete(ctx, key)
	return err
}

// readServiceDesiredVersionSnapshot reads the current ServiceDesiredVersion
// from etcd and returns a LocalOverrideSnapshot for use as a restore point.
func readServiceDesiredVersionSnapshot(serviceName string) (*cluster_controllerpb.LocalOverrideSnapshot, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	key := "/globular/resources/ServiceDesiredVersion/" + serviceName
	resp, err := cli.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("etcd get %s: %w", key, err)
	}
	if len(resp.Kvs) == 0 {
		// No existing desired state — snapshot will be empty; remove restores to nothing.
		return &cluster_controllerpb.LocalOverrideSnapshot{ServiceName: serviceName}, nil
	}

	var rec struct {
		Meta struct {
			Generation float64 `json:"generation"`
		} `json:"meta"`
		Spec struct {
			ServiceName string  `json:"service_name"`
			Version     string  `json:"version"`
			BuildNumber float64 `json:"build_number"`
			BuildID     string  `json:"build_id"`
			PublisherID string  `json:"publisher_id"`
		} `json:"spec"`
	}
	if err := json.Unmarshal(resp.Kvs[0].Value, &rec); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &cluster_controllerpb.LocalOverrideSnapshot{
		ServiceName: serviceName,
		Version:     rec.Spec.Version,
		BuildNumber: int64(rec.Spec.BuildNumber),
		BuildID:     rec.Spec.BuildID,
		PublisherID: rec.Spec.PublisherID,
		Generation:  int64(rec.Meta.Generation),
	}, nil
}

func desiredSnapshotMatches(current, official *cluster_controllerpb.LocalOverrideSnapshot) bool {
	if current == nil || official == nil {
		return false
	}
	if strings.TrimSpace(current.ServiceName) != strings.TrimSpace(official.ServiceName) {
		return false
	}
	if strings.TrimSpace(current.Version) != strings.TrimSpace(official.Version) {
		return false
	}
	if current.BuildNumber != official.BuildNumber {
		return false
	}
	if strings.TrimSpace(current.BuildID) != strings.TrimSpace(official.BuildID) {
		return false
	}
	if effectivePublisher(current.PublisherID) != effectivePublisher(official.PublisherID) {
		return false
	}
	return true
}

func effectivePublisher(publisher string) string {
	publisher = strings.TrimSpace(publisher)
	if publisher == "" {
		return "core@globular.io"
	}
	return publisher
}

func currentDesiredIsOutsideOverride(current *cluster_controllerpb.LocalOverrideSnapshot, ov *cluster_controllerpb.LocalOverride) bool {
	if current == nil || ov == nil {
		return false
	}
	if strings.TrimSpace(current.ServiceName) != strings.TrimSpace(ov.ServiceName) {
		return false
	}
	currentVersion := strings.TrimSpace(current.Version)
	if currentVersion == "" || hasLocalVersionSuffix(currentVersion) {
		return false
	}
	if currentVersion == strings.TrimSpace(ov.Version) {
		return false
	}
	if strings.TrimSpace(current.BuildID) != "" && strings.TrimSpace(current.BuildID) == strings.TrimSpace(ov.BuildID) {
		return false
	}
	if current.BuildNumber > 0 && current.BuildNumber == ov.BuildNumber && strings.TrimSpace(ov.Version) == currentVersion {
		return false
	}
	return true
}

// findArtifactByBuildID scans the repository ListArtifacts response for an
// artifact with the given build_id and service name, returning its publisher,
// version, and build_number.
func findArtifactByBuildID(repoAddr, serviceName, buildID string) (publisher, version string, buildNumber int64, err error) {
	client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return "", "", 0, fmt.Errorf("connect to repository: %w", err)
	}
	defer client.Close()

	arts, err := client.ListArtifacts()
	if err != nil {
		return "", "", 0, fmt.Errorf("list artifacts: %w", err)
	}

	for _, a := range arts {
		if a.GetBuildId() != buildID {
			continue
		}
		ref := a.GetRef()
		if ref == nil {
			continue
		}
		if !strings.EqualFold(ref.GetName(), serviceName) {
			continue
		}
		return ref.GetPublisherId(), ref.GetVersion(), a.GetBuildNumber(), nil
	}
	return "", "", 0, fmt.Errorf("build_id %s not found for service %q in repository", buildID, serviceName)
}

// hasLocalVersionSuffix returns true for versions carrying a local/dev/hotfix
// pre-release or build-metadata label (e.g. 1.2.43+local.ryzen.1).
func hasLocalVersionSuffix(version string) bool {
	lower := strings.ToLower(version)
	return strings.Contains(lower, "+local.") ||
		strings.Contains(lower, "-dev.") ||
		strings.Contains(lower, "-hotfix.") ||
		strings.Contains(lower, "+dev.") ||
		strings.Contains(lower, "+hotfix.")
}
