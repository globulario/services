package main

import (
	"fmt"
	"log"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// Additional phase constants not yet in the proto package.
const (
	ReleasePhaseRemoving = "REMOVING" // Uninstall plans dispatched
	ReleasePhaseRemoved  = "REMOVED"  // All nodes confirmed removal; resource will be garbage-collected
)

// validPhaseTransitions defines the allowed phase transitions for a release.
// Key: current phase → Value: set of allowed target phases.
//
// NOTE: ReleasePhasePlanned ("PLANNED") is defined but intentionally excluded
// from the transition map. It is reserved for future batch/canary rollouts.
// Currently RESOLVED → APPLYING is atomic, so nothing produces PLANNED.
var validPhaseTransitions = map[string]map[string]bool{
	"": {
		cluster_controllerpb.ReleasePhasePending:  true,
		cluster_controllerpb.ReleasePhaseResolved: true,
		cluster_controllerpb.ReleasePhaseFailed:   true, // immediate resolution failure
		ReleasePhaseRemoving:                      true,
	},
	cluster_controllerpb.ReleasePhasePending: {
		cluster_controllerpb.ReleasePhaseResolved: true,
		cluster_controllerpb.ReleasePhaseFailed:   true,
		ReleasePhaseRemoving:                      true,
	},
	cluster_controllerpb.ReleasePhaseResolved: {
		cluster_controllerpb.ReleasePhaseApplying: true,
		cluster_controllerpb.ReleasePhaseFailed:   true,
		ReleasePhaseRemoving:                      true,
	},
	cluster_controllerpb.ReleasePhaseApplying: {
		cluster_controllerpb.ReleasePhaseAvailable:  true,
		cluster_controllerpb.ReleasePhaseDegraded:   true,
		cluster_controllerpb.ReleasePhaseFailed:     true,
		cluster_controllerpb.ReleasePhaseRolledBack: true,
		ReleasePhaseRemoving:                        true,
	},
	cluster_controllerpb.ReleasePhaseAvailable: {
		cluster_controllerpb.ReleasePhasePending:  true, // drift re-resolve
		cluster_controllerpb.ReleasePhaseDegraded: true, // drift detected, some nodes unhealthy
		cluster_controllerpb.ReleasePhaseFailed:   true, // drift detected, all nodes unhealthy
		ReleasePhaseRemoving:                      true,
	},
	cluster_controllerpb.ReleasePhaseDegraded: {
		cluster_controllerpb.ReleasePhasePending:   true,
		cluster_controllerpb.ReleasePhaseAvailable: true,
		cluster_controllerpb.ReleasePhaseFailed:    true,
		ReleasePhaseRemoving:                       true,
	},
	cluster_controllerpb.ReleasePhaseFailed: {
		cluster_controllerpb.ReleasePhasePending: true, // re-apply
		ReleasePhaseRemoving:                     true,
	},
	cluster_controllerpb.ReleasePhaseRolledBack: {
		cluster_controllerpb.ReleasePhasePending: true, // re-apply
		ReleasePhaseRemoving:                     true,
	},
	ReleasePhaseRemoving: {
		ReleasePhaseRemoved:                     true,
		cluster_controllerpb.ReleasePhaseFailed: true,
	},
	// REMOVED is terminal — no outgoing transitions.
	ReleasePhaseRemoved: {},
}

// advancePhase validates that the transition from current to target is allowed.
// Returns nil if valid, error if invalid.
func advancePhase(current, target string) error {
	if current == target {
		return nil // no-op transitions are always allowed
	}
	allowed, ok := validPhaseTransitions[current]
	if !ok {
		return fmt.Errorf("unknown current phase %q", current)
	}
	if !allowed[target] {
		return fmt.Errorf("invalid phase transition %q → %q", current, target)
	}
	return nil
}

// emitPhaseTransition validates a phase transition, emits a cluster event, and
// returns an error on invalid transitions (hard enforcement). The event is
// always emitted for audit, even when the transition is invalid.
func (srv *server) emitPhaseTransition(releaseName, from, to, reason string) error {
	if from == to {
		return nil
	}
	transitionErr := advancePhase(from, to)
	if transitionErr != nil {
		log.Printf("release %s: BLOCKED: %v", releaseName, transitionErr)
	}

	severity := "INFO"
	switch to {
	case cluster_controllerpb.ReleasePhaseFailed, cluster_controllerpb.ReleasePhaseRolledBack:
		severity = "ERROR"
	case cluster_controllerpb.ReleasePhaseDegraded:
		severity = "WARN"
	case ReleasePhaseRemoving:
		severity = "WARN"
	}

	eventData := map[string]interface{}{
		"service":        releaseName,
		"from_phase":     from,
		"to_phase":       to,
		"reason":         reason,
		"severity":       severity,
		"correlation_id": fmt.Sprintf("release:%s", releaseName),
	}
	if transitionErr != nil {
		eventData["blocked"] = true
		eventData["error"] = transitionErr.Error()
	}
	srv.emitClusterEvent("service.phase_changed", eventData)

	return transitionErr
}
