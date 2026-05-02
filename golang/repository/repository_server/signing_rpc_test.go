package main

// signing_rpc_test.go — Phase CLI-B tests for trusted publishers and
// detached artifact signatures.

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// helper: produce a fresh ed25519 keypair encoded as PEM (raw-32-byte body).
func makeEd25519Keypair(t *testing.T) (priv ed25519.PrivateKey, pubPEM []byte, pub ed25519.PublicKey) {
	t.Helper()
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("ed25519 keygen: %v", err)
	}
	pubPEM = pem.EncodeToMemory(&pem.Block{
		Type:  "ED25519 PUBLIC KEY",
		Bytes: []byte(pubKey),
	})
	return privKey, pubPEM, pubKey
}

func TestParseEd25519PublicKey_RawAndPKIX(t *testing.T) {
	_, pemBytes, pub := makeEd25519Keypair(t)
	parsed, err := parseEd25519PublicKey(pemBytes)
	if err != nil {
		t.Fatalf("parse raw: %v", err)
	}
	if string(parsed) != string(pub) {
		t.Fatal("raw PEM round-trip mismatch")
	}
}

func TestTrustPublisher_AndListAndRevoke(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	_, pubPEM, _ := makeEd25519Keypair(t)

	if _, err := srv.TrustPublisher(ctx, &repopb.TrustPublisherRequest{
		PublisherId:  "core@globular.io",
		PublicKeyId:  "core-prod-2026",
		PublicKeyPem: pubPEM,
		Algorithm:    "ed25519",
	}); err != nil {
		t.Fatalf("TrustPublisher: %v", err)
	}

	resp, err := srv.ListTrustedPublishers(ctx, &repopb.ListTrustedPublishersRequest{
		PublisherId: "core@globular.io",
	})
	if err != nil {
		t.Fatalf("ListTrustedPublishers: %v", err)
	}
	if len(resp.GetPublishers()) != 1 {
		t.Fatalf("expected 1 publisher, got %d", len(resp.GetPublishers()))
	}
	if resp.GetPublishers()[0].GetTrustState() != repopb.TrustState_TRUST_TRUSTED {
		t.Fatalf("expected TRUSTED, got %s", resp.GetPublishers()[0].GetTrustState())
	}

	// Revoke and re-list.
	if _, err := srv.RevokePublisherKey(ctx, &repopb.RevokePublisherKeyRequest{
		PublisherId: "core@globular.io",
		PublicKeyId: "core-prod-2026",
		Reason:      "test_revoke",
	}); err != nil {
		t.Fatalf("RevokePublisherKey: %v", err)
	}
	resp, _ = srv.ListTrustedPublishers(ctx, &repopb.ListTrustedPublishersRequest{
		PublisherId: "core@globular.io",
	})
	if len(resp.GetPublishers()) != 1 {
		t.Fatalf("expected 1 publisher after revoke, got %d", len(resp.GetPublishers()))
	}
	if resp.GetPublishers()[0].GetTrustState() != repopb.TrustState_TRUST_REVOKED {
		t.Fatalf("expected REVOKED, got %s", resp.GetPublishers()[0].GetTrustState())
	}
}

