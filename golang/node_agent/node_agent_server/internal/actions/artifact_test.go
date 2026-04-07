package actions

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions/serviceports"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestArtifactFetchResolvesRepoRoot(t *testing.T) {
	repo := t.TempDir()
	service, version, platform := "svc", "1.0.0", "linux_amd64"
	srcPath := filepath.Join(repo, service, version, platform)
	if err := os.MkdirAll(srcPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	artifact := filepath.Join(srcPath, "svc.1.0.0.linux_amd64.tgz")
	if err := os.WriteFile(artifact, []byte("data"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	ActionArtifactRepoRoot = repo
	t.Cleanup(func() { ActionArtifactRepoRoot = "/var/lib/globular/repository/artifacts" })

	dest := filepath.Join(t.TempDir(), "out.tgz")
	args, _ := structpb.NewStruct(map[string]interface{}{
		"service":       service,
		"version":       version,
		"platform":      platform,
		"artifact_path": dest,
	})
	a := artifactFetchAction{}
	if _, err := a.Apply(context.Background(), args); err != nil {
		t.Fatalf("apply: %v", err)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(b) != "data" {
		t.Fatalf("dest content mismatch: %s", string(b))
	}
}

func TestServiceInstallPayloadPromotesFiles(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	systemdDir := filepath.Join(t.TempDir(), "systemd")
	configDir := filepath.Join(t.TempDir(), "config")
	stagingRoot := t.TempDir()
	sr := t.TempDir()

	ActionBinDir = binDir
	t.Cleanup(func() { ActionBinDir = "/usr/lib/globular/bin" })
	ActionSystemdDir = systemdDir
	t.Cleanup(func() { ActionSystemdDir = "/etc/systemd/system" })
	ActionConfigDir = configDir
	t.Cleanup(func() { ActionConfigDir = "/etc/globular" })
	ActionSkipSystemd = true
	t.Cleanup(func() { ActionSkipSystemd = false })
	ActionStagingRoot = stagingRoot
	t.Cleanup(func() { ActionStagingRoot = "" })
	ActionStateDir = sr
	t.Cleanup(func() { ActionStateDir = "/var/lib/globular" })
	serviceports.PortRange = "62001-62005"
	t.Cleanup(func() { serviceports.PortRange = "" })
	serviceports.BinDir = binDir
	t.Cleanup(func() { serviceports.BinDir = "/usr/lib/globular/bin" })
	serviceports.StateDir = sr
	t.Cleanup(func() { serviceports.StateDir = "/var/lib/globular" })

	artifactPath := filepath.Join(t.TempDir(), "svc.tgz")
	createTestArchive(t, artifactPath)

	args, _ := structpb.NewStruct(map[string]interface{}{
		"service":       "svc",
		"version":       "1.0.0",
		"artifact_path": artifactPath,
	})
	a := serviceInstallPayloadAction{}
	if _, err := a.Apply(context.Background(), args); err != nil {
		t.Fatalf("apply install: %v", err)
	}

	checkFile := func(path, want string) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(b) != want {
			t.Fatalf("%s content mismatch: %s", path, string(b))
		}
	}

	// Verify extracted files exist (systemd files skipped because ActionSkipSystemd=true).
	if _, err := os.Stat(filepath.Join(binDir, "svc_server")); err != nil {
		t.Fatalf("binary not extracted: %v", err)
	}
	checkFile(filepath.Join(configDir, "svc", "app.yaml"), "cfg")

	// Port config should be generated for known services (none for generic svc)
}

func TestServiceInstallPayloadCreatesConfigWithPort(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	systemdDir := filepath.Join(t.TempDir(), "systemd")
	configDir := filepath.Join(t.TempDir(), "config")
	stagingRoot := t.TempDir()
	sr := t.TempDir()

	ActionBinDir = binDir
	t.Cleanup(func() { ActionBinDir = "/usr/lib/globular/bin" })
	ActionSystemdDir = systemdDir
	t.Cleanup(func() { ActionSystemdDir = "/etc/systemd/system" })
	ActionConfigDir = configDir
	t.Cleanup(func() { ActionConfigDir = "/etc/globular" })
	ActionSkipSystemd = true
	t.Cleanup(func() { ActionSkipSystemd = false })
	ActionStagingRoot = stagingRoot
	t.Cleanup(func() { ActionStagingRoot = "" })
	ActionStateDir = sr
	t.Cleanup(func() { ActionStateDir = "/var/lib/globular" })
	serviceports.PortRange = "63001-63003"
	t.Cleanup(func() { serviceports.PortRange = "" })
	serviceports.BinDir = binDir
	t.Cleanup(func() { serviceports.BinDir = "/usr/lib/globular/bin" })
	serviceports.StateDir = sr
	t.Cleanup(func() { serviceports.StateDir = "/var/lib/globular" })

	artifactPath := filepath.Join(t.TempDir(), "rbac.tgz")
	createDescribeArchive(t, artifactPath, "rbac_server", `{"Id":"rbac-id","Address":"localhost:63001"}`)

	args, _ := structpb.NewStruct(map[string]interface{}{
		"service":       "rbac",
		"version":       "1.0.0",
		"artifact_path": artifactPath,
	})
	a := serviceInstallPayloadAction{}
	if _, err := a.Apply(context.Background(), args); err != nil {
		t.Fatalf("apply install: %v", err)
	}

	cfgPath := filepath.Join(sr, "services", "rbac-id.json")
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read cfg: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("unmarshal cfg: %v", err)
	}
	port := int(cfg["Port"].(float64))
	if port < 63001 || port > 63003 {
		t.Fatalf("port out of range: %d", port)
	}
}

// start action re-validates and rewrites invalid configs before systemctl
func TestServiceStartPreflightRewritesOutOfRange(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	sr := t.TempDir()
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	ActionBinDir = binDir
	t.Cleanup(func() { ActionBinDir = "/usr/lib/globular/bin" })
	ActionStateDir = sr
	t.Cleanup(func() { ActionStateDir = "/var/lib/globular" })
	serviceports.PortRange = "65001-65002"
	t.Cleanup(func() { serviceports.PortRange = "" })

	// Also set serviceports BinDir/StateDir since EnsureServicePortReady uses them.
	serviceports.BinDir = binDir
	t.Cleanup(func() { serviceports.BinDir = "/usr/lib/globular/bin" })
	serviceports.StateDir = sr
	t.Cleanup(func() { serviceports.StateDir = "/var/lib/globular" })

	binPath := filepath.Join(binDir, "resource_server")
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"resource-id\",\"Address\":\"localhost:65001\"}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	cfgPath := filepath.Join(sr, "services", "resource-id.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// out-of-range port
	cfg := map[string]any{"Id": "resource-id", "Address": "localhost:42", "Port": 42}
	b, _ := json.Marshal(cfg)
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	// Call preflight directly
	if err := serviceports.EnsureServicePortReady(context.Background(), "resource", "globular-resource.service"); err != nil {
		t.Fatalf("preflight: %v", err)
	}

	out, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read cfg: %v", err)
	}
	var final map[string]any
	if err := json.Unmarshal(out, &final); err != nil {
		t.Fatalf("unmarshal final: %v", err)
	}
	port := int(final["Port"].(float64))
	if port < 65001 || port > 65002 {
		t.Fatalf("port not rewritten into range: %d", port)
	}
}

func createTestArchive(t *testing.T, dest string) {
	t.Helper()
	f, err := os.Create(dest)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	addFile := func(name, content string, mode int64) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(content))}); err != nil {
			t.Fatalf("write hdr: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write body: %v", err)
		}
	}

	addFile("bin/svc_server", "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"svc-id\",\"Address\":\"localhost:62001\"}'; fi", 0o755)
	addFile("systemd/testsvc.service", "unit", 0o644)
	addFile("config/app.yaml", "cfg", 0o644)

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gz: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
}

func createDescribeArchive(t *testing.T, dest, exeName, describeJSON string) {
	t.Helper()
	f, err := os.Create(dest)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	// binary with describe output
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '" + describeJSON + "'; else exit 0; fi\n"
	addFile := func(name, content string, mode int64) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(content))}); err != nil {
			t.Fatalf("write hdr: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write body: %v", err)
		}
	}
	addFile("bin/"+exeName, script, 0o755)

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gz: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
}
