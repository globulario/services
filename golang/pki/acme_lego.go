package pki

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/dns/dns_client"
	"github.com/globulario/services/golang/security"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/registration"
)

// lego user
type acmeUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string                        { return u.Email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.Registration }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

// ensureACMEAccountKey writes/loads account key from client.pem
func (m *FileManager) ensureACMEAccountKey(dir, email string) (crypto.PrivateKey, error) {
	acctPem := filepath.Join(dir, "client.pem")
	if !exists(acctPem) {
		_, pkcs8, err := genECDSAKeyPKCS8()
		if err != nil {
			return nil, err
		}
		if err := writePEMFile(acctPem, &pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}, 0o400); err != nil {
			return nil, err
		}
	}
	blk, _, err := readPEMBlock(acctPem)
	if err != nil {
		return nil, err
	}
	return parseAnyPrivateKey(blk)
}

// atomicWriteFile writes to a temp dir then renames into place.
// mode is the file permission mode for the final file.
func atomicWriteFile(path string, mode os.FileMode, write func(tmpDir string) error) error {
	dir := filepath.Dir(path)

	// Create a temporary directory in the same parent
	tmpDir, err := os.MkdirTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("atomic write: mkdir temp: %w", err)
	}
	defer os.RemoveAll(tmpDir) // cleanup if something fails

	// Let the caller write into tmpDir
	if err := write(tmpDir); err != nil {
		return fmt.Errorf("atomic write: write func failed: %w", err)
	}

	// We expect the file to be written inside tmpDir with the same basename as path
	tmpFile := filepath.Join(tmpDir, filepath.Base(path))

	// fsync isn’t strictly required on Linux, but we can open + Sync if needed
	if f, err := os.Open(tmpFile); err == nil {
		_ = f.Sync()
		f.Close()
	}

	// Move into place atomically
	if err := os.Rename(tmpFile, path); err != nil {
		return fmt.Errorf("atomic write: rename failed: %w", err)
	}

	// Ensure final permissions
	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("atomic write: chmod failed: %w", err)
	}

	return nil
}

// acmeObtainOrRenewCSR uses the existing CSR at <name>.csr and writes <name>.crt + acme-ca.crt.
func (m *FileManager) acmeObtainOrRenewCSRNamed(dir string, name string) (crt string, ca string, err error) {

	csrFile := csrPath(dir, name)
	blk, _, err := readPEMBlock(csrFile)
	if err != nil {
		return "", "", fmt.Errorf("read CSR: %w", err)
	}
	if blk.Type != "CERTIFICATE REQUEST" && blk.Type != "NEW CERTIFICATE REQUEST" {
		return "", "", fmt.Errorf("invalid CSR PEM type %q", blk.Type)
	}
	csr, err := x509.ParseCertificateRequest(blk.Bytes)
	if err != nil {
		return "", "", fmt.Errorf("parse CSR: %w", err)
	}

	// lego config
	email := strings.TrimSpace(m.ACME.Email)
	if email == "" {
		return "", "", fmt.Errorf("acme: email is required")
	}
	accountKey, err := m.ensureACMEAccountKey(dir, email)
	if err != nil {
		return "", "", err
	}
	u := &acmeUser{Email: email, key: accountKey}
	cfg := lego.NewConfig(u)
	switch strings.ToLower(strings.TrimSpace(m.ACME.Directory)) {
	case "", "prod", "production":
		// default (LE prod)
	case "staging":
		cfg.CADirURL = lego.LEDirectoryStaging
	default:
		cfg.CADirURL = m.ACME.Directory
	}
	cfg.Certificate.KeyType = certcrypto.EC256

	client, err := lego.NewClient(cfg)
	if err != nil {
		return "", "", err
	}

	// Provider
	switch strings.ToLower(strings.TrimSpace(m.ACME.Provider)) {
	case "cloudflare":
		p, err := cloudflare.NewDNSProvider()
		if err != nil {
			return "", "", err
		}
		if err := client.Challenge.SetDNS01Provider(p); err != nil {
			return "", "", err
		}

	case "globular":

		// DNS-01 via Globular DNS HTTP API.
		if m.ACME.DNS == "" {
			return "", "", fmt.Errorf("acme: provider 'globular' but ACME.DNS empty")
		}
		gp := &globularDNSProvider{
			addr:    m.ACME.DNS,
			domain:  m.ACME.Domain,
			timeout: int(m.ACME.Timeout.Seconds()),
			email:   m.ACME.Email,
		}
		if err := client.Challenge.SetDNS01Provider(gp); err != nil {
			return "", "", err
		}

	default:
		// Fallback: HTTP-01 on :80
		prov := http01.NewProviderServer("", "80")
		if err := client.Challenge.SetHTTP01Provider(prov); err != nil {
			return "", "", err
		}
	}

	// Register (idempotent)
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "already registered") {
		return "", "", fmt.Errorf("acme register: %w", err)
	}
	u.Registration = reg

	req := certificate.ObtainForCSRRequest{
		CSR:    csr,
		Bundle: true,
	}

	// Timeout context
	ctx := context.Background()
	if m.ACME.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.ACME.Timeout)
		defer cancel()
	}

	// Perform order
	res, err := client.Certificate.ObtainForCSR(req)
	if err != nil {
		return "", "", fmt.Errorf("lego obtain: %w", err)
	}

	leaf := crtPath(dir, name) // <dir>/<name>.crt
	if err := atomicWriteFile(leaf, 0o444, func(tmp string) error {
		fn := filepath.Join(tmp, filepath.Base(leaf))
		return os.WriteFile(fn, res.Certificate, 0o444)
	}); err != nil {
		return "", "", err
	}

	// >>> write issuer where the rest of your code expects it
	issuer := filepath.Join(dir, name+".issuer.crt")
	if len(res.IssuerCertificate) > 0 {
		if err := atomicWriteFile(issuer, 0o444, func(tmp string) error {
			fn := filepath.Join(tmp, filepath.Base(issuer))
			return os.WriteFile(fn, res.IssuerCertificate, 0o444)
		}); err != nil {
			return "", "", err
		}
	} else {
		// If the CA didn’t return a chain, fail fast so callers don’t try to build a fullchain and crash later.
		return "", "", fmt.Errorf("lego obtain: issuer chain missing in response")
	}

	return leaf, issuer, nil
}

