package domain

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/globulario/services/golang/dnsprovider"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Reconciler manages the lifecycle of external domains:
// 1. DNS records (via dnsprovider)
// 2. ACME certificates (via DNS-01 challenges)
// 3. Ingress configuration (triggers xDS rebuild)
type Reconciler struct {
	etcdClient *clientv3.Client
	store      DomainStore
	logger     *slog.Logger

	// Base directory for domain certificates
	// Default: /var/lib/globular/domains
	certsDir string

	// File ownership (UID:GID) for certificate files
	// Default: globular user (detect from /var/lib/globular ownership)
	certUID int
	certGID int

	// Reconciliation interval
	interval time.Duration

	// Certificate renewal threshold (renew if expires < this)
	renewBefore time.Duration

	// Provider cache (zone -> provider)
	providersMu sync.RWMutex
	providers   map[string]dnsprovider.Provider

	// Stop channel
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// ReconcilerConfig configures the reconciler.
type ReconcilerConfig struct {
	EtcdClient  *clientv3.Client
	Logger      *slog.Logger
	CertsDir    string
	CertUID     int // Certificate file owner UID (0 = auto-detect)
	CertGID     int // Certificate file owner GID (0 = auto-detect)
	Interval    time.Duration
	RenewBefore time.Duration
}

// NewReconciler creates a new domain reconciler.
func NewReconciler(cfg ReconcilerConfig) (*Reconciler, error) {
	if cfg.EtcdClient == nil {
		return nil, fmt.Errorf("etcd client is required")
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	certsDir := cfg.CertsDir
	if certsDir == "" {
		certsDir = "/var/lib/globular/domains"
	}

	interval := cfg.Interval
	if interval == 0 {
		interval = 5 * time.Minute // Default: check every 5 minutes
	}

	renewBefore := cfg.RenewBefore
	if renewBefore == 0 {
		renewBefore = 30 * 24 * time.Hour // Default: renew 30 days before expiry
	}

	certUID := cfg.CertUID
	certGID := cfg.CertGID
	if certUID == 0 || certGID == 0 {
		// Auto-detect from /var/lib/globular ownership
		if info, err := os.Stat("/var/lib/globular"); err == nil {
			if stat, ok := info.Sys().(*syscall.Stat_t); ok {
				certUID = int(stat.Uid)
				certGID = int(stat.Gid)
			}
		}
	}

	store := NewEtcdDomainStore(cfg.EtcdClient)

	return &Reconciler{
		etcdClient:  cfg.EtcdClient,
		store:       store,
		logger:      logger,
		certsDir:    certsDir,
		certUID:     certUID,
		certGID:     certGID,
		interval:    interval,
		renewBefore: renewBefore,
		providers:   make(map[string]dnsprovider.Provider),
		stopCh:      make(chan struct{}),
	}, nil
}

// Start begins the reconciliation loop.
func (r *Reconciler) Start(ctx context.Context) error {
	r.logger.Info("starting domain reconciler",
		"interval", r.interval,
		"certs_dir", r.certsDir,
		"renew_before", r.renewBefore)

	// Ensure certs directory exists
	if err := os.MkdirAll(r.certsDir, 0755); err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	r.wg.Add(1)
	go r.reconcileLoop(ctx)

	return nil
}

// Stop gracefully stops the reconciler.
func (r *Reconciler) Stop() {
	close(r.stopCh)
	r.wg.Wait()
	r.logger.Info("domain reconciler stopped")
}

// reconcileLoop runs the reconciliation loop.
func (r *Reconciler) reconcileLoop(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Run once immediately
	r.reconcileAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.reconcileAll(ctx)
		}
	}
}

// reconcileAll reconciles all domain specs in etcd.
func (r *Reconciler) reconcileAll(ctx context.Context) {
	r.logger.Debug("starting reconciliation pass")

	// Get all domain specs from store
	specs, err := r.store.ListSpecs(ctx)
	if err != nil {
		r.logger.Error("failed to list domains from store", "error", err)
		return
	}

	if len(specs) == 0 {
		r.logger.Debug("no domains to reconcile")
		return
	}

	// Reconcile each domain
	for _, spec := range specs {
		if err := r.reconcileDomain(ctx, spec); err != nil {
			r.logger.Error("failed to reconcile domain",
				"fqdn", spec.FQDN,
				"error", err)

			// Update status to error
			_ = r.setStatusError(ctx, spec.FQDN, err)
		}
	}

	r.logger.Debug("reconciliation pass complete", "domains", len(specs))
}

