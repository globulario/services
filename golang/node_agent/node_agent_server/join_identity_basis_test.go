package main

import (
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/identity"
)

// The join request must advertise EXACTLY the basis this node derived its own
// id from (invariant identity.has_single_canonical_source_and_is_immutable).
//
// The controller mints AssignedNodeID with deterministicNodeID: FromMAC when
// labels["node.mac"] is present, FromHostAndIPs otherwise. So the label is not
// decoration — it selects which of the two lawful computations the controller
// runs. If this node derived its id from a MAC but omits the label, or derived
// from hostname+IPs but sends a MAC, the two parties compute different ids for
// one machine and both records persist.
//
// That is what happened on 2026-08-17: joinRequestLabels called SelectBestMAC()
// independently of the StableNodeID call that fixed the node's identity, and
// SelectBestMAC is not time-invariant.
func TestJoinRequestLabelsAdvertiseTheIdentityBasis(t *testing.T) {
	srv := &NodeAgentServer{}
	labels := srv.joinRequestLabels()

	basis := identity.IdentityBasisMAC()
	got, present := labels["node.mac"]

	if basis == "" {
		if present {
			t.Fatalf("no MAC basis, so this node's id derives from hostname+IPs, "+
				"but the join advertises node.mac=%q — the controller would derive "+
				"FromMAC and mint a different id than the node uses", got)
		}
		return
	}

	if !present {
		t.Fatalf("node id derives from MAC %q but the join omits node.mac — the "+
			"controller would fall back to FromHostAndIPs and mint a different id", basis)
	}
	if got != basis {
		t.Fatalf("join advertises node.mac=%q but the node derived its id from %q", got, basis)
	}
}

// Operator-supplied labels must survive, and must never be able to forge the
// identity basis.
func TestJoinRequestLabelsKeepConfiguredLabelsAndOwnNodeMAC(t *testing.T) {
	srv := &NodeAgentServer{}
	srv.cfg.Labels = map[string]string{
		"rack":     "r1",
		"node.mac": "00:00:00:00:00:01", // must not win over the real basis
	}
	labels := srv.joinRequestLabels()

	if labels["rack"] != "r1" {
		t.Fatalf("configured label lost: got %q", labels["rack"])
	}
	if basis := identity.IdentityBasisMAC(); basis != "" && labels["node.mac"] != basis {
		t.Fatalf("configured node.mac overrode the derived basis: got %q, want %q",
			labels["node.mac"], basis)
	}
}
