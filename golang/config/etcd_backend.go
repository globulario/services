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
)

var (
	cliMu     sync.Mutex
	cliShared *clientv3.Client
)

func etcdClient() (*clientv3.Client, error) {
	cliMu.Lock()
	defer cliMu.Unlock()

	// Reuse if still healthy.
	if cliShared != nil {
		if probeEtcdHealthy(cliShared, 1500*time.Millisecond) == nil {
			return cliShared, nil
		}
		_ = cliShared.Close()
		cliShared = nil
	}

	// Build endpoints (with scheme hints) from env / local config.
	raw := etcdEndpointsFromEnv() // may contain https://
	// ...
	// Decide if we must use TLS:
	//   - true if any endpoint scheme is https
	//   - or if local server TLS files are present
	forceTLS := false
	for _, ep := range raw {
		if strings.HasPrefix(strings.TrimSpace(strings.ToLower(ep)), "https://") {
			forceTLS = true
			break
		}
	}
	if !forceTLS && etcdServerTLSExists() {
		forceTLS = true
	}

	// Normalize to host:port for the client (TLS is specified separately).
	hostports := make([]string, 0, len(raw))
	for _, ep := range raw {
		u, err := url.Parse(ep)
		if err != nil {
			continue
		}
		h := u.Host
		if h == "" {
			h = strings.TrimPrefix(ep, "https://")
			h = strings.TrimPrefix(h, "http://")
		}
		// Ensure a port is present.
		host, port, err := net.SplitHostPort(h)
		if err != nil {
			host = h
			port = "2379"
		}
		hostports = append(hostports, net.JoinHostPort(host, port))
	}

	if len(hostports) == 0 {
		return nil, fmt.Errorf("no valid etcd endpoints after normalization")
	}

	cfg := clientv3.Config{
		Endpoints:        hostports,
		DialTimeout:      4 * time.Second,
		AutoSyncInterval: 30 * time.Second,
	}

	// Decide TLS strictly from local server state (and not from whatever a caller passed).
	// This avoids the “first record does not look like a TLS handshake” situation.
	if forceTLS {
		tlsCfg, err := GetEtcdTLS()
		if err != nil {
			return nil, fmt.Errorf("TLS required but not available: %w", err)
		}
		cfg.TLS = tlsCfg
	}

	c, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	if err := probeEtcdHealthy(c, 2*time.Second); err != nil {
		_ = c.Close()
		return nil, err
	}
	cliShared = c
	return cliShared, nil
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

// --- replace this whole block in etcd_backend.go ---

// probe for cert triplet in a directory
func hasServerTriplet(base string) bool {
	crt := filepath.Join(base, "server.crt")
	key := filepath.Join(base, "server.key")
	if _, err := os.Stat(key); os.IsNotExist(err) {
		key = filepath.Join(base, "server.pem")
	}
	ca := filepath.Join(base, "ca.crt")
	return fileExists(crt) && fileExists(key) && fileExists(ca)
}

// etcdEndpointsFromEnv: prefer HTTPS if TLS exists, else HTTP. Always emit scheme.
func etcdEndpointsFromEnv() []string {
	// explicit env (may include scheme) wins
	if s := os.Getenv("GLOBULAR_ETCD_ENDPOINTS"); s != "" {
		return mapSanitized(splitCSV(s))
	}
	if s := os.Getenv("ETCD_ENDPOINTS"); s != "" {
		return mapSanitized(splitCSV(s))
	}
	if s := os.Getenv("ETCDCTL_ENDPOINTS"); s != "" {
		return mapSanitized(splitCSV(s))
	}

	// Build stable advertised host (same logic as server)
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

	scheme := "http"
	if etcdServerTLSExists() {
		scheme = "https"
	}
	eps := []string{
		fmt.Sprintf("%s://127.0.0.1:2379", scheme),
		fmt.Sprintf("%s://%s:2379", scheme, host),
	}

	return mapSanitized(eps)
}

// GetEtcdTLS returns a tls.Config that trusts the local CA and (optionally) presents a client cert.
func GetEtcdTLS() (*tls.Config, error) {
	cfgDir := GetConfigDir()

	// Build search list identical to etcdServerTLSExists
	name := Utility.ToString(GetLocalConfigMust(true)["Name"])
	if name == "" {
		name, _ = GetName()
	}
	dom, _ := GetDomain()

	tryDirs := []string{}
	if name != "" && dom != "" && !strings.Contains(name, ".") {
		tryDirs = append(tryDirs, filepath.Join(cfgDir, "tls", name+"."+dom))
	}
	if name != "" {
		tryDirs = append(tryDirs, filepath.Join(cfgDir, "tls", name))
	}
	if hn, _ := GetHostname(); hn != "" {
		tryDirs = append(tryDirs, filepath.Join(cfgDir, "tls", hn))
	}
	tlsRoot := filepath.Join(cfgDir, "tls")
	if entries, err := os.ReadDir(tlsRoot); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				tryDirs = append(tryDirs, filepath.Join(tlsRoot, e.Name()))
			}
		}
	}
	// de-dup
	uniq := make([]string, 0, len(tryDirs))
	seen := map[string]bool{}
	for _, d := range tryDirs {
		if d != "" && !seen[d] {
			seen[d] = true
			uniq = append(uniq, d)
		}
	}

	var base string
	for _, d := range uniq {
		if hasServerTriplet(d) {
			base = d
			break
		}
	}
	if base == "" {
		return nil, fmt.Errorf("etcd TLS requested but no cert directory found")
	}

	caPath := filepath.Join(base, "ca.crt")
	clientCrt := filepath.Join(base, "client.crt")
	clientKey := filepath.Join(base, "client.pem")


	// Root pool: system + your CA
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

	tcfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    pool,
		// IMPORTANT:
		// - Leave ServerName empty so Go will:
		//   * use SNI automatically for DNS endpoints, and
		//   * verify by IP SAN for 127.0.0.1 / 10.0.0.63
		InsecureSkipVerify: false,
	}

	// Optional mTLS
	if fileExists(clientCrt) && fileExists(clientKey) {
		cert, err := tls.LoadX509KeyPair(clientCrt, clientKey)
		if err != nil {
			return nil, fmt.Errorf("load client keypair: %w", err)
		}
		tcfg.Certificates = []tls.Certificate{cert}
	}
	return tcfg, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// sanitize keeps the scheme and normalizes host:port.
