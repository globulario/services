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
//	scripts/   → extracted to staging; pre-install.sh runs before install,
//	             post-install.sh runs after install (like deb preinst/postinst)
//
// After extraction, creates any specified data directories, runs
// systemctl daemon-reload if systemd units were written, and executes
// scripts/post-install.sh if present in the archive (like a deb postinst).
//
// Lifecycle scripts receive these environment variables:
//
//	COMPONENT_NAME    — component name (e.g. "minio")
//	COMPONENT_VERSION — version being installed
//	STATE_DIR         — /var/lib/globular (or $GLOBULAR_STATE_DIR)
//	PREFIX            — /usr/lib/globular (or $GLOBULAR_INSTALL_PREFIX)
//	CONFIG_DIR        — /etc/globular (or $GLOBULAR_INSTALL_CONFIG_DIR)
//	NODE_IP           — routable IP of this node (best-effort detection)
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

	// Create a temporary directory for scripts extracted from the archive.
	scriptsDir, err := os.MkdirTemp("", "infra-scripts-*")
	if err != nil {
		return "", fmt.Errorf("infrastructure.install: create scripts dir: %w", err)
	}
	defer os.RemoveAll(scriptsDir)

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
		case strings.HasPrefix(name, "scripts/"):
			// Extract scripts to staging dir for post-install execution.
			dest = filepath.Join(scriptsDir, strings.TrimPrefix(name, "scripts/"))
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

	// Run pre-install script if present (like deb preinst).
	// Runs after extraction but before daemon-reload / service start, so it can
	// set up prerequisites: TLS certs, credentials, directories, permissions, etc.
	preInstall := filepath.Join(scriptsDir, "pre-install.sh")
	if _, err := os.Stat(preInstall); err == nil {
		if err := runLifecycleScript(ctx, preInstall, component, version, configDir); err != nil {
			return "", fmt.Errorf("infrastructure.install: pre-install script failed: %w", err)
		}
	}

	// Also run pre-start.sh (legacy name for MinIO-style pre-start setup).
	preStart := filepath.Join(scriptsDir, "pre-start.sh")
	if _, err := os.Stat(preStart); err == nil {
		if err := runLifecycleScript(ctx, preStart, component, version, configDir); err != nil {
			return "", fmt.Errorf("infrastructure.install: pre-start script failed: %w", err)
		}
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

	// Run post-install script if present in the archive (like deb postinst).
	// The script receives environment variables describing the component,
	// paths, and node IP so it can set up TLS, credentials, data dirs, etc.
	postInstall := filepath.Join(scriptsDir, "post-install.sh")
	if _, err := os.Stat(postInstall); err == nil {
		if err := runLifecycleScript(ctx, postInstall, component, version, configDir); err != nil {
			return "", fmt.Errorf("infrastructure.install: post-install script failed: %w", err)
		}
	}

	return fmt.Sprintf("infrastructure %s@%s installed (%d files)", component, version, fileCount), nil
}

// runLifecycleScript executes a lifecycle script (pre-install.sh, pre-start.sh,
// or post-install.sh) extracted from the package archive. The script runs with
// environment variables describing the component, paths, and node IP.
func runLifecycleScript(ctx context.Context, scriptPath, component, version, configDir string) error {
	// Ensure script is executable.
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		return fmt.Errorf("chmod %s: %w", filepath.Base(scriptPath), err)
	}

	stateDir := os.Getenv("GLOBULAR_STATE_DIR")
	if stateDir == "" {
		stateDir = "/var/lib/globular"
	}
	prefix := os.Getenv("GLOBULAR_INSTALL_PREFIX")
	if prefix == "" {
		prefix = "/usr/lib/globular"
	}

	scriptCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(scriptCtx, "/bin/bash", scriptPath)
	cmd.Env = append(os.Environ(),
		"COMPONENT_NAME="+component,
		"COMPONENT_VERSION="+version,
		"STATE_DIR="+stateDir,
		"PREFIX="+prefix,
		"CONFIG_DIR="+configDir,
		"NODE_IP="+detectNodeIP(),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", component, filepath.Base(scriptPath), err)
	}
	return nil
}

// detectNodeIP returns the routable IP of this node (best-effort).
func detectNodeIP() string {
	// Try the same method as setup-scylla-tls.sh: ip route get 8.8.8.8
	out, err := exec.Command("ip", "route", "get", "8.8.8.8").Output()
	if err == nil {
		fields := strings.Fields(string(out))
		for i, f := range fields {
			if f == "src" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
	// Fallback: hostname -I
	out, err = exec.Command("hostname", "-I").Output()
	if err == nil {
		parts := strings.Fields(string(out))
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return "127.0.0.1"
}

// ── infrastructure.uninstall ────────────────────────────────────────────────
//
// Stops and removes an infrastructure component.
// If a pre-remove.sh script was installed with the component (in
// /etc/globular/{component}/scripts/pre-remove.sh), it is executed
// before removing files.
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
