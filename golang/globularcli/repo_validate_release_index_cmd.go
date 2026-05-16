// repo_validate_release_index_cmd.go — Local validation of release-index.json.
//
// Validates structural correctness and version-authority invariants without
// requiring a live repository server. Reads from a local file or stdin.
//
// Usage:
//   globular repository validate-release-index release-index.json
//   cat release-index.json | globular repository validate-release-index -
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	validateReleaseIndexStrict bool
	validateReleaseIndexJSON   bool
)

var repoValidateReleaseIndexCmd = &cobra.Command{
	Use:   "validate-release-index <file|->",
	Short: "Validate a release-index.json file",
	Long: `Validate a release-index.json for structural correctness and version-authority
invariants. Works locally — no server connection required.

Checks:
  - schema_version present and supported
  - release_tag present
  - Each package has required fields: name, version, platform, kind, publisher
  - Each package has a valid sha256 digest
  - Each package has at least one artifact locator (asset_url or asset_path)
  - V2 BOM: changed_in_release is explicit, unchanged packages have origin_release
  - V2 BOM: build_id is non-numeric (UUID or upstream-prefixed)

Version authority checks (--strict or V2 BOM):
  - Unchanged packages must NOT carry platform_release as their version
  - Unchanged packages must have origin_release set
  - build_id must not be numeric-only (would be confused with build_number)

Exit codes:
  0  All checks pass
  1  Validation errors found
  2  File not found or parse error`,
	Example: `  # Validate a local BOM file
  globular repository validate-release-index release-index.json

  # Validate from stdin (pipe)
  cat release-index.json | globular repository validate-release-index -

  # Strict mode (also enforces install-path requirements)
  globular repository validate-release-index --strict release-index.json`,
	Args: cobra.ExactArgs(1),
	RunE: runRepoValidateReleaseIndex,
}

func runRepoValidateReleaseIndex(cmd *cobra.Command, args []string) error {
	path := args[0]

	// Read input.
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(io.LimitReader(os.Stdin, 10<<20))
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	var idx map[string]interface{}
	if err := json.Unmarshal(data, &idx); err != nil {
		fmt.Fprintf(os.Stderr, "error: parse release-index.json: %v\n", err)
		os.Exit(2)
	}

	result := validateReleaseIndex(idx, validateReleaseIndexStrict)

	if validateReleaseIndexJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	// Human-readable output.
	fmt.Printf("Release Index: %s\n", path)
	fmt.Printf("Schema:        %s\n", result.SchemaVersion)
	if result.PlatformRelease != "" {
		fmt.Printf("Platform:      %s\n", result.PlatformRelease)
	}
	fmt.Printf("Release Tag:   %s\n", result.ReleaseTag)
	fmt.Printf("Packages:      %d total (%d changed, %d unchanged)\n",
		result.TotalPackages, result.ChangedCount, result.UnchangedCount)
	fmt.Println()

	if len(result.Errors) == 0 {
		fmt.Printf("OK — %d checks passed\n", result.ChecksPassed)
		return nil
	}

	fmt.Printf("FAILED — %d error(s):\n\n", len(result.Errors))
	for i, e := range result.Errors {
		fmt.Printf("  %d. [%s] %s\n", i+1, e.Rule, e.Message)
	}
	fmt.Println()
	os.Exit(1)
	return nil
}

type releaseIndexValidationResult struct {
	SchemaVersion  string                      `json:"schema_version"`
	PlatformRelease string                     `json:"platform_release,omitempty"`
	ReleaseTag     string                      `json:"release_tag"`
	TotalPackages  int                         `json:"total_packages"`
	ChangedCount   int                         `json:"changed_count"`
	UnchangedCount int                         `json:"unchanged_count"`
	ChecksPassed   int                         `json:"checks_passed"`
	Errors         []releaseIndexValidationErr `json:"errors"`
	OK             bool                        `json:"ok"`
}

type releaseIndexValidationErr struct {
	Rule    string `json:"rule"`
	Package string `json:"package,omitempty"`
	Message string `json:"message"`
}

