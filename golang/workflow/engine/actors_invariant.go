package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// --------------------------------------------------------------------------
// Invariant enforcement actions (cluster.invariant.enforcement workflow)
// --------------------------------------------------------------------------

// InvariantConfig provides dependencies for cluster invariant enforcement.
type InvariantConfig struct {
	// ValidateWorkflows checks that all required workflow definitions exist
	// in etcd. Returns a report with present/missing lists.
	ValidateWorkflows func(ctx context.Context, required []string) (map[string]any, error)

	// RepairWorkflows re-seeds missing workflow definitions from local disk
	// to etcd. Returns count of repaired workflows.
	RepairWorkflows func(ctx context.Context, missing []string) (int, error)

	// ValidateInfraQuorum checks that infrastructure quorum requirements are
	// met: etcd on all nodes, ScyllaDB ≥ minScylla, MinIO ≥ minMinio.
	// Returns a quorum report with violations and promotion candidates.
	ValidateInfraQuorum func(ctx context.Context, minScylla, minMinio int, etcdAllNodes bool) (map[string]any, error)

	// EnforceQuorum auto-promotes nodes to restore infrastructure quorum.
	EnforceQuorum func(ctx context.Context, quorumReport map[string]any) error

	// VerifyQuorum re-checks quorum after enforcement to confirm resolution.
	VerifyQuorum func(ctx context.Context, minScylla, minMinio int) (bool, error)

	// ValidateFoundingProfiles checks that founding nodes (first 3 by join
	// order) have the required profiles: core + control-plane + storage.
	ValidateFoundingProfiles func(ctx context.Context) (map[string]any, error)

	// ValidateMinioStorage checks MinIO distributed storage health:
	// pool membership, join phases, credentials, config consistency.
	ValidateMinioStorage func(ctx context.Context) (map[string]any, error)

	// RepairMinioStorage fixes MinIO violations: resets join phases,
	// clears config hashes to force re-render, restarts service.
	RepairMinioStorage func(ctx context.Context, minioReport map[string]any) (map[string]any, error)

	// ValidatePKIHealth checks TLS certificate health across all nodes:
	// expiry, SAN coverage, chain validity, CA presence.
	ValidatePKIHealth func(ctx context.Context) (map[string]any, error)

	// RepairPKICerts fixes cert violations by restarting node-agent
	// to trigger re-issuance from the cluster CA.
	RepairPKICerts func(ctx context.Context, pkiReport map[string]any) (map[string]any, error)

	// EmitReport publishes the combined invariant enforcement report as a
	// cluster event for audit.
	EmitReport func(ctx context.Context, workflowReport, quorumReport, profileReport, minioReport, pkiReport map[string]any) error

	// MarkFailed records that invariant enforcement failed.
	MarkFailed func(ctx context.Context, reason string) error

	// EmitCompleted records that invariant enforcement succeeded.
	EmitCompleted func(ctx context.Context) error
}

// RegisterInvariantActions registers all cluster.invariant.enforcement
// workflow step handlers on the given router.
func RegisterInvariantActions(router *Router, cfg InvariantConfig) {
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.validate_workflows", invariantValidateWorkflows(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.repair_workflows", invariantRepairWorkflows(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.validate_infra_quorum", invariantValidateInfraQuorum(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.enforce_quorum", invariantEnforceQuorum(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.verify_quorum", invariantVerifyQuorum(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.validate_founding_profiles", invariantValidateFoundingProfiles(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.validate_minio_storage", invariantValidateMinioStorage(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.repair_minio_storage", invariantRepairMinioStorage(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.validate_pki_health", invariantValidatePKIHealth(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.repair_pki_certs", invariantRepairPKICerts(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.emit_report", invariantEmitReport(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.mark_failed", invariantMarkFailed(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.invariant.emit_completed", invariantEmitCompleted(cfg))
}

// --------------------------------------------------------------------------
// Step 1: validate_workflows
// --------------------------------------------------------------------------

func invariantValidateWorkflows(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		required := extractStringList(req.With, "required_workflows")
		if len(required) == 0 {
			return nil, fmt.Errorf("required_workflows list is empty")
		}

		if cfg.ValidateWorkflows == nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"workflow_report": map[string]any{"present": required, "missing": []string{}},
			}}, nil
		}

		report, err := cfg.ValidateWorkflows(ctx, required)
		if err != nil {
			return nil, fmt.Errorf("validate workflows: %w", err)
		}

		log.Printf("actor[invariant]: workflow completeness — %v", report)
		return &ActionResult{
			OK:      true,
			Message: fmt.Sprintf("checked %d workflows", len(required)),
			Output:  map[string]any{"workflow_report": report},
		}, nil
	}
}

// --------------------------------------------------------------------------
// Step 2: repair_missing_workflows
// --------------------------------------------------------------------------

func invariantRepairWorkflows(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		missing := extractStringList(req.With, "missing")
		if len(missing) == 0 {
			return &ActionResult{OK: true, Message: "nothing to repair"}, nil
		}

		if cfg.RepairWorkflows == nil {
			return nil, fmt.Errorf("repair_workflows not configured")
		}

		repaired, err := cfg.RepairWorkflows(ctx, missing)
		if err != nil {
			return nil, fmt.Errorf("repair workflows: %w", err)
		}

		log.Printf("actor[invariant]: repaired %d/%d missing workflows", repaired, len(missing))
		return &ActionResult{
			OK:      true,
			Message: fmt.Sprintf("repaired %d workflows", repaired),
			Output:  map[string]any{"repaired": repaired, "attempted": len(missing)},
		}, nil
	}
}

// --------------------------------------------------------------------------
// Step 3: validate_infra_quorum
// --------------------------------------------------------------------------

func invariantValidateInfraQuorum(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		minScylla := intFromWith(req.With, "min_scylla_nodes", 3)
		minMinio := intFromWith(req.With, "min_minio_nodes", 3)
		etcdAll, _ := req.With["etcd_all_nodes"].(bool)

		if cfg.ValidateInfraQuorum == nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"quorum_report": map[string]any{"violations": []any{}, "candidates": []any{}},
			}}, nil
		}

		report, err := cfg.ValidateInfraQuorum(ctx, minScylla, minMinio, etcdAll)
		if err != nil {
			return nil, fmt.Errorf("validate infra quorum: %w", err)
		}

		log.Printf("actor[invariant]: infra quorum — %v", report)
		return &ActionResult{
			OK:     true,
			Output: map[string]any{"quorum_report": report},
		}, nil
	}
}

