package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

// scyllaAgentArgs reads the scylla-manager-agent config to extract auth_token
// and HTTPS port, returning sctool flags like --auth-token and --port.
func scyllaAgentArgs() []string {
	configPaths := []string{
		"/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml",
		"/etc/scylla-manager-agent/scylla-manager-agent.yaml",
	}
	var data []byte
	for _, p := range configPaths {
		var err error
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if len(data) == 0 {
		return nil
	}

	var args []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "auth_token:") {
			token := strings.TrimSpace(strings.TrimPrefix(line, "auth_token:"))
			if token != "" {
				args = append(args, "--auth-token", token)
			}
		}
		if strings.HasPrefix(line, "https:") {
			addr := strings.TrimSpace(strings.TrimPrefix(line, "https:"))
			if _, portStr, err := net.SplitHostPort(addr); err == nil && portStr != "10001" {
				args = append(args, "--port", portStr)
			}
		}
	}
	return args
}

// ensureScyllaRegistered checks if ScyllaDB is running and registered in
// scylla-manager. If ScyllaDB is reachable but not registered, it auto-registers
// using the Globular domain as the cluster name.
// This runs in background at startup — failures are non-fatal.
func (srv *server) ensureScyllaRegistered() {
	// Wait a bit for scylla-manager to be ready
	time.Sleep(5 * time.Second)

	// Check if sctool is available
	if _, err := exec.LookPath("sctool"); err != nil {
		return
	}

	// Check if any cluster is already registered
	existing := detectScyllaClusters(srv.ScyllaManagerAPI)
	managed := 0
	for _, c := range existing {
		if !strings.HasPrefix(c, "native:") {
			managed++
		}
	}
	if managed > 0 {
		// Already registered — auto-fill ScyllaCluster if empty
		if srv.ScyllaCluster == "" {
			for _, c := range existing {
				if !strings.HasPrefix(c, "native:") {
					srv.ScyllaCluster = c
					slog.Info("auto-detected scylla-manager cluster", "cluster", c)
					break
				}
			}
		}
		return
	}

	// No clusters registered — check if ScyllaDB is actually running
	scyllaHost, nativeName := detectNativeScyllaDB()
	if nativeName == "" {
		// ScyllaDB not reachable, nothing to register
		return
	}

	// Use Globular domain as the scylla-manager cluster name
	clusterName := srv.Domain
	if clusterName == "" {
		clusterName = nativeName
	}

	slog.Info("auto-registering ScyllaDB in scylla-manager",
		"cluster_name", clusterName, "host", scyllaHost, "native_name", nativeName)

	args := []string{"cluster", "add", "--host", scyllaHost, "--name", clusterName}
	if srv.ScyllaManagerAPI != "" && srv.ScyllaManagerAPI != "http://127.0.0.1:5080" {
		args = append(args, "--api-url", srv.ScyllaManagerAPI)
	}
	args = append(args, scyllaAgentArgs()...)

	slog.Info("auto-registering ScyllaDB in scylla-manager",
		"cluster_name", clusterName, "host", scyllaHost, "args", strings.Join(args, " "))

	cmd := exec.Command("sctool", args...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		// Check if it failed because it's already registered (race condition)
		if strings.Contains(output, "already exists") || strings.Contains(output, "conflict") {
			slog.Info("ScyllaDB cluster already registered", "output", output)
		} else {
			slog.Warn("failed to auto-register ScyllaDB in scylla-manager",
				"error", err, "output", output)
		}
		return
	}

	slog.Info("ScyllaDB auto-registered in scylla-manager",
		"cluster_name", clusterName, "output", output)

	// Update config so backup provider can use it
	if srv.ScyllaCluster == "" {
		srv.ScyllaCluster = clusterName
	}
}

