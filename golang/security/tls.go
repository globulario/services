package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	config_ "github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

var (
	Root       = config_.GetGlobularExecPath()
	ConfigPath = config_.GetConfigDir() + "/config.json"
	keyPath    = config_.GetConfigDir() + "/keys"
)

/*
getCaCertificate retrieves the CA certificate from a remote authority.
It tries plain HTTP first, then HTTPS, and may override address/port if DNS is set in local config.
*/
func getCaCertificate(address string, port int) (string, error) {
	// If DNS is configured, prefer it
	if localCfg, err := config_.GetLocalConfig(true); err == nil && localCfg != nil {
		if dns, ok := localCfg["DNS"].(string); ok && len(dns) > 0 {
			address = dns
			port = 443
			if strings.Contains(address, ":") {
				parts := strings.Split(address, ":")
				port = Utility.ToInt(parts[1])
				address = parts[0]
			}
		}
	}

	// Try HTTP then HTTPS
	if crt, err := getCaCertificate_(address, port, "http"); err == nil {
		return crt, nil
	}
	if crt, err := getCaCertificate_(address, port, "https"); err == nil {
		return crt, nil
	}

	return "", fmt.Errorf("get CA certificate: unable to retrieve from %s:%d over http/https", address, port)
}

/*
getCaCertificate_ retrieves the CA certificate using the given protocol.
*/
func getCaCertificate_(address string, port int, protocol string) (string, error) {
	if len(address) == 0 {
		return "", errors.New("get CA certificate: no address provided")
	}

	url := protocol + "://" + address + ":" + Utility.ToString(port) + "/get_ca_certificate"
	resp, err := http.Get(url) // #nosec G107 (intentional outbound call to configured CA endpoint)
	if err != nil {
		return "", fmt.Errorf("get CA certificate: GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		// Read real body
		body, err := ioReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("get CA certificate: read response: %w", err)
		}
		return string(body), nil
	}

	return "", fmt.Errorf("get CA certificate: unexpected HTTP status %d from %s", resp.StatusCode, url)
}

/*
signCaCertificate submits a CSR to the remote CA for signing.
It tries HTTP first, then HTTPS, and may override address/port if DNS is set in local config.
*/
func signCaCertificate(address string, csr string, port int) (string, error) {
	// If DNS is configured, prefer it
	if localCfg, err := config_.GetLocalConfig(true); err == nil && localCfg != nil {
		if dns, ok := localCfg["DNS"].(string); ok && len(dns) > 0 {
			address = dns
			port = 443
			if strings.Contains(address, ":") {
				parts := strings.Split(address, ":")
				port = Utility.ToInt(parts[1])
				address = parts[0]
			}
		}
	}

	if crt, err := signCaCertificate_(address, csr, port, "http"); err == nil {
		return crt, nil
	}
	if crt, err := signCaCertificate_(address, csr, port, "https"); err == nil {
		return crt, nil
	}

	return "", fmt.Errorf("sign CA certificate: unable to sign at %s:%d over http/https", address, port)
}

/*
signCaCertificate_ calls the CA /sign_ca_certificate endpoint with the base64-encoded CSR.
*/
func signCaCertificate_(address string, csr string, port int, protocol string) (string, error) {
	if len(address) == 0 {
		return "", errors.New("sign CA certificate: no address provided")
	}

	csrStr := base64.StdEncoding.EncodeToString([]byte(csr))
	url := protocol + "://" + address + ":" + Utility.ToString(port) + "/sign_ca_certificate?csr=" + csrStr
	resp, err := http.Get(url) // #nosec G107
	if err != nil {
		return "", fmt.Errorf("sign CA certificate: GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		body, err := ioReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("sign CA certificate: read response: %w", err)
		}
		return string(body), nil
	}

	return "", fmt.Errorf("sign CA certificate: unexpected HTTP status %d from %s", resp.StatusCode, url)
}

// =========================== Certificate Authority ===========================

