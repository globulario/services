package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
)

// persistScyllaConfig writes auto-detected ScyllaDB config back to etcd. The
// startup detection/registration sets srv fields in memory only; without this
// the registered cluster is invisible to the configured backup path on the
// next process start (and TestScyllaConnection / preflight report it as
// "not configured" even though scylla-manager knows about it). Best-effort —
// a persistence failure is logged, not fatal.
func (srv *server) persistScyllaConfig() {
	if err := globular.SaveService(srv); err != nil {
		slog.Warn("failed to persist auto-detected scylla config to etcd", "error", err)
		return
	}
	slog.Info("persisted auto-detected scylla config to etcd",
		"cluster", srv.ScyllaCluster, "api_url", srv.ScyllaManagerAPI, "location", srv.ScyllaLocation)
}

// scyllaManagerAPIHost extracts the bare host from a Scylla Manager API URL
// (e.g. "http://10.0.0.63:5080/api/v1" → "10.0.0.63"). Returns the original
// string when parsing fails so the LAN check can still reason about it.
func scyllaManagerAPIHost(apiURL string) string {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		return ""
	}
	if u, err := url.Parse(apiURL); err == nil && u.Host != "" {
		host, _, splitErr := net.SplitHostPort(u.Host)
		if splitErr == nil {
			return host
		}
		return u.Host
	}
	// Best-effort: not a full URL, strip a possible port.
	host, _, splitErr := net.SplitHostPort(apiURL)
	if splitErr == nil {
		return host
	}
	return apiURL
}

// scyllaAgentArgs reads the scylla-manager-agent config to extract auth_token
// and HTTPS port, returning sctool flags like --auth-token and --port.
func scyllaAgentArgs() []string {
	cfg := readScyllaAgentConfig()
	if cfg.ReadErr != nil {
		return nil
	}
	var args []string
	if cfg.AuthToken != "" {
		args = append(args, "--auth-token", cfg.AuthToken)
	}
	if cfg.HTTPSPort != "" && cfg.HTTPSPort != "10001" {
		args = append(args, "--port", cfg.HTTPSPort)
	}
	return args
}

// isInvalidScyllaManagerAPIURL rejects persisted API URLs whose host is not a
// routable LAN identity. The scylla-manager server is a cluster service, so a
// loopback / localhost / wildcard / link-local endpoint is never a valid
// authority surface to persist or feed into sctool.
func isInvalidScyllaManagerAPIURL(u string) bool {
	host := scyllaManagerAPIHost(u)
	if strings.TrimSpace(host) == "" {
		return false
	}
	return config.ValidateLANAddress(host) != nil
}

// detectScyllaManagerAPIURL reads the scylla-manager config yaml and returns
// the API URL with /api/v1 appended (e.g. "http://10.0.0.63:5080/api/v1").
// Prefers https: over http: when both are present.
// Returns empty string if the config is not found or has no listener field.
func detectScyllaManagerAPIURL() string {
	return readScyllaManagerEndpoint().URL
}

// resolveWildcardAddr replaces a 0.0.0.0 bind address with the node's
// outbound LAN IP so the URL is usable as a client endpoint.
func resolveWildcardAddr(hostport string) string {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	if host == "0.0.0.0" || host == "" {
		if ip := outboundIP(); ip != "" {
			return net.JoinHostPort(ip, port)
		}
	}
	return hostport
}

// outboundIP returns the preferred outbound LAN IP by dialling a dummy UDP
// connection. Returns empty string on failure.
func outboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:53")
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String()
}

