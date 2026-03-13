package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/globulario/services/golang/cluster_controller/cluster_controller_server/internal/recovery"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/domain"
	globular_service "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/security"
	_ "github.com/globulario/services/golang/dnsprovider/cloudflare" // Register cloudflare provider
	_ "github.com/globulario/services/golang/dnsprovider/godaddy"    // Register godaddy provider
	_ "github.com/globulario/services/golang/dnsprovider/manual"     // Register manual provider
	planstore "github.com/globulario/services/golang/plan/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/keepalive"
	"path/filepath"
)

// jsonCodec registers a "json" gRPC codec so that ResourcesService messages
// (plain Go structs with json tags) can be decoded over the wire when a client
// sends Content-Type: application/grpc-web+json (or application/grpc+json).
type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error)        { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v interface{}) error   { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                                 { return "json" }

func init() { encoding.RegisterCodec(jsonCodec{}) }

// Version information (set via ldflags during build)
var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

func main() {
	// Define CLI flags
	cfgPath := flag.String("config", "/var/lib/globular/cluster-controller/config.json", "cluster controller configuration file")
	statePath := flag.String("state", defaultClusterStatePath, "cluster controller state file")
	describe := flag.Bool("describe", false, "print service metadata as JSON and exit")
	health := flag.Bool("health", false, "print service health as JSON and exit")
	version := flag.Bool("version", false, "print version information and exit")
	debug := flag.Bool("debug", false, "enable debug logging")
	help := flag.Bool("help", false, "show usage information")

	flag.Parse()

	// Handle --debug flag (set logger level)
	if *debug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
	}

	// Handle --help flag
	if *help {
		printUsage()
		os.Exit(0)
	}

	// Handle --version flag
	if *version {
		printVersion()
		os.Exit(0)
	}

	// Handle --describe flag
	if *describe {
		printDescribe()
		os.Exit(0)
	}

	// Handle --health flag
	if *health {
		printHealth()
		os.Exit(0)
	}

	// Environment variable overrides
	if env := os.Getenv("CLUSTER_STATE_PATH"); env != "" {
		*statePath = env
	}

	// Validate renderer registry (startup integrity check).
	if err := validateRenderers(); err != nil {
		logger.Error("renderer registry invalid", "error", err)
		os.Exit(1)
	}

	// Load configuration
	logger.Info("loading configuration", "config_path", *cfgPath)
	cfg, err := loadClusterControllerConfig(*cfgPath)
	if err != nil {
		logger.Error("failed to load config", "path", *cfgPath, "error", err)
		os.Exit(1)
	}

	// Load controller state
	logger.Info("loading controller state", "state_path", *statePath)
	state, err := loadControllerState(*statePath)
	if err != nil {
		logger.Error("failed to load state", "path", *statePath, "error", err)
		os.Exit(1)
	}

	// Seed configured join token into runtime state so Day-0 bootstrap can use it.
	// cfg.JoinToken comes from config.json "join_token" or CLUSTER_JOIN_TOKEN env var.
	if tok := strings.TrimSpace(cfg.JoinToken); tok != "" {
		if state.JoinTokens == nil {
			state.JoinTokens = make(map[string]*joinTokenRecord)
		}
		if state.JoinTokens[tok] == nil {
			state.JoinTokens[tok] = &joinTokenRecord{
				Token:     tok,
				ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7-day bootstrap window
				MaxUses:   10,                                  // allow multiple bootstrap attempts
			}
			if err := state.save(*statePath); err != nil {
				logger.Warn("failed to persist seeded join token", "err", err)
			} else {
				logger.Info("seeded Day-0 join token from config into runtime state")
			}
		}
	}

	// Create a DEDICATED etcd client for the cluster controller.
	// This is independent of the config package's shared singleton so that
	// health-probe reconnects in the config layer cannot destroy leader
	// election sessions, watches, or plan-store operations.
	var (
		planStore  planstore.PlanStore
		etcdClient *clientv3.Client
	)
	if c, err := config.NewEtcdClient(); err == nil {
		etcdClient = c
		planStore = planstore.NewEtcdPlanStore(c)
		logger.Info("etcd client connected (dedicated)", "endpoints", etcdClient.Endpoints())
	} else {
		logger.Warn("plan store unavailable", "error", err)
	}
	if etcdClient != nil {
		defer etcdClient.Close()
	}

	// Create gRPC listener
	address := fmt.Sprintf(":%d", cfg.Port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		logger.Error("failed to listen", "address", address, "error", err)
		os.Exit(1)
	}

	// Create gRPC server with TLS and recovery interceptors.
	// TLS is mandatory — use the same certificate paths as all other Globular services.
	logger.Debug("creating gRPC server with TLS and interceptors")
	serverOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			recovery.Unary(),
		),
		grpc.ChainStreamInterceptor(
			recovery.Stream(),
		),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
	}
	// Use the canonical server certificate (same paths as framework services).
	// GetTLSFile falls back to envoy-xds-client certs which are CLIENT-only
	// and cause "unsuitable certificate purpose" errors when used as server certs.
	certFile := config.GetLocalServerCertificatePath()
	keyFile := config.GetLocalServerKeyPath()
	caFile := config.GetLocalCACertificate()
	if certFile != "" && keyFile != "" && caFile != "" {
		tlsCfg := globular_service.GetTLSConfig(keyFile, certFile, caFile)
		if tlsCfg != nil {
			serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
			logger.Info("TLS enabled", "cert", certFile, "key", keyFile, "ca", caFile)
		} else {
			logger.Error("TLS config could not be created — refusing to start insecure")
			os.Exit(1)
		}
	} else {
		logger.Error("TLS certificate files not found — refusing to start insecure",
			"cert", certFile, "key", keyFile, "ca", caFile)
		os.Exit(1)
	}
	grpcServer := grpc.NewServer(serverOpts...)

	// Initialize server
	logger.Info("initializing cluster controller server")
	srv := newServer(cfg, *cfgPath, *statePath, state, planStore)
	srv.initResourceStore(etcdClient)

	// Gap 4: Verify built-in roles exist (including node-executor) at startup
	if err := security.EnsureBuiltinRolesExist(); err != nil {
		logger.Error("FATAL: built-in role bootstrap failed", "err", err)
		os.Exit(1)
	}
	logger.Info("built-in roles verified (including node-executor)")

	// Initialize plan signing (Ed25519 keypair for signed plans)
	if err := srv.initPlanSigner(); err != nil {
		logger.Warn("plan-signer: init failed (plans will be unsigned)", "err", err)
	}
	// Log dispatch mode (hardened vs compatibility)
	logPlanDispatchMode()

	// Register gRPC services
	cluster_controllerpb.RegisterClusterControllerServiceServer(grpcServer, srv)
	cluster_controllerpb.RegisterResourcesServiceServer(grpcServer, srv)
	logger.Debug("gRPC services registered")

	// Create background context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Bootstrap leader election
	leaderAddr := resolveLeaderAddr(address)
	logger.Info("bootstrapping leadership", "address", leaderAddr)
	bootstrapLeadership(ctx, srv, etcdClient, leaderAddr)

	// Start pprof server (debug endpoint)
	go startPprofServer()

	// Start background loops
	logger.Info("starting background loops")
	srv.startControllerRuntime(ctx, 4)
	srv.startAgentCleanupLoop(context.Background())
	srv.startOperationCleanupLoop(context.Background())
	srv.startHealthMonitorLoop(context.Background())

	// Start DNS reconciler if cluster domain is configured
	startDNSReconciler(srv, state)

	// Start domain reconciler for external domains (DNS providers + ACME)
	startDomainReconciler(ctx, etcdClient)

	// Register in the Globular service registry so the xDS watcher creates an Envoy cluster.
	// ClusterController is a standalone control-plane service that does not use the
	// globular_service framework; without this call it is invisible to service discovery.
	if err := config.SaveServiceConfiguration(map[string]interface{}{
		"Id":       "cluster_controller.ClusterControllerService",
		"Name":     "cluster_controller.ClusterControllerService",
		"Address":  "localhost",
		"Port":     cfg.Port,
		"Protocol": "grpc",
		"TLS":      true,
		"State":    "running",
		"Process":  os.Getpid(),
		"Version":  Version,
	}); err != nil {
		logger.Warn("failed to register in Globular service registry; xDS routing may be unavailable", "err", err)
	}

	// Start serving
	logger.Info("cluster controller ready",
		"address", address,
		"config", *cfgPath,
		"cluster_domain", cfg.ClusterDomain,
		"version", Version,
	)

	if err := grpcServer.Serve(lis); err != nil {
		logger.Error("grpc serve failed", "error", err)
		os.Exit(1)
	}
}

