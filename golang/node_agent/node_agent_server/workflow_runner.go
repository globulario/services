package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// RunWorkflowDefinition executes a workflow definition (e.g. node.join) to
// install all packages on this node. This replaces the one-plan-per-cycle
// reconciler with a single workflow run that installs packages in
// tiered parallel batches.
func (srv *NodeAgentServer) RunWorkflowDefinition(ctx context.Context, defPath string, inputs map[string]any) (*engine.Run, error) {
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
		ProbeInfraHealth: func(ctx context.Context, probeName string) bool {
			resp, err := srv.RunWorkflow(ctx, &node_agentpb.RunWorkflowRequest{
				WorkflowName: probeName,
			})
			return err == nil && resp.GetStatus() == "SUCCEEDED"
		},
	})

	// Wire controller actions (called remotely via the controller client).
	engine.RegisterControllerActions(router, engine.ControllerConfig{
		SetBootstrapPhase: func(ctx context.Context, nodeID, phase string) error {
			if err := srv.ensureControllerClient(ctx); err != nil {
				return fmt.Errorf("controller client: %w", err)
			}
			rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			_, err := srv.controllerClient.SetNodeBootstrapPhase(rpcCtx, &cluster_controllerpb.SetNodeBootstrapPhaseRequest{
				NodeId: nodeID,
				Phase:  phase,
			})
			if err != nil {
				return fmt.Errorf("set bootstrap phase %s for %s: %w", phase, nodeID, err)
			}
			log.Printf("workflow-runner: set phase %s for %s", phase, nodeID)
			return nil
		},
		EmitEvent: func(ctx context.Context, eventType string, data map[string]any) error {
			if err := srv.ensureControllerClient(ctx); err != nil {
				return fmt.Errorf("controller client: %w", err)
			}
			// Convert map[string]any → map[string]string (best-effort).
			strData := make(map[string]string, len(data))
			for k, v := range data {
				strData[k] = fmt.Sprint(v)
			}
			rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			_, err := srv.controllerClient.EmitWorkflowEvent(rpcCtx, &cluster_controllerpb.EmitWorkflowEventRequest{
				EventType: eventType,
				Data:      strData,
			})
			if err != nil {
				return fmt.Errorf("emit event %s: %w", eventType, err)
			}
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

	log.Printf("workflow-runner: starting %s for node %s", def.Metadata.Name, srv.nodeID)

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

