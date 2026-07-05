package nodeid

import "testing"

// TestFromMAC_Golden pins the canonical MAC derivation. These bytes are baked
// into every /globular/nodes/{id} etcd key and every signed JoinPlan
// AssignedNodeID — if this golden changes, the Namespace or grammar changed and
// every existing node id is orphaned. This is the byte-for-byte value the
// controller (deterministicNodeID) and node agent (StableNodeID) produced before
// the derivation was centralized here, so routing peers.go through FromMAC keeps
// the resource store consistent with the cluster identity.
func TestFromMAC_Golden(t *testing.T) {
	got := FromMAC("e0:d4:64:f0:86:f6")
	const want = "eb9a2dac-05b0-52ac-9002-99d8ffd35902"
	if got != want {
		t.Fatalf("FromMAC drifted: got %q, want %q — a change here orphans every node id", got, want)
	}
	// Must be UUID v5 (SHA1). The old resource/peers.go path used v3/MD5, which is
	// exactly the divergence this package removes.
	if got[14] != '5' {
		t.Errorf("canonical node id must be UUID v5, got version nibble %q in %s", string(got[14]), got)
	}
}

// TestFromHostAndIPs_GoldenAndOrderStable pins the host fallback and proves the
// id is independent of IP input order (IPs are sorted before hashing).
func TestFromHostAndIPs_GoldenAndOrderStable(t *testing.T) {
	const want = "bf1e2b00-d95d-559b-899f-68f449a2352b"
	a := FromHostAndIPs("globule-ryzen", []string{"10.0.0.63", "10.0.0.1"})
	b := FromHostAndIPs("globule-ryzen", []string{"10.0.0.1", "10.0.0.63"})
	if a != want {
		t.Fatalf("FromHostAndIPs drifted: got %q, want %q", a, want)
	}
	if a != b {
		t.Errorf("host fallback must be order-independent: %q != %q", a, b)
	}
}

// TestFromMAC_DoesNotMutateAndIsDeterministic sanity-checks purity.
func TestFromMAC_Deterministic(t *testing.T) {
	if FromMAC("aa:bb:cc:dd:ee:ff") != FromMAC("aa:bb:cc:dd:ee:ff") {
		t.Error("FromMAC must be deterministic")
	}
	if FromMAC("aa:bb:cc:dd:ee:ff") == FromMAC("00:11:22:33:44:55") {
		t.Error("different MACs must yield different ids")
	}
}

// TestFromHostAndIPs_DoesNotMutateInput guards the defensive copy.
func TestFromHostAndIPs_DoesNotMutateInput(t *testing.T) {
	ips := []string{"10.0.0.63", "10.0.0.1"}
	_ = FromHostAndIPs("h", ips)
	if ips[0] != "10.0.0.63" || ips[1] != "10.0.0.1" {
		t.Errorf("caller slice was mutated: %v", ips)
	}
}