// startPprofServer starts the pprof + Prometheus metrics HTTP server.
func startPprofServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/metrics", promhttp.Handler())

	// Bind to :0 so we get a free port, then register with Prometheus.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		logger.Error("metrics/pprof: listen failed", "error", err)
		return
	}
	port := ln.Addr().(*net.TCPAddr).Port
	logger.Info("metrics/pprof listening", "addr", ln.Addr().String())
	writePromTargetFile("cluster_controller", port)

	if err := http.Serve(ln, mux); err != nil {
		logger.Error("metrics/pprof server error", "error", err)
	}
}

const promTargetsDir = "/var/lib/globular/prometheus/targets"

func writePromTargetFile(job string, port int) {
	content := fmt.Sprintf("- targets: [\"127.0.0.1:%d\"]\n  labels:\n    job: %s\n", port, job)
	if err := os.MkdirAll(promTargetsDir, 0750); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(promTargetsDir, job+".yaml"), []byte(content), 0644)
}

// startDNSReconciler starts the DNS reconciler if cluster domain is configured.
func startDNSReconciler(srv *server, state *controllerState) {
	// Start DNS reconciler (PR2) - uses state domain (always present from Day-0)
	// PR7: Support multiple DNS endpoints for high availability
	// Day-0 Security: No hardcoded endpoints, use discovery
	// C3: DNS must be initialized from Day-0 using state.ClusterNetworkSpec
	clusterDomain := ""
	if state.ClusterNetworkSpec != nil {
		clusterDomain = strings.TrimSpace(state.ClusterNetworkSpec.ClusterDomain)
	}

	if clusterDomain == "" {
		logger.Info("dns reconciler: DISABLED (no cluster_domain in state)")
		return
	}

	var dnsEndpoints []string
	dnsEndpointsStr := os.Getenv("CLUSTER_DNS_ENDPOINTS")
	if dnsEndpointsStr != "" {
		// Parse comma-separated list of DNS endpoints
		dnsEndpoints = strings.Split(dnsEndpointsStr, ",")
		for i := range dnsEndpoints {
			dnsEndpoints[i] = strings.TrimSpace(dnsEndpoints[i])
		}
	}
	// If no endpoints specified, NewDNSReconciler will discover them

	dnsReconciler := NewDNSReconciler(srv, dnsEndpoints)
	dnsReconciler.Start()
	logger.Info("dns reconciler: ENABLED", "domain", clusterDomain, "endpoints", dnsEndpoints)
}

