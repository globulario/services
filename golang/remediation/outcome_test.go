package remediation

import (
	"strings"
	"testing"
	"time"
)

// TestRemediationWorkflowSuccessRequiresFindingResolved — contract test.
// The workflow may report SUCCEEDED to operators only when the underlying
// finding actually cleared. Verification alone is not enough — the verify
// step may have run and reported "still present", which must NOT be
// success.
func TestRemediationWorkflowSuccessRequiresFindingResolved(t *testing.T) {
	cases := []struct {
		name string
		o    Outcome
		want RemediationStatus
	}{
		{
			name: "dispatched + verified + resolved",
			o:    Outcome{FindingID: "f1", Dispatched: true, Verified: true, FindingResolved: true, VerifiedAt: time.Now()},
			want: StatusSucceeded,
		},
		{
			name: "dispatched + verified + still present",
			o:    Outcome{FindingID: "f1", Dispatched: true, Verified: true, FindingResolved: false, VerifiedAt: time.Now()},
			want: StatusDegraded,
		},
		{
			name: "dispatched but not yet verified",
			o:    Outcome{FindingID: "f1", Dispatched: true},
			want: StatusPending,
		},
		{
			name: "never dispatched",
			o:    Outcome{FindingID: "f1", DispatchError: "executor refused"},
			want: StatusFailed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.o.Status(); got != tc.want {
				t.Fatalf("status: got %s, want %s", got, tc.want)
			}
			isSuccess := tc.o.IsSuccess()
			if (tc.want == StatusSucceeded) != isSuccess {
				t.Fatalf("IsSuccess inconsistent with Status: status=%s success=%v", tc.want, isSuccess)
			}
		})
	}
}

// TestDispatchSuccessWithoutVerificationIsPending — contract test. The
// most common bug this contract guards against: an action runs, the
// executor returns 0, and the workflow declares victory before the
// verification step proves anything. That must always be PENDING.
func TestDispatchSuccessWithoutVerificationIsPending(t *testing.T) {
	o := Outcome{FindingID: "f-dispatch-only", Dispatched: true}
	if got := o.Status(); got != StatusPending {
		t.Fatalf("dispatch-without-verify: got %s, want PENDING_VERIFICATION", got)
	}
	if o.IsSuccess() {
		t.Fatal("dispatch-without-verify must not report success")
	}
	if !strings.Contains(o.Reason(), "awaiting verification") {
		t.Fatalf("reason must explain pending verification, got %q", o.Reason())
	}
}

// TestFindingRemainsActiveUntilVerifiedResolution — contract test. A
// finding stays in the active set until a remediation outcome with
// IsSuccess()==true is recorded. Pending and Degraded must NOT clear the
// finding — operators must still see it.
func TestFindingRemainsActiveUntilVerifiedResolution(t *testing.T) {
	active := NewActiveFindings("f-1", "f-2")
	if !active.IsActive("f-1") || !active.IsActive("f-2") {
		t.Fatal("seed findings must start active")
	}

	// Pending outcome does NOT clear.
	active.Record(Outcome{FindingID: "f-1", Dispatched: true})
	if !active.IsActive("f-1") {
		t.Fatal("pending outcome must NOT clear the finding")
	}

	// Degraded outcome does NOT clear.
	active.Record(Outcome{FindingID: "f-1", Dispatched: true, Verified: true, FindingResolved: false})
	if !active.IsActive("f-1") {
		t.Fatal("degraded outcome must NOT clear the finding")
	}

	// Failed outcome does NOT clear.
	active.Record(Outcome{FindingID: "f-2", DispatchError: "boom"})
	if !active.IsActive("f-2") {
		t.Fatal("failed outcome must NOT clear the finding")
	}

	// Verified + resolved outcome DOES clear.
	active.Record(Outcome{FindingID: "f-1", Dispatched: true, Verified: true, FindingResolved: true})
	if active.IsActive("f-1") {
		t.Fatal("verified + resolved outcome must clear the finding")
	}
	if !active.IsActive("f-2") {
		t.Fatal("clearing f-1 must not affect f-2")
	}
}
