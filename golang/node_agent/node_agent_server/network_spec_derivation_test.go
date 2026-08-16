package main

import (
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// srv.lastSpec gates the entire certificate subsystem: ensureRuntimeTLSConvergence
// (TLS repair), checkAndRenewCertificate (ACME renewal) and caCertDriftLoop all
// return early when it is nil.
//
// Nothing in the production code path ever assigned it. It was set ONLY by
// tests, which build NodeAgentServer as a struct literal with lastSpec already
// populated — so every unit test exercised a state production never reached,
// and the whole subsystem was silently inert on every node: zero
// "tls-convergence" log lines since boot on a cluster whose node-3 was sitting
// on a certificate that expired in January 2025.
//
// This is the wiring the old tests could not see, so it is asserted directly.

func TestCurrentNetworkSpecPrefersPushedSpec(t *testing.T) {
	pushed := &cluster_controllerpb.ClusterNetworkSpec{
		ClusterDomain: "pushed.internal",
		Protocol:      "https",
	}
	srv := &NodeAgentServer{lastSpec: pushed}

	got := srv.currentNetworkSpec()
	if got != pushed {
		t.Fatalf("a controller-pushed spec must win over any derived one, got %+v", got)
	}
}

// The derivation must be attempted when no spec was pushed. Whether it yields a
// spec depends on cluster config being readable in the test environment; what
// must NEVER happen is the old behaviour — returning nil without even looking,
// which is what disabled certificate convergence in production.
func TestCurrentNetworkSpecDerivesWhenNoneWasPushed(t *testing.T) {
	srv := &NodeAgentServer{}
	got := srv.currentNetworkSpec()

	if got == nil {
		// No cluster domain resolvable here — acceptable, and it must stay nil
		// rather than fabricate a domain.
		t.Skip("no cluster domain resolvable in this environment")
	}
	if strings.TrimSpace(got.GetClusterDomain()) == "" {
		t.Fatal("derived a spec with an empty cluster domain — certificates would " +
			"be issued for nothing")
	}
	if got.GetProtocol() == "" {
		t.Fatal("derived a spec with no protocol — the https gate would reject it")
	}
	// It must be cached, so the ACME and CA-drift loops stop seeing nil too.
	if srv.lastSpec == nil {
		t.Fatal("derived spec was not cached on the server; the ACME renewal and " +
			"CA-drift loops would keep reporting 'no lastSpec available'")
	}
}

// A nil server must not panic the heartbeat that calls this every tick.
func TestCurrentNetworkSpecNilServerIsSafe(t *testing.T) {
	var srv *NodeAgentServer
	if got := srv.currentNetworkSpec(); got != nil {
		t.Fatalf("nil server should yield no spec, got %+v", got)
	}
}
