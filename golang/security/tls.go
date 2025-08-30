package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	config_ "github.com/globulario/services/golang/config"
)

var (
	Root       = config_.GetGlobularExecPath()
	ConfigPath = config_.GetConfigDir() + "/config.json"
	keyPath    = config_.GetConfigDir() + "/keys"
)

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func writePEM(path string, block *pem.Block, mode os.FileMode) error {
	return os.WriteFile(path, pem.EncodeToMemory(block), mode)
}

func readPEM(path string) (*pem.Block, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("no PEM block in %s", path)
	}
	return block, nil
}

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
	return nil, errors.New("unsupported private key format")
}

func parseSANsFromConf(path string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(path, "san.conf"))
	if err != nil {
		return nil, err
	}
	var sans []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "DNS.") {
			if i := strings.Index(line, "="); i > 0 {
				val := strings.TrimSpace(line[i+1:])
				if val != "" {
					sans = append(sans, val)
				}
			}
		}
	}
	return sans, nil
}

func serialNumber() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, limit)
}

// --- CA key/cert ---

func GenerateAuthorityPrivateKey(path string, _ string) error {
	if fileExists(path + "/ca.key") {
		return nil
	}
	_, pkcs8, err := genECDSAKeyPKCS8()
	if err != nil {
		return err
	}
	return writePEM(path+"/ca.key", &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400)
}

func GenerateAuthorityTrustCertificate(path string, _ string, expiration_delay int, domain string) error {
	if fileExists(path + "/ca.crt") {
		return nil
	}
	b, err := readPEM(path + "/ca.key")
	if err != nil {
		return err
	}
	caSigner, err := parseAnyPrivateKey(b)
	if err != nil {
		return err
	}
	subj := pkix.Name{CommonName: domain + " Root CA"}
	serial, _ := serialNumber()
	now := time.Now()
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      subj,
		NotBefore:    now.Add(-5 * time.Minute),
		NotAfter:     now.Add(time.Duration(expiration_delay) * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:         true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, caSigner.Public(), caSigner)
	if err != nil {
		return err
	}
	return writePEM(path+"/ca.crt", &pem.Block{Type: "CERTIFICATE", Bytes: der}, 0o444)
}

// --- Server/Client keys ---

func GenerateSeverPrivateKey(path string, _ string) error {
	if fileExists(path + "/server.key") {
		return nil
	}
	_, pkcs8, err := genECDSAKeyPKCS8()
	if err != nil {
		return err
	}
	return writePEM(path+"/server.key", &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400)
}

func GenerateClientPrivateKey(path string, _ string) error {
	if fileExists(path + "/client.key") {
		return nil
	}
	_, pkcs8, err := genECDSAKeyPKCS8()
	if err != nil {
		return err
	}
	return writePEM(path+"/client.key", &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400)
}

// --- SAN Config ---

func GenerateSanConfig(domain, path, country, state, city, organization string, alternateDomains []string) error {
	if fileExists(path + "/san.conf") {
		return nil
	}
	cfg := fmt.Sprintf(`
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
C  = %s
ST = %s
L  = %s
O  = %s
CN = %s

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
`, country, state, city, organization, domain)

	for i, d := range append(alternateDomains, domain) {
		cfg += fmt.Sprintf("DNS.%d = %s\n", i, d)
	}
	return os.WriteFile(path+"/san.conf", []byte(cfg), 0o644)
}

// --- CSRs ---

func GenerateClientCertificateSigningRequest(path string, _ string, domain string) error {
	if fileExists(path + "/client.csr") {
		return nil
	}
	keyBlock, err := readPEM(path + "/client.key")
	if err != nil {
		return err
	}
	signer, err := parseAnyPrivateKey(keyBlock)
	if err != nil {
		return err
	}
	sans, _ := parseSANsFromConf(path)
	tpl := &x509.CertificateRequest{Subject: pkix.Name{CommonName: domain}, DNSNames: sans}
	der, err := x509.CreateCertificateRequest(rand.Reader, tpl, signer)
	if err != nil {
		return err
	}
	return writePEM(path+"/client.csr", &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}, 0o444)
}

func GenerateServerCertificateSigningRequest(path string, _ string, domain string) error {
	if fileExists(path + "/server.csr") {
		return nil
	}
	keyBlock, err := readPEM(path + "/server.key")
	if err != nil {
		return err
	}
	signer, err := parseAnyPrivateKey(keyBlock)
	if err != nil {
		return err
	}
	sans, _ := parseSANsFromConf(path)
	tpl := &x509.CertificateRequest{Subject: pkix.Name{CommonName: domain}, DNSNames: sans}
	der, err := x509.CreateCertificateRequest(rand.Reader, tpl, signer)
	if err != nil {
		return err
	}
	return writePEM(path+"/server.csr", &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}, 0o444)
}

// --- CA-signed leaf certs ---

func signCSRWithCA(csrPath, caCrtPath, caKeyPath, outPath string, days int, isServer bool) error {
	caBlock, err := readPEM(caCrtPath)
	if err != nil {
		return err
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return err
	}
	keyBlock, _ := readPEM(caKeyPath)
	caSigner, _ := parseAnyPrivateKey(keyBlock)
	csrBlock, _ := readPEM(csrPath)
	csr, _ := x509.ParseCertificateRequest(csrBlock.Bytes)

	now := time.Now()
	serial, _ := serialNumber()
	ext := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	if isServer {
		ext = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	}
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      csr.Subject,
		NotBefore:    now,
		NotAfter:     now.Add(time.Duration(days) * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  ext,
		DNSNames:     csr.DNSNames,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, caCert, csr.PublicKey, caSigner)
	if err != nil {
		return err
	}
	return writePEM(outPath, &pem.Block{Type: "CERTIFICATE", Bytes: der}, 0o444)
}

func GenerateSignedClientCertificate(path string, _ string, expiration_delay int) error {
	if fileExists(path + "/client.crt") {
		return nil
	}
	return signCSRWithCA(path+"/client.csr", path+"/ca.crt", path+"/ca.key", path+"/client.crt", expiration_delay, false)
}

func GenerateSignedServerCertificate(path string, _ string, expiration_delay int) error {
	if fileExists(path + "/server.crt") {
		return nil
	}
	return signCSRWithCA(path+"/server.csr", path+"/ca.crt", path+"/ca.key", path+"/server.crt", expiration_delay, true)
}

// --- PEM conversion (compat) ---

func KeyToPem(name string, path string, _ string) error {
	pemPath := filepath.Join(path, name+".pem")
	if fileExists(pemPath) {
		return nil
	}
	block, err := readPEM(filepath.Join(path, name+".key"))
	if err != nil {
		return err
	}
	signer, err := parseAnyPrivateKey(block)
	if err != nil {
		return err
	}
	pkcs8, _ := x509.MarshalPKCS8PrivateKey(signer)
	return writePEM(pemPath, &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400)
}

// --- Validation ---
func ValidateCertificateExpiration(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}
	cert_, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return err
	}
	if time.Now().After(cert_.NotAfter) {
		return errors.New("the certificate is expired " + cert_.NotAfter.Local().String())
	}
	return nil
}
