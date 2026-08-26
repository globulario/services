package main

import (
	"context"
	"strings"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// convergedNode is a node past bootstrap with the given profiles, reporting in.
func convergedNode(id string, profiles []string) *nodeState {
	return &nodeState{
		NodeID:         id,
		Identity:       storedIdentity{Hostname: id, Ips: []string{"10.10.0.1"}},
		Profiles:       profiles,
		BootstrapPhase: BootstrapWorkloadReady,
		AgentEndpoint:  id + ":11000",
		LastSeen:       time.Now(),
		ReportedAt:     time.Now(),
	}
}

// TestOutOfScopeNodeDoesNotDeferRelease drives the classification directly:
// a profile mismatch is out of scope, a node still booting is pending.
func TestOutOfScopeNodeDoesNotDeferRelease(t *testing.T) {
	const pkg = "ai-memory"
	cat := CatalogByName(pkg)
	if cat == nil || len(cat.Profiles) == 0 {
		t.Skipf("catalog entry for %s unusable in this build", pkg)
	}

	t.Run("profile mismatch alone does not defer", func(t *testing.T) {
		bad := convergedNode("n-bad-profile", []string{"media-server"})
		srv := newTestServer(t, &controllerState{
			Nodes: map[string]*nodeState{bad.NodeID: bad},
		})
		sel, err := srv.selectReleaseTargets(context.Background(),
			[]any{bad.NodeID}, pkg, "SERVICE", "")
		if err != nil {
			t.Fatalf("selectReleaseTargets: %v", err)
		}
		if len(sel.Targets) != 0 {
			t.Fatalf("a profile-mismatched node must never be a target, got %d", len(sel.Targets))
		}
		if sel.FinalizeStatus == cluster_controllerpb.ReleasePhaseDeferred {
			t.Errorf("profile mismatch is permanent — deferring retries it forever; got %s (%q)",
				sel.FinalizeStatus, sel.Reason)
		}
		if !strings.Contains(sel.Reason, "out of scope") {
			t.Errorf("expected the reason to say the node is out of scope, got %q", sel.Reason)
		}
	})

	t.Run("a node still booting DOES defer", func(t *testing.T) {
		// Transient: this node can become a target once it finishes bootstrap.
		booting := convergedNode("n-booting", cat.Profiles[:1])
		booting.BootstrapPhase = BootstrapAdmitted
		srv := newTestServer(t, &controllerState{
			Nodes: map[string]*nodeState{booting.NodeID: booting},
		})
		sel, err := srv.selectReleaseTargets(context.Background(),
			[]any{booting.NodeID}, pkg, "SERVICE", "")
		if err != nil {
			t.Fatalf("selectReleaseTargets: %v", err)
		}
		if sel.FinalizeStatus != cluster_controllerpb.ReleasePhaseDeferred {
			t.Errorf("a node still booting is genuinely pending work; expected %s, got %s",
				cluster_controllerpb.ReleasePhaseDeferred, sel.FinalizeStatus)
		}
	})
}
