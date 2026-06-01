// @awareness namespace=globular.platform
// @awareness component=platform_node_agent
// @awareness file_role=service_control_grpc_handler
// @awareness implements=globular.platform:intent.runtime_health.requires_live_observation
// @awareness risk=high
package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// allowedUnitPrefixes defines which systemd units can be controlled.
// keepalived ships with upstream's naming (keepalived.service rather than
// globular-keepalived.service) but is a Globular-managed package — the
// release pipeline applies it like any other infrastructure unit. Without
// this allow-listing, release.apply.package's verify_runtime step gets
// PermissionDenied from ControlService → the correlation defers up to
// max_defers (5) → workflow_server flood-loops "refusing dispatch
// (ABANDONED)" every ~30 ms forever.
var allowedUnitPrefixes = []string{
	"globular-",
	"scylla-server",
	"scylla-manager",
	"keepalived",
}

// runSystemctlFn and getUnitStateFn are seams for tests to substitute the
// real systemctl invocations. Production points at the real implementations
// in this file / server.go.
var (
	runSystemctlFn = runSystemctl
	getUnitStateFn = getUnitState
)

func isAllowedUnit(unit string) bool {
	for _, prefix := range allowedUnitPrefixes {
		if strings.HasPrefix(unit, prefix) {
			return true
		}
	}
	return false
}

// ControlService implements restart/stop/start/status for managed systemd units.
func (srv *NodeAgentServer) ControlService(ctx context.Context, req *node_agentpb.ControlServiceRequest) (*node_agentpb.ControlServiceResponse, error) {
	unit := strings.TrimSpace(req.GetUnit())
	action := strings.TrimSpace(strings.ToLower(req.GetAction()))

	if unit == "" {
		return nil, status.Error(codes.InvalidArgument, "unit is required")
	}
	if action == "" {
		return nil, status.Error(codes.InvalidArgument, "action is required")
	}

	// Ensure .service suffix
	if !strings.HasSuffix(unit, ".service") {
		unit = unit + ".service"
	}

	// Safety: only allow control of Globular-managed units
	if !isAllowedUnit(unit) {
		return nil, status.Errorf(codes.PermissionDenied, "unit %q is not a managed service — only globular-* and scylla-* units are allowed", unit)
	}

	// ── MinIO topology gate ───────────────────────────────────────────────
	// start/restart of globular-minio.service is only permitted on nodes that
	// are admitted into ObjectStoreDesiredState.Nodes. Reject before touching
	// systemctl so that no transient active window exists on non-members.
	// stop/status are always allowed (stopping a non-member is the right thing).
	if unit == "globular-minio.service" && (action == "start" || action == "restart") {
		state, loadErr := config.LoadObjectStoreDesiredState(ctx)
		if loadErr != nil {
			return nil, status.Errorf(codes.Unavailable,
				"minio topology gate: etcd unavailable — cannot verify pool membership before %s: %v", action, loadErr)
		}
		nodeIP := srv.nodeIP()
		if !nodeIPInPool(nodeIP, state) {
			return &node_agentpb.ControlServiceResponse{
				Ok:      false,
				Unit:    unit,
				Action:  action,
				State:   "held_not_in_topology",
				Message: fmt.Sprintf("minio topology gate: node ip=%s is not in ObjectStoreDesiredState.Nodes — %s rejected (run apply-topology first)", nodeIP, action),
			}, nil
		}
	}

	systemctl, err := systemctlLookPath("systemctl")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "systemctl not found: %v", err)
	}

	switch action {
	case "restart", "stop", "start":
		// Idempotency guard: skip systemctl when already in the target state.
		// Concurrent start invocations were causing a race storm on units with
		// destructive ExecStartPre (pkill of orphans), where each "start" of an
		// already-active unit would re-trigger ExecStartPre and kill the live
		// process. Restart is intentionally never skipped — the caller asked
		// for a recycle.
		if action == "start" || action == "stop" {
			state := getUnitStateFn(systemctl, unit)
			if isInTargetState(action, state) {
				return &node_agentpb.ControlServiceResponse{
					Ok:      true,
					Unit:    unit,
					Action:  action,
					State:   state,
					Message: fmt.Sprintf("%s skipped: already %s", action, state),
				}, nil
			}
		}

		if err := runSystemctlFn(systemctl, action, unit); err != nil {
			return &node_agentpb.ControlServiceResponse{
				Ok:      false,
				Unit:    unit,
				Action:  action,
				Message: fmt.Sprintf("%s failed: %v", action, err),
			}, nil
		}

		// Get state after action
		state := getUnitStateFn(systemctl, unit)
		return &node_agentpb.ControlServiceResponse{
			Ok:      true,
			Unit:    unit,
			Action:  action,
			State:   state,
			Message: fmt.Sprintf("%s completed successfully", action),
		}, nil

	case "status":
		state := getUnitStateFn(systemctl, unit)
		return &node_agentpb.ControlServiceResponse{
			Ok:      true,
			Unit:    unit,
			Action:  action,
			State:   state,
			Message: state,
		}, nil

	default:
		return nil, status.Errorf(codes.InvalidArgument, "invalid action %q — use: restart, stop, start, status", action)
	}
}

// getUnitState returns the ActiveState of a systemd unit.
func getUnitState(systemctl, unit string) string {
	out, err := exec.Command(systemctl, "show", "--property=ActiveState", "--value", unit).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// isInTargetState reports whether the unit's current ActiveState already
// satisfies the requested action and the systemctl call can be skipped.
//
// "start"  → active or activating already produces the desired outcome
// "stop"   → inactive, deactivating, or failed already produces the desired outcome
//
// "unknown" is treated as not-in-target — let systemctl run and report the
// real failure.
func isInTargetState(action, state string) bool {
	switch action {
	case "start":
		return state == "active" || state == "activating"
	case "stop":
		return state == "inactive" || state == "deactivating" || state == "failed"
	}
	return false
}
