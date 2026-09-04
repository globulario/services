package config

// Regression tests for the etcd client certificate source.
//
// These cover the failure that cost node-3 37 minutes of unresolvable
// "infra_unhealthy on etcd@<node>" drift in the 1.2.360 soak run: the agent
// cached a client certificate signed by the wrong CA, the correct certificate
// was restored on disk minutes later, and nothing rebuilt the cached config.
//
// The invariant under test is etcd.endpoint_reachability — "no silent
// fallback". A client that cannot present an acceptable certificate must not
// look, to its caller, exactly like a client that can.

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCA(t *testing.T, cn string) *testCA {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate CA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create CA cert: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse CA cert: %v", err)
	}
	return &testCA{cert: cert, key: key}
}

// writeLeaf issues a leaf from ca and writes the PEM pair into dir.
func writeLeaf(t *testing.T, dir, cn string, ca *testCA) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth,
		},
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatalf("create leaf: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal leaf key: %v", err)
	}

	certPath = filepath.Join(dir, "service.crt")
	keyPath = filepath.Join(dir, "service.key")
	writeFile(t, certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	writeFile(t, keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}))
	return certPath, keyPath
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func poolFor(ca *testCA) *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(ca.cert)
	return p
}

// leafCN returns the CN of the certificate the source hands back.
func leafCN(t *testing.T, c *tls.Certificate) string {
	t.Helper()
	if c == nil || len(c.Certificate) == 0 {
		return ""
	}
	parsed, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		t.Fatalf("parse returned cert: %v", err)
	}
	return parsed.Subject.CommonName
}

// TestEtcdClientCertSource_PicksUpRestoredCertWithoutRestart is the defect
// itself: a bad certificate is replaced on disk and the source must serve the
// new one. Before the fix the keypair was read once into cfg.Certificates and
// only a process restart could change it.
func TestEtcdClientCertSource_PicksUpRestoredCertWithoutRestart(t *testing.T) {
	dir := t.TempDir()
	clusterCA := newTestCA(t, "cluster-ca")
	chaosCA := newTestCA(t, "chaos-ca")

	// Start with the wrong-CA leaf, exactly as the chaos injection leaves it.
	certPath, keyPath := writeLeaf(t, dir, "chaos-expired", chaosCA)
	src := newEtcdClientCertSource(certPath, keyPath, poolFor(clusterCA))

	got, err := src.getClientCertificate(nil)
	if err != nil {
		t.Fatalf("first handshake: %v", err)
	}
	if cn := leafCN(t, got); cn != "chaos-expired" {
		t.Fatalf("first handshake served %q, want chaos-expired", cn)
	}

	// Restore the correct keypair, as chaos.restore_cert does.
	// mtime must differ for the reload to trigger; make that explicit rather
	// than relying on filesystem timestamp granularity.
	writeLeaf(t, dir, "service", clusterCA)
	future := time.Now().Add(2 * time.Second)
	for _, p := range []string{certPath, keyPath} {
		if err := os.Chtimes(p, future, future); err != nil {
			t.Fatalf("chtimes %s: %v", p, err)
		}
	}

	got, err = src.getClientCertificate(nil)
	if err != nil {
		t.Fatalf("handshake after restore: %v", err)
	}
	if cn := leafCN(t, got); cn != "service" {
		t.Fatalf("after restore the source still served %q — the cached certificate "+
			"was not reloaded, which is the 1.2.360 node-3 defect", cn)
	}
}

