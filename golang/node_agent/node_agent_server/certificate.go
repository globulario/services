package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/certs"
	"github.com/globulario/services/golang/pki"
	"google.golang.org/protobuf/types/known/structpb"
)

func (srv *NodeAgentServer) StartACMERenewal(ctx context.Context) {
	go srv.acmeRenewalLoop(ctx)
}

// StartCAKeySync starts a background loop that ensures the CA private key
// is available locally by pulling it from MinIO (globular-config/pki/ca.key).
// This enables any node to act as a certificate authority — if the original
// bootstrap node goes down, other nodes can still issue certificates.
func (srv *NodeAgentServer) StartCAKeySync(ctx context.Context) {
	go srv.caKeySyncLoop(ctx)
}

func (srv *NodeAgentServer) caKeySyncLoop(ctx context.Context) {
	// Initial delay: wait for MinIO to be available after startup.
	select {
	case <-ctx.Done():
		return
	case <-time.After(30 * time.Second):
	}

	srv.syncCAKeyFromMinIO()

	// Re-check every hour in case the key was rotated.
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.syncCAKeyFromMinIO()
		}
	}
}

func (srv *NodeAgentServer) syncCAKeyFromMinIO() {
	caKeyPath := config.GetCanonicalPKIDir() + "/ca.key"

	// If the key already exists locally, nothing to do.
	if _, err := os.Stat(caKeyPath); err == nil {
		return
	}

	data, err := config.GetClusterConfig(config.ConfigKeyCAKey)
	if err != nil {
		log.Printf("ca-key-sync: failed to fetch ca.key from MinIO: %v", err)
		return
	}
	if data == nil {
		log.Printf("ca-key-sync: ca.key not found in MinIO (globular-config/%s)", config.ConfigKeyCAKey)
		return
	}

	if err := os.MkdirAll(config.GetCanonicalPKIDir(), 0o755); err != nil {
		log.Printf("ca-key-sync: mkdir %s: %v", config.GetCanonicalPKIDir(), err)
		return
	}

	if err := os.WriteFile(caKeyPath, data, 0o400); err != nil {
		log.Printf("ca-key-sync: write %s: %v", caKeyPath, err)
		return
	}

	log.Printf("ca-key-sync: pulled ca.key from MinIO to %s (%d bytes)", caKeyPath, len(data))
}

func (srv *NodeAgentServer) acmeRenewalLoop(ctx context.Context) {
	// Check every 12 hours
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	// Run immediately on startup, then every 12h
	srv.checkAndRenewCertificate(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.checkAndRenewCertificate(ctx)
		}
	}
}

func (srv *NodeAgentServer) checkAndRenewCertificate(ctx context.Context) {
	srv.mu.Lock()
	spec := srv.lastSpec
	srv.mu.Unlock()

	if spec == nil {
		return
	}

	// Only renew if https and ACME enabled
	if !strings.EqualFold(spec.GetProtocol(), "https") || !spec.GetAcmeEnabled() {
		return
	}

	log.Printf("ACME renewal check: domain=%s", spec.GetClusterDomain())

	// Run tls.acme.ensure action
	handler := actions.Get("tls.acme.ensure")
	if handler == nil {
		log.Printf("ACME renewal: action not registered")
		return
	}

	args := map[string]interface{}{
		"domain":       spec.GetClusterDomain(),
		"admin_email":  spec.GetAdminEmail(),
		"acme_enabled": spec.GetAcmeEnabled(),
		"dns_addr":     config.ResolveDNSGrpcEndpoint(discoverServiceAddr(10006)),
	}

	argsStruct, err := structpb.NewStruct(args)
	if err != nil {
		log.Printf("ACME renewal: failed to create args: %v", err)
		return
	}

	if err := handler.Validate(argsStruct); err != nil {
		log.Printf("ACME renewal: validation failed: %v", err)
		return
	}

	result, err := handler.Apply(ctx, argsStruct)
	if err != nil {
		log.Printf("ACME renewal failed: %v", err)
		return
	}

	log.Printf("ACME renewal result: %s", result)

	// If certificate changed, restart services
	if strings.Contains(result, "issued") || strings.Contains(result, "renewed") {
		log.Printf("Certificate changed, restarting gateway/xds/envoy")
		srv.restartServicesAfterCertChange(ctx)
	}
}

