package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/globulario/services/golang/security"
)

// writeSelfSignedNoIPSANs writes a validly-signed leaf whose SANs carry a DNS
// name and no IP addresses — the exact shape a CA gateway returned on
// 2026-08-23 when asked to re-issue a node-agent certificate.
func writeSelfSignedNoIPSANs(t *testing.T, dir string) (certPath, keyPath string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "globular.internal"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		DNSNames:     []string{"globular.internal"}, // no IPAddresses, on purpose
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	certPath = filepath.Join(dir, "service.crt")
	keyPath = filepath.Join(dir, "service.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key),
	}), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}

// TestCertVerificationMustCarryRequiredIPs pins the difference between the
// check the TLS repair path used to make and the one it makes now.
//
// The repair loop DECIDED to regenerate using the node's required IPs, then
// VERIFIED the result passing nil for those IPs — so the verification could not
// observe the only thing that had gone wrong. On 2026-08-23 a re-issue returned
// a generic mesh certificate (CN=globular.internal, SAN DNS:globular.internal,
// no IP SANs), the nil-IP check saw a well-formed leaf, and node-agent logged
// "repaired runtime TLS certificate". Every controller infra probe to that node
// then failed with "cannot validate certificate for <ip> because it doesn't
// contain any IP SANs", the reconcile workflow logged remediation_no_progress
// 44 times in an hour, and the cluster reported the node ready with
// cluster-doctor showing zero errors the whole time.
//
// A verification that cannot fail for the defect it follows is decoration
// (diagnostics.must_measure_reality).
func TestCertVerificationMustCarryRequiredIPs(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := writeSelfSignedNoIPSANs(t, dir)

	// The old check: no required IPs. A cert with zero IP SANs looks fine.
	if regen, why := security.NeedsCertRegeneration(certPath, keyPath, "", nil, nil, 0); regen {
		t.Fatalf("precondition: cert should look acceptable when no IPs are required, got regen=true (%s)", why)
	}

	// The check the repair path must make: the node's own address is required.
	requiredIPs := []net.IP{net.ParseIP("10.10.0.13")}
	regen, why := security.NeedsCertRegeneration(certPath, keyPath, "", nil, requiredIPs, 0)
	if !regen {
		t.Fatal("a certificate with no IP SANs must be rejected when the node's IP is required — " +
			"this is the check that lets a repair notice it repaired nothing usable")
	}
	if why == "" {
		t.Error("rejection must carry a reason an operator can act on")
	}
}
