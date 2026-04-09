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
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/healthchecks"
	"github.com/globulario/services/golang/node_agent/node_agent_server/identity"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/certs"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/units"
	"github.com/globulario/services/golang/workflow"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/pki"
	"google.golang.org/grpc"
)

var defaultPort = "11000"

var (
	restartCommand    = restartUnit
	systemctlLookPath = exec.LookPath
	networkPKIManager = func(opts pki.Options) pki.Manager {
		return pki.NewFileManager(opts)
	}
)

// NodeAgentConfig holds all bootstrap-time configuration for the node agent.
// Values come from CLI flags or the persisted state file — never from os.Getenv.
type NodeAgentConfig struct {
	Port                    string
	AdvertiseAddr           string
	AdvertiseIP             string
	ClusterMode             bool
	Insecure                bool
	ClusterDomain           string
	ControllerEndpoint      string
	NodeID                  string
	NodeName                string
	JoinToken               string
	BootstrapToken          string
	AgentVersion            string
	ControllerCAPath        string
	ControllerSNI           string
	ControllerUseSystemRoots bool
	Labels                  map[string]string
	Domain                  string // node domain for FQDN construction

	// DNS overrides (optional, for multi-NIC nodes)
	DNSIPv4 string
	DNSIPv6 string
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

	// Bootstrap-time config passed from CLI flags (no os.Getenv at runtime)
	cfg NodeAgentConfig
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

	// Determine cluster domain early for controller discovery (PR3)
	clusterDomain := cfg.ClusterDomain

	// Controller endpoint discovery (PR3: prefer DNS in cluster mode)
	controllerEndpoint := strings.TrimSpace(cfg.ControllerEndpoint)
	if controllerEndpoint == "" && clusterDomain != "" && clusterMode {
		// In cluster mode with domain configured, use DNS-based discovery.
		// Controller port resolved from etcd; fall back to default 12000.
		controllerPort := "12000"
		if addr := config.ResolveServiceAddr("cluster_controller.ClusterControllerService", ""); addr != "" {
			if _, p, err := net.SplitHostPort(addr); err == nil && p != "" {
				controllerPort = p
			}
		}
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
	state.AdvertiseIP = strings.Split(advertised, ":")[0]

	return &NodeAgentServer{
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
	httpPort := spec.GetPortHttp()
	httpsPort := spec.GetPortHttps()
	domain := strings.TrimSpace(spec.GetClusterDomain())

	// Use routable IP for health checks on services that bind to NodeIP.
	// Envoy admin is intentionally local-only (127.0.0.1:9901).
	// Gateway binds 0.0.0.0 so either IP works.
	nodeIP := nodeRoutableIP()

	checks := []healthchecks.Check{
		{
			Name:           "minio",
			URL:            fmt.Sprintf("http://%s:9000/minio/health/ready", nodeIP),
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
		},
		{
			Name:           "envoy-admin",
			URL:            "http://127.0.0.1:9901/ready",
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
		},
	}
	if strings.EqualFold(spec.GetProtocol(), "https") {
		checks = append(checks, healthchecks.Check{
			Name:           "gateway-https",
			URL:            fmt.Sprintf("https://%s:%d/health", nodeIP, httpsPort),
			ExpectedStatus: []int{200},
			Timeout:        3 * time.Second,
			InsecureTLS:    true,
			HostHeader:     domain,
		})
	} else {
		checks = append(checks, healthchecks.Check{
			Name:           "gateway-http",
			URL:            fmt.Sprintf("http://%s:%d/health", nodeIP, httpPort),
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
		dnsAddr := nodeRoutableIP() + ":53"
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
		{"etcd", nodeRoutableIP() + ":2379"},
		{"minio-tcp", nodeRoutableIP() + ":9000"},
		{"scylla", nodeRoutableIP() + ":9042"},
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

func gatherIPs() []string {
	type ifaceIP struct {
		ip    string
		wired bool
	}
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
	return &cluster_controllerpb.NodeIdentity{
		Hostname:      hostname,
		Domain:        domain,
		AdvertiseFqdn: advertiseFqdn,
		Ips:           gatherIPs(),
		Os:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		AgentVersion:  srv.agentVersion,
	}
}

func detectUnits(ctx context.Context) []*node_agentpb.UnitStatus {
	if ctx == nil {
		ctx = context.Background()
	}

	// Must-check baseline — infrastructure units that may not have the
	// globular-* prefix (e.g. scylla-server).
	baseline := []string{
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
		"scylla-server.service",
		"globular-monitoring.service",
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

	// Build sorted unit list for deterministic output.
	unitList := make([]string, 0, len(discovered))
	for u := range discovered {
		unitList = append(unitList, u)
	}
	sort.Strings(unitList)

	// Query rich state for each unit.
	statuses := make([]*node_agentpb.UnitStatus, 0, len(unitList))
	for _, unit := range unitList {
		activeState, subState, loadState := queryUnitState(ctx, unit)
		details := subState
		if loadState != "" {
			details += " (load=" + loadState + ")"
		}
		statuses = append(statuses, &node_agentpb.UnitStatus{
			Name:    unit,
			State:   activeState,
			Details: details,
		})
	}
	return statuses
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
	srv.state.RequestID = srv.joinRequestID
	srv.state.NodeID = srv.nodeID
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

