package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/globulario/services/golang/workflow/engine"
	workflowpb "github.com/globulario/services/golang/workflow/workflowpb"
)

// restartableInfraComponents is the closed set this workflow accepts.
//
// These are PACKAGE identities, never systemd unit names. packageToUnit
// (identity.UnitForService) is the unit-name authority — scylladb resolves to
// scylla-server.service, not globular-scylladb.service — and accepting a raw
// unit here would let a caller bypass it.
var restartableInfraComponents = map[string]bool{
	"etcd":     true,
	"minio":    true,
	"scylladb": true,
}

// restartInfraCorrelationID derives the DURABLE identity of one restart attempt.
//
// The previous implementation minted `restart-<node>-<component>-<UnixNano>`,
// so every retry of the same remediation allocated a new identity and the
// Workflow Service could never recognise a replay. The identity now comes
// entirely from the remediation context, which means an identical retry
// reconciles with the existing run while a genuinely new parent attempt (new
// parent run or step) legitimately produces a new child.
//
// Readable composite rather than a digest: these ids appear in operator-facing
// run listings, and every component is already an opaque id, so hashing would
// cost debuggability without adding collision safety.
func restartInfraCorrelationID(parentRunID, parentStepID, findingID, nodeID, component string) string {
	return strings.Join([]string{
		"node.restart_infra_unit",
		parentRunID,
		parentStepID,
		findingID,
		nodeID,
		component,
	}, "/")
}

// RunRestartInfraUnitWorkflow dispatches the infra restart as a DURABLE child
// run through the Workflow Service.
//
// Ordering law: executeWorkflowCentralized persists the run and only then does
// the engine dispatch the node-agent step that performs the restart. Because
// this function performs no mutation itself, "persisted run happens-before host
// mutation" is structural — there is no code path here that can restart a unit
// without a persisted run behind it.
//
// It replaces a controller-owned inline path that called node-agent
// ControlService directly, fabricated a wall-clock run id, and stored a
// synthetic result in a process-local map that vanished on restart.
func (srv *server) RunRestartInfraUnitWorkflow(
	ctx context.Context,
	parentRunID, parentStepID, findingID, nodeID, endpoint, component string,
) (*workflowpb.ExecuteWorkflowResponse, error) {
	component = strings.ToLower(strings.TrimSpace(component))
	nodeID = strings.TrimSpace(nodeID)
	endpoint = strings.TrimSpace(endpoint)

	// Validate BEFORE creating any execution request: a malformed remediation
	// must not reach the Workflow Service at all.
	if nodeID == "" {
		return nil, fmt.Errorf("restart_infra_unit: node_id is required")
	}
	if endpoint == "" {
		return nil, fmt.Errorf("restart_infra_unit: agent_endpoint is required for node %s", nodeID)
	}
	if !restartableInfraComponents[component] {
		return nil, fmt.Errorf("restart_infra_unit: unsupported component %q (want etcd, minio or scylladb)", component)
	}
	if strings.TrimSpace(parentRunID) == "" || strings.TrimSpace(parentStepID) == "" {
		// Without durable parent identity the correlation id would not be
		// stable across retries, which is the whole point of this path.
		return nil, fmt.Errorf("restart_infra_unit: parent_run_id and parent_step_id are required for a durable child run")
	}

	router := engine.NewRouter()
	engine.RegisterNodeDirectApplyActions(router, srv.buildNodeDirectApplyConfig())

	inputs := map[string]any{
		"cluster_id":     srv.cfg.ClusterDomain,
		"parent_run_id":  parentRunID,
		"parent_step_id": parentStepID,
		"finding_id":     findingID,
		"node_id":        nodeID,
		"agent_endpoint": endpoint,
		"component":      component,
	}

	correlationID := restartInfraCorrelationID(parentRunID, parentStepID, findingID, nodeID, component)

	log.Printf("restart-infra-workflow: dispatching %s restart on node %s (correlation=%s)",
		component, nodeID, correlationID)

	start := time.Now()
	resp, err := srv.executeWorkflowCentralized(ctx, "node.restart_infra_unit", correlationID, inputs, router)
	elapsed := time.Since(start)
	if err != nil {
		log.Printf("restart-infra-workflow: %s on %s FAILED after %s: %v",
			component, nodeID, elapsed.Round(time.Millisecond), err)
		return nil, err
	}
	log.Printf("restart-infra-workflow: %s on %s finished in %s: status=%s run_id=%s",
		component, nodeID, elapsed.Round(time.Millisecond), resp.GetStatus(), resp.GetRunId())
	return resp, nil
}
