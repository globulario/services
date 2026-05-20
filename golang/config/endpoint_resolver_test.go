package config

import "testing"

func TestResolveDialTarget(t *testing.T) {
	cases := []struct {
		name             string
		in               string
		wantAddr         string
		wantServerName   string
		wantLoopRewrite  bool
	}{
		{"ipv4-loopback", "127.0.0.1:12000", "localhost:12000", "localhost", true},
		{"ipv6-loopback", "[::1]:12000", "localhost:12000", "localhost", true},
		{"localhost", "localhost:12000", "localhost:12000", "localhost", false},
		{"remote-host", "controller.globular.internal:12000", "controller.globular.internal:12000", "controller.globular.internal", false},
		{"bare-host", "controller.globular.internal", "controller.globular.internal", "controller.globular.internal", false},
		{"bare-loopback-ip", "127.0.0.1", "localhost", "localhost", true},
		{"empty", "", "", "", false},
		{"whitespace", "   127.0.0.1:10101  ", "localhost:10101", "localhost", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveDialTarget(tc.in)
			if got.Address != tc.wantAddr {
				t.Errorf("Address: got %q, want %q", got.Address, tc.wantAddr)
			}
			if got.ServerName != tc.wantServerName {
				t.Errorf("ServerName: got %q, want %q", got.ServerName, tc.wantServerName)
			}
			if got.WasLoopbackRewritten != tc.wantLoopRewrite {
				t.Errorf("WasLoopbackRewritten: got %v, want %v", got.WasLoopbackRewritten, tc.wantLoopRewrite)
			}
		})
	}
}

func TestNormalizeLoopback(t *testing.T) {
	// Sanity: the back-compat wrapper must match the resolver.
	cases := map[string]string{
		"127.0.0.1:12000":    "localhost:12000",
		"[::1]:10101":        "localhost:10101",
		"localhost:12000":    "localhost:12000",
		"example.com:443":    "example.com:443",
		"":                   "",
	}
	for in, want := range cases {
		if got := NormalizeLoopback(in); got != want {
			t.Errorf("NormalizeLoopback(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsLoopbackEndpoint(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:10101":      true,
		"[::1]:10101":          true,
		"localhost:10101":      true,
		"localhost":            true,
		"example.com:443":      false,
		"10.0.0.5:12000":       false,
		"":                     false,
	}
	for in, want := range cases {
		if got := IsLoopbackEndpoint(in); got != want {
			t.Errorf("IsLoopbackEndpoint(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestMustResolveDialTarget(t *testing.T) {
	// Valid endpoints succeed.
	dt, err := MustResolveDialTarget("localhost:12000")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if dt.Address != "localhost:12000" {
		t.Errorf("Address = %q, want localhost:12000", dt.Address)
	}

	dt, err = MustResolveDialTarget("10.0.0.63:12000")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if dt.ServerName != "10.0.0.63" {
		t.Errorf("ServerName = %q, want 10.0.0.63", dt.ServerName)
	}

	// Empty endpoint fails.
	_, err = MustResolveDialTarget("")
	if err == nil {
		t.Error("expected error for empty endpoint")
	}

	// Whitespace-only fails.
	_, err = MustResolveDialTarget("   ")
	if err == nil {
		t.Error("expected error for whitespace endpoint")
	}

	// Error is an *EndpointError with explicit message.
	var epErr *EndpointError
	if err != nil {
		var ok bool
		epErr, ok = err.(*EndpointError)
		if !ok {
			t.Fatalf("expected *EndpointError, got %T", err)
		}
		if epErr.Reason == "" {
			t.Error("EndpointError.Reason should not be empty")
		}
	}
}

// TestValidateLANAddress covers every rejection class plus the accept paths.
// Pins the rollout invariant that loopback / docker0 / link-local / multicast
// / unspecified addresses can never be treated as canonical LAN identity,
// while genuine private-LAN and routable public IPs pass through.
func TestValidateLANAddress(t *testing.T) {
	cases := []struct {
		addr    string
		wantErr bool
		why     string
	}{
		// Accept: RFC1918 LAN ranges and routable IPs
		{"10.0.0.63", false, "private LAN /8"},
		{"10.0.0.63:5080", false, "private LAN /8 with port"},
		{"192.168.1.100", false, "private LAN /16"},
		{"172.16.5.10", false, "private LAN /12 — below docker bridge"},
		{"172.31.255.254", false, "private LAN /12 — top of range, not docker"},
		{"203.0.113.5", false, "public IP — allowed (some clusters use them)"},
		{"http://10.0.0.63:5080", false, "URL form parsed as host"},
		// Accept: hostnames (deferred to downstream resolution)
		{"globule-ryzen.globular.internal", false, "bare hostname"},
		{"globule-ryzen.globular.internal:5080", false, "hostname with port"},
		// Accept: empty (separate validation concern)
		{"", false, "empty"},

		// Reject: loopback
		{"127.0.0.1", true, "loopback"},
		{"127.0.0.1:5080", true, "loopback with port"},
		{"::1", true, "ipv6 loopback"},
		{"[::1]:5080", true, "ipv6 loopback with port"},
		{"localhost", true, "localhost literal"},
		{"localhost:5080", true, "localhost literal with port"},
		// Reject: unspecified
		{"0.0.0.0", true, "unspecified v4"},
		{"0.0.0.0:5080", true, "unspecified with port"},
		{"::", true, "unspecified v6"},
		// Reject: link-local
		{"169.254.5.5", true, "link-local v4"},
		{"169.254.169.254:80", true, "link-local cloud metadata"},
		{"fe80::1", true, "link-local v6"},
		// Reject: multicast
		{"224.0.0.1", true, "multicast v4"},
		{"239.255.255.250", true, "ssdp multicast"},
		// Reject: docker default bridge
		{"172.17.0.1", true, "docker0 gateway"},
		{"172.17.0.1:5080", true, "docker0 with port"},
		{"172.17.42.99", true, "anywhere in 172.17/16"},
	}
	for _, tc := range cases {
		err := ValidateLANAddress(tc.addr)
		got := err != nil
		if got != tc.wantErr {
			t.Errorf("ValidateLANAddress(%q) = err? %v, want err? %v (%s)", tc.addr, got, tc.wantErr, tc.why)
		}
		if err != nil {
			if _, ok := err.(*EndpointError); !ok {
				t.Errorf("ValidateLANAddress(%q) returned %T, want *EndpointError", tc.addr, err)
			}
		}
	}
}
