package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"crypto/elliptic"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// CA file paths
func caKeyPath(dir string) string { return filepath.Join(dir, "ca.key") }
func caCrtPath(dir string) string { return filepath.Join(dir, "ca.crt") }

// ensureOrLoadLocalCA makes sure ca.key / ca.crt exist.
func (m *FileManager) ensureOrLoadLocalCA(dir, subjectCN string, days int) (keyFile, crtFile string, err error) {
	if err = ensureDir(dir); err != nil {
		return "", "", err
	}

	kf, cf := caKeyPath(dir), caCrtPath(dir)
	if exists(kf) && exists(cf) {
		return kf, cf, nil
	}

	// Generate CA private key (PKCS#8 ECDSA P-256)
	_, pkcs8, err := genECDSAKeyPKCS8()
	if err != nil {
		return "", "", err
	}
	if err := writePEMFile(kf, &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400); err != nil {
		return "", "", err
	}

	// Self-signed CA cert
	keyBlk, _, err := readPEMBlock(kf)
	if err != nil {
		return "", "", err
	}
	signer, err := parseAnyPrivateKey(keyBlk)
	if err != nil {
		return "", "", err
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	now := time.Now()
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: subjectCN + " Root CA"},
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.AddDate(0, 0, days),
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:         true, BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, signer.Public(), signer)
	if err != nil {
		return "", "", err
	}
	if err := writePEMFile(cf, &pem.Block{Type: "CERTIFICATE", Bytes: der}, 0o444); err != nil {
		return "", "", err
	}

	return kf, cf, nil
}

// ---- tiny helpers (PEM / keys) ----
func genECDSAKeyPKCS8() (crypto.Signer, []byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	return priv, pkcs8, nil
}


func writePEMFile(path string, block *pem.Block, mode os.FileMode) error {
	return os.WriteFile(path, pem.EncodeToMemory(block), mode)
}

func readPEMBlock(path string) (*pem.Block, []byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	blk, rest := pem.Decode(b)
	if blk == nil {
		return nil, nil, fmt.Errorf("no PEM block in %s", path)
	}
	return blk, rest, nil
}

func parseAnyPrivateKey(block *pem.Block) (crypto.Signer, error) {
	if k, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if s, ok := k.(crypto.Signer); ok {
			return s, nil
		}
	}
	if k, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return k, nil
	}
	return nil, fmt.Errorf("unsupported private key format")
}
