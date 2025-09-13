package security

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	config_ "github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
)

var (
	Root       = config_.GetGlobularExecPath()
	ConfigPath = config_.GetConfigDir() + "/config.json"
	keyPath    = config_.GetConfigDir() + "/keys"

	caOnce           sync.Once
	caAuthorityHost  string
	caAuthorityPort  int
	credOpsProcessMu sync.Mutex // in-process serialization for the creds dir
)

// ----------------------------------------------------------------------------
// Deterministic CA authority resolution
// ----------------------------------------------------------------------------

func resolveCAAuthority(defaultHost string, defaultPort int) (string, int) {
	caOnce.Do(func() {
		host, port := defaultHost, defaultPort
		if lc, err := config_.GetLocalConfig(true); err == nil && lc != nil {
			if dns, ok := lc["DNS"].(string); ok && strings.TrimSpace(dns) != "" {
				host = dns
				port = 443
				if i := strings.IndexByte(host, ':'); i > 0 {
					p := Utility.ToInt(host[i+1:])
					if p > 0 {
						port = p
					}
					host = host[:i]
				}
			}
		}
		caAuthorityHost, caAuthorityPort = host, port
	})
	return caAuthorityHost, caAuthorityPort
}

// Choose a protocol stably: use https for 443, else http. Only fall back to the
// other if the primary fails (we do not alternate per call).
func preferredProtocol(port int) (primary, fallback string) {
	if port == 443 {
		return "https", "http"
	}
	return "http", "https"
}

// ----------------------------------------------------------------------------
// CA retrieval / signing
// ----------------------------------------------------------------------------

func getCaCertificate(address string, port int) (string, error) {
	host, p := resolveCAAuthority(address, port)
	prim, alt := preferredProtocol(p)

	if crt, err := getCaCertificate_(host, p, prim); err == nil {
		return crt, nil
	}
	if crt, err := getCaCertificate_(host, p, alt); err == nil {
		return crt, nil
	}
	return "", fmt.Errorf("get CA certificate: unable to retrieve from %s:%d", host, p)
}

func getCaCertificate_(address string, port int, protocol string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", errors.New("get CA certificate: no address provided")
	}
	url := fmt.Sprintf("%s://%s:%d/get_ca_certificate", protocol, address, port)

	resp, err := http.Get(url) // #nosec G107
	if err != nil {
		return "", fmt.Errorf("get CA certificate: GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("get CA certificate: read %s: %w", url, err)
		}
		return string(body), nil
	}
	return "", fmt.Errorf("get CA certificate: unexpected HTTP %d from %s", resp.StatusCode, url)
}

func signCaCertificate(address string, csr string, port int) (string, error) {
	host, p := resolveCAAuthority(address, port)
	prim, alt := preferredProtocol(p)

	if crt, err := signCaCertificate_(host, csr, p, prim); err == nil {
		return crt, nil
	}
	if crt, err := signCaCertificate_(host, csr, p, alt); err == nil {
		return crt, nil
	}
	return "", fmt.Errorf("sign CA certificate: unable to sign at %s:%d", host, p)
}

func signCaCertificate_(address string, csr string, port int, protocol string) (string, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return "", errors.New("sign CA certificate: no address provided")
	}
	csrStr := base64.StdEncoding.EncodeToString([]byte(csr))
	url := fmt.Sprintf("%s://%s:%d/sign_ca_certificate?csr=%s", protocol, address, port, csrStr)

	resp, err := http.Get(url) // #nosec G107
	if err != nil {
		return "", fmt.Errorf("sign CA certificate: GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("sign CA certificate: read %s: %w", url, err)
		}
		return string(body), nil
	}
	return "", fmt.Errorf("sign CA certificate: unexpected HTTP %d from %s", resp.StatusCode, url)
}

