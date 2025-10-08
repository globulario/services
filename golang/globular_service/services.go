// Package globular_service provides helpers to initialize, run, and package
// Globular gRPC services with consistent config, TLS, keepalive and health setup.
package globular_service

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"github.com/kardianos/osext"
	"google.golang.org/grpc/keepalive"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// Service describes the minimal contract a Globular service must implement.
// NOTE: Public interface preserved verbatim (even fields that are now deprecated
// due to etcd-only configuration).
type Service interface {
	/** Getter/Setter **/

	// A unique instance identifier for the running service.
	GetId() string
	SetId(string)

	// The gRPC service name.
	GetName() string
	SetName(string)

	// Host machine MAC address.
	GetMac() string
	SetMac(string)

	// HTTP(S) address (host:port) where config can be fetched (informational).
	GetAddress() string
	SetAddress(string)

	// Human-readable description.
	GetDescription() string
	SetDescription(string)

	// Platform string (GOOS_GOARCH).
	SetPlatform(string)
	GetPlatform() string

	// Search keywords.
	GetKeywords() []string
	SetKeywords([]string)

	// Absolute executable path.
	GetPath() string
	SetPath(string)

	// Current service state (starting/running/stopped/failed).
	GetState() string
	SetState(string)

	// Deprecated: file-based config path (unused in etcd mode, kept for compat).
	GetConfigurationPath() string
	SetConfigurationPath(string)

	// Last error encountered (for status).
	GetLastError() string
	SetLastError(string)

	// Executable modification time (unix seconds).
	SetModTime(int64)
	GetModTime() int64

	// .proto file absolute path.
	GetProto() string
	SetProto(string)

	// gRPC port.
	GetPort() int
	SetPort(int)

	// Reverse proxy port for gRPC-Web.
	GetProxy() int
	SetProxy(int)

	// Main process PID.
	GetProcess() int
	SetProcess(int)

	// Proxy process PID.
	GetProxyProcess() int
	SetProxyProcess(int)

	// One of: http/https (transport to reach the service externally).
	GetProtocol() string
	SetProtocol(string)

	// Discovery endpoints.
	GetDiscoveries() []string
	SetDiscoveries([]string)

	// Repository endpoints.
	GetRepositories() []string
	SetRepositories([]string)

	// CORS: allow any origin?
	GetAllowAllOrigins() bool
	SetAllowAllOrigins(bool)

	// CORS: comma-separated list of allowed origins if not allowing all.
	GetAllowedOrigins() string
	SetAllowedOrigins(string)

	// Domain of the service.
	GetDomain() string
	SetDomain(string)

	// Binary checksum used for update tracking.
	GetChecksum() string
	SetChecksum(string)

	// TLS controls.
	GetTls() bool
	SetTls(bool)
	GetCertAuthorityTrust() string
	SetCertAuthorityTrust(string)
	GetCertFile() string
	SetCertFile(string)
	GetKeyFile() string
	SetKeyFile(string)

	// Service version (semver or similar).
	GetVersion() string
	SetVersion(string)

	// Publisher identifier.
	GetPublisherID() string
	SetPublisherID(string)

	// Auto-update flag.
	GetKeepUpToDate() bool
	SetKeepUptoDate(bool)

	// Keepalive management by supervisor.
	GetKeepAlive() bool
	SetKeepAlive(bool)

	// Action permissions metadata.
	GetPermissions() []interface{}
	SetPermissions([]interface{})

	/** Required dependencies **/
	SetDependency(string)
	GetDependencies() []string

	/** Lifecycle **/

	// Initialize service (loads desired from etcd, sets defaults, writes baseline desired/runtime).
	Init() error

	// Persist service configuration (desired + runtime split handled under the hood).
	Save() error

	// Stop the service (transition to stopped and cleanup).
	StopService() error

	// Start the service (spawn gRPC server).
	StartService() error

	// Dist builds a publishable directory layout under the given path.
	Dist(path string) (string, error)

	// Returns a curated set of default roles for the service.
	RolesDefault() []resourcepb.Role
}

