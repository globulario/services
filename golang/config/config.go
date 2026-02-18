// ==============================================
// config.go (system config + helpers; service config lives in etcd_backend.go)
// ==============================================
package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emicklei/proto"
	"github.com/globulario/services/golang/netutil"
	Utility "github.com/globulario/utility"
)

// NOTE: etcd-backed service helpers (SaveServiceConfiguration, PutRuntime,
// GetServicesConfigurations*, StartLive/StopLive, etcdClient, etc.) are defined
// in etcd_backend.go. Do NOT re-declare them here.

// ============================================================================
// Globals
// ============================================================================

var (
	// in-memory cache of local config when lazy=true
	config_ map[string]interface{}
)

// ============================================================================
// etcd keys (system config only)
// ============================================================================

const (
	etcdSystemConfigKey = "/globular/system/config"
)

// ============================================================================
// Addressing / Identity
// ============================================================================
type PortAllocator struct {
	from, to int
	mu       sync.Mutex
	used     map[int]string // port -> serviceId (for debugging/traceability)
}

// GetPortsRange returns the configured ports range as "from-to".
// Order of precedence:
//  1. env GLOB_PORTS_RANGE (e.g. "10020-10199")
//  2. global config key "PortsRange" (if you store it there; optional hook shown below)
//  3. fallback default "10000-20000"
func GetPortsRange() string {
	if v := strings.TrimSpace(os.Getenv("GLOB_PORTS_RANGE")); v != "" {
		return v
	}

	// OPTIONAL: if your global config has it; ignore errors silently.
	if gc, err := GetLocalConfig(true); err == nil && gc != nil {
		if pr, ok := gc["PortsRange"]; ok {
			if s := strings.TrimSpace(Utility.ToString(pr)); s != "" {
				return s
			}
		}
	}

	return "10000-20000"
}

// NewPortAllocator creates an allocator from a "from-to" string (e.g. "10020-10199").
func NewPortAllocator(rangeStr string) (*PortAllocator, error) {
	p := &PortAllocator{used: make(map[int]string)}
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid PortsRange %q", rangeStr)
	}
	p.from = Utility.ToInt(parts[0])
	p.to = Utility.ToInt(parts[1])
	if p.from <= 0 || p.to <= p.from {
		return nil, fmt.Errorf("invalid PortsRange bounds %q", rangeStr)
	}
	return p, nil
}

// NewDefaultPortAllocator builds an allocator using GetPortsRange() and preloads "used"
// from all current service configs (Port & Proxy).
func NewDefaultPortAllocator() (*PortAllocator, error) {

	p, err := NewPortAllocator(GetPortsRange())
	if err != nil {
		return nil, err
	}

	fmt.Println("PortAllocator range:", p.from, "-", p.to)
	// preload from etcd
	all, err := GetServicesConfigurations()
	if err != nil {
		// not fatal; return empty allocator
		return p, nil
	}
	p.ReserveExisting(all)
	return p, nil
}

// ReserveExisting marks existing services' Port and Proxy as used.
func (p *PortAllocator) ReserveExisting(services []map[string]interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range services {
		id := Utility.ToString(s["Id"])
		if port := Utility.ToInt(s["Port"]); port > 0 {
			p.used[port] = id
		}
		if proxy := Utility.ToInt(s["Proxy"]); proxy > 0 {
			p.used[proxy] = id
		}
	}
}

// ReservePort marks a specific port as used by an id (no-op if out of range).
func (p *PortAllocator) ReservePort(port int, ownerID string) {
	if port <= 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if port >= p.from && port <= p.to {
		p.used[port] = ownerID
	}
}

// IsFree returns true if port is inside the range and not used.
func (p *PortAllocator) IsFree(port int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if port < p.from || port > p.to {
		return false
	}
	_, taken := p.used[port]
	return !taken
}

// Next returns the next free port in-range (greedy ascending).
func (p *PortAllocator) Next(ownerID string) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for port := p.from; port <= p.to; port++ {
		if _, taken := p.used[port]; !taken {
			p.used[port] = ownerID
			return port, nil
		}
	}
	return 0, fmt.Errorf("no free port available in range %d-%d", p.from, p.to)
}

// NextPair returns two ports reserved for ownerID.
// Preference order:
//  1. A contiguous pair p, p+1 (most services assume Proxy = Port+1)
//  2. Any two free ports in-range, if no contiguous pair exists.
//
// If you want to *require* contiguity, remove the fallback block and return an error instead.
func (p *PortAllocator) NextPair(ownerID string) (int, int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 1) Prefer contiguous pairs p, p+1
	for port := p.from; port < p.to; port++ { // ensure port+1 <= p.to
		if _, taken := p.used[port]; taken {
			continue
		}
		if _, taken := p.used[port+1]; taken {
			continue
		}
		p.used[port] = ownerID
		p.used[port+1] = ownerID
		return port, port + 1, nil
	}

	// 2) Fallback: any two free (non-contiguous) ports
	first := 0
	for port := p.from; port <= p.to; port++ {
		if _, taken := p.used[port]; taken {
			continue
		}
		if first == 0 {
			first = port
			continue
		}
		p.used[first] = ownerID
		p.used[port] = ownerID
		return first, port, nil
	}

	return 0, 0, fmt.Errorf("no two free ports available in range %d-%d", p.from, p.to)
}

