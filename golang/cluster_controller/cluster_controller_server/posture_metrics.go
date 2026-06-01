package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// posture_metrics.go: Prometheus metrics for the cluster posture layer.
//
// postureStateGauge is a gauge vector rather than a single gauge so dashboards
// can graph which posture state is active without string parsing:
//
//	globular_controller_posture_state{posture="NORMAL"}       1 or 0
//	globular_controller_posture_state{posture="DEGRADED"}     1 or 0
//	globular_controller_posture_state{posture="RECOVERY_ONLY"} 1 or 0
//
// Exactly one label value will be 1 at any time.

var (
	// postureStateGauge is 1 for the active posture and 0 for others.
	// Use sum(postureStateGauge) == 1 as a sanity check in alerting rules.
	postureStateGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "posture_state",
		Help:      "Current cluster posture: 1 for the active state, 0 for others.",
	}, []string{"posture"})

	// postureTransitionsTotal counts transitions between posture states.
	postureTransitionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "posture_transitions_total",
		Help:      "Total cluster posture state transitions.",
	}, []string{"from", "to"})

	// postureSignalActive is 1 when the named signal is currently active.
	// Useful for dashboards and for understanding which signal drove a posture change.
	postureSignalActive = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "posture_signal_active",
		Help:      "1 when the named posture input signal is currently active.",
	}, []string{"signal"})

	// postureNodeFraction tracks the fraction of known nodes that are unreachable.
	// Observational only — not yet wired to posture transitions.
	postureNodeFraction = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "posture_node_fraction",
		Help:      "Fraction of known nodes in the given health state (observational).",
	}, []string{"state"}) // state: "known", "unreachable"

	// postureGateSuppressedTotal counts workflow dispatches suppressed by the
	// posture enforcement gate. Labels identify the active posture and the
	// workload class that was blocked — useful for correlating suppression
	// events with posture transitions in dashboards.
	postureGateSuppressedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "posture_gate_suppressed_total",
		Help:      "Workflow dispatches suppressed by the posture enforcement gate.",
	}, []string{"posture", "class"})
)
