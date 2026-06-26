package actions

import "testing"

// TestDesiredVersionCheckAppliesToKind locks the read-side half of
// invariant desired.keyed_by_kind_and_name: the I2 desired-version drift check
// (installed vs ServiceDesiredVersion) applies ONLY to SERVICE packages.
// INFRASTRUCTURE/COMMAND/APPLICATION packages have their own controller-owned
// desired authority (InfrastructureRelease/ApplicationRelease); comparing them
// against service-desired re-introduces the cross-kind stale-resolved ghost
// (the xds 1.2.235-vs-1.2.237 phantom drift).
func TestDesiredVersionCheckAppliesToKind(t *testing.T) {
	cases := []struct {
		kind string
		want bool
	}{
		{"SERVICE", true},
		{"service", true},
		{"  Service  ", true},
		{"INFRASTRUCTURE", false},
		{"infrastructure", false},
		{"COMMAND", false},
		{"APPLICATION", false},
		{"", false},
	}
	for _, c := range cases {
		if got := desiredVersionCheckAppliesToKind(c.kind); got != c.want {
			t.Errorf("desiredVersionCheckAppliesToKind(%q) = %v, want %v", c.kind, got, c.want)
		}
	}
}
