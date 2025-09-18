package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"

	Utility "github.com/globulario/utility"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	cliMu     sync.Mutex
	cliShared *clientv3.Client
)

// etcdClient returns a healthy shared client, selecting transport automatically.
func etcdClient() (*clientv3.Client, error) {
	cliMu.Lock()
	defer cliMu.Unlock()

	if cliShared != nil {
		if err := probeEtcdHealthy(cliShared, 3*time.Second); err == nil {
			return cliShared, nil
		}
		_ = cliShared.Close()
		cliShared = nil
	}

	tlsCfg, err := GetEtcdTLS()
	if err != nil {
		zap.L().Warn("GetEtcdTLS failed; will try HTTP", zap.Error(err))
		tlsCfg = nil
	} else {
		zap.L().Info("GetEtcdTLS succeeded; will try mTLS/TLS first")
	}

	// Build a "TLS but no client cert" config using system roots.
	sysPool, _ := x509.SystemCertPool()
	hostname, _ := GetHostname()


	type attempt struct {
		name     string
		tls      *tls.Config
		insecure bool // force HTTP
	}

	// Order: if we have mTLS config, try that, then plain TLS, then HTTP.
	// If we have no TLS config, try plain TLS (could work with LE/public cert), then HTTP.
	// Allow opting into a plain-TLS attempt via env (off by default).
	tryPlainTLS := strings.EqualFold(strings.TrimSpace(os.Getenv("GLOBULAR_ETCD_TRY_PLAINTLS")), "1") ||
		strings.EqualFold(strings.TrimSpace(os.Getenv("GLOBULAR_ETCD_TRY_PLAINTLS")), "true")

	var orders []attempt
	if tlsCfg != nil {
		orders = []attempt{
			{name: "mTLS", tls: tlsCfg},
		}
		if tryPlainTLS {
			// This will likely fail if etcd enforces client cert auth, so keep it opt-in.
			orders = append(orders, attempt{name: "TLS", tls: &tls.Config{
				MinVersion: tls.VersionTLS12,
				RootCAs:    sysPool,
				// SNI set to advertised hostname (already computed above)
				ServerName: hostname,
			}})
		}
		orders = append(orders, attempt{name: "HTTP", insecure: true})
	} else {
		// No client certs available: go straight to HTTP (loopback).
		orders = []attempt{
			{name: "HTTP", insecure: true},
		}
		if tryPlainTLS {
			orders = append([]attempt{{name: "TLS", tls: &tls.Config{
				MinVersion: tls.VersionTLS12,
				RootCAs:    sysPool,
				ServerName: hostname,
			}}}, orders...)
		}
	}

	deadline := time.Now().Add(20 * time.Second)
	var errs []error

	for time.Now().Before(deadline) {
		for _, o := range orders {
			cfg := clientv3.Config{
				Endpoints:   etcdEndpointsFromEnv(),
				DialTimeout: 4 * time.Second,
			}
			if !o.insecure {
				cfg.TLS = o.tls
			}

			zap.L().Info("etcdClient: trying transport", zap.String("mode", o.name), zap.Any("endpoints", cfg.Endpoints), zap.Bool("tls", cfg.TLS != nil))
			c, err := clientv3.New(cfg)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s dial: %w", o.name, err))
				continue
			}
			if err := probeEtcdHealthy(c, 3*time.Second); err == nil {
				cliShared = c
				return cliShared, nil
			}
			errs = append(errs, fmt.Errorf("%s health probe: %w", o.name, err))
			_ = c.Close()
		}
		time.Sleep(400 * time.Millisecond)
	}

	// Assemble a readable error.
	var b strings.Builder
	for i, e := range errs {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(e.Error())
	}
	return nil, fmt.Errorf("could not determine etcd transport (mTLS/TLS/HTTP failed; endpoint=%v): %s", etcdEndpointsFromEnv(), b.String())
}

// GetEtcdClient returns the shared healthy etcd client.
func GetEtcdClient() (*clientv3.Client, error) {
	return etcdClient()
}

// GetEtcdEndpointsHostPorts exposes the resolved endpoints (host:port form).
func GetEtcdEndpointsHostPorts() []string {

	return etcdEndpointsFromEnv()
}

// probeEtcdHealthy does a simple v3 GET (exercises full client path).
func probeEtcdHealthy(c *clientv3.Client, to time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), to)
	defer cancel()
	_, err := c.Get(ctx, "health-probe", clientv3.WithSerializable())
	return err
}

// -----------------------------
// Endpoint & TLS helpers
// -----------------------------

