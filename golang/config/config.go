package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/emicklei/proto"
	Utility "github.com/globulario/utility"
)

// ============================================================================
// Globals
// ============================================================================

var (
	// in-memory cache of local config when lazy=true
	config_ map[string]interface{}
)

// ============================================================================
// Addressing / Identity
// ============================================================================

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

// GetLocalServerCerificateKeyPath returns the path to the local server PEM key,
// or an empty string if missing.
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

// GetLocalClientCerificateKeyPath returns the path to the local client PEM key,
// or an empty string if missing.
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

// GetLocalCertificate returns the certificate filename from local config (may be empty).
func GetLocalCertificate() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		if s, _ := cfg["Certificate"].(string); s != "" {
			return s
		}
	}
	return ""
}

// GetLocalCertificateAuthorityBundle returns the CA bundle filename from local config (may be empty).
func GetLocalCertificateAuthorityBundle() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		if s, _ := cfg["CertificateAuthorityBundle"].(string); s != "" {
			return s
		}
	}
	return ""
}

// GetRootDir returns the directory of the running executable as "root".
func GetRootDir() string {
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return strings.ReplaceAll(dir, "\\", "/")
}

// GetGlobularExecPath returns the configured path to the Globular executable, or "".
func GetGlobularExecPath() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		if p, _ := cfg["Path"].(string); p != "" {
			return p
		}
	}
	return ""
}

// GetPublicDirs returns the aggregated list of public directories from all file services.
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

// GetServicesRoot forces services to be read from a configured root directory, if set.
func GetServicesRoot() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		if s, _ := cfg["ServicesRoot"].(string); s != "" {
			return s
		}
	}
	return ""
}

// GetServicesConfigDir returns a deterministic directory to *represent* service configs.
// We no longer depend on per-service config.json files; this is kept for compatibility
// where a path is needed (e.g., logs/UI), and for packaging layouts.
func GetServicesConfigDir() string {
	if root := GetServicesRoot(); root != "" {
		return root
	}
	if runtime.GOOS == "windows" {
		var programFiles string
		if runtime.GOARCH == "386" {
			programFiles, _ = Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
		} else {
			programFiles, _ = Utility.GetEnvironmentVariable("PROGRAMFILES")
		}
		if programFiles != "" {
			return strings.ReplaceAll(programFiles, "\\", "/") + "/globular/config/services"
		}
		return "C:/Program Files/globular/config/services"
	}
	// linux / freebsd / darwin
	return "/etc/globular/config/services"
}

// GetConfigDir returns the OS-specific directory where Globular config resides.
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

// GetDataDir returns the OS-specific data directory for Globular.
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

// GetWebRootDir returns the OS-specific webroot directory for Globular.
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

// GetToken reads a token for the given MAC (issuer) from the standard tokens directory.
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
// Service dependency ordering & retrieval
// ============================================================================

func OrderDependencys(services []map[string]interface{}) ([]string, error) {
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
	order, err := OrderDependencys(svcs)
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
// Remote config (HTTP) – unchanged (used for other hosts)
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
// Local config (now etcd-first; file fallback)
// ============================================================================

// GetLocalConfig returns the local server configuration. When lazy=true, it caches
// the map and does NOT expand the Services list. When lazy=false, it merges the
// etcd service desired+runtime docs into cfg["Services"] for convenience.
func GetLocalConfig(lazy bool) (map[string]interface{}, error) {
	if lazy && config_ != nil {
		return config_, nil
	}

	// 1) Try etcd system config
	cfg := map[string]interface{}{}
	if c, err := etcdClient(); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if res, err := c.Get(ctx, "/globular/system/config"); err == nil && len(res.Kvs) == 1 {
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

	// 2) Fallback to file (bootstrap/compat)
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
// Methods discovery (.proto) — no more service config.json
// ============================================================================

// GetServiceMethods parses the .proto for the given service (PublisherID+version)
// and returns the fully qualified gRPC method paths.
// Now reads the .proto path from the etcd service document ("Proto" field).
func GetServiceMethods(name string, PublisherID string, version string) ([]string, error) {
	methods := make([]string, 0)

	configs, err := GetServicesConfigurationsByName(name)
	if err != nil {
		return nil, err
	}

	var protoPath string
	for _, c := range configs {
		if Utility.ToString(c["PublisherID"]) == PublisherID && Utility.ToString(c["Version"]) == version {
			// Prefer explicit Proto path in etcd
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

	stack := make([]interface{}, 0)

	proto.Walk(def,
		proto.WithPackage(func(pk *proto.Package) { stack = append(stack, pk) }),
		proto.WithService(func(s *proto.Service) { stack = append(stack, s) }),
		proto.WithRPC(func(r *proto.RPC) { stack = append(stack, r) }),
	)

	var pkg, svc string
	for len(stack) > 0 {
		var x interface{}
		x, stack = stack[0], stack[1:]
		switch v := x.(type) {
		case *proto.Package:
			pkg = v.Name
		case *proto.Service:
			svc = v.Name
		case *proto.RPC:
			methods = append(methods, "/"+pkg+"."+svc+"/"+v.Name)
		}
	}
	return methods, nil
}
