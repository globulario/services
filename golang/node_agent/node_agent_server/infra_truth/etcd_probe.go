package infra_truth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// Default unit and config locations for the Globular-managed etcd member.
const (
	etcdUnitName            = "globular-etcd"
	etcdDefaultDataDir      = "/var/lib/globular/etcd"
	defaultEtcdProbeTimeout = 2 * time.Second
)

// EtcdRuntimeState is the native-API observed truth for the local etcd member.
// Every field is best effort: a zero/empty field means "not observed", recorded
// in Errors. Partial failure yields evidence, never a failed probe.
type EtcdRuntimeState struct {
	DaemonActive   bool
	LocalReachable bool // local endpoint Status() answered

	HasLeader bool
	IsLeader  bool
	IsLearner bool // this member is a non-voting learner (still joining)

	MemberID    string
	LeaderID    string
	RaftTerm    uint64
	DBSizeBytes int64
	Version     string

	MemberCount   int
	ObservedPeers []string // cluster-facing hosts of all members
	Alarms        []string // e.g. "NOSPACE", "CORRUPT"

	Errors []string // partial-failure evidence — non-empty != whole probe failed
}

// EtcdRuntimeObserver observes the local etcd member over its native v3 API
// (member list, endpoint status, alarm list) using mTLS. It dials localClientURL
// — the address the daemon actually advertises, NOT a hardcoded loopback — so the
// probe observes the real member (honors
// infra.runtime_truth_must_be_observed_via_native_api). The production
// implementation lives in the node-agent (which owns the etcd TLS client);
// infra_truth stays free of the etcd client dependency. A nil observer means the
// runtime layer is unavailable and is reported as such.
type EtcdRuntimeObserver func(ctx context.Context, localClientURL string) *EtcdRuntimeState

// EtcdProber probes one local etcd instance. Use NewEtcdProber for production
// defaults; the injection points exist so tests can run without a live daemon.
type EtcdProber struct {
	ConfigPath       string
	ComponentTimeout time.Duration

	// Injection points — nil means use the production default (except Observe,
	// which has no default here because the etcd client lives in the node-agent).
	DetectInstalled func(ctx context.Context) bool
	UnitActive      func(ctx context.Context) bool
	Observe         EtcdRuntimeObserver
	NowUnix         func() int64
}

// NewEtcdProber returns a prober with production defaults. The caller MUST set
// Observe to the node-agent's etcd native-API observer; without it the runtime
// layer reports "not observed" rather than fabricating health.
func NewEtcdProber() *EtcdProber {
	return &EtcdProber{
		ConfigPath:       EtcdConfigPath,
		ComponentTimeout: defaultEtcdProbeTimeout,
	}
}

func (p *EtcdProber) now() int64 {
	if p.NowUnix != nil {
		return p.NowUnix()
	}
	return time.Now().Unix()
}

// detectInstalled reports whether etcd is installed on this node. Default: the
// config file, the systemd unit, the data dir, or the binary exists.
func (p *EtcdProber) detectInstalled(ctx context.Context) bool {
	if p.DetectInstalled != nil {
		return p.DetectInstalled(ctx)
	}
	candidates := []string{
		p.ConfigPath,
		"/etc/systemd/system/" + etcdUnitName + ".service",
		"/lib/systemd/system/" + etcdUnitName + ".service",
		etcdDefaultDataDir,
		"/usr/local/bin/etcd",
		"/usr/bin/etcd",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return true
		}
	}
	return false
}

// unitActive reports whether globular-etcd.service is active.
func (p *EtcdProber) unitActive(ctx context.Context) bool {
	if p.UnitActive != nil {
		return p.UnitActive(ctx)
	}
	return exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", etcdUnitName).Run() == nil
}

// localEtcdClientURL picks the URL the observer should dial for the locally
// running member: the advertised client URL (what the daemon binds), falling
// back to the first listen-client URL. Empty when no client URL is configured.
func localEtcdClientURL(rendered *EtcdRenderedConfig) string {
	if rendered == nil {
		return ""
	}
	if len(rendered.AdvertiseClientURLs) > 0 {
		return rendered.AdvertiseClientURLs[0]
	}
	if len(rendered.ListenClientURLs) > 0 {
		return rendered.ListenClientURLs[0]
	}
	return ""
}