/*
InstallClientCertificates fetches/creates client-side TLS materials and returns
paths (key, cert, ca) for the given domain. It preserves existing certs if the
remote CA hasn't changed.
*/
func InstallClientCertificates(domain string, port int, path string, country string, state string, city string, organization string, alternateDomains []interface{}) (string, string, string, error) {
	return getClientCredentialConfig(path, domain, country, state, city, organization, alternateDomains, port)
}

/*
InstallServerCertificates fetches/creates server-side TLS materials and returns
paths (key, cert, ca) for the given domain. It preserves existing certs if the
remote CA hasn't changed.
*/
func InstallServerCertificates(domain string, port int, path string, country string, state string, city string, organization string, alternateDomains []interface{}) (string, string, string, error) {
	return getServerCredentialConfig(path, domain, country, state, city, organization, alternateDomains, port)
}

/*
getClientCredentialConfig prepares client TLS credentials under path and returns key/cert/ca paths.
If local CA differs from remote CA, it rotates local materials.
*/
func getClientCredentialConfig(path string, domain string, country string, state string, city string, organization string, alternateDomains []interface{}, port int) (keyPath string, certPath string, caPath string, err error) {
	address := domain
	const pwd = "1111" // kept to preserve original behavior

	if err = Utility.CreateDirIfNotExist(path); err != nil {
		return "", "", "", fmt.Errorf("client creds: ensure dir: %w", err)
	}

	// Normalize alternate domains
	alts := make([]string, 0, len(alternateDomains))
	for i := range alternateDomains {
		alts = append(alts, alternateDomains[i].(string))
	}
	for _, d := range alts {
		if strings.Contains(d, "*") {
			if strings.HasSuffix(domain, d[2:]) {
				domain = d[2:]
			}
		}
	}

	// Retrieve CA certificate from authority
	caCRT, err := getCaCertificate(address, port)
	if err != nil {
		return "", "", "", fmt.Errorf("client creds: get ca.crt: %w", err)
	}

	// If existing, compare CA and reuse if identical
	if Utility.Exists(path) &&
		Utility.Exists(path+"/client.pem") &&
		Utility.Exists(path+"/client.crt") &&
		Utility.Exists(path+"/ca.crt") {

		localSum := Utility.CreateFileChecksum(path + "/ca.crt")
		remoteSum := Utility.CreateDataChecksum([]byte(caCRT))
		if localSum != remoteSum {
			logger.Info("client creds: CA changed, rotating certs", "path", path)
			if err = os.RemoveAll(path); err != nil {
				return "", "", "", fmt.Errorf("client creds: remove path: %w", err)
			}
			if err = Utility.CreateDirIfNotExist(path); err != nil {
				return "", "", "", fmt.Errorf("client creds: recreate dir: %w", err)
			}
		} else {
			return path + "/client.pem", path + "/client.crt", path + "/ca.crt", nil
		}
	}

	// Write CA
	if err = os.WriteFile(path+"/ca.crt", []byte(caCRT), 0o444); err != nil {
		return "", "", "", fmt.Errorf("client creds: write ca.crt: %w", err)
	}

	// Generate materials
	if err = GenerateClientPrivateKey(path, pwd); err != nil {
		return "", "", "", fmt.Errorf("client creds: private key: %w", err)
	}
	if err = GenerateSanConfig(domain, path, country, state, city, organization, alts); err != nil {
		return "", "", "", fmt.Errorf("client creds: san.conf: %w", err)
	}
	if err = GenerateClientCertificateSigningRequest(path, pwd, domain); err != nil {
		return "", "", "", fmt.Errorf("client creds: CSR: %w", err)
	}

	// Sign via CA
	csr, err := os.ReadFile(path + "/client.csr")
	if err != nil {
		return "", "", "", fmt.Errorf("client creds: read CSR: %w", err)
	}
	clientCRT, err := signCaCertificate(address, string(csr), Utility.ToInt(port))
	if err != nil {
		return "", "", "", fmt.Errorf("client creds: sign via CA: %w", err)
	}
	if err = os.WriteFile(path+"/client.crt", []byte(clientCRT), 0o444); err != nil {
		return "", "", "", fmt.Errorf("client creds: write client.crt: %w", err)
	}

	if err = KeyToPem("client", path, pwd); err != nil {
		return "", "", "", fmt.Errorf("client creds: key->pem: %w", err)
	}

	return path + "/client.pem", path + "/client.crt", path + "/ca.crt", nil
}

