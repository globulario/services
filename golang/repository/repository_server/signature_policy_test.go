package main

// signature_policy_test.go — Phase F Part 3 tests for the central
// signaturePolicyDecision and the gates that consult it (resolver,
// DownloadArtifact, rollback eligibility, sync publish quarantine).

import (
	"context"
	"crypto/ed25519"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// strictPolicy returns a policy that requires signatures for core publishers.
func strictPolicy() *repopb.SignaturePolicy {
	return &repopb.SignaturePolicy{
		RequireSignaturesForCore:      true,
		RequireSignaturesForAll:       false,
		AllowUnsignedLocalDevelopment: false,
		TrustedCorePublishers:         []string{"core@globular.io"},
		QuarantineOnInvalidSignature:  true,
	}
}

func TestSignaturePolicy_Required_AllowsValidSignature(t *testing.T) {
	srv := newTestServer(t)
	srv.signaturePolicy.SetPolicyForTest(strictPolicy())
	ctx := context.Background()

	priv, pubPEM, _ := makeEd25519Keypair(t)
	if _, err := srv.TrustPublisher(ctx, &repopb.TrustPublisherRequest{
		PublisherId: "core@globular.io", PublicKeyId: "k1",
		PublicKeyPem: pubPEM, Algorithm: "ed25519",
	}); err != nil {
		t.Fatalf("trust: %v", err)
	}
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

	dec := srv.signaturePolicyDecision(ctx, ref,
		artifactKeyWithBuild(ref, 1), digest, "")
	if !dec.Allowed {
		t.Fatalf("valid+trusted signature must be allowed; reason=%s", dec.Reason)
	}
	if !dec.Required {
		t.Error("core publisher must be marked Required under strict policy")
	}
}

func TestSignaturePolicy_Required_BlocksMissingSignature(t *testing.T) {
	srv := newTestServer(t)
	srv.signaturePolicy.SetPolicyForTest(strictPolicy())
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	dec := srv.signaturePolicyDecision(ctx, ref, "any-key", "sha256:abcd", "")
	if dec.Allowed {
		t.Fatal("missing+required signature must NOT be allowed")
	}
	if !dec.Required {
		t.Fatal("core publisher must be Required")
	}
}

func TestSignaturePolicy_NotRequired_AllowsMissing(t *testing.T) {
	srv := newTestServer(t)
	srv.signaturePolicy.SetPolicyForTest(strictPolicy())
	ctx := context.Background()

	// Third-party publisher: not in TrustedCorePublishers.
	ref := &repopb.ArtifactRef{
		PublisherId: "third@example.com", Name: "thing",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	dec := srv.signaturePolicyDecision(ctx, ref, "any-key", "sha256:abcd", "")
	if !dec.Allowed {
		t.Fatalf("third-party unsigned must be allowed under non-strict policy; reason=%s", dec.Reason)
	}
	if dec.Required {
		t.Error("third-party should not be Required under core-only policy")
	}
}

func TestSignaturePolicy_RevokedKeyAlwaysBlocks(t *testing.T) {
	srv := newTestServer(t)
	srv.signaturePolicy.SetPolicyForTest(&repopb.SignaturePolicy{
		// Even with a permissive policy, REVOKED key must block.
		RequireSignaturesForCore: false,
		TrustedCorePublishers:    []string{"core@globular.io"},
	})
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
	_, _ = srv.RegisterArtifactSignature(ctx, &repopb.RegisterArtifactSignatureRequest{
		Ref: ref, BuildNumber: 1, Algorithm: "ed25519",
		PublicKeyId: "k1", SignatureBytes: sig,
	})
	_, _ = srv.RevokePublisherKey(ctx, &repopb.RevokePublisherKeyRequest{
		PublisherId: "core@globular.io", PublicKeyId: "k1", Reason: "test",
	})
	dec := srv.signaturePolicyDecision(ctx, ref, artifactKeyWithBuild(ref, 1), digest, "")
	if dec.Allowed {
		t.Fatal("revoked key signature must NEVER be allowed")
	}
}

func TestSignaturePolicy_AllowUnsignedLocalDev(t *testing.T) {
	srv := newTestServer(t)
	srv.signaturePolicy.SetPolicyForTest(&repopb.SignaturePolicy{
		RequireSignaturesForCore:      true,
		AllowUnsignedLocalDevelopment: true,
		TrustedCorePublishers:         []string{"core@globular.io"},
	})
	ctx := context.Background()
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	dec := srv.signaturePolicyDecision(ctx, ref, "any-key", "sha256:abcd", "LOCAL_DIR")
	if !dec.Allowed {
		t.Fatalf("LOCAL_DIR + dev policy must allow unsigned core; reason=%s", dec.Reason)
	}
}

// ── Resolver / DownloadArtifact / rollback wiring tests ──────────────────

func TestResolver_BlocksMissingRequiredSignature(t *testing.T) {
	srv := newTestServer(t)
	srv.signaturePolicy.SetPolicyForTest(strictPolicy())
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: "sha256:abcd", SizeBytes: 100,
	})
	key := artifactKeyWithBuild(ref, 1)
	_ = srv.transitionArtifactState(ctx, key, PipelinePublished, "test", "", ArtifactStateFields{
		BlobKey: binaryStorageKey(key), Checksum: "sha256:abcd", SizeBytes: 100,
	})
	// No signature registered → resolver gate must reject.
	if srv.isInstallableForRef(ctx, ref, 1, repopb.PublishState_PUBLISHED) {
		t.Fatal("missing required signature must make artifact non-installable")
	}
	row := manifestRow{
		ArtifactKey: key,
		PublisherID: "core@globular.io", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Checksum: "sha256:abcd",
		PublishState:  repopb.PublishState_PUBLISHED.String(),
		ArtifactState: string(PipelinePublished),
	}
	if srv.isRowInstallableWithSignaturePolicy(ctx, &row) {
		t.Fatal("isRowInstallableWithSignaturePolicy must reject unsigned core under strict policy")
	}
}

