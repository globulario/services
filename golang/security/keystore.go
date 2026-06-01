// @awareness namespace=globular.platform
// @awareness component=platform_security
// @awareness file_role=signing_key_store
// @awareness implements=globular.platform:intent.globular.security.ceremony_over_configuration
// @awareness risk=critical
package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

const (
	keyDirName = "keys"
	// PEM block types (use conventional ones so OpenSSL/Go can parse easily)
	pemTypePrivate = "PRIVATE KEY" // PKCS#8
	pemTypePublic  = "PUBLIC KEY"  // SubjectPublicKeyInfo
)

// Wire defaults at package init so callers don’t have to set the vars manually.
func init() {
	if GetIssuerSigningKey == nil {
		GetIssuerSigningKey = fileKeystoreGetIssuerSigningKey
	}
	if GetPeerPublicKey == nil {
		GetPeerPublicKey = fileKeystoreGetPeerPublicKey
	}
}

// ---------- Issuer signing-key cache ----------
// Prevents redundant filesystem scans and key rotation when the private-key
// file exists but the directory is very large (causing slow or fragile scans).

type cachedSigningKey struct {
	priv ed25519.PrivateKey
	kid  string
}

var (
	signingKeyCache   = make(map[string]cachedSigningKey)
	signingKeyCacheMu sync.RWMutex
)

// invalidateSigningKeyCache removes the cached key for issuer, forcing a fresh
// load from disk on the next call. Intended for testing and key rotation.
func invalidateSigningKeyCache(issuer string) {
	signingKeyCacheMu.Lock()
	delete(signingKeyCache, issuer)
	signingKeyCacheMu.Unlock()
}

// ---------- File keystore helpers ----------

func keyRoot() string {
	return config.GetKeysDir()
}

func legacyKeyRoot() string {
	return filepath.Join(config.GetConfigDir(), keyDirName)
}

func normID(id string) string { return strings.ReplaceAll(id, ":", "_") }

func kidFromPub(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	// base64url without padding, trim to 16 chars for brevity
	return base64.RawURLEncoding.EncodeToString(sum[:])[:16]
}

func readEd25519Private(path string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil || block.Type != pemTypePrivate {
		return nil, errors.New("invalid or missing PEM block for private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not ed25519")
	}
	return priv, nil
}

func readEd25519Public(path string) (ed25519.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseEd25519PublicPEM(b)
}

func parseEd25519PublicPEM(b []byte) (ed25519.PublicKey, error) {
	block, _ := pem.Decode(b)
	if block == nil || block.Type != pemTypePublic {
		return nil, errors.New("invalid or missing PEM block for public key")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("public key is not ed25519")
	}
	return pub, nil
}

func encodeEd25519PublicPEM(pub ed25519.PublicKey) ([]byte, error) {
	raw, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: pemTypePublic, Bytes: raw}), nil
}

func writeEd25519Private(path string, priv ed25519.PrivateKey) error {
	raw, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	if err := config.EnsureRuntimeDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: pemTypePrivate, Bytes: raw}), 0o600)
}

func writeEd25519Public(path string, pub ed25519.PublicKey) error {
	raw, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return err
	}
	if err := config.EnsureRuntimeDir(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: pemTypePublic, Bytes: raw}), 0o644)
}

// Try rotation-aware filename first (<issuer>_<kid>_*), then legacy (<issuer>_*).
func findExistingPrivate(issuer string) (ed25519.PrivateKey, string, error) {
	// First, check runtime keys.
	runtimePath := privateKeyPath(issuer, "")
	if Utility.Exists(runtimePath) {
		priv, err := readEd25519Private(runtimePath)
		if err != nil {
			return nil, "", err
		}
		kid := kidFromPub(priv.Public().(ed25519.PublicKey))
		return priv, kid, nil
	}
	// Scan runtime directory for rotated keys.
	if Utility.Exists(runtimeKeyRoot()) {
		entries, err := os.ReadDir(runtimeKeyRoot())
		if err == nil {
			prefix := normID(issuer) + "_"
			suffix := "_private"
			for _, e := range entries {
				name := e.Name()
				if !e.IsDir() && strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
					kid := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
					priv, err := readEd25519Private(filepath.Join(runtimeKeyRoot(), name))
					if err == nil {
						return priv, kid, nil
					}
				}
			}
		}
	}
	// Fallback to legacy config dir under /etc (read-only).
	legacyPath := filepath.Join(legacyKeyRoot(), fmt.Sprintf("%s_private", normID(issuer)))
	if Utility.Exists(legacyPath) {
		priv, err := readEd25519Private(legacyPath)
		if err != nil {
			return nil, "", err
		}
		kid := kidFromPub(priv.Public().(ed25519.PublicKey))
		return priv, kid, nil
	}
	return nil, "", os.ErrNotExist
}

// ---------- Hook implementations ----------

