package main

// Commit 4D: the production wiring from a remediation outcome to the bounded
// behavioral path.
//
// These tests assert the SEMANTICS of the wiring — what is recorded, what is
// deliberately not qualifying, and that a behavioral failure never reaches back
// into remediation truth. The evidence artifact itself is proven in
// workflow/engine/actors_doctor_evidence_test.go and against Scylla in
// ai_memory_server; this file proves the doctor hands the right thing to the
// right path and survives that path failing.

import (
	"testing"
	"time"

	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	"github.com/globulario/services/golang/remediation"
)

const (
	roFinding = "finding-1"
	roCluster = "cluster-1"
	roInvar   = "cluster.desired_state.absent"
	roEntity  = "svc/repository" // deliberately NOT the node id
	roNode    = "node-4"
	roRun     = "run-abc"
)

var (
	roDispatchAt = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	roVerifyAt   = roDispatchAt.Add(90 * time.Second)
)

func roOutcome(mut ...func(*remediation.Outcome)) remediation.Outcome {
	o := remediation.Outcome{
		FindingID: roFinding, WorkflowRunID: roRun,
		ClusterID: roCluster, InvariantID: roInvar, EntityRef: roEntity, NodeID: roNode,
		Dispatched: true, Verified: true, FindingResolved: true,
		DispatchedAt: roDispatchAt, VerifiedAt: roVerifyAt,
	}
	for _, f := range mut {
		f(&o)
	}
	return o
}

func roServer(rec behavioralObservationRecorder) *ClusterDoctorServer {
	return &ClusterDoctorServer{clusterID: roCluster, behavioralRecorder: rec}
}

// ── what gets recorded ──────────────────────────────────────────────────────

func TestEmitRemediationOutcome_SuccessfulCompleteEmitsQualifyingEvidence(t *testing.T) {
	rec := newFakeRecorder(true)
	roServer(rec).emitBehavioralRemediationOutcome(roOutcome())

	if rec.count() != 1 {
		t.Fatalf("expected exactly one bundle, got %d", rec.count())
	}
	b := rec.at(0)
	if len(b.Evidence) != 1 {
		t.Fatalf("expected one evidence row, got %d", len(b.Evidence))
	}
	ev := b.Evidence[0]

	if ev.Kind != observation.KindRemediationVerification {
		t.Errorf("kind = %q, want the qualifying verification kind", ev.Kind)
	}
	if ev.Result != observation.ResultFindingResolved {
		t.Errorf("result = %q, want %q", ev.Result, observation.ResultFindingResolved)
	}
	// Bound before it leaves the doctor. The satisfaction index is written from
	// Satisfies at persistence time, so an unbound row would be stored and yet
	// invisible to the governor.
	if len(ev.Satisfies) != 1 ||
		string(ev.Satisfies[0]) != "evidence.remediation.fresh_convergence_verification" {
		t.Fatalf("Satisfies = %v, want the verification requirement", ev.Satisfies)
	}
	if ev.ActionRef != roRun {
		t.Errorf("action_ref = %q, want %q", ev.ActionRef, roRun)
	}
	if ev.SourceRef != roFinding {
		t.Errorf("source_ref = %q, want the finding %q", ev.SourceRef, roFinding)
	}
	if ev.EntityRef != roEntity {
		t.Errorf("entity_ref = %q, want %q — not the node id", ev.EntityRef, roEntity)
	}
	// The recorder rejects a bundle with no signal, so an evidence-only bundle
	// would be dropped at Enqueue as malformed rather than delivered.
	if b.Signal.Project == "" || b.Signal.Domain == "" {
		t.Error("bundle must carry a signal with project and domain, or the recorder drops it")
	}
	if b.Signal.ClusterID != roCluster {
		t.Errorf("signal cluster = %q, want %q", b.Signal.ClusterID, roCluster)
	}
}

func TestEmitRemediationOutcome_IneligibleOutcomesAreDiagnosticOnly(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*remediation.Outcome)
		why   string
		check func(t *testing.T, o remediation.Outcome)
	}{
		{
			name: "successful but incomplete lineage",
			mut:  func(o *remediation.Outcome) { o.ClusterID = "" },
			why:  "a repair that worked but cannot be attributed is history, not citable verification",
			check: func(t *testing.T, o remediation.Outcome) {
				if !o.IsSuccess() {
					t.Fatal("this case must isolate lineage, not success")
				}
			},
		},
		{
			name: "unsuccessful with complete lineage",
			mut:  func(o *remediation.Outcome) { o.FindingResolved = false },
			why:  "complete provenance around a repair that did not work is not success",
			check: func(t *testing.T, o remediation.Outcome) {
				if !o.LineageComplete() {
					t.Fatalf("this case must isolate success, not lineage: %v", o.LineageDefects())
				}
			},
		},
		{
			name: "never dispatched",
			mut: func(o *remediation.Outcome) {
				o.Dispatched = false
				o.DispatchedAt = time.Time{}
				o.DispatchError = "blocked by approval gate"
			},
			why: "an action that never ran cannot have verified anything",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := roOutcome(tc.mut)
			if tc.check != nil {
				tc.check(t, o)
			}
			rec := newFakeRecorder(true)
			roServer(rec).emitBehavioralRemediationOutcome(o)

			if rec.count() != 1 {
				t.Fatalf("an ineligible outcome must still be RECORDED, got %d bundles", rec.count())
			}
			ev := rec.at(0).Evidence[0]
			if ev.Kind != observation.KindRemediationDiagnostic {
				t.Errorf("kind = %q, want the diagnostic kind (%s)", ev.Kind, tc.why)
			}
			if len(ev.Satisfies) != 0 {
				t.Errorf("an ineligible outcome must receive no binding, got %v", ev.Satisfies)
			}
			if ev.Result == observation.ResultFindingResolved {
				t.Error("an ineligible outcome must not carry the accepted result")
			}
			if ev.Metadata["not_qualifying_reason"] == "" {
				t.Error("an operator must be able to see WHY it does not qualify")
			}
		})
	}
}

