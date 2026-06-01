// @awareness namespace=globular.platform
// @awareness component=platform_domain
// @awareness file_role=acme_certificate_reconciler
// @awareness implements=globular.platform:intent.dns_pki.explicit_identity_over_convenient_routing
// @awareness risk=high
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
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/dnsprovider"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	// isLeader, if set, gates each reconciliation pass to leader-only.
	isLeader func() bool

	// Stop channel and lifecycle
	stopCh    chan struct{}
	stopOnce  sync.Once
	wg        sync.WaitGroup
	triggerCh chan struct{} // Signals an immediate reconcile pass
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
	// IsLeader, when set, is called before each reconciliation pass.
	// If it returns false the pass is skipped. Restricts ACME and DNS
	// mutations to the cluster-controller leader so non-leader nodes do
	// not race to set domain status or attempt DNS-01 challenges.
	IsLeader func() bool
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
		isLeader:    cfg.IsLeader,
		stopCh:      make(chan struct{}),
		triggerCh:   make(chan struct{}, 1),
	}, nil
}

// Trigger schedules an immediate reconcile pass without waiting for the next
// ticker tick. Safe to call from any goroutine; excess signals are dropped.
func (r *Reconciler) Trigger() {
	select {
	case r.triggerCh <- struct{}{}:
	default:
	}
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

	r.wg.Add(2)
	go r.reconcileLoop(ctx)
	go r.watchSpecChanges(ctx)

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
		case <-r.triggerCh:
			r.reconcileAll(ctx)
		}
	}
}

// watchSpecChanges watches etcd for new or updated domain specs and triggers
// an immediate reconcile pass so certificates are issued within seconds of save.
func (r *Reconciler) watchSpecChanges(ctx context.Context) {
	defer r.wg.Done()
	if r.etcdClient == nil {
		return
	}
	watchCh := r.etcdClient.Watch(ctx, EtcdDomainPrefix, clientv3.WithPrefix())
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case resp, ok := <-watchCh:
			if !ok {
				return
			}
			if resp.Err() != nil {
				continue
			}
			for _, ev := range resp.Events {
				if ev.Type == clientv3.EventTypePut {
					r.logger.Info("domain spec change detected — triggering reconcile",
						"key", string(ev.Kv.Key))
					r.Trigger()
					break
				}
			}
		}
	}
}

