package main

import (
	"sync"
	"testing"
	"time"

	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// fakeRecorder captures enqueued bundles without a queue or a connection.
type fakeRecorder struct {
	mu        sync.Mutex
	bundles   []observation.Bundle
	accept    bool
	stats     observation.Stats
	onEnqueue func()
}

func newFakeRecorder(accept bool) *fakeRecorder {
	return &fakeRecorder{accept: accept}
}

func (f *fakeRecorder) Enqueue(b observation.Bundle) bool {
	if f.onEnqueue != nil {
		f.onEnqueue()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bundles = append(f.bundles, b)
	return f.accept
}

func (f *fakeRecorder) Stats() observation.Stats { return f.stats }

func (f *fakeRecorder) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.bundles)
}

func (f *fakeRecorder) at(i int) observation.Bundle {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bundles[i]
}

func reportWith(findings ...*cluster_doctorpb.Finding) *cluster_doctorpb.ClusterReport {
	return &cluster_doctorpb.ClusterReport{
		Header:   &cluster_doctorpb.ReportHeader{Source: "cluster-doctor"},
		Findings: findings,
	}
}

// A finding is mapped and offered to the recorder with canonical project,
// domain and cluster identity preserved.
func TestEmitBehavioral_EnqueuesFinding(t *testing.T) {
	rec := newFakeRecorder(true)
	srv := &ClusterDoctorServer{clusterID: "cluster-1", behavioralRecorder: rec}

	srv.emitBehavioralClusterReport(reportWith(&cluster_doctorpb.Finding{
		FindingId:   "finding-1",
		InvariantId: "cluster.desired_state.absent",
	}))

	if rec.count() != 1 {
		t.Fatalf("want 1 enqueued bundle, got %d", rec.count())
	}
	sig := rec.at(0).Signal
	if sig.Project != behavioralProject {
		t.Errorf("project=%q want %q", sig.Project, behavioralProject)
	}
	if string(sig.Domain) != behavioralDomain {
		t.Errorf("domain=%q want %q", sig.Domain, behavioralDomain)
	}
	if sig.ClusterID != "cluster-1" {
		t.Errorf("cluster_id=%q want cluster-1", sig.ClusterID)
	}
}

// Several findings share one recorder — no per-finding client construction.
func TestEmitBehavioral_MultipleFindingsOneRecorder(t *testing.T) {
	rec := newFakeRecorder(true)
	srv := &ClusterDoctorServer{clusterID: "c", behavioralRecorder: rec}

	srv.emitBehavioralClusterReport(reportWith(
		&cluster_doctorpb.Finding{FindingId: "f1"},
		&cluster_doctorpb.Finding{FindingId: "f2"},
		&cluster_doctorpb.Finding{FindingId: "f3"},
	))

	if rec.count() != 3 {
		t.Fatalf("want 3 enqueued, got %d", rec.count())
	}
}

func TestEmitBehavioral_NilReport(t *testing.T) {
	rec := newFakeRecorder(true)
	srv := &ClusterDoctorServer{behavioralRecorder: rec}
	srv.emitBehavioralClusterReport(nil) // must not panic
	if rec.count() != 0 {
		t.Errorf("nil report → no enqueue, got %d", rec.count())
	}
}

func TestEmitBehavioral_EmptyReport(t *testing.T) {
	rec := newFakeRecorder(true)
	srv := &ClusterDoctorServer{behavioralRecorder: rec}
	srv.emitBehavioralClusterReport(reportWith())
	if rec.count() != 0 {
		t.Errorf("no findings → no enqueue, got %d", rec.count())
	}
}

// Nil entries are skipped without dropping the valid findings around them.
func TestEmitBehavioral_SkipsNilFindings(t *testing.T) {
	rec := newFakeRecorder(true)
	srv := &ClusterDoctorServer{clusterID: "c", behavioralRecorder: rec}

	srv.emitBehavioralClusterReport(reportWith(
		nil,
		&cluster_doctorpb.Finding{FindingId: "real"},
		nil,
	))

	if rec.count() != 1 {
		t.Fatalf("want 1 enqueued (nils skipped), got %d", rec.count())
	}
}

