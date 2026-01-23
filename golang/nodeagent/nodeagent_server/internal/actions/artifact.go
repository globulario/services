package actions

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/plan/versionutil"
	"google.golang.org/protobuf/types/known/structpb"
)

// artifact.fetch copies a local artifact into a deterministic staging path.
// It supports local sources only for now; remote fetch can be added later.
type artifactFetchAction struct{}

func (artifactFetchAction) Name() string { return "artifact.fetch" }

func (artifactFetchAction) Validate(args *structpb.Struct) error { return nil }

func (artifactFetchAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	source := strings.TrimSpace(fields["source"].GetStringValue())
	dest := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	service := strings.TrimSpace(fields["service"].GetStringValue())
	version := strings.TrimSpace(fields["version"].GetStringValue())
	platform := strings.TrimSpace(fields["platform"].GetStringValue())
	if dest == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create dest dir: %w", err)
	}
	if source == "" {
		if service == "" || version == "" || platform == "" {
			return "", fmt.Errorf("source is required when artifact not present")
		}
		source = resolveArtifactPath(service, version, platform)
	}
	if _, err := os.Stat(source); err != nil {
		return "", fmt.Errorf("artifact not found at %s: %w", source, err)
	}
	if _, err := os.Stat(dest); err == nil {
		return "artifact already present", nil
	}
	in, err := os.Open(source)
	if err != nil {
		return "", fmt.Errorf("open source: %w", err)
	}
	defer in.Close()
	if err := copyFileAtomic(dest, in); err != nil {
		return "", err
	}
	return "artifact fetched", nil
}

// artifact.verify performs a simple existence/digest check if provided.
type artifactVerifyAction struct{}

func (artifactVerifyAction) Name() string { return "artifact.verify" }

func (artifactVerifyAction) Validate(args *structpb.Struct) error { return nil }

func (artifactVerifyAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	path := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	if path == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("artifact missing: %w", err)
	}
	// TODO: add sha256 verification when digest provided.
	return "artifact verified", nil
}

type serviceInstallPayloadAction struct{}

func (serviceInstallPayloadAction) Name() string { return "service.install_payload" }

func (serviceInstallPayloadAction) Validate(args *structpb.Struct) error { return nil }

func (serviceInstallPayloadAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	service := strings.TrimSpace(fields["service"].GetStringValue())
	artifact := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	version := strings.TrimSpace(fields["version"].GetStringValue())
	if service == "" {
		return "", fmt.Errorf("service is required")
	}
	if artifact == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	stateRoot := strings.TrimSpace(os.Getenv("GLOBULAR_STATE_DIR"))
	if stateRoot == "" {
		stateRoot = "/var/lib/globular"
	}
	stagingRoot := filepath.Join(stateRoot, "staging", service)
	if testRoot := os.Getenv("GLOBULAR_STAGING_ROOT"); testRoot != "" {
		stagingRoot = filepath.Join(testRoot, service)
	}
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return "", fmt.Errorf("create staging dir: %w", err)
	}
	if _, err := os.MkdirTemp(stagingRoot, "extract-"); err != nil {
		return "", fmt.Errorf("create extract dir: %w", err)
	}
	f, err := os.Open(artifact)
	if err != nil {
		return "", fmt.Errorf("open artifact: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	binDir, systemdDir, configDir, skipSystemd := installPaths()
	var wroteUnit bool

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar: %w", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		name := strings.TrimLeft(hdr.Name, "./")
		var dest string
		switch {
		case strings.HasPrefix(name, "bin/"):
			dest = filepath.Join(binDir, filepath.Base(name))
		case strings.HasPrefix(name, "systemd/"), strings.HasPrefix(name, "units/"):
			dest = filepath.Join(systemdDir, filepath.Base(name))
			wroteUnit = true
		case strings.HasPrefix(name, "config/"):
			dest = filepath.Join(configDir, service, strings.TrimPrefix(name, "config/"))
		default:
			// ignore unsupported paths
			continue
		}
		if dest == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", fmt.Errorf("mkdir for %s: %w", dest, err)
		}
		tmp := dest + ".tmp"
		df, err := os.Create(tmp)
		if err != nil {
			return "", fmt.Errorf("create %s: %w", tmp, err)
		}
		if _, err := io.Copy(df, tr); err != nil {
			df.Close()
			os.Remove(tmp)
			return "", fmt.Errorf("write %s: %w", dest, err)
		}
		if err := df.Chmod(hdr.FileInfo().Mode()); err != nil {
			df.Close()
			os.Remove(tmp)
			return "", fmt.Errorf("chmod %s: %w", dest, err)
		}
		if err := df.Close(); err != nil {
			os.Remove(tmp)
			return "", fmt.Errorf("close %s: %w", dest, err)
		}
		if err := os.Rename(tmp, dest); err != nil {
			os.Remove(tmp)
			return "", fmt.Errorf("rename %s: %w", dest, err)
		}
	}

	if wroteUnit && !skipSystemd {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(cctx, "systemctl", "daemon-reload")
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("systemctl daemon-reload: %v (output: %s)", err, string(out))
		}
	}

	if version == "" {
		version = filepath.Base(artifact)
	}

	return fmt.Sprintf("service payload installed version=%s", version), nil
}

type serviceWriteVersionMarkerAction struct{}

func (serviceWriteVersionMarkerAction) Name() string { return "service.write_version_marker" }

func (serviceWriteVersionMarkerAction) Validate(args *structpb.Struct) error { return nil }

func (serviceWriteVersionMarkerAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	service := strings.TrimSpace(fields["service"].GetStringValue())
	version := fields["version"].GetStringValue()
	path := strings.TrimSpace(fields["path"].GetStringValue())
	if service == "" {
		return "", fmt.Errorf("service is required")
	}
	if path == "" {
		path = versionutil.MarkerPath(service)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create marker dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(version), 0o644); err != nil {
		return "", fmt.Errorf("write marker: %w", err)
	}
	return "version marker written", nil
}

func resolveArtifactPath(service, version, platform string) string {
	root := strings.TrimSpace(os.Getenv("GLOBULAR_ARTIFACT_REPO_ROOT"))
	if root == "" {
		root = "/var/lib/globular/repository/artifacts"
	}
	filename := fmt.Sprintf("%s.%s.%s.tgz", service, version, platform)
	return filepath.Join(root, service, version, platform, filename)
}

func copyFileAtomic(dest string, r io.Reader) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), "artifact-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("copy artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename artifact: %w", err)
	}
	return nil
}

func installPaths() (binDir, systemdDir, configDir string, skipSystemd bool) {
	binDir = os.Getenv("GLOBULAR_INSTALL_BIN_DIR")
	if binDir == "" {
		binDir = "/usr/local/bin"
	}
	systemdDir = os.Getenv("GLOBULAR_INSTALL_SYSTEMD_DIR")
	if systemdDir == "" {
		systemdDir = "/etc/systemd/system"
	}
	configDir = os.Getenv("GLOBULAR_INSTALL_CONFIG_DIR")
	if configDir == "" {
		configDir = "/etc/globular"
	}
	skipSystemd = os.Getenv("GLOBULAR_SKIP_SYSTEMD") == "1"
	return
}

func init() {
	Register(artifactFetchAction{})
	Register(artifactVerifyAction{})
	Register(serviceInstallPayloadAction{})
	Register(serviceWriteVersionMarkerAction{})
}