// reconcileAll reconciles all domain specs in etcd.
func (r *Reconciler) reconcileAll(ctx context.Context) {
	if r.isLeader != nil && !r.isLeader() {
		r.logger.Debug("domain reconciler: skipping pass (not leader)")
		return
	}
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

	// Reconcile each domain (retry transient gRPC errors up to 3 times)
	for _, spec := range specs {
		var lastErr error
		for attempt := 0; attempt < 3; attempt++ {
			if err := r.reconcileDomain(ctx, spec); err != nil {
				lastErr = err
				if isTransientGRPCError(err) && attempt < 2 {
					r.logger.Warn("transient error reconciling domain, retrying",
						"fqdn", spec.FQDN,
						"attempt", attempt+1,
						"error", err)
					time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
					continue
				}
				r.logger.Error("failed to reconcile domain",
					"fqdn", spec.FQDN,
					"error", err)
				_ = r.setStatusError(ctx, spec.FQDN, err)
			} else {
				lastErr = nil
			}
			break
		}
		_ = lastErr // used in loop
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

	// Load DNS provider (optional when using manual cert management)
	var provider dnsprovider.Provider
	if spec.ProviderRef != "" {
		var err error
		provider, err = r.getProvider(ctx, spec.ProviderRef, spec.Zone)
		if err != nil {
			return fmt.Errorf("failed to load provider: %w", err)
		}
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
		// Ensure the xDS symlink exists so Envoy can find the cert.
		// The reconciler writes to /var/lib/globular/domains/{fqdn}/,
		// but xDS reads from /var/lib/globular/config/tls/acme/{fqdn}/.
		// This symlink bridges the two paths and is idempotent.
		if err := r.ensureXDSSymlink(spec.FQDN); err != nil {
			// Log but don't fail — cert is valid, only the xDS path is missing.
			r.logger.Warn("failed to create xDS TLS symlink", "fqdn", spec.FQDN, "error", err)
		}
	}

	// Step 3: Update status to Ready (with cert expiry and current IP)
	status := &ExternalDomainStatus{
		LastReconcile: time.Now(),
		Phase:         "Ready",
		Message:       "Domain reconciled successfully",
		CurrentIP:     spec.TargetIP,
	}
	// Read cert expiry from disk
	if exp := r.readCertExpiry(spec.FQDN); exp != nil {
		status.CertExpiry = exp
	}
	// Resolve actual IP if "auto"
	if spec.TargetIP == "auto" || spec.TargetIP == "" {
		if ip := r.resolvePublicIP(); ip != "" {
			status.CurrentIP = ip
		}
	}
	if err := r.store.PutStatus(ctx, spec.FQDN, status); err != nil {
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

	// When using a wildcard certificate, also create a wildcard A/AAAA record
	// so that all subdomains (e.g. globule-ryzen.globular.cloud) resolve.
	if spec.UseWildcardCert {
		wildcardName := "*"
		r.logger.Info("updating wildcard DNS record",
			"zone", spec.Zone,
			"type", recordType,
			"ip", targetIP,
			"ttl", spec.TTL)
		if isV6 {
			if err := provider.UpsertAAAA(ctx, spec.Zone, wildcardName, targetIP, spec.TTL); err != nil {
				return fmt.Errorf("failed to upsert wildcard AAAA record: %w", err)
			}
		} else {
			if err := provider.UpsertA(ctx, spec.Zone, wildcardName, targetIP, spec.TTL); err != nil {
				return fmt.Errorf("failed to upsert wildcard A record: %w", err)
			}
		}
	}

	// Create standard service records: api.<zone>, dns.<zone>, and <node_id>.<zone>
	// dns.<zone> uses the public IP so Let's Encrypt can reach the NS during
	// ACME DNS-01 challenges (the registrar's NS glue record points here).
	for _, sub := range []string{"api", "dns", spec.NodeID} {
		if sub == "" {
			continue
		}
		if isV6 {
			_ = provider.UpsertAAAA(ctx, spec.Zone, sub, targetIP, spec.TTL)
		} else {
			_ = provider.UpsertA(ctx, spec.Zone, sub, targetIP, spec.TTL)
		}
	}

	return nil
}

// RenewRequestedMarker is the filename the admin handler writes to signal
// a forced renewal. Must match the constant in the admin handler package.
const RenewRequestedMarker = ".renew-requested"

// ensureCertificate ensures the ACME certificate exists and is valid.
//
// Safety: new certificates are obtained into a staging subdirectory first.
// Only after successful validation are the files atomically swapped into
// the live directory. Old certificates remain on disk throughout the
// entire ACME cycle — there is never a gap where no cert is present.
//
// When a .renew-requested marker file is present (written by the admin
// handler), renewal is forced even if the current cert is still valid.
// The marker is removed only after the new cert is successfully installed.
func (r *Reconciler) ensureCertificate(ctx context.Context, spec *ExternalDomainSpec, provider dnsprovider.Provider) error {
	domainDir := filepath.Join(r.certsDir, spec.FQDN)
	certFile := filepath.Join(domainDir, "fullchain.pem")
	markerFile := filepath.Join(domainDir, RenewRequestedMarker)

	// Check for forced renewal marker
	forceRenew := false
	if _, err := os.Stat(markerFile); err == nil {
		forceRenew = true
		r.logger.Info("forced renewal requested via marker",
			"fqdn", spec.FQDN,
			"marker", markerFile)
	}

	// Check if certificate exists and is still valid (skip if forced)
	if !forceRenew && r.isCertificateValid(certFile, spec.FQDN, spec.UseWildcardCert, spec.Zone) {
		r.logger.Debug("certificate still valid", "fqdn", spec.FQDN)
		// Ensure cert is also in etcd (idempotent — may already be there).
		// This covers the case where the cert was obtained before the etcd
		// distribution was implemented, or if etcd was wiped.
		r.publishCertToEtcd(spec.FQDN, domainDir)
		return nil
	}

	// Also handle legacy .renew-backup files from older handler versions:
	// if fullchain.pem is missing but a .renew-backup exists, restore it
	// first so there's no gap while we obtain the new one.
	backupFile := certFile + ".renew-backup"
	if _, err := os.Stat(certFile); os.IsNotExist(err) {
		if _, err := os.Stat(backupFile); err == nil {
			r.logger.Info("restoring legacy backup cert while renewing", "fqdn", spec.FQDN)
			_ = os.Rename(backupFile, certFile)
		}
	}

	// Need to obtain/renew certificate
	r.logger.Info("obtaining ACME certificate",
		"fqdn", spec.FQDN,
		"email", spec.ACME.Email,
		"challenge", spec.ACME.ChallengeType,
		"forced", forceRenew)

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

	// Set DNS-01 challenge provider.
	// Always disable lego's built-in authoritative-NS propagation check: lego
	// queries the NS record for the zone, resolves the nameserver to a local IP,
	// and tries UDP port 53 — which refuses connections inside the cluster.
	// Our solver's own waitForPropagation handles verification correctly:
	//   - publish_external=true  → public resolvers (1.1.1.1, 8.8.8.8) — TXT is
	//                              visible externally so LE validation will succeed.
	//   - publish_external=false → provider API query (internal zone, no public check).
	legoProvider := NewLegoProvider(provider, spec.Zone)
	if spec.PublishExternal {
		legoProvider.solver.SetPropagator(NewPublicResolverPropagator())
		r.logger.Info("using public resolvers for ACME propagation check",
			"fqdn", spec.FQDN)
	}
	if err := client.Challenge.SetDNS01Provider(legoProvider, dns01.DisableCompletePropagationRequirement()); err != nil {
		return fmt.Errorf("failed to set DNS-01 provider: %w", err)
	}

	// Obtain certificate
	// INV-DNS-EXT-1: Support wildcard cert issuance (*.zone)
	// IMPORTANT: Wildcard certificates (*.zone) do NOT cover the apex domain (zone)!
	// We must request BOTH the apex domain AND the wildcard in the same certificate.
	var certDomains []string
	if spec.UseWildcardCert {
		certDomains = []string{spec.Zone, "*." + spec.Zone}
		r.logger.Info("requesting wildcard certificate with apex domain",
			"domains", certDomains)
	} else {
		certDomains = []string{spec.FQDN}
	}

	request := certificate.ObtainRequest{
		Domains: certDomains,
		Bundle:  true,
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		// If the ACME account no longer exists on the CA (stale from a previous install),
		// delete the local account, re-register, and retry once.
		if strings.Contains(err.Error(), "accountDoesNotExist") {
			r.logger.Warn("ACME account stale — re-registering", "fqdn", spec.FQDN)
			os.Remove(filepath.Join(domainDir, "account.json"))

			user, err = r.loadOrCreateAccount(ctx, spec.ACME.Email, domainDir)
			if err != nil {
				return fmt.Errorf("failed to recreate account after stale detect: %w", err)
			}
			config = lego.NewConfig(user)
			config.Certificate.KeyType = certcrypto.EC256
			client, err = lego.NewClient(config)
			if err != nil {
				return fmt.Errorf("failed to recreate ACME client: %w", err)
			}
			if err := r.ensureRegistration(ctx, client, user, domainDir); err != nil {
				return fmt.Errorf("failed to re-register ACME account: %w", err)
			}
			if err := client.Challenge.SetDNS01Provider(legoProvider, dns01.DisableCompletePropagationRequirement()); err != nil {
				return fmt.Errorf("failed to set DNS-01 provider on retry: %w", err)
			}
			certificates, err = client.Certificate.Obtain(request)
			if err != nil {
				r.logger.Error("ACME obtain failed on retry — old certificate remains active",
					"fqdn", spec.FQDN, "error", err)
				return fmt.Errorf("failed to obtain certificate: %w", err)
			}
		} else {
			r.logger.Error("ACME obtain failed — old certificate remains active",
				"fqdn", spec.FQDN,
				"error", err)
			return fmt.Errorf("failed to obtain certificate: %w", err)
		}
	}

	// Stage new certificate into a temporary directory, validate, then swap.
	if err := r.stageThenSwapCertificate(domainDir, spec.FQDN, spec.UseWildcardCert, spec.Zone, certificates); err != nil {
		return fmt.Errorf("failed to stage certificate: %w", err)
	}

	// Success — remove the renewal marker and any legacy backup
	_ = os.Remove(markerFile)
	_ = os.Remove(backupFile)

	r.logger.Info("certificate obtained and installed successfully",
		"fqdn", spec.FQDN,
		"cert_file", certFile,
		"forced", forceRenew)

	return nil
}

// ensureXDSSymlink creates (or repairs) the symlink that Envoy's xDS server
// uses to locate ACME certificates. The reconciler writes certs to:
//
//	/var/lib/globular/domains/{fqdn}/
//
// xDS reads from:
//
//	/var/lib/globular/config/tls/acme/{fqdn}/
//
// This method creates the latter as a symlink to the former, bridging the two
// paths. It is idempotent: if a correct symlink already exists, it is a no-op.
func (r *Reconciler) ensureXDSSymlink(fqdn string) error {
	const xdsACMEBase = "/var/lib/globular/config/tls/acme"
	target := filepath.Join(r.certsDir, fqdn)   // e.g. /var/lib/globular/domains/foo.com
	link := filepath.Join(xdsACMEBase, fqdn)     // e.g. /var/lib/globular/config/tls/acme/foo.com

	// Ensure parent directory exists.
	if err := os.MkdirAll(xdsACMEBase, 0o755); err != nil {
		return fmt.Errorf("create xds acme dir: %w", err)
	}

	// Check existing entry.
	existing, err := os.Readlink(link)
	if err == nil {
		if existing == target {
			return nil // already correct
		}
		// Symlink points somewhere else — remove and recreate.
		if removeErr := os.Remove(link); removeErr != nil {
			return fmt.Errorf("remove stale xds symlink: %w", removeErr)
		}
	} else if !os.IsNotExist(err) {
		// Readlink fails on non-symlinks too; remove and let symlink win.
		if removeErr := os.Remove(link); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("remove non-symlink at xds path: %w", removeErr)
		}
	}

	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("create xds symlink %s -> %s: %w", link, target, err)
	}

	r.logger.Info("created xDS TLS symlink", "link", link, "target", target)
	return nil
}

