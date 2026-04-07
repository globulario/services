package actions

import (
	"context"
	"fmt"
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

func (infrastructureInstallAction) Apply(_ context.Context, args *structpb.Struct) (string, error) {
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

	return installerEngineInstall(component, version, stagingDir, dataDirsStr)
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
