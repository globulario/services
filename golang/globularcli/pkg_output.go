package main

// pkg_output.go — deterministic output types for `globular pkg publish`.
//
// JSON key order is determined by struct field order (encoding/json guarantee).
// Do NOT use map[string]any in output paths — that reintroduces random ordering.

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// PkgPublishOutput is the top-level response for `globular pkg publish`.
//
// Single-package mode: Status, Package, Repository, DescriptorAction, BundleID,
//   Digest, SizeBytes, DurationMS, Error are populated; Summary and Results are nil.
// Directory mode: Summary and Results are populated; Status, Package, etc. are empty.
type PkgPublishOutput struct {
	// One of: "success" | "failed" — single-package mode only.
	Status string `json:"status,omitempty" yaml:"status,omitempty"`

	// Populated for single-package mode.
	Package PkgPublishPackage `json:"package,omitempty" yaml:"package,omitempty"`

	// Repository address used, e.g. "localhost:10007".
	Repository string `json:"repository,omitempty" yaml:"repository,omitempty"`

	// One of: "created" | "updated" | "upserted".
	DescriptorAction string `json:"descriptor_action,omitempty" yaml:"descriptor_action,omitempty"`

	// Stable bundle identifier, e.g. "echo@0.0.1:linux_amd64".
	BundleID string `json:"bundle_id,omitempty" yaml:"bundle_id,omitempty"`

	// Content digest of the .tgz, e.g. "sha256:9f4c2e0d...".
	Digest string `json:"digest,omitempty" yaml:"digest,omitempty"`

	// Total size of the uploaded package in bytes.
	SizeBytes int64 `json:"size_bytes,omitempty" yaml:"size_bytes,omitempty"`

	// Wall-clock duration for the full operation in milliseconds.
	DurationMS int64 `json:"duration_ms,omitempty" yaml:"duration_ms,omitempty"`

	// Present only on failure (single-package mode).
	Error *PkgPublishError `json:"error,omitempty" yaml:"error,omitempty"`

	// Directory mode: aggregate counts.
	Summary *PkgPublishSummary `json:"summary,omitempty" yaml:"summary,omitempty"`

	// Directory mode: per-package results.
	Results []PkgPublishResult `json:"results,omitempty" yaml:"results,omitempty"`
}

// PkgPublishPackage holds the identifying fields of the published package.
type PkgPublishPackage struct {
	Name      string `json:"name,omitempty"      yaml:"name,omitempty"`
	Version   string `json:"version,omitempty"   yaml:"version,omitempty"`
	Platform  string `json:"platform,omitempty"  yaml:"platform,omitempty"`
	Publisher string `json:"publisher,omitempty" yaml:"publisher,omitempty"`
}

// PkgPublishError carries a gRPC-style error code and message.
type PkgPublishError struct {
	// gRPC status code name: "Unauthenticated" | "PermissionDenied" | "NotFound" | "Internal" | ...
	Code string `json:"code,omitempty" yaml:"code,omitempty"`
	// Human-readable single-line message.
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

// PkgPublishSummary holds directory-mode aggregate counts.
type PkgPublishSummary struct {
	Total      int   `json:"total"       yaml:"total"`
	Succeeded  int   `json:"succeeded"   yaml:"succeeded"`
	Failed     int   `json:"failed"      yaml:"failed"`
	DurationMS int64 `json:"duration_ms" yaml:"duration_ms"`
}

// PkgPublishResult is the per-package element in directory mode.
type PkgPublishResult struct {
	// "success" | "failed"
	Status string `json:"status,omitempty" yaml:"status,omitempty"`

	Name      string `json:"name,omitempty"      yaml:"name,omitempty"`
	Version   string `json:"version,omitempty"   yaml:"version,omitempty"`
	Platform  string `json:"platform,omitempty"  yaml:"platform,omitempty"`
	Publisher string `json:"publisher,omitempty" yaml:"publisher,omitempty"`

	Repository       string `json:"repository,omitempty"        yaml:"repository,omitempty"`
	DescriptorAction string `json:"descriptor_action,omitempty" yaml:"descriptor_action,omitempty"`
	BundleID         string `json:"bundle_id,omitempty"         yaml:"bundle_id,omitempty"`
	Digest           string `json:"digest,omitempty"            yaml:"digest,omitempty"`
	SizeBytes        int64  `json:"size_bytes,omitempty"        yaml:"size_bytes,omitempty"`
	DurationMS       int64  `json:"duration_ms,omitempty"       yaml:"duration_ms,omitempty"`

	Error *PkgPublishError `json:"error,omitempty" yaml:"error,omitempty"`
}

// Millis converts a duration to integer milliseconds.
func pkgMillis(d time.Duration) int64 { return d.Milliseconds() }

// pkgSHA256 returns the hex-encoded SHA-256 digest of the file at path.
func pkgSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return fmt.Sprintf("sha256:%x", h.Sum(nil))
}