// stageThenSwapCertificate writes new cert files to a staging subdirectory,
// validates the certificate is parseable and matches the expected domain,
// then atomically swaps each file into the live directory.
// If anything fails, the staging dir is cleaned up and old certs are untouched.
func (r *Reconciler) stageThenSwapCertificate(domainDir, fqdn string, useWildcard bool, zone string, certs *certificate.Resource) error {
	stagingDir := filepath.Join(domainDir, ".staging")

	// Clean up any leftover staging dir from a previous failed attempt
	_ = os.RemoveAll(stagingDir)

	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return fmt.Errorf("failed to create staging directory: %w", err)
	}

	// Ensure staging dir is cleaned up on failure
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	// Write to staging directory
	if err := writeFileAtomic(filepath.Join(stagingDir, "fullchain.pem"), certs.Certificate, 0644); err != nil {
		return fmt.Errorf("failed to write staged certificate: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(stagingDir, "privkey.pem"), certs.PrivateKey, 0600); err != nil {
		return fmt.Errorf("failed to write staged private key: %w", err)
	}
	if err := writeFileAtomic(filepath.Join(stagingDir, "chain.pem"), certs.IssuerCertificate, 0644); err != nil {
		return fmt.Errorf("failed to write staged issuer certificate: %w", err)
	}

	// Validate the staged certificate before swapping
	stagedCert := filepath.Join(stagingDir, "fullchain.pem")
	if !r.isCertificateValid(stagedCert, fqdn, useWildcard, zone) {
		return fmt.Errorf("staged certificate failed validation for %s", fqdn)
	}

	r.logger.Info("staged certificate validated, swapping into place",
		"fqdn", fqdn,
		"staging_dir", stagingDir)

	// Atomically swap each file from staging into the live directory
	files := []struct {
		name string
		perm os.FileMode
	}{
		{"fullchain.pem", 0644},
		{"privkey.pem", 0600},
		{"chain.pem", 0644},
	}

	for _, f := range files {
		src := filepath.Join(stagingDir, f.name)
		dst := filepath.Join(domainDir, f.name)

		// Read the staged file content
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("failed to read staged %s: %w", f.name, err)
		}

		// Atomic write to the live path (temp file + rename)
		if err := writeFileAtomic(dst, data, f.perm); err != nil {
			return fmt.Errorf("failed to swap %s into place: %w", f.name, err)
		}

		// Set ownership
		if r.certUID > 0 && r.certGID > 0 {
			_ = os.Chown(dst, r.certUID, r.certGID)
		}
	}

	success = true

	// Clean up staging directory
	_ = os.RemoveAll(stagingDir)

	// Publish certificate to etcd so all gateway nodes can access it.
	// Fire-and-forget: if etcd write fails, the local cert is still valid
	// and the next reconcile cycle will retry.
	r.publishCertToEtcd(fqdn, domainDir)

	return nil
}

