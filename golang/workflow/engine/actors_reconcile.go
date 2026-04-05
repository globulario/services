package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// --------------------------------------------------------------------------
// Reconcile controller actions (cluster.reconcile workflow)
// --------------------------------------------------------------------------

// ReconcileControllerConfig provides dependencies for cluster reconciliation.
type ReconcileControllerConfig struct {
	AdvanceInfraJoins func(ctx context.Context, clusterID string) error
	ScanDrift       func(ctx context.Context, clusterID, scope string, includeNodes []any) ([]any, error)
	ClassifyDrift   func(ctx context.Context, driftReport []any, maxRemediations int) ([]any, error)
	FinalizeClean   func(ctx context.Context, clusterID string) error
	MarkItemStarted func(ctx context.Context, item map[string]any) error
	ChooseWorkflow  func(ctx context.Context, item map[string]any) (map[string]any, error)
	MarkItemTerminal func(ctx context.Context, item, childResult map[string]any) error
	MarkItemFailed  func(ctx context.Context, item map[string]any) error
	AggregateResults func(ctx context.Context) (map[string]any, error)
	Finalize        func(ctx context.Context, aggregate map[string]any) error
	MarkFailed      func(ctx context.Context) error
	EmitCompleted   func(ctx context.Context) error
}

// RegisterReconcileControllerActions registers reconcile controller handlers.
func RegisterReconcileControllerActions(router *Router, cfg ReconcileControllerConfig) {
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.advance_infra_joins", reconcileAdvanceInfraJoins(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.scan_drift", reconcileScanDrift(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.classify_drift", reconcileClassifyDrift(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.finalize_clean", reconcileFinalizeClean(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.mark_item_started", reconcileMarkItemStarted(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.choose_workflow", reconcileChooseWorkflow(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.mark_item_terminal", reconcileMarkItemTerminal(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.mark_item_failed", reconcileMarkItemFailed(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.aggregate_results", reconcileAggregateResults(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.finalize", reconcileFinalize(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.mark_failed", reconcileMarkFailed(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile.emit_completed", reconcileEmitCompleted(cfg))
}

func reconcileAdvanceInfraJoins(cfg ReconcileControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		clusterID := fmt.Sprint(req.With["cluster_id"])
		if clusterID == "" {
			clusterID = fmt.Sprint(req.Inputs["cluster_id"])
		}
		if cfg.AdvanceInfraJoins != nil {
			if err := cfg.AdvanceInfraJoins(ctx, clusterID); err != nil {
				return nil, fmt.Errorf("advance infra joins: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func reconcileScanDrift(cfg ReconcileControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		clusterID := fmt.Sprint(req.With["cluster_id"])
		scope := fmt.Sprint(req.With["scope"])
		includeNodes, _ := req.With["include_nodes"].([]any)
		if cfg.ScanDrift != nil {
			items, err := cfg.ScanDrift(ctx, clusterID, scope, includeNodes)
			if err != nil {
				return nil, fmt.Errorf("scan drift: %w", err)
			}
			req.Outputs["drift_report"] = items
			return &ActionResult{OK: true, Output: map[string]any{"count": len(items)}}, nil
		}
		req.Outputs["drift_report"] = []any{}
		return &ActionResult{OK: true, Output: map[string]any{"count": 0}}, nil
	}
}

func reconcileClassifyDrift(cfg ReconcileControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		driftReport, _ := req.With["drift_report"].([]any)
		if driftReport == nil {
			if dr, ok := req.Outputs["drift_report"].([]any); ok {
				driftReport = dr
			}
		}
		maxRem := 50
		if m, ok := req.With["max_remediations"].(int); ok {
			maxRem = m
		}
		if cfg.ClassifyDrift != nil {
			items, err := cfg.ClassifyDrift(ctx, driftReport, maxRem)
			if err != nil {
				return nil, fmt.Errorf("classify drift: %w", err)
			}
			req.Outputs["remediation_items"] = items
			return &ActionResult{OK: true, Output: map[string]any{"count": len(items)}}, nil
		}
		req.Outputs["remediation_items"] = []any{}
		return &ActionResult{OK: true, Output: map[string]any{"count": 0}}, nil
	}
}

func reconcileFinalizeClean(cfg ReconcileControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		clusterID := fmt.Sprint(req.Inputs["cluster_id"])
		log.Printf("actor[controller]: reconcile clean — no remediation needed for %s", clusterID)
		if cfg.FinalizeClean != nil {
			if err := cfg.FinalizeClean(ctx, clusterID); err != nil {
				return nil, fmt.Errorf("finalize clean: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func reconcileMarkItemStarted(cfg ReconcileControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		item, _ := req.With["item"].(map[string]any)
		if cfg.MarkItemStarted != nil {
			if err := cfg.MarkItemStarted(ctx, item); err != nil {
				return nil, fmt.Errorf("mark item started: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func reconcileChooseWorkflow(cfg ReconcileControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		item, _ := req.With["item"].(map[string]any)
		if cfg.ChooseWorkflow != nil {
			choice, err := cfg.ChooseWorkflow(ctx, item)
			if err != nil {
				return nil, fmt.Errorf("choose workflow: %w", err)
			}
			req.Outputs["workflow_choice"] = choice
			return &ActionResult{OK: true, Output: choice}, nil
		}
		return &ActionResult{OK: true}, nil
	}
}

func reconcileMarkItemTerminal(cfg ReconcileControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		item, _ := req.With["item"].(map[string]any)
		childResult, _ := req.With["child_result"].(map[string]any)
		if childResult == nil {
			if cr, ok := req.Outputs["child_result"].(map[string]any); ok {
				childResult = cr
			}
		}
		if cfg.MarkItemTerminal != nil {
			if err := cfg.MarkItemTerminal(ctx, item, childResult); err != nil {
				return nil, fmt.Errorf("mark item terminal: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func reconcileMarkItemFailed(cfg ReconcileControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		item, _ := req.With["item"].(map[string]any)
		log.Printf("actor[controller]: reconcile item FAILED: %v", item)
		if cfg.MarkItemFailed != nil {
			if err := cfg.MarkItemFailed(ctx, item); err != nil {
				return nil, fmt.Errorf("mark item failed: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func reconcileAggregateResults(cfg ReconcileControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.AggregateResults != nil {
			agg, err := cfg.AggregateResults(ctx)
			if err != nil {
				return nil, fmt.Errorf("aggregate results: %w", err)
			}
			return &ActionResult{OK: true, Output: agg}, nil
		}
		return &ActionResult{OK: true, Output: map[string]any{"status": "ok"}}, nil
	}
}

func reconcileFinalize(cfg ReconcileControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		aggregate, _ := req.With["aggregate"].(map[string]any)
		if aggregate == nil {
			if agg, ok := req.Outputs["aggregate"].(map[string]any); ok {
				aggregate = agg
			}
		}
		if cfg.Finalize != nil {
			if err := cfg.Finalize(ctx, aggregate); err != nil {
				return nil, fmt.Errorf("finalize reconcile: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func reconcileMarkFailed(cfg ReconcileControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.MarkFailed != nil {
			if err := cfg.MarkFailed(ctx); err != nil {
				return nil, fmt.Errorf("mark reconcile failed: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func reconcileEmitCompleted(cfg ReconcileControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.EmitCompleted != nil {
			if err := cfg.EmitCompleted(ctx); err != nil {
				return nil, fmt.Errorf("emit completed: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

// --------------------------------------------------------------------------
// Workflow service actions (child workflow orchestration)
// --------------------------------------------------------------------------

// WorkflowServiceConfig provides dependencies for workflow-service actor.
type WorkflowServiceConfig struct {
	StartChild        func(ctx context.Context, workflowName string, inputs map[string]any) (string, error)
	WaitChildTerminal func(ctx context.Context, childRunID string) (map[string]any, error)
}

// RegisterWorkflowServiceActions registers workflow-service actor handlers.
func RegisterWorkflowServiceActions(router *Router, cfg WorkflowServiceConfig) {
	router.Register(v1alpha1.ActorWorkflowService, "workflow.start_child", workflowStartChild(cfg))
	router.Register(v1alpha1.ActorWorkflowService, "workflow.wait_child_terminal", workflowWaitChildTerminal(cfg))
}

func workflowStartChild(cfg WorkflowServiceConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		workflowName := fmt.Sprint(req.With["workflow_name"])
		inputs, _ := req.With["inputs"].(map[string]any)
		if cfg.StartChild != nil {
			runID, err := cfg.StartChild(ctx, workflowName, inputs)
			if err != nil {
				return nil, fmt.Errorf("start child %s: %w", workflowName, err)
			}
			result := map[string]any{"run_id": runID, "workflow_name": workflowName}
			req.Outputs["child_run"] = result
			return &ActionResult{OK: true, Output: result}, nil
		}
		result := map[string]any{"run_id": "mock-run", "workflow_name": workflowName}
		req.Outputs["child_run"] = result
		return &ActionResult{OK: true, Output: result}, nil
	}
}

func workflowWaitChildTerminal(cfg WorkflowServiceConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		childRunID := fmt.Sprint(req.With["child_run_id"])
		if childRunID == "" {
			if cr, ok := req.Outputs["child_run"].(map[string]any); ok {
				childRunID = fmt.Sprint(cr["run_id"])
			}
		}
		if cfg.WaitChildTerminal != nil {
			result, err := cfg.WaitChildTerminal(ctx, childRunID)
			if err != nil {
				return nil, fmt.Errorf("wait child %s: %w", childRunID, err)
			}
			req.Outputs["child_result"] = result
			return &ActionResult{OK: true, Output: result}, nil
		}
		result := map[string]any{"status": "SUCCEEDED", "run_id": childRunID}
		req.Outputs["child_result"] = result
		return &ActionResult{OK: true, Output: result}, nil
	}
}
