package actions

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/globulario/services/golang/dns/dnspb"
)

const (
	acmeProductionURL = "https://acme-v02.api.letsencrypt.org/directory"
	acmeStagingURL    = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

type acmeEnsureAction struct{}

func (acmeEnsureAction) Name() string { return "tls.acme.ensure" }

func (acmeEnsureAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}
	fields := args.GetFields()
	if strings.TrimSpace(fields["domain"].GetStringValue()) == "" {
		return errors.New("domain is required")
	}
	if strings.TrimSpace(fields["admin_email"].GetStringValue()) == "" {
		return errors.New("admin_email is required")
	}
	return nil
}

func (acmeEnsureAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()

	// Check if ACME is enabled
	acmeEnabled := fields["acme_enabled"].GetBoolValue()
	if !acmeEnabled {
		return "acme disabled, skipping", nil
	}

	domain := strings.TrimSpace(fields["domain"].GetStringValue())
	adminEmail := strings.TrimSpace(fields["admin_email"].GetStringValue())
	dnsAddr := strings.TrimSpace(fields["dns_addr"].GetStringValue())
	if dnsAddr == "" {
		dnsAddr = "localhost:10033" // default DNS service address
	}

	paths := tlsPaths(args)

	// Check if cert renewal is needed
	needsRenewal, reason, err := needsCertRenewal(paths.fullchain, domain)
	if err != nil {
		return "", fmt.Errorf("check cert renewal: %w", err)
	}

	if !needsRenewal {
		return "certificate valid, no renewal needed", nil
	}

	// Perform ACME DNS-01 challenge
	certChanged, err := obtainCertificate(ctx, domain, adminEmail, dnsAddr, paths)
	if err != nil {
		return "", fmt.Errorf("obtain certificate: %w", err)
	}

	if certChanged {
		return "certificate issued/renewed", nil
	}

	return fmt.Sprintf("certificate renewal triggered: %s", reason), nil
}

func init() {
	Register(acmeEnsureAction{})
}

// needsCertRenewal checks if certificate renewal is needed
// Returns: (needsRenewal bool, reason string, error)
func needsCertRenewal(certPath, domain string) (bool, string, error) {
	// Check if cert exists
	data, err := os.ReadFile(certPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return true, "certificate missing", nil
		}
		return false, "", fmt.Errorf("read cert: %w", err)
	}

	// Parse certificate
	block, _ := pem.Decode(data)
	if block == nil {
		return true, "invalid PEM format", nil
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return true, "failed to parse certificate", nil
	}

	// Check expiration (renew if < 30 days)
	now := nowFunc()
	daysUntilExpiry := cert.NotAfter.Sub(now).Hours() / 24
	if daysUntilExpiry < 30 {
		return true, fmt.Sprintf("expires in %.0f days", daysUntilExpiry), nil
	}

	// Check SAN mismatch
	if err := cert.VerifyHostname(domain); err != nil {
		return true, "SAN mismatch", nil
	}

	return false, "", nil
}

// globularDNSProvider implements the dns01.Provider interface
type globularDNSProvider struct {
	dnsAddr string
	domain  string
}

func newGlobularDNSProvider(dnsAddr, domain string) *globularDNSProvider {
	return &globularDNSProvider{
		dnsAddr: dnsAddr,
		domain:  domain,
	}
}

// Present creates the TXT record for ACME challenge
func (p *globularDNSProvider) Present(domain, token, keyAuth string) error {
	fqdn, value := dns01.GetRecord(domain, keyAuth)

	// Remove trailing dot from fqdn
	fqdn = strings.TrimSuffix(fqdn, ".")

	// Connect to DNS service
	cc, err := grpc.Dial(p.dnsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial DNS service: %w", err)
	}
	defer cc.Close()

	client := dnspb.NewDnsServiceClient(cc)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Set TXT record
	_, err = client.SetTXT(ctx, &dnspb.SetTXTRequest{
		Domain: fqdn,
		Txt:    value,
		Ttl:    300,
	})
	if err != nil {
		return fmt.Errorf("set TXT record: %w", err)
	}

	// Wait for propagation by querying our own DNS
	return p.waitForPropagation(fqdn, value)
}

// CleanUp removes the TXT record after ACME challenge
func (p *globularDNSProvider) CleanUp(domain, token, keyAuth string) error {
	fqdn, value := dns01.GetRecord(domain, keyAuth)

	// Remove trailing dot from fqdn
	fqdn = strings.TrimSuffix(fqdn, ".")

	// Connect to DNS service
	cc, err := grpc.Dial(p.dnsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial DNS service: %w", err)
	}
	defer cc.Close()

	client := dnspb.NewDnsServiceClient(cc)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Remove specific TXT record
	_, err = client.RemoveTXT(ctx, &dnspb.RemoveTXTRequest{
		Domain: fqdn,
		Txt:    value,
	})
	if err != nil {
		return fmt.Errorf("remove TXT record: %w", err)
	}

	return nil
}

