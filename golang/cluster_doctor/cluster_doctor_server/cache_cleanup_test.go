package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// recordingNodeAgentDialer captures every method invocation so tests can
// assert (a) which RPC was called, (b) with what params, and (c) whether
// the dialer was called at all in dry-run mode.
type recordingNodeAgentDialer struct {
	mu                       sync.Mutex
	systemctlCalls           []string
	fileDeleteCalls          []string
	deleteCacheArtifactCalls []deleteCacheCall
}

type deleteCacheCall struct {
	NodeID      string
	PublisherID string
	PackageName string
}

func (r *recordingNodeAgentDialer) SystemctlAction(_ context.Context, nodeID, unit, verb string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.systemctlCalls = append(r.systemctlCalls, verb+":"+unit+":"+nodeID)
	return "ok", nil
}

func (r *recordingNodeAgentDialer) FileDelete(_ context.Context, nodeID, path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fileDeleteCalls = append(r.fileDeleteCalls, path+":"+nodeID)
	return nil
}

func (r *recordingNodeAgentDialer) DeleteCacheArtifact(_ context.Context, nodeID, publisherID, packageName string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deleteCacheArtifactCalls = append(r.deleteCacheArtifactCalls, deleteCacheCall{
		NodeID: nodeID, PublisherID: publisherID, PackageName: packageName,
	})
	return "deleted cache: publisher=" + publisherID + " package=" + packageName, nil
}

// cacheCleanupFinding builds a Finding shaped like what
// rules.artifactIntegrity emits for cache_digest_mismatch — a HealAuto
// disposition + a structured DELETE_CACHE_ARTIFACT step at index 0 with
// the canonical (node_id, publisher_id, package_name) params.
func cacheCleanupFinding(findingID, nodeID, publisher, pkg string) rules.Finding {
	return rules.Finding{
		FindingID:   findingID,
		InvariantID: "artifact.cache_digest_mismatch",
		Summary:     "Cached artifact digest mismatch",
		EntityRef:   nodeID + "/" + pkg,
		Evidence: []*cluster_doctorpb.Evidence{{
			SourceService: "cluster_doctor",
			SourceRpc:     "snapshot",
			KeyValues: map[string]string{
				"node":         nodeID,
				"package":      pkg,
				"publisher_id": publisher,
			},
			Timestamp: timestamppb.Now(),
		}},
		Remediation: []*cluster_doctorpb.RemediationStep{{
			Order:       1,
			Description: "Delete the stale cached artifact",
			Action: &cluster_doctorpb.RemediationAction{
				ActionType: cluster_doctorpb.ActionType_DELETE_CACHE_ARTIFACT,
				Risk:       cluster_doctorpb.ActionRisk_RISK_LOW,
				Idempotent: true,
				Params: map[string]string{
					"node_id":      nodeID,
					"publisher_id": publisher,
					"package_name": pkg,
				},
			},
		}},
	}
}

// TestDeleteCacheArtifact_RejectsUnsafePublisherOrPackage walks the
// approval-gate matrix: malformed publisher_id or package_name values
// must require an approval token (i.e., be auto-rejected when no token
// is supplied). The canonical pair is auto-executable.
//
// Locks the M3 safety contract: even though the node-agent re-validates
// server-side, the executor's approval branch rejects bad inputs BEFORE
// dialing — failing fast and writing an audit record for the rejection.
func TestDeleteCacheArtifact_RejectsUnsafePublisherOrPackage(t *testing.T) {
	cases := []struct {
		name        string
		publisher   string
		pkg         string
		wantApprove bool
	}{
		{"canonical pair auto-executable", "core@globular.io", "event", false},
		{"empty publisher", "", "event", true},
		{"empty package", "core@globular.io", "", true},
		{"publisher with slash", "core@/etc", "event", true},
		{"package with slash", "core@globular.io", "evil/../etc", true},
		{"package with .. traversal", "core@globular.io", "..", true},
		{"publisher with backslash", "core@globular.io\\admin", "event", true},
		{"package with shell metachar", "core@globular.io", "event;rm -rf /", true},
		{"publisher with newline", "core@globular.io\n", "event", true},
		{"package with space", "core@globular.io", "event service", true},
		{"package too long", "core@globular.io", strings.Repeat("a", 129), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			action := &cluster_doctorpb.RemediationAction{
				ActionType: cluster_doctorpb.ActionType_DELETE_CACHE_ARTIFACT,
				Risk:       cluster_doctorpb.ActionRisk_RISK_LOW,
				Params: map[string]string{
					"node_id":      "node-1",
					"publisher_id": tc.publisher,
					"package_name": tc.pkg,
				},
			}
			needsApproval, reason := requiresApproval(action)
			if needsApproval != tc.wantApprove {
				t.Fatalf("requiresApproval = %v (reason=%q), want %v",
					needsApproval, reason, tc.wantApprove)
			}
		})
	}
}

