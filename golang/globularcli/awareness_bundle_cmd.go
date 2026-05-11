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
	BuiltAt string `json:"built_at"`
	BuiltBy string `json:"built_by,omitempty"`
	SHA256  string `json:"sha256,omitempty"`

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
	Long: `Packages the awareness graph.db, docs/awareness/*.yaml, and failuregraph seeds
into a distributable tar.gz with an embedded manifest.json.

The output file is suitable for:
  globular package publish --kind AWARENESS_BUNDLE --file <output>

Prerequisites:
  - graph.db must already be built: globular awareness build
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
			return fmt.Errorf("graph.db not found at %s — run 'globular awareness build' first", dbPath)
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
			Name:    "globular-awareness-bundle",
			Kind:    "AWARENESS_BUNDLE",
			Version: version,
			BuildID: buildID,
			BuiltAt: time.Now().UTC().Format(time.RFC3339),
			BuiltBy: hostname,
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
		type entry struct {
			srcPath  string
			arcPath  string
		}
		var entries []entry

		// graph.db
		entries = append(entries, entry{srcPath: dbPath, arcPath: "graph.db"})

		// docs/awareness/*.yaml
		if repoRoot != "" {
			docsDir := filepath.Join(repoRoot, "docs", "awareness")
			if info, err := os.Stat(docsDir); err == nil && info.IsDir() {
				err := filepath.WalkDir(docsDir, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if d.IsDir() {
						return nil
					}
					rel, _ := filepath.Rel(docsDir, path)
					entries = append(entries, entry{srcPath: path, arcPath: filepath.Join("docs", rel)})
					return nil
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: docs walk: %v\n", err)
				}
			}
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
					entries = append(entries, entry{
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
					entries = append(entries, entry{srcPath: path, arcPath: filepath.Join("seeds", rel)})
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

		// Rewrite the manifest.json inside the archive with the sha256.
		// For simplicity, we print it and the operator adds it when publishing.
		fmt.Fprintf(os.Stdout, "\nBundle ready:\n")
		fmt.Fprintf(os.Stdout, "  file:     %s\n", outputName)
		fmt.Fprintf(os.Stdout, "  sha256:   %s\n", sha256sum)
		fmt.Fprintf(os.Stdout, "  build_id: %s\n", buildID)
		fmt.Fprintf(os.Stdout, "  files:    %d entries\n", written+1)
		fmt.Fprintf(os.Stdout, "\nTo publish:\n")
		fmt.Fprintf(os.Stdout, "  globular package publish --name globular-awareness-bundle --kind AWARENESS_BUNDLE --version %s --build-id %s --file %s\n",
			version, buildID, outputName)
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
		fmt.Fprintf(os.Stdout, "  name:     %s\n", manifest.Name)
		fmt.Fprintf(os.Stdout, "  kind:     %s\n", manifest.Kind)
		fmt.Fprintf(os.Stdout, "  version:  %s\n", manifest.Version)
		fmt.Fprintf(os.Stdout, "  build_id: %s\n", manifest.BuildID)
		fmt.Fprintf(os.Stdout, "  built_at: %s\n", manifest.BuiltAt)
		if manifest.BuiltBy != "" {
			fmt.Fprintf(os.Stdout, "  built_by: %s\n", manifest.BuiltBy)
		}
		fmt.Fprintf(os.Stdout, "  sha256:   %s\n", hex.EncodeToString(h[:]))
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
	awarenessBundleBuildCmd.Flags().StringVar(&bundleCfg.dbPath, "db", "", "Path to graph.db (default: system or repo path)")
	awarenessBundleBuildCmd.Flags().StringVar(&bundleCfg.version, "version", "", "Bundle version string (default: 0.0.1)")
	awarenessBundleBuildCmd.Flags().StringVar(&bundleCfg.buildID, "build-id", "", "Bundle build_id UUID (default: auto-generated)")
	awarenessBundleBuildCmd.Flags().StringVar(&bundleCfg.outputDir, "output-dir", "", "Directory for output tar.gz (default: current dir)")

	awarenessBundleCmd.AddCommand(awarenessBundleBuildCmd)
	awarenessBundleCmd.AddCommand(awarenessBundleInspectCmd)

	awarenessCmd.AddCommand(awarenessBundleCmd)
}
