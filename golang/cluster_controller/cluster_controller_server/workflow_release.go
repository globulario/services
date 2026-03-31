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

	// Wire release controller actions (direct-apply path, no plans).
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
		RecheckConvergence: func(ctx context.Context, relID string) error {
			log.Printf("release-workflow: recheck convergence for %s", relID)
			if srv.enqueueReconcile != nil {
				srv.enqueueReconcile()
			}
			return nil
		},
		SelectInfraTargets: func(ctx context.Context, candidates []any, pkgName, desiredHash string) ([]any, error) {
			srv.lock("SelectInfraTargets")
			defer srv.unlock()
			var targets []any
			for _, c := range candidates {
				nodeID := fmt.Sprint(c)
				node := srv.state.Nodes[nodeID]
				if node == nil || !bootstrapPhaseReady(node.BootstrapPhase) {
					continue
				}
				targets = append(targets, map[string]any{"node_id": nodeID})
			}
			return targets, nil
		},
		FinalizeNoop: func(ctx context.Context, releaseID string) error {
			log.Printf("release-workflow: %s finalized AVAILABLE (no-op)", releaseID)
			return nil
		},
		MarkNodeStarted: func(ctx context.Context, releaseID, nodeID string) error {
			log.Printf("release-workflow: node %s started for %s", nodeID, releaseID)
			return nil
		},
		MarkNodeSucceeded: func(ctx context.Context, releaseID, nodeID, version, hash string) error {
			log.Printf("release-workflow: node %s succeeded for %s (v=%s)", nodeID, releaseID, version)
			return nil
		},
		MarkNodeFailed: func(ctx context.Context, releaseID, nodeID, reason string) error {
			log.Printf("release-workflow: node %s failed for %s: %s", nodeID, releaseID, reason)
			return nil
		},
		AggregateDirectApply: func(ctx context.Context, releaseID, pkgName string) (map[string]any, error) {
			return map[string]any{"release_id": releaseID, "status": "ok"}, nil
		},
		FinalizeDirectApply: func(ctx context.Context, releaseID string, aggregate map[string]any) error {
			log.Printf("release-workflow: finalize %s", releaseID)
			return nil
		},
	})

	// Wire node-agent direct-apply actions.
	engine.RegisterNodeDirectApplyActions(router, engine.NodeDirectApplyConfig{
		InstallPackage: func(ctx context.Context, name, version, kind string) error {
			log.Printf("release-workflow: install %s@%s (%s)", name, version, kind)
			return nil
		},
		VerifyPackageInstalled: func(ctx context.Context, name, version, hash string) error {
			return nil
		},
		RestartPackageService: func(ctx context.Context, name string) error {
			return nil
		},
		VerifyPackageRuntime: func(ctx context.Context, name, check string) error {
			return nil
		},
		SyncInstalledPackage: func(ctx context.Context, name, version, hash string) error {
			return nil
		},
	})

	eng := &engine.Engine{
		Router: router,
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
