package identity

import (
	"testing"

	"github.com/globulario/services/golang/nodeid"
)

// TestPartialBasisYieldsADifferentIdentity demonstrates WHY StableNodeID must
// refuse a partial hostname+IPs basis, rather than asserting on the guard's
// wording.
//
// FromHostAndIPs keys on hostname + sorted IPs. An id derived before the
// interfaces are up therefore cannot equal the id the same node derives a
// moment later — it is not a weaker identity, it is a different one. The old
// guard was `hostname == "" && len(ips) == 0`, which let exactly that happen.
//
// Cost, observed across releases 1.2.330 and 1.2.332: during a restart storm a
// node registered under such an id, the controller reported SIX members for a
// five-node cluster, and the phantom wrote real Layer-3 records —
// /globular/nodes/{phantom}/ held cluster-controller, etcd, repository and
// scylladb entries. The phantom id DIFFERED between occurrences
// (b68457f5-bfb6-5452-bccc-cc36f29d1bbc, then
// 12944a1b-cfae-5d2f-8056-e8f633c8d3dd) precisely because the IP set differed,
// so each event minted a new orphan rather than reusing one.
//
// intent:node_identity.hostname_ip_for_membership_domain_mac_for_other_axes —
// identity for membership is hostname AND IPs.
func TestPartialBasisYieldsADifferentIdentity(t *testing.T) {
	const host = "node-5"

	withoutIPs := nodeid.FromHostAndIPs(host, nil)
	withIPs := nodeid.FromHostAndIPs(host, []string{"10.10.0.15"})

	if withoutIPs == withIPs {
		t.Fatal("precondition failed: the derivation is supposed to key on IPs")
	}

	// This inequality IS the defect: a node that derives before its interfaces
	// are up can never converge on the id it will derive afterwards. Deriving
	// from a partial basis must therefore be refused, not merely discouraged.
	t.Logf("partial basis %s != complete basis %s — deriving early mints a "+
		"permanently divergent identity", withoutIPs, withIPs)

	// And the complete basis is order-independent, so a node with several
	// addresses still lands on one id regardless of enumeration order.
	a := nodeid.FromHostAndIPs(host, []string{"10.10.0.15", "192.168.1.5"})
	b := nodeid.FromHostAndIPs(host, []string{"192.168.1.5", "10.10.0.15"})
	if a != b {
		t.Errorf("a complete basis must be order-independent: %s != %s", a, b)
	}
}

// StableNodeID must surface a usable reason when it refuses, so an operator
// reading the log can tell "waiting for interfaces" from "misconfigured".
//
// This runs against the real host, so it asserts only on the shape of a
// refusal: whichever branch is taken must never return an empty id with a nil
// error — that combination is what let callers adopt a blank identity.
func TestStableNodeIDNeverReturnsAnEmptyIDWithoutError(t *testing.T) {
	id, err := StableNodeID()
	if err == nil && id == "" {
		t.Fatal("StableNodeID returned an empty id with no error — a caller " +
			"checking only err would adopt a blank identity")
	}
	if err != nil && id != "" {
		t.Errorf("a refusal must not also hand back an id, got %q with err %v", id, err)
	}
}
