package actions

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions/serviceports"
	"google.golang.org/protobuf/types/known/structpb"
)

// sha256Hex returns the lowercase hex SHA256 of the given bytes.
func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestArtifactFetchCacheHitVerified — Test 1 from the recovery plan:
// when the cached file matches the expected digest, fetch reuses it safely.
func TestArtifactFetchCacheHitVerified(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "cached.tgz")
	payload := []byte("cached-payload-build20")
	if err := os.WriteFile(dest, payload, 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	args, _ := structpb.NewStruct(map[string]interface{}{
		"artifact_path":   dest,
		"expected_sha256": sha256Hex(payload),
		"service":         "svc",
		"version":         "1.0.0",
		"platform":        "linux_amd64",
	})
	msg, err := artifactFetchAction{}.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if !strings.Contains(msg, "already present (verified)") {
		t.Fatalf("expected cache-hit-verified, got %q", msg)
	}
	// Destination must still contain the original payload.
	b, _ := os.ReadFile(dest)
	if string(b) != string(payload) {
		t.Fatalf("cache was clobbered: got %q", string(b))
	}
}

// TestArtifactFetchCacheMismatchRejected — Test 2 from the recovery plan:
// a corrupted cache file must be detected (not silently reused) and removed.
// We run fetch with no local source and no repo, so after the mismatch is
// detected, fetch should fail loudly — the point is to prove it NEVER
// returns success with stale bytes.
func TestArtifactFetchCacheMismatchRejected(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "cached.tgz")
	if err := os.WriteFile(dest, []byte("stale-build19-bytes"), 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	// The "correct" bytes would be something else — use its digest as expected.
	expected := sha256Hex([]byte("the-real-build20-bytes-that-are-not-here"))
	args, _ := structpb.NewStruct(map[string]interface{}{
		"artifact_path":   dest,
		"expected_sha256": expected,
		"service":         "svc",
		"version":         "1.0.0",
		"platform":        "linux_amd64",
	})
	// No repository, no local source → fetch should fail (correctly) rather
	// than silently return "already present".
	_, err := artifactFetchAction{}.Apply(context.Background(), args)
	if err == nil {
		t.Fatalf("expected error on cache mismatch with no source, got nil")
	}
	if strings.Contains(err.Error(), "already present") {
		t.Fatalf("fetch silently reused corrupt cache: %v", err)
	}
	// The stale file must have been removed (proving the mismatch path ran).
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Fatalf("corrupt cache file was not removed: %s", dest)
	}
}

// TestArtifactFetchCacheBlindReuseRefused — Test 4 from the recovery plan
// (+ defense-in-depth for Case C): when the caller passes NO expected
// checksum AND no way to resolve one (no repo address, no full identity),
// fetch must refuse blind cache reuse rather than silently return success.
func TestArtifactFetchCacheBlindReuseRefused(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "cached.tgz")
	if err := os.WriteFile(dest, []byte("unknown-provenance-bytes"), 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	// No expected_sha256, no repository_addr, no service/version/platform →
	// no way to validate identity. Must refuse.
	args, _ := structpb.NewStruct(map[string]interface{}{
		"artifact_path": dest,
	})
	_, err := artifactFetchAction{}.Apply(context.Background(), args)
	if err == nil {
		t.Fatalf("expected refuse-blind-reuse error, got nil (fetch silently reused cache)")
	}
	if !strings.Contains(err.Error(), "refuse blind cache reuse") {
		t.Fatalf("expected 'refuse blind cache reuse' error, got: %v", err)
	}
}

