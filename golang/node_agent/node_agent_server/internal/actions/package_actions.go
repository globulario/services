package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

// ── package.install ─────────────────────────────────────────────────────────
//
// Generic package extraction. Delegates to kind-specific logic:
//   - SERVICE → same as service.install_payload (bin/, systemd/, config/)
//   - APPLICATION → extracts to webroot (handled by application.install)
//   - INFRASTRUCTURE → extracts to infra paths (handled by infrastructure.install)
//
// For SERVICE kind, this is a convenience wrapper around the existing
// service.install_payload logic with kind-awareness and report_state built in.
//
// Args:
//
//	name          (string, required)
//	version       (string, required)
//	artifact_path (string, required)
//	kind          (string, required: "SERVICE", "APPLICATION", "INFRASTRUCTURE")
type packageInstallAction struct{}

func (packageInstallAction) Name() string { return "package.install" }

func (packageInstallAction) Validate(args *structpb.Struct) error {
	fields := args.GetFields()
	if strings.TrimSpace(fields["name"].GetStringValue()) == "" {
		return fmt.Errorf("package.install: name is required")
	}
	if strings.TrimSpace(fields["artifact_path"].GetStringValue()) == "" {
		return fmt.Errorf("package.install: artifact_path is required")
	}
	kind := strings.ToUpper(strings.TrimSpace(fields["kind"].GetStringValue()))
	if kind == "" {
		return fmt.Errorf("package.install: kind is required")
	}
	return nil
}

func (packageInstallAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	name := strings.TrimSpace(fields["name"].GetStringValue())
	version := strings.TrimSpace(fields["version"].GetStringValue())
	kind := strings.ToUpper(strings.TrimSpace(fields["kind"].GetStringValue()))

	switch kind {
	case "SERVICE":
		// Delegate to existing service.install_payload handler.
		handler := Get("service.install_payload")
		if handler == nil {
			return "", fmt.Errorf("package.install: service.install_payload action not registered")
		}
		// Map package args to service args.
		svcArgs, _ := structpb.NewStruct(map[string]interface{}{
			"service":       name,
			"version":       version,
			"artifact_path": fields["artifact_path"].GetStringValue(),
		})
		return handler.Apply(ctx, svcArgs)

	case "APPLICATION":
		handler := Get("application.install")
		if handler == nil {
			return "", fmt.Errorf("package.install: application.install action not registered")
		}
		return handler.Apply(ctx, args)

	case "INFRASTRUCTURE":
		handler := Get("infrastructure.install")
		if handler == nil {
			return "", fmt.Errorf("package.install: infrastructure.install action not registered")
		}
		return handler.Apply(ctx, args)

	default:
		return "", fmt.Errorf("package.install: unsupported kind %q", kind)
	}
}

// ── package.uninstall ───────────────────────────────────────────────────────
//
// Removes installed package files. Delegates to kind-specific uninstall.
//
// Args:
//
//	name (string, required)
//	kind (string, required)
type packageUninstallAction struct{}

func (packageUninstallAction) Name() string { return "package.uninstall" }

func (packageUninstallAction) Validate(args *structpb.Struct) error {
	fields := args.GetFields()
	if strings.TrimSpace(fields["name"].GetStringValue()) == "" {
		return fmt.Errorf("package.uninstall: name is required")
	}
	if strings.TrimSpace(fields["kind"].GetStringValue()) == "" {
		return fmt.Errorf("package.uninstall: kind is required")
	}
	return nil
}

