package main

import (
	"context"
	"strconv"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// The release work queue is fed by etcd watch events, and a dispatch writes to
// the release record. A dispatch that accomplishes nothing therefore re-triggers
// itself with no delay at all.
//
// Measured on the 5-node simulation, 1.2.359, 2026-09-03, while node-5's etcd was
// down: InfrastructureRelease core@globular.io/etcd was dispatched 169 times in
// five minutes — up to 118 controller log lines per second — because the workflow
// service answered every dispatch with "dispatch skipped: prior run <id> deferred
// until <ms>" and the controller read that refusal as a finished FAILED workflow.
//
// These tests pin the two bounds that replaced the spin: an exact deadline when
// the workflow service names one, and a floor under the rate in every case.

func TestParseDeferredDispatchSkip_ExtractsTheDeadline(t *testing.T) {
	deadline := time.Now().Add(90 * time.Second).Truncate(time.Millisecond)
	msg := "dispatch skipped: prior run 40999c11 deferred until " +
		strconv.FormatInt(deadline.UnixMilli(), 10) +
		" (defer_count=1): controller actor=node-agent action=node.verify_package_runtime failed"

	got, ok := parseDeferredDispatchSkip(msg)
	if !ok {
		t.Fatalf("skip not recognised: %q", msg)
	}
	if !got.Equal(deadline) {
		t.Errorf("deadline = %v, want %v", got, deadline)
	}
}

func TestParseDeferredDispatchSkip_IgnoresOrdinaryFailures(t *testing.T) {
	for _, msg := range []string{
		"",
		"controller actor=node-agent action=node.verify_package_runtime failed: status=inactive (want active)",
		"ABANDONED after 5/5 defers on step install_package: dependency blocked",
		"rpc error: code = Unavailable desc = connection refused",
	} {
		if _, ok := parseDeferredDispatchSkip(msg); ok {
			t.Errorf("parseDeferredDispatchSkip(%q) = true, want false — a real failure must keep its normal handling", msg)
		}
	}
}

// An unreadable deadline must never be read as "dispatch immediately": the one
// thing the message definitely says is that nothing ran.
func TestParseDeferredDispatchSkip_UnparsableDeadlineStillHolds(t *testing.T) {
	for _, msg := range []string{
		"dispatch skipped: prior run abc deferred until later (defer_count=1): x",
		"dispatch skipped: prior run abc (defer_count=1): x",
		"dispatch skipped: prior run abc deferred until 1 (defer_count=1): x", // long past
	} {
		until, ok := parseDeferredDispatchSkip(msg)
		if !ok {
			t.Fatalf("parseDeferredDispatchSkip(%q) = false, want true", msg)
		}
		if !until.After(time.Now().Add(minReleaseRedispatchInterval - 5*time.Second)) {
			t.Errorf("parseDeferredDispatchSkip(%q) held only until %v — must fall back to the floor", msg, until)
		}
	}
}

func TestHoldReleaseDispatch_BlocksThenExpires(t *testing.T) {
	srv := &server{}
	const id = "InfrastructureRelease/core@globular.io/etcd"

	if _, held := srv.releaseDispatchHeldUntil(id); held {
		t.Fatal("a release with no recorded hold must be dispatchable")
	}

	srv.holdReleaseDispatch(id, time.Now().Add(time.Minute))
	if _, held := srv.releaseDispatchHeldUntil(id); !held {
		t.Fatal("hold not honoured")
	}

	srv.holdReleaseDispatch(id, time.Now().Add(-time.Second)) // must not shorten
	if _, held := srv.releaseDispatchHeldUntil(id); !held {
		t.Fatal("a shorter hold shortened a longer one — the workflow service's deadline must win")
	}

	srv.releaseDispatchHold.Store(id, time.Now().Add(-time.Millisecond))
	if _, held := srv.releaseDispatchHeldUntil(id); held {
		t.Fatal("an elapsed hold must not block — a hold is a delay, never an abandonment")
	}
}

// A hold derived from a workflow-supplied timestamp is capped, so a malformed or
// far-future deadline delays a release instead of parking it forever.
func TestHoldReleaseDispatch_IsCapped(t *testing.T) {
	srv := &server{}
	const id = "ServiceRelease/dns"
	srv.holdReleaseDispatch(id, time.Now().Add(365*24*time.Hour))

	until, held := srv.releaseDispatchHeldUntil(id)
	if !held {
		t.Fatal("hold not recorded")
	}
	if until.After(time.Now().Add(maxReleaseRedispatchHold + time.Minute)) {
		t.Errorf("hold until %v exceeds the %s cap", until, maxReleaseRedispatchHold)
	}
}

// The hold removes the watch-event coupling that used to re-enter a RESOLVED
// InfrastructureRelease. Nothing else scheduled that phase — infra status has no
// NextRetryUnixMs — so the periodic bridge has to, or the storm is traded for a
// parked release.
func TestRequeueReleases_InfraResolvedIsRequeuedWhenNotHeld(t *testing.T) {
	srv, enqueued := requeueTestServer(t)
	seedInfraReleaseInPhase(t, srv, "core@globular.io/etcd", cluster_controllerpb.ReleasePhaseResolved)

	srv.requeueFailedReleases(context.Background())

	if !requeueContains(*enqueued, "core@globular.io/etcd") {
		t.Fatalf("RESOLVED InfrastructureRelease was not requeued; enqueued=%v", *enqueued)
	}
}

func TestRequeueReleases_InfraResolvedInsideHoldIsNotRequeued(t *testing.T) {
	srv, enqueued := requeueTestServer(t)
	seedInfraReleaseInPhase(t, srv, "core@globular.io/etcd", cluster_controllerpb.ReleasePhaseResolved)
	srv.holdReleaseDispatch("InfrastructureRelease/core@globular.io/etcd", time.Now().Add(2*time.Minute))

	srv.requeueFailedReleases(context.Background())

	if requeueContains(*enqueued, "core@globular.io/etcd") {
		t.Fatalf("a held release was requeued anyway; enqueued=%v", *enqueued)
	}
}

func seedInfraReleaseInPhase(t *testing.T, srv *server, name, phase string) {
	t.Helper()
	if _, err := srv.resources.Apply(context.Background(), "InfrastructureRelease",
		&cluster_controllerpb.InfrastructureRelease{
			Meta: &cluster_controllerpb.ObjectMeta{Name: name},
			Spec: &cluster_controllerpb.InfrastructureReleaseSpec{PublisherID: "core@globular.io", Component: "etcd"},
			Status: &cluster_controllerpb.InfrastructureReleaseStatus{
				Phase:                phase,
				LastTransitionUnixMs: time.Now().Add(-time.Hour).UnixMilli(),
			},
		}); err != nil {
		t.Fatalf("seed InfrastructureRelease %s: %v", name, err)
	}
}

// The membership sent with a wipe-and-rejoin must be the one the LIVE RING
// reports — and it must be the value alone, never the -state or -token keys that
// share its prefix.
func TestEtcdInitialClusterFromRenderedConfig(t *testing.T) {
	rendered := "name: \"node-5\"\n" +
		"initial-advertise-peer-urls: \"https://10.10.0.15:2380\"\n" +
		"initial-cluster: \"node-4=https://10.10.0.14:2380,globular-etcd=https://10.10.0.11:2380\"\n" +
		"initial-cluster-state: \"existing\"\n" +
		"initial-cluster-token: \"globular-etcd-cluster\"\n"

	got := etcdInitialClusterFromRenderedConfig(rendered)
	want := "node-4=https://10.10.0.14:2380,globular-etcd=https://10.10.0.11:2380"
	if got != want {
		t.Errorf("initial-cluster = %q, want %q", got, want)
	}

	// A render that produced no membership must yield "", which the caller reads
	// as "nothing lawful to send" and holds the destructive step.
	if got := etcdInitialClusterFromRenderedConfig("initial-cluster-state: \"new\"\n"); got != "" {
		t.Errorf("matched a prefix-sharing key: %q", got)
	}
	if got := etcdInitialClusterFromRenderedConfig(""); got != "" {
		t.Errorf("empty render yielded %q, want \"\"", got)
	}
}