func (srv *NodeAgentServer) restartServicesAfterCertChange(ctx context.Context) {
	// Restart gateway, xds, and envoy if present
	servicesToRestart := []string{"gateway", "xds", "envoy"}

	if srv.restartHook != nil {
		if err := srv.restartHook(servicesToRestart, nil); err != nil {
			log.Printf("restart services after cert change failed: %v", err)
		}
	}
}

type certWatcherDeps struct {
	kv                   certs.KV
	writeTLS             func(certs.CertBundle) error
	restartUnits         func() error
	runConvergenceChecks func(context.Context) error
	now                  func() time.Time
	debounce             time.Duration
}

func runCertWatcherOnce(ctx context.Context, domain string, stateGen uint64, lastRestart time.Time, deps certWatcherDeps) (uint64, time.Time, bool, error) {
	if deps.now == nil {
		deps.now = time.Now
	}
	if deps.debounce == 0 {
		deps.debounce = 10 * time.Second
	}
	if deps.kv == nil || strings.TrimSpace(domain) == "" {
		return stateGen, lastRestart, false, nil
	}

	gen, err := deps.kv.GetBundleGeneration(ctx, domain)
	if err != nil || gen == 0 || gen <= stateGen {
		return stateGen, lastRestart, false, err
	}
	bundle, err := deps.kv.GetBundle(ctx, domain)
	if err != nil {
		return stateGen, lastRestart, false, err
	}
	if deps.writeTLS != nil {
		if err := deps.writeTLS(bundle); err != nil {
			return stateGen, lastRestart, false, err
		}
	}

	restarted := false
	if deps.restartUnits != nil && (lastRestart.IsZero() || deps.now().Sub(lastRestart) >= deps.debounce) {
		if err := deps.restartUnits(); err != nil {
			return gen, lastRestart, false, err
		}
		lastRestart = deps.now()
		restarted = true
	}

	if restarted && deps.runConvergenceChecks != nil {
		if err := deps.runConvergenceChecks(ctx); err != nil {
			return gen, lastRestart, restarted, err
		}
	}

	return gen, lastRestart, restarted, nil
}

func (srv *NodeAgentServer) pollCertGeneration(ctx context.Context) {
	if srv == nil || srv.state == nil {
		return
	}
	if strings.ToLower(strings.TrimSpace(srv.state.Protocol)) != "https" {
		return
	}
	domain := strings.TrimSpace(srv.state.ClusterDomain)
	if domain == "" {
		return
	}
	kv := srv.getCertKV()
	if kv == nil {
		return
	}
	tlsDir, fullchainDst, keyDst, caDst := config.CanonicalTLSPaths(config.GetRuntimeConfigDir())
	deps := certWatcherDeps{
		kv: kv,
		writeTLS: func(bundle certs.CertBundle) error {
			if err := os.MkdirAll(tlsDir, 0o755); err != nil {
				return err
			}
			return writeCertBundleFiles(bundle, keyDst, fullchainDst, caDst)
		},
		restartUnits: func() error {
			units := orderRestartUnits([]string{"globular-xds.service", "globular-envoy.service", "globular-gateway.service"})
			restartFn := srv.performRestartUnits
			if srv.restartHook != nil {
				restartFn = srv.restartHook
			}
			return restartFn(units, nil)
		},
		now:      time.Now,
		debounce: 10 * time.Second,
	}
	if spec := srv.lastSpec; spec != nil {
		deps.runConvergenceChecks = func(c context.Context) error {
			checkFn := runConvergenceChecks
			if srv.healthCheckHook != nil {
				checkFn = srv.healthCheckHook
			}
			if err := checkFn(c, spec); err != nil {
				return err
			}
			log.Printf("cert watcher: convergence checks passed")
			return nil
		}
	}

	newGen, newRestart, _, err := runCertWatcherOnce(ctx, domain, srv.state.CertGeneration, srv.lastCertRestart, deps)
	if err != nil {
		log.Printf("cert watcher: %v", err)
		return
	}
	if newGen > srv.state.CertGeneration {
		srv.state.CertGeneration = newGen
		if err := srv.saveState(); err != nil {
			log.Printf("cert watcher: save state: %v", err)
		}
	}
	srv.lastCertRestart = newRestart
}