// ClaimPair reserves the given port/proxy for ownerID if they are in-range and free.
// It is idempotent: if a port is already reserved by the same owner, it's accepted.
func (p *PortAllocator) ClaimPair(ownerID string, port, proxy int) error {
	if port <= 0 || proxy <= 0 || port == proxy {
		return fmt.Errorf("invalid ports %d/%d", port, proxy)
	}
	if port < p.from || port > p.to || proxy < p.from || proxy > p.to {
		return fmt.Errorf("ports %d/%d out of range %d-%d", port, proxy, p.from, p.to)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if other, taken := p.used[port]; taken && other != ownerID {
		return fmt.Errorf("port %d already reserved by %s", port, other)
	}
	if other, taken := p.used[proxy]; taken && other != ownerID {
		return fmt.Errorf("port %d already reserved by %s", proxy, other)
	}

	// Reserve (idempotent for same owner)
	p.used[port] = ownerID
	p.used[proxy] = ownerID
	return nil
}

// Claim reserves the given port for ownerID if it is in-range and free.
// It is idempotent: if the port is already reserved by the same owner, it's accepted.
func (p *PortAllocator) Claim(ownerID string, port int) error {
	if port <= 0 {
		return fmt.Errorf("invalid port %d", port)
	}
	if port < p.from || port > p.to {
		return fmt.Errorf("port %d out of range %d-%d", port, p.from, p.to)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if other, taken := p.used[port]; taken && other != ownerID {
		return fmt.Errorf("port %d already reserved by %s", port, other)
	}

	// Reserve (idempotent for same owner)
	p.used[port] = ownerID
	return nil
}

// DebugUsed returns a sorted snapshot of used ports (useful for logs).
func (p *PortAllocator) DebugUsed() []int {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []int
	for k := range p.used {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}

// AllocatePortForService is a convenience: builds a default allocator and
// returns the next free port reserved for 'id'.
func AllocatePortForService(id string) (int, error) {
	p, err := NewDefaultPortAllocator()
	if err != nil {
		return 0, err
	}
	return p.Next(id)
}

// GetLocalIP returns the local IPv4 for the primary interface, or 127.0.0.1 if it
// cannot be determined.
func GetLocalIP() string {
	mac, err := GetMacAddress()
	if err != nil {
		return "127.0.0.1"
	}
	ip, err := Utility.MyLocalIP(mac)
	if err != nil {
		return "127.0.0.1"
	}
	return ip
}

// GetMacAddress returns the MAC address from local config when available.
// If not set, it derives the MAC from the primary IP interface.
func GetMacAddress() (string, error) {
	localConfig, err := GetLocalConfig(true)
	if err != nil {
		return "", err
	}

	if v, ok := localConfig["Mac"].(string); ok && v != "" {
		return v, nil
	}

	ip, err := Utility.GetPrimaryIPAddress()
	if err != nil {
		return "", fmt.Errorf("failed to retrieve MAC address: %w", err)
	}

	mac, err := Utility.MyMacAddr(ip)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve MAC address: %w", err)
	}
	return mac, nil
}

// GetAddress returns "<name>.<domain>:<port>" or "localhost[:port]" depending on
// protocol and local configuration.
func GetAddress() (string, error) {
	localConfig, err := GetLocalConfig(true)
	if err != nil {
		return "", err
	}

	if addr := strings.TrimSpace(Utility.ToString(localConfig["Address"])); addr != "" {
		return strings.ToLower(addr), nil
	}

	// Fallback to host:port construction (legacy behavior)
	name, err := GetName()
	if err != nil {
		return "", err
	}
	domain, err := GetDomain()
	if err != nil {
		return "", err
	}

	address := name
	if domain != "" {
		if domain == "localhost" {
			address = "localhost"
		} else {
			address = name + "." + domain
		}
	}

	if Utility.ToString(localConfig["Protocol"]) == "https" {
		address += ":" + Utility.ToString(localConfig["PortHTTPS"])
	} else {
		address += ":" + Utility.ToString(localConfig["PortHTTP"])
	}
	return strings.ToLower(address), nil
}

// GetName returns the server name from local config, or falls back to hostname.
func GetName() (string, error) {
	localConfig, err := GetLocalConfig(true)
	if err == nil {
		if s, ok := localConfig["Name"].(string); ok && s != "" {
			return strings.ToLower(s), nil
		}
	}
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	return strings.ToLower(hostname), nil
}

// GetDomain returns the configured domain, "localhost" if empty, or an error
// if no local configuration is available.
func GetDomain() (string, error) {
	localConfig, err := GetLocalConfig(true)
	if err == nil {
		if s, ok := localConfig["Domain"].(string); ok && s != "" {
			return strings.ToLower(s), nil
		}
		return netutil.DefaultClusterDomain(), nil
	}
	return "", errors.New("no local configuration found")
}

// GetGatewayEndpoint returns the Gateway HTTP/HTTPS endpoint for certificate operations.
// This endpoint is used for /get_ca_certificate and /sign_ca_certificate requests.
// Returns: (url, protocol, error) where:
//   - url is the full Gateway endpoint (e.g., "https://node.domain:8443")
//   - protocol is "https" or "http"
//   - error if configuration is missing or invalid
//
// Prefers HTTPS if Protocol="https" and PortHTTPS is configured, otherwise falls back to HTTP.
// This function MUST be used instead of gRPC service ports for certificate bootstrap operations.
func GetGatewayEndpoint() (string, string, error) {
	localConfig, err := GetLocalConfig(true)
	if err != nil {
		return "", "", fmt.Errorf("gateway endpoint: cannot read local configuration: %w", err)
	}

	// Get name and domain for address construction
	name, err := GetName()
	if err != nil {
		return "", "", fmt.Errorf("gateway endpoint: cannot determine name: %w", err)
	}
	domain, err := GetDomain()
	if err != nil {
		return "", "", fmt.Errorf("gateway endpoint: cannot determine domain: %w", err)
	}

	// Construct hostname
	hostname := name
	if domain != "" && domain != "localhost" {
		hostname = name + "." + domain
	} else {
		hostname = "localhost"
	}

	// Determine protocol and port
	protocol := strings.TrimSpace(strings.ToLower(Utility.ToString(localConfig["Protocol"])))
	if protocol == "" {
		protocol = "https" // Default to HTTPS per security hardening
	}

	var port int
	if protocol == "https" {
		port = Utility.ToInt(localConfig["PortHTTPS"])
		if port == 0 {
			port = 8443 // Fallback to standard HTTPS port
		}
	} else {
		port = Utility.ToInt(localConfig["PortHTTP"])
		if port == 0 {
			port = 8080 // Fallback to standard HTTP port
		}
	}

	if port < 1 || port > 65535 {
		return "", "", fmt.Errorf("gateway endpoint: invalid port %d (protocol=%s)", port, protocol)
	}

	url := fmt.Sprintf("%s://%s:%d", protocol, hostname, port)
	return url, protocol, nil
}

func GetHostname() (string, error) {
	name, err := GetName()
	if err != nil {
		return "", err
	}
	domain, err := GetDomain()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s.%s", name, domain), nil
}

// ============================================================================
// Canonical Path Functions (Migration - INV-PKI-1)
// ============================================================================

// GetCACertificatePath returns the ONLY canonical location for the system CA.
// INV-PKI-1: CA only at /var/lib/globular/pki/
func GetCACertificatePath() string {
	return filepath.Join(GetStateRootDir(), "pki", "ca.crt")
}

// GetCAKeyPath returns the ONLY canonical location for the CA private key.
func GetCAKeyPath() string {
	return filepath.Join(GetStateRootDir(), "pki", "ca.key")
}

// GetServiceCertPath returns canonical path for issued service certificate.
// serviceName: "services", "etcd", "gateway", "minio", "dns"
func GetServiceCertPath(serviceName string) (certPath string, keyPath string, err error) {
	base := filepath.Join(GetStateRootDir(), "pki", "issued", serviceName)
	certPath = filepath.Join(base, "service.crt")
	keyPath = filepath.Join(base, "service.key")

	if !Utility.Exists(certPath) {
		return "", "", fmt.Errorf("service certificate not found at canonical location: %s", certPath)
	}
	if !Utility.Exists(keyPath) {
		return "", "", fmt.Errorf("service key not found at canonical location: %s", keyPath)
	}

	return certPath, keyPath, nil
}

// GetDomainCertPath returns canonical path for ACME-issued domain certificate.
// INV-ACME-1: ACME certs only under /var/lib/globular/domains/<fqdn>/
func GetDomainCertPath(fqdn string) (certPath string, keyPath string, err error) {
	base := filepath.Join(GetStateRootDir(), "domains", fqdn)
	certPath = filepath.Join(base, "fullchain.pem")
	keyPath = filepath.Join(base, "privkey.pem")

	if !Utility.Exists(certPath) {
		return "", "", fmt.Errorf("domain certificate not found at canonical location: %s", certPath)
	}
	if !Utility.Exists(keyPath) {
		return "", "", fmt.Errorf("domain key not found at canonical location: %s", keyPath)
	}

	return certPath, keyPath, nil
}

// ============================================================================
// Legacy TLS Functions (kept for compatibility, use canonical paths with fallback)
// ============================================================================

func GetTLSFile(name, domain, file string) string {
	// Map of logical certificate names to possible filenames
	// Supports both traditional naming (server.crt), ACME/Let's Encrypt naming (fullchain.pem),
	// and Envoy naming (tls.crt)
	alternatives := map[string][]string{
		"server.crt": {"server.crt", "fullchain.pem", "cert.pem", "tls.crt"},
		"server.key": {"server.key", "privkey.pem", "key.pem", "tls.key"},
		"ca.crt":     {"ca.crt", "ca.pem"},
		"client.crt": {"client.crt", "cert.pem", "tls.crt"},
		"client.key": {"client.key", "privkey.pem", "key.pem", "tls.key"},
		"client.pem": {"client.pem", "privkey.pem", "key.pem", "tls.key"},
	}

	// Get list of alternative filenames to try (default to original if not in map)
	filenames := alternatives[file]
	if filenames == nil {
		filenames = []string{file}
	}

	// Try each directory with each possible filename
	// Include envoy-xds-client directory for Envoy's client certificates
	dirs := []string{
		GetRuntimeTLSDir(),
		GetAdminTLSDir(),
		"/var/lib/globular/pki/envoy-xds-client/current",
	}
	for _, dir := range dirs {
		for _, filename := range filenames {
			path := filepath.Join(dir, filename)
			if Utility.Exists(path) {
				return path
			}
		}
	}
	return ""
}

// GetLocalServerCertificatePath returns the server certificate path using canonical PKI location.
// Migration: Now uses canonical path only (no legacy fallbacks).
func GetLocalServerCertificatePath() string {
	certPath, _, err := GetServiceCertPath("services")
	if err != nil {
		return ""
	}
	return certPath
}

// GetLocalCACertificate returns the CA certificate path using canonical PKI location.
// Migration: Now uses canonical path only (no legacy fallbacks).
func GetLocalCACertificate() string {
	caPath := GetCACertificatePath()
	if !Utility.Exists(caPath) {
		return ""
	}
	return caPath
}

// GetLocalServerKeyPath returns the server key path using canonical PKI location.
// Migration: Now uses canonical path only (no legacy fallbacks).
func GetLocalServerKeyPath() string {
	_, keyPath, err := GetServiceCertPath("services")
	if err != nil {
		return ""
	}
	return keyPath
}

func GetLocalClientKeyPath() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		name, _ := cfg["Name"].(string)
		domain, _ := cfg["Domain"].(string)
		return GetTLSFile(name, domain, "client.pem")
	}
	return ""
}

