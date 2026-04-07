package config

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ServiceDesc is the strongly-typed structure emitted by `<service> --describe`.
// Keep fields in sync with your services' JSON output.
type ServiceDesc struct {
	Address            string        `json:"Address"`
	AllowAllOrigins    bool          `json:"AllowAllOrigins"`
	AllowedOrigins     string        `json:"AllowedOrigins"`
	CertAuthorityTrust string        `json:"CertAuthorityTrust"`
	CertFile           string        `json:"CertFile"`
	Checksum           string        `json:"Checksum"`
	Dependencies       []string      `json:"Dependencies"`
	Description        string        `json:"Description"`
	Discoveries        []string      `json:"Discoveries"`
	Domain             string        `json:"Domain"`
	Id                 string        `json:"Id"`
	KeepAlive          bool          `json:"KeepAlive"`
	KeepUpToDate       bool          `json:"KeepUpToDate"`
	KeyFile            string        `json:"KeyFile"`
	Keywords           []string      `json:"Keywords"`
	LastError          string        `json:"LastError"`
	Mac                string        `json:"Mac"`
	ModTime            int64         `json:"ModTime"`
	Name               string        `json:"Name"`
	Path               string        `json:"Path"`
	Permissions        []interface{} `json:"Permissions"`
	Platform           string        `json:"Platform"`
	Port               int           `json:"Port"`
	Process            int           `json:"Process"`
	Proto              string        `json:"Proto"`
	Protocol           string        `json:"Protocol"`
	Proxy              int           `json:"Proxy"`
	ProxyProcess       int           `json:"ProxyProcess"`
	PublisherID        string        `json:"PublisherID"`
	Repositories       []string      `json:"Repositories"`
	State              string        `json:"State"`
	TLS                bool          `json:"TLS"`
	Version            string        `json:"Version"`
}

// DiscoverExecutables scans a root folder for service binaries named "*_server" or "*_server.exe".
func DiscoverExecutables(root string) ([]string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("ServicesRoot is empty; set it in local config")
	}
	var bins []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return err
		}
		l := strings.ToLower(info.Name())
		if strings.HasSuffix(l, "_server") || strings.HasSuffix(l, "_server.exe") {
			bins = append(bins, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(bins)
	return bins, nil
}

// ResolveServiceExecutable converts a possibly-directory path into a concrete executable file.
// If `p` is a dir like ".../echo_server", it tries ".../echo_server/echo_server" then
// the first "*_server[.exe]" file inside. Ensures +x on Unix.
func ResolveServiceExecutable(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	fi, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", p, err)
	}
	if fi.Mode().IsRegular() {
		ensureExec(p, fi)
		return p, nil
	}
	if fi.IsDir() {
		base := filepath.Base(p)
		cand := filepath.Join(p, base) // same-name file inside dir
		if st, err := os.Stat(cand); err == nil && st.Mode().IsRegular() {
			ensureExec(cand, st)
			return cand, nil
		}
		// fallback: first *_server or *_server.exe in the dir
		ents, _ := os.ReadDir(p)
		for _, e := range ents {
			if e.IsDir() {
				continue
			}
			n := strings.ToLower(e.Name())
			if strings.HasSuffix(n, "_server") || strings.HasSuffix(n, "_server.exe") {
				full := filepath.Join(p, e.Name())
				if st, err := os.Stat(full); err == nil && st.Mode().IsRegular() {
					ensureExec(full, st)
					return full, nil
				}
			}
		}
		return "", fmt.Errorf("%s is a directory and no executable server was found inside", p)
	}
	return "", fmt.Errorf("unsupported file type for %s", p)
}

func ensureExec(path string, fi os.FileInfo) {
	if runtime.GOOS == "windows" {
		return // .exe doesn't need chmod
	}
	if fi.Mode()&0o111 == 0 {
		_ = os.Chmod(path, fi.Mode()|0o111)
	}
}

