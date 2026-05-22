package main

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// TestHasUnservedNodes_SkipsStaleHeartbeatNode verifies that a node with a stale
// heartbeat is not counted as "unserved" by hasUnservedNodes. This prevents the
// FAILED→PENDING→FAILED cycle that occurs when a dead node (e.g. removed and not
// yet rejoined) keeps triggering release re-dispatch indefinitely.
func TestHasUnservedNodes_SkipsStaleHeartbeatNode(t *testing.T) {
	t.Helper()

	liveNode := &nodeState{
		NodeID:         "live",
		Status:         "ready",
		BootstrapPhase: BootstrapWorkloadReady,
		LastSeen:       time.Now(),
		InstalledVersions: map[string]string{
			"event": "1.2.63",
		},
	}
	deadNode := &nodeState{
		NodeID:         "dead",
		Status:         "ready",
		BootstrapPhase: BootstrapWorkloadReady,
		// Stale: last seen > heartbeatStaleThreshold ago.
		LastSeen: time.Now().Add(-(heartbeatStaleThreshold + time.Minute)),
	}

	srv := newTestServer(t, &controllerState{
		Nodes: map[string]*nodeState{
			"live": liveNode,
			"dead": deadNode,
		},
	})

	// h represents a release that is AVAILABLE on the live node only.
	h := &releaseHandle{
		Name:               "core@globular.io/event",
		ResourceType:       "ServiceRelease",
		InstalledStateName: "event",
		InstalledStateKind: "SERVICE",
		ResolvedVersion:    "1.2.63",
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "live", Phase: cluster_controllerpb.ReleasePhaseAvailable},
		},
	}

	blocked := map[string]struct{}{}
	unserved := srv.hasUnservedNodes(h, blocked)
	if unserved {
		t.Fatal("hasUnservedNodes must return false when the only unserved node has a stale heartbeat (dead node)")
	}
}

// TestHasUnservedNodes_DetectsLiveUnservedNode verifies that a live node that
// hasn't received the release is still detected as unserved.
func TestHasUnservedNodes_DetectsLiveUnservedNode(t *testing.T) {
	t.Helper()

	liveServed := &nodeState{
		NodeID:         "n1",
		Status:         "ready",
		BootstrapPhase: BootstrapWorkloadReady,
		LastSeen:       time.Now(),
		InstalledVersions: map[string]string{
			"event": "1.2.63",
		},
	}
	liveUnserved := &nodeState{
		NodeID:         "n2",
		Status:         "ready",
		BootstrapPhase: BootstrapWorkloadReady,
		LastSeen:       time.Now(), // fresh heartbeat
	}

	srv := newTestServer(t, &controllerState{
		Nodes: map[string]*nodeState{
			"n1": liveServed,
			"n2": liveUnserved,
		},
	})

	h := &releaseHandle{
		Name:               "core@globular.io/event",
		ResourceType:       "ServiceRelease",
		InstalledStateName: "event",
		InstalledStateKind: "SERVICE",
		ResolvedVersion:    "1.2.63",
		Nodes: []*cluster_controllerpb.NodeReleaseStatus{
			{NodeID: "n1", Phase: cluster_controllerpb.ReleasePhaseAvailable},
			// n2 not in Nodes → unserved
		},
	}

	blocked := map[string]struct{}{}
	unserved := srv.hasUnservedNodes(h, blocked)
	if !unserved {
		t.Fatal("hasUnservedNodes must return true when a live node has not received the release")
	}
}

// TestStaleNodeIPs_ReturnsStaleOnly verifies that staleNodeIPs only includes IPs
// of nodes whose heartbeat exceeds heartbeatStaleThreshold.
func TestStaleNodeIPs_ReturnsStaleOnly(t *testing.T) {
	t.Helper()

	liveNode := &nodeState{
		NodeID:   "live",
		LastSeen: time.Now(),
		Identity: storedIdentity{Ips: []string{"10.0.0.1"}},
	}
	deadNode := &nodeState{
		NodeID:   "dead",
		LastSeen: time.Now().Add(-(heartbeatStaleThreshold + time.Minute)),
		Identity: storedIdentity{Ips: []string{"10.0.0.20"}},
	}

	srv := newTestServer(t, &controllerState{
		Nodes: map[string]*nodeState{
			"live": liveNode,
			"dead": deadNode,
		},
	})

	stale := srv.staleNodeIPs()

	if stale["10.0.0.1"] {
		t.Error("10.0.0.1 (live node) must NOT be in stale set")
	}
	if !stale["10.0.0.20"] {
		t.Error("10.0.0.20 (dead node) must be in stale set")
	}
}
