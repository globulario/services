// repo_upstream_cmds.go — Upstream source management for the repository.
//
// Commands:
//   globular repo register-upstream --name <n> --url <index-url> [--channel <c>] [--platform <p>] [--disabled]
//   globular repo list-upstreams
//   globular repo remove-upstream <name>
//   globular pkg sync-upstream --source <name> --tag <tag> [--dry-run] [--only a,b,c]
package main

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/spf13/cobra"
)

// ── Flag variables ────────────────────────────────────────────────────────────

var (
	upstreamName     string
	upstreamURL      string
	upstreamChannel  string
	upstreamPlatform string
	upstreamDisabled bool

	syncSource string
	syncTag    string
	syncDryRun bool
	syncOnly   string
)

// ── Commands ──────────────────────────────────────────────────────────────────

var repoRegisterUpstreamCmd = &cobra.Command{
	Use:   "register-upstream",
	Short: "Register or update an upstream package source",
	Long: `Register a named upstream source so the cluster can sync new releases.

The --url flag must contain a {tag} placeholder that the sync command
will substitute with the requested release tag:

  https://github.com/globulario/services/releases/download/{tag}/release-index.json

Only GITHUB_RELEASE type is supported in v1 (HTTP_INDEX is reserved).`,
	Example: `  # Register the default globulario upstream
  globular repo register-upstream \
    --name globulario-github \
    --url "https://github.com/globulario/services/releases/download/{tag}/release-index.json"

  # Register with a specific channel and platform
  globular repo register-upstream \
    --name my-upstream \
    --url "https://example.com/releases/{tag}/index.json" \
    --channel stable \
    --platform linux_amd64`,
	RunE: runRepoRegisterUpstream,
}

var repoListUpstreamsCmd = &cobra.Command{
	Use:   "list-upstreams",
	Short: "List registered upstream sources",
	RunE:  runRepoListUpstreams,
}

var repoRemoveUpstreamCmd = &cobra.Command{
	Use:   "remove-upstream <name>",
	Short: "Remove a registered upstream source",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoRemoveUpstream,
}

var pkgSyncUpstreamCmd = &cobra.Command{
	Use:   "sync-upstream",
	Short: "Import packages from a registered upstream release",
	Long: `Sync packages from a registered upstream source into the local repository.

Each artifact in the release index is verified by sha256 digest before
import. Existing artifacts with the same identity key are skipped.
Digest conflicts are counted as rejected (audit-logged, not imported).

Use --dry-run to preview what would be imported without writing anything.`,
	Example: `  # Preview what would be imported from v1.0.18
  globular pkg sync-upstream --source globulario-github --tag v1.0.18 --dry-run

  # Import all packages from v1.0.18
  globular pkg sync-upstream --source globulario-github --tag v1.0.18

  # Import only specific packages
  globular pkg sync-upstream --source globulario-github --tag v1.0.18 \
    --only dns,rbac,authentication`,
	RunE: runPkgSyncUpstream,
}

// ── Implementations ───────────────────────────────────────────────────────────

func repoClient() (*repository_client.Repository_Service_Client, error) {
	addr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if addr == "" {
		if a, err := config.GetMeshAddress(); err == nil {
			addr = a
		}
	}
	if addr == "" {
		return nil, fmt.Errorf("repository service not found (checked etcd and mesh)")
	}
	return repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
}

func runRepoRegisterUpstream(cmd *cobra.Command, args []string) error {
	if upstreamName == "" {
		return fmt.Errorf("--name is required")
	}
	if upstreamURL == "" {
		return fmt.Errorf("--url is required")
	}
	if !strings.Contains(upstreamURL, "{tag}") {
		return fmt.Errorf("--url must contain a {tag} placeholder (e.g. .../download/{tag}/release-index.json)")
	}

	platform := upstreamPlatform
	if platform == "" {
		platform = "linux_amd64"
	}
	channel := upstreamChannel
	if channel == "" {
		channel = "stable"
	}

	src := &repopb.UpstreamSource{
		Name:     upstreamName,
		Type:     repopb.UpstreamSourceType_GITHUB_RELEASE,
		IndexUrl: upstreamURL,
		Channel:  channel,
		Platform: platform,
		Enabled:  !upstreamDisabled,
	}

	rc, err := repoClient()
	if err != nil {
		return err
	}
	defer rc.Close()

	_, err = rc.RegisterUpstream(src)
	if err != nil {
		return fmt.Errorf("register upstream: %w", err)
	}

	fmt.Printf("Upstream %q registered\n", upstreamName)
	fmt.Printf("  type:     GITHUB_RELEASE\n")
	fmt.Printf("  url:      %s\n", upstreamURL)
	fmt.Printf("  channel:  %s\n", channel)
	fmt.Printf("  platform: %s\n", platform)
	fmt.Printf("  enabled:  %v\n", !upstreamDisabled)
	return nil
}

