package main

// release_promote_local_cmd.go — Dry-run planner for promoting a local package
// override into the official release pipeline.
//
// Identity lane contract:
//   local lane  — local@<host>, version with +local./−dev./−hotfix. suffix
//   official lane — core@globular.io, plain semver, enrolled in release-index.json
//
// What promote-local does NOT do (by design):
//   - Rename the local artifact in the repository
//   - Modify release-index.json
//   - Bump version numbers
//   - Push to GitHub
//   - Apply any cluster state change
//
// It is a planner: it reads the active override, inspects the artifact, and
// prints the exact sequence of commands the operator must run to produce a
// clean official release from the local build.

import (
	"fmt"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
	"context"
	"encoding/json"
	"time"
)

var (
	promoteLocalFromBuild  string
	promoteLocalAsVersion  string
	promoteLocalRepository string
)

var releasePromoteLocalCmd = &cobra.Command{
	Use:   "promote-local <service-name>",
	Short: "Plan the official release path for a local package override (dry-run only)",
	Long: `Print the exact steps required to promote a local override into the official
release pipeline. This command is DRY-RUN ONLY — it does not rename artifacts,
modify release-index.json, or change any cluster state.

The planned promotion path:
  1. Rebuild from source with official version and no local suffix
  2. Upload the new artifact to GitHub as a release asset
  3. Run: globular pkg publish --channel stable --file <artifact.tgz>
  4. Regenerate release-index.json: globular release regenerate-bom
  5. Validate the new BOM: globular release validate-index
  6. Apply the upgrade: globular platform-upgrade <new-tag>
  7. Remove the local override: globular pkg override remove <service>

Examples:
  globular release promote-local storage
  globular release promote-local storage --from-build 019e2eb5 --as-version 1.2.44
`,
	Args: cobra.ExactArgs(1),
	RunE: runReleasePromoteLocal,
}

func init() {
	releasePromoteLocalCmd.Flags().StringVar(&promoteLocalFromBuild, "from-build", "", "local build_id to promote (auto-read from active override if omitted)")
	releasePromoteLocalCmd.Flags().StringVar(&promoteLocalAsVersion, "as-version", "", "official semver to assign (e.g. 1.2.44); required if no active override")
	releasePromoteLocalCmd.Flags().StringVar(&promoteLocalRepository, "repository", "", "repository service address (auto-discovered if omitted)")
	releaseCmd.AddCommand(releasePromoteLocalCmd)
}

