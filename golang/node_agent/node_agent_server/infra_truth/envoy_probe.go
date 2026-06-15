package infra_truth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// Default unit and timeout for the Globular-managed Envoy data plane.
const (
	envoyUnitName            = "globular-envoy"
	defaultEnvoyProbeTimeout = 2 * time.Second
)

// EnvoyRuntimeState is the native admin-API observed truth for the local Envoy
// data plane. Every field is best effort: a zero/empty field means "not
// observed", recorded in Errors.
type EnvoyRuntimeState struct {
	DaemonActive   bool
	AdminReachable bool   // the admin API answered at all
	Ready          bool   // GET /ready returned 200 (fully initialized)
	ServerState    string // LIVE | PRE_INITIALIZING | INITIALIZING | DRAINING
	Version        string

	// xDS handshake counters (from /stats). The LDS wedge is
	// CDSUpdateSuccess > 0 while LDSUpdateAttempt == 0.
	CDSUpdateSuccess  int64
	LDSUpdateAttempt  int64
	LDSUpdateSuccess  int64
	LDSUpdateRejected int64
	ActiveClusters    int64
	ActiveListeners   int64

	Errors []string
}

// EnvoyRuntimeObserver observes the local Envoy via its native admin API
// (/ready, /server_info, /stats) over plain HTTP on loopback (the admin interface
// is loopback by design — no TLS). adminBaseURL is taken from the rendered
// bootstrap. The production implementation lives in the node-agent; infra_truth
// stays free of the HTTP dependency.
type EnvoyRuntimeObserver func(ctx context.Context, adminBaseURL string) *EnvoyRuntimeState

// EnvoyProber probes one local Envoy instance. Use NewEnvoyProber for production
// defaults; the injection points exist so tests can run without a live daemon.
type EnvoyProber struct {
	ConfigPath       string
	ComponentTimeout time.Duration

	DetectInstalled func(ctx context.Context) bool
	UnitActive      func(ctx context.Context) bool
	Observe         EnvoyRuntimeObserver
	NowUnix         func() int64
}

// NewEnvoyProber returns a prober with production defaults. The caller MUST set
// Observe to the node-agent's Envoy admin observer; without it the runtime layer
// reports "not observed" rather than fabricating health.
func NewEnvoyProber() *EnvoyProber {
	return &EnvoyProber{
		ConfigPath:       EnvoyBootstrapPath,
		ComponentTimeout: defaultEnvoyProbeTimeout,
	}
}

func (p *EnvoyProber) now() int64 {
	if p.NowUnix != nil {
		return p.NowUnix()
	}
	return time.Now().Unix()
}

func (p *EnvoyProber) componentTimeout() time.Duration {
	if p.ComponentTimeout > 0 {
		return p.ComponentTimeout
	}
	return defaultEnvoyProbeTimeout
}

func (p *EnvoyProber) detectInstalled(ctx context.Context) bool {
	if p.DetectInstalled != nil {
		return p.DetectInstalled(ctx)
	}
	candidates := []string{
		"/etc/systemd/system/" + envoyUnitName + ".service",
		"/lib/systemd/system/" + envoyUnitName + ".service",
		"/usr/lib/globular/bin/envoy",
		"/usr/local/bin/envoy",
		"/usr/bin/envoy",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return true
		}
	}
	return false
}

func (p *EnvoyProber) unitActive(ctx context.Context) bool {
	if p.UnitActive != nil {
		return p.UnitActive(ctx)
	}
	return exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", envoyUnitName).Run() == nil
}

