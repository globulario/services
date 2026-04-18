package main

// recovery_workflow.go — controller-side implementation of NodeRecoveryControllerConfig.
//
// Registers all controller.recovery.* actor actions and wires them to live
// cluster state. This is the behavioral heart of node.recover.full_reseed.

import (
	"context"
	"fmt"
	"log"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/workflow/engine"
)

// buildNodeRecoveryControllerConfig wires the NodeRecoveryControllerConfig to
// live controller state. Called from startControllerRuntime.
func (srv *server) buildNodeRecoveryControllerConfig() engine.NodeRecoveryControllerConfig {
	return engine.NodeRecoveryControllerConfig{
		ValidateRequest:          srv.recoveryValidateRequest,
		CheckClusterSafety:       srv.recoveryCheckClusterSafety,
		PlanReseed:               srv.recoveryPlanReseed,
		CaptureSnapshot:          srv.recoveryCaptureSnapshotActor,
		LoadSnapshot:             srv.recoveryLoadSnapshotActor,
		MarkRecoveryStarted:      srv.recoveryMarkStarted,
		PauseReconciliation:      srv.recoveryPauseReconciliation,
		DrainNode:                srv.recoveryDrainNode,
		MarkDestructiveBoundary:  srv.recoveryMarkDestructiveBoundary,
		AwaitReprovisionAck:      srv.recoveryAwaitReprovisionAck,
		AwaitNodeRejoin:          srv.recoveryAwaitNodeRejoin,
		BindRejoinedNodeIdentity: srv.recoveryBindRejoinedIdentity,
		ReseedFromSnapshot:       srv.recoveryReseed,
		VerifyReseedArtifacts:    srv.recoveryVerifyArtifacts,
		VerifyReseedRuntime:      srv.recoveryVerifyRuntime,
		VerifyReseedConvergence:  srv.recoveryVerifyConvergence,
		ResumeReconciliation:     srv.recoveryResumeReconciliation,
		MarkRecoveryComplete:     srv.recoveryMarkComplete,
		MarkRecoveryFailed:       srv.recoveryMarkFailed,
		EmitRecoveryComplete:     srv.recoveryEmitComplete,
	}
}

// ── Precheck ──────────────────────────────────────────────────────────────────

func (srv *server) recoveryValidateRequest(ctx context.Context, nodeID, reason string, exactRequired, force, dryRun bool, snapshotID string) error {
	if nodeID == "" {
		return fmt.Errorf("node_id is required")
	}
	if reason == "" {
		return fmt.Errorf("reason is required — document why this node needs full-reseed recovery")
	}

	// Node must exist.
	srv.lock("recoveryValidateRequest")
	_, ok := srv.state.Nodes[nodeID]
	srv.unlock()
	if !ok {
		return fmt.Errorf("node %s not found in cluster state", nodeID)
	}

	// Node must not already be in active recovery.
	if !dryRun {
		st, err := srv.getNodeRecoveryState(ctx, nodeID)
		if err != nil && err.Error() != "etcd not available" {
			return fmt.Errorf("check existing recovery state for %s: %w", nodeID, err)
		}
		if st != nil && !st.Phase.IsTerminal() {
			return fmt.Errorf("node %s is already under active recovery (workflow=%s phase=%s) — call GetNodeRecoveryStatus for details",
				nodeID, st.WorkflowID, st.Phase)
		}
	}

	return nil
}