// ----------------------------------------------------------------------------
// Public API: Install client/server certs (atomic, locked, CA-stable)
// ----------------------------------------------------------------------------
func InstallClientCertificates(domain string, port int, path string, country string, state string, city string, organization string, alternateDomains []interface{}) (string, string, string, error) {
	err := withCredsLock(path, func() error {
		_, _, _, e := getClientCredentialConfig(path, domain, country, state, city, organization, alternateDomains, port)
		return e
	})
	if err != nil {
		return "", "", "", err
	}
	return path + "/client.pem", path + "/client.crt", path + "/ca.crt", nil
}

func InstallServerCertificates(domain string, port int, path string, country string, state string, city string, organization string, alternateDomains []interface{}) (string, string, string, error) {
	err := withCredsLock(path, func() error {
		_, _, _, e := getServerCredentialConfig(path, domain, country, state, city, organization, alternateDomains, port)
		return e
	})
	if err != nil {
		return "", "", "", err
	}
	return path + "/server.key", path + "/server.crt", path + "/ca.crt", nil
}

// ----------------------------------------------------------------------------
// Atomic rotation utilities
// ----------------------------------------------------------------------------

func withCredsLock(dir string, fn func() error) error {
	credOpsProcessMu.Lock()
	defer credOpsProcessMu.Unlock()

	if err := Utility.CreateDirIfNotExist(dir); err != nil {
		return err
	}
	lockPath := filepath.Join(dir, ".cert.lock")
	const (
		lockHoldMax = 10 * time.Minute
		waitStep    = 150 * time.Millisecond
		waitMax     = 20 * time.Second
	)
	start := time.Now()

	for {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
		if err == nil {
			// got the lock
			defer func() { _ = os.Remove(lockPath) }()
			_, _ = f.WriteString(strconv.Itoa(os.Getpid()))
			_ = f.Close()
			return fn()
		}
		// Lock exists; if stale, remove it
		if st, statErr := os.Stat(lockPath); statErr == nil {
			if time.Since(st.ModTime()) > lockHoldMax {
				_ = os.Remove(lockPath) // best effort
				continue
			}
		}
		if time.Since(start) > waitMax {
			return fmt.Errorf("creds lock: timeout waiting for %s", lockPath)
		}
		time.Sleep(waitStep)
	}
}

