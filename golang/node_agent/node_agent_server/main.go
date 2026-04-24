package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"net/http"
	"path/filepath"

	"github.com/globulario/services/golang/config"
	globular_service "github.com/globulario/services/golang/globular_service"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Enable MinIO as the single source of truth for workflow definitions.
	// Falls back to local disk if MinIO is unreachable (e.g. during bootstrap).
	v1alpha1.EnableMinIOFetcher()

	// All configuration comes from CLI flags (bootstrap-time) or the state file.
	// No os.Getenv calls — etcd is the runtime source of truth.
	portFlag := flag.String("port", defaultPort, "gRPC listen port")
	bootstrapPlanFlag := flag.String("bootstrap-plan", "", "path to bootstrap plan JSON")
	etcdModeFlag := flag.String("etcd-mode", "managed", "etcd mode: managed|external")
	statePathFlag := flag.String("state-path", "/var/lib/globular/nodeagent/state.json", "path to node agent state file")
	advertiseAddrFlag := flag.String("advertise-addr", "", "advertise address (ip:port)")
	advertiseIPFlag := flag.String("advertise-ip", "", "advertise IP override")
	clusterModeFlag := flag.Bool("cluster-mode", true, "enable cluster mode (fail if no routable IP)")
	insecureFlag := flag.Bool("insecure", false, "use insecure gRPC connections")
	clusterDomainFlag := flag.String("cluster-domain", "", "cluster domain")
	controllerEndpointFlag := flag.String("controller-endpoint", "", "controller endpoint (host:port)")
	nodeIDFlag := flag.String("node-id", "", "node ID override")
	nodeNameFlag := flag.String("node-name", "", "node name override")
	joinTokenFlag := flag.String("join-token", "", "join token for cluster registration")
	bootstrapTokenFlag := flag.String("bootstrap-token", "", "bootstrap token")
	agentVersionFlag := flag.String("agent-version", "", "agent version string (injected by package installer)")
	controllerCAFlag := flag.String("controller-ca", "", "path to controller CA certificate")
	controllerSNIFlag := flag.String("controller-sni", "", "controller TLS SNI")
	controllerSystemRootsFlag := flag.Bool("controller-use-system-roots", false, "use system root CAs for controller TLS")
	labelsFlag := flag.String("labels", "", "comma-separated key=value node labels")
	domainFlag := flag.String("domain", "", "node domain for FQDN construction")
	tlsCertFlag := flag.String("tls-cert", "", "path to TLS server certificate")
	tlsKeyFlag := flag.String("tls-key", "", "path to TLS server key")
	tlsCAFlag := flag.String("tls-ca", "", "path to TLS CA certificate")
	clusterIDFlag := flag.String("cluster-id", "", "cluster identifier for workflow tracing")

	// DNS override flags (optional, for multi-NIC nodes)
	dnsIPv4Flag := flag.String("dns-ipv4", "", "override IPv4 for DNS A records")
	dnsIPv6Flag := flag.String("dns-ipv6", "", "override IPv6 for DNS AAAA records")
	dnsIfaceFlag := flag.String("dns-iface", "", "network interface for DNS IP selection")

	flag.Parse()

	port := *portFlag
	address := fmt.Sprintf("0.0.0.0:%s", port)

	statePath := *statePathFlag
	state, err := loadNodeAgentState(statePath)
	if err != nil {
		log.Printf("unable to load node agent state %s: %v", statePath, err)
	}

	cfg := NodeAgentConfig{
		Port:                     port,
		AdvertiseAddr:            *advertiseAddrFlag,
		AdvertiseIP:              *advertiseIPFlag,
		ClusterMode:              *clusterModeFlag,
		Insecure:                 *insecureFlag,
		ClusterDomain:            *clusterDomainFlag,
		ControllerEndpoint:       *controllerEndpointFlag,
		NodeID:                   *nodeIDFlag,
		NodeName:                 *nodeNameFlag,
		JoinToken:                *joinTokenFlag,
		BootstrapToken:           *bootstrapTokenFlag,
		AgentVersion:             *agentVersionFlag,
		ControllerCAPath:         *controllerCAFlag,
		ControllerSNI:            *controllerSNIFlag,
		ControllerUseSystemRoots: *controllerSystemRootsFlag,
		Labels:                   parseNodeAgentLabels(*labelsFlag),
		Domain:                   *domainFlag,
		DNSIPv4:                  *dnsIPv4Flag,
		DNSIPv6:                  *dnsIPv6Flag,
		DNSIface:                 *dnsIfaceFlag,
	}
	if cfg.JoinToken == "" && state != nil {
		cfg.JoinToken = strings.TrimSpace(state.JoinToken)
	}
	if cfg.AgentVersion == "" {
		cfg.AgentVersion = ""
	}

	srv := NewNodeAgentServer(statePath, state, cfg)
	srv.SetEtcdMode(*etcdModeFlag)
	if err := srv.saveState(); err != nil {
		log.Printf("unable to persist node agent startup state: %v", err)
	}
	// Plan store removed — workflows handle all execution.
	if planPath := strings.TrimSpace(*bootstrapPlanFlag); planPath != "" {
		if plan, err := loadBootstrapPlan(planPath); err != nil {
			log.Printf("unable to load bootstrap plan %s: %v", planPath, err)
		} else if len(plan) > 0 {
			srv.SetBootstrapPlan(plan)
			log.Printf("bootstrap plan loaded from %s", planPath)
		}
	}

	if srv.state != nil && srv.state.RequestID != "" && srv.nodeID == "" {
		srv.startJoinApprovalWatcher(context.Background(), srv.state.RequestID)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		var etcdErr error
		if srv.isEtcdManaged() {
			etcdCtx, etcdCancel := context.WithTimeout(ctx, 90*time.Second)
			defer etcdCancel()
			etcdErr = srv.EnsureEtcd(etcdCtx)
			if etcdErr != nil {
				log.Printf("etcd bootstrap failed: %v", etcdErr)
				return
			}
		}
		if err := srv.BootstrapIfNeeded(ctx); err != nil {
			log.Printf("bootstrap plan failed: %v", err)
		}
	}()

	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("unable to listen on %s: %v", address, err)
	}

	// TLS is mandatory. CLI flags override; fall back to standard Globular cert paths.
	serverOpts := []grpc.ServerOption{}
	certFile := strings.TrimSpace(*tlsCertFlag)
	keyFile := strings.TrimSpace(*tlsKeyFlag)
	caFile := strings.TrimSpace(*tlsCAFlag)
	// Use canonical server certs (same paths as framework services).
	if certFile == "" {
		certFile = config.GetLocalServerCertificatePath()
	}
	if keyFile == "" {
		keyFile = config.GetLocalServerKeyPath()
	}
	if caFile == "" {
		caFile = config.GetLocalCACertificate()
	}
	if certFile != "" && keyFile != "" && caFile != "" {
		tlsCfg := globular_service.GetTLSConfig(keyFile, certFile, caFile)
		if tlsCfg != nil {
			serverOpts = append(serverOpts, grpc.Creds(credentials.NewTLS(tlsCfg)))
			log.Printf("TLS enabled: cert=%s key=%s ca=%s", certFile, keyFile, caFile)
		} else {
			log.Fatalf("TLS config could not be created — refusing to start insecure")
		}
	} else {
		log.Fatalf("TLS certificate files not found (cert=%s key=%s ca=%s) — refusing to start insecure", certFile, keyFile, caFile)
	}
	grpcServer := grpc.NewServer(serverOpts...)
	// Connect to WorkflowService for plan execution tracing.
	// Uses lazy connection via discoverServiceAddr so the recorder works
	// even when the workflow service isn't available at startup (Day-1 join).
	// The address is re-resolved on each connection attempt — local port
	// first, then gateway fallback.
	wfClusterID := strings.TrimSpace(*clusterIDFlag)
	if wfClusterID == "" {
		// Resolve from etcd at runtime; default to "globular.internal" if unavailable.
		if domain, err := config.GetDomain(); err == nil && domain != "" {
			wfClusterID = domain
		} else {
			wfClusterID = "globular.internal"
		}
	}
	wfResolver := func() string {
		// Resolve workflow service address from etcd via service discovery.
		if addr := config.ResolveServiceAddr("workflow.WorkflowService", ""); addr != "" {
			return addr
		}
		return discoverServiceAddr(10220)
	}
	srv.workflowRec = workflow.NewRecorderWithResolver(wfResolver, wfClusterID)
	srv.clusterID = wfClusterID

	srv.StartHeartbeat(ctx)
	// Plan runner removed — workflows handle all execution.
	srv.StartACMERenewal(ctx)
	srv.StartCAKeySync(ctx)
	srv.StartIngressReconciliation(ctx)
	node_agentpb.RegisterNodeAgentServiceServer(grpcServer, srv)
	grpc_health_v1.RegisterHealthServer(grpcServer, health.NewServer())
	reflection.Register(grpcServer)

	// Register in the Globular service registry so the xDS watcher creates an Envoy cluster.
	// NodeAgent is a standalone control-plane service that does not use the
	// globular_service framework; without this call it is invisible to service discovery.
	if portNum, convErr := strconv.Atoi(port); convErr == nil {
		// Use the real advertised IP so that remote cluster-controller instances
		// can discover and reach this node agent.  advertisedAddr is "ip:port";
		// we only need the host part for the registry "Address" field.
		advertiseHost := strings.Split(srv.advertisedAddr, ":")[0]
		if regErr := config.SaveServiceConfiguration(map[string]interface{}{
			"Id":       "node_agent.NodeAgentService",
			"Name":     "node_agent.NodeAgentService",
			"Address":  advertiseHost,
			"Port":     portNum,
			"Protocol": "grpc",
			"TLS":      true,
			"State":    "running",
			"Process":  os.Getpid(),
			"Version":  cfg.AgentVersion,
		}); regErr != nil {
			log.Printf("warn: failed to register in Globular service registry; xDS routing may be unavailable: %v", regErr)
		}
	}

	// Start Prometheus metrics server.  Binds a stable port across restarts
	// when possible so Prometheus file_sd and xDS routing don't flap every
	// time the process restarts (see startMetricsServer for details).
	go startMetricsServer(srv.nodeID)

	log.Printf("node agent listening on %s", address)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("grpc serve failed: %v", err)
		}
	}()

	// Notify systemd that the service is ready (required for Type=notify units).
	// Also kick the watchdog periodically so WatchdogSec doesn't kill us.
	globular_service.SdNotify("READY=1")
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(15 * time.Second):
				globular_service.SdNotify("WATCHDOG=1")
			}
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down node agent (signal received)")
	grpcServer.GracefulStop()
	log.Printf("node agent stopped")
}