// TestArtifactFetchLocalSourceVerified exercises Test 3's contract:
// when a local source is available and the caller passes expected_sha256,
// the copy path validates the copied bytes before returning success.
func TestArtifactFetchLocalSourceVerified(t *testing.T) {
	repo := t.TempDir()
	service, version, platform := "svc", "1.0.0", "linux_amd64"
	srcPath := filepath.Join(repo, service, version, platform)
	if err := os.MkdirAll(srcPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	payload := []byte("published-build20-bytes")
	srcFile := filepath.Join(srcPath, "svc.1.0.0.linux_amd64.tgz")
	if err := os.WriteFile(srcFile, payload, 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	ActionArtifactRepoRoot = repo
	t.Cleanup(func() { ActionArtifactRepoRoot = "/var/lib/globular/repository/artifacts" })

	dest := filepath.Join(t.TempDir(), "out.tgz")
	args, _ := structpb.NewStruct(map[string]interface{}{
		"service":         service,
		"version":         version,
		"platform":        platform,
		"artifact_path":   dest,
		"expected_sha256": sha256Hex(payload),
	})
	msg, err := artifactFetchAction{}.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !strings.Contains(msg, "verified") {
		t.Fatalf("expected verification in result, got %q", msg)
	}
	b, _ := os.ReadFile(dest)
	if sha256Hex(b) != sha256Hex(payload) {
		t.Fatalf("dest bytes do not match published payload")
	}
}

// TestArtifactFetchLocalSourceMismatchFails verifies that if a local source
// exists but its bytes don't match the expected checksum, fetch fails loudly
// (instead of silently copying wrong bytes into place).
func TestArtifactFetchLocalSourceMismatchFails(t *testing.T) {
	repo := t.TempDir()
	service, version, platform := "svc", "1.0.0", "linux_amd64"
	srcPath := filepath.Join(repo, service, version, platform)
	if err := os.MkdirAll(srcPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	srcFile := filepath.Join(srcPath, "svc.1.0.0.linux_amd64.tgz")
	if err := os.WriteFile(srcFile, []byte("the-local-source-is-tampered"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	ActionArtifactRepoRoot = repo
	t.Cleanup(func() { ActionArtifactRepoRoot = "/var/lib/globular/repository/artifacts" })

	dest := filepath.Join(t.TempDir(), "out.tgz")
	args, _ := structpb.NewStruct(map[string]interface{}{
		"service":         service,
		"version":         version,
		"platform":        platform,
		"artifact_path":   dest,
		"expected_sha256": sha256Hex([]byte("what-the-caller-actually-wanted")),
	})
	_, err := artifactFetchAction{}.Apply(context.Background(), args)
	if err == nil {
		t.Fatalf("expected mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("expected sha256 mismatch error, got: %v", err)
	}
	if _, statErr := os.Stat(dest); statErr == nil {
		t.Fatalf("dest file should have been removed after mismatch")
	}
}

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

func TestServiceInstallPayloadExtractsPolicyFiles(t *testing.T) {
	binDir := t.TempDir()
	policyDir := t.TempDir()
	stagingRoot := t.TempDir()
	sr := t.TempDir()

	ActionBinDir = binDir
	t.Cleanup(func() { ActionBinDir = "/usr/lib/globular/bin" })
	ActionSkipSystemd = true
	t.Cleanup(func() { ActionSkipSystemd = false })
	ActionStagingRoot = stagingRoot
	t.Cleanup(func() { ActionStagingRoot = "" })
	ActionStateDir = sr
	t.Cleanup(func() { ActionStateDir = "/var/lib/globular" })
	ActionPolicyDir = policyDir
	t.Cleanup(func() { ActionPolicyDir = "/var/lib/globular/policy/services" })

	// Build an archive that contains policy files.
	artifactPath := filepath.Join(t.TempDir(), "svc.tgz")
	f, _ := os.Create(artifactPath)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	writeEntry := func(name, content string) {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}) //nolint:errcheck
		tw.Write([]byte(content))                                                         //nolint:errcheck
	}
	writeEntry("bin/svc_server", "#!/bin/sh")
	writeEntry("policy/permissions.generated.json", `{"schema_version":"2","permissions":[]}`)
	writeEntry("policy/roles.generated.json", `{"schema_version":"2","roles":[]}`)
	tw.Close() //nolint:errcheck
	gz.Close() //nolint:errcheck
	f.Close()  //nolint:errcheck

	args, _ := structpb.NewStruct(map[string]interface{}{
		"service":       "svc",
		"version":       "1.0.0",
		"artifact_path": artifactPath,
	})
	if _, err := (serviceInstallPayloadAction{}).Apply(context.Background(), args); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Verify permissions file installed to per-service policy directory.
	permFile := filepath.Join(policyDir, "svc", "permissions.generated.json")
	if _, err := os.Stat(permFile); err != nil {
		t.Fatalf("permissions.generated.json not installed: %v", err)
	}
	rolesFile := filepath.Join(policyDir, "svc", "roles.generated.json")
	if _, err := os.Stat(rolesFile); err != nil {
		t.Fatalf("roles.generated.json not installed: %v", err)
	}
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
	createDescribeArchive(t, artifactPath, "rbac_server", `{"Id":"rbac.RbacService","Address":"localhost:63001"}`)

	args, _ := structpb.NewStruct(map[string]interface{}{
		"service":       "rbac",
		"version":       "1.0.0",
		"artifact_path": artifactPath,
	})
	a := serviceInstallPayloadAction{}
	if _, err := a.Apply(context.Background(), args); err != nil {
		t.Fatalf("apply install: %v", err)
	}

	cfgPath := filepath.Join(sr, "services", "rbac.RbacService.json")
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
	script := "#!/bin/sh\nif [ \"$1\" = \"--describe\" ]; then echo '{\"Id\":\"resource.ResourceService\",\"Address\":\"localhost:65001\"}'; fi\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	cfgPath := filepath.Join(sr, "services", "resource.ResourceService.json")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// out-of-range port
	cfg := map[string]any{"Id": "resource.ResourceService", "Address": "localhost:42", "Port": 42}
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
