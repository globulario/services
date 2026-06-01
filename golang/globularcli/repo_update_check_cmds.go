// repo_update_check_cmds.go — Check for available upstream updates.
//
// Uses server-side dry-run sync to get policy-checked results.
// The CLI only formats the response — all policy logic lives on the server.
// @awareness namespace=globular.platform
// @awareness component=platform_cli
// @awareness file_role=repository_update_check_commands
// @awareness risk=medium
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/spf13/cobra"
)

var (
	updateCheckSource string
	updateCheckTag    string
	updateCheckLatest bool
	updateCheckJSON   bool
)

var repoUpdateCheckCmd = &cobra.Command{
	Use:   "update-check",
	Short: "Check for available upstream package updates",
	Long: `Fetch the release index from a registered upstream source and compare
against locally published packages. Uses server-side dry-run sync to
show policy-checked results including blocked updates and reasons.

This is a read-only operation — no packages are imported.`,
	Example: `  # Check latest available updates
  globular repo update-check --source globulario --latest

  # Check specific tag
  globular repo update-check --source globulario --tag v1.0.30

  # JSON output for admin UI
  globular repo update-check --source globulario --latest --json`,
	RunE: runRepoUpdateCheck,
}

func runRepoUpdateCheck(cmd *cobra.Command, args []string) error {
	if updateCheckSource == "" {
		return fmt.Errorf("--source is required")
	}
	if updateCheckTag != "" && updateCheckLatest {
		return fmt.Errorf("cannot use both --tag and --latest")
	}
	if updateCheckTag == "" && !updateCheckLatest {
		return fmt.Errorf("--tag or --latest is required")
	}

	rc, err := repoClient()
	if err != nil {
		return err
	}
	defer rc.Close()

	// Use server-side dry-run sync for policy-checked results.
	resp, err := rc.SyncFromUpstreamWithOptions(&repopb.SyncFromUpstreamRequest{
		SourceName:    updateCheckSource,
		ReleaseTag:    updateCheckTag,
		DryRun:        true,
		ResolveLatest: updateCheckLatest,
	})
	if err != nil {
		return fmt.Errorf("update-check: %w", err)
	}

	if updateCheckJSON {
		return printUpdateCheckJSON(resp)
	}
	return printUpdateCheckTable(resp)
}

func printUpdateCheckTable(resp *repopb.SyncFromUpstreamResponse) error {
	tagDisplay := resp.ResolvedTag
	if tagDisplay == "" {
		tagDisplay = "(unknown)"
	}
	fmt.Printf("SOURCE: %s  TAG: %s\n\n", resp.SourceName, tagDisplay)

	if len(resp.Results) == 0 {
		fmt.Println("No packages in upstream index.")
		return nil
	}

	fmt.Printf("%-28s  %-12s  %-12s  %-10s  %-12s  %-10s  %s\n",
		"PACKAGE", "LOCAL", "UPSTREAM", "CHANNEL", "BUILD", "ACTION", "POLICY")
	fmt.Println(strings.Repeat("-", 110))

	var updates, newPkgs, blocked int
	for _, r := range resp.Results {
		local := r.LocalVersion
		if local == "" {
			local = "-"
		}
		action := strings.ToUpper(r.Action)
		policy := "allowed"
		if r.BlockedReason != "" {
			policy = r.BlockedReason
			blocked++
		}
		buildStr := fmt.Sprintf("%d", r.BuildNumber)
		if r.BuildNumber == 0 {
			buildStr = "-"
		}

		fmt.Printf("%-28s  %-12s  %-12s  %-10s  %-12s  %-10s  %s\n",
			r.Name, local, r.Version, r.Channel, buildStr, action, policy)

		switch r.Action {
		case "update":
			updates++
		case "new":
			newPkgs++
		}
	}

	fmt.Printf("\n%d updates, %d new, %d blocked\n", updates, newPkgs, blocked)
	if updates > 0 || newPkgs > 0 {
		syncCmd := fmt.Sprintf("globular repo sync --source %s", resp.SourceName)
		if resp.ResolvedTag != "" {
			syncCmd += " --tag " + resp.ResolvedTag
		}
		fmt.Printf("\nTo import: %s\n", syncCmd)
	}
	return nil
}

func printUpdateCheckJSON(resp *repopb.SyncFromUpstreamResponse) error {
	type jsonResult struct {
		Name             string `json:"name"`
		Platform         string `json:"platform"`
		Publisher        string `json:"publisher"`
		Kind             string `json:"kind"`
		LocalVersion     string `json:"local_version"`
		UpstreamVersion  string `json:"upstream_version"`
		Channel          string `json:"channel"`
		BuildNumber      int64  `json:"build_number"`
		ChecksumPresent  bool   `json:"checksum_present"`
		Action           string `json:"action"`
		PolicyStatus     string `json:"policy_status"`
		BlockedReason    string `json:"blocked_reason,omitempty"`
	}
	type jsonOutput struct {
		SourceName  string       `json:"source_name"`
		ResolvedTag string       `json:"resolved_tag"`
		Packages    []jsonResult `json:"packages"`
		Summary     struct {
			Updates   int `json:"updates"`
			New       int `json:"new"`
			Blocked   int `json:"blocked"`
			UpToDate  int `json:"up_to_date"`
		} `json:"summary"`
	}

	out := jsonOutput{
		SourceName:  resp.SourceName,
		ResolvedTag: resp.ResolvedTag,
	}

	for _, r := range resp.Results {
		policyStatus := "allowed"
		if r.BlockedReason != "" {
			policyStatus = "blocked"
		}
		out.Packages = append(out.Packages, jsonResult{
			Name:            r.Name,
			Platform:        r.Platform,
			Publisher:       r.Publisher,
			Kind:            r.Kind,
			LocalVersion:    r.LocalVersion,
			UpstreamVersion: r.Version,
			Channel:         r.Channel,
			BuildNumber:     r.BuildNumber,
			ChecksumPresent: r.ChecksumPresent,
			Action:          r.Action,
			PolicyStatus:    policyStatus,
			BlockedReason:   r.BlockedReason,
		})

		switch r.Action {
		case "update":
			out.Summary.Updates++
		case "new":
			out.Summary.New++
		case "blocked":
			out.Summary.Blocked++
		case "up_to_date":
			out.Summary.UpToDate++
		}
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func init() {
	repoUpdateCheckCmd.Flags().StringVar(&updateCheckSource, "source", "", "Registered upstream source name (required)")
	repoUpdateCheckCmd.Flags().StringVar(&updateCheckTag, "tag", "", "Release tag to check against")
	repoUpdateCheckCmd.Flags().BoolVar(&updateCheckLatest, "latest", false, "Discover latest release from repo_url")
	repoUpdateCheckCmd.Flags().BoolVar(&updateCheckJSON, "json", false, "Output as JSON")
	repoCmd.AddCommand(repoUpdateCheckCmd)
}
