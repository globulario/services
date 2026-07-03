// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.server
// @awareness file_role=grpc_rpc_dispatcher_for_node_local_operations
// @awareness implements=globular.platform:intent.node_agent.is_executor_not_cluster_brain
// @awareness risk=high
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/node_agent/node_agent_server/healthchecks"
	"github.com/globulario/services/golang/node_agent/node_agent_server/identity"
	"github.com/globulario/services/golang/node_agent/node_agent_server/infra_truth"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/certs"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/installreceipt"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/units"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/pki"
	"github.com/globulario/services/golang/workflow"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

var defaultPort = "11000"

var (
	restartCommand    = restartUnit
	systemctlLookPath = exec.LookPath
	networkPKIManager = func(opts pki.Options) pki.Manager {
		return pki.NewFileManager(opts)
	}
	minioHealthURLForSpec = func(spec *cluster_controllerpb.ClusterNetworkSpec, nodeIP string) string {
		return fmt.Sprintf("https://%s:9000/minio/health/ready", nodeIP)
	}
	gatewayHealthURLForSpec = func(spec *cluster_controllerpb.ClusterNetworkSpec, nodeIP string) string {
		if strings.EqualFold(spec.GetProtocol(), "https") {
			return fmt.Sprintf("https://%s:%d/health", nodeIP, spec.GetPortHttps())
		}
		return fmt.Sprintf("http://%s:%d/health", nodeIP, spec.GetPortHttp())
	}
	dnsProbeAddr = func() string {
		if ip := nodeRoutableIP(); ip != "" {
			return ip + ":53"
		}
		return ""
	}
	tcpProbeAddrs = func() map[string]string {
		ip := nodeRoutableIP()
		if ip == "" {
			return nil
		}
		return map[string]string{
			"etcd":      ip + ":2379",
			"minio-tcp": ip + ":9000",
			"scylla":    ip + ":9042",
		}
	}
	envoyUnitActive = func() error {
		return runSystemctl("systemctl", "is-active", "globular-envoy.service")
	}
)

// NodeAgentConfig holds all bootstrap-time configuration for the node agent.
// Values come from CLI flags or the persisted state file — never from os.Getenv.
type NodeAgentConfig struct {
	Port                     string
	AdvertiseAddr            string
	AdvertiseIP              string
	ClusterMode              bool
	Insecure                 bool
	ClusterDomain            string
	ControllerEndpoint       string
	NodeID                   string
	NodeName                 string
	JoinToken                string
	BootstrapToken           string
	AgentVersion             string
	ControllerCAPath         string
	ControllerSNI            string
	ControllerUseSystemRoots bool
	Labels                   map[string]string
	Domain                   string // node domain for FQDN construction

	// DNS overrides (optional, for multi-NIC nodes)
	DNSIPv4  string
	DNSIPv6  string
	DNSIface string
}

// ControllerConnState tracks the node-agent's connectivity to the cluster controller.
type ControllerConnState string

const (
	ConnStateConnected     ControllerConnState = "connected"
	ConnStateDegraded      ControllerConnState = "degraded"
	ConnStateRediscovering ControllerConnState = "rediscovering"
	ConnStateUnreachable   ControllerConnState = "unreachable"
)

// NodeAgentServer implements the simplified node executor API.
type NodeAgentServer struct {
	node_agentpb.UnimplementedNodeAgentServiceServer

	mu                       sync.Mutex
	stateMu                  sync.Mutex
	controllerConnMu         sync.Mutex
	operations               map[string]*operation
	joinToken                string
	bootstrapToken           string
	controllerEndpoint       string
	agentVersion             string
	bootstrapPlan            []string
	nodeID                   string
	controllerConn           *grpc.ClientConn
	controllerClient         cluster_controllerpb.ClusterControllerServiceClient
	statePath                string
	state                    *nodeAgentState
	joinRequestID            string
	advertisedAddr           string
	useInsecure              bool
	joinPollCancel           context.CancelFunc
	joinPollMu               sync.Mutex
	wasJoining               bool // true when this process started with join credentials; never cleared
	etcdMode                 string
	controllerCAPath         string
	controllerSNI            string
	controllerUseSystemRoots bool
	controllerConnState      ControllerConnState
	lastControllerContact    time.Time
	consecutiveHeartbeatFail int
	lastNetworkGeneration    uint64
	controllerDialer         func(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error)
	controllerClientFactory  func(conn grpc.ClientConnInterface) cluster_controllerpb.ClusterControllerServiceClient
	controllerClientOverride func(addr string) cluster_controllerpb.ClusterControllerServiceClient
	// test hooks
	syncDNSHook           func(*cluster_controllerpb.ClusterNetworkSpec) error
	waitDNSHook           func(context.Context, *cluster_controllerpb.ClusterNetworkSpec) error
	ensureCertsHook       func(*cluster_controllerpb.ClusterNetworkSpec) error
	restartHook           func([]string, *operation) error
	objectstoreLayoutHook func(context.Context, string) error
	healthCheckHook       func(context.Context, *cluster_controllerpb.ClusterNetworkSpec) error

	certKV certs.KV

	lastCertRestart time.Time
	lastSpec        *cluster_controllerpb.ClusterNetworkSpec

	// Workflow tracing (nil-safe, fire-and-forget)
	workflowRec *workflow.Recorder
	clusterID   string

	// Infrastructure truth plane (Phase 1: ScyllaDB; Phase 2: etcd). Lazily
	// initialized via ensureInfraTruth so both NewNodeAgentServer and literal test
	// construction get a working cache/probers. The cache is read by the heartbeat
	// (never a slow inline probe) and refreshed by a background goroutine.
	infraOnce       sync.Once
	infraProbeCache *infra_truth.InfraProbeCache
	scyllaProber    *infra_truth.ScyllaProber
	etcdProber      *infra_truth.EtcdProber
	minioProber     *infra_truth.MinioProber
	envoyProber     *infra_truth.EnvoyProber

	// Bootstrap-time config passed from CLI flags (no os.Getenv at runtime)
	cfg NodeAgentConfig
}

