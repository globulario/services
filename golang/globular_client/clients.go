// ==============================================
// clients.go — client bootstrap, TLS, dialing,
// desired-config watch, and optional runtime overlay
// (quieter output + deduped logs)
// ==============================================

package globular_client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	tokensPath   = config.GetConfigDir() + "/tokens"
	clients      *sync.Map
	watchStarted sync.Map // key: client-id string -> bool

	// bootStartedAt is used to soften logs during the first seconds of process life.
	bootStartedAt = time.Now()
)

// logDeduper coalesces repetitive messages for a small time window.
type logDeduper struct{ seen sync.Map }

func (d *logDeduper) ShouldLog(key string, window time.Duration) bool {
	now := time.Now()
	if v, ok := d.seen.Load(key); ok {
		if last, _ := v.(time.Time); now.Sub(last) < window {
			return false
		}
	}
	d.seen.Store(key, now)
	return true
}

var dedup logDeduper

/* ===================== Public API ===================== */

// Client is the minimal contract the generated service clients must satisfy.
type Client interface {
	// Identity/meta
	GetAddress() string
	GetDomain() string
	GetId() string
	GetMac() string
	GetName() string
	GetState() string

	// Mutators
	SetAddress(string)
	SetDomain(string)
	SetId(string)
	SetMac(string)
	SetName(string)
	SetState(string)

	// Port/TLS
	SetPort(int)
	GetPort() int
	HasTLS() bool
	GetCertFile() string
	GetKeyFile() string
	GetCaFile() string
	SetTLS(bool)
	SetCertFile(string)
	SetKeyFile(string)
	SetCaFile(string)

	// Lifecycle / RPC
	Close()
	Reconnect() error
	Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error)
}

// normalizeControlAddress converts many address forms to a concrete "host:port".
func normalizeControlAddress(address, localAddr string, cfg map[string]interface{}) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return address
	}

	// Strip an optional scheme if present.
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		address = strings.TrimPrefix(strings.TrimPrefix(address, "https://"), "http://")
	}

	// If already host:port, keep it.
	if _, _, err := net.SplitHostPort(address); err == nil {
		return address
	}

	// Local helpers
	asStr := func(k string) string { return Utility.ToString(cfg[k]) }
	proto := strings.ToLower(asStr("Protocol"))
	if proto == "" {
		proto = "http"
	}
	httpPort := asStr("PortHTTP")
	if httpPort == "" { httpPort = "80" }
	httpsPort := asStr("PortHTTPS")
	if httpsPort == "" { httpsPort = "443" }
	defPort := httpPort
	if proto == "https" {
		defPort = httpsPort
	}
	domain := asStr("Domain")
	dnsHost := asStr("DNS")
	if dnsHost == "" {
		// Fallback: Name.Domain if DNS not set
		name := asStr("Name")
		if name != "" && domain != "" {
			dnsHost = name + "." + domain
		}
	}

	// If it's an IP matching our local IP or "localhost", use localAddr as-is.
	if address == "localhost" {
		return localAddr
	}
	if ip := net.ParseIP(address); ip != nil {
		if lip, _ := Utility.GetIpv4(localAddr); ip.String() == lip {
			return localAddr
		}
	}

	// If the given address equals the cluster domain, route to default DNS member.
	if domain != "" && strings.EqualFold(address, domain) && dnsHost != "" {
		return net.JoinHostPort(dnsHost, defPort)
	}

	// If it matches a peer by Domain, build "hostname.domain:port" using peer's ports (when present).
	if cfg != nil {
		if peers, ok := cfg["Peers"].([]interface{}); ok && len(peers) > 0 {
			for _, pi := range peers {
				p, ok := pi.(map[string]interface{})
				if !ok {
					continue
				}
				pDomain := Utility.ToString(p["Domain"])
				if !strings.EqualFold(address, pDomain) {
					continue
				}
				host := Utility.ToString(p["Hostname"])
				if host == "" {
					host = "localhost"
				}
				if pDomain != "" && pDomain != "localhost" && !strings.Contains(host, ".") {
					host = host + "." + pDomain
				}
				// pick the right control port for the peer
				port := Utility.ToString(p["Port"])
				if proto == "https" {
					if v := Utility.ToString(p["PortHTTPS"]); v != "" && v != "0" {
						port = v
					}
				} else {
					if v := Utility.ToString(p["PortHTTP"]); v != "" && v != "0" {
						port = v
					}
				}
				if port == "" || port == "0" {
					port = defPort
				}
				return net.JoinHostPort(host, port)
			}
		}
	}

	// If it's a bare hostname (no dots), attach the local domain.
	host := address
	if !strings.Contains(host, ".") && domain != "" {
		host = host + "." + domain
	}
	// Default: add the control port from our protocol.
	return net.JoinHostPort(host, defPort)
}

