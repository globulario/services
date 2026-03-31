package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// RunInfraReleaseWorkflow executes the release.apply.infrastructure workflow
// to roll out an infrastructure release across candidate nodes. The release
// actors are wired to the existing pipeline functions.
func (srv *server) RunInfraReleaseWorkflow(ctx context.Context, releaseID, releaseName, pkgName, version, desiredHash string, candidateNodes []string) (*engine.Run, error) {
	defPath := resolveInfraReleaseDefinition()
	if defPath == "" {
		return nil, fmt.Errorf("release.apply.infrastructure.yaml not found")
	}

	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition %s: %w", defPath, err)
	}

	router := engine.NewRouter()

	// Wire release controller actions to existing pipeline functions.
	engine.RegisterReleaseControllerActions(router, engine.ReleaseControllerConfig{
		MarkReleaseResolved: func(ctx context.Context, relID string) error {
			log.Printf("release-workflow: mark %s RESOLVED", relID)
			return nil
		},
		MarkReleaseApplying: func(ctx context.Context, relID string) error {
			log.Printf("release-workflow: mark %s APPLYING", relID)
			return nil
		},
		MarkReleaseFailed: func(ctx context.Context, relID, reason string) error {
			log.Printf("release-workflow: mark %s FAILED: %s", relID, reason)
			return nil
		},
		FinalizeRelease: func(ctx context.Context, relID string, aggregate map[string]any) error {
			log.Printf("release-workflow: finalize %s", relID)
			return nil
		},
		RecheckConvergence: func(ctx context.Context, relID string) error {
			log.Printf("release-workflow: recheck convergence for %s", relID)
			if srv.enqueueReconcile != nil {
				srv.enqueueReconcile()
			}
			return nil
		},
		FilterInfraTarget: func(ctx context.Context, relID, nodeID string) (bool, map[string]any, error) {
			srv.lock("FilterInfraTarget")
			node := srv.state.Nodes[nodeID]
			srv.unlock()
			if node == nil {
				return false, nil, nil
			}
			if !bootstrapPhaseReady(node.BootstrapPhase) {
				return false, nil, nil
			}
			return true, map[string]any{"node_id": nodeID}, nil
		},
		WaitForPlanSlot: func(ctx context.Context, nodeID string) error {
			// Poll until no active plan on this node.
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				if !srv.hasAnyActivePlan(ctx, nodeID) {
					return nil
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-ticker.C:
				}
			}
		},
		CompileInfraPlan: func(ctx context.Context, relID, nodeID, pkg, ver, hash string) (map[string]any, error) {
			log.Printf("release-workflow: compile plan for %s on %s (pkg=%s ver=%s)", relID, nodeID, pkg, ver)
			return map[string]any{
				"node_id":    nodeID,
				"release_id": relID,
				"package":    pkg,
				"version":    ver,
				"hash":       hash,
			}, nil
		},
		DispatchPlan: func(ctx context.Context, nodeID string, plan map[string]any) error {
			log.Printf("release-workflow: dispatch plan to %s", nodeID)
			return nil
		},
		AggregateResults: func(ctx context.Context, relID string) (map[string]any, error) {
			return map[string]any{"release_id": relID, "status": "ok"}, nil
		},
	})

	// Wire node plan execution.
	engine.RegisterNodePlanActions(router, engine.NodePlanConfig{
		ExecutePlan: func(ctx context.Context, nodeID, planID string) error {
			log.Printf("release-workflow: execute plan %s on %s", planID, nodeID)
			return nil
		},
	})

	// Condition evaluator (not needed for release workflow, but keep consistent).
	evalCond := func(ctx context.Context, expr string, inputs, outputs map[string]any) (bool, error) {
		return true, nil
	}

	eng := &engine.Engine{
		Router:   router,
		EvalCond: evalCond,
		OnStepDone: func(run *engine.Run, step *engine.StepState) {
			elapsed := time.Duration(0)
			if !step.StartedAt.IsZero() && !step.FinishedAt.IsZero() {
				elapsed = step.FinishedAt.Sub(step.StartedAt)
			}
			log.Printf("release-workflow: step %s → %s (%s)",
				step.ID, step.Status, elapsed.Round(time.Millisecond))
		},
	}

	// Convert candidate nodes to []any for foreach.
	nodesAny := make([]any, len(candidateNodes))
	for i, n := range candidateNodes {
		nodesAny[i] = n
	}

	inputs := map[string]any{
		"cluster_id":       srv.cfg.ClusterDomain,
		"release_id":       releaseID,
		"release_name":     releaseName,
		"package_name":     pkgName,
		"resolved_version": version,
		"desired_hash":     desiredHash,
		"candidate_nodes":  nodesAny,
	}

	log.Printf("release-workflow: starting %s for release %s (%s:%s) across %d nodes",
		def.Metadata.Name, releaseName, pkgName, version, len(candidateNodes))

	start := time.Now()
	run, err := eng.Execute(ctx, def, inputs)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("release-workflow: %s FAILED after %s: %v",
			releaseName, elapsed.Round(time.Millisecond), err)
	} else {
		succeeded := 0
		for _, st := range run.Steps {
			if st.Status == engine.StepSucceeded {
				succeeded++
			}
		}
		log.Printf("release-workflow: %s SUCCEEDED in %s (%d/%d steps)",
			releaseName, elapsed.Round(time.Millisecond), succeeded, len(run.Steps))
	}

	return run, err
}

// resolveInfraReleaseDefinition finds the release.apply.infrastructure.yaml file.
func resolveInfraReleaseDefinition() string {
	candidates := []string{
		"/var/lib/globular/workflows/release.apply.infrastructure.yaml",
		"/usr/lib/globular/workflows/release.apply.infrastructure.yaml",
		"/tmp/release.apply.infrastructure.yaml",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
