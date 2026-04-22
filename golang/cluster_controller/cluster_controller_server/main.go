package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/globulario/services/golang/cluster_controller/cluster_controller_server/internal/recovery"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	Utility "github.com/globulario/utility"
	"github.com/globulario/services/golang/domain"
	globular_service "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/policy"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/globulario/services/golang/workflow/workflowpb"
	_ "github.com/globulario/services/golang/dnsprovider/cloudflare" // Register cloudflare provider
	_ "github.com/globulario/services/golang/dnsprovider/godaddy"    // Register godaddy provider
	_ "github.com/globulario/services/golang/dnsprovider/local"      // Register local (globular-dns) provider
	_ "github.com/globulario/services/golang/dnsprovider/manual"     // Register manual provider
	"github.com/prometheus/client_golang/prometheus/promhttp"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/encoding"
	grpchealth "google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
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
	BuildTime      = "unknown"
	GitCommit      = "unknown"
	BuildNumberStr = "0" // injected via ldflags: -X main.BuildNumberStr=6
)

// BuildNumber returns the parsed build number from the ldflags-injected string.
func parseBuildNumber() int64 {
	n, _ := strconv.ParseInt(BuildNumberStr, 10, 64)
	return n
}

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

func main() {
	// Enable etcd as the primary source for core workflow definitions.
	// etcd is always available (on every node) — no MinIO dependency.
	v1alpha1.EnableEtcdFetcher()
	// MinIO as fallback for service-owned workflows (compute, doctor, etc.).
	v1alpha1.EnableMinIOFetcher()

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

	// State path comes from CLI flag only — no env var override.

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
	//
	// Always re-seed on every startup, not just when the token is absent. The install
	// script restarts the controller twice (once for the token, once for final
	// stabilization). Between those restarts a reconcile or state-save may overwrite
	// the state file and lose the token. Re-seeding is idempotent: if the token is
	// already present with remaining uses, the existing record is kept unchanged.
	if tok := strings.TrimSpace(cfg.JoinToken); tok != "" {
		if state.JoinTokens == nil {
			state.JoinTokens = make(map[string]*joinTokenRecord)
		}
		if existing := state.JoinTokens[tok]; existing == nil || existing.Uses >= existing.MaxUses {
			state.JoinTokens[tok] = &joinTokenRecord{
				Token:     tok,
				ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7-day bootstrap window
				MaxUses:   100,                                 // allow many join attempts (multi-node cluster)
			}
			if err := state.save(*statePath); err != nil {
				logger.Warn("failed to persist seeded join token", "err", err)
			} else {
				logger.Info("seeded Day-0 join token from config into runtime state")
			}
		} else {
			logger.Info("Day-0 join token already active in runtime state", "uses", existing.Uses, "max", existing.MaxUses)
		}
	}

	// Create a DEDICATED etcd client for the cluster controller.
	// This is independent of the config package's shared singleton so that
	// health-probe reconnects in the config layer cannot destroy leader
	// election sessions, watches, or plan-store operations.
	var etcdClient *clientv3.Client
	if c, err := config.NewEtcdClient(); err == nil {
		etcdClient = c
		logger.Info("etcd client connected (dedicated)", "endpoints", etcdClient.Endpoints())
	} else {
		logger.Warn("etcd client unavailable", "error", err)
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
	srv := newServer(cfg, *cfgPath, *statePath, state, etcdClient)
	srv.initResourceStore(etcdClient)
	if etcdClient != nil {
		srv.etcdMembers = newEtcdMemberManager(etcdClient)
	}
	srv.scyllaMembers = newScyllaClusterManager()
	srv.scyllaMembers.probeNodeHealth = func(ctx context.Context, endpoint string) bool {
		return srv.probeScyllaHealth(ctx, endpoint)
	}
	srv.minioPoolMgr = newMinioPoolManager()

	// Ensure cluster-roles.json is deployed on disk before checking roles.
	// On fresh installs, the file doesn't exist yet — deploy the embedded copy.
	if err := policy.EnsureClusterRolesDeployed(); err != nil {
		logger.Warn("failed to deploy embedded cluster-roles.json", "err", err)
	}

	// Reload roles from the (now-deployed) policy file.
	security.ReloadClusterRoles()

	// Verify built-in roles exist (including node-executor) at startup.
	if err := security.EnsureBuiltinRolesExist(); err != nil {
		logger.Error("FATAL: built-in role bootstrap failed", "err", err)
		os.Exit(1)
	}
	logger.Info("built-in roles verified (including node-executor)")

	// Legacy plan signer init (no-op — plan system removed).
	if err := srv.initPlanSigner(); err != nil {
		logger.Warn("plan-signer init failed", "err", err)
	}

	// Register gRPC services
	cluster_controllerpb.RegisterClusterControllerServiceServer(grpcServer, srv)
	cluster_controllerpb.RegisterResourcesServiceServer(grpcServer, srv)
	cluster_controllerpb.RegisterNodeRecoveryServiceServer(grpcServer, srv)
	workflowpb.RegisterWorkflowActorServiceServer(grpcServer, srv.actorServer)
	healthSrv := grpchealth.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthSrv)
	reflection.Register(grpcServer)
	logger.Info("gRPC services registered (controller + resources + recovery + workflow actor + health + reflection)")

	// Create background context with signal handler for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Load the authoritative state from etcd at startup so non-leader
	// controllers can serve fresh read-only queries (e.g. ListNodes) without
	// waiting to become leader. Leaders will reload again on campaign win.
	srv.reloadStateFromEtcd()

	// Bootstrap leader election
	leaderAddr := resolveLeaderAddr(address)
	logger.Info("bootstrapping leadership", "address", leaderAddr)
	bootstrapLeadership(ctx, srv, etcdClient, leaderAddr)

	// Periodic state refresh for non-leader controllers so their in-memory
	// view of cluster state (read by ListNodes, GetHealth, etc.) doesn't
	// diverge from etcd. Leaders refresh on heartbeat/reconcile; followers
	// only have this periodic pull.
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.isLeader() {
					srv.reloadStateFromEtcd()
				}
			}
		}
	}()

	// Start pprof server (debug endpoint)
	go startPprofServer()

	// Try to load dynamic catalog from repository.
	if err := LoadCatalogFromRepository(""); err != nil {
		logger.Warn("using static fallback catalog", "err", err)
	}

	// Periodically reload catalog from repository so new packages are
	// discovered without restarting the controller.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := LoadCatalogFromRepository(""); err != nil {
					logger.Debug("catalog reload from repository failed (will retry)", "err", err)
				}
				// System contract: desired state must track latest published builds.
				if srv.isLeader() {
					srv.reconcileDesiredFromRepository(ctx)
				}
			}
		}
	}()

	// Start gRPC server BEFORE background loops so health checks and RPCs
	// work immediately. Without this, blocking calls in initProjections or
	// startDomainReconciler prevent Serve() from running, causing Envoy
	// health checks to fail and the admin app to show 503.
	go func() {
		logger.Info("cluster controller serving", "address", address)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("grpc serve failed", "error", err)
			os.Exit(1)
		}
	}()

	// Start background loops
	logger.Info("starting background loops")
	srv.startControllerRuntime(ctx, 4)
	srv.startAgentCleanupLoop(ctx)
	srv.startOperationCleanupLoop(ctx)
	srv.startHealthMonitorLoop(ctx)
	srv.startLeaderLivenessCheck(ctx)

	// Bring up read-only projections (node_identity, …). Best-effort: the
	// server continues if ScyllaDB is unreachable. See projection-clauses.md.
	closeProjections, _ := srv.initProjections(ctx)
	defer closeProjections()

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
		"Address":  registryHost(leaderAddr),
		"Port":     cfg.Port,
		"Protocol": "grpc",
		"TLS":      true,
		"State":    "running",
		"Process":  os.Getpid(),
		"Version":  Version,
	}); err != nil {
		logger.Warn("failed to register in Globular service registry; xDS routing may be unavailable", "err", err)
	}

	// Signal systemd that the service is ready. The unit file uses Type=notify
	// + WatchdogSec=60, so without READY=1 the unit stays in "activating"
	// forever and Restart=on-failure cannot fire on a clean stop.
	globular_service.SdNotify("READY=1\nSTATUS=serving")
	globular_service.SdWatchdogLoop()

	// Block main goroutine — the gRPC server runs in its own goroutine above.
	logger.Info("cluster controller ready",
		"address", address,
		"config", *cfgPath,
		"cluster_domain", cfg.ClusterDomain,
		"version", Version,
	)

	// Wait for context cancellation (signal handler).
	<-ctx.Done()
	logger.Info("shutting down")
	grpcServer.GracefulStop()
}