func validateReleaseIndex(idx map[string]interface{}, strict bool) releaseIndexValidationResult {
	result := releaseIndexValidationResult{}
	var errs []releaseIndexValidationErr
	checks := 0

	addErr := func(rule, pkg, msg string) {
		errs = append(errs, releaseIndexValidationErr{Rule: rule, Package: pkg, Message: msg})
	}

	// ── Schema version ────────────────────────────────────────────────────────
	sv := strVal(idx, "schema_version")
	if sv == "" {
		// Integer schema_version (legacy CI).
		if svn, ok := idx["schema_version"].(float64); ok {
			if int(svn) == 1 {
				sv = "globular.repository.index/v1"
			} else if int(svn) == 2 {
				sv = "globular.repository.index/v2"
			}
		}
	}
	if sv == "" {
		addErr("schema.required", "", "schema_version is required")
	} else if sv != "globular.repository.index/v1" && sv != "globular.repository.index/v2" {
		addErr("schema.unsupported", "", fmt.Sprintf("unsupported schema_version %q", sv))
	} else {
		checks++
	}
	result.SchemaVersion = sv
	isV2 := sv == "globular.repository.index/v2"

	// ── Release tag ───────────────────────────────────────────────────────────
	releaseTag := strVal(idx, "release_tag")
	if releaseTag == "" {
		addErr("index.release_tag", "", "release_tag is required")
	} else {
		checks++
	}
	result.ReleaseTag = releaseTag

	// ── Platform release (V2) ─────────────────────────────────────────────────
	platformRelease := strVal(idx, "platform_release")
	result.PlatformRelease = platformRelease
	if isV2 && platformRelease == "" {
		addErr("index.platform_release", "", "platform_release is required for V2 BOM index")
	} else if platformRelease != "" {
		checks++
	}

	// ── Packages ──────────────────────────────────────────────────────────────
	packages, _ := idx["packages"].([]interface{})
	result.TotalPackages = len(packages)

	// Track build_id → digest for conflict detection (V2).
	buildIDDigest := map[string]string{}

	for i, p := range packages {
		pkg, ok := p.(map[string]interface{})
		if !ok {
			addErr("pkg.type", "", fmt.Sprintf("packages[%d]: not an object", i))
			continue
		}

		name := strMapVal(pkg, "name")
		pkgLabel := fmt.Sprintf("packages[%d]", i)
		if name != "" {
			pkgLabel = name
		}

		// Required fields.
		for _, field := range []string{"name", "version", "platform", "kind", "publisher"} {
			if strings.TrimSpace(strMapVal(pkg, field)) == "" {
				addErr("pkg.required_field", pkgLabel,
					fmt.Sprintf("%s: %s is required", pkgLabel, field))
			} else {
				checks++
			}
		}

		// Digest: at least one of artifact_sha256, package_digest, checksum.
		artifactSha := strMapVal(pkg, "artifact_sha256")
		pkgDigest := strMapVal(pkg, "package_digest")
		checksum := strMapVal(pkg, "checksum")
		digest := strings.TrimSpace(artifactSha)
		if digest == "" {
			digest = strings.TrimSpace(pkgDigest)
		}
		if digest == "" {
			digest = strings.TrimSpace(checksum)
		}
		if digest == "" {
			addErr("pkg.digest", pkgLabel,
				fmt.Sprintf("%s: artifact_sha256, package_digest, or checksum is required", pkgLabel))
		} else if !strings.HasPrefix(digest, "sha256:") {
			addErr("pkg.digest_prefix", pkgLabel,
				fmt.Sprintf("%s: digest must start with sha256:", pkgLabel))
		} else if len(strings.TrimPrefix(digest, "sha256:")) != 64 {
			addErr("pkg.digest_length", pkgLabel,
				fmt.Sprintf("%s: sha256 digest must be 64 hex chars", pkgLabel))
		} else {
			checks++
		}

		// Locator: at least one of asset_url, asset_path, filename.
		assetURL := strMapVal(pkg, "asset_url")
		assetPath := strMapVal(pkg, "asset_path")
		filename := strMapVal(pkg, "filename")
		if assetURL == "" && assetPath == "" && filename == "" {
			addErr("pkg.locator", pkgLabel,
				fmt.Sprintf("%s: asset_url, asset_path, or filename is required", pkgLabel))
		} else {
			checks++
		}

		version := strMapVal(pkg, "version")
		buildID := strings.TrimSpace(strMapVal(pkg, "build_id"))

		// changed_in_release handling.
		changedInRelease := true // v1 default
		if ci, ok := pkg["changed_in_release"].(bool); ok {
			changedInRelease = ci
		} else if isV2 {
			if _, present := pkg["changed_in_release"]; !present {
				addErr("pkg.changed_in_release", pkgLabel,
					fmt.Sprintf("%s: changed_in_release is required in V2 BOM", pkgLabel))
			}
		}

		if changedInRelease {
			result.ChangedCount++
		} else {
			result.UnchangedCount++

			// V2 invariant: unchanged packages must have origin_release.
			if isV2 {
				originRelease := strMapVal(pkg, "origin_release")
				if originRelease == "" {
					addErr("pkg.origin_release", pkgLabel,
						fmt.Sprintf("%s: origin_release is required for unchanged packages in V2 BOM", pkgLabel))
				} else {
					checks++
				}
			}

			// Version authority check: unchanged package must NOT carry platform_release.
			if isV2 && platformRelease != "" && version == platformRelease {
				addErr("version_authority.platform_stamp", pkgLabel,
					fmt.Sprintf(
						"%s: unchanged package version %q == platform_release %q — "+
							"this stamps the unchanged package with the platform version, "+
							"causing convergence failure (reconciler requests %s@%s which doesn't exist)",
						pkgLabel, version, platformRelease, name, platformRelease))
			}
		}

		// Strict/V2: build_id must not be numeric-only.
		if (strict || isV2) && buildID != "" {
			isNumeric := true
			for _, c := range buildID {
				if c < '0' || c > '9' {
					isNumeric = false
					break
				}
			}
			if isNumeric {
				addErr("version_authority.numeric_build_id", pkgLabel,
					fmt.Sprintf("%s: build_id %q is numeric-only — use UUID or upstream-derived id; "+
						"numeric build_ids are confused with build_number and break install identity",
						pkgLabel, buildID))
			} else {
				checks++
			}
		}

		// V2: check for build_id → digest conflicts (same build_id, different digest).
		if isV2 && buildID != "" && digest != "" {
			if prev, seen := buildIDDigest[buildID]; seen {
				if prev != digest {
					addErr("version_authority.build_id_conflict", pkgLabel,
						fmt.Sprintf(
							"%s: build_id %q maps to two different digests (%s, %s) — "+
								"build_id is immutable artifact identity; one build_id = one artifact",
							pkgLabel, buildID, prev, digest))
				}
			} else {
				buildIDDigest[buildID] = digest
				checks++
			}
		}
	}

	result.ChecksPassed = checks
	result.Errors = errs
	result.OK = len(errs) == 0
	return result
}

func init() {
	repoValidateReleaseIndexCmd.Flags().BoolVar(&validateReleaseIndexStrict, "strict", false,
		"Apply stricter install-path requirements (build_id required, numeric build_id rejected)")
	repoValidateReleaseIndexCmd.Flags().BoolVar(&validateReleaseIndexJSON, "json", false,
		"Output validation result as JSON")
}