func isNonRoutableEndpoint(endpoint string) bool {
	host, _, err := net.SplitHostPort(strings.TrimSpace(endpoint))
	if err != nil {
		return true
	}
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if host == "" {
		return true
	}
	switch strings.ToLower(host) {
	case "localhost", "0.0.0.0", "::":
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsUnspecified()
}

func NewNodeAgentServer(statePath string, state *nodeAgentState, cfg NodeAgentConfig) *NodeAgentServer {
	if state == nil {
		state = newNodeAgentState()
	}
	port := cfg.Port
	if port == "" {
		port = defaultPort
	}
	advertised := strings.TrimSpace(cfg.AdvertiseAddr)
	clusterMode := cfg.ClusterMode

	if advertised == "" {
		// Determine advertise IP using validated selection
		advertiseIP, err := identity.SelectAdvertiseIP(cfg.AdvertiseIP)
		if err != nil {
			if clusterMode {
				// In cluster mode, FAIL FAST if no valid IP
				log.Fatalf("node-agent: cannot determine advertise IP in cluster mode: %v", err)
			}
			// Development/single-node mode: use 0.0.0.0 (bind-all)
			log.Printf("node-agent: warning: no advertise IP, using 0.0.0.0 (development mode)")
			advertiseIP = "0.0.0.0"
		}
		advertised = fmt.Sprintf("%s:%s", advertiseIP, port)
	}

	// Validate advertise endpoint
	if err := identity.ValidateAdvertiseEndpoint(advertised, clusterMode); err != nil {
		log.Fatalf("node-agent: invalid advertise endpoint: %v", err)
	}
	useInsecure := cfg.Insecure

	// Determine cluster domain early for controller discovery (PR3).
	// Fallback order is important for wipe/reinstall day-1:
	// flags -> persisted state -> runtime config.
	clusterDomain := strings.TrimSpace(cfg.ClusterDomain)
	if clusterDomain == "" {
		clusterDomain = strings.TrimSpace(cfg.Domain)
	}
	if clusterDomain == "" {
		clusterDomain = strings.TrimSpace(state.ClusterDomain)
	}
	if clusterDomain == "" {
		if domain, err := config.GetDomain(); err == nil {
			clusterDomain = strings.TrimSpace(domain)
		}
	}
	protocol := strings.TrimSpace(state.Protocol)
	if protocol == "" {
		if localCfg, err := config.GetLocalConfig(true); err == nil {
			if v, ok := localCfg["Protocol"].(string); ok {
				protocol = strings.TrimSpace(strings.ToLower(v))
			}
		}
	}

	// Controller endpoint discovery:
	// etcd registry is source of truth; persisted state is only last-known cache.
	controllerEndpoint := strings.TrimSpace(cfg.ControllerEndpoint)
	if controllerEndpoint == "" {
		// DIRECT control endpoint (raw host:port), not mesh-routed :443 — the
		// node-agent must reach the controller without the Envoy mesh.
		controllerEndpoint = strings.TrimSpace(config.ResolveControllerDirectAddr())
	}
	if controllerEndpoint == "" {
		controllerEndpoint = state.ControllerEndpoint
	}
	if clusterMode && controllerEndpoint != "" && isNonRoutableEndpoint(controllerEndpoint) {
		log.Printf("node-agent: ignoring non-routable cached controller endpoint in cluster mode: %s", controllerEndpoint)
		controllerEndpoint = ""
	}
	if controllerEndpoint != "" {
		state.ControllerEndpoint = controllerEndpoint
	} else if clusterMode {
		state.ControllerEndpoint = ""
	}
	state.ControllerInsecure = useInsecure

	// Validate controller endpoint in cluster mode (PR3)
	if controllerEndpoint != "" && clusterMode {
		if err := identity.ValidateAdvertiseEndpoint(controllerEndpoint, clusterMode); err != nil {
			log.Printf("node-agent: WARNING - controller endpoint uses localhost in cluster mode: %s (this may prevent multi-node operation)", controllerEndpoint)
		}
	}

	nodeID := state.NodeID
	if nodeID == "" {
		nodeID = strings.TrimSpace(cfg.NodeID)
		state.NodeID = nodeID
	}

	// If no node ID is stored, derive one from hardware (MAC-based stable ID).
	// Do NOT override a controller-assigned ID — even if it differs from the
	// stable ID. The controller may have derived the ID from hostname+IPs
	// when the MAC wasn't available in the join request.
	if nodeID == "" {
		if stableID, err := identity.StableNodeID(); err == nil {
			log.Printf("node-agent: no node ID stored; using stable ID %s", stableID)
			nodeID = stableID
			state.NodeID = stableID
		}
	}

	// Node name selection (PR1)
	nodeName := cfg.NodeName
	if nodeName == "" {
		hostname, _ := os.Hostname()
		if hostname != "" {
			nodeName = identity.SanitizeNodeName(hostname)
		} else {
			nodeName = "node"
		}
	}
	state.NodeName = nodeName

	// Compute advertise FQDN (clusterDomain already defined above for controller discovery)
	if clusterDomain != "" {
		state.AdvertiseFQDN = fmt.Sprintf("%s.%s", nodeName, clusterDomain)
		state.ClusterDomain = clusterDomain
	}
	if protocol != "" {
		state.Protocol = protocol
	}
	state.AdvertiseIP = strings.Split(advertised, ":")[0]

	srv := &NodeAgentServer{
		operations:               make(map[string]*operation),
		joinToken:                strings.TrimSpace(cfg.JoinToken),
		bootstrapToken:           strings.TrimSpace(cfg.BootstrapToken),
		controllerEndpoint:       controllerEndpoint,
		agentVersion:             cfg.AgentVersion,
		bootstrapPlan:            nil,
		nodeID:                   nodeID,
		statePath:                statePath,
		state:                    state,
		joinRequestID:            state.RequestID,
		advertisedAddr:           advertised,
		useInsecure:              useInsecure,
		etcdMode:                 "managed",
		controllerCAPath:         strings.TrimSpace(cfg.ControllerCAPath),
		controllerSNI:            strings.TrimSpace(cfg.ControllerSNI),
		controllerUseSystemRoots: cfg.ControllerUseSystemRoots,
		lastNetworkGeneration:    state.NetworkGeneration,
		controllerDialer:         grpc.DialContext,
		controllerClientFactory:  cluster_controllerpb.NewClusterControllerServiceClient,
		cfg:                      cfg,
	}

	// Install the L2 desired-state resolver used by the
	// package.verify_integrity action. This closure is the
	// node-agent-side implementation of
	// invariant:four_layer.truth_read_via_owner_rpc_not_direct_storage:
	// L2 (desired state) is owned by the cluster_controller, so the
	// action reads it via the controller's GetDesiredState typed RPC
	// — never via direct etcd Get.
	actions.SetDesiredVersionResolver(srv.resolveDesiredVersions)

	// Record whether this process started with join credentials so
	// post-install scripts can detect Day-1 context. The join_id and
	// join_token are cleared from state.json by applyApprovedNodeID
	// before infrastructure packages are installed — but the post-install
	// scripts (e.g. ScyllaDB) need to know this is a fresh join, not an
	// upgrade. This flag is never cleared during the process lifetime.
	srv.wasJoining = state.JoinID != "" || strings.TrimSpace(cfg.JoinToken) != ""
	actions.SetJoinActiveFunc(func() bool { return srv.wasJoining })

	return srv
}

// ensureInfraTruth lazily initializes the infra truth-plane cache and prober.
// It is idempotent and safe to call from multiple goroutines; using sync.Once
// means both NewNodeAgentServer instances and literal test constructions get a
// working cache/prober without touching the constructor.
func (srv *NodeAgentServer) ensureInfraTruth() {
	srv.infraOnce.Do(func() {
		if srv.infraProbeCache == nil {
			srv.infraProbeCache = infra_truth.NewInfraProbeCache()
		}
		if srv.scyllaProber == nil {
			srv.scyllaProber = infra_truth.NewScyllaProber()
		}
		if srv.etcdProber == nil {
			p := infra_truth.NewEtcdProber()
			// Inject the node-agent's native-API observer; without it the runtime
			// layer reports "not observed" rather than fabricating health.
			p.Observe = observeEtcdRuntime
			srv.etcdProber = p
		}
		if srv.minioProber == nil {
			p := infra_truth.NewMinioProber()
			// Inject the node-agent's MinIO health observer (credential-free).
			p.Observe = observeMinioRuntime
			srv.minioProber = p
		}
		if srv.envoyProber == nil {
			p := infra_truth.NewEnvoyProber()
			// Inject the node-agent's Envoy admin-API observer.
			p.Observe = observeEnvoyRuntime
			srv.envoyProber = p
		}
	})
}

// resolveDesiredVersions fetches the cluster-wide desired-state map
// from the cluster_controller's typed GetDesiredState RPC and returns
// it keyed by lowercase short service name (e.g. "echo", not
// "core@globular.io/echo"). Best-effort: returns an empty map if the
// controller is unreachable or returns nothing, so the I2 invariant in
// package.verify_integrity is skipped rather than fabricated.
func (srv *NodeAgentServer) resolveDesiredVersions(ctx context.Context) map[string]actions.DesiredRef {
	out := map[string]actions.DesiredRef{}
	if srv == nil || srv.controllerEndpoint == "" {
		return out
	}
	if err := srv.ensureControllerClient(ctx); err != nil {
		return out
	}
	srv.controllerConnMu.Lock()
	client := srv.controllerClient
	srv.controllerConnMu.Unlock()
	if client == nil {
		return out
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := client.GetDesiredState(callCtx, &emptypb.Empty{})
	if err != nil || resp == nil {
		return out
	}
	for _, svc := range resp.GetServices() {
		id := svc.GetServiceId()
		if id == "" {
			continue
		}
		// service_id is "<publisher>/<name>" or just "<name>"; the
		// I2 invariant uses the short name keyed lowercase.
		if idx := strings.LastIndex(id, "/"); idx >= 0 {
			id = id[idx+1:]
		}
		out[strings.ToLower(id)] = actions.DesiredRef{
			Version: svc.GetVersion(),
			Build:   svc.GetBuildNumber(),
		}
	}
	return out
}

func (srv *NodeAgentServer) SetEtcdMode(mode string) {
	if mode == "" {
		return
	}
	srv.etcdMode = strings.ToLower(strings.TrimSpace(mode))
}

func (srv *NodeAgentServer) isEtcdManaged() bool {
	return strings.EqualFold(srv.etcdMode, "managed")
}

func (srv *NodeAgentServer) EnsureEtcd(ctx context.Context) error {
	if !srv.isEtcdManaged() {
		return nil
	}
	unit := units.UnitForService("etcd")
	if unit == "" {
		unit = "globular-etcd.service"
	}
	log.Printf("etcd bootstrap skipped; %s should be managed by systemd", unit)
	return nil
}

func (srv *NodeAgentServer) SetBootstrapPlan(plan []string) {
	srv.bootstrapPlan = append([]string(nil), plan...)
}

func (srv *NodeAgentServer) BootstrapIfNeeded(ctx context.Context) error {
	if len(srv.bootstrapPlan) == 0 {
		return nil
	}
	var reachable bool
	if srv.controllerEndpoint != "" {
		timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		if err := srv.ensureControllerClient(timeoutCtx); err == nil {
			reachable = true
		}
	}
	if reachable {
		return nil
	}
	// Bootstrap plan execution removed — uses workflow-native day0.bootstrap.
	return nil
}

// nodeRoutableIP returns this node's best routable IP.
// Falls back to config.GetRoutableIPv4() which scans interfaces.
// Never returns localhost — callers must handle the empty-string case.
func nodeRoutableIP() string {
	if ip, err := identity.SelectAdvertiseIP(""); err == nil {
		return ip
	}
	if ip := config.GetRoutableIPv4(); ip != "" {
		return ip
	}
	return ""
}

func buildHealthChecks(spec *cluster_controllerpb.ClusterNetworkSpec) []healthchecks.Check {
	if spec == nil {
		return nil
	}
	domain := strings.TrimSpace(spec.GetClusterDomain())

	nodeIP := nodeRoutableIP()
	if nodeIP == "" {
		return nil
	}

	checks := []healthchecks.Check{
		{
			Name:           "minio",
			URL:            minioHealthURLForSpec(spec, nodeIP),
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
			InsecureTLS:    true,
		},
	}
	if strings.EqualFold(spec.GetProtocol(), "https") {
		checks = append(checks, healthchecks.Check{
			Name:           "gateway-https",
			URL:            gatewayHealthURLForSpec(spec, nodeIP),
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
			InsecureTLS:    true,
			HostHeader:     domain,
		})
	} else {
		checks = append(checks, healthchecks.Check{
			Name:           "gateway-http",
			URL:            gatewayHealthURLForSpec(spec, nodeIP),
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
			HostHeader:     domain,
		})
	}
	return checks
}

func runConvergenceChecks(ctx context.Context, spec *cluster_controllerpb.ClusterNetworkSpec) error {
	if spec == nil {
		return nil
	}
	if err := healthchecks.RunChecks(ctx, buildHealthChecks(spec)); err != nil {
		return err
	}
	if err := runSupplementalChecks(ctx, spec); err != nil {
		return err
	}
	return nil
}

var dnsLookupHost = func(ctx context.Context, resolver *net.Resolver, host string) ([]string, error) {
	return resolver.LookupHost(ctx, host)
}

func runSupplementalChecks(ctx context.Context, spec *cluster_controllerpb.ClusterNetworkSpec) error {
	var errs []string
	domain := strings.TrimSpace(spec.GetClusterDomain())
	if domain == "" {
		errs = append(errs, "dns: empty domain")
	} else {
		dnsAddr := dnsProbeAddr()
		if dnsAddr == "" {
			errs = append(errs, "dns: no routable DNS probe address")
		} else {
			resolver := &net.Resolver{
				PreferGo: true,
				Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
					return net.DialTimeout("udp", dnsAddr, 3*time.Second)
				},
			}
			target := fmt.Sprintf("gateway.%s", domain)
			if _, err := dnsLookupHost(ctx, resolver, target); err != nil {
				errs = append(errs, fmt.Sprintf("dns lookup %s failed: %v", target, err))
			}
		}
	}

	for name, addr := range tcpProbeAddrs() {
		d := net.Dialer{Timeout: 3 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s dial %s: %v", name, addr, err))
			continue
		}
		conn.Close()
	}
	if err := envoyUnitActive(); err != nil {
		errs = append(errs, fmt.Sprintf("envoy unit inactive: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("supplemental health failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

func writeAtomicFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		tmp.Close()
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}
	defer cleanup()
	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	if err := os.Chmod(path, perm); err != nil {
		return err
	}
	tmpName = ""
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirFile.Close()
	if err := dirFile.Sync(); err != nil {
		return err
	}
	return nil
}

func copyFilePerm(src, dst string, perm os.FileMode) error {
	if src == "" {
		return fmt.Errorf("source file is empty")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := writeAtomicFile(dst, data, perm); err != nil {
		return err
	}
	return os.Chmod(dst, perm)
}

func concatFiles(dst string, parts ...string) error {
	if len(parts) == 0 {
		return fmt.Errorf("no parts to concat")
	}
	var out []byte
	for _, part := range parts {
		if part == "" {
			continue
		}
		data, err := os.ReadFile(part)
		if err != nil {
			return err
		}
		out = append(out, data...)
	}
	if len(out) == 0 {
		return fmt.Errorf("no content to write")
	}
	return writeAtomicFile(dst, out, 0o644)
}

func systemdUnitExists(systemctl, unit string) error {
	cmd := exec.Command(systemctl, "show", "--property=LoadState", unit)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
		}
		return err
	}
	return nil
}

func runSystemctl(systemctl, action, unit string) error {
	cmd := exec.Command(systemctl, action, unit)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return fmt.Errorf("%w: %s", err, trimmed)
		}
		return err
	}
	return nil
}