// Returns a full URL string like "https://host:2379".
// etcdServerTLSExists reports whether the local etcd server is configured for TLS.
func etcdServerTLSExists() bool {
	// Match where StartEtcdServer writes certs:
	// <config>/tls/<advHost>/{server.crt, server.key, ca.crt}
	cfgDir := GetConfigDir()
	// We don't know advHost here; check any host directory that has a full triplet.
	tlsRoot := filepath.Join(cfgDir, "tls")
	dirs, err := os.ReadDir(tlsRoot)
	if err != nil {
		return false
	}
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		dir := filepath.Join(tlsRoot, d.Name())
		crt := filepath.Join(dir, "server.crt")
		key := filepath.Join(dir, "server.key")
		if !Utility.Exists(key) {
			key = filepath.Join(dir, "server.pem")
		}
		ca := filepath.Join(dir, "ca.crt")
		if Utility.Exists(crt) && Utility.Exists(key) {
			// ca may be optional if client-cert-auth is false, but keep it permissive:
			return true
		}
		if Utility.Exists(crt) && Utility.Exists(key) && Utility.Exists(ca) {
			return true
		}
	}
	return false
}

// sanitize keeps scheme if present; if missing, picks https when server TLS exists, else http.
// Also upgrades http->https when server TLS exists.
func sanitize(ep string) (string, bool) {
	u := strings.TrimSpace(ep)
	if u == "" {
		return "", false
	}
	hasTLS := etcdServerTLSExists()

	// Add a default scheme so url.Parse works, but choose based on local TLS.
	if !strings.Contains(u, "://") {
		if hasTLS {
			u = "https://" + u
		} else {
			u = "http://" + u
		}
	}
	uu, err := url.Parse(u)
	if err != nil {
		return "", false
	}

	scheme := uu.Scheme
	if hasTLS && scheme == "http" {
		// Caller configured a bare host we defaulted to http; upgrade it.
		scheme = "https"
	}

	host := uu.Hostname()
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		return "", false
	}
	port := uu.Port()
	if port == "" {
		port = "2379"
	}
	return fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(host, port)), true
}

func mapSanitized(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, ep := range in {
		if s, ok := sanitize(ep); ok {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	if len(out) == 0 {
		// If we truly have nothing, prefer https when TLS exists.
		if etcdServerTLSExists() {
			out = []string{"https://127.0.0.1:2379"}
		} else {
			out = []string{"http://127.0.0.1:2379"}
		}
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
			if rres, err := c.Get(ctx, etcdKey(idOrName, rtKey)); err == nil  {
				if len(rres.Kvs) == 1 {
					_ = json.Unmarshal(rres.Kvs[0].Value, &r)
				}
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
	current, err := GetRuntime(id)
	if err != nil || current == nil {
		current = map[string]any{}
	}
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