// --------------------------------------------------------------------------
// Step 4: enforce_quorum
// --------------------------------------------------------------------------

func invariantEnforceQuorum(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		quorumReport, _ := req.With["quorum_report"].(map[string]any)
		if quorumReport == nil {
			// Try outputs from previous step.
			quorumReport, _ = req.Outputs["quorum_report"].(map[string]any)
		}
		if quorumReport == nil {
			return nil, fmt.Errorf("quorum_report not available")
		}

		if cfg.EnforceQuorum == nil {
			return &ActionResult{OK: true, Message: "enforce_quorum not configured"}, nil
		}

		if err := cfg.EnforceQuorum(ctx, quorumReport); err != nil {
			return nil, fmt.Errorf("enforce quorum: %w", err)
		}

		log.Printf("actor[invariant]: quorum enforcement applied")
		return &ActionResult{OK: true, Message: "quorum enforced"}, nil
	}
}

// --------------------------------------------------------------------------
// Step 4 verification: verify_quorum
// --------------------------------------------------------------------------

func invariantVerifyQuorum(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		minScylla := intFromWith(req.With, "min_scylla_nodes", 3)
		minMinio := intFromWith(req.With, "min_minio_nodes", 3)

		if cfg.VerifyQuorum == nil {
			return &ActionResult{OK: true}, nil
		}

		ok, err := cfg.VerifyQuorum(ctx, minScylla, minMinio)
		if err != nil {
			return nil, fmt.Errorf("verify quorum: %w", err)
		}
		if !ok {
			return nil, fmt.Errorf("quorum verification failed: insufficient nodes after enforcement")
		}

		return &ActionResult{OK: true, Message: "quorum verified"}, nil
	}
}

// --------------------------------------------------------------------------
// Step 5: validate_founding_profiles
// --------------------------------------------------------------------------

func invariantValidateFoundingProfiles(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.ValidateFoundingProfiles == nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"profile_report": map[string]any{"violations": []any{}},
			}}, nil
		}

		report, err := cfg.ValidateFoundingProfiles(ctx)
		if err != nil {
			return nil, fmt.Errorf("validate founding profiles: %w", err)
		}

		log.Printf("actor[invariant]: founding profiles — %v", report)
		return &ActionResult{
			OK:     true,
			Output: map[string]any{"profile_report": report},
		}, nil
	}
}

// --------------------------------------------------------------------------
// Step 6: emit_report
// --------------------------------------------------------------------------

func invariantEmitReport(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		workflowReport, _ := req.With["workflow_report"].(map[string]any)
		quorumReport, _ := req.With["quorum_report"].(map[string]any)
		profileReport, _ := req.With["profile_report"].(map[string]any)
		minioReport, _ := req.With["minio_report"].(map[string]any)
		pkiReport, _ := req.With["pki_report"].(map[string]any)

		// Also check outputs for reports from previous steps.
		if workflowReport == nil {
			workflowReport, _ = req.Outputs["workflow_report"].(map[string]any)
		}
		if quorumReport == nil {
			quorumReport, _ = req.Outputs["quorum_report"].(map[string]any)
		}
		if profileReport == nil {
			profileReport, _ = req.Outputs["profile_report"].(map[string]any)
		}
		if minioReport == nil {
			minioReport, _ = req.Outputs["minio_report"].(map[string]any)
		}
		if pkiReport == nil {
			pkiReport, _ = req.Outputs["pki_report"].(map[string]any)
		}

		if cfg.EmitReport != nil {
			if err := cfg.EmitReport(ctx, workflowReport, quorumReport, profileReport, minioReport, pkiReport); err != nil {
				return nil, fmt.Errorf("emit report: %w", err)
			}
		}

		log.Printf("actor[invariant]: enforcement report emitted")
		return &ActionResult{
			OK:      true,
			Message: "invariant enforcement complete",
			Output: map[string]any{
				"workflow_report": workflowReport,
				"quorum_report":  quorumReport,
				"profile_report": profileReport,
				"minio_report":   minioReport,
				"pki_report":     pkiReport,
			},
		}, nil
	}
}

