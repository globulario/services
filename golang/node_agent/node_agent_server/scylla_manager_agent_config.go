// @awareness namespace=globular.platform
// @awareness component=platform_node_agent
// @awareness file_role=scylladb_manager_agent_config_sync
// @awareness risk=medium
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
)

const (
	scyllaAgentConfigPrimary = "/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml"
	scyllaAgentConfigEtc     = "/etc/scylla-manager-agent/scylla-manager-agent.yaml"
)

// scyllaAgent* constants define non-conflicting ports for scylla-manager-agent.
// The agent defaults to :10001 (HTTPS), :5090 (Prometheus), 127.0.0.1:5112
// (debug), all of which conflict with Globular's dynamic service port
// allocation (10000+).
//
// The values must satisfy two constraints:
//  1. Outside Globular's service range 10000-10200.
//  2. Outside the Linux ephemeral port range (typically 32768-60999, see
//     /proc/sys/net/ipv4/ip_local_port_range). Picking ports inside that
//     range races against any local process making an outbound connection
//     — that's exactly how the earlier choice of 56001-56003 silently
//     crashed the agent with "bind: address already in use" when the
//     globular DNS service happened to grab 56002 as its source port.
//  3. Not collide with scylla-manager itself (5080), ScyllaDB (7000/9042/
//     9142/9160/10000/19042), or other well-known Globular ports.
const (
	scyllaAgentHTTPSPort      = "5612"
	scyllaAgentPrometheusPort = "5613"
	scyllaAgentDebugPort      = "5614"
)

// ensureScyllaManagerAgentAuthToken guarantees scylla-manager-agent has an
// auth_token, the correct scylla.api_address, and non-conflicting HTTPS /
// Prometheus ports in its YAML config.
func (srv *NodeAgentServer) ensureScyllaManagerAgentAuthToken(ctx context.Context) {
	if !scyllaManagerAgentUnitExists(ctx) {
		return
	}

	cfgPath := selectScyllaManagerAgentConfigPath()
	current, _ := os.ReadFile(cfgPath)
	content := string(current)

	nodeIP := nodeRoutableIP()
	// Safety gate: never write a Scylla agent config bound to loopback,
	// docker0, link-local, or any other non-LAN address. nodeRoutableIP()
	// occasionally returned docker0's IP in the wild (interface enumeration
	// order is non-deterministic), and YAML last-wins silently routed the
	// agent to an unreachable address. Refuse to reconcile rather than
	// persist bad state. The current healthy state — if any — stays put
	// until a routable LAN IP is available again.
	if nodeIP != "" {
		if err := config.ValidateLANAddress(nodeIP); err != nil {
			log.Printf("nodeagent: scylla-manager-agent reconcile aborted: %v — refusing to write non-LAN config", err)
			return
		}
	}
	// The auth token must be the SAME on every agent in the cluster — the
	// scylla-manager server caches one token per cluster and uses it to talk
	// to every host. A per-node random UUID (e.g. one minted by an install
	// script) silently breaks `sctool cluster add` with HTTP 401 against the
	// non-coordinator hosts. So compare against the derived cluster-wide
	// value, not just "is anything there?".
	derivedToken := deriveClusterScopedScyllaAuthToken()
	tokenMatches := currentAuthToken(content) == derivedToken
	hasURL := hasScyllaAPIURL(content, nodeIP)
	hasPorts := hasScyllaAgentPorts(content, nodeIP)
	if tokenMatches && hasURL && hasPorts {
		return
	}

	updated := content
	if !tokenMatches {
		updated = upsertAuthToken(updated, derivedToken)
	}
	if !hasURL && nodeIP != "" {
		updated = upsertScyllaAPIURL(updated, nodeIP)
	}
	if !hasPorts && nodeIP != "" {
		updated = upsertScyllaAgentPorts(updated, nodeIP)
	}

	// Skip writes when reconcile produced no actual change — avoids an
	// unnecessary agent restart on every heartbeat.
	if updated == content {
		return
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o750); err != nil {
		log.Printf("nodeagent: scylla-manager-agent config mkdir failed: %v", err)
		return
	}
	if err := os.WriteFile(cfgPath, []byte(updated), 0o640); err != nil {
		log.Printf("nodeagent: scylla-manager-agent config write failed (%s): %v", cfgPath, err)
		return
	}
	// scylla-manager-agent.service runs as User=scylla; without chgrp the
	// scylla user can't read this file and the unit crash-loops with
	// "permission denied". Mode 0640 only helps if the group is scylla.
	chgrpScyllaAgentConfig(cfgPath)
	log.Printf("nodeagent: ensured scylla-manager-agent config in %s (token=%v api_url=%v ports=%v)",
		cfgPath, !tokenMatches, !hasURL, !hasPorts)

	// The agent loads its config once at startup; a yaml change has zero
	// effect until the unit is restarted. Without this, every reconcile that
	// rewrote the yaml left the agent running on stale config — that's how
	// the duplicate-scylla-block corruption persisted in production for so
	// long. Routed through the supervisor package because the Makefile
	// security check bans direct os/exec from node_agent_server.
	if err := supervisor.Restart(ctx, "globular-scylla-manager-agent.service"); err != nil {
		log.Printf("nodeagent: scylla-manager-agent restart failed: %v", err)
		return
	}
	log.Printf("nodeagent: restarted globular-scylla-manager-agent.service after config change")
}