func atomicWriteCreds(target string, write func(tmp string) error) error {
	parent := filepath.Dir(target)
	base := filepath.Base(target)
	tmp := filepath.Join(parent, "."+base+".tmp-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	bak := filepath.Join(parent, "."+base+".bak-"+strconv.FormatInt(time.Now().UnixNano(), 10))

	if err := os.MkdirAll(tmp, 0o700); err != nil {
		return err
	}
	if err := write(tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	// move current aside
	if Utility.Exists(target) {
		if err := os.Rename(target, bak); err != nil {
			_ = os.RemoveAll(tmp)
			return err
		}
	}
	// swap in new
	if err := os.Rename(tmp, target); err != nil {
		// best-effort restore
		_ = os.Rename(bak, target)
		_ = os.RemoveAll(tmp)
		return err
	}
	_ = os.RemoveAll(bak)
	return nil
}

// ----------------------------------------------------------------------------
// Fingerprints (stable CA comparison)
// ----------------------------------------------------------------------------

func spkiFingerprintFromPEM(pemBytes []byte) (string, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return "", errors.New("fingerprint: no PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return hex.EncodeToString(sum[:]), nil
}

func fileSPKIFingerprint(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return spkiFingerprintFromPEM(data)
}

// ----------------------------------------------------------------------------
// Client creds
// ----------------------------------------------------------------------------

func getClientCredentialConfig(path string, domain string, country string, state string, city string, organization string, alternateDomains []interface{}, port int) (keyPath string, certPath string, caPath string, err error) {
	address := domain
	const pwd = "1111"

	if err = Utility.CreateDirIfNotExist(path); err != nil {
		return "", "", "", fmt.Errorf("client creds: ensure dir: %w", err)
	}

	alts := normalizeAltDomains(domain, alternateDomains, &domain)

	// Retrieve CA certificate from authority (deterministic)
	caCRT, err := getCaCertificate(address, port)
	if err != nil {
		return "", "", "", fmt.Errorf("client creds: get ca.crt: %w", err)
	}
	remoteFP, err := spkiFingerprintFromPEM([]byte(caCRT))
	if err != nil {
		return "", "", "", fmt.Errorf("client creds: parse remote CA: %w", err)
	}

	// If existing and same CA, reuse
	if Utility.Exists(filepath.Join(path, "client.pem")) &&
		Utility.Exists(filepath.Join(path, "client.crt")) &&
		Utility.Exists(filepath.Join(path, "ca.crt")) {

		localFP, err := fileSPKIFingerprint(filepath.Join(path, "ca.crt"))
		if err == nil && localFP == remoteFP {
			return path + "/client.pem", path + "/client.crt", path + "/ca.crt", nil
		}
	}

	// Build fresh set atomically
	err = atomicWriteCreds(path, func(tmp string) error {
		// write ca.crt
		if err := os.WriteFile(filepath.Join(tmp, "ca.crt"), []byte(caCRT), 0o444); err != nil {
			return fmt.Errorf("client creds: write ca.crt: %w", err)
		}
		if err := GenerateClientPrivateKey(tmp, pwd); err != nil {
			return fmt.Errorf("client creds: private key: %w", err)
		}
		if err := GenerateSanConfig(domain, tmp, country, state, city, organization, alts); err != nil {
			return fmt.Errorf("client creds: san.conf: %w", err)
		}
		if err := GenerateClientCertificateSigningRequest(tmp, pwd, domain); err != nil {
			return fmt.Errorf("client creds: CSR: %w", err)
		}
		csr, err := os.ReadFile(filepath.Join(tmp, "client.csr"))
		if err != nil {
			return fmt.Errorf("client creds: read CSR: %w", err)
		}
		host, p := resolveCAAuthority(address, port)
		clientCRT, err := signCaCertificate(host, string(csr), p)
		if err != nil {
			return fmt.Errorf("client creds: sign via CA: %w", err)
		}
		if err := os.WriteFile(filepath.Join(tmp, "client.crt"), []byte(clientCRT), 0o444); err != nil {
			return fmt.Errorf("client creds: write client.crt: %w", err)
		}
		if err := KeyToPem("client", tmp, pwd); err != nil {
			return fmt.Errorf("client creds: key->pem: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", "", "", err
	}

	return path + "/client.pem", path + "/client.crt", path + "/ca.crt", nil
}

// ----------------------------------------------------------------------------
// Server creds
// ----------------------------------------------------------------------------

func getServerCredentialConfig(path string, domain string, country string, state string, city string, organization string, alternateDomains []interface{}, port int) (keyPath string, certPath string, caPath string, err error) {
	const pwd = "1111"

	if err = Utility.CreateDirIfNotExist(path); err != nil {
		return "", "", "", fmt.Errorf("server creds: ensure dir: %w", err)
	}

	alts := normalizeAltDomains(domain, alternateDomains, &domain)

	// Retrieve CA from authority (deterministic)
	caCRT, err := getCaCertificate(domain, port)
	if err != nil {
		return "", "", "", fmt.Errorf("server creds: get ca.crt: %w", err)
	}
	remoteFP, err := spkiFingerprintFromPEM([]byte(caCRT))
	if err != nil {
		return "", "", "", fmt.Errorf("server creds: parse remote CA: %w", err)
	}

	// Reuse if CA unchanged
	if Utility.Exists(filepath.Join(path, "server.key")) &&
		Utility.Exists(filepath.Join(path, "server.crt")) &&
		Utility.Exists(filepath.Join(path, "ca.crt")) {

		localFP, err := fileSPKIFingerprint(filepath.Join(path, "ca.crt"))
		if err == nil && localFP == remoteFP {
			return path + "/server.key", path + "/server.crt", path + "/ca.crt", nil
		}
	}

	// Build fresh set atomically
	err = atomicWriteCreds(path, func(tmp string) error {
		if err := os.WriteFile(filepath.Join(tmp, "ca.crt"), []byte(caCRT), 0o444); err != nil {
			return fmt.Errorf("server creds: write ca.crt: %w", err)
		}
		if err := GenerateSeverPrivateKey(tmp, pwd); err != nil {
			return fmt.Errorf("server creds: private key: %w", err)
		}
		if err := GenerateSanConfig(domain, tmp, country, state, city, organization, alts); err != nil {
			return fmt.Errorf("server creds: san.conf: %w", err)
		}
		if err := GenerateServerCertificateSigningRequest(tmp, pwd, domain); err != nil {
			return fmt.Errorf("server creds: CSR: %w", err)
		}
		csr, err := os.ReadFile(filepath.Join(tmp, "server.csr"))
		if err != nil {
			return fmt.Errorf("server creds: read CSR: %w", err)
		}
		host, p := resolveCAAuthority(domain, port)
		crt, err := signCaCertificate(host, string(csr), p)
		if err != nil {
			return fmt.Errorf("server creds: sign via CA: %w", err)
		}
		if err := os.WriteFile(filepath.Join(tmp, "server.crt"), []byte(crt), 0o444); err != nil {
			return fmt.Errorf("server creds: write server.crt: %w", err)
		}
		if err := KeyToPem("server", tmp, pwd); err != nil {
			return fmt.Errorf("server creds: key->pem: %w", err)
		}
		return nil
	})
	if err != nil {
		return "", "", "", err
	}

	return path + "/server.key", path + "/server.crt", path + "/ca.crt", nil
}

// ----------------------------------------------------------------------------
// Generate full local CA + leafs (unchanged except minor cleanup)
// ----------------------------------------------------------------------------

func GenerateServicesCertificates(pwd string, expiration_delay int, domain string, path string, country string, state string, city string, organization string, alternateDomains []interface{}) error {
	if Utility.Exists(filepath.Join(path, "client.crt")) {
		return nil // already created
	}

	logger.Info("generate services certificates", "domain", domain, "alt", alternateDomains)

	alts := normalizeAltDomains(domain, alternateDomains, &domain)

	// Prefer external DNS authority if configured and distinct
	if localCfg, err := config_.GetLocalConfig(true); err == nil && localCfg != nil {
		if dns, ok := localCfg["DNS"].(string); ok && strings.TrimSpace(dns) != "" {
			dnsAddr := dns
			p := 443
			if i := strings.IndexByte(dnsAddr, ':'); i > 0 {
				p = Utility.ToInt(dnsAddr[i+1:])
				dnsAddr = dnsAddr[:i]
			}
			fqdn := fmt.Sprintf("%s.%s", localCfg["Name"], localCfg["Domain"])
			if dnsAddr != fqdn {
				if _, _, _, err := getServerCredentialConfig(path, dnsAddr, country, state, city, organization, alternateDomains, p); err != nil {
					return err
				}
				if _, _, _, err := getClientCredentialConfig(path, dnsAddr, country, state, city, organization, alternateDomains, p); err != nil {
					return err
				}
				return nil
			}
		}
	}

	// Local CA
	if err := GenerateSanConfig(domain, path, country, state, city, organization, alts); err != nil {
		return fmt.Errorf("generate services: san.conf: %w", err)
	} else {
		logger.Info("generated services: san.conf", "path", filepath.Join(path, "san.conf"))
	}
	if err := GenerateAuthorityPrivateKey(path, pwd); err != nil {
		return fmt.Errorf("generate services: ca.key: %w", err)
	} else {
		logger.Info("generated services: ca.key", "path", filepath.Join(path, "ca.key"))
	}
	if err := GenerateAuthorityTrustCertificate(path, pwd, expiration_delay, domain); err != nil {
		return fmt.Errorf("generate services: ca.crt: %w", err)
	} else {
		logger.Info("generated services: ca.crt", "path", filepath.Join(path, "ca.crt"))
	}

	if err := GenerateSeverPrivateKey(path, pwd); err != nil {
		return fmt.Errorf("generate services: server.key: %w", err)
	} else {
		logger.Info("generated services: server.key", "path", filepath.Join(path, "server.key"))
	}
	if err := GenerateServerCertificateSigningRequest(path, pwd, domain); err != nil {
		return fmt.Errorf("generate services: server.csr: %w", err)
	} else {
		logger.Info("generated services: server.csr", "path", filepath.Join(path, "server.csr"))
	}
	if err := GenerateSignedServerCertificate(path, pwd, expiration_delay); err != nil {
		return fmt.Errorf("generate services: server.crt: %w", err)
	} else {
		logger.Info("generated services: server.crt", "path", filepath.Join(path, "server.crt"))
	}
	if err := KeyToPem("server", path, pwd); err != nil {
		return fmt.Errorf("generate services: server.pem: %w", err)
	} else {
		logger.Info("generated services: server.pem", "path", filepath.Join(path, "server.pem"))
	}

	if err := GenerateClientPrivateKey(path, pwd); err != nil {
		return fmt.Errorf("generate services: client.key: %w", err)
	} else {
		logger.Info("generated services: client.key", "path", filepath.Join(path, "client.key"))
	}
	if err := GenerateClientCertificateSigningRequest(path, pwd, domain); err != nil {
		return fmt.Errorf("generate services: client.csr: %w", err)
	} else {
		logger.Info("generated services: client.csr", "path", filepath.Join(path, "client.csr"))
	}
	if err := GenerateSignedClientCertificate(path, pwd, expiration_delay); err != nil {
		return fmt.Errorf("generate services: client.crt: %w", err)
	} else {
		logger.Info("generated services: client.crt", "path", filepath.Join(path, "client.crt"))
	}
	if err := KeyToPem("client", path, pwd); err != nil {
		return fmt.Errorf("generate services: client.pem: %w", err)
	} else {
		logger.Info("generated services: client.pem", "path", filepath.Join(path, "client.pem"))
	}

	logger.Info("generated services certificates", "domain", domain, "alt", alternateDomains)

	return nil
}

// ----------------------------------------------------------------------------
// Peer key generation (unchanged except small tidy)
// ----------------------------------------------------------------------------

func DeletePublicKey(id string) error {
	id = strings.ReplaceAll(id, ":", "_")
	p := filepath.Join(keyPath, id+"_public")
	if !Utility.Exists(p) {
		logger.Info("delete public key: not found", "path", p)
		return nil
	}
	logger.Info("delete public key", "path", p)
	return os.Remove(p)
}

func GeneratePeerKeys(id string) error {
	if len(id) == 0 {
		return errors.New("generate peer keys: empty id")
	}
	id = strings.ReplaceAll(id, ":", "_")

	var (
		priv *ecdsa.PrivateKey
		err  error
	)

	if !Utility.Exists(filepath.Join(keyPath, id+"_private")) {
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
		f, err := os.Create(filepath.Join(keyPath, id+"_private"))
		if err != nil {
			return fmt.Errorf("generate peer keys: create private file: %w", err)
		}
		defer f.Close()
		if err = pem.Encode(f, &pem.Block{Type: "esdsa private key", Bytes: raw}); err != nil { // preserve original block type
			return fmt.Errorf("generate peer keys: pem encode private: %w", err)
		}
	} else {
		priv, err = readPrivateKey(id)
		if err != nil {
			return err
		}
	}

	pub := priv.PublicKey
	pubDER, err := x509.MarshalPKIXPublicKey(&pub)
	if err != nil {
		return fmt.Errorf("generate peer keys: marshal public: %w", err)
	}
	f, err := os.Create(filepath.Join(keyPath, id+"_public"))
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

func GetLocalKey() ([]byte, error) {
	if len(localKey) > 0 {
		return localKey, nil
	}
	mac, err := config_.GetMacAddress()
	if err != nil {
		return nil, fmt.Errorf("get local key: get mac: %w", err)
	}
	id := strings.ReplaceAll(mac, ":", "_")
	path := filepath.Join(keyPath, id+"_public")
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

func readPrivateKey(id string) (*ecdsa.PrivateKey, error) {
	id = strings.ReplaceAll(id, ":", "_")
	f, err := os.Open(filepath.Join(keyPath, id+"_private"))
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
		_ = os.Remove(filepath.Join(keyPath, id+"_private"))
		return nil, fmt.Errorf("corrupted private key for peer %s: deleted; reconnect peers to regenerate", id)
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func readPublicKey(id string) (*ecdsa.PublicKey, error) {
	id = strings.ReplaceAll(id, ":", "_")
	f, err := os.Open(filepath.Join(keyPath, id+"_public"))
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
		_ = os.Remove(filepath.Join(keyPath, id+"_public"))
		return nil, fmt.Errorf("corrupted public key for peer %s: deleted; reconnect peers to regenerate", id)
	}
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return pubAny.(*ecdsa.PublicKey), nil
}

func GetPeerKey(id string) ([]byte, error) {
	if len(id) == 0 {
		return nil, errors.New("get peer key: empty id")
	}
	id = strings.ReplaceAll(id, ":", "_")

	mac, err := config_.GetMacAddress()
	if err != nil {
		return nil, fmt.Errorf("get peer key: get mac: %w", err)
	}

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

	x, _ := pub.Curve.ScalarMult(pub.X, pub.Y, priv.D.Bytes())
	return []byte(x.String()), nil
}

func SetPeerPublicKey(id, encPub string) error {
	id = strings.ReplaceAll(id, ":", "_")
	path := filepath.Join(keyPath, id+"_public")
	if err := os.WriteFile(path, []byte(encPub), 0o644); err != nil {
		return fmt.Errorf("set peer public key: write %s: %w", path, err)
	}
	return nil
}

// ----------------------------------------------------------------------------
// Local PEM utilities / keys / CSRs / signing (mostly as-is, minor tidy)
// ----------------------------------------------------------------------------

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

// CA key/cert

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

// Server/Client keys

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

// SAN config

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
	return os.WriteFile(filepath.Join(path, "san.conf"), []byte(cfg), 0o644)
}

// CSRs

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

// Signing

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

func GenerateSignedClientCertificate(path string, _ string, expiration_delay int) error {
	if fileExists(path + "/client.crt") {
		return nil
	}
	return signCSRWithCA(filepath.Join(path, "client.csr"), filepath.Join(path, "ca.crt"), filepath.Join(path, "ca.key"), filepath.Join(path, "client.crt"), expiration_delay, false)
}

func GenerateSignedServerCertificate(path string, _ string, expiration_delay int) error {
	if fileExists(path + "/server.crt") {
		return nil
	}
	return signCSRWithCA(filepath.Join(path, "server.csr"), filepath.Join(path, "ca.crt"), filepath.Join(path, "ca.key"), filepath.Join(path, "server.crt"), expiration_delay, true)
}

// PEM conversion

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

// Validation

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

// ----------------------------------------------------------------------------
// small helpers
// ----------------------------------------------------------------------------

// normalizeAltDomains adjusts wildcard alts, may update *domain
func normalizeAltDomains(domain string, alternateDomains []interface{}, domainOut *string) []string {
	alts := make([]string, 0, len(alternateDomains))
	for i := range alternateDomains {
		alts = append(alts, alternateDomains[i].(string))
	}
	for _, d := range alts {
		if strings.HasPrefix(d, "*.") {
			if strings.HasSuffix(domain, d[2:]) {
				domain = d[2:]
			}
			altsStr := make([]string, len(alternateDomains))
			for i, v := range alternateDomains {
				altsStr[i] = v.(string)
			}
			if !Utility.Contains(altsStr, d[2:]) {
				alternateDomains = append(alternateDomains, d[2:])
			}
		}
	}
	if domainOut != nil {
		*domainOut = domain
	}
	out := make([]string, 0, len(alternateDomains))
	for i := range alternateDomains {
		out = append(out, alternateDomains[i].(string))
	}
	return out
}
