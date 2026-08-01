package main

import (
	"sync"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/remediation"
)

const (
	behavioralProject = "globular-services"
	behavioralDomain  = "cluster_operator"
)

// behavioralObservationRecorder is the narrow slice of the observation recorder
// this service needs. Kept minimal so tests can substitute a fake without
// standing up a queue or a connection.
type behavioralObservationRecorder interface {
	Enqueue(observation.Bundle) bool
	Stats() observation.Stats
}

// Notification seams. Package vars rather than fields so tests can observe the
// degraded paths without threading hooks through newServer.
var (
	behavioralDropsNotify = func(dropped int, sample *cluster_doctorpb.Finding, stats observation.Stats) {
		// Not Debug. A dropped learning event is operationally relevant: it is
		// the difference between "we have no evidence" and "we had evidence and
		// lost it". Aggregated per report so a saturated queue cannot turn one
		// doctor sweep into a warning storm.
		logger.Warn("behavioral observations dropped; findings not enqueued",
			"count", dropped,
			"sample_finding", sample.GetFindingId(),
			"sample_invariant", sample.GetInvariantId(),
			"sample_entity", sample.GetEntityRef(),
			"queue_depth", stats.QueueDepth,
			"recorder_last_error", stats.LastError,
		)
	}

	behavioralUnavailableNotify = func() {
		logger.Warn("behavioral recorder unavailable; doctor report delivered without learning persistence")
	}

	// A dropped remediation outcome is worse than a dropped finding: findings
	// recur on the next sweep, but a verification is a one-time fact about one
	// action. Logged individually (outcomes are rare, unlike per-sweep findings)
	// and states whether what was lost would have counted as governed evidence.
	behavioralOutcomeDropNotify = func(o remediation.Outcome, qualifying bool, stats observation.Stats) {
		logger.Warn("behavioral remediation outcome dropped; verification not recorded",
			"finding_id", o.FindingID,
			"workflow_run_id", o.WorkflowRunID,
			"invariant_id", o.InvariantID,
			"entity_ref", o.EntityRef,
			"status", string(o.Status()),
			"would_have_qualified", qualifying,
			"queue_depth", stats.QueueDepth,
			"recorder_last_error", stats.LastError,
		)
	}
)

// Unavailability is a steady state, not an event — rate-limit it so a doctor
// running without ai-memory does not fill the journal.
var (
	behavioralUnavailableMu   sync.Mutex
	behavioralUnavailableLast time.Time
)

const behavioralUnavailableLogInterval = 5 * time.Minute

func (s *ClusterDoctorServer) recordBehavioralUnavailable() {
	behavioralUnavailableMu.Lock()
	due := time.Since(behavioralUnavailableLast) >= behavioralUnavailableLogInterval
	if due {
		behavioralUnavailableLast = time.Now()
	}
	behavioralUnavailableMu.Unlock()
	if due {
		behavioralUnavailableNotify()
	}
}

// emitBehavioralClusterReport offers each cluster-wide finding to the bounded
// recorder.
//
// The doctor's primary duty is to observe and report; learning is secondary and
// must never delay or endanger it. Two consequences are deliberate:
//
//   - No goroutine. Enqueue is non-blocking by contract (see the recorder), so
//     the previous `go emitBehavioralDoctorFindings(...)` bought nothing and
//     cost an unbounded goroutine per report plus a gRPC dial per finding.
//   - No RecordBundle. That path dials and closes a connection per bundle; a
//     sweep over a large cluster turned into a burst of concurrent dials.
//
// A full queue or an absent recorder degrades LEARNING, never the report.
func (s *ClusterDoctorServer) emitBehavioralClusterReport(report *cluster_doctorpb.ClusterReport) {
	if report == nil || len(report.GetFindings()) == 0 {
		return
	}
	if s.behavioralRecorder == nil {
		s.recordBehavioralUnavailable()
		return
	}

	var (
		dropped int
		sample  *cluster_doctorpb.Finding
	)
	for _, finding := range report.GetFindings() {
		if finding == nil {
			continue
		}
		bundle := observation.FromDoctorFinding(
			behavioralProject,
			api.DomainRef(behavioralDomain),
			s.clusterID,
			report.GetHeader(),
			finding,
		)
		if !s.behavioralRecorder.Enqueue(bundle) {
			dropped++
			if sample == nil {
				sample = finding
			}
		}
	}
	if dropped > 0 {
		behavioralDropsNotify(dropped, sample, s.behavioralRecorder.Stats())
	}
}

// emitBehavioralRemediationOutcome records what a remediation actually
// achieved, closing the loop the finding path opens: the doctor observes a
// problem, the workflow acts, and this records whether the action worked —
// bound to the finding, the subject and the action that produced it.
//
// The eligibility decision lives entirely in the adapter. A successful,
// fully-attributable outcome becomes qualifying verification evidence; anything
// else becomes a diagnostic row that is recorded and deliberately unqualifiable.
// Re-deciding that here would put the same judgement in two places, and the two
// would drift.
//
// Delivery is best-effort by design, on the same terms as the finding path:
//
//   - No error return, and no effect on the caller. This runs inside the
//     workflow's verify step. A behavioral-memory outage must degrade LEARNING,
//     never turn a verified repair into a failed remediation — the workflow's
//     truth-consistency contract says success means the invariant cleared, not
//     that a side record was written.
//   - No goroutine and no per-event dial: Enqueue is non-blocking by contract
//     and the recorder holds one connection with a bounded queue.
//   - A drop is counted and logged, so degraded learning stays visible rather
//     than silent.
func (s *ClusterDoctorServer) emitBehavioralRemediationOutcome(o remediation.Outcome) {
	if s.behavioralRecorder == nil {
		s.recordBehavioralUnavailable()
		return
	}
	bundle, qualifies := observation.FromRemediationOutcome(
		behavioralProject, api.DomainRef(behavioralDomain), o,
	)
	if len(bundle.Evidence) == 0 {
		// An outcome with no identity at all. Nothing to key a stable record
		// on; recording it under a hash of nothing would collide with every
		// other such row.
		return
	}
	observation.BindRemediationEvidence(&bundle)

	if !s.behavioralRecorder.Enqueue(bundle) {
		behavioralOutcomeDropNotify(o, qualifies, s.behavioralRecorder.Stats())
	}
}
