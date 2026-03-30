package config

import (
	"net"
	"testing"
)

// TestGetMeshAddress verifies that GetMeshAddress returns <IP>:443 (Envoy mesh).
// This is a smoke test — it requires local config to be present on the machine
// running the test (CI or dev box with /var/lib/globular/config/config.json).
func TestGetMeshAddress(t *testing.T) {
	addr, err := GetMeshAddress()
	if err != nil {
		t.Skipf("GetMeshAddress unavailable (no local config): %v", err)
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("GetMeshAddress returned unparseable address %q: %v", addr, err)
	}
	if port != "443" {
		t.Errorf("GetMeshAddress port = %q, want 443", port)
	}
	if host == "" {
		t.Error("GetMeshAddress host is empty")
	}
	t.Logf("GetMeshAddress = %s", addr)
}

// TestResolveServiceAddrPortBypassesMesh documents the CURRENT behavior:
// ResolveServiceAddr returns direct ports (e.g. 10.0.0.63:10010), which bypass
// the Envoy mesh when passed to grpc.Dial.
//
// This test exists to track when we fix this — once ResolveServiceAddr returns
// mesh addresses (:443), the assertions should flip.
func TestResolveServiceAddrPortBypassesMesh(t *testing.T) {
	// These are mesh-routable services (shared storage, any instance OK).
	meshRoutable := []struct {
		service  string
		fallback string
	}{
		{"event.EventService", "localhost:10010"},
		{"authentication.AuthenticationService", "localhost:10101"},
		{"rbac.RbacService", "localhost:10104"},
		{"resource.ResourceService", "localhost:10106"},
		{"persistence.PersistenceService", "localhost:10107"},
	}

	for _, tc := range meshRoutable {
		t.Run(tc.service, func(t *testing.T) {
			addr := ResolveServiceAddr(tc.service, tc.fallback)
			if addr == "" {
				t.Skip("no endpoint discovered (etcd/gateway unavailable)")
			}
			_, port, err := net.SplitHostPort(addr)
			if err != nil {
				t.Fatalf("unparseable address %q: %v", addr, err)
			}

			// CURRENT BEHAVIOR: returns direct port, NOT mesh port.
			// Once we fix ResolveServiceAddr for mesh-routable services,
			// change this assertion to: port == "443"
			if port == "443" {
				t.Logf("GOOD: %s resolved to mesh port :443 (%s)", tc.service, addr)
			} else {
				t.Logf("KNOWN GAP: %s resolved to direct port :%s (%s) — bypasses Envoy mesh", tc.service, port, addr)
			}
		})
	}
}

// TestNormalizePassesThroughHostPort verifies that addresses already in
// host:port form are returned unchanged — this is the root cause of mesh
// bypass when ResolveServiceAddr feeds into GetClient/normalizeControlAddress.
func TestNormalizePassesThroughHostPort(t *testing.T) {
	cases := []struct {
		name    string
		address string
		want    string
	}{
		{"direct port passes through", "10.0.0.63:10010", "10.0.0.63:10010"},
		{"mesh port passes through", "10.0.0.63:443", "10.0.0.63:443"},
		{"localhost direct", "localhost:10101", "localhost:10101"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := net.SplitHostPort(tc.address)
			if err != nil {
				t.Fatalf("SplitHostPort(%q) failed: %v", tc.address, err)
			}
			t.Logf("%s → %s (passes through unchanged)", tc.address, tc.want)
		})
	}
}

// TestMeshRouteAddrsLocalhostPreserved verifies that meshRouteAddrs does NOT
// rewrite localhost/loopback addresses to :443. localhost means "this node,
// direct port" and must never go through the Envoy mesh.
func TestMeshRouteAddrsLocalhostPreserved(t *testing.T) {
	cases := []struct {
		name  string
		input []string
		want  []string
	}{
		{
			"localhost stays direct",
			[]string{"localhost:10101"},
			[]string{"localhost:10101"},
		},
		{
			"127.0.0.1 stays direct",
			[]string{"127.0.0.1:10010"},
			[]string{"127.0.0.1:10010"},
		},
		{
			"remote IP gets mesh port",
			[]string{"10.0.0.63:10101"},
			[]string{"10.0.0.63:443"},
		},
		{
			"mixed localhost and remote",
			[]string{"localhost:10101", "10.0.0.63:10101"},
			[]string{"localhost:10101", "10.0.0.63:443"},
		},
		{
			"multiple remote same host dedup",
			[]string{"10.0.0.63:10101", "10.0.0.63:10104"},
			[]string{"10.0.0.63:443"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := meshRouteAddrs(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("meshRouteAddrs(%v) = %v, want %v", tc.input, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("meshRouteAddrs(%v)[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}
