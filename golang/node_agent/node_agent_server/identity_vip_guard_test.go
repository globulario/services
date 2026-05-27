package main

import "testing"

// Regression guard: a floating keepalived VIP must never be published
// as stable node identity IP.
func TestExcludeIdentityIP_RemovesVIP(t *testing.T) {
	ips := []string{"10.0.0.100", "10.0.0.63", "192.168.1.10"}
	got := excludeIdentityIP(ips, "10.0.0.100")
	if len(got) != 2 {
		t.Fatalf("expected 2 IPs after VIP removal, got %v", got)
	}
	for _, ip := range got {
		if ip == "10.0.0.100" {
			t.Fatalf("VIP must be excluded from identity IPs: %v", got)
		}
	}
}

func TestExcludeIdentityIP_NoopWhenExcludedEmpty(t *testing.T) {
	ips := []string{"10.0.0.63"}
	got := excludeIdentityIP(ips, "")
	if len(got) != 1 || got[0] != "10.0.0.63" {
		t.Fatalf("unexpected change when excluded is empty: %v", got)
	}
}