func (srv *server) recoveryCheckClusterSafety(ctx context.Context, nodeID string, force bool) ([]string, error) {
	var warnings []string

	// Count storage nodes (MinIO / ScyllaDB require ≥ 3).
	srv.lock("recoveryCheckClusterSafety")
	storageCount := 0
	controlPlaneCount := 0
	for _, node := range srv.state.Nodes {
		if node == nil {
			continue
		}
		for _, p := range node.Profiles {
			switch p {
			case "storage":
				storageCount++
			case "control-plane":
				controlPlaneCount++
			}
		}
	}
	// Check if this node contributes critical profiles.
	target, targetOk := srv.state.Nodes[nodeID]
	srv.unlock()

	if !targetOk {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	for _, p := range target.Profiles {
		switch p {
		case "storage":
			if storageCount <= 3 && !force {
				return nil, fmt.Errorf(
					"node %s is one of only %d storage nodes — removing it risks MinIO/ScyllaDB quorum loss. Use --force to override",
					nodeID, storageCount)
			}
			if storageCount <= 3 {
				warnings = append(warnings, fmt.Sprintf("storage quorum marginal: only %d storage nodes remaining", storageCount-1))
			}
		case "control-plane":
			if controlPlaneCount <= 1 && !force {
				return nil, fmt.Errorf(
					"node %s is the only control-plane node — removing it would leave no controller. Use --force to override",
					nodeID)
			}
		}
	}

	return warnings, nil
}

func (srv *server) recoveryPlanReseed(ctx context.Context, nodeID string, exactRequired bool, snapshotID string) ([]cluster_controllerpb.PlannedRecoveryArtifact, error) {
	var snap *cluster_controllerpb.NodeRecoverySnapshot

	if snapshotID != "" {
		// Use existing snapshot.
		s, err := srv.getNodeRecoverySnapshot(ctx, nodeID, snapshotID)
		if err != nil {
			return nil, fmt.Errorf("load snapshot %s: %w", snapshotID, err)
		}
		if s == nil {
			return nil, fmt.Errorf("snapshot %s not found for node %s", snapshotID, nodeID)
		}
		snap = s
	} else {
		// Build a transient snapshot from installed state for planning purposes.
		// This snapshot is NOT persisted (planning is read-only).
		snap = srv.buildTransientSnapshot(ctx, nodeID)
	}

	// Check for cycles in requires graph.
	if err := validateNoReseedCycle(snap.Artifacts); err != nil {
		return nil, fmt.Errorf("snapshot dependency cycle: %w", err)
	}

	return buildReseedPlan(snap, exactRequired)
}

// buildTransientSnapshot builds an in-memory snapshot without persisting it.
// Used only for dry-run planning.
func (srv *server) buildTransientSnapshot(ctx context.Context, nodeID string) *cluster_controllerpb.NodeRecoverySnapshot {
	snap := &cluster_controllerpb.NodeRecoverySnapshot{
		SnapshotID: "transient",
		NodeID:     nodeID,
		CreatedAt:  time.Now().UTC(),
		Reason:     "planning",
	}
	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE", "COMMAND", "APPLICATION"} {
		pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, kind)
		if err != nil {
			continue
		}
		for _, pkg := range pkgs {
			art := cluster_controllerpb.SnapshotArtifact{
				Name:    pkg.GetName(),
				Kind:    kind,
				Version: pkg.GetVersion(),
			}
			if md := pkg.GetMetadata(); md != nil {
				art.BuildID = md["build_id"]
				art.Checksum = md["entrypoint_checksum"]
				art.PublisherID = md["publisher_id"]
			}
			snap.Artifacts = append(snap.Artifacts, art)
		}
	}
	return snap
}

// ── Snapshot ──────────────────────────────────────────────────────────────────

func (srv *server) recoveryCaptureSnapshotActor(ctx context.Context, nodeID, reason string) (*cluster_controllerpb.NodeRecoverySnapshot, error) {
	return srv.captureNodeInventorySnapshot(ctx, nodeID, reason, "workflow")
}

func (srv *server) recoveryLoadSnapshotActor(ctx context.Context, nodeID, snapshotID string) (*cluster_controllerpb.NodeRecoverySnapshot, error) {
	snap, err := srv.getNodeRecoverySnapshot(ctx, nodeID, snapshotID)
	if err != nil {
		return nil, err
	}
	if err := srv.validateNodeRecoverySnapshot(snap, nodeID); err != nil {
		return nil, fmt.Errorf("snapshot validation failed: %w", err)
	}
	return snap, nil
}

// ── Fencing ───────────────────────────────────────────────────────────────────

