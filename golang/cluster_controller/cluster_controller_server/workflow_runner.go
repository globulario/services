package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// RunBootstrapWorkflow delegates execution of the node.bootstrap workflow to
// the centralized WorkflowService. The controller actors are wired to the
// in-memory node state map so that set_phase and wait_condition operate on
// live state — these are invoked via actor callbacks from the workflow service.
func (srv *server) RunBootstrapWorkflow(ctx context.Context, nodeID string) (*workflowpb.ExecuteWorkflowResponse, error) {
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
	// NOTE: The centralized engine handles conditions, but the controller's
	// actors need the profiles available via inputs. The contains() condition
	// is evaluated by the engine's DefaultEvalCond.

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

	correlationID := "bootstrap:" + nodeID

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
	resp, err := srv.executeWorkflowCentralized(ctx, "node.bootstrap", correlationID, inputs, router)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("bootstrap-workflow: node %s FAILED after %s: %v",
			nodeID, elapsed.Round(time.Millisecond), err)
		return nil, err
	}

	if resp.Status == "SUCCEEDED" {
		log.Printf("bootstrap-workflow: node %s SUCCEEDED in %s",
			nodeID, elapsed.Round(time.Millisecond))
	} else {
		log.Printf("bootstrap-workflow: node %s FAILED in %s: %s",
			nodeID, elapsed.Round(time.Millisecond), resp.Error)
	}

	return resp, nil
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

	if check() {
		return nil
	}

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

// resolveBootstrapDefinition is no longer needed — definitions are loaded
// from MinIO by the centralized WorkflowService. Removed in Phase E.
// The old filesystem resolution paths were:
//   /var/lib/globular/workflows/node.bootstrap.yaml
//   /usr/lib/globular/workflows/node.bootstrap.yaml
//   /tmp/node.bootstrap.yaml

