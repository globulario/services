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

	"google.golang.org/protobuf/types/known/structpb"
)

// ── infrastructure.install ──────────────────────────────────────────────────
//
// Extracts an infrastructure component archive (etcd, minio, envoy, etc.)
// and places files in the appropriate system directories.
//
// Archive layout:
//
//	bin/       → /usr/lib/globular/bin/ (or $GLOBULAR_INSTALL_BIN_DIR)
//	systemd/   → /etc/systemd/system/  (or $GLOBULAR_INSTALL_SYSTEMD_DIR)
//	config/    → /etc/globular/{component}/
//
// After extraction, creates any specified data directories and runs
// systemctl daemon-reload if systemd units were written.
//
// Args:
//
//	name          (string, required) — component name (e.g. "etcd", "minio")
//	version       (string, required)
//	artifact_path (string, required) — path to .tar.gz archive
//	data_dirs     (string, optional) — comma-separated directories to create
type infrastructureInstallAction struct{}

func (infrastructureInstallAction) Name() string { return "infrastructure.install" }

func (infrastructureInstallAction) Validate(args *structpb.Struct) error {
	fields := args.GetFields()
	if strings.TrimSpace(fields["name"].GetStringValue()) == "" {
		return fmt.Errorf("infrastructure.install: name is required")
	}
	if strings.TrimSpace(fields["artifact_path"].GetStringValue()) == "" {
		return fmt.Errorf("infrastructure.install: artifact_path is required")
	}
	return nil
}

func (infrastructureInstallAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	component := strings.TrimSpace(fields["name"].GetStringValue())
	version := strings.TrimSpace(fields["version"].GetStringValue())
	artifactPath := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	dataDirsStr := strings.TrimSpace(fields["data_dirs"].GetStringValue())

	binDir, systemdDir, configDir, skipSystemd := installPaths()

	f, err := os.Open(artifactPath)
	if err != nil {
		return "", fmt.Errorf("infrastructure.install: open artifact: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("infrastructure.install: gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var wroteUnit bool
	fileCount := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("infrastructure.install: read tar: %w", err)
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
			if skipSystemd {
				continue
			}
			dest = filepath.Join(systemdDir, filepath.Base(name))
			wroteUnit = true
		case strings.HasPrefix(name, "config/"):
			dest = filepath.Join(configDir, component, strings.TrimPrefix(name, "config/"))
		default:
			continue
		}
		if dest == "" {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", fmt.Errorf("infrastructure.install: mkdir %s: %w", dest, err)
		}

		tmp := dest + ".tmp"
		df, err := os.Create(tmp)
		if err != nil {
			return "", fmt.Errorf("infrastructure.install: create %s: %w", tmp, err)
		}
		if _, err := io.Copy(df, tr); err != nil {
			df.Close()
			os.Remove(tmp)
			return "", fmt.Errorf("infrastructure.install: write %s: %w", dest, err)
		}
		if err := df.Chmod(hdr.FileInfo().Mode()); err != nil {
			df.Close()
			os.Remove(tmp)
			return "", fmt.Errorf("infrastructure.install: chmod %s: %w", dest, err)
		}
		df.Close()
		if err := os.Rename(tmp, dest); err != nil {
			os.Remove(tmp)
			return "", fmt.Errorf("infrastructure.install: rename %s: %w", dest, err)
		}
		fileCount++
	}

	// Reload systemd if units were written.
	if wroteUnit && !skipSystemd {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(cctx, "systemctl", "daemon-reload")
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("infrastructure.install: daemon-reload: %v (%s)", err, string(out))
		}
	}

	// Create data directories if specified.
	if dataDirsStr != "" {
		for _, dir := range strings.Split(dataDirsStr, ",") {
			dir = strings.TrimSpace(dir)
			if dir == "" {
				continue
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return "", fmt.Errorf("infrastructure.install: create data dir %s: %w", dir, err)
			}
		}
	}

	return fmt.Sprintf("infrastructure %s@%s installed (%d files)", component, version, fileCount), nil
}

// ── infrastructure.uninstall ────────────────────────────────────────────────
//
// Stops and removes an infrastructure component.
//
// Args:
//
//	name (string, required) — component name
//	unit (string, optional) — systemd unit name (default: globular-{name}.service)
type infrastructureUninstallAction struct{}

func (infrastructureUninstallAction) Name() string { return "infrastructure.uninstall" }

func (infrastructureUninstallAction) Validate(args *structpb.Struct) error {
	fields := args.GetFields()
	if strings.TrimSpace(fields["name"].GetStringValue()) == "" {
		return fmt.Errorf("infrastructure.uninstall: name is required")
	}
	return nil
}

func (infrastructureUninstallAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	component := strings.TrimSpace(fields["name"].GetStringValue())
	unit := strings.TrimSpace(fields["unit"].GetStringValue())
	if unit == "" {
		unit = "globular-" + component + ".service"
	}

	binDir, systemdDir, configDir, skipSystemd := installPaths()

	// Stop and disable the service unit.
	if !skipSystemd {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		// Best-effort stop.
		_ = exec.CommandContext(cctx, "systemctl", "stop", unit).Run()
		_ = exec.CommandContext(cctx, "systemctl", "disable", unit).Run()
	}

	// Remove binary.
	binPath := filepath.Join(binDir, component)
	_ = os.Remove(binPath)

	// Remove systemd unit.
	if !skipSystemd {
		unitPath := filepath.Join(systemdDir, unit)
		if err := os.Remove(unitPath); err == nil {
			cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			_ = exec.CommandContext(cctx, "systemctl", "daemon-reload").Run()
		}
	}

	// Remove config directory.
	cfgDir := filepath.Join(configDir, component)
	_ = os.RemoveAll(cfgDir)

	return fmt.Sprintf("infrastructure %s uninstalled (unit=%s)", component, unit), nil
}

func init() {
	Register(infrastructureInstallAction{})
	Register(infrastructureUninstallAction{})
}