// metricsPortDefault is the preferred port for the node_agent metrics HTTP
// server — sibling of the gRPC default on 11000. Tried before any ephemeral
// fallback so first-time starts are deterministic.
const metricsPortDefault = 11001

// metricsPortEtcdKey returns the etcd key where this node persists the last
// successfully-bound metrics port. Empty nodeID yields an empty key so the
// caller skips persistence (pre-registration startup).
func metricsPortEtcdKey(nodeID string) string {
	if strings.TrimSpace(nodeID) == "" {
		return ""
	}
	return fmt.Sprintf("/globular/nodes/%s/node_agent_metrics_port", nodeID)
}

// startMetricsServer launches the Prometheus /metrics HTTP server.
//
// Port selection (first successful bind wins):
//  1. Port persisted in etcd from a previous run (stable across restarts).
//  2. metricsPortDefault (11001) — deterministic fallback for first starts.
//  3. Ephemeral (0.0.0.0:0) — only if the above are taken.
//
// Whichever port is finally bound is written back to etcd so the next
// restart picks the same one, avoiding scrape-target flapping, stale xDS
// routes, and alert noise on up{job="node_agent"}.
func startMetricsServer(nodeID string) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	savedPort := loadSavedMetricsPort(nodeID)
	ln, chosen := bindMetricsListener(savedPort)
	if ln == nil {
		return
	}
	if savedPort != chosen {
		persistMetricsPort(nodeID, chosen)
	}

	log.Printf("metrics listening on 0.0.0.0:%d (saved=%d, default=%d)",
		chosen, savedPort, metricsPortDefault)
	nodeIP := resolveRoutableIP()
	writePromTargetFile("node_agent", chosen)
	registerMetricsService("node-agent-metrics", nodeIP, chosen)
	if err := http.Serve(ln, mux); err != nil {
		log.Printf("metrics server error: %v", err)
	}
}

