// @awareness namespace=globular.platform
// @awareness component=platform_security.approval_token
// @awareness file_role=single_use_audience_bound_remediation_approval_token_with_jti_replay_guard
// @awareness implements=globular.platform:intent.remediation.token_contract
// @awareness implements=globular.platform:intent.security.tokens_certificates_keys.cluster_trust_contract
// @awareness risk=critical
package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/golang-jwt/jwt/v5"
)

// ApprovalAudiencePrefix is prepended to the cluster id to form the approval
// audience. Keeping a distinct prefix prevents user-session tokens (audience =
// peer MAC) from being accepted by ValidateApprovalToken and vice versa.
const ApprovalAudiencePrefix = "remediation:"

// Default approval-token lifetime when callers do not set one explicitly.
// Short by design — approvals authorize one action and must not linger.
const defaultApprovalLifetime = 10 * time.Minute

// Maximum allowed lifetime; longer requests are rejected at mint time so an
// operator cannot inadvertently issue a long-lived bypass.
const maxApprovalLifetime = 1 * time.Hour

// Test seams: package-level callbacks for issuer/cluster lookup so tests can
// run without a populated local config file. Production code leaves them as
// the defaults below, which read /etc/globular/config/config.json.
var (
	approvalGetIssuer    = config.GetMacAddress
	approvalGetClusterID = config.GetDomain
)

// SetApprovalIssuerForTesting overrides the issuer lookup used by the
// approval-token mint/validate path. Returns a restore function the caller
// must defer. Intended for tests that wire an in-process Ed25519 keystore
// (see golang/security/approvaltest).
func SetApprovalIssuerForTesting(f func() (string, error)) func() {
	prev := approvalGetIssuer
	approvalGetIssuer = f
	return func() { approvalGetIssuer = prev }
}

// SetApprovalClusterIDForTesting overrides the cluster-id lookup used by
// the approval-token audience binding. Returns a restore function the
// caller must defer.
func SetApprovalClusterIDForTesting(f func() (string, error)) func() {
	prev := approvalGetClusterID
	approvalGetClusterID = f
	return func() { approvalGetClusterID = prev }
}

// ApprovalClaims is the signed payload of an approval token. The custom
// fields bind the approval to a specific action class, target, and desired
// generation. RegisteredClaims supplies audience, expiry, not-before, issuer,
// subject (actor identity), and jti (replay nonce).
type ApprovalClaims struct {
	ActionClass string `json:"action_class"` // e.g. "SYSTEMCTL_STOP", "PACKAGE_REINSTALL"
	Target      string `json:"target"`       // e.g. "node-id/service-id" or finding id
	Generation  string `json:"generation"`   // desired-state generation or evidence digest
	FindingID   string `json:"finding_id"`   // doctor finding the approval was issued for
	jwt.RegisteredClaims
}

// MintApprovalRequest carries the inputs needed to issue a token.
type MintApprovalRequest struct {
	Actor       string        // operator subject (principal_id) — required
	ActionClass string        // required
	Target      string        // required
	Generation  string        // required
	FindingID   string        // required
	Lifetime    time.Duration // optional; defaults to defaultApprovalLifetime
}

// ApprovalExpectation is what the validator must see in the token. Every
// field is required; mismatch on any field rejects the token.
type ApprovalExpectation struct {
	ActionClass string
	Target      string
	Generation  string
	FindingID   string
}

// ReplayStore tracks single-use token ids. Production implementations should
// persist to etcd with a TTL matching the token expiry; the in-memory store
// below is suitable for tests and single-process use.
type ReplayStore interface {
	// MarkUsed records that jti was used at the given time. Returns
	// ErrTokenAlreadyUsed if jti was already recorded.
	MarkUsed(jti string, expiresAt time.Time) error
}

// ErrTokenAlreadyUsed is returned by ReplayStore.MarkUsed and surfaced by
// ValidateApprovalToken when the same jti is presented twice.
var ErrTokenAlreadyUsed = errors.New("approval token already used")