// etcdEndpointsFromEnv: never return 0.0.0.0; prefer advertised DNS.
func etcdEndpointsFromEnv() []string {
	if s := os.Getenv("GLOBULAR_ETCD_ENDPOINTS"); s != "" {
		return mapSanitized(splitCSV(s))
	}
	if s := os.Getenv("ETCD_ENDPOINTS"); s != "" {
		return mapSanitized(splitCSV(s))
	}
	if s := os.Getenv("ETCDCTL_ENDPOINTS"); s != "" {
		return mapSanitized(splitCSV(s))
	}

	name := Utility.ToString(GetLocalConfigMust(true)["Name"])
	if name == "" {
		if n, _ := GetName(); n != "" {
			name = n
		}
	}
	dom, _ := GetDomain()
	host := strings.TrimSpace(name)
	if dom != "" && !strings.Contains(host, ".") {
		host = host + "." + dom
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "localhost"
	}

	// Return both loopback (HTTP listener) and the advertised host (TLS listener).
	eps := []string{"127.0.0.1:2379", net.JoinHostPort(host, "2379")}
	return mapSanitized(eps)
}

func sanitize(ep string) (string, bool) {
	u := ep
	if !strings.Contains(u, "://") {
		u = "http://" + u
	}
	if uu, err := url.Parse(u); err == nil {
		host := uu.Hostname()
		if host == "0.0.0.0" || host == "" || host == "::" || host == "[::]" {
			return "", false
		}
		port := uu.Port()
		if port == "" {
			port = "2379"
		}
		return net.JoinHostPort(host, port), true
	}
	return "", false
}

func mapSanitized(in []string) []string {
	var out []string
	for _, ep := range in {
		if hp, ok := sanitize(strings.TrimSpace(ep)); ok {
			out = append(out, hp)
		}
	}
	if len(out) == 0 {
		// fallback to DNS-first as in your existing code
		name := Utility.ToString(GetLocalConfigMust(true)["Name"])
		if name == "" {
			if n, _ := GetName(); n != "" {
				name = n
			}
		}
		dom, _ := GetDomain()
		host := strings.TrimSpace(name)
		if dom != "" && !strings.Contains(host, ".") {
			host = host + "." + dom
		}
		if host == "" {
			host = "localhost"
		}
		out = []string{host + ":2379"}
	}
	return out
}

// tiny helper so we don’t ignore errors silently
func GetLocalConfigMust(withCache bool) map[string]interface{} {
	cfg, _ := GetLocalConfig(withCache)
	if cfg == nil {
		cfg = map[string]interface{}{}
	}
	return cfg
}

// GetEtcdTLS returns a *tls.Config for talking to etcd over HTTPS (mTLS).
// Returns (nil, error) if files are missing/invalid.
func GetEtcdTLS() (*tls.Config, error) {

	advHost, _ := GetHostname()
	base := filepath.Join(GetConfigDir(), "tls", advHost)
	caPath := filepath.Join(base, "ca.crt")
	clientCrt := filepath.Join(base, "client.crt")
	clientKey := filepath.Join(base, "client.pem")

	// Check files
	for _, p := range []string{caPath, clientCrt, clientKey} {
		if _, err := os.Stat(p); err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("missing TLS file: %s", p)
			}
			return nil, fmt.Errorf("stat %s: %w", p, err)
		}
	}

	// Root pool: prefer system + your CA
	pool, _ := x509.SystemCertPool()
	if pool == nil {
		pool = x509.NewCertPool()
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read CA: %w", err)
	}
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("append CA failed (invalid PEM?)")
	}

	cert, err := tls.LoadX509KeyPair(clientCrt, clientKey)
	if err != nil {
		return nil, fmt.Errorf("load client keypair: %w", err)
	}

	// (Optional) sanity check the client cert has EKU clientAuth
	if err := ensureClientAuthEKU(cert); err != nil {
		return nil, err
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      pool,                    // verify the server (etcd)
		Certificates: []tls.Certificate{cert}, // present client cert (mTLS)
	}, nil
}

// ensureClientAuthEKU checks the leaf (if parseable) has clientAuth EKU.
func ensureClientAuthEKU(c tls.Certificate) error {
	if len(c.Certificate) == 0 {
		return fmt.Errorf("empty client certificate chain")
	}
	leaf, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		return nil
	} // don't hard fail if we can't parse; optional check
	hasClientAuth := false
	for _, eku := range leaf.ExtKeyUsage {
		if eku == x509.ExtKeyUsageClientAuth {
			hasClientAuth = true
			break
		}
	}
	if !hasClientAuth {
		return fmt.Errorf("client certificate missing ExtKeyUsage=clientAuth")
	}
	return nil
}

