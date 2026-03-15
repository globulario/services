package main

import (
	"context"
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
	"github.com/globulario/services/golang/node_agent/node_agent_server/healthchecks"
	"github.com/globulario/services/golang/node_agent/node_agent_server/identity"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/certs"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/units"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/pki"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/globulario/services/golang/plan/store"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"
)

var defaultPort = "11000"

const defaultPlanPollInterval = 5 * time.Second
const planLockTTL = 30

var (
	restartCommand    = restartUnit
	systemctlLookPath = exec.LookPath
	networkPKIManager = func(opts pki.Options) pki.Manager {
		return pki.NewFileManager(opts)
	}
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
	etcdMode                 string
	controllerCAPath         string
	controllerSNI            string
	controllerUseSystemRoots bool
	lastNetworkGeneration    uint64
	planStore                store.PlanStore
	planPollInterval         time.Duration
	lastPlanGeneration       uint64
	planRunnerCtx            context.Context
	planRunnerOnce           sync.Once
	controllerDialer         func(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error)
	controllerClientFactory  func(conn grpc.ClientConnInterface) cluster_controllerpb.ClusterControllerServiceClient
	controllerClientOverride func(addr string) cluster_controllerpb.ClusterControllerServiceClient
	lockAcquirer             func(context.Context, *planpb.NodePlan) (*planLockGuard, error)

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

	// Plan verification (Phase 1B)
	signerCache      map[string]signerCacheEntry
	signerCacheMu    sync.RWMutex
	rejectionTracker *planRejectionTracker
	lastSeenPlanID   string // for quarantine clearing
}

type lockablePlanStore interface {
	store.PlanStore
	Client() *clientv3.Client
}

type planLockGuard struct {
	client   *clientv3.Client
	leaseID  clientv3.LeaseID
	cancel   context.CancelFunc
	nodeID   string
	lockKeys []string
}

func (g *planLockGuard) release(ctx context.Context) {
	if g == nil {
		return
	}
	if g.cancel != nil {
		g.cancel()
	}
	if g.client != nil && g.leaseID != 0 {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		g.client.Revoke(ctx, g.leaseID)
	}
}

func (g *planLockGuard) keepAliveLoop(ch <-chan *clientv3.LeaseKeepAliveResponse) {
	if ch == nil {
		return
	}
	for range ch {
	}
}

func planLockKey(nodeID, lock string) string {
	return fmt.Sprintf("%s/%s/%s", store.PlanLockBaseKey, nodeID, lock)
}

func isTerminalState(state planpb.PlanState) bool {
	switch state {
	case planpb.PlanState_PLAN_SUCCEEDED, planpb.PlanState_PLAN_FAILED, planpb.PlanState_PLAN_ROLLED_BACK, planpb.PlanState_PLAN_EXPIRED, planpb.PlanState_PLAN_REJECTED, planpb.PlanState_PLAN_QUARANTINED:
		return true
	default:
		return false
	}
}

