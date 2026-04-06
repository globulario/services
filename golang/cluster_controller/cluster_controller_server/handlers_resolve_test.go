package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_controller/cluster_controller_server/projections"
)

// These tests cover the pure-function parts of the NodeIdentity projection's
// resolve path — the classifier, the deterministic mapping from nodeState
// to NodeIdentity (Clause 10), the fallback predicate, and the internal →
// proto translator. They run without scylla or a live server, so they lock
// in the contract even when the full test suite is not exercised.

func TestDetectIdentifierKind(t *testing.T) {
	cases := []struct {
		in   string
		want identifierKind
	}{
		{"eb9a2dac-05b0-52ac-9002-99d8ffd35902", kindNodeID},
		{"EB9A2DAC-05B0-52AC-9002-99D8FFD35902", kindNodeID}, // upper is valid
		{"e0:d4:64:f0:86:f6", kindMAC},
		{"E0:D4:64:F0:86:F6", kindMAC},
		{"10.0.0.63", kindIP},
		{"::1", kindIP},
		{"2001:db8::1", kindIP},
		{"globule-ryzen", kindHostname},
		{"node.example.com", kindHostname},
		// MAC-shaped but invalid colons count is rejected.
		{"e0:d4:64:f0:86", kindHostname},
	}
	for _, tc := range cases {
		if got := detectIdentifierKind(tc.in); got != tc.want {
			t.Errorf("detectIdentifierKind(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// TestNodeToIdentityDeterministic enforces Clause 10: the projector path
// and the fallback path MUST produce byte-identical NodeIdentity rows for
// the same nodeState. We call the mapping twice and compare; any added
// randomness or hidden time-dependency will be caught here.
func TestNodeToIdentityDeterministic(t *testing.T) {
	ns := &nodeState{
		NodeID: "eb9a2dac-05b0-52ac-9002-99d8ffd35902",
		Identity: storedIdentity{
			Hostname: "globule-ryzen",
			Ips:      []string{"10.0.0.63", ""}, // empty must be dropped
		},
		Metadata: map[string]string{"node.mac": "e0:d4:64:f0:86:f6"},
		Profiles: []string{"control-plane", "core", "gateway"},
		LastSeen: time.Unix(1712345678, 0),
	}

	a := nodeToIdentity(ns)
	b := nodeToIdentity(ns)

	if !reflect.DeepEqual(a, b) {
		t.Fatalf("nodeToIdentity non-deterministic: %#v vs %#v", a, b)
	}

	want := &projections.NodeIdentity{
		NodeID:     "eb9a2dac-05b0-52ac-9002-99d8ffd35902",
		Hostname:   "globule-ryzen",
		IPs:        []string{"10.0.0.63"},
		MACs:       []string{"e0:d4:64:f0:86:f6"},
		Labels:     []string{"control-plane", "core", "gateway"},
		ObservedAt: 1712345678,
	}
	if !reflect.DeepEqual(a, want) {
		t.Errorf("nodeToIdentity mapping wrong:\ngot:  %#v\nwant: %#v", a, want)
	}
}

// TestNodeToIdentityObservedAtFallback confirms the observed_at
// timestamp resolution order: LastSeen → ReportedAt → now(). Callers
// rely on observed_at being non-zero (per Clause 4: freshness is
// mandatory), so the mapping must never emit zero.
func TestNodeToIdentityObservedAtFallback(t *testing.T) {
	// Only ReportedAt set.
	ns1 := &nodeState{
		NodeID:     "n1",
		ReportedAt: time.Unix(1712000000, 0),
	}
	if got := nodeToIdentity(ns1).ObservedAt; got != 1712000000 {
		t.Errorf("ReportedAt fallback: got %d, want 1712000000", got)
	}
	// Neither set → must default to now (non-zero).
	ns2 := &nodeState{NodeID: "n2"}
	if got := nodeToIdentity(ns2).ObservedAt; got == 0 {
		t.Error("observed_at must never be zero (Clause 4: freshness is mandatory)")
	}
}

func TestNodeMatchesIdentifier(t *testing.T) {
	ns := &nodeState{
		NodeID:   "eb9a2dac-05b0-52ac-9002-99d8ffd35902",
		Identity: storedIdentity{Hostname: "globule-ryzen", Ips: []string{"10.0.0.63", "10.0.0.64"}},
		Metadata: map[string]string{"node.mac": "e0:d4:64:f0:86:f6"},
	}
	cases := []struct {
		name  string
		ident string
		kind  identifierKind
		want  bool
	}{
		{"nodeid-hit", "eb9a2dac-05b0-52ac-9002-99d8ffd35902", kindNodeID, true},
		{"nodeid-miss", "other", kindNodeID, false},
		{"hostname-hit", "globule-ryzen", kindHostname, true},
		{"hostname-miss", "other-host", kindHostname, false},
		// MAC matching is case-insensitive by contract.
		{"mac-hit-lower", "e0:d4:64:f0:86:f6", kindMAC, true},
		{"mac-hit-upper", "E0:D4:64:F0:86:F6", kindMAC, true},
		{"mac-miss", "aa:bb:cc:dd:ee:ff", kindMAC, false},
		{"ip-hit-primary", "10.0.0.63", kindIP, true},
		{"ip-hit-secondary", "10.0.0.64", kindIP, true},
		{"ip-miss", "10.0.0.99", kindIP, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nodeMatchesIdentifier(ns, tc.ident, tc.kind); got != tc.want {
				t.Errorf("nodeMatchesIdentifier(%q, %v) = %v, want %v", tc.ident, tc.kind, got, tc.want)
			}
		})
	}
}

// TestToProtoIdentitySource pins the contract from Clause 4: the source
// field is set by the caller (handler picks it), NEVER inferred from the
// data. This test catches a future refactor that tries to compute source
// inside the translator.
func TestToProtoIdentitySource(t *testing.T) {
	id := &projections.NodeIdentity{
		NodeID:     "n",
		Hostname:   "h",
		IPs:        []string{"1.2.3.4"},
		MACs:       []string{"aa:bb:cc:dd:ee:ff"},
		Labels:     []string{"gateway"},
		ObservedAt: 42,
	}
	// Same projection row, different sources, must round-trip.
	for _, src := range []string{"scylla", "cluster-controller", "node-agent"} {
		got := toProtoIdentity(id, src)
		if got.GetSource() != src {
			t.Errorf("Source lost: got %q, want %q", got.GetSource(), src)
		}
		if got.GetNodeId() != "n" || got.GetObservedAt() != 42 {
			t.Errorf("translator dropped fields: %#v", got)
		}
	}
}

// TestResolveNodeFallback exercises the handler's fallback path
// (projection nil → scan in-memory state) using a real *server struct
// with a populated state. No scylla, no RPC plumbing.
func TestResolveNodeFallback(t *testing.T) {
	srv := newTestServerWithNodes(
		&nodeState{
			NodeID:   "eb9a2dac-05b0-52ac-9002-99d8ffd35902",
			Identity: storedIdentity{Hostname: "globule-ryzen", Ips: []string{"10.0.0.63"}},
			Metadata: map[string]string{"node.mac": "e0:d4:64:f0:86:f6"},
			Profiles: []string{"gateway"},
			LastSeen: time.Unix(1712345678, 0),
		},
	)

	cases := []string{
		"eb9a2dac-05b0-52ac-9002-99d8ffd35902", // node_id
		"globule-ryzen",                        // hostname
		"10.0.0.63",                            // ip
		"e0:d4:64:f0:86:f6",                    // mac
	}
	for _, ident := range cases {
		t.Run(ident, func(t *testing.T) {
			kind := detectIdentifierKind(ident)
			got := srv.resolveFromState(ident, kind)
			if got == nil {
				t.Fatalf("resolveFromState(%q) = nil, want match", ident)
			}
			if got.NodeID != "eb9a2dac-05b0-52ac-9002-99d8ffd35902" {
				t.Errorf("wrong node: got %s", got.NodeID)
			}
		})
	}

	// Negative case: identifier that doesn't match any node.
	if got := srv.resolveFromState("no-such-host", kindHostname); got != nil {
		t.Errorf("resolveFromState(no-such-host) = %#v, want nil", got)
	}
}

// newTestServerWithNodes builds a minimal *server sufficient for the
// resolve-from-state path: just the state + lock. Avoids pulling in
// etcd / controller / projector dependencies.
func newTestServerWithNodes(nodes ...*nodeState) *server {
	s := &server{
		state: &controllerState{Nodes: map[string]*nodeState{}},
	}
	for _, n := range nodes {
		s.state.Nodes[n.NodeID] = n
	}
	return s
}
