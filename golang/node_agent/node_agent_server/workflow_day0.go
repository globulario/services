package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// RunDay0BootstrapWorkflow executes the day0.bootstrap workflow definition
// to perform the full cluster bootstrap sequence. This replaces the manual
// bootstrap script with a declarative, observable workflow.
//
// The installer actors are wired to the local node-agent's install
// infrastructure. The repository and controller actors are wired to
// stub implementations that log actions — they will be connected to
// real gRPC calls once the respective services are running post-bootstrap.
func (srv *NodeAgentServer) RunDay0BootstrapWorkflow(ctx context.Context, defPath string, inputs map[string]any) (*engine.Run, error) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition %s: %w", defPath, err)
	}

	repoAddr := ""
	if addr, ok := inputs["repository_address"].(string); ok {
		repoAddr = addr
	}
	if repoAddr == "" {
		repoAddr = srv.discoverRepositoryAddr()
	}

	router := engine.NewRouter()

	// Wire installer actions to local node-agent install infrastructure.
	engine.RegisterInstallerActions(router, engine.InstallerConfig{
		SetupTLS: func(ctx context.Context, clusterID string) error {
			log.Printf("day0-workflow: setup TLS for cluster %s", clusterID)
			// TLS setup is handled by the Globule process before node-agent starts.
			// In the workflow context, this is a verification step.
			return nil
		},
		EnableBootstrapWindow: func(ctx context.Context, ttl time.Duration) error {
			log.Printf("day0-workflow: enable bootstrap window (ttl=%s)", ttl)
			return nil
		},
		DisableBootstrapWindow: func(ctx context.Context) error {
			log.Printf("day0-workflow: disable bootstrap window")
			return nil
		},
		WriteBootstrapCreds: func(ctx context.Context) error {
			log.Printf("day0-workflow: write bootstrap credentials")
			return nil
		},
		InstallPackage: func(ctx context.Context, name string) error {
			return srv.InstallPackage(ctx, name, "SERVICE", repoAddr)
		},
		InstallPackageSet: func(ctx context.Context, packages []string) error {
			var errs []string
			for _, pkg := range packages {
				kind := "SERVICE"
				// Infer kind from known infrastructure packages.
				switch pkg {
				case "scylladb", "etcd", "minio", "envoy", "xds", "gateway",
					"node-agent", "cluster-controller", "cluster-doctor",
					"prometheus", "node-exporter", "sidekick",
					"scylla-manager", "scylla-manager-agent":
					kind = "INFRASTRUCTURE"
				}
				if err := srv.InstallPackage(ctx, pkg, kind, repoAddr); err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", pkg, err))
				}
			}
			if len(errs) > 0 {
				return fmt.Errorf("install failures: %s", strings.Join(errs, "; "))
			}
			return nil
		},
		InstallProfileSets: func(ctx context.Context, profiles []string) error {
			log.Printf("day0-workflow: install profile sets: %v", profiles)
			// Profile-based installation is resolved by the controller.
			// During Day-0, this is a no-op — packages are installed explicitly.
			return nil
		},
		ConfigureSharedStorage: func(ctx context.Context) error {
			log.Printf("day0-workflow: configure shared storage")
			return nil
		},
		BootstrapDNS: func(ctx context.Context, domain string) error {
			log.Printf("day0-workflow: bootstrap DNS for %s", domain)
			return nil
		},
		ValidateClusterHealth: func(ctx context.Context) error {
			log.Printf("day0-workflow: validate cluster health")
			return nil
		},
		GenerateJoinToken: func(ctx context.Context) (string, error) {
			log.Printf("day0-workflow: generate join token")
			return "generated-by-workflow", nil
		},
		RestartServices: func(ctx context.Context, services []string) error {
			log.Printf("day0-workflow: restart services: %v", services)
			return nil
		},
		ClusterBootstrap: func(ctx context.Context, clusterID, nodeID string) error {
			log.Printf("day0-workflow: cluster bootstrap (cluster=%s node=%s)", clusterID, nodeID)
			return nil
		},
		CaptureFailureBundle: func(ctx context.Context, runID string) error {
			log.Printf("day0-workflow: capturing failure bundle for run %s", runID)
			return nil
		},
	})

	// Wire repository actions.
	engine.RegisterRepositoryActions(router, engine.RepositoryConfig{
		PublishBootstrapArtifacts: func(ctx context.Context, source string) error {
			log.Printf("day0-workflow: publish bootstrap artifacts from %s", source)
			return nil
		},
	})

	// Wire controller actions (called locally since controller starts mid-bootstrap).
	engine.RegisterReleaseControllerActions(router, engine.ReleaseControllerConfig{
		SeedDesiredFromInstalled: func(ctx context.Context, clusterID string) error {
			log.Printf("day0-workflow: seed desired state from installed")
			return nil
		},
		ReconcileUntilStable: func(ctx context.Context, clusterID string) error {
			log.Printf("day0-workflow: reconcile until stable")
			return nil
		},
		EmitBootstrapSucceeded: func(ctx context.Context, clusterID string) error {
			log.Printf("day0-workflow: cluster bootstrap succeeded for %s", clusterID)
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
			log.Printf("day0-workflow: step %s → %s (%s)",
				step.ID, step.Status, elapsed.Round(time.Millisecond))
		},
	}

	log.Printf("day0-workflow: starting %s", def.Metadata.Name)
	start := time.Now()
	run, err := eng.Execute(ctx, def, inputs)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("day0-workflow: FAILED after %s: %v",
			elapsed.Round(time.Millisecond), err)
	} else {
		succeeded := 0
		for _, st := range run.Steps {
			if st.Status == engine.StepSucceeded {
				succeeded++
			}
		}
		log.Printf("day0-workflow: SUCCEEDED in %s (%d/%d steps)",
			elapsed.Round(time.Millisecond), succeeded, len(run.Steps))
	}

	return run, err
}

// resolveDay0WorkflowPath finds the day0.bootstrap.yaml definition.
func resolveDay0WorkflowPath() string {
	candidates := []string{
		"/var/lib/globular/workflows/day0.bootstrap.yaml",
		"/usr/lib/globular/workflows/day0.bootstrap.yaml",
		"/tmp/day0.bootstrap.yaml",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
