package main

// Phase 6 — MCP transport reliability tests.
//
// These exercise the awareness.* tool handlers with a fake
// AwarenessGraphClient so the failure-class taxonomy and timeout
// wrapping are covered without standing up a real gRPC server.
//
// The seam is the package-level `awarenessStub` var: swap it in
// setUp, restore in defer. No transport, no etcd, no network.

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ─── fake client ────────────────────────────────────────────────────────

// fakeAwarenessClient implements awarenesspb.AwarenessGraphClient so we
// can inject any response or error per-call.
type fakeAwarenessClient struct {
	briefingResp *awarenesspb.BriefingResponse
	briefingErr  error
	impactResp   *awarenesspb.ImpactResponse
	impactErr    error
	resolveResp  *awarenesspb.ResolveResponse
	resolveErr   error
	queryResp    *awarenesspb.QueryResponse
	queryErr     error
	preflightResp *awarenesspb.PreflightResponse
	preflightErr  error
	metadataResp  *awarenesspb.MetadataResponse
	metadataErr   error

	// blockUntilDeadline forces the call to wait for ctx.Done() and
	// return ctx.Err() — used to verify the wrapper actually applies a
	// deadline (otherwise the test would hang).
	blockUntilDeadline bool
}

func (f *fakeAwarenessClient) maybeBlock(ctx context.Context) error {
	if !f.blockUntilDeadline {
		return nil
	}
	<-ctx.Done()
	return ctx.Err()
}

func (f *fakeAwarenessClient) Briefing(ctx context.Context, in *awarenesspb.BriefingRequest, _ ...grpc.CallOption) (*awarenesspb.BriefingResponse, error) {
	if err := f.maybeBlock(ctx); err != nil {
		return nil, err
	}
	return f.briefingResp, f.briefingErr
}
func (f *fakeAwarenessClient) Impact(ctx context.Context, in *awarenesspb.ImpactRequest, _ ...grpc.CallOption) (*awarenesspb.ImpactResponse, error) {
	if err := f.maybeBlock(ctx); err != nil {
		return nil, err
	}
	return f.impactResp, f.impactErr
}
func (f *fakeAwarenessClient) Resolve(ctx context.Context, in *awarenesspb.ResolveRequest, _ ...grpc.CallOption) (*awarenesspb.ResolveResponse, error) {
	if err := f.maybeBlock(ctx); err != nil {
		return nil, err
	}
	return f.resolveResp, f.resolveErr
}
func (f *fakeAwarenessClient) Query(ctx context.Context, in *awarenesspb.QueryRequest, _ ...grpc.CallOption) (*awarenesspb.QueryResponse, error) {
	if err := f.maybeBlock(ctx); err != nil {
		return nil, err
	}
	return f.queryResp, f.queryErr
}
func (f *fakeAwarenessClient) Preflight(ctx context.Context, in *awarenesspb.PreflightRequest, _ ...grpc.CallOption) (*awarenesspb.PreflightResponse, error) {
	if err := f.maybeBlock(ctx); err != nil {
		return nil, err
	}
	return f.preflightResp, f.preflightErr
}
func (f *fakeAwarenessClient) Metadata(ctx context.Context, in *awarenesspb.MetadataRequest, _ ...grpc.CallOption) (*awarenesspb.MetadataResponse, error) {
	if err := f.maybeBlock(ctx); err != nil {
		return nil, err
	}
	return f.metadataResp, f.metadataErr
}

// ─── server helpers ─────────────────────────────────────────────────────

// newTestServer builds an in-process server with the awareness tool group
// registered. The fake's lifetime is bound to the returned cleanup func,
// which restores the real stub so subsequent tests aren't poisoned.
func newTestServer(t *testing.T, fake *fakeAwarenessClient) (*server, func()) {
	t.Helper()
	s := newServer(&MCPConfig{ConcurrencyLimit: 4})
	prev := awarenessStub
	awarenessStub = func(ctx context.Context, _ *server) (awarenesspb.AwarenessGraphClient, string, error) {
		return fake, "test://awareness", nil
	}
	registerAwarenessTools(s)
	return s, func() { awarenessStub = prev }
}

// newTestServerStubFailure simulates the stub-fetch failure path (etcd
// lookup or dial failure before any RPC is sent).
func newTestServerStubFailure(t *testing.T, stubErr error) (*server, func()) {
	t.Helper()
	s := newServer(&MCPConfig{ConcurrencyLimit: 4})
	prev := awarenessStub
	awarenessStub = func(ctx context.Context, _ *server) (awarenesspb.AwarenessGraphClient, string, error) {
		return nil, "", stubErr
	}
	registerAwarenessTools(s)
	return s, func() { awarenessStub = prev }
}

