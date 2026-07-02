package actions

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/globular-installer/pkg/installer"
	"google.golang.org/protobuf/types/known/structpb"
)

func createInfraArchive(t *testing.T, dest string) {
	t.Helper()
	f, err := os.Create(dest)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	add := func(name, content string, mode int64) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(content))}); err != nil {
			t.Fatalf("write header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write body %s: %v", name, err)
		}
	}
	add("bin/envoy", "new-binary", 0o755)
	add("systemd/globular-envoy.service", "[Unit]\nDescription=envoy-new\n", 0o644)
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
}

func TestInfrastructureInstall_FailureRollsBackPromotedFiles(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	systemdDir := filepath.Join(t.TempDir(), "systemd")
	stateDir := t.TempDir()
	ActionBinDir = binDir
	t.Cleanup(func() { ActionBinDir = "/usr/lib/globular/bin" })
	ActionSystemdDir = systemdDir
	t.Cleanup(func() { ActionSystemdDir = "/etc/systemd/system" })
	ActionStateDir = stateDir
	t.Cleanup(func() { ActionStateDir = "/var/lib/globular" })

	oldBin := filepath.Join(binDir, "envoy")
	oldUnit := filepath.Join(systemdDir, "globular-envoy.service")
	if err := os.MkdirAll(filepath.Dir(oldBin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(oldUnit), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldBin, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldUnit, []byte("[Unit]\nDescription=envoy-old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origRunner := infrastructureInstallRunner
	infrastructureInstallRunner = func(component, version, stagingDir, dataDirsStr string) (string, error) {
		targets, err := infrastructureTransactionTargets(component, stagingDir)
		if err != nil {
			return "", err
		}
		for _, target := range targets {
			if strings.HasSuffix(target, ".sha256") {
				if err := os.WriteFile(target, []byte("sidecar\n"), 0o644); err != nil {
					return "", err
				}
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(target, []byte("mutated"), 0o755); err != nil {
				return "", err
			}
		}
		return "", errors.New("engine failed after writes")
	}
	t.Cleanup(func() { infrastructureInstallRunner = origRunner })

	artifactPath := filepath.Join(t.TempDir(), "envoy.tgz")
	createInfraArchive(t, artifactPath)
	args, _ := structpb.NewStruct(map[string]interface{}{
		"name":           "envoy",
		"version":        "1.0.0",
		"artifact_path":  artifactPath,
		"transaction_id": "infra-rollback",
		"package_id":     "envoy",
	})
	_, err := (infrastructureInstallAction{}).Apply(context.Background(), args)
	if err == nil {
		t.Fatal("expected infrastructure install failure")
	}
	gotBin, err := os.ReadFile(oldBin)
	if err != nil {
		t.Fatalf("read rolled back binary: %v", err)
	}
	if string(gotBin) != "old-binary" {
		t.Fatalf("binary rollback failed: got %q", string(gotBin))
	}
	gotUnit, err := os.ReadFile(oldUnit)
	if err != nil {
		t.Fatalf("read rolled back unit: %v", err)
	}
	if string(gotUnit) != "[Unit]\nDescription=envoy-old\n" {
		t.Fatalf("unit rollback failed: got %q", string(gotUnit))
	}
	rec, err := loadInstallTransaction("infra-rollback")
	if err != nil {
		t.Fatalf("load transaction: %v", err)
	}
	if rec.Phase != InstallTxnPhaseRolledBack {
		t.Fatalf("phase = %q, want %q", rec.Phase, InstallTxnPhaseRolledBack)
	}
}

func TestInfrastructureInstall_RemovesUnitSidecarsAfterManagedInstall(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	systemdDir := filepath.Join(t.TempDir(), "systemd")
	stateDir := t.TempDir()
	ActionBinDir = binDir
	t.Cleanup(func() { ActionBinDir = "/usr/lib/globular/bin" })
	ActionSystemdDir = systemdDir
	t.Cleanup(func() { ActionSystemdDir = "/etc/systemd/system" })
	ActionStateDir = stateDir
	t.Cleanup(func() { ActionStateDir = "/var/lib/globular" })

	origRunner := infrastructureInstallRunner
	infrastructureInstallRunner = func(component, version, stagingDir, dataDirsStr string) (string, error) {
		targets, err := infrastructureTransactionTargets(component, stagingDir)
		if err != nil {
			return "", err
		}
		for _, target := range targets {
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return "", err
			}
			mode := os.FileMode(0o644)
			if filepath.Base(target) == "envoy" {
				mode = 0o755
			}
			if err := os.WriteFile(target, []byte("written"), mode); err != nil {
				return "", err
			}
		}
		return "ok", nil
	}
	t.Cleanup(func() { infrastructureInstallRunner = origRunner })

	artifactPath := filepath.Join(t.TempDir(), "envoy.tgz")
	createInfraArchive(t, artifactPath)
	args, _ := structpb.NewStruct(map[string]interface{}{
		"name":           "envoy",
		"version":        "1.0.0",
		"artifact_path":  artifactPath,
		"transaction_id": "infra-sidecar",
		"package_id":     "envoy",
	})
	if _, err := (infrastructureInstallAction{}).Apply(context.Background(), args); err != nil {
		t.Fatalf("apply infrastructure install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(systemdDir, "globular-envoy.service.sha256")); !os.IsNotExist(err) {
		t.Fatalf("managed infrastructure path must remove unit sidecar, got err=%v", err)
	}
	rec, err := loadInstallTransaction("infra-sidecar")
	if err != nil {
		t.Fatalf("load transaction: %v", err)
	}
	if rec.Phase != InstallTxnPhaseReloaded {
		t.Fatalf("phase = %q, want %q", rec.Phase, InstallTxnPhaseReloaded)
	}
}

func TestInstallerEngineInstall_MinioIsNonInteractiveAndPinsDataDir(t *testing.T) {
	stateDir := t.TempDir()
	ActionStateDir = stateDir
	t.Cleanup(func() { ActionStateDir = "/var/lib/globular" })

	var got installer.Options
	origNewContext := installerNewContext
	installerNewContext = func(opts installer.Options) (*installer.Context, error) {
		got = opts
		return nil, fmt.Errorf("stop after option capture")
	}
	t.Cleanup(func() { installerNewContext = origNewContext })

	_, err := installerEngineInstall("minio", "RELEASE.2025-09-07T16-13-09Z", t.TempDir(), "")
	if err == nil {
		t.Fatal("expected captured context error")
	}
	if !got.NonInteractive {
		t.Fatal("node-agent infrastructure install must run the installer non-interactively")
	}
	if !got.SkipStart {
		t.Fatal("node-agent infrastructure install must defer service start to the caller")
	}
	wantDataDir := filepath.Join(stateDir, "minio", "data")
	if got.MinioDataDir != wantDataDir {
		t.Fatalf("MinioDataDir = %q, want %q", got.MinioDataDir, wantDataDir)
	}
}
