// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.executor
// @awareness file_role=bounded_auto_remediation_dispatcher_with_audit_trail
// @awareness implements=globular.platform:intent.remediation.must_go_through_workflow
// @awareness implements=globular.platform:intent.autonomy.remediation_is_bounded_and_escalates
// @awareness implements=globular.platform:intent.audit.every_authority_change_is_explainable
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
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
// node-agents for node-local actions (systemctl, file_delete,
// delete_cache_artifact). The real implementation lives in
// cluster_doctor_server/node_agent_dialer.go.
type NodeAgentDialer interface {
	// SystemctlAction runs a systemctl verb (restart/start/stop) against a
	// unit on a target node.
	SystemctlAction(ctx context.Context, nodeID, unit, verb string) (string, error)
	// FileDelete deletes a path on a target node. Callers MUST validate the
	// path against the safe-trash allowlist before calling this.
	FileDelete(ctx context.Context, nodeID, path string) error
	// DeleteCacheArtifact calls the typed node_agent.DeleteCacheArtifact
	// RPC on the given node, which removes
	// /var/lib/globular/staging/<publisher>/<package>/latest.artifact. The
	// node-agent owns path construction and re-validates publisher/package
	// against its own staging-root containment check. Callers MUST validate
	// publisher_id and package_name against isValidPackageIdentifier first.
	DeleteCacheArtifact(ctx context.Context, nodeID, publisherID, packageName string) (string, error)
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
//
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

// validPackageIdentRegex matches the publisher_id / package_name shape
// allowed for DELETE_CACHE_ARTIFACT (Patch C Milestone 3). The intent is to
// reject anything that could escape /var/lib/globular/staging/ or otherwise
// be path-meaningful: '/', '\', '..', shell metacharacters, whitespace.
// Allowed: ASCII letters, digits, dot, underscore, '@' (publisher IDs are
// email-shaped), and '-'.
var validPackageIdentRegex = regexp.MustCompile(`^[a-zA-Z0-9._@-]+$`)

const maxPackageIdentLen = 128

// isValidPackageIdentifier returns true if s is a non-empty string whose
// every character is in the [a-zA-Z0-9._@-] allowlist and whose length
// does not exceed maxPackageIdentLen. Empty strings, '/' / '\' / '..',
// shell metacharacters, and overlong values all fail.
//
// Used by DELETE_CACHE_ARTIFACT's approval gate to reject inputs that
// would let a malicious or buggy rule construct a path outside the
// staging cache root. The node-agent re-validates server-side — this
// check is defense in depth.
func isValidPackageIdentifier(s string) bool {
	if s == "" || len(s) > maxPackageIdentLen {
		return false
	}
	if strings.Contains(s, "..") {
		return false
	}
	return validPackageIdentRegex.MatchString(s)
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
	case cluster_doctorpb.ActionType_DELETE_CACHE_ARTIFACT:
		// Auto-executable iff publisher_id and package_name both pass the
		// strict identifier check (no '/', no '\', no '..', charset
		// [a-zA-Z0-9._@-], length ≤128). The node-agent re-validates
		// server-side — this check is defense in depth so a malformed rule
		// or stale evidence cannot drift a path outside the cache root.
		pub := action.GetParams()["publisher_id"]
		pkg := action.GetParams()["package_name"]
		if !isValidPackageIdentifier(pub) {
			return true, fmt.Sprintf("DELETE_CACHE_ARTIFACT publisher_id %q fails identifier validation; approval required", pub)
		}
		if !isValidPackageIdentifier(pkg) {
			return true, fmt.Sprintf("DELETE_CACHE_ARTIFACT package_name %q fails identifier validation; approval required", pkg)
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
	case cluster_doctorpb.ActionType_DELETE_CACHE_ARTIFACT:
		return e.deleteCacheArtifact(ctx, params, dryRun)
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

// deleteCacheArtifact runs DELETE_CACHE_ARTIFACT against a node-agent. The
// healer uses this for artifact.cache_digest_mismatch auto-remediation
// (Patch C Milestone 3). Path construction lives entirely in the
// node-agent; the executor passes typed (publisher_id, package_name) and
// re-validates them defensively before dialing — node-agent re-validates
// server-side as a third layer.
func (e *ActionExecutor) deleteCacheArtifact(ctx context.Context, params map[string]string, dryRun bool) (string, error) {
	nodeID := params["node_id"]
	publisherID := params["publisher_id"]
	packageName := params["package_name"]
	if nodeID == "" {
		return "", fmt.Errorf("delete_cache_artifact: missing 'node_id' param")
	}
	if publisherID == "" {
		return "", fmt.Errorf("delete_cache_artifact: missing 'publisher_id' param")
	}
	if packageName == "" {
		return "", fmt.Errorf("delete_cache_artifact: missing 'package_name' param")
	}
	// Defensive re-validation. requiresApproval already enforced this for
	// the auto-execute path; an operator-approved token path skips that
	// check, so we re-assert at the executor boundary.
	if !isValidPackageIdentifier(publisherID) {
		return "", fmt.Errorf("delete_cache_artifact refuses publisher_id %q: fails identifier validation", publisherID)
	}
	if !isValidPackageIdentifier(packageName) {
		return "", fmt.Errorf("delete_cache_artifact refuses package_name %q: fails identifier validation", packageName)
	}
	if dryRun {
		return fmt.Sprintf("would delete cache artifact: publisher=%s package=%s on node %s",
			publisherID, packageName, nodeID), nil
	}
	if e.nodeAgentDialer == nil {
		return "", fmt.Errorf("no node-agent dialer configured")
	}
	return e.nodeAgentDialer.DeleteCacheArtifact(ctx, nodeID, publisherID, packageName)
}

// ── Audit logging ────────────────────────────────────────────────────────────

// RemediationAuditRetention is the cluster-wide retention window for
// remediation audit records. Operators and compliance tooling must read
// this constant rather than the inline lease number in auditRemediation —
// the retention policy is declared, not buried. See
// docs/intent/audit.retention_and_correlation_policy.yaml.
const RemediationAuditRetention = 30 * 24 * time.Hour

// auditRemediation writes a brief audit record to etcd. Append-only, used
// for forensics. Failures here are logged but never block execution. The
// record is stamped with a correlation id (generated if absent), redacted
// of any approval-token material that may have leaked into params, and
// leased for RemediationAuditRetention.
//
func auditRemediation(ctx context.Context, audit RemediationAudit) string {
	ts := time.Now().Unix()
	audit.Timestamp = ts
	if audit.AuditID == "" {
		audit.AuditID = fmt.Sprintf("rem-%d", ts)
	}
	if audit.CorrelationID == "" {
		audit.CorrelationID = audit.AuditID
	}
	audit = audit.Redacted()
	auditEtcdPersistFn(ctx, audit)
	return audit.AuditID
}

// auditEtcdPersistFn is the package-level seam through which audit records
// reach durable storage. Production code uses persistAuditToEtcd, which
// writes a TTL-leased row to /globular/cluster_doctor/audit/rem-* in the
// cluster etcd. Tests install a no-op or in-memory capture via
// TestMain (see audit_isolation_test.go) so test runs never write to
// production etcd. Live-etcd integration tests opt back in via
// GLOBULAR_LIVE_ETCD_TESTS=1.
var auditEtcdPersistFn = persistAuditToEtcd

// persistAuditToEtcd writes the audit record as a TTL-leased entry under
// /globular/cluster_doctor/audit/. Failures are swallowed — audit writes
// must never block remediation execution (the security boundary stays
// upstream of this function). See projection-clauses.md Clause 8.
func persistAuditToEtcd(ctx context.Context, audit RemediationAudit) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return
	}
	key := "/globular/cluster_doctor/audit/" + audit.AuditID
	putCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	body := audit.JSON()
	if body == "" {
		return
	}
	lease, err := cli.Grant(putCtx, int64(RemediationAuditRetention/time.Second))
	if err == nil {
		cli.Put(putCtx, key, body, clientv3.WithLease(lease.ID))
	} else {
		cli.Put(putCtx, key, body)
	}
}

// +globular:schema:key="/globular/cluster_doctor/audit/{audit_id}"
// +globular:schema:writer="cluster-doctor"
// +globular:schema:readers="ai-watcher,ai-executor"
// +globular:schema:description="Remediation audit trail. One record per execution attempt with action type, risk, outcome, and subject. TTL 30 days."
// +globular:schema:invariants="Immutable after write; TTL-leased in etcd (30 days); never blocked by audit write failure"
// +globular:schema:since_version="0.0.1"
type RemediationAudit struct {
	AuditID        string            `json:"audit_id"`
	CorrelationID  string            `json:"correlation_id,omitempty"`  // links doctor finding → workflow run → action → verification
	WorkflowRunID  string            `json:"workflow_run_id,omitempty"` // set when the doctor was invoked from a workflow
	TokenJTI       string            `json:"token_jti,omitempty"`       // jti of the approval token that authorized the action (never the token itself)
	Timestamp      int64             `json:"timestamp"`
	FindingID      string            `json:"finding_id"`
	InvariantID    string            `json:"invariant_id,omitempty"`
	EvidenceDigest string            `json:"evidence_digest,omitempty"`
	EvidenceTrust  string            `json:"evidence_trust,omitempty"` // AUTHORITATIVE|DEGRADED|STALE|UNTRUSTED
	FindingSummary string            `json:"finding_summary,omitempty"`
	StepIndex      uint32            `json:"step_index"`
	ActionType     string            `json:"action_type"`
	Risk           string            `json:"risk"`
	DryRun         bool              `json:"dry_run"`
	Executed       bool              `json:"executed"`
	Rejected       bool              `json:"rejected"`
	Reason         string            `json:"reason,omitempty"`
	Subject        string            `json:"subject"`
	Params         map[string]string `json:"params"`
}

// Redacted returns a copy of a with any approval-token-like material in
// Params replaced by the literal "<redacted>". Keys that look like tokens,
// secrets, or passwords are stripped regardless of value. Values that look
// like JWTs (three base64 segments separated by ".") are stripped even if
// the key looks innocuous. See
// docs/intent/audit.retention_and_correlation_policy.yaml.
func (a RemediationAudit) Redacted() RemediationAudit {
	if len(a.Params) == 0 {
		return a
	}
	out := a
	out.Params = make(map[string]string, len(a.Params))
	for k, v := range a.Params {
		if isSensitiveParamKey(k) || looksLikeJWT(v) {
			out.Params[k] = "<redacted>"
			continue
		}
		out.Params[k] = v
	}
	return out
}

func isSensitiveParamKey(k string) bool {
	lk := strings.ToLower(strings.TrimSpace(k))
	switch lk {
	case "approval_token", "token", "secret", "password", "api_key", "jwt", "authorization":
		return true
	}
	return strings.Contains(lk, "token") || strings.Contains(lk, "secret") || strings.Contains(lk, "password")
}

func looksLikeJWT(v string) bool {
	v = strings.TrimSpace(v)
	if len(v) < 20 || !strings.HasPrefix(v, "ey") {
		return false
	}
	if strings.Count(v, ".") != 2 {
		return false
	}
	for _, seg := range strings.Split(v, ".") {
		if len(seg) < 2 {
			return false
		}
	}
	return true
}

// JSON returns the audit record serialized to JSON for etcd storage.
// Tiny inline implementation — audit records are write-only.
func (a RemediationAudit) JSON() string {
	b, err := json.Marshal(a)
	if err != nil {
		return ""
	}
	return string(b)
}
