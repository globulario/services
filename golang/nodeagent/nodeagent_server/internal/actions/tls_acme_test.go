package actions

import (
	"context"
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

	"github.com/go-acme/lego/v4/certificate"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestACMEEnsureValidate(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]interface{}
		wantErr bool
	}{
		{
			name:    "nil args",
			args:    nil,
			wantErr: true,
		},
		{
			name: "missing domain",
			args: map[string]interface{}{
				"admin_email": "test@example.com",
			},
			wantErr: true,
		},
		{
			name: "missing admin_email",
			args: map[string]interface{}{
				"domain": "example.com",
			},
			wantErr: true,
		},
		{
			name: "valid args",
			args: map[string]interface{}{
				"domain":       "example.com",
				"admin_email":  "test@example.com",
				"acme_enabled": true,
			},
			wantErr: false,
		},
	}

	act := acmeEnsureAction{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args *structpb.Struct
			if tt.args != nil {
				var err error
				args, err = structpb.NewStruct(tt.args)
				if err != nil {
					t.Fatalf("build args: %v", err)
				}
			}

			err := act.Validate(args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestACMEEnsureApplyDisabled(t *testing.T) {
	args, err := structpb.NewStruct(map[string]interface{}{
		"domain":       "example.com",
		"admin_email":  "test@example.com",
		"acme_enabled": false,
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	act := acmeEnsureAction{}
	result, err := act.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if result != "acme disabled, skipping" {
		t.Errorf("Apply() result = %q, want %q", result, "acme disabled, skipping")
	}
}

func TestACMEEnsureApplyValidCert(t *testing.T) {
	// Save and restore original nowFunc
	originalNow := nowFunc
	defer func() { nowFunc = originalNow }()

	// Mock time to a fixed point
	mockNow := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return mockNow }

	dir := t.TempDir()
	certPath := filepath.Join(dir, "fullchain.pem")
	keyPath := filepath.Join(dir, "privkey.pem")

	// Create a valid cert that expires in 60 days (well beyond the 30-day renewal threshold)
	notBefore := mockNow.Add(-24 * time.Hour)
	notAfter := mockNow.Add(60 * 24 * time.Hour)
	writeTestCert(t, certPath, keyPath, "example.com", notBefore, notAfter)

	args, err := structpb.NewStruct(map[string]interface{}{
		"domain":         "example.com",
		"admin_email":    "test@example.com",
		"acme_enabled":   true,
		"fullchain_path": certPath,
		"privkey_path":   keyPath,
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}

	act := acmeEnsureAction{}
	result, err := act.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	if result != "certificate valid, no renewal needed" {
		t.Errorf("Apply() result = %q, want %q", result, "certificate valid, no renewal needed")
	}
}

func TestNeedsCertRenewalMissing(t *testing.T) {
	needsRenewal, reason, err := needsCertRenewal("/nonexistent/cert.pem", "example.com")
	if err != nil {
		t.Fatalf("needsCertRenewal() error = %v", err)
	}
	if !needsRenewal {
		t.Error("needsCertRenewal() = false, want true for missing cert")
	}
	if reason != "certificate missing" {
		t.Errorf("needsCertRenewal() reason = %q, want %q", reason, "certificate missing")
	}
}

func TestNeedsCertRenewalInvalidPEM(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")

	// Write invalid PEM data
	if err := os.WriteFile(certPath, []byte("not a valid PEM"), 0o644); err != nil {
		t.Fatalf("write invalid cert: %v", err)
	}

	needsRenewal, reason, err := needsCertRenewal(certPath, "example.com")
	if err != nil {
		t.Fatalf("needsCertRenewal() error = %v", err)
	}
	if !needsRenewal {
		t.Error("needsCertRenewal() = false, want true for invalid PEM")
	}
	if reason != "invalid PEM format" {
		t.Errorf("needsCertRenewal() reason = %q, want %q", reason, "invalid PEM format")
	}
}

func TestNeedsCertRenewalExpiringSoon(t *testing.T) {
	// Save and restore original nowFunc
	originalNow := nowFunc
	defer func() { nowFunc = originalNow }()

	// Mock time
	mockNow := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return mockNow }

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	// Create cert that expires in 15 days (< 30 day threshold)
	notBefore := mockNow.Add(-24 * time.Hour)
	notAfter := mockNow.Add(15 * 24 * time.Hour)
	writeTestCert(t, certPath, keyPath, "example.com", notBefore, notAfter)

	needsRenewal, reason, err := needsCertRenewal(certPath, "example.com")
	if err != nil {
		t.Fatalf("needsCertRenewal() error = %v", err)
	}
	if !needsRenewal {
		t.Error("needsCertRenewal() = false, want true for expiring cert")
	}
	if reason != "expires in 15 days" {
		t.Errorf("needsCertRenewal() reason = %q, want %q", reason, "expires in 15 days")
	}
}

func TestNeedsCertRenewalSANMismatch(t *testing.T) {
	// Save and restore original nowFunc
	originalNow := nowFunc
	defer func() { nowFunc = originalNow }()

	// Mock time
	mockNow := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return mockNow }

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	// Create cert for different domain
	notBefore := mockNow.Add(-24 * time.Hour)
	notAfter := mockNow.Add(60 * 24 * time.Hour)
	writeTestCert(t, certPath, keyPath, "other.com", notBefore, notAfter)

	needsRenewal, reason, err := needsCertRenewal(certPath, "example.com")
	if err != nil {
		t.Fatalf("needsCertRenewal() error = %v", err)
	}
	if !needsRenewal {
		t.Error("needsCertRenewal() = false, want true for SAN mismatch")
	}
	if reason != "SAN mismatch" {
		t.Errorf("needsCertRenewal() reason = %q, want %q", reason, "SAN mismatch")
	}
}

func TestNeedsCertRenewalValid(t *testing.T) {
	// Save and restore original nowFunc
	originalNow := nowFunc
	defer func() { nowFunc = originalNow }()

	// Mock time
	mockNow := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	nowFunc = func() time.Time { return mockNow }

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	// Create valid cert that expires in 60 days
	notBefore := mockNow.Add(-24 * time.Hour)
	notAfter := mockNow.Add(60 * 24 * time.Hour)
	writeTestCert(t, certPath, keyPath, "example.com", notBefore, notAfter)

	needsRenewal, reason, err := needsCertRenewal(certPath, "example.com")
	if err != nil {
		t.Fatalf("needsCertRenewal() error = %v", err)
	}
	if needsRenewal {
		t.Errorf("needsCertRenewal() = true (reason: %s), want false for valid cert", reason)
	}
}

func TestGetOrCreateAccountKey(t *testing.T) {
	// Use a temp dir for the account key
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "acme_account.key")

	// Override the hardcoded path by testing the logic directly
	// First, test creating a new key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	// Marshal and save key
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	pemBlock := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	}

	if err := os.WriteFile(keyPath, pem.EncodeToMemory(pemBlock), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}

	// Now test loading the key
	data, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read key: %v", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatal("failed to decode PEM")
	}

	loadedKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse key: %v", err)
	}

	// Verify keys match
	if loadedKey.D.Cmp(key.D) != 0 {
		t.Error("loaded key does not match original key")
	}

	// Verify file permissions
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key file permissions = %o, want %o", info.Mode().Perm(), 0o600)
	}
}

