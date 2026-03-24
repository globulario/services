package actions

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/globular-installer/pkg/installer"
	_ "github.com/globulario/globular-installer/pkg/platform/linux"
	"google.golang.org/protobuf/types/known/structpb"
)

// ── infrastructure.install ──────────────────────────────────────────────────
//
// Installs an infrastructure component (etcd, minio, envoy, etc.) using the
// shared installer engine when a spec is present in the package, or falls back
// to legacy archive extraction for old packages without specs.
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

	// Stage the package: extract .tgz to a temp directory.
	stagingDir, err := installer.ExtractPackageToTemp(artifactPath)
	if err != nil {
		return "", fmt.Errorf("infrastructure.install: stage package: %w", err)
	}
	defer os.RemoveAll(stagingDir)

	// Check if this package has a spec (package.json with spec ref, or specs/ dir).
	// If so, use the installer engine. Otherwise, fall back to legacy extraction.
	if hasInstallerSpec(stagingDir) {
		return installerEngineInstall(component, version, stagingDir, dataDirsStr)
	}

	return legacyInfraInstall(ctx, component, version, artifactPath, dataDirsStr)
}

// hasInstallerSpec returns true if the staged package contains a spec that the
// installer engine can use: either a package.json with a non-empty defaults.spec
// pointing to an existing file, or a specs/ directory with at least one .yaml/.yml file.
func hasInstallerSpec(stagingDir string) bool {
	// Check for package.json with a valid spec reference.
	if specPath, ok := loadManifestDefaults(stagingDir); ok {
		info, err := os.Stat(specPath)
		if err == nil && !info.IsDir() {
			return true
		}
		return false
	}
	// Check for specs/ directory with at least one YAML file.
	specsDir := filepath.Join(stagingDir, "specs")
	info, err := os.Stat(specsDir)
	if err != nil || !info.IsDir() {
		return false
	}
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".yaml" || ext == ".yml" {
			return true
		}
	}
	return false
}

// loadManifestDefaults reads package.json from stagingDir and returns the
// resolved spec path if defaults.spec is non-empty.
func loadManifestDefaults(stagingDir string) (specPath string, ok bool) {
	pkgJSON := filepath.Join(stagingDir, "package.json")
	data, err := os.ReadFile(pkgJSON)
	if err != nil {
		return "", false
	}
	var manifest struct {
		Defaults struct {
			Spec string `json:"spec"`
		} `json:"defaults"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return "", false
	}
	spec := strings.TrimSpace(manifest.Defaults.Spec)
	if spec == "" {
		return "", false
	}
	return filepath.Join(stagingDir, spec), true
}

// installerEngineInstall delegates infra installation to the shared installer engine.
func installerEngineInstall(component, version, stagingDir, dataDirsStr string) (string, error) {
	log.Printf("[installer-engine] using installer engine for %s@%s", component, version)

	opts := installer.Options{
		Version:    version,
		StagingDir: stagingDir,
		Force:      true,
		Verbose:    true,
	}

	// Inherit paths from environment if set, matching legacy behavior.
	if v := os.Getenv("GLOBULAR_INSTALL_PREFIX"); v != "" {
		opts.Prefix = v
	}
	if v := os.Getenv("GLOBULAR_STATE_DIR"); v != "" {
		opts.StateDir = v
	}
	if v := os.Getenv("GLOBULAR_INSTALL_CONFIG_DIR"); v != "" {
		opts.ConfigDir = v
	}

	ictx, err := installer.NewContext(opts)
	if err != nil {
		return "", fmt.Errorf("infrastructure.install: create installer context: %w", err)
	}

	report, err := installer.Install(ictx)
	if err != nil {
		return "", fmt.Errorf("infrastructure.install: installer engine failed for %s: %w", component, err)
	}

	// Create data directories if specified (same as legacy path).
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

	stepCount := len(report.Results)
	failedCount := report.ErrorCount()
	return fmt.Sprintf("infrastructure %s@%s installed via installer engine (%d steps, %d failed)", component, version, stepCount, failedCount), nil
}

// legacyInfraInstall is the original extraction-based install path for packages
// that do not contain an installer spec. This will be removed once all infra
// packages are rebuilt with specs.
func legacyInfraInstall(ctx context.Context, component, version, artifactPath, dataDirsStr string) (string, error) {
	log.Printf("[installer-engine] falling back to legacy install for %s@%s (no spec found)", component, version)

	binDir, systemdDir, configDir, skipSystemd := installPaths()

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

	preInstall := filepath.Join(scriptsDir, "pre-install.sh")
	if _, err := os.Stat(preInstall); err == nil {
		if err := runLifecycleScript(ctx, preInstall, component, version, configDir); err != nil {
			return "", fmt.Errorf("infrastructure.install: pre-install script failed: %w", err)
		}
	}

	preStart := filepath.Join(scriptsDir, "pre-start.sh")
	if _, err := os.Stat(preStart); err == nil {
		if err := runLifecycleScript(ctx, preStart, component, version, configDir); err != nil {
			return "", fmt.Errorf("infrastructure.install: pre-start script failed: %w", err)
		}
	}

	if wroteUnit && !skipSystemd {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(cctx, "systemctl", "daemon-reload")
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("infrastructure.install: daemon-reload: %v (%s)", err, string(out))
		}
	}

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

	postInstall := filepath.Join(scriptsDir, "post-install.sh")
	if _, err := os.Stat(postInstall); err == nil {
		if err := runLifecycleScript(ctx, postInstall, component, version, configDir); err != nil {
			return "", fmt.Errorf("infrastructure.install: post-install script failed: %w", err)
		}
	}

	return fmt.Sprintf("infrastructure %s@%s installed (%d files, legacy path)", component, version, fileCount), nil
}

// runLifecycleScript executes a lifecycle script (pre-install.sh, pre-start.sh,
// or post-install.sh) extracted from the package archive.
func runLifecycleScript(ctx context.Context, scriptPath, component, version, configDir string) error {
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
	out, err := exec.Command("ip", "route", "get", "8.8.8.8").Output()
	if err == nil {
		fields := strings.Fields(string(out))
		for i, f := range fields {
			if f == "src" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
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

	if !skipSystemd {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		_ = exec.CommandContext(cctx, "systemctl", "stop", unit).Run()
		_ = exec.CommandContext(cctx, "systemctl", "disable", unit).Run()
	}

	binPath := filepath.Join(binDir, component)
	_ = os.Remove(binPath)

	if !skipSystemd {
		unitPath := filepath.Join(systemdDir, unit)
		if err := os.Remove(unitPath); err == nil {
			cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			_ = exec.CommandContext(cctx, "systemctl", "daemon-reload").Run()
		}
	}

	cfgDir := filepath.Join(configDir, component)
	_ = os.RemoveAll(cfgDir)

	return fmt.Sprintf("infrastructure %s uninstalled (unit=%s)", component, unit), nil
}

func init() {
	Register(infrastructureInstallAction{})
	Register(infrastructureUninstallAction{})
}
