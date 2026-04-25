package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/config"
)

// validContract is the reference config the reconciler should produce.
func validContract() *config.MinioProxyConfig {
	return &config.MinioProxyConfig{
		Endpoint:     "minio.globular.internal:9000",
		Bucket:       "globular",
		Prefix:       "globular.internal",
		Secure:       true,
		CABundlePath: "/var/lib/globular/pki/ca.pem",
		Auth: &config.MinioProxyAuth{
			Mode:      config.MinioProxyAuthModeAccessKey,
			AccessKey: "ak",
			SecretKey: "sk",
		},
	}
}

// writeFile is a test helper that writes raw bytes to path.
func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestLoadMinioContractFromDisk_NotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.json")
	_, err := loadMinioContractFromDisk(path)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected os.ErrNotExist, got %v", err)
	}
}

func TestLoadMinioContractFromDisk_Corrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "minio.json")
	// Exact shape observed in production on nuc — two lines of plain text.
	writeFile(t, path, []byte("globular-b2e6ddc12ee0d30514a0\nbbce4b74bac328a88644fdde02930935\n"))
	_, err := loadMinioContractFromDisk(path)
	if err == nil {
		t.Fatal("expected a parse error for corrupt contract")
	}
	if errors.Is(err, os.ErrNotExist) {
		t.Fatalf("corrupt file misclassified as missing: %v", err)
	}
}

func TestWriteMinioContractAtomic_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "minio.json")
	cfg := validContract()
	if err := writeMinioContractAtomic(path, cfg, nil); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := loadMinioContractFromDisk(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !minioContractsEqual(cfg, loaded) {
		t.Fatalf("round-trip mismatch: wrote %+v, got %+v", cfg, loaded)
	}
	// Double-check mode.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("unexpected perm %o", info.Mode().Perm())
	}
}

func TestWriteMinioContractAtomic_OverwritesCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "minio.json")
	// Pre-seed with the exact plaintext corruption from nuc.
	writeFile(t, path, []byte("globular-ak\nglobular-sk\n"))

	cfg := validContract()
	if err := writeMinioContractAtomic(path, cfg, nil); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Reload must now succeed.
	loaded, err := loadMinioContractFromDisk(path)
	if err != nil {
		t.Fatalf("reload after repair: %v", err)
	}
	if !minioContractsEqual(cfg, loaded) {
		t.Fatalf("repair did not produce expected contract")
	}
}

func TestMinioContractsEqual(t *testing.T) {
	a := validContract()
	b := validContract()
	if !minioContractsEqual(a, b) {
		t.Fatal("identical configs should compare equal")
	}

	c := validContract()
	c.Endpoint = "different.example:9000"
	if minioContractsEqual(a, c) {
		t.Fatal("different endpoint should compare unequal")
	}

	if minioContractsEqual(nil, nil) != true {
		t.Fatal("nil,nil should compare equal")
	}
	if minioContractsEqual(a, nil) {
		t.Fatal("non-nil vs nil should compare unequal")
	}
}
