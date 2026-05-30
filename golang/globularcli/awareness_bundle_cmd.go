package main

// awareness_bundle_cmd.go: CLI commands for building and inspecting awareness bundles.
//
// Usage:
//
//	globular awareness bundle build [--repo <path>] [--db <path>] [--version <ver>] [--build-id <id>] [--output <file>]
//	globular awareness bundle inspect <bundle.tar.gz>

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/awareness/bundlesync"
	"github.com/globulario/services/golang/opsknowledge"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// awarenessBundleManifest is the manifest written to manifest.json in every bundle.
type awarenessBundleManifest struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	Version string `json:"version"`
	BuildID string `json:"build_id"`
	// SchemaVersion identifies the bundle format. Set to
	// bundlesync.CurrentBundleSchemaVersion at build time so consumers
	// (mcp.awareness_freshness_status, bundlesync.LoadManifest, the
	// runtime selfcheck) can detect format drift before activating an
	// incompatible bundle. Older bundles wrote this field as empty; the
	// publish command tolerates an empty value but rejects an explicit
	// value outside SupportedSchemaVersions.
	SchemaVersion string `json:"schema_version,omitempty"`
	BuiltAt       string `json:"built_at"`
	BuiltBy       string `json:"built_by,omitempty"`
	SHA256        string `json:"sha256,omitempty"`
	SizeBytes     int64  `json:"size_bytes,omitempty"`

	// OpsKnowledgeEntries records the per-entry canonical SHA256 of every
	// operational-knowledge seed entry packed into ops-knowledge/. The
	// hash matches what golang/opsknowledge.HashEntry produces, and what
	// the seed CLI stamps into ai-memory's metadata.seed_sha256. Lets a
	// runtime check verify ai-memory has not drifted from the bundle.
	OpsKnowledgeEntries []opsKnowledgeManifestEntry `json:"ops_knowledge_entries,omitempty"`
}

// opsKnowledgeManifestEntry is one row in the bundle manifest's
// ops_knowledge_entries list.
type opsKnowledgeManifestEntry struct {
	ID         string `json:"id"`
	FilePath   string `json:"file_path"` // relative to ops-knowledge/, e.g. "stages/day-0-bootstrap.yaml"
	Type       string `json:"type"`
	Title      string `json:"title"`
	SeedSHA256 string `json:"seed_sha256"`
}

// bundleFileEntry pairs a source path on disk with the path the file should
// occupy inside the bundle tar.gz. Exported (to the package) so the bundle
// build's file-walk helpers stay reusable and testable.
type bundleFileEntry struct {
	srcPath string
	arcPath string
}

// collectDocsAwarenessEntries walks docs/awareness/ and returns one
// bundleFileEntry per regular file under it, with arcPath = "docs/<rel>".
// Every awareness knowledge file ships under this directory by convention,
// including:
//
//   - failure_modes.yaml, invariants.yaml, context_aliases.yaml
//   - detector_mapping.yaml (P1-5 regression: this file must be in the
//     bundle so consumers can rebuild detector → failure_mode edges; a
//     missing file silently degrades coverage on every node that installs
//     the bundle)
//   - design_patterns.yaml, fix_cases.yaml, awareness_self_invariants.yaml
//   - failuregraph_seeds/*.yaml, contracts/*.yaml, decisions/*.yaml
//
// The walk is intentionally generic so any new YAML added under
// docs/awareness/ is shipped without code change; the tests pin the
// critical files so a refactor that adds a filter cannot silently drop them.
//
// Returns an empty slice (not nil) and no error when the directory does not
// exist, matching the RunE caller's "warn and continue" semantics.
func collectDocsAwarenessEntries(docsDir string) ([]bundleFileEntry, error) {
	info, err := os.Stat(docsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []bundleFileEntry{}, nil
		}
		return nil, fmt.Errorf("stat docs/awareness: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("docs/awareness is not a directory: %s", docsDir)
	}
	var entries []bundleFileEntry
	walkErr := filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(docsDir, path)
		if relErr != nil {
			return relErr
		}
		entries = append(entries, bundleFileEntry{
			srcPath: path,
			arcPath: filepath.ToSlash(filepath.Join("docs", rel)),
		})
		return nil
	})
	if walkErr != nil {
		return entries, fmt.Errorf("walk docs/awareness: %w", walkErr)
	}
	return entries, nil
}

var bundleCfg = struct {
	repoPath  string
	dbPath    string
	version   string
	buildID   string
	outputDir string
}{}

