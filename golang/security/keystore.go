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

// ---------- File keystore helpers ----------

func keyRoot() string {
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

func writeEd25519Private(path string, priv ed25519.PrivateKey) error {
	raw, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	if err := Utility.CreateDirIfNotExist(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: pemTypePrivate, Bytes: raw}), 0o600)
}

func writeEd25519Public(path string, pub ed25519.PublicKey) error {
	raw, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return err
	}
	if err := Utility.CreateDirIfNotExist(filepath.Dir(path)); err != nil {
		return err
	}
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: pemTypePublic, Bytes: raw}), 0o644)
}

// Try rotation-aware filename first (<issuer>_<kid>_*), then legacy (<issuer>_*).
func findExistingPrivate(issuer string) (ed25519.PrivateKey, string, error) {
	// Probe legacy first to keep backward compatibility.
	legacy := privateKeyPath(issuer, "")
	if Utility.Exists(legacy) {
		priv, err := readEd25519Private(legacy)
		if err != nil {
			return nil, "", err
		}
		kid := kidFromPub(priv.Public().(ed25519.PublicKey))
		return priv, kid, nil
	}
	// If you later store rotated keys, you can scan the dir for "<issuer>_*_private".
	entries, err := os.ReadDir(keyRoot())
	if err == nil {
		prefix := normID(issuer) + "_"
		suffix := "_private"
        // find first match
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() && strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
				kid := strings.TrimSuffix(strings.TrimPrefix(name, prefix), suffix)
				priv, err := readEd25519Private(filepath.Join(keyRoot(), name))
				if err == nil {
					return priv, kid, nil
				}
			}
		}
	}
	return nil, "", os.ErrNotExist
}

// ---------- Hook implementations ----------

// fileKeystoreGetIssuerSigningKey returns (and lazily creates) the issuer's Ed25519 private key
// and a stable KID derived from the public key. Keys are stored under:
//   <configDir>/keys/<normalized_issuer>[_<kid>]_private|public
func fileKeystoreGetIssuerSigningKey(issuer string) (ed25519.PrivateKey, string, error) {
	if issuer == "" {
		return nil, "", errors.New("issuer is empty")
	}

	// 1) Try existing key (legacy or rotated)
	if priv, kid, err := findExistingPrivate(issuer); err == nil {
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
			return pub, nil
		}
		// If kid-specific file not found, fall back to current public key
	}

	// Legacy/current key
	pub, err := readEd25519Public(publicKeyPath(issuer, ""))
	if err != nil {
		return nil, fmt.Errorf("load peer public key: %w", err)
	}
	return pub, nil
}

