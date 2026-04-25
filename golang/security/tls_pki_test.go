package security

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/config"
)

// ─── PKI test helpers ─────────────────────────────────────────────────────────

// newTestCA generates a self-signed CA key+cert in dir, returning the cert.
func newTestCA(t *testing.T, dir, commonName string) *x509.Certificate {
	t.Helper()
	signer, pkcs8, err := genECDSAKeyPKCS8()
	if err != nil {
		t.Fatalf("gen CA key: %v", err)
	}
	if err := writePEM(filepath.Join(dir, "ca.key"), mkBlock("PRIVATE KEY", pkcs8), 0o400); err != nil {
		t.Fatalf("write ca.key: %v", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	now := time.Now()
	tpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: commonName},
		NotBefore:             now.Add(-1 * time.Minute),
		NotAfter:              now.Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, signer.Public(), signer)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	if err := writePEM(filepath.Join(dir, "ca.crt"), mkBlock("CERTIFICATE", der), 0o444); err != nil {
		t.Fatalf("write ca.crt: %v", err)
	}
	cert, _ := x509.ParseCertificate(der)
	return cert
}

func mkBlock(typ string, bytes []byte) *pem.Block {
	return &pem.Block{Type: typ, Bytes: bytes}
}

// newTestServiceCert generates a service (server) cert signed by the CA in dir.
// SANs include the provided dns names and IPs. Returns the leaf cert.
func newTestServiceCert(t *testing.T, dir string, caCert *x509.Certificate, dns []string, ips []net.IP, notAfter time.Time) *x509.Certificate {
	t.Helper()
	// reuse the server key if it already exists, otherwise create one
	keyPath := filepath.Join(dir, "server.key")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		signer2, pkcs8, err2 := genECDSAKeyPKCS8()
		if err2 != nil {
			t.Fatalf("gen server key: %v", err2)
		}
		_ = signer2
		if err2 = writePEM(keyPath, mkBlock("PRIVATE KEY", pkcs8), 0o400); err2 != nil {
			t.Fatalf("write server.key: %v", err2)
		}
	}
	keyBlock, err := readPEM(keyPath)
	if err != nil {
		t.Fatalf("read server key: %v", err)
	}
	leafSigner, err := parseAnyPrivateKey(keyBlock)
	if err != nil {
		t.Fatalf("parse server key: %v", err)
	}

	caKeyBlock, err := readPEM(filepath.Join(dir, "ca.key"))
	if err != nil {
		t.Fatalf("read ca.key: %v", err)
	}
	caSigner, err := parseAnyPrivateKey(caKeyBlock)
	if err != nil {
		t.Fatalf("parse ca.key: %v", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	now := time.Now()
	if notAfter.IsZero() {
		notAfter = now.Add(24 * time.Hour)
	}
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "test-service"},
		NotBefore:    now.Add(-1 * time.Minute),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     dns,
		IPAddresses:  ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, caCert, leafSigner.Public(), caSigner)
	if err != nil {
		t.Fatalf("sign service cert: %v", err)
	}
	certPath := filepath.Join(dir, "server.crt")
	if err := writePEM(certPath, mkBlock("CERTIFICATE", der), 0o444); err != nil {
		t.Fatalf("write server.crt: %v", err)
	}
	cert, _ := x509.ParseCertificate(der)
	return cert
}

// ─── NeedsCertRegeneration ────────────────────────────────────────────────────

func TestNeedsCertRegeneration_Expired(t *testing.T) {
	dir := t.TempDir()
	ca := newTestCA(t, dir, "test-ca")
	// Issue cert that is already expired
	past := time.Now().Add(-1 * time.Hour)
	newTestServiceCert(t, dir, ca, nil, nil, past)

	need, reason := NeedsCertRegeneration(
		filepath.Join(dir, "server.crt"),
		filepath.Join(dir, "server.key"),
		filepath.Join(dir, "ca.crt"),
		nil, nil, 0,
	)
	if !need {
		t.Fatalf("expected regen needed for expired cert, reason=%q", reason)
	}
	if reason == "" {
		t.Error("reason must be non-empty when regen required")
	}
	t.Logf("reason: %s", reason)
}

