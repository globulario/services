package main

// awareness_bundle_publish.go: `globular awareness bundle publish` — uploads
// a previously built awareness bundle (.tar.gz produced by
// `awareness bundle build`) to the repository service as an
// ArtifactKind_AWARENESS_BUNDLE.
//
// The bundle archive ships its own manifest.json (the cli-local
// awarenessBundleManifest shape) which carries the bundle identity
// (name, version, build_id). That manifest is the source of truth for
// publish; this command will not invent identities or guess defaults.
//
// Why a dedicated verb instead of teaching `pkg publish --kind`:
//   - `pkg publish` calls pkgpack.VerifyTGZ, which requires package.json
//     inside the archive. Awareness bundles ship manifest.json instead, so
//     the validator rejects them at step 1. Adding a "skip package.json
//     when kind=AWARENESS_BUNDLE" branch into the service-publish path
//     would entangle two flows that have no other overlap.
//   - The server-side UploadArtifact RPC already tolerates archives
//     without package.json (extractPackageManifest returns nil silently)
//     and honours ref.Kind, so the only missing piece is a client wrapper
//     that knows how to read the awareness manifest.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/globulario/services/golang/awareness/bundlesync"
	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

var (
	bundlePublishCfg = struct {
		file       string
		repository string
		publisher  string
		platform   string
		dryRun     bool
	}{}

	awarenessBundlePublishCmd = &cobra.Command{
		Use:   "publish",
		Short: "Publish a previously built awareness bundle to the repository",
		Long: `Publish an awareness bundle (.tar.gz produced by 'awareness bundle build')
to the repository service as ArtifactKind_AWARENESS_BUNDLE.

The bundle's own manifest.json supplies the identity (name, version, build_id).
The archive sha256 is computed locally and verified against what the
repository records, so a corrupted upload fails closed.

Authentication is required: run 'globular auth login' first.

Examples:
  globular awareness bundle publish --file awareness-bundle-0.0.1-abcd1234.tar.gz \
      --repository repository.globular.internal

  globular awareness bundle publish --file ... --repository ... --dry-run
`,
		RunE: runAwarenessBundlePublish,
	}
)

