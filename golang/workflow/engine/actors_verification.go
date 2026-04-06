// actors_verification.go registers read-only verification handlers used
// by the resume-policy dispatch (WH-2) to prove whether a step's intended
// effect already exists. These are separate from the existing step actions
// because they return structured tri-state results (present/absent/inconclusive)
// instead of pass/fail errors.
//
// Dual-source verification: install/sync handlers check both local reality
// AND authoritative state per the hardening design.
//
// See docs/architecture/workflow-hardening-implementation.md §WH-4.
package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// ── Node-agent verification config ───────────────────────────────────────────

// NodeVerificationConfig provides dependencies for node-agent verification
// handlers. Each function is optional; nil means the verification returns
// inconclusive (safe fallback).
type NodeVerificationConfig struct {
	// VerifyPackagesInstalled checks if all named packages are installed
	// at the expected version on the local node. Returns (allInstalled, error).
	// Should check BOTH local reality (files/systemd) AND etcd state.
	VerifyPackagesInstalled func(ctx context.Context, nodeID string, packages []any) (bool, error)

	// VerifyInstalledStateSynced checks if the node's installed state in
	// etcd matches local reality. Returns (synced, error).
	VerifyInstalledStateSynced func(ctx context.Context, nodeID string) (bool, error)

	// VerifyPackageState checks a specific package's version and hash.
	// Returns (installed, error).
	VerifyPackageState func(ctx context.Context, nodeID, name, version, hash string) (bool, error)
}

// RegisterNodeVerificationActions registers verification handlers for
// the node-agent actor. These are used by resume-policy verify_effect
// to check if a step's effect already exists.
func RegisterNodeVerificationActions(router *Router, cfg NodeVerificationConfig) {
	router.Register(v1alpha1.ActorNodeAgent, "node.verify_packages_installed",
		nodeVerifyPackagesInstalled(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.verify_installed_state_synced",
		nodeVerifyInstalledStateSynced(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.verify_installed_package_state",
		nodeVerifyInstalledPackageState(cfg))
}

func nodeVerifyPackagesInstalled(cfg NodeVerificationConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := toStr(req.With["node_id"])
		packages, _ := req.With["packages"].([]any)

		if cfg.VerifyPackagesInstalled == nil {
			log.Printf("verify: packages_installed: no verifier configured")
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "all_installed": false,
				"reason": "no verifier configured",
			}}, nil
		}

		allInstalled, err := cfg.VerifyPackagesInstalled(ctx, nodeID, packages)
		if err != nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "all_installed": false,
				"reason": err.Error(),
			}}, nil
		}

		if allInstalled {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "present", "all_installed": true,
			}}, nil
		}
		return &ActionResult{OK: true, Output: map[string]any{
			"status": "absent", "all_installed": false,
		}}, nil
	}
}

func nodeVerifyInstalledStateSynced(cfg NodeVerificationConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := toStr(req.With["node_id"])

		if cfg.VerifyInstalledStateSynced == nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "synced": false,
			}}, nil
		}

		synced, err := cfg.VerifyInstalledStateSynced(ctx, nodeID)
		if err != nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "synced": false, "reason": err.Error(),
			}}, nil
		}

		if synced {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "present", "synced": true,
			}}, nil
		}
		return &ActionResult{OK: true, Output: map[string]any{
			"status": "absent", "synced": false,
		}}, nil
	}
}

func nodeVerifyInstalledPackageState(cfg NodeVerificationConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := toStr(req.With["node_id"])
		name := toStr(req.With["package_name"])
		version := toStr(req.With["version"])
		hash := toStr(req.With["desired_hash"])

		if cfg.VerifyPackageState == nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "installed": false,
			}}, nil
		}

		installed, err := cfg.VerifyPackageState(ctx, nodeID, name, version, hash)
		if err != nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "installed": false, "reason": err.Error(),
			}}, nil
		}

		if installed {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "present", "installed": true,
			}}, nil
		}
		return &ActionResult{OK: true, Output: map[string]any{
			"status": "absent", "installed": false,
		}}, nil
	}
}

// ── Controller verification config ──────────────────────────────────────────