// startDomainReconciler starts the external domain reconciler for DNS providers and ACME.
func startDomainReconciler(ctx context.Context, etcdClient *clientv3.Client) {
	if etcdClient == nil {
		logger.Info("domain reconciler: DISABLED (no etcd client)")
		return
	}

	// Auto-detect certificate ownership from /var/lib/globular
	certUID, certGID := 0, 0
	if info, err := os.Stat("/var/lib/globular"); err == nil {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			certUID = int(stat.Uid)
			certGID = int(stat.Gid)
		}
	}

	// Create domain reconciler
	reconciler, err := domain.NewReconciler(domain.ReconcilerConfig{
		EtcdClient:  etcdClient,
		Logger:      logger,
		CertsDir:    "/var/lib/globular/domains",
		CertUID:     certUID,
		CertGID:     certGID,
		Interval:    1 * time.Minute,       // Check every minute
		RenewBefore: 30 * 24 * time.Hour,   // Renew 30 days before expiry
	})
	if err != nil {
		logger.Error("domain reconciler: failed to create", "error", err)
		return
	}

	// Start reconciler
	if err := reconciler.Start(ctx); err != nil {
		logger.Error("domain reconciler: failed to start", "error", err)
		return
	}

	logger.Info("domain reconciler: ENABLED",
		"certs_dir", "/var/lib/globular/domains",
		"interval", "1m",
		"renew_before", "30d",
	)
}

// printUsage prints command-line usage information.
func printUsage() {
	fmt.Println("Globular Cluster Controller")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  cluster-controller [OPTIONS]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("ENVIRONMENT VARIABLES:")
	fmt.Println("  CLUSTER_STATE_PATH        Override state file path")
	fmt.Println("  CLUSTER_DNS_ENDPOINTS     Comma-separated DNS service endpoints")
	fmt.Println("  CLUSTER_PORT              Override gRPC listen port")
	fmt.Println("  CLUSTER_DOMAIN            Override cluster domain")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Start with default config")
	fmt.Println("  cluster-controller")
	fmt.Println()
	fmt.Println("  # Start with custom config")
	fmt.Println("  cluster-controller --config /etc/cluster/config.json")
	fmt.Println()
	fmt.Println("  # Enable debug logging")
	fmt.Println("  cluster-controller --debug")
	fmt.Println()
	fmt.Println("  # Print service metadata")
	fmt.Println("  cluster-controller --describe")
	fmt.Println()
}

// printVersion prints version information.
func printVersion() {
	info := map[string]string{
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}

// printDescribe prints service metadata as JSON.
func printDescribe() {
	metadata := map[string]interface{}{
		"name":        "cluster-controller",
		"version":     Version,
		"description": "Globular cluster controller manages nodes and orchestrates service deployments",
		"grpc_port":   12000,
		"pprof_port":  6060,
		"capabilities": []string{
			"node-management",
			"service-orchestration",
			"leader-election",
			"dns-reconciliation",
			"health-monitoring",
		},
		"build_info": map[string]string{
			"version":    Version,
			"build_time": BuildTime,
			"git_commit": GitCommit,
		},
	}
	data, _ := json.MarshalIndent(metadata, "", "  ")
	fmt.Println(string(data))
}

// printHealth prints service health status as JSON.
func printHealth() {
	health := map[string]interface{}{
		"status":  "unknown",
		"message": "health check requires running server instance",
		"checks": map[string]string{
			"note": "use GetClusterHealth RPC for cluster-wide health",
		},
	}
	data, _ := json.MarshalIndent(health, "", "  ")
	fmt.Println(string(data))
}
