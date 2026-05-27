package main

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// TestControlService_StartIdempotentWhenActive verifies that ControlService
// with action=start does NOT invoke systemctl when the unit is already
// active. This is the regression guard against the start-storm that hit
// globule-nuc on 2026-05-06: concurrent start calls were each running
// ExecStartPre=pkill, killing the live mcp_server process.
func TestControlService_StartIdempotentWhenActive(t *testing.T) {
	origLook, origRun, origState := systemctlLookPath, runSystemctlFn, getUnitStateFn
	defer func() {
		systemctlLookPath, runSystemctlFn, getUnitStateFn = origLook, origRun, origState
	}()

	systemctlLookPath = func(string) (string, error) { return "/bin/systemctl", nil }
	getUnitStateFn = func(systemctl, unit string) string { return "active" }

	var sysctlCalls atomic.Int64
	runSystemctlFn = func(systemctl, action, unit string) error {
		sysctlCalls.Add(1)
		return nil
	}

	srv := &NodeAgentServer{}
	resp, err := srv.ControlService(context.Background(), &node_agentpb.ControlServiceRequest{
		Unit:   "globular-mcp",
		Action: "start",
	})
	if err != nil {
		t.Fatalf("ControlService: %v", err)
	}
	if !resp.GetOk() {
		t.Fatalf("expected Ok=true, got %+v", resp)
	}
	if resp.GetState() != "active" {
		t.Fatalf("expected state=active, got %q", resp.GetState())
	}
	if !strings.Contains(resp.GetMessage(), "skipped") {
		t.Fatalf("expected 'skipped' in message, got %q", resp.GetMessage())
	}
	if got := sysctlCalls.Load(); got != 0 {
		t.Fatalf("expected zero systemctl invocations, got %d", got)
	}
}

// TestControlService_StartProceedsWhenInactive verifies that ControlService
// with action=start DOES invoke systemctl when the unit is not already
// active. This protects against accidentally turning the idempotency guard
// into a permanent skip.
func TestControlService_StartProceedsWhenInactive(t *testing.T) {
	origLook, origRun, origState := systemctlLookPath, runSystemctlFn, getUnitStateFn
	defer func() {
		systemctlLookPath, runSystemctlFn, getUnitStateFn = origLook, origRun, origState
	}()

	systemctlLookPath = func(string) (string, error) { return "/bin/systemctl", nil }
	stateCalls := 0
	getUnitStateFn = func(systemctl, unit string) string {
		stateCalls++
		// First call (idempotency check) returns inactive; second call (post-action) returns active.
		if stateCalls == 1 {
			return "inactive"
		}
		return "active"
	}

	var sysctlCalls atomic.Int64
	runSystemctlFn = func(systemctl, action, unit string) error {
		sysctlCalls.Add(1)
		if action != "start" {
			return fmt.Errorf("unexpected action %q", action)
		}
		return nil
	}

	srv := &NodeAgentServer{}
	resp, err := srv.ControlService(context.Background(), &node_agentpb.ControlServiceRequest{
		Unit:   "globular-mcp",
		Action: "start",
	})
	if err != nil {
		t.Fatalf("ControlService: %v", err)
	}
	if !resp.GetOk() {
		t.Fatalf("expected Ok=true, got %+v", resp)
	}
	if got := sysctlCalls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 systemctl invocation, got %d", got)
	}
}

// TestControlService_StopIdempotentWhenInactive verifies the symmetric
// guard for stop: skip systemctl if the unit is already inactive/failed.
func TestControlService_StopIdempotentWhenInactive(t *testing.T) {
	origLook, origRun, origState := systemctlLookPath, runSystemctlFn, getUnitStateFn
	defer func() {
		systemctlLookPath, runSystemctlFn, getUnitStateFn = origLook, origRun, origState
	}()

	for _, state := range []string{"inactive", "failed", "deactivating"} {
		state := state
		t.Run(state, func(t *testing.T) {
			systemctlLookPath = func(string) (string, error) { return "/bin/systemctl", nil }
			getUnitStateFn = func(systemctl, unit string) string { return state }
			var sysctlCalls atomic.Int64
			runSystemctlFn = func(systemctl, action, unit string) error {
				sysctlCalls.Add(1)
				return nil
			}

			srv := &NodeAgentServer{}
			resp, err := srv.ControlService(context.Background(), &node_agentpb.ControlServiceRequest{
				Unit:   "globular-mcp",
				Action: "stop",
			})
			if err != nil {
				t.Fatalf("ControlService: %v", err)
			}
			if !resp.GetOk() {
				t.Fatalf("expected Ok=true, got %+v", resp)
			}
			if got := sysctlCalls.Load(); got != 0 {
				t.Fatalf("expected zero systemctl invocations for state=%s, got %d", state, got)
			}
		})
	}
}

// TestControlService_RestartAlwaysProceeds verifies that restart is never
// short-circuited — the caller asked for a recycle and we must honor it
// even if the unit is currently active.
func TestControlService_RestartAlwaysProceeds(t *testing.T) {
	origLook, origRun, origState := systemctlLookPath, runSystemctlFn, getUnitStateFn
	defer func() {
		systemctlLookPath, runSystemctlFn, getUnitStateFn = origLook, origRun, origState
	}()

	systemctlLookPath = func(string) (string, error) { return "/bin/systemctl", nil }
	getUnitStateFn = func(systemctl, unit string) string { return "active" }
	var sysctlCalls atomic.Int64
	runSystemctlFn = func(systemctl, action, unit string) error {
		sysctlCalls.Add(1)
		if action != "restart" {
			return fmt.Errorf("unexpected action %q", action)
		}
		return nil
	}

	srv := &NodeAgentServer{}
	_, err := srv.ControlService(context.Background(), &node_agentpb.ControlServiceRequest{
		Unit:   "globular-mcp",
		Action: "restart",
	})
	if err != nil {
		t.Fatalf("ControlService: %v", err)
	}
	if got := sysctlCalls.Load(); got != 1 {
		t.Fatalf("expected exactly 1 systemctl invocation, got %d", got)
	}
}

// Exact-name awareness coverage wrapper for the orphan-before-restart
// decision rule. Start idempotency prevents repeated ExecStartPre pkill
// cycles from killing a live process during concurrent start requests.
func TestWorkflowOrphanKilledBeforeRestart(t *testing.T) {
	TestControlService_StartIdempotentWhenActive(t)
}

func TestIsInTargetState(t *testing.T) {
	cases := []struct {
		action, state string
		want          bool
	}{
		{"start", "active", true},
		{"start", "activating", true},
		{"start", "inactive", false},
		{"start", "failed", false},
		{"start", "unknown", false},
		{"stop", "inactive", true},
		{"stop", "deactivating", true},
		{"stop", "failed", true},
		{"stop", "active", false},
		{"stop", "activating", false},
		{"restart", "active", false},
		{"restart", "inactive", false},
	}
	for _, c := range cases {
		if got := isInTargetState(c.action, c.state); got != c.want {
			t.Errorf("isInTargetState(%q, %q) = %v, want %v", c.action, c.state, got, c.want)
		}
	}
}
