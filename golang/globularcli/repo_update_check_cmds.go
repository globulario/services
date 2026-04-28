// repo_update_check_cmds.go — Check for available upstream updates.
//
// For each registered upstream, fetches the release-index.json from the
// latest (or specified) tag and compares against the local catalog.
// Prints a table of packages with available updates.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/spf13/cobra"
)

var (
	updateCheckSource string
	updateCheckTag    string
)

var repoUpdateCheckCmd = &cobra.Command{
	Use:   "update-check",
	Short: "Check for available upstream package updates",
	Long: `Fetch the release index from a registered upstream source and compare
against locally published packages. Shows which packages have newer
versions available upstream.

This is a read-only operation — no packages are imported.`,
	Example: `  # Check for updates from the default upstream
  globular repo update-check --source globulario-github --tag v1.0.30

  # Compare specific tag against local catalog
  globular repo update-check --source my-upstream --tag v2.0.0`,
	RunE: runRepoUpdateCheck,
}

func runRepoUpdateCheck(cmd *cobra.Command, args []string) error {
	if updateCheckSource == "" {
		return fmt.Errorf("--source is required")
	}
	if updateCheckTag == "" {
		return fmt.Errorf("--tag is required")
	}

	// Get repository client for local catalog.
	rc, err := repoClient()
	if err != nil {
		return err
	}
	defer rc.Close()

	// List all registered upstreams to find the source.
	listResp, err := rc.ListUpstreams()
	if err != nil {
		return fmt.Errorf("list upstreams: %w", err)
	}

	var src *repopb.UpstreamSource
	for _, s := range listResp.Sources {
		if s.Name == updateCheckSource {
			src = s
			break
		}
	}
	if src == nil {
		return fmt.Errorf("upstream source %q not found", updateCheckSource)
	}

	// Build index URL and fetch.
	indexURL := strings.ReplaceAll(src.IndexUrl, "{tag}", updateCheckTag)
	fmt.Printf("Fetching release index from %s...\n\n", indexURL)

	idx, err := fetchUpdateCheckIndex(indexURL)
	if err != nil {
		return fmt.Errorf("fetch index: %w", err)
	}

	// Get local catalog.
	localArtifacts, err := rc.ListArtifacts()
	if err != nil {
		return fmt.Errorf("list local artifacts: %w", err)
	}

	// Build local version map: (name, platform) → latest version.
	type localKey struct{ name, platform string }
	localVersions := make(map[localKey]string)
	for _, m := range localArtifacts {
		ref := m.GetRef()
		k := localKey{ref.GetName(), ref.GetPlatform()}
		existing := localVersions[k]
		if existing == "" || ref.GetVersion() > existing {
			localVersions[k] = ref.GetVersion()
		}
	}

	// Compare.
	type updateRow struct {
		name         string
		platform     string
		localVer     string
		upstreamVer  string
		status       string
	}

	var rows []updateRow
	for _, entry := range idx.Packages {
		k := localKey{entry.Name, entry.Platform}
		localVer := localVersions[k]

		var st string
		switch {
		case localVer == "":
			st = "NEW"
		case localVer == entry.Version:
			st = "UP-TO-DATE"
		case localVer < entry.Version:
			st = "UPDATE"
		default:
			st = "AHEAD"
		}

		rows = append(rows, updateRow{
			name:        entry.Name,
			platform:    entry.Platform,
			localVer:    localVer,
			upstreamVer: entry.Version,
			status:      st,
		})
	}

	if len(rows) == 0 {
		fmt.Println("No packages in upstream index.")
		return nil
	}

	// Print table.
	fmt.Printf("%-30s  %-14s  %-14s  %-14s  %s\n",
		"PACKAGE", "PLATFORM", "LOCAL", "UPSTREAM", "STATUS")
	fmt.Println(strings.Repeat("-", 90))

	updates := 0
	newPkgs := 0
	for _, r := range rows {
		local := r.localVer
		if local == "" {
			local = "-"
		}
		fmt.Printf("%-30s  %-14s  %-14s  %-14s  %s\n",
			r.name, r.platform, local, r.upstreamVer, r.status)
		switch r.status {
		case "UPDATE":
			updates++
		case "NEW":
			newPkgs++
		}
	}

	fmt.Printf("\n%d updates available, %d new packages\n", updates, newPkgs)
	if updates > 0 || newPkgs > 0 {
		fmt.Printf("\nTo import: globular repo sync --source %s --tag %s\n", updateCheckSource, updateCheckTag)
	}
	return nil
}

// updateCheckIndex mirrors the release-index.json structure for update-check.
type updateCheckIndex struct {
	Packages []updateCheckEntry `json:"packages"`
}

type updateCheckEntry struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Platform string `json:"platform"`
}

func fetchUpdateCheckIndex(indexURL string) (*updateCheckIndex, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(indexURL)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", indexURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %d", indexURL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	var idx updateCheckIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("decode index: %w", err)
	}
	return &idx, nil
}

func init() {
	repoUpdateCheckCmd.Flags().StringVar(&updateCheckSource, "source", "", "Registered upstream source name (required)")
	repoUpdateCheckCmd.Flags().StringVar(&updateCheckTag, "tag", "", "Release tag to check against (required)")
	repoCmd.AddCommand(repoUpdateCheckCmd)
}
