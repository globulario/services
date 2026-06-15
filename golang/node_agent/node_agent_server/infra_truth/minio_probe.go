package infra_truth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// Default unit, port, and config locations for the Globular-managed MinIO member.
const (
	minioUnitName            = "globular-minio"
	minioS3Port              = 9000 // protocol-standard MinIO S3/HTTPS port
	defaultMinioProbeTimeout = 2 * time.Second
)

// MinioRuntimeState is the native-API observed truth for the local MinIO server,
// gathered from the unauthenticated health endpoints (no root credentials are
// handled by the probe). Every field is best effort: a zero/empty field means
// "not observed", recorded in Errors.
type MinioRuntimeState struct {
	DaemonActive bool

	Live        bool // GET /minio/health/live answered 200
	WriteQuorum bool // GET /minio/health/cluster answered 200 (pool has write quorum)
	ReadQuorum  bool // GET /minio/health/cluster/read answered 200 (pool has read quorum)

	Errors []string // partial-failure evidence — non-empty != whole probe failed
}

// MinioRuntimeObserver observes the local MinIO server over its native HTTPS
// health API. It dials healthBaseURL ("https://<node-ip>:9000") — the address the
// daemon actually serves, NOT a hardcoded loopback (MinIO's TLS cert covers the
// node IP, so a loopback dial would also fail cert verification). It is
// credential-free: only the unauthenticated /minio/health/* endpoints are used.
// The production implementation lives in the node-agent (which owns the cluster
// CA TLS client); infra_truth stays free of the HTTP/TLS-to-MinIO dependency.
type MinioRuntimeObserver func(ctx context.Context, healthBaseURL string) *MinioRuntimeState

// MinioProber probes one local MinIO instance. Use NewMinioProber for production
// defaults; the injection points exist so tests can run without a live daemon.
type MinioProber struct {
	ConfigPath       string
	ComponentTimeout time.Duration

	// Injection points — nil means use the production default (except Observe,
	// which has no default here because the HTTP/TLS client lives in the node-agent).
	DetectInstalled func(ctx context.Context) bool
	UnitActive      func(ctx context.Context) bool
	Observe         MinioRuntimeObserver
	NowUnix         func() int64
}

// NewMinioProber returns a prober with production defaults. The caller MUST set
// Observe to the node-agent's MinIO health observer; without it the runtime layer
// reports "not observed" rather than fabricating health.
func NewMinioProber() *MinioProber {
	return &MinioProber{
		ConfigPath:       MinioConfigPath,
		ComponentTimeout: defaultMinioProbeTimeout,
	}
}

func (p *MinioProber) now() int64 {
	if p.NowUnix != nil {
		return p.NowUnix()
	}
	return time.Now().Unix()
}

func (p *MinioProber) componentTimeout() time.Duration {
	if p.ComponentTimeout > 0 {
		return p.ComponentTimeout
	}
	return defaultMinioProbeTimeout
}

// detectInstalled reports whether MinIO is installed on this node. Default: the
// env file, the systemd unit, or the binary exists.
func (p *MinioProber) detectInstalled(ctx context.Context) bool {
	if p.DetectInstalled != nil {
		return p.DetectInstalled(ctx)
	}
	candidates := []string{
		p.ConfigPath,
		"/etc/systemd/system/" + minioUnitName + ".service",
		"/lib/systemd/system/" + minioUnitName + ".service",
		"/usr/local/bin/minio",
		"/usr/bin/minio",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return true
		}
	}
	return false
}

// unitActive reports whether globular-minio.service is active.
func (p *MinioProber) unitActive(ctx context.Context) bool {
	if p.UnitActive != nil {
		return p.UnitActive(ctx)
	}
	return exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", minioUnitName).Run() == nil
}

// minioHealthBaseURL builds the local health base URL from desired state (the
// node's own cluster-facing address). Empty when the local address is unknown.
func minioHealthBaseURL(desired *MinioDesired) string {
	if desired == nil || len(desired.ExpectedListenAddresses) == 0 {
		return ""
	}
	ip := stripQuotes(desired.ExpectedListenAddresses[0])
	if ip == "" {
		return ""
	}
	return fmt.Sprintf("https://%s:%d", ip, minioS3Port)
}

