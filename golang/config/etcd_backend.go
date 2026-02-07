package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	etcdserverpb "go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	Utility "github.com/globulario/utility"
)

var (
	cliMu     sync.Mutex
	cliShared *clientv3.Client
)

// -----------------------------
// etcd client + health / corruption detection
// -----------------------------

// ErrEtcdCorrupt is returned when etcd appears to be in a CORRUPT alarm state
// or when errors clearly indicate a corrupted data-dir.
var ErrEtcdCorrupt = errors.New("etcd store appears to be corrupted")

func etcdClient() (*clientv3.Client, error) {
	cliMu.Lock()
	defer cliMu.Unlock()

	// Reuse if still healthy.
	if cliShared != nil {
		if err := probeEtcdHealthy(cliShared, 1500*time.Millisecond); err == nil {
			return cliShared, nil
		} else {
			_ = cliShared.Close()
			cliShared = nil
			// If etcd is corrupt, bubble that up instead of retrying forever.
			if errors.Is(err, ErrEtcdCorrupt) {
				return nil, ErrEtcdCorrupt
			}
		}
	}

	// Build endpoints (with scheme hints) from env / local config.
	raw := etcdEndpointsFromEnv() // may contain https://

	// TLS is MANDATORY - no longer optional for security
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

	// TLS is MANDATORY for all etcd connections
	tlsCfg, err := GetEtcdTLS()
	if err != nil {
		return nil, fmt.Errorf("TLS required but not available (TLS is mandatory): %w", err)
	}
	cfg.TLS = tlsCfg

	c, err := clientv3.New(cfg)
	if err != nil {
		return nil, err
	}
	if err := probeEtcdHealthy(c, 2*time.Second); err != nil {
		_ = c.Close()
		if errors.Is(err, ErrEtcdCorrupt) {
			return nil, ErrEtcdCorrupt
		}
		return nil, err
	}
	cliShared = c
	return cliShared, nil
}

// GetEtcdClient returns the shared healthy etcd client.
func GetEtcdClient() (*clientv3.Client, error) {
	return etcdClient()
}

// GetEtcdEndpointsHostPorts exposes the resolved endpoints (currently as URL strings).
func GetEtcdEndpointsHostPorts() []string {
	return etcdEndpointsFromEnv()
}

// isCorruptionError is a cheap text-based check for corruption errors coming from etcd.
func isCorruptionError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "corrupt") || strings.Contains(s, "corruption")
}

// checkEtcdCorruptAlarm queries etcd's alarm list and returns ErrEtcdCorrupt
// if the CORRUPT alarm is present.
func checkEtcdCorruptAlarm(c *clientv3.Client, to time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), to)
	defer cancel()

	m := clientv3.NewMaintenance(c)
	resp, err := m.AlarmList(ctx)
	if err != nil {
		return err
	}
	for _, a := range resp.Alarms {
		if a.Alarm == etcdserverpb.AlarmType_CORRUPT {
			return ErrEtcdCorrupt
		}
	}
	return nil
}

// probeEtcdHealthy does a simple v3 GET (exercises full client path) and also
// checks the CORRUPT alarm.
func probeEtcdHealthy(c *clientv3.Client, to time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), to)
	defer cancel()

	_, err := c.Get(ctx, "health-probe", clientv3.WithSerializable())
	if err != nil {
		// If we see "corrupt" in the error, treat it as a hard corruption signal.
		if isCorruptionError(err) {
			return ErrEtcdCorrupt
		}
		// Best-effort alarm check as a second opinion.
		if aerr := checkEtcdCorruptAlarm(c, to); aerr == ErrEtcdCorrupt {
			return aerr
		}
		return err
	}

	// Even if Get works, quickly check alarms; etcd may have raised CORRUPT.
	if aerr := checkEtcdCorruptAlarm(c, to); aerr == ErrEtcdCorrupt {
		return aerr
	}
	// Ignore other maintenance errors; health probe is otherwise fine.
	return nil
}