// GetClient returns a cached Client for (name, normalized address) or creates one.
// It accepts an address in any of these forms:
//   - host:port                (e.g., "globule-ryzen.globular.io:443")
//   - FQDN or hostname         (e.g., "globule-ryzen.globular.io", "globule-ryzen")
//   - bare cluster domain      (e.g., "globular.io" -> maps to cfg.DNS + default control port)
//   - IP or "localhost"        (mapped appropriately)
//   - peer domain alias        (matches a peer in cfg.Peers, builds "hostname.domain:port")
func GetClient(address, name, fct string) (Client, error) {
	// Load local control address and config (best-effort; nil-safe below).
	localAddr, _ := config.GetAddress()
	localCfg, _ := config.GetLocalConfig(true)

	// Always normalize to "host:port" before doing anything else.
	address = normalizeControlAddress(address, localAddr, localCfg)

	// Initialize cache if needed and use the normalized address for the key.
	if clients == nil {
		clients = new(sync.Map)
	}
	idKey := Utility.GenerateUUID(name + ":" + address)
	if existing, ok := clients.Load(idKey); ok {
		return existing.(Client), nil
	}

	// Build the client via the provided constructor (fct must be "<pkg>.<Ctor>").
	results, err := Utility.CallFunction(fct, address, name)
	if err != nil {
		slog.Error("GetClient: constructor invocation failed",
			"function", fct, "address", address, "service", name, "err", err)
		return nil, err
	}
	if !results[1].IsNil() {
		err := results[1].Interface().(error)
		slog.Error("GetClient: constructor returned error",
			"function", fct, "address", address, "service", name, "err", err)
		return nil, err
	}

	client := results[0].Interface().(Client)
	clients.Store(idKey, client)
	slog.Debug("GetClient: client created", "service", name, "address", address)
	return client, nil
}

/* ===================== Initialization ===================== */

func portOpen(addr string, timeout time.Duration) bool {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ===================== InitClient (refactored) =====================

func InitClient(client Client, address string, id string) error {
	opts := loadInitOptionsFromEnv()

	// --- Validate inputs
	if address = strings.TrimSpace(address); address == "" {
		return fmt.Errorf("InitClient: no address provided (id=%s)", id)
	}
	if id = strings.TrimSpace(id); id == "" {
		return fmt.Errorf("InitClient: no id provided (address=%s)", address)
	}

	// --- Local config (used for address normalization, defaults, peers)
	// Not fatal when unavailable: a regular (non-root) user running the CLI
	// cannot read /var/lib/globular/config.json.  In that case we fall back to
	// an empty map so that normalizeControlAddress passes the explicit address
	// through unchanged.
	localAddr, _ := config.GetAddress()
	localCfg, err := config.GetLocalConfig(true)
	if err != nil || localCfg == nil {
		slog.Debug("InitClient: local config unavailable, using empty fallback", "err", err)
		localCfg = make(map[string]interface{})
	}

	// --- Normalize control-plane address (host:port)
	address = normalizeControlAddress(address, localAddr, localCfg)
	host, ctrlPort := splitHostPort(address)
	if ctrlPort == 0 {
		return fmt.Errorf("InitClient: invalid control address %q (missing port)", address)
	}
	localHost := strings.Split(localAddr, ":")[0]

	// Determine effective FQDN for TLS/certs usage
	effectiveHost := resolveEffectiveHost(address, host, localCfg)
	
	isLocal := (effectiveHost == localHost || effectiveHost == "localhost" || strings.HasPrefix(localHost, effectiveHost))

	client.SetAddress(address)

	// --- Pull desired configuration (from etcd)
	cfg, err := config.GetServiceConfigurationById(id)
	if err != nil || cfg == nil {
		slog.Error("InitClient: failed to fetch configuration", "id", id, "address", address, "err", err)
		return fmt.Errorf("InitClient: failed to fetch configuration id=%s from %s: %w", id, address, err)
	}

	// --- Optional: overlay runtime endpoint (/globular/runtime/<service>/<id>)
	if err := overlayRuntimeEndpoint(cfg, opts.requireRuntime, opts.runtimeGrace); err != nil {
		return err
	}

	// --- Ensure mandatory fields (Id/Port) with limited retries
	if err := ensureMandatoryFields(&cfg, id, opts.mandatoryAttempts, opts.mandatoryBaseDelay, opts.verboseInit); err != nil {
		return err
	}

	// --- Populate client identity/meta
	populateClientIdentity(client, cfg, isLocal)

	// --- TLS / mTLS setup (use effectiveHost, not bare cluster domain)
	if err := setupClientTLS(client, cfg, isLocal, effectiveHost, ctrlPort); err != nil {
		return err
	}

	slog.Debug("InitClient: client initialized",
		"id", client.GetId(), "name", client.GetName(), "domain", client.GetDomain(),
		"grpc_port", client.GetPort(), "tls", client.HasTLS(), "address", client.GetAddress(),
	)

	// --- Optional readiness: gRPC Health
	if opts.waitHealth {
		if err := waitForHealthReady(client, effectiveHost, opts); err != nil {
			return err
		}
	}

	startWatchersOnce(client)

	return nil
}

// ===================== Helpers =====================

type initOptions struct {
	requireRuntime     bool
	runtimeGrace       time.Duration
	waitHealth         bool
	healthAttempts     int
	healthInterval     time.Duration
	healthTotalTimeout time.Duration
	mandatoryAttempts  int
	mandatoryBaseDelay time.Duration
	verboseInit        bool
}

func loadInitOptionsFromEnv() initOptions {
	return initOptions{
		requireRuntime:     envGetBool("GLOBULAR_REQUIRE_RUNTIME", false),
		runtimeGrace:       envGetDuration("GLOBULAR_RUNTIME_GRACE", 15*time.Second),
		waitHealth:         envGetBool("GLOBULAR_WAIT_HEALTH", false),
		healthAttempts:     envGetInt("GLOBULAR_HEALTH_ATTEMPTS", 30),
		healthInterval:     envGetDuration("GLOBULAR_HEALTH_INTERVAL", 1*time.Second),
		healthTotalTimeout: envGetDuration("GLOBULAR_HEALTH_TIMEOUT", 0),
		mandatoryAttempts:  envGetInt("GLOBULAR_INIT_MANDATORY_ATTEMPTS", 4),
		mandatoryBaseDelay: envGetDuration("GLOBULAR_INIT_MANDATORY_SLEEP", 500*time.Millisecond),
		verboseInit:        envGetBool("GLOBULAR_CLIENT_VERBOSE_INIT", false),
	}
}

// resolveEffectiveHost returns the FQDN we should use for TLS (cert dir, SNI).
// It uses the normalized control address host; if that equals the bare domain
// but a peer entry has Hostname, it returns "hostname.domain". Env override wins.
func resolveEffectiveHost(normalizedAddr, bareDomain string, localCfg map[string]interface{}) string {
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_TLS_HOST_OVERRIDE")); v != "" {
		return v
	}
	host, _ := splitHostPort(normalizedAddr)
	// If we already have an FQDN not equal to bare domain, keep it.
	if strings.Contains(host, ".") && !strings.EqualFold(host, bareDomain) {
		return host
	}
	// Try peers
	if peers, ok := localCfg["Peers"].([]interface{}); ok {
		for _, pi := range peers {
			if p, ok := pi.(map[string]interface{}); ok && strings.EqualFold(asString(p["Domain"]), bareDomain) {
				hn := asString(p["Hostname"])
				if hn != "" && bareDomain != "" {
					return hn + "." + bareDomain
				}
			}
		}
	}
	return host
}

