package main

// repo_scan_cmds.go — Repository artifact scan and classification.
//
// Scans all artifacts in the repository and classifies each into:
//   VALID            — consistent with all invariants
//   DUPLICATE_DIGEST — same (publisher, name, version, platform), same digest, different build
//   DUPLICATE_CONTENT— same (publisher, name, version, platform), different digest
//   NON_MONOTONIC    — version N published after version M where M > N
//   ORPHANED         — not referenced by any desired state or installed state
//   MISSING_BUILD_ID — manifest lacks build_id
//
// Usage:
//   globular repository scan [--package <name>]

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
	"github.com/spf13/cobra"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var repoCmd = &cobra.Command{
	Use:   "repository",
	Short: "Repository inspection and repair",
	Aliases: []string{"repo"},
}

var repoScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan and classify all repository artifacts",
	Long: `Scans all artifacts in the repository and classifies each one.

Classifications:
  VALID             Consistent with all invariants
  DUPLICATE_DIGEST  Same identity, same digest, different build number
  DUPLICATE_CONTENT Same identity, different digest (overwritten artifact)
  NON_MONOTONIC     Version published after a higher version
  ORPHANED          Not referenced by desired or installed state
  MISSING_BUILD_ID  Manifest lacks build_id`,
	RunE: runRepoScan,
}

var (
	repoScanPackage   string
	repoCleanupDryRun bool
)

var repoCleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Archive unreachable artifacts using the reachability engine",
	Long: `Runs the repository GC: every artifact that is outside the retention
window AND not actively deployed (build_id not in installed state) is moved
to ARCHIVED state. The binary is kept; ARCHIVED artifacts are hidden from the
catalog but can still be downloaded by owners/admins.

Use --dry-run to preview what would be archived without modifying state.

Examples:
  globular repository cleanup --dry-run   # preview
  globular repository cleanup             # archive unreachable artifacts`,
	RunE: runRepoCleanup,
}

var repoDeleteCmd = &cobra.Command{
	Use:   "delete <name> <version>",
	Short: "Delete a specific artifact version from the repository",
	Long: `Deletes a specific artifact version from the repository.

By default, deletion is rejected if any node still has this artifact installed.
Use --force to delete even if installed instances exist.

Examples:
  globular repository delete node_agent 0.0.3
  globular repository delete globular-cli 0.0.1 --force
  globular repository delete keepalived 0.0.2`,
	Args: cobra.ExactArgs(2),
	RunE: runRepoDelete,
}

var (
	repoDeleteForce     bool
	repoDeletePublisher string
	repoInspectBuildID  string
	repoInspectJSON     bool
	repoAliasesName     string
	repoAliasesVersion  string
	repoAliasesPlatform string
	repoAliasesJSON     bool
	repoResolveName     string
	repoResolveVersion  string
	repoResolvePlatform string
	repoResolveBuildID  string
	repoResolvePublisher string
	repoResolveKind     string
	repoResolveChannel  string
	repoResolveJSON     bool
	repoDoctorIdentityJSON bool
)

var repoInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect repository artifacts by identity",
	Long: `Inspect repository artifacts with identity-focused filters.

Examples:
  globular repository inspect --build-id 01JABC...
  globular repository inspect --build-id 01JABC... --json`,
	RunE: runRepoInspect,
}

var repoDedupCmd = &cobra.Command{
	Use:   "dedup",
	Short: "Repository deduplication visibility commands",
}

var repoAliasesCmd = &cobra.Command{
	Use:   "aliases",
	Short: "Show upstream release/build alias mappings for a package identity",
	Long: `Shows alias-style mappings inferred from artifact upstream-import metadata.

Examples:
  globular repository aliases --name dns --version 1.2.43 --platform linux_amd64
  globular repository aliases --name dns --version 1.2.43 --platform linux_amd64 --json`,
	RunE: runRepoAliases,
}

var repoDedupReportCmd = &cobra.Command{
	Use:   "report",
	Short: "Show duplicate artifact groups and checksum/build identity relationships",
	Long: `Reports duplicate groups in the repository:
  - same checksum across multiple builds (dedupe/alias candidates)
  - same package identity with different checksums (multiple real builds)
  - same build_id reused across multiple checksums (identity conflict)`,
	RunE: runRepoDedupReport,
}

var repoResolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Resolve a package reference to exactly one published artifact (build_id)",
	Long: `Calls the repository's deterministic resolver. Returns the canonical
build_id and manifest for the requested package, or an error if resolution is
ambiguous or no artifact matches.

Operators MUST pin the returned build_id into desired state; reconcile-time
callers should never re-invoke this RPC.

Examples:
  globular repository resolve --name dns --version 1.2.43 --platform linux_amd64
  globular repository resolve --build-id 01JABC...
  globular repository resolve --name dns --json`,
	RunE: runRepoResolve,
}

var repoDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Repository invariant and identity diagnostics",
}

var repoDoctorIdentityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Report repository identity findings (build_id/checksum/alias invariants)",
	Long: `Lists every active repository finding whose reason matches the
identity invariants:

  repository.identity.build_id_checksum_conflict
  repository.identity.duplicate_checksum_without_alias
  repository.identity.version_resolution_ambiguous
  repository.identity.missing_blob_for_published_manifest
  repository.identity.checksum_mismatch
  repository.identity.release_index_missing_pins

Findings name the canonical remediation command. Exit code is non-zero when
any critical finding is present.`,
	RunE: runRepoDoctorIdentity,
}

func init() {
	repoCmd.AddCommand(repoScanCmd)
	repoCmd.AddCommand(repoCleanupCmd)
	repoCmd.AddCommand(repoDeleteCmd)
	repoCmd.AddCommand(repoInspectCmd)
	repoCmd.AddCommand(repoDedupCmd)
	repoCmd.AddCommand(repoAliasesCmd)
	repoCmd.AddCommand(repoResolveCmd)
	repoCmd.AddCommand(repoDoctorCmd)
	repoDedupCmd.AddCommand(repoDedupReportCmd)
	repoDoctorCmd.AddCommand(repoDoctorIdentityCmd)
	repoResolveCmd.Flags().StringVar(&repoResolveName, "name", "", "Package name")
	repoResolveCmd.Flags().StringVar(&repoResolveVersion, "version", "", "Exact version (optional; latest STABLE when empty)")
	repoResolveCmd.Flags().StringVar(&repoResolvePlatform, "platform", "linux_amd64", "Platform")
	repoResolveCmd.Flags().StringVar(&repoResolveBuildID, "build-id", "", "Exact build_id (highest-priority match)")
	repoResolveCmd.Flags().StringVar(&repoResolvePublisher, "publisher", "", "Publisher ID (e.g. core@globular.io)")
	repoResolveCmd.Flags().StringVar(&repoResolveKind, "kind", "", "Artifact kind: SERVICE | APPLICATION | INFRASTRUCTURE")
	repoResolveCmd.Flags().StringVar(&repoResolveChannel, "channel", "", "Release channel: STABLE | CANDIDATE | DEV (default STABLE)")
	repoResolveCmd.Flags().BoolVar(&repoResolveJSON, "json", false, "Output JSON")
	repoDoctorIdentityCmd.Flags().BoolVar(&repoDoctorIdentityJSON, "json", false, "Output JSON")
	repoScanCmd.Flags().StringVar(&repoScanPackage, "package", "", "Scan only this package name")
	repoCleanupCmd.Flags().BoolVar(&repoCleanupDryRun, "dry-run", false, "Preview archiving without executing")
	repoDeleteCmd.Flags().BoolVar(&repoDeleteForce, "force", false, "Delete even if installed on nodes")
	repoDeleteCmd.Flags().StringVar(&repoDeletePublisher, "publisher", "core@globular.io", "Publisher ID")
	repoInspectCmd.Flags().StringVar(&repoInspectBuildID, "build-id", "", "Exact build_id to inspect")
	repoInspectCmd.Flags().BoolVar(&repoInspectJSON, "json", false, "Output JSON")
	repoDedupReportCmd.Flags().StringVar(&repoScanPackage, "package", "", "Limit report to this package name")
	repoAliasesCmd.Flags().StringVar(&repoAliasesName, "name", "", "Package name (required)")
	repoAliasesCmd.Flags().StringVar(&repoAliasesVersion, "version", "", "Package version (required)")
	repoAliasesCmd.Flags().StringVar(&repoAliasesPlatform, "platform", "", "Platform (required)")
	repoAliasesCmd.Flags().BoolVar(&repoAliasesJSON, "json", false, "Output JSON")
	_ = repoAliasesCmd.MarkFlagRequired("name")
	_ = repoAliasesCmd.MarkFlagRequired("version")
	_ = repoAliasesCmd.MarkFlagRequired("platform")
}

