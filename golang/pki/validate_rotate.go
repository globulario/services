// @awareness namespace=globular.platform
// @awareness component=platform_pki.validate_rotate
// @awareness file_role=proactive_cert_rotation_before_expiry_reactive_is_forbidden
// @awareness implements=globular.platform:intent.pki.leaf_cert_rotation_is_proactive_not_reactive
// @awareness risk=critical
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

// RotateIfExpiring checks whether a leaf cert is expiring within renewBefore.
// It returns (true, nil) if rotation is needed, (false, nil) otherwise.
//
// IMPORTANT: this function does NOT perform rotation — it only signals that
// the cert is within the renewal window. The caller MUST call the appropriate
// Ensure* method (EnsureServerCert, EnsureClientCert, etc.) when this returns
// true. The boolean return means "rotation is needed", not "rotation was performed".
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
	// Cert is within the renewal window. Return (true, nil) to signal that
	// the caller should call the appropriate Ensure* method to renew it.
	// We return false here NOT because rotation was attempted, but because
	// we cannot perform it without knowing which cert profile was used.
	return true, nil
}

