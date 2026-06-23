package main

import "fmt"

// ScyllaDB schema for the behavioral-memory kernel (PR-2: ingestion half).
//
// Keyspace: behavioral_memory — separate from ai_memory so the two surfaces stay
// independent and the kernel can later be extracted to its own deployment without
// a keyspace rename. The ai-memory binary connects with keyspace=ai_memory, so
// every statement below is FULLY QUALIFIED (behavioral_memory.<table>) and runs
// over the same shared session via cross-keyspace addressing.
//
// PR-2 tables only: signals, claims, evidence, evidence_by_target, authorities,
// conditions, contradictions. Promotion/runtime tables (principles,
// principles_by_condition, outcomes, outcomes_by_theme) are NOT created here.
//
// Partition strategy: every primary table is keyed by ((project, domain, id)) so
// the ingestion RPCs can get/update a single entity by its canonical id with NO
// ALLOW FILTERING and no read-before-write scan. evidence_by_target is the one
// relation-lookup table (target → its evidence). Time-ordered and
// condition-indexed lookups (needed by ResolveGovernedContext / CheckAction) are
// deliberately deferred to PR-4, which adds dedicated lookup tables.
//
// RDF-readiness: every row carries a stable id + project + domain + status +
// provenance/source + first-class relation columns. metadata is an extension
// hatch only — governance relations are never hidden inside it.

const behavioralKeyspace = "behavioral_memory"

// createBehavioralKeyspaceCQL creates the behavioral_memory keyspace with the
// given replication factor (mirrors the ai_memory RF policy).
func createBehavioralKeyspaceCQL(rf int) string {
	return fmt.Sprintf(
		`CREATE KEYSPACE IF NOT EXISTS %s WITH replication = {'class': 'SimpleStrategy', 'replication_factor': %d}`,
		behavioralKeyspace, rf,
	)
}

const createSignalsTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.signals (
    project      text,
    domain       text,
    id           text,
    kind         text,
    source_kind  text,
    source_ref   text,
    entity_ref   text,
    scope        text,
    observed_at  bigint,
    payload      text,
    confidence   float,
    status       text,
    agent_id     text,
    memory_id    text,
    created_at   bigint,
    updated_at   bigint,
    metadata     map<text, text>,
    cluster_id   text,
    condition_ref text,
    severity     text,
    authority_level text,
    PRIMARY KEY ((project, domain, id))
)`

const createClaimsTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.claims (
    project        text,
    domain         text,
    id             text,
    signal_id      text,
    statement      text,
    subject_entity text,
    predicate      text,
    object_value   text,
    time_ref       bigint,
    status         text,
    confidence     float,
    source_id      text,
    agent_id       text,
    memory_id      text,
    created_at     bigint,
    updated_at     bigint,
    metadata       map<text, text>,
    PRIMARY KEY ((project, domain, id))
)`

const createEvidenceTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.evidence (
    project       text,
    domain        text,
    id            text,
    target_kind   text,
    target_id     text,
    evidence_kind text,
    lane          text,
    result        text,
    probe_ref     text,
    observed_at   bigint,
    payload       text,
    provenance    text,
    observed_from text,
    satisfies     set<text>,
    created_at    bigint,
    updated_at    bigint,
    metadata      map<text, text>,
    source_kind   text,
    source_ref    text,
    entity_ref    text,
    cluster_id    text,
    condition_ref text,
    severity      text,
    authority_level text,
    PRIMARY KEY ((project, domain, id))
)`

// evidence_by_target is the relation-lookup table: for a given target
// (claim/principle), list the evidence that supports it. Clustered by id so a
// target's evidence is retrievable without ALLOW FILTERING.
const createEvidenceByTargetTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.evidence_by_target (
    project     text,
    domain      text,
    target_id   text,
    id          text,
    target_kind text,
    evidence_kind text,
    lane        text,
    result      text,
    observed_at bigint,
    created_at  bigint,
    PRIMARY KEY ((project, domain, target_id), id)
) WITH CLUSTERING ORDER BY (id ASC)`

const createAuthoritiesTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.authorities (
    project         text,
    domain          text,
    id              text,
    title           text,
    governs         text,
    owner_kind      text,
    read_path       text,
    write_path      text,
    identity_source text,
    governs_refs    set<text>,
    status          text,
    created_at      bigint,
    updated_at      bigint,
    metadata        map<text, text>,
    PRIMARY KEY ((project, domain, id))
)`

const createConditionsTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.conditions (
    project     text,
    domain      text,
    id          text,
    title       text,
    detect_spec text,
    severity    text,
    status      text,
    created_at  bigint,
    updated_at  bigint,
    metadata    map<text, text>,
    PRIMARY KEY ((project, domain, id))
)`

const createContradictionsTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.contradictions (
    project    text,
    domain     text,
    id         text,
    kind       text,
    left_ref   text,
    right_ref  text,
    resolution text,
    note       text,
    created_at bigint,
    updated_at bigint,
    metadata   map<text, text>,
    PRIMARY KEY ((project, domain, id))
)`

// contradictions_by_target is a relation-lookup index (analogous to
// evidence_by_target): given a referenced entity (a claim or principle id), list
// the contradictions that reference it. Maintained by PutContradiction. It lets
// the promotion gate find OPEN contradictions blocking a principle without
// ALLOW FILTERING. It is NOT principles_by_condition (the deferred PR-4 hot-path
// table) — it is a maintenance index for an existing entity.
const createContradictionsByTargetTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.contradictions_by_target (
    project    text,
    domain     text,
    target_id  text,
    id         text,
    kind       text,
    resolution text,
    left_ref   text,
    right_ref  text,
    created_at bigint,
    PRIMARY KEY ((project, domain, target_id), id)
) WITH CLUSTERING ORDER BY (id ASC)`

// ── PR-3 governance tables ────────────────────────────────────────────────────

const createPrinciplesTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.principles (
    project               text,
    domain                text,
    id                    text,
    title                 text,
    applies_when          set<text>,
    authorities           set<text>,
    required_evidence     set<text>,
    forbidden_moves       set<text>,
    recommended_action    text,
    risk_level            text,
    revocation_rule       text,
    promotion_reason      text,
    status                text,
    superseded_by         text,
    narrowed_by           text,
    version               int,
    proposed_by           text,
    promoted_by           text,
    promotion_decision_id text,
    revocation_rule_id    text,
    contradiction_checked boolean,
    approved_by           text,
    approval_reason       text,
    approved_at           bigint,
    source_refs           set<text>,
    generated_from        set<text>,
    agent_id              text,
    memory_id             text,
    created_at            bigint,
    updated_at            bigint,
    metadata              map<text, text>,
    PRIMARY KEY ((project, domain, id))
)`

// promotion_decisions records EVERY promotion attempt — allowed, blocked, and
// review-required. Blocked promotion is also memory.
const createPromotionDecisionsTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.promotion_decisions (
    project                 text,
    domain                  text,
    id                      text,
    principle_id            text,
    decision                text,
    verdict                 text,
    missing_evidence        list<text>,
    unresolved_authority    list<text>,
    unresolved_conditions   list<text>,
    blocking_contradictions list<text>,
    blocked_by_forbidden    list<text>,
    risk_level              text,
    review_required         boolean,
    approved_by             text,
    reviewer                text,
    promotion_reason        text,
    reason                  text,
    actor                   text,
    created_at              bigint,
    metadata                map<text, text>,
    PRIMARY KEY ((project, domain, id))
)`

