package main

import (
	"time"

	"github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Workflow service control-plane signals. Kept intentionally low-cardinality.
var (
	workflowActiveRuns = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "active_runs",
		Help:      "Number of workflow runs currently executing on this instance.",
	})

	workflowRunStartTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "run_start_total",
		Help:      "Total workflow runs started on this instance.",
	})

	workflowRunFailTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "run_fail_total",
		Help:      "Total workflow runs that failed on this instance.",
	})

	workflowRunBlockedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "run_blocked_total",
		Help:      "Total workflow runs that entered BLOCKED state (awaiting approval).",
	})

	workflowLastRunStartUnix = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "last_run_start_unix",
		Help:      "Unix timestamp of the most recent workflow run start.",
	})

	workflowLastRunFinishUnix = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "last_run_finish_unix",
		Help:      "Unix timestamp of the most recent workflow run finish (any status).",
	})

	workflowLastStepUnix = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "last_step_unix",
		Help:      "Unix timestamp of the most recent completed step across all runs.",
	})

	workflowOldestActiveAgeSeconds = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "oldest_active_age_seconds",
		Help:      "Age in seconds of the oldest active workflow run on this instance (0 if none).",
	})
)

func (srv *server) metricsRunStart(runID string, now time.Time) {
	srv.metricsMu.Lock()
	defer srv.metricsMu.Unlock()
	if srv.runStart == nil {
		srv.runStart = make(map[string]time.Time)
	}
	srv.runStart[runID] = now
	workflowActiveRuns.Set(float64(len(srv.runStart)))
	workflowRunStartTotal.Inc()
	workflowLastRunStartUnix.Set(float64(now.Unix()))
	srv.updateOldestActive(now)
}

func (srv *server) metricsStep(now time.Time) {
	workflowLastStepUnix.Set(float64(now.Unix()))
	srv.metricsMu.Lock()
	srv.lastStepUnix = now
	srv.metricsMu.Unlock()
}

func (srv *server) metricsRunFinish(runID string, status workflowpb.RunStatus, now time.Time) {
	srv.metricsMu.Lock()
	delete(srv.runStart, runID)
	active := len(srv.runStart)
	srv.metricsMu.Unlock()

	workflowActiveRuns.Set(float64(active))
	workflowLastRunFinishUnix.Set(float64(now.Unix()))

	switch status {
	case workflowpb.RunStatus_RUN_STATUS_FAILED:
		workflowRunFailTotal.Inc()
	case workflowpb.RunStatus_RUN_STATUS_BLOCKED:
		workflowRunBlockedTotal.Inc()
	}

	srv.updateOldestActive(now)
}

// updateOldestActive recomputes the age of the oldest active run.
func (srv *server) updateOldestActive(now time.Time) {
	srv.metricsMu.Lock()
	defer srv.metricsMu.Unlock()
	if len(srv.runStart) == 0 {
		workflowOldestActiveAgeSeconds.Set(0)
		return
	}
	oldest := now
	for _, t := range srv.runStart {
		if t.Before(oldest) {
			oldest = t
		}
	}
	age := now.Sub(oldest).Seconds()
	workflowOldestActiveAgeSeconds.Set(age)
}
