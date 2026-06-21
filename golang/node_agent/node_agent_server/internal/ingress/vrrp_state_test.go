package ingress

import "testing"

// deriveVRRPState must treat VIP ownership as the authoritative master signal —
// a node that holds the VIP is MASTER even if a stale log says otherwise, and a
// node without the VIP is never MASTER even if a stale log says it was.
func TestDeriveVRRPState(t *testing.T) {
	cases := []struct {
		name             string
		keepalivedActive bool
		hasVIP           bool
		loggedState      string
		want             string
	}{
		{"keepalived down → FAULT", false, false, "", "FAULT"},
		{"keepalived down dominates stale MASTER log/VIP", false, true, "MASTER", "FAULT"},
		{"holds VIP → MASTER", true, true, "", "MASTER"},
		{"holds VIP wins over stale BACKUP log", true, true, "BACKUP", "MASTER"},
		{"active, no VIP → BACKUP", true, false, "", "BACKUP"},
		{"active, no VIP, logged FAULT → FAULT", true, false, "FAULT", "FAULT"},
		{"active, no VIP, stale MASTER log → still BACKUP", true, false, "MASTER", "BACKUP"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := deriveVRRPState(c.keepalivedActive, c.hasVIP, c.loggedState); got != c.want {
				t.Errorf("deriveVRRPState(active=%v, hasVIP=%v, logged=%q) = %q, want %q",
					c.keepalivedActive, c.hasVIP, c.loggedState, got, c.want)
			}
		})
	}
}