// PreflightCheck verifies that required CLI tools are available.
// It also detects infrastructure configuration (e.g. ScyllaDB cluster names)
// and returns them as synthetic ToolCheck entries with names like "scylla_cluster_detected".
func (srv *server) PreflightCheck(ctx context.Context, rqst *backup_managerpb.PreflightCheckRequest) (*backup_managerpb.PreflightCheckResponse, error) {
	tools := []struct {
		name        string
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
	sctoolAvailable := false

	for _, t := range tools {
		check := checkTool(t.name, t.versionArgs)
		checks = append(checks, check)
		if !check.Available {
			allOk = false
		}
		if t.name == "sctool" && check.Available {
			sctoolAvailable = true
		}
	}

	// Detect ScyllaDB cluster names if sctool is available
	if sctoolAvailable {
		clusters := detectScyllaClusters(srv.ScyllaManagerAPI)

		// If only native detection (not registered), auto-register now
		hasManaged := false
		for _, c := range clusters {
			if !strings.HasPrefix(c, "native:") && !strings.HasPrefix(c, "scylla_host:") {
				hasManaged = true
				break
			}
		}
		if !hasManaged {
			scyllaHost, nativeName := detectNativeScyllaDB()
			if nativeName != "" {
				clusterName := srv.Domain
				if clusterName == "" {
					clusterName = nativeName
				}
				slog.Info("preflight: auto-registering ScyllaDB",
					"cluster_name", clusterName, "host", scyllaHost)
				regArgs := []string{"cluster", "add", "--host", scyllaHost, "--name", clusterName}
				if srv.ScyllaManagerAPI != "" && srv.ScyllaManagerAPI != "http://127.0.0.1:5080" {
					regArgs = append(regArgs, "--api-url", srv.ScyllaManagerAPI)
				}
				regArgs = append(regArgs, scyllaAgentArgs()...)
				cmd := exec.Command("sctool", regArgs...)
				out, err := cmd.CombinedOutput()
				output := strings.TrimSpace(string(out))
				if err != nil && !strings.Contains(output, "already exists") && !strings.Contains(output, "conflict") {
					slog.Warn("preflight: auto-register failed", "error", err, "output", output)
				} else {
					slog.Info("preflight: ScyllaDB registered", "cluster_name", clusterName, "output", output)
					if srv.ScyllaCluster == "" {
						srv.ScyllaCluster = clusterName
					}
					// Re-detect now that it's registered
					clusters = detectScyllaClusters(srv.ScyllaManagerAPI)
				}
			}
		}

		for _, c := range clusters {
			checks = append(checks, &backup_managerpb.ToolCheck{
				Name:      "scylla_cluster_detected",
				Available: true,
				Version:   c,
			})
		}
	}

	return &backup_managerpb.PreflightCheckResponse{
		Tools: checks,
		AllOk: allOk,
	}, nil
}

// detectScyllaClusters runs "sctool cluster list" and parses registered cluster names.
// Also attempts to detect the native ScyllaDB cluster name via its REST API
// (port 10000) in case no clusters are registered in scylla-manager yet.
func detectScyllaClusters(apiURL string) []string {
	var clusters []string

	// 1. Try sctool cluster list (scylla-manager registered clusters)
	args := []string{"cluster", "list"}
	if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
		args = append(args, "--api-url", apiURL)
	}

	cmd := exec.Command("sctool", args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		clusters = parseScyllaClusterList(string(out))
	}

	// 2. If no scylla-manager clusters found, try detecting native ScyllaDB
	//    cluster name via the REST API (default port 10000).
	//    ScyllaDB may be bound to a specific IP (not localhost), so try all
	//    local interface addresses.
	if len(clusters) == 0 {
		scyllaHost, nativeName := detectNativeScyllaDB()
		if nativeName != "" {
			clusters = append(clusters, "native:"+nativeName)
			// Also include the host so the UI can use it for registration
			clusters = append(clusters, "scylla_host:"+scyllaHost)
		}
	}

	return clusters
}

// detectNativeScyllaDB tries to reach the ScyllaDB REST API (port 10000) on
// localhost and all local interface IPs. Returns the reachable host and cluster name.
func detectNativeScyllaDB() (host, clusterName string) {
	// Build list of addresses to try: localhost first, then all local IPs
	candidates := []string{"127.0.0.1"}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				ip := ipnet.IP
				if ip.IsLoopback() || ip.To4() == nil {
					continue
				}
				candidates = append(candidates, ip.String())
			}
		}
	}

	for _, addr := range candidates {
		url := fmt.Sprintf("http://%s:10000/storage_service/cluster_name", addr)
		cmd := exec.Command("curl", "-sf", "--connect-timeout", "2", url)
		out, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(out))
		name = strings.Trim(name, `"`)
		if name != "" {
			return addr, name
		}
	}
	return "", ""
}

// parseScyllaClusterList extracts cluster names from sctool cluster list output.
// The output is a table with pipe-separated columns. We look for the "Name" column
// in the header and extract values from data rows.
func parseScyllaClusterList(output string) []string {
	var clusters []string
	lines := strings.Split(output, "\n")

	// Find the name column index by locating "Name" in a header-like row
	nameColIdx := -1
	headerLineIdx := -1

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for separator: │ or | (sctool uses both depending on version)
		var sep string
		if strings.Contains(line, "│") {
			sep = "│"
		} else if strings.Contains(line, "|") {
			sep = "|"
		} else {
			continue
		}

		parts := strings.Split(line, sep)
		for j, p := range parts {
			if strings.TrimSpace(p) == "Name" {
				nameColIdx = j
				headerLineIdx = i
				break
			}
		}
		if nameColIdx >= 0 {
			break
		}
	}

	if nameColIdx < 0 {
		return nil
	}

	// Parse data rows after the header
	for i := headerLineIdx + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Skip border/separator lines
		isBorder := true
		for _, r := range line {
			if r != '─' && r != '┼' && r != '├' && r != '┤' && r != '╰' && r != '╯' &&
				r != '┴' && r != '-' && r != '+' && r != ' ' {
				isBorder = false
				break
			}
		}
		if isBorder {
			continue
		}

		var sep string
		if strings.Contains(line, "│") {
			sep = "│"
		} else if strings.Contains(line, "|") {
			sep = "|"
		} else {
			continue
		}

		parts := strings.Split(line, sep)
		if nameColIdx < len(parts) {
			name := strings.TrimSpace(parts[nameColIdx])
			if name != "" {
				clusters = append(clusters, name)
			}
		}
	}

	return clusters
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

	// Get version — some tools (e.g. sctool) need their daemon running
	// to report a version. If the binary exists but version fails, still
	// consider it available; the error is informational only.
	cmd := exec.Command(name, versionArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		check.Version = "installed (version unavailable)"
		// Only surface as error_message for tools that truly need their
		// daemon (sctool). For others the exit error is meaningful.
		switch name {
		case "sctool":
			check.ErrorMessage = "sctool found but scylla-manager may not be running"
		default:
			check.ErrorMessage = err.Error()
		}
	} else {
		// Take first line of version output
		lines := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)
		if len(lines) > 0 {
			check.Version = lines[0]
		}
	}

	return check
}
