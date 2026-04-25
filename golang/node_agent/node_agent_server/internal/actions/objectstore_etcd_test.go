package actions

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/config"
)

// sampleContract returns a minimal valid MinioProxyConfig for testing.
func sampleContract(endpoint string) *config.MinioProxyConfig {
	return &config.MinioProxyConfig{
		Endpoint: endpoint,
		Bucket:   "globular",
		Secure:   false,
		Auth: &config.MinioProxyAuth{
			Mode:      config.MinioProxyAuthModeAccessKey,
			AccessKey: "testkey",
			SecretKey: "testsecret",
		},
	}
}

// writeContractFile writes a MinioProxyConfig as a contract JSON file.
func writeContractFile(t *testing.T, path string, cfg *config.MinioProxyConfig) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	var buf bytes.Buffer
	if err := config.SaveMinioProxyConfigTo(&buf, cfg); err != nil {
		t.Fatalf("write contract: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

// TestLoadMinioConfigEtcdFirst verifies that when etcd is available and returns
// a valid config, loadMinioConfig returns the etcd value and a source string
// prefixed with "etcd:" — regardless of any local contract file content.
func TestLoadMinioConfigEtcdFirst(t *testing.T) {
	tmpDir := t.TempDir()
	contractPath := filepath.Join(tmpDir, "minio.json")

	// Write a local file with a clearly different endpoint.
	writeContractFile(t, contractPath, sampleContract("10.0.0.1:9000"))

	// Inject a mock etcd loader that returns a different (authoritative) endpoint.
	const etcdEndpoint = "10.0.0.63:9000"
	orig := buildMinioProxyConfigFn
	buildMinioProxyConfigFn = func() (*config.MinioProxyConfig, error) {
		return sampleContract(etcdEndpoint), nil
	}
	t.Cleanup(func() { buildMinioProxyConfigFn = orig })

	cfg, source, err := loadMinioConfig(contractPath, true)
	if err != nil {
		t.Fatalf("loadMinioConfig: %v", err)
	}
	if !strings.HasPrefix(source, "etcd:") {
		t.Fatalf("expected source to start with 'etcd:', got %q — local file must not take priority over etcd", source)
	}
	if cfg.Endpoint != etcdEndpoint {
		t.Fatalf("expected etcd endpoint %q, got %q — etcd must override local contract", etcdEndpoint, cfg.Endpoint)
	}
}

// TestLoadMinioConfigFallsBackToFileWhenEtcdUnavailable verifies that when
// etcd is unavailable, loadMinioConfig reads the local rendered contract and
// marks the source as "contract:..." (stale cache, not authoritative).
func TestLoadMinioConfigFallsBackToFileWhenEtcdUnavailable(t *testing.T) {
	tmpDir := t.TempDir()
	contractPath := filepath.Join(tmpDir, "minio.json")
	writeContractFile(t, contractPath, sampleContract("10.0.0.8:9000"))

	// Inject a mock etcd loader that always fails (simulates etcd unavailable).
	orig := buildMinioProxyConfigFn
	buildMinioProxyConfigFn = func() (*config.MinioProxyConfig, error) {
		return nil, fmt.Errorf("etcd connection refused")
	}
	t.Cleanup(func() { buildMinioProxyConfigFn = orig })

	cfg, source, err := loadMinioConfig(contractPath, false)
	if err != nil {
		t.Fatalf("loadMinioConfig: %v", err)
	}
	if !strings.HasPrefix(source, "contract:") {
		t.Fatalf("expected source to start with 'contract:', got %q — file fallback must be clearly labelled", source)
	}
	if cfg.Endpoint != "10.0.0.8:9000" {
		t.Fatalf("expected file endpoint, got %q", cfg.Endpoint)
	}
}

// TestNodeAgentDoesNotTreatLocalMinioJsonAsAuthority verifies that when etcd
// is available, the local minio.json contract is not used — even if the local
// file has a different (stale) endpoint. This is the core etcd-first invariant:
// local files are rendered artifacts of etcd state, never authoritative sources.
func TestNodeAgentDoesNotTreatLocalMinioJsonAsAuthority(t *testing.T) {
	tmpDir := t.TempDir()
	contractPath := filepath.Join(tmpDir, "minio.json")

	// Local file has stale DNS endpoint (the bug we fixed).
	writeContractFile(t, contractPath, sampleContract("minio.globular.internal:9000"))

	// etcd has the correct IP endpoint.
	const authoritative = "10.0.0.100:9000"
	orig := buildMinioProxyConfigFn
	buildMinioProxyConfigFn = func() (*config.MinioProxyConfig, error) {
		return sampleContract(authoritative), nil
	}
	t.Cleanup(func() { buildMinioProxyConfigFn = orig })

	cfg, source, err := loadMinioConfig(contractPath, false)
	if err != nil {
		t.Fatalf("loadMinioConfig: %v", err)
	}

	// The local file's DNS endpoint must never surface — etcd wins.
	if strings.Contains(cfg.Endpoint, "minio.globular.internal") {
		t.Fatalf("local file DNS endpoint leaked into result %q — etcd must always override local contract", cfg.Endpoint)
	}
	if cfg.Endpoint != authoritative {
		t.Fatalf("expected authoritative etcd endpoint %q, got %q", authoritative, cfg.Endpoint)
	}
	if !strings.HasPrefix(source, "etcd:") {
		t.Fatalf("source must be etcd:..., got %q", source)
	}
}
