package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Reconcile storm metrics — expose queue dynamics so operators can validate
// that restarts don't cause a full-cluster apply storm.
//
// Key signals:
//   - queue depth over time (gauge)
//   - admitted/resolved/dispatched release counts (counters by phase)
//   - watch-triggered re-enqueue count (counter)
//   - convergence filter suppression count (counter)
var (
	// controllerLoopHeartbeatUnix marks the last time a reconcile worker
	// completed an item. Alerts can fire when this timestamp goes stale,
	// catching deadlocks or blocked queues.
	controllerLoopHeartbeatUnix = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "loop_heartbeat_unix",
		Help:      "Unix timestamp of the last completed reconcile loop iteration.",
	})

	// workflowActiveRuns tracks in-flight cluster.reconcile executions so
	// dashboards/alerts can spot a stuck workflow (never finishes) or lack
	// of activity (never starts).
	workflowActiveRuns = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "workflow_active_runs",
		Help:      "Number of active cluster.reconcile workflow runs.",
	})

	// reconcileQueueDepth is the current number of items pending in the work queue.
	reconcileQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "reconcile_queue_depth",
		Help:      "Current number of items pending in the reconcile work queue.",
	})

	// reconcileEnqueueTotal counts items enqueued into the work queue, by source.
	// Sources: "initial" (startup), "watch" (etcd watch), "bridge" (periodic),
	// "staggered" (staggered initial enqueue).
	reconcileEnqueueTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "reconcile_enqueue_total",
		Help:      "Total items enqueued into the reconcile work queue, by source.",
	}, []string{"source"})

	// reconcileProcessedTotal counts items processed (dequeued) from the work queue.
	reconcileProcessedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "reconcile_processed_total",
		Help:      "Total items dequeued and processed from the reconcile work queue.",
	})

	// releasePhaseTransitions counts release phase transitions by type and phase.
	releasePhaseTransitions = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "release_phase_transitions_total",
		Help:      "Total release phase transitions, by resource type and target phase.",
	}, []string{"resource_type", "phase"})

	// releaseResolveDuration tracks the time spent in repository resolve calls.
	releaseResolveDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "release_resolve_duration_seconds",
		Help:      "Time spent resolving release versions from the repository.",
		Buckets:   prometheus.ExponentialBuckets(0.1, 2, 8), // 0.1s to 25.6s
	})

	// workflowDispatchTotal counts workflow dispatches by kind (install/upgrade/remove).
	workflowDispatchTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "workflow_dispatch_total",
		Help:      "Total workflow dispatches, by kind (install, upgrade, remove).",
	}, []string{"kind"})

	// convergenceFilterSuppressed counts services suppressed by the convergence
	// filter during startup (already converged, no work needed).
	convergenceFilterSuppressed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "convergence_filter_suppressed_total",
		Help:      "Services suppressed at startup because they were already converged.",
	})

	// reconcileDroppedNotLeader counts reconcile items dropped because this
	// instance is not the leader.
	reconcileDroppedNotLeader = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "reconcile_dropped_not_leader_total",
		Help:      "Reconcile items dropped because this instance is not the leader.",
	})

	// clusterReconcileSkippedTotal counts periodic reconcile ticks skipped
	// because a previous run is still active. Tracks coalescing behavior.
	clusterReconcileSkippedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "cluster_reconcile_skipped_total",
		Help:      "Cluster reconcile ticks skipped because a previous run is still active.",
	}, []string{"source"})

	// driftKindMismatchTotal counts desired-state entries blocked because
	// the artifact kind in the repository does not match the desired kind.
	driftKindMismatchTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "drift_kind_mismatch_total",
		Help:      "Desired-state entries blocked because artifact kind in repo does not match desired kind.",
	})

	// workflowCircuitBreakerOpenTotal counts how many times the workflow
	// dispatch circuit breaker has opened due to repeated RPC failures.
	workflowCircuitBreakerOpenTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "workflow_circuit_breaker_open_total",
		Help:      "Times the workflow dispatch circuit breaker opened.",
	})

	// workflowDispatchRejectedTotal counts workflow dispatches rejected
	// by the health gate while the circuit breaker is open.
	workflowDispatchRejectedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "workflow_dispatch_rejected_total",
		Help:      "Workflow dispatches rejected by the health gate circuit breaker.",
	})

	// reconcileCircuitOpenTotal counts times the reconcile circuit breaker
	// opened or rejected a dispatch due to repeated reconcile failures.
	reconcileCircuitOpenTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "reconcile_circuit_open_total",
		Help:      "Times the reconcile circuit breaker opened or rejected dispatch.",
	})

	// applyLoopDetectedTotal counts packages quarantined due to repeated
	// apply loops without convergence.
	applyLoopDetectedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "apply_loop_detected_total",
		Help:      "Packages quarantined due to repeated apply loops without convergence.",
	})

	// dispatchDedupSuppressedTotal counts dispatches suppressed by the
	// cross-path dedup registry (drift reconciler vs release pipeline).
	dispatchDedupSuppressedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "dispatch_dedup_suppressed_total",
		Help:      "Dispatches suppressed by cross-path dedup registry.",
	}, []string{"source", "held_by"})
)