func waitForFiles(paths []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		missing := []string{}
		for _, p := range paths {
			info, err := os.Stat(p)
			if err != nil || info.Size() == 0 {
				missing = append(missing, p)
			}
		}
		if len(missing) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("missing files after wait: %s", strings.Join(missing, ", "))
		}
		time.Sleep(time.Second)
	}
}

func mergeNetworkIntoConfig(basePath, overlay string) error {
	if strings.TrimSpace(overlay) == "" {
		return nil
	}
	var overlayData map[string]interface{}
	if err := json.Unmarshal([]byte(overlay), &overlayData); err != nil {
		return fmt.Errorf("parse overlay: %w", err)
	}
	if len(overlayData) == 0 {
		return nil
	}
	base := make(map[string]interface{})
	data, err := os.ReadFile(basePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read base config: %w", err)
		}
	} else if len(data) > 0 {
		if err := json.Unmarshal(data, &base); err != nil {
			return fmt.Errorf("parse base config: %w", err)
		}
	}
	if base == nil {
		base = make(map[string]interface{})
	}
	allowed := map[string]struct{}{
		"Domain":           {},
		"Protocol":         {},
		"PortHTTP":         {},
		"PortHTTPS":        {},
		"ACMEEnabled":      {},
		"AdminEmail":       {},
		"AlternateDomains": {},
	}
	for key, value := range overlayData {
		if _, ok := allowed[key]; ok {
			base[key] = value
		}
	}
	merged, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal merged config: %w", err)
	}
	if err := writeAtomicFile(basePath, merged, 0o644); err != nil {
		return fmt.Errorf("write merged config: %w", err)
	}
	return nil
}

