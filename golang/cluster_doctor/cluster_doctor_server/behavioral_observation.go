package main

import (
	"sync"
	"time"

	"github.com/globulario/services/golang/ai_memory/behavioral/api"
	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
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
