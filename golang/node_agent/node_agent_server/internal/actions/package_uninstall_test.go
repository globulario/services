package actions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestServiceUninstall_RemovesBinaryAndConfig(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	configDir := filepath.Join(dir, "config")
	systemdDir := filepath.Join(dir, "systemd")
	stDir := filepath.Join(dir, "state")
	versionsDir := filepath.Join(stDir, "versions", "gateway")

	os.MkdirAll(binDir, 0o755)
	os.MkdirAll(filepath.Join(configDir, "gateway"), 0o755)
	os.MkdirAll(systemdDir, 0o755)
	os.MkdirAll(versionsDir, 0o755)

	// Create fake binary, config, systemd unit, and version marker.
	os.WriteFile(filepath.Join(binDir, "gateway_server"), []byte("binary"), 0o755)
	os.WriteFile(filepath.Join(configDir, "gateway", "config.yaml"), []byte("config"), 0o644)
	os.WriteFile(filepath.Join(systemdDir, "globular-gateway.service"), []byte("[Unit]"), 0o644)
	os.WriteFile(filepath.Join(versionsDir, "version"), []byte("1.0.0"), 0o644)

	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)
	t.Setenv("GLOBULAR_INSTALL_CONFIG_DIR", configDir)
	t.Setenv("GLOBULAR_INSTALL_SYSTEMD_DIR", systemdDir)
	t.Setenv("GLOBULAR_SKIP_SYSTEMD", "1")
	t.Setenv("GLOBULAR_STATE_DIR", stDir)

	args, _ := structpb.NewStruct(map[string]interface{}{
		"name": "gateway",
		"kind": "SERVICE",
	})

	handler := Get("package.uninstall")
	if handler == nil {
		t.Fatal("package.uninstall action not registered")
	}

	msg, err := handler.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}
	if msg == "" {
		t.Fatal("expected non-empty message")
	}

	// Verify binary removed.
	if _, err := os.Stat(filepath.Join(binDir, "gateway_server")); !os.IsNotExist(err) {
		t.Error("binary should have been removed")
	}

	// Verify config directory removed.
	if _, err := os.Stat(filepath.Join(configDir, "gateway")); !os.IsNotExist(err) {
		t.Error("config directory should have been removed")
	}

	// Verify version marker removed.
	if _, err := os.Stat(versionsDir); !os.IsNotExist(err) {
		t.Error("version marker directory should have been removed")
	}

	// Systemd unit file is NOT removed when skipSystemd is true
	// (the file removal is gated by !skipSystemd in package_actions.go).
	if _, err := os.Stat(filepath.Join(systemdDir, "globular-gateway.service")); os.IsNotExist(err) {
		t.Error("systemd unit file should still exist when skipSystemd=true")
	}
}

func TestServiceUninstall_CustomUnit(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	systemdDir := filepath.Join(dir, "systemd")

	os.MkdirAll(binDir, 0o755)
	os.MkdirAll(systemdDir, 0o755)
	os.WriteFile(filepath.Join(binDir, "rbac_server"), []byte("binary"), 0o755)
	os.WriteFile(filepath.Join(systemdDir, "custom-rbac.service"), []byte("[Unit]"), 0o644)

	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)
	t.Setenv("GLOBULAR_INSTALL_SYSTEMD_DIR", systemdDir)
	t.Setenv("GLOBULAR_INSTALL_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("GLOBULAR_SKIP_SYSTEMD", "1")
	t.Setenv("GLOBULAR_STATE_DIR", filepath.Join(dir, "state"))

	args, _ := structpb.NewStruct(map[string]interface{}{
		"name": "rbac",
		"kind": "SERVICE",
		"unit": "custom-rbac.service",
	})

	handler := Get("package.uninstall")
	msg, err := handler.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("uninstall failed: %v", err)
	}
	if msg == "" {
		t.Fatal("expected non-empty message")
	}

	// Binary should be removed.
	if _, err := os.Stat(filepath.Join(binDir, "rbac_server")); !os.IsNotExist(err) {
		t.Error("binary should have been removed")
	}
}

func TestServiceUninstall_Idempotent(t *testing.T) {
	dir := t.TempDir()

	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", filepath.Join(dir, "bin"))
	t.Setenv("GLOBULAR_INSTALL_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("GLOBULAR_INSTALL_SYSTEMD_DIR", filepath.Join(dir, "systemd"))
	t.Setenv("GLOBULAR_SKIP_SYSTEMD", "1")
	t.Setenv("GLOBULAR_STATE_DIR", filepath.Join(dir, "state"))

	args, _ := structpb.NewStruct(map[string]interface{}{
		"name": "nonexistent",
		"kind": "SERVICE",
	})

	handler := Get("package.uninstall")
	_, err := handler.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("uninstall of non-existent service should succeed: %v", err)
	}
}

func TestApplicationUninstall_Idempotent(t *testing.T) {
	t.Setenv("GLOBULAR_STATE_DIR", t.TempDir())

	args, _ := structpb.NewStruct(map[string]interface{}{
		"name": "nonexistent-app",
	})

	handler := Get("application.uninstall")
	if handler == nil {
		t.Fatal("application.uninstall action not registered")
	}
	msg, err := handler.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("uninstall of non-existent app should succeed: %v", err)
	}
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
}
