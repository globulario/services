package ingress

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// NodeStatus represents the ingress status for a single node
type NodeStatus struct {
	NodeID    string `json:"node_id"`
	Phase     string `json:"phase"`       // "Ready" | "Error"
	VRRPState string `json:"vrrp_state"`  // "MASTER" | "BACKUP" | "FAULT" | "UNKNOWN"
	HasVIP    bool   `json:"has_vip"`     // True if VIP is present on this node's interface
	VIP       string `json:"vip"`         // The VIP address
	UpdatedAt int64  `json:"updated_at_unix"`
	LastError string `json:"last_error,omitempty"`
}

// WriteStatus writes node ingress status to etcd
func WriteStatus(ctx context.Context, client *clientv3.Client, nodeID string, status *NodeStatus) error {
	key := "/globular/ingress/v1/status/" + nodeID
	status.UpdatedAt = time.Now().Unix()

	data, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}

	_, err = client.Put(ctx, key, string(data))
	if err != nil {
		return fmt.Errorf("write status to etcd: %w", err)
	}

	return nil
}

// DetectVRRPState detects the current VRRP state and VIP presence on this node
// Returns the VRRP state ("MASTER", "BACKUP", "FAULT", "UNKNOWN") and whether the VIP is present
func DetectVRRPState(iface, vip string) (state string, hasVIP bool) {
	// Default state
	state = "UNKNOWN"
	hasVIP = false

	// Check if VIP is present on the interface
	hasVIP = checkVIPPresence(iface, vip)

	// Check if keepalived service is active
	isActive := checkServiceActive("keepalived")
	if !isActive {
		state = "FAULT"
		return state, hasVIP
	}

	// Parse journalctl for recent VRRP state transitions
	detectedState := parseKeepalivedLogs()
	if detectedState != "" {
		state = detectedState
	}

	return state, hasVIP
}

// checkVIPPresence checks if the VIP is assigned to the given interface
func checkVIPPresence(iface, vip string) bool {
	// Extract IP address without CIDR notation
	ipAddr := strings.Split(vip, "/")[0]

	// Run: ip addr show dev <iface>
	cmd := exec.Command("ip", "addr", "show", "dev", iface)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if output contains the VIP address
	return strings.Contains(string(output), ipAddr)
}

// checkServiceActive checks if a systemd service is active
func checkServiceActive(service string) bool {
	cmd := exec.Command("systemctl", "is-active", service)
	err := cmd.Run()
	// systemctl is-active returns exit code 0 if active
	return err == nil
}

// parseKeepalivedLogs parses recent keepalived journal logs to detect VRRP state
func parseKeepalivedLogs() string {
	// Run: journalctl -u keepalived -n 50 --no-pager
	cmd := exec.Command("journalctl", "-u", "keepalived", "-n", "50", "--no-pager")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	logs := string(output)

	// Regex patterns for VRRP state transitions
	// keepalived logs: "Entering MASTER STATE" or "Entering BACKUP STATE"
	masterRe := regexp.MustCompile(`Entering MASTER STATE`)
	backupRe := regexp.MustCompile(`Entering BACKUP STATE`)
	faultRe := regexp.MustCompile(`Entering FAULT STATE`)

	// Parse logs from most recent to oldest (reverse)
	lines := strings.Split(logs, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if masterRe.MatchString(line) {
			return "MASTER"
		}
		if backupRe.MatchString(line) {
			return "BACKUP"
		}
		if faultRe.MatchString(line) {
			return "FAULT"
		}
	}

	return ""
}
