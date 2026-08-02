package rules

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

// F4: a valid stale heartbeat must stay visible at ANY age.
//
// The rule previously gated on `age < maxPlausibleHeartbeatAgeSeconds` (30 days)
// as defense against the unset-timestamp artifact (`time() - 0` ≈ 56 years).
// That cutoff also discarded genuinely stale evidence: a controller or node dead
// 31, 100 or 500 days produced NO finding at all — precisely the case an operator
// most needs surfaced. The discriminator is now whether the IMPLIED heartbeat
// instant looks like the Unix epoch, so absence stays excluded while real
// staleness stays visible without any maximum age.

var promNow = time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)

func promSnap(age float64) *collector.Snapshot {
	return &collector.Snapshot{
		PromTS:      promNow,
		PromMetrics: map[string]float64{"controller_loop_heartbeat_age": age},
	}
}

func hasControllerStall(fs []Finding) bool {
	for _, f := range fs {
		if f.InvariantID == "prometheus.controller_stalled" {
			return true
		}
	}
	return false
}

func TestHeartbeatAge_ZeroTimestampIsNotStaleEvidence(t *testing.T) {
	// A never-set metric yields age == the whole Unix epoch at scrape time.
	age := float64(promNow.Unix())
	if heartbeatAgeIsRealEvidence(age, promNow) {
		t.Fatal("epoch-based age must be treated as absent evidence, not staleness")
	}
	if hasControllerStall(promRuntime{}.Evaluate(promSnap(age), Config{})) {
		t.Error("unset heartbeat must not produce a confident stall finding")
	}
}

func TestHeartbeatAge_MissingSampleProducesNoFinding(t *testing.T) {
	snap := &collector.Snapshot{PromTS: promNow, PromMetrics: map[string]float64{}}
	if hasControllerStall(promRuntime{}.Evaluate(snap, Config{})) {
		t.Error("absent sample must not be converted into a stale heartbeat")
	}
}

func TestHeartbeatAge_FreshRemainsHealthy(t *testing.T) {
	if hasControllerStall(promRuntime{}.Evaluate(promSnap(30), Config{})) {
		t.Error("a 30s-old heartbeat is fresh; no finding expected")
	}
}

func TestHeartbeatAge_JustBeyondThresholdFires(t *testing.T) {
	if !hasControllerStall(promRuntime{}.Evaluate(promSnap(181), Config{})) {
		t.Error("181s exceeds the 180s controller threshold; finding expected")
	}
}

// The regression cases: these are what the 30-day cutoff silently swallowed.
func TestHeartbeatAge_ThirtyOneDaysStillVisible(t *testing.T) {
	age := float64(31 * 24 * 3600)
	if !heartbeatAgeIsRealEvidence(age, promNow) {
		t.Fatal("a 31-day-old heartbeat is real evidence")
	}
	if !hasControllerStall(promRuntime{}.Evaluate(promSnap(age), Config{})) {
		t.Error("31-day stale heartbeat must remain visible (was hidden by the 30d cutoff)")
	}
}

func TestHeartbeatAge_FiveHundredDaysStillVisible(t *testing.T) {
	age := float64(500 * 24 * 3600)
	if !heartbeatAgeIsRealEvidence(age, promNow) {
		t.Fatal("a 500-day-old heartbeat is still real evidence")
	}
	if !hasControllerStall(promRuntime{}.Evaluate(promSnap(age), Config{})) {
		t.Error("500-day stale heartbeat must remain visible — there is no maximum age")
	}
}

func TestHeartbeatAge_FutureTimestampIsNotPastEvidence(t *testing.T) {
	if heartbeatAgeIsRealEvidence(-60, promNow) {
		t.Error("a negative age implies a future heartbeat; not past staleness evidence")
	}
}

// The non-leader guard (prometheus.controller_stalled.non_leader_guard) must
// survive this change untouched.
func TestHeartbeatAge_NonLeaderGuardPreserved(t *testing.T) {
	snap := promSnap(float64(31 * 24 * 3600))
	snap.PromMetrics["reconcile_dropped_not_leader"] = 5
	if hasControllerStall(promRuntime{}.Evaluate(snap, Config{})) {
		t.Error("non-leader scrape must still suppress the stall finding at any age")
	}
}
