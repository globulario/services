package main

// ScyllaDB schema for the Workflow service.
//
// Keyspace: workflow (SimpleStrategy, RF=1; adjust for cluster)

const workflowKeyspace = "workflow"

const createWorkflowKeyspaceCQL = `
CREATE KEYSPACE IF NOT EXISTS workflow
  WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}
`

// workflow_runs — main run table, partitioned by cluster_id for efficient scans.
const createRunsTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.workflow_runs (
    cluster_id        text,
    id                text,
    correlation_id    text,
    parent_run_id     text,
    node_id           text,
    node_hostname     text,
    component_name    text,
    component_kind    int,
    component_version text,
    release_kind      text,
    release_object_id text,
    desired_object_id text,
    plan_id           text,
    plan_generation   int,
    trigger_reason    int,
    status            int,
    current_actor     int,
    failure_class     int,
    summary           text,
    error_message     text,
    retry_count       int,
    acknowledged      boolean,
    acknowledged_by   text,
    acknowledged_at   timestamp,
    started_at        timestamp,
    updated_at        timestamp,
    finished_at       timestamp,
    PRIMARY KEY ((cluster_id), started_at, id)
) WITH CLUSTERING ORDER BY (started_at DESC, id ASC)
`

// Materialized view: runs by node for "show me all runs for nuc"
const createRunsByNodeTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.workflow_runs_by_node (
    cluster_id     text,
    node_id        text,
    started_at     timestamp,
    run_id         text,
    component_name text,
    status         int,
    summary        text,
    PRIMARY KEY ((cluster_id, node_id), started_at, run_id)
) WITH CLUSTERING ORDER BY (started_at DESC, run_id ASC)
`

// Materialized view: runs by component for "show me all scylladb installs"
const createRunsByComponentTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.workflow_runs_by_component (
    cluster_id     text,
    component_name text,
    started_at     timestamp,
    run_id         text,
    node_id        text,
    status         int,
    summary        text,
    PRIMARY KEY ((cluster_id, component_name), started_at, run_id)
) WITH CLUSTERING ORDER BY (started_at DESC, run_id ASC)
`

// workflow_steps — ordered steps within a run.
const createStepsTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.workflow_steps (
    cluster_id      text,
    run_id          text,
    seq             int,
    step_key        text,
    title           text,
    actor           int,
    phase           int,
    status          int,
    attempt         int,
    source_actor    int,
    target_actor    int,
    created_at      timestamp,
    started_at      timestamp,
    finished_at     timestamp,
    duration_ms     bigint,
    message         text,
    error_code      text,
    error_message   text,
    retryable       boolean,
    operator_action_required boolean,
    action_hint     text,
    details_json    text,
    PRIMARY KEY ((cluster_id, run_id), seq)
)
`

// workflow_artifact_refs — artifact references linked to runs/steps.
const createArtifactRefsTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.workflow_artifact_refs (
    cluster_id      text,
    run_id          text,
    id              text,
    step_seq        int,
    kind            int,
    name            text,
    version         text,
    digest          text,
    node_id         text,
    path            text,
    etcd_key        text,
    unit_name       text,
    config_path     text,
    package_name    text,
    package_version text,
    spec_path       text,
    script_path     text,
    metadata_json   text,
    created_at      timestamp,
    PRIMARY KEY ((cluster_id, run_id), id)
)
`

// workflow_events — append-only event stream for timeline rendering.
const createEventsTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.workflow_events (
    cluster_id  text,
    run_id      text,
    event_at    timestamp,
    event_id    text,
    step_seq    int,
    event_type  text,
    actor       int,
    old_value   text,
    new_value   text,
    message     text,
    PRIMARY KEY ((cluster_id, run_id), event_at, event_id)
) WITH CLUSTERING ORDER BY (event_at ASC, event_id ASC)
`

var schemaCQLStatements = []string{
	createRunsTableCQL,
	createRunsByNodeTableCQL,
	createRunsByComponentTableCQL,
	createStepsTableCQL,
	createArtifactRefsTableCQL,
	createEventsTableCQL,
}