// FindServiceBinary walks `root` and returns a FILE whose name or parent dir contains `short`
// and ends with "*_server[.exe]". Always returns a file path (never a directory).
func FindServiceBinary(root, short string) (string, error) {
	short = strings.ToLower(strings.TrimSpace(short))
	if short == "" {
		return "", fmt.Errorf("empty short name")
	}
	var match string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || match != "" {
			return nil
		}
		name := strings.ToLower(d.Name())

		if d.IsDir() {
			if strings.HasSuffix(name, "_server") && strings.Contains(name, short) {
				// prefer same-name file inside dir
				cand := filepath.Join(p, d.Name())
				if st, err := os.Stat(cand); err == nil && st.Mode().IsRegular() {
					ensureExec(cand, st)
					match = filepath.ToSlash(cand)
					return nil
				}
				// fallback: first *_server[.exe] file in that directory
				entries, _ := os.ReadDir(p)
				for _, e := range entries {
					if e.IsDir() {
						continue
					}
					en := strings.ToLower(e.Name())
					if strings.HasSuffix(en, "_server") || strings.HasSuffix(en, "_server.exe") {
						cand = filepath.Join(p, e.Name())
						if st, err := os.Stat(cand); err == nil && st.Mode().IsRegular() {
							ensureExec(cand, st)
							match = filepath.ToSlash(cand)
							return nil
						}
					}
				}
			}
			return nil
		}

		// file case
		parent := strings.ToLower(filepath.Base(filepath.Dir(p)))
		hasSuffix := strings.HasSuffix(name, "_server") || strings.HasSuffix(name, "_server.exe")
		if hasSuffix && (strings.Contains(name, short) || strings.Contains(parent, short)) {
			if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
				ensureExec(p, st)
				match = filepath.ToSlash(p)
			}
			return nil
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if match == "" {
		return "", fmt.Errorf("no service binary found for %q under %s", short, root)
	}
	return match, nil
}

// HostOnly extracts the host from "host", "host:1234" or "[IPv6]:1234".
func HostOnly(in string) string {
	in = strings.TrimSpace(strings.Trim(in, "[]"))
	if h, _, err := splitHostPort(in); err == nil {
		return h
	}
	// best-effort strip trailing :<digits>
	if i := strings.LastIndex(in, ":"); i > 0 {
		if _, err := strconv.Atoi(in[i+1:]); err == nil {
			return in[:i]
		}
	}
	return in
}

func splitHostPort(s string) (host, port string, err error) {
	i := strings.LastIndex(s, ":")
	if i <= 0 {
		return s, "", fmt.Errorf("missing port")
	}
	return s[:i], s[i+1:], nil
}


// RunDescribe executes the specified binary with the "--describe" flag, passing the provided environment variables,
// and waits for its output up to the given timeout. It expects the command's standard output to be a JSON-encoded
// ServiceDesc object, which it unmarshals and returns. If the command fails or the output is not valid JSON,
// an error is returned containing details and any stderr output.
//
// Parameters:
//   - bin: Path to the binary to execute.
//   - timeout: Maximum duration to wait for the command to complete.
//   - env: Additional environment variables to set for the command.
//
// Returns:
//   - ServiceDesc: The unmarshaled service description from the command's output.
//   - error: An error if the command fails or the output is invalid.
func RunDescribe(bin string, timeout time.Duration, env map[string]string) (ServiceDesc, error) {

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "--describe")
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		return ServiceDesc{}, fmt.Errorf("describe error: %w; stderr: %s", err, strings.TrimSpace(stderr.String()))
	}

	
	var d ServiceDesc
	if err := json.Unmarshal(stdout.Bytes(), &d); err != nil {
		return ServiceDesc{}, fmt.Errorf("invalid describe json from %s: %w", bin, err)
	}

	return d, nil
}

// tryLocalServiceConfig attempts to read service endpoint from local config file.
// Returns empty string if not found or error. No network calls, instant.
func tryLocalServiceConfig(serviceName string) string {
	// Check for local service config in standard location
	configPath := filepath.Join(GetConfigDir(), "services", serviceName+".json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return ""
	}

	var svcConfig map[string]interface{}
	if err := json.Unmarshal(data, &svcConfig); err != nil {
		return ""
	}

	// Extract port
	var port int
	switch p := svcConfig["Port"].(type) {
	case int:
		port = p
	case float64:
		port = int(p)
	case string:
		fmt.Sscanf(p, "%d", &port)
	}

	if port > 0 {
		host := "localhost"
		if addr, ok := svcConfig["Address"].(string); ok && addr != "" {
			if strings.Contains(addr, ":") {
				return addr
			}
			host = addr
		}
		return fmt.Sprintf("%s:%d", host, port)
	}

	return ""
}

// tryEtcdServiceConfig attempts to query etcd for service configuration.
// Returns empty string if etcd unavailable or service not found.
// Should be fast-fail if etcd is down (doesn't block).
func tryEtcdServiceConfig(serviceID string) string {
	// Try etcd-based service discovery
	svc, err := ResolveService(serviceID)
	if err != nil || svc == nil {
		return ""
	}

	// Extract port from service config
	var port int
	switch p := svc["Port"].(type) {
	case int:
		port = p
	case float64:
		port = int(p)
	case string:
		fmt.Sscanf(p, "%d", &port)
	}

	if port > 0 {
		host := "localhost"
		if addr, ok := svc["Address"].(string); ok && addr != "" {
			if strings.Contains(addr, ":") {
				return addr
			}
			host = addr
		}
		return fmt.Sprintf("%s:%d", host, port)
	}

	return ""
}

