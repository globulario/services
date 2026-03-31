package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"os"

	"github.com/globulario/services/golang/workflow_redesign_pkg/go/engine"
	"github.com/globulario/services/golang/workflow_redesign_pkg/go/v1alpha1"
)

// RunBootstrapWorkflow executes the node.bootstrap workflow definition to
// advance a node from admitted → workload_ready. The controller actors
// are wired to the in-memory node state map so that set_phase and
// wait_condition operate on live state.
func (srv *server) RunBootstrapWorkflow(ctx context.Context, nodeID string) (*engine.Run, error) {
	defPath := resolveBootstrapDefinition()
	if defPath == "" {
		return nil, fmt.Errorf("node.bootstrap.yaml not found")
	}

	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition %s: %w", defPath, err)
	}

	// Snapshot node profiles under lock.
	srv.lock("RunBootstrapWorkflow:snapshot")
	node := srv.state.Nodes[nodeID]
	if node == nil {
		srv.unlock()
		return nil, fmt.Errorf("node %s not found", nodeID)
	}
	profiles := append([]string(nil), node.Profiles...)
	hostname := node.Identity.Hostname
	srv.unlock()

	router := engine.NewRouter()

	// Wire controller bootstrap actions to in-memory node state.
	engine.RegisterControllerActions(router, engine.ControllerConfig{
		SetBootstrapPhase: func(ctx context.Context, nID, phase string) error {
			return srv.setBootstrapPhase(nID, phase)
		},
		EmitEvent: func(ctx context.Context, eventType string, data map[string]any) error {
			payload := make(map[string]interface{}, len(data))
			for k, v := range data {
				payload[k] = v
			}
			srv.emitClusterEvent(eventType, payload)
			return nil
		},
		WaitCondition: func(ctx context.Context, nID, condition string) error {
			return srv.waitBootstrapCondition(ctx, nID, condition)
		},
	})

	// Condition evaluator for contains() expressions.
	evalCond := func(ctx context.Context, expr string, inputs, outputs map[string]any) (bool, error) {
		if strings.HasPrefix(expr, "contains(inputs.node_profiles,") {
			nodeProfiles, ok := inputs["node_profiles"].([]any)
			if !ok {
				return false, nil
			}
			parts := strings.SplitN(expr, "'", 3)
			if len(parts) < 2 {
				return false, nil
			}
			target := parts[1]
			for _, p := range nodeProfiles {
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
			log.Printf("bootstrap-workflow: node %s step %s → %s (%s)",
				nodeID, step.ID, step.Status, elapsed.Round(time.Millisecond))
		},
	}

	// Convert profiles to []any for the condition evaluator.
	profilesAny := make([]any, len(profiles))
	for i, p := range profiles {
		profilesAny[i] = p
	}

	inputs := map[string]any{
		"cluster_id":    srv.cfg.ClusterDomain,
		"node_id":       nodeID,
		"node_hostname": hostname,
		"node_profiles": profilesAny,
	}

	// Mark the node as workflow-driven so reconcileBootstrapPhases skips it.
	srv.lock("RunBootstrapWorkflow:activate")
	if n := srv.state.Nodes[nodeID]; n != nil {
		n.BootstrapWorkflowActive = true
	}
	srv.unlock()
	defer func() {
		srv.lock("RunBootstrapWorkflow:deactivate")
		if n := srv.state.Nodes[nodeID]; n != nil {
			n.BootstrapWorkflowActive = false
		}
		srv.unlock()
	}()

	log.Printf("bootstrap-workflow: starting node.bootstrap for node %s (%s) profiles=%v",
		nodeID, hostname, profiles)

	start := time.Now()
	run, err := eng.Execute(ctx, def, inputs)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("bootstrap-workflow: node %s FAILED after %s: %v",
			nodeID, elapsed.Round(time.Millisecond), err)
	} else {
		succeeded := 0
		for _, st := range run.Steps {
			if st.Status == engine.StepSucceeded {
				succeeded++
			}
		}
		log.Printf("bootstrap-workflow: node %s SUCCEEDED in %s (%d/%d steps)",
			nodeID, elapsed.Round(time.Millisecond), succeeded, len(run.Steps))
	}

	return run, err
}

// setBootstrapPhase updates a node's bootstrap phase under lock and emits
// a transition event.
func (srv *server) setBootstrapPhase(nodeID, phase string) error {
	srv.lock("setBootstrapPhase")
	defer srv.unlock()
	node := srv.state.Nodes[nodeID]
	if node == nil {
		return fmt.Errorf("node %s not found", nodeID)
	}

	oldPhase := node.BootstrapPhase
	newPhase := BootstrapPhase(phase)
	if oldPhase == newPhase {
		return nil
	}

	node.BootstrapPhase = newPhase
	node.BootstrapStartedAt = time.Now()
	node.BootstrapError = ""

	log.Printf("bootstrap-workflow: node %s (%s) phase %s → %s",
		nodeID, node.Identity.Hostname, oldPhase, newPhase)

	srv.emitClusterEvent("node.bootstrap_phase_changed", map[string]interface{}{
		"severity":       "INFO",
		"node_id":        nodeID,
		"hostname":       node.Identity.Hostname,
		"from_phase":     string(oldPhase),
		"to_phase":       string(newPhase),
		"correlation_id": "bootstrap:" + nodeID,
	})

	recordBootstrapTransition(srv, node, oldPhase)
	return nil
}

// waitBootstrapCondition polls the in-memory node state until the named
// condition is satisfied or the context expires.
func (srv *server) waitBootstrapCondition(ctx context.Context, nodeID, condition string) error {
	check := func() bool {
		srv.lock("waitBootstrapCondition")
		defer srv.unlock()
		node := srv.state.Nodes[nodeID]
		if node == nil {
			return false
		}
		switch condition {
		case "node_has_etcd_unit":
			return nodeHasEtcdUnit(node)
		case "etcd_join_verified":
			return node.EtcdJoinPhase == EtcdJoinVerified
		case "xds_active":
			return nodeHasUnitActive(node, "globular-xds.service")
		case "envoy_active":
			return nodeHasUnitActive(node, "globular-envoy.service")
		case "storage_verified":
			allOK := true
			if nodeHasMinioProfile(node) && node.MinioJoinPhase != MinioJoinVerified {
				allOK = false
			}
			if nodeHasScyllaProfile(node) && node.ScyllaJoinPhase != ScyllaJoinVerified {
				allOK = false
			}
			return allOK
		default:
			log.Printf("bootstrap-workflow: unknown condition %q for node %s", condition, nodeID)
			return false
		}
	}

	// Already satisfied?
	if check() {
		return nil
	}

	// Poll every 3 seconds until satisfied or context expires.
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("condition %q not met for node %s: %w", condition, nodeID, ctx.Err())
		case <-ticker.C:
			if check() {
				return nil
			}
		}
	}
}

// resolveBootstrapDefinition finds the node.bootstrap.yaml file.
func resolveBootstrapDefinition() string {
	candidates := []string{
		"/var/lib/globular/workflows/node.bootstrap.yaml",
		"/usr/lib/globular/workflows/node.bootstrap.yaml",
		"/tmp/node.bootstrap.yaml",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
