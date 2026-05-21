package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	repository_client "github.com/globulario/services/golang/repository/repository_client"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
)

const (
	// awarenessInstallBase is the versioned immutable install root.
	awarenessInstallBase = "/usr/local/share/globular/awareness"
	// awarenessCurrentLink is the symlink pointing to the active bundle.
	awarenessCurrentLink = "/var/lib/globular/awareness/current"
	// awarenessStateDir is the directory that holds current + runtime evidence.
	awarenessStateDir = "/var/lib/globular/awareness"
	// awarenessBundleName is the canonical artifact name in the repository.
	awarenessBundleName = "globular-awareness-bundle"
	// awarenessManifestFile is the manifest filename inside the bundle tar.
	awarenessManifestFile = "manifest.json"
)

// AwarenessBundleManifest is the manifest embedded in each awareness bundle tar.gz.
type AwarenessBundleManifest struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Version  string `json:"version"`
	BuildID  string `json:"build_id"`
	BuiltAt  string `json:"built_at"`
	BuiltBy  string `json:"built_by,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

// LoadInstalledAwarenessBuildID returns the build_id of the currently installed
// awareness bundle, or "" if none is installed.
func LoadInstalledAwarenessBuildID() string {
	manifestPath := filepath.Join(awarenessCurrentLink, awarenessManifestFile)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return ""
	}
	var m AwarenessBundleManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	return m.BuildID
}

// FetchAndInstallAwarenessBundle fetches the latest AWARENESS_BUNDLE artifact
// from the repository and installs it to the system awareness directory.
// It is idempotent: if the installed build_id already matches, it returns immediately.
// Non-fatal: all errors are logged and the function returns "" on failure.
func FetchAndInstallAwarenessBundle(ctx context.Context) string {
	repoAddr := config.ResolveLocalServiceAddr("repository.PackageRepository")
	if repoAddr == "" {
		log.Printf("awareness-bundle: repository address not available — skipping fetch")
		return ""
	}

	rc, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		log.Printf("awareness-bundle: connect to repository at %s: %v", repoAddr, err)
		return ""
	}
	defer rc.Close()

	// Resolve latest AWARENESS_BUNDLE version.
	versions, err := rc.GetArtifactVersions("", awarenessBundleName, "all")
	if err != nil || len(versions) == 0 {
		versions, err = rc.GetArtifactVersions("", awarenessBundleName, "")
		if err != nil || len(versions) == 0 {
			log.Printf("awareness-bundle: no published bundle found in repository")
			return ""
		}
	}

	// Use the last published version (repository returns them in ascending order).
	latest := versions[len(versions)-1]
	ref := latest.GetRef()
	if ref == nil {
		log.Printf("awareness-bundle: latest manifest has no ref")
		return ""
	}

	expectedBuildID := latest.GetBuildId()

	// Check if already installed.
	installed := LoadInstalledAwarenessBuildID()
	if installed != "" && installed == expectedBuildID {
		return installed
	}

	log.Printf("awareness-bundle: fetching %s v%s (build_id=%s)", ref.GetName(), ref.GetVersion(), expectedBuildID)

	data, err := rc.DownloadArtifact(ref)
	if err != nil {
		log.Printf("awareness-bundle: download failed: %v", err)
		return ""
	}

	buildID, err := installAwarenessBundle(data)
	if err != nil {
		log.Printf("awareness-bundle: install failed: %v", err)
		return ""
	}

	log.Printf("awareness-bundle: installed build_id=%s", buildID)
	return buildID
}

// installAwarenessBundle extracts a bundle tar.gz, writes it to the versioned
// install path, and updates the /var/lib/globular/awareness/current symlink.
func installAwarenessBundle(data []byte) (string, error) {
	// Parse manifest first to get build_id before extracting everything.
	manifest, err := extractManifest(data)
	if err != nil {
		return "", fmt.Errorf("read manifest: %w", err)
	}
	if manifest.BuildID == "" {
		return "", fmt.Errorf("manifest missing build_id")
	}

	destDir := filepath.Join(awarenessInstallBase, manifest.BuildID)

	// Already installed — just re-link in case the symlink is stale.
	if _, err := os.Stat(destDir); err == nil {
		// Heal a legacy 0700 install in place: the doctor (globular user)
		// must be able to read the manifest; older installs landed at 0700
		// because os.MkdirTemp's default. Idempotent on a 0755 dir.
		if cerr := os.Chmod(destDir, 0o755); cerr != nil {
			log.Printf("awareness-bundle: warning: chmod existing %s: %v", destDir, cerr)
		}
		if err := updateCurrentSymlink(destDir); err != nil {
			return "", err
		}
		return manifest.BuildID, nil
	}

	// Extract to a temp dir, then rename to make the install atomic.
	tmp, err := os.MkdirTemp(awarenessInstallBase, ".bundle-extract-*")
	if err != nil {
		// Parent dir may not exist yet.
		if mkErr := os.MkdirAll(awarenessInstallBase, 0755); mkErr != nil {
			return "", fmt.Errorf("create install base: %w", mkErr)
		}
		tmp, err = os.MkdirTemp(awarenessInstallBase, ".bundle-extract-*")
		if err != nil {
			return "", fmt.Errorf("create temp dir: %w", err)
		}
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	if err := extractTarGz(data, tmp); err != nil {
		return "", fmt.Errorf("extract bundle: %w", err)
	}

	// os.MkdirTemp creates the staging directory with mode 0700, and
	// os.Rename preserves that mode on the destination. cluster_doctor
	// runs as the unprivileged `globular` user and must read the
	// manifest + ops-knowledge entries on every sweep. Without this
	// chmod, every doctor scan fires ops_knowledge.seed_integrity with
	// "manifest unreadable" — the bundle is on disk, just unreachable.
	// Bundle content is world-public (signed release artifact), so 0755
	// is correct.
	if err := os.Chmod(tmp, 0o755); err != nil {
		log.Printf("awareness-bundle: warning: chmod staging dir: %v", err)
	}
	if err := os.Rename(tmp, destDir); err != nil {
		return "", fmt.Errorf("install bundle: %w", err)
	}
	// Prevent rename cleanup from removing the now-installed dir.
	tmp = ""

	if err := os.MkdirAll(awarenessStateDir, 0755); err != nil {
		return "", fmt.Errorf("create state dir: %w", err)
	}
	// Pre-create the writable runtime dir owned by the globular service user.
	// The MCP server runs as `globular` and opens the bundle in composite
	// mode (immutable bundle + writable runtime.db sibling). Without this
	// dir present and writable, composite-mode init fails with EPERM and
	// MCP drops to degraded "no graph" mode — silently breaking every
	// awareness query.
	if err := ensureAwarenessRuntimeDir(); err != nil {
		log.Printf("awareness-bundle: warning: prepare runtime dir: %v", err)
	}
	if err := updateCurrentSymlink(destDir); err != nil {
		return "", err
	}

	return manifest.BuildID, nil
}

// ensureAwarenessRuntimeDir creates /var/lib/globular/awareness/runtime/ and
// chowns it to the globular service user. Idempotent.
func ensureAwarenessRuntimeDir() error {
	runtimeDir := filepath.Join(awarenessStateDir, "runtime")
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", runtimeDir, err)
	}
	u, err := user.Lookup("globular")
	if err != nil {
		return fmt.Errorf("lookup globular user: %w", err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return fmt.Errorf("parse uid: %w", err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("parse gid: %w", err)
	}
	if err := os.Chown(runtimeDir, uid, gid); err != nil {
		return fmt.Errorf("chown %s to globular: %w", runtimeDir, err)
	}
	return nil
}

// updateCurrentSymlink atomically updates /var/lib/globular/awareness/current
// to point to destDir.
func updateCurrentSymlink(destDir string) error {
	tmp := awarenessCurrentLink + ".tmp." + fmt.Sprintf("%d", time.Now().UnixNano())
	if err := os.Symlink(destDir, tmp); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}
	if err := os.Rename(tmp, awarenessCurrentLink); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("activate symlink: %w", err)
	}
	return nil
}

// extractManifest reads manifest.json from a tar.gz without extracting everything.
func extractManifest(data []byte) (*AwarenessBundleManifest, error) {
	gr, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		// Try reading data directly as bytes.
		gr, err = gzip.NewReader(newBytesReader(data))
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		base := filepath.Base(hdr.Name)
		if base != awarenessManifestFile {
			continue
		}
		raw, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read manifest entry: %w", err)
		}
		var m AwarenessBundleManifest
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("parse manifest: %w", err)
		}
		return &m, nil
	}
	return nil, fmt.Errorf("manifest.json not found in bundle")
}

// extractTarGz extracts all entries from a .tar.gz blob into destDir.
// Path traversal is rejected.
func extractTarGz(data []byte, destDir string) error {
	gr, err := gzip.NewReader(newBytesReader(data))
	if err != nil {
		return fmt.Errorf("gzip open: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}

		// Reject path traversal.
		clean := filepath.Clean(hdr.Name)
		if strings.HasPrefix(clean, "..") {
			return fmt.Errorf("unsafe path in bundle: %s", hdr.Name)
		}

		dest := filepath.Join(destDir, clean)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", dest, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", dest, err)
			}
			f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("create file %s: %w", dest, err)
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("write file %s: %w", dest, err)
			}
			f.Close()
		}
	}
	return nil
}

type bytesReader struct {
	data []byte
	pos  int
}

func newBytesReader(data []byte) *bytesReader { return &bytesReader{data: data} }

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// scheduleAwarenessBundleFetch runs FetchAndInstallAwarenessBundle once and
// is designed to be called from a goroutine spawned after the heartbeat loop
// detects repository availability. It is not retried here — the heartbeat
// loop will call it again on the next cycle if needed.
func scheduleAwarenessBundleFetch(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	return FetchAndInstallAwarenessBundle(ctx)
}

// AwarenessBundleRef is exposed so the repository kind switch in syncRepoArtifactsToEtcd
// can skip AWARENESS_BUNDLE artifacts (they are not tracked as installed packages).
func IsAwarenessBundleRef(kindInt int32) bool {
	return kindInt == int32(repositorypb.ArtifactKind_AWARENESS_BUNDLE)
}
