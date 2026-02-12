package domain

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-acme/lego/v4/registration"
)

// persistedAccount represents an ACME account saved to disk.
// This includes both the private key and registration state to prevent
// re-registration loops on restart.
type persistedAccount struct {
	Email        string                  `json:"email"`
	Registration *registration.Resource `json:"registration,omitempty"`
	Key          []byte                  `json:"key"` // PEM-encoded private key
}

// acmeUser implements lego's registration.User interface.
type acmeUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string {
	return u.Email
}

func (u *acmeUser) GetRegistration() *registration.Resource {
	return u.Registration
}

func (u *acmeUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// loadAccount loads an ACME account from disk.
// Returns the account if found, nil if not found (need to create), or error on failure.
func loadAccount(email string, dir string) (*acmeUser, error) {
	accountFile := filepath.Join(dir, "account.json")

	// Check if account file exists
	data, err := os.ReadFile(accountFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Account doesn't exist yet
		}
		return nil, fmt.Errorf("failed to read account file: %w", err)
	}

	// Parse account JSON
	var persisted persistedAccount
	if err := json.Unmarshal(data, &persisted); err != nil {
		return nil, fmt.Errorf("failed to unmarshal account: %w", err)
	}

	// Verify email matches
	if persisted.Email != email {
		return nil, fmt.Errorf("account email mismatch: expected %q, got %q", email, persisted.Email)
	}

	// Parse private key
	block, _ := pem.Decode(persisted.Key)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from account key")
	}

	var privateKey crypto.PrivateKey
	switch block.Type {
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse EC private key: %w", err)
		}
		privateKey = key

	case "PRIVATE KEY": // PKCS#8
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
		}
		privateKey = key

	default:
		return nil, fmt.Errorf("unsupported private key type: %s", block.Type)
	}

	return &acmeUser{
		Email:        persisted.Email,
		Registration: persisted.Registration,
		key:          privateKey,
	}, nil
}

// saveAccount saves an ACME account to disk.
// This persists both the private key and registration state to prevent
// re-registration on restart.
func saveAccount(user *acmeUser, dir string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create account directory: %w", err)
	}

	// Encode private key to PEM
	var keyPEM []byte
	switch key := user.key.(type) {
	case *ecdsa.PrivateKey:
		keyBytes, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			return fmt.Errorf("failed to marshal EC private key: %w", err)
		}
		keyPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "EC PRIVATE KEY",
			Bytes: keyBytes,
		})

	default:
		// Use PKCS#8 for other key types
		keyBytes, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return fmt.Errorf("failed to marshal private key: %w", err)
		}
		keyPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: keyBytes,
		})
	}

	// Create persisted account structure
	persisted := persistedAccount{
		Email:        user.Email,
		Registration: user.Registration,
		Key:          keyPEM,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal account: %w", err)
	}

	// Write to file atomically (temp + rename)
	accountFile := filepath.Join(dir, "account.json")
	tempFile := accountFile + ".tmp"

	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write account file: %w", err)
	}

	if err := os.Rename(tempFile, accountFile); err != nil {
		os.Remove(tempFile) // Clean up temp file
		return fmt.Errorf("failed to rename account file: %w", err)
	}

	return nil
}
