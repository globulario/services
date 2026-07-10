package main

import (
	"errors"
	"testing"

	"github.com/globulario/services/golang/subsystem"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestIsControllerElectionGap asserts the classifier only treats a controller
// "no elected leader yet" signal (FailedPrecondition "not leader" with an EMPTY
// leader_addr) as a transient election gap. A populated leader_addr must NOT be
// classified as a gap (it is a redirect handled by sendStatusWithRetry), and
// unrelated errors must not be misclassified.
func TestIsControllerElectionGap(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "empty leader_addr election gap",
			err:  status.Error(codes.FailedPrecondition, "not leader (leader_addr=, epoch=0)"),
			want: true,
		},
		{
			name: "not leader without addr marker",
			err:  status.Error(codes.FailedPrecondition, "not leader"),
			want: true,
		},
		{
			name: "populated leader_addr is a redirect, not a gap",
			err:  status.Error(codes.FailedPrecondition, "not leader (leader_addr=leader:9999, epoch=3)"),
			want: false,
		},
		{
			name: "different FailedPrecondition (stale leader)",
			err:  status.Error(codes.FailedPrecondition, "stale leader: my_epoch=2, current_epoch=3 — re-campaigning"),
			want: false,
		},
		{
			name: "unavailable transport error",
			err:  status.Error(codes.Unavailable, "connection refused"),
			want: false,
		},
		{
			name: "plain error",
			err:  errors.New("boom"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isControllerElectionGap(tc.err); got != tc.want {
				t.Fatalf("isControllerElectionGap(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// TestElectionGapDoesNotDegradeSubsystem is the regression for the doctor
// finding "heartbeat SUBSYSTEM_STATE_DEGRADED: not leader (leader_addr=, epoch=0)
// (consecutive errors: 4)". It replays the runHeartbeat classification: an
// election-gap error routes to TickWaiting (no error count) within the grace
// window, so the subsystem stays out of DEGRADED; only once the grace window is
// exhausted does it fall through to TickError and surface.
func TestElectionGapDoesNotDegradeSubsystem(t *testing.T) {
	const graceCycles = 6
	hb := subsystem.RegisterSubsystem("heartbeat-test-gap", 30_000_000_000)
	t.Cleanup(func() { subsystem.DeregisterSubsystem("heartbeat-test-gap") })

	gapErr := status.Error(codes.FailedPrecondition, "not leader (leader_addr=, epoch=0)")

	// Simulate the runHeartbeat decision for many consecutive election-gap ticks.
	graceCount := 0
	apply := func() {
		if isControllerElectionGap(gapErr) && graceCount < graceCycles {
			graceCount++
			hb.TickWaiting("controller has no elected leader")
			return
		}
		hb.TickError(gapErr)
	}

	// Well past the old degrade threshold (3) but within grace: must NOT degrade.
	for i := 0; i < graceCycles; i++ {
		apply()
	}
	if got := subsystemState(t, "heartbeat-test-gap"); got == subsystem.SubsystemDegraded || got == subsystem.SubsystemFailed {
		t.Fatalf("subsystem degraded during election grace window: state=%v", got)
	}

	// Beyond the grace window it must surface as a real error (degrades).
	for i := 0; i < 3; i++ {
		apply()
	}
	if got := subsystemState(t, "heartbeat-test-gap"); got != subsystem.SubsystemDegraded && got != subsystem.SubsystemFailed {
		t.Fatalf("subsystem did not surface a persistently leaderless controller: state=%v", got)
	}

	// A single success clears the condition and resets grace.
	hb.Tick()
	if got := subsystemState(t, "heartbeat-test-gap"); got != subsystem.SubsystemHealthy {
		t.Fatalf("subsystem not healthy after success: state=%v", got)
	}
}

func subsystemState(t *testing.T, name string) subsystem.SubsystemState {
	t.Helper()
	for _, e := range subsystem.SubsystemSnapshot() {
		if e.Name == name {
			return e.State
		}
	}
	t.Fatalf("subsystem %q not found in snapshot", name)
	return subsystem.SubsystemStopped
}