// overlayRuntimeEndpoint optionally replaces desired Port/TLS with runtime values.
func overlayRuntimeEndpoint(cfg map[string]interface{}, require bool, grace time.Duration) error {
	svcName := asString(cfg["Name"])
	svcId := asString(cfg["Id"])
	hp, secure, rerr := resolveFromEtcdRuntimeWithWait(context.Background(), svcName, svcId, func() time.Duration {
		if require {
			return grace
		}
		return 0
	}())
	switch {
	case rerr == nil && hp != "":
		_, port := splitHostPort(hp)
		if port > 0 {
			cfg["Port"] = port
		}
		if secure {
			cfg["TLS"] = true
		}
		return nil
	case require:
		return fmt.Errorf("InitClient: runtime endpoint not found for %s/%s after %s: %w", svcName, svcId, grace, rerr)
	case rerr != nil:
		slog.Debug("InitClient: runtime not found; using desired Port/TLS", "service", svcName, "id", svcId, "err", rerr)
	}
	return nil
}

// ensureMandatoryFields guarantees Id and Port are present, with limited retries.
func ensureMandatoryFields(cfg *map[string]interface{}, id string, attempts int, baseDelay time.Duration, verbose bool) error {
	for attempt := 1; attempt <= attempts; attempt++ {
		haveId := asString((*cfg)["Id"]) != ""
		_, havePort := (*cfg)["Port"]
		if haveId && havePort {
			break
		}
		if attempt == attempts {
			switch {
			case !haveId && !havePort:
				return fmt.Errorf("InitClient: missing service Id and Port for %s", id)
			case !haveId:
				return fmt.Errorf("InitClient: missing service Id for %s", id)
			default:
				return fmt.Errorf("InitClient: missing service Port for %s", id)
			}
		}
		sleep := baseDelay * time.Duration(1<<(attempt-1)) // 0.5s, 1s, 2s, ...
		logf := slog.Debug
		if verbose {
			logf = slog.Warn
		}
		logf("InitClient: mandatory field(s) missing; retrying",
			"id", id,
			"haveId", haveId,
			"havePort", havePort,
			"attempt", attempt,
			"sleep", sleep,
		)
		if refreshed, rerr := config.GetServiceConfigurationById(id); rerr == nil && refreshed != nil {
			*cfg = refreshed
		} else if rerr != nil {
			slog.Debug("InitClient: re-fetch configuration failed", "id", id, "err", rerr)
		}
		time.Sleep(sleep)
	}
	return nil
}

