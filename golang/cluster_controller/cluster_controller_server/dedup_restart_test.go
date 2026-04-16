package main

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// fakeNodeAgent counts ControlService calls and adds a small delay to
// simulate real restart latency so concurrent callers overlap.
type fakeNodeAgent struct {
	node_agentpb.UnimplementedNodeAgentServiceServer
	calls atomic.Int64
	delay time.Duration
}

func (f *fakeNodeAgent) ControlService(_ context.Context, req *node_agentpb.ControlServiceRequest) (*node_agentpb.ControlServiceResponse, error) {
	f.calls.Add(1)
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	return &node_agentpb.ControlServiceResponse{Ok: true, Unit: req.GetUnit()}, nil
}

// startFakeAgent starts an insecure gRPC server with the fake node agent and
// returns the listener address and a cleanup function.
func startFakeAgent(t *testing.T, agent *fakeNodeAgent) (string, func()) {
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

// newDedupTestServer creates a minimal server with the testDialNodeAgent seam
// pointing at the given address using insecure credentials.
func newDedupTestServer(addr string) *server {
	srv := &server{}
	srv.testDialNodeAgent = func(_ string) (*grpc.ClientConn, error) {
		return grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
	}
	return srv
}

// TestDedupRestart_ConcurrentSameKey verifies that 4 concurrent goroutines
// calling dedupRestart for the same (nodeID, unit) produce exactly 1 gRPC
// ControlService call and all return nil.
func TestDedupRestart_ConcurrentSameKey(t *testing.T) {
	agent := &fakeNodeAgent{delay: 100 * time.Millisecond}
	addr, stop := startFakeAgent(t, agent)
	defer stop()

	srv := newDedupTestServer(addr)
	ctx := context.Background()

	const n = 4
	var wg sync.WaitGroup
	errs := make([]error, n)

	// Use a barrier so all goroutines call dedupRestart near-simultaneously.
	barrier := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-barrier
			errs[idx] = srv.dedupRestart(ctx, "node-a", addr, "echo_server.service")
		}(i)
	}
	close(barrier) // release all at once
	wg.Wait()

	// All callers must return nil.
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d returned error: %v", i, err)
		}
	}

	// Exactly 1 gRPC call should have been made.
	got := agent.calls.Load()
	if got != 1 {
		t.Errorf("expected 1 ControlService call, got %d", got)
	}

	// inflightRestarts map must be empty after completion.
	srv.inflightRestarts.Range(func(key, value any) bool {
		t.Errorf("inflightRestarts still has key %v", key)
		return true
	})
}

// TestDedupRestart_DifferentKeysExecuteIndependently verifies that concurrent
// restarts for DIFFERENT (node, unit) pairs each produce their own gRPC call.
func TestDedupRestart_DifferentKeysExecuteIndependently(t *testing.T) {
	agent := &fakeNodeAgent{delay: 50 * time.Millisecond}
	addr, stop := startFakeAgent(t, agent)
	defer stop()

	srv := newDedupTestServer(addr)
	ctx := context.Background()

	type pair struct {
		node string
		unit string
	}
	pairs := []pair{
		{"node-a", "echo_server.service"},
		{"node-b", "echo_server.service"},
		{"node-a", "dns_server.service"},
		{"node-c", "rbac_server.service"},
	}

	var wg sync.WaitGroup
	errs := make([]error, len(pairs))

	barrier := make(chan struct{})
	for i, p := range pairs {
		wg.Add(1)
		go func(idx int, p pair) {
			defer wg.Done()
			<-barrier
			errs[idx] = srv.dedupRestart(ctx, p.node, addr, p.unit)
		}(i, p)
	}
	close(barrier)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("pair %d (%s/%s) returned error: %v", i, pairs[i].node, pairs[i].unit, err)
		}
	}

	got := agent.calls.Load()
	if got != int64(len(pairs)) {
		t.Errorf("expected %d ControlService calls (one per unique key), got %d", len(pairs), got)
	}

	srv.inflightRestarts.Range(func(key, value any) bool {
		t.Errorf("inflightRestarts still has key %v", key)
		return true
	})
}

// TestDedupRestart_NoStuckEntries verifies the map is cleaned up even when
// there are no concurrent waiters (single caller, simple path).
func TestDedupRestart_NoStuckEntries(t *testing.T) {
	agent := &fakeNodeAgent{}
	addr, stop := startFakeAgent(t, agent)
	defer stop()

	srv := newDedupTestServer(addr)
	ctx := context.Background()

	if err := srv.dedupRestart(ctx, "node-x", addr, "test.service"); err != nil {
		t.Fatalf("dedupRestart failed: %v", err)
	}

	srv.inflightRestarts.Range(func(key, value any) bool {
		t.Errorf("inflightRestarts still has key %v after single call", key)
		return true
	})

	if got := agent.calls.Load(); got != 1 {
		t.Errorf("expected 1 call, got %d", got)
	}
}