func GetLocalClientCertificatePath() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		name, _ := cfg["Name"].(string)
		domain, _ := cfg["Domain"].(string)
		return GetTLSFile(name, domain, "client.crt")
	}
	return ""
}

func GetLocalCertificate() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		name, _ := cfg["Name"].(string)
		domain, _ := cfg["Domain"].(string)
		return GetTLSFile(name, domain, domain+".crt")
	}
	return ""
}

func GetLocalCertificateAuthorityBundle() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		name, _ := cfg["Name"].(string)
		domain, _ := cfg["Domain"].(string)
		return GetTLSFile(name, domain, domain+".issuer.crt")
	}
	return ""
}

// Paths
func GetRootDir() string {
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return strings.ReplaceAll(dir, "\\", "/")
}

func GetGlobularExecPath() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		if p, _ := cfg["Path"].(string); p != "" {
			return p
		}
	}
	return ""
}

func GetPublicDirs() []string {
	public := make([]string, 0)
	services, err := GetServicesConfigurationsByName("file.FileService")
	if err != nil {
		return public
	}
	for _, s := range services {
		if arr, ok := s["Public"].([]interface{}); ok {
			for _, v := range arr {
				if path, ok := v.(string); ok {
					public = append(public, path)
				}
			}
		}
	}
	return public
}