func populateClientIdentity(client Client, cfg map[string]interface{}, isLocal bool) {
	client.SetId(asString(cfg["Id"]))
	client.SetPort(Utility.ToInt(cfg["Port"]))

	// Domain / Name / Mac / State
	if v := asString(cfg["Domain"]); v != "" {
		client.SetDomain(v)
	} else {
		slog.Debug("InitClient: missing service Domain; using placeholder", "id", client.GetId())
		client.SetDomain("unknown.local")
	}

	if v := asString(cfg["Name"]); v != "" {
		client.SetName(v)
	} else {
		slog.Debug("InitClient: missing service Name; using placeholder", "id", client.GetId())
		client.SetName("unknown-service")
	}

	if v := asString(cfg["Mac"]); v != "" {
		client.SetMac(v)
	} else if isLocal {
		if m, derr := config.GetMacAddress(); derr == nil && m != "" {
			client.SetMac(m)
		} else {
			slog.Debug("InitClient: missing service Mac (local); using placeholder", "id", client.GetId())
			client.SetMac("00:00:00:00:00:00")
		}
	} else {
		if dedup.ShouldLog("missing-mac:"+client.GetId(), time.Hour) {
			slog.Debug("InitClient: missing service Mac (remote); using placeholder", "id", client.GetId())
		}
		client.SetMac("00:00:00:00:00:00")
	}

	if v := asString(cfg["State"]); v != "" {
		client.SetState(v)
	} else {
		client.SetState("starting")
	}
}

// pickTLSBaseDir chooses a writable base dir.
// Order: $GLOBULAR_TLS_DIR > ~/.config/globular/tls
// INV-PKI-1: Removed obsolete config/tls fallback - client certs should be in user's home directory
func pickTLSBaseDir() string {
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_TLS_DIR")); v != "" {
		_ = os.MkdirAll(v, 0o755)
		return v
	}
	// Skip obsolete /var/lib/globular/config/tls fallback - go directly to user home
	home := os.Getenv("XDG_CONFIG_HOME")
	if home == "" {
		home = filepath.Join(os.Getenv("HOME"), ".config")
	}
	userBase := filepath.Join(home, "globular", "tls")
	_ = os.MkdirAll(userBase, 0o755)
	return userBase
}

// tryUseExistingClientCerts attempts to find {key, cert, ca} under baseDir/host.
func tryUseExistingClientCerts(baseDir, host string) (keyFile, certFile, caFile string, ok bool) {
	dir := filepath.Join(baseDir, host)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", "", "", false
	}
	var k, c, ca string
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		p := filepath.Join(dir, e.Name())
		switch {
		case (strings.Contains(name, "key") && (strings.HasSuffix(name, ".key") || strings.HasSuffix(name, ".pem"))) && k == "":
			k = p
		case (strings.Contains(name, "cert") || strings.Contains(name, "crt") || strings.Contains(name, "certificate")) && c == "":
			c = p
		case strings.Contains(name, "ca") && (strings.HasSuffix(name, ".pem") || strings.HasSuffix(name, ".crt")) && ca == "":
			ca = p
		}
	}
	if k != "" && c != "" && ca != "" {
		return k, c, ca, true
	}
	return "", "", "", false
}

// setupClientTLS configures TLS/mTLS on the client based on desired/runtime cfg.
// For remote peers, it reuses existing client certs under base/<effectiveHost>,
// and only calls InstallClientCertificates if necessary (can be disabled).
func setupClientTLS(client Client, cfg map[string]interface{}, isLocal bool, effectiveHost string, ctrlPort int) error {

	tlsEnabled, _ := cfg["TLS"].(bool)
	client.SetTLS(tlsEnabled)
	if !tlsEnabled {
		return nil
	}

	base := pickTLSBaseDir()

	// 1) Always try user's client certs first at base/effectiveHost (e.g. ~/.config/globular/tls/localhost/)
	if k, c, ca, ok := tryUseExistingClientCerts(base, effectiveHost); ok {
		client.SetKeyFile(k)
		client.SetCertFile(c)
		client.SetCaFile(ca)
		return nil
	}

	// 2) Fallback: if isLocal and no user certs, try remapping server paths to client paths
	if isLocal {
		// Re-map server paths to client paths when the same machine publishes them.
		certFile := strings.ReplaceAll(asString(cfg["CertFile"]), "server", "client")
		keyFile := strings.ReplaceAll(asString(cfg["KeyFile"]), "server", "client")
		caFile := asString(cfg["CertAuthorityTrust"])

		// Only use remapped paths if they actually exist
		if _, err := os.Stat(keyFile); err == nil {
			if _, err := os.Stat(certFile); err == nil {
				if _, err := os.Stat(caFile); err == nil {
					client.SetKeyFile(keyFile)
					client.SetCertFile(certFile)
					client.SetCaFile(caFile)
					return nil
				}
			}
		}
	}

	// 3) Optionally skip install if tests provide certs
	if strings.EqualFold(strings.TrimSpace(os.Getenv("GLOBULAR_TLS_INSTALL")), "0") {
		return fmt.Errorf("TLS install disabled and no existing client certs at %s", filepath.Join(base, effectiveHost))
	}

	// 4) Install into base/effectiveHost (not into a bare cluster domain)
	// Certificate operations are Gateway-aware and do NOT use gRPC service ports
	path := filepath.Join(base, effectiveHost)

	keyFile, certFile, caFile, err := security.InstallClientCertificates(
		effectiveHost, path,
		asString(cfg["Country"]), asString(cfg["State"]), asString(cfg["City"]), asString(cfg["Organization"]),
		nil,
	)
	if err != nil {
		slog.Error("InitClient: InstallClientCertificates failed (gateway unavailable?)", "domain", effectiveHost, "base", base, "err", err)
		return err
	}
	client.SetKeyFile(keyFile)
	client.SetCertFile(certFile)
	client.SetCaFile(caFile)
	return nil
}

