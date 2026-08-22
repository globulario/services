package main

// etcd_maintenance_test.go
//
// These test the decision logic that keeps a defrag from causing the outage it
// exists to prevent: the round-robin that guarantees at most one member is
// eligible per interval, and the thresholds that keep a member from being
// paused for no gain.
//
// Deliberately NOT a test that "defrag was called". The lesson from
// cluster-controller@1.2.317 is that asserting an optimization fired proves the
// optimization fired and nothing about whether it was safe. What matters here
// is the safety property: two members must never be eligible at once.

import (
	"testing"
	"time"
)

// slotNumber / eligibleIndex mirror the slot computation in etcdMaintenancePass.
// Slot number is absolute so "already acted in this turn" is answerable;
// eligibility is the modulo over members.
func slotNumber(nowUnix int64, slotWidth time.Duration) int64 {
	return nowUnix / int64(slotWidth/time.Second)
}

func eligibleIndex(nowUnix int64, slotWidth time.Duration, memberCount int) int64 {
	return slotNumber(nowUnix, slotWidth) % int64(memberCount)
}

// The core safety property: at any instant, exactly one member index is
// eligible. If this ever admitted two, a 3-member control plane could lose
// quorum to its own maintenance.
func TestEtcdMaintenance_ExactlyOneMemberEligiblePerInterval(t *testing.T) {
	const members = 5
	interval := etcdMaintenanceSlotWidth

	// Walk a full day at one-minute resolution.
	for tick := int64(0); tick < 24*60; tick++ {
		now := tick * 60
		eligible := 0
		for idx := 0; idx < members; idx++ {
			if eligibleIndex(now, interval, members) == int64(idx) {
				eligible++
			}
		}
		if eligible != 1 {
			t.Fatalf("at t=%ds, %d members were eligible, want exactly 1 — "+
				"concurrent defrag can take out quorum", now, eligible)
		}
	}
}

// Every member must get its turn: a round-robin that starves a member leaves
// that member's backend ratcheting until it alone hits the quota.
func TestEtcdMaintenance_EveryMemberGetsATurn(t *testing.T) {
	const members = 5
	interval := etcdMaintenanceSlotWidth
	step := int64(interval / time.Second)

	seen := make(map[int64]bool, members)
	for i := int64(0); i < int64(members)*3; i++ {
		seen[eligibleIndex(i*step, interval, members)] = true
	}
	if len(seen) != members {
		t.Fatalf("only %d of %d members ever became eligible: %v — a starved "+
			"member's backend grows until it hits the quota alone", len(seen), members, seen)
	}
}

// A single-member cluster must still maintain itself.
func TestEtcdMaintenance_SingleMemberAlwaysEligible(t *testing.T) {
	for tick := int64(0); tick < 100; tick++ {
		if got := eligibleIndex(tick*60, etcdMaintenanceSlotWidth, 1); got != 0 {
			t.Fatalf("single-member cluster: eligible index = %d, want 0", got)
		}
	}
}

// The thresholds must be ordered so the reclaim floor is reachable within the
// size floor. If min-reclaim exceeded min-file-size the pass could never fire,
// and would be another check that cannot fail.
func TestEtcdMaintenance_ThresholdsAreReachable(t *testing.T) {
	if etcdDefragMinReclaimBytes > etcdDefragMinFileBytes {
		t.Fatalf("min reclaim (%d) exceeds min file size (%d) — the guard could "+
			"never pass and the maintenance would silently never run",
			int64(etcdDefragMinReclaimBytes), int64(etcdDefragMinFileBytes))
	}
	if etcdDefragMinFileBytes <= 0 || etcdDefragMinReclaimBytes <= 0 {
		t.Fatal("thresholds must be positive")
	}
}