// isWiredInterface returns true for interface names that indicate a wired
// (Ethernet) connection. Wired interfaces are preferred over WiFi for
// cluster services (etcd, ScyllaDB, MinIO) because they have stable IPs.
func isWiredInterface(name string) bool {
	return strings.HasPrefix(name, "eth") ||
		strings.HasPrefix(name, "eno") ||
		strings.HasPrefix(name, "enp") ||
		strings.HasPrefix(name, "ens") ||
		strings.HasPrefix(name, "enx")
}

// defaultGatewayIPs returns the set of default-gateway IPs from the kernel
// routing table (/proc/net/route). These are the IPs of upstream routers, not
// addresses this node owns — they must never appear in the node's identity IP
// list or in /globular/cluster/scylla/hosts.
func defaultGatewayIPs() map[string]bool {
	gws := make(map[string]bool)
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return gws
	}
	for _, line := range strings.Split(string(data), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		// Column 1 is Destination (little-endian hex). Default route = "00000000".
		if fields[1] != "00000000" {
			continue
		}
		// Column 2 is Gateway (little-endian hex).
		var gw uint32
		if _, err := fmt.Sscanf(fields[2], "%x", &gw); err != nil {
			continue
		}
		if gw == 0 {
			continue
		}
		// Convert little-endian to dotted-decimal.
		ip := net.IPv4(byte(gw), byte(gw>>8), byte(gw>>16), byte(gw>>24))
		gws[ip.String()] = true
	}
	return gws
}