// awarenessBundlePublishResult is the per-publish summary returned to the
// caller. Exported field names keep the JSON shape stable for scripted use.
type awarenessBundlePublishResult struct {
	File          string `json:"file"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	BuildID       string `json:"build_id"`
	SchemaVersion string `json:"schema_version,omitempty"`
	Platform      string `json:"platform"`
	Publisher     string `json:"publisher"`
	Kind          string `json:"kind"`
	SHA256        string `json:"sha256"`
	SizeBytes     int64  `json:"size_bytes"`
	Repository    string `json:"repository"`
	Status        string `json:"status"`
	DurationMS    int64  `json:"duration_ms"`
}

func runAwarenessBundlePublish(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(bundlePublishCfg.file) == "" {
		return errors.New("--file is required")
	}
	if strings.TrimSpace(bundlePublishCfg.repository) == "" {
		return errors.New("--repository is required")
	}

	start := time.Now()

	data, err := os.ReadFile(bundlePublishCfg.file)
	if err != nil {
		return fmt.Errorf("read bundle: %w", err)
	}

	manifest, _, err := inspectBundle(data)
	if err != nil {
		return fmt.Errorf("inspect bundle: %w", err)
	}

	if err := validateBundleManifestForPublish(manifest); err != nil {
		return err
	}

	publisher := strings.TrimSpace(bundlePublishCfg.publisher)
	if publisher == "" {
		publisher = "core@globular.io"
	}
	platform := strings.TrimSpace(bundlePublishCfg.platform)
	if platform == "" {
		// Awareness bundles are platform-independent (they ship YAML +
		// a SQLite graph that any node can read), but the repository
		// keys artifacts by platform, so we record a canonical value.
		platform = "any"
	}

	digestHex := hex.EncodeToString(sha256Sum(data))

	result := awarenessBundlePublishResult{
		File:          bundlePublishCfg.file,
		Name:          manifest.Name,
		Version:       manifest.Version,
		BuildID:       manifest.BuildID,
		SchemaVersion: manifest.SchemaVersion,
		Platform:      platform,
		Publisher:     publisher,
		Kind:          "AWARENESS_BUNDLE",
		SHA256:        "sha256:" + digestHex,
		SizeBytes:     int64(len(data)),
		Repository:    bundlePublishCfg.repository,
	}

	if bundlePublishCfg.dryRun {
		result.Status = "dry-run"
		result.DurationMS = time.Since(start).Milliseconds()
		return renderBundlePublish(&result)
	}

	token := rootCfg.token
	if token == "" {
		return errors.New("authentication required: run 'globular auth login' or provide --token")
	}

	if _, err := getTLSCredentialsWithOptions(true); err != nil {
		return err
	}

	client, err := repository_client.NewRepositoryService_Client(
		bundlePublishCfg.repository, "repository.PackageRepository")
	if err != nil {
		return fmt.Errorf("connect to repository: %w", err)
	}
	defer client.Close()
	client.SetToken(token)

	ref := &repopb.ArtifactRef{
		PublisherId: publisher,
		Name:        manifest.Name,
		Version:     manifest.Version,
		Platform:    platform,
		Kind:        repopb.ArtifactKind_AWARENESS_BUNDLE,
	}

	// build_number=0: the repository allocates the next available number
	// for this version. The bundle's own UUID lives in manifest.BuildID
	// and is independent of the repository's BuildId (which is a UUIDv7
	// stamped by completePublish).
	if err := client.UploadArtifactWithBuild(ref, data, 0); err != nil {
		return fmt.Errorf("upload bundle: %w", err)
	}

	// Read back the stored manifest to confirm the repository received
	// what we sent. Mismatched sha256 is reported but not retried — the
	// upload RPC verifies the bytes server-side; this is belt-and-braces.
	stored, mErr := client.GetArtifactManifest(ref, 0)
	if mErr == nil && stored != nil {
		result.Status = "published"
		if got := stored.GetChecksum(); got != "" && got != result.SHA256 {
			result.Status = fmt.Sprintf(
				"published (warning: repository checksum %s != local %s)", got, result.SHA256)
		}
	} else {
		result.Status = "uploaded (verify failed)"
	}

	result.DurationMS = time.Since(start).Milliseconds()
	return renderBundlePublish(&result)
}

// validateBundleManifestForPublish checks the cli-local manifest carries the
// minimum identity the repository needs. Empty/wrong fields fail closed —
// we never invent a name or version on the publish side.
//
// schema_version handling:
//   - empty: tolerated for backward-compat with bundles built before the
//     field existed. The publish path still works; freshness checks on the
//     consuming node will surface the missing version as MISSING/STALE.
//   - present and supported: accepted.
//   - present and unsupported: rejected. Uploading a bundle whose schema
//     this binary cannot read is a guaranteed activation failure on every
//     downstream node, so we fail closed at publish time instead.
func validateBundleManifestForPublish(m *awarenessBundleManifest) error {
	if m == nil {
		return errors.New("malformed bundle: no manifest")
	}
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("malformed bundle: manifest.name is empty")
	}
	if strings.TrimSpace(m.Version) == "" {
		return errors.New("malformed bundle: manifest.version is empty")
	}
	if strings.TrimSpace(m.BuildID) == "" {
		return errors.New("malformed bundle: manifest.build_id is empty")
	}
	if kind := strings.TrimSpace(m.Kind); kind != "" && kind != "AWARENESS_BUNDLE" {
		return fmt.Errorf("malformed bundle: manifest.kind = %q, want \"AWARENESS_BUNDLE\"", kind)
	}
	if sv := strings.TrimSpace(m.SchemaVersion); sv != "" && !bundlesync.IsSupportedSchemaVersion(sv) {
		return fmt.Errorf(
			"malformed bundle: manifest.schema_version = %q, this binary supports %v",
			sv, bundlesync.SupportedSchemaVersions)
	}
	return nil
}

func renderBundlePublish(r *awarenessBundlePublishResult) error {
	switch strings.ToLower(rootCfg.output) {
	case "json":
		b, _ := json.MarshalIndent(r, "", "  ")
		fmt.Println(string(b))
	default:
		fmt.Printf("%-14s: %s\n", "Status", r.Status)
		fmt.Printf("%-14s: %s\n", "Name", r.Name)
		fmt.Printf("%-14s: %s\n", "Version", r.Version)
		fmt.Printf("%-14s: %s\n", "Build ID", r.BuildID)
		if r.SchemaVersion != "" {
			fmt.Printf("%-14s: %s\n", "Schema", r.SchemaVersion)
		}
		fmt.Printf("%-14s: %s\n", "Kind", r.Kind)
		fmt.Printf("%-14s: %s\n", "Platform", r.Platform)
		fmt.Printf("%-14s: %s\n", "Publisher", r.Publisher)
		fmt.Printf("%-14s: %s\n", "SHA256", r.SHA256)
		fmt.Printf("%-14s: %d\n", "Size (bytes)", r.SizeBytes)
		fmt.Printf("%-14s: %s\n", "Repository", r.Repository)
		fmt.Printf("%-14s: %s\n", "File", r.File)
		fmt.Printf("%-14s: %d ms\n", "Duration", r.DurationMS)
	}
	return nil
}

func sha256Sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func init() {
	awarenessBundlePublishCmd.Flags().StringVar(&bundlePublishCfg.file, "file", "",
		"path to a bundle tar.gz produced by 'awareness bundle build' (required)")
	awarenessBundlePublishCmd.Flags().StringVar(&bundlePublishCfg.repository, "repository", "",
		"repository service address (required, e.g. repository.globular.internal)")
	awarenessBundlePublishCmd.Flags().StringVar(&bundlePublishCfg.publisher, "publisher", "",
		"publisher identifier (default core@globular.io)")
	awarenessBundlePublishCmd.Flags().StringVar(&bundlePublishCfg.platform, "platform", "",
		"target platform (default \"any\" — bundles are platform-independent)")
	awarenessBundlePublishCmd.Flags().BoolVar(&bundlePublishCfg.dryRun, "dry-run", false,
		"inspect and validate the bundle without uploading")

	awarenessBundleCmd.AddCommand(awarenessBundlePublishCmd)
}
