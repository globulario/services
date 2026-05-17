package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/globulario/services/golang/awareness/bundlesync"
)

// ── Phase C.6 spec test 8: `globular awareness sync` end-to-end ──────────────
//
// Spins up a real httptest TLS peer that serves a matching manifest+bundle,
// writes the peer's cert as a CA file the CLI can read, then runs the sync
// command. Asserts that the install actually happened on disk.

// makeMatchingBundle returns a (gzip)tar with graph.db plus a matching manifest.
func makeMatchingBundle(t *testing.T, version, buildID string) ([]byte, bundlesync.Manifest) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	body := []byte("graph for " + version + "/" + buildID)
	hdr := &tar.Header{Name: "graph.db", Mode: 0644, Size: int64(len(body)), Typeflag: tar.TypeReg}
	tw.WriteHeader(hdr)
	tw.Write(body)
	tw.Close()
	gz.Close()
	data := buf.Bytes()

	h := sha256.Sum256(data)
	m := bundlesync.Manifest{
		Name:          bundlesync.BundleName,
		Version:       version,
		BuildID:       buildID,
		SchemaVersion: "awareness.bundle.v1",
		SHA256:        hex.EncodeToString(h[:]),
		SizeBytes:     int64(len(data)),
	}
	return data, m
}

// writeServerCertAsPEM writes the test server's cert to a CA-style PEM file
// the CLI can load with --ca.
func writeServerCertAsPEM(t *testing.T, cert *x509.Certificate, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "ca.crt")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if err := os.WriteFile(path, pemBytes, 0644); err != nil {
		t.Fatalf("write ca: %v", err)
	}
	return path
}

// writeFreshSelfSignedCAPEM generates a brand-new self-signed CA cert and
// writes it as PEM into dir/ca.crt. Used for the untrusted-TLS test:
// httptest.NewTLSServer reuses Go's baked-in LocalhostCert across instances,
// so writing a "different" httptest cert isn't actually different. A freshly
// generated CA is guaranteed not to chain to the test server.
func writeFreshSelfSignedCAPEM(t *testing.T, dir string) string {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-untrusted-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	path := filepath.Join(dir, "ca.crt")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(path, pemBytes, 0644); err != nil {
		t.Fatalf("write ca: %v", err)
	}
	return path
}

// TestSpec8_SyncCommandEndToEnd is the spec's test 8: "Sync Command".
//
// Runs the actual cobra Run function for `globular awareness sync` against a
// real TLS peer. On success, the bundle must be installed and the current
// symlink must point at it.
func TestSpec8_SyncCommandEndToEnd(t *testing.T) {
	resetFlags(t)

	// Peer side.
	bundleBytes, manifest := makeMatchingBundle(t, "v1.2.30", "abc123")
	mux := http.NewServeMux()
	mux.HandleFunc("/awareness/manifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(manifest)
	})
	mux.HandleFunc("/awareness/bundle", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(bundleBytes)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	// Local node side: bundle root, release-index, CA file.
	bundleRoot := t.TempDir()
	indexDir := t.TempDir()
	caDir := t.TempDir()
	indexPath := writeReleaseIndex(t, indexDir, bundlesync.ReleaseIndex{
		Version: manifest.Version, BuildID: manifest.BuildID,
	})
	caPath := writeServerCertAsPEM(t, srv.Certificate(), caDir)

	awarenessSyncCfg.from = srv.URL
	awarenessSyncCfg.bundleRoot = bundleRoot
	awarenessSyncCfg.releaseIndex = indexPath
	awarenessSyncCfg.caPath = caPath
	awarenessSyncCfg.json = true

	awarenessSyncCmd.SetContext(context.Background())
	var out bytes.Buffer
	awarenessSyncCmd.SetOut(&out)
	awarenessSyncCmd.SetErr(&out)

	if err := awarenessSyncCmd.RunE(awarenessSyncCmd, nil); err != nil {
		t.Fatalf("sync RunE: %v\nout=%s", err, out.String())
	}

	// Active symlink points at the new versioned dir.
	want := filepath.Join(bundleRoot, "installed", manifest.Version, manifest.BuildID)
	target, err := os.Readlink(filepath.Join(bundleRoot, "current"))
	if err != nil {
		t.Fatalf("readlink current: %v\nout=%s", err, out.String())
	}
	if target != want {
		t.Errorf("current → %s, want %s", target, want)
	}

	// graph.db is on disk.
	if _, err := os.Stat(filepath.Join(want, "graph.db")); err != nil {
		t.Errorf("graph.db missing post-install: %v", err)
	}

	// JSON output should mention both phases.
	outStr := out.String()
	if !bytes.Contains(out.Bytes(), []byte(`"pull"`)) {
		t.Errorf("expected pull section in JSON output; got %s", outStr)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"install"`)) {
		t.Errorf("expected install section in JSON output; got %s", outStr)
	}
}

// TestSpec8_SyncCommandFailsOnUntrustedTLS verifies the CLI propagates
// trust failures as a non-zero exit (RunE returns an error). Belt-and-
// suspenders: the bundlesync layer is already tested for this, but we also
// want the CLI integration path to behave the same way.
func TestSpec8_SyncCommandFailsOnUntrustedTLS(t *testing.T) {
	resetFlags(t)

	bundleBytes, manifest := makeMatchingBundle(t, "v1.2.30", "abc123")
	mux := http.NewServeMux()
	mux.HandleFunc("/awareness/manifest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(manifest)
	})
	mux.HandleFunc("/awareness/bundle", func(w http.ResponseWriter, r *http.Request) {
		w.Write(bundleBytes)
	})
	srv := httptest.NewTLSServer(mux)
	defer srv.Close()

	bundleRoot := t.TempDir()
	indexDir := t.TempDir()
	caDir := t.TempDir()
	indexPath := writeReleaseIndex(t, indexDir, bundlesync.ReleaseIndex{
		Version: manifest.Version, BuildID: manifest.BuildID,
	})

	// Generate a fresh self-signed CA that the peer's cert can't chain to.
	// (httptest.NewTLSServer reuses Go's baked-in LocalhostCert, so two test
	// servers' certs verify each other — a freshly generated CA does not.)
	caPath := writeFreshSelfSignedCAPEM(t, caDir)

	awarenessSyncCfg.from = srv.URL
	awarenessSyncCfg.bundleRoot = bundleRoot
	awarenessSyncCfg.releaseIndex = indexPath
	awarenessSyncCfg.caPath = caPath

	awarenessSyncCmd.SetContext(context.Background())
	var out bytes.Buffer
	awarenessSyncCmd.SetOut(&out)
	awarenessSyncCmd.SetErr(&out)

	if err := awarenessSyncCmd.RunE(awarenessSyncCmd, nil); err == nil {
		t.Fatalf("sync RunE did not error on untrusted TLS\nout=%s", out.String())
	}

	if _, err := os.Lstat(filepath.Join(bundleRoot, "current")); err == nil {
		t.Error("current must not exist when sync fails on TLS")
	}
}