// -----------------------------
// TLS helpers and endpoints
// -----------------------------

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

	canonical := filepath.Join(cfgDir, "tls", "etcd")
	legacyAllowed := strings.TrimSpace(os.Getenv("GLOBULAR_ALLOW_LEGACY_TLS_DIRS")) == "1"
	var base string
	if hasServerTriplet(canonical) {
		base = canonical
	} else if legacyAllowed {
		tryDirs := []string{}
		tlsRoot := filepath.Join(cfgDir, "tls")
		if entries, err := os.ReadDir(tlsRoot); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					tryDirs = append(tryDirs, filepath.Join(tlsRoot, e.Name()))
				}
			}
		}
		uniq := make([]string, 0, len(tryDirs))
		seen := map[string]bool{}
		for _, d := range tryDirs {
			if d != "" && !seen[d] {
				seen[d] = true
				uniq = append(uniq, d)
			}
		}
		for _, d := range uniq {
			if hasServerTriplet(d) {
				base = d
				break
			}
		}
	} else {
		return nil, fmt.Errorf("etcd TLS requested but no cert directory found (looked in %s)", canonical)
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
		MinVersion:         tls.VersionTLS12,
		RootCAs:            pool,
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

// etcdServerTLSExists reports whether the local etcd server is configured for TLS.
func etcdServerTLSExists() bool {
	// Match where StartEtcdServer writes certs:
	cfgDir := GetConfigDir()
	canonical := filepath.Join(cfgDir, "tls", "etcd")
	if hasServerTriplet(canonical) {
		return true
	}
	if strings.TrimSpace(os.Getenv("GLOBULAR_ALLOW_LEGACY_TLS_DIRS")) == "1" {
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
			if hasServerTriplet(dir) {
				return true
			}
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
	etcdPrefix          = "/globular/services/"
	cfgKey              = "config"
	rtKey               = "runtime"
	liveKey             = "live"
	globularRootPrefix  = "/globular/"
	etcdSnapshotDirName = "etcd-snapshots"
	servicesBackupName  = "globular_config_backup.json"
)

func etcdKey(id, leaf string) string {
	return etcdPrefix + id + "/" + leaf
}

// BootstrapServicesFromFiles loads all JSON configs from
// the services config directory (default: /var/lib/globular/services)
// and applies them to etcd by calling SaveServiceConfiguration for each.
//
// It is safe to call this on every startup: it will simply re-assert the
// desired configs into etcd (overwriting whatever was there with the
// file's contents).
func BootstrapServicesFromFiles() error {
	dir := GetServicesConfigDir()

	fi, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			// nothing to bootstrap yet
			return nil
		}
		return fmt.Errorf("bootstrap: stat %s: %w", dir, err)
	}
	if !fi.IsDir() {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("bootstrap: readdir %s: %w", dir, err)
	}

	count := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".json") {
			continue
		}

		path := filepath.Join(dir, name)
		b, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("BootstrapServicesFromFiles: read %s: %v\n", path, err)
			continue
		}

		var desired map[string]interface{}
		if err := json.Unmarshal(b, &desired); err != nil {
			fmt.Printf("BootstrapServicesFromFiles: unmarshal %s: %v\n", path, err)
			continue
		}

		id := Utility.ToString(desired["Id"])
		if id == "" {
			base := strings.TrimSuffix(name, filepath.Ext(name))
			id = base
			desired["Id"] = id
		}

		if err := SaveServiceConfiguration(desired); err != nil {
			fmt.Printf("BootstrapServicesFromFiles: SaveServiceConfiguration(%s): %v\n", id, err)
			continue
		}
		count++
	}

	fmt.Printf("BootstrapServicesFromFiles: loaded %d service configs from %s\n", count, dir)
	return nil
}