func runRepoListUpstreams(cmd *cobra.Command, args []string) error {
	rc, err := repoClient()
	if err != nil {
		return err
	}
	defer rc.Close()

	resp, err := rc.ListUpstreams()
	if err != nil {
		return fmt.Errorf("list upstreams: %w", err)
	}

	if len(resp.Sources) == 0 {
		fmt.Println("No upstream sources registered.")
		fmt.Println("Use 'globular repo register-upstream' to add one.")
		return nil
	}

	fmt.Printf("%-24s  %-12s  %-12s  %-12s  %-10s  %s\n",
		"NAME", "CHANNEL", "PLATFORM", "LAST_TAG", "ENABLED", "URL")
	fmt.Println(strings.Repeat("-", 100))
	for _, s := range resp.Sources {
		enabled := "yes"
		if !s.Enabled {
			enabled = "no"
		}
		lastTag := s.LastSyncedTag
		if lastTag == "" {
			lastTag = "-"
		}
		fmt.Printf("%-24s  %-12s  %-12s  %-12s  %-10s  %s\n",
			s.Name, s.Channel, s.Platform, lastTag, enabled, s.IndexUrl)
	}
	return nil
}

func runRepoRemoveUpstream(cmd *cobra.Command, args []string) error {
	name := args[0]

	rc, err := repoClient()
	if err != nil {
		return err
	}
	defer rc.Close()

	err = rc.RemoveUpstream(name)
	if err != nil {
		return fmt.Errorf("remove upstream: %w", err)
	}

	fmt.Printf("Upstream %q removed.\n", name)
	return nil
}

func runPkgSyncUpstream(cmd *cobra.Command, args []string) error {
	if syncSource == "" {
		return fmt.Errorf("--source is required")
	}
	if syncTag == "" {
		return fmt.Errorf("--tag is required")
	}

	var only []string
	if syncOnly != "" {
		for _, s := range strings.Split(syncOnly, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				only = append(only, s)
			}
		}
	}

	rc, err := repoClient()
	if err != nil {
		return err
	}
	defer rc.Close()

	if syncDryRun {
		fmt.Printf("Dry-run: previewing sync from %q @ %s\n\n", syncSource, syncTag)
	} else {
		fmt.Printf("Syncing from %q @ %s...\n\n", syncSource, syncTag)
	}

	resp, err := rc.SyncFromUpstream(syncSource, syncTag, syncDryRun, only)
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	// Print per-artifact results.
	if len(resp.Results) > 0 {
		fmt.Printf("%-28s  %-10s  %-12s  %-8s  %s\n", "PACKAGE", "VERSION", "BUILD_ID", "STATUS", "DETAIL")
		fmt.Println(strings.Repeat("-", 90))
		for _, r := range resp.Results {
			detail := r.Detail
			if detail == "" {
				detail = "-"
			}
			fmt.Printf("%-28s  %-10s  %-12s  %-8s  %s\n",
				r.Name, r.Version, r.BuildId, statusLabel(r.Status), detail)
		}
		fmt.Println()
	}

	// Summary.
	if syncDryRun {
		fmt.Printf("Would import: %d   would skip: %d   would reject: %d   would fail: %d\n",
			resp.Imported, resp.Skipped, resp.Rejected, resp.Failed)
	} else {
		fmt.Printf("Imported: %d   skipped: %d   rejected: %d   failed: %d\n",
			resp.Imported, resp.Skipped, resp.Rejected, resp.Failed)
	}

	if resp.Rejected > 0 || resp.Failed > 0 {
		return fmt.Errorf("sync completed with %d rejected and %d failed artifacts", resp.Rejected, resp.Failed)
	}
	return nil
}

func statusLabel(s repopb.UpstreamSyncStatus) string {
	switch s {
	case repopb.UpstreamSyncStatus_SYNC_IMPORTED:
		return "IMPORTED"
	case repopb.UpstreamSyncStatus_SYNC_SKIPPED:
		return "SKIPPED"
	case repopb.UpstreamSyncStatus_SYNC_REJECTED:
		return "REJECTED"
	case repopb.UpstreamSyncStatus_SYNC_FAILED:
		return "FAILED"
	case repopb.UpstreamSyncStatus_SYNC_WOULD_IMPORT:
		return "WOULD_IMPORT"
	case repopb.UpstreamSyncStatus_SYNC_WOULD_SKIP:
		return "WOULD_SKIP"
	case repopb.UpstreamSyncStatus_SYNC_WOULD_REJECT:
		return "WOULD_REJECT"
	case repopb.UpstreamSyncStatus_SYNC_WOULD_FAIL:
		return "WOULD_FAIL"
	default:
		return "UNKNOWN"
	}
}

// ── init ──────────────────────────────────────────────────────────────────────

func init() {
	// register-upstream flags
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamName, "name", "", "Upstream source name (required)")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamURL, "url", "", "Index URL with {tag} placeholder (required)")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamChannel, "channel", "stable", "Release channel")
	repoRegisterUpstreamCmd.Flags().StringVar(&upstreamPlatform, "platform", "linux_amd64", "Target platform")
	repoRegisterUpstreamCmd.Flags().BoolVar(&upstreamDisabled, "disabled", false, "Register but do not enable")

	// sync-upstream flags
	pkgSyncUpstreamCmd.Flags().StringVar(&syncSource, "source", "", "Registered upstream source name (required)")
	pkgSyncUpstreamCmd.Flags().StringVar(&syncTag, "tag", "", "Release tag to sync (required)")
	pkgSyncUpstreamCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Preview only — no artifacts are written")
	pkgSyncUpstreamCmd.Flags().StringVar(&syncOnly, "only", "", "Comma-separated list of package names to import")

	repoCmd.AddCommand(repoRegisterUpstreamCmd)
	repoCmd.AddCommand(repoListUpstreamsCmd)
	repoCmd.AddCommand(repoRemoveUpstreamCmd)

	pkgCmd.AddCommand(pkgSyncUpstreamCmd)
}
