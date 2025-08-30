package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/emicklei/proto"
	"github.com/fsnotify/fsnotify"
	Utility "github.com/globulario/utility"
)

// ============================================================================
// Globals
// ============================================================================

var (
	// in-memory cache of local config when lazy=true
	config_ map[string]interface{}

	// service-config access channels (serialized access)
	saveServiceConfigChan               = make(chan map[string]interface{})
	getServicesConfigChan               = make(chan map[string]interface{})
	getServiceConfigurationByIdChan     = make(chan map[string]interface{})
	getServicesConfigurationsByNameChan = make(chan map[string]interface{})
	exit                                = make(chan bool)

	// background loop state
	isInit bool
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
		p := GetConfigDir() + "/tls/" + cfg["Name"].(string) + "." + cfg["Domain"].(string) + "/server.pem"
		if Utility.Exists(p) {
			return p
		}
	}
	return ""
}

// GetLocalClientCerificateKeyPath returns the path to the local client PEM key,
// or an empty string if missing.
func GetLocalClientCerificateKeyPath() string {
	if cfg, err := GetLocalConfig(true); err == nil {
		p := GetConfigDir() + "/tls/" + cfg["Name"].(string) + "." + cfg["Domain"].(string) + "/client.pem"
		if Utility.Exists(p) {
			return p
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

// GetServicesDir tries to resolve the services directory from various common locations.
func GetServicesDir() string {
	if dir := GetServicesRoot(); dir != "" {
		return dir
	}

	root := GetRootDir()

	if Utility.Exists(root + "/services") {
		return root + "/services"
	}
	parent := root
	if i := strings.LastIndex(root, "/"); i > 0 {
		parent = root[:i]
	}
	if Utility.Exists(parent + "/services") {
		return parent + "/services"
	}
	if strings.Contains(root, "/services/") {
		return root[:strings.LastIndex(root, "/services/")] + "/services"
	}

	var programFilePath string
	if runtime.GOOS == "windows" {
		if runtime.GOARCH == "386" {
			programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
		} else {
			programFilePath, _ = Utility.GetEnvironmentVariable("PROGRAMFILES")
		}
		programFilePath += "/Globular"
	} else {
		programFilePath = "/usr/local/share/globular"
	}

	if Utility.Exists(programFilePath + "/services") {
		return programFilePath + "/services"
	}
	return ""
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

// hasServiceConfigs returns true if at least one "config.json" exists somewhere under dir.
func hasServiceConfigs(dir string) bool {
	if dir == "" || !Utility.Exists(dir) {
		return false
	}
	files, err := Utility.FindFileByName(dir, "config.json")
	return err == nil && len(files) > 0
}

// GetServicesConfigDir returns the directory containing service configs (config.json).
// Priority:
//   1) If GetServicesRoot() is set and contains any config.json (recursively), return it.
//   2) If running "Globular*" binary:
//        - parent-of-exec + "/services" if it contains configs
//        - fallback to GetConfigDir() + "/services" if it contains configs
//   3) Otherwise (running a service or other binary):
//        - GetServicesRoot() if it contains configs
//        - GetConfigDir() + "/services" if it contains configs
//        - GetServicesDir() (auto-detected) if it contains configs
//   4) OS defaults (if nothing above found):
//        - Linux/FreeBSD/Darwin: "/etc/globular/config/services"
//        - Windows: "<ProgramFiles>/globular/config/services"
//   5) Dev fallback using runtime.Caller to infer repo layout.
func GetServicesConfigDir() string {
	// 1) Explicit override via ServicesRoot (only if it actually has configs)
	if root := GetServicesRoot(); hasServiceConfigs(root) {
		return root
	}

	// Gather executable context
	execDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	execDir = strings.ReplaceAll(execDir, "\\", "/")
	execName := filepath.Base(os.Args[0])

	// 2) Running the Globular launcher?
	if strings.HasPrefix(execName, "Globular") {
		// Parent-of-exec "/services"
		if idx := strings.LastIndex(execDir, "/"); idx > 0 {
			parentServices := execDir[:idx] + "/services"
			if hasServiceConfigs(parentServices) {
				return parentServices
			}
		}
		// ConfigDir "/services"
		cfgServices := GetConfigDir() + "/services"
		if hasServiceConfigs(cfgServices) {
			return cfgServices
		}
	}

	// 3) Not the Globular launcher (likely a service binary)

	// Try ServicesRoot again (in case it was set but empty earlier)
	if root := GetServicesRoot(); hasServiceConfigs(root) {
		return root
	}

	// ConfigDir "/services"
	if cfg := GetConfigDir(); cfg != "" {
		cfgServices := cfg + "/services"
		if hasServiceConfigs(cfgServices) {
			return cfgServices
		}
	}

	// Auto-detected services dir
	if d := GetServicesDir(); hasServiceConfigs(d) {
		return d
	}

	// 4) OS-specific defaults
	if runtime.GOOS == "windows" {
		var programFiles string
		if runtime.GOARCH == "386" {
			programFiles, _ = Utility.GetEnvironmentVariable("PROGRAMFILES(X86)")
		} else {
			programFiles, _ = Utility.GetEnvironmentVariable("PROGRAMFILES")
		}
		if programFiles != "" {
			winServices := strings.ReplaceAll(programFiles, "\\", "/") + "/globular/config/services"
			if hasServiceConfigs(winServices) {
				return winServices
			}
		}
	} else {
		etcServices := "/etc/globular/config/services"
		if hasServiceConfigs(etcServices) {
			return etcServices
		}
	}

	// 5) Dev environment fallback: infer path from this file location
	if _, filename, _, ok := runtime.Caller(0); ok {
		filename = strings.ReplaceAll(filename, "\\", "/")
		const marker = "/services/golang/config/"
		if strings.Contains(filename, marker) {
			devRoot := filename[:strings.Index(filename, "/config/")]
			if hasServiceConfigs(devRoot) {
				return devRoot
			}
		}
	}

	// Nothing found
	return ""
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
// Small helpers for manipulating []map[string]any (kept for compatibility)
// ============================================================================

func insertObject(array []map[string]interface{}, value map[string]interface{}, index int) []map[string]interface{} {
	return append(array[:index], append([]map[string]interface{}{value}, array[index:]...)...)
}
func removeObject(array []map[string]interface{}, index int) []map[string]interface{} {
	return append(array[:index], array[index+1:]...)
}
func moveObject(array []map[string]interface{}, srcIndex int, dstIndex int) []map[string]interface{} {
	value := array[srcIndex]
	return insertObject(removeObject(array, srcIndex), value, dstIndex)
}
func getObjectIndex(value, name string, objects []map[string]interface{}) int {
	for i := range objects {
		if objects[i][name].(string) == value {
			return i
		}
	}
	return -1
}

// ============================================================================
// Service dependency ordering & retrieval
// ============================================================================

// OrderDependencys topologically sorts service names so that dependencies appear
// before dependent services. It expects each service to have "Name" and "Dependencies".
func OrderDependencys(services []map[string]interface{}) ([]string, error) {
	serviceMap := make(map[string]map[string]interface{})
	for _, s := range services {
		serviceMap[s["Name"].(string)] = s
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

// GetOrderedServicesConfigurations returns service configs ordered by dependencies.
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

// GetRemoteServiceConfig fetches remote /config and returns the service config by ID or Name.
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

// GetRemoteConfig fetches remote /config over HTTP(S).
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

// GetLocalConfig reads and returns the local server configuration from disk.
// If lazy=true, it returns a cached copy (and does not load service configs).
func GetLocalConfig(lazy bool) (map[string]interface{}, error) {
	if lazy && config_ != nil {
		return config_, nil
	}

	cfgPath := GetConfigDir() + "/config.json"
	if !Utility.Exists(cfgPath) {
		return nil, fmt.Errorf("no local Globular configuration found at path %s", cfgPath)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, err
	}
	cfg := make(map[string]interface{})
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if lazy {
		config_ = cfg
		return cfg, nil
	}

	// Expand services (full mode)
	cfg["Services"] = make(map[string]interface{})
	services, err := GetServicesConfigurations()
	if err != nil {
		return nil, err
	}
	for _, s := range services {
		cfg["Services"].(map[string]interface{})[s["Id"].(string)] = s
	}

	if name, _ := cfg["Name"].(string); name == "" {
		if n, err := GetName(); err == nil {
			cfg["Name"] = n
		}
	}
	return cfg, nil
}

// GetServiceMethods parses the .proto for the given service (PublisherID+version)
// and returns the fully qualified gRPC method paths.
func GetServiceMethods(name string, PublisherID string, version string) ([]string, error) {
	methods := make([]string, 0)

	configs, err := GetServicesConfigurationsByName(name)
	if err != nil {
		return nil, err
	}

	var path string
	for _, c := range configs {
		if c["PublisherID"] == PublisherID && c["Version"] == version {
			path = c["ConfigPath"].(string)
			break
		}
	}
	if path == "" {
		return nil, fmt.Errorf("no service found with name %s version %s and publisher id %s", name, version, PublisherID)
	}

	f, err := os.Open(path)
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

// ============================================================================
// Synchronized access to service configurations (background loop & helpers)
// ============================================================================

// initServiceConfiguration loads, normalizes, and enriches a service config.
func initServiceConfiguration(path, serviceDir string) (map[string]interface{}, error) {
	path = strings.ReplaceAll(path, "\\", "/")
	for isLocked(path) {
		time.Sleep(50 * time.Millisecond)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty configuration found at path %s", path)
	}

	s := make(map[string]interface{})
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}

	if info, err := os.Stat(path); err == nil {
		s["ModTime"] = info.ModTime().Unix()
	}

	localConfig, err := GetLocalConfig(true)
	if err != nil {
		return nil, err
	}

	if s["Protocol"] == nil || s["Name"] == nil {
		return s, nil
	}

	// Ensure ID
	if s["Id"] == nil {
		s["Id"] = Utility.RandomUUID()
	}

	// Fix Path if missing
	if sp, ok := s["Path"].(string); ok && sp != "" && !Utility.Exists(sp) {
		execname := filepath.Base(sp)
		if files, err := Utility.FindFileByName(serviceDir, execname); err == nil && len(files) > 0 {
			s["Path"] = files[0]
		}
	}

	// Keep configuration path
	s["ConfigPath"] = path

	// Set Root for services
	if s["Root"] != nil {
		if s["Name"] == "file.FileService" {
			s["Root"] = GetDataDir() + "/files"
		} else {
			s["Root"] = GetDataDir()
		}
	}

	// Resolve .proto path if missing
	if protoPath, ok := s["Proto"].(string); ok && !Utility.Exists(protoPath) {
		execPath := s["Path"].(string)
		execName := filepath.Base(execPath)

		protoName := execName
		if i := strings.Index(protoName, "."); i != -1 {
			protoName = protoName[:i]
		}
		if strings.Contains(protoName, "_server") {
			protoName = protoName[:strings.LastIndex(protoName, "_server")]
		}
		protoName += ".proto"

		base := execPath[:strings.Index(execPath, "/services/")] + "/services"
		if Utility.Exists(base) {
			if files, err := Utility.FindFileByName(base, protoName); err == nil && len(files) > 0 {
				s["Proto"] = files[0]
			} else {
				// try service name
				protoName = s["Name"].(string) + ".proto"
				if files, err := Utility.FindFileByName(base, protoName); err == nil && len(files) > 0 {
					s["Proto"] = files[0]
				}
			}
		}
	}

	// TLS settings inherited from globule when protocol is https
	if cert, _ := localConfig["Certificate"].(string); cert != "" && localConfig["Protocol"] == "https" {
		s["TLS"] = true
		name := localConfig["Name"].(string)
		domain := localConfig["Domain"].(string)
		s["KeyFile"] = GetConfigDir() + "/tls/" + name + "." + domain + "/server.pem"
		s["CertFile"] = GetConfigDir() + "/tls/" + name + "." + domain + "/server.crt"
		s["CertAuthorityTrust"] = GetConfigDir() + "/tls/" + name + "." + domain + "/ca.crt"
		if s["CertificateAuthorityBundle"] != nil {
			s["CertificateAuthorityBundle"] = localConfig["CertificateAuthorityBundle"]
		}
		if s["Certificate"] != nil {
			s["Certificate"] = localConfig["Certificate"]
		}
	} else {
		s["TLS"] = false
		s["KeyFile"] = ""
		s["CertFile"] = ""
		s["CertAuthorityTrust"] = ""
	}

	// Misc enrich
	if d, _ := GetDomain(); d != "" {
		s["Domain"] = d
	}
	if a, _ := GetAddress(); a != "" {
		s["Address"] = a
	}
	s["Mac"] = localConfig["Mac"]

	// Session timeout
	if s["SessionTimeout"] != nil {
		s["SessionTimeout"] = localConfig["SessionTimeout"]
	}

	return s, nil
}

// isLocked checks for a sibling .lock file next to the json path.
func isLocked(path string) bool {
	lock := strings.Replace(path, "json", "lock", -1)
	return Utility.Exists(lock)
}

func lock(path string) bool {
	lock := strings.Replace(path, "json", "lock", -1)
	return Utility.WriteStringToFile(lock, "") == nil
}

func unlock(path string) bool {
	lock := strings.Replace(path, "json", "lock", -1)
	for Utility.Exists(lock) {
		time.Sleep(10 * time.Millisecond)
		_ = os.Remove(lock)
	}
	return true
}

func removeAllLocks() {
	if locks, err := Utility.FindFileByName(GetServicesConfigDir(), "config.lock"); err == nil {
		for _, l := range locks {
			_ = os.Remove(l)
		}
	}
	if locks, err := Utility.FindFileByName(GetConfigDir(), "config.lock"); err == nil {
		for _, l := range locks {
			_ = os.Remove(l)
		}
	}
}

// initConfig initializes in-memory service configuration cache and starts the
// background fsnotify watcher for config reloads.
func initConfig() error {
	if isInit {
		return nil
	}

	// Cleanup any leftover locks when running under Globular
	if strings.HasPrefix(filepath.Base(os.Args[0]), "Globular") {
		removeAllLocks()
	}

	serviceConfigDir := GetServicesConfigDir()
	files, err := Utility.FindFileByName(serviceConfigDir, "config.json")
	services := make([]map[string]interface{}, 0)

	if err != nil || len(files) == 0 {
		// mac/darwin app bundle install migration helper
		if strings.HasPrefix(filepath.Base(os.Args[0]), "Globular") && runtime.GOOS == "darwin" {
			dir := GetRootDir()
			if Utility.Exists(dir + "/etc/globular/config/services") {
				if !Utility.Exists("/etc/globular/config/services") {
					_ = Utility.Move(dir+"/etc/globular/config/services", "/etc/globular/config")
				}
				_ = os.RemoveAll(dir + "/etc")

				if entries, err := Utility.ReadDir(dir + "/bin"); err == nil {
					for _, e := range entries {
						if !e.IsDir() {
							src := dir + "/bin/" + e.Name()
							_ = Utility.Move(src, "/usr/local/bin/")
							_ = os.Chmod(src, 0o755)
						}
					}
				}
				if libs, err := Utility.ReadDir(dir + "/lib"); err == nil {
					for _, e := range libs {
						if !e.IsDir() {
							_ = Utility.Move(dir+"/lib/"+e.Name(), "/usr/local/lib")
						}
					}
				}
				_ = Utility.CreateDirIfNotExist("/var/globular/applications")
				if apps, err := Utility.ReadDir(dir + "/var/globular/applications"); err == nil {
					for _, a := range apps {
						if !a.IsDir() {
							_ = Utility.Move(dir+"/var/globular/applications/"+a.Name(), "/var/globular/applications")
						}
					}
				}
				files, _ = Utility.FindFileByName(GetServicesConfigDir(), "config.json")
			}
		}
	}

	if len(files) == 0 {
		return fmt.Errorf("no services configuration was found at path %s", serviceConfigDir)
	}

	serviceDir := GetServicesDir()
	execname := filepath.Base(os.Args[0])

	for _, path := range files {
		s, err := initServiceConfiguration(path, serviceDir)
		if err != nil {
			return fmt.Errorf("fail to initialize service config %s: %w", path, err)
		}
		s["ConfigPath"] = strings.ReplaceAll(path, "\\", "/")
		services = append(services, s)

		if strings.HasPrefix(execname, "Globular") {
			if sp, _ := s["Path"].(string); sp != "" && !Utility.Exists(sp) {
				serviceName := filepath.Base(sp)
				if found, err := Utility.FindFileByName(serviceDir, serviceName); err == nil && len(found) > 0 {
					s["Path"] = found[0]
					if jsonStr, err := Utility.ToJson(s); err == nil {
						_ = os.WriteFile(path, []byte(jsonStr), 0o644)
					}
				}
			}
		}
	}

	isInit = true
	go accesServiceConfigurationFile(services)

	return nil
}

func setServiceConfiguration(index int, services []map[string]interface{}) {
	s := services[index]
	path := strings.ReplaceAll(s["ConfigPath"].(string), "\\", "/")
	if s["ModTime"] == nil {
		s["ModTime"] = int64(0)
	}
	if Utility.Exists(path) {
		if info, err := os.Stat(path); err == nil && Utility.ToInt(s["ModTime"]) < Utility.ToInt(info.ModTime().Unix()) {
			if s2, err := initServiceConfiguration(path, GetServicesDir()); err == nil {
				s2["ModTime"] = info.ModTime().Unix()
				services[index] = s2
			}
		}
	}
}

// accesServiceConfigurationFile is the single goroutine that watches config
// changes and services read/write requests via channels.
func accesServiceConfigurationFile(services []map[string]interface{}) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("fsnotify watcher error:", err)
		return
	}
	defer watcher.Close()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	defer close(ch)

	go func() {
		for evt := range watcher.Events {
			if evt.Op == fsnotify.Write {
				config_ = nil // bust cache
			}
		}
	}()

	configPath := GetConfigDir() + "/config.json"
	if err := watcher.Add(configPath); err != nil {
		fmt.Println("watcher add error:", err)
		return
	}

	for {
		select {
		case <-exit:
			return

		case infos := <-saveServiceConfigChan:
			s := infos["service_config"].(map[string]interface{})
			path := s["ConfigPath"].(string)
			ret := infos["return"].(chan error)

			jsonStr, err := Utility.ToJson(s)
			if err != nil {
				ret <- err
				continue
			}
			if jsonStr == "" {
				ret <- errors.New("no configuration to save")
				continue
			}

			// detect change vs in-memory copy
			index := -1
			hasChange := true
			for i := range services {
				if services[i]["Id"] == s["Id"] {
					index = i
					break
				}
			}
			if index == -1 {
				index = len(services)
				services = append(services, s)
			} else {
				prevJSON, _ := Utility.ToJson(services[index])
				hasChange = prevJSON != jsonStr
			}

			if hasChange {
				for isLocked(path) {
					time.Sleep(50 * time.Millisecond)
				}
				lock(path)
				err = os.WriteFile(path, []byte(jsonStr), 0o644)
				unlock(path)
				if err != nil {
					ret <- err
					continue
				}
				services[index]["ModTime"] = int64(0)
				setServiceConfiguration(index, services)
			}
			ret <- nil

		case infos := <-getServicesConfigChan:
			copyList := make([]map[string]interface{}, 0, len(services))
			for i := range services {
				setServiceConfiguration(i, services)
				data, _ := Utility.ToJson(services[i])
				m := make(map[string]interface{})
				_ = json.Unmarshal([]byte(data), &m)
				copyList = append(copyList, m)
			}
			infos["return"].(chan map[string]interface{}) <- map[string]interface{}{"services": copyList}

		case infos := <-getServiceConfigurationByIdChan:
			id := infos["id"].(string)
			var s map[string]interface{}
			for i := range services {
				if services[i]["Id"].(string) == id ||
					services[i]["Name"].(string) == id ||
					strings.ReplaceAll(services[i]["ConfigPath"].(string), "\\", "/") == id {
					setServiceConfiguration(i, services)
					data, _ := Utility.ToJson(services[i])
					_ = json.Unmarshal([]byte(data), &s)
					break
				}
			}
			var err error
			if s == nil {
				err = fmt.Errorf("no service found with id %s", id)
			}
			infos["return"].(chan map[string]interface{}) <- map[string]interface{}{"service": s, "error": err}

		case infos := <-getServicesConfigurationsByNameChan:
			name := infos["name"].(string)
			var out []map[string]interface{}
			for i := range services {
				if services[i]["Name"] == name {
					setServiceConfiguration(i, services)
					data, _ := Utility.ToJson(services[i])
					m := make(map[string]interface{})
					_ = json.Unmarshal([]byte(data), &m)
					out = append(out, m)
				}
			}
			var err error
			if len(out) == 0 {
				err = fmt.Errorf("no services found with name %s", name)
			}
			infos["return"].(chan map[string]interface{}) <- map[string]interface{}{"services": out, "error": err}
		}
	}
}

// Exit stops the background configuration processing goroutine.
func Exit() {
	exit <- true
}

// GetServicesConfigurations returns all installed service configurations.
func GetServicesConfigurations() ([]map[string]interface{}, error) {
	if err := initConfig(); err != nil {
		return nil, err
	}

	infos := map[string]interface{}{"return": make(chan map[string]interface{})}
	getServicesConfigChan <- infos
	results := <-infos["return"].(chan map[string]interface{})
	if results["error"] != nil {
		return nil, results["error"].(error)
	}
	return results["services"].([]map[string]interface{}), nil
}

// SaveServiceConfiguration persists a service configuration to its ConfigPath.
func SaveServiceConfiguration(s map[string]interface{}) error {
	configPath, _ := s["ConfigPath"].(string)
	if configPath == "" {
		return errors.New("no configuration path was given")
	}

	// If new file: write it directly
	if !Utility.Exists(configPath) {
		jsonStr, err := Utility.ToJson(s)
		if err != nil {
			return err
		}
		return os.WriteFile(configPath, []byte(jsonStr), 0o644)
	}

	if err := initConfig(); err != nil {
		return err
	}

	data, _ := Utility.ToJson(s)
	copyMap := make(map[string]interface{})
	_ = json.Unmarshal([]byte(data), &copyMap)

	infos := map[string]interface{}{
		"service_config": copyMap,
		"return":         make(chan error),
	}
	saveServiceConfigChan <- infos
	return <-infos["return"].(chan error)
}

// GetServicesConfigurationsByName returns all service configurations with a given "Name".
func GetServicesConfigurationsByName(name string) ([]map[string]interface{}, error) {
	if err := initConfig(); err != nil {
		return nil, err
	}

	infos := map[string]interface{}{
		"name":   name,
		"return": make(chan map[string]interface{}),
	}
	getServicesConfigurationsByNameChan <- infos
	results := <-infos["return"].(chan map[string]interface{})
	if results["error"] != nil {
		return nil, results["error"].(error)
	}
	return results["services"].([]map[string]interface{}), nil
}

// GetServiceConfigurationById returns a service configuration by ID, by Name,
// or by ConfigPath.
func GetServiceConfigurationById(id string) (map[string]interface{}, error) {
	if err := initConfig(); err != nil {
		return nil, err
	}

	infos := map[string]interface{}{
		"id":     id,
		"return": make(chan map[string]interface{}),
	}
	getServiceConfigurationByIdChan <- infos
	results := <-infos["return"].(chan map[string]interface{})
	if results["error"] != nil {
		return nil, results["error"].(error)
	}
	return results["service"].(map[string]interface{}), nil
}
