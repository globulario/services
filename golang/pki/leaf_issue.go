package pki

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

// signLeafFromCSRLocalCA reads <name>.csr in dir and writes <name>.crt signed by local CA.
func (m *FileManager) signLeafFromCSRLocalCA(dir, name string, eku []x509.ExtKeyUsage, days int) (string, error) {
	caKey, caCrt, err := m.ensureOrLoadLocalCA(dir, name, 3650) // CA 10 years
	if err != nil {
		return "", err
	}
	caBlk, _, err := readPEMBlock(caCrt)
	if err != nil {
		return "", err
	}
	caCert, err := x509.ParseCertificate(caBlk.Bytes)
	if err != nil {
		return "", err
	}
	keyBlk, _, err := readPEMBlock(caKey)
	if err != nil {
		return "", err
	}
	caSigner, err := parseAnyPrivateKey(keyBlk)
	if err != nil {
		return "", err
	}
	csrBlk, _, err := readPEMBlock(csrPath(dir, name))
	if err != nil {
		return "", err
	}
	if csrBlk.Type != "CERTIFICATE REQUEST" && csrBlk.Type != "NEW CERTIFICATE REQUEST" {
		return "", fmt.Errorf("invalid CSR type %q", csrBlk.Type)
	}
	csr, err := x509.ParseCertificateRequest(csrBlk.Bytes)
	if err != nil {
		return "", err
	}
	if err := csr.CheckSignature(); err != nil {
		return "", err
	}

	now := time.Now()
	tpl := &x509.Certificate{
		SerialNumber:          serial128(),
		Subject:               csr.Subject,
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(0, 0, days),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           eku,
		BasicConstraintsValid: true,
		DNSNames:              csr.DNSNames,
		IPAddresses:           csr.IPAddresses,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, caCert, csr.PublicKey, caSigner)
	if err != nil {
		return "", err
	}
	leaf := crtPath(dir, name)
	if err := os.WriteFile(leaf, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o444); err != nil {
		return "", err
	}
	return leaf, nil
}

func serial128() *big.Int {
	n, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	return n
}