// reconcileDomain reconciles a single domain spec.
func (r *Reconciler) reconcileDomain(ctx context.Context, spec *ExternalDomainSpec) error {
	r.logger.Info("reconciling domain",
		"fqdn", spec.FQDN,
		"zone", spec.Zone,
		"provider", spec.ProviderRef)

	// Validate spec
	if err := spec.Validate(); err != nil {
		return fmt.Errorf("invalid spec: %w", err)
	}

	// Load DNS provider
	provider, err := r.getProvider(ctx, spec.ProviderRef, spec.Zone)
	if err != nil {
		return fmt.Errorf("failed to load provider: %w", err)
	}

	// Step 1: Ensure DNS A/AAAA record exists
	if err := r.ensureDNSRecord(ctx, spec, provider); err != nil {
		return fmt.Errorf("failed to ensure DNS record: %w", err)
	}

	// Step 2: If ACME enabled, ensure certificate exists and is valid
	if spec.ACME.Enabled {
		if err := r.ensureCertificate(ctx, spec, provider); err != nil {
			return fmt.Errorf("failed to ensure certificate: %w", err)
		}
	}

	// Step 3: Update status to Ready
	if err := r.setStatus(ctx, spec.FQDN, "Ready", "Domain reconciled successfully"); err != nil {
		return fmt.Errorf("failed to set status: %w", err)
	}

	return nil
}

// ensureDNSRecord ensures the DNS A/AAAA record exists.
func (r *Reconciler) ensureDNSRecord(ctx context.Context, spec *ExternalDomainSpec, provider dnsprovider.Provider) error {
	targetIP := spec.TargetIP

	// Resolve "auto" to actual public IP
	if targetIP == "auto" {
		ip, err := r.discoverPublicIP()
		if err != nil {
			return fmt.Errorf("failed to discover public IP: %w", err)
		}
		targetIP = ip
	}

	relativeName := spec.RelativeName()

	// Check if record already exists with correct value
	records, err := provider.GetRecords(ctx, spec.Zone, relativeName, "A")
	if err != nil {
		r.logger.Warn("failed to query existing DNS records",
			"fqdn", spec.FQDN,
			"error", err)
	} else {
		// Check if record already has correct value
		for _, rec := range records {
			if rec.Value == targetIP {
				r.logger.Debug("DNS record already correct",
					"fqdn", spec.FQDN,
					"ip", targetIP)
				return nil
			}
		}
	}

	// Create/update DNS A record
	r.logger.Info("updating DNS record",
		"fqdn", spec.FQDN,
		"ip", targetIP,
		"ttl", spec.TTL)

	if err := provider.UpsertA(ctx, spec.Zone, relativeName, targetIP, spec.TTL); err != nil {
		return fmt.Errorf("failed to upsert A record: %w", err)
	}

	return nil
}

// ensureCertificate ensures the ACME certificate exists and is valid.
func (r *Reconciler) ensureCertificate(ctx context.Context, spec *ExternalDomainSpec, provider dnsprovider.Provider) error {
	domainDir := filepath.Join(r.certsDir, spec.FQDN)
	certFile := filepath.Join(domainDir, "fullchain.pem")

	// Check if certificate exists and is still valid
	if r.isCertificateValid(certFile, spec.FQDN) {
		r.logger.Debug("certificate still valid", "fqdn", spec.FQDN)
		return nil
	}

	// Need to obtain/renew certificate
	r.logger.Info("obtaining ACME certificate",
		"fqdn", spec.FQDN,
		"email", spec.ACME.Email,
		"challenge", spec.ACME.ChallengeType)

	// Create domain directory
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		return fmt.Errorf("failed to create domain directory: %w", err)
	}

	// Create ACME account key if doesn't exist
	accountKeyFile := filepath.Join(domainDir, "account.key")
	accountKey, err := r.loadOrCreateAccountKey(accountKeyFile)
	if err != nil {
		return fmt.Errorf("failed to load account key: %w", err)
	}

	// Create ACME user
	user := &acmeUser{
		Email: spec.ACME.Email,
		key:   accountKey,
	}

	// Create lego config
	config := lego.NewConfig(user)
	config.Certificate.KeyType = certcrypto.EC256

	// Set directory URL
	switch strings.ToLower(spec.ACME.Directory) {
	case "", "prod", "production":
		// Default: Let's Encrypt production
	case "staging":
		config.CADirURL = lego.LEDirectoryStaging
	default:
		config.CADirURL = spec.ACME.Directory
	}

	// Create lego client
	client, err := lego.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create ACME client: %w", err)
	}

	// Register account if new
	if user.Registration == nil {
		reg, err := client.Registration.Register(registration.RegisterOptions{
			TermsOfServiceAgreed: true,
		})
		if err != nil {
			return fmt.Errorf("failed to register ACME account: %w", err)
		}
		user.Registration = reg
	}

	// Set DNS-01 challenge provider
	if err := client.Challenge.SetDNS01Provider(NewLegoProvider(provider, spec.Zone)); err != nil {
		return fmt.Errorf("failed to set DNS-01 provider: %w", err)
	}

	// Obtain certificate
	request := certificate.ObtainRequest{
		Domains: []string{spec.FQDN},
		Bundle:  true,
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return fmt.Errorf("failed to obtain certificate: %w", err)
	}

	// Write certificate files
	if err := r.writeCertificate(domainDir, certificates); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	r.logger.Info("certificate obtained successfully",
		"fqdn", spec.FQDN,
		"cert_file", certFile)

	return nil
}

