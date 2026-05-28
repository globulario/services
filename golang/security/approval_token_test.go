package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// approvalTestEnv installs an in-process Ed25519 keypair and stubs the
// issuer/cluster lookups so MintApprovalToken/ValidateApprovalToken work
// without touching /etc/globular. It restores the previous globals on
// cleanup so other tests in the package are not affected.
func approvalTestEnv(t *testing.T) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	const (
		issuer    = "00:11:22:33:44:55"
		clusterID = "test.cluster.local"
		kid       = "test-kid"
	)
	prevIssuerFn := approvalGetIssuer
	prevClusterFn := approvalGetClusterID
	prevSign := GetIssuerSigningKey
	prevVerify := GetPeerPublicKey

	approvalGetIssuer = func() (string, error) { return issuer, nil }
	approvalGetClusterID = func() (string, error) { return clusterID, nil }
	GetIssuerSigningKey = func(iss string) (ed25519.PrivateKey, string, error) {
		if iss != issuer {
			return nil, "", errors.New("unexpected issuer in signing lookup: " + iss)
		}
		return priv, kid, nil
	}
	GetPeerPublicKey = func(iss, k string) (ed25519.PublicKey, error) {
		if iss != issuer {
			return nil, errors.New("unexpected issuer in verify lookup: " + iss)
		}
		if k != "" && k != kid {
			return nil, errors.New("unexpected kid in verify lookup: " + k)
		}
		return pub, nil
	}

	t.Cleanup(func() {
		approvalGetIssuer = prevIssuerFn
		approvalGetClusterID = prevClusterFn
		GetIssuerSigningKey = prevSign
		GetPeerPublicKey = prevVerify
	})
}

func sampleMintRequest() MintApprovalRequest {
	return MintApprovalRequest{
		Actor:       "operator@cluster",
		ActionClass: "SYSTEMCTL_STOP",
		Target:      "globule-ryzen/echo_server",
		Generation:  "gen-42",
		FindingID:   "finding-abc123",
	}
}

func sampleExpectation(req MintApprovalRequest) ApprovalExpectation {
	return ApprovalExpectation{
		ActionClass: req.ActionClass,
		Target:      req.Target,
		Generation:  req.Generation,
		FindingID:   req.FindingID,
	}
}

// TestApprovalTokenRejectsEmptyOrUnsignedToken — contract: non-empty != approved.
// Validator must reject empty strings, whitespace-only, and tokens not signed
// by the cluster issuer key.
func TestApprovalTokenRejectsEmptyOrUnsignedToken(t *testing.T) {
	approvalTestEnv(t)
	replay := NewInMemoryReplayStore()
	expect := sampleExpectation(sampleMintRequest())

	// Empty.
	if _, err := ValidateApprovalToken("", expect, replay); err == nil {
		t.Fatal("empty token must be rejected")
	}
	// Whitespace.
	if _, err := ValidateApprovalToken("   ", expect, replay); err == nil {
		t.Fatal("whitespace-only token must be rejected")
	}

	// Token signed by an unrelated key (impersonation attempt). The peer
	// public-key lookup will still return the cluster's real public key
	// (keyed by issuer), so verification must fail at signature check.
	_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate stranger key: %v", err)
	}
	now := time.Now()
	cluster, _ := approvalGetClusterID()
	issuer, _ := approvalGetIssuer()
	claims := &ApprovalClaims{
		ActionClass: expect.ActionClass,
		Target:      expect.Target,
		Generation:  expect.Generation,
		FindingID:   expect.FindingID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "jti-forged",
			Issuer:    issuer,
			Subject:   "imposter",
			Audience:  jwt.ClaimStrings{ApprovalAudiencePrefix + cluster},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	forged, err := tok.SignedString(otherPriv)
	if err != nil {
		t.Fatalf("sign forged token: %v", err)
	}
	if _, err := ValidateApprovalToken(forged, expect, replay); err == nil {
		t.Fatal("forged-signature token must be rejected")
	}
}

