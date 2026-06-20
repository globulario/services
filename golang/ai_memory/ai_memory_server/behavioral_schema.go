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
    PRIMARY KEY ((project, domain, id))
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
	createOutcomesTableCQL,
	createOutcomesByThemeTableCQL,
}