// globularDNSProvider is a minimal lego DNS-01 provider that talks to your DNS service.
// Adjust endpoints to your existing DNS HTTP API:
//
//	POST {base}/acme/present  JSON: {"fqdn": "...", "value": "...", "ttl": 60}
//	POST {base}/acme/cleanup  JSON: {"fqdn": "...", "value": "..."}
//
// and return 2xx on success.
type globularDNSProvider struct {
	addr    string
	domain  string
	timeout int
	email   string
}

func (p *globularDNSProvider) Present(domain, token, keyAuth string) error {

	fqdn, value := dns01.GetRecord(domain, keyAuth)
	if p.addr == "" {
		return errors.New("dns address not configured")
	}
	c, err := dns_client.NewDnsService_Client(p.addr, "dns.DnsService")
	if err != nil {
		return fmt.Errorf("dns client: %w", err)
	}
	defer c.Close()

	tk, err := security.GenerateToken(p.timeout, c.GetMac(), "sa", "", p.email, p.domain)
	if err != nil {
		return fmt.Errorf("token: %w", err)
	}

	// Use a 60s TTL like the rest of your records.
	if err := c.SetText(tk, fqdn, []string{value}, 60); err != nil {
		return fmt.Errorf("set TXT %q: %w", fqdn, err)
	}
	deadline := time.Now().Add(120 * time.Second)
	for {
		if vals, err := c.GetText(fqdn); err == nil {
			for _, v := range vals {
				if v == value {
					goto udpcheck
				}
			}
		}
	udpcheck:
		host := p.addr
		if strings.Contains(host, ":") {
			host, _, _ = net.SplitHostPort(p.addr)
		}
		udpAddr := strings.TrimSpace(os.Getenv("GLOBULAR_DNS_UDP_ADDR"))
		if udpAddr == "" {
			udpAddr = net.JoinHostPort(host, "53")
		}
		r := &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, "udp", udpAddr)
			},
		}
		if txts, err := r.LookupTXT(context.Background(), fqdn); err == nil {
			for _, v := range txts {
				if v == value {
					return nil
				}
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for TXT %s propagation", fqdn)
		}
		time.Sleep(2 * time.Second)
	}
}

func (p *globularDNSProvider) CleanUp(domain, token, keyAuth string) error {

	fqdn, _ := dns01.GetRecord(domain, keyAuth)
	if p.addr == "" {
		return nil
	}
	c, err := dns_client.NewDnsService_Client(p.addr, "dns.DnsService")
	if err != nil {
		return fmt.Errorf("dns client: %w", err)
	}
	defer c.Close()

	tk, err := security.GenerateToken(p.timeout, c.GetMac(), "sa", "", p.email, p.domain)
	if err != nil {
		return fmt.Errorf("token: %w", err)
	}

	if err := c.RemoveText(tk, fqdn); err != nil {
		return fmt.Errorf("remove TXT %q: %w", fqdn, err)
	}
	return nil
}