// startPprofServer starts the pprof + Prometheus metrics HTTP server.
func startPprofServer() {
	const metricsPort = 40377
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/metrics", promhttp.Handler())

	// Prefer a fixed port so Prometheus targets stay stable.
	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", metricsPort))
	if err != nil {
		// Fallback to :0 if the preferred port is taken.
		logger.Warn("metrics/pprof: preferred port unavailable, falling back to random", "port", metricsPort, "error", err)
		ln, err = net.Listen("tcp", "0.0.0.0:0")
		if err != nil {
			logger.Error("metrics/pprof: listen failed", "error", err)
			return
		}
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
	// Use routable IP so Prometheus can scrape from any node.
	nodeIP, ipErr := config.GetRoutableIP()
	if ipErr != nil || nodeIP == "" {
		return // cannot write target file without routable IP
	}
	if strings.HasPrefix(nodeIP, "127.") || strings.HasPrefix(nodeIP, "::1") {
		// Fallback to IPv4 if primary resolution returned loopback.
		nodeIP = config.GetRoutableIPv4()
	}
	if nodeIP == "" || strings.HasPrefix(nodeIP, "127.") || strings.HasPrefix(nodeIP, "::1") {
		logger.Warn("metrics/pprof: no routable IP found for target file; skipping")
		return
	}
	hostname, _ := os.Hostname()
	content := fmt.Sprintf("- targets: [\"%s:%d\"]\n  labels:\n    job: %s\n    instance: %s:%d\n    node: %s\n", nodeIP, port, job, nodeIP, port, hostname)
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

	// DNS endpoints discovered from etcd/cluster membership — no env override.
	var dnsEndpoints []string

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
//
// The output MUST include the standard fields the node-agent's install
// pipeline expects: "Id", "Port", "Address" (see
// node_agent_server/internal/actions/serviceports/config.go:describePayload).
// Without these, the install step fails with "describe returned empty Id"
// and the release workflow dies at apply_per_node.
func printDescribe() {
	// Deterministic ID: UUID5 from "Name:Version:MAC" — same scheme as all
	// Globular services. Each node gets a unique Id (different MAC).
	// Version includes build number so the Id changes on upgrade.
	mac, _ := config.GetMacAddress()
	fullVersion := Version
	if BuildNumberStr != "" && BuildNumberStr != "0" {
		fullVersion = Version + "-b" + BuildNumberStr
	}
	id := Utility.GenerateUUID("cluster_controller.ClusterControllerService:" + fullVersion + ":" + mac)

	// Resolve port from etcd (source of truth), fall back to config file.
	port := 0
	if svcCfg, err := config.GetServiceConfigurationsByName("cluster_controller.ClusterControllerService"); err == nil && svcCfg != nil {
		if p, ok := svcCfg["Port"]; ok {
			port = Utility.ToInt(p)
		}
	}
	if port == 0 {
		cfg, _ := loadClusterControllerConfig("/var/lib/globular/cluster-controller/config.json")
		if cfg != nil {
			port = cfg.Port
		}
	}
	metadata := map[string]interface{}{
		// ── Standard fields (required by node-agent install pipeline) ──
		"Id":      id,
		"Port":    port,
		"Address": fmt.Sprintf("0.0.0.0:%d", port),
		// ── Extended fields (informational) ──
		"Name":        "cluster-controller",
		"Version":     fullVersion,
		"Description": "Globular cluster controller manages nodes and orchestrates service deployments",
		"pprof_port":  6060,
		"capabilities": []string{
			"node-management",
			"service-orchestration",
			"leader-election",
			"dns-reconciliation",
			"health-monitoring",
		},
		"build_info": map[string]string{
			"version":    fullVersion,
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
