// @awareness namespace=globular.platform
// @awareness component=platform_pki.san
// @awareness file_role=subject_alternative_name_construction_all_identities_required_vip_included
// @awareness implements=globular.platform:intent.pki.san_must_include_all_service_identities
// @awareness implements=globular.platform:intent.dns_pki.explicit_identity_over_convenient_routing
// @awareness risk=critical
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

// csrCoversAllSANs returns true if the PEM-encoded CSR at csrFile already
// contains every DNS name in dns and every IP in ips. Used to detect stale
// CSRs that were generated before the VIP or other SANs were added.
func csrCoversAllSANs(csrFile string, dns []string, ips []string) bool {
	blk, _, err := readPEMBlock(csrFile)
	if err != nil {
		return false
	}
	csr, err := x509.ParseCertificateRequest(blk.Bytes)
	if err != nil {
		return false
	}
	dnsSet := make(map[string]struct{}, len(csr.DNSNames))
	for _, d := range csr.DNSNames {
		dnsSet[strings.ToLower(d)] = struct{}{}
	}
	for _, required := range dns {
		if _, ok := dnsSet[strings.ToLower(required)]; !ok {
			return false
		}
	}
	ipSet := make(map[string]struct{}, len(csr.IPAddresses))
	for _, ip := range csr.IPAddresses {
		ipSet[ip.String()] = struct{}{}
	}
	for _, required := range ips {
		ip := net.ParseIP(strings.TrimSpace(required))
		if ip == nil {
			continue
		}
		if _, ok := ipSet[ip.String()]; !ok {
			return false
		}
	}
	return true
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
		// If the existing CSR is missing any required SANs (e.g. VIP added
		// after initial bootstrap), delete it and the signed cert so they are
		// regenerated below with the full SAN set.
		if !csrCoversAllSANs(cf, dns, ips) {
			_ = os.Remove(cf)
			_ = os.Remove(crtPath(dir, name))
		} else {
			return keyFile, cf, nil
		}
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
