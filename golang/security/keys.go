package security

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

// Keep the same directory naming convention as before.
var keyPath = filepath.Join(config.GetConfigDir(), "keys")

// normalize id (MAC) for filenames
func norm(id string) string { return strings.ReplaceAll(id, ":", "_") }

// -------- Public compatibility layer (names preserved) --------

// DeletePublicKey deletes the (legacy or rotated) public key file(s) for the given id.
func DeletePublicKey(id string) error {
	n := norm(id)

	// Legacy path
	legacy := filepath.Join(keyPath, n+"_public")
	if Utility.Exists(legacy) {
		if err := os.Remove(legacy); err != nil {
			return err
		}
	}

	// Also remove any rotation-aware public key files, e.g. <id>_<kid>_public
	if !Utility.Exists(keyPath) {
		return nil
	}
	entries, err := os.ReadDir(keyPath)
	if err != nil {
		return nil // best effort
	}
	prefix, suffix := n+"_", "_public"
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
			_ = os.Remove(filepath.Join(keyPath, name))
		}
	}
	return nil
}

// GeneratePeerKeys generates (or ensures) an Ed25519 keypair for the *local* peer id.
// For safety, we only generate a private key when id == local MAC.
func GeneratePeerKeys(id string) error {
	if id == "" {
		return errors.New("generate peer keys: empty id")
	}
	localMAC, err := config.GetMacAddress()
	if err != nil {
		return fmt.Errorf("generate peer keys: get mac: %w", err)
	}
	if norm(id) != norm(localMAC) {
		return fmt.Errorf("generate peer keys: refusing to create private key for non-local id %q", id)
	}
	// Will lazily create and persist if missing; also writes public key.
	_, _, err = fileKeystoreGetIssuerSigningKey(id)
	return err
}

var localKey []byte // in-memory cache of local public key bytes

// GetLocalKey returns the PEM-encoded public key bytes for the local peer (legacy-compatible).
func GetLocalKey() ([]byte, error) {
	if len(localKey) > 0 {
		return localKey, nil
	}
	mac, err := config.GetMacAddress()
	if err != nil {
		return nil, fmt.Errorf("get local key: get mac: %w", err)
	}
	pubPath := publicKeyPath(mac, "") // legacy path: <id>_public
	if !Utility.Exists(pubPath) {
		// Ensure a keypair exists (first run)
		if _, _, err := fileKeystoreGetIssuerSigningKey(mac); err != nil {
			return nil, fmt.Errorf("get local key: ensure keypair: %w", err)
		}
	}
	b, err := os.ReadFile(pubPath)
	if err != nil {
		return nil, fmt.Errorf("get local key: read %s: %w", pubPath, err)
	}
	localKey = b
	return localKey, nil
}

// GetPeerKey returns a peer "key" as []byte to preserve the old signature.
// In the old ECDSA code, this returned an ECDH shared secret. With Ed25519
// (sign-only), we return the peer's PEM-encoded public key bytes instead.
// If you previously relied on the ECDH secret, migrate to X25519/Noise for KEX.
func GetPeerKey(id string) ([]byte, error) {
	if id == "" {
		return nil, errors.New("get peer key: empty id")
	}
	n := norm(id)

	// Local short-circuit
	mac, err := config.GetMacAddress()
	if err != nil {
		return nil, fmt.Errorf("get peer key: get mac: %w", err)
	}
	if n == norm(mac) {
		return GetLocalKey()
	}

	// Try kid-aware and legacy public key files
	// 1) Legacy path first (most common)
	pubPath := publicKeyPath(id, "")
	if Utility.Exists(pubPath) {
		return os.ReadFile(pubPath)
	}

	// 2) If rotated, return any current rotated public key (first match)
	if Utility.Exists(keyPath) {
		entries, err := os.ReadDir(keyPath)
		if err == nil {
			prefix, suffix := n+"_", "_public"
			for _, e := range entries {
				name := e.Name()
				if !e.IsDir() && strings.HasPrefix(name, prefix) && strings.HasSuffix(name, suffix) {
					return os.ReadFile(filepath.Join(keyPath, name))
				}
			}
		}
	}

	return nil, fmt.Errorf("get peer key: no public key found for %s", id)
}

// SetPeerPublicKey writes a peer public key to <configDir>/keys/<id>_public.
// Accepts PEM content in encPub (unchanged from previous behavior).
func SetPeerPublicKey(id, encPub string) error {
	if id == "" {
		return errors.New("set peer public key: empty id")
	}
	path := publicKeyPath(id, "") // legacy filename
	if err := Utility.CreateDirIfNotExist(filepath.Dir(path)); err != nil {
		return fmt.Errorf("set peer public key: ensure dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(encPub), 0o644); err != nil {
		return fmt.Errorf("set peer public key: write %s: %w", path, err)
	}
	return nil
}

// -------- Minimal helpers reused from keystore.go --------

func privateKeyPath(issuer, kid string) string {
	n := normID(issuer)
	if kid != "" {
		return filepath.Join(keyRoot(), fmt.Sprintf("%s_%s_private", n, kid))
	}
	// legacy single-key filename
	return filepath.Join(keyRoot(), fmt.Sprintf("%s_private", n))
}

func publicKeyPath(issuer, kid string) string {
	n := normID(issuer)
	if kid != "" {
		return filepath.Join(keyRoot(), fmt.Sprintf("%s_%s_public", n, kid))
	}
	// legacy single-key filename
	return filepath.Join(keyRoot(), fmt.Sprintf("%s_public", n))
}

// fileKeystoreGetIssuerSigningKey is declared in keystore.go; repeated signature here for clarity.
// func fileKeystoreGetIssuerSigningKey(issuer string) (ed25519.PrivateKey, string, error)

// fileKeystoreGetPeerPublicKey exists in keystore.go; not directly used here,
// but ValidateToken will call it through GetPeerPublicKey var.
var _ ed25519.PrivateKey
