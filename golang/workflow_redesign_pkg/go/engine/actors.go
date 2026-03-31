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

	"github.com/globulario/services/golang/workflow_redesign_pkg/go/v1alpha1"
)

// RegisterNodeAgentActions registers all node-agent actor handlers.
// These wrap the actual install/verify/sync operations on the local node.
func RegisterNodeAgentActions(router *Router, cfg NodeAgentConfig) {
	router.Register(v1alpha1.ActorNodeAgent, "node.install_packages", nodeInstallPackages(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.verify_services_active", nodeVerifyServicesActive(cfg))
	router.Register(v1alpha1.ActorNodeAgent, "node.sync_installed_state", nodeSyncInstalledState(cfg))
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
// Default implementations
// --------------------------------------------------------------------------

// DefaultIsServiceActive checks if a service is ready to accept traffic.
// For most services: systemctl is-active. For ScyllaDB: port 9042 probe
// (systemd shows "active" during Raft join but CQL isn't ready yet).
func DefaultIsServiceActive(name string) bool {
	switch name {
	case "scylladb":
		// ScyllaDB: probe CQL native transport port directly.
		conn, err := net.DialTimeout("tcp", "127.0.0.1:9042", 2*time.Second)
		if err != nil {
			return false
		}
		conn.Close()
		return true
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