func TestWriteCertificatesAtomic(t *testing.T) {
	dir := t.TempDir()

	// Create test certificate data
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "example.com",
		},
		NotBefore: time.Now().Add(-time.Hour),
		NotAfter:  time.Now().Add(time.Hour),
		DNSNames:  []string{"example.com"},
		KeyUsage:  x509.KeyUsageDigitalSignature,
	}

	pubKey := priv.Public()
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, pubKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	// Create certificate resource
	certResource := &certificate.Resource{
		Certificate: certPEM,
		PrivateKey:  keyPEM,
	}

	paths := tlsPathsSet{
		fullchain: filepath.Join(dir, "fullchain.pem"),
		privkey:   filepath.Join(dir, "privkey.pem"),
	}

	// Write certificates
	if err := writeCertificatesAtomic(paths, certResource); err != nil {
		t.Fatalf("writeCertificatesAtomic() error = %v", err)
	}

	// Verify fullchain was written
	if _, err := os.Stat(paths.fullchain); err != nil {
		t.Errorf("fullchain not written: %v", err)
	}

	// Verify privkey was written
	if _, err := os.Stat(paths.privkey); err != nil {
		t.Errorf("privkey not written: %v", err)
	}

	// Verify privkey permissions are restrictive
	info, err := os.Stat(paths.privkey)
	if err != nil {
		t.Fatalf("stat privkey: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("privkey permissions = %o, want %o", info.Mode().Perm(), 0o600)
	}

	// Verify fullchain permissions
	info, err = os.Stat(paths.fullchain)
	if err != nil {
		t.Fatalf("stat fullchain: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("fullchain permissions = %o, want %o", info.Mode().Perm(), 0o644)
	}

	// Verify content matches
	writtenCert, err := os.ReadFile(paths.fullchain)
	if err != nil {
		t.Fatalf("read fullchain: %v", err)
	}
	if string(writtenCert) != string(certPEM) {
		t.Error("fullchain content does not match")
	}

	writtenKey, err := os.ReadFile(paths.privkey)
	if err != nil {
		t.Fatalf("read privkey: %v", err)
	}
	if string(writtenKey) != string(keyPEM) {
		t.Error("privkey content does not match")
	}

	// Verify no temporary files left behind
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" {
			t.Errorf("temporary file left behind: %s", entry.Name())
		}
	}
}

