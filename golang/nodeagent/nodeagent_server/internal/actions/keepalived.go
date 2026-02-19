package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/ingress"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/ingress/keepalived"
	"github.com/globulario/services/golang/nodeagent/nodeagent_server/internal/supervisor"
)

const (
	keepalivedConfigPath     = "/etc/keepalived/keepalived.conf"
	healthScriptPath         = "/usr/lib/globular/bin/check-ingress.sh"
	defaultPriority          = 100
	keepalivedServiceName    = "keepalived"
)

type keepalivedReconcileAction struct {
	etcdClient *clientv3.Client
}

func init() {
	// Note: etcd client will be set via SetEtcdClient or passed via context
	Register(&keepalivedReconcileAction{})
}

func (a *keepalivedReconcileAction) Name() string {
	return "ingress.keepalived.reconcile"
}

func (a *keepalivedReconcileAction) Validate(args *structpb.Struct) error {
	if args == nil {
		return errors.New("args required")
	}

	// Required: spec_json
	specJSON := strings.TrimSpace(args.GetFields()["spec_json"].GetStringValue())
	if specJSON == "" {
		return errors.New("spec_json is required")
	}

	// Required: node_id
	nodeID := strings.TrimSpace(args.GetFields()["node_id"].GetStringValue())
	if nodeID == "" {
		return errors.New("node_id is required")
	}

	// Validate spec_json is valid JSON
	var spec ingress.Spec
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		return fmt.Errorf("invalid spec_json: %w", err)
	}

	return nil
}