func GetServicesRoot() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		if s, _ := cfg["ServicesRoot"].(string); s != "" {
			return s
		}
	}
	return ""
}

// GetConfigDir returns the configuration directory.
// DEPRECATED: Use GetRuntimeConfigDir() instead. This function is kept for backward compatibility
// but now delegates to GetRuntimeConfigDir() to consolidate all configuration under /var/lib/globular.
func GetConfigDir() string {
	return GetRuntimeConfigDir()
}

func GetRuntimeConfigDir() string {
	// Linux only for now; runtime config is stored directly under the state root directory.
	// Tokens, keys, TLS, and config.json all live directly under /var/lib/globular.
	return GetStateRootDir()
}

func GetRuntimeConfigPath() string {
	return filepath.Join(GetRuntimeConfigDir(), "config.json")
}

func GetAdminConfigPath() string {
	return filepath.Join(GetConfigDir(), "config.json")
}

// GetTokensDir returns the path where runtime tokens are stored.
func GetTokensDir() string {
	return filepath.Join(GetRuntimeConfigDir(), "tokens")
}

// GetKeysDir returns the location where peer keys belong at runtime.
func GetKeysDir() string {
	return filepath.Join(GetRuntimeConfigDir(), "keys")
}

// GetRuntimeTLSDir returns the location for auto-generated TLS materials (stateful).
// INV-PKI-1: Updated to use canonical PKI service certificate directory.
func GetRuntimeTLSDir() string {
	// Canonical PKI location for service certificates
	return filepath.Join(GetRuntimeConfigDir(), "pki", "issued", "services")
}

// CanonicalTLSPaths returns the canonical TLS directory and files for data plane usage.
// INV-PKI-1: Updated to use canonical PKI directory structure.
func CanonicalTLSPaths(runtimeConfigDir string) (tlsDir, fullchain, privkey, ca string) {
	tlsDir = filepath.Join(runtimeConfigDir, "pki", "issued", "services")
	fullchain = filepath.Join(tlsDir, "service.crt")
	privkey = filepath.Join(tlsDir, "service.key")
	ca = filepath.Join(runtimeConfigDir, "pki", "ca.pem")
	return
}