// ResolveDNSGrpcEndpoint discovers the DNS service gRPC endpoint dynamically.
// H1 Hardening: Reordered to prevent etcd-first blocking during Day-0 bootstrap.
// Priority order (with fast timeouts):
// 1. Local service config file (no network, instant)
// 2. Binary --describe (local exec, <1s)
// 3. etcd service configuration (network, may be unavailable at Day-0)
// 4. Fallback default
//
// Returns the DNS gRPC endpoint as "host:port".
func ResolveDNSGrpcEndpoint(fallback string) string {
	// Method 1: Try local service config file (fastest, no network)
	if endpoint := tryLocalServiceConfig("dns"); endpoint != "" {
		return endpoint
	}

	// Method 2: Try --describe from binary (local exec, fast)
	root := GetServicesRoot()
	if root != "" {
		binPath, err := FindServiceBinary(root, "dns")
		if err == nil {
			desc, err := RunDescribe(binPath, 1*time.Second, nil)
			if err == nil && desc.Port > 0 {
				host := "localhost"
				if desc.Address != "" {
					host = desc.Address
				}
				endpoint := fmt.Sprintf("%s:%d", host, desc.Port)
				return endpoint
			}
		}
	}

	// Method 3: Try etcd (last resort for network-based discovery)
	// This may fail/timeout during Day-0 if etcd is not ready
	if endpoint := tryEtcdServiceConfig("dns.DnsService"); endpoint != "" {
		return endpoint
	}

	// Method 4: Fallback default
	return fallback
}

// ResolveDNSResolverEndpoint discovers the DNS resolver listening endpoint.
// This is the UDP/TCP DNS port (typically 53) where the DNS service listens
// for standard DNS queries, not the gRPC port.
//
// Returns the DNS resolver endpoint as "ip:port".
func ResolveDNSResolverEndpoint() string {
	// Standard DNS port — the DNS service always listens on :53.
	return "127.0.0.1:53"
}

// svcPort extracts the Port field from a service config map.
func svcPort(svc map[string]interface{}) int {
	switch p := svc["Port"].(type) {
	case int:
		return p
	case float64:
		return int(p)
	case string:
		v, _ := strconv.Atoi(p)
		return v
	}
	return 0
}

// svcHost extracts the host from a service config map.
// The Address field may be "host:port" or just "host"; Domain is a fallback.
func svcHost(svc map[string]interface{}) string {
	if addr, ok := svc["Address"].(string); ok && addr != "" {
		if h, _, err := net.SplitHostPort(addr); err == nil {
			return h
		}
		return addr
	}
	return "localhost"
}

// NOTE: tryLocalServicesDir was removed. Disk JSON files are no longer written
// or read for service discovery. etcd is the single source of truth.

// tryGatewayConfig fetches the gateway's /config endpoint over HTTPS (or HTTP)
// and returns all endpoints for the named service. This endpoint is accessible
// to any user who has the CA certificate in their home directory, regardless of
// group membership on the server. It always reflects the live service state
// because the gateway updates its config when services register or deregister.
func tryGatewayConfig(serviceName string) []string {
	domain, err := GetDomain()
	if err != nil || domain == "" {
		domain = "localhost"
	}

	// Load CA cert from user home dir (written by generate-user-client-cert.sh).
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir, _ = os.UserHomeDir()
	}
	caPath := filepath.Join(homeDir, ".config", "globular", "tls", domain, "ca.crt")
	caData, err := os.ReadFile(caPath)
	if err != nil {
		// Try canonical system path as fallback (may not be readable by regular users).
		caData, err = os.ReadFile(GetCACertificatePath())
		if err != nil {
			return nil
		}
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caData) {
		return nil
	}

	type attempt struct {
		scheme string
		port   int
	}
	// Try HTTPS (8443) first, then plain HTTP (8080).
	attempts := []attempt{{"https", 8443}, {"http", 8080}}

	for _, a := range attempts {
		url := fmt.Sprintf("%s://localhost:%d/config", a.scheme, a.port)
		var client *http.Client
		if a.scheme == "https" {
			client = &http.Client{
				Timeout: 2 * time.Second,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs:    certPool,
						ServerName: domain,
					},
				},
			}
		} else {
			client = &http.Client{Timeout: 2 * time.Second}
		}

		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		var cfg map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
			continue
		}

		// The gateway config has a "Services" map keyed by UUID.
		services, _ := cfg["Services"].(map[string]interface{})
		var addrs []string
		for _, svcRaw := range services {
			svc, ok := svcRaw.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := svc["Name"].(string)
			if !strings.EqualFold(name, serviceName) {
				continue
			}
			port := svcPort(svc)
			if port == 0 {
				continue
			}
			addrs = append(addrs, fmt.Sprintf("%s:%d", svcHost(svc), port))
		}
		if len(addrs) > 0 {
			return addrs
		}
	}
	return nil
}