func waitForHealthReady(client Client, controlHost string, opts initOptions) error {
	targetHost := client.GetDomain()
	if targetHost == "" || targetHost == "unknown.local" {
		targetHost = controlHost
	}
	target := net.JoinHostPort(targetHost, strconv.Itoa(client.GetPort()))

	// Overall timeout (optional)
	ctx := context.Background()
	var cancel context.CancelFunc
	if opts.healthTotalTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.healthTotalTimeout)
		defer cancel()
	}

	// TLS is MANDATORY - insecure connections are no longer allowed
	var dialOpts []grpc.DialOption
	creds, cerr := makeTLSCreds(client.GetCertFile(), client.GetKeyFile(), client.GetCaFile(), targetHost)
	if cerr != nil {
		return fmt.Errorf("InitClient: building TLS creds failed (TLS is mandatory): %w", cerr)
	}
	dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))

	var lastErr error
	for i := 1; i <= opts.healthAttempts; i++ {
		if !portOpen(target, 500*time.Millisecond) {
			lastErr = fmt.Errorf("tcp port not open yet")
		} else {
			conn, derr := grpc.DialContext(ctx, target, dialOpts...)
			if derr != nil {
				lastErr = derr
			} else {
				hc := healthpb.NewHealthClient(conn)
				svc := client.GetName()
				if svc == "" || svc == "unknown-service" {
					svc = ""
				}
				callCtx, cancelOne := context.WithTimeout(ctx, 3*time.Second)
				resp, herr := hc.Check(callCtx, &healthpb.HealthCheckRequest{Service: svc})
				cancelOne()
				_ = conn.Close()

				if herr == nil && resp.GetStatus() == healthpb.HealthCheckResponse_SERVING {
					infoQuiet("InitClient: health SERVING", "target", target, "service", svc)
					return nil
				}
				if herr != nil {
					st, _ := status.FromError(herr)
					code := st.Code()
					// Accept servers that don't implement health; port-open is our readiness.
					if code == codes.Unimplemented || code == codes.NotFound {
						infoQuiet("InitClient: no health endpoint; proceeding (port is open)", "target", target)
						return nil
					}
					lastErr = herr
				} else {
					lastErr = errors.New("health not SERVING yet")
				}
			}
		}

		if i == opts.healthAttempts {
			return fmt.Errorf("InitClient: readiness check failed after %d attempts (%s): %w", opts.healthAttempts, target, lastErr)
		}
		time.Sleep(opts.healthInterval)
	}
	return nil
}

/* ===================== Runtime lookup ===================== */

func makeTLSCreds(certFile, keyFile, caFile, serverName string) (credentials.TransportCredentials, error) {
	// Load client cert if present (mTLS), otherwise allow empty pair.
	var certs []tls.Certificate
	if certFile != "" && keyFile != "" {
		c, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}

	var rootCAs *x509.CertPool
	if caFile != "" {
		rootCAs = x509.NewCertPool()
		b, err := os.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		if ok := rootCAs.AppendCertsFromPEM(b); !ok {
			return nil, fmt.Errorf("append CA from %s failed", caFile)
		}
	}

	tcfg := &tls.Config{
		ServerName:   serverName, // important for SNI/hostname verification
		Certificates: certs,
		RootCAs:      rootCAs,
		MinVersion:   tls.VersionTLS12,
	}
	return credentials.NewTLS(tcfg), nil
}

// resolveFromEtcdRuntimeWithWait tries to fetch a runtime endpoint under
// /globular/runtime/<service>/<id>. When `grace` > 0, it retries until deadline.
func resolveFromEtcdRuntimeWithWait(ctx context.Context, service, id string, grace time.Duration) (string, bool, error) {
	if service == "" {
		return "", false, fmt.Errorf("empty service name")
	}
	var deadline time.Time
	if grace > 0 {
		deadline = time.Now().Add(grace)
	}
	backoff := 200 * time.Millisecond
	for {
		hp, sec, e := resolveFromEtcdRuntime(ctx, service, id)
		if e == nil && hp != "" {
			return hp, sec, nil
		}
		if deadline.IsZero() || time.Now().After(deadline) {
			if e == nil {
				e = fmt.Errorf("runtime not found for %s/%s", service, id)
			}
			return "", false, e
		}
		if backoff < 2*time.Second {
			backoff *= 2
		}
		select {
		case <-ctx.Done():
			return "", false, ctx.Err()
		case <-time.After(backoff):
		}
	}
}

