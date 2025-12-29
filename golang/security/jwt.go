package security

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
	"github.com/golang-jwt/jwt/v5" // maintained fork
	"google.golang.org/grpc/metadata"
)

// ============================================================================
// Integration points (provide implementations in your keystore/control plane)
// ============================================================================

// GetIssuerSigningKey must return the issuer's Ed25519 private key and its KID.
// - issuer: your peer/node identity (e.g., MAC, SPIFFE, DNS).
// - kid:    key id to place in JWT header for rotation.
var GetIssuerSigningKey func(issuer string) (ed25519.PrivateKey, string, error)

// GetPeerPublicKey must return the issuer's Ed25519 public key for verification.
// - issuer: token "iss"
// - kid:    token header "kid"
var GetPeerPublicKey func(issuer, kid string) (ed25519.PublicKey, error)

// ----------------------------------------------------------------------------
// Types
// ----------------------------------------------------------------------------

// Authentication holds a bearer token for gRPC per-RPC credentials.
type Authentication struct {
	Token string
}

// GetRequestMetadata returns the outgoing metadata containing the JWT token.
func (a *Authentication) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	// Send both a custom header and a standard Authorization header.
	return map[string]string{
		"token":         a.Token,
		"authorization": "Bearer " + a.Token,
	}, nil
}

// RequireTransportSecurity indicates whether the credentials require TLS.
func (a *Authentication) RequireTransportSecurity() bool { return true }

// Claims is the signed JWT payload. It embeds jwt.RegisteredClaims.
type Claims struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Domain     string `json:"domain"`      // Where the token was generated
	UserDomain string `json:"user_domain"` // User's domain/tenant
	Address    string `json:"address"`
	jwt.RegisteredClaims
}

// ----------------------------------------------------------------------------
// Package-level state
// ----------------------------------------------------------------------------

var (
	logger = slog.Default()

	// in-memory cache of local tokens keyed by RAW MAC (with colons).
	// (File paths use a normalized MAC; see normalizeMACForFile)
	tokens sync.Map
)

// ----------------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------------

func normalizeMACForFile(mac string) string {
	return strings.ReplaceAll(mac, ":", "_")
}

func tokenDir() string { return config.GetTokensDir() }

func tokenPathForMAC(mac string) string {
	return filepath.Join(tokenDir(), normalizeMACForFile(mac)+"_token")
}

const (
	defaultSessionTimeoutMinutes = 60                 // fallback if config is missing/zero
	tokenExpirySkew              = 60 * time.Second   // leeway to handle clock skew
	maxRefreshWindow             = 7 * 24 * time.Hour // 7 days
)

// readSessionTimeout reads SessionTimeout (minutes) from /etc/globular/config/config.json.
// Returns a sane default if missing, invalid, or zero.
func readSessionTimeout() (int, error) {
	paths := []string{
		filepath.Join(config.GetRuntimeConfigDir(), "config.json"),
		filepath.Join(config.GetConfigDir(), "config.json"),
	}
	var lastErr error
	for _, cfgPath := range paths {
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			lastErr = err
			continue
		}
		var globular map[string]interface{}
		if err := json.Unmarshal(data, &globular); err != nil {
			lastErr = fmt.Errorf("read session timeout: invalid json in %s: %w", cfgPath, err)
			continue
		}
		v := Utility.ToInt(globular["SessionTimeout"])
		if v <= 0 {
			return defaultSessionTimeoutMinutes, nil
		}
		return v, nil
	}
	if lastErr != nil {
		return defaultSessionTimeoutMinutes, fmt.Errorf("read session timeout: %w", lastErr)
	}
	return defaultSessionTimeoutMinutes, fmt.Errorf("read session timeout: config not found")
}

// ----------------------------------------------------------------------------
// Public API
// ----------------------------------------------------------------------------

