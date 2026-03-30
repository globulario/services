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

// SearchServiceLogs searches journalctl logs with time range, pattern, and severity filtering.
func (srv *NodeAgentServer) SearchServiceLogs(ctx context.Context, req *node_agentpb.SearchServiceLogsRequest) (*node_agentpb.SearchServiceLogsResponse, error) {
	unit := strings.TrimSpace(req.GetUnit())
	if unit == "" {
		return nil, status.Error(codes.InvalidArgument, "unit is required")
	}
	if !strings.HasPrefix(unit, "globular-") && !strings.HasPrefix(unit, "scylla") {
		return nil, status.Errorf(codes.PermissionDenied, "unit %q not allowed — only globular-* and scylla-* units", unit)
	}

	// Ensure .service suffix
	if !strings.HasSuffix(unit, ".service") && !strings.HasSuffix(unit, ".timer") {
		unit = unit + ".service"
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	// Build journalctl command
	args := []string{
		"--no-pager",
		"--output=short-iso",
		"-u", unit,
		"-n", fmt.Sprintf("%d", limit+1), // +1 to detect truncation
	}

	if req.GetSince() != "" {
		args = append(args, "--since", req.GetSince())
	}
	if req.GetUntil() != "" {
		args = append(args, "--until", req.GetUntil())
	}
	if req.GetPriority() != "" {
		args = append(args, "-p", req.GetPriority())
	}
	if req.GetPattern() != "" {
		args = append(args, "--grep", req.GetPattern())
		if !req.GetCaseSensitive() {
			args = append(args, "--case-sensitive=no")
		}
	}

	cmd := exec.CommandContext(ctx, "journalctl", args...)
	output, err := cmd.Output()
	if err != nil {
		// journalctl returns exit 1 when no entries match — not an error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return &node_agentpb.SearchServiceLogsResponse{
				Unit:       unit,
				MatchCount: 0,
				Lines:      nil,
				Since:      req.GetSince(),
				Until:      req.GetUntil(),
			}, nil
		}
		return nil, status.Errorf(codes.Internal, "journalctl: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}

	truncated := false
	if len(lines) > limit {
		truncated = true
		lines = lines[:limit]
	}

	return &node_agentpb.SearchServiceLogsResponse{
		Unit:       unit,
		MatchCount: int32(len(lines)),
		Lines:      lines,
		Since:      req.GetSince(),
		Until:      req.GetUntil(),
		Truncated:  truncated,
	}, nil
}