// A full queue degrades LEARNING, not the report: emit returns normally and the
// drop is surfaced, aggregated once per report rather than per finding.
func TestEmitBehavioral_QueueFullIsVisibleAndNonFatal(t *testing.T) {
	orig := behavioralDropsNotify
	defer func() { behavioralDropsNotify = orig }()

	var (
		calls      int
		gotDropped int
		gotSample  string
	)
	behavioralDropsNotify = func(dropped int, sample *cluster_doctorpb.Finding, _ observation.Stats) {
		calls++
		gotDropped = dropped
		gotSample = sample.GetFindingId()
	}

	rec := newFakeRecorder(false) // queue full
	srv := &ClusterDoctorServer{clusterID: "c", behavioralRecorder: rec}

	srv.emitBehavioralClusterReport(reportWith(
		&cluster_doctorpb.Finding{FindingId: "f1"},
		&cluster_doctorpb.Finding{FindingId: "f2"},
	))

	if calls != 1 {
		t.Errorf("drops aggregated once per report, got %d notifications", calls)
	}
	if gotDropped != 2 {
		t.Errorf("dropped=%d want 2", gotDropped)
	}
	if gotSample != "f1" {
		t.Errorf("sample=%q want first dropped finding f1", gotSample)
	}
}

// A nil recorder must not silently swallow the condition, and must not fail the
// report. Degraded learning is not degraded cluster health.
func TestEmitBehavioral_RecorderUnavailableIsVisible(t *testing.T) {
	orig := behavioralUnavailableNotify
	defer func() { behavioralUnavailableNotify = orig }()

	// Defeat the rate limiter for this assertion.
	behavioralUnavailableMu.Lock()
	behavioralUnavailableLast = time.Time{}
	behavioralUnavailableMu.Unlock()

	var called int
	behavioralUnavailableNotify = func() { called++ }

	srv := &ClusterDoctorServer{clusterID: "c"} // no recorder
	srv.emitBehavioralClusterReport(reportWith(&cluster_doctorpb.Finding{FindingId: "f1"}))

	if called != 1 {
		t.Errorf("unavailable recorder must be surfaced once, got %d", called)
	}
}

// Repeated unavailability is rate-limited — a doctor running without ai-memory
// must not fill the journal.
func TestEmitBehavioral_UnavailableIsRateLimited(t *testing.T) {
	orig := behavioralUnavailableNotify
	defer func() { behavioralUnavailableNotify = orig }()

	behavioralUnavailableMu.Lock()
	behavioralUnavailableLast = time.Time{}
	behavioralUnavailableMu.Unlock()

	var called int
	behavioralUnavailableNotify = func() { called++ }

	srv := &ClusterDoctorServer{clusterID: "c"}
	for i := 0; i < 5; i++ {
		srv.emitBehavioralClusterReport(reportWith(&cluster_doctorpb.Finding{FindingId: "f"}))
	}
	if called != 1 {
		t.Errorf("rate-limited unavailability → 1 log, got %d", called)
	}
}

// The report path must not await persistence. Proven by a recorder whose
// Enqueue signals on a channel and returns immediately: emit completes with the
// signal already delivered and no acknowledgment ever awaited.
func TestEmitBehavioral_DoesNotAwaitPersistence(t *testing.T) {
	enqueued := make(chan struct{}, 1)
	rec := newFakeRecorder(true)
	rec.onEnqueue = func() { enqueued <- struct{}{} }

	srv := &ClusterDoctorServer{clusterID: "c", behavioralRecorder: rec}

	done := make(chan struct{})
	go func() {
		srv.emitBehavioralClusterReport(reportWith(&cluster_doctorpb.Finding{FindingId: "f1"}))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("emit blocked — the report path must never wait on behavioral persistence")
	}
	select {
	case <-enqueued:
	default:
		t.Fatal("expected the bundle to have been offered to the recorder")
	}
}
