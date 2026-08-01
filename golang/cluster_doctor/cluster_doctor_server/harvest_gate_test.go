package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
)

// These tests guard a DELIBERATE LOOSENING of a safety gate, so each case states
// what it permits and why that is still safe.
//
// The invariant kept: never auto-remediate a finding derived from data that
// could not be collected (doctor.healer_auto_remediation_on_reduced_harvest).
// The invariant dropped: that one unreachable service anywhere disables every
// repair in the cluster. That was never the safety property — it was the
// granularity, and it made the healer stand down hardest exactly when the
// cluster was degraded.

const (
	gtNode  = "c777633e-6d07-5713-9c4c-deb3317eee25"
	gtOther = "a166b992-b66d-53cb-b7c7-61dfa4dd5a36"
)

func gtSnapshot(incomplete bool, errs ...collector.DataError) *collector.Snapshot {
	return &collector.Snapshot{DataIncomplete: incomplete, DataErrors: errs}
}

func gtUnitFinding(node string) rules.Finding {
	return rules.Finding{
		FindingID:   "f-1",
		InvariantID: "node.systemd.units_running",
		EntityRef:   node + "/globular-torrent.service",
	}
}

// TestHarvestGate_FullHarvestAllowsEverything: with nothing failed there is
// nothing to be conservative about.
func TestHarvestGate_FullHarvestAllowsEverything(t *testing.T) {
	g := newHarvestGate(gtSnapshot(false))
	if ok, why := g.enforceable(gtUnitFinding(gtNode)); !ok {
		t.Errorf("a complete harvest must not suppress anything, got: %s", why)
	}
}

// TestHarvestGate_UnrelatedSourceDoesNotBlock is the whole point of the change.
//
// Every one of the four bring-ups on 2026-08-01 was blocked by a source the
// drifted-unit finding never reads. Under the old cycle-wide rule each of these
// vetoed all repairs cluster-wide.
func TestHarvestGate_UnrelatedSourceDoesNotBlock(t *testing.T) {
	for _, tc := range []struct{ name, service, rpc string }{
		{"controller unreachable (node-3 envoy)", "cluster_controller", "GetClusterHealthV1"},
		{"workflow degraded by scylla", "workflow", "ListCorrelationDeferState"},
		{"repository", "repository", "ListArtifacts"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := newHarvestGate(gtSnapshot(true,
				collector.DataError{Service: tc.service, RPC: tc.rpc, Err: errors.New("unavailable")}))

			ok, why := g.enforceable(gtUnitFinding(gtNode))
			if !ok {
				t.Errorf("a units_running finding reads node_agent, not %q — it must not be\n"+
					"suppressed because an unrelated source failed.\ngot: %s", tc.service, why)
			}
		})
	}
}

// TestHarvestGate_DegradedSourceOnSameNodeBlocks: the safety property itself.
// The finding is derived from this node's agent inventory, and that inventory
// is exactly what failed — so the finding may be a false positive.
func TestHarvestGate_DegradedSourceOnSameNodeBlocks(t *testing.T) {
	g := newHarvestGate(gtSnapshot(true,
		collector.DataError{Service: "node_agent@" + gtNode, RPC: "GetInventory", Err: errors.New("deadline")}))

	ok, why := g.enforceable(gtUnitFinding(gtNode))
	if ok {
		t.Fatal("a finding derived from the node_agent inventory of the SAME node whose " +
			"inventory failed to collect must not be auto-remediated")
	}
	if !strings.Contains(why, gtNode) {
		t.Errorf("the reason must name the node so an operator can act on it, got: %s", why)
	}
}

// TestHarvestGate_DegradedSourceOnOtherNodeDoesNotBlock: node-scoped failures
// stay node-scoped. node-2's agent being unreachable says nothing about whether
// node-1's units were observed correctly.
func TestHarvestGate_DegradedSourceOnOtherNodeDoesNotBlock(t *testing.T) {
	g := newHarvestGate(gtSnapshot(true,
		collector.DataError{Service: "node_agent@" + gtOther, RPC: "dial", Err: errors.New("no agent_endpoint")}))

	if ok, why := g.enforceable(gtUnitFinding(gtNode)); !ok {
		t.Errorf("a per-node source failure on %s must not suppress a finding about %s.\ngot: %s",
			gtOther, gtNode, why)
	}
}