func gatherIPs() []string {
	type ifaceIP struct {
		ip    string
		wired bool
	}

	// Exclude default gateway IPs — they belong to the upstream router, not
	// to this node. If a transient interface (keepalived, PPPoE, bridge) is
	// momentarily bound to the gateway address, gatherIPs must not report it
	// as a node-owned address; doing so would corrupt /globular/cluster/scylla/hosts.
	gatewayIPs := defaultGatewayIPs()

	var collected []ifaceIP
	seen := make(map[string]struct{})
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		// Skip down or loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		wired := isWiredInterface(iface.Name)
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			// Skip nil, loopback, or IPv6 addresses
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue
			}
			text := ip.String()
			if _, ok := seen[text]; ok {
				continue
			}
			// Skip upstream gateway addresses — this node does not own them.
			if gatewayIPs[text] {
				continue
			}
			seen[text] = struct{}{}
			collected = append(collected, ifaceIP{ip: text, wired: wired})
		}
	}

	// Sort: wired first, then private IPs first, then lexicographic.
	sort.SliceStable(collected, func(i, j int) bool {
		// Wired beats WiFi
		if collected[i].wired != collected[j].wired {
			return collected[i].wired
		}
		// Private beats public
		ipI := net.ParseIP(collected[i].ip)
		ipJ := net.ParseIP(collected[j].ip)
		if ipI != nil && ipJ != nil {
			privI := isPrivateIP(ipI)
			privJ := isPrivateIP(ipJ)
			if privI != privJ {
				return privI
			}
		}
		return collected[i].ip < collected[j].ip
	})

	ips := make([]string, len(collected))
	for i, c := range collected {
		ips[i] = c.ip
	}
	return ips
}

// excludeIdentityIP removes a single IP from an identity IP list.
// Used to prevent floating VIP addresses from being published as stable node identity.
func excludeIdentityIP(ips []string, excluded string) []string {
	excluded = strings.TrimSpace(excluded)
	if excluded == "" || len(ips) == 0 {
		return ips
	}
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		if strings.TrimSpace(ip) == excluded {
			continue
		}
		out = append(out, ip)
	}
	return out
}

func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	ip = ip.To4()
	if ip == nil {
		return false
	}

	// 10.0.0.0/8
	if ip[0] == 10 {
		return true
	}
	// 172.16.0.0/12
	if ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31 {
		return true
	}
	// 192.168.0.0/16
	if ip[0] == 192 && ip[1] == 168 {
		return true
	}
	return false
}

func (srv *NodeAgentServer) buildNodeIdentity() *cluster_controllerpb.NodeIdentity {
	hostname, _ := os.Hostname()
	domain := srv.cfg.Domain
	// Build an advertise FQDN so other nodes can resolve this host without
	// relying on bare-hostname DNS (which only works with /etc/hosts or
	// search-domain config). Prefer hostname.domain; fall back to hostname.
	advertiseFqdn := hostname
	if domain != "" && !strings.Contains(hostname, ".") {
		advertiseFqdn = hostname + "." + domain
	}
	ips := gatherIPs()
	// Never publish keepalived VIP as stable node identity. VIP is a floating
	// ingress address, not per-node identity.
	ips = excludeIdentityIP(ips, srv.lookupIngressVIP())

	return &cluster_controllerpb.NodeIdentity{
		Hostname:      hostname,
		Domain:        domain,
		AdvertiseFqdn: advertiseFqdn,
		Ips:           ips,
		Os:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		AgentVersion:  srv.agentVersion,
	}
}

