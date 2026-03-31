package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/globulario/services/golang/workflow_redesign_pkg/go/engine"
	"github.com/globulario/services/golang/workflow_redesign_pkg/go/v1alpha1"
)

// RunJoinWorkflow executes the node.join workflow definition to install
// all packages on this node. This replaces the one-plan-per-cycle
// reconciler with a single workflow run that installs packages in
// tiered parallel batches.
func (srv *NodeAgentServer) RunJoinWorkflow(ctx context.Context, defPath string, inputs map[string]any) (*engine.Run, error) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition %s: %w", defPath, err)
	}

	router := engine.NewRouter()

	// Wire node-agent actions.
	repoAddr := ""
	if srv.state != nil && srv.state.ControllerEndpoint != "" {
		repoAddr = srv.discoverRepositoryAddr()
	}

	engine.RegisterNodeAgentActions(router, engine.NodeAgentConfig{
		NodeID: srv.nodeID,
		FetchAndInstall: func(ctx context.Context, pkg engine.PackageRef) error {
			return srv.InstallPackage(ctx, pkg.Name, pkg.Kind, repoAddr)
		},
		IsServiceActive: func(name string) bool {
			return engine.DefaultIsServiceActive(name)
		},
		SyncInstalledState: func(ctx context.Context) error {
			srv.syncInstalledStateToEtcd(ctx)
			return nil
		},
	})

	// Wire controller actions (called remotely via the controller client).
	engine.RegisterControllerActions(router, engine.ControllerConfig{
		SetBootstrapPhase: func(ctx context.Context, nodeID, phase string) error {
			log.Printf("workflow-runner: would set phase %s for %s (not yet wired)", phase, nodeID)
			return nil
		},
		EmitEvent: func(ctx context.Context, eventType string, data map[string]any) error {
			log.Printf("workflow-runner: event %s %v", eventType, data)
			return nil
		},
	})

	// Condition evaluator for contains() expressions.
	evalCond := func(ctx context.Context, expr string, inputs, outputs map[string]any) (bool, error) {
		if strings.HasPrefix(expr, "contains(inputs.node_profiles,") {
			profiles, ok := inputs["node_profiles"].([]any)
			if !ok {
				return false, nil
			}
			parts := strings.SplitN(expr, "'", 3)
			if len(parts) < 2 {
				return false, nil
			}
			target := parts[1]
			for _, p := range profiles {
				if fmt.Sprint(p) == target {
					return true, nil
				}
			}
			return false, nil
		}
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
			log.Printf("workflow-runner: step %s → %s (%s)",
				step.ID, step.Status, elapsed.Round(time.Millisecond))
		},
	}

	log.Printf("workflow-runner: starting %s for node %s — disabling plan-runner", def.Metadata.Name, srv.nodeID)
	atomic.StoreInt32(&srv.workflowRunning, 1)
	defer atomic.StoreInt32(&srv.workflowRunning, 0)

	start := time.Now()
	run, err := eng.Execute(ctx, def, inputs)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("workflow-runner: %s FAILED after %s: %v",
			def.Metadata.Name, elapsed.Round(time.Millisecond), err)
	} else {
		succeeded := 0
		for _, st := range run.Steps {
			if st.Status == engine.StepSucceeded {
				succeeded++
			}
		}
		log.Printf("workflow-runner: %s SUCCEEDED in %s (%d/%d steps)",
			def.Metadata.Name, elapsed.Round(time.Millisecond), succeeded, len(run.Steps))
	}

	return run, err
}

// StartWorkflowSignalHandler listens for SIGUSR1 and triggers the
// node.join workflow. This is a temporary mechanism for testing — in
// production, the controller will trigger workflows via gRPC.
//
// Usage: kill -USR1 <node-agent-pid>
func (srv *NodeAgentServer) StartWorkflowSignalHandler(ctx context.Context) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				log.Printf("workflow-runner: SIGUSR1 received — starting node.join workflow")

				// Look for the definition in standard locations.
				defPath := ""
				for _, p := range []string{
					"/var/lib/globular/workflows/node.join.yaml",
					"/tmp/node.join.yaml",
				} {
					if _, err := os.Stat(p); err == nil {
						defPath = p
						break
					}
				}
				if defPath == "" {
					log.Printf("workflow-runner: node.join.yaml not found in /var/lib/globular/workflows/ or /tmp/")
					continue
				}

				inputs := map[string]any{
					"cluster_id":    "globular.internal",
					"node_id":       srv.nodeID,
					"node_hostname": srv.state.NodeName,
					"node_ip":       srv.state.AdvertiseIP,
				}

				go func() {
					wfCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
					defer cancel()
					srv.RunJoinWorkflow(wfCtx, defPath, inputs)
				}()
			}
		}
	}()
}
