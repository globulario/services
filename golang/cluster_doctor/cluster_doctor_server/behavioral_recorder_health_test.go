package main

import (
	"context"
	"sync"
	"testing"
	"time"

	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// statsRecorder is a recorder whose Stats are set directly, so a health class
// can be produced without a queue or a connection.
type statsRecorder struct{ stats observation.Stats }

func (f *statsRecorder) Enqueue(observation.Bundle) bool          { return true }
func (f *statsRecorder) Stats() observation.Stats                 { return f.stats }
func (f *statsRecorder) enqueueFinding(*cluster_doctorpb.Finding) {}

type healthEvent struct {
	prev, cur observation.RecorderHealth
	stats     observation.Stats
}

// captureHealth installs the notify seam and returns the collected events.
func captureHealth(t *testing.T) *[]healthEvent {
	t.Helper()
	var got []healthEvent
	orig := behavioralRecorderHealthNotify
	behavioralRecorderHealthNotify = func(prev, cur observation.RecorderHealth, s observation.Stats) {
		got = append(got, healthEvent{prev, cur, s})
	}
	t.Cleanup(func() { behavioralRecorderHealthNotify = orig })
	return &got
}

// TestRecorderHealth_AcceptedThenFailedBecomesVisibleWithoutNewEnqueue is
// required behaviour 3 from #238, and the whole reason this surface exists.
//
// The recorder increments Failed after a terminal delivery failure, but the
// doctor only read Stats() when Enqueue ITSELF returned false. A bundle accepted
// into the queue and later exhausted by the worker was therefore invisible:
// "no behavioral events occurred" and "events were accepted and lost" produced
// identical silence. Polling is what separates them.
func TestRecorderHealth_AcceptedThenFailedBecomesVisibleWithoutNewEnqueue(t *testing.T) {
	events := captureHealth(t)
	rec := &statsRecorder{}
	s := &ClusterDoctorServer{behavioralRecorder: rec}

	// Bundle accepted, delivered fine.
	rec.stats = observation.Stats{Enqueued: 1, Persisted: 1, LastSuccessAt: time.Unix(100, 0)}
	s.projectRecorderHealth(time.Unix(100, 0))

	// A later bundle is accepted and then terminally lost by the worker. No
	// further Enqueue happens — the old code would never have looked again.
	rec.stats = observation.Stats{
		Enqueued: 2, Persisted: 1, Failed: 1,
		LastSuccessAt: time.Unix(100, 0), LastFailureAt: time.Unix(200, 0),
		LastError: "rpc error: code = Unavailable",
	}
	s.projectRecorderHealth(time.Unix(200, 0))

	if len(*events) != 2 {
		t.Fatalf("got %d transitions, want 2: %+v", len(*events), *events)
	}
	last := (*events)[1]
	if last.cur != observation.RecorderFailing {
		t.Errorf("class = %q, want %q — an accepted-then-failed bundle must surface "+
			"without a subsequent enqueue rejection", last.cur, observation.RecorderFailing)
	}
	if last.stats.LastError == "" || last.stats.Failed == 0 {
		t.Error("the emission must carry the evidence an operator needs (Failed, LastError)")
	}
}

// TestRecorderHealth_EmitsOnlyOnTransition guards against warning storms. A
// stuck failure restated every tick trains an operator to filter the line —
// the failure mode of an alert that is technically correct and practically
// ignored.
func TestRecorderHealth_EmitsOnlyOnTransition(t *testing.T) {
	events := captureHealth(t)
	rec := &statsRecorder{stats: observation.Stats{
		Enqueued: 5, Failed: 5, LastFailureAt: time.Unix(10, 0), LastError: "down",
	}}
	s := &ClusterDoctorServer{behavioralRecorder: rec}

	for i := 0; i < 6; i++ {
		s.projectRecorderHealth(time.Unix(int64(10+i), 0))
	}
	if len(*events) != 1 {
		t.Errorf("got %d emissions for one sustained failure, want 1 — a repeated "+
			"warning is a filtered warning", len(*events))
	}
}

// TestRecorderHealth_RecoveryIsReportedOnlyAfterSuccess is required behaviour 4.
// The degraded state must not clear on a timer or on the absence of new
// failures; only an observed successful persist downgrades it.
func TestRecorderHealth_RecoveryIsReportedOnlyAfterSuccess(t *testing.T) {
	events := captureHealth(t)
	rec := &statsRecorder{stats: observation.Stats{
		Enqueued: 3, Failed: 3, LastFailureAt: time.Unix(10, 0), LastError: "down",
	}}
	s := &ClusterDoctorServer{behavioralRecorder: rec}
	s.projectRecorderHealth(time.Unix(10, 0))

	// Time passes with no new failures and no successes. Still failing.
	s.projectRecorderHealth(time.Unix(999, 0))
	if len(*events) != 1 {
		t.Fatalf("state changed without an observed success: %+v", *events)
	}

	// An actual successful persist after the failure.
	rec.stats.Persisted = 1
	rec.stats.LastSuccessAt = time.Unix(1000, 0)
	s.projectRecorderHealth(time.Unix(1000, 0))

	if len(*events) != 2 {
		t.Fatalf("recovery was not reported: %+v", *events)
	}
	if (*events)[1].cur != observation.RecorderRecovered {
		t.Errorf("class = %q, want %q", (*events)[1].cur, observation.RecorderRecovered)
	}
}

// TestRecorderHealth_IdleIsNotReportedAsHealthy is required behaviour 5. A
// recorder nobody has asked anything is not a working recorder, and reporting
// it as healthy is the false green this issue exists to remove.
func TestRecorderHealth_IdleIsNotReportedAsHealthy(t *testing.T) {
	events := captureHealth(t)
	s := &ClusterDoctorServer{behavioralRecorder: &statsRecorder{}}
	s.projectRecorderHealth(time.Unix(1, 0))

	if len(*events) != 1 {
		t.Fatalf("want one initial classification, got %+v", *events)
	}
	if got := (*events)[0].cur; got != observation.RecorderIdle {
		t.Errorf("class = %q, want %q — no delivery attempted must stay distinguishable "+
			"from healthy delivery", got, observation.RecorderIdle)
	}
}

// TestRecorderHealth_NilRecorderIsUnavailableNotSilent — a doctor wired without
// a recorder records nothing at all. That is the strongest form of the
// ambiguity #238 names, so it must not be the quietest.
func TestRecorderHealth_NilRecorderIsUnavailableNotSilent(t *testing.T) {
	events := captureHealth(t)
	s := &ClusterDoctorServer{}
	s.projectRecorderHealth(time.Unix(1, 0))

	if len(*events) != 1 {
		t.Fatalf("a nil recorder produced no health signal: %+v", *events)
	}
	if got := (*events)[0].cur; got != recorderUnavailable {
		t.Errorf("class = %q, want %q", got, recorderUnavailable)
	}
}

// TestRecorderHealth_ErrorIsBounded — an unbounded remote error string reaching
// a log line makes the diagnosis harder to read than the disease.
func TestRecorderHealth_ErrorIsBounded(t *testing.T) {
	long := make([]byte, 4000)
	for i := range long {
		long[i] = 'x'
	}
	got := truncateRecorderError(string(long))
	if len(got) > maxRecorderErrorLen+len("… (truncated)") {
		t.Errorf("error not bounded: %d bytes", len(got))
	}
}

// ── required behaviour 6: the primary report survives a dead learning path ───

// unavailableRecorder models behavioural-memory being down the way it actually
// presents: the queue still ACCEPTS bundles (Enqueue is a local channel send,
// it has no idea the far end is gone), and the loss only shows up later in
// Stats. That is precisely why an enqueue-rejection signal could not see it.
type unavailableRecorder struct {
	mu       sync.Mutex
	accepted int
}

func (u *unavailableRecorder) Enqueue(observation.Bundle) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.accepted++
	return true // accepted into the queue — and terminally lost afterwards
}