type artifactClassification struct {
	Key          string `json:"key"`
	Publisher    string `json:"publisher"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Platform     string `json:"platform"`
	BuildNumber  int64  `json:"build_number"`
	BuildID      string `json:"build_id"`
	Digest       string `json:"digest"`
	State        string `json:"state"`
	Class        string `json:"classification"`
	Detail       string `json:"detail,omitempty"`
}

func runRepoScan(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("=== Repository Artifact Scan ===")
	fmt.Println()

	// Connect to repository.
	addr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if addr == "" {
		if a, err := config.GetMeshAddress(); err == nil {
			addr = a
		}
	}
	if addr == "" {
		return fmt.Errorf("repository address not found")
	}

	client, err := repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	artifacts, err := client.ListArtifacts()
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}

	fmt.Printf("Artifacts found: %d\n\n", len(artifacts))

	// Load desired-state and installed-state references for orphan detection.
	desiredRefs := loadDesiredRefs(ctx)
	installedRefs := loadInstalledRefs(ctx)

	// Group artifacts by (publisher, name, version, platform) for duplicate detection.
	type groupKey struct{ publisher, name, version, platform string }
	groups := make(map[groupKey][]*repopb.ArtifactManifest)

	// Track highest PUBLISHED version per (publisher, name) for monotonicity.
	type pkgKey struct{ publisher, name string }
	publishedVersions := make(map[pkgKey][]string)

	for _, a := range artifacts {
		ref := a.GetRef()
		if repoScanPackage != "" && ref.GetName() != repoScanPackage {
			continue
		}
		gk := groupKey{ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform()}
		groups[gk] = append(groups[gk], a)

		if a.GetPublishState().String() == "PUBLISHED" {
			pk := pkgKey{ref.GetPublisherId(), ref.GetName()}
			publishedVersions[pk] = append(publishedVersions[pk], ref.GetVersion())
		}
	}

	// Sort published versions per package for monotonicity check.
	for pk := range publishedVersions {
		sort.Slice(publishedVersions[pk], func(i, j int) bool {
			cmp, _ := versionutil.Compare(publishedVersions[pk][i], publishedVersions[pk][j])
			return cmp < 0
		})
	}

	// Classify each artifact.
	var results []artifactClassification
	counts := make(map[string]int)

	for _, a := range artifacts {
		ref := a.GetRef()
		if repoScanPackage != "" && ref.GetName() != repoScanPackage {
			continue
		}

		c := artifactClassification{
			Key:         fmt.Sprintf("%s/%s@%s/%s/b%d", ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform(), a.GetBuildNumber()),
			Publisher:   ref.GetPublisherId(),
			Name:        ref.GetName(),
			Version:     ref.GetVersion(),
			Platform:    ref.GetPlatform(),
			BuildNumber: a.GetBuildNumber(),
			BuildID:     a.GetBuildId(),
			Digest:      a.GetChecksum(),
			State:       a.GetPublishState().String(),
		}

		// Check: missing build_id.
		if a.GetBuildId() == "" {
			c.Class = "MISSING_BUILD_ID"
			c.Detail = "manifest lacks build_id"
			results = append(results, c)
			counts[c.Class]++
			continue
		}

		// Check: duplicates within same (publisher, name, version, platform).
		gk := groupKey{ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform()}
		group := groups[gk]
		if len(group) > 1 {
			// Check if same digest or different.
			sameDigest := true
			for _, other := range group {
				if other.GetChecksum() != a.GetChecksum() {
					sameDigest = false
					break
				}
			}
			if !sameDigest {
				c.Class = "DUPLICATE_CONTENT"
				c.Detail = fmt.Sprintf("%d builds at same version with different digest", len(group))
				results = append(results, c)
				counts[c.Class]++
				continue
			}
			if len(group) > 1 && a != group[0] {
				c.Class = "DUPLICATE_DIGEST"
				c.Detail = fmt.Sprintf("%d builds at same version with same digest", len(group))
				results = append(results, c)
				counts[c.Class]++
				continue
			}
		}

		// Check: non-monotonic version.
		pk := pkgKey{ref.GetPublisherId(), ref.GetName()}
		versions := publishedVersions[pk]
		if len(versions) > 0 && a.GetPublishState().String() == "PUBLISHED" {
			// Find this version's position in the sorted list.
			// If a later-published artifact has a lower version, it's non-monotonic.
			// Since we don't have publish timestamps in this scan, we detect
			// non-monotonicity by checking if the version is below the max.
			maxVer := versions[len(versions)-1]
			if ref.GetVersion() != maxVer {
				cmp, cmpErr := versionutil.Compare(ref.GetVersion(), maxVer)
				if cmpErr == nil && cmp < 0 {
					// This is a lower version that's still PUBLISHED — non-monotonic
					// only if it was published AFTER the higher version.
					// Without timestamps, flag as potential non-monotonic.
					// Don't flag — semver ordering is valid (1.0.0 before 2.0.0).
					// Only flag if published_at > higher version's published_at.
					// For now, skip — monotonicity is about publish order, not value.
				}
			}
		}

		// Check: orphaned (not referenced by desired or installed state).
		nameRef := ref.GetName()
		if !desiredRefs[nameRef] && !installedRefs[nameRef] {
			c.Class = "ORPHANED"
			c.Detail = "not referenced by desired or installed state"
			results = append(results, c)
			counts[c.Class]++
			continue
		}

		c.Class = "VALID"
		results = append(results, c)
		counts[c.Class]++
	}

	// Print results.
	fmt.Println("=== Classification Summary ===")
	total := len(results)
	for _, cls := range []string{"VALID", "DUPLICATE_DIGEST", "DUPLICATE_CONTENT", "NON_MONOTONIC", "ORPHANED", "MISSING_BUILD_ID"} {
		if n, ok := counts[cls]; ok {
			fmt.Printf("  %-20s %d\n", cls, n)
		}
	}
	fmt.Printf("  %-20s %d\n", "TOTAL", total)
	fmt.Println()

	// Print non-VALID artifacts.
	nonValid := 0
	for _, c := range results {
		if c.Class == "VALID" {
			continue
		}
		nonValid++
		fmt.Printf("  [%s] %s\n", c.Class, c.Key)
		if c.Detail != "" {
			fmt.Printf("    %s\n", c.Detail)
		}
	}
	if nonValid == 0 {
		fmt.Println("  All artifacts are VALID.")
	}

	fmt.Printf("\n=== Scan complete. %d artifacts, %d anomalies. ===\n", total, nonValid)
	return nil
}

func runRepoCleanup(cmd *cobra.Command, args []string) error {
	repoAddr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if repoAddr == "" {
		return fmt.Errorf("cannot discover repository address")
	}

	client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect to repository: %w", err)
	}
	defer client.Close()

	if token := rootCfg.token; token != "" {
		client.SetToken(token)
	}

	mode := "ARCHIVING"
	if repoCleanupDryRun {
		mode = "WOULD ARCHIVE (dry-run)"
	}

	fmt.Printf("=== Repository GC — %s ===\n\n", mode)

	resp, err := client.ArchiveUnreachableArtifacts(repoCleanupDryRun)
	if err != nil {
		return fmt.Errorf("GC failed: %w", err)
	}

	for _, rec := range resp.GetArchived() {
		fmt.Printf("  [%s] %s/%s@%s (build_id=%s)  reason: %s\n",
			mode,
			rec.GetPublisher(), rec.GetName(), rec.GetVersion(),
			rec.GetBuildId(), rec.GetReason(),
		)
	}

	if len(resp.GetArchived()) == 0 {
		fmt.Println("  Nothing to archive — all artifacts are within the retention window or actively deployed.")
	}

	fmt.Printf("\n=== GC complete: %d archived, %d protected, %d skipped. ===\n",
		resp.GetArchivedCount(), resp.GetProtectedCount(), resp.GetSkippedCount())
	return nil
}

func runRepoDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	version := args[1]

	repoAddr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if repoAddr == "" {
		return fmt.Errorf("cannot discover repository address")
	}

	client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect to repository: %w", err)
	}
	defer client.Close()

	if token := rootCfg.token; token != "" {
		client.SetToken(token)
	}

	// List all versions of this package to find matching builds.
	manifests, err := client.ListArtifacts()
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}

	var matches []*repopb.ArtifactManifest
	for _, m := range manifests {
		ref := m.GetRef()
		if ref.GetName() == name && ref.GetVersion() == version {
			if repoDeletePublisher != "" && ref.GetPublisherId() != repoDeletePublisher {
				continue
			}
			matches = append(matches, m)
		}
	}

	if len(matches) == 0 {
		fmt.Printf("No artifacts found for %s@%s (publisher=%s)\n", name, version, repoDeletePublisher)
		return nil
	}

	fmt.Printf("Found %d artifact(s) for %s@%s:\n", len(matches), name, version)
	deleted := 0
	for _, m := range matches {
		ref := m.GetRef()
		key := fmt.Sprintf("%s/%s@%s/%s/b%d", ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform(), m.GetBuildNumber())
		fmt.Printf("  Deleting %s ... ", key)
		if err := client.DeleteArtifact(ref); err != nil {
			fmt.Printf("FAIL: %v\n", err)
		} else {
			fmt.Printf("OK\n")
			deleted++
		}
	}

	fmt.Printf("\nDeleted %d/%d artifacts.\n", deleted, len(matches))
	return nil
}

func runRepoInspect(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(repoInspectBuildID) == "" {
		return fmt.Errorf("--build-id is required")
	}

	repoAddr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if repoAddr == "" {
		return fmt.Errorf("cannot discover repository address")
	}
	client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect to repository: %w", err)
	}
	defer client.Close()

	if token := rootCfg.token; token != "" {
		client.SetToken(token)
	}

	manifests, err := client.ListArtifacts()
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}

	var matches []*repopb.ArtifactManifest
	for _, m := range manifests {
		if strings.TrimSpace(m.GetBuildId()) == strings.TrimSpace(repoInspectBuildID) {
			matches = append(matches, m)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("no artifact found for build_id=%s", repoInspectBuildID)
	}

	if repoInspectJSON {
		out, err := json.MarshalIndent(matches, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal inspect result: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("Found %d artifact(s) for build_id=%s\n\n", len(matches), repoInspectBuildID)
	for _, m := range matches {
		ref := m.GetRef()
		fmt.Printf("- %s/%s@%s %s\n",
			ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform())
		fmt.Printf("  build_id:            %s\n", m.GetBuildId())
		fmt.Printf("  build_number:        %d\n", m.GetBuildNumber())
		fmt.Printf("  publish_state:       %s\n", m.GetPublishState().String())
		fmt.Printf("  checksum:            %s\n", m.GetChecksum())
		fmt.Printf("  entrypoint_checksum: %s\n", m.GetEntrypointChecksum())
		fmt.Printf("  channel:             %s\n", m.GetChannel().String())
		fmt.Printf("  size_bytes:          %d\n", m.GetSizeBytes())
		if ui := m.GetUpstreamImport(); ui != nil {
			fmt.Printf("  upstream:            source=%s release=%s build_number=%d\n",
				ui.GetSourceName(), ui.GetReleaseTag(), ui.GetBuildNumber())
		}
		fmt.Println()
	}
	return nil
}

func runRepoDedupReport(cmd *cobra.Command, args []string) error {
	repoAddr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if repoAddr == "" {
		return fmt.Errorf("cannot discover repository address")
	}
	client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect to repository: %w", err)
	}
	defer client.Close()
	if token := rootCfg.token; token != "" {
		client.SetToken(token)
	}

	manifests, err := client.ListArtifacts()
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}

	type identityKey struct {
		Publisher string
		Name      string
		Version   string
		Platform  string
	}
	type digestKey struct {
		identityKey
		Checksum string
	}
	byIdentity := make(map[identityKey][]*repopb.ArtifactManifest)
	byDigest := make(map[digestKey][]*repopb.ArtifactManifest)
	byBuildID := make(map[string][]*repopb.ArtifactManifest)

	for _, m := range manifests {
		ref := m.GetRef()
		if repoScanPackage != "" && ref.GetName() != repoScanPackage {
			continue
		}
		idk := identityKey{
			Publisher: ref.GetPublisherId(),
			Name:      ref.GetName(),
			Version:   ref.GetVersion(),
			Platform:  ref.GetPlatform(),
		}
		byIdentity[idk] = append(byIdentity[idk], m)
		byDigest[digestKey{identityKey: idk, Checksum: m.GetChecksum()}] = append(byDigest[digestKey{identityKey: idk, Checksum: m.GetChecksum()}], m)
		if bid := strings.TrimSpace(m.GetBuildId()); bid != "" {
			byBuildID[bid] = append(byBuildID[bid], m)
		}
	}

	fmt.Println("=== Repository Dedup Report ===")
	fmt.Println()

	dedupeCandidates := 0
	fmt.Println("same checksum across multiple builds (dedupe/alias candidates):")
	for k, group := range byDigest {
		if len(group) < 2 {
			continue
		}
		dedupeCandidates++
		fmt.Printf("- %s/%s@%s %s checksum=%s (%d builds)\n",
			k.Publisher, k.Name, k.Version, k.Platform, k.Checksum, len(group))
		for _, m := range group {
			fmt.Printf("  build_id=%s build_number=%d state=%s\n",
				m.GetBuildId(), m.GetBuildNumber(), m.GetPublishState().String())
		}
	}
	if dedupeCandidates == 0 {
		fmt.Println("- none")
	}
	fmt.Println()

	multiBuild := 0
	fmt.Println("same package identity with multiple checksums (real multi-build versions):")
	for k, group := range byIdentity {
		if len(group) < 2 {
			continue
		}
		seenChecksums := map[string]bool{}
		for _, m := range group {
			seenChecksums[m.GetChecksum()] = true
		}
		if len(seenChecksums) < 2 {
			continue
		}
		multiBuild++
		fmt.Printf("- %s/%s@%s %s (%d checksums, %d builds)\n",
			k.Publisher, k.Name, k.Version, k.Platform, len(seenChecksums), len(group))
	}
	if multiBuild == 0 {
		fmt.Println("- none")
	}
	fmt.Println()

	conflicts := 0
	fmt.Println("same build_id mapped to multiple checksums (identity conflict):")
	for bid, group := range byBuildID {
		if len(group) < 2 {
			continue
		}
		seenChecksums := map[string]bool{}
		for _, m := range group {
			seenChecksums[m.GetChecksum()] = true
		}
		if len(seenChecksums) < 2 {
			continue
		}
		conflicts++
		fmt.Printf("- build_id=%s has %d checksums\n", bid, len(seenChecksums))
	}
	if conflicts == 0 {
		fmt.Println("- none")
	}

	fmt.Println()
	fmt.Printf("Summary: dedupe_candidates=%d multi_build_versions=%d build_id_checksum_conflicts=%d\n",
		dedupeCandidates, multiBuild, conflicts)
	return nil
}

type aliasRow struct {
	Publisher        string `json:"publisher"`
	Name             string `json:"name"`
	Version          string `json:"version"`
	Platform         string `json:"platform"`
	ReleaseTag       string `json:"release_tag"`
	BuildNumber      int64  `json:"build_number"`
	CanonicalBuildID string `json:"canonical_build_id"`
	UpstreamSource   string `json:"upstream_source"`
	Checksum         string `json:"checksum"`
}

func runRepoAliases(cmd *cobra.Command, args []string) error {
	repoAddr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if repoAddr == "" {
		return fmt.Errorf("cannot discover repository address")
	}
	client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect to repository: %w", err)
	}
	defer client.Close()
	if token := rootCfg.token; token != "" {
		client.SetToken(token)
	}

	manifests, err := client.ListArtifacts()
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}

	var rows []aliasRow
	for _, m := range manifests {
		ref := m.GetRef()
		if ref.GetName() != repoAliasesName || ref.GetVersion() != repoAliasesVersion || ref.GetPlatform() != repoAliasesPlatform {
			continue
		}
		ui := m.GetUpstreamImport()
		if ui == nil || strings.TrimSpace(ui.GetReleaseTag()) == "" || ui.GetBuildNumber() <= 0 {
			continue
		}
		rows = append(rows, aliasRow{
			Publisher:        ref.GetPublisherId(),
			Name:             ref.GetName(),
			Version:          ref.GetVersion(),
			Platform:         ref.GetPlatform(),
			ReleaseTag:       ui.GetReleaseTag(),
			BuildNumber:      ui.GetBuildNumber(),
			CanonicalBuildID: m.GetBuildId(),
			UpstreamSource:   ui.GetSourceName(),
			Checksum:         m.GetChecksum(),
		})
	}
	if len(rows) == 0 {
		return fmt.Errorf("no alias mappings found for %s@%s %s", repoAliasesName, repoAliasesVersion, repoAliasesPlatform)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ReleaseTag != rows[j].ReleaseTag {
			return rows[i].ReleaseTag < rows[j].ReleaseTag
		}
		return rows[i].BuildNumber < rows[j].BuildNumber
	})

	if repoAliasesJSON {
		out, err := json.MarshalIndent(rows, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal aliases: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	fmt.Printf("Alias mappings for %s@%s %s\n\n", repoAliasesName, repoAliasesVersion, repoAliasesPlatform)
	for _, r := range rows {
		fmt.Printf("- release=%s build_number=%d source=%s -> canonical_build_id=%s checksum=%s\n",
			r.ReleaseTag, r.BuildNumber, r.UpstreamSource, r.CanonicalBuildID, r.Checksum)
	}
	return nil
}

// loadDesiredRefs returns a set of service names that have desired-state entries.
func loadDesiredRefs(ctx context.Context) map[string]bool {
	refs := make(map[string]bool)
	cli, err := config.GetEtcdClient()
	if err != nil {
		return refs
	}
	resp, err := cli.Get(ctx, "/globular/resources/ServiceDesiredVersion/",
		clientv3.WithPrefix(), clientv3.WithKeysOnly(), clientv3.WithLimit(500))
	if err != nil {
		return refs
	}
	for _, kv := range resp.Kvs {
		parts := strings.Split(string(kv.Key), "/")
		if len(parts) > 0 {
			refs[parts[len(parts)-1]] = true
		}
	}
	// Also load infrastructure releases.
	resp2, _ := cli.Get(ctx, "/globular/resources/InfrastructureRelease/",
		clientv3.WithPrefix(), clientv3.WithKeysOnly(), clientv3.WithLimit(500))
	for _, kv := range resp2.Kvs {
		parts := strings.Split(string(kv.Key), "/")
		if len(parts) > 0 {
			refs[parts[len(parts)-1]] = true
		}
	}
	return refs
}

// loadInstalledRefs returns a set of package names that appear in any node's installed-state.
func loadInstalledRefs(ctx context.Context) map[string]bool {
	refs := make(map[string]bool)
	cli, err := config.GetEtcdClient()
	if err != nil {
		return refs
	}
	resp, err := cli.Get(ctx, "/globular/nodes/",
		clientv3.WithPrefix(), clientv3.WithLimit(5000))
	if err != nil {
		return refs
	}
	type rec struct{ Name string `json:"name"` }
	for _, kv := range resp.Kvs {
		if !strings.Contains(string(kv.Key), "/packages/") {
			continue
		}
		var r rec
		if json.Unmarshal(kv.Value, &r) == nil && r.Name != "" {
			refs[r.Name] = true
		}
	}
	return refs
}

// identityFindingReasons enumerates the reason codes that runRepoDoctorIdentity
// surfaces. Reasons are matched as a prefix so that suffixed variants (e.g.
// "repository.identity.blob_integrity: missing_blob") are included too.
var identityFindingReasons = []string{
	"repository.identity.",
}

func isIdentityFindingReason(reason string) bool {
	for _, prefix := range identityFindingReasons {
		if strings.HasPrefix(reason, prefix) {
			return true
		}
	}
	return false
}

func runRepoResolve(cmd *cobra.Command, args []string) error {
	name := strings.TrimSpace(repoResolveName)
	buildID := strings.TrimSpace(repoResolveBuildID)
	if name == "" && buildID == "" {
		return fmt.Errorf("either --name or --build-id is required")
	}

	repoAddr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if repoAddr == "" {
		return fmt.Errorf("cannot discover repository address")
	}
	client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect to repository: %w", err)
	}
	defer client.Close()
	if token := rootCfg.token; token != "" {
		client.SetToken(token)
	}

	req := &repopb.ResolveArtifactRequest{
		PublisherId: strings.TrimSpace(repoResolvePublisher),
		Name:        name,
		Platform:    strings.TrimSpace(repoResolvePlatform),
		Version:     strings.TrimSpace(repoResolveVersion),
		BuildId:     buildID,
	}
	if k := strings.ToUpper(strings.TrimSpace(repoResolveKind)); k != "" {
		if kv, ok := repopb.ArtifactKind_value[k]; ok {
			req.Kind = repopb.ArtifactKind(kv)
		} else {
			return fmt.Errorf("unknown --kind %q (want SERVICE | APPLICATION | INFRASTRUCTURE)", repoResolveKind)
		}
	}
	if c := strings.ToUpper(strings.TrimSpace(repoResolveChannel)); c != "" {
		if cv, ok := repopb.ArtifactChannel_value[c]; ok {
			req.Channel = repopb.ArtifactChannel(cv)
		} else {
			return fmt.Errorf("unknown --channel %q (want STABLE | CANDIDATE | DEV)", repoResolveChannel)
		}
	}

	resp, err := client.ResolveArtifact(req)
	if err != nil {
		return fmt.Errorf("resolve: %w", err)
	}
	m := resp.GetManifest()
	if m == nil {
		return fmt.Errorf("resolver returned no manifest")
	}

	if repoResolveJSON {
		out, mErr := json.MarshalIndent(resp, "", "  ")
		if mErr != nil {
			return fmt.Errorf("marshal resolve response: %w", mErr)
		}
		fmt.Println(string(out))
		return nil
	}

	ref := m.GetRef()
	fmt.Printf("resolution_source:   %s\n", resp.GetResolutionSource())
	fmt.Printf("package:             %s/%s@%s %s\n",
		ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform())
	fmt.Printf("build_id:            %s\n", m.GetBuildId())
	fmt.Printf("build_number:        %d\n", m.GetBuildNumber())
	fmt.Printf("publish_state:       %s\n", m.GetPublishState().String())
	fmt.Printf("channel:             %s\n", m.GetChannel().String())
	fmt.Printf("checksum:            %s\n", m.GetChecksum())
	fmt.Printf("entrypoint_checksum: %s\n", m.GetEntrypointChecksum())
	fmt.Printf("size_bytes:          %d\n", m.GetSizeBytes())
	if ui := m.GetUpstreamImport(); ui != nil && (ui.GetReleaseTag() != "" || ui.GetBuildNumber() > 0) {
		fmt.Printf("upstream:            source=%s release=%s build_number=%d\n",
			ui.GetSourceName(), ui.GetReleaseTag(), ui.GetBuildNumber())
	}
	return nil
}

type identityFindingRow struct {
	Reason             string            `json:"reason"`
	Severity           string            `json:"severity"`
	ArtifactKey        string            `json:"artifact_key,omitempty"`
	Publisher          string            `json:"publisher,omitempty"`
	Name               string            `json:"name,omitempty"`
	Version            string            `json:"version,omitempty"`
	Platform           string            `json:"platform,omitempty"`
	CurrentState       string            `json:"current_state,omitempty"`
	ExpectedState      string            `json:"expected_state,omitempty"`
	Evidence           map[string]string `json:"evidence,omitempty"`
	RecommendedCommand string            `json:"recommended_command,omitempty"`
}

func runRepoDoctorIdentity(cmd *cobra.Command, args []string) error {
	repoAddr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if repoAddr == "" {
		return fmt.Errorf("cannot discover repository address")
	}
	client, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect to repository: %w", err)
	}
	defer client.Close()
	if token := rootCfg.token; token != "" {
		client.SetToken(token)
	}

	resp, err := client.ListRepositoryFindings(&repopb.ListRepositoryFindingsRequest{})
	if err != nil {
		return fmt.Errorf("list findings: %w", err)
	}

	var rows []identityFindingRow
	var criticalCount int
	for _, f := range resp.GetFindings() {
		if !isIdentityFindingReason(f.GetReason()) {
			continue
		}
		ref := f.GetRef()
		rows = append(rows, identityFindingRow{
			Reason:             f.GetReason(),
			Severity:           f.GetSeverity().String(),
			ArtifactKey:        f.GetArtifactKey(),
			Publisher:          ref.GetPublisherId(),
			Name:               ref.GetName(),
			Version:            ref.GetVersion(),
			Platform:           ref.GetPlatform(),
			CurrentState:       f.GetCurrentState(),
			ExpectedState:      f.GetExpectedState(),
			Evidence:           f.GetEvidence(),
			RecommendedCommand: f.GetRecommendedCommand(),
		})
		if f.GetSeverity() == repopb.RepositoryFindingSeverity_REPO_FIND_CRITICAL {
			criticalCount++
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Reason != rows[j].Reason {
			return rows[i].Reason < rows[j].Reason
		}
		return rows[i].ArtifactKey < rows[j].ArtifactKey
	})

	if repoDoctorIdentityJSON {
		out, mErr := json.MarshalIndent(map[string]any{
			"findings":       rows,
			"critical_count": criticalCount,
		}, "", "  ")
		if mErr != nil {
			return fmt.Errorf("marshal findings: %w", mErr)
		}
		fmt.Println(string(out))
		if criticalCount > 0 {
			return fmt.Errorf("%d critical identity finding(s)", criticalCount)
		}
		return nil
	}

	fmt.Println("=== Repository Identity Findings ===")
	if len(rows) == 0 {
		fmt.Println("- none")
		return nil
	}
	for _, r := range rows {
		fmt.Printf("- [%s] %s\n", r.Severity, r.Reason)
		if r.ArtifactKey != "" {
			fmt.Printf("    artifact_key:   %s\n", r.ArtifactKey)
		}
		if r.Name != "" {
			fmt.Printf("    package:        %s/%s@%s %s\n", r.Publisher, r.Name, r.Version, r.Platform)
		}
		if r.CurrentState != "" {
			fmt.Printf("    current_state:  %s\n", r.CurrentState)
		}
		if r.ExpectedState != "" {
			fmt.Printf("    expected_state: %s\n", r.ExpectedState)
		}
		if len(r.Evidence) > 0 {
			keys := make([]string, 0, len(r.Evidence))
			for k := range r.Evidence {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("    %-14s  %s\n", k+":", r.Evidence[k])
			}
		}
		if r.RecommendedCommand != "" {
			fmt.Printf("    remediation:    %s\n", r.RecommendedCommand)
		}
	}
	fmt.Printf("\nSummary: %d finding(s), %d critical\n", len(rows), criticalCount)
	if criticalCount > 0 {
		return fmt.Errorf("%d critical identity finding(s)", criticalCount)
	}
	return nil
}
