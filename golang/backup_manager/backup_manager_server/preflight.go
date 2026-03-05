package main

import (
	"context"
	"os/exec"
	"strings"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

// PreflightCheck verifies that required CLI tools are available.
func (srv *server) PreflightCheck(ctx context.Context, rqst *backup_managerpb.PreflightCheckRequest) (*backup_managerpb.PreflightCheckResponse, error) {
	tools := []struct {
		name    string
		versionArgs []string
	}{
		{"etcdctl", []string{"version"}},
		{"restic", []string{"version"}},
		{"rclone", []string{"version"}},
		{"sctool", []string{"version"}},
		{"sha256sum", []string{"--version"}},
	}

	var checks []*backup_managerpb.ToolCheck
	allOk := true

	for _, t := range tools {
		check := checkTool(t.name, t.versionArgs)
		checks = append(checks, check)
		if !check.Available {
			allOk = false
		}
	}

	return &backup_managerpb.PreflightCheckResponse{
		Tools: checks,
		AllOk: allOk,
	}, nil
}

func checkTool(name string, versionArgs []string) *backup_managerpb.ToolCheck {
	check := &backup_managerpb.ToolCheck{Name: name}

	path, err := exec.LookPath(name)
	if err != nil {
		check.Available = false
		check.ErrorMessage = "not found in PATH"
		return check
	}

	check.Available = true
	check.Path = path

	// Get version
	cmd := exec.Command(name, versionArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		check.Version = "unknown"
		check.ErrorMessage = err.Error()
	} else {
		// Take first line of version output
		lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
		if len(lines) > 0 {
			check.Version = lines[0]
		}
	}

	return check
}
