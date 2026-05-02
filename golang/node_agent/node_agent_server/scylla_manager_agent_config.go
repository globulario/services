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

// ensureScyllaManagerAgentAuthToken guarantees scylla-manager-agent has an
// auth_token in its YAML config. This keeps Day-0 and join installs convergent
// even when package defaults omit auth_token.
func (srv *NodeAgentServer) ensureScyllaManagerAgentAuthToken(ctx context.Context) {
	if !scyllaManagerAgentUnitExists(ctx) {
		return
	}

	cfgPath := selectScyllaManagerAgentConfigPath()
	current, _ := os.ReadFile(cfgPath)
	if hasNonEmptyAuthToken(string(current)) {
		return
	}

	token := deriveClusterScopedScyllaAuthToken()
	updated := upsertAuthToken(string(current), token)

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o750); err != nil {
		log.Printf("nodeagent: scylla-manager-agent auth_token mkdir failed: %v", err)
		return
	}
	if err := os.WriteFile(cfgPath, []byte(updated), 0o640); err != nil {
		log.Printf("nodeagent: scylla-manager-agent auth_token write failed (%s): %v", cfgPath, err)
		return
	}
	// scylla-manager-agent.service runs as User=scylla; without chgrp the
	// scylla user can't read this file and the unit crash-loops with
	// "permission denied". Mode 0640 only helps if the group is scylla.
	chgrpScyllaAgentConfig(cfgPath)
	log.Printf("nodeagent: ensured scylla-manager-agent auth_token in %s", cfgPath)
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