func runtimePrefix() string {
	pfx := strings.TrimRight(os.Getenv("GLOBULAR_RUNTIME_PREFIX"), "/")
	if pfx == "" {
		pfx = "/globular/runtime"
	}
	return pfx
}

// resolveFromEtcdRuntime reads /<prefix>/<service>/<id>. If missing, it falls
// back to the first instance under /<prefix>/<service>/ (WithPrefix).
func resolveFromEtcdRuntime(ctx context.Context, service, id string) (hostport string, secure bool, err error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return "", false, err
	}

	pfx := runtimePrefix()

	if id != "" {
		key := fmt.Sprintf("%s/%s/%s", pfx, service, id)
		ctx2, cancel := context.WithTimeout(ctx, 2*time.Second)
		resp, e := cli.Get(ctx2, key)
		cancel()
		if e == nil && resp.Count > 0 {
			if hp, sec, ok := decodeRuntimeValue(resp.Kvs[0].Value); ok {
				return hp, sec, nil
			}
		}
	}

	// Fallback: first instance under service/
	ctx2, cancel := context.WithTimeout(ctx, 3*time.Second)
	resp, e := cli.Get(ctx2, fmt.Sprintf("%s/%s/", pfx, service), clientv3.WithPrefix())
	cancel()
	if e != nil || resp.Count == 0 {
		if e == nil {
			e = fmt.Errorf("no runtime keys for %s", service)
		}
		return "", false, e
	}
	for _, kv := range resp.Kvs {
		if hp, sec, ok := decodeRuntimeValue(kv.Value); ok {
			return hp, sec, nil
		}
	}
	return "", false, fmt.Errorf("no valid runtime value for %s", service)
}

func decodeRuntimeValue(v []byte) (hostport string, secure bool, ok bool) {
	s := strings.TrimSpace(string(v))
	if strings.Count(s, ":") == 1 && !strings.ContainsAny(s, " \t\r\n{}[]") {
		return s, false, true
	}
	var m map[string]interface{}
	if err := json.Unmarshal(v, &m); err == nil && len(m) > 0 {
		host := asString(m["address"])
		if host == "" {
			host = asString(m["addr"])
		}
		if host == "" {
			host = asString(m["host"])
		}
		port := asInt(m["port"])
		sec := asBool(m["secure"]) || asBool(m["tls"]) || asBool(m["https"])
		if host != "" && port > 0 {
			return fmt.Sprintf("%s:%d", host, port), sec, true
		}
		if hp := asString(m["endpoint"]); hp != "" && strings.Count(hp, ":") == 1 {
			return hp, sec, true
		}
	}
	return "", false, false
}

/* ===================== TLS + Dial + Context ===================== */

// GetClientTlsConfig builds a tls.Config for a client connection (mTLS).
func GetClientTlsConfig(client Client) (*tls.Config, error) {
    // Load Root CA (required)
    root := x509.NewCertPool()
    ca := client.GetCaFile()
    b, err := os.ReadFile(ca)
    if err != nil {
        return nil, fmt.Errorf("GetClientTlsConfig: read CA %s failed: %w", ca, err)
    }
    if ok := root.AppendCertsFromPEM(b); !ok {
        return nil, fmt.Errorf("GetClientTlsConfig: append CA from %s failed", ca)
    }

    // Build base config (server-auth)
    sni := strings.Split(client.GetAddress(), ":")[0]
    if v := strings.TrimSpace(os.Getenv("GLOBULAR_TLS_SERVERNAME")); v != "" {
        sni = v
    }
    cfg := &tls.Config{
        ServerName: sni,
        RootCAs:    root,
        MinVersion: tls.VersionTLS12,
    }

    // Try to load client cert (mTLS). If not present/allowed, proceed without it.
    certFile, keyFile := client.GetCertFile(), client.GetKeyFile()
    if certFile != "" && keyFile != "" {
        if cert, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil {
            cfg.Certificates = []tls.Certificate{cert}
        } else if os.IsNotExist(err) || errors.Is(err, os.ErrPermission) {
            slog.Warn("GetClientTlsConfig: client keypair unavailable; using server-auth only",
                "cert", certFile, "key", keyFile, "err", err)
            // continue without client cert
        } else {
            return nil, fmt.Errorf("GetClientTlsConfig: load client keypair: %w", err)
        }
    }

    return cfg, nil
}

// GetClientConnection dials a gRPC connection to the client's current endpoint,
// wiring in the unary interceptor that performs quiet/backoff reconnects.
func GetClientConnection(client Client) (*grpc.ClientConn, error) {
	address := client.GetAddress()
	if strings.Contains(address, ":") {
		address = strings.Split(address, ":")[0]
	}
	target := address + ":" + Utility.ToString(client.GetPort())
	var cc *grpc.ClientConn
	var err error

	if client.HasTLS() {
		tcfg, err := GetClientTlsConfig(client)
		if err != nil {
			slog.Error("GetClientConnection: TLS config error", "target", target, "err", err)
			return nil, err
		}
		cc, err = grpc.Dial(target,
			grpc.WithTransportCredentials(credentials.NewTLS(tcfg)),
			grpc.WithUnaryInterceptor(clientInterceptor(client)),
		)
	} else {
		cc, err = grpc.Dial(target,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(clientInterceptor(client)),
		)
	}
	if err != nil {
		slog.Error("GetClientConnection: dial failed", "target", target, "tls", client.HasTLS(), "err", err)
		return nil, err
	}
	slog.Debug("GetClientConnection: connected", "target", target, "tls", client.HasTLS())
	return cc, nil
}

