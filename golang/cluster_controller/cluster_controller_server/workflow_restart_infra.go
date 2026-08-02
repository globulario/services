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

// restartInfraCorrelationID derives the DURABLE identity of one restart episode.
//
// Two defects are closed here. The original implementation minted
// `restart-<node>-<component>-<UnixNano>`, so every retry allocated a new
// identity and a replay could never be recognised. Keying instead on the
// stable drift ENTITY reference fixed retries but over-corrected: after a
// drift cleared, a later independent outage of the same component aliased
// onto the first episode's run, collapsing separate remediation episodes into
// one durable record.
//
// The identity is therefore scoped to the drift EPISODE — the persisted
// unresolved-row incarnation (see infra_drift_episode.go). Same unresolved
// episode reconciles to one child run across ticks and restarts; a genuinely
// new episode gets a new run, so both remain independently queryable.
//
// Readable composite rather than a digest: these ids appear in operator-facing
// run listings, and every component is already an opaque id, so hashing would
// cost debuggability without adding collision safety.
func restartInfraCorrelationID(clusterID, episodeID, nodeID, component string) string {
	return strings.Join([]string{
		"node.restart_infra_unit",
		clusterID,
		episodeID,
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
	parentRunID, parentStepID, findingID, episodeID, nodeID, endpoint, component string,
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
	// Fail closed. A missing episode identity means the drift-history
	// authority cannot prove WHICH unresolved episode this is, and guessing
	// would either alias a new outage onto a finished run or allocate a fresh
	// run every tick. Neither is acceptable, so no run is created at all and
	// the drift stays unresolved for a later tick.
	episodeID = strings.TrimSpace(episodeID)
	if episodeID == "" {
		return nil, fmt.Errorf("restart_infra_unit: drift_episode_id is required; refusing to dispatch a restart whose remediation episode cannot be proven")
	}

	router := engine.NewRouter()
	engine.RegisterNodeDirectApplyActions(router, srv.buildNodeDirectApplyConfig())

	inputs := map[string]any{
		"cluster_id":       srv.cfg.ClusterDomain,
		"parent_run_id":    parentRunID,
		"parent_step_id":   parentStepID,
		"finding_id":       findingID,
		"drift_episode_id": episodeID,
		"node_id":          nodeID,
		"agent_endpoint":   endpoint,
		"component":        component,
	}

	correlationID := restartInfraCorrelationID(srv.cfg.ClusterDomain, episodeID, nodeID, component)

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
