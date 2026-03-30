package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// allowedUnitPrefixes defines which systemd units can be controlled.
var allowedUnitPrefixes = []string{
	"globular-",
	"scylla-server",
	"scylla-manager",
}

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

	systemctl, err := systemctlLookPath("systemctl")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "systemctl not found: %v", err)
	}

	switch action {
	case "restart", "stop", "start":
		if err := runSystemctl(systemctl, action, unit); err != nil {
			return &node_agentpb.ControlServiceResponse{
				Ok:      false,
				Unit:    unit,
				Action:  action,
				Message: fmt.Sprintf("%s failed: %v", action, err),
			}, nil
		}

		// Get state after action
		state := getUnitState(systemctl, unit)
		return &node_agentpb.ControlServiceResponse{
			Ok:      true,
			Unit:    unit,
			Action:  action,
			State:   state,
			Message: fmt.Sprintf("%s completed successfully", action),
		}, nil

	case "status":
		state := getUnitState(systemctl, unit)
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