/*
getServerCredentialConfig prepares server TLS credentials under path and returns key/cert/ca paths.
If local CA differs from remote CA, it rotates local materials.
*/
func getServerCredentialConfig(path string, domain string, country string, state string, city string, organization string, alternateDomains []interface{}, port int) (keyPath string, certPath string, caPath string, err error) {
	const pwd = "1111" // kept to preserve original behavior

	if err = Utility.CreateDirIfNotExist(path); err != nil {
		return "", "", "", fmt.Errorf("server creds: ensure dir: %w", err)
	}

	// Normalize alternate domains
	alts := make([]string, 0, len(alternateDomains))
	for i := range alternateDomains {
		alts = append(alts, alternateDomains[i].(string))
	}
	for _, d := range alts {
		if strings.Contains(d, "*") {
			if strings.HasSuffix(domain, d[2:]) {
				domain = d[2:]
			}
		}
	}

	// Retrieve CA from authority
	caCRT, err := getCaCertificate(domain, port)
	if err != nil {
		return "", "", "", fmt.Errorf("server creds: get ca.crt: %w", err)
	}

	// Reuse existing materials if CA unchanged
	if Utility.Exists(path) &&
		Utility.Exists(path+"/server.pem") &&
		Utility.Exists(path+"/server.crt") &&
		Utility.Exists(path+"/ca.crt") {

		localSum := Utility.CreateFileChecksum(path + "/ca.crt")
		remoteSum := Utility.CreateDataChecksum([]byte(caCRT))
		if localSum != remoteSum {
			if err = os.RemoveAll(path); err != nil {
				return "", "", "", fmt.Errorf("server creds: remove path: %w", err)
			}
			if err = Utility.CreateDirIfNotExist(path); err != nil {
				return "", "", "", fmt.Errorf("server creds: recreate dir: %w", err)
			}
		} else {
			return path + "/server.pem", path + "/server.crt", path + "/ca.crt", nil
		}
	}

	// Write CA
	if err = os.WriteFile(path+"/ca.crt", []byte(caCRT), 0o444); err != nil {
		return "", "", "", fmt.Errorf("server creds: write ca.crt: %w", err)
	}

	// Generate materials
	if err = GenerateSeverPrivateKey(path, pwd); err != nil {
		return "", "", "", fmt.Errorf("server creds: private key: %w", err)
	}
	if err = GenerateSanConfig(domain, path, country, state, city, organization, alts); err != nil {
		return "", "", "", fmt.Errorf("server creds: san.conf: %w", err)
	}
	if err = GenerateServerCertificateSigningRequest(path, pwd, domain); err != nil {
		return "", "", "", fmt.Errorf("server creds: CSR: %w", err)
	}

	// Sign via CA
	csr, err := os.ReadFile(path + "/server.csr")
	if err != nil {
		return "", "", "", fmt.Errorf("server creds: read CSR: %w", err)
	}
	crt, err := signCaCertificate(domain, string(csr), Utility.ToInt(port))
	if err != nil {
		return "", "", "", fmt.Errorf("server creds: sign via CA: %w", err)
	}
	if err = os.WriteFile(path+"/server.crt", []byte(crt), 0o444); err != nil {
		return "", "", "", fmt.Errorf("server creds: write server.crt: %w", err)
	}

	if err = KeyToPem("server", path, pwd); err != nil {
		return "", "", "", fmt.Errorf("server creds: key->pem: %w", err)
	}

	return path + "/server.pem", path + "/server.crt", path + "/ca.crt", nil
}

