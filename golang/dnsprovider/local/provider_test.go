package local

import (
	"net"
	"strconv"
	"testing"

	"github.com/globulario/services/golang/config"
)

// TestNormalizeDNSAddress_ValidExplicit covers inputs that already carry a
// well-formed host:port and must pass through untouched. These are the
// "happy paths" the operator typically configures by hand.
func TestNormalizeDNSAddress_ValidExplicit(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"hostname with port", "globule-nuc:10006"},
		{"ipv4 with port", "10.0.0.8:10006"},
		{"ipv6 with port", "[::1]:10006"},
		{"upper port boundary", "globule-nuc:65535"},
		{"lower port boundary", "globule-nuc:1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, repaired, err := normalizeDNSAddress(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if repaired {
				t.Fatalf("input %q should not have been repaired (got %q)", tc.in, out)
			}
			if out != tc.in {
				t.Fatalf("input %q should pass through unchanged, got %q", tc.in, out)
			}
		})
	}
}

// TestNormalizeDNSAddress_RepairFromEtcd covers inputs whose port is missing
// or out of range. The function must consult service discovery and, when an
// endpoint is available, substitute the canonical port while preserving the
// caller's host. When no canonical endpoint is available (offline test envs)
// the function must surface a clear error instead of returning a broken
// address — that silent half-broken case is what burns the Let's Encrypt
// rate limit in production.
func TestNormalizeDNSAddress_RepairFromEtcd(t *testing.T) {
	canonical := config.ResolveDNSGrpcEndpoint("")
	canonicalPort := ""
	if canonical != "" {
		if _, p, err := net.SplitHostPort(canonical); err == nil {
			canonicalPort = p
		}
	}
	if canonicalPort == "" {
		t.Skip("service discovery has no dns endpoint in this environment — skip repair assertions")
	}

	cases := []struct {
		name     string
		in       string
		wantHost string
	}{
		{"hostname without port", "globule-nuc", "globule-nuc"},
		{"ipv4 without port", "10.0.0.8", "10.0.0.8"},
		{"port out of range high", "10.0.0.8:100006", "10.0.0.8"},
		{"port out of range zero", "10.0.0.8:0", "10.0.0.8"},
		{"port non-numeric", "10.0.0.8:abc", "10.0.0.8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, repaired, err := normalizeDNSAddress(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !repaired {
				t.Fatalf("input %q should have been flagged as repaired (got %q)", tc.in, out)
			}
			gotHost, gotPort, splitErr := net.SplitHostPort(out)
			if splitErr != nil {
				t.Fatalf("repaired address %q is not a valid host:port: %v", out, splitErr)
			}
			if gotHost != tc.wantHost {
				t.Fatalf("repaired address %q dropped the host: want host=%q got %q", out, tc.wantHost, gotHost)
			}
			n, convErr := strconv.Atoi(gotPort)
			if convErr != nil || n <= 0 || n > 65535 {
				t.Fatalf("repaired port %q must be a valid TCP port (1..65535)", gotPort)
			}
		})
	}
}

// TestNormalizeDNSAddress_EmptyInputFallsBackToCanonical confirms that an
// empty credentials.address yields the full canonical endpoint from etcd,
// not an error — operators who omit the field rely on this default.
func TestNormalizeDNSAddress_EmptyInputFallsBackToCanonical(t *testing.T) {
	canonical := config.ResolveDNSGrpcEndpoint("")
	if canonical == "" {
		t.Skip("service discovery has no dns endpoint in this environment")
	}
	out, repaired, err := normalizeDNSAddress("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repaired {
		t.Fatalf("empty input should be flagged as repaired")
	}
	if out != canonical {
		t.Fatalf("empty input should return canonical %q, got %q", canonical, out)
	}
}
