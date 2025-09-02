// Package globular_service provides helpers to initialize, run, and package
// Globular gRPC services with consistent config, TLS, keepalive and health setup.
package globular_service

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
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

	"github.com/fsnotify/fsnotify"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	Utility "github.com/globulario/utility"
	"github.com/kardianos/osext"
	"google.golang.org/grpc/keepalive"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// Service describes the minimal contract a Globular service must implement.
// NOTE: Public interface preserved verbatim.
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

	// HTTP(S) address (host:port) where config can be fetched.
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

	// Current service state (starting/running/stopped).
	GetState() string
	SetState(string)

	// Config file path (config.json).
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

	// One of: http/https/tls (transport to reach config).
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

	// Whether to serve gRPC with TLS.
	GetTls() bool
	SetTls(bool)

	// CA file path trusted by the server.
	GetCertAuthorityTrust() string
	SetCertAuthorityTrust(string)

	// Server certificate file path.
	GetCertFile() string
	SetCertFile(string)

	// Server key file path.
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

	// Initialize service (loads config, sets defaults, writes baseline config).
	Init() error

	// Persist service configuration to disk.
	Save() error

	// Stop the service (transition to stopped and cleanup).
	StopService() error

	// Start the service (spawn gRPC server).
	StartService() error

	// Dist builds a publishable directory layout under the given path.
	Dist(path string) (string, error)
}

// InitService initializes common service attributes from CLI args, executable
// location and configuration files, then persists the configuration.
// Public signature preserved.
func InitService(s Service) error {
	execPath, _ := osext.Executable()
	execPath = strings.ReplaceAll(execPath, "\\", "/")
	s.SetPath(execPath)

	// Determine configuration path & service id from arguments or defaults.
	if len(os.Args) == 3 {
		s.SetId(os.Args[1])
		s.SetConfigurationPath(strings.ReplaceAll(os.Args[2], "\\", "/"))
	} else if len(os.Args) == 2 {
		s.SetId(os.Args[1])
	} else if len(os.Args) == 1 {
		servicesDir := config.GetServicesDir()
		dir, _ := osext.ExecutableFolder()
		path := strings.ReplaceAll(dir, "\\", "/")

		if !strings.HasPrefix(path, servicesDir) {
			// Development mode: keep config next to executable.
			s.SetConfigurationPath(path + "/config.json")
		} else {
			servicesConfigDir := config.GetServicesConfigDir()
			configPath := strings.Replace(path, servicesDir, servicesConfigDir, -1)
			if Utility.Exists(configPath + "/config.json") {
				s.SetConfigurationPath(configPath + "/config.json")
			} else {
				s.SetConfigurationPath(path + "/config.json")
			}
		}
	}

	if len(s.GetConfigurationPath()) == 0 {
		err := fmt.Errorf("no configuration path determined for service %q", s.GetId())
		slog.Error("InitService: configuration path missing", "service", s.GetName(), "id", s.GetId(), "err", err)
		return err
	}

	// Load or create configuration.
	if Utility.Exists(s.GetConfigurationPath()) {
		if len(s.GetId()) > 0 {
			cfg, err := config.GetServiceConfigurationById(s.GetId())
			if err != nil {
				slog.Error("InitService: failed to read service configuration by id", "id", s.GetId(), "path", s.GetConfigurationPath(), "err", err)
				return err
			}
			str, err := Utility.ToJson(cfg)
			if err != nil {
				slog.Error("InitService: failed to marshal configuration", "id", s.GetId(), "err", err)
				return err
			}
			if err := json.Unmarshal([]byte(str), &s); err != nil {
				slog.Error("InitService: failed to unmarshal configuration into service", "id", s.GetId(), "err", err)
				return err
			}
		} else {
			data, err := os.ReadFile(s.GetConfigurationPath())
			if err != nil {
				slog.Error("InitService: failed to read configuration file", "path", s.GetConfigurationPath(), "err", err)
				return err
			}
			if err := json.Unmarshal(data, &s); err != nil {
				slog.Error("InitService: invalid configuration JSON", "path", s.GetConfigurationPath(), "err", err)
				return err
			}
		}
	} else {
		// No existing configuration; assign a new ID.
		s.SetId(Utility.RandomUUID())
	}

	// Fill contextual values.
	address, _ := config.GetAddress()
	domain, _ := config.GetDomain()
	macAddress, _ := config.GetMacAddress()

	s.SetMac(macAddress)
	s.SetAddress(address)
	s.SetDomain(domain)

	// Startup state.
	s.SetState("starting")
	s.SetProcess(os.Getpid())

	// Platform & checksum
	s.SetPlatform(runtime.GOOS + "_" + runtime.GOARCH)
	s.SetChecksum(Utility.CreateFileChecksum(execPath))

	slog.Info("InitService: initialized", "service", s.GetName(), "id", s.GetId(), "path", s.GetPath(), "config", s.GetConfigurationPath())
	return SaveService(s)
}

// SaveService persists the current service configuration.
// Public signature preserved.
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