func (u *unavailableRecorder) Stats() observation.Stats {
	u.mu.Lock()
	n := uint64(u.accepted)
	u.mu.Unlock()
	return observation.Stats{
		Enqueued: n, Failed: n,
		LastFailureAt: time.Unix(300, 0),
		LastError:     "rpc error: code = Unavailable desc = behavioral-memory unreachable",
	}
}

// TestRecorderHealth_PrimaryReportUnaffectedWhileDeliveryIsUnavailable is
// required behaviour 6 from #238.
//
// emitBehavioralClusterReport is the ONLY coupling between the primary report
// and behavioral persistence — server.go calls it from GetClusterReport after
// render.ClusterReport, and healer_loop.go calls it on the sweep. So proving
// the contract here proves it for both callers: a dead learning path must not
// fail the report, delay it, or alter a single finding.
//
// Learning is subordinate to reporting. If that ever inverts, an AI-memory
// outage stops being a degraded-learning event and becomes a cluster-diagnosis
// outage — the supplementary subsystem taking the primary one down with it.
func TestRecorderHealth_PrimaryReportUnaffectedWhileDeliveryIsUnavailable(t *testing.T) {
	captureHealth(t) // silence the transition log
	rec := &unavailableRecorder{}
	s := &ClusterDoctorServer{clusterID: "cluster-1", behavioralRecorder: rec}

	report := reportWith(
		&cluster_doctorpb.Finding{FindingId: "f1", Category: "desired_state"},
		&cluster_doctorpb.Finding{FindingId: "f2", Category: "runtime"},
	)
	before := len(report.GetFindings())

	done := make(chan struct{})
	go func() { defer close(done); s.emitBehavioralClusterReport(report) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("report path blocked on behavioral delivery: learning must never gate reporting")
	}

	if got := len(report.GetFindings()); got != before {
		t.Fatalf("findings mutated by the learning path: %d → %d", before, got)
	}
	for i, want := range []string{"f1", "f2"} {
		if got := report.GetFindings()[i].GetFindingId(); got != want {
			t.Fatalf("finding %d = %q, want %q — the report must be identical with delivery down", i, got, want)
		}
	}

	// And the failure is not merely survived, it is REPORTED. Surviving in
	// silence would be the original defect wearing a passing test.
	events := captureHealth(t)
	s.projectRecorderHealth(time.Unix(300, 0))
	if len(*events) != 1 || (*events)[0].cur != observation.RecorderFailing {
		t.Fatalf("delivery failure not surfaced while the report succeeded: %+v", *events)
	}
}

