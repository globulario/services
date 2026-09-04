package globular_service

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writePair(t *testing.T, certPath, keyPath, cn string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
	}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
}

func leafCN(t *testing.T, c *x509.Certificate) string {
	t.Helper()
	return c.Subject.CommonName
}

// TestCertReloaderServesReplacedMaterial pins that a server picks up a
// certificate that was replaced on disk.
//
// GetTLSConfig used to snapshot the keypair into tls.Config.Certificates at
// startup, so a rotation or a repair-path re-issue changed the files and
// nothing else: the process kept presenting the old certificate until it
// restarted. Observed 2026-08-23 — node-3's on-disk cert was CN=node-3 with
// IP:10.10.0.13 while :11000 served a CN=globular.internal cert with no IP
// SANs, every controller probe to that node failed with "doesn't contain any
// IP SANs", the reconcile workflow logged remediation_no_progress 44 times in
// an hour, and cluster-doctor reported zero errors the entire time. Restarting
// the process fixed it immediately, which is the proof that only the process
// was stale.
func TestCertReloaderServesReplacedMaterial(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "service.crt")
	keyPath := filepath.Join(dir, "service.key")

	writePair(t, certPath, keyPath, "before-rotation")
	r := newCertReloader(certPath, keyPath)

	first, err := r.getCertificate(nil)
	if err != nil {
		t.Fatalf("initial getCertificate: %v", err)
	}
	leaf, err := x509.ParseCertificate(first.Certificate[0])
	if err != nil {
		t.Fatalf("parse initial leaf: %v", err)
	}
	if got := leafCN(t, leaf); got != "before-rotation" {
		t.Fatalf("expected the material on disk, got CN=%q", got)
	}

	// Replace the material, as rotation or a repair re-issue does. mtime must
	// differ for the reloader to notice.
	time.Sleep(10 * time.Millisecond)
	writePair(t, certPath, keyPath, "after-rotation")
	future := time.Now().Add(time.Second)
	_ = os.Chtimes(certPath, future, future)
	_ = os.Chtimes(keyPath, future, future)

	second, err := r.getCertificate(nil)
	if err != nil {
		t.Fatalf("getCertificate after replacement: %v", err)
	}
	leaf2, err := x509.ParseCertificate(second.Certificate[0])
	if err != nil {
		t.Fatalf("parse reloaded leaf: %v", err)
	}
	if got := leafCN(t, leaf2); got != "after-rotation" {
		t.Errorf("server must serve the REPLACED certificate; still serving CN=%q "+
			"— this is the stale-process defect the reloader exists to prevent", got)
	}
}

// A half-written rotation must not take the listener down: keep serving the
// last good certificate rather than failing the handshake.
func TestCertReloaderKeepsLastGoodOnBadReload(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "service.crt")
	keyPath := filepath.Join(dir, "service.key")

	writePair(t, certPath, keyPath, "good")
	r := newCertReloader(certPath, keyPath)
	if _, err := r.getCertificate(nil); err != nil {
		t.Fatalf("initial load: %v", err)
	}

	// Truncated cert, as a partially-written file looks.
	if err := os.WriteFile(certPath, []byte("-----BEGIN CERTIFICATE-----\ntrunc"), 0o600); err != nil {
		t.Fatalf("write bad cert: %v", err)
	}
	future := time.Now().Add(time.Second)
	_ = os.Chtimes(certPath, future, future)

	got, err := r.getCertificate(nil)
	if err != nil {
		t.Fatalf("a bad reload must not fail the handshake: %v", err)
	}
	leaf, err := x509.ParseCertificate(got.Certificate[0])
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cn := leafCN(t, leaf); cn != "good" {
		t.Errorf("must keep serving the last good certificate, got CN=%q", cn)
	}
}