// ------------------------------
// Initialization / persistence
// ------------------------------

// InitService initializes common service attributes from CLI args, executable
// location, then loads desired from etcd (no config.json). If no desired config
// exists, a new one is created with sensible defaults.
// Public signature preserved.
func InitService(s Service) error {
	execPath, _ := osext.Executable()
	execPath = strings.ReplaceAll(execPath, "\\", "/")
	s.SetPath(execPath)

	// ID from arg[1] (supervisor launches child with the Id only)
	if len(os.Args) >= 2 {
		s.SetId(os.Args[1])
	}
	if s.GetId() == "" {
		s.SetId(Utility.RandomUUID())
	}

	// Contextual values.
	address, _ := config.GetAddress()
	domain, _ := config.GetDomain()
	mac, _ := config.GetMacAddress()

	s.SetMac(mac)
	s.SetAddress(address)
	s.SetDomain(domain)

	// Startup runtime.
	s.SetState("starting")
	s.SetProcess(os.Getpid())

	// Platform & checksum.
	s.SetPlatform(runtime.GOOS + "_" + runtime.GOARCH)
	s.SetChecksum(Utility.CreateFileChecksum(execPath))

	// Try to load desired from etcd and apply to this instance.
	if cfg, err := config.GetServiceConfigurationById(s.GetId()); err == nil && cfg != nil {
		applyDesiredToService(s, cfg)
	} else {
		// No existing desired; set conservative defaults if missing.
		if s.GetPort() == 0 {
			// leave 0 = caller decides; supervisor usually injects.
		}
		if s.GetProxy() == 0 && s.GetPort() != 0 {
			s.SetProxy(s.GetPort() + 1)
		}
		if s.GetProtocol() == "" {
			s.SetProtocol("http")
		}
	}

	// TLS configuration.
	if ca := config.GetLocalCACertificate(); ca != "" {
		s.SetCertAuthorityTrust(ca)
	}
	if cf := config.GetLocalServerCertificatePath(); cf != "" {
		s.SetCertFile(cf)
	}
	if kf := config.GetLocalServerKeyPath(); kf != "" {
		s.SetKeyFile(kf)
	}

	s.SetTls(s.GetTls() || (s.GetCertFile() != "" && s.GetKeyFile() != ""))


	// Persist initial snapshot (desired + starting runtime)
	return SaveService(s)
}

// SaveService persists the current service configuration to etcd (desired + runtime
// handled by config package). Public signature preserved.
func SaveService(s Service) error {
	s.SetModTime(time.Now().Unix())
	cfg, err := Utility.ToMap(s)

	if err != nil {
		slog.Error("SaveService: to map failed", "service", s.GetName(), "err", err)
		return err
	}
	if err := config.SaveServiceConfiguration(cfg); err != nil {
		slog.Error("SaveService: save failed", "service", s.GetName(), "err", err)
		return err
	}
	return nil
}

// ------------------------------
// Packaging (no config.json)
// ------------------------------

// Dist generates a versioned distribution tree for the service under distPath.
// It copies the executable and the .proto file. (config.json was removed.)
// Public signature preserved.
func Dist(distPath string, s Service) (string, error) {
	path := distPath + "/" + s.GetPublisherID() + "/" + s.GetName() + "/" + s.GetVersion() + "/" + s.GetId()
	if err := Utility.CreateDirIfNotExist(path); err != nil {
		return "", fmt.Errorf("Dist: create dir %q: %w", path, err)
	}

	// Copy .proto if present
	if p := s.GetProto(); p != "" && Utility.Exists(p) {
		if err := Utility.Copy(p, distPath+"/"+s.GetPublisherID()+"/"+s.GetName()+"/"+s.GetVersion()+"/"+s.GetName()+".proto"); err != nil {
			return "", fmt.Errorf("Dist: copy proto: %w", err)
		}
	}

	// Copy executable
	execName := s.GetPath()[strings.LastIndex(s.GetPath(), "/")+1:]
	if err := Utility.Copy(s.GetPath(), path+"/"+execName); err != nil {
		return "", fmt.Errorf("Dist: copy executable: %w", err)
	}

	// Optional: write a minimal metadata.json for tooling
	meta := map[string]any{
		"id":           s.GetId(),
		"name":         s.GetName(),
		"version":      s.GetVersion(),
		"publisher":    s.GetPublisherID(),
		"proto":        s.GetName() + ".proto",
		"executable":   execName,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}
	if b, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(path+"/metadata.json", b, 0o644)
	}

	return path, nil
}