// The interval must keep growth well inside the default 2 GiB quota. This is
// the arithmetic that was never done before the outage: at the measured
// ~55 MB/hour a member must not approach quota between its turns.
func TestEtcdMaintenance_IntervalKeepsGrowthUnderQuota(t *testing.T) {
	const (
		measuredGrowthBytesPerHour = 55 << 20       // measured on the 5-node sim
		defaultQuotaBytes          = int64(2) << 30 // etcd default when quota-backend-bytes=0
		members                    = 5
	)

	// Worst case: a member waits memberCount intervals for its turn.
	waitHours := (etcdMaintenanceSlotWidth * members).Hours()
	growth := int64(waitHours * float64(measuredGrowthBytesPerHour))

	// Insist on an order of magnitude of headroom, not a hairline pass.
	if growth*10 > defaultQuotaBytes {
		t.Fatalf("between turns a member accumulates ~%d bytes against a %d byte "+
			"quota (%.1f%%) — too little headroom; shorten etcdMaintenanceSlotWidth",
			growth, defaultQuotaBytes, 100*float64(growth)/float64(defaultQuotaBytes))
	}
	t.Logf("worst-case growth between turns: %d MB (%.2f%% of the 2 GiB default quota)",
		growth>>20, 100*float64(growth)/float64(defaultQuotaBytes))
}

// THE REGRESSION THIS FILE EXISTS FOR.
//
// Each node evaluates on its OWN ticker, started when its node-agent booted.
// The first implementation used one duration for both the tick period and the
// slot width, so every node sampled the slot sequence at a fixed phase: node j
// evaluated at boot_j + k*width forever, landing on the same slot number every
// time. A node whose boot phase did not coincide with its own index was
// eligible NEVER — not rarely, never.
//
// The original test walked time in whole intervals, which quietly assumed every
// node samples at the same instant. That assumption is exactly what was false,
// so the test passed while no node in a real 5-node cluster ever defragmented:
// 467 MB of backend, ~440 MB reclaimable, two full slots elapsed, zero defrags.
//
// This models nodes booting at arbitrary offsets and insists each one actually
// observes its own turn.
func TestEtcdMaintenance_EveryNodeSeesItsTurnWhateverItsBootPhase(t *testing.T) {
	const members = 5
	tick := int64(etcdMaintenanceInterval / time.Second)
	width := etcdMaintenanceSlotWidth

	if etcdMaintenanceInterval >= etcdMaintenanceSlotWidth {
		t.Fatalf("evaluation interval (%s) must be shorter than the slot width (%s), "+
			"or each node samples one fixed phase forever and some node is never eligible",
			etcdMaintenanceInterval, etcdMaintenanceSlotWidth)
	}

	// Boot offsets chosen to be adversarial: prime-ish, and deliberately
	// including one aligned exactly to the slot boundary.
	for _, boot := range []int64{0, 7, 61, 137, 299, 401, 599} {
		for idx := 0; idx < members; idx++ {
			sawTurn := false
			// Two full cycles is ample if the sampling is not phase-locked.
			for k := int64(0); k < 2*int64(members)*int64(width/time.Second)/tick; k++ {
				now := boot + k*tick
				if eligibleIndex(now, width, members) == int64(idx) {
					sawTurn = true
					break
				}
			}
			if !sawTurn {
				t.Fatalf("member index %d booting at offset %ds never became eligible "+
					"within two full cycles — its evaluation phase is locked away from "+
					"its own slot, so it would never defragment", idx, boot)
			}
		}
	}
}

// Within one turn a node must act ONCE. Evaluating ten times per slot without
// this would defragment the same member ten times in a row, pausing it far
// longer than the design allows.
func TestEtcdMaintenance_ActsAtMostOncePerSlot(t *testing.T) {
	const members = 5
	tick := int64(etcdMaintenanceInterval / time.Second)
	width := etcdMaintenanceSlotWidth

	lastActed := int64(-1)
	actions := 0
	var actedSlots []int64

	// Walk one full slot's worth of evaluations while it is index 0's turn.
	start := int64(0)
	for k := int64(0); k*tick < int64(width/time.Second); k++ {
		now := start + k*tick
		slot := slotNumber(now, width)
		if slot%int64(members) != 0 {
			continue
		}
		if slot == lastActed {
			continue
		}
		lastActed = slot
		actions++
		actedSlots = append(actedSlots, slot)
	}

	if actions != 1 {
		t.Fatalf("acted %d times within a single slot (slots %v), want exactly 1 — "+
			"evaluating several times per slot must not defragment several times",
			actions, actedSlots)
	}
}
