package identity

import (
	"testing"

	"github.com/globulario/services/golang/nodeid"
)

// A node must map to exactly one node_id everywhere
// (invariant identity.has_single_canonical_source_and_is_immutable).
//
// node_id has two lawful computations — nodeid.FromMAC, and nodeid.FromHostAndIPs
// when no MAC exists — and two parties compute it: this node, and the cluster
// controller from the join label "node.mac". They diverged because SelectBestMAC
// was asked separately by each consumer and is not time-invariant: it needs an
// interface UP with an IPv4 assigned, so it fails early in boot and succeeds
// later. One consumer got a MAC and the other did not, and the node ended up
// with two identities at once — 7 etcd node records for 5 nodes on 2026-08-17,
// with a rejoined node heartbeating under one id while its state.json carried
// the other.
//
// These tests pin the property that makes both parties agree: the basis is
// decided ONCE per process, and everything downstream is a function of it.

func TestIdentityBasisMACIsMemoized(t *testing.T) {
	first := IdentityBasisMAC()
	for i := 0; i < 50; i++ {
		if got := IdentityBasisMAC(); got != first {
			t.Fatalf("IdentityBasisMAC() is not stable: call 0 = %q, call %d = %q. "+
				"A basis that changes mid-process is exactly how one machine gets two node_ids.",
				first, i+1, got)
		}
	}
}

func TestStableNodeIDIsStableAcrossCalls(t *testing.T) {
	first, err := StableNodeID()
	if err != nil {
		t.Skipf("no usable identity on this host: %v", err)
	}
	for i := 0; i < 50; i++ {
		got, err := StableNodeID()
		if err != nil {
			t.Fatalf("StableNodeID() failed on call %d after succeeding: %v", i+1, err)
		}
		if got != first {
			t.Fatalf("StableNodeID() is not stable: call 0 = %q, call %d = %q", first, i+1, got)
		}
	}
}

// The id must be a pure function of the basis, so the controller — which sees
// only the basis, via the node.mac label — computes the same value.
func TestStableNodeIDDerivesFromTheAdvertisedBasis(t *testing.T) {
	id, err := StableNodeID()
	if err != nil {
		t.Skipf("no usable identity on this host: %v", err)
	}

	mac := IdentityBasisMAC()
	if mac != "" {
		want := nodeid.FromMAC(mac)
		if id != want {
			t.Fatalf("basis is MAC %q so the id must be FromMAC(basis)=%q, got %q. "+
				"The controller derives FromMAC(labels[node.mac]); any other value "+
				"means the two parties disagree.", mac, want, id)
		}
		return
	}

	// No MAC: the node must land on the hostname+IPs computation — the same one
	// the controller falls back to when the join carries no node.mac label.
	hostname := ""
	if h, herr := hostnameSafe(); herr == nil {
		hostname = h
	}
	ips, _ := gatherNonLoopbackIPs()
	want := nodeid.FromHostAndIPs(hostname, ips)
	if id != want {
		t.Fatalf("basis is empty so the id must be FromHostAndIPs(...)=%q, got %q", want, id)
	}
}