// ProbeStructured runs the full layered probe and assembles the InfraProbeResult.
// It NEVER fails the whole probe because a native-API call failed — partial
// failures land in result.errors. desired may be nil with a non-nil desiredErr
// (desired state could not be built) — that becomes an explicit
// infra.desired_state_unavailable violation.
func (p *MinioProber) ProbeStructured(ctx context.Context, desired *MinioDesired, desiredErr error) *cluster_controllerpb.InfraProbeResult {
	start := time.Now()
	nodeID := ""
	if desired != nil {
		nodeID = desired.NodeID
	}
	res := &cluster_controllerpb.InfraProbeResult{
		Component:    ComponentMinio,
		NodeId:       nodeID,
		ProbedAtUnix: p.now(),
		Desired:      map[string]string{},
		Rendered:     map[string]string{},
		Runtime:      map[string]string{},
	}
	if desired != nil {
		res.Desired = desired.minioDesiredMap()
		res.ExpectedPeers = desired.ExpectedPeers
	}

	ctx, cancel := context.WithTimeout(ctx, p.componentTimeout())
	defer cancel()

	// Layer 0: installed?
	res.Installed = p.detectInstalled(ctx)
	if !res.Installed {
		res.Lifecycle = deriveMinioLifecycle(false, desired, nil, nil, nil, p.now())
		res.Summary = "MinIO is not installed on this node"
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
			fmt.Sprintf("could not compute desired state for MinIO: %v", desiredErr),
			desiredErr.Error(),
			"Ensure the node-agent can read the ObjectStoreDesiredState (mode, pool nodes, drives_per_node) from etcd; desired state drives every attestation.",
		))
		res.Desired["source"] = SourceDesiredStateUnavailable
	}

	// Layer 1: rendered config.
	rendered, err := parseMinioEnv(p.ConfigPath)
	if err != nil {
		res.Errors = append(res.Errors, err.Error())
	}
	res.Rendered = rendered.renderedMap()

	// Layer 2: attestation.
	violations = append(violations, AttestMinioConfig(desired, rendered)...)
	res.Violations = violations
	res.ConfigValid = rendered.Present && !hasSeverity(violations, SeverityCritical) && !hasSeverity(violations, SeverityError)

	// Layer 3: runtime truth (best effort, bounded).
	runtime := p.probeRuntime(ctx, desired)
	res.DaemonActive = runtime.DaemonActive
	res.Runtime = minioRuntimeMap(runtime)
	res.Errors = append(res.Errors, runtime.Errors...)

	// Layer 4: lifecycle FSM.
	res.Lifecycle = deriveMinioLifecycle(true, desired, rendered, runtime, violations, p.now())
	res.Healthy = res.Lifecycle.GetState() == cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY
	res.Summary = minioSummarize(res)
	res.ProbeDurationMs = time.Since(start).Milliseconds()
	return res
}

// probeRuntime gathers native-API truth. The unit must be active before the
// observer is consulted; a nil observer or an unknown local address is recorded
// as evidence, never silently treated as "healthy".
func (p *MinioProber) probeRuntime(ctx context.Context, desired *MinioDesired) *MinioRuntimeState {
	if !p.unitActive(ctx) {
		return &MinioRuntimeState{DaemonActive: false, Errors: []string{minioUnitName + " unit is not active"}}
	}
	if p.Observe == nil {
		return &MinioRuntimeState{DaemonActive: true, Errors: []string{"MinIO health observer not configured — runtime truth unavailable"}}
	}
	base := minioHealthBaseURL(desired)
	if base == "" {
		return &MinioRuntimeState{DaemonActive: true, Errors: []string{"local MinIO address unknown (desired state unavailable) — cannot reach the health endpoint"}}
	}
	rt := p.Observe(ctx, base)
	if rt == nil {
		return &MinioRuntimeState{DaemonActive: true, Errors: []string{"MinIO observer returned no runtime state"}}
	}
	rt.DaemonActive = true
	return rt
}

func minioRuntimeMap(rt *MinioRuntimeState) map[string]string {
	return map[string]string{
		"daemon_active": fmt.Sprintf("%t", rt.DaemonActive),
		"live":          fmt.Sprintf("%t", rt.Live),
		"write_quorum":  fmt.Sprintf("%t", rt.WriteQuorum),
		"read_quorum":   fmt.Sprintf("%t", rt.ReadQuorum),
	}
}

func minioSummarize(res *cluster_controllerpb.InfraProbeResult) string {
	state := "unknown"
	blocking := ""
	if res.Lifecycle != nil {
		state = res.Lifecycle.GetStateLabel()
		blocking = res.Lifecycle.GetBlockingReason()
	}
	switch {
	case !res.Installed:
		return "MinIO not installed"
	case res.Healthy:
		return "MinIO is a healthy pool member with write quorum"
	case blocking != "":
		return fmt.Sprintf("MinIO lifecycle=%s: %s", state, blocking)
	default:
		return fmt.Sprintf("MinIO lifecycle=%s", state)
	}
}
