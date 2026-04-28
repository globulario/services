// repo_inspect_upstream_cmd.go — Inspect a registered upstream source.
package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var repoInspectUpstreamCmd = &cobra.Command{
	Use:   "inspect-upstream <name>",
	Short: "Show detailed status of a registered upstream source",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoInspectUpstream,
}

func runRepoInspectUpstream(cmd *cobra.Command, args []string) error {
	name := args[0]

	rc, err := repoClient()
	if err != nil {
		return err
	}
	defer rc.Close()

	resp, err := rc.ListUpstreams()
	if err != nil {
		return fmt.Errorf("list upstreams: %w", err)
	}

	for _, s := range resp.Sources {
		if s.Name != name {
			continue
		}

		fmt.Printf("Name:               %s\n", s.Name)
		fmt.Printf("Type:               %s\n", s.Type)
		if s.RepoUrl != "" {
			fmt.Printf("Repo URL:           %s\n", s.RepoUrl)
		}
		fmt.Printf("Index URL:          %s\n", s.IndexUrl)
		fmt.Printf("Channel:            %s\n", s.Channel)
		fmt.Printf("Platform:           %s\n", s.Platform)
		fmt.Printf("Enabled:            %v\n", s.Enabled)
		fmt.Printf("Trust Policy:       %s\n", orDefault(s.TrustPolicy, "import (default)"))
		fmt.Printf("Credentials:        %s\n", orDefault(s.CredentialsRef, "(not set)"))
		fmt.Printf("Require Checksum:   %v\n", s.RequireChecksum)
		if s.DefaultPublisherId != "" {
			fmt.Printf("Default Publisher:  %s\n", s.DefaultPublisherId)
		}
		if len(s.AllowedPublishers) > 0 {
			fmt.Printf("Allowed Publishers: %s\n", strings.Join(s.AllowedPublishers, ", "))
		}
		if len(s.AllowedKinds) > 0 {
			fmt.Printf("Allowed Kinds:      %s\n", strings.Join(s.AllowedKinds, ", "))
		}
		if len(s.AllowedChannels) > 0 {
			fmt.Printf("Allowed Channels:   %s\n", strings.Join(s.AllowedChannels, ", "))
		}
		fmt.Printf("Include Prereleases: %v\n", s.IncludePrereleases)

		// Sync status
		lastTag := orDefault(s.LastSyncedTag, "-")
		lastStatus := orDefault(s.LastSyncStatus, "-")
		lastSync := "-"
		if s.LastSyncUnix > 0 {
			lastSync = time.Unix(s.LastSyncUnix, 0).UTC().Format(time.RFC3339)
		}
		fmt.Printf("\nLast Synced Tag:    %s\n", lastTag)
		fmt.Printf("Last Sync:          %s (%s)\n", lastSync, lastStatus)
		if s.LastSyncError != "" {
			fmt.Printf("Last Sync Error:    %s\n", s.LastSyncError)
		}
		return nil
	}

	return fmt.Errorf("upstream source %q not found", name)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func init() {
	repoCmd.AddCommand(repoInspectUpstreamCmd)
}