// isCertificateValid checks if a certificate file exists and is valid.
func (r *Reconciler) isCertificateValid(certFile string, domain string) bool {
	// Read certificate
	data, err := os.ReadFile(certFile)
	if err != nil {
		return false
	}

	// Parse PEM
	block, _ := pem.Decode(data)
	if block == nil {
		return false
	}

	// Parse certificate
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}

	// Check domain matches
	if err := cert.VerifyHostname(domain); err != nil {
		r.logger.Warn("certificate domain mismatch",
			"expected", domain,
			"cert_cn", cert.Subject.CommonName)
		return false
	}

	// Check expiry (renew if expires within renewBefore threshold)
	timeUntilExpiry := time.Until(cert.NotAfter)
	if timeUntilExpiry < r.renewBefore {
		r.logger.Info("certificate needs renewal",
			"fqdn", domain,
			"expires", cert.NotAfter,
			"days_remaining", int(timeUntilExpiry.Hours()/24))
		return false
	}

	return true
}

// loadOrCreateAccountKey loads or creates an ACME account key.
func (r *Reconciler) loadOrCreateAccountKey(keyFile string) (crypto.PrivateKey, error) {
	// Try to load existing key
	if data, err := os.ReadFile(keyFile); err == nil {
		block, _ := pem.Decode(data)
		if block != nil {
			if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
				return key, nil
			}
		}
	}

	// Generate new key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	// Marshal to DER
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}

	// Write PEM file
	pemBlock := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: der,
	}

	if err := os.WriteFile(keyFile, pem.EncodeToMemory(pemBlock), 0600); err != nil {
		return nil, err
	}

	// Set ownership if configured
	if r.certUID > 0 && r.certGID > 0 {
		os.Chown(keyFile, r.certUID, r.certGID)
	}

	return key, nil
}

// writeCertificate writes the certificate and key files.
func (r *Reconciler) writeCertificate(domainDir string, certificates *certificate.Resource) error {
	// Write certificate (fullchain)
	certFile := filepath.Join(domainDir, "fullchain.pem")
	if err := os.WriteFile(certFile, certificates.Certificate, 0644); err != nil {
		return err
	}

	// Write private key
	keyFile := filepath.Join(domainDir, "privkey.pem")
	if err := os.WriteFile(keyFile, certificates.PrivateKey, 0600); err != nil {
		return err
	}

	// Write issuer cert (CA)
	issuerFile := filepath.Join(domainDir, "chain.pem")
	if err := os.WriteFile(issuerFile, certificates.IssuerCertificate, 0644); err != nil {
		return err
	}

	// Set ownership if configured
	if r.certUID > 0 && r.certGID > 0 {
		os.Chown(certFile, r.certUID, r.certGID)
		os.Chown(keyFile, r.certUID, r.certGID)
		os.Chown(issuerFile, r.certUID, r.certGID)
	}

	return nil
}

// getProvider loads a DNS provider from etcd cache.
func (r *Reconciler) getProvider(ctx context.Context, providerRef, zone string) (dnsprovider.Provider, error) {
	// Check cache first
	r.providersMu.RLock()
	if provider, exists := r.providers[providerRef]; exists {
		r.providersMu.RUnlock()
		return provider, nil
	}
	r.providersMu.RUnlock()

	// Load from etcd
	key := ProviderKey(providerRef)
	resp, err := r.etcdClient.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider config from etcd: %w", err)
	}

	if resp.Count == 0 {
		return nil, fmt.Errorf("provider %q not found", providerRef)
	}

	// Parse provider config
	var cfg dnsprovider.Config
	if err := json.Unmarshal(resp.Kvs[0].Value, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse provider config: %w", err)
	}

	// Create provider
	provider, err := dnsprovider.NewProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	// Cache it
	r.providersMu.Lock()
	r.providers[providerRef] = provider
	r.providersMu.Unlock()

	return provider, nil
}

// discoverPublicIP discovers the node's public IP address.
func (r *Reconciler) discoverPublicIP() (string, error) {
	// TODO: Implement public IP discovery
	// Options:
	// 1. Query external service (e.g., icanhazip.com, ipify.org)
	// 2. Read from node metadata/config
	// 3. Use configured static IP

	return "", fmt.Errorf("public IP discovery not implemented yet (use explicit --target-ip)")
}

// setStatus updates the domain status in etcd using the separate status key.
// This prevents concurrent updates from overwriting the user spec.
func (r *Reconciler) setStatus(ctx context.Context, fqdn string, phase string, message string) error {
	status := &ExternalDomainStatus{
		LastReconcile: time.Now(),
		Phase:         phase,
		Message:       message,
	}

	if err := r.store.PutStatus(ctx, fqdn, status); err != nil {
		r.logger.Error("failed to update domain status", "fqdn", fqdn, "error", err)
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

// setStatusError updates the domain status to Error phase with the given error message.
func (r *Reconciler) setStatusError(ctx context.Context, fqdn string, err error) error {
	return r.setStatus(ctx, fqdn, "Error", err.Error())
}

// acmeUser implements the lego registration.User interface.
type acmeUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string {
	return u.Email
}

func (u *acmeUser) GetRegistration() *registration.Resource {
	return u.Registration
}

func (u *acmeUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}
