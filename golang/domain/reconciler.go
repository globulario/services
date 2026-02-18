package domain

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net/http"
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

// cachedProvider wraps a DNS provider with cache metadata to detect staleness.
type cachedProvider struct {
	provider dnsprovider.Provider
	modRev   int64     // etcd ModRevision of the provider config
	zone     string    // Zone this provider is configured for
	loadedAt time.Time // When this provider was cached
}

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

	// Provider cache (providerRef -> cached provider)
	providersMu sync.RWMutex
	providers   map[string]*cachedProvider

	// Stop channel and lifecycle
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
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
		providers:   make(map[string]*cachedProvider),
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
// Stop stops the reconciler gracefully.
// Safe to call multiple times - only the first call has effect.
func (r *Reconciler) Stop() {
	r.stopOnce.Do(func() {
		close(r.stopCh)
		r.wg.Wait()
		r.logger.Info("domain reconciler stopped")
	})
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

	// Step 1: Ensure DNS A/AAAA record exists (only if PublishExternal=true)
	// INV-DNS-EXT-1: Never publish node-specific records externally
	if spec.PublishExternal {
		if err := r.ensureDNSRecord(ctx, spec, provider); err != nil {
			return fmt.Errorf("failed to ensure DNS record: %w", err)
		}
		r.logger.Info("external DNS record published", "fqdn", spec.FQDN)
	} else {
		r.logger.Info("skipping external DNS publication (publish_external=false)", "fqdn", spec.FQDN)
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
	if spec.TargetIsAuto() {
		ip, err := r.discoverPublicIP()
		if err != nil {
			return fmt.Errorf("failed to discover public IP: %w", err)
		}
		targetIP = ip
	}

	// Parse IP to determine record type (A or AAAA)
	parsedIP, isV6, err := spec.ParsedTargetIP()
	if err != nil && !spec.TargetIsAuto() {
		return fmt.Errorf("invalid target IP: %w", err)
	}
	if parsedIP == "" {
		parsedIP = targetIP // Use resolved auto IP
	}

	// Detect IPv6 from actual IP if auto-detected
	if spec.TargetIsAuto() {
		isV6 = strings.Contains(targetIP, ":")
	}

	recordType := "A"
	if isV6 {
		recordType = "AAAA"
	}

	relativeName := spec.RelativeName()

	// Check if record already exists with correct value
	records, err := provider.GetRecords(ctx, spec.Zone, relativeName, recordType)
	if err != nil {
		r.logger.Warn("failed to query existing DNS records",
			"fqdn", spec.FQDN,
			"type", recordType,
			"error", err)
	} else {
		// Check if record already has correct value
		for _, rec := range records {
			if rec.Value == targetIP {
				r.logger.Debug("DNS record already correct",
					"fqdn", spec.FQDN,
					"type", recordType,
					"ip", targetIP)
				return nil
			}
		}
	}

	// Create/update DNS record (A or AAAA)
	r.logger.Info("updating DNS record",
		"fqdn", spec.FQDN,
		"type", recordType,
		"ip", targetIP,
		"ttl", spec.TTL)

	if isV6 {
		if err := provider.UpsertAAAA(ctx, spec.Zone, relativeName, targetIP, spec.TTL); err != nil {
			return fmt.Errorf("failed to upsert AAAA record: %w", err)
		}
	} else {
		if err := provider.UpsertA(ctx, spec.Zone, relativeName, targetIP, spec.TTL); err != nil {
			return fmt.Errorf("failed to upsert A record: %w", err)
		}
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

	// Load or create ACME account (includes private key + registration state)
	user, err := r.loadOrCreateAccount(ctx, spec.ACME.Email, domainDir)
	if err != nil {
		return fmt.Errorf("failed to load/create account: %w", err)
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

	// Ensure account is registered (idempotent - only registers if needed)
	if err := r.ensureRegistration(ctx, client, user, domainDir); err != nil {
		return fmt.Errorf("failed to ensure ACME registration: %w", err)
	}

	// Set DNS-01 challenge provider
	if err := client.Challenge.SetDNS01Provider(NewLegoProvider(provider, spec.Zone)); err != nil {
		return fmt.Errorf("failed to set DNS-01 provider: %w", err)
	}

	// Obtain certificate
	// INV-DNS-EXT-1: Support wildcard cert issuance (*.zone)
	// CRITICAL: Wildcard certs (*.zone) do NOT match apex domain (zone)
	// Solution: Request multi-domain SAN certificate with BOTH apex and wildcard
	var certDomains []string
	if spec.UseWildcardCert {
		// Request both apex domain and wildcard subdomain in same certificate
		// This allows the certificate to work for both globular.cloud AND *.globular.cloud
		certDomains = []string{spec.Zone, "*." + spec.Zone}
		r.logger.Info("requesting wildcard certificate with apex domain",
			"domains", certDomains)
	} else {
		// Request only the specific FQDN
		certDomains = []string{spec.FQDN}
	}

	request := certificate.ObtainRequest{
		Domains: certDomains,
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

// writeCertificate writes the certificate and key files.
func (r *Reconciler) writeCertificate(domainDir string, certificates *certificate.Resource) error {
	// Write certificate atomically (fullchain)
	certFile := filepath.Join(domainDir, "fullchain.pem")
	if err := writeFileAtomic(certFile, certificates.Certificate, 0644); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Write private key atomically (secure permissions)
	keyFile := filepath.Join(domainDir, "privkey.pem")
	if err := writeFileAtomic(keyFile, certificates.PrivateKey, 0600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Write issuer cert atomically (CA chain)
	issuerFile := filepath.Join(domainDir, "chain.pem")
	if err := writeFileAtomic(issuerFile, certificates.IssuerCertificate, 0644); err != nil {
		return fmt.Errorf("failed to write issuer certificate: %w", err)
	}

	// Set ownership after all files are written atomically
	if r.certUID > 0 && r.certGID > 0 {
		os.Chown(certFile, r.certUID, r.certGID)
		os.Chown(keyFile, r.certUID, r.certGID)
		os.Chown(issuerFile, r.certUID, r.certGID)
	}

	return nil
}

// getProvider loads a DNS provider from etcd cache.
func (r *Reconciler) getProvider(ctx context.Context, providerRef, zone string) (dnsprovider.Provider, error) {
	// Load provider config from etcd to get current ModRevision
	key := ProviderKey(providerRef)
	resp, err := r.etcdClient.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider config from etcd: %w", err)
	}

	if resp.Count == 0 {
		return nil, fmt.Errorf("provider %q not found", providerRef)
	}

	currentModRev := resp.Kvs[0].ModRevision

	// Check cache and validate staleness
	r.providersMu.RLock()
	cached, exists := r.providers[providerRef]
	r.providersMu.RUnlock()

	if exists {
		// Check if cache is still valid (same modRev and zone)
		if cached.modRev == currentModRev && cached.zone == zone {
			r.logger.Debug("using cached provider",
				"ref", providerRef,
				"zone", zone,
				"modRev", currentModRev)
			return cached.provider, nil
		}

		// Cache stale - provider config changed or different zone
		r.logger.Info("provider config changed, rebuilding",
			"ref", providerRef,
			"oldModRev", cached.modRev,
			"newModRev", currentModRev,
			"oldZone", cached.zone,
			"newZone", zone)
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

	// Cache it with metadata
	r.providersMu.Lock()
	r.providers[providerRef] = &cachedProvider{
		provider: provider,
		modRev:   currentModRev,
		zone:     zone,
		loadedAt: time.Now(),
	}
	r.providersMu.Unlock()

	r.logger.Info("provider cached",
		"ref", providerRef,
		"zone", zone,
		"modRev", currentModRev)

	return provider, nil
}

// discoverPublicIP discovers the node's public IP address by querying external services.
func (r *Reconciler) discoverPublicIP() (string, error) {
	// Try multiple services in order for reliability
	services := []string{
		"https://api.ipify.org",
		"https://icanhazip.com",
		"https://ifconfig.me/ip",
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	var lastErr error
	for _, service := range services {
		r.logger.Debug("discovering public IP", "service", service)

		resp, err := client.Get(service)
		if err != nil {
			r.logger.Warn("failed to query IP service", "service", service, "error", err)
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err := fmt.Errorf("HTTP %d", resp.StatusCode)
			r.logger.Warn("IP service returned non-200", "service", service, "status", resp.StatusCode)
			lastErr = err
			continue
		}

		bodyBytes := make([]byte, 256)
		n, err := resp.Body.Read(bodyBytes)
		if err != nil && n == 0 {
			r.logger.Warn("failed to read response body", "service", service, "error", err)
			lastErr = err
			continue
		}

		ip := strings.TrimSpace(string(bodyBytes[:n]))
		if ip == "" {
			err := fmt.Errorf("empty response")
			r.logger.Warn("IP service returned empty response", "service", service)
			lastErr = err
			continue
		}

		// Validate IP format
		if !strings.Contains(ip, ".") && !strings.Contains(ip, ":") {
			err := fmt.Errorf("invalid IP format: %s", ip)
			r.logger.Warn("IP service returned invalid format", "service", service, "response", ip)
			lastErr = err
			continue
		}

		r.logger.Info("discovered public IP", "ip", ip, "service", service)
		return ip, nil
	}

	if lastErr != nil {
		return "", fmt.Errorf("failed to discover public IP from all services: %w", lastErr)
	}
	return "", fmt.Errorf("failed to discover public IP: no services responded")
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

// acmeUser is now defined in acme_account.go

// loadOrCreateAccount loads an ACME account from disk or creates a new one.
// This includes both the private key and registration state to prevent re-registration loops.
func (r *Reconciler) loadOrCreateAccount(ctx context.Context, email string, dir string) (*acmeUser, error) {
	// Try to load existing account
	user, err := loadAccount(email, dir)
	if err != nil {
		return nil, fmt.Errorf("failed to load account: %w", err)
	}

	if user != nil {
		// Account found
		r.logger.Debug("loaded existing ACME account", "email", email, "registered", user.Registration != nil)
		return user, nil
	}

	// No account found - create new one
	r.logger.Info("creating new ACME account", "email", email)

	// Generate account private key
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate account key: %w", err)
	}

	user = &acmeUser{
		Email: email,
		key:   key,
	}

	// Save account (without registration yet)
	if err := saveAccount(user, dir); err != nil {
		return nil, fmt.Errorf("failed to save account: %w", err)
	}

	return user, nil
}

// ensureRegistration ensures the ACME account is registered with the CA.
// This is idempotent - if already registered, does nothing. If not registered,
// registers and saves the registration state to prevent re-registration loops.
func (r *Reconciler) ensureRegistration(ctx context.Context, client *lego.Client, user *acmeUser, dir string) error {
	// Check if already registered
	if user.Registration != nil {
		r.logger.Debug("ACME account already registered", "email", user.Email)
		return nil
	}

	// Register with CA
	r.logger.Info("registering ACME account with CA", "email", user.Email)
	reg, err := client.Registration.Register(registration.RegisterOptions{
		TermsOfServiceAgreed: true,
	})
	if err != nil {
		return fmt.Errorf("failed to register: %w", err)
	}

	// Update user with registration
	user.Registration = reg

	// Save registration state
	if err := saveAccount(user, dir); err != nil {
		return fmt.Errorf("failed to save registration: %w", err)
	}

	r.logger.Info("ACME account registered successfully", "email", user.Email)
	return nil
}
