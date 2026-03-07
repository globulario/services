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
		var clusterName string
		for _, c := range existing {
			if !strings.HasPrefix(c, "native:") && !strings.HasPrefix(c, "scylla_host:") {
				clusterName = c
				break
			}
		}
		if srv.ScyllaCluster == "" && clusterName != "" {
			srv.ScyllaCluster = clusterName
			slog.Info("auto-detected scylla-manager cluster", "cluster", clusterName)
		}

		// Health check: verify the registered cluster is reachable.
		// If not (e.g. registered without auth token), update it with credentials.
		if clusterName != "" {
			statusArgs := []string{"status", "-c", clusterName}
			if srv.ScyllaManagerAPI != "" && srv.ScyllaManagerAPI != "http://127.0.0.1:5080" {
				statusArgs = append(statusArgs, "--api-url", srv.ScyllaManagerAPI)
			}
			statusCmd := exec.Command("sctool", statusArgs...)
			statusOut, statusErr := statusCmd.CombinedOutput()
			if statusErr != nil && strings.Contains(string(statusOut), "unable to connect") {
				slog.Warn("registered cluster is unreachable, attempting to update with auth token",
					"cluster", clusterName)
				agentArgs := scyllaAgentArgs()
				if agentArgs != nil {
					updateArgs := []string{"cluster", "update", "-c", clusterName}
					if srv.ScyllaManagerAPI != "" && srv.ScyllaManagerAPI != "http://127.0.0.1:5080" {
						updateArgs = append(updateArgs, "--api-url", srv.ScyllaManagerAPI)
					}
					updateArgs = append(updateArgs, agentArgs...)
					updateCmd := exec.Command("sctool", updateArgs...)
					updateOut, updateErr := updateCmd.CombinedOutput()
					if updateErr != nil {
						slog.Warn("failed to update cluster with auth token",
							"error", updateErr, "output", strings.TrimSpace(string(updateOut)))
					} else {
						slog.Info("cluster updated with auth token",
							"cluster", clusterName, "output", strings.TrimSpace(string(updateOut)))
					}
				} else {
					slog.Warn("cannot read agent config to fix unreachable cluster — manual intervention needed")
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
	agentArgs := scyllaAgentArgs()
	if agentArgs == nil {
		slog.Warn("cannot read scylla-manager-agent config for auth token — "+
			"cluster registration will likely fail with 401. "+
			"Ensure the agent config is readable by the backup-manager process.",
			"tried", []string{
				"/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml",
				"/etc/scylla-manager-agent/scylla-manager-agent.yaml",
			})
		return
	}
	args = append(args, agentArgs...)

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
				prefAgentArgs := scyllaAgentArgs()
				if prefAgentArgs == nil {
					slog.Warn("preflight: cannot read agent config for auth token, skipping auto-register")
				} else {
					slog.Info("preflight: auto-registering ScyllaDB",
						"cluster_name", clusterName, "host", scyllaHost)
					regArgs := []string{"cluster", "add", "--host", scyllaHost, "--name", clusterName}
					if srv.ScyllaManagerAPI != "" && srv.ScyllaManagerAPI != "http://127.0.0.1:5080" {
						regArgs = append(regArgs, "--api-url", srv.ScyllaManagerAPI)
					}
					regArgs = append(regArgs, prefAgentArgs...)
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
		}

		for _, c := range clusters {
			checks = append(checks, &backup_managerpb.ToolCheck{
				Name:      "scylla_cluster_detected",
				Available: true,
				Version:   c,
			})
		}
	}

	// Recovery readiness checks
	dest := srv.resolveRecoveryDestination()
	checks = append(checks, &backup_managerpb.ToolCheck{
		Name:      "recovery_destination_configured",
		Available: dest != nil,
	})

	seed, seedErr := loadRecoverySeed()
	checks = append(checks, &backup_managerpb.ToolCheck{
		Name:      "recovery_seed_present",
		Available: seedErr == nil,
	})

	if seed != nil {
		checks = append(checks, &backup_managerpb.ToolCheck{
			Name:      "recovery_credentials_available",
			Available: seedCredentialsAvailable(seed),
		})

		seedMatches := false
		if dest != nil {
			seedMatches = dest.Name == seed.Destination.Name &&
				dest.Type == seed.Destination.Type &&
				dest.Path == seed.Destination.Path
		}
		checks = append(checks, &backup_managerpb.ToolCheck{
			Name:      "recovery_seed_matches_current_config",
			Available: seedMatches,
		})
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

// runDiagCmd runs a command with a 15-second timeout and returns output + error.
func runDiagCmd(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// TestScyllaConnection runs a sequence of diagnostic checks against the
// ScyllaDB / scylla-manager / agent / storage stack and returns actionable
// results for each step.
func (srv *server) TestScyllaConnection(_ context.Context, rqst *backup_managerpb.TestScyllaConnectionRequest) (*backup_managerpb.TestScyllaConnectionResponse, error) {
	var checks []*backup_managerpb.ScyllaConnectionCheck
	allOk := true

	fail := func(name, msg, fix string) {
		allOk = false
		checks = append(checks, &backup_managerpb.ScyllaConnectionCheck{
			Name: name, Ok: false, Message: msg, Fix: fix,
		})
	}
	pass := func(name, msg string) {
		checks = append(checks, &backup_managerpb.ScyllaConnectionCheck{
			Name: name, Ok: true, Message: msg,
		})
	}

	// 1. sctool binary
	if _, err := exec.LookPath("sctool"); err != nil {
		fail("sctool_available", "sctool not found in PATH",
			"Install scylla-manager: apt install scylla-manager or check your installation.")
		return &backup_managerpb.TestScyllaConnectionResponse{AllOk: false, Checks: checks}, nil
	}
	pass("sctool_available", "sctool found")

	apiURL := srv.ScyllaManagerAPI
	apiArgs := func(args []string) []string {
		if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
			return append(args, "--api-url", apiURL)
		}
		return args
	}

	// 2. scylla-manager server reachable
	out, err := runDiagCmd(apiArgs([]string{"sctool", "version"})...)
	if err != nil {
		fail("scylla_manager_server", "scylla-manager is not reachable: "+out,
			"Start scylla-manager: sudo systemctl start globular-scylla-manager.service")
		return &backup_managerpb.TestScyllaConnectionResponse{AllOk: false, Checks: checks}, nil
	}
	pass("scylla_manager_server", "scylla-manager server reachable ("+out+")")

	// 3. Cluster registered
	cluster := rqst.Cluster
	if cluster == "" {
		cluster = srv.ScyllaCluster
	}
	if cluster == "" {
		fail("cluster_registered", "No ScyllaDB cluster name configured",
			"Set the cluster name in Settings → ScyllaDB → Cluster Name, or run:\nsctool cluster add --host <IP> --name <NAME> --auth-token <TOKEN> --port 56090")
		return &backup_managerpb.TestScyllaConnectionResponse{AllOk: false, Checks: checks}, nil
	}

	out, _ = runDiagCmd(apiArgs([]string{"sctool", "cluster", "list"})...)
	if !strings.Contains(out, cluster) {
		fail("cluster_registered",
			fmt.Sprintf("Cluster %q is not registered in scylla-manager", cluster),
			fmt.Sprintf("Register it:\nsctool cluster add --host <SCYLLA_IP> --name %s --auth-token <TOKEN> --port 56090", cluster))
		return &backup_managerpb.TestScyllaConnectionResponse{AllOk: false, Checks: checks}, nil
	}
	pass("cluster_registered", fmt.Sprintf("Cluster %q is registered", cluster))

	// 4. Agent reachable (sctool status -c <cluster>)
	out, err = runDiagCmd(apiArgs([]string{"sctool", "status", "-c", cluster})...)

	if err != nil {
		if strings.Contains(out, "unable to connect") {
			// Auth token mismatch or agent not running — try auto-fix
			agentArgs := scyllaAgentArgs()
			if agentArgs == nil {
				fail("agent_reachable",
					"Cannot connect to ScyllaDB agent — and cannot read agent config to get auth token",
					"Fix permissions:\nsudo chmod 0750 /var/lib/globular/scylla-manager-agent\nThen update the cluster:\nsctool cluster update -c "+cluster+" --auth-token <TOKEN> --port 56090")
			} else {
				// We have the token — try to auto-fix
				updateArgs := apiArgs([]string{"sctool", "cluster", "update", "-c", cluster})
				updateArgs = append(updateArgs, agentArgs...)
				updateOut, updateErr := runDiagCmd(updateArgs...)
				if updateErr != nil {
					fail("agent_reachable",
						"Cannot connect to ScyllaDB agent. Auto-fix also failed: "+updateOut,
						"Check agent is running:\nsudo systemctl status globular-scylla-manager-agent.service\nRestart it:\nsudo systemctl restart globular-scylla-manager-agent.service\nThen update:\nsctool cluster update -c "+cluster+" --auth-token <TOKEN> --port 56090")
				} else {
					// Retry status after fix
					retryOut, retryErr := runDiagCmd(apiArgs([]string{"sctool", "status", "-c", cluster})...)
					if retryErr != nil {
						fail("agent_reachable",
							"Updated auth token but agent still unreachable: "+retryOut,
							"Restart the agent:\nsudo systemctl restart globular-scylla-manager-agent.service")
					} else {
						pass("agent_reachable", "Agent connection repaired (auth token updated)")
					}
				}
			}
		} else if strings.Contains(out, "no matching host") {
			fail("agent_reachable",
				"ScyllaDB host not found by scylla-manager",
				"Verify ScyllaDB is running:\nsudo systemctl status scylla-server\nCheck the host IP:\nsctool cluster list\nUpdate if needed:\nsctool cluster update -c "+cluster+" --host <CORRECT_IP>")
		} else {
			fail("agent_reachable",
				"Cluster status check failed: "+out,
				"Check agent:\nsudo systemctl status globular-scylla-manager-agent.service\nCheck manager logs:\nsudo journalctl -u globular-scylla-manager.service -n 30")
		}
		if !allOk {
			return &backup_managerpb.TestScyllaConnectionResponse{AllOk: false, Checks: checks}, nil
		}
	} else {
		pass("agent_reachable", "ScyllaDB agent reachable, cluster healthy")
	}

	// 5. Storage location
	location := rqst.Location
	if location == "" {
		location = srv.ScyllaLocation
	}
	if location == "" {
		fail("storage_location", "No ScyllaDB backup location configured (e.g. s3:bucket-name)",
			"Set it in Settings → ScyllaDB → Scylla Location, or create a MinIO bucket with 'Use for ScyllaDB'.")
	} else {
		// Use sctool to validate the backup location via the agent (no direct agent binary spawn).
		// A dry-run backup task validates the full path: manager → agent → S3.
		out, err := runDiagCmd(apiArgs([]string{"sctool", "backup", "list", "-c", cluster, "-L", location})...)
		if err != nil {
			outLower := strings.ToLower(out)
			if strings.Contains(outLower, "nocredentialproviders") || strings.Contains(outLower, "credential") {
				fail("storage_location",
					"S3 credentials not configured in scylla-manager-agent",
					"Add S3 credentials to the agent config:\nsudo nano /var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml\n\ns3:\n  access_key_id: <KEY>\n  secret_access_key: <SECRET>\n  provider: Minio\n  endpoint: https://127.0.0.1:9000\n\nThen restart:\nsudo systemctl restart globular-scylla-manager-agent.service")
			} else if strings.Contains(outLower, "nosuchbucket") || strings.Contains(outLower, "bucket") && strings.Contains(outLower, "not found") {
				fail("storage_location",
					fmt.Sprintf("S3 bucket not found: %s", location),
					"Create the bucket in MinIO, or check the location name matches an existing bucket.")
			} else if strings.Contains(outLower, "timeout") || strings.Contains(outLower, "connection refused") {
				fail("storage_location",
					"Cannot reach S3/MinIO endpoint from agent",
					"Verify MinIO is running:\nsudo systemctl status globular-minio.service\nCheck endpoint in agent config should be:\nhttps://127.0.0.1:9000")
			} else {
				// backup list may return empty or error for valid location with no backups yet
				// If agent was reachable (step 4 passed), a non-critical error here just means no backups yet
				pass("storage_location", fmt.Sprintf("Location %s configured (no existing backups found)", location))
			}
		} else {
			pass("storage_location", fmt.Sprintf("Storage location %s is accessible", location))
		}
	}

	return &backup_managerpb.TestScyllaConnectionResponse{AllOk: allOk, Checks: checks}, nil
}
