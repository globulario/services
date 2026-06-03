package main

// dedup_restart_cooldown_test.go — Phase 29.
//
// Pins the post-success cooldown behaviour added to dedupRestart to
// stop the Envoy restart-storm documented in
// docs/awareness/reports/envoy_lds_cds_wedge.md.
//
// Contract pinned here (mirrors the Phase 29 task spec gates 1–7):
//   1. Same node/unit restart called rapidly → first dispatches,
//      subsequent calls within the cooldown are suppressed.
//   2. Same key while in-flight → coalesced (existing semantics,
//      retained by Phase 29 changes).
//   3. After cooldown elapses → next call dispatches normally.
//   4. Identity change (different unit name, which is how the
//      package name reaches dedupRestart) → no suppression.
//   5. Non-equivalent actions on different nodes/units → independent.
//   6. Envoy-specific: 10 rapid maybe_restart calls produce 1
//      ControlService call.
//   7. Failure path → does NOT write a cooldown marker, so the next
//      call dispatches immediately (caller's backoff stays authoritative).

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// failingAgent always returns an error from ControlService, simulating
// a restart that the node-agent rejected (e.g. unit not loaded).
type failingAgent struct {
	node_agentpb.UnimplementedNodeAgentServiceServer
	calls atomic.Int64
}

func (f *failingAgent) ControlService(_ context.Context, _ *node_agentpb.ControlServiceRequest) (*node_agentpb.ControlServiceResponse, error) {
	f.calls.Add(1)
	return nil, errors.New("simulated restart failure")
}

func startFailingAgent(t *testing.T, agent *failingAgent) (string, func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	gs := grpc.NewServer()
	node_agentpb.RegisterNodeAgentServiceServer(gs, agent)
	go gs.Serve(lis)
	return lis.Addr().String(), func() { gs.Stop() }
}

// newCooldownTestServer builds a dedup-test server with a fixed-clock
// seam and a short cooldown so the tests are deterministic and fast.
func newCooldownTestServer(addr string, now func() time.Time, cooldown time.Duration) *server {
	srv := &server{
		restartCooldown: cooldown,
		testNow:         now,
	}
	srv.testDialNodeAgent = func(_ string) (*grpc.ClientConn, error) {
		return grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
	}
	return srv
}

// fixedClock returns a now() function backed by a pointer so tests can
// advance the clock by reassigning the underlying time.
type fixedClock struct{ t time.Time }

func (c *fixedClock) now() time.Time     { return c.t }
func (c *fixedClock) add(d time.Duration) { c.t = c.t.Add(d) }

func TestDedupCooldown_RapidRedispatchSuppressed(t *testing.T) {
	// Gate (1): the Envoy storm signature. After one successful restart,
	// every subsequent call within the cooldown returns nil WITHOUT
	// invoking ControlService — that's what stops the workflow's
	// verify_effect re-dispatcher from SIGTERM'ing Envoy in a loop.
	agent := &fakeNodeAgent{}
	addr, stop := startFakeAgent(t, agent)
	defer stop()

	clk := &fixedClock{t: time.Unix(1_700_000_000, 0)}
	srv := newCooldownTestServer(addr, clk.now, 10*time.Second)
	ctx := context.Background()

	for i := 0; i < 6; i++ {
		if err := srv.dedupRestart(ctx, "node-a", addr, "globular-envoy.service"); err != nil {
			t.Fatalf("call %d: dedupRestart returned err=%v", i, err)
		}
		// Advance the clock by far less than the cooldown — these are
		// the rapid back-to-back dispatches that caused the storm.
		clk.add(200 * time.Millisecond)
	}

	if got := agent.calls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 ControlService call within cooldown, got %d (storm not suppressed)", got)
	}
}

func TestDedupCooldown_AfterCooldown_DispatchesAgain(t *testing.T) {
	// Gate (3): once the cooldown elapses, a real desired restart MUST
	// reach systemd. Without this, a degraded-then-recovered service
	// would be permanently un-restartable.
	agent := &fakeNodeAgent{}
	addr, stop := startFakeAgent(t, agent)
	defer stop()

	clk := &fixedClock{t: time.Unix(1_700_000_000, 0)}
	srv := newCooldownTestServer(addr, clk.now, 10*time.Second)
	ctx := context.Background()

	if err := srv.dedupRestart(ctx, "node-a", addr, "globular-envoy.service"); err != nil {
		t.Fatalf("first dedupRestart err=%v", err)
	}
	// Jump past the cooldown.
	clk.add(11 * time.Second)
	if err := srv.dedupRestart(ctx, "node-a", addr, "globular-envoy.service"); err != nil {
		t.Fatalf("post-cooldown dedupRestart err=%v", err)
	}

	if got := agent.calls.Load(); got != 2 {
		t.Fatalf("expected 2 ControlService calls (one per cooldown window), got %d", got)
	}
}

func TestDedupCooldown_DifferentUnit_NotSuppressed(t *testing.T) {
	// Gate (4) + (5): the cooldown for envoy must NOT block a concurrent
	// restart of a different package on the same node. Suppression is
	// per-key, not global.
	agent := &fakeNodeAgent{}
	addr, stop := startFakeAgent(t, agent)
	defer stop()

	clk := &fixedClock{t: time.Unix(1_700_000_000, 0)}
	srv := newCooldownTestServer(addr, clk.now, 10*time.Second)
	ctx := context.Background()

	if err := srv.dedupRestart(ctx, "node-a", addr, "globular-envoy.service"); err != nil {
		t.Fatalf("envoy: %v", err)
	}
	// Same node, different unit → independent cooldown bucket.
	if err := srv.dedupRestart(ctx, "node-a", addr, "globular-dns.service"); err != nil {
		t.Fatalf("dns: %v", err)
	}

	if got := agent.calls.Load(); got != 2 {
		t.Fatalf("expected 2 ControlService calls (one per unique unit), got %d", got)
	}
}