// TestApprovalTokenRejectsWrongAudience — contract: audience binds the token
// to the cluster's remediation surface. A token minted for a different
// cluster (or with the user-session audience) must not be accepted.
func TestApprovalTokenRejectsWrongAudience(t *testing.T) {
	approvalTestEnv(t)
	replay := NewInMemoryReplayStore()
	req := sampleMintRequest()
	expect := sampleExpectation(req)

	issuer, _ := approvalGetIssuer()
	priv, kid, err := GetIssuerSigningKey(issuer)
	if err != nil {
		t.Fatalf("get signing key: %v", err)
	}
	now := time.Now()
	claims := &ApprovalClaims{
		ActionClass: req.ActionClass,
		Target:      req.Target,
		Generation:  req.Generation,
		FindingID:   req.FindingID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "jti-aud",
			Issuer:    issuer,
			Subject:   req.Actor,
			// Wrong audience: the user-session validator would accept
			// something keyed on a peer MAC; the approval validator must not.
			Audience:  jwt.ClaimStrings{"00:aa:bb:cc:dd:ee"},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := ValidateApprovalToken(signed, expect, replay); err == nil {
		t.Fatal("wrong-audience token must be rejected")
	}

	// Cross-cluster: same prefix but a different cluster id.
	claims2 := *claims
	claims2.ID = "jti-aud-2"
	claims2.Audience = jwt.ClaimStrings{ApprovalAudiencePrefix + "other.cluster.local"}
	tok2 := jwt.NewWithClaims(jwt.SigningMethodEdDSA, &claims2)
	tok2.Header["kid"] = kid
	signed2, err := tok2.SignedString(priv)
	if err != nil {
		t.Fatalf("sign cross-cluster: %v", err)
	}
	if _, err := ValidateApprovalToken(signed2, expect, replay); err == nil {
		t.Fatal("cross-cluster audience must be rejected")
	}
}

// TestApprovalTokenRejectsExpiredToken — contract: expiry is hard. A token
// past its exp must be rejected even though everything else is correct.
func TestApprovalTokenRejectsExpiredToken(t *testing.T) {
	approvalTestEnv(t)
	replay := NewInMemoryReplayStore()
	req := sampleMintRequest()
	expect := sampleExpectation(req)

	issuer, _ := approvalGetIssuer()
	cluster, _ := approvalGetClusterID()
	priv, kid, err := GetIssuerSigningKey(issuer)
	if err != nil {
		t.Fatalf("get signing key: %v", err)
	}
	// Past expiry, outside the leeway window.
	now := time.Now()
	expired := now.Add(-(tokenExpirySkew + 30*time.Second))
	claims := &ApprovalClaims{
		ActionClass: req.ActionClass,
		Target:      req.Target,
		Generation:  req.Generation,
		FindingID:   req.FindingID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "jti-expired",
			Issuer:    issuer,
			Subject:   req.Actor,
			Audience:  jwt.ClaimStrings{ApprovalAudiencePrefix + cluster},
			IssuedAt:  jwt.NewNumericDate(expired.Add(-5 * time.Minute)),
			NotBefore: jwt.NewNumericDate(expired.Add(-5 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(expired),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := ValidateApprovalToken(signed, expect, replay); err == nil {
		t.Fatal("expired token must be rejected")
	}

	// And: mint-time rejects an unreasonable lifetime.
	tooLong := sampleMintRequest()
	tooLong.Lifetime = 2 * maxApprovalLifetime
	if _, err := MintApprovalToken(tooLong); err == nil {
		t.Fatal("mint must reject lifetime exceeding maxApprovalLifetime")
	}
}

// TestApprovalTokenRejectsReplay — contract: jti is single-use. Even a
// perfectly valid token must be rejected on the second presentation.
func TestApprovalTokenRejectsReplay(t *testing.T) {
	approvalTestEnv(t)
	replay := NewInMemoryReplayStore()
	req := sampleMintRequest()
	expect := sampleExpectation(req)

	signed, err := MintApprovalToken(req)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	// First use: accepted.
	first, err := ValidateApprovalToken(signed, expect, replay)
	if err != nil {
		t.Fatalf("first validation must succeed: %v", err)
	}
	if first.ID == "" {
		t.Fatal("first validation must return a non-empty jti")
	}

	// Replay: rejected with the sentinel error.
	if _, err := ValidateApprovalToken(signed, expect, replay); !errors.Is(err, ErrTokenAlreadyUsed) {
		t.Fatalf("replay must be rejected with ErrTokenAlreadyUsed, got: %v", err)
	}
}

// TestApprovalTokenBindsActionTargetAndGeneration — contract: the token
// authorizes exactly one (action_class, target, generation, finding_id)
// tuple. A token for a different action class or a different desired
// generation must not be honored, even though signature and audience pass.
func TestApprovalTokenBindsActionTargetAndGeneration(t *testing.T) {
	approvalTestEnv(t)
	req := sampleMintRequest()
	signed, err := MintApprovalToken(req)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*ApprovalExpectation)
		want   string
	}{
		{
			name:   "wrong action_class",
			mutate: func(e *ApprovalExpectation) { e.ActionClass = "PACKAGE_REINSTALL" },
			want:   "action_class mismatch",
		},
		{
			name:   "wrong target",
			mutate: func(e *ApprovalExpectation) { e.Target = "globule-nuc/echo_server" },
			want:   "target mismatch",
		},
		{
			name:   "wrong generation",
			mutate: func(e *ApprovalExpectation) { e.Generation = "gen-43" },
			want:   "generation mismatch",
		},
		{
			name:   "wrong finding_id",
			mutate: func(e *ApprovalExpectation) { e.FindingID = "finding-other" },
			want:   "finding_id mismatch",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Fresh replay store per case so the failure isn't masked by replay.
			replay := NewInMemoryReplayStore()
			expect := sampleExpectation(req)
			tc.mutate(&expect)
			_, err := ValidateApprovalToken(signed, expect, replay)
			if err == nil {
				t.Fatalf("validator must reject %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error must mention %q, got: %v", tc.want, err)
			}
		})
	}

	// And: the matching expectation does pass (sanity).
	replay := NewInMemoryReplayStore()
	if _, err := ValidateApprovalToken(signed, sampleExpectation(req), replay); err != nil {
		t.Fatalf("matching expectation must pass: %v", err)
	}
}