// chgrpScyllaAgentConfig sets the file's group to "scylla" so the
// scylla-manager-agent unit (User=scylla, Group=scylla) can read its config.
// Per CLAUDE.md: never hardcode UIDs/GIDs — always resolve via user.Lookup.
func chgrpScyllaAgentConfig(path string) {
	u, err := user.Lookup("scylla")
	if err != nil {
		log.Printf("nodeagent: scylla user not found — cannot chgrp %s: %v", path, err)
		return
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		log.Printf("nodeagent: invalid scylla GID %q: %v", u.Gid, err)
		return
	}
	if err := os.Chown(path, -1, gid); err != nil {
		log.Printf("nodeagent: cannot chgrp scylla group on %s: %v", path, err)
	}
}

func scyllaManagerAgentUnitExists(ctx context.Context) bool {
	out, err := exec.CommandContext(ctx, "systemctl", "list-unit-files",
		"globular-scylla-manager-agent.service", "--no-legend", "--no-pager").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}

func selectScyllaManagerAgentConfigPath() string {
	if _, err := os.Stat(scyllaAgentConfigPrimary); err == nil {
		return scyllaAgentConfigPrimary
	}
	if _, err := os.Stat(scyllaAgentConfigEtc); err == nil {
		return scyllaAgentConfigEtc
	}
	return scyllaAgentConfigPrimary
}

func deriveClusterScopedScyllaAuthToken() string {
	domain, err := config.GetDomain()
	if err != nil || strings.TrimSpace(domain) == "" {
		domain = "globular.internal"
	}
	caPath := config.GetCACertificatePath()
	caBytes, _ := os.ReadFile(caPath)
	caHash := sha256.Sum256(caBytes)

	seed := fmt.Sprintf("%s|%x", strings.ToLower(strings.TrimSpace(domain)), caHash[:])
	sum := sha256.Sum256([]byte(seed))
	// 48 hex chars (24 bytes) keeps token reasonably compact but strong.
	return hex.EncodeToString(sum[:24])
}

// hasScyllaAPIURL returns true only when the config has exactly one scylla:
// block AND its api_address matches expectedIP. Duplicate scylla: blocks
// (caused by earlier versions blindly appending) are reported as "missing" so
// the caller rewrites — YAML's last-wins semantics mean duplicates silently
// pick up whichever block was written last, which is often a stale IP (e.g.
// docker0) that breaks scylla-manager → agent → ScyllaDB connectivity.
//
// Scylla Manager Agent uses a nested block:
//
//	scylla:
//	  api_address: NODE_IP
//	  api_port: 10000
func hasScyllaAPIURL(content, expectedIP string) bool {
	blocks := extractScyllaBlocks(content)
	if len(blocks) != 1 {
		return false
	}
	v := blocks[0].apiAddress
	if v == "" {
		return false
	}
	return expectedIP == "" || v == expectedIP
}

type scyllaBlock struct {
	apiAddress string
	apiPort    string
}

// extractScyllaBlocks parses every top-level `scylla:` block in the YAML
// and returns its api_address/api_port. A top-level block starts with a line
// that begins exactly with `scylla:` at column 0 and ends at the next line
// that starts in column 0 with non-whitespace.
func extractScyllaBlocks(content string) []scyllaBlock {
	var out []scyllaBlock
	var cur *scyllaBlock
	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}
	for _, raw := range strings.Split(content, "\n") {
		// Top-level (column-0) line — closes any current block first.
		isTopLevel := len(raw) > 0 && raw[0] != ' ' && raw[0] != '\t'
		trimmed := strings.TrimSpace(raw)
		if isTopLevel {
			flush()
			if trimmed == "scylla:" {
				cur = &scyllaBlock{}
			}
			continue
		}
		// Inside a block: pick up api_address / api_port.
		if cur != nil {
			if strings.HasPrefix(trimmed, "api_address:") {
				v := strings.TrimSpace(strings.TrimPrefix(trimmed, "api_address:"))
				cur.apiAddress = strings.Trim(v, `"'`)
			} else if strings.HasPrefix(trimmed, "api_port:") {
				v := strings.TrimSpace(strings.TrimPrefix(trimmed, "api_port:"))
				cur.apiPort = strings.Trim(v, `"'`)
			}
		}
	}
	flush()
	return out
}