func TestSignature_ValidTrustedPublisher_AllowsVerify(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	priv, pubPEM, _ := makeEd25519Keypair(t)

	// Trust the key.
	if _, err := srv.TrustPublisher(ctx, &repopb.TrustPublisherRequest{
		PublisherId: "core@globular.io", PublicKeyId: "k1",
		PublicKeyPem: pubPEM, Algorithm: "ed25519",
	}); err != nil {
		t.Fatalf("trust: %v", err)
	}

	// Seed an artifact with a known checksum.
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	digest := "sha256:abcd"
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: digest, SizeBytes: 100,
	})

	// Sign the canonical digest payload.
	sig := ed25519.Sign(priv, signedPayload(digest))
	resp, err := srv.RegisterArtifactSignature(ctx, &repopb.RegisterArtifactSignatureRequest{
		Ref: ref, BuildNumber: 1, Algorithm: "ed25519",
		PublicKeyId: "k1", SignatureBytes: sig,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if resp.GetStatus() != repopb.SignatureStatus_SIGNATURE_OK {
		t.Fatalf("register status: got %s, want SIGNATURE_OK", resp.GetStatus())
	}

	v, err := srv.VerifyArtifactSignature(ctx, &repopb.VerifyArtifactSignatureRequest{
		Ref: ref, BuildNumber: 1,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if v.GetStatus() != repopb.SignatureStatus_SIGNATURE_OK {
		t.Fatalf("verify status: got %s, want OK (reason=%s)", v.GetStatus(), v.GetReason())
	}
}

func TestSignature_MissingSignature(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: "sha256:abcd", SizeBytes: 100,
	})
	v, err := srv.VerifyArtifactSignature(ctx, &repopb.VerifyArtifactSignatureRequest{
		Ref: ref, BuildNumber: 1,
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if v.GetStatus() != repopb.SignatureStatus_SIGNATURE_MISSING {
		t.Fatalf("got %s, want SIGNATURE_MISSING", v.GetStatus())
	}
}

func TestSignature_InvalidSignature_Detected(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	priv, pubPEM, _ := makeEd25519Keypair(t)
	_, _ = srv.TrustPublisher(ctx, &repopb.TrustPublisherRequest{
		PublisherId: "core@globular.io", PublicKeyId: "k1",
		PublicKeyPem: pubPEM, Algorithm: "ed25519",
	})
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	digest := "sha256:abcd"
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: digest, SizeBytes: 100,
	})
	// Sign DIFFERENT payload to produce a non-matching signature.
	bad := ed25519.Sign(priv, []byte("not the digest"))
	resp, err := srv.RegisterArtifactSignature(ctx, &repopb.RegisterArtifactSignatureRequest{
		Ref: ref, BuildNumber: 1, Algorithm: "ed25519",
		PublicKeyId: "k1", SignatureBytes: bad,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if resp.GetStatus() != repopb.SignatureStatus_SIGNATURE_INVALID {
		t.Fatalf("got %s, want SIGNATURE_INVALID", resp.GetStatus())
	}
}

func TestSignature_RevokedKey_BlocksVerification(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	priv, pubPEM, _ := makeEd25519Keypair(t)
	_, _ = srv.TrustPublisher(ctx, &repopb.TrustPublisherRequest{
		PublisherId: "core@globular.io", PublicKeyId: "k1",
		PublicKeyPem: pubPEM, Algorithm: "ed25519",
	})
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	digest := "sha256:abcd"
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: digest, SizeBytes: 100,
	})
	sig := ed25519.Sign(priv, signedPayload(digest))
	if _, err := srv.RegisterArtifactSignature(ctx, &repopb.RegisterArtifactSignatureRequest{
		Ref: ref, BuildNumber: 1, Algorithm: "ed25519",
		PublicKeyId: "k1", SignatureBytes: sig,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	// Revoke the key.
	_, _ = srv.RevokePublisherKey(ctx, &repopb.RevokePublisherKeyRequest{
		PublisherId: "core@globular.io", PublicKeyId: "k1", Reason: "test",
	})
	v, _ := srv.VerifyArtifactSignature(ctx, &repopb.VerifyArtifactSignatureRequest{
		Ref: ref, BuildNumber: 1,
	})
	if v.GetStatus() != repopb.SignatureStatus_SIGNATURE_REVOKED_KEY {
		t.Fatalf("got %s, want SIGNATURE_REVOKED_KEY", v.GetStatus())
	}
}

func TestSignature_UntrustedPublisher(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	priv, _, _ := makeEd25519Keypair(t)
	// Don't trust the publisher.
	ref := &repopb.ArtifactRef{
		PublisherId: "evil@attacker.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	digest := "sha256:abcd"
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: digest, SizeBytes: 100,
	})
	sig := ed25519.Sign(priv, signedPayload(digest))
	resp, err := srv.RegisterArtifactSignature(ctx, &repopb.RegisterArtifactSignatureRequest{
		Ref: ref, BuildNumber: 1, Algorithm: "ed25519",
		PublicKeyId: "ghost-key", SignatureBytes: sig,
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if resp.GetStatus() != repopb.SignatureStatus_SIGNATURE_UNTRUSTED_PUBLISHER {
		t.Fatalf("got %s, want SIGNATURE_UNTRUSTED_PUBLISHER", resp.GetStatus())
	}
}
