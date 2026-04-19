package main

import (
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

func TestResolveDNSEndpoint(t *testing.T) {
	got := resolveDNSEndpoint(nil)
	if got == "" || !strings.Contains(got, ":") {
		t.Fatalf("expected valid endpoint with port, got %s", got)
	}
}

func TestNormalizeDomain(t *testing.T) {
	cases := map[string]string{
		"*.Example.COM":      "example.com",
		" example.com. ":     "example.com",
		"":                   "",
		"*.sub.domain.local": "sub.domain.local",
	}
	for in, want := range cases {
		if got := normalizeDomain(in); got != want {
			t.Fatalf("normalizeDomain(%q)=%q want %q", in, got, want)
		}
	}
}

func TestDnsAdminEmail(t *testing.T) {
	spec := &cluster_controllerpb.ClusterNetworkSpec{
		ClusterDomain: "example.com",
	}
	if got := dnsAdminEmail(spec); got != "admin@example.com" {
		t.Fatalf("expected default admin email, got %s", got)
	}
	spec.AdminEmail = "ops@example.com"
	if got := dnsAdminEmail(spec); got != "ops@example.com" {
		t.Fatalf("expected provided email, got %s", got)
	}
}

func TestMakeDNSTokenUsesNodeID(t *testing.T) {
	spec := &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "example.com"}
	tk, err := makeDNSToken("node-123", nil, spec)
	if err != nil {
		// Skip if key generation requires cluster infrastructure.
		msg := err.Error()
		if strings.Contains(msg, "permission denied") ||
			strings.Contains(msg, "no local Globular configuration") ||
			strings.Contains(msg, "get mac address") ||
			strings.Contains(msg, "get address") {
			t.Skipf("cluster config not available, skipping: %v", err)
		}
		t.Fatalf("expected no error: %v", err)
	}
	if strings.TrimSpace(tk) == "" {
		t.Fatalf("expected non-empty token")
	}
}

func TestMakeDNSTokenEmptyNodeIDFails(t *testing.T) {
	spec := &cluster_controllerpb.ClusterNetworkSpec{ClusterDomain: "example.com"}
	_, err := makeDNSToken("", nil, spec)
	if err == nil {
		t.Fatal("expected error when nodeID is empty and client is nil")
	}
	if !strings.Contains(err.Error(), "empty node identity") {
		t.Fatalf("expected 'empty node identity' error, got %q", err.Error())
	}
}

func TestParseIPv4(t *testing.T) {
	if got := parseIPv4(" 1.2.3.4 "); got != "1.2.3.4" {
		t.Fatalf("expected 1.2.3.4, got %q", got)
	}
	if got := parseIPv4("not-an-ip"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	if got := parseIPv4("2001:db8::1"); got != "" {
		t.Fatalf("expected empty for v6, got %q", got)
	}
}

func TestParseIPv6(t *testing.T) {
	if got := parseIPv6(" 2001:db8::1 "); got == "" {
		t.Fatalf("expected non-empty IPv6")
	}
	if got := parseIPv6("1.2.3.4"); got != "" {
		t.Fatalf("expected empty for v4, got %q", got)
	}
	if got := parseIPv6("not-an-ip"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestSelectDNSIPsOverride(t *testing.T) {
	srv := &NodeAgentServer{
		cfg: NodeAgentConfig{
			DNSIPv4: "1.2.3.4",
			DNSIPv6: "2001:db8::1",
		},
	}

	v4, v6 := srv.selectDNSIPs()
	if v4 != "1.2.3.4" {
		t.Fatalf("expected v4 override, got %q", v4)
	}
	if v6 == "" {
		t.Fatalf("expected v6 override, got empty")
	}
}
