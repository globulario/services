package main

import (
	"testing"
	"time"
)

// TestJoinReusesAnExistingIdentity pins that a machine already in the cluster is
// never admitted a second time under a differently-derived id.
//
// deterministicNodeID prefers labels["node.mac"] and falls back to hostname+IPs.
// Neither basis is stable across a process restart: IdentityBasisMAC requires an
// interface that is up WITH a routable IP, so an agent restarting before its
// interface is ready advertises no MAC and derives the hostname id, and after it
// is ready advertises one and derives a different id.
//
// Observed on the 5-node sim: node-2 held canonical c8a09d9e (hostname+IPs) and
// was admitted again as b68457f5 — exactly nodeid.FromMAC of its own container
// eth0 MAC 02:42:0a:0a:00:0c. node-5 likewise as 2da500c8 from :0f. Each phantom
// carried per-node records, cluster-doctor reported CRITICAL on a healthy
// cluster, and every scenario asserting zero doctor errors failed.
func TestJoinReusesAnExistingIdentity(t *testing.T) {
	const canonical = "c8a09d9e-3813-5357-ab58-93aa410f27fb"
	const phantom = "b68457f5-bfb6-5452-bccc-cc36f29d1bbc"

	srv := newTestServerWithNodes(&nodeState{
		NodeID:     canonical,
		Identity:   storedIdentity{Hostname: "node-2", Ips: []string{"10.10.0.12"}},
		Status:     "ready",
		LastSeen:   time.Now(),
		ReportedAt: time.Now(),
	})

	t.Run("same machine under a different basis reuses the assigned id", func(t *testing.T) {
		got := srv.existingNodeIDForIdentityLocked(
			storedIdentity{Hostname: "node-2", Ips: []string{"10.10.0.12"}}, phantom)
		if got != canonical {
			t.Fatalf("a machine already in the cluster was about to be admitted twice: "+
				"derived %s, expected reuse of %s, got %q", phantom, canonical, got)
		}
	})

	t.Run("the id it already holds is not reconciled against itself", func(t *testing.T) {
		if got := srv.existingNodeIDForIdentityLocked(
			storedIdentity{Hostname: "node-2", Ips: []string{"10.10.0.12"}}, canonical); got != "" {
			t.Errorf("deriving the id the node already holds needs no reuse; got %q", got)
		}
	})

	t.Run("a genuinely different machine is never merged", func(t *testing.T) {
		// Same subnet, different host and IP — must NOT collapse onto node-2.
		if got := srv.existingNodeIDForIdentityLocked(
			storedIdentity{Hostname: "node-3", Ips: []string{"10.10.0.13"}}, "fresh-id"); got != "" {
			t.Errorf("merging two different machines onto one id is worse than a "+
				"duplicate; got %q", got)
		}
	})

	t.Run("hostname match alone is not enough", func(t *testing.T) {
		// Hostname reuse after a re-image, with a different address, must not
		// silently inherit the old machine's identity.
		if got := srv.existingNodeIDForIdentityLocked(
			storedIdentity{Hostname: "node-2", Ips: []string{"10.99.0.99"}}, "fresh-id"); got != "" {
			t.Errorf("hostname without a shared IP is weak evidence; got %q", got)
		}
	})

	t.Run("an incomplete basis says nothing", func(t *testing.T) {
		for _, id := range []storedIdentity{
			{Hostname: "", Ips: []string{"10.10.0.12"}},
			{Hostname: "node-2", Ips: nil},
		} {
			if got := srv.existingNodeIDForIdentityLocked(id, "fresh-id"); got != "" {
				t.Errorf("a partial basis must not be used to claim an existing identity; got %q", got)
			}
		}
	})
}
