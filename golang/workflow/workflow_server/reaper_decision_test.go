package main

import (
	"testing"
	"time"

	"github.com/globulario/services/golang/workflow/workflowpb"
)

// TestRunIsRecoverableWhenUnleased pins the eligibility for universal recovery:
// only EXECUTING runs are driven when unowned. Terminal, DEFERRED (parked), and
// especially BLOCKED (awaiting operator approval) must never be resumed.
func TestRunIsRecoverableWhenUnleased(t *testing.T) {
	recoverable := map[workflowpb.RunStatus]bool{
		workflowpb.RunStatus_RUN_STATUS_EXECUTING:   true,
		workflowpb.RunStatus_RUN_STATUS_BLOCKED:     false,
		workflowpb.RunStatus_RUN_STATUS_DEFERRED:    false,
		workflowpb.RunStatus_RUN_STATUS_SUCCEEDED:   false,
		workflowpb.RunStatus_RUN_STATUS_FAILED:      false,
		workflowpb.RunStatus_RUN_STATUS_CANCELED:    false,
		workflowpb.RunStatus_RUN_STATUS_ROLLED_BACK: false,
		workflowpb.RunStatus_RUN_STATUS_SUPERSEDED:  false,
		workflowpb.RunStatus_RUN_STATUS_UNKNOWN:     false,
		// Pre-execution state is conservatively NOT recovered here — only a run
		// that reached EXECUTING but lost its owner is driven.
		workflowpb.RunStatus_RUN_STATUS_PENDING: false,
	}
	for status, want := range recoverable {
		if got := runIsRecoverableWhenUnleased(int(status)); got != want {
			t.Errorf("runIsRecoverableWhenUnleased(%s) = %v, want %v", status, got, want)
		}
	}
}

// withReaperTunables sets the package-level reaper tunables to small, explicit
// values for the duration of a test and restores them afterward. The values are
// chosen so the boundaries are unambiguous.
func withReaperTunables(t *testing.T) {
	t.Helper()
	origGrace := reaperLeaseGracePeriod
	origProgress := reaperProgressDeadline
	origStale := reaperStaleThreshold
	reaperLeaseGracePeriod = 2 * time.Minute
	reaperProgressDeadline = 10 * time.Minute
	reaperStaleThreshold = 15 * time.Minute
	t.Cleanup(func() {
		reaperLeaseGracePeriod = origGrace
		reaperProgressDeadline = origProgress
		reaperStaleThreshold = origStale
	})
}

func msAgo(now time.Time, d time.Duration) int64 {
	return now.Add(-d).UnixMilli()
}

// TestReaperDecide covers the liveness policy exhaustively. The single most
// important case is "alive + slow-but-progressing step" → Skip, which is the
// guard against failure_mode:workflow.orphan_lease_slow_step_double_claim.
func TestReaperDecide(t *testing.T) {
	withReaperTunables(t)
	now := time.Unix(1_800_000_000, 0)

	cases := []struct {
		name      string
		lease     leaseLiveness
		updatedAt time.Time
		startedAt time.Time
		want      reaperVerdict
	}{
		{
			name: "alive and advancing → skip",
			lease: leaseLiveness{present: true, owner: "exec:a", heartbeatAt: msAgo(now, 5*time.Second),
				lastProgressAt: msAgo(now, 20*time.Second)},
			startedAt: now.Add(-30 * time.Minute),
			want:      reaperSkip,
		},
		{
			name: "alive but slow live step within deadline → skip (double-claim guard)",
			lease: leaseLiveness{present: true, owner: "exec:a", heartbeatAt: msAgo(now, 5*time.Second),
				lastProgressAt: msAgo(now, 9*time.Minute)}, // < 10m deadline
			startedAt: now.Add(-30 * time.Minute),
			want:      reaperSkip,
		},
		{
			name: "alive but hung: no progress past deadline → revoke",
			lease: leaseLiveness{present: true, owner: "exec:a", heartbeatAt: msAgo(now, 5*time.Second),
				lastProgressAt: msAgo(now, 11*time.Minute)}, // > 10m deadline
			startedAt: now.Add(-30 * time.Minute),
			want:      reaperRevokeForResume,
		},
		{
			name: "alive, no progress signal yet (lastProgressAt==0) → skip",
			lease: leaseLiveness{present: true, owner: "exec:a", heartbeatAt: msAgo(now, 5*time.Second),
				lastProgressAt: 0},
			startedAt: now.Add(-30 * time.Minute),
			want:      reaperSkip,
		},
		{
			name: "tombstoned (revoked) lease, heartbeat stale → skip (resume pending)",
			lease: leaseLiveness{present: true, owner: revokedExecutorID, heartbeatAt: 0,
				lastProgressAt: 0},
			startedAt: now.Add(-30 * time.Minute),
			want:      reaperSkip,
		},
		{
			name:      "no lease, activity within threshold → skip",
			lease:     leaseLiveness{present: false},
			updatedAt: now.Add(-5 * time.Minute),
			startedAt: now.Add(-30 * time.Minute),
			want:      reaperSkip,
		},
		{
			name:      "no lease, stale past threshold → fail",
			lease:     leaseLiveness{present: false},
			updatedAt: now.Add(-20 * time.Minute),
			startedAt: now.Add(-30 * time.Minute),
			want:      reaperMarkFailed,
		},
		{
			name: "dead executor (heartbeat stale) + stale past threshold → fail",
			lease: leaseLiveness{present: true, owner: "exec:dead", heartbeatAt: msAgo(now, 20*time.Minute),
				lastProgressAt: msAgo(now, 20*time.Minute)},
			updatedAt: now.Add(-20 * time.Minute),
			startedAt: now.Add(-30 * time.Minute),
			want:      reaperMarkFailed,
		},
		{
			name: "dead executor (heartbeat stale) but activity recent → skip",
			lease: leaseLiveness{present: true, owner: "exec:dead", heartbeatAt: msAgo(now, 5*time.Minute),
				lastProgressAt: msAgo(now, 5*time.Minute)},
			updatedAt: now.Add(-1 * time.Minute),
			startedAt: now.Add(-30 * time.Minute),
			want:      reaperSkip,
		},
		{
			name:      "no lease, updatedAt zero, started past threshold → fail",
			lease:     leaseLiveness{present: false},
			updatedAt: time.Time{},
			startedAt: now.Add(-20 * time.Minute),
			want:      reaperMarkFailed,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := reaperDecide(now, tc.lease, tc.updatedAt, tc.startedAt)
			if got != tc.want {
				t.Fatalf("reaperDecide = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestRecordProgressAndRevokeWithoutScylla verifies the new lease methods are
// safe no-ops when the ScyllaDB fence is unavailable (single-node/degraded).
func TestRecordProgressAndRevokeWithoutScylla(t *testing.T) {
	srv := &server{} // session == nil
	m := newExecutorLeaseManager(srv)

	// Must not panic.
	m.RecordProgress("run-x")

	ok, err := m.RevokeLease("run-x", "exec:whoever")
	if err != nil {
		t.Fatalf("RevokeLease without ScyllaDB should not error, got: %v", err)
	}
	if ok {
		t.Error("RevokeLease without ScyllaDB should report not-applied (false)")
	}
}
