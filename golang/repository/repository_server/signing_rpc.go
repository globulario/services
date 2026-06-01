package main

// signing_rpc.go — Phase CLI-B public RPCs for trust + signature.

import (
	"context"
	"strings"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── TrustPublisher ────────────────────────────────────────────────────────

func (srv *server) TrustPublisher(ctx context.Context, req *repopb.TrustPublisherRequest) (*repopb.TrustPublisherResponse, error) {
	if err := srv.requireCapability(CapRepoWrite); err != nil {
		return nil, err
	}
	pubID := strings.TrimSpace(req.GetPublisherId())
	keyID := canonicalKeyID(req.GetPublicKeyId())
	if pubID == "" {
		return nil, status.Error(codes.InvalidArgument, "publisher_id is required")
	}
	if keyID == "" {
		return nil, status.Error(codes.InvalidArgument, "public_key_id is required")
	}
	if len(req.GetPublicKeyPem()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "public_key_pem is required")
	}
	if _, err := parseEd25519PublicKey(req.GetPublicKeyPem()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid public key PEM: %v", err)
	}
	algo := strings.TrimSpace(req.GetAlgorithm())
	if algo == "" {
		algo = "ed25519"
	}
	if !strings.EqualFold(algo, "ed25519") {
		return nil, status.Errorf(codes.Unimplemented, "algorithm %q not supported (only ed25519)", algo)
	}
	createdBy := ""
	if a := security.FromContext(ctx); a != nil {
		createdBy = a.Subject
	}
	now := time.Now().Unix()
	p := &repopb.TrustedPublisher{
		PublisherId:    pubID,
		PublicKeyId:    keyID,
		PublicKeyPem:   req.GetPublicKeyPem(),
		Algorithm:      algo,
		TrustState:     repopb.TrustState_TRUST_TRUSTED,
		ValidFromUnix:  now,
		ValidUntilUnix: req.GetValidUntilUnix(),
		CreatedBy:      createdBy,
		CreatedUnix:    now,
		Notes:          req.GetNotes(),
	}
	if err := srv.saveTrustedPublisher(ctx, p); err != nil {
		return nil, status.Errorf(codes.Internal, "save trusted publisher: %v", err)
	}
	srv.publishAuditEvent(ctx, "repository.trust.publisher", map[string]any{
		"publisher_id":  pubID,
		"public_key_id": keyID,
		"algorithm":     algo,
		"created_by":    createdBy,
	})
	return &repopb.TrustPublisherResponse{Publisher: p}, nil
}

// ── RevokePublisherKey ────────────────────────────────────────────────────

func (srv *server) RevokePublisherKey(ctx context.Context, req *repopb.RevokePublisherKeyRequest) (*repopb.RevokePublisherKeyResponse, error) {
	if err := srv.requireCapability(CapRepoWrite); err != nil {
		return nil, err
	}
	pubID := strings.TrimSpace(req.GetPublisherId())
	keyID := canonicalKeyID(req.GetPublicKeyId())
	if pubID == "" || keyID == "" {
		return nil, status.Error(codes.InvalidArgument, "publisher_id and public_key_id are required")
	}
	p := srv.loadTrustedPublisher(ctx, pubID, keyID)
	if p == nil {
		return nil, status.Errorf(codes.NotFound, "no trusted publisher key %s/%s", pubID, keyID)
	}
	p.TrustState = repopb.TrustState_TRUST_REVOKED
	p.Notes = strings.TrimSpace("revoked: " + req.GetReason() + " | " + p.GetNotes())
	if err := srv.saveTrustedPublisher(ctx, p); err != nil {
		return nil, status.Errorf(codes.Internal, "revoke: %v", err)
	}
	srv.publishAuditEvent(ctx, "repository.trust.revoke", map[string]any{
		"publisher_id":  pubID,
		"public_key_id": keyID,
		"reason":        req.GetReason(),
	})
	return &repopb.RevokePublisherKeyResponse{Publisher: p}, nil
}

// ── ListTrustedPublishers ─────────────────────────────────────────────────

func (srv *server) ListTrustedPublishers(ctx context.Context, req *repopb.ListTrustedPublishersRequest) (*repopb.ListTrustedPublishersResponse, error) {
	if err := srv.requireCapability(CapRepoQuery); err != nil {
		return nil, err
	}
	rows := srv.loadAllTrustedPublishers(ctx, strings.TrimSpace(req.GetPublisherId()))
	return &repopb.ListTrustedPublishersResponse{Publishers: rows}, nil
}

// ── RegisterArtifactSignature ─────────────────────────────────────────────