// ProbeStructured runs the full layered probe and assembles the InfraProbeResult.
// It NEVER fails the whole probe because a native-API call failed.
func (p *EnvoyProber) ProbeStructured(ctx context.Context, desired *InfraDesiredState, desiredErr error) *cluster_controllerpb.InfraProbeResult {
	start := time.Now()
	nodeID := ""
	if desired != nil {
		nodeID = desired.NodeID
	}
	res := &cluster_controllerpb.InfraProbeResult{
		Component:    ComponentEnvoy,
		NodeId:       nodeID,
		ProbedAtUnix: p.now(),
		Desired:      map[string]string{},
		Rendered:     map[string]string{},
		Runtime:      map[string]string{},
	}
	if desired != nil {
		res.Desired = desired.desiredMap()
	}

	ctx, cancel := context.WithTimeout(ctx, p.componentTimeout())
	defer cancel()

	// Layer 0: installed?
	res.Installed = p.detectInstalled(ctx)
	if !res.Installed {
		res.Lifecycle = deriveEnvoyLifecycle(false, nil, nil, nil, p.now())
		res.Summary = "Envoy is not installed on this node"
		res.ConfigValid = false
		res.Healthy = false
		res.ProbeDurationMs = time.Since(start).Milliseconds()
		return res
	}

	var violations []*cluster_controllerpb.InfraViolation
	if desiredErr != nil {
		violations = append(violations, newViolation(
			"infra.desired_state_unavailable",
			SeverityError,
			fmt.Sprintf("could not compute desired state for Envoy: %v", desiredErr),
			desiredErr.Error(),
			"Ensure the node-agent can read this node's identity (node id, local IP); desired state carries provenance for the attestation.",
		))
		res.Desired["source"] = SourceDesiredStateUnavailable
	}

	// Layer 1: rendered bootstrap.
	rendered, err := parseEnvoyBootstrap(p.ConfigPath)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
	}
	res.Rendered = rendered.renderedMap()

	// Layer 2: attestation.
	violations = append(violations, AttestEnvoyConfig(desired, rendered)...)
	res.Violations = violations
	res.ConfigValid = rendered.Present && !hasSeverity(violations, SeverityCritical) && !hasSeverity(violations, SeverityError)

	// Layer 3: runtime truth via the admin API (best effort, bounded).
	runtime := p.probeRuntime(ctx, rendered)
	res.DaemonActive = runtime.DaemonActive
	res.Runtime = envoyRuntimeMap(runtime)
	res.Errors = append(res.Errors, runtime.Errors...)

	// Layer 4: lifecycle FSM.
	res.Lifecycle = deriveEnvoyLifecycle(true, rendered, runtime, violations, p.now())
	res.Healthy = res.Lifecycle.GetState() == cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY
	res.Summary = envoySummarize(res)
	res.ProbeDurationMs = time.Since(start).Milliseconds()
	return res
}

func (p *EnvoyProber) probeRuntime(ctx context.Context, rendered *EnvoyRenderedConfig) *EnvoyRuntimeState {
	if !p.unitActive(ctx) {
		return &EnvoyRuntimeState{DaemonActive: false, Errors: []string{envoyUnitName + " unit is not active"}}
	}
	if p.Observe == nil {
		return &EnvoyRuntimeState{DaemonActive: true, Errors: []string{"Envoy admin observer not configured — runtime truth unavailable"}}
	}
	base := rendered.adminBaseURL()
	rt := p.Observe(ctx, base)
	if rt == nil {
		return &EnvoyRuntimeState{DaemonActive: true, Errors: []string{"Envoy observer returned no runtime state"}}
	}
	rt.DaemonActive = true
	return rt
}

func envoyRuntimeMap(rt *EnvoyRuntimeState) map[string]string {
	m := map[string]string{
		"daemon_active":      fmt.Sprintf("%t", rt.DaemonActive),
		"admin_reachable":    fmt.Sprintf("%t", rt.AdminReachable),
		"ready":              fmt.Sprintf("%t", rt.Ready),
		"cds_update_success": fmt.Sprintf("%d", rt.CDSUpdateSuccess),
		"lds_update_attempt": fmt.Sprintf("%d", rt.LDSUpdateAttempt),
		"lds_update_success": fmt.Sprintf("%d", rt.LDSUpdateSuccess),
		"active_listeners":   fmt.Sprintf("%d", rt.ActiveListeners),
		"active_clusters":    fmt.Sprintf("%d", rt.ActiveClusters),
	}
	if rt.ServerState != "" {
		m["server_state"] = rt.ServerState
	}
	if rt.Version != "" {
		m["version"] = rt.Version
	}
	if rt.LDSUpdateRejected > 0 {
		m["lds_update_rejected"] = fmt.Sprintf("%d", rt.LDSUpdateRejected)
	}
	return m
}

func envoySummarize(res *cluster_controllerpb.InfraProbeResult) string {
	state := "unknown"
	blocking := ""
	if res.Lifecycle != nil {
		state = res.Lifecycle.GetStateLabel()
		blocking = res.Lifecycle.GetBlockingReason()
	}
	switch {
	case !res.Installed:
		return "Envoy not installed"
	case res.Healthy:
		return "Envoy data plane is serving — xDS connected, listeners active"
	case blocking != "":
		return fmt.Sprintf("Envoy lifecycle=%s: %s", state, blocking)
	default:
		return fmt.Sprintf("Envoy lifecycle=%s", state)
	}
}
