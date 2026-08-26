package main

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// TestDriftIgnoresGhostNodes pins that a release is never judged against a node
// the cluster no longer has.
//
// detectServiceDrift iterates the release's OWN recorded node list and looks the
// node up in srv.state.Nodes, which is membership authority. When that lookup
// missed, execution fell through to the version and health probes, every one of
// which reads from the nil node and fails, so the release was marked DEGRADED →
// FAILED → PENDING on every pass. Nothing about a node that does not exist ever
// changes, so the release could never recover.
//
// Observed live on the 5-node sim: ServiceRelease persistence cycled 75 times in
// a ten-minute window against node 2da500c8-32d8-5ffc-8452-6d8af5c02038, which
// was absent from the node list and had zero keys under /globular/nodes/, while
// all five real nodes were reported "artifact+runtime converged" in the same
// pass. That churn is why a cluster-wide release audit never reached failed:0,
// which failed platform-upgrade-release-boundary and rollback-guard.
func TestDriftIgnoresGhostNodes(t *testing.T) {
	const ghostID = "2da500c8-32d8-5ffc-8452-6d8af5c02038"
	const realID = "c777633e-6d07-5713-9c4c-deb3317eee25"

	real := &nodeState{
		NodeID:         realID,
		Identity:       storedIdentity{Hostname: "node-1", Ips: []string{"10.10.0.11"}},
		Profiles:       []string{"core", "control-plane"},
		BootstrapPhase: BootstrapWorkloadReady,
		LastSeen:       time.Now(),
		ReportedAt:     time.Now(),
	}
	// The ghost is deliberately NOT in state.Nodes — that is what makes it a ghost.
	srv := newTestServer(t, &controllerState{
		Nodes: map[string]*nodeState{realID: real},
	})

	rel := &cluster_controllerpb.ServiceRelease{
		Spec: &cluster_controllerpb.ServiceReleaseSpec{ServiceName: "persistence"},
	}
	var persisted statusPatch
	patched := false
	h := &releaseHandle{
		Name:               "core@globular.io/persistence",
		ResourceType:       "ServiceRelease",
		InstalledStateName: "persistence",
		InstalledStateKind: "SERVICE",
		Phase:              cluster_controllerpb.ReleasePhaseAvailable,
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: realID, Phase: cluster_controllerpb.ReleasePhaseAvailable},
			{NodeID: ghostID, Phase: cluster_controllerpb.ReleasePhaseAvailable},
		},
		PatchStatus: func(_ context.Context, p statusPatch) error {
			persisted = p
			patched = true
			return nil
		},
	}

	_ = srv.detectServiceDrift(context.Background(), rel, h)

	if !patched {
		t.Skip("detectServiceDrift returned before persisting; nothing to assert in this fixture")
	}
	for _, n := range persisted.Nodes {
		if n == nil {
			continue
		}
		if n.NodeID == ghostID {
			t.Fatalf("ghost node %s survived into the persisted release node list "+
				"(phase %q) — a node the cluster does not have cannot be drifting, and "+
				"the verdict can never be cleared", ghostID, n.Phase)
		}
	}
}
