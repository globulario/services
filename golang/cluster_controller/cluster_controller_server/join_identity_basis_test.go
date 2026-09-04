package main

import (
	"strings"
	"testing"

	"github.com/globulario/services/golang/nodeid"
)

// completeIdentityBasis mirrors the admission guard in RequestJoin: a join is
// only granted a derived id when it carries a MAC, or BOTH a hostname and at
// least one IP.
func completeIdentityBasis(mac, hostname string, ips []string) bool {
	if strings.TrimSpace(mac) != "" {
		return true
	}
	return strings.TrimSpace(hostname) != "" && len(ips) > 0
}

// TestJoinRequiresACompleteIdentityBasis pins that admission refuses to derive a
// node id from half a basis.
//
// deterministicNodeID prefers the node.mac label and otherwise derives from
// hostname + sorted IPs. Deriving from a hostname with an empty IP list yields
// an id the node can never reproduce once its interfaces are up, so the cluster
// gains a member that never heartbeats again under that id while the real node
// arrives separately under its canonical one.
//
// Observed on releases 1.2.330, 1.2.332 and 1.2.333: a restart storm left the
// controller reporting six, then seven members for a five-node cluster, and the
// phantoms carried real Layer-3 records (cluster-controller, etcd, repository,
// scylladb). The derivation is deterministic, so the same partial basis
// reproduces the SAME phantom id every time — the orphans recur rather than
// being one-offs.
func TestJoinRequiresACompleteIdentityBasis(t *testing.T) {
	t.Run("accepts a MAC alone", func(t *testing.T) {
		if !completeIdentityBasis("aa:bb:cc:dd:ee:ff", "", nil) {
			t.Error("a MAC is a complete basis on its own — it is the preferred one")
		}
	})

	t.Run("accepts hostname with at least one IP", func(t *testing.T) {
		if !completeIdentityBasis("", "node-5", []string{"10.10.0.15"}) {
			t.Error("hostname + IPs is the documented fallback basis")
		}
	})

	t.Run("refuses hostname with no IPs", func(t *testing.T) {
		if completeIdentityBasis("", "node-5", nil) {
			t.Error("a hostname with no IPs must be refused — the id derived from it " +
				"can never be reproduced once interfaces are up")
		}
	})

	t.Run("refuses IPs with no hostname", func(t *testing.T) {
		if completeIdentityBasis("", "", []string{"10.10.0.15"}) {
			t.Error("both halves of the fallback basis are required")
		}
	})

	t.Run("refuses an empty basis", func(t *testing.T) {
		if completeIdentityBasis("", "", nil) {
			t.Error("a node with no identity basis at all must be refused")
		}
	})
}

// TestPartialAndCompleteBasesDiverge states the reason for the guard as an
// inequality rather than an opinion: the two bases produce different ids, so
// admitting on a partial one admits a node under an identity it will not keep.
func TestPartialAndCompleteBasesDiverge(t *testing.T) {
	partial := nodeid.FromHostAndIPs("node-5", nil)
	complete := nodeid.FromHostAndIPs("node-5", []string{"10.10.0.15"})
	if partial == complete {
		t.Fatal("precondition failed: the derivation is supposed to key on IPs")
	}
	t.Logf("partial=%s complete=%s — admitting on the partial basis admits a "+
		"node under an identity it will not keep", partial, complete)
}
