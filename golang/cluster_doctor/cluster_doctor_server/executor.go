package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// ActionExecutor runs structured RemediationActions. Every execution path
// goes through typed handlers — no free-form shell. See projection-clauses.md
// Clause 8.
//
// Hand-grenade enforcement is hardcoded below and cannot be disabled by the
// rule author. Rules tag actions with a risk level as a HINT; the executor
// uses the blocklist to overrule unsafe combinations.
type ActionExecutor struct {
	// nodeAgentDialer dials a node-agent gRPC endpoint on a given node.
	// Injected so tests can swap in a fake. In production, points to the
	// cluster-controller's node-agent client pool.
	nodeAgentDialer NodeAgentDialer
}

// NodeAgentDialer is the narrow interface cluster-doctor uses to reach
// node-agents for node-local actions (systemctl, file_delete, etc.).
// The real implementation lives in cluster_doctor_server/internal/agent.go
// (or wherever the existing node-agent client lives).
type NodeAgentDialer interface {
	// SystemctlAction runs a systemctl verb (restart/start/stop) against a
	// unit on a target node.
	SystemctlAction(ctx context.Context, nodeID, unit, verb string) (string, error)
	// FileDelete deletes a path on a target node. Callers MUST validate the
	// path against the safe-trash allowlist before calling this.
	FileDelete(ctx context.Context, nodeID, path string) error
}

// Safe-trash allowlist for FILE_DELETE auto-execution. Matches stale install
// artifacts. Any FILE_DELETE whose path doesn't match is gated MEDIUM+.
var safeTrashPrefixes = []string{
	"/usr/lib/globular/bin/",
}
var safeTrashSuffixes = []string{
	".tmp",
	".bak",
}

// Unit-name prefix for systemctl actions. Restricts operations to
// Globular-managed services only.
const managedUnitPrefix = "globular-"

// hardBlocked returns (true, reason) if the action type cannot be
// auto-executed regardless of risk tag. These are the hand grenades from
// projection-clauses.md Clause 8.
func hardBlocked(action *cluster_doctorpb.RemediationAction) (bool, string) {
	switch action.GetActionType() {
	case cluster_doctorpb.ActionType_ETCD_PUT:
		return true, "ETCD_PUT actions require operator approval and narrow invariants — never auto-executable"
	case cluster_doctorpb.ActionType_ETCD_DELETE:
		return true, "ETCD_DELETE actions require operator approval and narrow invariants — never auto-executable"
	case cluster_doctorpb.ActionType_NODE_REMOVE:
		return true, "NODE_REMOVE is destructive and requires explicit CLI approval"
	}
	return false, ""
}

// isSafeTrashPath reports whether a FILE_DELETE path falls under the
// safe-trash allowlist. Only these paths can be auto-deleted.
func isSafeTrashPath(path string) bool {
	matchedPrefix := false
	for _, p := range safeTrashPrefixes {
		if strings.HasPrefix(path, p) {
			matchedPrefix = true
			break
		}
	}
	if !matchedPrefix {
		return false
	}
	for _, s := range safeTrashSuffixes {
		if strings.HasSuffix(path, s) {
			return true
		}
	}
	return false
}

// requiresApproval determines whether an action needs an approval token.
// Returns (needs_token, reason). The reason is surfaced on rejection.
func requiresApproval(action *cluster_doctorpb.RemediationAction) (bool, string) {
	// Rule-tagged risk.
	switch action.GetRisk() {
	case cluster_doctorpb.ActionRisk_RISK_HIGH:
		return true, "RISK_HIGH actions require explicit operator approval"
	case cluster_doctorpb.ActionRisk_RISK_MEDIUM:
		return true, "RISK_MEDIUM actions require operator approval"
	}

	// Type-specific overrides beyond the hard blocklist.
	switch action.GetActionType() {
	case cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
		cluster_doctorpb.ActionType_SYSTEMCTL_START:
		unit := action.GetParams()["unit"]
		if !strings.HasPrefix(unit, managedUnitPrefix) {
			return true, fmt.Sprintf("systemctl actions auto-execute only on %s*, got %q", managedUnitPrefix, unit)
		}
		return false, ""
	case cluster_doctorpb.ActionType_SYSTEMCTL_STOP:
		// Stopping a service is disruptive even for managed units. Treat as
		// MEDIUM even if the rule author tagged LOW.
		return true, "SYSTEMCTL_STOP requires approval — stopping services is disruptive"
	case cluster_doctorpb.ActionType_FILE_DELETE:
		path := action.GetParams()["path"]
		if !isSafeTrashPath(path) {
			return true, fmt.Sprintf("FILE_DELETE outside safe-trash allowlist requires approval (path=%q)", path)
		}
		return false, ""
	case cluster_doctorpb.ActionType_PACKAGE_REINSTALL:
		return true, "PACKAGE_REINSTALL requires approval — replaces running binaries"
	}
	// Unknown action types require approval by default.
	return true, fmt.Sprintf("action_type %s requires approval by default", action.GetActionType())
}