var awarenessBundleCmd = &cobra.Command{
	Use:   "bundle",
	Short: "Awareness bundle — build and inspect distributable awareness artifacts",
	Long: `Awareness bundles package the compiled awareness graph, YAML knowledge files,
and failure-graph seeds into a signed release artifact.

Bundles are published to the Globular repository with kind=AWARENESS_BUNDLE and
distributed to joining nodes via the release BOM, exactly like service packages.

Each node unpacks the bundle to /usr/local/share/globular/awareness/<build_id>/
and activates it via the /var/lib/globular/awareness/current symlink.

The controller requires AWARENESS_READY (bundle installed) before a Day-1 node
advances to workload_ready.`,
}

var awarenessBundleBuildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build an awareness bundle tar.gz from the current graph and knowledge files",
	Long: `Packages the awareness graph.json, docs/awareness/*.yaml, and failuregraph seeds
into a distributable tar.gz with an embedded manifest.json.

The output file is suitable for:
  globular awareness bundle publish --file <output> --repository <addr>

Prerequisites:
  - graph.json must already be built: globular awareness build
  - docs/awareness/*.yaml must exist (or be at the configured path)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := resolveRepoRoot(bundleCfg.repoPath)
		if err != nil && bundleCfg.dbPath == "" {
			return fmt.Errorf("cannot resolve repo root and --db not set: %w", err)
		}

		dbPath := bundleCfg.dbPath
		if dbPath == "" {
			dbPath = resolveAwarenessDBPath(repoRoot)
		}
		if _, err := os.Stat(dbPath); err != nil {
			return fmt.Errorf("graph.json not found at %s — run 'globular awareness build' first", dbPath)
		}

		buildID := bundleCfg.buildID
		if buildID == "" {
			buildID = uuid.New().String()
		}

		version := bundleCfg.version
		if version == "" {
			version = "0.0.1"
		}

		hostname, _ := os.Hostname()
		manifest := awarenessBundleManifest{
			Name:          "globular-awareness-bundle",
			Kind:          "AWARENESS_BUNDLE",
			Version:       version,
			BuildID:       buildID,
			SchemaVersion: bundlesync.CurrentBundleSchemaVersion,
			BuiltAt:       time.Now().UTC().Format(time.RFC3339),
			BuiltBy:       hostname,
		}

		outputName := fmt.Sprintf("awareness-bundle-%s-%s.tar.gz", version, buildID[:8])
		if bundleCfg.outputDir != "" {
			outputName = filepath.Join(bundleCfg.outputDir, outputName)
		}

		fmt.Fprintf(os.Stdout, "Building awareness bundle\n")
		fmt.Fprintf(os.Stdout, "  db:       %s\n", dbPath)
		fmt.Fprintf(os.Stdout, "  build_id: %s\n", buildID)
		fmt.Fprintf(os.Stdout, "  version:  %s\n", version)
		fmt.Fprintf(os.Stdout, "  output:   %s\n\n", outputName)

		// Collect files to pack.
		var entries []bundleFileEntry

		// graph.json
		entries = append(entries, bundleFileEntry{srcPath: dbPath, arcPath: "graph.json"})

		// docs/awareness/*.yaml — every knowledge file (including
		// detector_mapping.yaml; see the helper's docstring).
		if repoRoot != "" {
			docsEntries, err := collectDocsAwarenessEntries(filepath.Join(repoRoot, "docs", "awareness"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: docs walk: %v\n", err)
			}
			entries = append(entries, docsEntries...)
		}

		// docs/operational-knowledge/{stages,runbooks,service-roles}/*.yaml
		// + per-entry canonical SHA256 stamped into the manifest so the
		// bundle is self-describing for ai-memory drift checks.
		if repoRoot != "" {
			opsDir := filepath.Join(repoRoot, "docs", "operational-knowledge")
			if info, err := os.Stat(opsDir); err == nil && info.IsDir() {
				files, err := opsknowledge.LoadDir(opsDir)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: ops-knowledge load: %v\n", err)
				}
				for _, f := range files {
					rel, _ := filepath.Rel(opsDir, f.Path)
					entries = append(entries, bundleFileEntry{
						srcPath: f.Path,
						arcPath: filepath.Join("ops-knowledge", rel),
					})
					for _, e := range f.Entries {
						hash, err := opsknowledge.HashEntry(e)
						if err != nil {
							fmt.Fprintf(os.Stderr, "warning: hash %s: %v\n", e.ID, err)
							continue
						}
						manifest.OpsKnowledgeEntries = append(manifest.OpsKnowledgeEntries, opsKnowledgeManifestEntry{
							ID:         e.ID,
							FilePath:   filepath.ToSlash(rel),
							Type:       e.Type,
							Title:      e.Title,
							SeedSHA256: hash,
						})
					}
				}
			}
		}

		// failuregraph seeds
		if repoRoot != "" {
			seedsDir := filepath.Join(repoRoot, "golang", "awareness", "failuregraph", "seeds")
			if info, err := os.Stat(seedsDir); err == nil && info.IsDir() {
				err := filepath.WalkDir(seedsDir, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if d.IsDir() {
						return nil
					}
					rel, _ := filepath.Rel(seedsDir, path)
					entries = append(entries, bundleFileEntry{srcPath: path, arcPath: filepath.Join("seeds", rel)})
					return nil
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: seeds walk: %v\n", err)
				}
			}
		}

		// Write tar.gz (manifest last so readers can seek to it if needed).
		f, err := os.Create(outputName)
		if err != nil {
			return fmt.Errorf("create output: %w", err)
		}
		defer f.Close()

		// Write via a hash so we can compute sha256 of the tar content.
		h := sha256.New()
		mw := io.MultiWriter(f, h)

		gw := gzip.NewWriter(mw)
		tw := tar.NewWriter(gw)

		written := 0
		for _, e := range entries {
			info, err := os.Stat(e.srcPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: stat %s: %v\n", e.srcPath, err)
				continue
			}
			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				continue
			}
			hdr.Name = e.arcPath
			if err := tw.WriteHeader(hdr); err != nil {
				return fmt.Errorf("tar header %s: %w", e.arcPath, err)
			}
			src, err := os.Open(e.srcPath)
			if err != nil {
				return fmt.Errorf("open %s: %w", e.srcPath, err)
			}
			n, err := io.Copy(tw, src)
			src.Close()
			if err != nil {
				return fmt.Errorf("pack %s: %w", e.arcPath, err)
			}
			fmt.Fprintf(os.Stdout, "  packed: %s (%d bytes)\n", e.arcPath, n)
			written++
		}

		// Write manifest.json.
		manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal manifest: %w", err)
		}
		mhdr := &tar.Header{
			Name:     "manifest.json",
			Mode:     0644,
			Size:     int64(len(manifestJSON)),
			ModTime:  time.Now(),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(mhdr); err != nil {
			return fmt.Errorf("tar manifest header: %w", err)
		}
		if _, err := tw.Write(manifestJSON); err != nil {
			return fmt.Errorf("write manifest: %w", err)
		}

		tw.Close()
		gw.Close()
		f.Close()

		sha256sum := hex.EncodeToString(h.Sum(nil))

		// The bundle sha256 here covers the whole archive (manifest + content)
		// and is what `awareness bundle publish` re-computes before upload.
		// The repository will record this as the artifact checksum.

		// Write a sidecar manifest.json next to the bundle, with the bundle's
		// sha256 and size populated. The install command (via
		// bundlesync.VerifyBundle) requires both to validate the on-disk
		// archive before swapping the active symlink — without the sidecar,
		// install fails with "manifest.sha256 is empty" even when the bundle
		// itself is valid. CI emits this sidecar as part of the artifact
		// pipeline; previously the local `globular awareness bundle build`
		// only printed sha256 to stdout, which broke `globular awareness
		// install` against locally-built bundles on every cluster.
		bundleInfo, statErr := os.Stat(outputName)
		if statErr != nil {
			return fmt.Errorf("stat finalised bundle %s: %w", outputName, statErr)
		}
		manifest.SHA256 = sha256sum
		manifest.SizeBytes = bundleInfo.Size()
		sidecarJSON, marshalErr := json.MarshalIndent(manifest, "", "  ")
		if marshalErr != nil {
			return fmt.Errorf("marshal sidecar manifest: %w", marshalErr)
		}
		sidecarPath := outputName + ".manifest.json"
		if writeErr := os.WriteFile(sidecarPath, sidecarJSON, 0644); writeErr != nil {
			return fmt.Errorf("write sidecar manifest %s: %w", sidecarPath, writeErr)
		}

		fmt.Fprintf(os.Stdout, "\nBundle ready:\n")
		fmt.Fprintf(os.Stdout, "  file:     %s\n", outputName)
		fmt.Fprintf(os.Stdout, "  sidecar:  %s\n", sidecarPath)
		fmt.Fprintf(os.Stdout, "  sha256:   %s\n", sha256sum)
		fmt.Fprintf(os.Stdout, "  size:     %d bytes\n", bundleInfo.Size())
		fmt.Fprintf(os.Stdout, "  build_id: %s\n", buildID)
		fmt.Fprintf(os.Stdout, "  files:    %d entries\n", written+1)
		fmt.Fprintf(os.Stdout, "\nTo publish:\n")
		fmt.Fprintf(os.Stdout, "  globular awareness bundle publish --file %s --repository <repository-address>\n",
			outputName)
		return nil
	},
}