// bindMetricsListener tries saved → default → ephemeral and returns the
// listener plus the actually-bound port. Returns (nil, 0) on total failure.
func bindMetricsListener(savedPort int) (net.Listener, int) {
	candidates := []int{}
	if savedPort > 0 {
		candidates = append(candidates, savedPort)
	}
	if metricsPortDefault != savedPort {
		candidates = append(candidates, metricsPortDefault)
	}
	for _, p := range candidates {
		if ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p)); err == nil {
			return ln, p
		} else {
			log.Printf("metrics: port %d unavailable (%v), trying next", p, err)
		}
	}
	// Ephemeral fallback — OS picks any free port.
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		log.Printf("metrics: ephemeral listen failed: %v", err)
		return nil, 0
	}
	return ln, ln.Addr().(*net.TCPAddr).Port
}

// loadSavedMetricsPort reads the last-bound metrics port from etcd. Returns
// 0 if unset, unreachable, malformed, or out of range — the caller then
// falls through to metricsPortDefault.
func loadSavedMetricsPort(nodeID string) int {
	key := metricsPortEtcdKey(nodeID)
	if key == "" {
		return 0
	}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := cli.Get(ctx, key)
	if err != nil || len(resp.Kvs) == 0 {
		return 0
	}
	p, err := strconv.Atoi(strings.TrimSpace(string(resp.Kvs[0].Value)))
	if err != nil || p <= 0 || p > 65535 {
		return 0
	}
	return p
}

