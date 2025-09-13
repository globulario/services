package pki

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// helper: ceil duration to whole days, with a minimum
func ceilDays(d time.Duration, minDays int) int {
	if d <= 0 {
		return minDays
	}
	// Ceil to days
	days := int((d + 24*time.Hour - 1) / (24 * time.Hour))
	if days < minDays {
		days = minDays
	}
	return days
}

// EnsurePeerCert -> DNS + IP SANs, EKU server+client (for etcd peers).
func (m *FileManager) EnsurePeerCert(dir string, subject string, dns []string, ips []string, ttl time.Duration) (key, crt, ca string, err error) {
	if ttl <= 0 {
		ttl = time.Duration(m.LocalCA.ValidDays) * 24 * time.Hour
	}

	days := ceilDays(ttl, 365) // enforce ≥ 1 year

	if _, _, err = ensureKeyAndCSRWithSANs(dir, "peer", subject, dns, ips); err != nil {
		return "", "", "", err
	}
	eku := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	leaf, err := m.signLeafFromCSRLocalCA(dir, "peer", eku, days)
	if err != nil {
		return "", "", "", err
	}
	return keyPath(dir, "peer"), leaf, caCrtPath(dir), nil
}

// parseSubjectDN parses a DN string like "CN=example.com,O=Org,C=US"
// into a pkix.Name struct.
func parseSubjectDN(dn string) pkix.Name {
    name := pkix.Name{}
    parts := strings.Split(dn, ",")
    for _, part := range parts {
        kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
        if len(kv) != 2 {
            continue
        }
        key := strings.ToUpper(strings.TrimSpace(kv[0]))
        val := strings.TrimSpace(kv[1])
        switch key {
        case "CN":
            name.CommonName = val
        case "C":
            name.Country = []string{val}
        case "O":
            name.Organization = []string{val}
        case "OU":
            name.OrganizationalUnit = []string{val}
        case "L":
            name.Locality = []string{val}
        case "ST":
            name.Province = []string{val}
        }
    }
    return name
}

// ensureCSRUsingKey ensures <base>.csr exists for the given key file.
// It never overwrites the key; it just generates (or refreshes) the CSR.
func ensureCSRUsingKey(dir, base, subject string, sans []string, keyFile string) (csrPathOut string, err error) {
    csr := csrPath(dir, base)
    // Optionally: if CSR exists, reuse it
    if exists(csr) {
        return csr, nil
    }

    // Load existing private key (RSA or EC) from keyFile (e.g., server.pem)
    blk, _, err := readPEMBlock(keyFile)
    if err != nil {
        return "", fmt.Errorf("read key %s: %w", keyFile, err)
    }
    pk, err := parseAnyPrivateKey(blk)
    if err != nil {
        return "", fmt.Errorf("parse key %s: %w", keyFile, err)
    }

    // Build CSR template with subject and SANs
    tmpl := &x509.CertificateRequest{
        Subject:  parseSubjectDN(subject), // implement or keep your existing subject builder
        DNSNames: sans,
    }
    der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, pk)
    if err != nil {
        return "", fmt.Errorf("create CSR: %w", err)
    }

    // Write <base>.csr atomically
    if err := writePEMFile(csr, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}, 0o444); err != nil {
        return "", fmt.Errorf("write csr: %w", err)
    }
    return csr, nil
}

// EnsurePublicACMECert issues a Let's Encrypt cert to separate files:
//   <base>.key, <base>.csr, <base>.crt, <base>.issuer.crt, <base>.fullchain.pem  (+ fullchain.pem alias)
// It leaves server.key/server.crt for internal mTLS (local CA) untouched.
// EnsurePublicACMECert issues a LE cert to <base>.crt/.issuer.crt/.fullchain.pem
// using the EXISTING server key (server.pem) for the CSR.
func (m *FileManager) EnsurePublicACMECert(
    dir, base, subject string, dns []string, ttl time.Duration,
) (keyFile, leafFile, issuerFile, fullchainFile string, err error) {

    // Use the existing local mTLS key for the CSR (e.g., server.pem)
    serverKey := filepath.Join(dir, "server.pem")
    if !exists(serverKey) {
        return "", "", "", "", fmt.Errorf("server key not found: %s", serverKey)
    }

    if _, err := ensureCSRUsingKey(dir, base, subject, dns, serverKey); err != nil {
        return "", "", "", "", err
    }

    if !m.ACME.Enabled {
        return "", "", "", "", fmt.Errorf("ACME is disabled; enable ACME to issue public certs")
    }

    // Obtain public leaf/issuer with the CSR you just made
    leaf, issuer, err := m.acmeObtainOrRenewCSRNamed(dir, base)
    if err != nil {
        return "", "", "", "", err
    }

    fullchain := filepath.Join(dir, base+".fullchain.pem")
    if err := writeConcat(fullchain, []string{leaf, issuer}, 0o444); err != nil {
        return "", "", "", "", fmt.Errorf("build fullchain: %w", err)
    }

    // Also write plain alias if your HTTPS bootstrap expects it
    alias := filepath.Join(dir, "fullchain.pem")
    _ = atomicWriteFile(alias, 0o444, func(tmp string) error {
        b, rerr := os.ReadFile(fullchain)
        if rerr != nil { return rerr }
        return os.WriteFile(filepath.Join(tmp, "fullchain.pem"), b, 0o444)
    })

    // Return the *server* key as the matching key for HTTPS
    return serverKey, leaf, issuer, fullchain, nil
}


func (m *FileManager) EnsureServerCert(dir string, subject string, dns []string, ttl time.Duration) (key, crt, ca string, err error) {
	if ttl <= 0 {
		ttl = time.Duration(m.LocalCA.ValidDays) * 24 * time.Hour
	}
	if _, _, err = ensureKeyAndCSRWithSANs(dir, "server", subject, dns, nil); err != nil {
		return "", "", "", err
	}

	// Local CA path only (internal mTLS)
	days := ceilDays(ttl, 365) // enforce ≥ 1 year
	eku := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}

	leaf, err := m.signLeafFromCSRLocalCA(dir, "server", eku, days)
	if err != nil {
		return "", "", "", err
	}
	return keyPath(dir, "server"), leaf, caCrtPath(dir), nil
}

// EnsureClientCert -> client-auth EKU.
func (m *FileManager) EnsureClientCert(dir string, subject string, dns []string, ttl time.Duration) (key, crt, ca string, err error) {
	if ttl <= 0 {
		ttl = time.Duration(m.LocalCA.ValidDays) * 24 * time.Hour
	}
	if _, _, err = ensureKeyAndCSRWithSANs(dir, "client", subject, dns, nil); err != nil {
		return "", "", "", err
	}
	
	days := ceilDays(ttl, 365) // enforce ≥ 1 year
	eku := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	leaf, err := m.signLeafFromCSRLocalCA(dir, "client", eku, days)
	if err != nil {
		return "", "", "", err
	}
	return keyPath(dir, "client"), leaf, caCrtPath(dir), nil
}

// Convenience: typical filenames used by stacks (server.*)
func ServerFiles(dir string) (key, crt, ca string) {
	return filepath.Join(dir, "server.key"), filepath.Join(dir, "server.crt"), filepath.Join(dir, "ca.crt")
}