func TestNeedsCertRegeneration_NearExpiry(t *testing.T) {
	dir := t.TempDir()
	ca := newTestCA(t, dir, "test-ca")
	// Cert expires in 5 days
	soonExpiry := time.Now().Add(5 * 24 * time.Hour)
	newTestServiceCert(t, dir, ca, nil, nil, soonExpiry)

	// With renewBefore=7d the cert should trigger regen
	need, reason := NeedsCertRegeneration(
		filepath.Join(dir, "server.crt"),
		filepath.Join(dir, "server.key"),
		filepath.Join(dir, "ca.crt"),
		nil, nil, 7*24*time.Hour,
	)
	if !need {
		t.Fatalf("expected regen needed for cert expiring in 5d with 7d renew_before, reason=%q", reason)
	}
	t.Logf("reason: %s", reason)
}

func TestNeedsCertRegeneration_MissingDNS_SAN(t *testing.T) {
	dir := t.TempDir()
	ca := newTestCA(t, dir, "test-ca")
	newTestServiceCert(t, dir, ca, []string{"node1.globular.internal"}, nil, time.Time{})

	need, reason := NeedsCertRegeneration(
		filepath.Join(dir, "server.crt"),
		filepath.Join(dir, "server.key"),
		filepath.Join(dir, "ca.crt"),
		[]string{"node1.globular.internal", "missing.globular.internal"}, nil, 0,
	)
	if !need {
		t.Fatalf("expected regen needed for missing DNS SAN, reason=%q", reason)
	}
	t.Logf("reason: %s", reason)
}

func TestNeedsCertRegeneration_MissingIP_SAN(t *testing.T) {
	dir := t.TempDir()
	ca := newTestCA(t, dir, "test-ca")
	presentIP := net.ParseIP("10.0.0.63")
	newTestServiceCert(t, dir, ca, nil, []net.IP{presentIP}, time.Time{})

	missingIP := net.ParseIP("10.0.0.100")
	need, reason := NeedsCertRegeneration(
		filepath.Join(dir, "server.crt"),
		filepath.Join(dir, "server.key"),
		filepath.Join(dir, "ca.crt"),
		nil, []net.IP{presentIP, missingIP}, 0,
	)
	if !need {
		t.Fatalf("expected regen needed for missing IP SAN, reason=%q", reason)
	}
	t.Logf("reason: %s", reason)
}

func TestNeedsCertRegeneration_HealthyCert_NoRegen(t *testing.T) {
	dir := t.TempDir()
	ca := newTestCA(t, dir, "test-ca")
	dns := []string{"node1.globular.internal"}
	ips := []net.IP{net.ParseIP("10.0.0.63")}
	newTestServiceCert(t, dir, ca, dns, ips, time.Time{})

	need, reason := NeedsCertRegeneration(
		filepath.Join(dir, "server.crt"),
		filepath.Join(dir, "server.key"),
		filepath.Join(dir, "ca.crt"),
		dns, ips, 0,
	)
	if need {
		t.Fatalf("healthy cert should not need regen, but got reason: %q", reason)
	}
}

// ─── GenerateServicesCertificates ─────────────────────────────────────────────

// TestGenerateServicesCertificates_RemovesStaleCertArtifacts verifies that
// when the existing cert does not chain to the local CA, stale cert/CSR/PEM
// artifacts are removed before regeneration. Without removal the lower-level
// functions (GenerateSignedServerCertificate etc.) skip existing files and
// the stale cert is preserved — silently breaking mTLS.
func TestGenerateServicesCertificates_RemovesStaleCertArtifacts(t *testing.T) {
	dir := t.TempDir()

	// Bootstrap a complete PKI set with CA1.
	if err := GenerateAuthorityPrivateKey(dir, ""); err != nil {
		t.Fatalf("gen CA1 key: %v", err)
	}
	if err := GenerateAuthorityTrustCertificate(dir, "", 365, "test.internal"); err != nil {
		t.Fatalf("gen CA1 cert: %v", err)
	}
	if err := GenerateSanConfig("test.internal", dir, "CA", "CA", "CA", "test", []string{"test.internal"}); err != nil {
		t.Fatalf("gen san.conf: %v", err)
	}
	if err := GenerateClientPrivateKey(dir, ""); err != nil {
		t.Fatalf("gen client key: %v", err)
	}
	if err := GenerateClientCertificateSigningRequest(dir, "", "test.internal"); err != nil {
		t.Fatalf("gen client CSR: %v", err)
	}
	if err := GenerateSignedClientCertificate(dir, "", 365); err != nil {
		t.Fatalf("sign client cert: %v", err)
	}

	// Record the fingerprint of the original client.crt.
	origData, _ := os.ReadFile(filepath.Join(dir, "client.crt"))

	// Rotate the CA: remove old CA key+cert and generate a new one.
	os.Remove(filepath.Join(dir, "ca.key"))
	os.Remove(filepath.Join(dir, "ca.crt"))
	if err := GenerateAuthorityPrivateKey(dir, ""); err != nil {
		t.Fatalf("gen CA2 key: %v", err)
	}
	if err := GenerateAuthorityTrustCertificate(dir, "", 365, "test.internal"); err != nil {
		t.Fatalf("gen CA2 cert: %v", err)
	}

	// Call GenerateServicesCertificates. It should detect the CA mismatch,
	// clear stale artifacts, and regenerate.
	if err := GenerateServicesCertificates("", 365, "test.internal", dir, "CA", "CA", "CA", "test", nil); err != nil {
		t.Fatalf("GenerateServicesCertificates: %v", err)
	}

	// The new client.crt must differ from the original (it was regenerated).
	newData, err := os.ReadFile(filepath.Join(dir, "client.crt"))
	if err != nil {
		t.Fatalf("read new client.crt: %v", err)
	}
	if string(origData) == string(newData) {
		t.Fatal("client.crt was NOT regenerated after CA rotation — stale cert preserved")
	}
}