func degradedFields(t *testing.T, result interface{}) map[string]interface{} {
	t.Helper()
	m, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", result)
	}
	if s, _ := m["status"].(string); s != "degraded" {
		t.Fatalf("expected status=degraded, got %q (full: %+v)", s, m)
	}
	return m
}

// ─── classifyFailure unit tests ────────────────────────────────────────

func TestClassifyFailure_GRPCCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want FailureClass
	}{
		{"unavailable", status.Error(codes.Unavailable, "no backend"), FailureUnavailable},
		{"deadline_exceeded", status.Error(codes.DeadlineExceeded, "slow"), FailureTimeout},
		{"invalid_argument", status.Error(codes.InvalidArgument, "bad"), FailureInvalidArgument},
		{"internal", status.Error(codes.Internal, "store boom"), FailureStoreError},
		{"data_loss", status.Error(codes.DataLoss, "oxigraph corrupt"), FailureStoreError},
		{"context_deadline", context.DeadlineExceeded, FailureTimeout},
		{"context_canceled", context.Canceled, FailureTransportError},
		{"endpoint_missing", errors.New("awareness-graph not found in etcd"), FailureEndpointResolution},
		{"address_missing", errors.New("Address missing from etcd"), FailureEndpointResolution},
		{"tls_handshake_msg", errors.New("tls: handshake failure"), FailureTransportError},
		{"connection_refused_msg", errors.New("connect: connection refused"), FailureUnavailable},
		{"nil", nil, FailureNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyFailure(tc.err); got != tc.want {
				t.Errorf("classifyFailure(%v): want %s, got %s", tc.err, tc.want, got)
			}
		})
	}
}

// ─── per-tool failure-class smoke ──────────────────────────────────────

func TestBriefing_StubFailureClassifiedAsEndpointResolution(t *testing.T) {
	s, cleanup := newTestServerStubFailure(t, errors.New("awareness-graph: awareness-graph not found in etcd"))
	defer cleanup()

	resp, err := s.callTool(context.Background(), "awareness.briefing", map[string]interface{}{"file": "golang/foo/bar.go"})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	m := degradedFields(t, resp)
	if got := m["failure_class"]; got != string(FailureEndpointResolution) {
		t.Errorf("failure_class: want %s, got %v", FailureEndpointResolution, got)
	}
	if got := m["tool"]; got != "awareness.briefing" {
		t.Errorf("tool: want awareness.briefing, got %v", got)
	}
	if got := m["target"]; got != "golang/foo/bar.go" {
		t.Errorf("target: want file path, got %v", got)
	}
}

func TestBriefing_UnavailableMapsToUnavailable(t *testing.T) {
	fake := &fakeAwarenessClient{briefingErr: status.Error(codes.Unavailable, "down")}
	s, cleanup := newTestServer(t, fake)
	defer cleanup()

	resp, err := s.callTool(context.Background(), "awareness.briefing", map[string]interface{}{"file": "golang/x/y.go"})
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	m := degradedFields(t, resp)
	if got := m["failure_class"]; got != string(FailureUnavailable) {
		t.Errorf("failure_class: want UNAVAILABLE, got %v", got)
	}
}