// GetCanonicalPKIDir returns the canonical PKI root directory for CA and service certificates.
// H2 Hardening: Stable, persistent location for PKI material (not in work/ subdirectories).
// Returns: /var/lib/globular/pki
func GetCanonicalPKIDir() string {
	return filepath.Join(GetRuntimeConfigDir(), "pki")
}

// GetCanonicalCAPaths returns canonical paths for the cluster CA certificate authority.
// H2 Hardening: CA private key and certificate in stable, persistent location.
// Returns: (keyPath, certPath, bundlePath) where:
//   - keyPath: /var/lib/globular/pki/ca.key (mode 0400)
//   - certPath: /var/lib/globular/pki/ca.crt (mode 0444)
//   - bundlePath: /var/lib/globular/pki/ca.pem (symlink or copy of ca.crt for compatibility)
func GetCanonicalCAPaths() (keyPath, certPath, bundlePath string) {
	pkiDir := GetCanonicalPKIDir()
	keyPath = filepath.Join(pkiDir, "ca.key")
	certPath = filepath.Join(pkiDir, "ca.crt")
	bundlePath = filepath.Join(pkiDir, "ca.pem")
	return
}

// GetLegacyCAPaths returns the old CA paths for migration compatibility.
// These paths are checked during migration to move existing CAs to canonical locations.
func GetLegacyCAPaths() []string {
	runtimeDir := GetRuntimeConfigDir()
	return []string{
		filepath.Join(runtimeDir, "config", "tls", "work", "ca.key"),
		filepath.Join(runtimeDir, "config", "tls", "work", "ca.crt"),
	}
}

// GetAdminTLSDir returns the location for admin-provided TLS files (read-only).
// INV-PKI-1: Updated to use canonical PKI root directory.
func GetAdminTLSDir() string {
	return filepath.Join(GetConfigDir(), "pki")
}

// EnsureRuntimeDir verifies runtime paths do not resolve under /etc and creates them.
func EnsureRuntimeDir(path string) error {
	clean := filepath.Clean(path)
	if strings.HasPrefix(clean, "/etc/") {
		return fmt.Errorf("refusing to create runtime dir under /etc: %s", clean)
	}
	return Utility.CreateDirIfNotExist(clean)
}

// GetStateRootDir returns the base path services should write into.
// On Linux this defaults to /var/lib/globular unless overridden via GLOBULAR_STATE_DIR.
// On Windows it mirrors the Program Files globular directory.
func GetStateRootDir() string {
	if runtime.GOOS == "windows" {
		var programFilePath string
		if runtime.GOARCH == "386" {
			programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
		} else {
			programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES")
		}
		if strings.TrimSpace(programFilePath) == "" {
			programFilePath = "C:/Program Files"
		}
		return strings.ReplaceAll(programFilePath, "\\", "/") + "/globular"
	}
	if dir := strings.TrimSpace(os.Getenv("GLOBULAR_STATE_DIR")); dir != "" {
		return dir
	}
	return "/var/lib/globular"
}

// GetServicesConfigDir returns the directory where service configs are stored.
// Services store their configs as <uuid>.json files in this directory.
// This can be overridden via the GLOBULAR_SERVICES_DIR environment variable.
func GetServicesConfigDir() string {
	if dir := strings.TrimSpace(os.Getenv("GLOBULAR_SERVICES_DIR")); dir != "" {
		return dir
	}
	return filepath.Join(GetStateRootDir(), "services")
}

func GetDataDir() string {
	return filepath.Join(GetStateRootDir(), "data")
}

func GetWebRootDir() string {
	return filepath.Join(GetStateRootDir(), "webroot")
}

func GetToken(mac string) (string, error) {
	key := strings.ReplaceAll(mac, ":", "_") + "_token"
	candidates := []string{
		filepath.Join(GetTokensDir(), key),
		filepath.Join(GetConfigDir(), "tokens", key),
	}
	var lastErr error
	for _, path := range candidates {
		if !Utility.Exists(path) {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			lastErr = fmt.Errorf("get token: read %s: %w", path, err)
			continue
		}
		return string(data), nil
	}
	if lastErr != nil {
		return "", lastErr
	}
	return "", fmt.Errorf("get token: no token found for mac %s", mac)
}

// ============================================================================
// Service dependency helpers (use etcd_backend.go getters)
// ============================================================================

func OrderDependencies(services []map[string]interface{}) ([]string, error) {
	serviceMap := make(map[string]map[string]interface{})
	for _, s := range services {
		if n, ok := s["Name"].(string); ok && n != "" {
			serviceMap[n] = s
		}
	}

	var ordered []string
	visited := make(map[string]bool)

	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		s, ok := serviceMap[name]
		if !ok {
			return fmt.Errorf("service not found: %s", name)
		}
		visited[name] = true

		if deps, ok := s["Dependencies"].([]interface{}); ok {
			for _, d := range deps {
				if dn, _ := d.(string); dn != "" && !visited[dn] {
					if err := visit(dn); err != nil {
						return err
					}
				}
			}
		}
		if !Utility.Contains(ordered, name) {
			ordered = append(ordered, name)
		}
		return nil
	}

	for _, s := range services {
		if name, _ := s["Name"].(string); name != "" && !visited[name] {
			if err := visit(name); err != nil {
				return nil, err
			}
		}
	}
	return ordered, nil
}

