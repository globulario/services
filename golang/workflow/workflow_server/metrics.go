// @awareness namespace=globular.platform
// @awareness component=platform_workflow.server
// @awareness file_role=workflow_metrics_emission
// @awareness risk=medium
package main

import (
	"time"

	"github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Workflow service control-plane signals.
//
// Intent: observability.must_explain_cause_not_only_metric — metrics must
// include labels that allow operators to navigate to the workflow type that
// is failing. Labels are bounded-cardinality (workflow_name comes from a
// fixed set of workflow definitions).
var (
	workflowActiveRuns = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "active_runs",
		Help:      "Number of workflow runs currently executing on this instance.",
	})

	workflowRunStartTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "run_start_total",
		Help:      "Total workflow runs started on this instance.",
	}, []string{"workflow_name"})

	workflowRunFailTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "run_fail_total",
		Help:      "Total workflow runs that failed on this instance.",
	}, []string{"workflow_name"})

	workflowRunBlockedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "run_blocked_total",
		Help:      "Total workflow runs that entered BLOCKED state (awaiting approval).",
	}, []string{"workflow_name"})

	workflowRunSuccessTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "workflow",
		Name:      "run_success_total",
		Help:      "Total workflow runs that succeeded on this instance.",
	}, []string{"workflow_name"})

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

type activeRun struct {
	start        time.Time
	workflowName string
}

func (srv *server) metricsRunStart(runID, workflowName string, now time.Time) {
	srv.metricsMu.Lock()
	if srv.runStart == nil {
		srv.runStart = make(map[string]activeRun)
	}
	srv.runStart[runID] = activeRun{start: now, workflowName: workflowName}
	workflowActiveRuns.Set(float64(len(srv.runStart)))
	workflowRunStartTotal.WithLabelValues(workflowName).Inc()
	workflowLastRunStartUnix.Set(float64(now.Unix()))
	srv.metricsMu.Unlock()
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
	ar := srv.runStart[runID]
	delete(srv.runStart, runID)
	active := len(srv.runStart)
	srv.metricsMu.Unlock()

	wfName := ar.workflowName

	workflowActiveRuns.Set(float64(active))
	workflowLastRunFinishUnix.Set(float64(now.Unix()))

	switch status {
	case workflowpb.RunStatus_RUN_STATUS_FAILED:
		workflowRunFailTotal.WithLabelValues(wfName).Inc()
	case workflowpb.RunStatus_RUN_STATUS_BLOCKED:
		workflowRunBlockedTotal.WithLabelValues(wfName).Inc()
	case workflowpb.RunStatus_RUN_STATUS_SUCCEEDED:
		workflowRunSuccessTotal.WithLabelValues(wfName).Inc()
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
	for _, ar := range srv.runStart {
		if ar.start.Before(oldest) {
			oldest = ar.start
		}
	}
	age := now.Sub(oldest).Seconds()
	workflowOldestActiveAgeSeconds.Set(age)
}
