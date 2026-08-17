package main

// event_client_reconnect_test.go
//
// The event client was dialed once at boot and cached in an atomic pointer, so
// when its connection died the controller published into a closed connection
// forever. The circuit breaker made that look handled — it opened, logged, and
// reopened every 30 seconds indefinitely, which reads like backpressure rather
// than a permanently broken client.
//
// Observed on the 5-node simulation, 2026-08-17, after the resilience suite
// restarted services: 116 occurrences of "grpc: the client connection is
// closing", 5 within the last 3 minutes, across cluster.dns_reconciled,
// cluster.drift_detected, cluster.reconcile.clean and
// controller.invariant_enforcement_report. Cluster health, liveness and etcd
// were all fine — only the event stream was silently dead.
//
// Same shape as the workflow client pinned at boot (fixed in 1be9dbd5): a
// cached client encodes an assumption that the peer never restarts or moves.
// This tests the discrimination the fix depends on, because getting it wrong in
// either direction is harmful: missing a dead connection leaves the original
// bug, and re-dialling on ordinary publish errors churns a healthy client.

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestIsDeadClientConn_DetectsConnectionsThatNeverRecover(t *testing.T) {
	dead := []struct {
		name string
		err  error
	}{
		{
			// The exact error observed in production.
			name: "client connection is closing",
			err: status.Error(codes.Canceled,
				`grpc: the client connection is closing`),
		},
		{
			name: "transport is closing",
			err:  status.Error(codes.Unavailable, `transport is closing`),
		},
		{
			name: "connection refused",
			err: status.Error(codes.Unavailable,
				`connection error: desc = "transport: Error while dialing: dial tcp 10.10.0.11:10002: connect: connection refused"`),
		},
		{
			// The service moved to a node that no longer resolves.
			name: "no such host",
			err: status.Error(codes.Unavailable,
				`connection error: desc = "transport: Error while dialing: dial tcp: lookup event.globular.internal: no such host"`),
		},
	}

	for _, tc := range dead {
		t.Run(tc.name, func(t *testing.T) {
			if !isDeadClientConn(tc.err) {
				t.Fatalf("isDeadClientConn(%v) = false, want true — an undetected dead "+
					"connection means every later publish fails forever while the "+
					"circuit breaker makes it look like backpressure", tc.err)
			}
		})
	}
}

// The other half, and the one that protects a working cluster: an ordinary
// failed publish must NOT cause a re-dial. Treating every error as fatal would
// drop and rebuild a perfectly good client on any transient hiccup.
func TestIsDeadClientConn_LeavesHealthyClientsAlone(t *testing.T) {
	alive := []struct {
		name string
		err  error
	}{
		{"nil", nil},
		{"deadline exceeded", status.Error(codes.DeadlineExceeded, "context deadline exceeded")},
		{"permission denied", status.Error(codes.PermissionDenied, "rbac: denied")},
		{"invalid argument", status.Error(codes.InvalidArgument, "bad event payload")},
		{"internal", status.Error(codes.Internal, "event service panicked")},
		{"plain error", errors.New("some non-grpc failure")},
		{"context cancelled by caller", context.Canceled},
		{
			// Unavailable, but transient: the peer is up and pushing back.
			name: "unavailable but not a dead conn",
			err:  status.Error(codes.Unavailable, "service unavailable: too many concurrent requests"),
		},
	}

	for _, tc := range alive {
		t.Run(tc.name, func(t *testing.T) {
			if isDeadClientConn(tc.err) {
				t.Fatalf("isDeadClientConn(%v) = true, want false — re-dialling on an "+
					"ordinary error churns a healthy client on every transient failure", tc.err)
			}
		})
	}
}
