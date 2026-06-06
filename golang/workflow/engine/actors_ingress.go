// @awareness namespace=globular.platform
// @awareness component=platform_workflow.actors_ingress
// @awareness file_role=ingress_spec_restore_workflow_actions_lift_of_hidden_workflow
// @awareness implements=globular.platform:intent.workflow.source_of_operational_truth
// @awareness implements=globular.platform:invariant.workflow.every_state_mutation_belongs_to_a_workflow_instance
// @awareness protects=globular.platform:failure_mode.hidden_workflow.controller_ingress_spec_guard_restore_path
// @awareness risk=high
package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// ──────────────────────────────────────────────────────────────────────────
// cluster.ingress_spec_restore controller actions
// ──────────────────────────────────────────────────────────────────────────
//
// cluster.ingress_spec_restore is the workflow-native replacement for the
// previously-inline restoreIngressSpecFromBackup function in
// cluster_controller_server/ingress_spec_guard.go. The inline implementation
// was flagged in the 2026-06-05 hidden-workflow audit
// (failure_mode hidden_workflow.controller_ingress_spec_guard_restore_path):
// it performed multi-step state-mutating work (read backup → compose spec →
// write live key + backup key) without a durable workflow run.
//
// The lift introduces three workflow steps, each with a durable receipt:
//
//   1. controller.ingress.load_spec_backup    — read backup key, return bytes
//   2. controller.ingress.compose_restore_spec — build restored OR seed spec
//   3. controller.ingress.publish_restore_spec — atomically write live+backup
//
// The ingress-spec-guard tick remains the trigger but now dispatches this
// workflow instead of doing the work inline. Each restore attempt produces
// a workflow_runs row naming the leader, the trigger reason, and the
// outcome of each step.

// IngressControllerConfig provides dependencies for the
// cluster.ingress_spec_restore workflow's controller-side actions. The
// cluster_controller wires these to its etcd accessors, the
// publishIngressSpec helper, and the normalizeIngressSpec helper —
// keeping the controller's typed-RPC contract intact.
type IngressControllerConfig struct {
	// LoadBackup reads the ingress spec backup bytes from etcd. Returns
	// (nil, false, nil) if the backup key is absent. Errors are reserved
	// for etcd read failures (connectivity, auth) — they are NOT used to
	// signal "absent backup".
	LoadBackup func(ctx context.Context) (backupBytes []byte, present bool, err error)

	// ComposeRestoreSpec builds the spec to publish. If backupPresent is
	// true and backupBytes parse cleanly, the result is the normalized
	// restored spec. Otherwise the result is the normalized
	// explicit-disabled seed baseline. Returns the JSON bytes that
	// PublishRestoreSpec will write.
	ComposeRestoreSpec func(ctx context.Context, backupPresent bool, backupBytes []byte) (specBytes []byte, source string, err error)

	// PublishRestoreSpec writes the bytes to BOTH ingressSpecKey and
	// ingressSpecBackupKey. Wraps the existing publishIngressSpec helper
	// so the same critical-key write guard applies.
	PublishRestoreSpec func(ctx context.Context, specBytes []byte) error
}

// RegisterIngressControllerActions registers the three controller-side
// actions the cluster.ingress_spec_restore workflow YAML declares.
func RegisterIngressControllerActions(router *Router, cfg IngressControllerConfig) {
	router.Register(v1alpha1.ActorClusterController, "controller.ingress.load_spec_backup",
		ingressLoadSpecBackup(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.ingress.compose_restore_spec",
		ingressComposeRestoreSpec(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.ingress.publish_restore_spec",
		ingressPublishRestoreSpec(cfg))
}

func ingressLoadSpecBackup(cfg IngressControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.LoadBackup == nil {
			return nil, fmt.Errorf("ingress.load_spec_backup: handler not wired")
		}
		bytes, present, err := cfg.LoadBackup(ctx)
		if err != nil {
			return nil, fmt.Errorf("load_spec_backup: %w", err)
		}
		log.Printf("actor[controller]: ingress.load_spec_backup present=%v len=%d",
			present, len(bytes))
		// Result.Output is what propagates into run.Outputs for subsequent
		// steps' $.<field> expressions. Step-local req.Outputs is NOT merged
		// by the engine, so threaded values MUST live here.
		return &ActionResult{
			OK: true,
			Output: map[string]any{
				"backup_present": present,
				"backup_bytes":   string(bytes),
				"backup_len":     len(bytes),
			},
		}, nil
	}
}

func ingressComposeRestoreSpec(cfg IngressControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.ComposeRestoreSpec == nil {
			return nil, fmt.Errorf("ingress.compose_restore_spec: handler not wired")
		}
		// Inputs threaded from load_spec_backup via run.Outputs (flat keys).
		backupPresent, _ := req.With["backup_present"].(bool)
		backupStr, _ := req.With["backup_bytes"].(string)

		specBytes, source, err := cfg.ComposeRestoreSpec(ctx, backupPresent, []byte(backupStr))
		if err != nil {
			return nil, fmt.Errorf("compose_restore_spec: %w", err)
		}
		log.Printf("actor[controller]: ingress.compose_restore_spec source=%s len=%d",
			source, len(specBytes))
		return &ActionResult{
			OK: true,
			Output: map[string]any{
				"spec_bytes": string(specBytes),
				"source":     source,
				"spec_len":   len(specBytes),
			},
		}, nil
	}
}

func ingressPublishRestoreSpec(cfg IngressControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.PublishRestoreSpec == nil {
			return nil, fmt.Errorf("ingress.publish_restore_spec: handler not wired")
		}
		specStr, _ := req.With["spec_bytes"].(string)
		if specStr == "" {
			return nil, fmt.Errorf("publish_restore_spec: spec_bytes is empty (compose step did not produce a spec)")
		}
		if err := cfg.PublishRestoreSpec(ctx, []byte(specStr)); err != nil {
			return nil, fmt.Errorf("publish_restore_spec: %w", err)
		}
		log.Printf("actor[controller]: ingress.publish_restore_spec len=%d — wrote live+backup",
			len(specStr))
		return &ActionResult{OK: true}, nil
	}
}
