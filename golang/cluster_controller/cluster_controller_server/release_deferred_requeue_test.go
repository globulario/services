package main

import (
	"context"
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// A release parked in DEFERRED or WAITING must be re-enqueued by the periodic
// release bridge once its backoff has elapsed.
//
// reconcileRelease carries a backoff branch for each of those phases that waits
// releaseWaitingBackoff and then re-enters PENDING. Those branches only run when
// something enqueues the release, and the work queue is fed by etcd watch
// events — so a release that parks and stops writing emits no event and is never
// reconsidered. The rescue code was unreachable in exactly the state it exists
// to rescue.
//
// Measured on the 5-node simulation, release 1.2.355, 2026-09-02: cluster-doctor,
// mcp, node-agent, resource and search entered DEFERRED at 19:12-19:14 UTC and
// were still DEFERRED at 19:44 with no intervening reconcile.
// platform-upgrade-release-boundary polled release.audit for 600s and failed with
// pending=5.

func requeueTestServer(t *testing.T) (*server, *[]string) {
	t.Helper()
	srv := &server{state: &controllerState{Nodes: map[string]*nodeState{}}}
	srv.resources = resourcestore.NewMemStore()
	var enqueued []string
	srv.releaseEnqueue = func(name string) { enqueued = append(enqueued, name) }
	srv.infraReleaseEnqueue = func(name string) { enqueued = append(enqueued, name) }
	return srv, &enqueued
}

func seedServiceReleaseForRequeue(t *testing.T, srv *server, name, phase string, transitionedAgo time.Duration) {
	t.Helper()
	if _, err := srv.resources.Apply(context.Background(), "ServiceRelease", &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: name},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{PublisherID: "core@globular.io", ServiceName: name},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase:                phase,
			LastTransitionUnixMs: time.Now().Add(-transitionedAgo).UnixMilli(),
		},
	}); err != nil {
		t.Fatalf("seed ServiceRelease %s: %v", name, err)
	}
}

func seedInfraReleaseForRequeue(t *testing.T, srv *server, name string, transitionedAgo time.Duration) {
	t.Helper()
	if _, err := srv.resources.Apply(context.Background(), "InfrastructureRelease",
		&cluster_controllerpb.InfrastructureRelease{
			Meta: &cluster_controllerpb.ObjectMeta{Name: name},
			Spec: &cluster_controllerpb.InfrastructureReleaseSpec{PublisherID: "core@globular.io", Component: "etcd"},
			Status: &cluster_controllerpb.InfrastructureReleaseStatus{
				Phase:                cluster_controllerpb.ReleasePhaseDeferred,
				LastTransitionUnixMs: time.Now().Add(-transitionedAgo).UnixMilli(),
			},
		}); err != nil {
		t.Fatalf("seed InfrastructureRelease %s: %v", name, err)
	}
}

func requeueContains(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func TestRequeueReleases_DeferredPastBackoffIsRequeued(t *testing.T) {
	for _, phase := range []string{
		cluster_controllerpb.ReleasePhaseDeferred,
		cluster_controllerpb.ReleasePhaseWaiting,
	} {
		t.Run(phase, func(t *testing.T) {
			srv, enqueued := requeueTestServer(t)
			seedServiceReleaseForRequeue(t, srv, "core@globular.io/mcp", phase,
				releaseWaitingBackoff+time.Minute)

			srv.requeueFailedReleases(context.Background())

			if !requeueContains(*enqueued, "core@globular.io/mcp") {
				t.Fatalf("a %s release past releaseWaitingBackoff must be re-enqueued; "+
					"without it the phase branch that re-enters PENDING never runs and the "+
					"release parks forever. enqueued=%v", phase, *enqueued)
			}
		})
	}
}

// The backoff must still be honoured: enqueueing on every 2-minute bridge tick
// regardless of age would turn the rescue into a churn source, which is the
// failure workflow_release.go's markOutOfScope split was written to end.
func TestRequeueReleases_DeferredInsideBackoffIsNotRequeued(t *testing.T) {
	srv, enqueued := requeueTestServer(t)
	seedServiceReleaseForRequeue(t, srv, "core@globular.io/mcp",
		cluster_controllerpb.ReleasePhaseDeferred, releaseWaitingBackoff/2)

	srv.requeueFailedReleases(context.Background())

	if requeueContains(*enqueued, "core@globular.io/mcp") {
		t.Fatalf("a DEFERRED release still inside releaseWaitingBackoff must not be "+
			"re-enqueued; enqueued=%v", *enqueued)
	}
}

// AVAILABLE is terminal for this scan — widening the switch must not start
// re-enqueueing releases that have nothing left to do.
func TestRequeueReleases_TerminalPhasesAreLeftAlone(t *testing.T) {
	srv, enqueued := requeueTestServer(t)
	seedServiceReleaseForRequeue(t, srv, "core@globular.io/rbac",
		cluster_controllerpb.ReleasePhaseAvailable, time.Hour)

	srv.requeueFailedReleases(context.Background())

	if requeueContains(*enqueued, "core@globular.io/rbac") {
		t.Fatalf("AVAILABLE releases must not be re-enqueued; enqueued=%v", *enqueued)
	}
}

// InfrastructureRelease has the identical dead branch in reconcileInfraRelease,
// and its own loop in the same scan.
func TestRequeueReleases_InfraDeferredPastBackoffIsRequeued(t *testing.T) {
	srv, enqueued := requeueTestServer(t)
	seedInfraReleaseForRequeue(t, srv, "core@globular.io/etcd", releaseWaitingBackoff+time.Minute)

	srv.requeueFailedReleases(context.Background())

	if !requeueContains(*enqueued, "core@globular.io/etcd") {
		t.Fatalf("a DEFERRED InfrastructureRelease past releaseWaitingBackoff must be "+
			"re-enqueued; enqueued=%v", *enqueued)
	}
}

func TestRequeueReleases_InfraDeferredInsideBackoffIsNotRequeued(t *testing.T) {
	srv, enqueued := requeueTestServer(t)
	seedInfraReleaseForRequeue(t, srv, "core@globular.io/etcd", releaseWaitingBackoff/2)

	srv.requeueFailedReleases(context.Background())

	if requeueContains(*enqueued, "core@globular.io/etcd") {
		t.Fatalf("a DEFERRED InfrastructureRelease inside releaseWaitingBackoff must not "+
			"be re-enqueued; enqueued=%v", *enqueued)
	}
}
