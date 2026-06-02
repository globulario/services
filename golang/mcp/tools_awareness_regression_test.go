package main

// Phase 7 — regression guards for the MCP awareness tool surface.
//
// These tests pin properties that, if they ever regress, would either
// expose a footgun (raw SPARQL passthrough), break the etcd-only
// invariant (localhost fallback), or invalidate the failure-class
// contract (auto-retry on a failure). They are intentionally narrow
// and assertion-heavy.

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	awarenesspb "github.com/globulario/awareness-graph/golang/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ─── 1. No arbitrary SPARQL exposure on the MCP surface ────────────────

// TestAwarenessTools_NoRawSPARQLOnSurface walks every registered
// awareness tool's input schema and verifies no property is named
// or described as accepting raw SPARQL. The user-facing safety
// property: an MCP caller should never be able to pass an
// arbitrary SPARQL string through a tool argument.
//
// The awareness.query tool accepts a structured mode/file/id/class —
// NOT a free-form query string. This test makes the surface invariant
// explicit so a future "add a `sparql` field for power users" PR fails
// loudly.
//
// The server-side counterpart (TestQuery_RawSPARQLLikeInputRejected
// in awareness-graph/golang/server/main_test.go) verifies the server
// rejects SPARQL-like syntax in the `id` field. This test guards the
// MCP layer above it.
func TestAwarenessTools_NoRawSPARQLOnSurface(t *testing.T) {
	s := newServer(&MCPConfig{ConcurrencyLimit: 1})
	registerAwarenessTools(s)

	for name, tool := range s.tools {
		if !strings.HasPrefix(name, "awareness") {
			continue
		}
		// Tool description must not advertise raw SPARQL.
		if containsAnyToken(strings.ToLower(tool.def.Description), "sparql", "raw query", "raw sparql") {
			t.Errorf("tool %s description mentions raw SPARQL: %q",
				name, tool.def.Description)
		}
		for propName, prop := range tool.def.InputSchema.Properties {
			if containsAnyToken(strings.ToLower(propName), "sparql") {
				t.Errorf("tool %s has property named %q — raw SPARQL must NOT be exposed on the MCP surface",
					name, propName)
			}
			if containsAnyToken(strings.ToLower(prop.Description), "sparql", "raw query", "free-form sparql") {
				t.Errorf("tool %s property %q description mentions raw SPARQL: %q",
					name, propName, prop.Description)
			}
		}
	}
}