// ControllerVerificationConfig provides dependencies for controller
// verification handlers.
type ControllerVerificationConfig struct {
	// VerifyReleaseStatus checks if a release's phase matches the expected status.
	VerifyReleaseStatus func(ctx context.Context, releaseID, expectedStatus string) (bool, error)

	// VerifyNodeReleaseStatus checks if a node's per-release status matches expected.
	VerifyNodeReleaseStatus func(ctx context.Context, releaseID, nodeID, expectedStatus string) (bool, error)

	// VerifyReleaseTerminal checks if a release is in a terminal state.
	VerifyReleaseTerminal func(ctx context.Context, releaseID string) (bool, error)

	// VerifyBootstrapPhase checks if a node's bootstrap phase matches expected.
	VerifyBootstrapPhase func(ctx context.Context, nodeID, expectedPhase string) (bool, error)
}

// RegisterControllerVerificationActions registers verification handlers
// for the cluster-controller actor.
func RegisterControllerVerificationActions(router *Router, cfg ControllerVerificationConfig) {
	router.Register(v1alpha1.ActorClusterController, "controller.release.verify_status",
		controllerVerifyReleaseStatus(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.release.verify_node_status",
		controllerVerifyNodeReleaseStatus(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.release.verify_terminal_status",
		controllerVerifyReleaseTerminal(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.bootstrap.verify_phase",
		controllerVerifyBootstrapPhase(cfg))
}

func controllerVerifyReleaseStatus(cfg ControllerVerificationConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := toStr(req.With["release_id"])
		expected := toStr(req.With["expected_status"])

		if cfg.VerifyReleaseStatus == nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "status_matches": false,
			}}, nil
		}

		matches, err := cfg.VerifyReleaseStatus(ctx, releaseID, expected)
		if err != nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "status_matches": false, "reason": err.Error(),
			}}, nil
		}

		status := "absent"
		if matches {
			status = "present"
		}
		return &ActionResult{OK: true, Output: map[string]any{
			"status": status, "status_matches": matches,
		}}, nil
	}
}

func controllerVerifyNodeReleaseStatus(cfg ControllerVerificationConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := toStr(req.With["release_id"])
		nodeID := toStr(req.With["node_id"])
		expected := toStr(req.With["expected_status"])

		if cfg.VerifyNodeReleaseStatus == nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "status_matches": false,
			}}, nil
		}

		matches, err := cfg.VerifyNodeReleaseStatus(ctx, releaseID, nodeID, expected)
		if err != nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "status_matches": false, "reason": err.Error(),
			}}, nil
		}

		status := "absent"
		if matches {
			status = "present"
		}
		return &ActionResult{OK: true, Output: map[string]any{
			"status": status, "status_matches": matches,
		}}, nil
	}
}

func controllerVerifyReleaseTerminal(cfg ControllerVerificationConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := toStr(req.With["release_id"])

		if cfg.VerifyReleaseTerminal == nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "terminal": false,
			}}, nil
		}

		terminal, err := cfg.VerifyReleaseTerminal(ctx, releaseID)
		if err != nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "terminal": false, "reason": err.Error(),
			}}, nil
		}

		status := "absent"
		if terminal {
			status = "present"
		}
		return &ActionResult{OK: true, Output: map[string]any{
			"status": status, "terminal": terminal,
		}}, nil
	}
}

func controllerVerifyBootstrapPhase(cfg ControllerVerificationConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := toStr(req.With["node_id"])
		expected := toStr(req.With["phase"])

		if cfg.VerifyBootstrapPhase == nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "phase_matches": false,
			}}, nil
		}

		matches, err := cfg.VerifyBootstrapPhase(ctx, nodeID, expected)
		if err != nil {
			return &ActionResult{OK: true, Output: map[string]any{
				"status": "inconclusive", "phase_matches": false, "reason": fmt.Sprintf("%v", err),
			}}, nil
		}

		status := "absent"
		if matches {
			status = "present"
		}
		return &ActionResult{OK: true, Output: map[string]any{
			"status": status, "phase_matches": matches,
		}}, nil
	}
}
