package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/metadata"
)

// ----------------------------------------------------------------------------
// Types
// ----------------------------------------------------------------------------

// Authentication holds a bearer token for gRPC per-RPC credentials.
type Authentication struct {
	Token string
}

// GetRequestMetadata returns the outgoing metadata containing the JWT token.
func (a *Authentication) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"token": a.Token}, nil
}

// RequireTransportSecurity indicates whether the credentials require TLS.
func (a *Authentication) RequireTransportSecurity() bool { return true }

// Claims is the signed JWT payload. It embeds jwt.StandardClaims.
type Claims struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Domain     string `json:"domain"` // Where the token was generated
	UserDomain string `json:"user_domain"`
	Address    string `json:"address"`
	jwt.StandardClaims
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

func tokenDir() string { return filepath.Join(config.GetConfigDir(), "tokens") }

func tokenPathForMAC(mac string) string {
	return filepath.Join(tokenDir(), normalizeMACForFile(mac)+"_token")
}

// readSessionTimeout reads SessionTimeout (minutes) from /etc/globular/config/config.json.
func readSessionTimeout() (int, error) {
	cfgPath := filepath.Join(config.GetConfigDir(), "config.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return 0, fmt.Errorf("read session timeout: cannot read %s: %w", cfgPath, err)
	}

	var globular map[string]interface{}
	if err := json.Unmarshal(data, &globular); err != nil {
		return 0, fmt.Errorf("read session timeout: invalid json in %s: %w", cfgPath, err)
	}

	return Utility.ToInt(globular["SessionTimeout"]), nil
}

// resolveJWTKey selects the proper HMAC key based on issuer/audience.
func resolveJWTKey(claims *Claims) ([]byte, error) {
	macAddress, err := config.GetMacAddress()
	if err != nil {
		return nil, fmt.Errorf("resolve jwt key: get mac address: %w", err)
	}

	// Prefer audience key when set and different from local mac.
	if aud := claims.Audience; len(aud) > 0 && aud != macAddress {
		key, err := GetPeerKey(aud)
		if err != nil {
			return nil, fmt.Errorf("resolve jwt key: get peer key for audience %q: %w", aud, err)
		}
		return key, nil
	}

	key, err := GetPeerKey(claims.Issuer)
	if err != nil {
		return nil, fmt.Errorf("resolve jwt key: get peer key for issuer %q: %w", claims.Issuer, err)
	}
	return key, nil
}

// ----------------------------------------------------------------------------
// Public API
// ----------------------------------------------------------------------------

// GenerateToken creates and signs a JWT for the given user.
// The token expires after 'timeout' minutes. 'mac' indicates the intended
// audience peer (optional). The signing key is chosen by issuer/audience.
func GenerateToken(timeout int, mac, userId, userName, email, userDomain string) (string, error) {
	now := time.Now()
	expiresAt := now.Add(time.Duration(timeout) * time.Minute)

	issuer, err := config.GetMacAddress()
	if err != nil {
		return "", fmt.Errorf("generate token: get mac address: %w", err)
	}

	audience := ""
	if mac != "" && mac != issuer {
		audience = mac
	}

	var jwtKey []byte
	if audience != "" {
		jwtKey, err = GetPeerKey(audience)
		if err != nil {
			return "", fmt.Errorf("generate token: get peer key for audience %q: %w", audience, err)
		}
	} else {
		jwtKey, err = GetPeerKey(issuer)
		if err != nil {
			return "", fmt.Errorf("generate token: get peer key for issuer %q: %w", issuer, err)
		}
	}

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
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expiresAt.Unix(),
			IssuedAt:  now.Unix(),
			Id:        userId,
			Subject:   userId,
			Issuer:    issuer,
			Audience:  audience,
		},
	}

	// Build & sign token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(jwtKey)
	if err != nil {
		return "", fmt.Errorf("generate token: sign: %w", err)
	}

	// Validate before returning (paranoid sanity check)
	if _, err := ValidateToken(signed); err != nil {
		logger.Error("generate token: self-validate failed", "err", err)
		return "", fmt.Errorf("generate token: self-validate failed: %w", err)
	}

	return signed, nil
}

// ValidateToken parses and validates a signed JWT string and returns its claims.
func ValidateToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}

	tkn, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		// Enforce HMAC signing method
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return resolveJWTKey(claims)
	})
	if err != nil {
		return claims, fmt.Errorf("validate token: parse: %w", err)
	}
	if !tkn.Valid {
		return claims, errors.New("validate token: token signature invalid")
	}
	if time.Now().After(time.Unix(claims.ExpiresAt, 0)) {
		return claims, fmt.Errorf("validate token: token expired at %s", time.Unix(claims.ExpiresAt, 0).Format(time.RFC3339))
	}
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
	username := claims.Id + "@" + claims.UserDomain
	return username, token, nil
}

// SetLocalToken generates a token for the given identity and writes it
// to /etc/globular/config/tokens/<normalized_mac>_token, also caching it in memory.
func SetLocalToken(mac, domain, id, name, email string, timeout int) error {
	rawMAC := mac                     // keep raw MAC for cache key
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

	// Write file with normalized filename
	if err := os.WriteFile(filepath.Join(tokenDir(), normMAC+"_token"), []byte(tokenString), 0o644); err != nil {
		logger.Error("set local token: write file failed", "mac", rawMAC, "err", err)
		return fmt.Errorf("set local token: write file: %w", err)
	}

	// Cache using RAW MAC as key
	tokens.Store(rawMAC, tokenString)
	return nil
}

// GetLocalToken returns a valid local token for the given MAC, refreshing it
// when possible if it's expired (within 7 days). It reads from memory first,
// then from the standard token file if needed.
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

	// If it's expired, allow refresh within 7 days grace.
	if claims != nil && time.Unix(claims.ExpiresAt, 0).After(time.Now().AddDate(0, 0, -7)) {
		newToken, rErr := refreshLocalToken(token)
		if rErr != nil {
			return "", fmt.Errorf("get local token: refresh failed: %w", rErr)
		}
		tokens.Store(mac, newToken)
		return newToken, nil
	}

	return "", errors.New("get local token: token expired beyond 7-day refresh window")
}

// ----------------------------------------------------------------------------
// Internal functions (unchanged visibility)
// ----------------------------------------------------------------------------

// refreshLocalToken refreshes a (recently expired) local token using its original claims.
func refreshLocalToken(token string) (string, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		// Validate signing method
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return resolveJWTKey(claims)
	})
	if err != nil && !strings.Contains(err.Error(), "token is expired") {
		return "", fmt.Errorf("refresh local token: parse: %w", err)
	}

	timeout, err := readSessionTimeout()
	if err != nil {
		return "", fmt.Errorf("refresh local token: %w", err)
	}

	newToken, err := GenerateToken(timeout, claims.StandardClaims.Issuer, claims.Id, claims.Username, claims.Email, claims.UserDomain)
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
