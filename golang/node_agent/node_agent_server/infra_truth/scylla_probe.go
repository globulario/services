package infra_truth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// Default native-API endpoints and timeouts for ScyllaDB. The probe is bounded
// so it can never become a new availability risk.
const (
	defaultScyllaRESTBase   = "http://127.0.0.1:10000"
	defaultScyllaCQLAddr    = "127.0.0.1:9042"
	defaultComponentTimeout = 2 * time.Second        // overall budget per probe
	defaultSubTimeout       = 800 * time.Millisecond // per native-API call
)

// ScyllaProber probes one local ScyllaDB instance. Use NewScyllaProber for
// production defaults; the injection points exist so tests can run without a
// live daemon.
type ScyllaProber struct {
	ConfigPath       string
	RESTBase         string
	CQLAddr          string
	ComponentTimeout time.Duration
	SubTimeout       time.Duration
	EnableCQL        bool

	// Injection points — nil means use the production default.
	DetectInstalled func(ctx context.Context) bool
	UnitActive      func(ctx context.Context) bool
	HTTPClient      *http.Client
	NowUnix         func() int64
}

// NewScyllaProber returns a prober with production defaults. CQL probing is on by
// default but is separately timeout-bounded so a hung CQL port cannot stall the
// whole probe.
func NewScyllaProber() *ScyllaProber {
	return &ScyllaProber{
		ConfigPath:       ScyllaConfigPath,
		RESTBase:         defaultScyllaRESTBase,
		CQLAddr:          defaultScyllaCQLAddr,
		ComponentTimeout: defaultComponentTimeout,
		SubTimeout:       defaultSubTimeout,
		EnableCQL:        true,
	}
}

func (p *ScyllaProber) now() int64 {
	if p.NowUnix != nil {
		return p.NowUnix()
	}
	return time.Now().Unix()
}

func (p *ScyllaProber) httpClient() *http.Client {
	if p.HTTPClient != nil {
		return p.HTTPClient
	}
	return &http.Client{Timeout: p.SubTimeout}
}

// detectInstalled reports whether ScyllaDB is installed on this node. Default:
// the config file, the systemd unit, or the binary exists.
func (p *ScyllaProber) detectInstalled(ctx context.Context) bool {
	if p.DetectInstalled != nil {
		return p.DetectInstalled(ctx)
	}
	candidates := []string{
		p.ConfigPath,
		"/etc/systemd/system/scylla-server.service",
		"/lib/systemd/system/scylla-server.service",
		"/usr/bin/scylla",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return true
		}
	}
	return false
}

// unitActive reports whether scylla-server.service is active (systemctl is-active).
func (p *ScyllaProber) unitActive(ctx context.Context) bool {
	if p.UnitActive != nil {
		return p.UnitActive(ctx)
	}
	return exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", "scylla-server").Run() == nil
}

// ProbeStructured runs the full layered probe and assembles the InfraProbeResult.
// It NEVER fails the whole probe because a native-API call failed — partial
// failures land in result.errors with daemon/api readiness flags telling the
// truth. desired may be nil with a non-nil desiredErr (desired state could not
// be built) — that becomes an explicit infra.desired_state_unavailable violation.
func (p *ScyllaProber) ProbeStructured(ctx context.Context, desired *InfraDesiredState, desiredErr error) *cluster_controllerpb.InfraProbeResult {
	start := time.Now()
	nodeID := ""
	if desired != nil {
		nodeID = desired.NodeID
	}
	res := &cluster_controllerpb.InfraProbeResult{
		Component:    ComponentScylla,
		NodeId:       nodeID,
		ProbedAtUnix: p.now(),
		Desired:      map[string]string{},
		Rendered:     map[string]string{},
		Runtime:      map[string]string{},
	}
	if desired != nil {
		res.Desired = desired.desiredMap()
		res.ExpectedPeers = desired.ExpectedPeers
	}

	// Overall budget for the probe.
	ctx, cancel := context.WithTimeout(ctx, p.ComponentTimeout)
	defer cancel()

	// Layer 0: installed?
	res.Installed = p.detectInstalled(ctx)
	if !res.Installed {
		res.Lifecycle = deriveScyllaLifecycle(false, desired, nil, nil, nil, p.now())
		res.Summary = "ScyllaDB is not installed on this node"
		res.ConfigValid = false
		res.Healthy = false
		res.ProbeDurationMs = time.Since(start).Milliseconds()
		return res
	}

	// Desired-state unavailable is an explicit violation, never a silent skip.
	var violations []*cluster_controllerpb.InfraViolation
	if desiredErr != nil {
		violations = append(violations, newViolation(
			"infra.desired_state_unavailable",
			SeverityError,
			fmt.Sprintf("could not compute desired state for ScyllaDB: %v", desiredErr),
			desiredErr.Error(),
			"Ensure the node-agent can read cluster membership (node id, local IP, seeds) from etcd; desired state drives every attestation.",
		))
		res.Desired["source"] = SourceDesiredStateUnavailable
	}

	// Layer 1: rendered config.
	rendered, err := parseScyllaYAML(p.ConfigPath)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
	}
	res.Rendered = rendered.renderedMap()

	// Layer 2: attestation.
	violations = append(violations, AttestScyllaConfig(desired, rendered)...)
	res.Violations = violations
	res.ConfigValid = rendered.Present && !hasSeverity(violations, SeverityCritical) && !hasSeverity(violations, SeverityError)

	// Layer 3: runtime truth (best effort, bounded).
	runtime := p.probeRuntime(ctx)
	res.DaemonActive = runtime.DaemonActive
	res.Runtime = runtimeMap(runtime)
	res.ObservedPeers = runtime.ObservedPeers
	res.Errors = append(res.Errors, runtime.Errors...)
	res.PeersMatch = peersMatch(desired, runtime.ObservedPeers)

	// Layer 4: lifecycle FSM.
	res.Lifecycle = deriveScyllaLifecycle(true, desired, rendered, runtime, violations, p.now())
	res.Healthy = res.Lifecycle.GetState() == cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY
	res.Summary = summarize(res)
	res.ProbeDurationMs = time.Since(start).Milliseconds()
	return res
}