func (a *keepalivedReconcileAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	// Extract arguments
	specJSON := strings.TrimSpace(args.GetFields()["spec_json"].GetStringValue())
	nodeID := strings.TrimSpace(args.GetFields()["node_id"].GetStringValue())
	dryRun := args.GetFields()["dry_run"].GetBoolValue()

	// Get etcd client from context or use stored client
	etcdClient := a.etcdClient
	if etcdClient == nil {
		if ctxClient, ok := ctx.Value("etcd_client").(*clientv3.Client); ok {
			etcdClient = ctxClient
		}
	}

	// Parse spec
	var spec ingress.Spec
	if err := json.Unmarshal([]byte(specJSON), &spec); err != nil {
		return "", fmt.Errorf("parse spec: %w", err)
	}

	// Handle ModeDisabled or non-vip_failover modes
	if spec.Mode != ingress.ModeVIPFailover {
		return a.disableKeepalived(ctx, nodeID, etcdClient, dryRun)
	}

	if spec.VIPFailover == nil {
		return "", errors.New("vip_failover configuration is required when mode is vip_failover")
	}

	// Validate VIPFailover spec
	if err := validateVIPFailoverSpec(*spec.VIPFailover); err != nil {
		// Write error status
		if etcdClient != nil {
			status := &ingress.NodeStatus{
				NodeID:    nodeID,
				Phase:     "Error",
				VRRPState: "UNKNOWN",
				HasVIP:    false,
				VIP:       spec.VIPFailover.VIP,
				LastError: fmt.Sprintf("invalid spec: %v", err),
			}
			ingress.WriteStatus(ctx, etcdClient, nodeID, status)
		}
		return "", fmt.Errorf("invalid vip_failover spec: %w", err)
	}

	// Validate this node is in Participants
	isParticipant := false
	for _, p := range spec.VIPFailover.Participants {
		if p == nodeID {
			isParticipant = true
			break
		}
	}

	if !isParticipant {
		return a.disableKeepalived(ctx, nodeID, etcdClient, dryRun)
	}

	// Get priority for this node (default to 100 if not specified)
	priority := defaultPriority
	if p, ok := spec.VIPFailover.Priority[nodeID]; ok {
		priority = p
	}

	// Render keepalived config
	renderInput := keepalived.RenderInput{
		NodeID:   nodeID,
		Priority: priority,
		Spec:     *spec.VIPFailover,
	}

	configContent, err := keepalived.RenderConfig(renderInput)
	if err != nil {
		return "", fmt.Errorf("render config: %w", err)
	}

	// Render health script (if TCP ports are configured)
	var healthScriptContent string
	if len(spec.VIPFailover.CheckTCPPorts) > 0 {
		healthScriptContent, err = keepalived.RenderHealthScriptTCP(spec.VIPFailover.CheckTCPPorts)
		if err != nil {
			return "", fmt.Errorf("render health script: %w", err)
		}
	}

	if dryRun {
		return fmt.Sprintf("dry-run: would configure keepalived for node %s with priority %d", nodeID, priority), nil
	}

	// Write keepalived config (atomic write with change detection)
	configChanged, err := writeFileIfChanged(keepalivedConfigPath, configContent, 0644)
	if err != nil {
		return "", fmt.Errorf("write keepalived config: %w", err)
	}

	// Write health script (if configured)
	if healthScriptContent != "" {
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(healthScriptPath), 0755); err != nil {
			return "", fmt.Errorf("create health script dir: %w", err)
		}

		if _, err := writeFileIfChanged(healthScriptPath, healthScriptContent, 0755); err != nil {
			return "", fmt.Errorf("write health script: %w", err)
		}
	}

	// Ensure keepalived is installed (check for binary, don't install)
	if err := ensureKeepalivedPresent(); err != nil {
		// Write error status
		if etcdClient != nil {
			status := &ingress.NodeStatus{
				NodeID:    nodeID,
				Phase:     "Error",
				VRRPState: "UNKNOWN",
				HasVIP:    false,
				VIP:       spec.VIPFailover.VIP,
				LastError: fmt.Sprintf("keepalived not installed: %v", err),
			}
			ingress.WriteStatus(ctx, etcdClient, nodeID, status)
		}
		return "", fmt.Errorf("keepalived not installed: %w", err)
	}

	// Enable keepalived service
	if err := supervisor.Enable(ctx, keepalivedServiceName); err != nil {
		return "", fmt.Errorf("enable keepalived: %w", err)
	}

	// Start or reload keepalived
	isActive, _ := supervisor.IsActive(ctx, keepalivedServiceName)
	if !isActive {
		// Start if not running
		if err := supervisor.Start(ctx, keepalivedServiceName); err != nil {
			return "", fmt.Errorf("start keepalived: %w", err)
		}
	} else if configChanged {
		// Reload if config changed and service is running
		// keepalived supports reload via SIGHUP
		if err := reloadKeepalived(ctx); err != nil {
			// Fallback to restart if reload fails
			if err := supervisor.Restart(ctx, keepalivedServiceName); err != nil {
				return "", fmt.Errorf("restart keepalived: %w", err)
			}
		}
	}

	// Wait for keepalived to become active
	if err := supervisor.WaitActive(ctx, keepalivedServiceName, 10*time.Second); err != nil {
		return "", fmt.Errorf("wait for keepalived active: %w", err)
	}

	// Detect VRRP state and VIP presence
	vrrpState, hasVIP := ingress.DetectVRRPState(spec.VIPFailover.Interface, spec.VIPFailover.VIP)

	// Write status to etcd
	if etcdClient != nil {
		status := &ingress.NodeStatus{
			NodeID:    nodeID,
			Phase:     "Ready",
			VRRPState: vrrpState,
			HasVIP:    hasVIP,
			VIP:       spec.VIPFailover.VIP,
		}

		if err := ingress.WriteStatus(ctx, etcdClient, nodeID, status); err != nil {
			// Log error but don't fail the action
			return fmt.Sprintf("keepalived configured successfully, but status write failed: %v", err), nil
		}
	}

	result := fmt.Sprintf("keepalived configured for node %s (priority %d, state %s, VIP present: %v)",
		nodeID, priority, vrrpState, hasVIP)

	if configChanged {
		result += " [config updated]"
	}

	return result, nil
}