// TestCacheCleanup_WritesSingleEtcdAudit verifies that one successful
// dispatch through ExecuteRemediation produces exactly one audit record.
// Audit records are written by auditRemediation() inside
// ExecuteRemediation — there must be no parallel audit ring for the
// healer cycle.
//
// The capture stub from withStubbedAuditEtcd intercepts the persist
// call so we can count records WITHOUT touching production etcd. The
// TestMain default already prevents leaks; this stub upgrades the test
// from "no leak" to "exactly-one-record" assertion.
func TestCacheCleanup_WritesSingleEtcdAudit(t *testing.T) {
	withStubbedGatePersistence(t)
	captured := withStubbedAuditEtcd(t)

	dialer := &recordingNodeAgentDialer{}
	srv := &ClusterDoctorServer{
		cfg:      defaultConfig(),
		executor: &ActionExecutor{nodeAgentDialer: dialer},
	}
	srv.isAuthoritative.Store(true)

	f := cacheCleanupFinding("f-audit-1", "node-uuid-1", "core@globular.io", "event")
	srv.lastFindings = []rules.Finding{f}

	resp, err := srv.ExecuteRemediation(context.Background(), &cluster_doctorpb.ExecuteRemediationRequest{
		FindingId: f.FindingID,
		StepIndex: 0,
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("ExecuteRemediation err=%v", err)
	}
	if !resp.GetExecuted() {
		t.Fatalf("expected Executed=true, got status=%q reason=%q",
			resp.GetStatus(), resp.GetReason())
	}
	if resp.GetAuditId() == "" {
		t.Fatalf("expected non-empty AuditId (canonical /globular/cluster_doctor/audit/rem-* form)")
	}
	if !strings.HasPrefix(resp.GetAuditId(), "rem-") {
		t.Fatalf("AuditId %q must use the canonical rem-* shape", resp.GetAuditId())
	}
	// Capture proof: exactly one audit persist call.
	if got := len(*captured); got != 1 {
		t.Fatalf("expected exactly 1 audit record persisted, got %d: %+v", got, *captured)
	}
	rec := (*captured)[0]
	if rec.FindingID != f.FindingID {
		t.Fatalf("audit FindingID = %q, want %q", rec.FindingID, f.FindingID)
	}
	if rec.ActionType != "DELETE_CACHE_ARTIFACT" {
		t.Fatalf("audit ActionType = %q, want DELETE_CACHE_ARTIFACT", rec.ActionType)
	}
	if !rec.Executed {
		t.Fatalf("audit Executed = false, want true")
	}
	if rec.DryRun {
		t.Fatalf("audit DryRun = true, want false")
	}
	// Exactly one node-agent dispatch.
	if got := len(dialer.deleteCacheArtifactCalls); got != 1 {
		t.Fatalf("expected exactly 1 DeleteCacheArtifact dial, got %d: %+v",
			got, dialer.deleteCacheArtifactCalls)
	}
	// No other RPCs were called.
	if got := len(dialer.systemctlCalls); got != 0 {
		t.Fatalf("unexpected SystemctlAction calls: %+v", dialer.systemctlCalls)
	}
	if got := len(dialer.fileDeleteCalls); got != 0 {
		t.Fatalf("unexpected FileDelete calls: %+v", dialer.fileDeleteCalls)
	}
}

// TestCacheCleanup_DryRun_NoMutation verifies that DryRun=true forwards
// to the executor's DELETE_CACHE_ARTIFACT handler and produces NO
// node-agent dial. The handler returns a "would delete" string; the
// audit record reflects dry_run=true.
func TestCacheCleanup_DryRun_NoMutation(t *testing.T) {
	withStubbedGatePersistence(t)
	captured := withStubbedAuditEtcd(t)

	dialer := &recordingNodeAgentDialer{}
	srv := &ClusterDoctorServer{
		cfg:      defaultConfig(),
		executor: &ActionExecutor{nodeAgentDialer: dialer},
	}
	srv.isAuthoritative.Store(true)

	f := cacheCleanupFinding("f-dry-1", "node-uuid-2", "core@globular.io", "event")
	srv.lastFindings = []rules.Finding{f}

	resp, err := srv.ExecuteRemediation(context.Background(), &cluster_doctorpb.ExecuteRemediationRequest{
		FindingId: f.FindingID,
		StepIndex: 0,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("ExecuteRemediation err=%v", err)
	}
	if resp.GetExecuted() {
		t.Fatalf("DryRun=true must produce Executed=false; got Executed=true")
	}
	if resp.GetStatus() != "dry_run_ok" {
		t.Fatalf("status = %q, want dry_run_ok", resp.GetStatus())
	}
	if !strings.Contains(resp.GetOutput(), "would delete cache artifact") {
		t.Fatalf("dry-run output should describe the would-be action; got %q", resp.GetOutput())
	}
	// No node-agent dispatch in dry-run.
	if got := len(dialer.deleteCacheArtifactCalls); got != 0 {
		t.Fatalf("DryRun=true must NOT invoke DeleteCacheArtifact dialer; got %d calls: %+v",
			got, dialer.deleteCacheArtifactCalls)
	}
	// Audit record exists (gate observability) but reflects dry_run=true /
	// executed=false. No state mutation; full audit trail.
	if got := len(*captured); got != 1 {
		t.Fatalf("expected 1 audit record (dry-run is still audited), got %d", got)
	}
	rec := (*captured)[0]
	if !rec.DryRun {
		t.Fatalf("audit DryRun = false, want true")
	}
	if rec.Executed {
		t.Fatalf("audit Executed = true, want false (dry-run)")
	}
}

// TestCacheCleanup_FindingNotClearedUntilVerification verifies that
// ExecuteRemediation does NOT mutate s.lastFindings — the finding clears
// only when the next snapshot evaluation no longer produces it. This is
// the M3 verification contract: healing is proved by re-evaluation, not
// by the action itself.
func TestCacheCleanup_FindingNotClearedUntilVerification(t *testing.T) {
	withStubbedGatePersistence(t)

	dialer := &recordingNodeAgentDialer{}
	srv := &ClusterDoctorServer{
		cfg:      defaultConfig(),
		executor: &ActionExecutor{nodeAgentDialer: dialer},
	}
	srv.isAuthoritative.Store(true)

	f := cacheCleanupFinding("f-verify-1", "node-uuid-3", "core@globular.io", "event")
	srv.lastFindings = []rules.Finding{f}

	// Snapshot lastFindings before dispatch.
	if len(srv.lastFindings) != 1 {
		t.Fatalf("precondition: expected 1 finding in lastFindings, got %d", len(srv.lastFindings))
	}

	resp, err := srv.ExecuteRemediation(context.Background(), &cluster_doctorpb.ExecuteRemediationRequest{
		FindingId: f.FindingID,
		StepIndex: 0,
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("ExecuteRemediation err=%v", err)
	}
	if !resp.GetExecuted() {
		t.Fatalf("precondition: expected executed dispatch, got status=%q", resp.GetStatus())
	}

	// CRITICAL: lastFindings must still contain the finding. Clearing
	// happens via the next snapshot's EvaluateAll cycle, not by the
	// dispatch itself.
	if got := len(srv.lastFindings); got != 1 {
		t.Fatalf("after dispatch, lastFindings should still contain the finding (clears on re-evaluation); got %d entries",
			got)
	}
	if found, ok := rules.FindByID(srv.lastFindings, f.FindingID); !ok || found.FindingID != f.FindingID {
		t.Fatalf("finding %s must persist in lastFindings until re-evaluation; FindByID(%s)=%v ok=%v",
			f.FindingID, f.FindingID, found, ok)
	}

	// Simulate a fresh snapshot showing the underlying state is healed
	// (cache no longer mismatched → no finding emitted). The doctor's
	// cacheFindings call publishes the empty set; the finding clears.
	srv.cacheFindings([]rules.Finding{}, true)

	if got := len(srv.lastFindings); got != 0 {
		t.Fatalf("after re-evaluation with cache restored, finding should clear; lastFindings has %d entries",
			got)
	}

	_ = time.Now() // keep the time import live in case future assertions need timestamps
}