// TestEtcdClientCertSource_UnchangedMtimeIsServedFromCache is the negative
// control for the reload test: without it, a source that simply re-read the
// files on every handshake would pass that test and the mtime guard would be
// untested. New content is written while the mtime is pinned, so only a cache
// hit produces the old certificate.
func TestEtcdClientCertSource_UnchangedMtimeIsServedFromCache(t *testing.T) {
	dir := t.TempDir()
	clusterCA := newTestCA(t, "cluster-ca")
	certPath, keyPath := writeLeaf(t, dir, "service", clusterCA)

	certInfo, err := os.Stat(certPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	keyInfo, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	src := newEtcdClientCertSource(certPath, keyPath, poolFor(clusterCA))
	first, err := src.getClientCertificate(nil)
	if err != nil {
		t.Fatalf("first handshake: %v", err)
	}

	// Different content, identical mtime.
	writeLeaf(t, dir, "replacement", clusterCA)
	if err := os.Chtimes(certPath, certInfo.ModTime(), certInfo.ModTime()); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if err := os.Chtimes(keyPath, keyInfo.ModTime(), keyInfo.ModTime()); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	second, err := src.getClientCertificate(nil)
	if err != nil {
		t.Fatalf("second handshake: %v", err)
	}
	if first != second || leafCN(t, second) != "service" {
		t.Fatalf("an unchanged mtime produced a reload (served %q) — the guard is "+
			"not working, and every handshake would pay a disk read",
			leafCN(t, second))
	}
}

// TestEtcdClientCertSource_VanishedFilesKeepLastGoodCert covers the fault case
// that absence must NOT be confused with: once a node has presented a
// certificate, the files disappearing is a fault. Downgrading to no
// certificate there would turn a missing file into an unexplained timeout
// against client-cert-auth true — the silent fallback etcd.endpoint_reachability
// forbids.
func TestEtcdClientCertSource_VanishedFilesKeepLastGoodCert(t *testing.T) {
	dir := t.TempDir()
	clusterCA := newTestCA(t, "cluster-ca")
	certPath, keyPath := writeLeaf(t, dir, "service", clusterCA)
	src := newEtcdClientCertSource(certPath, keyPath, poolFor(clusterCA))

	if _, err := src.getClientCertificate(nil); err != nil {
		t.Fatalf("first handshake: %v", err)
	}
	os.Remove(certPath)
	os.Remove(keyPath)

	got, err := src.getClientCertificate(nil)
	if err != nil {
		t.Fatalf("handshake after the files vanished: %v", err)
	}
	if cn := leafCN(t, got); cn != "service" {
		t.Fatalf("after the keypair vanished the source served %q; it must keep "+
			"presenting the last good certificate rather than silently "+
			"downgrading to none", cn)
	}
}

// TestEtcdClientCertSource_PresentButUnusableIsAnError covers the discarded
// LoadX509KeyPair error. A present-but-corrupt keypair used to yield a config
// with no client certificate at all, which against client-cert-auth true is an
// unexplained timeout rather than a reported failure.
func TestEtcdClientCertSource_PresentButUnusableIsAnError(t *testing.T) {
	dir := t.TempDir()
	clusterCA := newTestCA(t, "cluster-ca")
	certPath := filepath.Join(dir, "service.crt")
	keyPath := filepath.Join(dir, "service.key")
	writeFile(t, certPath, []byte("-----BEGIN CERTIFICATE-----\nnot a certificate\n-----END CERTIFICATE-----\n"))
	writeFile(t, keyPath, []byte("-----BEGIN EC PRIVATE KEY-----\nnot a key\n-----END EC PRIVATE KEY-----\n"))

	src := newEtcdClientCertSource(certPath, keyPath, poolFor(clusterCA))
	if _, err := src.getClientCertificate(nil); err == nil {
		t.Fatal("a present but unusable keypair must be reported, not silently " +
			"replaced by an empty certificate")
	}
}

// TestEtcdClientCertSource_AbsentKeypairStillConnects protects the Day-0
// window: etcd starts with client-cert-auth false before any service cert
// exists, and a node must be able to reach it then.
func TestEtcdClientCertSource_AbsentKeypairStillConnects(t *testing.T) {
	dir := t.TempDir()
	clusterCA := newTestCA(t, "cluster-ca")
	src := newEtcdClientCertSource(
		filepath.Join(dir, "service.crt"),
		filepath.Join(dir, "service.key"),
		poolFor(clusterCA),
	)

	got, err := src.getClientCertificate(nil)
	if err != nil {
		t.Fatalf("an absent keypair must not fail the handshake during Day-0: %v", err)
	}
	if got == nil || len(got.Certificate) != 0 {
		t.Fatalf("expected an empty certificate, got %+v", got)
	}
}

// TestEtcdClientCertSource_WrongCALeafIsNamed asserts the condition Go reports
// nowhere is at least named by us. Without this the operator sees only
// "context deadline exceeded" against every endpoint.
func TestEtcdClientCertSource_WrongCALeafIsNamed(t *testing.T) {
	dir := t.TempDir()
	clusterCA := newTestCA(t, "cluster-ca")
	chaosCA := newTestCA(t, "chaos-ca")
	certPath, keyPath := writeLeaf(t, dir, "chaos-expired", chaosCA)

	src := newEtcdClientCertSource(certPath, keyPath, poolFor(clusterCA))
	if _, err := src.getClientCertificate(nil); err != nil {
		t.Fatalf("a wrong-CA leaf must still be served (etcd decides): %v", err)
	}
	if !src.warnedUntrusted {
		t.Fatal("a leaf that does not chain to the cluster CA must be reported: " +
			"Go drops it during the handshake and surfaces no error, so this is " +
			"the only place the condition is nameable before it becomes a timeout")
	}

	// And the control: a correctly-issued leaf must NOT be flagged.
	writeLeaf(t, dir, "service", clusterCA)
	future := time.Now().Add(2 * time.Second)
	for _, p := range []string{certPath, keyPath} {
		if err := os.Chtimes(p, future, future); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}
	if _, err := src.getClientCertificate(nil); err != nil {
		t.Fatalf("valid leaf: %v", err)
	}
	if src.warnedUntrusted {
		t.Fatal("a leaf issued by the cluster CA was reported as untrusted")
	}
}

// TestEtcdClientCertSource_EndToEndAgainstRequiringServer ties the unit
// behaviour to the observable symptom: a real handshake against a server
// configured the way etcd is (client-cert-auth: true).
func TestEtcdClientCertSource_EndToEndAgainstRequiringServer(t *testing.T) {
	dir := t.TempDir()
	clusterCA := newTestCA(t, "cluster-ca")
	chaosCA := newTestCA(t, "chaos-ca")

	serverDir := t.TempDir()
	srvCertPath, srvKeyPath := writeLeaf(t, serverDir, "etcd", clusterCA)
	srvPair, err := tls.LoadX509KeyPair(srvCertPath, srvKeyPath)
	if err != nil {
		t.Fatalf("server keypair: %v", err)
	}

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{srvPair},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    poolFor(clusterCA),
		MinVersion:   tls.VersionTLS12,
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	serverErr := make(chan error, 2)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			serverErr <- c.(*tls.Conn).Handshake()
			c.Close()
		}
	}()

	certPath, keyPath := writeLeaf(t, dir, "chaos-expired", chaosCA)
	src := newEtcdClientCertSource(certPath, keyPath, poolFor(clusterCA))
	dial := func() error {
		c, err := tls.Dial("tcp", ln.Addr().String(), &tls.Config{
			RootCAs:              poolFor(clusterCA),
			GetClientCertificate: src.getClientCertificate,
			MinVersion:           tls.VersionTLS12,
		})
		if err == nil {
			c.Close()
		}
		return <-serverErr
	}

	if err := dial(); err == nil {
		t.Fatal("a chaos-CA leaf must be rejected by a client-cert-auth server")
	}

	// Restore, and the very next connection must be accepted — no restart.
	writeLeaf(t, dir, "service", clusterCA)
	future := time.Now().Add(2 * time.Second)
	for _, p := range []string{certPath, keyPath} {
		if err := os.Chtimes(p, future, future); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}
	if err := dial(); err != nil {
		t.Fatalf("after restoring the correct keypair the handshake must succeed "+
			"without rebuilding the client, got: %v", err)
	}
}