// ---------------------------------------------------------------------------
// etcd cert distribution
// ---------------------------------------------------------------------------

const acmeCertEtcdPrefix = "/globular/acme/certs/"

// acmeCertBundle holds the PEM-encoded cert files for etcd storage.
type acmeCertBundle struct {
	Fullchain string `json:"fullchain"`
	Privkey   string `json:"privkey"`
	Chain     string `json:"chain"`
	UpdatedAt string `json:"updated_at"`
}

// publishCertToEtcd writes the ACME certificate to etcd so every node in
// the cluster can sync it locally. The key is /globular/acme/certs/<fqdn>.
func (r *Reconciler) publishCertToEtcd(fqdn, domainDir string) {
	fullchain, err := os.ReadFile(filepath.Join(domainDir, "fullchain.pem"))
	if err != nil {
		r.logger.Warn("acme-etcd: failed to read fullchain.pem", "fqdn", fqdn, "error", err)
		return
	}
	privkey, err := os.ReadFile(filepath.Join(domainDir, "privkey.pem"))
	if err != nil {
		r.logger.Warn("acme-etcd: failed to read privkey.pem", "fqdn", fqdn, "error", err)
		return
	}
	chain, _ := os.ReadFile(filepath.Join(domainDir, "chain.pem")) // optional

	bundle := acmeCertBundle{
		Fullchain: string(fullchain),
		Privkey:   string(privkey),
		Chain:     string(chain),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		r.logger.Warn("acme-etcd: marshal failed", "fqdn", fqdn, "error", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := acmeCertEtcdPrefix + fqdn
	if _, err := r.etcdClient.Put(ctx, key, string(data)); err != nil {
		r.logger.Warn("acme-etcd: failed to publish cert", "fqdn", fqdn, "error", err)
		return
	}
	r.logger.Info("acme-etcd: certificate published to etcd", "fqdn", fqdn)
}

// SyncACMECertsFromEtcd reads all ACME certificates from etcd and writes
// them to local disk. Call this on startup on every gateway node.
//
// Certs are written to:
//   - /var/lib/globular/pki/acme/<fqdn>/fullchain.pem
//   - /var/lib/globular/pki/acme/<fqdn>/privkey.pem
//   - /var/lib/globular/pki/acme/<fqdn>/chain.pem
//
// Additionally creates the xDS symlink at:
//   - /var/lib/globular/config/tls/acme/<fqdn>/ → pki/acme/<fqdn>/
//
// Also writes a backup copy to /var/lib/globular/domains/<fqdn>/ so the
// domain reconciler's cert-valid check works on non-leader nodes.
func SyncACMECertsFromEtcd(etcdClient *clientv3.Client, logger *slog.Logger) error {
	if etcdClient == nil {
		return fmt.Errorf("no etcd client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := etcdClient.Get(ctx, acmeCertEtcdPrefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("acme-sync: etcd get failed: %w", err)
	}

	for _, kv := range resp.Kvs {
		fqdn := strings.TrimPrefix(string(kv.Key), acmeCertEtcdPrefix)
		if fqdn == "" {
			continue
		}

		var bundle acmeCertBundle
		if err := json.Unmarshal(kv.Value, &bundle); err != nil {
			logger.Warn("acme-sync: bad cert bundle", "fqdn", fqdn, "error", err)
			continue
		}

		if err := writeACMECertLocally(fqdn, &bundle, logger); err != nil {
			logger.Warn("acme-sync: write failed", "fqdn", fqdn, "error", err)
			continue
		}
		logger.Info("acme-sync: cert synced", "fqdn", fqdn, "updated_at", bundle.UpdatedAt)
	}

	return nil
}

// WatchACMECerts watches etcd for ACME cert changes and syncs them locally.
// Blocks until ctx is cancelled. Run as a goroutine on each gateway node.
func WatchACMECerts(ctx context.Context, etcdClient *clientv3.Client, logger *slog.Logger) {
	if etcdClient == nil {
		return
	}

	wch := etcdClient.Watch(ctx, acmeCertEtcdPrefix, clientv3.WithPrefix())
	for wresp := range wch {
		for _, ev := range wresp.Events {
			if ev.Type != clientv3.EventTypePut {
				continue
			}
			fqdn := strings.TrimPrefix(string(ev.Kv.Key), acmeCertEtcdPrefix)
			if fqdn == "" {
				continue
			}

			var bundle acmeCertBundle
			if err := json.Unmarshal(ev.Kv.Value, &bundle); err != nil {
				logger.Warn("acme-watch: bad cert bundle", "fqdn", fqdn, "error", err)
				continue
			}

			if err := writeACMECertLocally(fqdn, &bundle, logger); err != nil {
				logger.Warn("acme-watch: write failed", "fqdn", fqdn, "error", err)
				continue
			}
			logger.Info("acme-watch: cert updated", "fqdn", fqdn)
		}
	}
}

// writeACMECertLocally writes the cert bundle to local disk paths.
func writeACMECertLocally(fqdn string, bundle *acmeCertBundle, logger *slog.Logger) error {
	// Primary path: /var/lib/globular/pki/acme/<fqdn>/
	pkiDir := filepath.Join("/var/lib/globular/pki/acme", fqdn)
	if err := os.MkdirAll(pkiDir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", pkiDir, err)
	}

	// Backup path: /var/lib/globular/domains/<fqdn>/
	domainsDir := filepath.Join("/var/lib/globular/domains", fqdn)
	if err := os.MkdirAll(domainsDir, 0755); err != nil {
		logger.Warn("acme-sync: backup dir creation failed", "path", domainsDir, "error", err)
		// Continue — primary path is more important
	}

	// Write cert files to both paths
	files := []struct {
		name string
		data string
		perm os.FileMode
	}{
		{"fullchain.pem", bundle.Fullchain, 0644},
		{"privkey.pem", bundle.Privkey, 0600},
		{"chain.pem", bundle.Chain, 0644},
	}

	for _, f := range files {
		if f.data == "" {
			continue
		}
		// Primary
		if err := writeFileAtomic(filepath.Join(pkiDir, f.name), []byte(f.data), f.perm); err != nil {
			return fmt.Errorf("write %s/%s: %w", pkiDir, f.name, err)
		}
		// Backup
		_ = writeFileAtomic(filepath.Join(domainsDir, f.name), []byte(f.data), f.perm)
	}

	// Set ownership to globular user if possible
	if info, err := os.Stat("/var/lib/globular"); err == nil {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			uid, gid := int(stat.Uid), int(stat.Gid)
			for _, dir := range []string{pkiDir, domainsDir} {
				filepath.Walk(dir, func(path string, _ os.FileInfo, _ error) error {
					_ = os.Chown(path, uid, gid)
					return nil
				})
			}
		}
	}

	// Create xDS symlink: /var/lib/globular/config/tls/acme/<fqdn> → pki/acme/<fqdn>
	xdsDir := "/var/lib/globular/config/tls/acme"
	if err := os.MkdirAll(xdsDir, 0755); err == nil {
		link := filepath.Join(xdsDir, fqdn)
		existing, err := os.Readlink(link)
		if err != nil || existing != pkiDir {
			_ = os.Remove(link)
			_ = os.Symlink(pkiDir, link)
		}
	}

	return nil
}

// isCertificateValid checks if a certificate file exists and is valid.
// For wildcard certificates, validates against the wildcard pattern (*.zone).
func (r *Reconciler) isCertificateValid(certFile string, domain string, useWildcard bool, zone string) bool {
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
	// For wildcard certificates, validate against the wildcard pattern
	checkDomain := domain
	if useWildcard {
		checkDomain = "*." + zone
	}

	if err := cert.VerifyHostname(checkDomain); err != nil {
		r.logger.Warn("certificate domain mismatch",
			"expected", checkDomain,
			"actual_cn", cert.Subject.CommonName,
			"useWildcard", useWildcard)
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

	// If the provider self-repaired its configuration during construction
	// (e.g. missing or out-of-range port → port resolved from etcd), persist
	// the corrected Config back to etcd so the same fix does not have to
	// happen on every reload. The CAS write changes the ModRevision, which
	// invalidates our cache on the next pass and triggers a clean reload.
	if rep, ok := provider.(dnsprovider.SelfRepairer); ok {
		if newCfg, repaired := rep.RepairedConfig(); repaired {
			data, marshalErr := json.Marshal(newCfg)
			if marshalErr != nil {
				r.logger.Warn("failed to marshal repaired provider config",
					"ref", providerRef,
					"error", marshalErr)
			} else if _, putErr := r.etcdClient.Put(ctx, key, string(data)); putErr != nil {
				r.logger.Warn("provider auto-repaired its config but persist failed",
					"ref", providerRef,
					"error", putErr)
			} else {
				r.logger.Info("provider config repaired and persisted to etcd",
					"ref", providerRef,
					"zone", zone)
			}
		}
	}

	// Preflight the provider end-to-end (Set/Get/Remove TXT) before handing
	// it out. A broken provider that silently fails SetTXT is the single
	// largest source of ACME failures + Let's Encrypt rate-limit burn, so
	// fail fast and noisily here instead.
	if pf, ok := provider.(dnsprovider.Preflighter); ok {
		if pfErr := pf.Preflight(ctx, zone); pfErr != nil {
			return nil, fmt.Errorf("provider %q preflight failed: %w", providerRef, pfErr)
		}
		r.logger.Info("provider preflight passed",
			"ref", providerRef,
			"zone", zone)
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

// readCertExpiry reads the fullchain.pem for a domain and returns its NotAfter time.
func (r *Reconciler) readCertExpiry(fqdn string) *time.Time {
	certFile := filepath.Join(r.certsDir, fqdn, "fullchain.pem")
	data, err := os.ReadFile(certFile)
	if err != nil {
		return nil
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil
	}
	return &cert.NotAfter
}

// isTransientGRPCError returns true for gRPC errors that are likely transient
// and worth retrying (e.g. connection closing during service restart).
func isTransientGRPCError(err error) bool {
	if err == nil {
		return false
	}
	s, ok := status.FromError(err)
	if !ok {
		// Not a gRPC status — check for wrapped gRPC errors
		return strings.Contains(err.Error(), "connection is closing") ||
			strings.Contains(err.Error(), "connection refused") ||
			strings.Contains(err.Error(), "transport is closing")
	}
	switch s.Code() {
	case codes.Unavailable, codes.Canceled, codes.Aborted, codes.DeadlineExceeded:
		return true
	}
	return false
}

// resolvePublicIP returns the current public IP, or empty string on failure.
func (r *Reconciler) resolvePublicIP() string {
	ip, err := r.discoverPublicIP()
	if err != nil {
		return ""
	}
	return ip
}

// resolveLocalNS returns the local DNS service's UDP port-53 address
// (e.g. "10.0.0.63:53") by extracting the host from the gRPC endpoint.
// Returns empty string if the DNS service address is unavailable.
func (r *Reconciler) resolveLocalNS() string {
	grpcAddr := config.ResolveDNSGrpcEndpoint("")
	if grpcAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(grpcAddr)
	if err != nil || host == "" {
		return ""
	}
	return host + ":53"
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