// persistMetricsPort writes the bound metrics port back to etcd so the next
// restart can reuse it. Best-effort; failures are logged but not fatal
// since an ephemeral fallback on next start will still work.
func persistMetricsPort(nodeID string, port int) {
	key := metricsPortEtcdKey(nodeID)
	if key == "" || port <= 0 {
		return
	}
	cli, err := config.GetEtcdClient()
	if err != nil {
		log.Printf("metrics: cannot persist port %d: etcd client: %v", port, err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := cli.Put(ctx, key, strconv.Itoa(port)); err != nil {
		log.Printf("metrics: cannot persist port %d to %s: %v", port, key, err)
	}
}

const promTargetsDir = "/var/lib/globular/prometheus/targets"

func writePromTargetFile(job string, port int) {
	// Emit this node's scrape target as a per-hostname file so that multiple
	// nodes writing into a shared file_sd_configs directory don't clobber
	// each other's entries. Prometheus globs `*.yaml` in the targets dir so
	// any naming that's unique per host will be picked up.
	nodeIP := resolveRoutableIP()
	hostname, _ := os.Hostname()
	target := portTarget(nodeIP, port) // already "ip:port"
	if target == "" {
		return
	}
	content := fmt.Sprintf(
		"- targets: [\"%s\"]\n  labels:\n    job: %s\n    instance: %s\n    node: %s\n",
		target, job, target, hostname,
	)
	if err := os.MkdirAll(promTargetsDir, 0750); err != nil {
		return
	}
	// Per-hostname filename avoids cross-node clobbering. We also remove any
	// legacy `<job>.yaml` written by older versions so stale, malformed or
	// wrong-node entries don't linger next to the fresh per-host file.
	filename := job + ".yaml"
	if hostname != "" {
		filename = fmt.Sprintf("%s_%s.yaml", job, hostname)
		legacy := filepath.Join(promTargetsDir, job+".yaml")
		if _, err := os.Stat(legacy); err == nil {
			_ = os.Remove(legacy)
		}
	}
	_ = os.WriteFile(filepath.Join(promTargetsDir, filename), []byte(content), 0644)
}

// resolveRoutableIP returns a non-loopback routable IP, preferring IPv4.
func resolveRoutableIP() string {
	nodeIP, ipErr := config.GetRoutableIP()
	if ipErr != nil || nodeIP == "" {
		nodeIP = config.GetRoutableIPv4()
	}
	if strings.HasPrefix(nodeIP, "127.") || strings.HasPrefix(nodeIP, "::1") {
		nodeIP = config.GetRoutableIPv4()
	}
	return nodeIP
}

// registerMetricsService publishes this node's metrics endpoint into the
// Globular service registry so xDS can expose it cluster-wide.
func registerMetricsService(name, ip string, port int) {
	if ip == "" {
		log.Printf("metrics: skip registry publish (no routable IP)")
		return
	}
	hostname, _ := os.Hostname()
	serviceID := fmt.Sprintf("%s-%s", name, hostname)
	conf := map[string]interface{}{
		"Id":       serviceID,
		"Name":     name,
		"Address":  ip,
		"Port":     port,
		"Protocol": "http",
		"TLS":      false,
		"State":    "running",
		"Process":  os.Getpid(),
		"Version":  "metrics",
	}
	if err := config.SaveServiceConfiguration(conf); err != nil {
		log.Printf("metrics: failed to register service %s: %v", serviceID, err)
	}
}

// portTarget returns ip:port for a routable address, or "" if none is available.
func portTarget(ip string, port int) string {
	if ip == "" {
		return ""
	}
	return fmt.Sprintf("%s:%d", ip, port)
}

func loadBootstrapPlan(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return nil, nil
	}
	var plan []string
	if err := json.Unmarshal(b, &plan); err != nil {
		return nil, err
	}
	clean := make([]string, 0, len(plan))
	for _, svc := range plan {
		if svc = strings.TrimSpace(svc); svc != "" {
			clean = append(clean, svc)
		}
	}
	return clean, nil
}