// Dist generates a versioned distribution tree for the service under distPath.
// Public signature preserved.
func Dist(distPath string, s Service) (string, error) {
	path := distPath + "/" + s.GetPublisherID() + "/" + s.GetName() + "/" + s.GetVersion() + "/" + s.GetId()
	if err := Utility.CreateDirIfNotExist(path); err != nil {
		return "", fmt.Errorf("Dist: create dir %q: %w", path, err)
	}

	// Copy .proto
	if err := Utility.Copy(s.GetProto(), distPath+"/"+s.GetPublisherID()+"/"+s.GetName()+"/"+s.GetVersion()+"/"+s.GetName()+".proto"); err != nil {
		return "", fmt.Errorf("Dist: copy proto: %w", err)
	}

	// Copy config.json
	configPath := s.GetPath()[0:strings.LastIndex(s.GetPath(), "/")] + "/config.json"
	if !Utility.Exists(configPath) {
		return "", errors.New("Dist: missing config.json (run the service once to generate it)")
	}
	if err := Utility.Copy(configPath, path+"/config.json"); err != nil {
		return "", fmt.Errorf("Dist: copy config.json: %w", err)
	}

	// Copy executable
	execName := s.GetPath()[strings.LastIndex(s.GetPath(), "/")+1:]
	if !Utility.Exists(configPath) {
		return "", errors.New("Dist: missing config.json (run the service once to generate it)")
	}
	if err := Utility.Copy(s.GetPath(), path+"/"+execName); err != nil {
		return "", fmt.Errorf("Dist: copy executable: %w", err)
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

	return &tls.Config{
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
func InitGrpcServer(s Service, unaryInterceptor grpc.UnaryServerInterceptor, streamInterceptor grpc.StreamServerInterceptor) (*grpc.Server, error) {
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

var event_client_ *event_client.Event_Client

// getEventClient lazily initializes and returns a process-wide Event client.
func getEventClient() (*event_client.Event_Client, error) {
	if event_client_ != nil {
		return event_client_, nil
	}
	address, err := config.GetAddress()
	if err != nil {
		return nil, err
	}
	event_client_, err = event_client.NewEventService_Client(address, "event.EventService")
	if err != nil {
		return nil, err
	}
	return event_client_, nil
}

// helper: get the etcd client from config package without exporting its client symbol
func configEtcdClient() (*clientv3.Client, error) {
	// nasty but simple: ask for our own config key and reuse the client's grpc conn
	// better: expose a tiny getter in config, but keeping your public API unchanged here.
	return clientv3.New(clientv3.Config{
		Endpoints:   []string{strings.Split(strings.SplitN(Utility.ToString(config.GetAddress), ":", 2)[0], ",")[0] + ":2379"},
		DialTimeout: 3 * time.Second,
	})
}

// StartService starts the gRPC server, sets up config-file watching for live
// reload notifications, and blocks until SIGINT/SIGTERM.
// Public signature preserved.
func StartService(s Service, srv *grpc.Server) error {
	address := "0.0.0.0"
	lis, err := net.Listen("tcp", address+":"+strconv.Itoa(s.GetPort()))
	if err != nil {
		err_ := fmt.Errorf("StartService: listen %s:%d: %w", address, s.GetPort(), err)
		s.SetLastError(err_.Error())
		StopService(s, srv)
		return err_
	}

	// Serve gRPC in background.
	go func() {
		if err := srv.Serve(lis); err != nil {
			s.SetLastError(err.Error())
			StopService(s, srv)
			return
		}
	}()

	// Etcd watch for desired config changes on this service id
	go func(id string) {
		cli, err := configEtcdClient() // small wrapper below
		if err != nil {
			slog.Warn("StartService: etcd client init failed", "service", s.GetName(), "err", err)
			return
		}
		key := "/globular/services/" + id + "/config"
		wch := cli.Watch(context.Background(), key)
		for w := range wch {
			for _, ev := range w.Events {
				if ev.IsCreate() || ev.IsModify() {
					// Fetch merged (desired+runtime) via existing API to keep behavior
					cfg, err := config.GetServiceConfigurationById(id)
					if err != nil {
						slog.Warn("StartService: failed to fetch updated configuration", "service", s.GetName(), "err", err)
						continue
					}
					if data, err := json.Marshal(cfg); err == nil {
						_ = json.Unmarshal(data, &s)
						// broadcast like before
						if ec, e := getEventClient(); e == nil {
							_ = ec.Publish("update_globular_service_configuration_evt", data)
						}
						// Optional: honor a desired “ShouldStop” flag if you add one
						if s.GetState() == "stopped" {
							StopService(s, srv)
							os.Exit(0)
						}
					}
				}
			}
		}
	}(s.GetId())

	// Mark running and persist runtime
	time.Sleep(1 * time.Second)
	s.SetState("running")
	s.SetLastError("")
	s.SetProcess(os.Getpid())
	_ = SaveService(s)

	// Wait for termination signal.
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch

	// Graceful shutdown.
	StopService(s, srv)
	return SaveService(s)
}

// StopService transitions the service to "stopped", clears PID and stops gRPC.
// Public signature preserved.
func StopService(s Service, srv *grpc.Server) error {
	s.SetState("stopped")
	s.SetProcess(-1)
	s.SetLastError("")
	if srv != nil {
		srv.Stop()
	}
	slog.Info("StopService: service stopped", "service", s.GetName(), "id", s.GetId())
	return nil
}