// upsertScyllaAPIURL removes every existing top-level `scylla:` block (plus
// legacy top-level `api_url:` lines from older code) and writes one canonical
// block with the supplied nodeIP. Scylla's REST API port is always 10000.
//
// Earlier versions of this function blindly appended a new `scylla:` block on
// every reconcile, accumulating duplicates with stale IPs (e.g. docker0). With
// YAML's last-wins rule, that silently rerouted the agent to an unreachable
// address and broke scylla-manager cluster registration. Stripping first is
// the only way to converge to a single source of truth.
func upsertScyllaAPIURL(content, nodeIP string) string {
	cleaned := stripScyllaBlocks(content)
	cleaned = stripLegacyTopLevel(cleaned, "api_url:")
	if !strings.HasSuffix(cleaned, "\n") {
		cleaned += "\n"
	}
	cleaned += fmt.Sprintf("\nscylla:\n  api_address: %s\n  api_port: 10000\n", nodeIP)
	return cleaned
}

// stripScyllaBlocks removes every top-level `scylla:` block and its indented
// body. Leaves all other content untouched.
func stripScyllaBlocks(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	skipping := false
	for _, raw := range lines {
		isTopLevel := len(raw) > 0 && raw[0] != ' ' && raw[0] != '\t'
		trimmed := strings.TrimSpace(raw)
		if isTopLevel {
			if trimmed == "scylla:" {
				skipping = true
				continue
			}
			skipping = false
		}
		if skipping {
			continue
		}
		out = append(out, raw)
	}
	// Collapse runs of blank lines left behind by stripping.
	result := strings.Join(out, "\n")
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	return result
}

// stripLegacyTopLevel removes top-level lines starting with the given prefix
// (e.g. "api_url:") that older code may have written by mistake.
func stripLegacyTopLevel(content, prefix string) string {
	lines := strings.Split(content, "\n")
	var out []string
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), prefix) {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// hasScyllaAgentPorts returns true if the config has non-default https,
// prometheus, and debug ports that match the expected node IP.
func hasScyllaAgentPorts(content, nodeIP string) bool {
	wantHTTPS := nodeIP + ":" + scyllaAgentHTTPSPort
	wantProm := ":" + scyllaAgentPrometheusPort
	wantDebug := "127.0.0.1:" + scyllaAgentDebugPort
	hasHTTPS, hasProm, hasDebug := false, false, false
	for _, raw := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(raw)
		check := func(prefix, want string, flag *bool) {
			if strings.HasPrefix(trimmed, prefix) {
				v := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
				v = strings.Trim(v, `"'`)
				if v == want {
					*flag = true
				}
			}
		}
		check("https:", wantHTTPS, &hasHTTPS)
		check("prometheus:", wantProm, &hasProm)
		check("debug:", wantDebug, &hasDebug)
	}
	return hasHTTPS && hasProm && hasDebug
}

// upsertScyllaAgentPorts replaces or inserts https:, prometheus:, and debug:
// lines to avoid conflicts with Globular's dynamic port allocation (10000+),
// scylla-manager itself (5090), and other concurrent agents (5112).
func upsertScyllaAgentPorts(content, nodeIP string) string {
	httpsLine := "https: " + nodeIP + ":" + scyllaAgentHTTPSPort
	promLine := "prometheus: :" + scyllaAgentPrometheusPort
	debugLine := "debug: 127.0.0.1:" + scyllaAgentDebugPort

	lines := strings.Split(content, "\n")
	var out []string
	replacedHTTPS, replacedProm, replacedDebug := false, false, false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "https:") {
			out = append(out, httpsLine)
			replacedHTTPS = true
			continue
		}
		if strings.HasPrefix(trimmed, "prometheus:") {
			out = append(out, promLine)
			replacedProm = true
			continue
		}
		if strings.HasPrefix(trimmed, "debug:") {
			out = append(out, debugLine)
			replacedDebug = true
			continue
		}
		out = append(out, l)
	}
	result := strings.Join(out, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	if !replacedHTTPS {
		result += httpsLine + "\n"
	}
	if !replacedProm {
		result += promLine + "\n"
	}
	if !replacedDebug {
		result += debugLine + "\n"
	}
	return result
}

func hasNonEmptyAuthToken(content string) bool {
	return currentAuthToken(content) != ""
}

// currentAuthToken returns the first non-comment auth_token value found in the
// YAML, with surrounding quotes stripped. Returns "" if none present or empty.
func currentAuthToken(content string) string {
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "auth_token:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "auth_token:"))
			return strings.Trim(v, `"'`)
		}
	}
	return ""
}

func upsertAuthToken(content, token string) string {
	lines := strings.Split(content, "\n")
	replaced := false
	for i, raw := range lines {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "auth_token:") {
			lines[i] = "auth_token: " + token
			replaced = true
		}
	}
	if !replaced {
		if strings.TrimSpace(content) != "" && !strings.HasSuffix(content, "\n") {
			lines = append(lines, "")
		}
		lines = append(lines, "auth_token: "+token)
	}
	result := strings.Join(lines, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result
}
