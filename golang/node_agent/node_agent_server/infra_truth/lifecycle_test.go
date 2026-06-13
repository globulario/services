package infra_truth

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func memberRuntime() *ScyllaRuntimeState {
	return &ScyllaRuntimeState{
		DaemonActive:      true,
		RESTAPIReady:      true,
		CQLReady:          true,
		OperationMode:     "NORMAL",
		BootstrapProgress: 100,
		GossipLive:        2,
		ObservedPeers:     []string{"10.0.0.8", "10.0.0.20"},
	}
}

func TestDeriveScyllaLifecycle_NotInstalled(t *testing.T) {
	obs := deriveScyllaLifecycle(false, nil, nil, nil, nil, 100)
	if obs.GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_NOT_PRESENT {
		t.Fatalf("state=%v want NOT_PRESENT", obs.GetState())
	}
}

func TestDeriveScyllaLifecycle_ConfigInvalidStalled(t *testing.T) {
	r := validRendered()
	r.ListenAddress = "127.0.0.1" // critical loopback
	violations := AttestScyllaConfig(joiningDesired(), r)
	rt := memberRuntime() // daemon is active...
	obs := deriveScyllaLifecycle(true, joiningDesired(), r, rt, violations, 100)
	if obs.GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_STALLED {
		t.Fatalf("state=%v want STALLED (daemon active on a critically invalid config)", obs.GetState())
	}
	if obs.GetBlockingReason() == "" {
		t.Error("STALLED must carry a blocking_reason")
	}
}

func TestDeriveScyllaLifecycle_MemberReady(t *testing.T) {
	obs := deriveScyllaLifecycle(true, joiningDesired(), validRendered(), memberRuntime(), nil, 100)
	if obs.GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_MEMBER_READY {
		t.Fatalf("state=%v want MEMBER_READY", obs.GetState())
	}
}

func TestDeriveScyllaLifecycle_CQLReadyButNoPeersJoining(t *testing.T) {
	rt := memberRuntime()
	rt.ObservedPeers = nil
	rt.GossipLive = 0
	rt.BootstrapProgress = -1
	obs := deriveScyllaLifecycle(true, joiningDesired(), validRendered(), rt, nil, 100)
	if obs.GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_STALLED {
		t.Fatalf("state=%v want STALLED (CQL ready but isolated joining node)", obs.GetState())
	}
}

func TestDeriveScyllaLifecycle_ConfigNotRendered(t *testing.T) {
	obs := deriveScyllaLifecycle(true, joiningDesired(), &ScyllaRenderedConfig{Present: false}, &ScyllaRuntimeState{}, nil, 100)
	if obs.GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_PACKAGE_INSTALLED {
		t.Fatalf("state=%v want PACKAGE_INSTALLED", obs.GetState())
	}
}

func TestProbeScyllaStructured_NotInstalled(t *testing.T) {
	p := NewScyllaProber()
	p.DetectInstalled = func(ctx context.Context) bool { return false }
	p.NowUnix = func() int64 { return 100 }
	res := p.ProbeStructured(context.Background(), nil, nil)
	if res.GetInstalled() {
		t.Fatal("installed should be false")
	}
	if res.GetLifecycle().GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_NOT_PRESENT {
		t.Fatalf("lifecycle=%v want NOT_PRESENT", res.GetLifecycle().GetState())
	}
	if res.GetHealthy() {
		t.Fatal("not-installed must not be healthy")
	}
	if len(res.GetErrors()) != 0 {
		t.Fatalf("not-installed should report no probe errors, got %v", res.GetErrors())
	}
}

func TestProbeScyllaStructured_DesiredUnavailable(t *testing.T) {
	p := NewScyllaProber()
	p.DetectInstalled = func(ctx context.Context) bool { return true }
	p.UnitActive = func(ctx context.Context) bool { return false }
	p.ConfigPath = "/nonexistent/scylla.yaml"
	p.EnableCQL = false
	p.NowUnix = func() int64 { return 100 }
	res := p.ProbeStructured(context.Background(), nil, context.DeadlineExceeded)
	if !containsViolation(res.GetViolations(), "infra.desired_state_unavailable", SeverityError) {
		t.Fatalf("expected infra.desired_state_unavailable violation, got %+v", res.GetViolations())
	}
	if res.GetDesired()["source"] != SourceDesiredStateUnavailable {
		t.Errorf("desired source=%q", res.GetDesired()["source"])
	}
}

func TestProbeCache_HeartbeatUsesCachedResult(t *testing.T) {
	c := NewInfraProbeCache()
	base := time.Unix(1700000000, 0)
	r := &cluster_controllerpb.InfraProbeResult{Component: ComponentScylla, Healthy: true}
	c.Put(ComponentScylla, r, base)

	// Fresh read: not stale.
	fresh := c.Snapshot(base.Add(10*time.Second), 2*time.Minute)
	if len(fresh) != 1 {
		t.Fatalf("expected 1 cached result, got %d", len(fresh))
	}
	if fresh[0].GetProbeStale() {
		t.Error("10s-old entry must not be stale under a 2m threshold")
	}
	if fresh[0].GetProbeAgeSeconds() != 10 {
		t.Errorf("probe_age=%d want 10", fresh[0].GetProbeAgeSeconds())
	}

	// Aged read: stale and stamped.
	stale := c.Snapshot(base.Add(5*time.Minute), 2*time.Minute)
	if !stale[0].GetProbeStale() {
		t.Error("5m-old entry must be stale under a 2m threshold (never serve cache as live)")
	}
	if stale[0].GetProbeAgeSeconds() != 300 {
		t.Errorf("probe_age=%d want 300", stale[0].GetProbeAgeSeconds())
	}

	// Mutating the returned clone must not corrupt the cache.
	stale[0].Healthy = false
	again, _, _ := c.Get(ComponentScylla)
	if !again.GetHealthy() {
		t.Error("cache entry was mutated through a returned clone")
	}
}