// ResolveServiceAddrs returns all available endpoints for a service identified by
// its fully-qualified name (e.g. "authentication.AuthenticationService").
//
// In a cluster, multiple instances may be running on different nodes; this
// function returns all of them so callers can implement load balancing.
//
// Discovery order:
//  1. Local services directory (/var/lib/globular/services/*.json) — reflects the
//     actual running state on this node; works when the CLI user is in the
//     globular group.
//  2. Gateway /config endpoint — accessible to any user with the CA certificate;
//     the gateway always reflects the live service state.
//  3. etcd — authoritative for cross-node cluster discovery, but may contain
//     stale entries from previous runs or reconfigured services.
func ResolveServiceAddrs(serviceName string) []string {
	// 1. Try etcd — single source of truth for service configs.
	if svcs, err := GetServicesConfigurationsByName(serviceName); err == nil && len(svcs) > 0 {
		var addrs []string
		for _, s := range svcs {
			port := svcPort(s)
			if port == 0 {
				continue
			}
			addrs = append(addrs, fmt.Sprintf("%s:%d", svcHost(s), port))
		}
		if len(addrs) > 0 {
			addrs = meshRouteAddrs(addrs)
			slog.Debug("service discovery: resolved via etcd", "service", serviceName, "addrs", addrs)
			return addrs
		}
	}

	// 2. Fall back to gateway config endpoint — useful when etcd is unreachable
	// (e.g. non-root CLI user without etcd certs).
	if addrs := tryGatewayConfig(serviceName); len(addrs) > 0 {
		addrs = meshRouteAddrs(addrs)
		slog.Debug("service discovery: resolved via gateway", "service", serviceName, "addrs", addrs)
		return addrs
	}

	slog.Debug("service discovery: no endpoint found", "service", serviceName)
	return nil
}

// ResolveLocalServiceAddr resolves the local instance of a service from etcd.
// Returns the address exactly as registered — the source of truth for both
// the hostname and the port. No hardcoded fallbacks.
//
// Returns empty string if the service is not found or not running locally.
func ResolveLocalServiceAddr(serviceName string) string {
	svcs, err := GetServicesConfigurationsByName(serviceName)
	if err != nil || len(svcs) == 0 {
		return ""
	}
	localIP := GetRoutableIPv4()
	for _, s := range svcs {
		host := svcHost(s)
		port := svcPort(s)
		if port == 0 {
			continue
		}
		if host == localIP {
			return fmt.Sprintf("%s:%d", host, port)
		}
	}
	return ""
}

// meshRouteAddrs rewrites resolved addresses so that gRPC traffic goes through
// the Envoy service mesh on port 443 instead of hitting direct service ports.
// Envoy uses gRPC path-based routing (e.g. /authentication.AuthenticationService/*)
// so any instance on any node can serve the request. This gives us:
//   - HA failover: if one node's service dies, Envoy routes to another
//   - Load balancing: Envoy distributes across healthy upstream instances
//   - Unified TLS: Envoy terminates/originates TLS, services don't need to
//
// The addresses are deduplicated because multiple services on the same host
// all resolve to host:443.
func meshRouteAddrs(addrs []string) []string {
	seen := make(map[string]bool, len(addrs))
	var mesh []string
	for _, addr := range addrs {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			// Can't parse — keep as-is.
			if !seen[addr] {
				seen[addr] = true
				mesh = append(mesh, addr)
			}
			continue
		}
		// localhost / loopback means "this node, direct port" — never rewrite.
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			direct := net.JoinHostPort(host, port)
			if !seen[direct] {
				seen[direct] = true
				mesh = append(mesh, direct)
			}
			continue
		}
		meshAddr := net.JoinHostPort(host, "443")
		if !seen[meshAddr] {
			seen[meshAddr] = true
			mesh = append(mesh, meshAddr)
		}
	}
	return mesh
}

// ResolveServiceAddr resolves a single endpoint for the named service.
// When multiple instances are available (cluster deployment), one is chosen at
// random to distribute load across instances.
// Returns fallback when no instance can be discovered.
func ResolveServiceAddr(serviceName, fallback string) string {
	addrs := ResolveServiceAddrs(serviceName)
	if len(addrs) == 0 {
		return fallback
	}
	if len(addrs) == 1 {
		return addrs[0]
	}
	// Random load balancing across all available instances.
	return addrs[rand.Intn(len(addrs))]
}