func splitCSV(s string) []string {
	var out []string
	f := ""
	for _, r := range s {
		if r == ',' || r == ';' || r == ' ' {
			if f != "" {
				out = append(out, f)
				f = ""
			}
		} else {
			f += string(r)
		}
	}
	if f != "" {
		out = append(out, f)
	}
	return out
}

// -----------------------------
// Logging for etcd client
// -----------------------------

// GLOB_ETCD_LOG: silent|error|warn|info|debug (default: silent)
func etcdZapLoggerFromEnv() *zap.Logger {
	level := strings.ToLower(strings.TrimSpace(os.Getenv("GLOB_ETCD_LOG")))
	switch level {
	case "", "silent", "off", "none":
		return zap.NewNop()
	case "error":
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
		cfg.EncoderConfig.TimeKey = "ts"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		l, _ := cfg.Build()
		return l
	case "warn", "warning":
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
		cfg.EncoderConfig.TimeKey = "ts"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		l, _ := cfg.Build()
		return l
	case "info":
		cfg := zap.NewProductionConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
		cfg.EncoderConfig.TimeKey = "ts"
		cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		l, _ := cfg.Build()
		return l
	case "debug":
		cfg := zap.NewDevelopmentConfig()
		cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		l, _ := cfg.Build()
		return l
	default:
		return zap.NewNop()
	}
}

// -----------------------------
// Desired/runtime split helpers
// -----------------------------
var runtimeKeys = map[string]struct{}{
	"Process":      {},
	"ProxyProcess": {},
	"State":        {},
	"LastError":    {},
	"ModTime":      {}, // ignore in desired
}

func splitDesiredRuntime(s map[string]interface{}) (desired, runtime map[string]interface{}) {
	desired = make(map[string]interface{}, len(s))
	runtime = map[string]interface{}{
		"UpdatedAt": time.Now().Unix(),
	}
	for k, v := range s {
		if _, ok := runtimeKeys[k]; ok {
			if k != "ModTime" {
				runtime[k] = v
			}
			continue
		}
		desired[k] = v
	}
	if _, ok := desired["Id"]; !ok && s["Id"] != nil {
		desired["Id"] = s["Id"]
	}
	return
}

func mergeDesiredRuntime(desired, runtime map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range desired {
		out[k] = v
	}
	for k, v := range runtime {
		out[k] = v
	}
	if out["Process"] == nil {
		out["Process"] = -1
	}
	if out["ProxyProcess"] == nil {
		out["ProxyProcess"] = -1
	}
	if out["State"] == nil {
		out["State"] = "stopped"
	}
	return out
}

// -----------------------------
// Public API (etcd-backed)
// -----------------------------

const (
	etcdPrefix = "/globular/services/"
	cfgKey     = "config"
	rtKey      = "runtime"
	liveKey    = "live"
)

func etcdKey(id, leaf string) string {
	return etcdPrefix + id + "/" + leaf
}

// SaveServiceConfiguration persists desired/runtime in separate keys.
func SaveServiceConfiguration(s map[string]interface{}) error {
	id := Utility.ToString(s["Id"])
	if id == "" {
		return errors.New("SaveServiceConfiguration: missing Id")
	}
	c, err := etcdClient()
	if err != nil {
		return err
	}

	desired, runtime := splitDesiredRuntime(s)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	desBytes, _ := json.MarshalIndent(desired, "", "  ")
	if _, err = c.Put(ctx, etcdKey(id, cfgKey), string(desBytes)); err != nil {
		return fmt.Errorf("save desired: %w", err)
	}

	rtBytes, _ := json.Marshal(runtime)
	if _, err = c.Put(ctx, etcdKey(id, rtKey), string(rtBytes)); err != nil {
		return fmt.Errorf("save runtime: %w", err)
	}
	return nil
}

// IsEtcdHealthy checks any endpoint for health within timeout.
func IsEtcdHealthy(endpoints []string, to time.Duration) bool {
	c, err := etcdClient()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), to)
	defer cancel()
	for _, ep := range endpoints {
		if _, err := c.Status(ctx, ep); err == nil {
			return true
		}
	}
	return false
}