// --------------------------------------------------------------------------
// Step 6: validate_minio_storage
// --------------------------------------------------------------------------

func invariantValidateMinioStorage(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.ValidateMinioStorage == nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"minio_report": map[string]any{"violations": []any{}},
			}}, nil
		}

		report, err := cfg.ValidateMinioStorage(ctx)
		if err != nil {
			return nil, fmt.Errorf("validate minio storage: %w", err)
		}

		log.Printf("actor[invariant]: minio storage — %v", report)
		return &ActionResult{
			OK:     true,
			Output: map[string]any{"minio_report": report},
		}, nil
	}
}

// --------------------------------------------------------------------------
// Step 6b: repair_minio_storage
// --------------------------------------------------------------------------

func invariantRepairMinioStorage(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		minioReport, _ := req.With["minio_report"].(map[string]any)
		if minioReport == nil {
			minioReport, _ = req.Outputs["minio_report"].(map[string]any)
		}
		if minioReport == nil {
			return &ActionResult{OK: true, Message: "no minio report available"}, nil
		}

		if cfg.RepairMinioStorage == nil {
			return &ActionResult{OK: true, Message: "repair_minio_storage not configured"}, nil
		}

		result, err := cfg.RepairMinioStorage(ctx, minioReport)
		if err != nil {
			return nil, fmt.Errorf("repair minio storage: %w", err)
		}

		log.Printf("actor[invariant]: minio storage repair — %v", result)
		return &ActionResult{
			OK:     true,
			Output: map[string]any{"minio_repair": result},
		}, nil
	}
}

// --------------------------------------------------------------------------
// Step 7: validate_pki_health
// --------------------------------------------------------------------------

func invariantValidatePKIHealth(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.ValidatePKIHealth == nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"pki_report": map[string]any{"violations": []any{}},
			}}, nil
		}

		report, err := cfg.ValidatePKIHealth(ctx)
		if err != nil {
			return nil, fmt.Errorf("validate pki health: %w", err)
		}

		log.Printf("actor[invariant]: pki health — %v", report)
		return &ActionResult{
			OK:     true,
			Output: map[string]any{"pki_report": report},
		}, nil
	}
}

// --------------------------------------------------------------------------
// Step 7b: repair_pki_certs
// --------------------------------------------------------------------------

func invariantRepairPKICerts(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		pkiReport, _ := req.With["pki_report"].(map[string]any)
		if pkiReport == nil {
			pkiReport, _ = req.Outputs["pki_report"].(map[string]any)
		}
		if pkiReport == nil {
			return &ActionResult{OK: true, Message: "no pki report available"}, nil
		}

		if cfg.RepairPKICerts == nil {
			return &ActionResult{OK: true, Message: "repair_pki_certs not configured"}, nil
		}

		result, err := cfg.RepairPKICerts(ctx, pkiReport)
		if err != nil {
			return nil, fmt.Errorf("repair pki certs: %w", err)
		}

		log.Printf("actor[invariant]: pki cert repair — %v", result)
		return &ActionResult{
			OK:     true,
			Output: map[string]any{"pki_repair": result},
		}, nil
	}
}

// --------------------------------------------------------------------------
// onFailure / onSuccess
// --------------------------------------------------------------------------

func invariantMarkFailed(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		reason := "unknown"
		if r, ok := req.With["reason"].(string); ok && r != "" {
			reason = r
		}

		log.Printf("actor[invariant]: enforcement FAILED: %s", reason)

		if cfg.MarkFailed != nil {
			if err := cfg.MarkFailed(ctx, reason); err != nil {
				return nil, fmt.Errorf("mark failed: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func invariantEmitCompleted(cfg InvariantConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		log.Printf("actor[invariant]: enforcement SUCCEEDED")

		if cfg.EmitCompleted != nil {
			if err := cfg.EmitCompleted(ctx); err != nil {
				return nil, fmt.Errorf("emit completed: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// extractStringList extracts a []string from a with map key that may be
// []any (JSON-deserialized) or []string.
func extractStringList(with map[string]any, key string) []string {
	raw, ok := with[key]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, fmt.Sprint(item))
		}
		return out
	default:
		return nil
	}
}

// intFromWith extracts an integer from a with map, returning defaultVal if
// not present or not convertible.
func intFromWith(with map[string]any, key string, defaultVal int) int {
	raw, ok := with[key]
	if !ok {
		return defaultVal
	}
	switch v := raw.(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return defaultVal
	}
}