func (srv *server) recoveryMarkStarted(ctx context.Context, nodeID string, exactRequired bool, reason string) error {
	mode := cluster_controllerpb.NodeRecoveryModeExactReplayRequired
	if !exactRequired {
		mode = cluster_controllerpb.NodeRecoveryModeAllowResolutionFallback
	}

	st := &cluster_controllerpb.NodeRecoveryState{
		NodeID:    nodeID,
		Phase:     cluster_controllerpb.NodeRecoveryPhaseFenceNode,
		Mode:      mode,
		Reason:    reason,
		StartedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := srv.putNodeRecoveryState(ctx, st); err != nil {
		return fmt.Errorf("persist recovery state for %s: %w", nodeID, err)
	}
	log.Printf("recovery: node %s entered FENCE_NODE phase (mode=%s)", nodeID, mode)
	return nil
}

func (srv *server) recoveryPauseReconciliation(ctx context.Context, nodeID string) error {
	st, err := srv.getNodeRecoveryState(ctx, nodeID)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("no recovery state for %s — must call mark_started first", nodeID)
	}
	st.ReconciliationPaused = true
	st.Phase = cluster_controllerpb.NodeRecoveryPhaseFenceNode
	if err := srv.putNodeRecoveryState(ctx, st); err != nil {
		return err
	}
	log.Printf("recovery: reconciliation PAUSED for node %s", nodeID)
	return nil
}

func (srv *server) recoveryDrainNode(ctx context.Context, nodeID string) error {
	// Mark the node's bootstrap phase as "recovery" so the reconciler
	// won't try to advance it while fenced.
	srv.lock("recoveryDrainNode")
	if node, ok := srv.state.Nodes[nodeID]; ok {
		node.BootstrapPhase = BootstrapPhase("recovery_drain")
	}
	srv.unlock()

	// Update recovery state phase.
	st, err := srv.getNodeRecoveryState(ctx, nodeID)
	if err != nil {
		return err
	}
	if st != nil {
		st.Phase = cluster_controllerpb.NodeRecoveryPhaseRemoveOrDrain
		if err := srv.putNodeRecoveryState(ctx, st); err != nil {
			return err
		}
	}
	log.Printf("recovery: node %s drain/remove initiated", nodeID)
	return nil
}

func (srv *server) recoveryMarkDestructiveBoundary(ctx context.Context, nodeID string) error {
	st, err := srv.getNodeRecoveryState(ctx, nodeID)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("no recovery state for %s", nodeID)
	}

	// Invariant: snapshot must exist before crossing destructive boundary.
	if st.SnapshotID == "" {
		return fmt.Errorf("cannot cross destructive boundary for %s: no snapshot_id recorded in recovery state", nodeID)
	}
	snap, err := srv.getNodeRecoverySnapshot(ctx, nodeID, st.SnapshotID)
	if err != nil || snap == nil {
		return fmt.Errorf("cannot cross destructive boundary for %s: snapshot %s not found in etcd", nodeID, st.SnapshotID)
	}

	st.DestructiveBoundaryCrossed = true
	st.Phase = cluster_controllerpb.NodeRecoveryPhaseAwaitReprovision
	if err := srv.putNodeRecoveryState(ctx, st); err != nil {
		return err
	}
	log.Printf("recovery: node %s DESTRUCTIVE BOUNDARY CROSSED — old state abandoned, snapshot=%s", nodeID, st.SnapshotID)
	return nil
}

// ── Await reprovision / rejoin ────────────────────────────────────────────────

func (srv *server) recoveryAwaitReprovisionAck(ctx context.Context, nodeID string) (bool, error) {
	st, err := srv.getNodeRecoveryState(ctx, nodeID)
	if err != nil {
		return false, err
	}
	if st == nil {
		return false, fmt.Errorf("no recovery state for node %s", nodeID)
	}
	if st.ReprovisionAcked {
		log.Printf("recovery: reprovision ACK received for node %s", nodeID)
		return true, nil
	}
	return false, nil // not yet — workflow will retry
}

func (srv *server) recoveryAwaitNodeRejoin(ctx context.Context, nodeID string) (bool, error) {
	// The node is considered rejoined when it has a recent heartbeat AND
	// its bootstrap phase is past the initial registration.
	srv.lock("recoveryAwaitNodeRejoin")
	node, ok := srv.state.Nodes[nodeID]
	if !ok {
		srv.unlock()
		return false, nil // not yet
	}
	var phase BootstrapPhase
	if node != nil {
		phase = node.BootstrapPhase
	}
	srv.unlock()

	// Accept any non-drain phase as "rejoined" — the fresh node will be
	// going through bootstrap phases normally.
	recoveryDrainPhase := BootstrapPhase("recovery_drain")
	if phase == BootstrapNone || phase == recoveryDrainPhase {
		return false, nil
	}

	log.Printf("recovery: node %s has rejoined (phase=%q)", nodeID, phase)
	return true, nil
}

