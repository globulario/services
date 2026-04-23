package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
)

func TestFileKeystoreGetPeerPublicKey_FetchesFromClusterOnMiss(t *testing.T) {
	t.Setenv("GLOBULAR_STATE_DIR", t.TempDir())

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	enc, err := encodeEd25519PublicPEM(pub)
	if err != nil {
		t.Fatalf("encode public key: %v", err)
	}

	origFetch := fetchPeerPublicKeyFromCluster
	fetchPeerPublicKeyFromCluster = func(issuer, kid string) ([]byte, error) {
		if issuer != "00:11:22:33:44:55" || kid != "kid-test" {
			t.Fatalf("unexpected fetch args issuer=%q kid=%q", issuer, kid)
		}
		return enc, nil
	}
	defer func() { fetchPeerPublicKeyFromCluster = origFetch }()

	got, err := fileKeystoreGetPeerPublicKey("00:11:22:33:44:55", "kid-test")
	if err != nil {
		t.Fatalf("fileKeystoreGetPeerPublicKey() error = %v", err)
	}
	if string(got) != string(pub) {
		t.Fatalf("unexpected public key bytes")
	}

	if _, err := readEd25519Public(publicKeyPath("00:11:22:33:44:55", "kid-test")); err != nil {
		t.Fatalf("kid-aware public key not cached locally: %v", err)
	}
	if _, err := readEd25519Public(publicKeyPath("00:11:22:33:44:55", "")); err != nil {
		t.Fatalf("legacy public key not cached locally: %v", err)
	}
}

func TestFileKeystoreGetIssuerSigningKey_DoesNotFailWhenPublishFails(t *testing.T) {
	t.Setenv("GLOBULAR_STATE_DIR", t.TempDir())

	origPublish := publishPeerPublicKeyToCluster
	publishPeerPublicKeyToCluster = func(issuer, kid string, encPub []byte) error {
		return errors.New("forced publish failure")
	}
	defer func() { publishPeerPublicKeyToCluster = origPublish }()

	if _, _, err := fileKeystoreGetIssuerSigningKey("00:aa:bb:cc:dd:ee"); err != nil {
		t.Fatalf("fileKeystoreGetIssuerSigningKey() should ignore publish errors, got: %v", err)
	}
}
