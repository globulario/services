package main

import (
	"os"
	"strings"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
)

func TestResolveDNSEndpoint(t *testing.T) {
	os.Unsetenv("GLOBULAR_DNS_ENDPOINT")
	if got := resolveDNSEndpoint(nil); got != defaultDnsEndpoint {
		t.Fatalf("expected default endpoint, got %s", got)
	}
	os.Setenv("GLOBULAR_DNS_ENDPOINT", "1.2.3.4:1234")
	defer os.Unsetenv("GLOBULAR_DNS_ENDPOINT")
	if got := resolveDNSEndpoint(nil); got != "1.2.3.4:1234" {
		t.Fatalf("expected env endpoint, got %s", got)
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
	spec := &clustercontrollerpb.ClusterNetworkSpec{
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
	spec := &clustercontrollerpb.ClusterNetworkSpec{ClusterDomain: "example.com"}
	os.Unsetenv("GLOBULAR_DNS_TOKEN")
	tkOverride := "test-token-nodeid"
	os.Setenv("GLOBULAR_DNS_TOKEN", tkOverride)
	defer os.Unsetenv("GLOBULAR_DNS_TOKEN")
	tk, err := makeDNSToken("node-123", nil, spec)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if strings.TrimSpace(tk) != tkOverride {
		t.Fatalf("expected override token, got %q", tk)
	}
}

func TestMakeDNSTokenUsesEnvFallback(t *testing.T) {
	spec := &clustercontrollerpb.ClusterNetworkSpec{ClusterDomain: "example.com"}
	os.Unsetenv("GLOBULAR_DNS_TOKEN")
	os.Setenv("GLOBULAR_DNS_TOKEN", "test-token-env")
	defer os.Unsetenv("GLOBULAR_DNS_TOKEN")
	tk, err := makeDNSToken("", nil, spec)
	if err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	if strings.TrimSpace(tk) != "test-token-env" {
		t.Fatalf("expected override token from env, got %q", tk)
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

func TestSelectDNSIPsEnvOverride(t *testing.T) {
	os.Setenv(envDNSIPv4, "1.2.3.4")
	os.Setenv(envDNSIPv6, "2001:db8::1")
	os.Unsetenv(envDNSIface)
	defer os.Unsetenv(envDNSIPv4)
	defer os.Unsetenv(envDNSIPv6)

	v4, v6 := selectDNSIPs()
	if v4 != "1.2.3.4" {
		t.Fatalf("expected v4 override, got %q", v4)
	}
	if v6 == "" {
		t.Fatalf("expected v6 override, got empty")
	}
}