// GetClientContext returns a metadata-enriched context (token/domain/mac).
func GetClientContext(client Client) context.Context {
	_ = Utility.CreateDirIfNotExist(tokensPath)
	token, err := security.GetLocalToken(client.GetMac())
	address := client.GetAddress()
	if strings.Contains(address, ":") {
		address = strings.Split(address, ":")[0]
	}
	var md metadata.MD
	if err == nil {
		md = metadata.New(map[string]string{
			"token":  string(token),
			"domain": address,
			"mac":    client.GetMac(),
		})
	} else {
		md = metadata.New(map[string]string{
			"token":  "",
			"domain": address,
			"mac":    client.GetMac(),
		})
	}
	return metadata.NewOutgoingContext(context.Background(), md)
}

/* ===================== Reflection helper ===================== */

// InvokeClientRequest calls a generated client method by method-name (final path
// segment of the full gRPC method), passing (ctx, rqst) and returning (reply).
func InvokeClientRequest(client interface{}, ctx context.Context, method string, rqst interface{}) (interface{}, error) {
	methodName := method[strings.LastIndex(method, "/")+1:]
	reply, callErr := Utility.CallMethod(client, methodName, []interface{}{ctx, rqst})
	if callErr != nil {
		if reflect.TypeOf(callErr).Kind() == reflect.String {
			return nil, errors.New(callErr.(string))
		}
		return nil, callErr.(error)
	}
	return reply, nil
}

/* ===================== etcd desired watch ===================== */

func startWatchersOnce(c Client) {
	key := c.GetId()
	if key == "" {
		return
	}
	if _, ok := watchStarted.Load(key); ok {
		return
	}
	watchStarted.Store(key, true)
	go watchDesiredForClient(c)
	// (optional) add a runtime watcher later if you publish under /globular/runtime.
}

func watchDesiredForClient(c Client) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		slog.Warn("client watch: etcd client init failed", "service", c.GetName(), "err", err)
		return
	}
	path := "/globular/services/" + c.GetId() + "/config"
	wch := cli.Watch(context.Background(), path)
	slog.Debug("client watch: watching desired", "service", c.GetName(), "id", c.GetId(), "key", path)

	for w := range wch {
		for _, ev := range w.Events {
			if ev.IsCreate() || ev.IsModify() {
				cfg, err := config.GetServiceConfigurationById(c.GetId())
				if err != nil || cfg == nil {
					slog.Warn("client watch: fetch updated configuration failed", "service", c.GetName(), "err", err)
					continue
				}

				// If service is closing, stop watching to avoid reconnect churn.
				if st, ok := cfg["State"].(string); ok && st != "" {
					state := strings.ToLower(strings.TrimSpace(st))
					if state == "closing" || state == "closed" {
						if dedup.ShouldLog("svc-closing:"+c.GetId(), envGetDuration("GLOBULAR_CLIENT_DEDUP_WINDOW", 2*time.Second)) {
							infoQuiet("client watch: service is closing/closed", "service", c.GetName(), "id", c.GetId())
						}
						return
					}
				}

				oldPort := c.GetPort()
				oldTLS := c.HasTLS()

				// minimal fields we care about
				if v, ok := cfg["Port"]; ok {
					c.SetPort(Utility.ToInt(v))
				}
				if v, ok := cfg["TLS"].(bool); ok {
					c.SetTLS(v)
				}
				if st, ok := cfg["State"].(string); ok && st != "" {
					c.SetState(st)
				}

				// React to endpoint changes (dedup & quiet by default).
				if c.GetPort() != oldPort || c.HasTLS() != oldTLS {
					if dedup.ShouldLog("endpoint-change:"+c.GetId(), envGetDuration("GLOBULAR_CLIENT_DEDUP_WINDOW", 2*time.Second)) {
						infoQuiet("client watch: endpoint changed; reconnecting",
							"service", c.GetName(), "id", c.GetId(),
							"old_port", oldPort, "new_port", c.GetPort(),
							"old_tls", oldTLS, "new_tls", c.HasTLS())
					}

					tlsChanged := (c.HasTLS() != oldTLS)
					portChanged := (c.GetPort() != oldPort)

					if tlsChanged || portChanged {
						// (Re)populate TLS files if needed.
						if c.HasTLS() && (c.GetCertFile() == "" || c.GetCaFile() == "") {
							if err := populateClientTLS(c); err != nil {
								slog.Warn("client watch: TLS populate failed; keeping TLS disabled until ready",
									"service", c.GetName(), "err", err)
								c.SetTLS(false) // prevent broken TLS dials
							}
						}

						for i := 0; i < 5; i++ {
							if err := c.Reconnect(); err == nil {
								break
							}
							time.Sleep(500 * time.Millisecond)
						}
					}
					// attempt a reconnect loop (non-fatal if it fails; interceptor will also retry)
					for i := 0; i < 5; i++ {
						if err := c.Reconnect(); err == nil {
							break
						}
						time.Sleep(500 * time.Millisecond)
					}
				}

				if strings.EqualFold(c.GetState(), "closing") || strings.EqualFold(c.GetState(), "closed") {
					slog.Warn("client watch: service is closing/closed", "service", c.GetName(), "id", c.GetId())
				}
			}
		}
	}
}