// Execute runs the action (or validates it in dry-run mode). Does NOT check
// approval — callers verify approval tokens first. Returns a string summary
// of the outcome suitable for the audit log and CLI output.
func (e *ActionExecutor) Execute(ctx context.Context, action *cluster_doctorpb.RemediationAction, dryRun bool) (string, error) {
	if action == nil {
		return "", fmt.Errorf("action is nil")
	}

	// Hard blocklist — never runs even with approval (those actions go
	// through a separate higher-ceremony path not in this RPC).
	if blocked, reason := hardBlocked(action); blocked {
		return "", fmt.Errorf("action blocked: %s", reason)
	}

	params := action.GetParams()

	switch action.GetActionType() {
	case cluster_doctorpb.ActionType_SYSTEMCTL_RESTART:
		return e.systemctl(ctx, params, "restart", dryRun)
	case cluster_doctorpb.ActionType_SYSTEMCTL_START:
		return e.systemctl(ctx, params, "start", dryRun)
	case cluster_doctorpb.ActionType_SYSTEMCTL_STOP:
		return e.systemctl(ctx, params, "stop", dryRun)
	case cluster_doctorpb.ActionType_FILE_DELETE:
		return e.fileDelete(ctx, params, dryRun)
	}
	return "", fmt.Errorf("action_type %s not supported by executor", action.GetActionType())
}

func (e *ActionExecutor) systemctl(ctx context.Context, params map[string]string, verb string, dryRun bool) (string, error) {
	unit := params["unit"]
	nodeID := params["node_id"]
	if unit == "" {
		return "", fmt.Errorf("systemctl: missing 'unit' param")
	}
	if nodeID == "" {
		return "", fmt.Errorf("systemctl: missing 'node_id' param")
	}
	if !strings.HasPrefix(unit, managedUnitPrefix) {
		return "", fmt.Errorf("systemctl refuses unit %q: not a Globular-managed unit", unit)
	}
	if dryRun {
		return fmt.Sprintf("would run: systemctl %s %s on node %s", verb, unit, nodeID), nil
	}
	if e.nodeAgentDialer == nil {
		return "", fmt.Errorf("no node-agent dialer configured")
	}
	return e.nodeAgentDialer.SystemctlAction(ctx, nodeID, unit, verb)
}

func (e *ActionExecutor) fileDelete(ctx context.Context, params map[string]string, dryRun bool) (string, error) {
	path := params["path"]
	nodeID := params["node_id"]
	if path == "" {
		return "", fmt.Errorf("file_delete: missing 'path' param")
	}
	if nodeID == "" {
		return "", fmt.Errorf("file_delete: missing 'node_id' param")
	}
	if !isSafeTrashPath(path) {
		return "", fmt.Errorf("file_delete refuses path %q: not in safe-trash allowlist", path)
	}
	if dryRun {
		return fmt.Sprintf("would delete: %s on node %s", path, nodeID), nil
	}
	if e.nodeAgentDialer == nil {
		return "", fmt.Errorf("no node-agent dialer configured")
	}
	if err := e.nodeAgentDialer.FileDelete(ctx, nodeID, path); err != nil {
		return "", err
	}
	return fmt.Sprintf("deleted %s on %s", path, nodeID), nil
}

// ── Audit logging ────────────────────────────────────────────────────────────

// auditRemediation writes a brief audit record to etcd. Append-only, used
// for forensics. Failures here are logged but never block execution.
func auditRemediation(ctx context.Context, audit RemediationAudit) string {
	ts := time.Now().Unix()
	audit.Timestamp = ts
	if audit.AuditID == "" {
		audit.AuditID = fmt.Sprintf("rem-%d", ts)
	}
	cli, err := config.GetEtcdClient()
	if err != nil {
		return audit.AuditID
	}
	key := "/globular/cluster_doctor/audit/" + audit.AuditID
	putCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if body := audit.JSON(); body != "" {
		// TTL: 30 days
		lease, err := cli.Grant(putCtx, 30*24*3600)
		if err == nil {
			cli.Put(putCtx, key, body, clientv3.WithLease(lease.ID))
		} else {
			cli.Put(putCtx, key, body)
		}
	}
	return audit.AuditID
}

// +globular:schema:key="/globular/cluster_doctor/audit/{audit_id}"
// +globular:schema:writer="cluster-doctor"
// +globular:schema:readers="ai-watcher,ai-executor"
// +globular:schema:description="Remediation audit trail. One record per execution attempt with action type, risk, outcome, and subject. TTL 30 days."
// +globular:schema:invariants="Immutable after write; TTL-leased in etcd (30 days); never blocked by audit write failure"
// +globular:schema:since_version="0.0.1"
type RemediationAudit struct {
	AuditID    string `json:"audit_id"`
	Timestamp  int64  `json:"timestamp"`
	FindingID  string `json:"finding_id"`
	StepIndex  uint32 `json:"step_index"`
	ActionType string `json:"action_type"`
	Risk       string `json:"risk"`
	DryRun     bool   `json:"dry_run"`
	Executed   bool   `json:"executed"`
	Rejected   bool   `json:"rejected"`
	Reason     string `json:"reason,omitempty"`
	Subject    string `json:"subject"`
	Params     map[string]string `json:"params"`
}

// JSON returns the audit record serialized to JSON for etcd storage.
// Tiny inline implementation — audit records are write-only.
func (a RemediationAudit) JSON() string {
	// Use fmt.Sprintf for stability — avoids pulling encoding/json just
	// for an append-only audit record. The shape is fixed.
	paramsStr := "{"
	first := true
	for k, v := range a.Params {
		if !first {
			paramsStr += ","
		}
		paramsStr += fmt.Sprintf("%q:%q", k, v)
		first = false
	}
	paramsStr += "}"
	return fmt.Sprintf(
		`{"audit_id":%q,"timestamp":%d,"finding_id":%q,"step_index":%d,"action_type":%q,"risk":%q,"dry_run":%t,"executed":%t,"rejected":%t,"reason":%q,"subject":%q,"params":%s}`,
		a.AuditID, a.Timestamp, a.FindingID, a.StepIndex,
		a.ActionType, a.Risk, a.DryRun, a.Executed, a.Rejected,
		a.Reason, a.Subject, paramsStr,
	)
}