// revocation_rules records a revocation/supersession/narrowing WITHOUT deleting
// the principle.
const createRevocationRulesTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.revocation_rules (
    project           text,
    domain            text,
    id                text,
    principle_id      text,
    action            text,
    revocation_reason text,
    condition         text,
    note              text,
    actor             text,
    superseded_by     text,
    narrowed_scope    text,
    created_at        bigint,
    metadata          map<text, text>,
    PRIMARY KEY ((project, domain, id))
)`

// ── PR-4 runtime tables ───────────────────────────────────────────────────────

// principles_by_condition is the runtime hot-path index: condition → promoted
// principles. Only PromotePrinciple inserts; Revoke/Supersede/Narrow deletes.
// The index therefore holds ONLY active promoted mappings — lookup needs no
// status filter and no ALLOW FILTERING. This is the bridge from promotion to
// runtime behavior (CheckAction / ResolveGovernedContext).
const createPrinciplesByConditionTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.principles_by_condition (
    project      text,
    domain       text,
    condition_id text,
    principle_id text,
    risk_level   text,
    promoted_at  bigint,
    PRIMARY KEY ((project, domain, condition_id), principle_id)
) WITH CLUSTERING ORDER BY (principle_id ASC)`

// action_checks is the audit trail of every CheckAction verdict.
const createActionChecksTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.action_checks (
    project                   text,
    domain                    text,
    id                        text,
    action_type               text,
    target                    text,
    conditions                list<text>,
    allowed                   boolean,
    status                    text,
    violated_principles       list<text>,
    checked_against_principles list<text>,
    missing_evidence          list<text>,
    unresolved_authority      list<text>,
    forbidden_matched         list<text>,
    recommended_steps         list<text>,
    explanation               text,
    agent_id                  text,
    created_at                bigint,
    metadata                  map<text, text>,
    governed                  boolean,
    PRIMARY KEY ((project, domain, id))
)`

// alterActionChecksAddGovernedCQL backfills the governed column on deployments
// whose action_checks table predates PR-13 (CREATE IF NOT EXISTS does not add
// columns to an existing table). ScyllaDB has no ALTER ... ADD IF NOT EXISTS, so
// the migration runs this best-effort and tolerates the "column already exists"
// error (the idempotent re-run / fresh-install case).
const alterActionChecksAddGovernedCQL = `ALTER TABLE behavioral_memory.action_checks ADD governed boolean`

// PR-9 added governed-observation columns (cluster_id, condition_ref, severity,
// authority_level) to signals and evidence — but ONLY in their CREATE TABLE
// statements. CREATE TABLE IF NOT EXISTS does not add columns to a table that
// already exists, so any deployment whose signals/evidence tables predate PR-9
// is missing these columns and every RecordSignal/RecordEvidence write fails
// with "Unknown identifier <col>". These backfill ALTERs repair such tables;
// like the action_checks backfill, ScyllaDB has no ALTER ... ADD IF NOT EXISTS,
// so the migration runs them best-effort and tolerates the "column already
// exists" error (the idempotent re-run / fresh-install case). Whenever a column
// is added to a CREATE TABLE, a matching backfill ALTER MUST be added here.
const (
	alterSignalsAddClusterID       = `ALTER TABLE behavioral_memory.signals ADD cluster_id text`
	alterSignalsAddConditionRef    = `ALTER TABLE behavioral_memory.signals ADD condition_ref text`
	alterSignalsAddSeverity        = `ALTER TABLE behavioral_memory.signals ADD severity text`
	alterSignalsAddAuthorityLevel  = `ALTER TABLE behavioral_memory.signals ADD authority_level text`
	alterEvidenceAddClusterID      = `ALTER TABLE behavioral_memory.evidence ADD cluster_id text`
	alterEvidenceAddConditionRef   = `ALTER TABLE behavioral_memory.evidence ADD condition_ref text`
	alterEvidenceAddSeverity       = `ALTER TABLE behavioral_memory.evidence ADD severity text`
	alterEvidenceAddAuthorityLevel = `ALTER TABLE behavioral_memory.evidence ADD authority_level text`
	// evidence (unlike signals) also gained source_kind/source_ref/entity_ref in
	// PR-9 — signals carried those from v1, but the evidence table added them, so
	// pre-PR-9 evidence tables lack them and RecordEvidence fails with
	// "Unknown identifier source_kind" (observed live when the infra_probe feed
	// went active). Backfill them too.
	alterEvidenceAddSourceKind = `ALTER TABLE behavioral_memory.evidence ADD source_kind text`
	alterEvidenceAddSourceRef  = `ALTER TABLE behavioral_memory.evidence ADD source_ref text`
	alterEvidenceAddEntityRef  = `ALTER TABLE behavioral_memory.evidence ADD entity_ref text`
)

// governance_coverage counts CheckAction verdicts that were governed (an
// applicable promoted principle was evaluated) vs ungoverned (default-allow), so
// the gate's reach is measurable. Counter columns require a counter-only table.
const createGovernanceCoverageTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.governance_coverage (
    project          text,
    domain           text,
    governed_count   counter,
    ungoverned_count counter,
    PRIMARY KEY ((project, domain))
)`

