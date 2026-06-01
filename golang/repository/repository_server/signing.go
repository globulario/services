// @awareness namespace=globular.platform
// @awareness component=platform_repository.signing
// @awareness file_role=artifact_signing_and_signature_verification
// @awareness implements=globular.platform:intent.repository.signature_policy_gates_trust
// @awareness risk=high
package main

// signing.go — Phase CLI-B signing / trusted publisher / signature verification.
//
// Globular signs artifact digests with ed25519 keys (consistent with the
// existing PKI under /var/lib/globular/keys/). Public keys are registered as
// TrustedPublisher rows in ScyllaDB. The repository never stores private keys.
//
// Verification flow:
//   1. Load the ArtifactSignature row for (artifact_key, public_key_id).
//   2. Verify the signature against the artifact's digest (sha256:<hex>).
//   3. Look up the TrustedPublisher row; check trust_state, validity window.
//   4. Return SignatureStatus.
//
// Policy gating (require_signatures_for_core, require_signatures_for_all,
// allow_unsigned_local_development) is read from repository config — in this
// pass policy is permissive: SIGNATURE_MISSING is INCONCLUSIVE rather than
// QUARANTINED. Strict mode is a one-line flip in this file once operators
// register keys cluster-wide.

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/pem"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gocql/gocql"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── Public-key parsing ────────────────────────────────────────────────────

// parseEd25519PublicKey accepts a PEM-encoded ed25519 public key block of the
// form "BEGIN PUBLIC KEY" (PKIX) or "BEGIN ED25519 PUBLIC KEY" (raw 32 bytes
// in the PEM body). Returns the raw 32-byte key.
func parseEd25519PublicKey(pemBytes []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM block")
	}
	switch block.Type {
	case "PUBLIC KEY", "ED25519 PUBLIC KEY":
		// Heuristic: ed25519 raw key is exactly 32 bytes.
		if len(block.Bytes) == ed25519.PublicKeySize {
			return ed25519.PublicKey(block.Bytes), nil
		}
		// PKIX-wrapped — strip the SPKI prefix. ed25519 SPKI prefix is 12 bytes.
		const spkiPrefixLen = 12
		if len(block.Bytes) == spkiPrefixLen+ed25519.PublicKeySize {
			return ed25519.PublicKey(block.Bytes[spkiPrefixLen:]), nil
		}
		return nil, fmt.Errorf("unsupported ed25519 PEM body length %d", len(block.Bytes))
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q", block.Type)
	}
}

// signedPayload returns the canonical bytes that get signed: the digest of
// the artifact, normalized into "sha256:<hex>" form.
func signedPayload(digest string) []byte {
	return []byte(canonicalDigest(digest))
}

// VerifySignatureBytes validates a detached signature against an artifact
// digest using the trusted publisher's key. Returns the SignatureStatus.
func VerifySignatureBytes(digest string, sig []byte, pub ed25519.PublicKey) repopb.SignatureStatus {
	if len(sig) == 0 {
		return repopb.SignatureStatus_SIGNATURE_MISSING
	}
	if len(pub) != ed25519.PublicKeySize {
		return repopb.SignatureStatus_SIGNATURE_INVALID
	}
	if canonicalDigest(digest) == "" {
		return repopb.SignatureStatus_SIGNATURE_DIGEST_MISMATCH
	}
	if !ed25519.Verify(pub, signedPayload(digest), sig) {
		return repopb.SignatureStatus_SIGNATURE_INVALID
	}
	return repopb.SignatureStatus_SIGNATURE_OK
}

// ── Scylla CRUD: trusted_publishers ───────────────────────────────────────

