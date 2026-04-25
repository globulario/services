package main

import (
	"crypto/tls"
	"crypto/x509"
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/user"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/certs"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/ingress"
	"github.com/globulario/services/golang/pki"
	"github.com/globulario/services/golang/security"
	"google.golang.org/protobuf/types/known/structpb"
)

func (srv *NodeAgentServer) StartACMERenewal(ctx context.Context) {
	go srv.acmeRenewalLoop(ctx)
}

// StartCAKeySync starts a background loop that ensures the CA private key
// is available locally by pulling it from MinIO (globular-config/pki/ca.key).
// This enables any node to act as a certificate authority — if the original
// bootstrap node goes down, other nodes can still issue certificates.
// caKeySyncEnabled gates the CA private key sync from MinIO.
// Disabled by default: the CA private key must only reside on the signer
// authority node. Enabling this in production requires RBAC signer role,
// audit logging, and verified healthy distributed MinIO. See item 7 in the
// PKI hardening plan.
var caKeySyncEnabled = false

// EnableCAKeySync opts in to CA key sync from MinIO. Only call from explicit
// operator tooling that has verified MinIO health and RBAC requirements.
func EnableCAKeySync() { caKeySyncEnabled = true }

func (srv *NodeAgentServer) StartCAKeySync(ctx context.Context) {
	if !caKeySyncEnabled {
		log.Printf("ca-key-sync: DISABLED by default (CA private key must stay on signer authority; enable with EnableCAKeySync())")
		return
	}
	go srv.caKeySyncLoop(ctx)
}

func validateLeafSignedByCA(certPath, keyPath, caPath string) error {
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return fmt.Errorf("read CA certificate %s: %w", caPath, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return fmt.Errorf("parse CA certificate bundle %s", caPath)
	}

	leafPEM, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("read leaf certificate %s: %w", certPath, err)
	}
	block, _ := pem.Decode(leafPEM)
	if block == nil {
		return fmt.Errorf("decode leaf certificate PEM %s", certPath)
	}
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("parse leaf certificate %s: %w", certPath, err)
	}

	if _, err := tls.LoadX509KeyPair(certPath, keyPath); err != nil {
		return fmt.Errorf("leaf/key pair invalid (%s,%s): %w", certPath, keyPath, err)
	}

	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		return fmt.Errorf("leaf not signed by current CA (%s): %w", caPath, err)
	}
	return nil
}

func (srv *NodeAgentServer) ensureRuntimeTLSConvergence(ctx context.Context) {
	if srv == nil || srv.state == nil || srv.lastSpec == nil {
		return
	}
	if strings.ToLower(strings.TrimSpace(srv.lastSpec.GetProtocol())) != "https" {
		return
	}

	certPath := config.GetLocalServerCertificatePath()
	keyPath := config.GetLocalServerKeyPath()
	caPath := config.GetLocalCACertificate()
	err := validateLeafSignedByCA(certPath, keyPath, caPath)
	if err == nil {
		return
	}

	log.Printf("tls-convergence: runtime TLS chain invalid; repairing certs from current CA: %v", err)
	if repairErr := srv.ensureNetworkCerts(srv.lastSpec); repairErr != nil {
		log.Printf("tls-convergence: repair failed: %v", repairErr)
		return
	}
	log.Printf("tls-convergence: repaired runtime TLS certificate chain")
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

	ensureCAKeyPermissions(caKeyPath)

	log.Printf("ca-key-sync: pulled ca.key from MinIO to %s (%d bytes)", caKeyPath, len(data))
}