// SaveServiceConfiguration persists desired/runtime in separate keys
// and mirrors the desired config to a JSON file on disk:
//
//	<ServicesConfigDir>/<Id>.json  (default: /var/lib/globular/services/<Id>.json)
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

	// Mirror desired config to disk (best effort).
	if err := saveServiceConfigFile(id, desired); err != nil {
		fmt.Printf("SaveServiceConfiguration: failed to persist %s to disk: %v\n", id, err)
	}

	// Fire-and-forget backup of /globular keys (best-effort).
	go func() {
		_, _ = BackupGlobularKeysJSON()
	}()

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
			if rres, err := c.Get(ctx, etcdKey(idOrName, rtKey)); err == nil {
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

// -----------------------------
// Backup & snapshot helpers
// -----------------------------

// CreateEtcdSnapshot saves a binary etcd snapshot under
//
//	<configDir>/etcd-snapshots/etcd-<unix>.db
//
// and returns the snapshot filepath.
//
// You still need to use "etcdutl snapshot restore" offline to rebuild
// a corrupted data-dir from this file, but this gives you the snapshot.
func CreateEtcdSnapshot() (string, error) {
	c, err := etcdClient()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	m := clientv3.NewMaintenance(c)
	r, err := m.Snapshot(ctx)
	if err != nil {
		return "", fmt.Errorf("snapshot: %w", err)
	}
	defer r.Close()

	snapDir := filepath.Join(GetConfigDir(), etcdSnapshotDirName)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return "", fmt.Errorf("snapshot mkdir: %w", err)
	}

	fn := filepath.Join(snapDir, fmt.Sprintf("etcd-%d.db", time.Now().Unix()))
	f, err := os.Create(fn)
	if err != nil {
		return "", fmt.Errorf("snapshot create: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("snapshot copy: %w", err)
	}
	return fn, nil
}

// BackupGlobularKeysJSON exports all keys under "/globular/" (including
// /globular/services, /globular/accounts, etc.) to a JSON file:
//
//	<configDir>/backups/globular_config_backup.json
//
// It returns the full path to the backup file.
func BackupGlobularKeysJSON() (string, error) {
	c, err := etcdClient()
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := c.Get(ctx, globularRootPrefix, clientv3.WithPrefix())
	if err != nil {
		return "", fmt.Errorf("backup etcd get: %w", err)
	}

	type kv struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	payload := struct {
		CreatedAt int64 `json:"created_at"`
		Items     []kv  `json:"items"`
	}{
		CreatedAt: time.Now().Unix(),
		Items:     make([]kv, 0, len(resp.Kvs)),
	}
	for _, kvp := range resp.Kvs {
		payload.Items = append(payload.Items, kv{
			Key:   string(kvp.Key),
			Value: string(kvp.Value),
		})
	}

	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("backup marshal: %w", err)
	}

	backupDir := filepath.Join(GetConfigDir(), "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", fmt.Errorf("backup mkdir: %w", err)
	}

	tmp := filepath.Join(backupDir, servicesBackupName+".tmp")
	final := filepath.Join(backupDir, servicesBackupName)

	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return "", fmt.Errorf("backup write tmp: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return "", fmt.Errorf("backup rename: %w", err)
	}
	return final, nil
}

// RestoreGlobularKeysJSON replays all keys from a JSON backup file created
// by BackupGlobularKeysJSON into the current etcd cluster.
//
// Use this AFTER you have rebuilt or re-initialized your etcd data-dir.
func RestoreGlobularKeysJSON(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("restore read file: %w", err)
	}

	var payload struct {
		CreatedAt int64 `json:"created_at"`
		Items     []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"items"`
	}
	if err := json.Unmarshal(b, &payload); err != nil {
		return fmt.Errorf("restore unmarshal: %w", err)
	}

	c, err := etcdClient()
	if err != nil {
		return fmt.Errorf("restore etcd connect: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, kv := range payload.Items {
		if _, err := c.Put(ctx, kv.Key, kv.Value); err != nil {
			return fmt.Errorf("restore put %s: %w", kv.Key, err)
		}
	}
	return nil
}

// saveServiceConfigFile writes the "desired" config to
//
//	<ServicesConfigDir>/<id>.json  (default: /var/lib/globular/services/<id>.json)
//
// using an atomic tmp+rename write.
func saveServiceConfigFile(id string, desired map[string]interface{}) error {
	if id == "" {
		return errors.New("saveServiceConfigFile: empty id")
	}

	dir := GetServicesConfigDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("saveServiceConfigFile mkdir: %w", err)
	}

	// Ensure Id is present in the payload.
	if _, ok := desired["Id"]; !ok {
		desired["Id"] = id
	}

	b, err := json.MarshalIndent(desired, "", "  ")
	if err != nil {
		return fmt.Errorf("saveServiceConfigFile marshal: %w", err)
	}

	tmp := filepath.Join(dir, id+".json.tmp")
	final := filepath.Join(dir, id+".json")

	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("saveServiceConfigFile write tmp: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("saveServiceConfigFile rename: %w", err)
	}
	return nil
}
