package main

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus control-plane signals exported by the node agent. These metrics
// stay intentionally small and self-contained so they remain available even
// during partial outages (e.g., controller unreachable).
var (
	// nodeAgentHeartbeatSuccessUnix is the Unix timestamp of the last successful
	// ReportStatus heartbeat to the cluster controller. Alert on age.
	nodeAgentHeartbeatSuccessUnix = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "node_agent",
		Name:      "heartbeat_success_unix",
		Help:      "Unix timestamp of the last successful heartbeat to the controller.",
	})

	// nodeAgentHeartbeatFailTotal counts failed heartbeat attempts (network,
	// auth, controller down, etc.). Use rate() for error budget alerting.
	nodeAgentHeartbeatFailTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "node_agent",
		Name:      "heartbeat_fail_total",
		Help:      "Total failed heartbeats to the controller.",
	})

	// nodeAgentControllerState indicates the current controller connectivity
	// state as an enumerated gauge. Values: connected=0, degraded=1,
	// rediscovering=2, unreachable=3.
	nodeAgentControllerState = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "node_agent",
		Name:      "controller_state",
		Help:      "Current controller connectivity state (0=connected,1=degraded,2=rediscovering,3=unreachable).",
	})

	// nodeAgentHeartbeatConsecutiveFailures tracks back-to-back failures to
	// highlight lingering link problems even if occasional successes occur.
	nodeAgentHeartbeatConsecutiveFailures = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "node_agent",
		Name:      "heartbeat_consecutive_failures",
		Help:      "Consecutive failed heartbeats to the controller.",
	})
)

func setControllerStateGauge(state ControllerConnState) {
	switch state {
	case ConnStateConnected:
		nodeAgentControllerState.Set(0)
	case ConnStateDegraded:
		nodeAgentControllerState.Set(1)
	case ConnStateRediscovering:
		nodeAgentControllerState.Set(2)
	case ConnStateUnreachable:
		nodeAgentControllerState.Set(3)
	default:
		nodeAgentControllerState.Set(-1)
	}
}

func recordHeartbeatSuccess(now time.Time) {
	nodeAgentHeartbeatSuccessUnix.Set(float64(now.Unix()))
	nodeAgentHeartbeatConsecutiveFailures.Set(0)
}

func recordHeartbeatFailure(consecutive int) {
	nodeAgentHeartbeatFailTotal.Inc()
	nodeAgentHeartbeatConsecutiveFailures.Set(float64(consecutive))
}