func (srv *NodeAgentServer) ensureNetworkCerts(spec *cluster_controllerpb.ClusterNetworkSpec) error {
	if spec == nil || strings.ToLower(spec.GetProtocol()) != "https" {
		return nil
	}
	domain := strings.TrimSpace(spec.GetClusterDomain())
	if domain == "" {
		return errors.New("cluster_domain is required when protocol=https")
	}
	if spec.GetAcmeEnabled() && strings.TrimSpace(spec.GetAdminEmail()) == "" {
		return errors.New("admin_email is required for ACME")
	}
	// Cluster services (MinIO, workflow, repository, etc.) are addressed via
	// DNS names like <service>.<domain>. Include the wildcard so one cert per
	// node covers every service alias, plus the node's hostname FQDN.
	host, _ := os.Hostname()
	dns := []string{domain, "*." + domain}
	if host != "" {
		dns = append(dns, host, host+"."+domain)
	}
	dns = append(dns, spec.GetAlternateDomains()...)
	tlsDir, fullchainDst, keyDst, caDst := config.CanonicalTLSPaths(config.GetRuntimeConfigDir())
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		return fmt.Errorf("create tls dir: %w", err)
	}
	kv := srv.getCertKV()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	isLeader := true
	var release func()
	if kv != nil {
		lockLeader, unlock, err := kv.AcquireCertIssuerLock(ctx, domain, srv.nodeID, 30*time.Second)
		if err != nil {
			return fmt.Errorf("acquire cert issuer lock: %w", err)
		}
		isLeader = lockLeader
		release = unlock
	}
	if release != nil {
		defer release()
	}

	waitTimeout := 60 * time.Second
	if v := strings.TrimSpace(os.Getenv("CERT_WAIT_SECONDS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			waitTimeout = time.Duration(n) * time.Second
		}
	}

	if kv == nil && !srv.isIssuerNode() {
		if err := waitForFiles([]string{keyDst, fullchainDst, caDst}, waitTimeout); err != nil {
			return fmt.Errorf("wait for tls files: %w", err)
		}
		return nil
	}

	if kv != nil && !isLeader {
		bundle, err := kv.WaitForBundle(ctx, domain, waitTimeout)
		if err != nil {
			return fmt.Errorf("wait for tls bundle: %w", err)
		}
		if err := writeCertBundleFiles(bundle, keyDst, fullchainDst, caDst); err != nil {
			return fmt.Errorf("write cert bundle: %w", err)
		}
		if srv.state != nil {
			srv.state.CertGeneration = bundle.Generation
			_ = srv.saveState()
		}
		log.Printf("nodeagent: fetched cert bundle for %s generation %d", domain, bundle.Generation)
		return nil
	}

	// H2 Hardening: Use canonical PKI directory for CA storage
	canonicalPKIDir := config.GetCanonicalPKIDir()
	legacyPaths := config.GetLegacyCAPaths()
	migrated, err := pki.MigrateCAIfNeeded(canonicalPKIDir, legacyPaths)
	if err != nil {
		log.Printf("nodeagent: CA migration error: %v", err)
	} else if migrated {
		log.Printf("nodeagent: migrated CA from legacy location to %s", canonicalPKIDir)
	}

	opts := pki.Options{
		Storage: pki.FileStorage{},
		LocalCA: pki.LocalCAConfig{
			Enabled:   true,
			Org:       "Globular Internal CA",
			ValidDays: 3650, // 10 years for internal CA certs
		},
	}
	if spec.GetAcmeEnabled() {
		opts.ACME = pki.ACMEConfig{
			Enabled:  true,
			Email:    strings.TrimSpace(spec.GetAdminEmail()),
			Domain:   domain,
			Provider: "globular",
			DNS:      strings.TrimSpace(os.Getenv("GLOBULAR_DNS_ADDR")),
		}
		if opts.ACME.DNS == "" {
			opts.ACME.DNS = config.ResolveDNSGrpcEndpoint(discoverServiceAddr(10006))
		}
	}

	// H2 Hardening: Use canonical PKI directory for CA, certificates are issued here
	manager := networkPKIManager(opts)
	var bundle certs.CertBundle
	if spec.GetAcmeEnabled() {
		subject := fmt.Sprintf("CN=%s", domain)
		keyFile, _, issuerFile, fullchainFile, err := manager.EnsurePublicACMECert(canonicalPKIDir, domain, subject, dns, 90*24*time.Hour)
		if err != nil {
			return fmt.Errorf("issue ACME certs: %w", err)
		}
		if err := copyFilePerm(keyFile, keyDst, 0o600); err != nil {
			return err
		}
		if err := copyFilePerm(fullchainFile, fullchainDst, 0o644); err != nil {
			return err
		}
		if err := copyFilePerm(issuerFile, caDst, 0o644); err != nil {
			return err
		}
	} else {
		keyFile, leafFile, caFile, err := manager.EnsureServerCert(canonicalPKIDir, domain, dns, 90*24*time.Hour)
		if err != nil {
			return fmt.Errorf("issue server certs: %w", err)
		}
		if err := copyFilePerm(keyFile, keyDst, 0o600); err != nil {
			return err
		}
		if err := concatFiles(fullchainDst, leafFile, caFile); err != nil {
			return fmt.Errorf("build fullchain: %w", err)
		}
		if caFile != "" {
			if err := copyFilePerm(caFile, caDst, 0o644); err != nil {
				return err
			}
		}
	}

	keyBytes, err := os.ReadFile(keyDst)
	if err != nil {
		return fmt.Errorf("read key for publish: %w", err)
	}
	fullchainBytes, err := os.ReadFile(fullchainDst)
	if err != nil {
		return fmt.Errorf("read fullchain for publish: %w", err)
	}
	caBytes, _ := os.ReadFile(caDst)
	bundle = certs.CertBundle{
		Key:        keyBytes,
		Fullchain:  fullchainBytes,
		CA:         caBytes,
		Generation: uint64(time.Now().UnixNano()),
		UpdatedMS:  time.Now().UnixMilli(),
	}

	if kv != nil {
		if err := kv.PutBundle(ctx, domain, bundle); err != nil {
			log.Printf("nodeagent: failed to publish cert bundle: %v", err)
		} else {
			log.Printf("nodeagent: published cert bundle for %s generation %d", domain, bundle.Generation)
			srv.state.CertGeneration = bundle.Generation
			_ = srv.saveState()
		}
	}
	return nil
}