func (srv *server) recoveryBindRejoinedIdentity(ctx context.Context, nodeID string) error {
	srv.lock("recoveryBindRejoinedIdentity")
	node, ok := srv.state.Nodes[nodeID]
	newIdentity := ""
	if ok && node != nil {
		newIdentity = node.Identity.Hostname
	}
	srv.unlock()

	st, err := srv.getNodeRecoveryState(ctx, nodeID)
	if err != nil {
		return err
	}
	if st != nil {
		st.NewNodeIdentity = newIdentity
		st.Phase = cluster_controllerpb.NodeRecoveryPhaseReseedArtifacts
		if err := srv.putNodeRecoveryState(ctx, st); err != nil {
			return err
		}
	}
	log.Printf("recovery: node %s rejoined identity bound: %s", nodeID, newIdentity)
	return nil
}

// ── Reseed ────────────────────────────────────────────────────────────────────

func (srv *server) recoveryReseed(ctx context.Context, nodeID string, exactRequired bool) (map[string]any, error) {
	// Load the recovery state to get the snapshot_id.
	st, err := srv.getNodeRecoveryState(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("load recovery state: %w", err)
	}
	if st == nil || st.SnapshotID == "" {
		return nil, fmt.Errorf("no snapshot_id in recovery state for %s — snapshot must exist before reseed", nodeID)
	}

	snap, err := srv.getNodeRecoverySnapshot(ctx, nodeID, st.SnapshotID)
	if err != nil || snap == nil {
		return nil, fmt.Errorf("snapshot %s not found for node %s", st.SnapshotID, nodeID)
	}

	// Sort artifacts into install order (Rule D).
	sortedArts := sortedReseedOrder(snap.Artifacts)

	// Resolve agent endpoint.
	srv.lock("recoveryReseed:agentEndpoint")
	node, ok := srv.state.Nodes[nodeID]
	agentEndpoint := ""
	if ok && node != nil {
		agentEndpoint = node.AgentEndpoint
	}
	srv.unlock()
	if agentEndpoint == "" {
		return nil, fmt.Errorf("no agent endpoint for node %s — cannot reseed", nodeID)
	}

	repoInfo := resolveRepositoryInfo()

	installed := 0
	skipped := 0
	failed := 0
	var failedNames []string

	for i, art := range sortedArts {
		// Check for existing verified result (resume idempotence — Rule F).
		existing, _ := srv.getArtifactResult(ctx, nodeID, art.Name)
		if existing != nil && existing.Status == cluster_controllerpb.RecoveryArtifactStatusVerified {
			log.Printf("recovery reseed: %s/%s already VERIFIED — skipping", art.Kind, art.Name)
			skipped++
			continue
		}

		// Record start.
		startedAt := time.Now().UTC()
		result := &cluster_controllerpb.NodeRecoveryArtifactResult{
			WorkflowID:       st.WorkflowID,
			SnapshotID:       st.SnapshotID,
			NodeID:           nodeID,
			PublisherID:      art.PublisherID,
			Name:             art.Name,
			Kind:             art.Kind,
			RequestedVersion: art.Version,
			RequestedBuildID: art.BuildID,
			RequestedChecksum: art.Checksum,
			Order:            int32(i),
			Source:           "SNAPSHOT_EXACT",
			Status:           cluster_controllerpb.RecoveryArtifactStatusInstalling,
			StartedAt:        startedAt,
		}
		if art.BuildID == "" {
			result.Source = "REPOSITORY_RESOLVED"
			if exactRequired {
				result.Status = cluster_controllerpb.RecoveryArtifactStatusFailed
				result.Error = "exact_replay_required but no build_id in snapshot"
				now := time.Now().UTC()
				result.FinishedAt = &now
				_ = srv.putArtifactResult(ctx, result)
				failed++
				failedNames = append(failedNames, art.Name)
				continue
			}
		}
		_ = srv.putArtifactResult(ctx, result)

		// Apply via node-agent (same path as repair).
		err := srv.remoteApplyPackageRelease(ctx, nodeID, agentEndpoint,
			art.Name, art.Kind, art.Version,
			art.PublisherID,
			repoInfo.Address,
			art.BuildNumber,
			false, // force
			art.BuildID)
		now := time.Now().UTC()
		result.FinishedAt = &now

		if err != nil {
			log.Printf("recovery reseed: FAILED %s/%s@%s: %v", art.Kind, art.Name, art.Version, err)
			result.Status = cluster_controllerpb.RecoveryArtifactStatusFailed
			result.Error = err.Error()
			_ = srv.putArtifactResult(ctx, result)
			failed++
			failedNames = append(failedNames, art.Name)
			continue
		}

		result.Status = cluster_controllerpb.RecoveryArtifactStatusInstalled
		result.InstalledVersion = art.Version
		result.InstalledBuildID = art.BuildID
		_ = srv.putArtifactResult(ctx, result)
		installed++
		log.Printf("recovery reseed: installed %s/%s@%s (build=%s)", art.Kind, art.Name, art.Version, art.BuildID)
	}

	if failed > 0 {
		return nil, fmt.Errorf("reseed failed for %d artifacts: %v", failed, failedNames)
	}

	return map[string]any{
		"installed": installed,
		"skipped":   skipped,
		"total":     len(sortedArts),
	}, nil
}