// ── learning must never reach back into remediation ─────────────────────────

// A refused enqueue is counted and logged, and the call still returns normally.
// The workflow step that invoked it must be unaffected.
func TestEmitRemediationOutcome_DeliveryFailureIsVisibleAndHarmless(t *testing.T) {
	var (
		notified   bool
		gotQualify bool
		gotFinding string
	)
	orig := behavioralOutcomeDropNotify
	behavioralOutcomeDropNotify = func(o remediation.Outcome, qualifying bool, _ observation.Stats) {
		notified, gotQualify, gotFinding = true, qualifying, o.FindingID
	}
	defer func() { behavioralOutcomeDropNotify = orig }()

	rec := newFakeRecorder(false) // refuses everything
	roServer(rec).emitBehavioralRemediationOutcome(roOutcome())

	if !notified {
		t.Fatal("a dropped verification must be reported — silent loss is indistinguishable from never having the evidence")
	}
	if !gotQualify {
		t.Error("the notification must say the lost record WOULD have qualified")
	}
	if gotFinding != roFinding {
		t.Errorf("notification finding = %q, want %q", gotFinding, roFinding)
	}
}

// An absent recorder degrades learning only. The outcome's own verdict is
// untouched — behavioral persistence is not part of remediation truth.
func TestEmitRemediationOutcome_AbsentRecorderDoesNotFalsifyOutcome(t *testing.T) {
	var unavailable bool
	orig := behavioralUnavailableNotify
	behavioralUnavailableNotify = func() { unavailable = true }
	defer func() { behavioralUnavailableNotify = orig }()
	behavioralUnavailableMu.Lock()
	behavioralUnavailableLast = time.Time{}
	behavioralUnavailableMu.Unlock()

	o := roOutcome()
	srv := &ClusterDoctorServer{clusterID: roCluster} // no recorder
	srv.emitBehavioralRemediationOutcome(o)

	if !unavailable {
		t.Error("an absent recorder must be reported")
	}
	if !o.IsSuccess() || o.Status() != remediation.StatusSucceeded {
		t.Fatal("behavioral unavailability must not change the remediation verdict")
	}
}

// An outcome with no identity at all has nothing to key a stable record on.
func TestEmitRemediationOutcome_IdentitylessOutcomeRecordsNothing(t *testing.T) {
	rec := newFakeRecorder(true)
	roServer(rec).emitBehavioralRemediationOutcome(remediation.Outcome{})
	if rec.count() != 0 {
		t.Fatalf("an outcome with no finding and no run must not be recorded, got %d", rec.count())
	}
}

// ── replay ──────────────────────────────────────────────────────────────────

// A resumed or retried workflow re-reports the same result. The identity must be
// derived from the run and the subject so both the evidence row and its
// satisfaction-index row upsert instead of accumulating.
func TestEmitRemediationOutcome_ReplayIsIdempotent(t *testing.T) {
	rec := newFakeRecorder(true)
	srv := roServer(rec)
	o := roOutcome()

	srv.emitBehavioralRemediationOutcome(o)
	srv.emitBehavioralRemediationOutcome(o)

	if rec.count() != 2 {
		t.Fatalf("expected two enqueue attempts, got %d", rec.count())
	}
	a, b := rec.at(0), rec.at(1)
	if a.Evidence[0].ID != b.Evidence[0].ID {
		t.Fatalf("replay produced a different evidence id (%q vs %q) — the store would accumulate duplicates",
			a.Evidence[0].ID, b.Evidence[0].ID)
	}
	if a.Signal.ID != b.Signal.ID {
		t.Fatalf("replay produced a different signal id (%q vs %q)", a.Signal.ID, b.Signal.ID)
	}
	// observed_at participates in the satisfaction index clustering key: a
	// re-stamped clock on replay would create a SECOND index row for one fact.
	if a.Evidence[0].ObservedAt != b.Evidence[0].ObservedAt {
		t.Fatalf("replay changed observed_at (%d vs %d) — the index would hold the same fact twice",
			a.Evidence[0].ObservedAt, b.Evidence[0].ObservedAt)
	}

	// A genuinely different run is a different fact and must NOT collapse.
	other := roOutcome(func(o *remediation.Outcome) { o.WorkflowRunID = "run-other" })
	srv.emitBehavioralRemediationOutcome(other)
	if rec.at(2).Evidence[0].ID == a.Evidence[0].ID {
		t.Fatal("a different workflow run must produce a distinct evidence id")
	}
}
