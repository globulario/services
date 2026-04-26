package main

import (
	"context"
	"fmt"
	"log"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// bootstrapPhaseTimeout is the maximum time a node may spend in any single
// bootstrap phase before being marked as failed.
const bootstrapPhaseTimeout = 5 * time.Minute

// eventEmitter is the subset of server used by the bootstrap state machine.
type eventEmitter interface {
	emitClusterEvent(eventType string, data map[string]interface{})
	getWorkflowRecorder() *workflow.Recorder
}

// reconcileBootstrapPhases drives the bootstrap state machine for all nodes
// that have not yet reached workload_ready. It is called once per reconcile cycle.
//
// Phase transitions:
//
//	admitted        → infra_preparing  (immediate)
//	infra_preparing → etcd_joining     (etcd unit file present, or skip if no etcd profile)
//	etcd_joining    → etcd_ready       (EtcdJoinPhase == verified)
//	etcd_ready      → xds_ready        (globular-xds.service active, or skip)
//	xds_ready       → envoy_ready      (globular-envoy.service active, or skip)
//	envoy_ready     → workload_ready   (immediate)
//
// Phases are skipped when the node's profiles don't include the relevant service.
// Returns true if any node state was modified.
func reconcileBootstrapPhases(nodes []*nodeState, emitter eventEmitter) (dirty bool) {
	now := time.Now()

	for _, node := range nodes {
		if node == nil {
			continue
		}
		// Skip nodes that are fully done or legacy.
		if node.BootstrapPhase == BootstrapNone || node.BootstrapPhase == BootstrapWorkloadReady {
			continue
		}
		// Skip nodes whose bootstrap is being driven by the workflow engine.
		if node.BootstrapWorkflowActive {
			continue
		}
		// Auto-retry failed nodes: reset to admitted so the phase machine
		// re-evaluates. The conditions that caused the failure (e.g. missing
		// profile, DNS) may have been fixed since the failure.
		if node.BootstrapPhase == BootstrapFailed {
			log.Printf("bootstrap: node %s (%s) auto-retrying from bootstrap_failed",
				node.NodeID, node.Identity.Hostname)
			node.BootstrapPhase = BootstrapAdmitted
			node.BootstrapError = ""
			node.BootstrapStartedAt = now
			dirty = true
		}

		oldPhase := node.BootstrapPhase

		switch node.BootstrapPhase {
		case BootstrapAdmitted:
			// Immediate transition to infra_preparing.
			node.BootstrapPhase = BootstrapInfraPreparing
			node.BootstrapStartedAt = now
			node.BootstrapError = ""
			dirty = true

		case BootstrapInfraPreparing:
			// Wait for infrastructure packages to be installed.
			// Signal: etcd unit file present (for etcd-profiled nodes),
			// or skip etcd phases entirely if node has no etcd profile.
			if nodeHasEtcdProfile(node) {
				if nodeHasEtcdUnit(node) {
					node.BootstrapPhase = BootstrapEtcdJoining
					node.BootstrapStartedAt = now
					dirty = true
				} else if phaseTimedOut(node, now) {
					failBootstrap(node, "timeout waiting for etcd package installation")
					dirty = true
				}
			} else {
				// No etcd profile — skip etcd phases, advance to post-etcd.
				node.BootstrapPhase = advancePastEtcd(node, now)
				node.BootstrapStartedAt = now
				dirty = true
			}

		case BootstrapEtcdJoining:
			// Wait for the etcd join state machine to reach verified.
			// The etcd state machine (reconcileEtcdJoinPhases) runs separately.
			if node.EtcdJoinPhase == EtcdJoinVerified {
				node.BootstrapPhase = BootstrapEtcdReady
				node.BootstrapStartedAt = now
				dirty = true
			} else if node.EtcdJoinPhase == EtcdJoinFailed {
				failBootstrap(node, "etcd join failed: "+node.EtcdJoinError)
				dirty = true
			} else if phaseTimedOut(node, now) {
				failBootstrap(node, "timeout waiting for etcd join")
				dirty = true
			}

		case BootstrapEtcdReady:
			// etcd is verified. Wait for xDS to come up.
			if nodeHasXdsProfile(node) {
				if nodeHasUnitActive(node, "globular-xds.service") {
					node.BootstrapPhase = BootstrapXdsReady
					node.BootstrapStartedAt = now
					dirty = true
				} else if phaseTimedOut(node, now) {
					failBootstrap(node, "timeout waiting for xDS service")
					dirty = true
				}
			} else {
				// No xds profile — skip to envoy or workload.
				node.BootstrapPhase = advancePastXds(node, now)
				node.BootstrapStartedAt = now
				dirty = true
			}

		case BootstrapXdsReady:
			// xDS is active. Wait for Envoy.
			if nodeHasEnvoyProfile(node) {
				if nodeHasUnitActive(node, "globular-envoy.service") {
					node.BootstrapPhase = BootstrapEnvoyReady
					node.BootstrapStartedAt = now
					dirty = true
				} else if phaseTimedOut(node, now) {
					failBootstrap(node, "timeout waiting for Envoy service")
					dirty = true
				}
			} else {
				// No envoy/gateway profile — skip to storage or workload.
				node.BootstrapPhase = advancePastEnvoy(node)
				node.BootstrapStartedAt = now
				dirty = true
			}

		case BootstrapEnvoyReady:
			// Envoy is active — but do not advance until required infra runtime is
			// converged (active + fresh heartbeat). Installed-state alone is not
			// sufficient.
			if ok, reason := bootstrapRequiredInfraRuntimeConverged(node, now); !ok {
				node.BlockedReason = "day1_infra_runtime_blocked"
				node.BlockedDetails = reason
				node.BootstrapError = reason
				log.Printf("bootstrap: node %s (%s) runtime blocked at %s: %s",
					node.NodeID, node.Identity.Hostname, node.BootstrapPhase, reason)
				break
			}
			node.BlockedReason = ""
			node.BlockedDetails = ""
			if nodeNeedsStorageJoin(node) {
				node.BootstrapPhase = BootstrapStorageJoining
				node.BootstrapStartedAt = now
				node.BootstrapError = ""
				dirty = true
			} else {
				node.BootstrapPhase = BootstrapWorkloadReady
				node.BootstrapStartedAt = now
				node.BootstrapError = ""
				dirty = true
			}

		case BootstrapStorageJoining:
			// Verify storage services are active and healthy.
			// MinIO: globular-minio.service must be active.
			// ScyllaDB: ScyllaJoinPhase must be verified (gossip ring joined).
			allReady := true
			var waiting string

			if nodeHasMinioProfile(node) {
				if node.MinioJoinPhase != MinioJoinVerified && node.MinioJoinPhase != MinioJoinNonMember {
					allReady = false
					waiting = "globular-minio.service (join phase: " + string(node.MinioJoinPhase) + ")"
				}
			}
			if nodeHasScyllaProfile(node) {
				if node.ScyllaJoinPhase != ScyllaJoinVerified {
					allReady = false
					waiting = "scylla-server.service (join phase: " + string(node.ScyllaJoinPhase) + ")"
				}
			}

			if allReady {
				if ok, reason := bootstrapRequiredInfraRuntimeConverged(node, now); !ok {
					node.BlockedReason = "day1_infra_runtime_blocked"
					node.BlockedDetails = reason
					node.BootstrapError = reason
					log.Printf("bootstrap: node %s (%s) runtime blocked at %s: %s",
						node.NodeID, node.Identity.Hostname, node.BootstrapPhase, reason)
					break
				}
				node.BlockedReason = ""
				node.BlockedDetails = ""
				node.BootstrapPhase = BootstrapWorkloadReady
				node.BootstrapStartedAt = now
				node.BootstrapError = ""
				dirty = true
			} else if phaseTimedOut(node, now) {
				failBootstrap(node, "timeout waiting for storage service: "+waiting)
				dirty = true
			}
		}

		// Emit event + workflow step on phase transition.
		if node.BootstrapPhase != oldPhase && emitter != nil {
			log.Printf("bootstrap: node %s (%s) phase %s → %s",
				node.NodeID, node.Identity.Hostname, oldPhase, node.BootstrapPhase)
			emitter.emitClusterEvent("node.bootstrap_phase_changed", map[string]interface{}{
				"severity":       "INFO",
				"node_id":        node.NodeID,
				"hostname":       node.Identity.Hostname,
				"from_phase":     string(oldPhase),
				"to_phase":       string(node.BootstrapPhase),
				"correlation_id": "bootstrap:" + node.NodeID,
			})
			recordBootstrapTransition(emitter, node, oldPhase)
		}
	}

	return dirty
}

// phaseTimedOut returns true if the node has been in its current bootstrap
// phase longer than bootstrapPhaseTimeout.
func phaseTimedOut(node *nodeState, now time.Time) bool {
	if node.BootstrapStartedAt.IsZero() {
		return false
	}
	return now.Sub(node.BootstrapStartedAt) > bootstrapPhaseTimeout
}

// failBootstrap marks a node as bootstrap_failed with the given reason.
func failBootstrap(node *nodeState, reason string) {
	log.Printf("bootstrap: node %s (%s) failed: %s", node.NodeID, node.Identity.Hostname, reason)
	node.BootstrapPhase = BootstrapFailed
	node.BootstrapError = reason
}

// advancePastEtcd returns the next bootstrap phase after skipping etcd.
// Depends on whether the node has xds/envoy profiles.
func advancePastEtcd(node *nodeState, now time.Time) BootstrapPhase {
	if nodeHasXdsProfile(node) {
		return BootstrapEtcdReady // will wait for xDS in next cycle
	}
	return advancePastXds(node, now)
}

// advancePastXds returns the next phase after skipping xDS.
func advancePastXds(node *nodeState, now time.Time) BootstrapPhase {
	if nodeHasEnvoyProfile(node) {
		return BootstrapXdsReady // will wait for envoy in next cycle
	}
	return advancePastEnvoy(node)
}

// advancePastEnvoy returns the next phase after skipping Envoy.
func advancePastEnvoy(node *nodeState) BootstrapPhase {
	if nodeNeedsStorageJoin(node) {
		return BootstrapStorageJoining
	}
	return BootstrapWorkloadReady
}

// --- Profile and unit helpers ---

// nodeHasComponentProfile returns true if the node has any profile that
// includes the named component. This is catalog-driven: if the component
// exists in the catalog, its Profiles list is used; otherwise returns false.
func nodeHasComponentProfile(node *nodeState, componentName string) bool {
	comp := CatalogByName(componentName)
	if comp == nil {
		return false
	}
	return nodeHasProfile(&memberNode{Profiles: node.Profiles}, comp.Profiles)
}

// nodeHasEtcdProfile returns true if the node has a profile that runs etcd.
func nodeHasEtcdProfile(node *nodeState) bool {
	return nodeHasComponentProfile(node, "etcd")
}

// nodeHasXdsProfile returns true if the node has a profile that runs xDS.
func nodeHasXdsProfile(node *nodeState) bool {
	return nodeHasComponentProfile(node, "xds")
}

// nodeHasEnvoyProfile returns true if the node has a gateway profile (runs Envoy).
func nodeHasEnvoyProfile(node *nodeState) bool {
	return nodeHasComponentProfile(node, "envoy")
}

// nodeHasUnitActive returns true if the node reports the given unit as "active".
func nodeHasUnitActive(node *nodeState, unitName string) bool {
	for _, u := range node.Units {
		if u.Name == unitName && u.State == "active" {
			return true
		}
	}
	return false
}

// nodeHasMinioProfile returns true if the node has a profile that runs MinIO.
func nodeHasMinioProfile(node *nodeState) bool {
	return nodeHasComponentProfile(node, "minio")
}

// nodeHasScyllaProfile returns true if the node has a scylla or database profile.
func nodeHasScyllaProfile(node *nodeState) bool {
	return nodeHasComponentProfile(node, "scylladb")
}

// nodeNeedsStorageJoin returns true if the node hosts storage services
// (MinIO or ScyllaDB) that need explicit join verification.
func nodeNeedsStorageJoin(node *nodeState) bool {
	return nodeHasMinioProfile(node) || nodeHasScyllaProfile(node)
}

func requiredBootstrapInfraPackages(node *nodeState) []string {
	var out []string
	add := func(name string) {
		for _, s := range out {
			if s == name {
				return
			}
		}
		out = append(out, name)
	}
	for _, name := range []string{
		"etcd",
		"minio",
		"scylladb",
		"envoy",
		"gateway",
		"xds",
		"prometheus",
		"alertmanager",
		"node-exporter",
		"sidekick",
	} {
		if !nodeHasComponentProfile(node, name) {
			continue
		}
		// Non-pool-member nodes correctly hold MinIO inactive; skip the runtime
		// check so bootstrap is not permanently blocked at envoy_ready.
		if name == "minio" && node.MinioJoinPhase == MinioJoinNonMember {
			continue
		}
		add(name)
	}
	return out
}

func bootstrapRequiredInfraRuntimeConverged(node *nodeState, now time.Time) (bool, string) {
	for _, pkg := range requiredBootstrapInfraPackages(node) {
		pc := classifyPackageConvergence(
			node,
			pkg,
			"INFRASTRUCTURE",
			"",
			"",
			"",
			&node_agentpb.InstalledPackage{Version: "bootstrap-runtime-check"},
			now,
		)
		if !pc.RuntimeOK {
			// During bootstrap, tolerate "missing/unknown" runtime signals for
			// components that haven't reported unit state yet; hard-block only
			// on observed unhealthy states (inactive/failed/stale).
			if pc.RuntimeState == RuntimeMissing || pc.RuntimeState == RuntimeUnknown {
				continue
			}
			return false, fmt.Sprintf("Day1InfraRuntimeBlocked: %s (%s)", pkg, pc.Reason)
		}
	}
	return true, ""
}

// recordBootstrapTransition records the phase transition in a BOOTSTRAP
// workflow run. Creates the run on first transition (admitted → infra_preparing),
// records a step for each subsequent transition, and finishes the run on
// terminal states (workload_ready or bootstrap_failed).
func recordBootstrapTransition(emitter eventEmitter, node *nodeState, fromPhase BootstrapPhase) {
	rec := emitter.getWorkflowRecorder()
	if rec == nil {
		return
	}
	ctx := context.Background()

	// Start a new BOOTSTRAP run on the first transition.
	if node.BootstrapRunID == "" {
		node.BootstrapRunID = rec.StartRun(ctx, &workflow.RunParams{
			NodeID:        node.NodeID,
			NodeHostname:  node.Identity.Hostname,
			ReleaseKind:   "Bootstrap",
			TriggerReason: workflowpb.TriggerReason_TRIGGER_REASON_BOOTSTRAP,
			CorrelationID: fmt.Sprintf("bootstrap/%s", node.NodeID),
			WorkflowName:  "node.bootstrap",
		})
		if node.BootstrapRunID == "" {
			return
		}
	}

	runID := node.BootstrapRunID
	toPhase := node.BootstrapPhase

	// Record the phase transition as a workflow step.
	stepKey := fmt.Sprintf("bootstrap_%s", toPhase)
	title := fmt.Sprintf("Bootstrap: %s → %s", fromPhase, toPhase)
	stepStatus := workflow.StepSucceeded
	msg := fmt.Sprintf("node %s (%s)", node.NodeID, node.Identity.Hostname)

	if toPhase == BootstrapFailed {
		stepStatus = workflow.StepFailed
		msg = node.BootstrapError
	}

	stepSeq := rec.RecordStep(ctx, runID, &workflow.StepParams{
		StepKey: stepKey,
		Title:   title,
		Actor:   workflow.ActorController,
		Phase:   workflow.PhaseVerify,
		Status:  stepStatus,
		Message: msg,
	})

	// For non-terminal phases, mark step as completed immediately.
	if toPhase != BootstrapFailed && toPhase != BootstrapWorkloadReady {
		rec.CompleteStep(ctx, runID, stepSeq, msg, 0)
		return
	}

	// Terminal states: finish the run.
	if toPhase == BootstrapWorkloadReady {
		rec.FinishRun(ctx, runID, workflow.Succeeded,
			fmt.Sprintf("node %s (%s) bootstrap complete", node.NodeID, node.Identity.Hostname),
			"", workflow.NoFailure)
		node.BootstrapRunID = "" // clear for potential future re-bootstrap
	} else if toPhase == BootstrapFailed {
		rec.FailStep(ctx, runID, stepSeq, "bootstrap.failed", node.BootstrapError,
			"Check node connectivity, DNS, and service health", workflowpb.FailureClass_FAILURE_CLASS_DEPENDENCY, true)
		rec.FinishRun(ctx, runID, workflow.Failed,
			fmt.Sprintf("node %s (%s) bootstrap failed: %s", node.NodeID, node.Identity.Hostname, node.BootstrapError),
			node.BootstrapError, workflowpb.FailureClass_FAILURE_CLASS_DEPENDENCY)
		node.BootstrapRunID = "" // clear for auto-retry
	}
}