// CreateServicePackage tars+gzips the dist output for a given platform.
// Public signature preserved.
func CreateServicePackage(s Service, distPath string, platform string) (string, error) {
	id := s.GetPublisherID() + "%" + s.GetName() + "%" + s.GetVersion() + "%" + s.GetId() + "%" + platform

	path, err := Dist(distPath, s)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	Utility.CompressDir(path, &buf)

	outPath := os.TempDir() + string(os.PathSeparator) + id + ".tar.gz"
	file, err := os.OpenFile(outPath, os.O_CREATE|os.O_RDWR, 0o755)
	if err != nil {
		return "", fmt.Errorf("CreateServicePackage: open %q: %w", outPath, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, &buf); err != nil {
		return "", fmt.Errorf("CreateServicePackage: write archive: %w", err)
	}

	if err := os.RemoveAll(path); err != nil {
		return "", fmt.Errorf("CreateServicePackage: cleanup dist: %w", err)
	}

	return outPath, nil
}

// GetPlatform returns GOOS_GOARCH of the current runtime.
// Public signature preserved.
func GetPlatform() string {
	return runtime.GOOS + "_" + runtime.GOARCH
}

// ------------------------------
// TLS helpers
// ------------------------------

// GetTLSConfig loads server TLS credentials and constructs a *tls.Config.
// It logs errors with slog and returns nil on failure (caller must handle).
// Public signature preserved (return type only).
func GetTLSConfig(key string, cert string, ca string) *tls.Config {

	tlsCer, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		slog.Error("GetTLSConfig: load keypair failed", "cert", cert, "key", key, "err", err)
		return nil
	}

	caBytes, err := os.ReadFile(ca)
	if err != nil {
		slog.Error("GetTLSConfig: read CA failed", "ca", ca, "err", err)
		return nil
	}

	certPool := x509.NewCertPool()
	if ok := certPool.AppendCertsFromPEM(caBytes); !ok {
		slog.Error("GetTLSConfig: append CA certs failed")
		return nil
	}

	hostname, _ := config.GetHostname()
	return &tls.Config{
		ServerName:   hostname, // no SNI
		Certificates: []tls.Certificate{tlsCer},
		ClientAuth:   tls.RequireAnyClientCert,
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			opts := x509.VerifyOptions{
				Roots:         certPool,
				CurrentTime:   time.Now(),
				Intermediates: x509.NewCertPool(),
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			}
			for _, c := range rawCerts[1:] {
				opts.Intermediates.AppendCertsFromPEM(c)
			}
			c, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return fmt.Errorf("tls: parse client cert: %w", err)
			}
			if _, err = c.Verify(opts); err != nil {
				return fmt.Errorf("tls: verify client cert: %w", err)
			}
			return nil
		},
	}
}

// Keepalive / concurrency parameters for gRPC servers.
const (
	grpcKeepaliveTime        = 30 * time.Second
	grpcKeepaliveTimeout     = 5 * time.Second
	grpcKeepaliveMinTime     = 30 * time.Second
	grpcMaxConcurrentStreams = 1_000_000
)