func (srv *server) RegisterArtifactSignature(ctx context.Context, req *repopb.RegisterArtifactSignatureRequest) (*repopb.RegisterArtifactSignatureResponse, error) {
	if err := srv.requireCapability(CapRepoWrite); err != nil {
		return nil, err
	}
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	canonicalizeRefVersion(ref)
	keyID := canonicalKeyID(req.GetPublicKeyId())
	if keyID == "" {
		return nil, status.Error(codes.InvalidArgument, "public_key_id is required")
	}
	if len(req.GetSignatureBytes()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "signature_bytes is required")
	}
	algo := strings.TrimSpace(req.GetAlgorithm())
	if algo == "" {
		algo = "ed25519"
	}

	buildNumber := req.GetBuildNumber()
	if buildNumber == 0 {
		buildNumber = srv.resolveLatestBuildNumber(ctx, ref)
	}
	if buildNumber == 0 {
		return nil, status.Errorf(codes.NotFound, "no builds for %s/%s@%s [%s]",
			ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform())
	}
	key := artifactKeyWithBuild(ref, buildNumber)
	_, _, m, mErr := srv.readManifestAndStateByKey(ctx, key)
	if mErr != nil || m == nil {
		return nil, status.Errorf(codes.NotFound, "manifest for %s not found: %v", key, mErr)
	}
	digest := m.GetChecksum()
	if digest == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "artifact %s has no checksum to sign against", key)
	}

	signedBy := ""
	if a := security.FromContext(ctx); a != nil {
		signedBy = a.Subject
	}
	sig := &repopb.ArtifactSignature{
		ArtifactKey:    key,
		Digest:         canonicalDigest(digest),
		Algorithm:      algo,
		SignatureBytes: req.GetSignatureBytes(),
		PublicKeyId:    keyID,
		SignedBy:       signedBy,
		SignedAtUnix:   time.Now().Unix(),
		ProvenanceRef:  req.GetProvenanceRef(),
	}
	if err := srv.saveArtifactSignature(ctx, sig); err != nil {
		return nil, status.Errorf(codes.Internal, "save signature: %v", err)
	}

	// Verify against trusted-publishers immediately so the response carries
	// the trust outcome.
	st, _, _, _ := srv.verifyArtifactSignature(ctx, key, digest, ref.GetPublisherId())

	srv.publishAuditEvent(ctx, "repository.signature.register", map[string]any{
		"artifact_key":  key,
		"public_key_id": keyID,
		"signed_by":     signedBy,
		"status":        st.String(),
	})
	return &repopb.RegisterArtifactSignatureResponse{
		Signature: sig,
		Status:    st,
	}, nil
}

// ── VerifyArtifactSignature ───────────────────────────────────────────────

func (srv *server) VerifyArtifactSignature(ctx context.Context, req *repopb.VerifyArtifactSignatureRequest) (*repopb.VerifyArtifactSignatureResponse, error) {
	if err := srv.requireCapability(CapRepoQuery); err != nil {
		return nil, err
	}
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	canonicalizeRefVersion(ref)
	buildNumber := req.GetBuildNumber()
	if buildNumber == 0 {
		buildNumber = srv.resolveLatestBuildNumber(ctx, ref)
	}
	if buildNumber == 0 {
		return nil, status.Errorf(codes.NotFound, "no builds for %s/%s@%s [%s]",
			ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform())
	}
	key := artifactKeyWithBuild(ref, buildNumber)
	_, _, m, _ := srv.readManifestAndStateByKey(ctx, key)
	expectedDigest := ""
	if m != nil {
		expectedDigest = m.GetChecksum()
	}
	st, sig, pub, reason := srv.verifyArtifactSignature(ctx, key, expectedDigest, ref.GetPublisherId())
	return &repopb.VerifyArtifactSignatureResponse{
		Status:    st,
		Reason:    reason,
		Signature: sig,
		Publisher: pub,
	}, nil
}

// ── ListArtifactSignatures ────────────────────────────────────────────────

func (srv *server) ListArtifactSignatures(ctx context.Context, req *repopb.ListArtifactSignaturesRequest) (*repopb.ListArtifactSignaturesResponse, error) {
	if err := srv.requireCapability(CapRepoQuery); err != nil {
		return nil, err
	}
	ref := req.GetRef()
	if ref == nil {
		return nil, status.Error(codes.InvalidArgument, "ref is required")
	}
	canonicalizeRefVersion(ref)
	buildNumber := req.GetBuildNumber()
	if buildNumber == 0 {
		buildNumber = srv.resolveLatestBuildNumber(ctx, ref)
	}
	if buildNumber == 0 {
		return nil, status.Errorf(codes.NotFound, "no builds for %s/%s@%s [%s]",
			ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), ref.GetPlatform())
	}
	key := artifactKeyWithBuild(ref, buildNumber)
	sigs := srv.loadAllArtifactSignatures(ctx, key)
	return &repopb.ListArtifactSignaturesResponse{Signatures: sigs}, nil
}