// ensureCAKeyPermissions makes ca.key readable by the globular group so services
// running as the globular user can issue client certificates without manual chmod.
func ensureCAKeyPermissions(path string) {
	const desiredMode = 0o640

	// Set mode first (best-effort).
	if err := os.Chmod(path, desiredMode); err != nil {
		log.Printf("ca-key-sync: chmod %s: %v", path, err)
	}

	// Try to set group to "globular" while keeping current owner.
	fi, err := os.Stat(path)
	if err != nil {
		return
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	uid := int(stat.Uid)
	gid := int(stat.Gid)

	if grp, err := user.LookupGroup("globular"); err == nil {
		if gidParsed, err := strconv.Atoi(grp.Gid); err == nil && gidParsed != gid {
			if err := os.Chown(path, uid, gidParsed); err != nil {
				log.Printf("ca-key-sync: chown %s to :globular failed: %v", path, err)
			}
		}
	}
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

	// Collect IP SANs: node's routable IPs + ingress VIP (if this node
	// participates in keepalived). Without the VIP in the cert, any gRPC
	// client connecting via the floating VIP will fail TLS verification.
	ips := collectNodeIPs()
	if vip := srv.lookupIngressVIP(); vip != "" {
		ips = append(ips, vip)
	}
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

	const waitTimeout = 60 * time.Second

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
			DNS:      config.ResolveDNSGrpcEndpoint(discoverServiceAddr(10006)),
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
		keyFile, leafFile, caFile, err := manager.EnsureServerCert(canonicalPKIDir, domain, dns, ips, 90*24*time.Hour)
		if err != nil {
			return fmt.Errorf("issue server certs: %w", err)
		}
		// Post-issuance validation: fail loud if the cert is missing
		// required IP SANs. Without this, a missing VIP in the cert
		// causes silent gRPC timeouts that are extremely hard to diagnose.
		if len(ips) > 0 {
			if err := manager.ValidateCertPair(leafFile, keyFile, nil, nil, ips); err != nil {
				log.Printf("CRITICAL: issued cert is missing required IP SANs: %v", err)
				return fmt.Errorf("cert validation failed after issuance: %w", err)
			}
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

// collectNodeIPs returns the non-loopback unicast IPs of this node.
func collectNodeIPs() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []string
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() || ipNet.IP.IsLinkLocalUnicast() {
			continue
		}
		out = append(out, ipNet.IP.String())
	}
	return out
}

// lookupIngressVIP reads the ingress spec from etcd and returns the VIP
// if this node is a participant. Returns "" if no VIP is configured or
// this node is not a keepalived participant.
func (srv *NodeAgentServer) lookupIngressVIP() string {
	etcdClient, err := config.GetEtcdClient()
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := etcdClient.Get(ctx, ingressSpecKey)
	if err != nil || len(resp.Kvs) == 0 {
		return ""
	}
	var spec ingress.Spec
	if err := json.Unmarshal(resp.Kvs[0].Value, &spec); err != nil {
		return ""
	}
	if spec.Mode != ingress.ModeVIPFailover || spec.VIPFailover == nil {
		return ""
	}
	// Only include the VIP if this node participates in keepalived.
	if srv.nodeID != "" {
		found := false
		for _, p := range spec.VIPFailover.Participants {
			if p == srv.nodeID {
				found = true
				break
			}
		}
		if !found {
			return ""
		}
	}
	// Strip CIDR if present (e.g. "10.0.0.100/24" → "10.0.0.100").
	vip := strings.TrimSpace(spec.VIPFailover.VIP)
	if ip, _, err := net.ParseCIDR(vip); err == nil {
		return ip.String()
	}
	if net.ParseIP(vip) != nil {
		return vip
	}
	return ""
}

func (srv *NodeAgentServer) isIssuerNode() bool {
	// Read issuer identity from cluster config; fallback to "node-0".
	issuer := "node-0"
	if gc, err := config.GetLocalConfig(true); err == nil && gc != nil {
		if v, ok := gc["CertIssuerNode"]; ok {
			if s := strings.TrimSpace(fmt.Sprintf("%v", v)); s != "" {
				issuer = s
			}
		}
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

// StartCACertDriftCheck starts a background loop that detects CA rotation
// by comparing the local CA's SPKI fingerprint against the authoritative value
// published by the cluster controller at /globular/pki/ca. When drift is
// detected the service cert is regenerated immediately.
func (srv *NodeAgentServer) StartCACertDriftCheck(ctx context.Context) {
	go srv.caCertDriftLoop(ctx)
}

func (srv *NodeAgentServer) caCertDriftLoop(ctx context.Context) {
	// Delay startup: give etcd a chance to become reachable.
	select {
	case <-ctx.Done():
		return
	case <-time.After(20 * time.Second):
	}

	srv.reconcileCACertDrift(ctx)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			srv.reconcileCACertDrift(ctx)
		}
	}
}

// reconcileCACertDrift compares the local CA's SPKI fingerprint to the one
// published by the controller in etcd. If they differ the node's service cert
// was signed by a rotated CA and must be regenerated. This is the detection
// half of the CA rotation convergence path.
func (srv *NodeAgentServer) reconcileCACertDrift(ctx context.Context) {
	caPath := config.GetLocalCACertificate()
	if caPath == "" {
		return // CA not yet present on this node
	}

	etcdMeta, err := config.LoadCAMetadata(ctx)
	if err != nil {
		log.Printf("ca-drift: load etcd CA metadata: %v (skipping)", err)
		return
	}
	if etcdMeta == nil {
		// Controller has not published CA metadata yet (pre-bootstrap).
		return
	}

	localFP, err := security.FileSPKIFingerprint(caPath)
	if err != nil {
		log.Printf("ca-drift: compute local CA fingerprint: %v", err)
		return
	}

	if localFP == etcdMeta.Fingerprint {
		return // CA unchanged — nothing to do
	}

	log.Printf("ca-drift: LOCAL CA fingerprint %s does not match controller's %s (generation %d) — regenerating service cert",
		localFP, etcdMeta.Fingerprint, etcdMeta.Generation)

	// Regenerate service certs using the current CA.
	if srv.lastSpec != nil {
		if err := srv.ensureNetworkCerts(srv.lastSpec); err != nil {
			log.Printf("ca-drift: cert regeneration failed: %v", err)
			return
		}
		log.Printf("ca-drift: service cert regenerated for CA generation %d", etcdMeta.Generation)
	} else {
		log.Printf("ca-drift: detected but no lastSpec available — cert regeneration deferred until next TLS convergence cycle")
	}
}
