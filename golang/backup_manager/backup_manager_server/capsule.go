package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// CapsuleContext provides each provider with paths inside the backup capsule.
type CapsuleContext struct {
	BackupID   string
	CapsuleDir string // DataDir/artifacts/<backup_id>
	ProviderDir string // CapsuleDir/provider/<name>
	PayloadDir  string // CapsuleDir/payload/<name>
}

// CapsuleDir returns the root capsule directory for a backup.
func (srv *server) CapsuleDir(backupID string) string {
	return filepath.Join(srv.DataDir, "artifacts", backupID)
}

// CapsuleManifestPath returns the manifest.json path inside a capsule.
func (srv *server) CapsuleManifestPath(backupID string) string {
	return filepath.Join(srv.CapsuleDir(backupID), "manifest.json")
}

// NewCapsuleContext creates a CapsuleContext for a provider and ensures dirs exist.
func (srv *server) NewCapsuleContext(backupID, providerName string) (*CapsuleContext, error) {
	capsuleDir := srv.CapsuleDir(backupID)
	providerDir := filepath.Join(capsuleDir, "provider", providerName)
	payloadDir := filepath.Join(capsuleDir, "payload", providerName)

	for _, d := range []string{providerDir, payloadDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("create capsule dir %s: %w", d, err)
		}
	}

	return &CapsuleContext{
		BackupID:    backupID,
		CapsuleDir:  capsuleDir,
		ProviderDir: providerDir,
		PayloadDir:  payloadDir,
	}, nil
}

// EnsureCapsuleDir creates the capsule root directory.
func (srv *server) EnsureCapsuleDir(backupID string) error {
	return os.MkdirAll(srv.CapsuleDir(backupID), 0755)
}

// CapsuleWriteFile writes data atomically to a path relative to the capsule.
func CapsuleWriteFile(baseDir, relPath string, data []byte) error {
	abs := filepath.Join(baseDir, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return err
	}
	tmp := abs + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, abs)
}

// CapsuleChecksumFile computes sha256 of a file and writes it to path+".sha256".
func CapsuleChecksumFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(data))
	return checksum, os.WriteFile(path+".sha256", []byte(checksum+"\n"), 0644)
}