// =============================== Server Keys ================================

/*
GenerateServicesCertificates creates a full local CA and signs both server and client
certificates for the given domain (when no external DNS/CA is used).
If DNS is configured and not equal to the local service FQDN, it fetches credentials from the DNS authority instead.
*/
func GenerateServicesCertificates(pwd string, expiration_delay int, domain string, path string, country string, state string, city string, organization string, alternateDomains []interface{}) error {
	if Utility.Exists(path + "/client.crt") {
		return nil // already created
	}

	logger.Info("generate services certificates", "domain", domain, "alt", alternateDomains)

	// Normalize alts and expand wildcard roots
	alts := make([]string, 0, len(alternateDomains))
	for i := range alternateDomains {
		alts = append(alts, alternateDomains[i].(string))
	}
	for _, d := range alts {
		if strings.HasPrefix(d, "*.") {
			if strings.HasSuffix(domain, d[2:]) {
				domain = d[2:]
			}
			// Convert alternateDomains to []string for Utility.Contains
			altsStr := make([]string, len(alternateDomains))
			for i, v := range alternateDomains {
				altsStr[i] = v.(string)
			}
			if !Utility.Contains(altsStr, d[2:]) {
				alternateDomains = append(alternateDomains, d[2:])
			}
		}
	}

	// Prefer external DNS authority when configured and distinct from local FQDN
	if localCfg, err := config_.GetLocalConfig(true); err == nil && localCfg != nil {
		if dns, ok := localCfg["DNS"].(string); ok && len(dns) > 0 {
			dnsAddr := dns
			port := 443
			if strings.Contains(dnsAddr, ":") {
				parts := strings.Split(dnsAddr, ":")
				port = Utility.ToInt(parts[1])
				dnsAddr = parts[0]
			}
			if fqdn := localCfg["Name"].(string) + "." + localCfg["Domain"].(string); dnsAddr != fqdn {
				// Server credentials via DNS CA
				if _, _, _, err := getServerCredentialConfig(path, dnsAddr, country, state, city, organization, alternateDomains, port); err != nil {
					return err
				}
				// Client credentials via DNS CA
				if _, _, _, err := getClientCredentialConfig(path, dnsAddr, country, state, city, organization, alternateDomains, port); err != nil {
					return err
				}
				return nil
			}
		}
	}

	// Build local CA & sign leaf certs
	if err := GenerateSanConfig(domain, path, country, state, city, organization, alts); err != nil {
		return fmt.Errorf("generate services: san.conf: %w", err)
	}
	if err := GenerateAuthorityPrivateKey(path, pwd); err != nil {
		return fmt.Errorf("generate services: ca.key: %w", err)
	}
	if err := GenerateAuthorityTrustCertificate(path, pwd, expiration_delay, domain); err != nil {
		return fmt.Errorf("generate services: ca.crt: %w", err)
	}

	if err := GenerateSeverPrivateKey(path, pwd); err != nil {
		return fmt.Errorf("generate services: server.key: %w", err)
	}
	if err := GenerateServerCertificateSigningRequest(path, pwd, domain); err != nil {
		return fmt.Errorf("generate services: server.csr: %w", err)
	}
	if err := GenerateSignedServerCertificate(path, pwd, expiration_delay); err != nil {
		return fmt.Errorf("generate services: server.crt: %w", err)
	}
	if err := KeyToPem("server", path, pwd); err != nil {
		return fmt.Errorf("generate services: server.pem: %w", err)
	}

	if err := GenerateClientPrivateKey(path, pwd); err != nil {
		return fmt.Errorf("generate services: client.key: %w", err)
	}
	if err := GenerateClientCertificateSigningRequest(path, pwd, domain); err != nil {
		return fmt.Errorf("generate services: client.csr: %w", err)
	}
	if err := GenerateSignedClientCertificate(path, pwd, expiration_delay); err != nil {
		return fmt.Errorf("generate services: client.crt: %w", err)
	}
	if err := KeyToPem("client", path, pwd); err != nil {
		return fmt.Errorf("generate services: client.pem: %w", err)
	}

	return nil
}