// InMemoryReplayStore is a process-local replay store. It expires entries
// lazily on access; callers may also call Reap to drop expired entries.
type InMemoryReplayStore struct {
	mu   sync.Mutex
	used map[string]time.Time // jti -> expiry
}

// NewInMemoryReplayStore returns a fresh store.
func NewInMemoryReplayStore() *InMemoryReplayStore {
	return &InMemoryReplayStore{used: make(map[string]time.Time)}
}

// MarkUsed implements ReplayStore.
func (s *InMemoryReplayStore) MarkUsed(jti string, expiresAt time.Time) error {
	if jti == "" {
		return errors.New("approval token: jti is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if exp, ok := s.used[jti]; ok && exp.After(now) {
		return ErrTokenAlreadyUsed
	}
	s.used[jti] = expiresAt
	return nil
}

// Reap removes entries whose expiry has passed. Safe to call concurrently.
func (s *InMemoryReplayStore) Reap() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, exp := range s.used {
		if !exp.After(now) {
			delete(s.used, k)
		}
	}
}

// MintApprovalToken signs an approval token using the cluster issuer's
// Ed25519 key. The audience is "remediation:<cluster_id>" so the token
// cannot be replayed against the user-session validator.
//
func MintApprovalToken(req MintApprovalRequest) (string, error) {
	if err := validateMintRequest(req); err != nil {
		return "", err
	}
	lifetime := req.Lifetime
	if lifetime <= 0 {
		lifetime = defaultApprovalLifetime
	}
	if lifetime > maxApprovalLifetime {
		return "", fmt.Errorf("approval token: lifetime %s exceeds max %s", lifetime, maxApprovalLifetime)
	}

	issuer, err := approvalGetIssuer()
	if err != nil {
		return "", fmt.Errorf("approval token: get issuer mac: %w", err)
	}
	clusterID, err := approvalGetClusterID()
	if err != nil {
		return "", fmt.Errorf("approval token: get cluster id: %w", err)
	}
	audience := ApprovalAudiencePrefix + clusterID

	jti, err := randomJTI()
	if err != nil {
		return "", err
	}
	now := time.Now()
	claims := &ApprovalClaims{
		ActionClass: req.ActionClass,
		Target:      req.Target,
		Generation:  req.Generation,
		FindingID:   req.FindingID,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Issuer:    issuer,
			Subject:   req.Actor,
			Audience:  jwt.ClaimStrings{audience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(lifetime)),
		},
	}

	if GetIssuerSigningKey == nil {
		return "", errors.New("approval token: GetIssuerSigningKey not configured")
	}
	priv, kid, err := GetIssuerSigningKey(issuer)
	if err != nil {
		return "", fmt.Errorf("approval token: get issuer signing key: %w", err)
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	if kid != "" {
		tok.Header["kid"] = kid
	}
	signed, err := tok.SignedString(priv)
	if err != nil {
		return "", fmt.Errorf("approval token: sign: %w", err)
	}
	return signed, nil
}