// The same contract on the remediation-outcome path. A verified repair must not
// be downgraded because a side record could not be written: the workflow's
// truth-consistency contract says success means the invariant cleared, not that
// behavioral-memory acknowledged anything.
func TestRecorderHealth_RemediationOutcomeUnaffectedWhileDeliveryIsUnavailable(t *testing.T) {
	captureHealth(t)
	rec := &unavailableRecorder{}

	done := make(chan struct{})
	var evidenceID string
	go func() {
		defer close(done)
		evidenceID = roServer(rec).emitBehavioralRemediationOutcome(roOutcome())
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("remediation verify step blocked on behavioral delivery")
	}
	if evidenceID == "" {
		t.Fatal("a fully-attributable outcome must still yield a citable evidence id with delivery down")
	}
}

// A doctor wired without a recorder at all must not fail the report either, and
// must not be silent about it. Unavailable and idle are different operator
// problems: idle means nothing has been asked of a working recorder.
func TestRecorderHealth_NoRecorderStillProducesTheReport(t *testing.T) {
	events := captureHealth(t)
	s := &ClusterDoctorServer{clusterID: "cluster-1"} // no recorder

	report := reportWith(&cluster_doctorpb.Finding{FindingId: "f1"})
	s.emitBehavioralClusterReport(report)
	if len(report.GetFindings()) != 1 {
		t.Fatal("report damaged by the absence of a recorder")
	}

	s.projectRecorderHealth(time.Unix(400, 0))
	if len(*events) != 1 || (*events)[0].cur != recorderUnavailable {
		t.Fatalf("missing recorder must be reported as unavailable, got %+v", *events)
	}
}

// The projection must not depend on the healer being enabled.
//
// It rode the healer tick first, and that was wrong: startHealerLoop returns
// early when healer_enabled=false, so a doctor with healing turned off would
// still run a recorder, still emit bundles from GetClusterReport, and have its
// delivery health permanently invisible. A config flag must not be able to
// restore the silence this whole change exists to remove.
func TestRecorderHealth_LoopRunsWithoutTheHealer(t *testing.T) {
	events := make(chan observation.RecorderHealth, 4)
	orig := behavioralRecorderHealthNotify
	behavioralRecorderHealthNotify = func(_, cur observation.RecorderHealth, _ observation.Stats) {
		select {
		case events <- cur:
		default:
		}
	}
	t.Cleanup(func() { behavioralRecorderHealthNotify = orig })

	// healer_enabled=false: startHealerLoop would return before ticking at all.
	s := &ClusterDoctorServer{
		clusterID:          "cluster-1",
		cfg:                &clusterdoctorConfig{HealerEnabled: false},
		behavioralRecorder: &unavailableRecorder{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.startRecorderHealthLoop(ctx)

	select {
	case got := <-events:
		// Idle: the fake has accepted nothing yet, and idle is not health.
		if got != observation.RecorderIdle {
			t.Fatalf("first projection = %q, want %q", got, observation.RecorderIdle)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no projection with the healer disabled: recorder health is invisible")
	}
}

// A doctor that comes up with no recorder at all must say so at startup, not
// one interval later. The first tick is the one an operator reads after a
// restart, and "nothing yet" there is indistinguishable from healthy.
func TestRecorderHealth_LoopProjectsImmediatelyOnStart(t *testing.T) {
	events := make(chan observation.RecorderHealth, 4)
	orig := behavioralRecorderHealthNotify
	behavioralRecorderHealthNotify = func(_, cur observation.RecorderHealth, _ observation.Stats) {
		select {
		case events <- cur:
		default:
		}
	}
	t.Cleanup(func() { behavioralRecorderHealthNotify = orig })

	s := &ClusterDoctorServer{clusterID: "cluster-1"} // no recorder
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	s.startRecorderHealthLoop(ctx)
	select {
	case got := <-events:
		if got != recorderUnavailable {
			t.Fatalf("first projection = %q, want %q", got, recorderUnavailable)
		}
		if elapsed := time.Since(start); elapsed >= recorderHealthInterval {
			t.Fatalf("first projection waited %s — it must not wait a full interval", elapsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no projection at startup")
	}
}