// controlPortFromAddress extracts the control plane port from a "host:port" string.
func controlPortFromAddress(addr string) int {
	_, p := splitHostPort(addr)
	return p
}

// populateClientTLS (re)ensures TLS files exist when the endpoint changes.
// For remote peers we use a user-writable TLS base and the effective host.
func populateClientTLS(c Client) error {
	// Extract host (without port) from control address
	host := strings.Split(strings.TrimSpace(c.GetAddress()), ":")[0]

	localAddr, _ := config.GetAddress()
	localHost := strings.Split(localAddr, ":")[0]
	isLocal := (host == localHost || host == "localhost" || strings.HasPrefix(localHost, host))

	if isLocal {
		// derive client paths from desired server paths
		if cfg, err := config.GetServiceConfigurationById(c.GetId()); err == nil && cfg != nil {
			certFile := strings.ReplaceAll(asString(cfg["CertFile"]), "server", "client")
			keyFile := strings.ReplaceAll(asString(cfg["KeyFile"]), "server", "client")
			caFile := asString(cfg["CertAuthorityTrust"])
			if certFile != "" {
				c.SetCertFile(certFile)
			}
			if keyFile != "" {
				c.SetKeyFile(keyFile)
			}
			if caFile != "" {
				c.SetCaFile(caFile)
			}
			if c.GetCertFile() == "" && c.GetCaFile() == "" {
				return fmt.Errorf("no local TLS paths in desired config")
			}
			return nil
		}
		return fmt.Errorf("cannot read desired config for local TLS (id=%s)", c.GetId())
	}

	// Remote
	base := pickTLSBaseDir()

	// Reuse existing if present
	if k, cfile, caf, ok := tryUseExistingClientCerts(base, host); ok {
		c.SetKeyFile(k)
		c.SetCertFile(cfile)
		c.SetCaFile(caf)
		return nil
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("GLOBULAR_TLS_INSTALL")), "0") {
		return fmt.Errorf("TLS install disabled and no existing client certs at %s", filepath.Join(base, host))
	}

	// Need to install; derive control port from address
	ctrlPort := controlPortFromAddress(c.GetAddress())
	if ctrlPort == 0 {
		return fmt.Errorf("invalid control address %q: no port", c.GetAddress())
	}
	// Certificate operations are Gateway-aware and do NOT use gRPC service ports
	path := filepath.Join(base, host)
	keyFile, certFile, caFile, err := security.InstallClientCertificates(
		host, path,
		"", "", "", "",
		nil,
	)
	if err != nil {
		return err
	}
	c.SetKeyFile(keyFile)
	c.SetCertFile(certFile)
	c.SetCaFile(caFile)
	return nil
}

/* ===================== Small helpers ===================== */

func envGetDuration(name string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
func envGetBool(name string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	v = strings.ToLower(v)
	return v == "1" || v == "true" || v == "yes"
}
func envGetInt(name string, def int) int {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
func asString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func asInt(v interface{}) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return int(n)
		}
	}
	if s, ok := v.(string); ok && s != "" {
		n, _ := strconv.Atoi(s)
		return n
	}
	return 0
}
func asBool(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		u := strings.ToLower(strings.TrimSpace(t))
		return u == "true" || u == "1" || u == "yes"
	}
	return false
}
func splitHostPort(hp string) (string, int) {
	i := strings.LastIndex(hp, ":")
	if i <= 0 || i >= len(hp)-1 {
		return hp, 0
	}
	p, _ := strconv.Atoi(hp[i+1:])
	return hp[:i], p
}

// quietDuringBoot returns true during an initial grace period so we can demote
// transient connect/init logs from INFO to DEBUG.
func quietDuringBoot() bool {
	return time.Since(bootStartedAt) < envGetDuration("GLOBULAR_CLIENT_BOOT_GRACE", 8*time.Second)
}

// isQuietLog returns true if we should demote chatty INFO logs to DEBUG.
func isQuietLog() bool {
	return envGetBool("GLOBULAR_CLIENT_QUIET", true) || quietDuringBoot()
}

// infoQuiet logs Info unless quiet mode is active, in which case it logs Debug.
func infoQuiet(msg string, args ...any) {
	if isQuietLog() {
		slog.Debug(msg, args...)
	} else {
		slog.Info(msg, args...)
	}
}