// disableKeepalived stops and disables keepalived on this node
func (a *keepalivedReconcileAction) disableKeepalived(ctx context.Context, nodeID string, etcdClient *clientv3.Client, dryRun bool) (string, error) {
	if dryRun {
		return fmt.Sprintf("dry-run: would disable keepalived on node %s", nodeID), nil
	}

	// Check if keepalived is active
	isActive, _ := supervisor.IsActive(ctx, keepalivedServiceName)

	// Stop keepalived if running
	if isActive {
		if err := supervisor.Stop(ctx, keepalivedServiceName); err != nil {
			// Log error but continue
		}
	}

	// Disable keepalived
	if err := supervisor.Disable(ctx, keepalivedServiceName); err != nil {
		// Ignore errors if service doesn't exist
	}

	// Write status to etcd
	if etcdClient != nil {
		status := &ingress.NodeStatus{
			NodeID:    nodeID,
			Phase:     "Ready",
			VRRPState: "UNKNOWN",
			HasVIP:    false,
			VIP:       "",
		}

		if err := ingress.WriteStatus(ctx, etcdClient, nodeID, status); err != nil {
			return fmt.Sprintf("keepalived disabled, but status write failed: %v", err), nil
		}
	}

	return fmt.Sprintf("keepalived disabled on node %s", nodeID), nil
}

// writeFileIfChanged atomically writes content to path if content differs from existing file
// Returns true if file was changed, false if unchanged
func writeFileIfChanged(path, content string, mode os.FileMode) (bool, error) {
	// Check if file exists and content is unchanged
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == content {
		return false, nil // No change
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("create directory: %w", err)
	}

	// Write to temporary file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(content), mode); err != nil {
		return false, fmt.Errorf("write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return false, fmt.Errorf("rename temp file: %w", err)
	}

	return true, nil
}

// ensureKeepalivedPresent checks that keepalived binary is installed
// Does NOT install packages - expects keepalived to be pre-installed by installer
func ensureKeepalivedPresent() error {
	// Check for keepalived binary
	paths := []string{
		"/usr/sbin/keepalived",
		"/sbin/keepalived",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return nil // Found
		}
	}

	// Try which command as fallback
	cmd := exec.Command("which", "keepalived")
	if err := cmd.Run(); err == nil {
		return nil // Found via which
	}

	return fmt.Errorf("keepalived binary not found (expected to be installed by installer)")
}

// reloadKeepalived sends SIGHUP to keepalived to reload configuration
func reloadKeepalived(ctx context.Context) error {
	// keepalived reloads config on SIGHUP
	cmd := exec.CommandContext(ctx, "systemctl", "reload", keepalivedServiceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reload keepalived: %w", err)
	}
	return nil
}

// SetEtcdClient sets the etcd client for this action (used for dependency injection)
func (a *keepalivedReconcileAction) SetEtcdClient(client *clientv3.Client) {
	a.etcdClient = client
}

// validateVIPFailoverSpec validates the VIPFailoverSpec for correctness
func validateVIPFailoverSpec(spec ingress.VIPFailoverSpec) error {
	// Validate VIP
	if strings.TrimSpace(spec.VIP) == "" {
		return errors.New("VIP is required")
	}

	// Parse VIP to ensure it's valid (remove CIDR if present for validation)
	vipAddr := strings.Split(spec.VIP, "/")[0]
	if net.ParseIP(vipAddr) == nil {
		return fmt.Errorf("VIP %q is not a valid IP address", vipAddr)
	}

	// Validate Interface
	if strings.TrimSpace(spec.Interface) == "" {
		return errors.New("Interface is required")
	}

	// Check if interface exists on the system
	if _, err := net.InterfaceByName(spec.Interface); err != nil {
		return fmt.Errorf("interface %q not found on system: %w", spec.Interface, err)
	}

	// Validate VRID range
	if spec.VirtualRouterID < 1 || spec.VirtualRouterID > 255 {
		return fmt.Errorf("virtual_router_id must be in range [1-255], got %d", spec.VirtualRouterID)
	}

	// Validate Participants
	if len(spec.Participants) == 0 {
		return errors.New("Participants list is required and cannot be empty")
	}

	// Validate Priority values (if specified)
	for nodeID, priority := range spec.Priority {
		if priority < 1 || priority > 254 {
			return fmt.Errorf("priority for node %q must be in range [1-254], got %d", nodeID, priority)
		}
	}

	return nil
}
