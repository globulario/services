package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMinIOTLSConfig_Loopback(t *testing.T) {
	for _, ep := range []string{
		"127.0.0.1:9000",
		"[::1]:9000",
		"localhost:9000",
		"127.0.0.1",
		"localhost",
		"https://127.0.0.1:9000",
		"https://localhost:9000",
	} {
		cfg, err := MinIOTLSConfig(ep)
		if err != nil {
			t.Errorf("MinIOTLSConfig(%q) returned error: %v", ep, err)
			continue
		}
		if !cfg.InsecureSkipVerify {
			t.Errorf("MinIOTLSConfig(%q): expected InsecureSkipVerify=true for loopback", ep)
		}
	}
}

func TestMinIOTLSConfig_NonLoopback_WithCA(t *testing.T) {
	// Create a temporary self-signed CA certificate.
	tmpDir := t.TempDir()
	caPath := filepath.Join(tmpDir, "ca.crt")
	writeTempCA(t, caPath)

	// Override GetLocalCACertificate by setting the well-known path via env.
	// Instead, we rely on the fallback: set the well-known path to our temp file.
	// We need to make /var/lib/globular/pki/ca.crt point to our file, but that
	// requires root. Instead, ensure GetLocalCACertificate returns our temp path.
	//
	// The simplest approach: since GetLocalCACertificate reads from GetCACertificatePath
	// which uses GetStateRootDir, we can set GLOBULAR_STATE_ROOT.
	pkiDir := filepath.Join(tmpDir, "pki")
	if err := os.MkdirAll(pkiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	caPathCanonical := filepath.Join(pkiDir, "ca.crt")
	data, err := os.ReadFile(caPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caPathCanonical, data, 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GLOBULAR_STATE_DIR", tmpDir)

	cfg, err := MinIOTLSConfig("192.0.2.1:9000")
	if err != nil {
		t.Fatalf("MinIOTLSConfig returned error: %v", err)
	}
	if cfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false for non-loopback with CA")
	}
	if cfg.RootCAs == nil {
		t.Error("RootCAs should be set when CA is available")
	}
	if cfg.MinVersion == 0 {
		t.Error("MinVersion should be set")
	}
}

func TestMinIOTLSConfig_NonLoopback_NoCA(t *testing.T) {
	// Point to a non-existent state root so no CA is found.
	tmpDir := t.TempDir()
	t.Setenv("GLOBULAR_STATE_DIR", tmpDir)

	_, err := MinIOTLSConfig("192.0.2.1:9000")
	if err == nil {
		t.Fatal("expected error for non-loopback endpoint without CA, got nil")
	}
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"127.0.0.1:9000", "127.0.0.1"},
		{"localhost:9000", "localhost"},
		{"192.0.2.1:9000", "192.0.2.1"},
		{"https://192.0.2.1:9000", "192.0.2.1"},
		{"http://localhost:9000", "localhost"},
		{"minio.globular.internal:9000", "minio.globular.internal"},
		{"localhost", "localhost"},
		{"10.0.0.63", "10.0.0.63"},
		{"[::1]:9000", "::1"},
	}
	for _, tt := range tests {
		got := extractHost(tt.input)
		if got != tt.want {
			t.Errorf("extractHost(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsLoopback(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"127.0.0.1", true},
		{"::1", true},
		{"localhost", true},
		{"127.0.0.2", true}, // net.IP.IsLoopback covers full 127.0.0.0/8
		{"10.0.0.1", false},
		{"192.168.1.1", false},
		{"minio.globular.internal", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isLoopback(tt.host)
		if got != tt.want {
			t.Errorf("isLoopback(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

// writeTempCA generates a self-signed CA certificate and writes it to path.
func writeTempCA(t *testing.T, path string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatal(err)
	}
}
