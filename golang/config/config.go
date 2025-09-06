// ==============================================
// config.go (system config + helpers; service config lives in etcd_backend.go)
// ==============================================
package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emicklei/proto"
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
	if gc, err := GetLocalConfig(false); err == nil && gc != nil {
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

	localConfig, err := GetLocalConfig(true)
	if err != nil {
		return "", err
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
		return "localhost", nil
	}
	return "", errors.New("no local configuration found")
}

// TLS helper paths
func GetLocalServerCerificateKeyPath() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		name, _ := cfg["Name"].(string)
		domain, _ := cfg["Domain"].(string)
		if name != "" && domain != "" {
			p := GetConfigDir() + "/tls/" + name + "." + domain + "/server.pem"
			if Utility.Exists(p) {
				return p
			}
		}
	}
	return ""
}

func GetLocalClientCerificateKeyPath() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		name, _ := cfg["Name"].(string)
		domain, _ := cfg["Domain"].(string)
		if name != "" && domain != "" {
			p := GetConfigDir() + "/tls/" + name + "." + domain + "/client.pem"
			if Utility.Exists(p) {
				return p
			}
		}
	}
	return ""
}

func GetLocalCertificate() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		if s, _ := cfg["Certificate"].(string); s != "" {
			return s
		}
	}
	return ""
}

func GetLocalCertificateAuthorityBundle() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		if s, _ := cfg["CertificateAuthorityBundle"].(string); s != "" {
			return s
		}
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

func GetConfigDir() string {
	if runtime.GOOS == "windows" {
		var programFilePath string
		if runtime.GOARCH == "386" {
			programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
		} else {
			programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES")
		}
		return strings.ReplaceAll(programFilePath, "\\", "/") + "/globular/config"
	}
	// linux / freebsd / darwin
	return "/etc/globular/config"
}

func GetDataDir() string {
	if runtime.GOOS == "windows" {
		var programFilePath string
		if runtime.GOARCH == "386" {
			programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
		} else {
			programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES")
		}
		return strings.ReplaceAll(programFilePath, "\\", "/") + "/globular/data"
	}
	// linux / freebsd / darwin
	return "/var/globular/data"
}

func GetWebRootDir() string {
	if runtime.GOOS == "windows" {
		var programFilePath string
		if runtime.GOARCH == "386" {
			programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
		} else {
			programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES")
		}
		return strings.ReplaceAll(programFilePath, "\\", "/") + "/globular/webroot"
	}
	// linux / freebsd / darwin
	return "/var/globular/webroot"
}

func GetToken(mac string) (string, error) {
	path := GetConfigDir() + "/tokens/" + strings.ReplaceAll(mac, ":", "_") + "_token"
	if !Utility.Exists(path) {
		return "", fmt.Errorf("no token found for domain %s at path %s", mac, path)
	}
	token, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read token at %s: %w", path, err)
	}
	return string(token), nil
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
// Remote config (HTTP) – used for peers
// ============================================================================

func GetRemoteServiceConfig(address string, port int, id string) (map[string]interface{}, error) {
	if address == "" {
		return nil, errors.New("fail to get remote service Config: no address was given")
	}
	if id == "" {
		return nil, errors.New("no service ID was given")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 5 * time.Second}

	if port == 0 {
		port = 80
	}

	// Try HTTP first
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s:%d/config", address, port), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		// Try HTTPS
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://%s:%d/config", address, port), nil)
		if err != nil {
			return nil, err
		}
		resp, err = client.Do(req)
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}

	// Retry on HTTP→HTTPS mismatch message
	if strings.Contains(string(body), "Client sent an HTTP request to an HTTPS server.") {
		if port == 0 {
			port = 443
		}
		req, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://%s:%d/config", address, port), nil)
		if err != nil {
			return nil, err
		}
		resp, err = client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err = io.ReadAll(resp.Body)
		if err != nil && err.Error() != "EOF" {
			return nil, err
		}
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, err
	}

	if id != "" {
		// find by Name or Id in Services map
		if m, ok := cfg["Services"].(map[string]interface{}); ok {
			for _, raw := range m {
				if s, ok := raw.(map[string]interface{}); ok {
					n, _ := s["Name"].(string)
					i, _ := s["Id"].(string)
					if n == id || i == id {
						return s, nil
					}
				}
			}
		}
	}
	return cfg, nil
}

func GetRemoteConfig(address string, port int) (map[string]interface{}, error) {
	if address == "" {
		return nil, errors.New("fail to get remote config no address was given")
	}

	if port == 0 {
		port = 80
	}

	resp, err := http.Get("http://" + address + ":" + Utility.ToString(port) + "/config")
	if err != nil {
		resp, err = http.Get("https://" + address + ":" + Utility.ToString(port) + "/config")
		if err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil && err.Error() != "EOF" {
		return nil, err
	}

	if strings.Contains(string(body), "Client sent an HTTP request to an HTTPS server.") {
		if port == 0 {
			port = 443
		}
		resp, err = http.Get("https://" + address + ":" + Utility.ToString(port) + "/config")
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		body, err = io.ReadAll(resp.Body)
		if err != nil && err.Error() != "EOF" {
			return nil, err
		}
	}

	var cfg map[string]interface{}
	if err := json.Unmarshal(body, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ============================================================================
// Local system config: etcd-first; file fallback is bootstrap ONLY
// ============================================================================

func GetLocalConfig(lazy bool) (map[string]interface{}, error) {
	if lazy && config_ != nil {
		return config_, nil
	}

	// 1) Try etcd system config
	cfg := map[string]interface{}{}
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

	// 2) Bootstrap fallback: local file for the *system* config (not services)
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
	if h, _, err := net.SplitHostPort(localAddr); err == nil { localHost = h }
	localHost = strings.ToLower(localHost)

	for _, s := range cands {
		score := int64(0)
		state := strings.ToLower(Utility.ToString(s["State"]))
		if state == "running" { score += 1000 }
		if Utility.ToInt(s["Process"]) > 0 { score += 200 }
		delta := now - int64(Utility.ToInt(s["UpdatedAt"]))
		if delta < 3600 { score += 100 }
		if delta < 60   { score += 50 }
		addr := strings.ToLower(strings.TrimSpace(Utility.ToString(s["Address"])))
		if addr == "127.0.0.1" || addr == "localhost" || addr == localHost { score += 50 }
		if d := strings.ToLower(Utility.ToString(s["Domain"])); confDom != "" && d == confDom { score += 20 }

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
