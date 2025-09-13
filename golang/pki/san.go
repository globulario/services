package pki

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strings"
)

func keyPath(dir, name string) string { return filepath.Join(dir, name+".key") }
func csrPath(dir, name string) string { return filepath.Join(dir, name+".csr") }
func crtPath(dir, name string) string { return filepath.Join(dir, name+".crt") }

// ensureDir creates the directory if it does not exist.
func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// ensurePKCS8Key creates <name>.key if missing and returns its signer.
func ensurePKCS8Key(dir, name string) (string, crypto.Signer, error) {
	kf := keyPath(dir, name)
	if exists(kf) {
		blk, _, err := readPEMBlock(kf)
		if err != nil {
			return "", nil, err
		}
		signer, err := parseAnyPrivateKey(blk)
		return kf, signer, err
	}
	if err := ensureDir(dir); err != nil {
		return "", nil, err
	}
	_, pkcs8, err := genECDSAKeyPKCS8()
	if err != nil {
		return "", nil, err
	}
	if err := writePEMFile(kf, &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400); err != nil {
		return "", nil, err
	}
	blk, _, err := readPEMBlock(kf)
	if err != nil {
		return "", nil, err
	}
	signer, err := parseAnyPrivateKey(blk)
	return kf, signer, err
}

// ensureKeyAndCSRWithSANs writes <name>.csr for subject and SANs.
func ensureKeyAndCSRWithSANs(dir, name, subjectCN string, dns []string, ips []string) (keyFile, csrFile string, err error) {
	keyFile, signer, err := ensurePKCS8Key(dir, name)
	if err != nil {
		return "", "", err
	}

	// --- Legacy alias for consumers expecting *.pem (e.g., etcd expects server.pem)
	pemAlias := filepath.Join(dir, name+".pem")
	if !exists(pemAlias) {
		if b, readErr := os.ReadFile(keyFile); readErr == nil {
			_ = os.WriteFile(pemAlias, b, 0o400)
		}
	}

	cf := csrPath(dir, name)
	if exists(cf) {
		return keyFile, cf, nil
	}

	var ipList []net.IP
	for _, s := range ips {
		ip := net.ParseIP(strings.TrimSpace(s))
		if ip != nil {
			ipList = append(ipList, ip)
		}
	}

	tpl := &x509.CertificateRequest{
		Subject:     pkix.Name{CommonName: subjectCN},
		DNSNames:    dns,
		IPAddresses: ipList,
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, tpl, signer)
	if err != nil {
		return "", "", err
	}
	if err := writePEMFile(cf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}, 0o444); err != nil {
		return "", "", err
	}
	return keyFile, cf, nil
}

// Exported helper used by Globule bootstrap to keep legacy behavior.
func EnsureServerKeyAndCSR(dir, subject string, country, state, city, org string, dns []string) error {
	_, _, err := ensureKeyAndCSRWithSANs(dir, "server", subject, dns, nil)
	return err
}
