package main

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The legacy auto-join loop classified PermissionDenied, NotFound and
// InvalidArgument as non-retriable and omitted FailedPrecondition entirely, so a
// preflight refusal retried at the 60s ceiling forever.
//
// Observed 2026-08-18: the founding node holds a join token and has no JoinID,
// so it takes this legacy path to join the cluster it already leads. Preflight
// refused it as "node identity conflict: hostname already present" and it
// retried 96 times in 2.5 hours, exhausting a 100-use token and locking every
// genuine joiner out of the cluster.

func TestJoinErrorIsPermanentRejection(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"permission denied", status.Error(codes.PermissionDenied, "token uses exhausted"), true},
		{"not found", status.Error(codes.NotFound, "join token not found"), true},
		{"invalid argument", status.Error(codes.InvalidArgument, "bad identity"), true},
		{"failed precondition is NOT permanent", status.Error(codes.FailedPrecondition, "join preflight blocked: ..."), false},
		{"unavailable is transient", status.Error(codes.Unavailable, "controller down"), false},
		{"deadline exceeded is transient", status.Error(codes.DeadlineExceeded, "timeout"), false},
		{"plain error", errors.New("dial tcp: connection refused"), false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := joinErrorIsPermanentRejection(tc.err); got != tc.want {
				t.Errorf("joinErrorIsPermanentRejection() = %v, want %v", got, tc.want)
			}
		})
	}
}

// FailedPrecondition must be recognised so the caller can BOUND it. Treating it
// as permanent would strand a node whose NIC is not up yet ("routable
// non-loopback IP is required"); treating it as ordinary-transient is what
// produced the infinite loop.
func TestJoinErrorIsPreflightRefusal(t *testing.T) {
	if !joinErrorIsPreflightRefusal(status.Error(codes.FailedPrecondition, "join preflight blocked: hostname already present")) {
		t.Error("a preflight refusal must be recognised so its retries can be bounded")
	}
	for _, err := range []error{
		status.Error(codes.PermissionDenied, "x"),
		status.Error(codes.Unavailable, "x"),
		errors.New("boom"),
		nil,
	} {
		if joinErrorIsPreflightRefusal(err) {
			t.Errorf("non-preflight error classified as a preflight refusal: %v", err)
		}
	}
}

// The two predicates must stay disjoint: an error that is both permanent and a
// bounded-retry refusal would make the loop's behaviour depend on check order.
func TestJoinErrorClassificationsAreDisjoint(t *testing.T) {
	for _, code := range []codes.Code{
		codes.PermissionDenied, codes.NotFound, codes.InvalidArgument,
		codes.FailedPrecondition, codes.Unavailable, codes.DeadlineExceeded,
		codes.Internal, codes.Unknown,
	} {
		err := status.Error(code, "x")
		if joinErrorIsPermanentRejection(err) && joinErrorIsPreflightRefusal(err) {
			t.Errorf("code %s is classified as both permanent and a bounded-retry refusal", code)
		}
	}
}