// putTrustedPublisher upserts a row in trusted_publishers.
func (s *scyllaStore) putTrustedPublisher(ctx context.Context, p *repopb.TrustedPublisher) error {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("scylla: not connected")
	}
	return sess.Query(`INSERT INTO trusted_publishers
		(publisher_id, public_key_id, public_key_pem, algorithm, trust_state,
		 valid_from_unix, valid_until_unix, created_by, created_unix, notes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.GetPublisherId(), p.GetPublicKeyId(), p.GetPublicKeyPem(),
		p.GetAlgorithm(), p.GetTrustState().String(),
		p.GetValidFromUnix(), p.GetValidUntilUnix(),
		p.GetCreatedBy(), p.GetCreatedUnix(), p.GetNotes(),
	).WithContext(ctx).Exec()
}

func (s *scyllaStore) getTrustedPublisher(ctx context.Context, publisherID, keyID string) (*repopb.TrustedPublisher, error) {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return nil, fmt.Errorf("scylla: not connected")
	}
	var (
		pemBytes []byte
		alg, ts, createdBy, notes string
		validFrom, validUntil, createdUnix int64
	)
	err := sess.Query(`SELECT public_key_pem, algorithm, trust_state,
		valid_from_unix, valid_until_unix, created_by, created_unix, notes
		FROM trusted_publishers WHERE publisher_id = ? AND public_key_id = ?`,
		publisherID, keyID).WithContext(ctx).Scan(
		&pemBytes, &alg, &ts, &validFrom, &validUntil, &createdBy, &createdUnix, &notes)
	if err != nil {
		return nil, err
	}
	state := repopb.TrustState_TRUST_STATE_UNSPECIFIED
	if v, ok := repopb.TrustState_value[ts]; ok {
		state = repopb.TrustState(v)
	}
	return &repopb.TrustedPublisher{
		PublisherId:    publisherID,
		PublicKeyId:    keyID,
		PublicKeyPem:   pemBytes,
		Algorithm:      alg,
		TrustState:     state,
		ValidFromUnix:  validFrom,
		ValidUntilUnix: validUntil,
		CreatedBy:      createdBy,
		CreatedUnix:    createdUnix,
		Notes:          notes,
	}, nil
}

func (s *scyllaStore) listTrustedPublishers(ctx context.Context, publisherID string) ([]*repopb.TrustedPublisher, error) {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return nil, fmt.Errorf("scylla: not connected")
	}
	var iter *gocql.Iter
	if publisherID == "" {
		iter = sess.Query(`SELECT publisher_id, public_key_id, public_key_pem, algorithm, trust_state,
			valid_from_unix, valid_until_unix, created_by, created_unix, notes FROM trusted_publishers`).
			WithContext(ctx).Iter()
	} else {
		iter = sess.Query(`SELECT publisher_id, public_key_id, public_key_pem, algorithm, trust_state,
			valid_from_unix, valid_until_unix, created_by, created_unix, notes FROM trusted_publishers
			WHERE publisher_id = ?`, publisherID).WithContext(ctx).Iter()
	}
	var out []*repopb.TrustedPublisher
	var (
		pubID, keyID, alg, ts, createdBy, notes string
		pemBytes []byte
		validFrom, validUntil, createdUnix int64
	)
	for iter.Scan(&pubID, &keyID, &pemBytes, &alg, &ts, &validFrom, &validUntil, &createdBy, &createdUnix, &notes) {
		state := repopb.TrustState_TRUST_STATE_UNSPECIFIED
		if v, ok := repopb.TrustState_value[ts]; ok {
			state = repopb.TrustState(v)
		}
		buf := make([]byte, len(pemBytes))
		copy(buf, pemBytes)
		out = append(out, &repopb.TrustedPublisher{
			PublisherId: pubID, PublicKeyId: keyID, PublicKeyPem: buf,
			Algorithm: alg, TrustState: state,
			ValidFromUnix: validFrom, ValidUntilUnix: validUntil,
			CreatedBy: createdBy, CreatedUnix: createdUnix, Notes: notes,
		})
	}
	return out, iter.Close()
}

// ── Scylla CRUD: artifact_signatures ──────────────────────────────────────

func (s *scyllaStore) putArtifactSignature(ctx context.Context, sig *repopb.ArtifactSignature) error {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return fmt.Errorf("scylla: not connected")
	}
	return sess.Query(`INSERT INTO artifact_signatures
		(artifact_key, public_key_id, digest, algorithm, signature_bytes,
		 signed_by, signed_at_unix, provenance_ref)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sig.GetArtifactKey(), sig.GetPublicKeyId(), sig.GetDigest(),
		sig.GetAlgorithm(), sig.GetSignatureBytes(),
		sig.GetSignedBy(), sig.GetSignedAtUnix(), sig.GetProvenanceRef(),
	).WithContext(ctx).Exec()
}