// detectUnits reports the active/inactive state of every managed systemd unit
// on this node. Set skipMinioService=true when the node is not a member of the
// MinIO pool — the topology gate already stopped globular-minio.service on
// non-members, so reporting it as inactive would produce a false-positive drift
// finding in the cluster doctor.
func detectUnits(ctx context.Context, nodeID string, skipMinioService bool) []*node_agentpb.UnitStatus {
	if ctx == nil {
		ctx = context.Background()
	}

	// Must-check baseline — infrastructure units that may not have the
	// globular-* prefix (e.g. scylla-server, keepalived).
	baseline := []string{
		"globular-etcd.service",
		"globular-dns.service",
		"globular-event.service",
		"globular-rbac.service",
		"globular-file.service",
		"globular-gateway.service",
		"globular-xds.service",
		"globular-envoy.service",
		"scylla-server.service",
		"keepalived.service",
	}
	if !skipMinioService {
		baseline = append(baseline, "globular-minio.service")
	}

	// Dynamic discovery: find all installed globular-*.service unit files.
	discovered := make(map[string]bool)
	for _, u := range baseline {
		discovered[u] = true
	}
	discoverCtx, discoverCancel := context.WithTimeout(ctx, 3*time.Second)
	if out, err := exec.CommandContext(discoverCtx, "systemctl", "list-unit-files",
		"globular-*.service", "--no-legend", "--no-pager").Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) >= 1 && strings.HasSuffix(fields[0], ".service") {
				discovered[fields[0]] = true
			}
		}
	}
	discoverCancel()

	// Dynamic discovery (systemctl list-unit-files glob) adds all globular-*.service
	// entries unconditionally. Remove minio explicitly for non-pool nodes so they
	// don't generate false-positive drift findings.
	if skipMinioService {
		delete(discovered, "globular-minio.service")
	}

	// Build sorted unit list for deterministic output.
	unitList := make([]string, 0, len(discovered))
	for u := range discovered {
		unitList = append(unitList, u)
	}
	sort.Strings(unitList)

	// Pre-fetch installed_state once and build unit-name → pkg lookup.
	// Authority order for unit drift detection (see docs/architecture/
	// retire-systemd-sidecars.md): installed_state.metadata.unit_file_sha256
	// is the SOLE authority. Legacy .sha256 sidecars are consumed only as
	// a one-time migration seed when metadata is absent but a sidecar exists.
	// Both absent → fail closed (installed_state_missing_or_unproven).
	unitToPkg := buildUnitToPackageMap(ctx, nodeID)

	// Query rich state for each unit.
	statuses := make([]*node_agentpb.UnitStatus, 0, len(unitList))
	for _, unit := range unitList {
		activeState, subState, loadState := queryUnitState(ctx, unit)
		details := subState
		if loadState != "" {
			details += " (load=" + loadState + ")"
		}
		// Unit definition drift detection. installed_state is authoritative.
		// Sidecar is consulted only as a one-time legacy migration input;
		// after migration, sidecar is never read again for this unit.
		state := activeState
		if d := checkUnitHashDrift(ctx, unit, unitToPkg[unit]); d != "" {
			details += " [" + d + "]"
			state = d
		}
		statuses = append(statuses, &node_agentpb.UnitStatus{
			Name:    unit,
			State:   state,
			Details: details,
		})
	}
	return statuses
}

// buildUnitToPackageMap returns a unit-name → installed-package lookup for
// every package on this node that records a unit_file_path in its receipt
// metadata, falling back to the `globular-<name>.service` convention for
// packages installed before receipts existed. The map's value is the
// authoritative installed_state record for that unit's expected sha and
// receipt provenance.
//
// Returns an empty (non-nil) map if installed_state is unreachable; the
// heartbeat must still produce UnitStatus entries even when etcd is
// temporarily unavailable, falling back to fail-closed semantics in
// checkUnitHashDrift.
func buildUnitToPackageMap(ctx context.Context, nodeID string) map[string]*node_agentpb.InstalledPackage {
	out := make(map[string]*node_agentpb.InstalledPackage)
	if nodeID == "" {
		return out
	}
	pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, "")
	if err != nil {
		return out
	}
	for _, pkg := range pkgs {
		if pkg.GetName() == "" {
			continue
		}
		// Prefer explicit unit_file_path from the receipt.
		if path := receiptUnitFilePath(pkg); path != "" {
			out[filepath.Base(path)] = pkg
			continue
		}
		// Fall back to the canonical naming convention for packages that
		// do not yet have a receipt-stamped unit_file_path. This handles
		// pre-refactor installs whose installed_state predates this code.
		unit := "globular-" + pkg.GetName() + ".service"
		out[unit] = pkg
	}
	return out
}

// checkUnitHashDrift inspects unitName's on-disk content against the
// authoritative expected sha. Returns one of:
//
//	""                                    no drift OR unmanaged unit
//	"unit_file_drift"                     authority disagrees with disk
//	"installed_state_missing_or_unproven" no authority anywhere — fail closed
//
// Authority resolution order:
//
//  1. installed_state.metadata.unit_file_sha256 wins always. After this
//     refactor, every install action stamps the receipt; absence after
//     fresh installs is itself a signal that the install path failed to
//     produce a receipt.
//  2. Legacy /etc/systemd/system/<unit>.sha256 sidecar — read ONCE as
//     a migration seed when installed_state metadata is absent. The
//     value is stamped into installed_state with migration_source=
//     legacy_sidecar; subsequent calls find the value in metadata and
//     never re-read the sidecar.
//  3. Neither installed_state nor sidecar present → fail closed.
//     Returning "installed_state_missing_or_unproven" makes the gap
//     observable to the doctor rather than silently downgrading to
//     "no opinion."
//
// pkg may be nil when no installed_state record exists for this unit
// (truly unmanaged unit such as keepalived/scylla-server). In that case
// the function returns "" — drift detection requires a managed
// receipt-bearing package to compare against.
func checkUnitHashDrift(ctx context.Context, unitName string, pkg *node_agentpb.InstalledPackage) string {
	unitPath := filepath.Join("/etc/systemd/system", unitName)
	current, err := os.ReadFile(unitPath)
	if err != nil {
		// Unit file missing on disk — runtime state handles this; no
		// drift signal needed.
		return ""
	}
	sum := sha256.Sum256(current)
	currentSha := hex.EncodeToString(sum[:])

	// Authority 1: installed_state receipt.
	if expected := receiptUnitFileSha256(pkg); expected != "" {
		if currentSha == strings.ToLower(strings.TrimSpace(expected)) {
			return ""
		}
		return "unit_file_drift"
	}

	// pkg is nil ⇒ no installed_state record for this unit. Truly
	// unmanaged (third-party unit, baseline unit without receipt yet).
	// Returning "" preserves the pre-refactor "no opinion" semantics for
	// units we cannot classify.
	if pkg == nil {
		return ""
	}

	// Authority 2: legacy sidecar migration.
	//
	// The pkg parameter is a snapshot pre-fetched by buildUnitToPackageMap
	// at the start of GetUnitStatus. If a canonical install commits a
	// fresh receipt between that snapshot and this code path, the snapshot
	// lacks the receipt and a blind migration write here would clobber a
	// freshly-stamped canonical receipt with the 4-key legacy_sidecar
	// shape — losing installed_by, binary_sha256, and the proof fields
	// every cycle. Live regression observed 2026-06-03 across installs
	// 1.2.147 → 1.2.150; see project_receipt_wipe_in_heartbeat.md.
	//
	// Re-read installed_state from etcd to validate the snapshot before
	// any write. If the fresh row carries any receipt provenance, the
	// snapshot is stale and migration MUST NOT proceed. Sidecar migration
	// is vestigial — sidecar files retired by docs/architecture/retire-
	// systemd-sidecars.md still linger on disk after pre-refactor installs,
	// but they cannot override a canonical receipt.
	fresh, _ := installed_state.GetInstalledPackage(ctx, pkg.GetNodeId(), pkg.GetKind(), pkg.GetName())
	if proceed, fallback := shouldMigrateFromSidecar(fresh, currentSha); !proceed {
		logMigrationDecision(unitName, "skip", fresh, currentSha, "")
		return fallback
	}

	sidecarPath := unitPath + ".sha256"
	sidecarData, err := os.ReadFile(sidecarPath)
	if err != nil {
		// Authority 3: fail closed. The doctor surfaces this so an
		// operator can run the deliberate repair action rather than
		// silently trusting filesystem evidence.
		return "installed_state_missing_or_unproven"
	}
	sidecarSha := strings.TrimSpace(string(sidecarData))
	if sidecarSha == "" {
		return "installed_state_missing_or_unproven"
	}
	// One-time migration write. Mutate the FRESH pkg (not the stale
	// snapshot) so any non-receipt fields that landed between snapshot
	// and now (entrypoint_checksum, proof_*, etc.) are preserved.
	stampMigrationFromLegacySidecar(fresh, unitPath, sidecarSha)
	logMigrationDecision(unitName, "apply", fresh, currentSha, sidecarSha)
	if werr := installed_state.WriteInstalledPackage(ctx, fresh); werr != nil {
		log.Printf("install_receipt: legacy-sidecar migration write failed for %s: %v (will retry next heartbeat)", unitName, werr)
		// Fall through and still classify drift this cycle so the
		// operator sees something rather than silence.
	}
	if currentSha == strings.ToLower(sidecarSha) {
		return ""
	}
	return "unit_file_drift"
}

