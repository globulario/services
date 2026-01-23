package rolling

import "testing"

func TestAdmitSerialBlocksIfUpgrading(t *testing.T) {
	allowed, reason := AdmitRolling(RollingPolicy{Serial: true}, []NodeRollState{{NodeID: "n1", IsUpgrading: true}})
	if allowed {
		t.Fatalf("expected deny, got allow")
	}
	if reason == "" {
		t.Fatalf("expected reason")
	}
}

func TestAdmitMaxUnavailableBlocks(t *testing.T) {
	policy := RollingPolicy{MaxUnavailable: 1}
	allowed, _ := AdmitRolling(policy, []NodeRollState{{NodeID: "n1", IsHealthy: false}})
	if allowed {
		t.Fatalf("expected deny when one unhealthy and maxUnavailable=1")
	}
}

func TestAdmitAllowsWhenHealthy(t *testing.T) {
	policy := RollingPolicy{MaxUnavailable: 1}
	allowed, _ := AdmitRolling(policy, []NodeRollState{{NodeID: "n1", IsHealthy: true}})
	if !allowed {
		t.Fatalf("expected allow")
	}
}