// outcomes records what happened after an action/check.
const createOutcomesTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.outcomes (
    project            text,
    domain             text,
    id                 text,
    action_check_id    text,
    principle_ids      list<text>,
    evidence_ids       list<text>,
    supports_principles list<text>,
    weakens_principles  list<text>,
    status             text,
    severe             boolean,
    human_marked       boolean,
    incident_id        text,
    theme              text,
    note               text,
    agent_id           text,
    created_at         bigint,
    metadata           map<text, text>,
    PRIMARY KEY ((project, domain, id))
)`

// outcomes_by_theme is the lookup for repeated outcome patterns (later promotion
// proposals). Clustered by created_at DESC so recent outcomes for a theme come first.
const createOutcomesByThemeTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.outcomes_by_theme (
    project      text,
    domain       text,
    theme        text,
    created_at   bigint,
    id           text,
    status       text,
    severe       boolean,
    human_marked boolean,
    incident_id  text,
    PRIMARY KEY ((project, domain, theme), created_at, id)
) WITH CLUSTERING ORDER BY (created_at DESC, id ASC)`

// promotion_candidates is the PR-10 human-review queue for repeated outcome
// themes. It stores an explicit principle draft plus the supporting repeated
// outcomes/evidence that justified surfacing the candidate.
const createPromotionCandidatesTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.promotion_candidates (
    project                  text,
    domain                   text,
    id                       text,
    theme                    text,
    status                   text,
    title                    text,
    summary                  text,
    rationale                text,
    supporting_outcome_ids   list<text>,
    supporting_evidence_ids  list<text>,
    repeat_count             int,
    draft_principle_id       text,
    draft_title              text,
    draft_applies_when       set<text>,
    draft_authorities        set<text>,
    draft_required_evidence  set<text>,
    draft_forbidden_moves    set<text>,
    draft_recommended_action text,
    draft_risk_level         text,
    draft_revocation_rule    text,
    draft_promotion_reason   text,
    draft_status             text,
    draft_version            int,
    draft_proposed_by        text,
    draft_source_refs        set<text>,
    draft_generated_from     set<text>,
    generated_by             text,
    created_at               bigint,
    updated_at               bigint,
    materialized_principle_id text,
    metadata                 map<text, text>,
    PRIMARY KEY ((project, domain, id))
)`

const createReconciliationReportsTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.reconciliation_reports (
    project                    text,
    domain                     text,
    id                         text,
    promotion_candidate_id     text,
    theme                      text,
    awg_invariant_ids          list<text>,
    awg_failure_mode_ids       list<text>,
    awg_test_ids               list<text>,
    findings                   list<text>,
    summary                    text,
    outcome_count              int,
    failure_count              int,
    success_count              int,
    severe_count               int,
    proposed_awg_invariant_ids list<text>,
    proposed_awg_failure_mode_ids list<text>,
    proposed_awg_test_ids      list<text>,
    proposed_behavioral_theme  text,
    actor                      text,
    created_at                 bigint,
    metadata                   map<text, text>,
    PRIMARY KEY ((project, domain, id))
)`

