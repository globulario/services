package globular_client

// Tests for the cluster_uid refusal diagnosis (defect 5, option c).
//
// The value of this code is entirely in what it says when a request is refused,
// so the tests assert the message names the cause — and, just as importantly,
// that it leaves every error it does not understand exactly as it found it.
// An annotator that rewrites unrelated errors is worse than none: it would put
// a confident, wrong explanation on failures that have nothing to do with
// membership.

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func resetClusterUIDState() {
	clusterUIDMu.Lock()
	clusterUIDReason = ""
	clusterUIDMu.Unlock()
}

// TestAnnotate_NamesTheCauseWhenTheBadgeWasUnavailable is the point of the
// change: the operator gets the root cause instead of a bare Unauthenticated.
func TestAnnotate_NamesTheCauseWhenTheBadgeWasUnavailable(t *testing.T) {
	resetClusterUIDState()
	clusterUIDMu.Lock()
	clusterUIDReason = "open /var/lib/globular/pki/issued/services/service.key: permission denied"
	clusterUIDMu.Unlock()

	in := status.Error(codes.Unauthenticated, "cluster_uid required after cluster initialization")
	out := annotateClusterUIDRefusal(in)

	msg := status.Convert(out).Message()
	if !strings.Contains(msg, "permission denied") {
		t.Fatalf("the refusal must carry the reason the badge was unavailable; got %q", msg)
	}
	if !strings.Contains(msg, "cluster_uid required after cluster initialization") {
		t.Fatalf("the server's original message must be preserved; got %q", msg)
	}
	if status.Code(out) != codes.Unauthenticated {
		t.Fatalf("the status code must not change, got %s", status.Code(out))
	}
}

// TestAnnotate_SaysNothingWhenTheBadgeWasAvailable is the control that stops
// this inventing a diagnosis. If the client DID attach cluster_uid, an
// Unauthenticated refusal is about something else — a membership mismatch, say
// — and attributing it to an unreadable keypair would send the reader to the
// wrong place entirely.
func TestAnnotate_SaysNothingWhenTheBadgeWasAvailable(t *testing.T) {
	resetClusterUIDState() // no recorded failure == the badge was attached

	in := status.Error(codes.Unauthenticated, "cluster_uid required after cluster initialization")
	out := annotateClusterUIDRefusal(in)

	if status.Convert(out).Message() != status.Convert(in).Message() {
		t.Fatalf("with no recorded failure the error must pass through untouched;\n got %q\nwant %q",
			status.Convert(out).Message(), status.Convert(in).Message())
	}
}

// TestAnnotate_LeavesUnrelatedErrorsAlone covers the errors this must never
// reshape. A wrong explanation is more expensive than none.
func TestAnnotate_LeavesUnrelatedErrorsAlone(t *testing.T) {
	clusterUIDMu.Lock()
	clusterUIDReason = "permission denied"
	clusterUIDMu.Unlock()
	defer resetClusterUIDState()

	cases := []struct {
		name string
		err  error
	}{
		{"nil", nil},
		{"plain error", errors.New("connection refused")},
		{"unavailable", status.Error(codes.Unavailable, "transport is closing")},
		{"permission denied", status.Error(codes.PermissionDenied, "rbac: not allowed")},
		{"unauthenticated but unrelated", status.Error(codes.Unauthenticated, "token expired")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := annotateClusterUIDRefusal(tc.err)
			if tc.err == nil {
				if out != nil {
					t.Fatalf("nil must stay nil, got %v", out)
				}
				return
			}
			if out.Error() != tc.err.Error() {
				t.Fatalf("unrelated error was rewritten:\n got %q\nwant %q", out.Error(), tc.err.Error())
			}
		})
	}
}

// TestNoteClusterUIDAvailable_ClearsAStaleReason: a client that recovers the
// badge must stop blaming a condition it no longer has.
func TestNoteClusterUIDAvailable_ClearsAStaleReason(t *testing.T) {
	noteClusterUIDUnavailable(errors.New("etcd unreachable"))
	clusterUIDMu.RLock()
	got := clusterUIDReason
	clusterUIDMu.RUnlock()
	if got == "" {
		t.Fatal("the reason was not recorded at all")
	}

	noteClusterUIDAvailable()

	in := status.Error(codes.Unauthenticated, "cluster_uid required after cluster initialization")
	if status.Convert(annotateClusterUIDRefusal(in)).Message() != status.Convert(in).Message() {
		t.Fatal("a recovered client still attributed refusals to the old failure")
	}
}

// TestNoteClusterUIDUnavailable_RecordsTheActualError guards the specific
// regression: the error used to be discarded, which is why this condition
// reached production as an unattributable intermittent refusal.
func TestNoteClusterUIDUnavailable_RecordsTheActualError(t *testing.T) {
	resetClusterUIDState()
	noteClusterUIDUnavailable(errors.New("service.key: permission denied"))

	clusterUIDMu.RLock()
	got := clusterUIDReason
	clusterUIDMu.RUnlock()

	if !strings.Contains(got, "permission denied") {
		t.Fatalf("the underlying error must be kept verbatim, got %q", got)
	}
}
