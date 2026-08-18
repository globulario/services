package main

// behavioral_recorder_health.go projects recorder delivery health into an
// operator-visible surface (#238 defect 2).
//
// The gap it closes: the recorder increments Failed/LastFailureAt/LastError
// after a terminal delivery failure, but the doctor only read Stats() when
// Enqueue itself returned false. A bundle ACCEPTED into the queue and later
// exhausted by the worker was invisible — "no behavioral events occurred" and
// "events were accepted and lost" produced the same silence.
//
// So this polls, on every healer tick, independently of any enqueue rejection.
// A failure that happened between ticks is still reported, because the state is
// read rather than waited for.
//
// NODE-SCOPED, and deliberately not a cluster finding. Each doctor instance has
// its own recorder, so this describes THIS process's delivery health, not the
// cluster's. Feeding it into the cluster delta cache is the shape
// doctor.cache_poisoning_from_node_scoped_findings names and
// use_node_scope_findings_for_cluster_delta forbids — a per-node condition
// entering a cluster-level cache produces phantom events at every observer.
//
// Not leader-gated either, and that is the point. Remediation is leader-only
// because only one actor may act; OBSERVING your own recorder is not an action
// and needs no election. Gating it would make a follower's failing recorder
// permanently invisible — precisely the silence this exists to break.

import (
	"time"

	observation "github.com/globulario/services/golang/ai_memory/domains/cluster_operator/observation"
)

// recorderHealthState is the transition tracker. Emission is edge-triggered:
// only a CHANGE in class is reported.
//
// Level-triggered emission would restate a stuck failure on every tick, and a
// warning that repeats forever trains an operator to filter it — the failure
// mode of an alert that is technically correct and practically ignored. The
// tradeoff is accepted deliberately: a sustained failure is announced once, so
// the surface stays readable, and the current class remains queryable in the
// report at any time.
type recorderHealthState struct {
	last     observation.RecorderHealth
	lastSeen time.Time
}

// projectRecorderHealth reads recorder health and emits on transition.
//
// It never returns an error and never blocks on I/O: reading Stats is a local
// mutex-guarded snapshot. Behavioral persistence is supplementary, and a
// degraded learning pipeline must not fail or delay the primary doctor report
// (#238 required behaviour: keep behavioral persistence supplementary).
func (s *ClusterDoctorServer) projectRecorderHealth(now time.Time) {
	var (
		stats  observation.Stats
		health observation.RecorderHealth
	)
	if s.behavioralRecorder == nil {
		// "Recorder unavailable" is a doctor-side observation, not a recorder
		// one: a recorder that does not exist cannot report on itself. It is
		// classified here rather than in Stats.Health() for that reason, and it
		// must not be silent — a doctor wired without a recorder produces no
		// behavioral events at all, which is the exact ambiguity #238 names.
		health = recorderUnavailable
	} else {
		stats = s.behavioralRecorder.Stats()
		health = stats.Health()
	}

	prev := s.recorderHealth.last
	if prev == health {
		s.recorderHealth.lastSeen = now
		return
	}
	s.recorderHealth.last = health
	s.recorderHealth.lastSeen = now

	// prev == "" is the first observation of this process, not a transition
	// from healthy. Reporting it as a change would announce a state nothing
	// moved away from.
	behavioralRecorderHealthNotify(prev, health, stats)
}

// behavioralRecorderHealthNotify is the emission seam. A package var so tests
// can observe transitions without a log scraper.
var behavioralRecorderHealthNotify = func(prev, cur observation.RecorderHealth, stats observation.Stats) {
	fields := []interface{}{
		"health", string(cur),
		"scope", "node-local recorder; not a cluster health verdict",
		"enqueued", stats.Enqueued,
		"persisted", stats.Persisted,
		"retried", stats.Retried,
		"dropped", stats.Dropped,
		"failed", stats.Failed,
		"queue_depth", stats.QueueDepth,
	}
	if prev != "" {
		fields = append(fields, "previous", string(prev))
	}
	if !stats.LastSuccessAt.IsZero() {
		fields = append(fields, "last_success_at", stats.LastSuccessAt.UTC().Format(time.RFC3339))
	}
	if !stats.LastFailureAt.IsZero() {
		fields = append(fields, "last_failure_at", stats.LastFailureAt.UTC().Format(time.RFC3339))
	}
	if stats.LastError != "" {
		fields = append(fields, "last_error", truncateRecorderError(stats.LastError))
	}

	switch cur {
	case observation.RecorderFailing, recorderUnavailable:
		// WARN, never ERROR. The cluster is not less healthy because learning
		// is degraded — the doctor's own verdict is unaffected. Raising this to
		// ERROR would make a supplementary subsystem look like a cluster fault.
		logger.Warn("behavioral recorder: delivery is failing; learning is degraded, cluster verdict unaffected", fields...)
	case observation.RecorderQueuePressure:
		logger.Warn("behavioral recorder: queue pressure; observations may be dropped", fields...)
	default:
		logger.Info("behavioral recorder: delivery health changed", fields...)
	}
}

// recorderUnavailable is the doctor-side class for "this process has no
// recorder at all". Distinct from RecorderIdle: idle means a recorder exists
// and has been asked nothing, unavailable means nothing can ever be recorded.
// An operator acts differently on each.
const recorderUnavailable observation.RecorderHealth = "recorder_unavailable"

// maxRecorderErrorLen bounds the error echoed into the health surface. An
// unbounded remote error string reaching a log line is the shape
// meta.diagnostic_output_must_be_bounded warns about: one failure becoming a
// diagnosis harder to read than the disease.
const maxRecorderErrorLen = 240

func truncateRecorderError(s string) string {
	if len(s) <= maxRecorderErrorLen {
		return s
	}
	return s[:maxRecorderErrorLen] + "… (truncated)"
}
