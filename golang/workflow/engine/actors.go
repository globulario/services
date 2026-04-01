package engine

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// RegisterNodeAgentActions registers all node-agent actor handlers.
// These wrap the actual install/verify/sync operations on the local node.
func RegisterNodeAgentActions(router *Router, cfg NodeAgentConfig) {
	router.Register(v1alpha1.ActorNodeAgent, "node.install_packages", nodeInstallPackages(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.verify_services_active", nodeVerifyServicesActive(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.sync_installed_state", nodeSyncInstalledState(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.probe_infra_health", nodeProbeInfraHealth(cfg))
}

// RegisterControllerActions registers cluster-controller actor handlers.
func RegisterControllerActions(router *Router, cfg ControllerConfig) {
	router.Register(v1alpha1.ActorClusterController, "controller.bootstrap.set_phase", controllerSetPhase(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.bootstrap.mark_failed", controllerMarkFailed(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.bootstrap.emit_ready", controllerEmitReady(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.bootstrap.wait_condition", controllerWaitCondition(cfg))
}

// --------------------------------------------------------------------------
// Config
// --------------------------------------------------------------------------

// NodeAgentConfig provides dependencies for node-agent actions.
type NodeAgentConfig struct {
	// FetchAndInstall fetches a package from the repository and installs it.
	// This is the core operation — wraps the existing installer engine.
	FetchAndInstall func(ctx context.Context, pkg PackageRef) error

	// IsServiceActive checks if a systemd unit is active.
	IsServiceActive func(name string) bool

	// SyncInstalledState publishes installed packages to etcd.
	SyncInstalledState func(ctx context.Context) error

	// ProbeInfraHealth runs a named infrastructure health probe
	// (e.g. "probe-scylla-health") and returns true if healthy.
	ProbeInfraHealth func(ctx context.Context, probeName string) bool

	// NodeID is the local node identifier.
	NodeID string
}

// ControllerConfig provides dependencies for controller actions.
type ControllerConfig struct {
	// SetBootstrapPhase updates a node's bootstrap phase.
	SetBootstrapPhase func(ctx context.Context, nodeID, phase string) error

	// EmitEvent publishes a cluster event.
	EmitEvent func(ctx context.Context, eventType string, data map[string]any) error

	// WaitCondition polls until a bootstrap condition is satisfied.
	WaitCondition func(ctx context.Context, nodeID, condition string) error
}

// PackageRef identifies a package to install.
type PackageRef struct {
	Name string
	Kind string // SERVICE, INFRASTRUCTURE, COMMAND
}

// --------------------------------------------------------------------------
// Node-agent actions
// --------------------------------------------------------------------------

func nodeInstallPackages(cfg NodeAgentConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		pkgs, err := extractPackageList(req.With)
		if err != nil {
			return nil, fmt.Errorf("parse packages: %w", err)
		}
		if len(pkgs) == 0 {
			return &ActionResult{OK: true, Message: "no packages to install"}, nil
		}

		log.Printf("actor[node-agent]: installing %d packages: %s",
			len(pkgs), packageNames(pkgs))

		// Install all packages in parallel within this step.
		var wg sync.WaitGroup
		var mu sync.Mutex
		var errors []string
		installed := 0

		for _, pkg := range pkgs {
			pkg := pkg
			wg.Add(1)
			go func() {
				defer wg.Done()
				start := time.Now()
				if err := cfg.FetchAndInstall(ctx, pkg); err != nil {
					mu.Lock()
					errors = append(errors, fmt.Sprintf("%s: %v", pkg.Name, err))
					mu.Unlock()
					log.Printf("actor[node-agent]: FAILED %s (%v)", pkg.Name, err)
					return
				}
				mu.Lock()
				installed++
				mu.Unlock()
				log.Printf("actor[node-agent]: installed %s (%s)", pkg.Name, time.Since(start).Round(time.Millisecond))
			}()
		}
		wg.Wait()

		if len(errors) > 0 {
			return nil, fmt.Errorf("failed to install %d/%d packages: %s",
				len(errors), len(pkgs), strings.Join(errors, "; "))
		}

		return &ActionResult{
			OK:      true,
			Message: fmt.Sprintf("installed %d packages", installed),
			Output:  map[string]any{"installed": installed},
		}, nil
	}
}

func nodeVerifyServicesActive(cfg NodeAgentConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		services, _ := req.With["services"].([]any)
		if len(services) == 0 {
			return &ActionResult{OK: true}, nil
		}

		var inactive []string
		for _, s := range services {
			name := fmt.Sprint(s)
			if !cfg.IsServiceActive(name) {
				inactive = append(inactive, name)
			}
		}

		if len(inactive) > 0 {
			return nil, fmt.Errorf("services not active: %s", strings.Join(inactive, ", "))
		}

		return &ActionResult{OK: true, Message: fmt.Sprintf("all %d services active", len(services))}, nil
	}
}

func nodeSyncInstalledState(cfg NodeAgentConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.SyncInstalledState == nil {
			return &ActionResult{OK: true, Message: "sync not configured"}, nil
		}
		if err := cfg.SyncInstalledState(ctx); err != nil {
			return nil, fmt.Errorf("sync installed state: %w", err)
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeProbeInfraHealth(cfg NodeAgentConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		probeName, _ := req.With["probe"].(string)
		if probeName == "" {
			return nil, fmt.Errorf("probe_infra_health: missing 'probe' input")
		}
		if cfg.ProbeInfraHealth == nil {
			return nil, fmt.Errorf("probe_infra_health: not configured")
		}
		if cfg.ProbeInfraHealth(ctx, probeName) {
			return &ActionResult{OK: true, Message: probeName + " healthy"}, nil
		}
		return nil, fmt.Errorf("%s: not healthy", probeName)
	}
}

// --------------------------------------------------------------------------
// Controller actions
// --------------------------------------------------------------------------

func controllerSetPhase(cfg ControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		phase := fmt.Sprint(req.With["phase"])
		nodeID := fmt.Sprint(req.Inputs["node_id"])
		if cfg.SetBootstrapPhase != nil {
			if err := cfg.SetBootstrapPhase(ctx, nodeID, phase); err != nil {
				return nil, fmt.Errorf("set phase %s: %w", phase, err)
			}
		}
		log.Printf("actor[controller]: node %s → phase %s", nodeID, phase)
		return &ActionResult{OK: true, Output: map[string]any{"phase": phase}}, nil
	}
}

func controllerMarkFailed(cfg ControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		reason := fmt.Sprint(req.With["reason"])
		nodeID := fmt.Sprint(req.Inputs["node_id"])
		log.Printf("actor[controller]: node %s bootstrap FAILED: %s", nodeID, reason)
		if cfg.EmitEvent != nil {
			cfg.EmitEvent(ctx, "node.bootstrap.failed", map[string]any{
				"node_id": nodeID, "reason": reason,
			})
		}
		return &ActionResult{OK: true}, nil
	}
}

func controllerEmitReady(cfg ControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		nodeID := fmt.Sprint(req.Inputs["node_id"])
		log.Printf("actor[controller]: node %s bootstrap READY", nodeID)
		if cfg.EmitEvent != nil {
			cfg.EmitEvent(ctx, "node.bootstrap.ready", map[string]any{"node_id": nodeID})
		}
		return &ActionResult{OK: true}, nil
	}
}

func controllerWaitCondition(cfg ControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		condition := fmt.Sprint(req.With["condition"])
		nodeID := fmt.Sprint(req.Inputs["node_id"])
		if cfg.WaitCondition != nil {
			if err := cfg.WaitCondition(ctx, nodeID, condition); err != nil {
				return nil, fmt.Errorf("wait condition %s: %w", condition, err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{"condition": condition, "satisfied": true}}, nil
	}
}

// --------------------------------------------------------------------------
// Installer actions (Day-0 bootstrap)
// --------------------------------------------------------------------------

// InstallerConfig provides dependencies for installer actor actions.
// These wrap the local bootstrap primitives on the node-agent.
type InstallerConfig struct {
	SetupTLS                func(ctx context.Context, clusterID string) error
	EnableBootstrapWindow   func(ctx context.Context, ttl time.Duration) error
	DisableBootstrapWindow  func(ctx context.Context) error
	WriteBootstrapCreds     func(ctx context.Context) error
	InstallPackage          func(ctx context.Context, name string) error
	InstallPackageSet       func(ctx context.Context, packages []string) error
	InstallProfileSets      func(ctx context.Context, profiles []string) error
	ConfigureSharedStorage  func(ctx context.Context) error
	BootstrapDNS            func(ctx context.Context, domain string) error
	ValidateClusterHealth   func(ctx context.Context) error
	GenerateJoinToken       func(ctx context.Context) (string, error)
	RestartServices         func(ctx context.Context, services []string) error
	ClusterBootstrap        func(ctx context.Context, clusterID, nodeID string) error
	CaptureFailureBundle    func(ctx context.Context, runID string) error
}

// RegisterInstallerActions registers all installer actor handlers for Day-0.
func RegisterInstallerActions(router *Router, cfg InstallerConfig) {
	router.Register(v1alpha1.ActorInstaller, "installer.setup_tls", installerSetupTLS(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.enable_bootstrap_window", installerEnableBootstrapWindow(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.disable_bootstrap_window", installerDisableBootstrapWindow(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.write_bootstrap_credentials", installerWriteBootstrapCreds(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.install_package", installerInstallPackage(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.install_package_set", installerInstallPackageSet(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.install_profile_sets", installerInstallProfileSets(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.configure_shared_storage", installerConfigureSharedStorage(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.bootstrap_dns", installerBootstrapDNS(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.validate_cluster_health", installerValidateClusterHealth(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.generate_join_token", installerGenerateJoinToken(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.restart_bootstrap_services", installerRestartServices(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.cluster_bootstrap", installerClusterBootstrap(cfg))
	router.Register(v1alpha1.ActorInstaller, "installer.capture_bootstrap_failure_bundle", installerCaptureFailureBundle(cfg))
}

func installerSetupTLS(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		clusterID := fmt.Sprint(req.Inputs["cluster_id"])
		if cfg.SetupTLS != nil {
			if err := cfg.SetupTLS(ctx, clusterID); err != nil {
				return nil, fmt.Errorf("setup TLS: %w", err)
			}
		}
		return &ActionResult{OK: true, Message: "TLS configured"}, nil
	}
}

func installerEnableBootstrapWindow(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		ttl := 30 * time.Minute
		if raw, ok := req.With["ttl"].(string); ok {
			if d, err := time.ParseDuration(raw); err == nil {
				ttl = d
			}
		}
		if cfg.EnableBootstrapWindow != nil {
			if err := cfg.EnableBootstrapWindow(ctx, ttl); err != nil {
				return nil, fmt.Errorf("enable bootstrap window: %w", err)
			}
		}
		return &ActionResult{OK: true, Message: fmt.Sprintf("bootstrap window enabled (ttl=%s)", ttl)}, nil
	}
}

func installerDisableBootstrapWindow(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.DisableBootstrapWindow != nil {
			if err := cfg.DisableBootstrapWindow(ctx); err != nil {
				return nil, fmt.Errorf("disable bootstrap window: %w", err)
			}
		}
		return &ActionResult{OK: true, Message: "bootstrap window disabled"}, nil
	}
}

func installerWriteBootstrapCreds(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.WriteBootstrapCreds != nil {
			if err := cfg.WriteBootstrapCreds(ctx); err != nil {
				return nil, fmt.Errorf("write bootstrap credentials: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func installerInstallPackage(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		name := fmt.Sprint(req.With["package"])
		if name == "" {
			return nil, fmt.Errorf("package name is required")
		}
		if cfg.InstallPackage != nil {
			if err := cfg.InstallPackage(ctx, name); err != nil {
				return nil, fmt.Errorf("install %s: %w", name, err)
			}
		}
		log.Printf("actor[installer]: installed %s", name)
		return &ActionResult{OK: true, Output: map[string]any{"package": name}}, nil
	}
}

func installerInstallPackageSet(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		raw, _ := req.With["packages"].([]any)
		if len(raw) == 0 {
			return &ActionResult{OK: true, Message: "no packages"}, nil
		}
		names := make([]string, 0, len(raw))
		for _, p := range raw {
			names = append(names, fmt.Sprint(p))
		}
		if cfg.InstallPackageSet != nil {
			if err := cfg.InstallPackageSet(ctx, names); err != nil {
				return nil, fmt.Errorf("install package set: %w", err)
			}
		}
		log.Printf("actor[installer]: installed package set: %s", strings.Join(names, ", "))
		return &ActionResult{OK: true, Output: map[string]any{"installed": len(names)}}, nil
	}
}

func installerInstallProfileSets(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		raw, _ := req.With["profiles"].([]any)
		if len(raw) == 0 {
			return &ActionResult{OK: true}, nil
		}
		profiles := make([]string, 0, len(raw))
		for _, p := range raw {
			profiles = append(profiles, fmt.Sprint(p))
		}
		if cfg.InstallProfileSets != nil {
			if err := cfg.InstallProfileSets(ctx, profiles); err != nil {
				return nil, fmt.Errorf("install profile sets: %w", err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{"profiles": profiles}}, nil
	}
}

func installerConfigureSharedStorage(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.ConfigureSharedStorage != nil {
			if err := cfg.ConfigureSharedStorage(ctx); err != nil {
				return nil, fmt.Errorf("configure shared storage: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func installerBootstrapDNS(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		domain := fmt.Sprint(req.Inputs["domain"])
		if cfg.BootstrapDNS != nil {
			if err := cfg.BootstrapDNS(ctx, domain); err != nil {
				return nil, fmt.Errorf("bootstrap DNS: %w", err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{"domain": domain}}, nil
	}
}

func installerValidateClusterHealth(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.ValidateClusterHealth != nil {
			if err := cfg.ValidateClusterHealth(ctx); err != nil {
				return nil, fmt.Errorf("cluster health validation: %w", err)
			}
		}
		return &ActionResult{OK: true, Message: "cluster healthy"}, nil
	}
}

func installerGenerateJoinToken(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.GenerateJoinToken == nil {
			return &ActionResult{OK: true, Message: "token generation not configured"}, nil
		}
		token, err := cfg.GenerateJoinToken(ctx)
		if err != nil {
			return nil, fmt.Errorf("generate join token: %w", err)
		}
		return &ActionResult{OK: true, Output: map[string]any{"token": token}}, nil
	}
}

func installerRestartServices(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		raw, _ := req.With["services"].([]any)
		if len(raw) == 0 {
			return &ActionResult{OK: true}, nil
		}
		services := make([]string, 0, len(raw))
		for _, s := range raw {
			services = append(services, fmt.Sprint(s))
		}
		if cfg.RestartServices != nil {
			if err := cfg.RestartServices(ctx, services); err != nil {
				return nil, fmt.Errorf("restart services: %w", err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{"restarted": len(services)}}, nil
	}
}

func installerClusterBootstrap(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		clusterID := fmt.Sprint(req.Inputs["cluster_id"])
		nodeID := fmt.Sprint(req.Inputs["bootstrap_node_id"])
		if cfg.ClusterBootstrap != nil {
			if err := cfg.ClusterBootstrap(ctx, clusterID, nodeID); err != nil {
				return nil, fmt.Errorf("cluster bootstrap: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func installerCaptureFailureBundle(cfg InstallerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		if cfg.CaptureFailureBundle != nil {
			cfg.CaptureFailureBundle(ctx, req.RunID)
		}
		return &ActionResult{OK: true}, nil
	}
}

// --------------------------------------------------------------------------
// Repository actions (Day-0 bootstrap)
// --------------------------------------------------------------------------

// RepositoryConfig provides dependencies for repository actor actions.
type RepositoryConfig struct {
	PublishBootstrapArtifacts func(ctx context.Context, source string) error
}

// RegisterRepositoryActions registers repository actor handlers.
func RegisterRepositoryActions(router *Router, cfg RepositoryConfig) {
	router.Register(v1alpha1.ActorRepository, "repository.publish_bootstrap_artifacts", repoPublishBootstrapArtifacts(cfg))
}

func repoPublishBootstrapArtifacts(cfg RepositoryConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		source := fmt.Sprint(req.With["source"])
		if cfg.PublishBootstrapArtifacts != nil {
			if err := cfg.PublishBootstrapArtifacts(ctx, source); err != nil {
				return nil, fmt.Errorf("publish bootstrap artifacts: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

// --------------------------------------------------------------------------
// Extended controller actions (release management + Day-0)
// --------------------------------------------------------------------------

// ReleaseControllerConfig provides dependencies for release-management
// controller actions used by release.apply.infrastructure and Day-0 workflows.
type ReleaseControllerConfig struct {
	// Release lifecycle
	MarkReleaseResolved  func(ctx context.Context, releaseID string) error
	MarkReleaseApplying  func(ctx context.Context, releaseID string) error
	MarkReleaseFailed    func(ctx context.Context, releaseID, reason string) error
	RecheckConvergence   func(ctx context.Context, releaseID string) error

	// Day-0 extras
	SeedDesiredFromInstalled func(ctx context.Context, clusterID string) error
	ReconcileUntilStable     func(ctx context.Context, clusterID string) error
	EmitBootstrapSucceeded   func(ctx context.Context, clusterID string) error

	// Generic package target selection (release.apply.package)
	SelectPackageTargets func(ctx context.Context, candidateNodes []any, pkgName, pkgKind, desiredHash string) ([]any, error)

	// Direct-apply infrastructure release (replaces plan-based path)
	SelectInfraTargets   func(ctx context.Context, candidateNodes []any, pkgName, desiredHash string) ([]any, error)
	FinalizeNoop         func(ctx context.Context, releaseID string) error
	MarkNodeStarted      func(ctx context.Context, releaseID, nodeID string) error
	MarkNodeSucceeded    func(ctx context.Context, releaseID, nodeID, version, hash string) error
	MarkNodeFailed       func(ctx context.Context, releaseID, nodeID, reason string) error
	AggregateDirectApply func(ctx context.Context, releaseID, pkgName string) (map[string]any, error)
	FinalizeDirectApply  func(ctx context.Context, releaseID string, aggregate map[string]any) error
}

// RegisterReleaseControllerActions registers release-management and Day-0
// controller actor handlers.
func RegisterReleaseControllerActions(router *Router, cfg ReleaseControllerConfig) {
	// Release lifecycle
	router.Register(v1alpha1.ActorClusterController, "controller.release.mark_resolved", releaseMarkResolved(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.release.mark_applying", releaseMarkApplying(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.release.mark_failed", releaseMarkFailed(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.release.recheck_convergence", releaseRecheckConvergence(cfg))

	// Day-0 extras
	router.Register(v1alpha1.ActorClusterController, "controller.seed_desired_from_installed", controllerSeedDesired(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.reconcile_until_stable", controllerReconcileUntilStable(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.emit_cluster_bootstrap_succeeded", controllerEmitBootstrapSucceeded(cfg))

	// Generic package target selection
	router.Register(v1alpha1.ActorClusterController, "controller.release.select_package_targets", releaseSelectPackageTargets(cfg))

	// Direct-apply infrastructure release actions
	router.Register(v1alpha1.ActorClusterController, "controller.release.select_infrastructure_targets", releaseSelectTargets(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.release.finalize_noop", releaseFinalizeNoop(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.release.mark_node_started", releaseMarkNodeStarted(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.release.mark_node_succeeded", releaseMarkNodeSucceeded(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.release.mark_node_failed", releaseMarkNodeFailed(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.release.aggregate_direct_apply_results", releaseAggregateDirectApply(cfg))
	router.Register(v1alpha1.ActorClusterController, "controller.release.finalize_direct_apply", releaseFinalizeDirectApply(cfg))
}

func releaseMarkResolved(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := fmt.Sprint(req.Inputs["release_id"])
		if cfg.MarkReleaseResolved != nil {
			if err := cfg.MarkReleaseResolved(ctx, releaseID); err != nil {
				return nil, fmt.Errorf("mark resolved: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func releaseMarkApplying(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := fmt.Sprint(req.Inputs["release_id"])
		if cfg.MarkReleaseApplying != nil {
			if err := cfg.MarkReleaseApplying(ctx, releaseID); err != nil {
				return nil, fmt.Errorf("mark applying: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func releaseMarkFailed(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := fmt.Sprint(req.Inputs["release_id"])
		reason := fmt.Sprint(req.With["reason"])
		if cfg.MarkReleaseFailed != nil {
			if err := cfg.MarkReleaseFailed(ctx, releaseID, reason); err != nil {
				return nil, fmt.Errorf("mark failed: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func releaseRecheckConvergence(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := fmt.Sprint(req.Inputs["release_id"])
		if cfg.RecheckConvergence != nil {
			if err := cfg.RecheckConvergence(ctx, releaseID); err != nil {
				return nil, fmt.Errorf("recheck convergence: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func controllerSeedDesired(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		clusterID := fmt.Sprint(req.Inputs["cluster_id"])
		if cfg.SeedDesiredFromInstalled != nil {
			if err := cfg.SeedDesiredFromInstalled(ctx, clusterID); err != nil {
				return nil, fmt.Errorf("seed desired from installed: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func controllerReconcileUntilStable(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		clusterID := fmt.Sprint(req.Inputs["cluster_id"])
		if cfg.ReconcileUntilStable != nil {
			if err := cfg.ReconcileUntilStable(ctx, clusterID); err != nil {
				return nil, fmt.Errorf("reconcile until stable: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func controllerEmitBootstrapSucceeded(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		clusterID := fmt.Sprint(req.Inputs["cluster_id"])
		if cfg.EmitBootstrapSucceeded != nil {
			if err := cfg.EmitBootstrapSucceeded(ctx, clusterID); err != nil {
				return nil, fmt.Errorf("emit bootstrap succeeded: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

// --------------------------------------------------------------------------

// --------------------------------------------------------------------------
// Direct-apply controller actions (workflow-native infrastructure release)
// --------------------------------------------------------------------------

func releaseSelectTargets(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		candidates, _ := req.With["candidate_nodes"].([]any)
		pkgName := fmt.Sprint(req.With["package_name"])
		desiredHash := fmt.Sprint(req.With["desired_hash"])
		var targets []any
		if cfg.SelectInfraTargets != nil {
			var err error
			targets, err = cfg.SelectInfraTargets(ctx, candidates, pkgName, desiredHash)
			if err != nil {
				return nil, fmt.Errorf("select targets: %w", err)
			}
		} else {
			targets = candidates
		}
		// Write targets directly to outputs so $.selected_targets resolves
		// to the array (not a wrapper map). The export mechanism would
		// store the whole Output map, but we need a bare []any.
		req.Outputs["selected_targets"] = targets
		return &ActionResult{OK: true, Output: map[string]any{"count": len(targets)}}, nil
	}
}

func releaseSelectPackageTargets(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		candidates, _ := req.With["candidate_nodes"].([]any)
		pkgName := fmt.Sprint(req.With["package_name"])
		pkgKind := fmt.Sprint(req.With["package_kind"])
		desiredHash := fmt.Sprint(req.With["desired_hash"])
		var targets []any
		if cfg.SelectPackageTargets != nil {
			var err error
			targets, err = cfg.SelectPackageTargets(ctx, candidates, pkgName, pkgKind, desiredHash)
			if err != nil {
				return nil, fmt.Errorf("select package targets: %w", err)
			}
		} else if cfg.SelectInfraTargets != nil {
			// Fall back to infra selector if no package-specific one.
			var err error
			targets, err = cfg.SelectInfraTargets(ctx, candidates, pkgName, desiredHash)
			if err != nil {
				return nil, fmt.Errorf("select targets: %w", err)
			}
		} else {
			targets = candidates
		}
		req.Outputs["selected_targets"] = targets
		return &ActionResult{OK: true, Output: map[string]any{"count": len(targets)}}, nil
	}
}

func releaseFinalizeNoop(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := fmt.Sprint(req.Inputs["release_id"])
		if cfg.FinalizeNoop != nil {
			if err := cfg.FinalizeNoop(ctx, releaseID); err != nil {
				return nil, fmt.Errorf("finalize noop: %w", err)
			}
		}
		log.Printf("actor[controller]: release %s finalized as AVAILABLE (no-op)", releaseID)
		return &ActionResult{OK: true}, nil
	}
}

func releaseMarkNodeStarted(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := fmt.Sprint(req.Inputs["release_id"])
		nodeID := fmt.Sprint(req.With["node_id"])
		if cfg.MarkNodeStarted != nil {
			if err := cfg.MarkNodeStarted(ctx, releaseID, nodeID); err != nil {
				return nil, fmt.Errorf("mark node started: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func releaseMarkNodeSucceeded(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := fmt.Sprint(req.Inputs["release_id"])
		nodeID := fmt.Sprint(req.With["node_id"])
		version := fmt.Sprint(req.With["version"])
		hash := fmt.Sprint(req.With["desired_hash"])
		if cfg.MarkNodeSucceeded != nil {
			if err := cfg.MarkNodeSucceeded(ctx, releaseID, nodeID, version, hash); err != nil {
				return nil, fmt.Errorf("mark node succeeded: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func releaseMarkNodeFailed(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := fmt.Sprint(req.Inputs["release_id"])
		nodeID := fmt.Sprint(req.With["node_id"])
		pkgName := fmt.Sprint(req.With["package_name"])
		reason := fmt.Sprintf("package %s failed on node %s", pkgName, nodeID)
		if cfg.MarkNodeFailed != nil {
			if err := cfg.MarkNodeFailed(ctx, releaseID, nodeID, reason); err != nil {
				return nil, fmt.Errorf("mark node failed: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func releaseAggregateDirectApply(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := fmt.Sprint(req.Inputs["release_id"])
		pkgName := fmt.Sprint(req.With["package_name"])
		if cfg.AggregateDirectApply != nil {
			agg, err := cfg.AggregateDirectApply(ctx, releaseID, pkgName)
			if err != nil {
				return nil, fmt.Errorf("aggregate: %w", err)
			}
			return &ActionResult{OK: true, Output: agg}, nil
		}
		return &ActionResult{OK: true}, nil
	}
}

func releaseFinalizeDirectApply(cfg ReleaseControllerConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		releaseID := fmt.Sprint(req.Inputs["release_id"])
		aggregate, _ := req.With["aggregate"].(map[string]any)
		if aggregate == nil {
			if agg, ok := req.Outputs["aggregate"].(map[string]any); ok {
				aggregate = agg
			}
		}
		if cfg.FinalizeDirectApply != nil {
			if err := cfg.FinalizeDirectApply(ctx, releaseID, aggregate); err != nil {
				return nil, fmt.Errorf("finalize direct apply: %w", err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

// --------------------------------------------------------------------------
// Node-agent direct-apply actions (workflow-native infrastructure release)
// --------------------------------------------------------------------------

// NodeDirectApplyConfig provides dependencies for direct node-agent package operations.
type NodeDirectApplyConfig struct {
	InstallPackage         func(ctx context.Context, name, version, kind string) error
	VerifyPackageInstalled func(ctx context.Context, name, version, hash string) error
	RestartPackageService  func(ctx context.Context, name string) error
	MaybeRestartPackage    func(ctx context.Context, name, kind, restartPolicy string) error
	VerifyPackageRuntime   func(ctx context.Context, name, healthCheck string) error
	SyncInstalledPackage   func(ctx context.Context, name, version, hash string) error

	// Removal actions (release.remove.package workflow)
	StopPackageService        func(ctx context.Context, name string) error
	DisablePackageService     func(ctx context.Context, name string) error
	UninstallPackage          func(ctx context.Context, name, kind string) error
	ClearInstalledPackageState func(ctx context.Context, name, kind string) error
}

// RegisterNodeDirectApplyActions registers direct package operation handlers.
func RegisterNodeDirectApplyActions(router *Router, cfg NodeDirectApplyConfig) {
	router.Register(v1alpha1.ActorNodeAgent, "node.install_package", nodeInstallPackage(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.verify_package_installed", nodeVerifyPackage(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.restart_package_service", nodeRestartService(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.maybe_restart_package", nodeMaybeRestartPackage(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.verify_package_runtime", nodeVerifyRuntime(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.sync_installed_package_state", nodeSyncPackageState(cfg))

	// Removal actions
	router.Register(v1alpha1.ActorNodeAgent, "node.stop_package_service", nodeStopPackageService(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.disable_package_service", nodeDisablePackageService(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.uninstall_package", nodeUninstallPackage(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.clear_installed_package_state", nodeClearInstalledPackageState(cfg))
}

// nodeContextKey is the context key for per-node metadata (node_id, agent_endpoint).
type nodeContextKey struct{}

// NodeContext carries per-node metadata through action handler contexts.
type NodeContext struct {
	NodeID        string
	AgentEndpoint string
}

// WithNodeContext attaches node metadata to a context.
func WithNodeContext(ctx context.Context, nc NodeContext) context.Context {
	return context.WithValue(ctx, nodeContextKey{}, nc)
}

// GetNodeContext extracts node metadata from a context.
func GetNodeContext(ctx context.Context) (NodeContext, bool) {
	nc, ok := ctx.Value(nodeContextKey{}).(NodeContext)
	return nc, ok
}

// enrichNodeContext extracts node_id and agent_endpoint from the action
// request and attaches them to the context for callbacks.
func enrichNodeContext(ctx context.Context, req ActionRequest) context.Context {
	nodeID := ""
	endpoint := ""
	// Try With first (explicit step params), then Inputs (foreach item).
	if v, ok := req.With["node_id"]; ok {
		nodeID = fmt.Sprint(v)
	} else if v, ok := req.Inputs["node_id"]; ok {
		nodeID = fmt.Sprint(v)
	}
	if v, ok := req.With["agent_endpoint"]; ok {
		endpoint = fmt.Sprint(v)
	} else if v, ok := req.Inputs["agent_endpoint"]; ok {
		endpoint = fmt.Sprint(v)
	} else if v, ok := req.Inputs["target.agent_endpoint"]; ok {
		endpoint = fmt.Sprint(v)
	}
	return WithNodeContext(ctx, NodeContext{NodeID: nodeID, AgentEndpoint: endpoint})
}

func nodeInstallPackage(cfg NodeDirectApplyConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		ctx = enrichNodeContext(ctx, req)
		name := fmt.Sprint(req.With["package_name"])
		version := fmt.Sprint(req.With["version"])
		kind := fmt.Sprint(req.With["kind"])
		if cfg.InstallPackage != nil {
			if err := cfg.InstallPackage(ctx, name, version, kind); err != nil {
				return nil, fmt.Errorf("install %s: %w", name, err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{"package": name, "version": version}}, nil
	}
}

func nodeVerifyPackage(cfg NodeDirectApplyConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		ctx = enrichNodeContext(ctx, req)
		name := fmt.Sprint(req.With["package_name"])
		version := fmt.Sprint(req.With["version"])
		hash := fmt.Sprint(req.With["desired_hash"])
		if cfg.VerifyPackageInstalled != nil {
			if err := cfg.VerifyPackageInstalled(ctx, name, version, hash); err != nil {
				return nil, fmt.Errorf("verify %s: %w", name, err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{"verified": true}}, nil
	}
}

func nodeRestartService(cfg NodeDirectApplyConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		ctx = enrichNodeContext(ctx, req)
		name := fmt.Sprint(req.With["package_name"])
		if cfg.RestartPackageService != nil {
			if err := cfg.RestartPackageService(ctx, name); err != nil {
				return nil, fmt.Errorf("restart %s: %w", name, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeMaybeRestartPackage(cfg NodeDirectApplyConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		ctx = enrichNodeContext(ctx, req)
		name := fmt.Sprint(req.With["package_name"])
		kind := fmt.Sprint(req.With["package_kind"])
		policy := fmt.Sprint(req.With["restart_policy"])
		if cfg.MaybeRestartPackage != nil {
			if err := cfg.MaybeRestartPackage(ctx, name, kind, policy); err != nil {
				return nil, fmt.Errorf("maybe restart %s: %w", name, err)
			}
		} else if cfg.RestartPackageService != nil {
			// Fall back to unconditional restart for INFRASTRUCTURE/SERVICE kinds.
			if kind == "COMMAND" {
				return &ActionResult{OK: true, Message: "skip restart for COMMAND"}, nil
			}
			if err := cfg.RestartPackageService(ctx, name); err != nil {
				return nil, fmt.Errorf("restart %s: %w", name, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeVerifyRuntime(cfg NodeDirectApplyConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		ctx = enrichNodeContext(ctx, req)
		name := fmt.Sprint(req.With["package_name"])
		check := fmt.Sprint(req.With["health_check"])
		if cfg.VerifyPackageRuntime != nil {
			if err := cfg.VerifyPackageRuntime(ctx, name, check); err != nil {
				return nil, fmt.Errorf("verify runtime %s: %w", name, err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{"healthy": true}}, nil
	}
}

func nodeSyncPackageState(cfg NodeDirectApplyConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		ctx = enrichNodeContext(ctx, req)
		name := fmt.Sprint(req.With["package_name"])
		version := fmt.Sprint(req.With["version"])
		hash := fmt.Sprint(req.With["desired_hash"])
		if cfg.SyncInstalledPackage != nil {
			if err := cfg.SyncInstalledPackage(ctx, name, version, hash); err != nil {
				return nil, fmt.Errorf("sync state %s: %w", name, err)
			}
		}
		return &ActionResult{OK: true, Output: map[string]any{"synced": true}}, nil
	}
}

// --------------------------------------------------------------------------
// Package removal actions (release.remove.package workflow)
// --------------------------------------------------------------------------

func nodeStopPackageService(cfg NodeDirectApplyConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		ctx = enrichNodeContext(ctx, req)
		name := fmt.Sprint(req.With["package_name"])
		if cfg.StopPackageService != nil {
			if err := cfg.StopPackageService(ctx, name); err != nil {
				return nil, fmt.Errorf("stop %s: %w", name, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeDisablePackageService(cfg NodeDirectApplyConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		ctx = enrichNodeContext(ctx, req)
		name := fmt.Sprint(req.With["package_name"])
		if cfg.DisablePackageService != nil {
			if err := cfg.DisablePackageService(ctx, name); err != nil {
				return nil, fmt.Errorf("disable %s: %w", name, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeUninstallPackage(cfg NodeDirectApplyConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		ctx = enrichNodeContext(ctx, req)
		name := fmt.Sprint(req.With["package_name"])
		kind := fmt.Sprint(req.With["package_kind"])
		if cfg.UninstallPackage != nil {
			if err := cfg.UninstallPackage(ctx, name, kind); err != nil {
				return nil, fmt.Errorf("uninstall %s: %w", name, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

func nodeClearInstalledPackageState(cfg NodeDirectApplyConfig) ActionHandler {
	return func(ctx context.Context, req ActionRequest) (*ActionResult, error) {
		ctx = enrichNodeContext(ctx, req)
		name := fmt.Sprint(req.With["package_name"])
		kind := fmt.Sprint(req.With["package_kind"])
		if cfg.ClearInstalledPackageState != nil {
			if err := cfg.ClearInstalledPackageState(ctx, name, kind); err != nil {
				return nil, fmt.Errorf("clear state %s: %w", name, err)
			}
		}
		return &ActionResult{OK: true}, nil
	}
}

// --------------------------------------------------------------------------
// Default implementations
// --------------------------------------------------------------------------

// DefaultIsServiceActive checks if a service is ready to accept traffic.
// For most services: systemctl is-active. For ScyllaDB: port 9042 probe
// (systemd shows "active" during Raft join but CQL isn't ready yet).
func DefaultIsServiceActive(name string) bool {
	switch name {
	case "scylladb":
		// ScyllaDB: probe CQL native transport port directly.
		// Try localhost first, then all local IPs (ScyllaDB binds to
		// the node's advertised IP, not 127.0.0.1).
		for _, host := range scyllaProbeHosts() {
			conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, "9042"), 2*time.Second)
			if err == nil {
				conn.Close()
				return true
			}
		}
		return false
	default:
		unit := "globular-" + name + ".service"
		switch name {
		case "etcd":
			unit = "globular-etcd.service"
		}
		out, err := exec.Command("systemctl", "is-active", unit).Output()
		return err == nil && strings.TrimSpace(string(out)) == "active"
	}
}

// scyllaProbeHosts returns candidate hosts for ScyllaDB CQL probe.
// ScyllaDB typically binds to the node's advertised IP, not 127.0.0.1.
func scyllaProbeHosts() []string {
	var hosts []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return hosts
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				hosts = append(hosts, ipnet.IP.String())
			}
		}
	}
	return hosts
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func extractPackageList(with map[string]any) ([]PackageRef, error) {
	raw, ok := with["packages"]
	if !ok {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("packages must be a list, got %T", raw)
	}
	var pkgs []PackageRef
	for _, item := range list {
		switch v := item.(type) {
		case map[string]any:
			name, _ := v["name"].(string)
			kind, _ := v["kind"].(string)
			if name == "" {
				continue
			}
			pkgs = append(pkgs, PackageRef{Name: name, Kind: kind})
		case string:
			pkgs = append(pkgs, PackageRef{Name: v, Kind: "SERVICE"})
		}
	}
	return pkgs, nil
}

func packageNames(pkgs []PackageRef) string {
	names := make([]string, len(pkgs))
	for i, p := range pkgs {
		names[i] = p.Name
	}
	return strings.Join(names, ", ")
}