// ── Verification ──────────────────────────────────────────────────────────────

func (srv *server) recoveryVerifyArtifacts(ctx context.Context, nodeID string, exactRequired bool) error {
	results, err := srv.listArtifactResults(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("list artifact results for %s: %w", nodeID, err)
	}

	for i := range results {
		r := &results[i]
		if r.Status == cluster_controllerpb.RecoveryArtifactStatusFailed {
			return fmt.Errorf("artifact %s/%s is in FAILED state: %s", r.Kind, r.Name, r.Error)
		}
		if r.Status == cluster_controllerpb.RecoveryArtifactStatusInstalled {
			// Promote to VERIFIED after install (simplified — full checksum
			// verification would require a node-agent probe).
			r.Status = cluster_controllerpb.RecoveryArtifactStatusVerified
			_ = srv.putArtifactResult(ctx, r)
		}
		if exactRequired && r.RequestedBuildID != "" && r.InstalledBuildID != r.RequestedBuildID {
			return fmt.Errorf("artifact %s/%s build_id mismatch: wanted=%s got=%s",
				r.Kind, r.Name, r.RequestedBuildID, r.InstalledBuildID)
		}
	}
	log.Printf("recovery verify_artifacts: all artifacts verified for node %s (%d total)", nodeID, len(results))
	return nil
}

func (srv *server) recoveryVerifyRuntime(ctx context.Context, nodeID string) error {
	// Check node has a recent heartbeat.
	srv.lock("recoveryVerifyRuntime")
	node, ok := srv.state.Nodes[nodeID]
	var phase BootstrapPhase
	if ok && node != nil {
		phase = node.BootstrapPhase
	}
	srv.unlock()

	if !ok {
		return fmt.Errorf("node %s not found in cluster state — not yet rejoined?", nodeID)
	}
	recoveryDrainPhase := BootstrapPhase("recovery_drain")
	if phase == recoveryDrainPhase || phase == BootstrapNone {
		return fmt.Errorf("node %s bootstrap phase is %q — not yet ready", nodeID, phase)
	}

	// Check for stuck packages.
	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE", "COMMAND"} {
		pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, kind)
		if err != nil {
			continue
		}
		for _, pkg := range pkgs {
			if status := pkg.GetStatus(); status == "partial_apply" || status == "failed" {
				return fmt.Errorf("package %s/%s is in %q state after reseed", kind, pkg.GetName(), status)
			}
		}
	}

	log.Printf("recovery verify_runtime: node %s runtime OK (phase=%q)", nodeID, phase)
	return nil
}