func (s *scyllaStore) listArtifactSignatures(ctx context.Context, artifactKey string) ([]*repopb.ArtifactSignature, error) {
	s.mu.Lock()
	sess := s.session
	s.mu.Unlock()
	if sess == nil {
		return nil, fmt.Errorf("scylla: not connected")
	}
	iter := sess.Query(`SELECT public_key_id, digest, algorithm, signature_bytes,
		signed_by, signed_at_unix, provenance_ref FROM artifact_signatures
		WHERE artifact_key = ?`, artifactKey).WithContext(ctx).Iter()
	var out []*repopb.ArtifactSignature
	var (
		keyID, digest, alg, signedBy, provRef string
		sigBytes []byte
		signedAt int64
	)
	for iter.Scan(&keyID, &digest, &alg, &sigBytes, &signedBy, &signedAt, &provRef) {
		buf := make([]byte, len(sigBytes))
		copy(buf, sigBytes)
		out = append(out, &repopb.ArtifactSignature{
			ArtifactKey: artifactKey, PublicKeyId: keyID, Digest: digest,
			Algorithm: alg, SignatureBytes: buf, SignedBy: signedBy,
			SignedAtUnix: signedAt, ProvenanceRef: provRef,
		})
	}
	return out, iter.Close()
}

// ── In-memory fallback (used when scylla == nil) ─────────────────────────

// Tests run without a Scylla session. Mirror trust/signature data into
// per-server maps so Trust/Verify still work end-to-end.
//
// These maps are guarded by the existing artifactStateMu — operators rarely
// hit this path concurrently and the lock is already cheap.
type trustCache struct {
	publishers map[string]map[string]*repopb.TrustedPublisher // publisherID → keyID → row
	signatures map[string]map[string]*repopb.ArtifactSignature // artifactKey → keyID → sig
}

func (srv *server) initTrustCache() {
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	if srv.trust == nil {
		srv.trust = &trustCache{
			publishers: map[string]map[string]*repopb.TrustedPublisher{},
			signatures: map[string]map[string]*repopb.ArtifactSignature{},
		}
	}
}

func (srv *server) cacheTrustedPublisher(p *repopb.TrustedPublisher) {
	srv.initTrustCache()
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	if _, ok := srv.trust.publishers[p.GetPublisherId()]; !ok {
		srv.trust.publishers[p.GetPublisherId()] = map[string]*repopb.TrustedPublisher{}
	}
	srv.trust.publishers[p.GetPublisherId()][p.GetPublicKeyId()] = p
}

func (srv *server) lookupTrustedPublisher(publisherID, keyID string) *repopb.TrustedPublisher {
	srv.initTrustCache()
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	if m, ok := srv.trust.publishers[publisherID]; ok {
		return m[keyID]
	}
	return nil
}

func (srv *server) listCachedTrustedPublishers(publisherID string) []*repopb.TrustedPublisher {
	srv.initTrustCache()
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	var out []*repopb.TrustedPublisher
	if publisherID == "" {
		for _, m := range srv.trust.publishers {
			for _, p := range m {
				out = append(out, p)
			}
		}
		return out
	}
	for _, p := range srv.trust.publishers[publisherID] {
		out = append(out, p)
	}
	return out
}

func (srv *server) cacheArtifactSignature(sig *repopb.ArtifactSignature) {
	srv.initTrustCache()
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	if _, ok := srv.trust.signatures[sig.GetArtifactKey()]; !ok {
		srv.trust.signatures[sig.GetArtifactKey()] = map[string]*repopb.ArtifactSignature{}
	}
	srv.trust.signatures[sig.GetArtifactKey()][sig.GetPublicKeyId()] = sig
}

func (srv *server) listCachedSignatures(artifactKey string) []*repopb.ArtifactSignature {
	srv.initTrustCache()
	srv.artifactStateMu.Lock()
	defer srv.artifactStateMu.Unlock()
	var out []*repopb.ArtifactSignature
	for _, sig := range srv.trust.signatures[artifactKey] {
		out = append(out, sig)
	}
	return out
}

// ── High-level helpers (used by RPC handlers) ────────────────────────────

func (srv *server) saveTrustedPublisher(ctx context.Context, p *repopb.TrustedPublisher) error {
	srv.cacheTrustedPublisher(p)
	if srv.scylla != nil {
		if ss, ok := srv.scylla.(*scyllaStore); ok {
			return ss.putTrustedPublisher(ctx, p)
		}
	}
	return nil
}