func TestRollbackCandidate_BlocksRevokedPublisherKey(t *testing.T) {
	srv := newTestServer(t)
	srv.signaturePolicy.SetPolicyForTest(strictPolicy())
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
	key := artifactKeyWithBuild(ref, 1)
	_ = srv.transitionArtifactState(ctx, key, PipelinePublished, "test", "", ArtifactStateFields{
		BlobKey: binaryStorageKey(key), Checksum: digest, SizeBytes: 100,
	})
	sig := ed25519.Sign(priv, signedPayload(digest))
	_, _ = srv.RegisterArtifactSignature(ctx, &repopb.RegisterArtifactSignatureRequest{
		Ref: ref, BuildNumber: 1, Algorithm: "ed25519",
		PublicKeyId: "k1", SignatureBytes: sig,
	})
	// Revoke after signing — rollback eligibility must reject.
	_, _ = srv.RevokePublisherKey(ctx, &repopb.RevokePublisherKeyRequest{
		PublisherId: "core@globular.io", PublicKeyId: "k1", Reason: "test",
	})

	eli := srv.evaluateRollbackCandidate(ctx, ref, 1)
	if eli.GetEligible() {
		t.Fatal("rollback candidate with REVOKED key must be ineligible")
	}
}

func TestDevPolicy_AllowsUnsignedLocalOnlyWhenEnabled(t *testing.T) {
	// Feature-flag check: AllowUnsignedLocalDevelopment must be required for
	// LOCAL_DIR to skip the signature gate. With the flag off, LOCAL_DIR
	// behaves like any other source.
	srv := newTestServer(t)
	srv.signaturePolicy.SetPolicyForTest(&repopb.SignaturePolicy{
		RequireSignaturesForCore:      true,
		AllowUnsignedLocalDevelopment: false, // strict
		TrustedCorePublishers:         []string{"core@globular.io"},
	})
	ctx := context.Background()
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	dec := srv.signaturePolicyDecision(ctx, ref, "key", "sha256:abc", "LOCAL_DIR")
	if dec.Allowed {
		t.Fatal("AllowUnsignedLocalDevelopment=false must NOT allow LOCAL_DIR unsigned core")
	}

	// Now flip the flag.
	srv.signaturePolicy.SetPolicyForTest(&repopb.SignaturePolicy{
		RequireSignaturesForCore:      true,
		AllowUnsignedLocalDevelopment: true,
		TrustedCorePublishers:         []string{"core@globular.io"},
	})
	dec = srv.signaturePolicyDecision(ctx, ref, "key", "sha256:abc", "LOCAL_DIR")
	if !dec.Allowed {
		t.Fatal("AllowUnsignedLocalDevelopment=true must allow LOCAL_DIR unsigned core")
	}
}