// InitGrpcServer constructs a *grpc.Server with standard keepalive, metrics,
// health, TLS (if enabled), and supplied interceptors.
// Public signature preserved.
func InitGrpcServer(s Service) (*grpc.Server, error) {
	var opts []grpc.ServerOption

	// Connection management.
	opts = append(opts,
		grpc.MaxConcurrentStreams(grpcMaxConcurrentStreams),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    grpcKeepaliveTime,
			Timeout: grpcKeepaliveTimeout,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             grpcKeepaliveMinTime,
			PermitWithoutStream: true,
		}),
	)

	// TLS (if enabled).
	if s.GetTls() {
		cfg := GetTLSConfig(s.GetKeyFile(), s.GetCertFile(), s.GetCertAuthorityTrust())
		if cfg == nil {
			return nil, fmt.Errorf("InitGrpcServer: TLS enabled but TLS config could not be created")
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(cfg)))
	}

	// Lazily obtain interceptors. This does not touch etcd at import time.
	unaryInterceptor, streamInterceptor, err := interceptors.Load()
	if err != nil {
		return nil, err
	}

	// Interceptors + Prometheus.
	switch {
	case unaryInterceptor != nil && streamInterceptor != nil:
		opts = append(
			opts,
			grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(unaryInterceptor, grpc_prometheus.UnaryServerInterceptor)),
			grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(streamInterceptor, grpc_prometheus.StreamServerInterceptor)),
		)
	default:
		opts = append(
			opts,
			grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
			grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		)
	}

	srv := grpc.NewServer(opts...)

	// Health + metrics.
	grpc_health_v1.RegisterHealthServer(srv, health.NewServer())
	grpc_prometheus.Register(srv)

	slog.Info("InitGrpcServer: gRPC server initialized",
		"service", s.GetName(),
		"tls", s.GetTls(),
		"port", s.GetPort(),
	)
	return srv, nil
}

// ------------------------------
// Service run + etcd watch
// ------------------------------

// StartService starts the gRPC server, sets up etcd watch for desired
// configuration changes on this service id, and blocks until SIGINT/SIGTERM.
// Public signature preserved.
func StartService(s Service, srv *grpc.Server) error {

	address := "0.0.0.0"
	lis, err := net.Listen("tcp", address+":"+strconv.Itoa(s.GetPort()))
	if err != nil {
		err_ := fmt.Errorf("StartService: listen %s:%d: %w", address, s.GetPort(), err)
		s.SetLastError(err_.Error())
		_ = putRuntimeStopped(s, err_.Error())
		if srv != nil {
			srv.Stop()
		}
		return err_
	}

	// Serve gRPC in background.
	go func() {
		if err := srv.Serve(lis); err != nil {
			s.SetLastError(err.Error())
			_ = putRuntimeFailed(s, err.Error())
			return
		}
	}()

	// Mark running and persist runtime only (avoid clobbering desired).
	_ = putRuntimeRunning(s)

	// Etcd watch for desired config changes on this service id.
	go watchDesiredConfig(s, srv)

	// Wait for termination signal.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch

	// Graceful shutdown.
	StopService(s, srv)
	return nil
}

// at top-level in services.go:
// top-level
// at top of services.go
func gracefulStopWithTimeout(srv *grpc.Server, d time.Duration) {
	if srv == nil {
		return
	}
	done := make(chan struct{})
	go func() { srv.GracefulStop(); close(done) }()
	select {
	case <-done:
		return
	case <-time.After(d):
		srv.Stop()
	}
}

// in StopService:
func StopService(s Service, srv *grpc.Server) error {
	s.SetState("closed")
	s.SetProcess(-1)
	s.SetLastError("")
	_ = putRuntimeClosed(s, "")
	if srv != nil {
		// env-tunable grace; default ~5s is usually plenty
		d := 5 * time.Second
		if v := strings.TrimSpace(os.Getenv("GLOBULAR_GRACEFUL_STOP")); v != "" {
			if dd, err := time.ParseDuration(v); err == nil && dd > 0 {
				d = dd
			}
		}
		gracefulStopWithTimeout(srv, d)
	}
	slog.Info("StopService: service stopped", "service", s.GetName(), "id", s.GetId())
	return nil
}

