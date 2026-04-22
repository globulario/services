package config

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
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

// ErrEtcdCorrupt is returned when etcd appears to be in a CORRUPT alarm state
// or when errors clearly indicate a corrupted data-dir.
var ErrEtcdCorrupt = errors.New("etcd store appears to be corrupted")

func etcdClient() (*clientv3.Client, error) {
	cliMu.Lock()
	defer cliMu.Unlock()

	// Reuse existing client if available.
	if cliShared != nil {
		if err := probeEtcdHealthy(cliShared, 4*time.Second); err == nil {
			return cliShared, nil
		} else if errors.Is(err, ErrEtcdCorrupt) {
			// Hard corruption — close and report.
			_ = cliShared.Close()
			cliShared = nil
			return nil, ErrEtcdCorrupt
		}
		// Transient failure (timeout, reconnecting, etc.) — return the
		// existing client anyway.  The etcd client library handles
		// reconnection internally; closing it here destroys the connection
		// for ALL goroutines that hold a reference (leader election,
		// resource store watches, etc.) and causes cascading failures.
		return cliShared, nil
	}

	// Build endpoints (with scheme hints) from env / local config.
	raw := etcdEndpointsFromEnv() // may contain https://
	hostports := normalizeEndpoints(raw)

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

// NewEtcdClient creates a brand-new, independent etcd client with the same
// TLS configuration and endpoints as the shared client.  The caller owns the
// returned client and must Close() it when done.  Use this when you need a
// long-lived client whose lifecycle must NOT be coupled to the shared
// singleton (e.g. leader election sessions, watches that must survive config
// probes).
func NewEtcdClient() (*clientv3.Client, error) {
	raw := etcdEndpointsFromEnv()
	hostports := normalizeEndpoints(raw)
	if len(hostports) == 0 {
		return nil, fmt.Errorf("no valid etcd endpoints after normalization")
	}

	tlsCfg, err := GetEtcdTLS()
	if err != nil {
		return nil, fmt.Errorf("TLS required but not available: %w", err)
	}

	c, err := clientv3.New(clientv3.Config{
		Endpoints:        hostports,
		DialTimeout:      4 * time.Second,
		AutoSyncInterval: 30 * time.Second,
		TLS:              tlsCfg,
	})
	if err != nil {
		return nil, err
	}
	if err := probeEtcdHealthy(c, 2*time.Second); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
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

// normalizeEndpoints strips URL schemes and ensures host:port format for the
// etcd client library (which requires bare host:port, TLS is configured separately).
func normalizeEndpoints(raw []string) []string {
	hostports := make([]string, 0, len(raw))
	seen := map[string]bool{}
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
		host, port, err := net.SplitHostPort(h)
		if err != nil {
			host = h
			port = "2379"
		}
		hp := net.JoinHostPort(host, port)
		if !seen[hp] {
			seen[hp] = true
			hostports = append(hostports, hp)
		}
	}
	return preferLocalEtcdEndpoint(hostports)
}

// preferLocalEtcdEndpoint reorders endpoints so local addresses are first.
// This avoids retry storms when the first endpoint is a down remote member
// while a local etcd instance is available.
func preferLocalEtcdEndpoint(in []string) []string {
	if len(in) < 2 {
		return in
	}
	localHosts := map[string]bool{
		"127.0.0.1": true,
		"localhost": true,
	}
	if ip := strings.TrimSpace(GetRoutableIPv4()); ip != "" {
		localHosts[ip] = true
	}
	if name, err := os.Hostname(); err == nil {
		name = strings.TrimSpace(strings.ToLower(name))
		if name != "" {
			localHosts[name] = true
		}
	}

	local := make([]string, 0, len(in))
	remote := make([]string, 0, len(in))
	for _, ep := range in {
		host, _, err := net.SplitHostPort(ep)
		if err != nil {
			remote = append(remote, ep)
			continue
		}
		if localHosts[strings.ToLower(strings.TrimSpace(host))] {
			local = append(local, ep)
		} else {
			remote = append(remote, ep)
		}
	}
	if len(local) == 0 {
		return in
	}
	return append(local, remote...)
}

// etcdEndpointsFile is the path where the cluster controller renders the list
// of all etcd member endpoints. Each line is a URL like "https://10.0.0.63:2379".
const etcdEndpointsFile = "/var/lib/globular/config/etcd_endpoints"

// etcdEndpointsFromEnv resolves etcd endpoints from on-disk config only.
// Environment variables are NOT consulted — the endpoints file and local
// config are the sole sources of truth.
func etcdEndpointsFromEnv() []string {
	// Read cluster-rendered endpoints file (written by controller reconciliation).
	// This file contains all etcd member URLs, one per line.
	if eps := readEndpointsFile(etcdEndpointsFile); len(eps) > 0 {
		return mapSanitized(eps)
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

	scheme := "http"
	if etcdServerTLSExists() {
		scheme = "https"
	}

	// Use routable IP for etcd — never 127.0.0.1 (breaks multi-node clusters).
	// Try: 1) hostname FQDN, 2) detected routable IP, 3) raw hostname.
	var eps []string

	if host != "" && host != "0.0.0.0" && host != "::" && host != "[::]" && strings.Contains(host, ".") {
		eps = append(eps, fmt.Sprintf("%s://%s:2379", scheme, host))
	}

	// Always include the routable IP as a direct fallback.
	if routableIP, err := GetRoutableIP(); err == nil {
		ep := fmt.Sprintf("%s://%s:2379", scheme, routableIP)
		found := false
		for _, existing := range eps {
			if existing == ep {
				found = true
				break
			}
		}
		if !found {
			eps = append(eps, ep)
		}
	}

	if len(eps) == 0 {
		// Last resort: use bare hostname but prefer routable IP over
		// unqualified hostname (which won't resolve on other nodes).
		if host != "" && host != "0.0.0.0" {
			if ip := GetRoutableIPv4(); ip != "" {
				eps = append(eps, fmt.Sprintf("%s://%s:2379", scheme, ip))
			} else {
				eps = append(eps, fmt.Sprintf("%s://%s:2379", scheme, host))
			}
		}
	}

	return mapSanitized(eps)
}

// readEndpointsFile reads a newline-separated file of etcd endpoint URLs.
// Returns nil if the file doesn't exist or is empty.
func readEndpointsFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var eps []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			eps = append(eps, line)
		}
	}
	return eps
}