func (packageUninstallAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	kind := strings.ToUpper(strings.TrimSpace(fields["kind"].GetStringValue()))

	switch kind {
	case "SERVICE":
		name := strings.TrimSpace(fields["name"].GetStringValue())
		unit := strings.TrimSpace(fields["unit"].GetStringValue())
		if unit == "" {
			unit = "globular-" + name + ".service"
		}

		binDir, systemdDir, configDir, skipSystemd := installPaths()

		// Stop and disable the systemd unit before removing files.
		if !skipSystemd {
			cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			// Best-effort stop and disable — unit may not exist.
			_ = exec.CommandContext(cctx, "systemctl", "stop", unit).Run()
			_ = exec.CommandContext(cctx, "systemctl", "disable", unit).Run()
			cancel()
		}

		// Remove binary.
		binPath := filepath.Join(binDir, executableForService(name))
		_ = os.Remove(binPath)

		// Remove systemd unit file and reload.
		if !skipSystemd {
			unitPath := filepath.Join(systemdDir, unit)
			if err := os.Remove(unitPath); err == nil {
				cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
				_ = exec.CommandContext(cctx, "systemctl", "daemon-reload").Run()
				cancel()
			}
		}

		// Remove package-owned config directory.
		// User-managed config in other locations is preserved.
		cfgDir := filepath.Join(configDir, name)
		_ = os.RemoveAll(cfgDir)

		// Remove generated authorization policy files for this service.
		policyDir := filepath.Join(ActionPolicyDir, name)
		_ = os.RemoveAll(policyDir)

		// Remove version marker.
		markerDir := filepath.Join(stateDir(), "versions", name)
		_ = os.RemoveAll(markerDir)

		return fmt.Sprintf("service %s uninstalled (unit=%s)", name, unit), nil

	case "APPLICATION":
		handler := Get("application.uninstall")
		if handler == nil {
			return "", fmt.Errorf("package.uninstall: application.uninstall action not registered")
		}
		return handler.Apply(ctx, args)

	case "INFRASTRUCTURE":
		handler := Get("infrastructure.uninstall")
		if handler == nil {
			return "", fmt.Errorf("package.uninstall: infrastructure.uninstall action not registered")
		}
		return handler.Apply(ctx, args)

	default:
		return "", fmt.Errorf("package.uninstall: unsupported kind %q", kind)
	}
}

// ── package.verify ──────────────────────────────────────────────────────────
//
// Verifies an installed package's integrity by checking its binary checksum.
//
// Args:
//
//	name              (string, required)
//	kind              (string, required)
//	expected_checksum (string, optional — if empty, just checks file exists)
type packageVerifyAction struct{}

func (packageVerifyAction) Name() string { return "package.verify" }

func (packageVerifyAction) Validate(args *structpb.Struct) error {
	fields := args.GetFields()
	if strings.TrimSpace(fields["name"].GetStringValue()) == "" {
		return fmt.Errorf("package.verify: name is required")
	}
	return nil
}

func (packageVerifyAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	name := strings.TrimSpace(fields["name"].GetStringValue())
	kind := strings.ToUpper(strings.TrimSpace(fields["kind"].GetStringValue()))
	expected := strings.TrimSpace(fields["expected_checksum"].GetStringValue())

	// Resolve the installed binary/content path based on kind.
	var targetPath string
	switch kind {
	case "SERVICE", "":
		binDir, _, _, _ := installPaths()
		targetPath = filepath.Join(binDir, executableForService(name))
	case "APPLICATION":
		targetPath = filepath.Join(appsDir(), name)
	case "INFRASTRUCTURE":
		binDir, _, _, _ := installPaths()
		targetPath = filepath.Join(binDir, name)
	default:
		return "", fmt.Errorf("package.verify: unsupported kind %q", kind)
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		return "", fmt.Errorf("package.verify: %s not found at %s", name, targetPath)
	}

	if expected == "" {
		return fmt.Sprintf("package %s exists (%d bytes)", name, info.Size()), nil
	}

	// For directories (applications), skip checksum.
	if info.IsDir() {
		return fmt.Sprintf("package %s directory exists", name), nil
	}

	actual, err := fileSHA256(targetPath)
	if err != nil {
		return "", fmt.Errorf("package.verify: checksum %s: %w", targetPath, err)
	}
	if actual != expected {
		return "", fmt.Errorf("package.verify: checksum mismatch for %s: got %s, expected %s", name, actual, expected)
	}
	return fmt.Sprintf("package %s verified (sha256=%s)", name, actual[:16]+"..."), nil
}

// ── helpers ─────────────────────────────────────────────────────────────────

func stateDir() string {
	return ActionStateDir
}

func appsDir() string {
	return filepath.Join(stateDir(), "applications")
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func init() {
	Register(packageInstallAction{})
	Register(packageUninstallAction{})
	Register(packageVerifyAction{})
}