func NewNodeAgentServer(statePath string, state *nodeAgentState) *NodeAgentServer {
	if state == nil {
		state = newNodeAgentState()
	}
	port := getEnv("NODE_AGENT_PORT", defaultPort)
	advertised := strings.TrimSpace(os.Getenv("NODE_AGENT_ADVERTISE_ADDR"))
	clusterMode := getEnv("NODE_AGENT_CLUSTER_MODE", "true") != "false"

	if advertised == "" {
		// Determine advertise IP using validated selection
		advertiseIP, err := identity.SelectAdvertiseIP(os.Getenv("NODE_AGENT_ADVERTISE_IP"))
		if err != nil {
			if clusterMode {
				// In cluster mode, FAIL FAST if no valid IP
				log.Fatalf("node-agent: cannot determine advertise IP in cluster mode: %v", err)
			}
			// Development/single-node mode: allow localhost
			log.Printf("node-agent: warning: no advertise IP, using localhost (development mode)")
			advertiseIP = "127.0.0.1"
		}
		advertised = fmt.Sprintf("%s:%s", advertiseIP, port)
	}

	// Validate advertise endpoint
	if err := identity.ValidateAdvertiseEndpoint(advertised, clusterMode); err != nil {
		log.Fatalf("node-agent: invalid advertise endpoint: %v", err)
	}
	useInsecure := strings.EqualFold(getEnv("NODE_AGENT_INSECURE", "false"), "true")

	// Determine cluster domain early for controller discovery (PR3)
	clusterDomain := getEnv("CLUSTER_DOMAIN", "")

	// Controller endpoint discovery (PR3: prefer DNS in cluster mode)
	controllerEndpoint := strings.TrimSpace(os.Getenv("NODE_AGENT_CONTROLLER_ENDPOINT"))
	if controllerEndpoint == "" && clusterDomain != "" && clusterMode {
		// In cluster mode with domain configured, use DNS-based discovery
		controllerPort := getEnv("CLUSTER_CONTROLLER_PORT", "12000")
		controllerEndpoint = fmt.Sprintf("controller.%s:%s", clusterDomain, controllerPort)
		log.Printf("node-agent: using DNS-based controller discovery: %s", controllerEndpoint)
	}
	if controllerEndpoint == "" {
		controllerEndpoint = state.ControllerEndpoint
	} else {
		state.ControllerEndpoint = controllerEndpoint
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
		nodeID = strings.TrimSpace(os.Getenv("NODE_AGENT_NODE_ID"))
		state.NodeID = nodeID
	}

	// Ensure the node ID matches the hardware-derived stable ID.
	// After a restore, state.json may contain a stale random UUID from the backup.
	// Replace it with the deterministic stable ID so the node keeps its identity
	// across restores without needing to re-register via the join flow.
	if stableID, err := identity.StableNodeID(); err == nil {
		if nodeID != "" && nodeID != stableID {
			log.Printf("node-agent: stored node ID %s does not match stable ID %s (post-restore?); adopting stable ID", nodeID, stableID)
			nodeID = stableID
			state.NodeID = stableID
			state.RequestID = ""
		} else if nodeID == "" {
			log.Printf("node-agent: no node ID stored; using stable ID %s", stableID)
			nodeID = stableID
			state.NodeID = stableID
		}
	}

	// Node name selection (PR1)
	nodeName := getEnv("NODE_AGENT_NODE_NAME", "")
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
	state.AdvertiseIP = strings.Split(advertised, ":")[0]

	return &NodeAgentServer{
		operations:               make(map[string]*operation),
		joinToken:                strings.TrimSpace(os.Getenv("NODE_AGENT_JOIN_TOKEN")),
		bootstrapToken:           strings.TrimSpace(os.Getenv("NODE_AGENT_BOOTSTRAP_TOKEN")),
		controllerEndpoint:       controllerEndpoint,
		agentVersion:             getEnv("NODE_AGENT_VERSION", "0.0.1"),
		bootstrapPlan:            nil,
		nodeID:                   nodeID,
		statePath:                statePath,
		state:                    state,
		joinRequestID:            state.RequestID,
		advertisedAddr:           advertised,
		useInsecure:              useInsecure,
		etcdMode:                 "managed",
		controllerCAPath:         strings.TrimSpace(os.Getenv("NODE_AGENT_CONTROLLER_CA")),
		controllerSNI:            strings.TrimSpace(os.Getenv("NODE_AGENT_CONTROLLER_SNI")),
		controllerUseSystemRoots: strings.EqualFold(os.Getenv("NODE_AGENT_CONTROLLER_USE_SYSTEM_ROOTS"), "true"),
		planPollInterval:         defaultPlanPollInterval,
		lastPlanGeneration:       state.LastPlanGeneration,
		lastNetworkGeneration:    state.NetworkGeneration,
		controllerDialer:         grpc.DialContext,
		controllerClientFactory:  cluster_controllerpb.NewClusterControllerServiceClient,
		rejectionTracker:         newPlanRejectionTracker(),
		signerCache:              make(map[string]signerCacheEntry),
	}
}

func (srv *NodeAgentServer) SetEtcdMode(mode string) {
	if mode == "" {
		return
	}
	srv.etcdMode = strings.ToLower(strings.TrimSpace(mode))
}