// ValidateApprovalToken verifies the token's signature, audience, expiry,
// not-before, action/target/generation/finding binding, and single-use replay.
// It returns the parsed claims on success or a descriptive error on any
// failure. Callers MUST treat any error as a hard rejection.
// replay.MarkUsed is called LAST so a malformed or expired token is never
// recorded as used — only well-formed tokens count against the jti nonce.
//
func ValidateApprovalToken(tokenStr string, expect ApprovalExpectation, replay ReplayStore) (*ApprovalClaims, error) {
	if strings.TrimSpace(tokenStr) == "" {
		return nil, errors.New("approval token: token is empty")
	}
	if replay == nil {
		return nil, errors.New("approval token: replay store is required")
	}
	if err := validateExpectation(expect); err != nil {
		return nil, err
	}

	claims := &ApprovalClaims{}
	keyFunc := func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodEdDSA {
			return nil, fmt.Errorf("approval token: unexpected signing method: %v", t.Header["alg"])
		}
		iss := claims.Issuer
		if iss == "" {
			return nil, errors.New("approval token: missing issuer")
		}
		var kid string
		if k, ok := t.Header["kid"].(string); ok {
			kid = k
		}
		if GetPeerPublicKey == nil {
			return nil, errors.New("approval token: GetPeerPublicKey not configured")
		}
		pub, err := GetPeerPublicKey(iss, kid)
		if err != nil {
			return nil, fmt.Errorf("approval token: get public key (iss=%s,kid=%s): %w", iss, kid, err)
		}
		return pub, nil
	}

	clusterID, err := approvalGetClusterID()
	if err != nil {
		return nil, fmt.Errorf("approval token: get cluster id: %w", err)
	}
	expectedAud := ApprovalAudiencePrefix + clusterID
	parseOpts := []jwt.ParserOption{
		jwt.WithLeeway(tokenExpirySkew),
		jwt.WithAudience(expectedAud),
		jwt.WithIssuedAt(),
	}
	parsed, err := jwt.ParseWithClaims(tokenStr, claims, keyFunc, parseOpts...)
	if err != nil {
		return nil, fmt.Errorf("approval token: parse: %w", err)
	}
	if !parsed.Valid {
		return nil, errors.New("approval token: signature invalid")
	}

	// Bind to requested action/target/generation/finding.
	if claims.ActionClass != expect.ActionClass {
		return nil, fmt.Errorf("approval token: action_class mismatch (token=%q want=%q)",
			claims.ActionClass, expect.ActionClass)
	}
	if claims.Target != expect.Target {
		return nil, fmt.Errorf("approval token: target mismatch (token=%q want=%q)",
			claims.Target, expect.Target)
	}
	if claims.Generation != expect.Generation {
		return nil, fmt.Errorf("approval token: generation mismatch (token=%q want=%q)",
			claims.Generation, expect.Generation)
	}
	if claims.FindingID != expect.FindingID {
		return nil, fmt.Errorf("approval token: finding_id mismatch (token=%q want=%q)",
			claims.FindingID, expect.FindingID)
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, errors.New("approval token: actor (subject) is empty")
	}
	if claims.ID == "" {
		return nil, errors.New("approval token: jti is empty")
	}
	if claims.ExpiresAt == nil {
		return nil, errors.New("approval token: expiry is missing")
	}

	// Replay enforcement is the last step so a malformed/expired token is
	// not recorded as "used" — only well-formed tokens count against jti.
	if err := replay.MarkUsed(claims.ID, claims.ExpiresAt.Time); err != nil {
		return nil, fmt.Errorf("approval token: %w", err)
	}
	return claims, nil
}

func validateMintRequest(req MintApprovalRequest) error {
	switch {
	case strings.TrimSpace(req.Actor) == "":
		return errors.New("approval token: actor is required")
	case strings.TrimSpace(req.ActionClass) == "":
		return errors.New("approval token: action_class is required")
	case strings.TrimSpace(req.Target) == "":
		return errors.New("approval token: target is required")
	case strings.TrimSpace(req.Generation) == "":
		return errors.New("approval token: generation is required")
	case strings.TrimSpace(req.FindingID) == "":
		return errors.New("approval token: finding_id is required")
	}
	return nil
}

func validateExpectation(e ApprovalExpectation) error {
	switch {
	case strings.TrimSpace(e.ActionClass) == "":
		return errors.New("approval token: expected action_class is required")
	case strings.TrimSpace(e.Target) == "":
		return errors.New("approval token: expected target is required")
	case strings.TrimSpace(e.Generation) == "":
		return errors.New("approval token: expected generation is required")
	case strings.TrimSpace(e.FindingID) == "":
		return errors.New("approval token: expected finding_id is required")
	}
	return nil
}

func randomJTI() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("approval token: generate jti: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}