func GetOrderedServicesConfigurations() ([]map[string]interface{}, error) {
	svcs, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}
	order, err := OrderDependencies(svcs)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(order))
	for _, name := range order {
		for _, s := range svcs {
			if s["Name"].(string) == name {
				out = append(out, s)
				break
			}
		}
	}
	return out, nil
}

// ============================================================================
// Local system config: etcd-first; file fallback is bootstrap ONLY
// ============================================================================

func GetLocalConfig(lazy bool) (map[string]interface{}, error) {
	if lazy && config_ != nil {
		return config_, nil
	}

	// 1) Bootstrap fallback: local file for the *system* config (not services)
	cfg := map[string]interface{}{}
	cfgPath := GetConfigDir() + "/config.json"

	if !Utility.Exists(cfgPath) {
		return nil, fmt.Errorf("no local Globular configuration found (etcd empty and no file at %s)", cfgPath)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if lazy {
		config_ = cfg
		return cfg, nil
	}

	// 2) Try etcd system config
	if c, err := etcdClient(); err == nil { // etcdClient is in etcd_backend.go
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if res, err := c.Get(ctx, etcdSystemConfigKey); err == nil && len(res.Kvs) == 1 {
			if jerr := json.Unmarshal(res.Kvs[0].Value, &cfg); jerr == nil {
				if lazy {
					config_ = cfg
					return cfg, nil
				}
				// expand services from etcd
				cfg["Services"] = make(map[string]interface{})
				if svcs, err := GetServicesConfigurations(); err == nil {
					for _, s := range svcs {
						if id, _ := s["Id"].(string); id != "" {
							cfg["Services"].(map[string]interface{})[id] = s
						}
					}
				}
				if name, _ := cfg["Name"].(string); name == "" {
					if n, err := GetName(); err == nil {
						cfg["Name"] = n
					}
				}
				return cfg, nil
			}
		}
	}

	// Services always come from etcd; keep the map present for compatibility.
	cfg["Services"] = make(map[string]interface{})
	if svcs, err := GetServicesConfigurations(); err == nil {
		for _, s := range svcs {
			if id, _ := s["Id"].(string); id != "" {
				cfg["Services"].(map[string]interface{})[id] = s
			}
		}
	}
	if name, _ := cfg["Name"].(string); name == "" {
		if n, err := GetName(); err == nil {
			cfg["Name"] = n
		}
	}
	return cfg, nil
}

// EnsureLocalConfig validates or bootstraps the runtime config under the state directory.
// Returns true if the runtime config was created or rewritten.
func EnsureLocalConfig() (bool, error) {
	stateRoot := GetStateRootDir()
	runtimeDir := GetRuntimeConfigDir()
	if err := EnsureRuntimeDir(runtimeDir); err != nil {
		return false, fmt.Errorf("ensure local config: runtime dir: %w", err)
	}

	cfgPath := GetRuntimeConfigPath()
	var cfg map[string]interface{}

	if Utility.Exists(cfgPath) {
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			return false, fmt.Errorf("ensure local config: read %s: %w", cfgPath, err)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			if bakErr := backupConfigFile(cfgPath); bakErr != nil {
				return false, fmt.Errorf("ensure local config: invalid JSON and backup failed: %v (%w)", bakErr, err)
			}
			cfg = nil
		} else if !isValidLocalConfig(cfg) {
			if bakErr := backupConfigFile(cfgPath); bakErr != nil {
				return false, fmt.Errorf("ensure local config: invalid contents and backup failed: %v", bakErr)
			}
			cfg = nil
		} else {
			// Repair domain/address if needed
			_ = repairLocalConfig(cfg)

			changed := ensureEtcdRuntimeDefaults(cfg, stateRoot)
			if rewrite := normalizeLocalConfigArrays(cfg) || changed; rewrite {
				if err := writeLocalConfig(cfgPath, cfg); err != nil {
					return false, err
				}
				config_ = cfg
				return true, nil
			}
			config_ = cfg
			return false, nil
		}
	}

	opts := buildMinimalLocalConfig()
	if admin := loadAdminConfig(); admin != nil {
		opts = admin
	}
	_ = repairLocalConfig(opts)
	changed := ensureEtcdRuntimeDefaults(opts, stateRoot)
	if rewrite := normalizeLocalConfigArrays(opts) || changed; rewrite {
		// ensure consistent ordering after normalization
	}
	if err := writeLocalConfig(cfgPath, opts); err != nil {
		return false, err
	}
	config_ = opts
	return true, nil
}

