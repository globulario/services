// @awareness namespace=globular.platform
// @awareness component=platform_workflow.actors_platform_upgrade
// @awareness file_role=platform_upgrade_per_node_per_package_decision_orchestration
// @awareness implements=globular.platform:intent.workflow.source_of_operational_truth
// @awareness implements=globular.platform:intent.controller.decides_but_does_not_execute_leaf_work
// @awareness risk=high
package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// ──────────────────────────────────────────────────────────────────────────
// platform.upgrade controller actions
// ──────────────────────────────────────────────────────────────────────────
//
// platform.upgrade is the workflow-native replacement for the old direct-
// etcd-write `globular platform-upgrade` CLI. The CLI used to call
// upsertServiceDesiredVersion() directly, bypassing the controller's
// validation, profile checks, and version comparison. v1.2.159 incident:
// bulk apply of all 57 BOM packages reintroduced 7 operator-removed
// services and created 28 fresh DesiredBuildIdOrphaned findings.
//
// The contract this workflow enforces:
//
//   for each (node, package in BOM):
//     if node.profiles ∩ package.profiles == ∅:   skip
//     if not installed on this node:                skip (respect removal)
//     if BOM_version > installed_version:           dispatch release.apply.package
//     if BOM_version <= installed_version:          skip (never downgrade)
//
// Upgrade dispatch uses the LOCAL REPOSITORY's build_id for the BOM
// version, not the BOM's — the repository is authoritative for what's
// actually installable; the BOM may reference build_ids the local repo
// never received.

// UpgradeDecision is the result of evaluating one (node, package) pair.
// Captured for audit; the dispatch step consumes only those with
// Action == "upgrade".
type UpgradeDecision struct {
	NodeID           string `json:"node_id"`
	PackageName      string `json:"package_name"`
	PackageKind      string `json:"package_kind"`
	InstalledVersion string `json:"installed_version,omitempty"`
	BOMVersion       string `json:"bom_version"`
	LocalBuildID     string `json:"local_build_id,omitempty"`
	// Action is one of:
	//   "profile_skip"    — node.profiles ∩ package.profiles is empty
	//   "not_installed"   — package not installed on this node (operator removed
	//                       or never installed); never auto-install at upgrade time
	//   "up_to_date"      — installed_version == BOM_version
	//   "skip_downgrade"  — installed_version > BOM_version
	//   "upgrade"         — installed_version < BOM_version, dispatch release.apply.package
	//   "missing_in_repo" — BOM version exists but the local repo has no
	//                       resolvable build_id; refuse to dispatch
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
}

// PlatformUpgradeControllerConfig provides dependencies for the
// platform.upgrade workflow's controller-side actions. Wired by
// cluster_controller_server when it builds the workflow router.
type PlatformUpgradeControllerConfig struct {
	// Evaluate computes the per-(node, package) decisions for a BOM tag.
	// Returns the full audit (including skips) plus the subset that need
	// upgrades. Errors are returned for true infrastructure failures
	// (BOM cannot be fetched, controller state unavailable). Per-decision
	// validation is recorded in the audit, not raised as an error.
	Evaluate func(ctx context.Context, releaseTag string) (audit []UpgradeDecision, upgrades []UpgradeDecision, err error)

	// DispatchUpgrades fires release.apply.package for each upgrade
	// decision. The release.apply.package workflow handles per-node
	// rollout, retries, and verification — the platform-upgrade workflow
	// only orchestrates the per-package dispatch.
	DispatchUpgrades func(ctx context.Context, releaseTag string, upgrades []UpgradeDecision) error

	// Audit records the per-(node, package) decision history durably.
	// Best-effort; failures are logged but do not fail the workflow.
	Audit func(ctx context.Context, releaseTag string, decisions []UpgradeDecision) error
}