// GetEtcdTLS returns a tls.Config for etcd clients.
//
// Etcd is configured with client-cert-auth: false, meaning it does NOT require
// clients to present a certificate. The client only needs to trust the server's
// TLS certificate (i.e. have the CA). We optionally present the service cert if
// it happens to be available, but it is NOT required.
func GetEtcdTLS() (*tls.Config, error) {
	caPath := GetCACertificatePath()
	if !fileExists(caPath) {
		return nil, fmt.Errorf("TLS required but not available (TLS is mandatory): CA certificate not found at canonical location: %s", caPath)
	}

	caData, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caData) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	cfg := &tls.Config{
		RootCAs:    caPool,
		MinVersion: tls.VersionTLS12,
	}

	// Optionally include the service client cert if it exists.
	// Etcd has client-cert-auth: false so this is never required, but some
	// deployments may choose to enable mutual TLS in future.
	svcDir := filepath.Join(GetStateRootDir(), "pki", "issued", "services")
	certPath := filepath.Join(svcDir, "service.crt")
	keyPath := filepath.Join(svcDir, "service.key")
	if fileExists(certPath) && fileExists(keyPath) {
		cert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err == nil {
			cfg.Certificates = []tls.Certificate{cert}
		}
	}

	return cfg, nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// etcdServerTLSExists reports whether the local etcd server is configured for TLS.
// Etcd uses the canonical service cert at pki/issued/services/service.{crt,key}.
func etcdServerTLSExists() bool {
	// INV-PKI-1: etcd server cert lives in the canonical service cert directory.
	svcDir := filepath.Join(GetStateRootDir(), "pki", "issued", "services")
	if fileExists(filepath.Join(svcDir, "service.crt")) && fileExists(filepath.Join(svcDir, "service.key")) {
		return true
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
		// Last resort: use routable IP, never 127.0.0.1.
		scheme := "http"
		if etcdServerTLSExists() {
			scheme = "https"
		}
		if routableIP, err := GetRoutableIP(); err == nil {
			out = []string{fmt.Sprintf("%s://%s:2379", scheme, routableIP)}
		} else {
			// Absolute fallback — only on truly isolated nodes.
			out = []string{fmt.Sprintf("%s://%s:2379", scheme, GetLocalIP())}
		}
	}
	return out
}

// tiny helper so we don't ignore errors silently
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