func loadAdminConfig() map[string]interface{} {
	adminPath := GetAdminConfigPath()
	if !Utility.Exists(adminPath) {
		return nil
	}
	data, err := os.ReadFile(adminPath)
	if err != nil {
		return nil
	}
	cfg := map[string]interface{}{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	if !isValidLocalConfig(cfg) {
		return nil
	}
	return cfg
}

func isValidLocalConfig(cfg map[string]interface{}) bool {
	name := strings.TrimSpace(Utility.ToString(cfg["Name"]))
	if name == "" {
		return false
	}
	protocol := strings.TrimSpace(strings.ToLower(Utility.ToString(cfg["Protocol"])))
	if protocol == "" {
		return false
	}
	return true
}

func normalizeLocalConfigArrays(cfg map[string]interface{}) bool {
	changed := false
	if setNormalizedSlice(cfg, "Peers") {
		changed = true
	}
	if setNormalizedSlice(cfg, "AlternateDomains") {
		changed = true
	}
	return changed
}

func setNormalizedSlice(cfg map[string]interface{}, key string) bool {
	orig, ok := cfg[key]
	normalized := normalizeStringSlice(orig)
	if ok {
		if _, isSlice := orig.([]string); isSlice {
			return false
		}
	}
	cfg[key] = normalized
	return true
}

func normalizeStringSlice(value interface{}) []string {
	out := make([]string, 0)
	switch v := value.(type) {
	case []string:
		out = append(out, v...)
	case []interface{}:
		for _, e := range v {
			if s, ok := e.(string); ok {
				if trimmed := strings.TrimSpace(s); trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
	case string:
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func repairLocalConfig(cfg map[string]interface{}) bool {
	changed := false

	// Domain repair
	domain := netutil.NormalizeDomain(Utility.ToString(cfg["Domain"]))
	if err := netutil.ValidateClusterDomain(domain); err != nil {
		domain = netutil.DefaultClusterDomain()
		changed = true
	}
	cfg["Domain"] = domain

	// Address repair
	addr := strings.TrimSpace(Utility.ToString(cfg["Address"]))
	if addr == "" || isLoopbackHost(addr) {
		if ip := resolveAdvertiseAddress(); ip != "" {
			cfg["Address"] = ip
			changed = true
		}
	}
	return changed
}

func isLoopbackHost(addr string) bool {
	h := addr
	if strings.Contains(addr, ":") {
		h, _, _ = strings.Cut(addr, ":")
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

func resolveAdvertiseAddress() string {
	explicit := strings.TrimSpace(os.Getenv("GLOBULAR_ADVERTISE_ADDRESS"))
	preferred := strings.TrimSpace(os.Getenv("GLOBULAR_PREFERRED_IFACE"))
	if ip, err := netutil.ResolveAdvertiseIP(preferred, explicit); err == nil && ip != nil {
		return ip.String()
	}
	return ""
}

func buildMinimalLocalConfig() map[string]interface{} {
	name, err := os.Hostname()
	if err != nil {
		name = "localhost"
	}
	addr := resolveAdvertiseAddress()
	return map[string]interface{}{
		"Name":              strings.TrimSpace(name),
		"Domain":            installerDefaultDomain(),
		"Address":           addr,
		"Protocol":          "https",
		"Peers":             []string{},
		"AlternateDomains":  []string{},
		"MutateHostsFile":   false,
		"MutateResolvConf":  false,
		"Services":          map[string]interface{}{},
		"EnablePeerUpserts": false,
	}
}

func installerDefaultDomain() string {
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_DOMAIN")); v != "" {
		return netutil.NormalizeDomain(v)
	}
	localConf := filepath.Join(GetRuntimeConfigDir(), "local.conf")
	if domain := readConfKey(localConf, "GLOBULAR_DOMAIN"); domain != "" {
		return netutil.NormalizeDomain(domain)
	}
	if domain := readConfKey(localConf, "DOMAIN"); domain != "" {
		return netutil.NormalizeDomain(domain)
	}
	return netutil.DefaultClusterDomain()
}

func readConfKey(path, key string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, "=") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if strings.TrimSpace(parts[0]) != key {
			continue
		}
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"'`)
		return value
	}
	return ""
}

func writeLocalConfig(cfgPath string, cfg map[string]interface{}) error {
	dir := filepath.Dir(cfgPath)
	tmpFile, err := os.CreateTemp(dir, ".config.json.tmp")
	if err != nil {
		return fmt.Errorf("ensure local config: tmp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("ensure local config: marshal: %w", err)
	}

	if _, err = tmpFile.Write(data); err != nil {
		return fmt.Errorf("ensure local config: write tmp: %w", err)
	}
	if err = tmpFile.Sync(); err != nil {
		return fmt.Errorf("ensure local config: sync tmp: %w", err)
	}
	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("ensure local config: close tmp: %w", err)
	}

	if err = os.Rename(tmpName, cfgPath); err != nil {
		return fmt.Errorf("ensure local config: rename: %w", err)
	}
	if err = os.Chmod(cfgPath, 0o644); err != nil {
		return fmt.Errorf("ensure local config: chmod: %w", err)
	}
	if err = chownGlobular(cfgPath); err != nil {
		return fmt.Errorf("ensure local config: chown: %w", err)
	}
	return nil
}

func backupConfigFile(cfgPath string) error {
	backup := fmt.Sprintf("%s.bak.%d", cfgPath, time.Now().UnixNano())
	return os.Rename(cfgPath, backup)
}

func chownGlobular(path string) error {
	u, err := user.Lookup("globular")
	if err != nil {
		return nil
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return nil
	}
	return os.Chown(path, uid, gid)
}

func ensureEtcdRuntimeDefaults(cfg map[string]interface{}, stateRoot string) bool {
	changed := false
	changed = setDefaultIfMissing(cfg, "EtcdEnabled", true) || changed
	changed = setDefaultIfMissing(cfg, "EtcdMode", "standalone") || changed

	name := Utility.ToString(cfg["EtcdName"])
	if name == "" {
		name = Utility.ToString(cfg["Name"])
		if name == "" {
			if h, err := os.Hostname(); err == nil {
				name = h
			}
		}
		if name == "" {
			name = "globular"
		}
	}
	changed = setDefaultIfMissing(cfg, "EtcdName", name) || changed

	changed = setDefaultIfMissing(cfg, "EtcdClientPort", "2379") || changed
	changed = setDefaultIfMissing(cfg, "EtcdPeerPort", "2380") || changed

	dataDir := filepath.Join(stateRoot, "etcd")
	changed = setDefaultIfMissing(cfg, "EtcdDataDir", dataDir) || changed

	configPath := filepath.Join(dataDir, "etcd.yml")
	changed = setDefaultIfMissing(cfg, "EtcdConfigPath", configPath) || changed

	return changed
}

func setDefaultIfMissing(cfg map[string]interface{}, key string, value interface{}) bool {
	if _, ok := cfg[key]; ok {
		return false
	}
	cfg[key] = value
	return true
}

// ============================================================================
// Methods discovery (.proto) — reads path from etcd-backed service docs
// ============================================================================
func ResolveService(idOrName string) (map[string]interface{}, error) {

	// exact id
	if s, err := GetServiceConfigurationById(idOrName); err == nil && s != nil {
		return s, nil
	}

	// best by name
	all, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}

	name := strings.ToLower(strings.TrimSpace(idOrName))
	var cands []map[string]interface{}
	for _, s := range all {
		if strings.ToLower(Utility.ToString(s["Name"])) == name {
			cands = append(cands, s)
		}
	}
	if len(cands) == 0 {
		return nil, fmt.Errorf("no services found with name %q", idOrName)
	}

	// score: prefer running, proc>0, recent UpdatedAt, local address, same domain
	var best map[string]interface{}
	var bestScore int64 = -1
	now := time.Now().Unix()
	confDom, _ := GetDomain()
	confDom = strings.ToLower(strings.TrimSpace(confDom))
	localAddr, _ := Utility.GetPrimaryIPAddress()
	localHost := localAddr
	if h, _, err := net.SplitHostPort(localAddr); err == nil {
		localHost = h
	}
	localHost = strings.ToLower(localHost)

	for _, s := range cands {
		score := int64(0)
		state := strings.ToLower(Utility.ToString(s["State"]))
		if state == "running" {
			score += 1000
		}
		if Utility.ToInt(s["Process"]) > 0 {
			score += 200
		}
		delta := now - int64(Utility.ToInt(s["UpdatedAt"]))
		if delta < 3600 {
			score += 100
		}
		if delta < 60 {
			score += 50
		}
		addr := strings.ToLower(strings.TrimSpace(Utility.ToString(s["Address"])))
		if addr == "127.0.0.1" || addr == "localhost" || addr == localHost {
			score += 50
		}
		if d := strings.ToLower(Utility.ToString(s["Domain"])); confDom != "" && d == confDom {
			score += 20
		}

		if score > bestScore {
			bestScore, best = score, s
		}
	}

	return best, nil
}

func GetServiceMethods(name string, PublisherID string, version string) ([]string, error) {
	methods := make([]string, 0)

	configs, err := GetServicesConfigurationsByName(name)
	if err != nil {
		return nil, err
	}

	var protoPath string
	for _, c := range configs {
		if Utility.ToString(c["PublisherID"]) == PublisherID && Utility.ToString(c["Version"]) == version {
			protoPath = Utility.ToString(c["Proto"])
			// Legacy fallback (not recommended): if Proto missing, try alongside binary
			if protoPath == "" {
				bin := Utility.ToString(c["Path"])
				if bin != "" {
					dir := filepath.Dir(bin)
					base := Utility.ToString(c["Name"])
					if base != "" {
						p := filepath.Join(dir, base+".proto")
						if Utility.Exists(p) {
							protoPath = p
						}
					}
				}
			}
			break
		}
	}
	if protoPath == "" {
		return nil, fmt.Errorf("no .proto path found for service %s version %s publisher %s", name, version, PublisherID)
	}

	f, err := os.Open(protoPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	p := proto.NewParser(f)
	def, _ := p.Parse()

	var pkg, svc string
	proto.Walk(def,
		proto.WithPackage(func(pk *proto.Package) { pkg = pk.Name }),
		proto.WithService(func(s *proto.Service) { svc = s.Name }),
		proto.WithRPC(func(r *proto.RPC) {
			methods = append(methods, "/"+pkg+"."+svc+"/"+r.Name)
		}),
	)
	return methods, nil
}
