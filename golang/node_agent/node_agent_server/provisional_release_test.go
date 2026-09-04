package main

import (
	"testing"
)

// TestAdoptingACanonicalIDReleasesBothSuppressions pins that the provisional
// guards have an EXIT.
//
// nodeIDProvisional gates two writers — syncInstalledStateToEtcd (0dd286dd) and
// the metrics-port persister (dd38635c). Both were added to stop a node filing
// records under an identity the cluster had not granted. Neither cleared the
// flag, so on any node that started provisional the suppression LATCHED for the
// life of the process: installed state never synced, and no metrics-port key was
// ever written under the canonical id.
//
// That second one is load-bearing beyond metrics: the quickstart harness counts
// cluster members by the metrics-port key, so a node missing it reads as absent.
// On 1.2.340 it surfaced as all_nodes_heartbeating "expected >= 5, got 4",
// failing three functional scenarios on a cluster whose five agents were all up
// with zero restarts.
func TestAdoptingACanonicalIDReleasesBothSuppressions(t *testing.T) {
	const provisional = "12944a1b-cfae-5d2f-8056-e8f633c8d3dd"
	const granted = "c8a09d9e-3813-5357-ab58-93aa410f27fb"

	var persistedID string
	var persistedPort int
	orig := persistMetricsPortFn
	persistMetricsPortFn = func(nodeID string, port int) {
		persistedID, persistedPort = nodeID, port
	}
	t.Cleanup(func() { persistMetricsPortFn = orig })

	boundMetricsPort.Store(11001)

	srv := &NodeAgentServer{
		nodeID:            provisional,
		nodeIDProvisional: true,
		state:             &nodeAgentState{NodeID: provisional},
		// statePath empty: saveState is a no-op, keeping this a unit test.
	}
	if !srv.nodeIDProvisional {
		t.Fatal("precondition: the agent should start provisional in this fixture")
	}

	srv.applyApprovedNodeID(granted)

	t.Run("the provisional latch is released", func(t *testing.T) {
		if srv.nodeIDProvisional {
			t.Error("nodeIDProvisional survived adoption — installed-state sync stays " +
				"suppressed for the life of the process")
		}
	})

	t.Run("the granted id is adopted", func(t *testing.T) {
		if srv.nodeID != granted {
			t.Errorf("nodeID = %q, want %q", srv.nodeID, granted)
		}
	})

	t.Run("the metrics port is persisted under the granted id", func(t *testing.T) {
		if persistedID != granted {
			t.Errorf("metrics port persisted under %q, want %q — a node with no key "+
				"under its canonical subtree reads as absent to anything counting them",
				persistedID, granted)
		}
		if persistedPort != 11001 {
			t.Errorf("persisted port = %d, want 11001", persistedPort)
		}
	})
}

// TestNoMetricsPortPersistedWhenNothingBound guards the other direction: with no
// listener bound there is no port to file, and adoption must not invent one.
func TestNoMetricsPortPersistedWhenNothingBound(t *testing.T) {
	called := false
	orig := persistMetricsPortFn
	persistMetricsPortFn = func(string, int) { called = true }
	t.Cleanup(func() { persistMetricsPortFn = orig })

	boundMetricsPort.Store(0)
	srv := &NodeAgentServer{
		nodeID:            "provisional-id",
		nodeIDProvisional: true,
		state:             &nodeAgentState{NodeID: "provisional-id"},
	}
	srv.applyApprovedNodeID("c8a09d9e-3813-5357-ab58-93aa410f27fb")

	if called {
		t.Error("persisted a metrics port with no listener bound")
	}
}