// GetServicesConfigurations lists and merges all services under /globular/services/.
func GetServicesConfigurations() ([]map[string]interface{}, error) {
	c, err := etcdClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.Get(ctx, etcdPrefix, clientv3.WithPrefix())
	if err != nil {

		return nil, err
	}

	desiredByID := map[string]map[string]interface{}{}
	runtimeByID := map[string]map[string]interface{}{}

	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if !strings.HasPrefix(key, etcdPrefix) {
			continue
		}
		rest := strings.TrimPrefix(key, etcdPrefix)
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 {
			continue
		}
		id, leaf := parts[0], parts[1]

		switch leaf {
		case cfgKey:
			var d map[string]interface{}
			if err := json.Unmarshal(kv.Value, &d); err != nil {
				continue
			}
			desiredByID[id] = d
		case rtKey:
			var r map[string]interface{}
			if err := json.Unmarshal(kv.Value, &r); err != nil {
				continue
			}
			runtimeByID[id] = r
		}
	}

	var out []map[string]interface{}
	for id, d := range desiredByID {
		r := runtimeByID[id]
		if r == nil {
			r = map[string]interface{}{}
		}
		m := mergeDesiredRuntime(d, r)
		out = append(out, m)
	}
	return out, nil
}

// GetServiceConfigurationById resolves by exact Id, then by Name among all services.
func GetServiceConfigurationById(idOrName string) (map[string]interface{}, error) {
	c, err := etcdClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	// Try exact Id
	if dres, err := c.Get(ctx, etcdKey(idOrName, cfgKey)); err == nil && len(dres.Kvs) == 1 {
		var d map[string]interface{}
		if json.Unmarshal(dres.Kvs[0].Value, &d) == nil {
			var r map[string]interface{}
			if rres, _ := c.Get(ctx, etcdKey(idOrName, rtKey)); len(rres.Kvs) == 1 {
				_ = json.Unmarshal(rres.Kvs[0].Value, &r)
			}
			return mergeDesiredRuntime(d, r), nil
		}
	}

	// Fallback: scan and match by Name
	all, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}
	for _, s := range all {
		if Utility.ToString(s["Id"]) == idOrName || strings.EqualFold(Utility.ToString(s["Name"]), idOrName) {
			return s, nil
		}
	}
	return nil, fmt.Errorf("no service found with id/name %q", idOrName)
}

// Plural by-name
func GetServicesConfigurationsByName(name string) ([]map[string]interface{}, error) {
	all, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}
	var out []map[string]interface{}
	for _, s := range all {
		if strings.EqualFold(Utility.ToString(s["Name"]), name) {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no services found with name %s", name)
	}
	return out, nil
}

func nonEmpty(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

// Singular by-name: choose the "best" candidate.
func GetServiceConfigurationsByName(name string) (map[string]interface{}, error) {
	candidates, err := GetServicesConfigurationsByName(name)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no services found with name %s", name)
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}

	type ver struct{ major, minor, patch int }
	parseVer := func(v any) ver {
		s := strings.TrimSpace(Utility.ToString(v))
		if s == "" {
			return ver{}
		}
		parts := strings.Split(s, ".")
		out := ver{}
		if len(parts) > 0 {
			out.major, _ = strconv.Atoi(nonEmpty(parts[0]))
		}
		if len(parts) > 1 {
			out.minor, _ = strconv.Atoi(nonEmpty(parts[1]))
		}
		if len(parts) > 2 {
			out.patch, _ = strconv.Atoi(nonEmpty(parts[2]))
		}
		return out
	}

	getUpdatedAt := func(m map[string]interface{}) int64 {
		switch v := m["UpdatedAt"].(type) {
		case int64:
			return v
		case int:
			return int64(v)
		case float64:
			return int64(v)
		case string:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				return n
			}
		}
		return 0
	}

	isRunning := func(m map[string]interface{}) bool {
		return strings.EqualFold(Utility.ToString(m["State"]), "running")
	}

	hasPort := func(m map[string]interface{}) bool {
		return Utility.ToInt(m["Port"]) > 0
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if isRunning(a) != isRunning(b) {
			return isRunning(a)
		}
		ua, ub := getUpdatedAt(a), getUpdatedAt(b)
		if ua != ub {
			return ua > ub
		}
		va, vb := parseVer(a["Version"]), parseVer(b["Version"])
		if va.major != vb.major {
			return va.major > vb.major
		}
		if va.minor != vb.minor {
			return va.minor > vb.minor
		}
		if va.patch != vb.patch {
			return va.patch > vb.patch
		}
		return hasPort(a) && !hasPort(b)
	})
	return candidates[0], nil
}

// -----------------------------
// Runtime helpers
// -----------------------------

// Lightweight runtime getters (kept for compatibility).
func runtimeKey(id string) string { return fmt.Sprintf("/globular/services/%s/runtime", id) }