// fileKeystoreGetIssuerSigningKey returns (and lazily creates) the issuer's Ed25519 private key
// and a stable KID derived from the public key. Keys are stored under:
//
//	<configDir>/keys/<normalized_issuer>[_<kid>]_private|public
//
// The result is cached in memory after the first successful load to avoid
// redundant directory scans (large key directories) and unintended key rotation.
func fileKeystoreGetIssuerSigningKey(issuer string) (ed25519.PrivateKey, string, error) {
	if issuer == "" {
		return nil, "", errors.New("issuer is empty")
	}

	// Fast path: return cached key without touching the filesystem.
	signingKeyCacheMu.RLock()
	if entry, ok := signingKeyCache[issuer]; ok {
		signingKeyCacheMu.RUnlock()
		return entry.priv, entry.kid, nil
	}
	signingKeyCacheMu.RUnlock()

	signingKeyCacheMu.Lock()
	defer signingKeyCacheMu.Unlock()

	// Re-check under write lock to avoid a race.
	if entry, ok := signingKeyCache[issuer]; ok {
		return entry.priv, entry.kid, nil
	}

	priv, kid, err := loadOrGenerateSigningKey(issuer)
	if err != nil {
		return nil, "", err
	}
	signingKeyCache[issuer] = cachedSigningKey{priv: priv, kid: kid}
	return priv, kid, nil
}

// loadOrGenerateSigningKey performs the actual disk / generate logic for fileKeystoreGetIssuerSigningKey.
func loadOrGenerateSigningKey(issuer string) (ed25519.PrivateKey, string, error) {
	// 1) Try existing key (legacy or rotated)
	if priv, kid, err := findExistingPrivate(issuer); err == nil {
		pub := priv.Public().(ed25519.PublicKey)
		if enc, encErr := encodeEd25519PublicPEM(pub); encErr == nil {
			_ = publishPeerPublicKeyToCluster(issuer, kid, enc)
		}
		return priv, kid, nil
	}

	// 2) Generate new pair
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", fmt.Errorf("generate ed25519 key: %w", err)
	}
	kid := kidFromPub(pub)

	// Persist under rotation-aware filenames, and also keep legacy names for compatibility if desired.
	privPath := privateKeyPath(issuer, kid)
	pubPath := publicKeyPath(issuer, kid)
	if err := writeEd25519Private(privPath, priv); err != nil {
		return nil, "", fmt.Errorf("write private key: %w", err)
	}
	if err := writeEd25519Public(pubPath, pub); err != nil {
		return nil, "", fmt.Errorf("write public key: %w", err)
	}

	// Write/refresh legacy names too (optional but keeps older code working)
	legacyPriv := privateKeyPath(issuer, "")
	legacyPub := publicKeyPath(issuer, "")
	_ = writeEd25519Private(legacyPriv, priv)
	_ = writeEd25519Public(legacyPub, pub)
	if enc, encErr := encodeEd25519PublicPEM(pub); encErr == nil {
		_ = publishPeerPublicKeyToCluster(issuer, kid, enc)
	}

	return priv, kid, nil
}

// fileKeystoreGetPeerPublicKey loads a peer's Ed25519 public key.
// It will try a rotation-aware file first (<issuer>_<kid>_public) if kid is given,
// otherwise it falls back to the legacy (<issuer>_public) filename.
func fileKeystoreGetPeerPublicKey(issuer, kid string) (ed25519.PublicKey, error) {
	if issuer == "" {
		return nil, errors.New("issuer is empty")
	}

	// Prefer kid-aware path if provided
	if kid != "" {
		if pub, err := readEd25519Public(publicKeyPath(issuer, kid)); err == nil {
			// Verify the cached key actually belongs to this KID (prevents stale-cache signature failures).
			if kidFromPub(pub) == kid {
				return pub, nil
			}
			// Cached key fingerprint doesn't match — fall through to etcd.
		}
		if enc, err := fetchPeerPublicKeyFromCluster(issuer, kid); err == nil {
			if pub, parseErr := parseEd25519PublicPEM(enc); parseErr == nil {
				// Only cache under the KID-specific path when the fingerprint matches.
				if kidFromPub(pub) == kid {
					_ = writeEd25519Public(publicKeyPath(issuer, kid), pub)
				}
				_ = writeEd25519Public(publicKeyPath(issuer, ""), pub)
				return pub, nil
			}
		}
		// If kid-specific file not found, fall back to current public key
	}

	// Legacy/current key
	pub, err := readEd25519Public(publicKeyPath(issuer, ""))
	if err != nil {
		if enc, ferr := fetchPeerPublicKeyFromCluster(issuer, ""); ferr == nil {
			if p, parseErr := parseEd25519PublicPEM(enc); parseErr == nil {
				_ = writeEd25519Public(publicKeyPath(issuer, ""), p)
				return p, nil
			}
		}
		return nil, fmt.Errorf("load peer public key: %w", err)
	}
	return pub, nil
}
