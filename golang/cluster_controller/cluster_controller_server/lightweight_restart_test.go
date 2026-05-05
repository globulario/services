package main

// lightweight_restart_test.go — Formal tests for the tryLightweightRestart
// direct-restart path.
//
// G11 invariant: workflow service repair must NOT require the workflow engine
// to be healthy. The lightweight-restart path calls node-agent's ControlService
// RPC directly (no workflow service involved), providing a non-circular recovery
// path for failed service units.
//
// These tests verify the guards inside tryLightweightRestart without dialing a
// real node-agent, exercising the function up to the point where a real gRPC
// connection would be needed.

import (
	"context"
	"testing"
	"time"
)

// TestLightweightRestart_BackoffPreventsCall verifies that when BackoffUntil
// is in the future (active backoff), tryLightweightRestart returns false
// immediately without attempting an agent connection. This ensures the
// controller does not hammer the agent while a restart is cooling down.
func TestLightweightRestart_BackoffPreventsCall(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Status:         "ready",
				AgentEndpoint:  "10.0.0.8:11000",
				BootstrapPhase: BootstrapWorkloadReady,
				RestartAttempts: map[string]*restartAttempt{
					"globular-workflow": {
						Count:        2,
						BackoffUntil: time.Now().Add(10 * time.Minute),
					},
				},
			},
		},
	}
	srv := newTestServer(t, state)
	node := state.Nodes["n1"]

	// tryLightweightRestart must return false during active backoff — no agent
	// dial is attempted, so even with an unreachable endpoint it should not block.
	got := srv.tryLightweightRestart(context.Background(), node, "n1",
		"globular-workflow", "globular-workflow.service", "test-release")

	if got {
		t.Error("tryLightweightRestart must return false during active backoff")
	}
}

// TestLightweightRestart_NoEndpointReturnsFalse verifies that when the node
// has no AgentEndpoint configured (e.g. node joined but agent not yet reachable),
// tryLightweightRestart returns false without panicking. An attempt with no
// endpoint would be meaningless and should be skipped cleanly.
func TestLightweightRestart_NoEndpointReturnsFalse(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Status:         "ready",
				AgentEndpoint:  "", // no endpoint
				BootstrapPhase: BootstrapWorkloadReady,
			},
		},
	}
	srv := newTestServer(t, state)
	node := state.Nodes["n1"]

	got := srv.tryLightweightRestart(context.Background(), node, "n1",
		"globular-workflow", "globular-workflow.service", "test-release")

	if got {
		t.Error("tryLightweightRestart must return false when AgentEndpoint is empty")
	}
}

// TestLightweightRestart_BlockedReasonSkipsWithNoEndpoint verifies that when
// a service is in a blocked state (repeated precondition failures) AND the
// agent endpoint is empty, the function returns false cleanly. This prevents
// an infinite retry loop when the blocking condition can't be verified.
func TestLightweightRestart_BlockedReasonSkipsWithNoEndpoint(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Status:         "ready",
				AgentEndpoint:  "", // no endpoint — can't verify clearance
				BootstrapPhase: BootstrapWorkloadReady,
				RestartAttempts: map[string]*restartAttempt{
					"globular-workflow": {
						BlockedReason: "server certificate missing",
						BlockedSince:  time.Now().Add(-5 * time.Minute),
					},
				},
			},
		},
	}
	srv := newTestServer(t, state)
	node := state.Nodes["n1"]

	got := srv.tryLightweightRestart(context.Background(), node, "n1",
		"globular-workflow", "globular-workflow.service", "test-release")

	if got {
		t.Error("tryLightweightRestart must return false when blocked and endpoint unavailable")
	}
}

// TestLightweightRestart_InitializesRestartAttemptMap verifies that
// tryLightweightRestart initialises RestartAttempts map when nil, so the
// caller's nodeState is properly mutated for subsequent backoff checks.
// This is a state-mutation contract: after a call, RestartAttempts must be
// non-nil so the backoff can be tracked.
func TestLightweightRestart_InitializesRestartAttemptMap(t *testing.T) {
	state := &controllerState{
		Nodes: map[string]*nodeState{
			"n1": {
				NodeID:         "n1",
				Status:         "ready",
				AgentEndpoint:  "", // no endpoint → returns false before dial
				BootstrapPhase: BootstrapWorkloadReady,
				RestartAttempts: nil, // explicitly nil
			},
		},
	}
	srv := newTestServer(t, state)
	node := state.Nodes["n1"]

	_ = srv.tryLightweightRestart(context.Background(), node, "n1",
		"globular-workflow", "globular-workflow.service", "test-release")

	// After the call, the map must be non-nil even if restart was skipped.
	if node.RestartAttempts == nil {
		t.Error("tryLightweightRestart must initialise RestartAttempts map")
	}
}
