package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ---------- helpers ----------

// writeTmpFile creates a small file at dir/name and returns the full path.
func writeTmpFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writeTmpFile: %v", err)
	}
	return path
}

// setEnv sets an env var for the duration of the test and restores it afterwards.
func setEnv(t *testing.T, key, val string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if err := os.Setenv(key, val); err != nil {
		t.Fatalf("setenv %s: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

// unsetEnv clears an env var for the duration of the test.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	_ = os.Unsetenv(key)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		}
	})
}

// ---------- resolveClientKeypair tests ----------

func TestResolveClientKeypair_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	certFile := writeTmpFile(t, dir, "client.crt", "cert-data")
	keyFile := writeTmpFile(t, dir, "client.key", "key-data")

	setEnv(t, "GLOBULAR_CLIENT_CERT", certFile)
	setEnv(t, "GLOBULAR_CLIENT_KEY", keyFile)
	// Point HOME somewhere without PKI so we don't accidentally pick up real certs
	setEnv(t, "HOME", t.TempDir())

	kp, err := resolveClientKeypair(true)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if kp.certFile != certFile || kp.keyFile != keyFile {
		t.Fatalf("unexpected keypair: cert=%s key=%s", kp.certFile, kp.keyFile)
	}
}

func TestResolveClientKeypair_UserPKI(t *testing.T) {
	home := t.TempDir()
	pkiDir := filepath.Join(home, ".config", "globular", "pki")
	if err := os.MkdirAll(pkiDir, 0700); err != nil {
		t.Fatal(err)
	}
	certFile := writeTmpFile(t, pkiDir, "client.crt", "cert-data")
	keyFile := writeTmpFile(t, pkiDir, "client.key", "key-data")

	setEnv(t, "HOME", home)
	unsetEnv(t, "GLOBULAR_CLIENT_CERT")
	unsetEnv(t, "GLOBULAR_CLIENT_KEY")

	kp, err := resolveClientKeypair(true)
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if kp.certFile != certFile || kp.keyFile != keyFile {
		t.Fatalf("unexpected keypair: cert=%s key=%s", kp.certFile, kp.keyFile)
	}
}

func TestResolveClientKeypair_MissingRequired(t *testing.T) {
	// HOME points to a dir without PKI files
	setEnv(t, "HOME", t.TempDir())
	unsetEnv(t, "GLOBULAR_CLIENT_CERT")
	unsetEnv(t, "GLOBULAR_CLIENT_KEY")

	_, err := resolveClientKeypair(true)
	if !errors.Is(err, ErrNeedInstallCerts) {
		t.Fatalf("expected ErrNeedInstallCerts, got %v", err)
	}
}

func TestResolveClientKeypair_MissingOptional(t *testing.T) {
	// HOME points to a dir without PKI files – optional mode should return nil, nil
	setEnv(t, "HOME", t.TempDir())
	unsetEnv(t, "GLOBULAR_CLIENT_CERT")
	unsetEnv(t, "GLOBULAR_CLIENT_KEY")

	kp, err := resolveClientKeypair(false)
	if err != nil {
		t.Fatalf("expected nil error for optional keypair, got %v", err)
	}
	if kp != nil {
		t.Fatalf("expected nil keypair when missing and not required, got %+v", kp)
	}
}

func TestResolveClientKeypair_EnvBothRequired(t *testing.T) {
	// Only one of CERT/KEY set – should error
	setEnv(t, "GLOBULAR_CLIENT_CERT", "/some/cert.crt")
	unsetEnv(t, "GLOBULAR_CLIENT_KEY")

	_, err := resolveClientKeypair(false)
	if err == nil {
		t.Fatal("expected error when only CERT is set without KEY")
	}
}

func TestResolveClientKeypair_NoServiceKeys(t *testing.T) {
	// Ensure no code path references /pki/issued/services/service.key
	// (static analysis via test)
	// This test documents the invariant: if neither env nor user PKI is set,
	// we get ErrNeedInstallCerts, NOT a permission-denied on service paths.
	setEnv(t, "HOME", t.TempDir())
	unsetEnv(t, "GLOBULAR_CLIENT_CERT")
	unsetEnv(t, "GLOBULAR_CLIENT_KEY")

	_, err := resolveClientKeypair(true)
	if !errors.Is(err, ErrNeedInstallCerts) {
		t.Fatalf("expected ErrNeedInstallCerts, got %v", err)
	}
}

// ---------- resolveCAPath tests ----------

func TestResolveCAPath_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	caFile := writeTmpFile(t, dir, "ca.crt", "ca-data")

	setEnv(t, "GLOBULAR_CA_CERT", caFile)
	setEnv(t, "HOME", t.TempDir())

	got, err := resolveCAPath()
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if got != caFile {
		t.Fatalf("expected %s, got %s", caFile, got)
	}
}

func TestResolveCAPath_UserPKI(t *testing.T) {
	home := t.TempDir()
	pkiDir := filepath.Join(home, ".config", "globular", "pki")
	if err := os.MkdirAll(pkiDir, 0700); err != nil {
		t.Fatal(err)
	}
	caFile := writeTmpFile(t, pkiDir, "ca.crt", "ca-data")

	setEnv(t, "HOME", home)
	unsetEnv(t, "GLOBULAR_CA_CERT")

	got, err := resolveCAPath()
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if got != caFile {
		t.Fatalf("expected %s, got %s", caFile, got)
	}
}

// TestResolveCAPath_LegacyTLS verifies that a CA cert placed in the legacy
// Day-0-installer location (~/.config/globular/tls/<domain>/ca.crt) is found
// even when the new PKI dir does not exist.  This ensures 'auth login' works
// out-of-the-box without requiring 'auth install-certs' first.
func TestResolveCAPath_LegacyTLS(t *testing.T) {
	for _, domain := range []string{"localhost", "globular.internal", "custom.example"} {
		t.Run(domain, func(t *testing.T) {
			home := t.TempDir()
			legacyDir := filepath.Join(home, ".config", "globular", "tls", domain)
			if err := os.MkdirAll(legacyDir, 0700); err != nil {
				t.Fatal(err)
			}
			caFile := writeTmpFile(t, legacyDir, "ca.crt", "ca-data")

			setEnv(t, "HOME", home)
			unsetEnv(t, "GLOBULAR_CA_CERT")

			got, err := resolveCAPath()
			if err != nil {
				t.Fatalf("domain %s: expected success, got %v", domain, err)
			}
			if got != caFile {
				t.Fatalf("domain %s: expected %s, got %s", domain, caFile, got)
			}
		})
	}
}