func (srv *NodeAgentServer) isIssuerNode() bool {
	issuer := strings.TrimSpace(os.Getenv("GLOBULAR_CERT_ISSUER_NODE"))
	if issuer == "" {
		issuer = "node-0"
	}
	if srv == nil || strings.TrimSpace(srv.nodeID) == "" {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(srv.nodeID), issuer)
}

func (srv *NodeAgentServer) getCertKV() certs.KV {
	if srv.certKV != nil {
		return srv.certKV
	}
	// Plan store removed — etcd KV for cert operations needs separate wiring.
	// TODO: pass etcd client directly to node-agent for cert renewal.
	return nil
}

func writeCertBundleFiles(bundle certs.CertBundle, keyDst, fullchainDst, caDst string) error {
	if err := writeAtomicFile(keyDst, bundle.Key, 0o600); err != nil {
		return err
	}
	if err := writeAtomicFile(fullchainDst, bundle.Fullchain, 0o644); err != nil {
		return err
	}
	if len(bundle.CA) > 0 {
		if err := writeAtomicFile(caDst, bundle.CA, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func (srv *NodeAgentServer) performRestartUnits(units []string, op *operation) error {
	if len(units) == 0 {
		return nil
	}
	systemctl, err := systemctlLookPath("systemctl")
	if err != nil {
		return fmt.Errorf("systemctl lookup: %w", err)
	}
	var errs []string
	resolved := resolveUnits(units, func(u string) bool {
		return systemdUnitExists(systemctl, u) == nil
	})
	for idx, unit := range resolved {
		percent := int32(30 + idx*10)
		if percent > 95 {
			percent = 95
		}
		if op != nil {
			op.broadcast(op.newEvent(cluster_controllerpb.OperationPhase_OP_RUNNING, fmt.Sprintf("restart %s", unit), percent, false, ""))
		}
		if err := restartCommand(systemctl, unit); err != nil {
			log.Printf("nodeagent: %s reload/restart: %v", unit, err)
			var details string
			journal, jerr := exec.Command(systemctl, "status", unit, "--no-pager", "-n", "50").CombinedOutput()
			if jerr == nil {
				details = string(journal)
			}
			errs = append(errs, fmt.Sprintf("%s: %v %s", unit, err, details))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("restart failures: %s", strings.Join(errs, "; "))
	}
	return nil
}

func restartUnit(systemctl, unit string) error {
	if err := systemdUnitExists(systemctl, unit); err != nil {
		return err
	}
	if err := runSystemctl(systemctl, "reload", unit); err != nil {
		if err := runSystemctl(systemctl, "restart", unit); err != nil {
			return err
		}
	}
	return nil
}
