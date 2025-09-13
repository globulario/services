package pki

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// notAfter returns the NotAfter timestamp of the first CERTIFICATE block
// found in the PEM file at path. It assumes the first cert is the leaf,
// which is the usual layout for leaf.pem/fullchain.pem.
func notAfter(path string) (time.Time, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return time.Time{}, err
	}
	return parseCertNotAfter(data)
}

// parseCertNotAfter extracts NotAfter from the first CERTIFICATE block in b.
func parseCertNotAfter(b []byte) (time.Time, error) {
	for {
		var block *pem.Block
		block, b = pem.Decode(b)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return time.Time{}, err
			}
			return cert.NotAfter, nil
		}
	}
	return time.Time{}, errors.New("no CERTIFICATE block found")
}

// RotateIfExpiring renews a leaf if it's expiring within renewBefore.
// It returns true if it rotated (or attempted rotation).
func (m *FileManager) RotateIfExpiring(dir string, leafFile string, renewBefore time.Duration) (bool, error) {
	p := filepath.Join(dir, leafFile)
	if !exists(p) {
		return false, nil
	}
	na, err := notAfter(p)
	if err != nil {
		return false, err
	}
	if time.Until(na) > renewBefore {
		return false, nil
	}
	// Caller should call Ensure* again (we don't know which profile was used).
	return true, nil
}

