package programming

import "testing"

// TestNewLoadsAndValidates asserts the embedded seed parses, every principle ref
// resolves within the pack's catalogs, and the generative-pairing rule holds.
func TestNewLoadsAndValidates(t *testing.T) {
	p, err := New()
	if err != nil {
		t.Fatalf("New() failed to load/validate seed: %v", err)
	}
	if p.Name() != DomainName {
		t.Fatalf("Name() = %q, want %q", p.Name(), DomainName)
	}
	c := p.Catalogs()
	if len(c.Authorities) == 0 || len(c.Conditions) == 0 ||
		len(c.ForbiddenMoves) == 0 || len(c.RequiredEvidence) == 0 || len(c.Principles) == 0 {
		t.Fatalf("empty catalog: auth=%d cond=%d forb=%d ev=%d prin=%d",
			len(c.Authorities), len(c.Conditions), len(c.ForbiddenMoves),
			len(c.RequiredEvidence), len(c.Principles))
	}
	// Multi-domain addition: authority entries carry a comparable rank within the
	// programming lattice.
	for _, a := range c.Authorities {
		if a.Fields["lattice"] != "programming" || a.Fields["rank"] == "" {
			t.Errorf("authority %q missing lattice/rank fields: %v", a.ID, a.Fields)
		}
	}
}
