package main

// awareness_sync_cmds.go — Phase C.5 CLI: status, pull, verify, install, sync.
//
// All five commands wrap the golang/awareness/bundlesync library. They are
// thin: the heavy lifting (verification, atomic install, retry, source
// discovery) lives one layer down. The CLI's job is to turn flags into
// library calls and pretty-print the result.
//
// Commands:
//
//   globular awareness status
//   globular awareness pull --from <url> [--ca <path>] [--out <dir>] [--release-index <path>]
//   globular awareness verify <bundle.tar.gz> [--manifest <path>] [--release-index <path>]
//   globular awareness install <bundle.tar.gz> [--manifest <path>] [--release-index <path>] [--bundle-root <path>]
//   globular awareness sync --from <url> [--ca <path>] [--release-index <path>] [--bundle-root <path>]
//
// All commands honor --json for machine-readable output.

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/awareness/bundlesync"
	"github.com/spf13/cobra"
)

// ── Production defaults ──────────────────────────────────────────────────────
//
// These match the canonical paths in CLAUDE.md / system reminders. Override
// via flags when running off a non-standard layout (dev, tests, recovery).

const (
	defaultBundleRoot   = "/var/lib/globular/awareness"
	defaultReleaseIndex = "/var/lib/globular/release-index.json"
	defaultClusterCA    = "/var/lib/globular/pki/ca.crt"
)

// shared flags across the new commands.
var awarenessSyncCfg = struct {
	from          string // peer base URL (pull/sync)
	outDir        string // pull output dir
	manifestPath  string // override manifest path (verify/install)
	releaseIndex  string // path to release-index.json
	bundleRoot    string // active bundle layout root
	caPath        string // cluster CA cert path
	json          bool   // machine-readable output
	timeoutSec    int    // pull timeout
	expectVersion string // override expected version
	expectBuildID string // override expected build_id
}{}

// ── status ───────────────────────────────────────────────────────────────────

var awarenessStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the active awareness bundle status and freshness verdict",
	Long: `Reads the local release-index and active bundle manifest and prints the
freshness verdict (AWARENESS_READY, AWARENESS_BUNDLE_MISSING, ...).
Equivalent to mcp.awareness_freshness_status, but works without a running MCP server.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		releaseIndexPath := pickReleaseIndexPath()
		bundleRoot := pickBundleRoot()

		ri, riErr := loadReleaseIndexFromDisk(releaseIndexPath)
		manifestPath := filepath.Join(bundleRoot, "current", "manifest.json")
		var manifest *bundlesync.Manifest
		var manifestErr error
		if _, err := os.Stat(manifestPath); errors.Is(err, os.ErrNotExist) {
			// no manifest yet
		} else if m, err := bundlesync.LoadManifest(manifestPath); err != nil {
			manifestErr = err
		} else {
			manifest = m
		}

		report := buildStatusReport(ri, riErr, manifest, manifestErr, releaseIndexPath, manifestPath)

		if awarenessSyncCfg.json {
			return writeAwarenessJSON(cmd.OutOrStdout(), report)
		}
		return writeStatusHuman(cmd.OutOrStdout(), report)
	},
}

// ── pull ─────────────────────────────────────────────────────────────────────

var awarenessPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull an awareness bundle from a peer MCP over HTTPS",
	Long: `Fetches the manifest first, verifies it matches the release-index, then
streams the bundle and verifies sha256 + tar safety. Cluster CA verification
is mandatory — there is no insecure fallback.

The pulled bundle and its sidecar manifest are written to --out (default:
current directory). The bundle is NOT installed by this command — use
'install' or 'sync' for that.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if awarenessSyncCfg.from == "" {
			return errors.New("--from <peer URL> is required (e.g. https://10.0.0.8:10260)")
		}
		out := awarenessSyncCfg.outDir
		if out == "" {
			out = "."
		}

		ri, err := resolveExpectedRelease()
		if err != nil {
			return err
		}

		pool, err := loadClusterCAPool(pickCAPath())
		if err != nil {
			return fmt.Errorf("load cluster CA: %w", err)
		}

		ctx, cancel := context.WithTimeout(cmd.Context(), pullTimeout())
		defer cancel()

		res, pullErr := bundlesync.PullBundle(ctx, bundlesync.PullOptions{
			PeerURL:         awarenessSyncCfg.from,
			OutDir:          out,
			ExpectedVersion: ri.Version,
			ExpectedBuildID: ri.BuildID,
			ClusterCAPool:   pool,
			Timeout:         pullTimeout(),
		})

		if awarenessSyncCfg.json {
			_ = writeAwarenessJSON(cmd.OutOrStdout(), res)
		} else {
			writePullHuman(cmd.OutOrStdout(), res)
		}
		if !res.OK {
			return fmt.Errorf("pull failed: %s", res.Reason)
		}
		_ = pullErr // already surfaced via res
		return nil
	},
}