// GenerateToken creates and signs a JWT for the given user.
// - timeout: expiry in minutes (fallback to config or default).
// - mac:     intended audience (peer MAC / service id). Optional; empty means “any cluster audience policy”.
// The token is always signed with the **issuer's** private key (asymmetric).
func GenerateToken(timeout int, mac, userId, userName, email, userDomain string) (string, error) {
	// Normalize/secure timeout
	if timeout <= 0 {
		if cfgTimeout, err := readSessionTimeout(); err == nil && cfgTimeout > 0 {
			timeout = cfgTimeout
		} else {
			timeout = defaultSessionTimeoutMinutes
		}
	}

	now := time.Now()
	expiresAt := now.Add(time.Duration(timeout) * time.Minute)

	issuer, err := config.GetMacAddress()
	if err != nil {
		return "", fmt.Errorf("generate token: get mac address: %w", err)
	}

	// Audience is optional; use it to scope tokens to a peer/service.
	audience := mac
	// If you prefer to scope to a logical API instead of peer MAC, set audience to that name here.

	domain, err := config.GetDomain()
	if err != nil {
		return "", fmt.Errorf("generate token: get domain: %w", err)
	}
	address, err := config.GetAddress()
	if err != nil {
		return "", fmt.Errorf("generate token: get address: %w", err)
	}

	claims := &Claims{
		ID:         userId,
		Username:   userName,
		UserDomain: userDomain,
		Email:      email,
		Domain:     domain,
		Address:    address,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        userId,
			Subject:   userId,
			Issuer:    issuer,
			Audience:  jwt.ClaimStrings{},
		},
	}
	if audience != "" {
		claims.Audience = jwt.ClaimStrings{audience}
	}

	// Load issuer private key + kid from your keystore
	if GetIssuerSigningKey == nil {
		return "", errors.New("generate token: GetIssuerSigningKey not configured")
	}
	priv, kid, err := GetIssuerSigningKey(issuer)
	if err != nil {
		return "", fmt.Errorf("generate token: get issuer signing key: %w", err)
	}

	// Build & sign token (EdDSA / Ed25519)
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	if kid != "" {
		token.Header["kid"] = kid
	}
	signed, err := token.SignedString(priv)
	if err != nil {
		return "", fmt.Errorf("generate token: sign: %w", err)
	}

	// Paranoid self-validate before returning
	if _, err := ValidateToken(signed); err != nil {
		logger.Error("generate token: self-validate failed", "err", err)
		return "", fmt.Errorf("generate token: self-validate failed: %w", err)
	}
	return signed, nil
}

// ValidateToken parses and validates a signed JWT string and returns its claims.
// Signature is verified using the **issuer's public key** looked up by iss + kid.
func ValidateToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}

	keyFunc := func(t *jwt.Token) (interface{}, error) {
		// Enforce EdDSA/Ed25519
		if t.Method != jwt.SigningMethodEdDSA {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}

		iss, _ := t.Claims.(*Claims)
		if iss == nil || iss.Issuer == "" {
			return nil, errors.New("validate token: missing issuer")
		}

		var kid string
		if k, ok := t.Header["kid"].(string); ok {
			kid = k
		}

		if GetPeerPublicKey == nil {
			return nil, errors.New("validate token: GetPeerPublicKey not configured")
		}
		pub, err := GetPeerPublicKey(iss.Issuer, kid)
		if err != nil {
			return nil, fmt.Errorf("validate token: get public key (iss=%s,kid=%s): %w", iss.Issuer, kid, err)
		}
		return pub, nil
	}

	parsed, err := jwt.ParseWithClaims(tokenStr, claims, keyFunc,
		jwt.WithLeeway(tokenExpirySkew),
		jwt.WithAudience(""), // we'll enforce aud at the service/router layer
	)
	if err != nil {
		return claims, fmt.Errorf("validate token: parse: %w", err)
	}
	if !parsed.Valid {
		return claims, errors.New("validate token: token signature invalid")
	}

	// Expiry check is already covered by jwt.ParseWithClaims + leeway.
	return claims, nil
}

// GetClientAddress extracts the client Address from the token in gRPC metadata.
func GetClientAddress(ctx context.Context) (string, error) {
	token := extractTokenFromContext(ctx)
	if token == "" {
		return "", errors.New("get client address: no token found in context metadata")
	}
	claims, err := ValidateToken(token)
	if err != nil {
		return "", fmt.Errorf("get client address: %w", err)
	}
	if claims.Address == "" {
		return "", errors.New("get client address: no address present in token claims")
	}
	return claims.Address, nil
}

// GetClientId returns "<id>@<userDomain>" and the raw token from gRPC metadata.
func GetClientId(ctx context.Context) (string, string, error) {
	token := extractTokenFromContext(ctx)
	if token == "" {
		return "", "", errors.New("get client id: no token found in context metadata")
	}
	claims, err := ValidateToken(token)
	if err != nil {
		return "", "", fmt.Errorf("get client id: %w", err)
	}
	if claims.UserDomain == "" {
		return "", "", errors.New("get client id: token missing user domain")
	}
	username := claims.ID + "@" + claims.UserDomain
	return username, token, nil
}