// ------------------------------
// etcd helpers (local client + watch)
// ------------------------------

// services.go

// watchDesiredConfig monitors the etcd key corresponding to the service's configuration
// and applies updates to the service instance when changes are detected. It listens for
// create or modify events on the configuration key, fetches the updated configuration,
// and applies relevant changes to the service. If the service's port or proxy settings
// are changed, a warning is logged indicating that a restart is required for the changes
// to take effect. The updated configuration is also broadcast to the event bus to maintain
// compatibility with legacy listeners.
//
// Parameters:
//
//	s   - The service instance to monitor and update.
//	srv - The gRPC server instance associated with the service.
func watchDesiredConfig(s Service, srv *grpc.Server) {
	cli, err := config.GetEtcdClient()
	if err != nil { /* ... */
		return
	}

	key := "/globular/services/" + s.GetId() + "/config"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wch := cli.Watch(ctx, key)
	for w := range wch {
		for _, ev := range w.Events {
			if ev.IsCreate() || ev.IsModify() {
				cfg, err := config.GetServiceConfigurationById(s.GetId())
				if err != nil { /* ... */
					continue
				}

				oldPort, oldProxy := s.GetPort(), s.GetProxy()
				applyDesiredToService(s, cfg)

				if stRaw, ok := cfg["State"]; ok && strings.EqualFold(Utility.ToString(stRaw), "closing") {
					slog.Info("closing requested via desired config; shutting down",
						"service", s.GetName(), "id", s.GetId())
					s.SetState("closing")
					_ = putRuntimeClosing(s, "")
					cancel() // <- stop this watch right away
					go func() {
						time.Sleep(100 * time.Millisecond)
						_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
					}()
					return
				}

				if s.GetPort() != oldPort || s.GetProxy() != oldProxy {
					slog.Warn("port/proxy change detected in desired; restart required to take effect",
						"service", s.GetName(), "id", s.GetId(),
						"old_port", oldPort, "new_port", s.GetPort(),
						"old_proxy", oldProxy, "new_proxy", s.GetProxy())
				}
			}
		}
	}
}