// probeRuntime gathers native-API truth in layers. Each layer is independently
// bounded and its failure is recorded as evidence rather than aborting.
func (p *ScyllaProber) probeRuntime(ctx context.Context) *ScyllaRuntimeState {
	rt := &ScyllaRuntimeState{BootstrapProgress: -1, GossipLive: -1}

	rt.DaemonActive = p.unitActive(ctx)
	if !rt.DaemonActive {
		rt.Errors = append(rt.Errors, "scylla-server unit is not active")
		return rt // partial: nothing else is reachable
	}

	// REST: operation mode (also proves the local API is up).
	if mode, err := p.restString(ctx, "/storage_service/operation_mode"); err == nil {
		rt.RESTAPIReady = true
		rt.OperationMode = mode
	} else {
		rt.Errors = append(rt.Errors, fmt.Sprintf("rest operation_mode: %v", err))
	}

	// REST: live gossip endpoints → observed peers + gossip live count.
	if live, err := p.restStringSlice(ctx, "/gossiper/endpoint/live"); err == nil {
		rt.ObservedPeers = live
		rt.GossipLive = len(live)
	} else {
		rt.Errors = append(rt.Errors, fmt.Sprintf("rest gossiper/live: %v", err))
	}

	// REST: local host id (best effort).
	if hid, err := p.restString(ctx, "/storage_service/hostid/local"); err == nil {
		rt.HostID = hid
	}

	// CQL: optional, separately bounded TCP reachability check.
	if p.EnableCQL {
		if p.tcpReachable(ctx, p.CQLAddr) {
			rt.CQLReady = true
		} else {
			rt.Errors = append(rt.Errors, fmt.Sprintf("cql %s not reachable", p.CQLAddr))
		}
	}
	return rt
}

func (p *ScyllaProber) restBytes(ctx context.Context, path string) ([]byte, error) {
	sub, cancel := context.WithTimeout(ctx, p.SubTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(sub, http.MethodGet, p.RESTBase+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return body, nil
}

// restString fetches a JSON string scalar (ScyllaDB returns e.g. "NORMAL").
func (p *ScyllaProber) restString(ctx context.Context, path string) (string, error) {
	body, err := p.restBytes(ctx, path)
	if err != nil {
		return "", err
	}
	var s string
	if err := json.Unmarshal(body, &s); err == nil {
		return s, nil
	}
	// Fall back to the raw trimmed body for endpoints that return bare text.
	return stripQuotes(string(body)), nil
}

// restStringSlice fetches a JSON array of strings (e.g. live endpoints).
func (p *ScyllaProber) restStringSlice(ctx context.Context, path string) ([]string, error) {
	body, err := p.restBytes(ctx, path)
	if err != nil {
		return nil, err
	}
	var out []string
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("decode array: %w", err)
	}
	return out, nil
}

func (p *ScyllaProber) tcpReachable(ctx context.Context, addr string) bool {
	sub, cancel := context.WithTimeout(ctx, p.SubTimeout)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(sub, "tcp", addr)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// peersMatch reports whether the observed live peers cover every expected non-self
// peer. With no desired state we cannot judge, so return false.
func peersMatch(desired *InfraDesiredState, observed []string) bool {
	if desired == nil || len(desired.ExpectedPeers) == 0 {
		return false
	}
	self := ""
	if len(desired.ExpectedListenAddresses) > 0 {
		self = desired.ExpectedListenAddresses[0]
	}
	obs := map[string]bool{}
	for _, o := range observed {
		obs[stripQuotes(o)] = true
	}
	for _, exp := range desired.ExpectedPeers {
		exp = stripQuotes(exp)
		if exp == "" || exp == self {
			continue
		}
		if !obs[exp] {
			return false
		}
	}
	return true
}

func runtimeMap(rt *ScyllaRuntimeState) map[string]string {
	m := map[string]string{
		"daemon_active":  fmt.Sprintf("%t", rt.DaemonActive),
		"rest_api_ready": fmt.Sprintf("%t", rt.RESTAPIReady),
		"cql_ready":      fmt.Sprintf("%t", rt.CQLReady),
	}
	if rt.OperationMode != "" {
		m["operation_mode"] = rt.OperationMode
	}
	if rt.GossipLive >= 0 {
		m["gossip_live"] = fmt.Sprintf("%d", rt.GossipLive)
	}
	if len(rt.ObservedPeers) > 0 {
		m["observed_peers"] = strings.Join(rt.ObservedPeers, ",")
	}
	if rt.HostID != "" {
		m["host_id"] = rt.HostID
	}
	return m
}

func summarize(res *cluster_controllerpb.InfraProbeResult) string {
	state := "unknown"
	blocking := ""
	if res.Lifecycle != nil {
		state = res.Lifecycle.GetStateLabel()
		blocking = res.Lifecycle.GetBlockingReason()
	}
	switch {
	case !res.Installed:
		return "ScyllaDB not installed"
	case res.Healthy:
		return "ScyllaDB is a healthy cluster member"
	case blocking != "":
		return fmt.Sprintf("ScyllaDB lifecycle=%s: %s", state, blocking)
	default:
		return fmt.Sprintf("ScyllaDB lifecycle=%s", state)
	}
}
