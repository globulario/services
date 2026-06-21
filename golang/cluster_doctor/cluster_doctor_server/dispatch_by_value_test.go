package main

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestGatedDispatcher_ResolvesFindingByValue_NoCacheClobber locks in the AWG
// re-audit fix (meta.code.local_state_must_not_become_hidden_authority): the
// healer dispatcher must execute against the Finding it already holds, NOT by
// clobbering the shared lastFindings lookup cache and round-tripping through
// finding_id resolution. The previous code overwrote lastFindings with a
// one-element slice; a concurrent GetClusterReport could overwrite it again
// between the write and the read, so remediation acted on the wrong evidence
// (or spuriously NotFound).
func TestGatedDispatcher_ResolvesFindingByValue_NoCacheClobber(t *testing.T) {
	withStubbedGatePersistence(t)

	action := &cluster_doctorpb.RemediationAction{
		ActionType: cluster_doctorpb.ActionType_SYSTEMCTL_RESTART,
		Risk:       cluster_doctorpb.ActionRisk_RISK_LOW,
		Params:     map[string]string{"unit": "globular-node-agent.service", "node_id": "node-1"},
	}
	target := rules.Finding{
		FindingID:   "dispatch-target",
		InvariantID: "runtime.desired_enabled_not_alive",
		Summary:     "unit is not running",
		EntityRef:   "node-1",
		Evidence: []*cluster_doctorpb.Evidence{{
			SourceService: "cluster_controller",
			SourceRpc:     "GetClusterHealthV1",
			KeyValues:     map[string]string{"node": "node-1"},
			Timestamp:     timestamppb.Now(),
		}},
		Remediation: []*cluster_doctorpb.RemediationStep{{Order: 1, Action: action}},
	}
	key := remediationGateKey(target.FindingID, 0, action.GetActionType())
	autoRemediationCooldownByTarget.Delete(key)
	autoRemediationGateByTarget.Delete(key)

	// The shared lookup cache holds a DIFFERENT finding — as if a concurrent
	// report RPC populated it. The dispatcher must not depend on it, and must
	// not overwrite it.
	other := rules.Finding{FindingID: "some-other-finding"}
	srv := &ClusterDoctorServer{
		executor:     &ActionExecutor{nodeAgentDialer: &fakeNodeAgentDialer{}},
		lastFindings: []rules.Finding{other},
	}
	srv.isAuthoritative.Store(true)

	g := &gatedDispatcher{server: srv}
	executed, _, err := g.Dispatch(context.Background(), target, "restart", false)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !executed {
		t.Fatal("dispatcher must resolve the finding by value and execute, even though it is absent from lastFindings")
	}

	// The shared cache must be untouched (NOT clobbered to [target]).
	srv.lastFindingsMu.RLock()
	defer srv.lastFindingsMu.RUnlock()
	if len(srv.lastFindings) != 1 || srv.lastFindings[0].FindingID != "some-other-finding" {
		t.Errorf("dispatcher must not clobber lastFindings; got %+v", srv.lastFindings)
	}
}