// ======================== Peer key generation (ECDH) ========================

/*
DeletePublicKey removes a stored peer public key file by peer ID (MAC).
No error is returned if the file does not exist.
*/
func DeletePublicKey(id string) error {
	id = strings.ReplaceAll(id, ":", "_")
	p := keyPath + "/" + id + "_public"
	if !Utility.Exists(p) {
		logger.Info("delete public key: not found", "path", p)
		return nil
	}
	logger.Info("delete public key", "path", p)
	return os.Remove(p)
}

/*
GeneratePeerKeys creates (or reuses) an ECDSA keypair for the given peer id (MAC),
storing the private and public keys under the standard keyPath.
*/
func GeneratePeerKeys(id string) error {
	if len(id) == 0 {
		return errors.New("generate peer keys: empty id")
	}
	id = strings.ReplaceAll(id, ":", "_")

	var (
		priv *ecdsa.PrivateKey
		err  error
	)

	if !Utility.Exists(keyPath + "/" + id + "_private") {
		// New private key (P-521)
		priv, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
		if err != nil {
			return fmt.Errorf("generate peer keys: ecdsa key: %w", err)
		}
		raw, err := x509.MarshalECPrivateKey(priv)
		if err != nil {
			return fmt.Errorf("generate peer keys: marshal ec private key: %w", err)
		}
		if err = Utility.CreateDirIfNotExist(keyPath); err != nil {
			return fmt.Errorf("generate peer keys: ensure dir: %w", err)
		}
		f, err := os.Create(keyPath + "/" + id + "_private")
		if err != nil {
			return fmt.Errorf("generate peer keys: create private file: %w", err)
		}
		defer f.Close()
		if err = pem.Encode(f, &pem.Block{Type: "esdsa private key", Bytes: raw}); err != nil { // keep original block type
			return fmt.Errorf("generate peer keys: pem encode private: %w", err)
		}
	} else {
		priv, err = readPrivateKey(id)
		if err != nil {
			return err
		}
	}

	// Write public key
	pub := priv.PublicKey
	pubDER, err := x509.MarshalPKIXPublicKey(&pub)
	if err != nil {
		return fmt.Errorf("generate peer keys: marshal public: %w", err)
	}
	f, err := os.Create(keyPath + "/" + id + "_public")
	if err != nil {
		return fmt.Errorf("generate peer keys: create public file: %w", err)
	}
	defer f.Close()
	if err = pem.Encode(f, &pem.Block{Type: "ecdsa public key", Bytes: pubDER}); err != nil {
		return fmt.Errorf("generate peer keys: pem encode public: %w", err)
	}
	return nil
}

var localKey = []byte{} // in-memory cache of local public key bytes