func (srv *server) loadTrustedPublisher(ctx context.Context, publisherID, keyID string) *repopb.TrustedPublisher {
	if p := srv.lookupTrustedPublisher(publisherID, keyID); p != nil {
		return p
	}
	if srv.scylla != nil {
		if ss, ok := srv.scylla.(*scyllaStore); ok {
			if p, err := ss.getTrustedPublisher(ctx, publisherID, keyID); err == nil && p != nil {
				srv.cacheTrustedPublisher(p)
				return p
			}
		}
	}
	return nil
}

func (srv *server) loadAllTrustedPublishers(ctx context.Context, publisherID string) []*repopb.TrustedPublisher {
	if srv.scylla != nil {
		if ss, ok := srv.scylla.(*scyllaStore); ok {
			if rows, err := ss.listTrustedPublishers(ctx, publisherID); err == nil {
				for _, r := range rows {
					srv.cacheTrustedPublisher(r)
				}
				return rows
			}
		}
	}
	return srv.listCachedTrustedPublishers(publisherID)
}

func (srv *server) saveArtifactSignature(ctx context.Context, sig *repopb.ArtifactSignature) error {
	srv.cacheArtifactSignature(sig)
	if srv.scylla != nil {
		if ss, ok := srv.scylla.(*scyllaStore); ok {
			return ss.putArtifactSignature(ctx, sig)
		}
	}
	return nil
}

func (srv *server) loadAllArtifactSignatures(ctx context.Context, artifactKey string) []*repopb.ArtifactSignature {
	if srv.scylla != nil {
		if ss, ok := srv.scylla.(*scyllaStore); ok {
			if rows, err := ss.listArtifactSignatures(ctx, artifactKey); err == nil {
				for _, r := range rows {
					srv.cacheArtifactSignature(r)
				}
				return rows
			}
		}
	}
	return srv.listCachedSignatures(artifactKey)
}

// verifyArtifactSignature classifies the most recent signature on an
// artifact against the trusted-publisher table.
func (srv *server) verifyArtifactSignature(ctx context.Context, artifactKey, expectedDigest, publisherID string) (
	repopb.SignatureStatus, *repopb.ArtifactSignature, *repopb.TrustedPublisher, string,
) {
	sigs := srv.loadAllArtifactSignatures(ctx, artifactKey)
	if len(sigs) == 0 {
		return repopb.SignatureStatus_SIGNATURE_MISSING, nil, nil, "no signatures registered for artifact"
	}
	// Pick the most recent signature.
	var newest *repopb.ArtifactSignature
	for _, s := range sigs {
		if newest == nil || s.GetSignedAtUnix() > newest.GetSignedAtUnix() {
			newest = s
		}
	}

	pub := srv.loadTrustedPublisher(ctx, publisherID, newest.GetPublicKeyId())
	if pub == nil {
		return repopb.SignatureStatus_SIGNATURE_UNTRUSTED_PUBLISHER, newest, nil,
			fmt.Sprintf("publisher %s key %s not registered", publisherID, newest.GetPublicKeyId())
	}
	if pub.GetTrustState() == repopb.TrustState_TRUST_REVOKED {
		return repopb.SignatureStatus_SIGNATURE_REVOKED_KEY, newest, pub, "key marked REVOKED"
	}
	if pub.GetValidUntilUnix() > 0 && pub.GetValidUntilUnix() < time.Now().Unix() {
		return repopb.SignatureStatus_SIGNATURE_EXPIRED_KEY, newest, pub, "key past valid_until"
	}
	if !digestEqual(newest.GetDigest(), expectedDigest) {
		return repopb.SignatureStatus_SIGNATURE_DIGEST_MISMATCH, newest, pub,
			"signed digest does not match artifact digest"
	}
	pubKey, err := parseEd25519PublicKey(pub.GetPublicKeyPem())
	if err != nil {
		return repopb.SignatureStatus_SIGNATURE_INVALID, newest, pub, "parse public key: " + err.Error()
	}
	if !ed25519.Verify(pubKey, signedPayload(expectedDigest), newest.GetSignatureBytes()) {
		return repopb.SignatureStatus_SIGNATURE_INVALID, newest, pub, "signature does not verify"
	}
	return repopb.SignatureStatus_SIGNATURE_OK, newest, pub, "ok"
}

// canonicalKeyID strips whitespace and lowercases — same normalization rule
// digest helpers use, applied to publisher key identifiers.
func canonicalKeyID(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// debugPEM exists so we don't trip the unused-import check during fast iter
// when bytes / pem / slog haven't all been used yet by the rest of this file.
var _ = bytes.Buffer{}
var _ = slog.Default
