// @awareness namespace=globular.platform
// @awareness component=platform_controller.workflow_ingress_restore
// @awareness file_role=controller_side_handlers_and_dispatch_for_cluster_ingress_spec_restore_workflow
// @awareness implements=globular.platform:invariant.workflow.every_state_mutation_belongs_to_a_workflow_instance
// @awareness protects=globular.platform:failure_mode.hidden_workflow.controller_ingress_spec_guard_restore_path
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/globulario/services/golang/workflow/engine"
)

// workflow_ingress_restore.go — controller-side handlers and dispatch helper
// for the cluster.ingress_spec_restore workflow. Lifts the previously-inline
// restoreIngressSpecFromBackup function (ingress_spec_guard.go) into a
// declarative workflow with durable step receipts. See failure_mode
// hidden_workflow.controller_ingress_spec_guard_restore_path for the audit
// finding that motivated this lift.

// buildIngressControllerConfig assembles the IngressControllerConfig the
// engine needs to drive the cluster.ingress_spec_restore workflow. Each
// handler delegates to controller-internal helpers so the canonical typed-
// RPC contracts (publishIngressSpec, normalizeIngressSpec) are preserved.
func (srv *server) buildIngressControllerConfig() engine.IngressControllerConfig {
	return engine.IngressControllerConfig{
		LoadBackup: func(ctx context.Context) ([]byte, bool, error) {
			kv := srv.kv
			if kv == nil {
				kv = srv.etcdClient
			}
			if kv == nil {
				return nil, false, fmt.Errorf("etcd unavailable")
			}
			rctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			resp, err := kv.Get(rctx, ingressSpecBackupKey)
			if err != nil {
				return nil, false, fmt.Errorf("get backup: %w", err)
			}
			if len(resp.Kvs) == 0 || len(resp.Kvs[0].Value) == 0 {
				return nil, false, nil
			}
			return resp.Kvs[0].Value, true, nil
		},

		ComposeRestoreSpec: func(ctx context.Context, backupPresent bool, backupBytes []byte) ([]byte, string, error) {
			// Restored-from-backup path: parse, normalize, return.
			if backupPresent && len(backupBytes) > 0 {
				var spec ingressDesiredSpec
				if uerr := json.Unmarshal(backupBytes, &spec); uerr == nil {
					spec = srv.normalizeIngressSpec(spec)
					b, err := json.Marshal(spec)
					if err != nil {
						return nil, "", fmt.Errorf("marshal restored: %w", err)
					}
					return b, "backup", nil
				}
				// Backup present but unparseable. Fall through to seed.
				log.Printf("ingress-restore: backup present but JSON parse failed — seeding explicit-disabled baseline")
			}

			// Seed-default path: explicit-disabled baseline so Day-0 has
			// authoritative intent. Same shape as the inline implementation.
			seed := srv.normalizeIngressSpec(ingressDesiredSpec{
				Mode:             ingressModeDisabled,
				ExplicitDisabled: true,
				Reason:           "day0 bootstrap default: ingress not yet configured",
			})
			b, err := json.Marshal(seed)
			if err != nil {
				return nil, "", fmt.Errorf("marshal seed: %w", err)
			}
			return b, "seed", nil
		},

		PublishRestoreSpec: func(ctx context.Context, specBytes []byte) error {
			var spec ingressDesiredSpec
			if err := json.Unmarshal(specBytes, &spec); err != nil {
				return fmt.Errorf("unmarshal spec_bytes: %w", err)
			}
			// publishIngressSpec already enforces ValidateCriticalKeyWrite
			// for both ingressSpecKey and ingressSpecBackupKey.
			return srv.publishIngressSpec(ctx, spec)
		},
	}
}

// dispatchIngressSpecRestore starts a cluster.ingress_spec_restore workflow
// run and waits for it to terminate. Called from ensureIngressDesiredState
// (the guard tick) when the live spec is absent or unparseable. Returns the
// workflow's terminal status; the caller logs on failure but does not
// escalate — the next tick will retry.
func (srv *server) dispatchIngressSpecRestore(ctx context.Context, triggerReason string) error {
	router := engine.NewRouter()
	engine.RegisterIngressControllerActions(router, srv.buildIngressControllerConfig())

	inputs := map[string]any{
		"cluster_id":     srv.cfg.ClusterDomain,
		"trigger_reason": triggerReason,
	}
	// correlation_id deduplicates within a 30s tick window. Two restores
	// within the same second collapse to one run; subsequent ticks get a
	// fresh ID. The WF-DEFER B3 abandoned-after-N-defers machinery applies.
	corrID := fmt.Sprintf("ingress-spec-restore-%d", time.Now().Unix())

	resp, err := srv.executeWorkflowCentralized(ctx, "cluster.ingress_spec_restore", corrID, inputs, router)
	if err != nil {
		return fmt.Errorf("dispatch ingress_spec_restore: %w", err)
	}
	if resp.Status == "FAILED" {
		return fmt.Errorf("ingress_spec_restore workflow failed: %s", resp.Error)
	}
	log.Printf("ingress-restore: workflow run_id=%s status=%s trigger=%s",
		resp.RunId, resp.Status, triggerReason)
	return nil
}

