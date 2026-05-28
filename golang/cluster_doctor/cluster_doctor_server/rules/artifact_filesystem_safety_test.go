package rules

import (
	"os"
	"path/filepath"
	"testing"
)

func TestArtifactFilesystemSafetyLocal_WarnsOnUnsafeModes(t *testing.T) {
	td := t.TempDir()

	binDir := filepath.Join(td, "bin")
	etcdDir := filepath.Join(td, "etcd")
	caKey := filepath.Join(td, "ca.key")
	policy := filepath.Join(td, "install-policy.json")

	if err := os.MkdirAll(binDir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(etcdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caKey, []byte("k"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("{}"), 0o666); err != nil {
		t.Fatal(err)
	}
	// Runner umask may narrow modes from WriteFile/MkdirAll. Force the
	// intended insecure modes so findings are deterministic across CI hosts.
	if err := os.Chmod(binDir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(etcdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(caKey, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(policy, 0o666); err != nil {
		t.Fatal(err)
	}

	oldBin, oldEtcd, oldCA, oldPolicy, oldRootOwner := artifactBinDirPath, artifactEtcdDataDirPath, artifactCAKeyPath, artifactInstallPolicyPath, artifactRequireRootCAOwner
	artifactBinDirPath, artifactEtcdDataDirPath, artifactCAKeyPath, artifactInstallPolicyPath = binDir, etcdDir, caKey, policy
	artifactRequireRootCAOwner = false
	t.Cleanup(func() {
		artifactBinDirPath, artifactEtcdDataDirPath, artifactCAKeyPath, artifactInstallPolicyPath = oldBin, oldEtcd, oldCA, oldPolicy
		artifactRequireRootCAOwner = oldRootOwner
	})

	findings := (artifactFilesystemSafetyLocal{}).Evaluate(nil, testConfig())
	if len(findings) < 3 {
		t.Fatalf("expected >=3 findings, got %d: %+v", len(findings), findings)
	}
}

func TestArtifactFilesystemSafetyLocal_CleanModes_NoFindings(t *testing.T) {
	td := t.TempDir()

	binDir := filepath.Join(td, "bin")
	etcdDir := filepath.Join(td, "etcd")
	caKey := filepath.Join(td, "ca.key")
	policy := filepath.Join(td, "install-policy.json")

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(etcdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caKey, []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policy, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldBin, oldEtcd, oldCA, oldPolicy, oldRootOwner := artifactBinDirPath, artifactEtcdDataDirPath, artifactCAKeyPath, artifactInstallPolicyPath, artifactRequireRootCAOwner
	artifactBinDirPath, artifactEtcdDataDirPath, artifactCAKeyPath, artifactInstallPolicyPath = binDir, etcdDir, caKey, policy
	artifactRequireRootCAOwner = false
	t.Cleanup(func() {
		artifactBinDirPath, artifactEtcdDataDirPath, artifactCAKeyPath, artifactInstallPolicyPath = oldBin, oldEtcd, oldCA, oldPolicy
		artifactRequireRootCAOwner = oldRootOwner
	})

	findings := (artifactFilesystemSafetyLocal{}).Evaluate(nil, testConfig())
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}