// SetLocalToken generates a token for the given identity and writes it
// to /etc/globular/config/tokens/<normalized_mac>_token, also caching it in memory.
func SetLocalToken(mac, domain, id, name, email string, timeout int) error {
	rawMAC := mac                       // keep raw MAC for cache key
	normMAC := normalizeMACForFile(mac) // for filesystem path

	path := tokenPathForMAC(rawMAC)
	_ = os.Remove(path) // best-effort cleanup

	tokenString, err := GenerateToken(timeout, rawMAC, id, name, email, domain)
	if err != nil {
		logger.Error("set local token: generate failed", "mac", rawMAC, "err", err)
		return fmt.Errorf("set local token: generate: %w", err)
	}

	// Ensure token directory exists
	if err := os.MkdirAll(tokenDir(), 0o755); err != nil {
		return fmt.Errorf("set local token: ensure token dir: %w", err)
	}

	// Write file with normalized filename (0600 because tokens are secrets)
	if err := os.WriteFile(filepath.Join(tokenDir(), normMAC+"_token"), []byte(tokenString), 0o600); err != nil {
		logger.Error("set local token: write file failed", "mac", rawMAC, "err", err)
		return fmt.Errorf("set local token: write file: %w", err)
	}

	// Cache using RAW MAC as key
	tokens.Store(rawMAC, tokenString)
	return nil
}

// GetLocalToken returns a valid local token for the given MAC, refreshing it
// when possible if it's expired (within 7 days).
func GetLocalToken(mac string) (string, error) {
	// 1) Try in-memory cache
	if v, ok := tokens.Load(mac); ok {
		if token, _ := v.(string); token != "" {
			if _, err := ValidateToken(token); err == nil {
				return token, nil
			}
		}
	}

	// 2) Try file
	token, err := readTokenFromFile(mac)
	if err != nil || token == "" {
		return "", fmt.Errorf("get local token: no token found for mac %s", mac)
	}

	// 3) Validate or refresh
	claims, vErr := ValidateToken(token)
	if vErr == nil {
		tokens.Store(mac, token)
		return token, nil
	}

	// If it's expired, allow refresh within 7 days grace based on original exp.
	if claims != nil && time.Until(claims.ExpiresAt.Time) > -maxRefreshWindow {
		newToken, rErr := refreshLocalToken(token)
		if rErr != nil {
			return "", fmt.Errorf("get local token: refresh failed: %w", rErr)
		}
		tokens.Store(mac, newToken)
		return newToken, nil
	}

	return "", errors.New("get local token: token expired beyond refresh window")
}

// ----------------------------------------------------------------------------
// Internal functions
// ----------------------------------------------------------------------------

// refreshLocalToken refreshes a (recently expired) local token using its original claims.
func refreshLocalToken(token string) (string, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		// Enforce EdDSA
		if t.Method != jwt.SigningMethodEdDSA {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		iss, _ := t.Claims.(*Claims)
		if iss == nil || iss.Issuer == "" {
			return nil, errors.New("refresh local token: missing issuer")
		}
		var kid string
		if k, ok := t.Header["kid"].(string); ok {
			kid = k
		}
		if GetPeerPublicKey == nil {
			return nil, errors.New("refresh local token: GetPeerPublicKey not configured")
		}
		return GetPeerPublicKey(iss.Issuer, kid)
	}, jwt.WithLeeway(tokenExpirySkew))
	// Ignore expiration errors here; we only need the claims to re-issue.

	if err != nil && !isOnlyExpiryError(err) {
		return "", fmt.Errorf("refresh local token: parse: %w", err)
	}

	timeout, err := readSessionTimeout()
	if err != nil || timeout <= 0 {
		timeout = defaultSessionTimeoutMinutes
	}

	// Preserve original audience (peer/service scope) on refresh.
	var aud string
	if len(claims.Audience) > 0 {
		aud = claims.Audience[0]
	}

	newToken, err := GenerateToken(timeout, aud, claims.ID, claims.Username, claims.Email, claims.UserDomain)
	if err != nil {
		return "", fmt.Errorf("refresh local token: generate: %w", err)
	}
	return newToken, nil
}

func readTokenFromFile(mac string) (string, error) {
	path := tokenPathForMAC(mac)
	// Backward compat: also try normalized mac in case path layout changed earlier
	data, err := os.ReadFile(path)
	if err != nil {
		alt := filepath.Join(tokenDir(), normalizeMACForFile(mac)+"_token")
		if data2, err2 := os.ReadFile(alt); err2 == nil {
			return string(data2), nil
		}
		return "", fmt.Errorf("read token: %w", err)
	}
	return string(data), nil
}

// extractTokenFromContext returns token from "token" or "authorization: Bearer <...>".
func extractTokenFromContext(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	// Preferred custom key
	if vals := md.Get("token"); len(vals) > 0 {
		return strings.TrimSpace(strings.Join(vals, ""))
	}
	// Fallback to Authorization: Bearer
	for _, v := range md.Get("authorization") {
		if s := strings.TrimSpace(v); strings.HasPrefix(strings.ToLower(s), "bearer ") {
			return strings.TrimSpace(s[len("bearer "):])
		}
	}
	return ""
}

// isOnlyExpiryError returns true if err indicates token expiration (and nothing else).
func isOnlyExpiryError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "token is expired") || strings.Contains(msg, "expired")
}
