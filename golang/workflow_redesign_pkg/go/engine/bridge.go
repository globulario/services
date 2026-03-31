// Package engine bridge: defines the FetchAndInstall function type and
// helpers for connecting to real systems. The actual implementation lives
// in the node-agent process (installer_api.go) since it needs access to
// the internal action registry.
//
// The workflow engine itself is process-agnostic — it calls whatever
// FetchAndInstall function is provided via NodeAgentConfig.
package engine

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverRepositoryAddr reads the controller endpoint from the node-agent
// state file and derives the repository mesh address (same host, port 443).
func DiscoverRepositoryAddr() string {
	stateRoot := strings.TrimSpace(os.Getenv("GLOBULAR_STATE_DIR"))
	if stateRoot == "" {
		stateRoot = "/var/lib/globular"
	}
	data, err := os.ReadFile(filepath.Join(stateRoot, "nodeagent", "state.json"))
	if err != nil {
		return ""
	}
	var state struct {
		ControllerEndpoint string `json:"controller_endpoint"`
	}
	if json.Unmarshal(data, &state) != nil || state.ControllerEndpoint == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(state.ControllerEndpoint)
	if err != nil || host == "" {
		return ""
	}
	return host + ":443"
}