// ── verify ───────────────────────────────────────────────────────────────────

var awarenessVerifyCmd = &cobra.Command{
	Use:   "verify <bundle.tar.gz>",
	Short: "Verify a candidate bundle's manifest, sha256, and tar safety",
	Long: `Runs the same checks an install would: manifest matches release-index,
schema supported, sha256 matches, tar archive contains no unsafe entries.
Read-only — does not install or modify any path.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bundlePath := args[0]
		manifestPath := awarenessSyncCfg.manifestPath
		if manifestPath == "" {
			var err error
			manifestPath, err = resolveManifestSidecar(bundlePath)
			if err != nil {
				return err
			}
		}

		ri, err := resolveExpectedRelease()
		if err != nil {
			return err
		}

		res, _ := bundlesync.VerifyBundle(bundlePath, manifestPath, ri)
		if awarenessSyncCfg.json {
			_ = writeAwarenessJSON(cmd.OutOrStdout(), res)
		} else {
			writeVerifyHuman(cmd.OutOrStdout(), res, bundlePath, manifestPath)
		}
		if !res.OK {
			return fmt.Errorf("verify failed: %s (%s)", res.State, res.Reason)
		}
		return nil
	},
}

// ── install ──────────────────────────────────────────────────────────────────

var awarenessInstallCmd = &cobra.Command{
	Use:   "install <bundle.tar.gz>",
	Short: "Verify and atomically install an awareness bundle",
	Long: `Runs verify; on success, extracts to a versioned dir under --bundle-root
and atomically swaps the 'current' symlink. Prior bundles are preserved on
disk for rollback. A running MCP server may need to be restarted to load
the new bundle.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bundlePath := args[0]
		manifestPath := awarenessSyncCfg.manifestPath
		if manifestPath == "" {
			var err error
			manifestPath, err = resolveManifestSidecar(bundlePath)
			if err != nil {
				return err
			}
		}

		ri, err := resolveExpectedRelease()
		if err != nil {
			return err
		}

		res, installErr := bundlesync.InstallBundle(bundlesync.InstallOptions{
			BundlePath:   bundlePath,
			ManifestPath: manifestPath,
			BundleRoot:   pickBundleRoot(),
			ReleaseIndex: ri,
		})
		if awarenessSyncCfg.json {
			_ = writeAwarenessJSON(cmd.OutOrStdout(), res)
		} else {
			writeInstallHuman(cmd.OutOrStdout(), res)
		}
		if !res.OK {
			if installErr != nil {
				return fmt.Errorf("install failed: %s (%v)", res.Reason, installErr)
			}
			return fmt.Errorf("install failed: %s", res.Reason)
		}
		return nil
	},
}

// ── sync ─────────────────────────────────────────────────────────────────────

var awarenessSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Pull + install in one go (peer MCP → versioned dir → current symlink)",
	Long: `Convenience wrapper: pull from --from to a temp dir, verify, install
atomically. Refuses to proceed if the peer's manifest doesn't match the local
release-index. Same network-trust rules as 'pull' (cluster CA required).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if awarenessSyncCfg.from == "" {
			return errors.New("--from <peer URL> is required")
		}

		ri, err := resolveExpectedRelease()
		if err != nil {
			return err
		}
		pool, err := loadClusterCAPool(pickCAPath())
		if err != nil {
			return fmt.Errorf("load cluster CA: %w", err)
		}

		// Pull into a temp dir under bundle root staging — install will
		// atomically rename out of staging anyway, and putting the temp
		// near the install root keeps the rename on the same filesystem.
		bundleRoot := pickBundleRoot()
		stagingDir, err := os.MkdirTemp(filepath.Join(bundleRoot, "staging"), "sync-")
		if err != nil {
			// Try MkdirAll for the staging parent first.
			if mkErr := os.MkdirAll(filepath.Join(bundleRoot, "staging"), 0755); mkErr == nil {
				stagingDir, err = os.MkdirTemp(filepath.Join(bundleRoot, "staging"), "sync-")
			}
		}
		if err != nil {
			return fmt.Errorf("create staging dir: %w", err)
		}
		defer os.RemoveAll(stagingDir)

		ctx, cancel := context.WithTimeout(cmd.Context(), pullTimeout())
		defer cancel()

		pullRes, pullErr := bundlesync.PullBundle(ctx, bundlesync.PullOptions{
			PeerURL:         awarenessSyncCfg.from,
			OutDir:          stagingDir,
			ExpectedVersion: ri.Version,
			ExpectedBuildID: ri.BuildID,
			ClusterCAPool:   pool,
			Timeout:         pullTimeout(),
		})
		if !pullRes.OK {
			if awarenessSyncCfg.json {
				_ = writeAwarenessJSON(cmd.OutOrStdout(), map[string]interface{}{
					"phase": "pull",
					"pull":  pullRes,
				})
			} else {
				fmt.Fprintln(cmd.OutOrStderr(), "[sync] pull failed:")
				writePullHuman(cmd.OutOrStderr(), pullRes)
			}
			if pullErr != nil {
				return fmt.Errorf("pull failed: %s (%v)", pullRes.Reason, pullErr)
			}
			return fmt.Errorf("pull failed: %s", pullRes.Reason)
		}

		installRes, installErr := bundlesync.InstallBundle(bundlesync.InstallOptions{
			BundlePath:   pullRes.BundlePath,
			ManifestPath: pullRes.ManifestPath,
			BundleRoot:   bundleRoot,
			ReleaseIndex: ri,
		})
		if awarenessSyncCfg.json {
			_ = writeAwarenessJSON(cmd.OutOrStdout(), map[string]interface{}{
				"phase":   "install",
				"pull":    pullRes,
				"install": installRes,
			})
		} else {
			writePullHuman(cmd.OutOrStdout(), pullRes)
			writeInstallHuman(cmd.OutOrStdout(), installRes)
		}
		if !installRes.OK {
			if installErr != nil {
				return fmt.Errorf("install failed: %s (%v)", installRes.Reason, installErr)
			}
			return fmt.Errorf("install failed: %s", installRes.Reason)
		}
		return nil
	},
}

// ── helpers ──────────────────────────────────────────────────────────────────

func pickReleaseIndexPath() string {
	if awarenessSyncCfg.releaseIndex != "" {
		return awarenessSyncCfg.releaseIndex
	}
	return defaultReleaseIndex
}

func pickBundleRoot() string {
	if awarenessSyncCfg.bundleRoot != "" {
		return awarenessSyncCfg.bundleRoot
	}
	return defaultBundleRoot
}

func pickCAPath() string {
	if awarenessSyncCfg.caPath != "" {
		return awarenessSyncCfg.caPath
	}
	return defaultClusterCA
}

func pullTimeout() time.Duration {
	if awarenessSyncCfg.timeoutSec > 0 {
		return time.Duration(awarenessSyncCfg.timeoutSec) * time.Second
	}
	return 60 * time.Second
}

// resolveExpectedRelease decides which (version, build_id) pair the operation
// must match. CLI flags override the on-disk release-index when provided —
// useful for emergency installs where the index hasn't been bumped yet.
func resolveExpectedRelease() (*bundlesync.ReleaseIndex, error) {
	if awarenessSyncCfg.expectVersion != "" && awarenessSyncCfg.expectBuildID != "" {
		return &bundlesync.ReleaseIndex{
			Version: awarenessSyncCfg.expectVersion,
			BuildID: awarenessSyncCfg.expectBuildID,
		}, nil
	}
	return loadReleaseIndexFromDisk(pickReleaseIndexPath())
}

func loadReleaseIndexFromDisk(path string) (*bundlesync.ReleaseIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read release-index %s: %w", path, err)
	}
	// Flat shape first.
	var flat bundlesync.ReleaseIndex
	if err := json.Unmarshal(data, &flat); err == nil && flat.Version != "" {
		return &flat, nil
	}
	// Then nested {"active":{...}}.
	var nested struct {
		Active *bundlesync.ReleaseIndex `json:"active"`
	}
	if err := json.Unmarshal(data, &nested); err == nil && nested.Active != nil && nested.Active.Version != "" {
		return nested.Active, nil
	}
	return nil, fmt.Errorf("release-index %s: no usable version/build_id", path)
}

func loadClusterCAPool(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("no certificates parsed from %s", path)
	}
	return pool, nil
}

// defaultManifestSidecar returns the conventional sidecar path next to a
// bundle file: "<bundle>.manifest.json" or "manifest.json" in the same dir
// when the bundle is named bundle.tar.gz.
func defaultManifestSidecar(bundlePath string) string {
	dir := filepath.Dir(bundlePath)
	base := filepath.Base(bundlePath)
	// Common case: bundle.tar.gz → manifest.json next to it.
	if strings.HasSuffix(base, ".tar.gz") {
		generic := filepath.Join(dir, "manifest.json")
		if _, err := os.Stat(generic); err == nil {
			return generic
		}
	}
	// Fallback: same basename with .manifest.json suffix.
	return strings.TrimSuffix(bundlePath, ".tar.gz") + ".manifest.json"
}

func resolveManifestSidecar(bundlePath string) (string, error) {
	candidates := []string{
		defaultManifestSidecar(bundlePath),
		bundlePath + ".manifest.json",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return synthesizeManifestSidecar(bundlePath)
}

func synthesizeManifestSidecar(bundlePath string) (string, error) {
	f, err := os.Open(bundlePath)
	if err != nil {
		return "", fmt.Errorf("open bundle %s: %w", bundlePath, err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("open bundle gzip %s: %w", bundlePath, err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var embedded struct {
		Name          string `json:"name"`
		Version       string `json:"version"`
		BuildID       string `json:"build_id"`
		SchemaVersion string `json:"schema_version"`
		SHA256        string `json:"sha256"`
		SizeBytes     int64  `json:"size_bytes,omitempty"`
		GraphHash     string `json:"graph_hash,omitempty"`
		SourceCommit  string `json:"source_commit,omitempty"`
	}
	found := false
	for {
		hdr, rerr := tr.Next()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return "", fmt.Errorf("read bundle tar %s: %w", bundlePath, rerr)
		}
		if filepath.Base(hdr.Name) != "manifest.json" {
			continue
		}
		blob, readErr := io.ReadAll(tr)
		if readErr != nil {
			return "", fmt.Errorf("read embedded manifest %s: %w", bundlePath, readErr)
		}
		if unmarshalErr := json.Unmarshal(blob, &embedded); unmarshalErr != nil {
			return "", fmt.Errorf("parse embedded manifest %s: %w", bundlePath, unmarshalErr)
		}
		found = true
		break
	}
	if !found {
		return "", fmt.Errorf("embedded manifest.json not found in %s and no sidecar manifest found", bundlePath)
	}

	raw, err := os.ReadFile(bundlePath)
	if err != nil {
		return "", fmt.Errorf("read bundle %s: %w", bundlePath, err)
	}
	sum := sha256.Sum256(raw)
	manifest := bundlesync.Manifest{
		Name:          embedded.Name,
		Version:       embedded.Version,
		BuildID:       embedded.BuildID,
		SchemaVersion: embedded.SchemaVersion,
		SHA256:        embedded.SHA256,
		SizeBytes:     embedded.SizeBytes,
		GraphHash:     embedded.GraphHash,
		SourceCommit:  embedded.SourceCommit,
	}
	if manifest.Name == "" {
		manifest.Name = bundlesync.BundleName
	}
	if manifest.SchemaVersion == "" {
		manifest.SchemaVersion = "awareness.bundle.v1"
	}
	if manifest.SHA256 == "" {
		manifest.SHA256 = hex.EncodeToString(sum[:])
	}
	if manifest.SizeBytes == 0 {
		manifest.SizeBytes = int64(len(raw))
	}
	if manifest.Version == "" || manifest.BuildID == "" {
		return "", fmt.Errorf("embedded manifest in %s missing required version/build_id fields", bundlePath)
	}

	out := strings.TrimSuffix(bundlePath, ".tar.gz") + ".manifest.json"
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode synthesized manifest for %s: %w", bundlePath, err)
	}
	if err := os.WriteFile(out, data, 0644); err != nil {
		return "", fmt.Errorf("write synthesized manifest %s: %w", out, err)
	}
	return out, nil
}

// statusReport is the JSON shape printed by `awareness status`.
type statusReport struct {
	State                bundlesync.State         `json:"state"`
	Reason               string                   `json:"reason,omitempty"`
	OK                   bool                     `json:"ok"`
	ReleaseIndex         *bundlesync.ReleaseIndex `json:"release_index,omitempty"`
	ReleaseIndexPath     string                   `json:"release_index_path"`
	Manifest             *bundlesync.Manifest     `json:"manifest,omitempty"`
	ManifestPath         string                   `json:"manifest_path"`
	VersionMatchesIndex  bool                     `json:"version_matches_release"`
	BuildIDMatchesIndex  bool                     `json:"build_id_matches_release"`
	SchemaSupported      bool                     `json:"schema_supported"`
	GraphHashPresent     bool                     `json:"graph_hash_present"`
	SourceCommitPresent  bool                     `json:"source_commit_present"`
}

func buildStatusReport(ri *bundlesync.ReleaseIndex, riErr error, m *bundlesync.Manifest, mErr error, riPath, mfPath string) statusReport {
	rep := statusReport{
		ReleaseIndexPath: riPath,
		ManifestPath:     mfPath,
	}

	if riErr != nil {
		rep.State = bundlesync.StateAwarenessBundleVerifyFailed
		rep.Reason = riErr.Error()
		return rep
	}
	rep.ReleaseIndex = ri

	if m == nil {
		if mErr != nil {
			rep.State = bundlesync.StateAwarenessBundleVerifyFailed
			rep.Reason = mErr.Error()
		} else {
			rep.State = bundlesync.StateAwarenessBundleMissing
			rep.Reason = "no manifest installed"
		}
		return rep
	}

	report := bundlesync.CheckAwarenessFreshness(m, ri, nil)
	rep.State = report.State
	rep.Reason = report.Reason
	rep.OK = report.OK
	rep.Manifest = m
	rep.VersionMatchesIndex = report.VersionMatchesRelease
	rep.BuildIDMatchesIndex = report.BuildIDMatchesRelease
	rep.SchemaSupported = report.SchemaSupported
	rep.GraphHashPresent = report.GraphHashPresent
	rep.SourceCommitPresent = report.SourceCommitPresent
	return rep
}

// ── pretty-printers ──────────────────────────────────────────────────────────

func writeAwarenessJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func writeStatusHuman(w io.Writer, r statusReport) error {
	fmt.Fprintf(w, "state:               %s\n", r.State)
	if r.Reason != "" {
		fmt.Fprintf(w, "reason:              %s\n", r.Reason)
	}
	fmt.Fprintf(w, "release-index:       %s\n", r.ReleaseIndexPath)
	if r.ReleaseIndex != nil {
		fmt.Fprintf(w, "  version:           %s\n", r.ReleaseIndex.Version)
		fmt.Fprintf(w, "  build_id:          %s\n", r.ReleaseIndex.BuildID)
	}
	fmt.Fprintf(w, "manifest:            %s\n", r.ManifestPath)
	if r.Manifest != nil {
		fmt.Fprintf(w, "  version:           %s\n", r.Manifest.Version)
		fmt.Fprintf(w, "  build_id:          %s\n", r.Manifest.BuildID)
		fmt.Fprintf(w, "  schema:            %s (supported=%v)\n", r.Manifest.SchemaVersion, r.SchemaSupported)
		fmt.Fprintf(w, "  graph_hash:        %s\n", labelOrEmpty(r.Manifest.GraphHash))
		fmt.Fprintf(w, "  source_commit:     %s\n", labelOrEmpty(r.Manifest.SourceCommit))
	}
	fmt.Fprintf(w, "version_match:       %v\n", r.VersionMatchesIndex)
	fmt.Fprintf(w, "build_id_match:      %v\n", r.BuildIDMatchesIndex)
	return nil
}

func writePullHuman(w io.Writer, r *bundlesync.PullResult) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "pull state:          %s\n", r.State)
	if r.OK {
		fmt.Fprintf(w, "  bundle:            %s\n", r.BundlePath)
		fmt.Fprintf(w, "  manifest:          %s\n", r.ManifestPath)
		fmt.Fprintf(w, "  size_bytes:        %d\n", r.SizeBytes)
		fmt.Fprintf(w, "  sha256:            %s\n", r.SHA256)
		fmt.Fprintf(w, "  tls_trust:         %s\n", r.TLSTrust)
	}
	if r.Reason != "" {
		fmt.Fprintf(w, "  reason:            %s\n", r.Reason)
	}
}

func writeVerifyHuman(w io.Writer, r *bundlesync.VerifyResult, bundlePath, manifestPath string) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "verify:              %s\n", r.State)
	if r.OK {
		fmt.Fprintf(w, "  ok:                true\n")
	}
	if r.Reason != "" {
		fmt.Fprintf(w, "  reason:            %s\n", r.Reason)
	}
	fmt.Fprintf(w, "  bundle:            %s\n", bundlePath)
	fmt.Fprintf(w, "  manifest:          %s\n", manifestPath)
	if r.ExpectedVersion != "" || r.ActualVersion != "" {
		fmt.Fprintf(w, "  version (expected/actual): %s / %s\n", r.ExpectedVersion, r.ActualVersion)
	}
	if r.ExpectedBuildID != "" || r.ActualBuildID != "" {
		fmt.Fprintf(w, "  build_id (expected/actual): %s / %s\n", r.ExpectedBuildID, r.ActualBuildID)
	}
	if r.ManifestSHA256 != "" {
		fmt.Fprintf(w, "  manifest sha256:   %s\n", r.ManifestSHA256)
	}
	if r.ActualSHA256 != "" && r.ActualSHA256 != r.ManifestSHA256 {
		fmt.Fprintf(w, "  actual sha256:     %s\n", r.ActualSHA256)
	}
	if len(r.TarViolations) > 0 {
		fmt.Fprintf(w, "  tar violations:    %d\n", len(r.TarViolations))
		for _, v := range r.TarViolations {
			fmt.Fprintf(w, "    %s: %s\n", v.Reason, v.Name)
		}
	}
}

func writeInstallHuman(w io.Writer, r *bundlesync.InstallResult) {
	if r == nil {
		return
	}
	fmt.Fprintf(w, "install:             %s\n", r.State)
	if r.OK {
		fmt.Fprintf(w, "  ok:                true\n")
		fmt.Fprintf(w, "  installed_path:    %s\n", r.InstalledPath)
		fmt.Fprintf(w, "  symlink_updated:   %v\n", r.SymlinkUpdated)
		fmt.Fprintf(w, "  already_present:   %v\n", r.AlreadyPresent)
		if r.PreviousActive != "" {
			fmt.Fprintf(w, "  previous_active:   %s\n", r.PreviousActive)
		}
		fmt.Fprintln(w, "note: a running MCP server may need to restart to load the new bundle.")
	}
	if r.Reason != "" {
		fmt.Fprintf(w, "  reason:            %s\n", r.Reason)
	}
}

func labelOrEmpty(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

// ── registration ─────────────────────────────────────────────────────────────

func init() {
	// Common flags shared across status/pull/verify/install/sync.
	for _, c := range []*cobra.Command{
		awarenessStatusCmd,
		awarenessPullCmd,
		awarenessVerifyCmd,
		awarenessInstallCmd,
		awarenessSyncCmd,
	} {
		c.Flags().BoolVar(&awarenessSyncCfg.json, "json", false, "Emit JSON instead of human-readable output")
		c.Flags().StringVar(&awarenessSyncCfg.releaseIndex, "release-index", "", "Path to release-index.json (default: "+defaultReleaseIndex+")")
		c.Flags().StringVar(&awarenessSyncCfg.bundleRoot, "bundle-root", "", "Active bundle layout root (default: "+defaultBundleRoot+")")
		c.Flags().StringVar(&awarenessSyncCfg.expectVersion, "version", "", "Override expected bundle version")
		c.Flags().StringVar(&awarenessSyncCfg.expectBuildID, "build-id", "", "Override expected bundle build_id")
	}

	// Pull-specific.
	for _, c := range []*cobra.Command{awarenessPullCmd, awarenessSyncCmd} {
		c.Flags().StringVar(&awarenessSyncCfg.from, "from", "", "Peer MCP base URL (e.g. https://10.0.0.8:10260) [REQUIRED]")
		c.Flags().StringVar(&awarenessSyncCfg.caPath, "ca", "", "Cluster CA cert path for TLS verification (default: "+defaultClusterCA+")")
		c.Flags().IntVar(&awarenessSyncCfg.timeoutSec, "timeout", 60, "Pull timeout (seconds)")
	}
	awarenessPullCmd.Flags().StringVar(&awarenessSyncCfg.outDir, "out", "", "Output directory for pulled bundle + manifest (default: current dir)")

	// Verify/install accept --manifest override.
	for _, c := range []*cobra.Command{awarenessVerifyCmd, awarenessInstallCmd} {
		c.Flags().StringVar(&awarenessSyncCfg.manifestPath, "manifest", "", "Path to sidecar manifest.json (default: alongside bundle)")
	}

	awarenessCmd.AddCommand(awarenessStatusCmd)
	awarenessCmd.AddCommand(awarenessPullCmd)
	awarenessCmd.AddCommand(awarenessVerifyCmd)
	awarenessCmd.AddCommand(awarenessInstallCmd)
	awarenessCmd.AddCommand(awarenessSyncCmd)
}
