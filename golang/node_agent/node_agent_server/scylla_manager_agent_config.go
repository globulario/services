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

// ensureScyllaManagerAgentAuthToken guarantees scylla-manager-agent has both
// an auth_token and the correct api_url in its YAML config. Scylla binds its
// REST API to the node's advertise IP (not 0.0.0.0), so the agent must be
// told the actual IP or it will keep retrying 0.0.0.0:10000 and failing.
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
	if hasToken && hasURL {
		return
	}

	updated := content
	if !hasToken {
		updated = upsertAuthToken(updated, deriveClusterScopedScyllaAuthToken())
	}
	if !hasURL && nodeIP != "" {
		updated = upsertScyllaAPIURL(updated, nodeIP)
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
	log.Printf("nodeagent: ensured scylla-manager-agent config in %s (token=%v api_url=%v)", cfgPath, !hasToken, !hasURL)
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