func TestDedupCooldown_DifferentNode_NotSuppressed(t *testing.T) {
	// Gate (5) refined: same unit on different nodes must each restart.
	agent := &fakeNodeAgent{}
	addr, stop := startFakeAgent(t, agent)
	defer stop()

	clk := &fixedClock{t: time.Unix(1_700_000_000, 0)}
	srv := newCooldownTestServer(addr, clk.now, 10*time.Second)
	ctx := context.Background()

	for _, node := range []string{"node-a", "node-b", "node-c"} {
		if err := srv.dedupRestart(ctx, node, addr, "globular-envoy.service"); err != nil {
			t.Fatalf("node %s: %v", node, err)
		}
	}

	if got := agent.calls.Load(); got != 3 {
		t.Fatalf("expected 3 ControlService calls (one per node), got %d", got)
	}
}

func TestDedupCooldown_FailedRestart_DoesNotBlockRetry(t *testing.T) {
	// Gate (7): a failed restart must NOT seed a cooldown entry —
	// otherwise a transient error (e.g. unit transiently unloaded)
	// would lock the package out of recovery for 10s. The caller's
	// retry policy stays authoritative.
	agent := &failingAgent{}
	addr, stop := startFailingAgent(t, agent)
	defer stop()

	clk := &fixedClock{t: time.Unix(1_700_000_000, 0)}
	srv := newCooldownTestServer(addr, clk.now, 10*time.Second)
	ctx := context.Background()

	if err := srv.dedupRestart(ctx, "node-a", addr, "globular-envoy.service"); err == nil {
		t.Fatalf("first call: expected failure, got nil")
	}
	// No clock advance — caller's retry fires immediately.
	if err := srv.dedupRestart(ctx, "node-a", addr, "globular-envoy.service"); err == nil {
		t.Fatalf("second call: expected failure, got nil")
	}

	if got := agent.calls.Load(); got != 2 {
		t.Fatalf("expected both failures to reach the agent (no cooldown on failure), got %d calls", got)
	}

	// And recentRestarts must remain empty.
	count := 0
	srv.recentRestarts.Range(func(_, _ any) bool { count++; return true })
	if count != 0 {
		t.Fatalf("recentRestarts must stay empty on failure, found %d entries", count)
	}
}

func TestDedupCooldown_EnvoyStormScenario_OneDispatchPerWindow(t *testing.T) {
	// Gate (6): the exact Envoy storm reproduced live on globule-ryzen
	// (00:35–00:47 EDT 2026-06-03). The workflow re-dispatches
	// node.maybe_restart_package roughly every 200-300 ms because the
	// step's `resume_policy: verify_effect` keeps failing while Envoy
	// is still in cold init. With the Phase 29 cooldown, only the
	// first dispatch in each cooldown window reaches systemd — giving
	// Envoy enough time to finish CDS + LDS.
	agent := &fakeNodeAgent{}
	addr, stop := startFakeAgent(t, agent)
	defer stop()

	clk := &fixedClock{t: time.Unix(1_700_000_000, 0)}
	srv := newCooldownTestServer(addr, clk.now, 10*time.Second)
	ctx := context.Background()

	// 10 rapid dispatches within the cooldown — Phase 23/28 signature.
	for i := 0; i < 10; i++ {
		if err := srv.dedupRestart(ctx, "globule-ryzen", addr, "globular-envoy.service"); err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
		clk.add(250 * time.Millisecond)
	}
	if got := agent.calls.Load(); got != 1 {
		t.Fatalf("storm window: expected 1 dispatch, got %d", got)
	}

	// After cooldown, the next "real" workflow tick is allowed through.
	// This proves the gate doesn't permanently silence the package.
	clk.add(11 * time.Second)
	if err := srv.dedupRestart(ctx, "globule-ryzen", addr, "globular-envoy.service"); err != nil {
		t.Fatalf("post-cooldown: %v", err)
	}
	if got := agent.calls.Load(); got != 2 {
		t.Fatalf("post-cooldown: expected 2 total dispatches, got %d", got)
	}
}

func TestDedupCooldown_RecentRestartsStaleEntryPruned(t *testing.T) {
	// Housekeeping: a successful entry that has aged past the cooldown
	// must be evicted on the next access, so the map does not grow
	// unboundedly for packages that are restarted, then go idle for
	// hours, then restarted again.
	agent := &fakeNodeAgent{}
	addr, stop := startFakeAgent(t, agent)
	defer stop()

	clk := &fixedClock{t: time.Unix(1_700_000_000, 0)}
	srv := newCooldownTestServer(addr, clk.now, 10*time.Second)
	ctx := context.Background()

	if err := srv.dedupRestart(ctx, "node-a", addr, "globular-envoy.service"); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Confirm the entry is present.
	if _, ok := srv.recentRestarts.Load("node-a::globular-envoy.service"); !ok {
		t.Fatal("expected recentRestarts entry after successful restart")
	}

	// Jump way past the cooldown and call again — the stale marker
	// must be evicted as part of the cooldown check, then a fresh
	// marker written by the new successful restart.
	clk.add(time.Hour)
	if err := srv.dedupRestart(ctx, "node-a", addr, "globular-envoy.service"); err != nil {
		t.Fatalf("second: %v", err)
	}
	v, ok := srv.recentRestarts.Load("node-a::globular-envoy.service")
	if !ok {
		t.Fatal("expected fresh recentRestarts entry after dispatch beyond cooldown")
	}
	got, _ := v.(time.Time)
	if !got.Equal(clk.now()) {
		t.Fatalf("recentRestarts entry not refreshed: have %v, want %v", got, clk.now())
	}
}