func runReleasePromoteLocal(cmd *cobra.Command, args []string) error {
	serviceName := strings.TrimSpace(args[0])

	// ── 1. Read active override from etcd ─────────────────────────────────────
	ov, err := readLocalOverrideForPromotion(serviceName)
	if err != nil {
		return fmt.Errorf("read active override for %s: %w", serviceName, err)
	}
	if ov == nil && promoteLocalFromBuild == "" {
		return fmt.Errorf("no active override found for %s and --from-build not provided.\n"+
			"Either activate an override first or provide --from-build <build_id>", serviceName)
	}

	buildID := promoteLocalFromBuild
	localVersion := ""
	basedOn := ""
	localPublisher := ""

	if ov != nil {
		if buildID == "" {
			buildID = ov.BuildID
		}
		localVersion = ov.Version
		localPublisher = ov.PublisherID
		basedOn = ov.BasedOnVersion
		if basedOn == "" && ov.OfficialSnapshot != nil {
			basedOn = ov.OfficialSnapshot.Version
		}
	}

	// ── 2. Look up artifact metadata from repository ──────────────────────────
	repoAddr := promoteLocalRepository
	if repoAddr == "" {
		repoAddr = config.ResolveServiceAddr("repository.PackageRepository", "")
	}

	var artifactPlatform, artifactKind string
	if repoAddr != "" && buildID != "" {
		artifactPlatform, artifactKind = lookupArtifactMeta(repoAddr, serviceName, buildID)
	}

	// ── 3. Determine target official version ─────────────────────────────────
	officialVersion := promoteLocalAsVersion
	if officialVersion == "" && basedOn != "" {
		// Suggest bumping the patch version from based_on
		officialVersion = suggestNextPatch(basedOn)
	}
	if officialVersion == "" {
		officialVersion = "<new-official-version>"
	}

	// Validate: official version must not carry a local suffix
	if hasLocalVersionSuffix(officialVersion) {
		return fmt.Errorf("--as-version %q carries a local/dev/hotfix suffix — the promoted version must be plain semver (e.g. 1.2.44)", officialVersion)
	}

	// ── 4. Print the promotion plan ───────────────────────────────────────────
	fmt.Printf("╔══════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  LOCAL OVERRIDE PROMOTION PLAN (DRY-RUN)                ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════╝\n\n")

	fmt.Printf("Service:         %s\n", serviceName)
	if localPublisher != "" {
		fmt.Printf("Local publisher: %s\n", localPublisher)
	}
	if localVersion != "" {
		fmt.Printf("Local version:   %s\n", localVersion)
	}
	if buildID != "" {
		fmt.Printf("Local build_id:  %s\n", buildID)
	}
	if basedOn != "" {
		fmt.Printf("Based on:        %s (official)\n", basedOn)
	}
	fmt.Printf("Target version:  %s (planned official)\n", officialVersion)
	if artifactPlatform != "" {
		fmt.Printf("Platform:        %s\n", artifactPlatform)
	}
	if artifactKind != "" {
		fmt.Printf("Kind:            %s\n", artifactKind)
	}

	fmt.Printf("\n── PROMOTION PATH ──────────────────────────────────────────\n\n")

	fmt.Printf("STEP 1  Rebuild from source with official version\n")
	fmt.Printf("        The local artifact %s must NOT be renamed in the repository.\n", buildID)
	fmt.Printf("        Rebuild clean, injecting the official version via ldflags:\n\n")
	fmt.Printf("          export SERVICE_VERSION=%s\n", officialVersion)
	fmt.Printf("          export BUILD_ID=$(uuidgen)\n")
	fmt.Printf("          go build -ldflags \"-X main.Version=${SERVICE_VERSION} -X main.BuildID=${BUILD_ID}\" \\\n")
	fmt.Printf("            ./%s/%s_server\n\n", serviceName, serviceName)

	fmt.Printf("STEP 2  Package the new binary\n")
	fmt.Printf("          ./build-all-packages.sh --only %s\n", serviceName)
	fmt.Printf("        Output: %s_%s_linux_amd64.tgz\n\n", serviceName, officialVersion)

	fmt.Printf("STEP 3  Upload to GitHub as a release asset\n")
	fmt.Printf("          gh release upload v<platform-tag> %s_%s_linux_amd64.tgz\n\n", serviceName, officialVersion)

	fmt.Printf("STEP 4  Publish to cluster repository\n")
	fmt.Printf("          globular pkg publish \\\n")
	fmt.Printf("            --channel stable \\\n")
	fmt.Printf("            --file %s_%s_linux_amd64.tgz \\\n", serviceName, officialVersion)
	fmt.Printf("            --repository <repo-endpoint>\n\n")

	fmt.Printf("STEP 5  Regenerate release-index.json\n")
	fmt.Printf("        Release-index.json is the BOM truth — it must be regenerated\n")
	fmt.Printf("        to include the new version. This requires a tagged GitHub release.\n")
	fmt.Printf("          globular release regenerate-bom --tag v<platform-tag>\n\n")

	fmt.Printf("STEP 6  Validate the new BOM\n")
	fmt.Printf("          globular release validate-index release-index.json\n\n")

	fmt.Printf("STEP 7  Apply the upgrade to the cluster\n")
	fmt.Printf("          globular platform-upgrade v<platform-tag>\n\n")

	fmt.Printf("STEP 8  Remove the local override\n")
	fmt.Printf("          globular pkg override remove %s\n\n", serviceName)

	fmt.Printf("── WARNINGS ────────────────────────────────────────────────\n\n")
	fmt.Printf("  • Do NOT rename the local artifact in the repository. The local build\n")
	fmt.Printf("    continues to carry its local identity (%s).\n", localVersion)
	fmt.Printf("    The official build is a SEPARATE artifact with a new build_id.\n\n")
	fmt.Printf("  • Do NOT inject --trimpath or skip ldflags. The CI pipeline injects\n")
	fmt.Printf("    version metadata. The promoted binary MUST report %s.\n\n", officialVersion)
	fmt.Printf("  • Do NOT bump the platform release tag without running regenerate-bom.\n")
	fmt.Printf("    The release-index.json is the authoritative BOM — git tags are not.\n\n")
	fmt.Printf("  • platform-upgrade will warn if the local override is still active\n")
	fmt.Printf("    when you run it. Remove the override (step 8) AFTER upgrade completes.\n\n")

	fmt.Printf("(dry-run — no changes applied)\n")
	return nil
}

// readLocalOverrideForPromotion reads the LocalOverride record from etcd.
// Returns (nil, nil) when no override exists for the service.
func readLocalOverrideForPromotion(serviceName string) (*cluster_controllerpb.LocalOverride, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return nil, nil // etcd unavailable is non-fatal for a planner
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := cluster_controllerpb.LocalOverrideKeyPrefix + serviceName
	resp, err := cli.Get(ctx, key, clientv3.WithLimit(1))
	if err != nil || len(resp.Kvs) == 0 {
		return nil, nil
	}
	var ov cluster_controllerpb.LocalOverride
	if err := json.Unmarshal(resp.Kvs[0].Value, &ov); err != nil {
		return nil, fmt.Errorf("parse override record: %w", err)
	}
	return &ov, nil
}

// lookupArtifactMeta returns the platform and kind strings for a build_id
// from the repository, or empty strings if the lookup fails.
func lookupArtifactMeta(repoAddr, serviceName, buildID string) (platform, kind string) {
	client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return "", ""
	}
	defer client.Close()
	arts, err := client.ListArtifacts()
	if err != nil {
		return "", ""
	}
	for _, a := range arts {
		if a.GetBuildId() != buildID {
			continue
		}
		ref := a.GetRef()
		if ref == nil || !strings.EqualFold(ref.GetName(), serviceName) {
			continue
		}
		return ref.GetPlatform(), ref.GetKind().String()
	}
	return "", ""
}

// suggestNextPatch increments the patch component of a semver string by 1.
// Returns the input unchanged if it doesn't parse cleanly.
func suggestNextPatch(version string) string {
	v := strings.TrimPrefix(version, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return ""
	}
	patch := parts[2]
	// Strip any pre-release/build suffix from patch
	patch = strings.SplitN(patch, "+", 2)[0]
	patch = strings.SplitN(patch, "-", 2)[0]
	var n int
	if _, err := fmt.Sscanf(patch, "%d", &n); err != nil {
		return ""
	}
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], n+1)
}
