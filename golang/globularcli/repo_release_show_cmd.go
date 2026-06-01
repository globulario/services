// repo_release_show_cmd.go — Display the composition of a platform release.
//
// Fetches release-index.json for the given tag and displays a bill of materials
// showing per-package versions, change status, and origin releases.
// @awareness namespace=globular.platform
// @awareness component=platform_cli
// @awareness file_role=release_show_command
// @awareness risk=medium
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/globulario/services/golang/repository/upstream"
	"github.com/spf13/cobra"
)

var (
	releaseShowSource string
	releaseShowJSON   bool
)

var repoReleaseShowCmd = &cobra.Command{
	Use:   "release-show <tag>",
	Short: "Display the bill of materials for a platform release",
	Long: `Fetch the release-index.json for the given tag and display which packages
are included, their versions, whether they changed in this release, and
which origin release produced each package artifact.

This is a read-only operation.`,
	Example: `  # Show release composition
  globular repo release-show v1.0.84

  # Show with JSON output
  globular repo release-show v1.0.84 --json

  # Use a registered upstream source
  globular repo release-show v1.0.84 --source globulario`,
	Args: cobra.ExactArgs(1),
	RunE: runRepoReleaseShow,
}

func runRepoReleaseShow(cmd *cobra.Command, args []string) error {
	tag := args[0]

	var indexURL string
	if releaseShowSource != "" {
		// Use registered upstream source to build URL.
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
			if s.Name == releaseShowSource {
				indexURL = strings.ReplaceAll(s.IndexUrl, "{tag}", tag)
				break
			}
		}
		if indexURL == "" {
			return fmt.Errorf("upstream source %q not found", releaseShowSource)
		}
	} else {
		// Default: try globulario GitHub releases.
		owner, repo := "globulario", "services"
		indexURL = upstream.DeriveIndexURL(owner, repo)
		indexURL = strings.ReplaceAll(indexURL, "{tag}", tag)
	}

	// Fetch the release index.
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(indexURL)
	if err != nil {
		return fmt.Errorf("fetch release index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch release index: HTTP %d from %s", resp.StatusCode, upstream.RedactAssetURL(indexURL))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("read release index: %w", err)
	}

	var idx map[string]interface{}
	if err := json.Unmarshal(data, &idx); err != nil {
		return fmt.Errorf("parse release index: %w", err)
	}

	if releaseShowJSON {
		fmt.Println(string(data))
		return nil
	}

	// Display header.
	platformRelease := strVal(idx, "platform_release")
	if platformRelease == "" {
		platformRelease = strVal(idx, "globular_version")
	}
	publisher := strVal(idx, "publisher_id")
	if publisher == "" {
		publisher = strVal(idx, "publisher")
	}
	generatedAt := strVal(idx, "generated_at")
	refs := arrVal(idx, "referenced_releases")

	fmt.Printf("Platform Release: Globular %s\n", platformRelease)
	fmt.Printf("Release Tag:      %s\n", strVal(idx, "release_tag"))
	fmt.Printf("Publisher:        %s\n", publisher)
	if generatedAt != "" {
		fmt.Printf("Generated:        %s\n", generatedAt)
	}
	if len(refs) > 0 {
		fmt.Printf("Referenced:       %s\n", strings.Join(refs, ", "))
	}
	fmt.Println()

	// Display packages table.
	packages, _ := idx["packages"].([]interface{})
	if len(packages) == 0 {
		fmt.Println("No packages in release index.")
		return nil
	}

	fmt.Printf("%-28s  %-14s  %-6s  %-8s  %-12s  %s\n",
		"PACKAGE", "VERSION", "BUILD", "CHANGED", "ORIGIN", "KIND")
	fmt.Println(strings.Repeat("-", 100))

	changed := 0
	unchanged := 0
	for _, p := range packages {
		pkg, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		name := strMapVal(pkg, "name")
		version := strMapVal(pkg, "version")
		kind := strMapVal(pkg, "kind")
		origin := strMapVal(pkg, "origin_release")
		buildNum := ""
		if bn, ok := pkg["build_number"].(float64); ok && bn > 0 {
			buildNum = fmt.Sprintf("%d", int64(bn))
		} else {
			buildNum = "-"
		}

		changedStr := "yes"
		isChanged := true
		if ci, ok := pkg["changed_in_release"].(bool); ok {
			isChanged = ci
		}
		if !isChanged {
			changedStr = "no"
			unchanged++
		} else {
			changed++
		}
		if origin == "" {
			origin = strVal(idx, "release_tag")
		}

		fmt.Printf("%-28s  %-14s  %-6s  %-8s  %-12s  %s\n",
			name, version, buildNum, changedStr, origin, kind)
	}

	total := changed + unchanged
	fmt.Printf("\nChanged: %d / %d   Unchanged: %d / %d\n", changed, total, unchanged, total)
	return nil
}

func strVal(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func strMapVal(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func arrVal(m map[string]interface{}, key string) []string {
	arr, ok := m[key].([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, v := range arr {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func init() {
	repoReleaseShowCmd.Flags().StringVar(&releaseShowSource, "source", "", "Registered upstream source name")
	repoReleaseShowCmd.Flags().BoolVar(&releaseShowJSON, "json", false, "Output raw JSON")
	repoCmd.AddCommand(repoReleaseShowCmd)
}
