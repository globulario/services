package actions

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

	"google.golang.org/protobuf/types/known/structpb"
)

func TestTlsEnsureMissing(t *testing.T) {
	dir := t.TempDir()
	args, err := structpb.NewStruct(map[string]interface{}{
		"fullchain_path": filepath.Join(dir, "fullchain.pem"),
		"privkey_path":   filepath.Join(dir, "privkey.pem"),
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}
	act := tlsEnsureAction{}
	if _, applyErr := act.Apply(nil, args); applyErr == nil {
		t.Fatalf("expected error for missing tls material")
	}
}

func TestTlsCertValidForDomain(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	writeTestCert(t, certPath, keyPath, "example.com", time.Now().Add(-time.Hour), time.Now().Add(time.Hour))

	args, err := structpb.NewStruct(map[string]interface{}{
		"domain":    "example.com",
		"cert_path": certPath,
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}
	validAct := tlsCertValidAction{}
	if _, applyErr := validAct.Apply(nil, args); applyErr != nil {
		t.Fatalf("expected cert valid, got %v", applyErr)
	}

	args, err = structpb.NewStruct(map[string]interface{}{
		"domain":    "bad.example.com",
		"cert_path": certPath,
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}
	if _, applyErr := validAct.Apply(nil, args); applyErr == nil {
		t.Fatalf("expected mismatch error for domain")
	}

	args, err = structpb.NewStruct(map[string]interface{}{
		"domain":    "example.com",
		"cert_path": certPath,
	})
	if err != nil {
		t.Fatalf("build args: %v", err)
	}
	writeTestCert(t, certPath, keyPath, "example.com", time.Now().Add(-2*time.Hour), time.Now().Add(-time.Hour))
	if _, applyErr := validAct.Apply(nil, args); applyErr == nil {
		t.Fatalf("expected expiration error")
	}
}

func writeTestCert(t *testing.T, certPath, keyPath, domain string, notBefore, notAfter time.Time) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,
		DNSNames:  []string{domain},
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	certOut, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("open cert file: %v", err)
	}
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	certOut.Close()

	keyOut, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("open key file: %v", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		t.Fatalf("write key: %v", err)
	}
	keyOut.Close()
}
