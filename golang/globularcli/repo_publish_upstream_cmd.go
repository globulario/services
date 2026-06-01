// repo_publish_upstream_cmd.go — Publish packages to an upstream registry.
//
// Phase 2: dry-run only. Real GitHub upload is not implemented.
package main

import (
	"fmt"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/spf13/cobra"
)

var (
	publishSource string
	publishTag    string
	publishDryRun bool
)

var repoPublishUpstreamCmd = &cobra.Command{
	Use:   "publish-upstream",
	Short: "Publish local packages to an upstream registry (dry-run only)",
	Long: `Generate a publish plan for pushing local PUBLISHED packages to a
GitHub Release. Currently supports dry-run only — no GitHub writes.

The plan shows which packages would be uploaded, their checksums,
sizes, and the release-index.json that would be generated.`,
	Example: `  # Preview publish plan
  globular repo publish-upstream --source globulario --tag v1.0.31 --dry-run`,
	RunE: runRepoPublishUpstream,
}

func runRepoPublishUpstream(cmd *cobra.Command, args []string) error {
	if publishSource == "" {
		return fmt.Errorf("--source is required")
	}
	if publishTag == "" {
		return fmt.Errorf("--tag is required")
	}
	if !publishDryRun {
		return fmt.Errorf("upstream publish execution is not implemented yet; use --dry-run to preview")
	}

	rc, err := repoClient()
	if err != nil {
		return err
	}
	defer rc.Close()

	// List local PUBLISHED artifacts.
	artifacts, err := rc.ListArtifacts()
	if err != nil {
		return fmt.Errorf("list artifacts: %w", err)
	}

	// Filter to PUBLISHED only.
	var published []*repopb.ArtifactManifest
	for _, m := range artifacts {
		if m.GetPublishState() == repopb.PublishState_PUBLISHED {
			published = append(published, m)
		}
	}

	if len(published) == 0 {
		fmt.Println("No PUBLISHED artifacts to include in release.")
		return nil
	}

	fmt.Printf("Publish plan for source %q @ %s:\n\n", publishSource, publishTag)
	fmt.Println("Would create GitHub Release:", publishTag)
	fmt.Println("Would upload:")

	var totalSize int64
	fmt.Printf("  %-45s  %-20s  %s\n", "PACKAGE", "CHECKSUM", "SIZE")
	fmt.Println("  " + strings.Repeat("-", 85))

	for _, m := range published {
		ref := m.GetRef()
		name := fmt.Sprintf("%s-%s-%s.tgz", ref.GetName(), ref.GetVersion(), ref.GetPlatform())
		checksum := m.GetChecksum()
		if len(checksum) > 20 {
			checksum = checksum[:20] + "..."
		}
		size := m.GetSizeBytes()
		totalSize += size
		fmt.Printf("  %-45s  %-20s  %s\n", name, checksum, formatBytes(size))
	}

	fmt.Printf("  %-45s  %-20s  %s\n", "release-index.json", "", "(generated)")
	fmt.Printf("\nTotal: %d packages, %s\n", len(published), formatBytes(totalSize))
	fmt.Println("RBAC action required: repository.upstream.publish")
	fmt.Printf("\nTo execute (when implemented): globular repo publish-upstream --source %s --tag %s\n", publishSource, publishTag)
	return nil
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

func init() {
	repoPublishUpstreamCmd.Flags().StringVar(&publishSource, "source", "", "Upstream source name (required)")
	repoPublishUpstreamCmd.Flags().StringVar(&publishTag, "tag", "", "Release tag to publish (required)")
	repoPublishUpstreamCmd.Flags().BoolVar(&publishDryRun, "dry-run", false, "Preview only — no GitHub writes")
	repoCmd.AddCommand(repoPublishUpstreamCmd)
}