/*
GetLocalKey returns the local peer JWT key material (public key bytes).
It expects the local peer public key file to exist.
*/
func GetLocalKey() ([]byte, error) {
	if len(localKey) > 0 {
		return localKey, nil
	}
	mac, err := config_.GetMacAddress()
	if err != nil {
		return nil, fmt.Errorf("get local key: get mac: %w", err)
	}

	id := strings.ReplaceAll(mac, ":", "_")
	path := keyPath + "/" + id + "_public"
	if !Utility.Exists(path) {
		return nil, fmt.Errorf("get local key: no public key found at %s", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("get local key: read %s: %w", path, err)
	}
	localKey = b
	return localKey, nil
}

/*
readPrivateKey loads the ECDSA private key for a given peer id (MAC).
If the key is corrupted, it deletes it and returns an error describing the remediation.
*/
func readPrivateKey(id string) (*ecdsa.PrivateKey, error) {
	id = strings.ReplaceAll(id, ":", "_")
	f, err := os.Open(keyPath + "/" + id + "_private")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, info.Size())
	if _, err = f.Read(buf); err != nil {
		return nil, fmt.Errorf("read private key: read: %w", err)
	}

	block, _ := pem.Decode(buf)
	if block == nil {
		_ = os.Remove(keyPath + "/" + id + "_private")
		return nil, fmt.Errorf("corrupted private key for peer %s: deleted; reconnect peers to regenerate", id)
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func readPublicKey(id string) (*ecdsa.PublicKey, error) {
	id = strings.ReplaceAll(id, ":", "_")
	f, err := os.Open(keyPath + "/" + id + "_public")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, info.Size())
	if _, err = f.Read(buf); err != nil {
		return nil, fmt.Errorf("read public key: read: %w", err)
	}

	block, _ := pem.Decode(buf)
	if block == nil {
		_ = os.Remove(keyPath + "/" + id + "_public")
		return nil, fmt.Errorf("corrupted public key for peer %s: deleted; reconnect peers to regenerate", id)
	}
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return pubAny.(*ecdsa.PublicKey), nil
}

/*
GetPeerKey derives a shared JWT key for the given peer id (MAC) using ECDH-like ScalarMult.
If id matches the local MAC, it returns the local public key bytes.
*/
func GetPeerKey(id string) ([]byte, error) {
	if len(id) == 0 {
		return nil, errors.New("get peer key: empty id")
	}
	id = strings.ReplaceAll(id, ":", "_")

	mac, err := config_.GetMacAddress()
	if err != nil {
		return nil, fmt.Errorf("get peer key: get mac: %w", err)
	}

	// Local issuer: use local key bytes
	if id == strings.ReplaceAll(mac, ":", "_") {
		return GetLocalKey()
	}

	if err = Utility.CreateDirIfNotExist(keyPath); err != nil {
		return nil, fmt.Errorf("get peer key: ensure dir: %w", err)
	}

	pub, err := readPublicKey(id)
	if err != nil {
		return nil, err
	}
	priv, err := readPrivateKey(mac)
	if err != nil {
		return nil, err
	}

	// Shared secret point X coordinate as key material
	x, _ := pub.Curve.ScalarMult(pub.X, pub.Y, priv.D.Bytes())
	return []byte(x.String()), nil
}

/*
SetPeerPublicKey stores a peer's public key (PEM) by peer id (MAC).
*/
func SetPeerPublicKey(id, encPub string) error {
	id = strings.ReplaceAll(id, ":", "_")
	path := keyPath + "/" + id + "_public"
	if err := os.WriteFile(path, []byte(encPub), 0o644); err != nil {
		return fmt.Errorf("set peer public key: write %s: %w", path, err)
	}
	return nil
}

// ============================ Local PEM Utilities ===========================

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
		return nil, fmt.Errorf("read PEM: no PEM block in %s", path)
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
	return nil, errors.New("parse private key: unsupported format")
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
				if val := strings.TrimSpace(line[i+1:]); val != "" {
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

// ------------------------------- CA key/cert -------------------------------

/*
GenerateAuthorityPrivateKey creates ca.key in PKCS#8 format if it does not exist.
*/
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

/*
GenerateAuthorityTrustCertificate creates a self-signed CA certificate (ca.crt) if missing.
*/
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
		SerialNumber:          serial,
		Subject:               subj,
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(time.Duration(expiration_delay) * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, caSigner.Public(), caSigner)
	if err != nil {
		return err
	}
	return writePEM(path+"/ca.crt", &pem.Block{Type: "CERTIFICATE", Bytes: der}, 0o444)
}

// ---------------------------- Server/Client keys ---------------------------

/*
GenerateSeverPrivateKey creates server.key in PKCS#8 format if missing.
(Spelling preserved to avoid API break.)
*/
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

/*
GenerateClientPrivateKey creates client.key in PKCS#8 format if missing.
*/
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

// ------------------------------- SAN Config --------------------------------

/*
GenerateSanConfig writes san.conf with subject and SAN entries for domain/alt names if missing.
*/
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

// ---------------------------------- CSRs -----------------------------------

/*
GenerateClientCertificateSigningRequest creates client.csr for the domain if missing.
*/
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

/*
GenerateServerCertificateSigningRequest creates server.csr for the domain if missing.
*/
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

// --------------------------- CA-signed leaf certs ---------------------------

func signCSRWithCA(csrPath, caCrtPath, caKeyPath, outPath string, days int, isServer bool) error {
	caBlock, err := readPEM(caCrtPath)
	if err != nil {
		return err
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return err
	}
	keyBlock, err := readPEM(caKeyPath)
	if err != nil {
		return err
	}
	caSigner, err := parseAnyPrivateKey(keyBlock)
	if err != nil {
		return err
	}
	csrBlock, err := readPEM(csrPath)
	if err != nil {
		return err
	}
	csr, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	if err != nil {
		return err
	}

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

/*
GenerateSignedClientCertificate signs client.csr with ca.key/ca.crt and writes client.crt if missing.
*/
func GenerateSignedClientCertificate(path string, _ string, expiration_delay int) error {
	if fileExists(path + "/client.crt") {
		return nil
	}
	return signCSRWithCA(path+"/client.csr", path+"/ca.crt", path+"/ca.key", path+"/client.crt", expiration_delay, false)
}

/*
GenerateSignedServerCertificate signs server.csr with ca.key/ca.crt and writes server.crt if missing.
*/
func GenerateSignedServerCertificate(path string, _ string, expiration_delay int) error {
	if fileExists(path + "/server.crt") {
		return nil
	}
	return signCSRWithCA(path+"/server.csr", path+"/ca.crt", path+"/ca.key", path+"/server.crt", expiration_delay, true)
}

// -------------------------- PEM conversion (compat) -------------------------

/*
KeyToPem converts <name>.key to <name>.pem (PKCS#8), if not already present.
*/
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
	pkcs8, err := x509.MarshalPKCS8PrivateKey(signer)
	if err != nil {
		return err
	}
	return writePEM(pemPath, &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400)
}

// -------------------------------- Validation -------------------------------

/*
ValidateCertificateExpiration loads a cert/key pair and returns an error if the certificate is expired.
*/
func ValidateCertificateExpiration(certFile, keyFile string) error {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("validate cert expiration: load pair: %w", err)
	}
	c, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("validate cert expiration: parse: %w", err)
	}
	if time.Now().After(c.NotAfter) {
		return fmt.Errorf("the certificate is expired %s", c.NotAfter.Local().String())
	}
	return nil
}

// ------------------------------ small helpers ------------------------------

func ioReadAll(r ioReader) ([]byte, error) { return ioReadAllImpl(r) }

// decouple for testability without importing io in the public section
type ioReader interface{ Read([]byte) (int, error) }

func ioReadAllImpl(r ioReader) ([]byte, error) {
	const chunk = 32 * 1024
	var b []byte
	buf := make([]byte, chunk)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			b = append(b, buf[:n]...)
		}
		if err != nil {
			if errors.Is(err, os.ErrClosed) {
				return b, nil
			}
			if err.Error() == "EOF" {
				return b, nil
			}
			if errors.Is(err, ioEOF{}) { // placeholder, see note below
				return b, nil
			}
			if err != nil {
				if strings.Contains(err.Error(), "EOF") {
					return b, nil
				}
				return b, err
			}
		}
	}
}

// NOTE: In a normal project we’d simply import "io" and use io.ReadAll + io.EOF.
// I kept a tiny local reader shim to avoid changing your import set too much.
// If you're fine with importing "io", replace ioReadAll with io.ReadAll and remove the shim.
type ioEOF struct{}

func (ioEOF) Error() string {
	return "EOF"
}