func (srv *NodeAgentServer) SetPlanStore(ps store.PlanStore) {
	srv.planStore = ps
	if srv.planPollInterval <= 0 {
		srv.planPollInterval = defaultPlanPollInterval
	}
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
	plan := buildBootstrapPlan(srv.bootstrapPlan)
	if len(plan.GetUnitActions()) == 0 {
		return nil
	}
	op := srv.registerOperation("bootstrap plan", srv.bootstrapPlan)
	go srv.runPlan(ctx, op, plan)
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func buildHealthChecks(spec *cluster_controllerpb.ClusterNetworkSpec) []healthchecks.Check {
	if spec == nil {
		return nil
	}
	httpPort := spec.GetPortHttp()
	httpsPort := spec.GetPortHttps()
	domain := strings.TrimSpace(spec.GetClusterDomain())

	checks := []healthchecks.Check{
		{
			Name:           "minio",
			URL:            firstNonEmpty(strings.TrimSpace(os.Getenv("GLOBULAR_HEALTH_MINIO_URL")), "http://127.0.0.1:9000/minio/health/ready"),
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
		},
		{
			Name:           "envoy-admin",
			URL:            firstNonEmpty(strings.TrimSpace(os.Getenv("GLOBULAR_HEALTH_ENVOY_URL")), "http://127.0.0.1:9901/ready"),
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
		},
	}
	if strings.EqualFold(spec.GetProtocol(), "https") {
		checks = append(checks, healthchecks.Check{
			Name:           "gateway-https",
			URL:            firstNonEmpty(strings.TrimSpace(os.Getenv("GLOBULAR_HEALTH_GATEWAY_URL")), fmt.Sprintf("https://127.0.0.1:%d/health", httpsPort)),
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
			InsecureTLS:    true,
			HostHeader:     domain,
		})
	} else {
		checks = append(checks, healthchecks.Check{
			Name:           "gateway-http",
			URL:            firstNonEmpty(strings.TrimSpace(os.Getenv("GLOBULAR_HEALTH_GATEWAY_URL")), fmt.Sprintf("http://127.0.0.1:%d/health", httpPort)),
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
		dnsAddr := strings.TrimSpace(os.Getenv("GLOBULAR_DNS_UDP_ADDR"))
		if dnsAddr == "" {
			dnsAddr = "127.0.0.1:53"
		}
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

	addrs := []struct {
		name string
		addr string
	}{
		{"etcd", firstNonEmpty(os.Getenv("GLOBULAR_ETCD_ADDR"), "127.0.0.1:2379")},
		{"minio-tcp", firstNonEmpty(os.Getenv("GLOBULAR_MINIO_ADDR"), "127.0.0.1:9000")},
		{"scylla", firstNonEmpty(os.Getenv("GLOBULAR_SCYLLA_ADDR"), "127.0.0.1:9042")},
	}
	for _, a := range addrs {
		d := net.Dialer{Timeout: 3 * time.Second}
		conn, err := d.DialContext(ctx, "tcp", a.addr)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s dial %s: %v", a.name, a.addr, err))
			continue
		}
		conn.Close()
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

func gatherIPs() []string {
	var ips []string
	seen := make(map[string]struct{})
	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}
	for _, iface := range ifaces {
		// Skip down or loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
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
			seen[text] = struct{}{}
			ips = append(ips, text)
		}
	}

	// Sort IPs: prefer private network addresses (10.x, 172.16-31.x, 192.168.x) first
	sort.Slice(ips, func(i, j int) bool {
		ipI := net.ParseIP(ips[i])
		ipJ := net.ParseIP(ips[j])
		if ipI == nil || ipJ == nil {
			return ips[i] < ips[j]
		}

		privateI := isPrivateIP(ipI)
		privateJ := isPrivateIP(ipJ)

		// Private IPs come first
		if privateI != privateJ {
			return privateI
		}

		// Otherwise, sort by IP string
		return ips[i] < ips[j]
	})

	return ips
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

func buildNodeIdentity() *cluster_controllerpb.NodeIdentity {
	hostname, _ := os.Hostname()
	return &cluster_controllerpb.NodeIdentity{
		Hostname:     hostname,
		Domain:       os.Getenv("NODE_AGENT_DOMAIN"),
		Ips:          gatherIPs(),
		Os:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		AgentVersion: getEnv("NODE_AGENT_VERSION", "0.0.1"),
	}
}

func detectUnits(ctx context.Context) []*node_agentpb.UnitStatus {
	if ctx == nil {
		ctx = context.Background()
	}
	known := []string{
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
		"globular-event.service",
		"globular-rbac.service",
		"globular-file.service",
		"globular-minio.service",
		"globular-gateway.service",
		"globular-xds.service",
		"globular-envoy.service",
	}
	statuses := make([]*node_agentpb.UnitStatus, 0, len(known))
	for _, unit := range known {
		state := "unknown"
		details := ""
		unitCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		active, err := supervisor.IsActive(unitCtx, unit)
		cancel()
		if err != nil {
			details = err.Error()
		} else {
			if active {
				state = "active"
			} else {
				state = "inactive"
			}
			statusCtx, statusCancel := context.WithTimeout(ctx, 2*time.Second)
			if out, err := supervisor.Status(statusCtx, unit); err == nil {
				details = out
			} else if details == "" {
				details = err.Error()
			}
			statusCancel()
		}
		statuses = append(statuses, &node_agentpb.UnitStatus{
			Name:    unit,
			State:   state,
			Details: details,
		})
	}
	return statuses
}

func buildBootstrapPlan(services []string) *cluster_controllerpb.NodePlan {
	actions := make([]*cluster_controllerpb.UnitAction, 0, len(services))
	for _, svc := range services {
		unit := units.UnitForService(svc)
		if unit == "" {
			continue
		}
		actions = append(actions, &cluster_controllerpb.UnitAction{
			UnitName: unit,
			Action:   "start",
		})
	}
	if len(actions) == 0 {
		return &cluster_controllerpb.NodePlan{
			Profiles: []string{"bootstrap"},
		}
	}
	return &cluster_controllerpb.NodePlan{
		Profiles:    []string{"bootstrap"},
		UnitActions: actions,
	}
}

func (srv *NodeAgentServer) buildBootstrapPlanWithNetwork(profiles []string, clusterDomain string) *cluster_controllerpb.NodePlan {
	// Build unit actions from profiles
	plan := buildBootstrapPlan(profiles)
	plan.Profiles = append([]string(nil), profiles...)

	// Add network configuration if domain is provided
	domain := strings.TrimSpace(clusterDomain)
	if domain == "" {
		return plan
	}

	// Create default network spec for bootstrap
	spec := &cluster_controllerpb.ClusterNetworkSpec{
		ClusterDomain: domain,
		Protocol:      "https", // TLS is mandatory for all services
		PortHttp:      8080,
		PortHttps:     8443,
		AcmeEnabled:   false,
		AdminEmail:    "",
	}

	// Build rendered config
	rendered := make(map[string]string)

	// Add network spec snapshot
	if specJSON, err := protojson.Marshal(spec); err == nil {
		rendered["cluster.network.spec.json"] = string(specJSON)
	}

	// Add network overlay
	configPayload := map[string]interface{}{
		"Domain":    spec.ClusterDomain,
		"Protocol":  spec.Protocol,
		"PortHTTP":  spec.PortHttp,
		"PortHTTPS": spec.PortHttps,
	}
	if cfgJSON, err := json.MarshalIndent(configPayload, "", "  "); err == nil {
		rendered["/var/lib/globular/network.json"] = string(cfgJSON)
	}

	// Add network generation (bootstrap starts at 1)
	rendered["cluster.network.generation"] = "1"

	// Add restart units for network config
	restartUnits := []string{
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
		"globular-xds.service",
		"globular-envoy.service",
		"globular-gateway.service",
		"globular-minio.service",
	}
	if unitsJSON, err := json.Marshal(restartUnits); err == nil {
		rendered["reconcile.restart_units"] = string(unitsJSON)
	}

	plan.RenderedConfig = rendered
	return plan
}

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
	srv.state.RequestID = srv.joinRequestID
	srv.state.NodeID = srv.nodeID
	srv.state.LastPlanGeneration = srv.lastPlanGeneration
	srv.state.NetworkGeneration = srv.lastNetworkGeneration
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

func (srv *NodeAgentServer) applyApprovedNodeID(nodeID string) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return
	}
	srv.stateMu.Lock()
	srv.nodeID = nodeID
	srv.state.NodeID = nodeID
	srv.state.RequestID = ""
	srv.joinRequestID = ""
	srv.stateMu.Unlock()
	if err := srv.saveState(); err != nil {
		log.Printf("warn: persist approved node id: %v", err)
	}
	srv.startPlanRunnerLoop()

	// Now that we have a node ID, immediately sync installed packages to etcd.
	// On Day-0 the initial sync in heartbeatLoop runs before bootstrap assigns
	// the node ID, so this is the first opportunity to populate the registry.
	go srv.syncInstalledStateToEtcd(context.Background())
}

func parseNodeAgentLabels() map[string]string {
	raw := strings.TrimSpace(os.Getenv("NODE_AGENT_LABELS"))
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

func getEnv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
