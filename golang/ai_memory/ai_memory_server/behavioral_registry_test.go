package main

import "testing"

// TestEveryShippedDomainPackIsRegistered guards the failure this test was
// written for: the programming pack shipped a full seed catalog and was never
// registered here, so its domain answered every catalog query as empty and a
// principle citing a real authority id was rejected as unresolved. A pack that
// exists but is not registered is invisible rather than broken, which is why
// nobody noticed.
func TestEveryShippedDomainPackIsRegistered(t *testing.T) {
	reg := behavioralRegistry()
	for _, name := range []string{"cluster_operator", "programming"} {
		if _, ok := reg.Lookup(name); !ok {
			t.Fatalf("domain pack %q is not registered, so its catalogs are empty at runtime", name)
		}
	}
}