// TestHarvestGate_ClusterWideSourceFailureBlocks: a source that failed without a
// node scope failed for everyone, so no finding depending on it is provable.
func TestHarvestGate_ClusterWideSourceFailureBlocks(t *testing.T) {
	g := newHarvestGate(gtSnapshot(true,
		collector.DataError{Service: "node_agent", RPC: "ListNodes", Err: errors.New("unavailable")}))

	if ok, _ := g.enforceable(gtUnitFinding(gtNode)); ok {
		t.Error("a cluster-wide failure of a source the finding depends on must suppress it")
	}
}

// TestHarvestGate_UndeclaredInvariantBlocks is the conservative default.
//
// Undeclared means unknown, and the gate must never permit enforcement on data
// it cannot prove was collected. This is what keeps the loosening bounded: only
// invariants whose sources somebody wrote down and reviewed can benefit.
func TestHarvestGate_UndeclaredInvariantBlocks(t *testing.T) {
	g := newHarvestGate(gtSnapshot(true,
		collector.DataError{Service: "workflow", RPC: "X", Err: errors.New("unavailable")}))

	f := rules.Finding{FindingID: "f-2", InvariantID: "some.undeclared.invariant", EntityRef: gtNode + "/x"}
	ok, why := g.enforceable(f)
	if ok {
		t.Fatal("an invariant with no declared data sources must not be auto-remediated on a " +
			"reduced harvest — unknown provenance is not proof of safety")
	}
	if !strings.Contains(why, "declares no data sources") {
		t.Errorf("the reason must say the declaration is missing, so the fix is obvious, got: %s", why)
	}
}

// TestHarvestGate_NodelessFindingBlocksOnPerNodeFailure: if the finding names no
// node, a per-node failure of a source it depends on cannot be ruled out.
// Refusing beats assuming the healthy nodes were the relevant ones.
func TestHarvestGate_NodelessFindingBlocksOnPerNodeFailure(t *testing.T) {
	g := newHarvestGate(gtSnapshot(true,
		collector.DataError{Service: "node_agent@" + gtOther, RPC: "GetInventory", Err: errors.New("x")}))

	f := rules.Finding{FindingID: "f-3", InvariantID: "node.systemd.units_running", EntityRef: "cluster-wide"}
	if ok, _ := g.enforceable(f); ok {
		t.Error("a finding with no node scope must not be enforced while the source it depends " +
			"on is degraded on some node — the gate cannot tell whether it is the relevant one")
	}
}

// TestHarvestGate_NilSnapshotBlocks: absent data is maximally unknown.
func TestHarvestGate_NilSnapshotBlocks(t *testing.T) {
	g := newHarvestGate(nil)
	if ok, _ := g.enforceable(gtUnitFinding(gtNode)); ok {
		t.Error("no snapshot must suppress enforcement, not permit it")
	}
}

// TestHarvestGate_DeclaredInvariantsAreAutoHealable keeps the table honest.
//
// A declaration for an invariant that can never auto-heal is dead weight that
// reads as coverage; one MISSING for an invariant that can auto-heal silently
// pins it to observe-only forever on any degraded cluster.
func TestHarvestGate_DeclaredInvariantsAreAutoHealable(t *testing.T) {
	for id := range sourcesForInvariant {
		r := rules.LookupPolicy(id)
		if r.Disposition != rules.HealAuto {
			t.Errorf("%s declares data sources but its policy disposition is %q, not %q —\n"+
				"the declaration only matters for findings the healer may act on",
				id, r.Disposition, rules.HealAuto)
		}
	}
	for _, r := range rules.PolicyV1() {
		if r.Disposition != rules.HealAuto || r.AutoAction == "" {
			continue
		}
		if _, ok := sourcesForInvariant[r.InvariantID]; !ok {
			t.Errorf("%s is auto-healable but declares no data sources, so it can never be\n"+
				"enforced on a reduced-harvest snapshot — add it to sourcesForInvariant",
				r.InvariantID)
		}
	}
}
