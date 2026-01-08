package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/pki"
)

func TestMergeNetworkIntoConfigPreservesUnknown(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	base := map[string]interface{}{
		"Foo":    "bar",
		"Nested": map[string]interface{}{"keep": true},
	}
	data, err := json.Marshal(base)
	if err != nil {
		t.Fatalf("marshal base: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o644); err != nil {
		t.Fatalf("write base config: %v", err)
	}

	overlay := `{"Domain":"example.com","Protocol":"https","PortHTTP":8080,"ACMEEnabled":true}`
	if err := mergeNetworkIntoConfig(configPath, overlay); err != nil {
		t.Fatalf("merge network overlay: %v", err)
	}
	finalData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read merged config: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(finalData, &result); err != nil {
		t.Fatalf("unmarshal merged config: %v", err)
	}
	if result["Foo"] != "bar" {
		t.Fatalf("expected Foo preserved, got %v", result["Foo"])
	}
	if nested, ok := result["Nested"].(map[string]interface{}); !ok || nested["keep"] != true {
		t.Fatalf("expected Nested kept, got %v", result["Nested"])
	}
	if result["Protocol"] != "https" {
		t.Fatalf("expected protocol updated: %v", result["Protocol"])
	}
}

func TestPerformRestartUnitsFailsWhenCommandFails(t *testing.T) {
	origRestart := restartCommand
	origLookPath := systemctlLookPath
	defer func() {
		restartCommand = origRestart
		systemctlLookPath = origLookPath
	}()
	restartCommand = func(systemctl, unit string) error {
		if unit == "globular-etcd.service" {
			return fmt.Errorf("restart failed")
		}
		return nil
	}
	systemctlLookPath = func(name string) (string, error) {
		return "/bin/systemctl", nil
	}
	srv := &NodeAgentServer{}
	err := srv.performRestartUnits([]string{"globular-etcd.service"}, nil)
	if err == nil || !strings.Contains(err.Error(), "globular-etcd.service") {
		t.Fatalf("expected failure referencing unit, got %v", err)
	}
}

func TestEnsureNetworkCertsUsesACME(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("GLOBULAR_STATE_DIR", tmpDir)
	fake := &fakePKIManager{}
	orig := networkPKIManager
	networkPKIManager = func(opts pki.Options) pki.Manager {
		return fake
	}
	defer func() {
		networkPKIManager = orig
	}()
	srv := &NodeAgentServer{}
	spec := &clustercontrollerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
		Protocol:      "https",
		AcmeEnabled:   true,
		AdminEmail:    "ops@example.com",
	}
	if err := srv.ensureNetworkCerts(spec); err != nil {
		t.Fatalf("ensureNetworkCerts: %v", err)
	}
	if !fake.acmeCalled {
		t.Fatalf("expected ACME path invoked")
	}
}

type fakePKIManager struct {
	acmeCalled bool
}

func (f *fakePKIManager) EnsurePeerCert(dir string, subject string, dns []string, ips []string, ttl time.Duration) (string, string, string, error) {
	return "", "", "", nil
}

func (f *fakePKIManager) EnsureServerCert(dir string, subject string, dns []string, ttl time.Duration) (string, string, string, error) {
	return "", "", "", nil
}

func (f *fakePKIManager) EnsurePublicACMECert(dir, base, subject string, dns []string, ttl time.Duration) (string, string, string, string, error) {
	f.acmeCalled = true
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", "", "", err
	}
	key := filepath.Join(dir, "server.key")
	leaf := filepath.Join(dir, "leaf.crt")
	issuer := filepath.Join(dir, "issuer.crt")
	fullchain := filepath.Join(dir, "fullchain.crt")
	os.WriteFile(key, []byte("key"), 0o600)
	os.WriteFile(leaf, []byte("leaf"), 0o644)
	os.WriteFile(issuer, []byte("issuer"), 0o644)
	os.WriteFile(fullchain, []byte("fullchain"), 0o644)
	return key, leaf, issuer, fullchain, nil
}

func (f *fakePKIManager) EnsureClientCert(dir string, subject string, dns []string, ttl time.Duration) (string, string, string, error) {
	return "", "", "", nil
}

func (f *fakePKIManager) ValidateCertPair(certFile, keyFile string, requireEKUs []int, requireDNS []string, requireIPs []string) error {
	return nil
}

func (f *fakePKIManager) RotateIfExpiring(dir string, leafFile string, renewBefore time.Duration) (bool, error) {
	return false, nil
}

func (f *fakePKIManager) EnsureServerKeyAndCSR(dir, commonName, country, state, city, org string, dns []string) error {
	return nil
}

func TestIsAllowedRenderTarget(t *testing.T) {
	allowed := []string{
		"/etc/globular/config.json",
		"/var/lib/globular/state.yaml",
		"/etc/systemd/system/globular.service",
	}
	for _, path := range allowed {
		if !isAllowedRenderTarget(path) {
			t.Fatalf("expected %s allowed", path)
		}
	}
	rejected := []string{
		"relative/path",
		"../etc/passwd",
		"/tmp/globular/config",
		"/etc/globular/../passwd",
	}
	for _, path := range rejected {
		if isAllowedRenderTarget(path) {
			t.Fatalf("expected %s rejected", path)
		}
	}
}

func TestCopyFilePermSetsMode(t *testing.T) {
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "src")
	dst := filepath.Join(tmpDir, "dst")
	data := []byte("payload")
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}
	if err := copyFilePerm(src, dst, 0o600); err != nil {
		t.Fatalf("copy file: %v", err)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600, got %o", info.Mode().Perm())
	}
}