// ensureScyllaRegistered checks if ScyllaDB is running and registered in
// scylla-manager. If ScyllaDB is reachable but not registered, it auto-registers
// using the Globular domain as the cluster name.
// This runs in background at startup — failures are non-fatal.
func (srv *server) ensureScyllaRegistered() {
	// Retry with backoff: a single miss at startup (Scylla REST API not yet
	// serving, the manager HTTPS listener still coming up, a transient sctool
	// error) must NOT leave the cluster permanently unregistered — that is how a
	// Day-0 reinstall ended up with cluster_count=0. Bounded so a node with no
	// local ScyllaDB does not loop forever.
	for attempt := 0; attempt < 30; attempt++ {
		wait := 60
		switch {
		case attempt == 0:
			wait = 5
		case attempt < 4:
			wait = 15
		case attempt < 8:
			wait = 30
		}
		time.Sleep(time.Duration(wait) * time.Second)
		if srv.tryRegisterScylla() {
			return
		}
	}
	slog.Warn("scylla auto-registration still incomplete after retries — " +
		"run `sctool cluster add` or restart backup_manager once ScyllaDB is reachable")
}

// tryRegisterScylla performs one registration attempt using the CORRECT
// parameters — the manager's HTTPS + LAN-IP API URL (never a loopback / non-LAN
// endpoint) and the node's routable IP for the Scylla REST probe. Returns true
// when the cluster is registered or nothing more can be done; false to retry.
func (srv *server) tryRegisterScylla() bool {
	// Check if sctool is available
	if _, err := execLookPath("sctool"); err != nil {
		return true // sctool absent — retrying will not help
	}

	managerEndpoint := readScyllaManagerEndpoint()
	if managerEndpoint.URL == "" {
		slog.Warn("scylla_manager.registration.skipped.manager_unreachable",
			"reason", "manager_endpoint_missing",
			"config_path", managerEndpoint.Path)
		return false
	}
	if isInvalidScyllaManagerAPIURL(managerEndpoint.URL) {
		slog.Warn("scylla_manager.registration.skipped.manager_unreachable",
			"reason", "manager_endpoint_invalid",
			"endpoint", managerEndpoint.URL,
			"config_path", managerEndpoint.Path)
		return false
	}
	if !endpointReachable(managerEndpoint.URL) {
		slog.Warn("scylla_manager.registration.skipped.manager_unreachable",
			"reason", "manager_endpoint_unreachable",
			"endpoint", managerEndpoint.URL,
			"scheme", managerEndpoint.Scheme,
			"config_path", managerEndpoint.Path)
		return false
	}
	if srv.ScyllaManagerAPI != managerEndpoint.URL {
		srv.ScyllaManagerAPI = managerEndpoint.URL
		slog.Info("auto-detected scylla-manager API URL", "url", srv.ScyllaManagerAPI)
	}

	// Clean up duplicate cluster entries (accumulate across wipe+Day0 cycles).
	srv.deduplicateScyllaManagerClusters(srv.ScyllaManagerAPI)

	// Check if any cluster is already registered
	existing := srv.detectScyllaClusters(srv.ScyllaManagerAPI)
	registered := realRegisteredClusters(existing)
	if len(registered) > 0 {
		// Already registered — auto-fill ScyllaCluster if empty
		clusterName := registered[0]
		if srv.ScyllaCluster == "" && clusterName != "" {
			srv.ScyllaCluster = clusterName
			slog.Info("auto-detected scylla-manager cluster", "cluster", clusterName)
			// Persist so the registered cluster is visible to the configured
			// backup path (and preflight/TestScyllaConnection) on next start,
			// instead of being re-derived in memory every time.
			srv.persistScyllaConfig()
		}

		// Health check: verify the registered cluster is reachable.
		// If not (e.g. registered without auth token), update it with credentials.
		if clusterName != "" {
			statusArgs := append([]string{"status", "-c", clusterName}, srv.scyllaAPIArgs(srv.ScyllaManagerAPI)...)
			statusCmd := execCommand("sctool", statusArgs...)
			statusOut, statusErr := statusCmd.CombinedOutput()
			if statusErr != nil && strings.Contains(string(statusOut), "unable to connect") {
				slog.Warn("registered cluster is unreachable, attempting to update with auth token",
					"cluster", clusterName)
				agentArgs := scyllaAgentArgs()
				if agentArgs != nil {
					updateArgs := append([]string{"cluster", "update", "-c", clusterName}, srv.scyllaAPIArgs(srv.ScyllaManagerAPI)...)
					updateArgs = append(updateArgs, agentArgs...)
					updateCmd := execCommand("sctool", updateArgs...)
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
		slog.Info("scylla_manager.registration.already_registered",
			"endpoint", srv.ScyllaManagerAPI,
			"scheme", managerEndpoint.Scheme,
			"cluster_count", len(registered),
			"cluster", clusterName)
		return true
	}

	// No clusters registered — check if ScyllaDB is actually running
	scyllaHost, nativeName := nativeScyllaDBDetector()
	if nativeName == "" {
		// ScyllaDB not reachable yet — retry. Transient at Day-1 (Scylla still
		// starting); logged so a persistent block is diagnosable.
		slog.Info("scylla auto-registration: local ScyllaDB not yet reachable on its LAN REST API — will retry",
			"api_url", srv.ScyllaManagerAPI)
		return false
	}

	// Use Globular domain as the scylla-manager cluster name
	clusterName := srv.Domain
	if clusterName == "" {
		clusterName = nativeName
	}

	slog.Info("auto-registering ScyllaDB in scylla-manager",
		"cluster_name", clusterName, "host", scyllaHost, "native_name", nativeName)

	agentCfg := readScyllaAgentConfig()
	if agentCfg.ReadErr != nil {
		slog.Warn("scylla_manager.registration.skipped.agent_config_unreadable",
			"path", agentCfg.Path,
			"owner", agentCfg.Owner,
			"group", agentCfg.Group,
			"mode", agentCfg.Mode,
			"error", agentCfg.ReadErr)
		return false
	}
	if strings.TrimSpace(agentCfg.AuthToken) == "" {
		slog.Warn("scylla_manager.registration.skipped.agent_token_missing",
			"path", agentCfg.Path,
			"owner", agentCfg.Owner,
			"group", agentCfg.Group,
			"mode", agentCfg.Mode,
			"https", agentCfg.HTTPSAddr)
		return false
	}
	if strings.TrimSpace(agentCfg.HTTPSPort) == "" {
		slog.Warn("scylla_manager.registration.skipped.agent_unreachable",
			"reason", "agent_https_port_missing",
			"path", agentCfg.Path,
			"https", agentCfg.HTTPSAddr)
		return false
	}
	agentEndpoint := net.JoinHostPort(scyllaHost, agentCfg.HTTPSPort)
	if !addressReachable(agentEndpoint) {
		slog.Warn("scylla_manager.registration.skipped.agent_unreachable",
			"reason", "agent_endpoint_unreachable",
			"endpoint", agentEndpoint,
			"path", agentCfg.Path)
		return false
	}

	args := append([]string{"cluster", "add", "--host", scyllaHost, "--name", clusterName}, srv.scyllaAPIArgs(srv.ScyllaManagerAPI)...)
	args = append(args, "--auth-token", agentCfg.AuthToken, "--port", agentCfg.HTTPSPort)

	slog.Info("auto-registering ScyllaDB in scylla-manager",
		"cluster_name", clusterName, "host", scyllaHost, "args", strings.Join(args, " "))

	cmd := execCommand("sctool", args...)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil {
		// Check if it failed because it's already registered (race condition)
		if strings.Contains(output, "already exists") || strings.Contains(output, "conflict") {
			slog.Info("scylla_manager.registration.already_registered",
				"endpoint", srv.ScyllaManagerAPI,
				"scheme", managerEndpoint.Scheme,
				"cluster", clusterName,
				"output", output)
			return true
		}
		slog.Warn("scylla_manager.registration.failed.sctool",
			"error", err,
			"output", output,
			"endpoint", srv.ScyllaManagerAPI,
			"scheme", managerEndpoint.Scheme,
			"cluster_name", clusterName,
			"host", scyllaHost)
		return false
	}

	verified := realRegisteredClusters(srv.detectScyllaClusters(srv.ScyllaManagerAPI))
	if len(verified) == 0 {
		slog.Warn("scylla_manager.registration.failed.sctool",
			"error", "cluster add returned success but cluster list is still empty",
			"output", output,
			"endpoint", srv.ScyllaManagerAPI,
			"scheme", managerEndpoint.Scheme,
			"cluster_name", clusterName,
			"host", scyllaHost)
		return false
	}

	slog.Info("scylla_manager.registration.succeeded",
		"cluster_name", clusterName,
		"host", scyllaHost,
		"endpoint", srv.ScyllaManagerAPI,
		"scheme", managerEndpoint.Scheme,
		"cluster_count", len(verified),
		"output", output)

	// Update config so backup provider can use it
	if srv.ScyllaCluster == "" {
		srv.ScyllaCluster = clusterName
	}
	// Persist the freshly-registered cluster to etcd so it survives restarts
	// and is visible to the configured backup path.
	srv.persistScyllaConfig()
	return true
}

// PreflightCheck verifies that required CLI tools are available.
// It also detects infrastructure configuration (e.g. ScyllaDB cluster names)
// and returns them as synthetic ToolCheck entries with names like "scylla_cluster_detected".
func (srv *server) PreflightCheck(ctx context.Context, rqst *backup_managerpb.PreflightCheckRequest) (*backup_managerpb.PreflightCheckResponse, error) {
	// Lazy-detect scylla-manager API URL on every preflight: the startup
	// goroutine has a 5s sleep and races with early UI calls. Without this,
	// sctool falls back to its built-in loopback default, which fails on any
	// node where scylla-manager is bound to a LAN IP. Also overrides any
	// persisted loopback / non-LAN value.
	if srv.ScyllaManagerAPI == "" || isInvalidScyllaManagerAPIURL(srv.ScyllaManagerAPI) {
		if detected := detectScyllaManagerAPIURL(); detected != "" && detected != srv.ScyllaManagerAPI {
			slog.Info("preflight: overriding scylla-manager API URL",
				"old", srv.ScyllaManagerAPI, "new", detected)
			srv.ScyllaManagerAPI = detected
		}
	}

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
		var check *backup_managerpb.ToolCheck
		if t.name == "sctool" {
			// sctool version requires the API to be reachable; pass API flags
			// so the check works correctly when HTTPS + mTLS is configured.
			check = checkTool(t.name, append(t.versionArgs, srv.scyllaAPIArgs(srv.ScyllaManagerAPI)...))
		} else {
			check = checkTool(t.name, t.versionArgs)
		}
		checks = append(checks, check)
		if !check.Available {
			allOk = false
		}
		if t.name == "sctool" && check.Available {
			sctoolAvailable = true
		}
	}

	// Surface the resolved scylla-manager API URL so the admin UI can pre-fill
	// it instead of falling back to a hardcoded 127.0.0.1 default.
	if srv.ScyllaManagerAPI != "" {
		checks = append(checks, &backup_managerpb.ToolCheck{
			Name:      "scylla_manager_api_url",
			Available: true,
			Version:   srv.ScyllaManagerAPI,
		})

		// Rollout safety gate: reject loopback / docker0 / non-LAN addresses
		// as the Scylla Manager API host. The rollout orchestrator can read
		// this ToolCheck and refuse to advance a node whose API still points
		// at a non-LAN identity. Mirrors the same invariant the node_agent
		// reconciler enforces locally — both layers should agree.
		laneCheck := &backup_managerpb.ToolCheck{Name: "scylla_manager_api_lan_check"}
		apiHost := scyllaManagerAPIHost(srv.ScyllaManagerAPI)
		if err := config.ValidateLANAddress(apiHost); err != nil {
			laneCheck.Available = false
			laneCheck.ErrorMessage = err.Error()
			allOk = false
		} else {
			laneCheck.Available = true
			laneCheck.Version = apiHost
		}
		checks = append(checks, laneCheck)
	}

	// Detect ScyllaDB cluster names if sctool is available
	if sctoolAvailable {
		clusters := srv.detectScyllaClusters(srv.ScyllaManagerAPI)

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
// (port 56093 or 10000) in case no clusters are registered in scylla-manager yet.
func (srv *server) detectScyllaClusters(apiURL string) []string {
	var clusters []string

	// 1. Try sctool cluster list (scylla-manager registered clusters)
	args := append([]string{"cluster", "list"}, srv.scyllaAPIArgs(apiURL)...)
	cmd := execCommand("sctool", args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		clusters = parseScyllaClusterList(string(out))
	}

	// 2. If no scylla-manager clusters found, try detecting native ScyllaDB
	//    cluster name via the REST API (default port 10000).
	//    ScyllaDB may be bound to a specific IP (not localhost), so try all
	//    local interface addresses.
	if len(clusters) == 0 {
		scyllaHost, nativeName := nativeScyllaDBDetector()
		if nativeName != "" {
			clusters = append(clusters, "native:"+nativeName)
			// Also include the host so the UI can use it for registration
			clusters = append(clusters, "scylla_host:"+scyllaHost)
		}
	}

	return clusters
}

// detectNativeScyllaDB tries to reach the ScyllaDB REST API on localhost and
// all local interface IPs. It tries port 56093 (Globular default) first, then
// falls back to 10000 (ScyllaDB default). Returns the reachable host and cluster name.
func detectNativeScyllaDB() (host, clusterName string) {
	// ScyllaDB's REST API binds the node's routable LAN IP (scylla.yaml
	// api_address), NEVER loopback — day-0 and node-agent enforce this. Probe
	// only the routable LAN IPs. Deliberately no 127.0.0.1 fallback: if it
	// somehow answered we would register the node with host=127.0.0.1, which
	// scylla-manager cannot use to reach the agent — a broken registration that
	// is worse than none. No LAN IP answering => return "" and let the caller
	// retry (Scylla not up yet, or misconfigured).
	var candidates []string
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

	// ScyllaDB REST API port (default 10000)
	ports := []int{10000}

	for _, port := range ports {
		for _, addr := range candidates {
			url := fmt.Sprintf("http://%s:%d/storage_service/cluster_name", addr, port)
			cmd := execCommand("curl", "-sf", "--connect-timeout", "2", url)
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

// deduplicateScyllaManagerClusters removes stale duplicate cluster entries
// from scylla-manager. After repeated wipe+Day0 cycles, scylla-manager's
// SQLite DB accumulates multiple entries with the same name. This breaks
// every sctool command that uses --cluster <name>.
func (srv *server) deduplicateScyllaManagerClusters(apiURL string) {
	args := append([]string{"cluster", "list"}, srv.scyllaAPIArgs(apiURL)...)
	out, err := execCommand("sctool", args...).CombinedOutput()
	if err != nil {
		return
	}

	type entry struct{ id, name string }
	var entries []entry

	// Parse ID and Name columns.
	lines := strings.Split(string(out), "\n")
	idCol, nameCol, headerIdx := -1, -1, -1
	for i, line := range lines {
		line = strings.TrimSpace(line)
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
			col := strings.TrimSpace(p)
			if col == "ID" {
				idCol = j
			}
			if col == "Name" {
				nameCol = j
			}
		}
		if idCol >= 0 && nameCol >= 0 {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return
	}

	for i := headerIdx + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
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
		if idCol < len(parts) && nameCol < len(parts) {
			id := strings.TrimSpace(parts[idCol])
			name := strings.TrimSpace(parts[nameCol])
			if id != "" && name != "" {
				entries = append(entries, entry{id: id, name: name})
			}
		}
	}

	// Group by name, find duplicates.
	byName := map[string][]entry{}
	for _, e := range entries {
		byName[e.name] = append(byName[e.name], e)
	}

	for name, group := range byName {
		if len(group) <= 1 {
			continue
		}

		// Determine which cluster is active by checking healthcheck tasks.
		activeID := ""
		for _, e := range group {
			taskArgs := append([]string{"tasks", "-c", e.id}, srv.scyllaAPIArgs(apiURL)...)
			tOut, tErr := execCommand("sctool", taskArgs...).CombinedOutput()
			if tErr != nil {
				continue
			}
			// Active cluster has healthchecks with a scheduled Next run.
			for _, tLine := range strings.Split(string(tOut), "\n") {
				if strings.Contains(tLine, "healthcheck/") && strings.Contains(tLine, "DONE") {
					fields := strings.Fields(tLine)
					for _, f := range fields {
						if len(f) == 4 && f >= "2025" && f <= "2030" {
							activeID = e.id
							break
						}
					}
					if activeID != "" {
						break
					}
				}
			}
			if activeID != "" {
				break
			}
		}
		if activeID == "" {
			activeID = group[len(group)-1].id // fallback: keep newest
		}

		for _, e := range group {
			if e.id == activeID {
				continue
			}
			delArgs := append([]string{"cluster", "delete", "-c", e.id}, srv.scyllaAPIArgs(apiURL)...)
			if err := execCommand("sctool", delArgs...).Run(); err != nil {
				slog.Warn("dedup: failed to delete stale cluster", "id", e.id, "name", name, "err", err)
			} else {
				slog.Info("dedup: deleted stale scylla-manager cluster", "id", e.id, "name", name, "kept", activeID)
			}
		}
	}
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

	// Lazy-detect the manager API URL when unset, so the test doesn't fall
	// back to sctool's built-in loopback default when admin connects to a node
	// whose manager binds to a LAN IP. Also overrides any persisted loopback /
	// non-LAN value when present.
	apiURL := srv.ScyllaManagerAPI
	if apiURL == "" || isInvalidScyllaManagerAPIURL(apiURL) {
		if detected := detectScyllaManagerAPIURL(); detected != "" && detected != apiURL {
			slog.Info("TestScyllaConnection: overriding scylla-manager API URL",
				"old", apiURL, "new", detected)
			apiURL = detected
			srv.ScyllaManagerAPI = detected
		}
	}
	apiArgs := func(args []string) []string {
		return append(args, srv.scyllaAPIArgs(apiURL)...)
	}

	// 2. scylla-manager server reachable
	versionArgs := apiArgs([]string{"sctool", "version"})
	slog.Info("TestScyllaConnection: running sctool", "args", versionArgs, "apiURL", apiURL)
	out, err := runDiagCmd(versionArgs...)
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
					"Add S3 credentials to the agent config:\nsudo nano /var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml\n\ns3:\n  access_key_id: <KEY>\n  secret_access_key: <SECRET>\n  provider: Minio\n  endpoint: https://<MINIO_IP>:9000\n\nThen restart:\nsudo systemctl restart globular-scylla-manager-agent.service")
			} else if strings.Contains(outLower, "nosuchbucket") || strings.Contains(outLower, "bucket") && strings.Contains(outLower, "not found") {
				fail("storage_location",
					fmt.Sprintf("S3 bucket not found: %s", location),
					"Create the bucket in MinIO, or check the location name matches an existing bucket.")
			} else if strings.Contains(outLower, "timeout") || strings.Contains(outLower, "connection refused") {
				fail("storage_location",
					"Cannot reach S3/MinIO endpoint from agent",
					"Verify MinIO is running:\nsudo systemctl status globular-minio.service\nCheck agent config endpoint resolves via cluster DNS (minio.<domain>:9000)")
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
