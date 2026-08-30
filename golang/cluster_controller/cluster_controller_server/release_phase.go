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
// Currently the workflow-native path is RESOLVED → AVAILABLE/DEGRADED/FAILED,
// and nothing produces PLANNED.
var validPhaseTransitions = map[string]map[string]bool{
	"": {
		cluster_controllerpb.ReleasePhasePending:  true,
		cluster_controllerpb.ReleasePhaseResolved: true,
		cluster_controllerpb.ReleasePhaseFailed:   true, // immediate resolution failure
		ReleasePhaseRemoving:                      true,
	},
	cluster_controllerpb.ReleasePhasePending: {
		cluster_controllerpb.ReleasePhaseResolved: true,
		cluster_controllerpb.ReleasePhaseWaiting:  true, // artifact not published — retry after backoff
		cluster_controllerpb.ReleasePhaseFailed:   true,
		ReleasePhaseRemoving:                      true,
	},
	cluster_controllerpb.ReleasePhaseWaiting: {
		cluster_controllerpb.ReleasePhasePending:  true, // retry after releaseWaitingBackoff
		cluster_controllerpb.ReleasePhaseResolved: true, // artifact appeared, resolve succeeded
		cluster_controllerpb.ReleasePhaseFailed:   true, // persistent resolve failure
		ReleasePhaseRemoving:                      true,
	},
	cluster_controllerpb.ReleasePhaseResolved: {
		cluster_controllerpb.ReleasePhaseAvailable:  true, // workflow finished: all nodes converged
		cluster_controllerpb.ReleasePhaseDegraded:   true, // workflow finished: some nodes failed
		cluster_controllerpb.ReleasePhaseDeferred:   true, // workflow found no dispatchable targets; retry later
		cluster_controllerpb.ReleasePhaseFailed:     true, // workflow finished: failure
		cluster_controllerpb.ReleasePhaseRolledBack: true,
		ReleasePhaseRemoving:                        true,
	},
	// APPLYING is a legacy label kept only for API boundary compatibility.
	// Internal code never writes this phase (use workflow run state instead).
	// Transitions out exist for databases that still contain legacy APPLYING rows.
	cluster_controllerpb.ReleasePhaseApplying: {
		cluster_controllerpb.ReleasePhaseAvailable:  true,
		cluster_controllerpb.ReleasePhaseDegraded:   true,
		cluster_controllerpb.ReleasePhaseDeferred:   true,
		cluster_controllerpb.ReleasePhaseFailed:     true,
		cluster_controllerpb.ReleasePhaseRolledBack: true,
		cluster_controllerpb.ReleasePhaseResolved:   true,
		ReleasePhaseRemoving:                        true,
	},
	cluster_controllerpb.ReleasePhaseDeferred: {
		cluster_controllerpb.ReleasePhasePending: true, // retry target selection after backoff
		cluster_controllerpb.ReleasePhaseFailed:  true,
		ReleasePhaseRemoving:                     true,
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

// emitPhaseTransition validates a phase transition and emits a cluster event.
// The event is always emitted for audit, even when the transition is invalid.
//
// It returns the validation error, but it does NOT itself enforce anything —
// enforcement depends entirely on what the caller does with that error, and the
// two caller families differ:
//
//   - release_reconciler.go (3 sites) enforces: it returns the error and
//     abandons the patch, so the write really is blocked.
//   - workflow_release.go patchReleasePhase does not: it has already assigned
//     rel.Status.Phase before calling here, discards the error, and persists
//     through applyWorkflowRelease. The write proceeds.
//
// So the same state machine is binding for one writer and advisory for the
// other. Do not read a rejected transition as "the phase did not change" without
// checking which path emitted it — see the wording of the log line below, which
// exists precisely because that misreading has already happened.
func (srv *server) emitPhaseTransition(releaseName, from, to, reason string) error {
	if from == to {
		return nil
	}
	transitionErr := advancePhase(from, to)
	if transitionErr != nil {
		// Deliberately not the word "BLOCKED". This function does not block
		// anything; only a caller that honours the returned error does, and one
		// of the two caller families does not. Logging "BLOCKED" here stated an
		// outcome this code cannot know, and it was read — by a reader working
		// only from these logs — as proof that writes were being dropped and
		// releases stranded. They were not: on the workflow path the phase had
		// already been assigned and was persisted immediately afterwards.
		//
		// Say what is actually true (this transition is not in the declared
		// model) and leave the consequence to the caller that decides it.
		log.Printf("release %s: phase transition not in declared model: %v "+
			"(enforcement is the caller's; this line alone does not mean the write was dropped)",
			releaseName, transitionErr)
	}

	severity := "INFO"
	switch to {
	case cluster_controllerpb.ReleasePhaseFailed, cluster_controllerpb.ReleasePhaseRolledBack:
		severity = "ERROR"
	case cluster_controllerpb.ReleasePhaseDegraded, cluster_controllerpb.ReleasePhaseDeferred:
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