// waitForPropagation queries DNS until TXT record is visible
func (p *globularDNSProvider) waitForPropagation(fqdn, value string) error {
	cc, err := grpc.Dial(p.dnsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial DNS service: %w", err)
	}
	defer cc.Close()

	client := dnspb.NewDnsServiceClient(cc)

	// Try for up to 60 seconds
	for i := 0; i < 60; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := client.GetTXT(ctx, &dnspb.GetTXTRequest{Domain: fqdn})
		cancel()

		if err == nil {
			for _, txt := range resp.Txt {
				if txt == value {
					return nil // Found it!
				}
			}
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("TXT record not visible after 60 seconds")
}

// acmeUser implements the registration.User interface
type acmeUser struct {
	email        string
	registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string {
	return u.email
}

func (u *acmeUser) GetRegistration() *registration.Resource {
	return u.registration
}

func (u *acmeUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// obtainCertificate performs ACME certificate issuance/renewal
func obtainCertificate(ctx context.Context, domain, adminEmail, dnsAddr string, paths tlsPathsSet) (bool, error) {
	// Create or load ACME account key
	accountKey, err := getOrCreateAccountKey()
	if err != nil {
		return false, fmt.Errorf("get account key: %w", err)
	}

	user := &acmeUser{
		email: adminEmail,
		key:   accountKey,
	}

	// Create lego config
	config := lego.NewConfig(user)
	config.CADirURL = acmeProductionURL // Use production Let's Encrypt
	config.Certificate.KeyType = certcrypto.EC256

	// Create lego client
	client, err := lego.NewClient(config)
	if err != nil {
		return false, fmt.Errorf("create lego client: %w", err)
	}

	// Set up DNS-01 challenge provider
	provider := newGlobularDNSProvider(dnsAddr, domain)
	err = client.Challenge.SetDNS01Provider(provider)
	if err != nil {
		return false, fmt.Errorf("set DNS provider: %w", err)
	}

	// Register account if needed
	if user.registration == nil {
		reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return false, fmt.Errorf("register ACME account: %w", err)
		}
		user.registration = reg
	}

	// Request certificate
	request := certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return false, fmt.Errorf("obtain certificate: %w", err)
	}

	// Write certificates atomically
	if err := writeCertificatesAtomic(paths, certificates); err != nil {
		return false, fmt.Errorf("write certificates: %w", err)
	}

	return true, nil
}

// getOrCreateAccountKey loads or creates ACME account key
func getOrCreateAccountKey() (crypto.PrivateKey, error) {
	keyPath := "/etc/globular/tls/acme_account.key"

	// Try to load existing key
	data, err := os.ReadFile(keyPath)
	if err == nil {
		block, _ := pem.Decode(data)
		if block != nil {
			key, err := x509.ParseECPrivateKey(block.Bytes)
			if err == nil {
				return key, nil
			}
		}
	}

	// Create new key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	// Save key
	keyBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal key: %w", err)
	}

	pemBlock := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	}

	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
	}

	if err := os.WriteFile(keyPath, pem.EncodeToMemory(pemBlock), 0o600); err != nil {
		return nil, fmt.Errorf("write key: %w", err)
	}

	return key, nil
}

// writeCertificatesAtomic writes certificates atomically with correct permissions
func writeCertificatesAtomic(paths tlsPathsSet, certs *certificate.Resource) error {
	// Create directory with secure permissions
	dir := filepath.Dir(paths.fullchain)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	// Write fullchain atomically
	tmpCert := paths.fullchain + ".tmp"
	if err := os.WriteFile(tmpCert, certs.Certificate, 0o644); err != nil {
		return fmt.Errorf("write tmp cert: %w", err)
	}
	if err := os.Rename(tmpCert, paths.fullchain); err != nil {
		os.Remove(tmpCert)
		return fmt.Errorf("rename cert: %w", err)
	}

	// Write privkey atomically with strict permissions
	tmpKey := paths.privkey + ".tmp"
	if err := os.WriteFile(tmpKey, certs.PrivateKey, 0o600); err != nil {
		return fmt.Errorf("write tmp key: %w", err)
	}
	if err := os.Rename(tmpKey, paths.privkey); err != nil {
		os.Remove(tmpKey)
		return fmt.Errorf("rename key: %w", err)
	}

	return nil
}

// CertFingerprint computes SHA256 fingerprint of certificate for change detection
func CertFingerprint(certPath string) (string, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