// shouldMigrateFromSidecar is the pure decision half of the migration
// branch in checkUnitHashDrift. Given the FRESH etcd installed_state
// row and the on-disk unit sha, it returns:
//
//	proceed=true,  fallback=""               — migration may run
//	proceed=false, fallback=<verdict>        — skip migration; caller
//	                                            returns the verdict as
//	                                            the drift classification
//
// Migration is allowed ONLY when fresh etcd has no receipt provenance:
// no unit_file_sha256, no installed_by, no migration_source. Any of
// those signals a canonical or prior-migrated receipt that must not be
// overwritten by a 4-key legacy shape.
//
// fresh==nil (record disappeared between snapshot and now) is treated
// as "no opinion" — backing off avoids re-creating a stale record from
// stale snapshot evidence.
func shouldMigrateFromSidecar(fresh *node_agentpb.InstalledPackage, currentSha string) (proceed bool, fallback string) {
	if fresh == nil {
		return false, ""
	}
	md := fresh.GetMetadata()
	if expected := strings.TrimSpace(md[installreceipt.KeyUnitFileSha256]); expected != "" {
		// Fresh etcd already has the authoritative unit_file_sha256.
		// Classify drift against it instead of touching anything.
		if currentSha == strings.ToLower(expected) {
			return false, ""
		}
		return false, "unit_file_drift"
	}
	if strings.TrimSpace(md[installreceipt.KeyInstalledBy]) != "" {
		// Canonical install just landed but the unit_file_sha256 has
		// not been populated yet (rare transient — installer stamped
		// binary_sha256 but unit file was missing at stamp time).
		// Fail closed so the next cycle re-reads and converges.
		return false, "installed_state_missing_or_unproven"
	}
	if strings.TrimSpace(md[installreceipt.KeyMigrationSource]) != "" {
		// A previous migration cycle already stamped migration_source
		// for this unit. The migration write is idempotent only in the
		// "no canonical receipt arrived" world; once we are here, the
		// safe default is to NOT re-stamp and let the next heartbeat
		// surface drift via authority 1 once the canonical install
		// completes.
		return false, ""
	}
	return true, ""
}

// logMigrationDecision emits a single line per decision so an operator
// can trace whether a heartbeat tick wrote a migration receipt and why.
// Decision is "skip" (snapshot was stale, fresh had receipt) or "apply"
// (fresh had no receipt; sidecar consumed as last-resort seed).
func logMigrationDecision(unitName, decision string, fresh *node_agentpb.InstalledPackage, currentSha, sidecarSha string) {
	installedBy, migrationSource, expectedSha, name, version, buildID := "", "", "", "", "", ""
	if fresh != nil {
		md := fresh.GetMetadata()
		installedBy = md[installreceipt.KeyInstalledBy]
		migrationSource = md[installreceipt.KeyMigrationSource]
		expectedSha = md[installreceipt.KeyUnitFileSha256]
		name = fresh.GetName()
		version = fresh.GetVersion()
		buildID = fresh.GetBuildId()
	}
	log.Printf("install_receipt: migration %s for %s — name=%s version=%s build_id=%s fresh{installed_by=%q migration_source=%q unit_file_sha256=%s} disk=%s sidecar=%s",
		decision, unitName, name, version, buildID,
		installedBy, migrationSource, shortSha(expectedSha), shortSha(currentSha), shortSha(sidecarSha))
}

// shortSha truncates a sha256 hex string to the first 16 characters
// for log compactness. Empty input returns "" (not "????…").
func shortSha(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) > 16 {
		return s[:16] + "…"
	}
	return s
}

// queryUnitState returns the ActiveState, SubState, and LoadState of a systemd unit.
func queryUnitState(ctx context.Context, unit string) (activeState, subState, loadState string) {
	unitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(unitCtx, "systemctl", "show",
		"--property=ActiveState,SubState,LoadState", unit).Output()
	if err != nil {
		return "unknown", "", ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if k, v, ok := strings.Cut(line, "="); ok {
			switch k {
			case "ActiveState":
				activeState = v
			case "SubState":
				subState = v
			case "LoadState":
				loadState = v
			}
		}
	}
	if activeState == "" {
		activeState = "unknown"
	}
	return
}

