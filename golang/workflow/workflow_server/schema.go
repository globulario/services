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
    workflow_name     text,
    superseded_by     text,
    wait_reason       text,
    retry_attempt     int,
    max_retries       int,
    backoff_until_ms  bigint,
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

// workflow_run_summaries — one row per (cluster, workflow_name) summarizing
// lifetime + last-known-good/bad stats. Used for dashboard widgets where
// full per-run detail is unnecessary (e.g. periodic cluster.reconcile runs).
// Bounded size: O(# workflow definitions) regardless of run frequency.
const createRunSummariesTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.workflow_run_summaries (
    cluster_id            text,
    workflow_name         text,
    total_runs            bigint,
    success_runs          bigint,
    failure_runs          bigint,
    last_run_id           text,
    last_run_status       int,
    last_started_at       timestamp,
    last_finished_at      timestamp,
    last_duration_ms      bigint,
    last_success_id       text,
    last_success_at       timestamp,
    last_failure_id       text,
    last_failure_at       timestamp,
    last_failure_reason   text,
    updated_at            timestamp,
    PRIMARY KEY ((cluster_id), workflow_name)
)
`

// --- Convergence telemetry tables ------------------------------------------
// These tables capture the delta between workflow intent and cluster reality.
// AI agents query them to identify contract mismatches, convergence loops,
// and stuck drift that the workflow engine cannot self-heal.

// workflow_step_outcomes — bounded per-step aggregates for every workflow.
// Primary key guarantees one row per (cluster, workflow, step) regardless of
// execution frequency. Answers "which step fails most / takes longest".
const createStepOutcomesTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.workflow_step_outcomes (
    cluster_id         text,
    workflow_name      text,
    step_id            text,
    total_executions   bigint,
    success_count      bigint,
    failure_count      bigint,
    skipped_count      bigint,
    last_status        int,
    last_started_at    timestamp,
    last_finished_at   timestamp,
    last_duration_ms   bigint,
    last_error_code    text,
    last_error_message text,
    first_seen_at      timestamp,
    updated_at         timestamp,
    PRIMARY KEY ((cluster_id), workflow_name, step_id)
) WITH CLUSTERING ORDER BY (workflow_name ASC, step_id ASC)
`

// phase_transition_log — append-only history of phase transitions per resource.
// TTL 7 days keeps the log bounded while preserving enough history for AI
// oscillation analysis. Answers "which resources cycle through states".
const createPhaseTransitionLogTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.phase_transition_log (
    cluster_id    text,
    resource_type text,
    resource_name text,
    event_at      timestamp,
    event_id      text,
    from_phase    text,
    to_phase      text,
    reason        text,
    caller        text,
    blocked       boolean,
    PRIMARY KEY ((cluster_id, resource_type, resource_name), event_at, event_id)
) WITH CLUSTERING ORDER BY (event_at DESC, event_id ASC)
    AND default_time_to_live = 604800
`

// drift_unresolved — sticky drift counter. A drift item is "unresolved" while
// it keeps appearing in consecutive scan_drift outputs. Cleared when the item
// is no longer observed. Answers "what drift does the system fail to fix".
const createDriftUnresolvedTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.drift_unresolved (
    cluster_id          text,
    drift_type          text,
    entity_ref          text,
    consecutive_cycles  int,
    first_observed_at   timestamp,
    last_observed_at    timestamp,
    chosen_workflow     text,
    last_remediation_id text,
    PRIMARY KEY ((cluster_id), drift_type, entity_ref)
) WITH CLUSTERING ORDER BY (drift_type ASC, entity_ref ASC)
`

// --- Incident model tables (see docs/incidents-design.md) -----------------

// workflow.incidents — operator-facing aggregate of related signals.
// One row per stable (category, signature) within a cluster. Persists across
// OPEN/RESOLVING/RESOLVED/ACKED transitions so operators have memory.
const createIncidentsTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.incidents (
    cluster_id        text,
    id                text,
    category          text,
    signature         text,
    status            int,
    severity          int,
    headline          text,
    occurrence_count  int,
    first_seen_at     timestamp,
    last_seen_at      timestamp,
    entity_ref        text,
    entity_type       text,
    acknowledged      boolean,
    acknowledged_by   text,
    acknowledged_at   timestamp,
    assigned_to       text,
    evidence_json     text,
    diagnoses_json    text,
    proposed_fixes_json text,
    absent_scans      int,
    PRIMARY KEY ((cluster_id), id)
)
`

// workflow.incident_actions — append-only log of operator/AI actions.
// Answers "who did what to this incident and when".
const createIncidentActionsTableCQL = `
CREATE TABLE IF NOT EXISTS workflow.incident_actions (
    cluster_id   text,
    incident_id  text,
    action_at    timestamp,
    action_id    text,
    action       text,
    actor        text,
    fix_id       text,
    comment      text,
    PRIMARY KEY ((cluster_id, incident_id), action_at, action_id)
) WITH CLUSTERING ORDER BY (action_at DESC, action_id ASC)
`

var schemaCQLStatements = []string{
	createRunsTableCQL,
	createRunsByNodeTableCQL,
	createRunsByComponentTableCQL,
	createStepsTableCQL,
	createArtifactRefsTableCQL,
	createEventsTableCQL,
	createRunSummariesTableCQL,
	createStepOutcomesTableCQL,
	createPhaseTransitionLogTableCQL,
	createDriftUnresolvedTableCQL,
	createIncidentsTableCQL,
	createIncidentActionsTableCQL,
	createExecutorLeasesTableCQL,
	createStepReceiptsTableCQL,
}