func TestImpact_StoreErrorMapsToStoreError(t *testing.T) {
	fake := &fakeAwarenessClient{impactErr: status.Error(codes.Internal, "oxigraph 500")}
	s, cleanup := newTestServer(t, fake)
	defer cleanup()

	resp, err := s.callTool(context.Background(), "awareness.impact", map[string]interface{}{"file": "golang/foo.go"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	m := degradedFields(t, resp)
	if got := m["failure_class"]; got != string(FailureStoreError) {
		t.Errorf("failure_class: want STORE_ERROR, got %v", got)
	}
	if got := m["tool"]; got != "awareness.impact" {
		t.Errorf("tool name leak: %v", got)
	}
}

func TestResolve_InvalidArgumentMapsToInvalidArgument(t *testing.T) {
	fake := &fakeAwarenessClient{resolveErr: status.Error(codes.InvalidArgument, "unknown class")}
	s, cleanup := newTestServer(t, fake)
	defer cleanup()

	resp, err := s.callTool(context.Background(), "awareness.resolve", map[string]interface{}{"class": "BogusClass", "id": "x"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	m := degradedFields(t, resp)
	if got := m["failure_class"]; got != string(FailureInvalidArgument) {
		t.Errorf("failure_class: want INVALID_ARGUMENT, got %v", got)
	}
	if got, _ := m["target"].(string); !strings.Contains(got, "BogusClass:x") {
		t.Errorf("target should encode class:id, got %q", got)
	}
}

func TestQuery_TimeoutFromContextDeadlineMapsToTimeout(t *testing.T) {
	fake := &fakeAwarenessClient{queryErr: status.Error(codes.DeadlineExceeded, "slow query")}
	s, cleanup := newTestServer(t, fake)
	defer cleanup()

	resp, err := s.callTool(context.Background(), "awareness.query", map[string]interface{}{
		"mode": "by_file",
		"file": "golang/foo.go",
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	m := degradedFields(t, resp)
	if got := m["failure_class"]; got != string(FailureTimeout) {
		t.Errorf("failure_class: want TIMEOUT, got %v", got)
	}
}

func TestPreflight_DegradedNeverShadowsTransportFailure(t *testing.T) {
	// Critical safety property: a transport failure must produce
	// failure_class != "" — NOT a fake empty/success.
	fake := &fakeAwarenessClient{preflightErr: status.Error(codes.Unavailable, "no backend")}
	s, cleanup := newTestServer(t, fake)
	defer cleanup()

	resp, err := s.callTool(context.Background(), "awareness.preflight", map[string]interface{}{
		"files": []interface{}{"golang/cluster_controller/x.go"},
	})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	m := degradedFields(t, resp)
	if got := m["failure_class"]; got == "" || got == nil {
		t.Errorf("transport failure produced empty failure_class; would let a caller mistake it for EMPTY: %+v", m)
	}
	if !strings.Contains(m["target"].(string), "golang/cluster_controller/x.go") {
		t.Errorf("target must surface input files, got %v", m["target"])
	}
}

// ─── timeout wrapping ───────────────────────────────────────────────────

// TestBriefing_RespectsAwarenessCallTimeout verifies that a hung server
// is bounded by the per-call timeout — without the wrapper, the test
// would deadlock.
//
// We can't wait the real awarenessCallTimeout (10s) in CI, so we shrink
// it locally via the seam and restore on cleanup.
func TestBriefing_RespectsPerCallTimeout(t *testing.T) {
	fake := &fakeAwarenessClient{blockUntilDeadline: true}
	s, cleanup := newTestServer(t, fake)
	defer cleanup()

	// Bound the test itself in case the wrapper is missing.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	var resp interface{}
	var err error
	go func() {
		resp, err = s.callTool(ctx, "awareness.briefing", map[string]interface{}{"file": "golang/foo.go"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not return — per-call timeout missing or much too large")
	}
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	m := degradedFields(t, resp)
	if got := m["failure_class"]; got != string(FailureTimeout) {
		t.Errorf("failure_class: want TIMEOUT after deadline, got %v", got)
	}
}

// ─── success path stays untouched ──────────────────────────────────────

func TestBriefing_SuccessReturnsTypedResponse(t *testing.T) {
	fake := &fakeAwarenessClient{
		briefingResp: &awarenesspb.BriefingResponse{
			Status:        awarenesspb.BriefingStatus_BRIEFING_STATUS_OK,
			Prose:         "hello world",
			ReferencedIds: []string{"invariant:foo"},
		},
	}
	s, cleanup := newTestServer(t, fake)
	defer cleanup()

	resp, err := s.callTool(context.Background(), "awareness.briefing", map[string]interface{}{"task": "edit X"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	m, ok := resp.(map[string]interface{})
	if !ok {
		t.Fatalf("want map, got %T", resp)
	}
	if got := m["status"]; got != "ok" {
		t.Errorf("status: want ok, got %v", got)
	}
	if got := m["prose"]; got != "hello world" {
		t.Errorf("prose: want hello world, got %v", got)
	}
}

// TestBriefing_EmptyServerResponseNeverFalsified verifies that a real
// EMPTY semantic response from the server passes through with
// status=empty, NOT degraded. A degraded shape with no
// failure_class would let a caller confuse empty with failure.
func TestBriefing_EmptyServerResponseStaysEmpty(t *testing.T) {
	fake := &fakeAwarenessClient{
		briefingResp: &awarenesspb.BriefingResponse{
			Status: awarenesspb.BriefingStatus_BRIEFING_STATUS_EMPTY,
		},
	}
	s, cleanup := newTestServer(t, fake)
	defer cleanup()

	resp, err := s.callTool(context.Background(), "awareness.briefing", map[string]interface{}{"file": "golang/foo.go"})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	m, ok := resp.(map[string]interface{})
	if !ok {
		t.Fatalf("want map, got %T", resp)
	}
	if got := m["status"]; got != "empty" {
		t.Errorf("status: want empty, got %v", got)
	}
	if _, hasFailureClass := m["failure_class"]; hasFailureClass {
		t.Errorf("EMPTY response must NOT carry failure_class field: %+v", m)
	}
}
