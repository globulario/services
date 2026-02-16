package netutil

import "testing"

func TestDefaultClusterDomain(t *testing.T) {
	if got := DefaultClusterDomain(); got != "globular.internal" {
		t.Fatalf("DefaultClusterDomain = %s, want globular.internal", got)
	}
}

func TestNormalizeDomain(t *testing.T) {
	if got := NormalizeDomain(" Example.COM. "); got != "example.com" {
		t.Fatalf("NormalizeDomain returned %s", got)
	}
}

func TestValidateClusterDomain(t *testing.T) {
	cases := []struct {
		domain string
		ok     bool
	}{
		{"globular.internal", true},
		{"localhost", false},
		{"127.0.0.1", false},
		{"", false},
	}
	for _, c := range cases {
		err := ValidateClusterDomain(c.domain)
		if c.ok && err != nil {
			t.Fatalf("ValidateClusterDomain(%s) unexpected error: %v", c.domain, err)
		}
		if !c.ok && err == nil {
			t.Fatalf("ValidateClusterDomain(%s) expected error", c.domain)
		}
	}
}

func TestResolveAdvertiseIPExplicit(t *testing.T) {
	ip, err := ResolveAdvertiseIP("", "10.1.2.3")
	if err != nil {
		t.Fatalf("ResolveAdvertiseIP explicit failed: %v", err)
	}
	if ip.String() != "10.1.2.3" {
		t.Fatalf("ResolveAdvertiseIP got %s", ip.String())
	}
	if _, err := ResolveAdvertiseIP("", "127.0.0.1"); err == nil {
		t.Fatalf("expected loopback explicit to fail")
	}
}