// ── v10 list-by-scope indexes ─────────────────────────────────────────────────
//
// promotion_candidates and reconciliation_reports are keyed by ((project,domain,id))
// for single-entity get/upsert, but their List RPCs enumerate every row in a
// (project,domain) scope. A query filtering by the (project,domain) PREFIX of a
// COMPOSITE partition key is rejected by ScyllaDB without ALLOW FILTERING — which
// is exactly why ListPromotionCandidates / ListReconciliationReports failed at
// runtime ("Cannot execute this query as it might involve data filtering").
//
// These index tables make the list a single-partition read: partition by
// ((project,domain)), cluster by id. They follow the same relation-table idiom as
// evidence_by_target / contradictions_by_target / outcomes_by_theme — additive
// CREATE IF NOT EXISTS so they are forward-safe to re-run (no destructive DDL, no
// PRIMARY KEY change on the live entity tables). The writer upserts the id into the
// index; theme/status/created_at are read from (and sorted on) the entity row, so
// the index carries only the keys. There is no delete path for either entity, so
// the index needs no maintenance beyond upsert.
const createPromotionCandidatesByScopeTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.promotion_candidates_by_scope (
    project text,
    domain  text,
    id      text,
    PRIMARY KEY ((project, domain), id)
) WITH CLUSTERING ORDER BY (id ASC)`

const createReconciliationReportsByScopeTableCQL = `
CREATE TABLE IF NOT EXISTS behavioral_memory.reconciliation_reports_by_scope (
    project text,
    domain  text,
    id      text,
    PRIMARY KEY ((project, domain), id)
) WITH CLUSTERING ORDER BY (id ASC)`

// behavioralSchemaStatements is the ordered list of table DDL (keyspace created
// separately). All use IF NOT EXISTS and are safe to re-run.
var behavioralSchemaStatements = []string{
	// PR-2 ingestion tables.
	createSignalsTableCQL,
	createClaimsTableCQL,
	createEvidenceTableCQL,
	createEvidenceByTargetTableCQL,
	createAuthoritiesTableCQL,
	createConditionsTableCQL,
	createContradictionsTableCQL,
	// PR-3 governance tables + contradiction lookup index.
	createContradictionsByTargetTableCQL,
	createPrinciplesTableCQL,
	createPromotionDecisionsTableCQL,
	createRevocationRulesTableCQL,
	// PR-4 runtime tables.
	createPrinciplesByConditionTableCQL,
	createActionChecksTableCQL,
	alterActionChecksAddGovernedCQL, // PR-13: backfill governed on pre-existing tables
	createOutcomesTableCQL,
	createOutcomesByThemeTableCQL,
	createPromotionCandidatesTableCQL,
	createReconciliationReportsTableCQL,
	// v10 list-by-scope indexes: make the List RPCs single-partition reads instead
	// of (project,domain)-prefix scans on a composite partition key (ALLOW FILTERING).
	createPromotionCandidatesByScopeTableCQL,
	createReconciliationReportsByScopeTableCQL,
	// PR-13 governance-coverage counters.
	createGovernanceCoverageTableCQL,
	// v8 backfill: PR-9 columns that shipped only in CREATE TABLE, so pre-PR-9
	// signals/evidence tables are missing them. Runs after the CREATEs above;
	// tolerates "column already exists" on fresh installs / re-runs.
	alterSignalsAddClusterID,
	alterSignalsAddConditionRef,
	alterSignalsAddSeverity,
	alterSignalsAddAuthorityLevel,
	alterEvidenceAddClusterID,
	alterEvidenceAddConditionRef,
	alterEvidenceAddSeverity,
	alterEvidenceAddAuthorityLevel,
	alterEvidenceAddSourceKind,
	alterEvidenceAddSourceRef,
	alterEvidenceAddEntityRef,
}