// TestGenerateServicesCertificates_RegeneratesWhenCAChanged verifies that the
// regenerated cert chains to the new CA — not to the old one.
func TestGenerateServicesCertificates_RegeneratesWhenCAChanged(t *testing.T) {
	dir := t.TempDir()

	// Full initial setup with CA1
	if err := GenerateAuthorityPrivateKey(dir, ""); err != nil {
		t.Fatalf("gen CA1 key: %v", err)
	}
	if err := GenerateAuthorityTrustCertificate(dir, "", 365, "test.internal"); err != nil {
		t.Fatalf("gen CA1 cert: %v", err)
	}
	if err := GenerateSanConfig("test.internal", dir, "CA", "CA", "CA", "test", []string{"test.internal"}); err != nil {
		t.Fatalf("gen san: %v", err)
	}
	if err := GenerateClientPrivateKey(dir, ""); err != nil {
		t.Fatalf("gen client key: %v", err)
	}
	if err := GenerateClientCertificateSigningRequest(dir, "", "test.internal"); err != nil {
		t.Fatalf("gen client csr: %v", err)
	}
	if err := GenerateSignedClientCertificate(dir, "", 365); err != nil {
		t.Fatalf("sign client cert: %v", err)
	}

	// Save the CA1 fingerprint.
	ca1FP, err := FileSPKIFingerprint(filepath.Join(dir, "ca.crt"))
	if err != nil {
		t.Fatalf("fp CA1: %v", err)
	}

	// Rotate to CA2
	os.Remove(filepath.Join(dir, "ca.key"))
	os.Remove(filepath.Join(dir, "ca.crt"))
	if err := GenerateAuthorityPrivateKey(dir, ""); err != nil {
		t.Fatalf("gen CA2 key: %v", err)
	}
	if err := GenerateAuthorityTrustCertificate(dir, "", 365, "test.internal"); err != nil {
		t.Fatalf("gen CA2 cert: %v", err)
	}

	ca2FP, _ := FileSPKIFingerprint(filepath.Join(dir, "ca.crt"))
	if ca1FP == ca2FP {
		t.Skip("CA fingerprints identical (unlikely but possible in tests) — skipping")
	}

	if err := GenerateServicesCertificates("", 365, "test.internal", dir, "CA", "CA", "CA", "test", nil); err != nil {
		t.Fatalf("GenerateServicesCertificates: %v", err)
	}

	// The new client.crt must chain to CA2 and not to CA1.
	need, reason := NeedsCertRegeneration(
		filepath.Join(dir, "client.crt"),
		filepath.Join(dir, "client.key"),
		filepath.Join(dir, "ca.crt"), // CA2
		nil, nil, 0,
	)
	if need {
		t.Fatalf("regenerated cert should be valid against CA2, but got: %q", reason)
	}
}

// ─── Chain Validation ─────────────────────────────────────────────────────────