// ProbeStructured runs the full layered probe and assembles the InfraProbeResult.
// It NEVER fails the whole probe because a native-API call failed — partial
// failures land in result.errors with daemon/reachability flags telling the
// truth. desired may be nil with a non-nil desiredErr (desired state could not
// be built) — that becomes an explicit infra.desired_state_unavailable violation.
func (p *EtcdProber) ProbeStructured(ctx context.Context, desired *InfraDesiredState, desiredErr error) *cluster_controllerpb.InfraProbeResult {
	start := time.Now()
	nodeID := ""
	if desired != nil {
		nodeID = desired.NodeID
	}
	res := &cluster_controllerpb.InfraProbeResult{
		Component:    ComponentEtcd,
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
	ctx, cancel := context.WithTimeout(ctx, p.componentTimeout())
	defer cancel()

	// Layer 0: installed?
	res.Installed = p.detectInstalled(ctx)
	if !res.Installed {
		res.Lifecycle = deriveEtcdLifecycle(false, desired, nil, nil, nil, p.now())
		res.Summary = "etcd is not installed on this node"
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
			fmt.Sprintf("could not compute desired state for etcd: %v", desiredErr),
			desiredErr.Error(),
			"Ensure the node-agent can read cluster membership (node id, local IP, etcd endpoints) from local config/etcd; desired state drives every attestation.",
		))
		res.Desired["source"] = SourceDesiredStateUnavailable
	}

	// Layer 1: rendered config.
	rendered, err := parseEtcdYAML(p.ConfigPath)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
	}
	res.Rendered = rendered.renderedMap()

	// Layer 3: runtime truth (best effort, bounded). Observe the member at the
	// address it actually advertises, not a hardcoded loopback. Gathered BEFORE
	// attestation so config rules whose predicted harm is an empirical runtime fact
	// (e.g. a self-only initial-cluster "will isolate a joining member") can be
	// reconciled against the observed quorum instead of firing a false positive on
	// an already-established member.
	runtime := p.probeRuntime(ctx, rendered)
	res.DaemonActive = runtime.DaemonActive
	res.Runtime = etcdRuntimeMap(runtime)
	res.ObservedPeers = runtime.ObservedPeers
	res.Errors = append(res.Errors, runtime.Errors...)
	res.PeersMatch = peersMatch(desired, runtime.ObservedPeers)

	// Layer 2: attestation (runtime-aware where a rule's harm is empirical).
	violations = append(violations, AttestEtcdConfig(desired, rendered, runtime)...)
	res.Violations = violations
	res.ConfigValid = rendered.Present && !hasSeverity(violations, SeverityCritical) && !hasSeverity(violations, SeverityError)

	// Layer 4: lifecycle FSM.
	res.Lifecycle = deriveEtcdLifecycle(true, desired, rendered, runtime, violations, p.now())
	res.Healthy = res.Lifecycle.GetState() == cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY
	res.Summary = etcdSummarize(res)
	res.ProbeDurationMs = time.Since(start).Milliseconds()
	return res
}

func (p *EtcdProber) componentTimeout() time.Duration {
	if p.ComponentTimeout > 0 {
		return p.ComponentTimeout
	}
	return defaultEtcdProbeTimeout
}

// probeRuntime gathers native-API truth. The unit must be active before the
// observer is consulted; a nil observer (etcd client unavailable) is recorded as
// evidence, never silently treated as "no peers".
func (p *EtcdProber) probeRuntime(ctx context.Context, rendered *EtcdRenderedConfig) *EtcdRuntimeState {
	active := p.unitActive(ctx)
	if !active {
		return &EtcdRuntimeState{DaemonActive: false, Errors: []string{etcdUnitName + " unit is not active"}}
	}
	if p.Observe == nil {
		return &EtcdRuntimeState{DaemonActive: true, Errors: []string{"etcd native-API observer not configured — runtime truth unavailable"}}
	}
	localURL := localEtcdClientURL(rendered)
	if localURL == "" {
		return &EtcdRuntimeState{DaemonActive: true, Errors: []string{"no client URL in rendered config to observe — cannot reach the local member"}}
	}
	rt := p.Observe(ctx, localURL)
	if rt == nil {
		return &EtcdRuntimeState{DaemonActive: true, Errors: []string{"etcd observer returned no runtime state"}}
	}
	rt.DaemonActive = true
	return rt
}

func etcdRuntimeMap(rt *EtcdRuntimeState) map[string]string {
	m := map[string]string{
		"daemon_active":   fmt.Sprintf("%t", rt.DaemonActive),
		"local_reachable": fmt.Sprintf("%t", rt.LocalReachable),
		"has_leader":      fmt.Sprintf("%t", rt.HasLeader),
		"is_leader":       fmt.Sprintf("%t", rt.IsLeader),
		"is_learner":      fmt.Sprintf("%t", rt.IsLearner),
	}
	if rt.MemberID != "" {
		m["member_id"] = rt.MemberID
	}
	if rt.LeaderID != "" {
		m["leader_id"] = rt.LeaderID
	}
	if rt.RaftTerm > 0 {
		m["raft_term"] = fmt.Sprintf("%d", rt.RaftTerm)
	}
	if rt.DBSizeBytes > 0 {
		m["db_size_bytes"] = fmt.Sprintf("%d", rt.DBSizeBytes)
	}
	if rt.Version != "" {
		m["version"] = rt.Version
	}
	if rt.MemberCount > 0 {
		m["member_count"] = fmt.Sprintf("%d", rt.MemberCount)
	}
	if len(rt.ObservedPeers) > 0 {
		m["observed_peers"] = strings.Join(rt.ObservedPeers, ",")
	}
	if len(rt.Alarms) > 0 {
		m["alarms"] = strings.Join(rt.Alarms, ",")
	}
	return m
}

func etcdSummarize(res *cluster_controllerpb.InfraProbeResult) string {
	state := "unknown"
	blocking := ""
	if res.Lifecycle != nil {
		state = res.Lifecycle.GetStateLabel()
		blocking = res.Lifecycle.GetBlockingReason()
	}
	switch {
	case !res.Installed:
		return "etcd not installed"
	case res.Healthy:
		return "etcd is a healthy cluster member"
	case blocking != "":
		return fmt.Sprintf("etcd lifecycle=%s: %s", state, blocking)
	default:
		return fmt.Sprintf("etcd lifecycle=%s", state)
	}
}
