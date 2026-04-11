package main

import (
	"os"
	"testing"
)

// Test that TLS auto-detection enables HTTPS when service cert/key exist.
func TestMaybeEnableTLSFromServiceCert(t *testing.T) {
	// Setup temp cert/key files.
	dir := t.TempDir()
	cert := dir + "/service.crt"
	key := dir + "/service.key"
	if err := os.WriteFile(cert, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	if err := os.WriteFile(key, []byte("dummy"), 0644); err != nil {
		t.Fatalf("write key: %v", err)
	}

	// Override global paths and restore after.
	oldCert, oldKey := serviceCertPath, serviceKeyPath
	serviceCertPath, serviceKeyPath = cert, key
	t.Cleanup(func() {
		serviceCertPath, serviceKeyPath = oldCert, oldKey
	})

	cfg := defaultConfig()
	cfg.HTTPUseTLS = false
	cfg.HTTPTLSCertFile = ""
	cfg.HTTPTLSKeyFile = ""
	cfg.HTTPAdvertiseHost = ""

	changed := maybeEnableTLSFromServiceCert(cfg)
	if !changed {
		t.Fatalf("expected config to change")
	}
	if !cfg.HTTPUseTLS {
		t.Fatalf("expected HTTPUseTLS true")
	}
	if cfg.HTTPTLSCertFile != cert || cfg.HTTPTLSKeyFile != key {
		t.Fatalf("cert/key not set: got %s %s", cfg.HTTPTLSCertFile, cfg.HTTPTLSKeyFile)
	}
	if cfg.HTTPAdvertiseHost == "" {
		t.Fatalf("expected advertise host to be set")
	}
}

// Test no change when cert/key missing.
func TestMaybeEnableTLSFromServiceCertMissing(t *testing.T) {
	dir := t.TempDir()
	// Point to non-existent files.
	cert := dir + "/missing.crt"
	key := dir + "/missing.key"
	oldCert, oldKey := serviceCertPath, serviceKeyPath
	serviceCertPath, serviceKeyPath = cert, key
	t.Cleanup(func() {
		serviceCertPath, serviceKeyPath = oldCert, oldKey
	})

	cfg := defaultConfig()
	cfg.HTTPUseTLS = false
	changed := maybeEnableTLSFromServiceCert(cfg)
	if changed {
		t.Fatalf("expected no change when cert/key absent")
	}
}
