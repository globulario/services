package engine

// actors_node_recovery.go — workflow actor handlers for node.recover.full_reseed.
//
// Each action in the workflow YAML maps to a registered handler here.
// All logic is expressed as callbacks injected via NodeRecoveryControllerConfig —
// this keeps the engine package free of cluster-controller implementation details.

import (
	"context"
	"fmt"
	"log"

	"github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// NodeRecoveryControllerConfig provides dependency injection for all
// node.recover.full_reseed actor actions. The cluster controller server
// populates this struct and calls RegisterNodeRecoveryControllerActions.
type NodeRecoveryControllerConfig struct {
	// Precheck
	ValidateRequest    func(ctx context.Context, nodeID, reason string, exactRequired, force, dryRun bool, snapshotID string) error
	CheckClusterSafety func(ctx context.Context, nodeID string, force bool) ([]string, error) // returns warnings

	// Snapshot
	CaptureSnapshot func(ctx context.Context, nodeID, reason string) (*cluster_controllerpb.NodeRecoverySnapshot, error)
	LoadSnapshot    func(ctx context.Context, nodeID, snapshotID string) (*cluster_controllerpb.NodeRecoverySnapshot, error)
	PlanReseed      func(ctx context.Context, nodeID string, exactRequired bool, snapshotID string) ([]cluster_controllerpb.PlannedRecoveryArtifact, error)

	// Fencing
	MarkRecoveryStarted    func(ctx context.Context, nodeID string, exactRequired bool, reason string) error
	PauseReconciliation    func(ctx context.Context, nodeID string) error
	DrainNode              func(ctx context.Context, nodeID string) error
	MarkDestructiveBoundary func(ctx context.Context, nodeID string) error

	// Reprovision / rejoin
	AwaitReprovisionAck      func(ctx context.Context, nodeID string) (bool, error) // returns (acked, err)
	AwaitNodeRejoin          func(ctx context.Context, nodeID string) (bool, error) // returns (rejoined, err)
	BindRejoinedNodeIdentity func(ctx context.Context, nodeID string) error

	// Reseed
	ReseedFromSnapshot func(ctx context.Context, nodeID string, exactRequired bool) (map[string]any, error)

	// Verification
	VerifyReseedArtifacts   func(ctx context.Context, nodeID string, exactRequired bool) error
	VerifyReseedRuntime     func(ctx context.Context, nodeID string) error
	VerifyReseedConvergence func(ctx context.Context, nodeID string) error

	// Finalization
	ResumeReconciliation    func(ctx context.Context, nodeID string) error
	MarkRecoveryComplete    func(ctx context.Context, nodeID string) error
	MarkRecoveryFailed      func(ctx context.Context, nodeID string, reason string) error
	EmitRecoveryComplete    func(ctx context.Context, nodeID string) error
}

// RegisterNodeRecoveryControllerActions registers all controller.recovery.*
// handlers into the workflow router.
func RegisterNodeRecoveryControllerActions(router *Router, cfg NodeRecoveryControllerConfig) {
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.validate_full_reseed_request", recoveryValidateRequest(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.check_cluster_safety", recoveryCheckClusterSafety(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.plan_full_reseed", recoveryPlanReseed(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.capture_node_inventory_snapshot", recoveryCaptureSnapshot(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.load_node_recovery_snapshot", recoveryLoadSnapshot(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.mark_node_recovery_started", recoveryMarkStarted(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.pause_node_reconciliation", recoveryPauseReconciliation(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.drain_node", recoveryDrainNode(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.mark_destructive_boundary", recoveryMarkDestructiveBoundary(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.await_node_reprovision_ack", recoveryAwaitReprovisionAck(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.await_node_rejoin", recoveryAwaitNodeRejoin(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.bind_rejoined_node_identity", recoveryBindRejoinedIdentity(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.reseed_node_from_snapshot", recoveryReseedFromSnapshot(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.verify_reseed_artifacts", recoveryVerifyArtifacts(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.verify_reseed_runtime", recoveryVerifyRuntime(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.verify_reseed_convergence", recoveryVerifyConvergence(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.resume_node_reconciliation", recoveryResumeReconciliation(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.mark_node_recovery_complete", recoveryMarkComplete(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.mark_node_recovery_failed", recoveryMarkFailed(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.recovery.emit_recovery_complete", recoveryEmitComplete(cfg))
}

// ── Actor handler implementations ─────────────────────────────────────────────

func recoveryValidateRequest(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		reason := fmt.Sprint(req.With["reason"])
		exactRequired, _ := req.With["exact_replay_required"].(bool)
		force, _ := req.With["force"].(bool)
		dryRun, _ := req.With["dry_run"].(bool)
		snapshotID := fmt.Sprint(req.With["snapshot_id"])
		if snapshotID == "<nil>" {
			snapshotID = ""
		}

		log.Printf("actor[recovery]: validate_request node=%s exact=%v force=%v dry_run=%v", nodeID, exactRequired, force, dryRun)
		if cfg.ValidateRequest != nil {
			if err := cfg.ValidateRequest(ctx, nodeID, reason, exactRequired, force, dryRun, snapshotID); err != nil {
				return nil, fmt.Errorf("validate recovery request: %w", err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{
			"node_id": nodeID,
			"dry_run": dryRun,
		}}, nil
	}
}

func recoveryCheckClusterSafety(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		force, _ := req.With["force"].(bool)

		log.Printf("actor[recovery]: check_cluster_safety node=%s force=%v", nodeID, force)
		var warnings []string
		if cfg.CheckClusterSafety != nil {
			w, err := cfg.CheckClusterSafety(ctx, nodeID, force)
			if err != nil {
				return nil, fmt.Errorf("cluster safety check for %s: %w", nodeID, err)
			}
			warnings = w
		}
		return &ActionResult{OK: true, Output: map[string]any{
			"warnings": warnings,
		}}, nil
	}
}

func recoveryPlanReseed(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		exactRequired, _ := req.With["exact_replay_required"].(bool)
		snapshotID := fmt.Sprint(req.With["snapshot_id"])
		if snapshotID == "<nil>" {
			snapshotID = ""
		}

		log.Printf("actor[recovery]: plan_full_reseed node=%s exact=%v snapshot=%q", nodeID, exactRequired, snapshotID)
		var planned []cluster_controllerpb.PlannedRecoveryArtifact
		if cfg.PlanReseed != nil {
			p, err := cfg.PlanReseed(ctx, nodeID, exactRequired, snapshotID)
			if err != nil {
				return nil, fmt.Errorf("plan reseed for %s: %w", nodeID, err)
			}
			planned = p
		}
		return &ActionResult{OK: true, Output: map[string]any{
			"planned_artifacts": planned,
			"artifact_count":    len(planned),
		}}, nil
	}
}

func recoveryCaptureSnapshot(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		reason := fmt.Sprint(req.With["reason"])

		log.Printf("actor[recovery]: capture_snapshot node=%s", nodeID)
		var snapID string
		if cfg.CaptureSnapshot != nil {
			snap, err := cfg.CaptureSnapshot(ctx, nodeID, reason)
			if err != nil {
				return nil, fmt.Errorf("capture snapshot for %s: %w", nodeID, err)
			}
			snapID = snap.SnapshotID
			req.Outputs["snapshot_id"] = snapID
			req.Outputs["snapshot_artifact_count"] = len(snap.Artifacts)
			req.Outputs["snapshot_exact_replay_possible"] = snap.ExactReplayPossible
		}
		return &ActionResult{OK: true, Output: map[string]any{
			"snapshot_id": snapID,
		}}, nil
	}
}

func recoveryLoadSnapshot(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		snapshotID := fmt.Sprint(req.With["snapshot_id"])

		log.Printf("actor[recovery]: load_snapshot node=%s snapshot=%s", nodeID, snapshotID)
		if cfg.LoadSnapshot != nil {
			snap, err := cfg.LoadSnapshot(ctx, nodeID, snapshotID)
			if err != nil {
				return nil, fmt.Errorf("load snapshot %s for %s: %w", snapshotID, nodeID, err)
			}
			if snap == nil {
				return nil, fmt.Errorf("snapshot %s not found for node %s", snapshotID, nodeID)
			}
			req.Outputs["snapshot_id"] = snap.SnapshotID
			req.Outputs["snapshot_artifact_count"] = len(snap.Artifacts)
		}
		return &ActionResult{OK: true, Output: map[string]any{
			"snapshot_id": snapshotID,
		}}, nil
	}
}

func recoveryMarkStarted(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		exactRequired, _ := req.With["exact_replay_required"].(bool)
		reason := fmt.Sprint(req.With["reason"])

		log.Printf("actor[recovery]: mark_recovery_started node=%s exact=%v", nodeID, exactRequired)
		if cfg.MarkRecoveryStarted != nil {
			if err := cfg.MarkRecoveryStarted(ctx, nodeID, exactRequired, reason); err != nil {
				return nil, fmt.Errorf("mark recovery started for %s: %w", nodeID, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func recoveryPauseReconciliation(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[recovery]: pause_reconciliation node=%s", nodeID)
		if cfg.PauseReconciliation != nil {
			if err := cfg.PauseReconciliation(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("pause reconciliation for %s: %w", nodeID, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func recoveryDrainNode(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[recovery]: drain_node node=%s", nodeID)
		if cfg.DrainNode != nil {
			if err := cfg.DrainNode(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("drain node %s: %w", nodeID, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func recoveryMarkDestructiveBoundary(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[recovery]: mark_destructive_boundary node=%s — old installed state ABANDONED", nodeID)
		if cfg.MarkDestructiveBoundary != nil {
			if err := cfg.MarkDestructiveBoundary(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("mark destructive boundary for %s: %w", nodeID, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func recoveryAwaitReprovisionAck(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[recovery]: await_reprovision_ack node=%s — polling for operator ACK", nodeID)
		if cfg.AwaitReprovisionAck != nil {
			acked, err := cfg.AwaitReprovisionAck(ctx, nodeID)
			if err != nil {
				return nil, fmt.Errorf("await reprovision ack for %s: %w", nodeID, err)
			}
			if !acked {
				// Signal the workflow engine to retry this step.
				return nil, fmt.Errorf("reprovision not yet acknowledged for node %s — retry", nodeID)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func recoveryAwaitNodeRejoin(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[recovery]: await_node_rejoin node=%s — waiting for fresh node", nodeID)
		if cfg.AwaitNodeRejoin != nil {
			rejoined, err := cfg.AwaitNodeRejoin(ctx, nodeID)
			if err != nil {
				return nil, fmt.Errorf("await rejoin for %s: %w", nodeID, err)
			}
			if !rejoined {
				return nil, fmt.Errorf("node %s has not rejoined yet — retry", nodeID)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func recoveryBindRejoinedIdentity(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[recovery]: bind_rejoined_identity node=%s", nodeID)
		if cfg.BindRejoinedNodeIdentity != nil {
			if err := cfg.BindRejoinedNodeIdentity(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("bind rejoined identity for %s: %w", nodeID, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func recoveryReseedFromSnapshot(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		exactRequired, _ := req.With["exact_replay_required"].(bool)

		log.Printf("actor[recovery]: reseed_from_snapshot node=%s exact=%v", nodeID, exactRequired)
		if cfg.ReseedFromSnapshot != nil {
			result, err := cfg.ReseedFromSnapshot(ctx, nodeID, exactRequired)
			if err != nil {
				return nil, fmt.Errorf("reseed node %s: %w", nodeID, err)
			}
			return &ActionResult{OK: true, Output: result}, nil
		}
		return &ActionResult{OK: true}, nil
	}
}

func recoveryVerifyArtifacts(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		exactRequired, _ := req.With["exact_replay_required"].(bool)

		log.Printf("actor[recovery]: verify_artifacts node=%s exact=%v", nodeID, exactRequired)
		if cfg.VerifyReseedArtifacts != nil {
			if err := cfg.VerifyReseedArtifacts(ctx, nodeID, exactRequired); err != nil {
				return nil, fmt.Errorf("artifact verification failed for %s: %w", nodeID, err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{"artifacts_verified": true}}, nil
	}
}

func recoveryVerifyRuntime(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[recovery]: verify_runtime node=%s", nodeID)
		if cfg.VerifyReseedRuntime != nil {
			if err := cfg.VerifyReseedRuntime(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("runtime verification failed for %s: %w", nodeID, err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{"runtime_verified": true}}, nil
	}
}

func recoveryVerifyConvergence(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[recovery]: verify_convergence node=%s", nodeID)
		if cfg.VerifyReseedConvergence != nil {
			if err := cfg.VerifyReseedConvergence(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("convergence verification failed for %s: %w", nodeID, err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{"convergence_verified": true}}, nil
	}
}

func recoveryResumeReconciliation(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[recovery]: resume_reconciliation node=%s", nodeID)
		if cfg.ResumeReconciliation != nil {
			if err := cfg.ResumeReconciliation(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("resume reconciliation for %s: %w", nodeID, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func recoveryMarkComplete(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[recovery]: mark_complete node=%s — RECOVERY COMPLETE", nodeID)
		if cfg.MarkRecoveryComplete != nil {
			if err := cfg.MarkRecoveryComplete(ctx, nodeID); err != nil {
				return nil, fmt.Errorf("mark recovery complete for %s: %w", nodeID, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func recoveryMarkFailed(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		reason := ""
		if e, ok := req.With["error"].(string); ok {
			reason = e
		}
		log.Printf("actor[recovery]: mark_failed node=%s reason=%q — node remains FENCED", nodeID, reason)
		if cfg.MarkRecoveryFailed != nil {
			if err := cfg.MarkRecoveryFailed(ctx, nodeID, reason); err != nil {
				log.Printf("actor[recovery]: error in mark_failed handler for %s: %v", nodeID, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func recoveryEmitComplete(cfg NodeRecoveryControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.With["node_id"])
		log.Printf("actor[recovery]: emit_recovery_complete node=%s", nodeID)
		if cfg.EmitRecoveryComplete != nil {
			if err := cfg.EmitRecoveryComplete(ctx, nodeID); err != nil {
				log.Printf("actor[recovery]: emit_recovery_complete for %s: %v (non-fatal)", nodeID, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}