var awarenessBundleInspectCmd = &cobra.Command{
	Use:   "inspect <bundle.tar.gz>",
	Short: "Inspect an awareness bundle and print its manifest",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read bundle: %w", err)
		}

		manifest, files, err := inspectBundle(data)
		if err != nil {
			return err
		}

		h := sha256.Sum256(data)
		fmt.Fprintf(os.Stdout, "Awareness bundle: %s\n\n", path)
		fmt.Fprintf(os.Stdout, "  name:           %s\n", manifest.Name)
		fmt.Fprintf(os.Stdout, "  kind:           %s\n", manifest.Kind)
		fmt.Fprintf(os.Stdout, "  version:        %s\n", manifest.Version)
		fmt.Fprintf(os.Stdout, "  build_id:       %s\n", manifest.BuildID)
		if manifest.SchemaVersion != "" {
			fmt.Fprintf(os.Stdout, "  schema_version: %s\n", manifest.SchemaVersion)
		}
		fmt.Fprintf(os.Stdout, "  built_at:       %s\n", manifest.BuiltAt)
		if manifest.BuiltBy != "" {
			fmt.Fprintf(os.Stdout, "  built_by:       %s\n", manifest.BuiltBy)
		}
		fmt.Fprintf(os.Stdout, "  sha256:         %s\n", hex.EncodeToString(h[:]))
		if n := len(manifest.OpsKnowledgeEntries); n > 0 {
			fmt.Fprintf(os.Stdout, "\nOperational-knowledge seed (%d entries):\n", n)
			for _, e := range manifest.OpsKnowledgeEntries {
				fmt.Fprintf(os.Stdout, "  %s  %s  %s\n", e.SeedSHA256[:12], e.Type, e.ID)
			}
		}
		fmt.Fprintf(os.Stdout, "\nContents (%d files):\n", len(files))
		for _, f := range files {
			fmt.Fprintf(os.Stdout, "  %s\n", f)
		}
		return nil
	},
}

