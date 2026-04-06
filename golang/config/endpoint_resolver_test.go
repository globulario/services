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