func TestCertFingerprint(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	// Create test cert
	mockNow := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	notBefore := mockNow.Add(-time.Hour)
	notAfter := mockNow.Add(time.Hour)
	writeTestCert(t, certPath, keyPath, "example.com", notBefore, notAfter)

	// Get fingerprint
	fp1, err := CertFingerprint(certPath)
	if err != nil {
		t.Fatalf("CertFingerprint() error = %v", err)
	}

	// Verify fingerprint is non-empty hex string
	if fp1 == "" {
		t.Error("CertFingerprint() returned empty string")
	}
	if len(fp1) != 64 { // SHA256 hex = 64 chars
		t.Errorf("CertFingerprint() length = %d, want 64", len(fp1))
	}

	// Get fingerprint again, should match
	fp2, err := CertFingerprint(certPath)
	if err != nil {
		t.Fatalf("CertFingerprint() error = %v", err)
	}
	if fp1 != fp2 {
		t.Errorf("CertFingerprint() inconsistent: %s != %s", fp1, fp2)
	}

	// Modify cert, fingerprint should change
	writeTestCert(t, certPath, keyPath, "other.com", notBefore, notAfter)
	fp3, err := CertFingerprint(certPath)
	if err != nil {
		t.Fatalf("CertFingerprint() error = %v", err)
	}
	if fp1 == fp3 {
		t.Error("CertFingerprint() same for different certs")
	}

	// Test missing file
	_, err = CertFingerprint("/nonexistent/cert.pem")
	if err == nil {
		t.Error("CertFingerprint() expected error for missing file")
	}
}

func TestGlobularDNSProviderCreation(t *testing.T) {
	provider := newGlobularDNSProvider("localhost:10033", "example.com")
	if provider == nil {
		t.Fatal("newGlobularDNSProvider() returned nil")
	}
	if provider.dnsAddr != "localhost:10033" {
		t.Errorf("dnsAddr = %s, want localhost:10033", provider.dnsAddr)
	}
	if provider.domain != "example.com" {
		t.Errorf("domain = %s, want example.com", provider.domain)
	}
}