// RegisterPlatformUpgradeControllerActions registers the three controller-side
// actions the platform.upgrade workflow YAML declares.
func RegisterPlatformUpgradeControllerActions(router *Router, cfg PlatformUpgradeControllerConfig) {
	router.Register(v1alpha1.ActorClusterController, "controller.platform_upgrade.evaluate",
		platformUpgradeEvaluate(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.platform_upgrade.dispatch_upgrades",
		platformUpgradeDispatch(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.platform_upgrade.audit",
		platformUpgradeAudit(cfg))
}

func platformUpgradeEvaluate(cfg PlatformUpgradeControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseTag := fmt.Sprint(req.With["release_tag"])
		if releaseTag == "" {
			return nil, fmt.Errorf("platform_upgrade.evaluate: release_tag is required")
		}
		if cfg.Evaluate == nil {
			return nil, fmt.Errorf("platform_upgrade.evaluate: handler not wired")
		}

		audit, upgrades, err := cfg.Evaluate(ctx, releaseTag)
		if err != nil {
			return nil, fmt.Errorf("evaluate: %w", err)
		}

		// Bucketed counts for log + audit summary.
		buckets := map[string]int{}
		for _, d := range audit {
			buckets[d.Action]++
		}
		log.Printf("actor[controller]: platform_upgrade.evaluate tag=%s "+
			"decisions=%d  upgrades=%d  buckets=%v",
			releaseTag, len(audit), len(upgrades), buckets)

		// Make the upgrade list and full audit available to subsequent
		// steps via run outputs. The YAML's `when` guards reference
		// upgrade_targets, and the audit step reads decisions.
		req.Outputs["decisions"] = decisionsToMaps(audit)
		req.Outputs["upgrade_targets"] = decisionsToMaps(upgrades)

		return &ActionResult{
			OK: true,
			Output: map[string]any{
				"decisions_count": len(audit),
				"upgrades_count":  len(upgrades),
				"buckets":         buckets,
			},
		}, nil
	}
}

func platformUpgradeDispatch(cfg PlatformUpgradeControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseTag := fmt.Sprint(req.With["release_tag"])
		raw, _ := req.With["upgrade_targets"].([]any)
		upgrades := mapsToDecisions(raw)
		if len(upgrades) == 0 {
			log.Printf("actor[controller]: platform_upgrade.dispatch — no upgrade targets, no-op")
			return &ActionResult{OK: true}, nil
		}
		if cfg.DispatchUpgrades == nil {
			return nil, fmt.Errorf("platform_upgrade.dispatch: handler not wired")
		}
		if err := cfg.DispatchUpgrades(ctx, releaseTag, upgrades); err != nil {
			return nil, fmt.Errorf("dispatch: %w", err)
		}
		log.Printf("actor[controller]: platform_upgrade.dispatch tag=%s dispatched=%d",
			releaseTag, len(upgrades))
		return &ActionResult{
			OK: true,
			Output: map[string]any{
				"dispatched_count": len(upgrades),
			},
		}, nil
	}
}

func platformUpgradeAudit(cfg PlatformUpgradeControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseTag := fmt.Sprint(req.With["release_tag"])
		raw, _ := req.With["decisions"].([]any)
		decisions := mapsToDecisions(raw)
		if cfg.Audit == nil {
			// Audit is best-effort; absence of the handler is not fatal.
			return &ActionResult{OK: true}, nil
		}
		if err := cfg.Audit(ctx, releaseTag, decisions); err != nil {
			// Log but do not fail — the audit is supplementary.
			log.Printf("actor[controller]: platform_upgrade.audit best-effort write failed: %v", err)
		}
		return &ActionResult{OK: true}, nil
	}
}

// decisionsToMaps converts typed UpgradeDecisions to the generic
// []map[string]any shape the workflow engine carries through `with` and
// outputs. Round-trip safe: mapsToDecisions inverts this.
func decisionsToMaps(in []UpgradeDecision) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, d := range in {
		out = append(out, map[string]any{
			"node_id":           d.NodeID,
			"package_name":      d.PackageName,
			"package_kind":      d.PackageKind,
			"installed_version": d.InstalledVersion,
			"bom_version":       d.BOMVersion,
			"local_build_id":    d.LocalBuildID,
			"action":            d.Action,
			"reason":            d.Reason,
		})
	}
	return out
}

func mapsToDecisions(in []any) []UpgradeDecision {
	out := make([]UpgradeDecision, 0, len(in))
	for _, e := range in {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, UpgradeDecision{
			NodeID:           strFromMap(m, "node_id"),
			PackageName:      strFromMap(m, "package_name"),
			PackageKind:      strFromMap(m, "package_kind"),
			InstalledVersion: strFromMap(m, "installed_version"),
			BOMVersion:       strFromMap(m, "bom_version"),
			LocalBuildID:     strFromMap(m, "local_build_id"),
			Action:           strFromMap(m, "action"),
			Reason:           strFromMap(m, "reason"),
		})
	}
	return out
}

func strFromMap(m map[string]any, key string) string {
	if v, ok := m[key]; ok && v != nil {
		return fmt.Sprint(v)
	}
	return ""
}