// pkgBundleID returns the stable bundle identifier "name@version:platform".
func pkgBundleID(name, version, platform string) string {
	return fmt.Sprintf("%s@%s:%s", name, version, platform)
}

// renderPkgPublish prints out in the requested format (table, json, yaml).
func renderPkgPublish(out *PkgPublishOutput, format string) error {
	switch strings.ToLower(format) {
	case "json":
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	case "yaml":
		b, err := yaml.Marshal(out)
		if err != nil {
			return err
		}
		fmt.Print(string(b))
	default: // "table"
		renderPkgPublishTable(out)
	}
	return nil
}

const tableSep = "────────────────────────────────────────────────────────────"

func renderPkgPublishTable(out *PkgPublishOutput) {
	if out.Summary != nil {
		// Directory mode
		fmt.Println("PUBLISH SUMMARY")
		fmt.Println(tableSep)
		fmt.Printf("%-12s: %d\n", "Total", out.Summary.Total)
		fmt.Printf("%-12s: %d\n", "Succeeded", out.Summary.Succeeded)
		fmt.Printf("%-12s: %d\n", "Failed", out.Summary.Failed)
		fmt.Printf("%-12s: %dms\n", "Duration", out.Summary.DurationMS)
		if len(out.Results) > 0 {
			fmt.Println()
			fmt.Println("DETAILS")
			fmt.Println(tableSep)
			for _, r := range out.Results {
				if r.Error != nil {
					fmt.Printf("✗ %-10s %-8s %-14s %s\n", r.Name, r.Version, r.Platform, r.Publisher)
					fmt.Printf("    Error   : %s (%s)\n", r.Error.Message, r.Error.Code)
				} else {
					fmt.Printf("✓ %-10s %-8s %-14s %s\n", r.Name, r.Version, r.Platform, r.Publisher)
				}
			}
		}
		return
	}

	// Single-package mode
	fmt.Println("PUBLISH RESULT")
	fmt.Println(tableSep)
	statusStr := strings.ToUpper(out.Status)
	fmt.Printf("%-12s: %s\n", "Status", statusStr)
	if out.Package.Name != "" {
		fmt.Printf("%-12s: %s\n", "Name", out.Package.Name)
		fmt.Printf("%-12s: %s\n", "Version", out.Package.Version)
		fmt.Printf("%-12s: %s\n", "Platform", out.Package.Platform)
		fmt.Printf("%-12s: %s\n", "Publisher", out.Package.Publisher)
	}
	if out.Repository != "" {
		fmt.Printf("%-12s: %s\n", "Repository", out.Repository)
	}
	if out.BundleID != "" {
		fmt.Printf("%-12s: %s\n", "BundleID", out.BundleID)
	}
	if out.DescriptorAction != "" {
		fmt.Printf("%-12s: %s\n", "Descriptor", out.DescriptorAction)
	}
	if out.Digest != "" {
		fmt.Printf("%-12s: %s\n", "Digest", out.Digest)
	}
	if out.SizeBytes > 0 {
		fmt.Printf("%-12s: %.1f MB\n", "Size", float64(out.SizeBytes)/1e6)
	}
	if out.DurationMS > 0 {
		fmt.Printf("%-12s: %dms\n", "Duration", out.DurationMS)
	}
	if out.Error != nil {
		fmt.Printf("%-12s: %s\n", "ErrorCode", out.Error.Code)
		fmt.Printf("%-12s: %s\n", "Error", out.Error.Message)
	}
}