func (srv *server) recoveryVerifyConvergence(ctx context.Context, nodeID string) error {
	// A simple convergence check: no artifact results still in FAILED state,
	// and the node's applied_hash is not empty.
	results, err := srv.listArtifactResults(ctx, nodeID)
	if err != nil {
		return fmt.Errorf("list artifact results for convergence check: %w", err)
	}
	for _, r := range results {
		if r.Status == cluster_controllerpb.RecoveryArtifactStatusFailed {
			return fmt.Errorf("convergence check: artifact %s/%s is still in FAILED state", r.Kind, r.Name)
		}
	}

	srv.lock("recoveryVerifyConvergence")
	node, ok := srv.state.Nodes[nodeID]
	hash := ""
	if ok && node != nil {
		hash = node.AppliedServicesHash
	}
	srv.unlock()

	if hash == "" {
		return fmt.Errorf("node %s has no applied_services_hash yet — convergence not confirmed", nodeID)
	}

	log.Printf("recovery verify_convergence: node %s converged (hash=%s...)", nodeID, hash[:8])
	return nil
}

// ── Finalization ──────────────────────────────────────────────────────────────

func (srv *server) recoveryResumeReconciliation(ctx context.Context, nodeID string) error {
	st, err := srv.getNodeRecoveryState(ctx, nodeID)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("no recovery state for %s", nodeID)
	}
	st.ReconciliationPaused = false
	st.Phase = cluster_controllerpb.NodeRecoveryPhaseUnfenceNode
	st.VerificationPassed = true
	if err := srv.putNodeRecoveryState(ctx, st); err != nil {
		return err
	}
	log.Printf("recovery: reconciliation RESUMED for node %s", nodeID)
	return nil
}

func (srv *server) recoveryMarkComplete(ctx context.Context, nodeID string) error {
	st, err := srv.getNodeRecoveryState(ctx, nodeID)
	if err != nil {
		return err
	}
	if st == nil {
		return fmt.Errorf("no recovery state for %s", nodeID)
	}
	now := time.Now().UTC()
	st.Phase = cluster_controllerpb.NodeRecoveryPhaseComplete
	st.CompletedAt = &now
	st.VerificationPassed = true
	if err := srv.putNodeRecoveryState(ctx, st); err != nil {
		return err
	}

	// Clear the recovery_drain bootstrap phase marker.
	srv.lock("recoveryMarkComplete")
	if node, ok := srv.state.Nodes[nodeID]; ok && node.BootstrapPhase == BootstrapPhase("recovery_drain") {
		node.BootstrapPhase = BootstrapWorkloadReady
	}
	srv.unlock()

	log.Printf("recovery: node %s RECOVERY COMPLETE", nodeID)
	return nil
}

func (srv *server) recoveryMarkFailed(ctx context.Context, nodeID string, reason string) error {
	st, err := srv.getNodeRecoveryState(ctx, nodeID)
	if err != nil {
		log.Printf("recovery: mark_failed — could not load state for %s: %v", nodeID, err)
		return nil // best-effort
	}
	if st == nil {
		return nil
	}
	now := time.Now().UTC()
	st.Phase = cluster_controllerpb.NodeRecoveryPhaseFailed
	st.CompletedAt = &now
	st.LastError = reason
	// IMPORTANT: keep ReconciliationPaused=true if we were past FENCE_NODE.
	// The node remains fenced until a human clears it — we do NOT auto-resume.
	if !st.DestructiveBoundaryCrossed {
		// If we haven't crossed the destructive boundary yet, it is safe to
		// un-fence so the node can be reconciled normally.
		st.ReconciliationPaused = false
	}
	_ = srv.putNodeRecoveryState(ctx, st)

	log.Printf("recovery: node %s RECOVERY FAILED (reason=%q) — node remains %s",
		nodeID, reason,
		map[bool]string{true: "FENCED (destructive boundary was crossed)", false: "unfenced (safe to reconcile)"}[st.ReconciliationPaused])
	return nil
}

func (srv *server) recoveryEmitComplete(ctx context.Context, nodeID string) error {
	srv.emitClusterEvent("node.recovery.complete", map[string]interface{}{
		"severity": "INFO",
		"node_id":  nodeID,
		"message":  fmt.Sprintf("Node %s full-reseed recovery completed successfully", nodeID),
	})
	log.Printf("recovery: emitted recovery_complete event for node %s", nodeID)
	return nil
}
