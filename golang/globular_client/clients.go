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
	"reflect"

	//"runtime/debug"
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

// GetClient returns a cached Client if one exists for (name,address); otherwise,
// it constructs one by calling the provided factory function `fct` on the target
// address/name. Peer "Domain" aliases are resolved from the local Globular config.
func GetClient(address, name, fct string) (Client, error) {
	localAddress, _ := config.GetAddress()
	if localAddress != address {
		// Resolve peer config to host:port if a matching peer domain is found.
		localConfig, _ := config.GetLocalConfig(true)
		if peers, ok := localConfig["Peers"].([]interface{}); ok {
			for _, pi := range peers {
				if p, ok := pi.(map[string]interface{}); ok && p["Domain"].(string) == address {
					host := p["Hostname"].(string)
					if p["Domain"].(string) != "localhost" {
						host += "." + p["Domain"].(string)
					}
					address = host + ":" + Utility.ToString(p["Port"])
					break
				}
			}
		}
	}
	if clients == nil {
		clients = new(sync.Map)
	}
	id := Utility.GenerateUUID(name + ":" + address)
	if existing, ok := clients.Load(id); ok {
		return existing.(Client), nil
	}
	results, err := Utility.CallFunction(fct, address, name)
	if err != nil {
		slog.Error("GetClient: constructor invocation failed", "function", fct, "address", address, "service", name, "err", err)
		return nil, err
	}
	if !results[1].IsNil() {
		err := results[1].Interface().(error)
		slog.Error("GetClient: constructor returned error", "function", fct, "address", address, "service", name, "err", err)
		return nil, err
	}
	client := results[0].Interface().(Client)
	clients.Store(id, client)
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
	localAddr, _ := config.GetAddress()
	localCfg, err := config.GetLocalConfig(true)
	if err != nil || localCfg == nil {
		slog.Error("InitClient: cannot read local configuration", "err", err)
		return fmt.Errorf("InitClient: cannot read local configuration: %w", err)
	}

	// --- Normalize control-plane address (host:port) and detect locality
	address = normalizeControlAddress(address, localAddr, localCfg)
	host, ctrlPort := splitHostPort(address)
	if ctrlPort == 0 {
		return fmt.Errorf("InitClient: invalid control address %q (missing port)", address)
	}
	localHost := strings.Split(localAddr, ":")[0]
	isLocal := (host == localHost || host == "localhost" || strings.HasPrefix(localHost, host))

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

	// --- TLS / mTLS setup
	if err := setupClientTLS(client, cfg, isLocal, host, ctrlPort); err != nil {
		return err
	}

	slog.Debug("InitClient: client initialized",
		"id", client.GetId(), "name", client.GetName(), "domain", client.GetDomain(),
		"grpc_port", client.GetPort(), "tls", client.HasTLS(), "address", client.GetAddress())

	// --- Optional readiness: gRPC Health
	if opts.waitHealth {
		if err := waitForHealthReady(client, host, opts); err != nil {
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

// normalizeControlAddress converts an input like "peer-domain" or "host"
// into "host:port" based on local config, peers list, and protocol defaults.
// It prefers the peer's HTTPS control port when the peer's Protocol == "https".
func normalizeControlAddress(address, localAddr string, localCfg map[string]interface{}) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return address
	}

	// If it's already host:port, keep it.
	if strings.Count(address, ":") == 1 {
		return address
	}

	// If it's an IP and equals our local IP, map to local control address.
	if ip := net.ParseIP(address); ip != nil {
		if localIP, _ := Utility.GetIpv4(localAddr); ip.String() == localIP {
			return localAddr // already host:port
		}
	}

	// If it matches our local host/domain, use local control port per local protocol.
	if strings.HasPrefix(localAddr, address) || strings.Split(localAddr, ":")[0] == address {
		if asString(localCfg["Protocol"]) == "https" {
			return net.JoinHostPort(address, Utility.ToString(localCfg["PortHTTPS"]))
		}
		return net.JoinHostPort(address, Utility.ToString(localCfg["PortHTTP"]))
	}

	// Otherwise, try to resolve from Peers in local config.
	if peers, ok := localCfg["Peers"].([]interface{}); ok {
		for _, pi := range peers {
			p, ok := pi.(map[string]interface{})
			if !ok {
				continue
			}
			if asString(p["Domain"]) != address {
				continue
			}
			host := asString(p["Hostname"])
			if d := asString(p["Domain"]); d != "" && d != "localhost" {
				host += "." + d
			}
			// Prefer explicit peer HTTPS/HTTP ports if they exist; else fall back to legacy "Port".
			if strings.EqualFold(asString(p["Protocol"]), "https") {
				if v := Utility.ToString(p["PortHTTPS"]); v != "" && v != "0" {
					return net.JoinHostPort(host, v)
				}
			} else {
				if v := Utility.ToString(p["PortHTTP"]); v != "" && v != "0" {
					return net.JoinHostPort(host, v)
				}
			}
			// Fallback to single Port field if provided.
			if v := Utility.ToString(p["Port"]); v != "" && v != "0" {
				return net.JoinHostPort(host, v)
			}
		}
	}

	// Final fallback: use our local protocol defaults with the given host.
	if asString(localCfg["Protocol"]) == "https" {
		return net.JoinHostPort(address, Utility.ToString(localCfg["PortHTTPS"]))
	}
	return net.JoinHostPort(address, Utility.ToString(localCfg["PortHTTP"]))
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

	// Safe to set on the client afterwards
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

// setupClientTLS configures TLS/mTLS on the client based on desired/runtime cfg.
// For remote peers, it ensures the client certs exist by calling
// security.InstallClientCertificates(...), which in turn talks to your
// HTTPS handlers (/get_ca_certificate, /sign_ca_certificate, etc.) on the
// peer's control-plane port.
func setupClientTLS(client Client, cfg map[string]interface{}, isLocal bool, controlAddr string, ctrlPort int) error {
	tlsEnabled, _ := cfg["TLS"].(bool)
	client.SetTLS(tlsEnabled)
	if !tlsEnabled {
		return nil
	}

	host := strings.TrimSpace(controlAddr)

	if isLocal {
		// Re-map server paths to client paths when the same machine publishes them.
		certFile := strings.ReplaceAll(asString(cfg["CertFile"]), "server", "client")
		keyFile := strings.ReplaceAll(asString(cfg["KeyFile"]), "server", "client")
		client.SetKeyFile(keyFile)
		client.SetCertFile(certFile)
		client.SetCaFile(asString(cfg["CertAuthorityTrust"]))
		return nil
	}

	// Remote peer: ensure client certs exist under /tls/<host>.
	// InstallClientCertificates will reach the peer's control-plane HTTPS/HTTP,
	// using the ctrlPort we derived from the normalized control address.
	path := config.GetConfigDir() + "/tls/" + host
	keyFile, certFile, caFile, err := security.InstallClientCertificates(
		host, ctrlPort, path,
		asString(cfg["Country"]), asString(cfg["State"]), asString(cfg["City"]), asString(cfg["Organization"]),
		nil, // SANs optional here; the security package can fill from server if needed
	)
	if err != nil {
		slog.Error("InitClient: InstallClientCertificates failed", "domain", host, "port", ctrlPort, "err", err)
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

	// Dial options: TLS or insecure
	var dialOpts []grpc.DialOption
	if client.HasTLS() {
		creds, cerr := makeTLSCreds(client.GetCertFile(), client.GetKeyFile(), client.GetCaFile(), targetHost)
		if cerr != nil {
			return fmt.Errorf("InitClient: building TLS creds failed: %w", cerr)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

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
	certFile := client.GetCertFile()
	if certFile == "" {
		return nil, errors.New("GetClientTlsConfig: missing client certificate file for client " + client.GetName())
	}
	keyFile := client.GetKeyFile()
	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("GetClientTlsConfig: load key pair failed (cert=%s, key=%s): %w", certFile, keyFile, err)
	}
	certPool := x509.NewCertPool()
	caPem, err := os.ReadFile(client.GetCaFile())
	if err != nil {
		return nil, fmt.Errorf("GetClientTlsConfig: read CA file failed: %w", err)
	}
	if ok := certPool.AppendCertsFromPEM(caPem); !ok {
		return nil, errors.New("GetClientTlsConfig: append CA certificate failed")
	}
	sni := strings.Split(client.GetAddress(), ":")[0]
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_TLS_SERVERNAME")); v != "" {
		sni = v
	}
	return &tls.Config{
		ServerName:   sni,
		Certificates: []tls.Certificate{certificate},
		RootCAs:      certPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
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
						// Dedup once per service, then stop watching to avoid reconnect/spam.
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
							// Re-run the same logic from InitClient’s TLS section
							// Wrap it into a helper so you can call it from both places.
							if err := populateClientTLS(c); err != nil {
								slog.Warn("client watch: TLS populate failed; keeping TLS disabled until ready",
									"service", c.GetName(), "err", err)
								c.SetTLS(false) // prevent broken TLS dials
							}
						}

						if dedup.ShouldLog("endpoint-change:"+c.GetId(), envGetDuration("GLOBULAR_CLIENT_DEDUP_WINDOW", 2*time.Second)) {
							infoQuiet("client watch: endpoint changed; reconnecting",
								"service", c.GetName(), "id", c.GetId(),
								"old_port", oldPort, "new_port", c.GetPort(),
								"old_tls", oldTLS, "new_tls", c.HasTLS())
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

				// If service requested to close, future calls will fail fast anyway.
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
// For remote peers we use the **peer's control port** (from c.GetAddress()) so
// that the security package can call the peer's HTTPS handlers correctly.
func populateClientTLS(c Client) error {
	host, _ := func() (string, int) {
		parts := strings.Split(strings.TrimSpace(c.GetAddress()), ":")
		if len(parts) >= 2 {
			p, _ := strconv.Atoi(parts[len(parts)-1])
			return strings.Join(parts[:len(parts)-1], ":"), p
		}
		return c.GetAddress(), 0
	}()

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

	// Remote: use the **peer's** control port from c.GetAddress(), not our local ports.
	ctrlPort := controlPortFromAddress(c.GetAddress())
	if ctrlPort == 0 {
		return fmt.Errorf("invalid control address %q: no port", c.GetAddress())
	}

	path := config.GetConfigDir() + "/tls/" + host
	keyFile, certFile, caFile, err := security.InstallClientCertificates(
		host, ctrlPort, path,
		"", "", "", "", // org info optional here
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