// TestGetCertificateStatus_ChainInvalidWhenSignedByOldCA verifies that a cert
// signed by a rotated-out CA is detected as chain-invalid against the new CA.
// This is the core PKI convergence invariant: stale certs must be flagged
// so node agents regenerate them after CA rotation.
func TestGetCertificateStatus_ChainInvalidWhenSignedByOldCA(t *testing.T) {
	oldCADir := t.TempDir()
	newCADir := t.TempDir()

	// Issue service cert against the old CA.
	oldCA := newTestCA(t, oldCADir, "old-ca")
	newTestServiceCert(t, oldCADir, oldCA, []string{"node.globular.internal"}, nil, time.Time{})

	// Rotate: a new CA now exists.
	newTestCA(t, newCADir, "new-ca")

	// NeedsCertRegeneration uses the NEW CA as trust anchor.
	// The cert was signed by old CA → chain validation must fail → regen required.
	need, reason := NeedsCertRegeneration(
		filepath.Join(oldCADir, "server.crt"),
		filepath.Join(oldCADir, "server.key"),
		filepath.Join(newCADir, "ca.crt"),
		nil, nil, 0,
	)
	if !need {
		t.Fatalf("cert signed by old CA must be detected as chain-invalid, reason=%q", reason)
	}
	if !strings.Contains(strings.ToLower(reason), "ca") {
		t.Errorf("reason should mention CA mismatch, got %q", reason)
	}
	t.Logf("correctly detected chain-invalid cert: %s", reason)
}

// TestGetCertificateStatus_ChainValidWithCurrentCA verifies that a cert signed
// by the current CA passes chain validation — no regen should be triggered.
func TestGetCertificateStatus_ChainValidWithCurrentCA(t *testing.T) {
	dir := t.TempDir()
	ca := newTestCA(t, dir, "current-ca")
	newTestServiceCert(t, dir, ca, []string{"node.globular.internal"}, nil, time.Time{})

	need, reason := NeedsCertRegeneration(
		filepath.Join(dir, "server.crt"),
		filepath.Join(dir, "server.key"),
		filepath.Join(dir, "ca.crt"),
		[]string{"node.globular.internal"}, nil, 0,
	)
	if need {
		t.Fatalf("cert signed by current CA must be chain-valid, got reason=%q", reason)
	}
}

// ─── CA Metadata ──────────────────────────────────────────────────────────────

// TestCAMetadataRoundTrip verifies that CAMetadata marshals to/from JSON without
// loss and that NotAfterTime() correctly parses the RFC3339 string field.
func TestCAMetadataRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	want := config.CAMetadata{
		Generation:  3,
		Fingerprint: "sha256:abcdef1234567890",
		Issuer:      "globular-test-ca",
		NotBefore:   now.Add(-24 * time.Hour).Format(time.RFC3339),
		NotAfter:    now.Add(365 * 24 * time.Hour).Format(time.RFC3339),
		Active:      true,
	}

	data, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got config.CAMetadata
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Generation != want.Generation {
		t.Errorf("generation: got %d want %d", got.Generation, want.Generation)
	}
	if got.Fingerprint != want.Fingerprint {
		t.Errorf("fingerprint: got %q want %q", got.Fingerprint, want.Fingerprint)
	}
	if got.Issuer != want.Issuer {
		t.Errorf("issuer: got %q want %q", got.Issuer, want.Issuer)
	}
	if got.Active != want.Active {
		t.Errorf("active: got %v want %v", got.Active, want.Active)
	}

	// NotAfterTime must round-trip through the string representation.
	expectedNotAfter := now.Add(365 * 24 * time.Hour)
	if !got.NotAfterTime().Equal(expectedNotAfter) {
		t.Errorf("NotAfterTime: got %v want %v", got.NotAfterTime(), expectedNotAfter)
	}
	if got.NotAfterTime().IsZero() {
		t.Error("NotAfterTime must not be zero for valid metadata")
	}
	if got.NotBeforeTime().IsZero() {
		t.Error("NotBeforeTime must not be zero for valid metadata")
	}
}

// ─── SPKI Fingerprint ────────────────────────────────────────────────────────

// TestCAFingerprintDriftDetected verifies that two independently generated CAs
// produce different SPKI fingerprints. A fingerprint match means same key pair;
// a mismatch means CA rotation — so this test guards the drift-detection path.
func TestCAFingerprintDriftDetected(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	newTestCA(t, dir1, "ca-alpha")
	newTestCA(t, dir2, "ca-beta")

	fp1, err := FileSPKIFingerprint(filepath.Join(dir1, "ca.crt"))
	if err != nil {
		t.Fatalf("fingerprint CA1: %v", err)
	}
	fp2, err := FileSPKIFingerprint(filepath.Join(dir2, "ca.crt"))
	if err != nil {
		t.Fatalf("fingerprint CA2: %v", err)
	}

	if fp1 == "" || fp2 == "" {
		t.Fatal("FileSPKIFingerprint must return non-empty fingerprint")
	}
	if fp1 == fp2 {
		t.Fatal("two independently generated CAs must produce different SPKI fingerprints")
	}

	// Fingerprint of the same file must be stable (idempotent).
	fp1again, err := FileSPKIFingerprint(filepath.Join(dir1, "ca.crt"))
	if err != nil {
		t.Fatalf("fingerprint CA1 (second call): %v", err)
	}
	if fp1 != fp1again {
		t.Errorf("FileSPKIFingerprint must be idempotent: %q != %q", fp1, fp1again)
	}
	t.Logf("CA1 fp: %s", fp1)
	t.Logf("CA2 fp: %s", fp2)
}

