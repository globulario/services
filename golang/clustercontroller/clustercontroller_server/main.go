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

	"github.com/globulario/services/golang/clustercontroller/clustercontroller_server/internal/recovery"
	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/domain"
	_ "github.com/globulario/services/golang/dnsprovider/cloudflare" // Register cloudflare provider
	_ "github.com/globulario/services/golang/dnsprovider/godaddy"    // Register godaddy provider
	_ "github.com/globulario/services/golang/dnsprovider/manual"     // Register manual provider
	planstore "github.com/globulario/services/golang/plan/store"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

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

	// Initialize etcd client and plan store
	var (
		planStore  planstore.PlanStore
		etcdClient *clientv3.Client
	)
	if c, err := config.GetEtcdClient(); err == nil {
		etcdClient = c
		planStore = planstore.NewEtcdPlanStore(c)
		logger.Info("etcd client connected", "endpoints", etcdClient.Endpoints())
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

	// Create gRPC server with recovery interceptors
	logger.Debug("creating gRPC server with interceptors")
	grpcServer := grpc.NewServer(
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
	)

	// Initialize server
	logger.Info("initializing cluster controller server")
	srv := newServer(cfg, *cfgPath, *statePath, state, planStore)
	srv.initResourceStore(etcdClient)

	// Register gRPC services
	clustercontrollerpb.RegisterClusterControllerServiceServer(grpcServer, srv)
	clustercontrollerpb.RegisterResourcesServiceServer(grpcServer, srv)
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

// startPprofServer starts the pprof debug HTTP server.
func startPprofServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	logger.Info("starting pprof server", "address", "127.0.0.1:6060")
	if err := http.ListenAndServe("127.0.0.1:6060", mux); err != nil {
		logger.Error("pprof server error", "error", err)
	}
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