// applyDesiredToService copies expected desired fields from a map onto the Service via setters.
func applyDesiredToService(s Service, m map[string]any) {
	if m == nil {
		return
	}
	// Simple scalar fields
	if m["Id"] != nil {
		if v := Utility.ToString(m["Id"]); v != "" {
			s.SetId(v)
		}
	}

	if m["Name"] != nil {
		if v := Utility.ToString(m["Name"]); v != "" {
			s.SetName(v)
		}
	}

	if m["Description"] != nil {
		if v := Utility.ToString(m["Description"]); v != "" {
			s.SetDescription(v)
		}
	}
	if m["Domain"] != nil {
		if v := Utility.ToString(m["Domain"]); v != "" {
			s.SetDomain(v)
		}
	}
	if m["Address"] != nil {
		if v := Utility.ToString(m["Address"]); v != "" {
			s.SetAddress(v)
		}
	}
	if m["Protocol"] != nil {
		if v := Utility.ToString(m["Protocol"]); v != "" {
			s.SetProtocol(v)
		}
	}
	if m["Checksum"] != nil {
		if v := Utility.ToString(m["Checksum"]); v != "" {
			s.SetChecksum(v)
		}
	}
	if m["PublisherID"] != nil {
		if v := Utility.ToString(m["PublisherID"]); v != "" {
			s.SetPublisherID(v)
		}
	}
	if m["Version"] != nil {
		if v := Utility.ToString(m["Version"]); v != "" {
			s.SetVersion(v)
		}
	}
	if m["Proto"] != nil {
		if v := Utility.ToString(m["Proto"]); v != "" {
			s.SetProto(v)
		}
	}
	if m["Path"] != nil {
		if v := Utility.ToString(m["Path"]); v != "" {
			s.SetPath(v)
		}
	}

	// Arrays
	if m["Keywords"] != nil {
		if v, ok := m["Keywords"].([]any); ok {
			var out []string
			for _, x := range v {
				out = append(out, Utility.ToString(x))
			}
			s.SetKeywords(out)
		}
	}
	if m["Discoveries"] != nil {
		if v, ok := m["Discoveries"].([]any); ok {
			var out []string
			for _, x := range v {
				out = append(out, Utility.ToString(x))
			}
			s.SetDiscoveries(out)
		}
	}
	if m["Repositories"] != nil {
		if v, ok := m["Repositories"].([]any); ok {
			var out []string
			for _, x := range v {
				out = append(out, Utility.ToString(x))
			}
			s.SetRepositories(out)
		}
	}

	if v, ok := m["Permissions"].([]any); ok {
		// pass-through []any
		s.SetPermissions(v)
	}

	if m["Dependencies"] != nil {
		if v, ok := m["Dependencies"].([]any); ok {
			var out []string
			for _, x := range v {
				out = append(out, Utility.ToString(x))
			}
			// clear then set to preserve semantics
			for _, d := range out {
				s.SetDependency(d)
			}
		}
	}

	// Ports & TLS
	if m["Port"] != nil {
		s.SetPort(Utility.ToInt(m["Port"]))
	}
	if m["Proxy"] != nil {
		s.SetProxy(Utility.ToInt(m["Proxy"]))
	}
	if m["TLS"] != nil {
		s.SetTls(Utility.ToBool(m["TLS"]))
	}


	if m["CertAuthorityTrust"] != nil {
		if v := Utility.ToString(m["CertAuthorityTrust"]); v != "" {
			s.SetCertAuthorityTrust(v)
		}
	}

	if m["CertFile"] != nil {
		if v := Utility.ToString(m["CertFile"]); v != "" {
			s.SetCertFile(v)
		}
	}

	if m["KeyFile"] != nil {
		if v := Utility.ToString(m["KeyFile"]); v != "" {
			s.SetKeyFile(v)
		}
	} 

	// TLS enabled if both cert+key are set
	s.SetTls(s.GetTls() || (s.GetCertFile() != "" && s.GetKeyFile() != ""))

	// CORS
	if m["AllowAllOrigins"] != nil {
		s.SetAllowAllOrigins(Utility.ToBool(m["AllowAllOrigins"]))
	}
	if m["AllowedOrigins"] != nil {
		if v := Utility.ToString(m["AllowedOrigins"]); v != "" {
			s.SetAllowedOrigins(v)
		}
	}

	// Keepalive/update flags
	if m["KeepAlive"] != nil {
		s.SetKeepAlive(Utility.ToBool(m["KeepAlive"]))
	}
	if m["KeepUpToDate"] != nil {
		s.SetKeepUptoDate(Utility.ToBool(m["KeepUpToDate"]))
	}
}

// ------------------------------
// runtime setters in etcd
// ------------------------------

func putRuntimeRunning(s Service) error {
	return config.PutRuntime(s.GetId(), map[string]any{
		"Process":   os.Getpid(),
		"State":     "running",
		"LastError": "",
	})
}

func putRuntimeFailed(s Service, lastErr string) error {
	return config.PutRuntime(s.GetId(), map[string]any{
		"Process":   os.Getpid(),
		"State":     "failed",
		"LastError": lastErr,
	})
}

func putRuntimeStopped(s Service, lastErr string) error {
	return config.PutRuntime(s.GetId(), map[string]any{
		"Process":   -1,
		"State":     "stopped",
		"LastError": lastErr,
	})
}

func putRuntimeClosing(s Service, lastErr string) error {
	return config.PutRuntime(s.GetId(), map[string]any{
		"Process":   os.Getpid(),
		"State":     "closing",
		"LastError": lastErr,
	})
}

func putRuntimeClosed(s Service, lastErr string) error {
	return config.PutRuntime(s.GetId(), map[string]any{
		"Process":   -1,
		"State":     "closed",
		"LastError": lastErr,
	})
}