// buildBootstrapPlan deleted — bootstrap uses workflow-native day0.bootstrap.

func (srv *NodeAgentServer) saveState() error {
	if srv.statePath == "" {
		return nil
	}
	srv.stateMu.Lock()
	defer srv.stateMu.Unlock()
	if srv.state == nil {
		srv.state = newNodeAgentState()
	}
	srv.state.ControllerEndpoint = srv.controllerEndpoint
	srv.state.ControllerInsecure = srv.useInsecure
	srv.state.RequestID = srv.joinRequestID
	srv.state.NodeID = srv.nodeID
	srv.state.JoinToken = strings.TrimSpace(srv.joinToken)
	srv.state.NetworkGeneration = srv.lastNetworkGeneration
	if domain, err := config.GetDomain(); err == nil && strings.TrimSpace(domain) != "" {
		srv.state.ClusterDomain = strings.TrimSpace(domain)
	}
	if localCfg, err := config.GetLocalConfig(true); err == nil {
		if v, ok := localCfg["Protocol"].(string); ok && strings.TrimSpace(v) != "" {
			srv.state.Protocol = strings.TrimSpace(strings.ToLower(v))
		}
	}
	return srv.state.save(srv.statePath)
}

func (srv *NodeAgentServer) startJoinApprovalWatcher(ctx context.Context, requestID string) {
	if requestID == "" {
		return
	}
	srv.joinPollMu.Lock()
	if srv.joinPollCancel != nil {
		srv.joinPollCancel()
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	srv.joinPollCancel = cancel
	srv.joinPollMu.Unlock()
	go srv.watchJoinStatus(ctx, requestID)
}

func (srv *NodeAgentServer) watchJoinStatus(ctx context.Context, requestID string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := srv.ensureControllerClient(ctx); err != nil {
			log.Printf("join status: controller unreachable: %v", err)
			if !waitOrDone(ctx, ticker) {
				return
			}
			continue
		}
		resp, err := srv.controllerClient.GetJoinRequestStatus(ctx, &cluster_controllerpb.GetJoinRequestStatusRequest{
			RequestId: requestID,
		})
		if err != nil {
			log.Printf("join status poll error: %v", err)
			if !waitOrDone(ctx, ticker) {
				return
			}
			continue
		}
		switch strings.ToLower(resp.GetStatus()) {
		case "approved":
			if nodeID := resp.GetNodeId(); nodeID != "" {
				srv.applyApprovedNodeID(nodeID)
				log.Printf("join request %s approved (node %s)", requestID, nodeID)
			}
			// Store node-scoped identity token if provided
			if token := resp.GetNodeToken(); token != "" {
				principal := resp.GetNodePrincipal()
				if err := srv.storeNodeToken(token, principal); err != nil {
					log.Printf("WARN: failed to store node token: %v", err)
				} else {
					log.Printf("node identity set: principal=%s", principal)
				}
			}
			return
		case "rejected":
			log.Printf("join request %s rejected: %s", requestID, resp.GetMessage())
			return
		}
		if !waitOrDone(ctx, ticker) {
			return
		}
	}
}

func waitOrDone(ctx context.Context, ticker *time.Ticker) bool {
	select {
	case <-ctx.Done():
		return false
	case <-ticker.C:
		return true
	}
}

func (srv *NodeAgentServer) joinRequestLabels() map[string]string {
	labels := make(map[string]string)
	for k, v := range srv.cfg.Labels {
		labels[k] = v
	}
	if mac, err := identity.SelectBestMAC(); err == nil && mac != "" {
		labels["node.mac"] = mac
	}
	if len(labels) == 0 {
		return nil
	}
	return labels
}

func (srv *NodeAgentServer) applyApprovedNodeID(nodeID string) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return
	}
	srv.stateMu.Lock()
	srv.nodeID = nodeID
	srv.state.NodeID = nodeID
	srv.state.RequestID = ""
	srv.state.JoinID = ""        // clear v2 join_id so auto-join doesn't re-fire on restart
	srv.state.JoinPlanJSON = nil // clear stored plan; no longer needed after approval
	srv.joinRequestID = ""
	srv.joinToken = "" // clear so auto-join doesn't re-fire on restart
	srv.stateMu.Unlock()
	if err := srv.saveState(); err != nil {
		log.Printf("warn: persist approved node id: %v", err)
	}
	// Now that we have a node ID, immediately sync installed packages to etcd.
	go srv.syncInstalledStateToEtcd(context.Background())
}

func (srv *NodeAgentServer) storeNodeToken(token, principal string) error {
	tokenDir := "/var/lib/globular/tokens"
	if err := os.MkdirAll(tokenDir, 0750); err != nil {
		return fmt.Errorf("create token dir: %w", err)
	}
	tokenPath := filepath.Join(tokenDir, "node_token")
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		return fmt.Errorf("write node token: %w", err)
	}
	principalPath := filepath.Join(tokenDir, "node_principal")
	if err := os.WriteFile(principalPath, []byte(principal), 0600); err != nil {
		return fmt.Errorf("write node principal: %w", err)
	}
	return nil
}

func parseNodeAgentLabels(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	labels := make(map[string]string)
	pairs := strings.Split(raw, ",")
	for _, pair := range pairs {
		if pair = strings.TrimSpace(pair); pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			continue
		}
		labels[key] = value
	}
	if len(labels) == 0 {
		return nil
	}
	return labels
}

func convertNodeAgentUnits(units []*node_agentpb.UnitStatus) []*cluster_controllerpb.NodeUnitStatus {
	if len(units) == 0 {
		return nil
	}
	out := make([]*cluster_controllerpb.NodeUnitStatus, 0, len(units))
	for _, unit := range units {
		if unit == nil {
			continue
		}
		out = append(out, &cluster_controllerpb.NodeUnitStatus{
			Name:    unit.GetName(),
			State:   unit.GetState(),
			Details: unit.GetDetails(),
		})
	}
	return out
}
