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
)

const (
	scyllaAgentConfigPrimary = "/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml"
	scyllaAgentConfigEtc     = "/etc/scylla-manager-agent/scylla-manager-agent.yaml"
)

// scyllaAgentHTTPSPort and scyllaAgentPrometheusPort are fixed non-conflicting
// ports for scylla-manager-agent. The agent defaults to :10001 (HTTPS) and
// :5090 (Prometheus), both of which conflict with Globular's dynamic service
// port allocation (starting at 10000) and scylla-manager itself (port 5090).
const (
	scyllaAgentHTTPSPort      = "56001"
	scyllaAgentPrometheusPort = "56002"
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
	hasToken := hasNonEmptyAuthToken(content)
	hasURL := hasScyllaAPIURL(content, nodeIP)
	hasPorts := hasScyllaAgentPorts(content, nodeIP)
	if hasToken && hasURL && hasPorts {
		return
	}

	updated := content
	if !hasToken {
		updated = upsertAuthToken(updated, deriveClusterScopedScyllaAuthToken())
	}
	if !hasURL && nodeIP != "" {
		updated = upsertScyllaAPIURL(updated, nodeIP)
	}
	if !hasPorts && nodeIP != "" {
		updated = upsertScyllaAgentPorts(updated, nodeIP)
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
		cfgPath, !hasToken, !hasURL, !hasPorts)
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

// hasScyllaAPIURL returns true if the config already has scylla.api_address
// pointing at the expected node IP. An empty expectedIP means any non-empty
// value is fine.
//
// Scylla Manager Agent uses a nested block:
//
//	scylla:
//	  api_address: NODE_IP
//	  api_port: 10000
func hasScyllaAPIURL(content, expectedIP string) bool {
	inScyllaBlock := false
	for _, raw := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "scylla:" || strings.HasPrefix(trimmed, "scylla:") {
			inScyllaBlock = true
			continue
		}
		if inScyllaBlock {
			if trimmed == "" || (len(raw) > 0 && raw[0] != ' ' && raw[0] != '\t') {
				inScyllaBlock = false
				continue
			}
			if strings.HasPrefix(trimmed, "api_address:") {
				v := strings.TrimSpace(strings.TrimPrefix(trimmed, "api_address:"))
				v = strings.Trim(v, `"'`)
				if v == "" {
					return false
				}
				return expectedIP == "" || v == expectedIP
			}
		}
	}
	return false
}

// upsertScyllaAPIURL appends a scylla: block with api_address/api_port if one
// doesn't already exist. Scylla's REST API port is always 10000.
// The config key is nested (not top-level) — top-level api_url is rejected by
// the agent with "field api_url not found in type agent.Config".
func upsertScyllaAPIURL(content, nodeIP string) string {
	// Strip any legacy top-level api_url we may have written by mistake.
	lines := strings.Split(content, "\n")
	var filtered []string
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "api_url:") {
			continue
		}
		filtered = append(filtered, l)
	}
	result := strings.Join(filtered, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	result += fmt.Sprintf("\nscylla:\n  api_address: %s\n  api_port: 10000\n", nodeIP)
	return result
}

// hasScyllaAgentPorts returns true if the config has non-default https and
// prometheus ports that match the expected node IP.
func hasScyllaAgentPorts(content, nodeIP string) bool {
	wantHTTPS := nodeIP + ":" + scyllaAgentHTTPSPort
	wantProm := ":" + scyllaAgentPrometheusPort
	hasHTTPS := false
	hasProm := false
	for _, raw := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "https:") {
			v := strings.TrimSpace(strings.TrimPrefix(trimmed, "https:"))
			v = strings.Trim(v, `"'`)
			if v == wantHTTPS {
				hasHTTPS = true
			}
		}
		if strings.HasPrefix(trimmed, "prometheus:") {
			v := strings.TrimSpace(strings.TrimPrefix(trimmed, "prometheus:"))
			v = strings.Trim(v, `"'`)
			if v == wantProm {
				hasProm = true
			}
		}
	}
	return hasHTTPS && hasProm
}

// upsertScyllaAgentPorts replaces or inserts the https: and prometheus: lines
// to avoid conflicts with Globular's dynamic port allocation (10000+) and the
// scylla-manager server (port 5090).
func upsertScyllaAgentPorts(content, nodeIP string) string {
	httpsLine := "https: " + nodeIP + ":" + scyllaAgentHTTPSPort
	promLine := "prometheus: :" + scyllaAgentPrometheusPort

	lines := strings.Split(content, "\n")
	var out []string
	replacedHTTPS, replacedProm := false, false
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
	return result
}

func hasNonEmptyAuthToken(content string) bool {
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "auth_token:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "auth_token:"))
			v = strings.Trim(v, `"'`)
			return v != ""
		}
	}
	return false
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