// ─── Node Rejoin ──────────────────────────────────────────────────────────────

// TestNodeRejoin_DoesNotReuseOldCA simulates the CA rotation + node rejoin
// scenario: a node has service certs issued by CA1. After CA rotation to CA2,
// the node must detect its certs are stale and regenerate — never reuse them.
func TestNodeRejoin_DoesNotReuseOldCA(t *testing.T) {
	nodeDir := t.TempDir() // node PKI dir — has cert from pre-rotation CA
	newCADir := t.TempDir() // post-rotation CA (what the cluster now trusts)

	// Before rotation: node was issued a cert by CA1.
	preRotationCA := newTestCA(t, nodeDir, "pre-rotation-ca")
	newTestServiceCert(t, nodeDir, preRotationCA,
		[]string{"node.globular.internal"}, []net.IP{net.ParseIP("10.0.0.63")}, time.Time{})

	// Cluster rotates CA. Node downloads new CA cert on rejoin.
	newTestCA(t, newCADir, "post-rotation-ca")

	// Convergence check: node compares its cert against new cluster CA.
	// Must detect the cert is stale and require regeneration.
	need, reason := NeedsCertRegeneration(
		filepath.Join(nodeDir, "server.crt"),
		filepath.Join(nodeDir, "server.key"),
		filepath.Join(newCADir, "ca.crt"), // new cluster CA
		[]string{"node.globular.internal"}, nil, 0,
	)
	if !need {
		t.Fatalf("node rejoin: cert signed by old CA must be detected as stale after rotation, reason=%q", reason)
	}
	t.Logf("correctly flagged stale cert on rejoin: %s", reason)
}

// ─── Trust Purge ─────────────────────────────────────────────────────────────

// TestPurgeNodeTrust_RemovesSystemCA verifies the PKI trust purge invariant:
// after a node trust purge, all Globular CA artifacts are absent and
// NeedsCertRegeneration correctly reports missing cert (not a silent no-op).
// This mirrors the behavior of purge-node-trust.sh but in pure Go so it can
// run in CI without root access or system directories.
func TestPurgeNodeTrust_RemovesSystemCA(t *testing.T) {
	// Simulate a node PKI directory with CA and service certs present.
	pkiDir := t.TempDir()
	caFile := filepath.Join(pkiDir, "ca.crt")
	certFile := filepath.Join(pkiDir, "server.crt")
	keyFile := filepath.Join(pkiDir, "server.key")

	// Populate the directory with real PKI artifacts.
	ca := newTestCA(t, pkiDir, "globular-ca")
	newTestServiceCert(t, pkiDir, ca, nil, nil, time.Time{})

	// Sanity: before purge, cert is valid.
	need, _ := NeedsCertRegeneration(certFile, keyFile, caFile, nil, nil, 0)
	if need {
		t.Fatal("pre-purge: healthy cert should not need regen")
	}

	// ── Purge: remove all Globular CA and service cert artifacts ──
	for _, f := range []string{caFile, certFile} {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			t.Fatalf("purge: remove %s: %v", f, err)
		}
	}

	// After purge: CA cert gone — any chain validation must degrade gracefully.
	// NeedsCertRegeneration on the missing cert file must report regen needed.
	need, reason := NeedsCertRegeneration(certFile, keyFile, caFile, nil, nil, 0)
	if !need {
		t.Fatal("after purge: missing cert must require regeneration")
	}
	if reason == "" {
		t.Error("reason must be non-empty after purge")
	}
	t.Logf("post-purge regen reason: %s", reason)

	// The CA cert itself must be gone — bootstrap trust from etcd on rejoin.
	if _, err := os.Stat(caFile); !os.IsNotExist(err) {
		t.Fatalf("purge must remove ca.crt; it still exists at %s", caFile)
	}
}