func containsAnyToken(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

// ─── 2. No localhost fallback on endpoint resolution ───────────────────

// TestAwarenessEndpoint_NoLocalhostFallback verifies that a stub
// failure for endpoint resolution surfaces as a classified failure
// (ENDPOINT_RESOLUTION) — NOT as a silent fallback to localhost or
// 127.0.0.1. The hard rule "etcd is the sole source of truth"
// forbids localhost fallbacks; if etcd has nothing the service must
// error, not invent a default.
//
// We exercise the bridge code path through the stub seam: the real
// awarenessEndpoint() (in clients.go) returns an error if etcd has
// no record; we model that here by injecting an
// "awareness-graph not found in etcd" error and assert the
// degraded result's failure_class.
func TestAwarenessEndpoint_NoLocalhostFallback(t *testing.T) {
	s := newServer(&MCPConfig{ConcurrencyLimit: 1})
	prev := awarenessStub
	awarenessStub = func(ctx context.Context, _ *server) (awarenesspb.AwarenessGraphClient, string, error) {
		return nil, "", errors.New("awareness-graph: awareness-graph not found in etcd")
	}
	defer func() { awarenessStub = prev }()
	registerAwarenessTools(s)

	resp, err := s.callTool(context.Background(), "awareness.briefing", map[string]interface{}{
		"file": "golang/foo.go",
	})
	if err != nil {
		t.Fatalf("handler should not propagate err; got %v", err)
	}
	m, ok := resp.(map[string]interface{})
	if !ok {
		t.Fatalf("want degraded map, got %T", resp)
	}
	if m["status"] != "degraded" {
		t.Fatalf("status: want degraded, got %v", m["status"])
	}
	if m["failure_class"] != string(FailureEndpointResolution) {
		t.Errorf("failure_class: want %s, got %v",
			FailureEndpointResolution, m["failure_class"])
	}
	// Hard property: error string MUST NOT mention 127.0.0.1 or localhost,
	// confirming the bridge never invented an address.
	errStr, _ := m["error"].(string)
	for _, banned := range []string{"127.0.0.1", "localhost"} {
		if strings.Contains(errStr, banned) {
			t.Errorf("degraded error string contains %q — looks like a localhost fallback: %q",
				banned, errStr)
		}
	}
}

// ─── 3. No automatic retry inside the MCP bridge ───────────────────────

// retryCountingClient counts how many times each gRPC method is
// invoked. The MCP contract is one tool call = one upstream RPC; if
// any test below sees a call count > 1, the bridge has grown a hidden
// retry loop and the failure-class contract no longer holds (a
// retried call would surface only the second attempt's class).
type retryCountingClient struct {
	briefingCalls  atomic.Int64
	impactCalls    atomic.Int64
	resolveCalls   atomic.Int64
	queryCalls     atomic.Int64
	preflightCalls atomic.Int64

	briefingErr  error
	impactErr    error
	resolveErr   error
	queryErr     error
	preflightErr error
}

func (c *retryCountingClient) Briefing(ctx context.Context, _ *awarenesspb.BriefingRequest, _ ...grpc.CallOption) (*awarenesspb.BriefingResponse, error) {
	c.briefingCalls.Add(1)
	return nil, c.briefingErr
}
func (c *retryCountingClient) Impact(ctx context.Context, _ *awarenesspb.ImpactRequest, _ ...grpc.CallOption) (*awarenesspb.ImpactResponse, error) {
	c.impactCalls.Add(1)
	return nil, c.impactErr
}
func (c *retryCountingClient) Resolve(ctx context.Context, _ *awarenesspb.ResolveRequest, _ ...grpc.CallOption) (*awarenesspb.ResolveResponse, error) {
	c.resolveCalls.Add(1)
	return nil, c.resolveErr
}
func (c *retryCountingClient) Query(ctx context.Context, _ *awarenesspb.QueryRequest, _ ...grpc.CallOption) (*awarenesspb.QueryResponse, error) {
	c.queryCalls.Add(1)
	return nil, c.queryErr
}
func (c *retryCountingClient) Preflight(ctx context.Context, _ *awarenesspb.PreflightRequest, _ ...grpc.CallOption) (*awarenesspb.PreflightResponse, error) {
	c.preflightCalls.Add(1)
	return nil, c.preflightErr
}
func (c *retryCountingClient) Metadata(ctx context.Context, _ *awarenesspb.MetadataRequest, _ ...grpc.CallOption) (*awarenesspb.MetadataResponse, error) {
	return nil, nil
}

// TestAwarenessTools_NoAutomaticRetry asserts that a single tool
// invocation produces exactly one upstream RPC, even on failure.
// Auto-retry would mask the failure class an operator needs to see
// and would change a cheap call into an expensive one. The MCP
// bridge is intentionally a thin forwarder — the caller decides
// whether to retry.
func TestAwarenessTools_NoAutomaticRetry(t *testing.T) {
	for _, tc := range []struct {
		toolName string
		args     map[string]interface{}
		errToSet func(*retryCountingClient)
		count    func(*retryCountingClient) int64
	}{
		{
			toolName: "awareness.briefing",
			args:     map[string]interface{}{"file": "golang/foo.go"},
			errToSet: func(c *retryCountingClient) {
				c.briefingErr = status.Error(codes.Unavailable, "down")
			},
			count: func(c *retryCountingClient) int64 { return c.briefingCalls.Load() },
		},
		{
			toolName: "awareness.impact",
			args:     map[string]interface{}{"file": "golang/foo.go"},
			errToSet: func(c *retryCountingClient) {
				c.impactErr = status.Error(codes.Internal, "store boom")
			},
			count: func(c *retryCountingClient) int64 { return c.impactCalls.Load() },
		},
		{
			toolName: "awareness.resolve",
			args:     map[string]interface{}{"class": "Invariant", "id": "x.y"},
			errToSet: func(c *retryCountingClient) {
				c.resolveErr = status.Error(codes.DeadlineExceeded, "slow")
			},
			count: func(c *retryCountingClient) int64 { return c.resolveCalls.Load() },
		},
		{
			toolName: "awareness.query",
			args:     map[string]interface{}{"mode": "by_file", "file": "golang/foo.go"},
			errToSet: func(c *retryCountingClient) {
				c.queryErr = status.Error(codes.Unavailable, "down")
			},
			count: func(c *retryCountingClient) int64 { return c.queryCalls.Load() },
		},
		{
			toolName: "awareness.preflight",
			args:     map[string]interface{}{"files": []interface{}{"golang/foo.go"}},
			errToSet: func(c *retryCountingClient) {
				c.preflightErr = status.Error(codes.Unavailable, "down")
			},
			count: func(c *retryCountingClient) int64 { return c.preflightCalls.Load() },
		},
	} {
		t.Run(tc.toolName, func(t *testing.T) {
			counter := &retryCountingClient{}
			tc.errToSet(counter)

			s := newServer(&MCPConfig{ConcurrencyLimit: 1})
			prev := awarenessStub
			awarenessStub = func(ctx context.Context, _ *server) (awarenesspb.AwarenessGraphClient, string, error) {
				return counter, "test://awareness", nil
			}
			defer func() { awarenessStub = prev }()
			registerAwarenessTools(s)

			if _, err := s.callTool(context.Background(), tc.toolName, tc.args); err != nil {
				t.Fatalf("handler should not propagate err; got %v", err)
			}
			if got := tc.count(counter); got != 1 {
				t.Errorf("%s invoked the upstream %d times — must be exactly 1 (no retry)",
					tc.toolName, got)
			}
		})
	}
}

// ─── 4. All 5 tools register + are addressable ─────────────────────────

// TestAwarenessTools_AllFiveRegistered guards against accidental
// removal of any of the 5 single-source awareness tools (the
// composite awareness_diagnose is separate). Without this, a
// regression that deregistered one tool could ship silently — the
// tool would just stop appearing in the MCP tool list and callers
// would get "unknown tool" instead of an explicit error.
func TestAwarenessTools_AllFiveRegistered(t *testing.T) {
	s := newServer(&MCPConfig{ConcurrencyLimit: 1})
	registerAwarenessTools(s)

	want := []string{
		"awareness.briefing",
		"awareness.impact",
		"awareness.resolve",
		"awareness.query",
		"awareness.preflight",
	}
	for _, name := range want {
		if !s.hasTool(name) {
			t.Errorf("tool %q is missing from awareness tool group", name)
		}
	}
}