func GetRuntime(id string) (map[string]any, error) {
	if id == "" {
		return nil, errors.New("GetRuntime: empty id")
	}
	cli, err := etcdClient()
	if err != nil {
		return nil, fmt.Errorf("GetRuntime: etcd connect: %w", err)
	}
	key := runtimeKey(id)

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		resp, err := cli.Get(ctx, key, clientv3.WithSerializable())
		cancel()
		if err == nil {
			if len(resp.Kvs) == 0 {
				nowSec := time.Now().Unix()
				return map[string]any{"Process": -1, "State": "stopped", "LastError": "", "UpdatedAt": nowSec}, nil
			}
			var rt map[string]any
			if uerr := json.Unmarshal(resp.Kvs[0].Value, &rt); uerr != nil {
				return nil, fmt.Errorf("GetRuntime: unmarshal: %w", uerr)
			}
			if _, ok := rt["UpdatedAt"]; !ok {
				rt["UpdatedAt"] = time.Now().Unix()
			}
			return rt, nil
		}
		lastErr = err
		time.Sleep(time.Duration(200*(attempt+1)) * time.Millisecond)
	}
	return nil, fmt.Errorf("GetRuntime: etcd get %s: %w", key, lastErr)
}

func PutRuntime(id string, patch map[string]any) error {
	if id == "" {
		return errors.New("PutRuntime: empty id")
	}
	if patch == nil {
		patch = map[string]any{}
	}
	current, _ := GetRuntime(id)
	for k, v := range patch {
		current[k] = v
	}
	current["UpdatedAt"] = time.Now().Unix()

	b, err := json.Marshal(current)
	if err != nil {
		return fmt.Errorf("PutRuntime: marshal: %w", err)
	}
	cli, err := etcdClient()
	if err != nil {
		return fmt.Errorf("PutRuntime: etcd connect: %w", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		_, err = cli.Put(ctx, runtimeKey(id), string(b))
		cancel()
		if err == nil {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("PutRuntime: etcd put: %w", err)
}

// -----------------------------
// Live lease (liveness key)
// -----------------------------

var (
	liveMu     sync.Mutex
	liveLeases = map[string]*LiveLease{}
)

type LiveLease struct {
	LeaseID clientv3.LeaseID
	cancel  context.CancelFunc
}

func StartLive(id string, ttlSeconds int64) (*LiveLease, error) {
	c, err := etcdClient()
	if err != nil {
		return nil, err
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 15
	}
	lease := clientv3.NewLease(c)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	g, err := lease.Grant(ctx, ttlSeconds)
	if err != nil {
		return nil, err
	}

	if _, err = c.Put(context.Background(), etcdKey(id, liveKey), "", clientv3.WithLease(g.ID)); err != nil {
		_, _ = lease.Revoke(context.Background(), g.ID)
		return nil, err
	}

	kaCtx, kaCancel := context.WithCancel(context.Background())
	ch, err := lease.KeepAlive(kaCtx, g.ID)
	if err != nil {
		kaCancel()
		_, _ = lease.Revoke(context.Background(), g.ID)
		return nil, err
	}
	go func() {
		for range ch {
		}
	}()

	ll := &LiveLease{LeaseID: g.ID, cancel: kaCancel}
	liveMu.Lock()
	liveLeases[id] = ll
	liveMu.Unlock()
	return ll, nil
}

func StopLive(id string) {
	liveMu.Lock()
	ll := liveLeases[id]
	delete(liveLeases, id)
	liveMu.Unlock()

	if ll == nil {
		return
	}
	ll.cancel()
	if c, err := etcdClient(); err == nil {
		_, _ = c.Lease.Revoke(context.Background(), ll.LeaseID)
	}
}

// -----------------------------
// Runtime watcher
// -----------------------------

type RuntimeEvent struct {
	ID      string
	Runtime map[string]interface{}
}

func WatchRuntimes(ctx context.Context, cb func(RuntimeEvent)) error {
	c, err := etcdClient()
	if err != nil {
		return err
	}

	wch := c.Watch(ctx, etcdPrefix, clientv3.WithPrefix())
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case wr, ok := <-wch:
			if !ok {
				return errors.New("etcd watch channel closed")
			}
			for _, ev := range wr.Events {
				if ev.Kv == nil {
					continue
				}
				key := string(ev.Kv.Key) // /globular/services/<id>/runtime
				if !strings.HasPrefix(key, etcdPrefix) || !strings.HasSuffix(key, "/"+rtKey) {
					continue
				}
				rest := strings.TrimPrefix(key, etcdPrefix)
				parts := strings.SplitN(rest, "/", 2)
				if len(parts) != 2 {
					continue
				}
				id := parts[0]
				var rt map[string]interface{}
				if err := json.Unmarshal(ev.Kv.Value, &rt); err != nil {
					continue
				}
				cb(RuntimeEvent{ID: id, Runtime: rt})
			}
		}
	}
}