func inspectBundle(data []byte) (*awarenessBundleManifest, []string, error) {
	gr, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		gr, err = gzip.NewReader(newBytesReader(data))
		if err != nil {
			return nil, nil, fmt.Errorf("gzip: %w", err)
		}
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	var manifest *awarenessBundleManifest
	var files []string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("tar: %w", err)
		}
		files = append(files, hdr.Name)
		if filepath.Base(hdr.Name) == "manifest.json" {
			raw, err := io.ReadAll(tr)
			if err != nil {
				return nil, nil, err
			}
			var m awarenessBundleManifest
			if err := json.Unmarshal(raw, &m); err != nil {
				return nil, nil, fmt.Errorf("parse manifest: %w", err)
			}
			manifest = &m
		}
	}
	if manifest == nil {
		return nil, files, fmt.Errorf("no manifest.json found in bundle")
	}
	return manifest, files, nil
}

// newBytesReader wraps []byte as an io.Reader for gzip.NewReader.
type bundleBytesReader struct{ data []byte; pos int }

func newBytesReader(data []byte) *bundleBytesReader { return &bundleBytesReader{data: data} }
func (r *bundleBytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func init() {
	awarenessBundleBuildCmd.Flags().StringVar(&bundleCfg.repoPath, "repo", "", "Repo root (default: auto-detected from git)")
	awarenessBundleBuildCmd.Flags().StringVar(&bundleCfg.dbPath, "db", "", "Path to graph.json (default: system or repo path)")
	awarenessBundleBuildCmd.Flags().StringVar(&bundleCfg.version, "version", "", "Bundle version string (default: 0.0.1)")
	awarenessBundleBuildCmd.Flags().StringVar(&bundleCfg.buildID, "build-id", "", "Bundle build_id UUID (default: auto-generated)")
	awarenessBundleBuildCmd.Flags().StringVar(&bundleCfg.outputDir, "output-dir", "", "Directory for output tar.gz (default: current dir)")

	awarenessBundleCmd.AddCommand(awarenessBundleBuildCmd)
	awarenessBundleCmd.AddCommand(awarenessBundleInspectCmd)

	awarenessCmd.AddCommand(awarenessBundleCmd)
}
