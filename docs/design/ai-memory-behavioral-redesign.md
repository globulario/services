# AI-Memory Behavioral Redesign — Architecture & Implementation Plan

> Status: **proposal / implementation-ready**. Source brief: `ai_memory_behavioral_redesign_brief.md`.
> This document answers the 17-part Claude Task in §21 of the brief. It is grounded in the
> *current* ai-memory surface and the *current* AWG governance model (both mapped below), not on
> a guessed surface. A Go developer can begin PR-1 from §16.

---

## 0. The one decision that shapes everything

The brief asks for a "governed behavioral-memory" kernel with a ladder:

```
raw signal → claim → evidence-linked fact → authority-mapped fact → condition-scoped rule
→ contradiction-tested → proposed principle → promoted principle → enforced → revoked/narrowed
```

**The awareness-graph (AWG) already implements this ladder — for code and repair.** Its proto
(`awareness-graph/proto/awareness_graph.proto:624-744`) already defines `EvidenceLaneMode`
(STATIC_ONLY / RUNTIME_REQUIRED / HYBRID), `CertificationVerdict`, `PromotionDecision`
(ALLOWED / BLOCKED / REVIEW_REQUIRED), `ProofObligation`, `ProofSlot`, `ForbiddenRepairMove`,
`RepairClaim`, `GovernanceCertification`. Its RDF vocab already has `OutcomeFeedback`,
`RuntimeEvidence`, `LearningEvent`, `ProofObligation`, `ProofSlot`. Its promotion gate
(`golang/extractor/promotion_gate.go`, `promotion_proposal.go`) already enforces "candidates
accumulate evidence, never auto-promote, quality bar before active, CLI-guarded writes."

So the behavioral-memory kernel is **not a new invention** — it is the *generalization of AWG's
governance-certification model from the "code repair" domain to the "runtime operation" domain.*

| | AWG (today) | behavioral-memory (this design) |
|---|---|---|
| Governs | a **code change / repair claim** | a **runtime action / operational claim** |
| Contract | invariant, forbidden_fix, required_test | principle, forbidden_move, required_evidence |
| Authority | which actor owns this *code/state* | which actor owns this *runtime truth* (etcd, Scylla, owner-RPC, human) |
| Evidence | static (YAML) + runtime (test/probe) | static (authored) + runtime (probe/metric/log) |
| Verdict | CertificationVerdict + PromotionDecision | ActionCheck verdict + PromotionDecision |
| Lifecycle | draft→candidate→active→deprecated/superseded | RAW_SIGNAL→…→PROMOTED→REVOKED/SUPERSEDED/NARROWED |

This gives us the extraction north-star the brief demands (§3, §8, §24): the kernel's domain model
must be **vocabulary-compatible with AWG's governance enums and RDF classes**, so that when the
kernel is later lifted into a standalone `BehavioralMemoryService`, AWG can become its second
consumer without a model rewrite. The shared meta-principle is already authored in AWG:
`meta.no_resolution_without_a_respected_contract` / `meta.contract_must_be_explicit_before_resolution`
— behavioral-memory is the *runtime* enforcement of that same meta-principle.

**Practical consequence for every type we define below:** reuse AWG's enum *names and semantics*
(EvidenceLane, PromotionDecision, status ladder) even though we generate them in our own package.
Do not coin a competing vocabulary.

---

## 1. Current state (verified, so the plan evolves reality not a guess)

**Service.** `golang/ai_memory/ai_memory_server/` — one gRPC service `AiMemoryService`
(`proto/ai_memory.proto:12-103`), ScyllaDB-backed, keyspace `ai_memory`, two tables.

- `memories`: PK `((project), type, created_at, id)`; columns include `tags set<text>`,
  `metadata map<text,text>`, `related_ids list<text>`, `reference_count`, `ttl_seconds`,
  `agent_id`, `conversation_id`, `cluster_id`. (`schema.go:67-85`)
- `sessions`: PK `((project), created_at, id)`. (`schema.go:87-101`)
- RPCs: Store, Query, Get, Update, Delete, List, SaveSession, ResumeSession, Stop.
- Migration coordinator: etcd mutex `/globular/migrations/scylla/ai_memory`, schema-version
  constant, 60s lock TTL, falls back to uncoordinated if etcd down. (`migration_coordinator.go`)
- Seed immutability guard: entries with `metadata.source=seed` + `metadata.immutable=true` are
  mutable only by principal `sa`. (`immutability.go`)
- MCP: 8 tools in `golang/mcp/tools_memory.go` resolve `ai_memory.AiMemoryService` from etcd.

**The gap the brief names:** governance concepts today can only live inside `metadata map<text,text>`
(`root_cause`, `confidence`, `related_to`…). That is exactly the "junk drawer" §22 forbids. The
redesign makes signal / claim / evidence / authority / condition / contradiction / principle /
outcome **first-class rows**, while keeping `memories`/`sessions` and the existing API intact.

---

## 2. Architecture & deployment decision

**One deployed service now; two gRPC service definitions; one in-process kernel behind an
interface; clean extraction path later.** (Matches brief §12 preferred direction.)

```
ai-memory binary (single deployment, single port, single Scylla cluster)
├── grpc.Server
│   ├── AiMemoryService          (existing — unchanged surface, "raw memory / compat layer")
│   └── BehavioralMemoryService  (NEW — thin handlers, delegate to kernel)
└── behavioral kernel (Core interface)  ← in-process today, gRPC-promotable tomorrow
        └── domains/cluster_operator (first domain pack)
```

- **Why register a second gRPC service in the same binary rather than just internal functions?**
  Because the brief's core design rule (§4) is "service-grade request/response interfaces from day
  one." Defining `BehavioralMemoryService` in proto *now* — generated, registered on the same
  `grpc.Server` via the existing `LifecycleManager.RegisterService` hook — gives us the
  protocol-shaped boundary immediately, at zero extra deployment cost. The handlers are ~10 lines
  each (decode → call `Core` → encode).
- **Extraction later = mechanical.** Move `behavioral/` + the handler file into a new
  `behavioral_memory_server` binary; ai-memory keeps a `BehavioralMemoryServiceClient` and the
  `memory_adapter` becomes a network call. No domain-model change. This is exactly how AWG keeps
  its store behind a port boundary (`meta.storage_is_not_semantic_authority`).
- **Do NOT** split the Scylla keyspace prematurely or stand up a second deployment now (§22).

---

## 3. Package layout (Globular-conventional refinement of brief §13)

```
proto/
  ai_memory.proto                         # unchanged
  behavioral_memory.proto                 # NEW — BehavioralMemoryService + governance messages

golang/ai_memory/
  ai_memory_server/                       # existing service binary; gains behavioral wiring
    server.go                             # registers BOTH services on grpc.Server
    behavioral_handlers.go                # NEW — gRPC surface → kernel (thin)
    schema.go                             # gains behavioral_memory keyspace DDL (bump version)
    migration_coordinator.go              # unchanged mechanism, new schema version

  behavioral/                             # the kernel — NO cluster-specific fields anywhere here
    api/
      core.go                             # Core interface (the 12 ops)
      requests.go  responses.go  types.go # service-grade request/response structs + domain types
      status.go                           # GovernanceStatus ladder, EvidenceLane, PromotionDecision
    core/
      service.go                          # Core impl; orchestrates the sub-recorders
      signal.go claim.go evidence.go authority.go condition.go contradiction.go
      promotion.go context_resolver.go action_checker.go outcome.go revocation.go explain.go
    store/
      store.go                            # Store interface (port)
      scylla_store.go                     # Scylla adapter (the only place CQL lives)
      memory_adapter.go                   # links behavioral rows ↔ ai_memory.memories rows
    domain/
      registry.go domain.go               # Domain interface + registry (pluggable packs)

  domains/cluster_operator/               # FIRST domain pack — cluster-specific IDs live ONLY here
    pack.go                               # implements domain.Domain; registers catalogs
    authorities.go conditions.go forbidden_moves.go required_evidence.go
    evidence_probes.go                    # maps required-evidence refs → runtime probe calls
    seed/                                 # YAML seed catalogs (authorities/conditions/forbidden)
      authorities.yaml conditions.yaml forbidden_moves.yaml principles.seed.yaml

  migrations/scylla/                       # *.cql mirrors of schema.go DDL (review artifact)

  behavioral/.../*_test.go                 # unit + governance-gate tests
  tests/integration/                       # end-to-end ladder tests against a Scylla container
```

**Domain isolation rule (brief §15) is enforced structurally:** `behavioral/` may not import
`domains/`; cluster IDs like `condition.cluster.etcd.nospace_alarm` are *string refs* resolved
through the registry, never Go fields. A `go vet`-style import-direction test guards this (§15 test).

---

## 4. Public Go interface (the kernel boundary)

`behavioral/api/core.go` — service-shaped, in-process today, 1:1 with the proto RPCs:

```go
package api

import "context"

// Core is the behavioral-memory kernel. Every method is request/response shaped so it can be
// promoted to gRPC without changing the domain model (brief §4). Implementations are domain-
// agnostic; domain specifics arrive via DomainRef strings resolved through the registry.
type Core interface {
    // Ingestion / ladder
    RecordSignal(ctx context.Context, req *RecordSignalRequest) (*RecordSignalResponse, error)
    ExtractClaim(ctx context.Context, req *ExtractClaimRequest) (*ExtractClaimResponse, error)
    RecordEvidence(ctx context.Context, req *RecordEvidenceRequest) (*RecordEvidenceResponse, error)
    MapAuthority(ctx context.Context, req *MapAuthorityRequest) (*MapAuthorityResponse, error)
    RecordContradiction(ctx context.Context, req *RecordContradictionRequest) (*RecordContradictionResponse, error)

    // Governance
    ProposePrinciple(ctx context.Context, req *ProposePrincipleRequest) (*ProposePrincipleResponse, error)
    PromotePrinciple(ctx context.Context, req *PromotePrincipleRequest) (*PromotePrincipleResponse, error)
    RevokePrinciple(ctx context.Context, req *RevokePrincipleRequest) (*RevokePrincipleResponse, error)
    ExplainPrinciple(ctx context.Context, req *ExplainPrincipleRequest) (*ExplainPrincipleResponse, error)

    // Runtime decision support (the hot path for agents)
    ResolveGovernedContext(ctx context.Context, req *ResolveGovernedContextRequest) (*ResolveGovernedContextResponse, error)
    CheckAction(ctx context.Context, req *CheckActionRequest) (*CheckActionResponse, error)
    RecordOutcome(ctx context.Context, req *RecordOutcomeRequest) (*RecordOutcomeResponse, error)
}
```

Representative request/response and domain types (`api/types.go`, `api/status.go`) — note these
**mirror AWG enum semantics**:

```go
// GovernanceStatus is the promotion ladder (brief §16). Ordered; only forward transitions
// are valid except the terminal revocation set.
type GovernanceStatus string
const (
    StatusRawSignal          GovernanceStatus = "RAW_SIGNAL"
    StatusExtractedClaim     GovernanceStatus = "EXTRACTED_CLAIM"
    StatusCandidateFact      GovernanceStatus = "CANDIDATE_FACT"
    StatusEvidenceLinked     GovernanceStatus = "EVIDENCE_LINKED"
    StatusAuthorityMapped    GovernanceStatus = "AUTHORITY_MAPPED"
    StatusConditionScoped    GovernanceStatus = "CONDITION_SCOPED"
    StatusContradictionTested GovernanceStatus = "CONTRADICTION_TESTED"
    StatusProposedPrinciple  GovernanceStatus = "PROPOSED_PRINCIPLE"
    StatusPromotedPrinciple  GovernanceStatus = "PROMOTED_PRINCIPLE"
    StatusRevoked            GovernanceStatus = "REVOKED"
    StatusSuperseded         GovernanceStatus = "SUPERSEDED"
    StatusNarrowed           GovernanceStatus = "NARROWED"
)

// EvidenceLane mirrors AWG EvidenceLaneMode (awareness_graph.proto:631-636).
type EvidenceLane string
const (
    LaneStaticOnly      EvidenceLane = "STATIC_ONLY"
    LaneRuntimeRequired EvidenceLane = "RUNTIME_REQUIRED"
    LaneHybrid          EvidenceLane = "HYBRID"
)

// PromotionDecision mirrors AWG PromotionDecision (awareness_graph.proto:652-657).
type PromotionDecision string
const (
    PromotionAllowed        PromotionDecision = "ALLOWED"
    PromotionBlocked        PromotionDecision = "BLOCKED"
    PromotionReviewRequired PromotionDecision = "REVIEW_REQUIRED"
)

// SignalKind keeps runtime fact / interpretation / correction / health distinct (brief §19) —
// these MUST NOT be collapsed into one untyped note.
type SignalKind string
const (
    SignalObservedRuntimeFact SignalKind = "OBSERVED_RUNTIME_FACT"
    SignalAgentInterpretation SignalKind = "AGENT_INTERPRETATION"
    SignalHumanCorrection     SignalKind = "HUMAN_CORRECTION"
    SignalAutomatedHealth     SignalKind = "AUTOMATED_HEALTH"
    SignalHistoricalMemory    SignalKind = "HISTORICAL_MEMORY"
    SignalPromotedPrinciple   SignalKind = "PROMOTED_PRINCIPLE"
)

// DomainRef / ConditionRef / AuthorityRef / ForbiddenMoveRef / RequiredEvidenceRef are opaque
// string IDs resolved through the domain registry. The kernel never interprets their contents —
// that is what keeps cluster specifics out of the kernel (brief §15).
type (
    DomainRef           string
    ConditionRef        string
    AuthorityRef        string
    ForbiddenMoveRef    string
    RequiredEvidenceRef string
)

// Principle is the generic governed rule (brief §15 "Good" shape — NO cluster fields).
type Principle struct {
    ID               string
    Project          string
    Domain           DomainRef
    Title            string
    AppliesWhen      []ConditionRef
    Authorities      []AuthorityRef
    RequiredEvidence []RequiredEvidenceRef
    ForbiddenMoves   []ForbiddenMoveRef
    RecommendedAction string
    RiskLevel        string            // info|low|high|irreversible
    RevocationRule   string            // when this principle should be narrowed/revoked
    PromotionReason  string
    Status           GovernanceStatus
    SupersededBy     string
    Version          int
    Provenance       Provenance
}

// CheckActionResponse is the most important runtime feature (brief §17).
type CheckActionResponse struct {
    Allowed             bool
    Status              string // allowed|blocked|needs_evidence|needs_authority|needs_human_approval
    ViolatedPrinciples  []string
    MissingEvidence     []RequiredEvidenceRef
    UnresolvedAuthority []AuthorityRef
    ForbiddenMatched    []ForbiddenMoveRef
    RecommendedSteps    []string
    Explanation         string
}
```

---

## 5. Proto evolution

New file `proto/behavioral_memory.proto`, `package behavioral_memory`. Same authz-annotation
convention as every other Globular proto. The 12 RPCs from §4, plus messages that mirror the Go
types. Critical enums are coined to **match AWG names** so a future merge is a rename-free union:

```protobuf
service BehavioralMemoryService {
  rpc RecordSignal(RecordSignalRequest) returns (RecordSignalResponse) { option (globular.auth.authz) = {...}; }
  rpc ExtractClaim(...) returns (...);
  rpc RecordEvidence(...) returns (...);
  rpc MapAuthority(...) returns (...);
  rpc RecordContradiction(...) returns (...);
  rpc ProposePrinciple(...) returns (...);
  rpc PromotePrinciple(...) returns (...);      // gated; may return REVIEW_REQUIRED
  rpc RevokePrinciple(...) returns (...);
  rpc ExplainPrinciple(...) returns (...);
  rpc ResolveGovernedContext(...) returns (...); // the operator-brain retrieval endpoint
  rpc CheckAction(...) returns (...);            // pre-action gate
  rpc RecordOutcome(...) returns (...);
}

enum EvidenceLaneMode {                 // names identical to AWG
  EVIDENCE_LANE_MODE_UNSPECIFIED = 0;
  EVIDENCE_LANE_STATIC_ONLY = 1;
  EVIDENCE_LANE_RUNTIME_REQUIRED = 2;
  EVIDENCE_LANE_HYBRID = 3;
}
enum PromotionDecision {                // names identical to AWG
  PROMOTION_DECISION_UNSPECIFIED = 0;
  PROMOTION_ALLOWED = 1;
  PROMOTION_BLOCKED = 2;
  PROMOTION_REVIEW_REQUIRED = 3;
}
enum GovernanceStatus { /* the 12-rung ladder from §4 */ }
```

`option go_package = ".../golang/ai_memory/behavioral_memorypb";`. Wire into `generateCode.sh`
exactly like `ai_memory.proto`. **Backward compatibility:** `ai_memory.proto` is untouched — no
field renumbering, no removals — so every existing client keeps working (brief §12).

---

## 6. Data model & ScyllaDB tables

New keyspace `behavioral_memory` (same Scylla cluster, same RF policy `min(hosts,3)`, same
migration coordinator pattern — new mutex key `/globular/migrations/scylla/behavioral_memory`).
Scylla is query-first, so partitions are chosen for the two hot reads — `ResolveGovernedContext`
and `CheckAction` — both keyed by **(project, domain, condition)**.

Core tables (DDL for the load-bearing ones; the rest follow the same shape):

```sql
-- Raw operational input. Typed by signal_kind so runtime-fact / interpretation / correction
-- are never collapsed (brief §19). memory_id links back to ai_memory.memories for provenance.
CREATE TABLE IF NOT EXISTS behavioral_memory.signals (
    project text, domain text, id text,
    signal_kind text, source_kind text, source_ref text,
    entity_ref text, cluster_id text,
    observed_at bigint, payload text, confidence float,
    agent_id text, memory_id text,
    status text, metadata map<text,text>, created_at bigint,
    PRIMARY KEY ((project, domain), created_at, id)
) WITH CLUSTERING ORDER BY (created_at DESC, id ASC);

-- Structured statements derived from signals.
CREATE TABLE IF NOT EXISTS behavioral_memory.claims (
    project text, domain text, id text,
    signal_id text, statement text,
    subject_entity text, predicate text, object_value text, time_ref bigint,
    status text, confidence float, source_id text,
    metadata map<text,text>, created_at bigint, updated_at bigint,
    PRIMARY KEY ((project, domain), created_at, id)
) WITH CLUSTERING ORDER BY (created_at DESC, id ASC);

-- Evidence attaches to a claim OR a principle (target_kind discriminates). lane = AWG lane mode.
CREATE TABLE IF NOT EXISTS behavioral_memory.evidence (
    project text, domain text, id text,
    target_kind text, target_id text,
    evidence_kind text, lane text, result text,   -- result: pass|fail|stale|unknown
    probe_ref text, observed_at bigint, payload text, provenance text,
    created_at bigint,
    PRIMARY KEY ((project, domain), created_at, id)
) WITH CLUSTERING ORDER BY (created_at DESC, id ASC);
CREATE TABLE IF NOT EXISTS behavioral_memory.evidence_by_target (
    project text, domain text, target_id text, id text,
    evidence_kind text, lane text, result text, observed_at bigint,
    PRIMARY KEY ((project, domain, target_id), id)
);

-- Promoted/candidate rules. Sets hold the generic refs; cluster IDs are strings, never columns.
CREATE TABLE IF NOT EXISTS behavioral_memory.principles (
    project text, domain text, id text,
    title text, status text, risk_level text,
    applies_when set<text>, authorities set<text>,
    required_evidence set<text>, forbidden_moves set<text>,
    recommended_action text, revocation_rule text, promotion_reason text,
    superseded_by text, version int,
    proposed_by text, promoted_by text,
    metadata map<text,text>, created_at bigint, updated_at bigint,
    PRIMARY KEY ((project, domain), id)
);

-- HOT-PATH lookup: condition → promoted principles. Maintained on Promote/Revoke/Narrow.
-- Drives ResolveGovernedContext & CheckAction without ALLOW FILTERING.
CREATE TABLE IF NOT EXISTS behavioral_memory.principles_by_condition (
    project text, domain text, condition_id text, status text, principle_id text,
    PRIMARY KEY ((project, domain, condition_id), status, principle_id)
);

-- Outcomes, grouped by theme for the promotion-proposal pipeline (mirrors AWG OutcomeSignal).
CREATE TABLE IF NOT EXISTS behavioral_memory.outcomes (
    project text, domain text, id text,
    action_check_id text, principle_ids set<text>, evidence_ids set<text>,
    status text, severe boolean, human_marked boolean, incident_id text,
    theme text, note text, agent_id text, created_at bigint,
    PRIMARY KEY ((project, domain), created_at, id)
) WITH CLUSTERING ORDER BY (created_at DESC, id ASC);
CREATE TABLE IF NOT EXISTS behavioral_memory.outcomes_by_theme (
    project text, domain text, theme text, id text,
    status text, severe boolean, human_marked boolean, incident_id text, created_at bigint,
    PRIMARY KEY ((project, domain, theme), created_at, id)
) WITH CLUSTERING ORDER BY (created_at DESC, id ASC);
```

Remaining tables follow the same `((project, domain), …)` shape and are listed rather than
spelled out: `authorities`, `conditions`, `forbidden_moves`, `required_evidence` (domain-pack
**catalogs**, PK `((project, domain), id)`), `contradictions` (PK `((project,domain), created_at, id)`),
`action_checks`, `promotion_decisions`. Every row carries the §14 common fields:
`id, project, domain/cluster scope, created_at, updated_at, actor, status, provenance, metadata`.
`metadata` is an **extension hatch only** — never the schema (brief §14, §22).

---

## 7. Migration plan from `Memory` + `metadata`

1. **Additive, never destructive.** `memories`/`sessions` keep their schema and API. Bump the
   migration-coordinator schema version; the coordinator creates the new keyspace under the same
   etcd mutex it already uses (`migration_coordinator.go`).
2. **`memory_adapter`** links the two worlds: when a `signal`/`claim`/`outcome` is born from an
   existing memory, it stores `memory_id`; when an agent stores a memory of type `DEBUG`/`DECISION`,
   the adapter can *optionally* emit a `RAW_SIGNAL` (off by default; opt-in per call).
3. **Backfill (one-shot, idempotent, `sa`-run):** a `behavioral migrate` CLI scans
   `ai_memory.memories` for governance breadcrumbs hiding in `metadata`
   (`root_cause`, `confidence`, `related_to`, `source`) and emits CANDIDATE_FACT claims linked
   back via `memory_id`. **It promotes nothing** — backfilled claims enter the ladder at
   `CANDIDATE_FACT` and must earn promotion. This respects §22 ("do not promote observations
   directly").
4. **Seed immutability reused:** domain-pack catalog rows (authorities/conditions/forbidden) are
   stamped `metadata.source=seed, immutable=true` and guarded by the existing `sa`-only mutation
   check, so a stray agent can't rewrite the cluster authority map.

---

## 8. Cluster-operator domain model (first pack)

`golang/ai_memory/domains/cluster_operator/`. Everything cluster-specific lives **here only**.

- **Authorities** (`authorities.yaml`): `authority.cluster.etcd.member_health`,
  `authority.cluster.scylla.schema_agreement`, `authority.cluster.minio.pool_health`,
  `authority.cluster.envoy.route_config`, `authority.cluster.owner_service.runtime_state`,
  `authority.cluster.human.irreversible_ops`. Each row: `owner_kind` (datastore|service_rpc|human|proxy|dns),
  `read_path`, `write_path`, `identity_source`. Encodes brief §7's authority table as data.
- **Conditions** (`conditions.yaml`): `condition.cluster.etcd.nospace_alarm`,
  `condition.cluster.scylla.schema_disagreement`, `condition.cluster.minio.pool_degraded`,
  `condition.cluster.service.health_check_failed`. Each carries a `detect_spec` → a probe ref.
- **Forbidden moves** (`forbidden_moves.yaml`): `forbidden.cluster.restart_before_quorum_check`,
  `forbidden.cluster.minio_drive_count_change_without_mirror`,
  `forbidden.cluster.claim_recovery_without_authoritative_evidence`. These map to existing
  hard-won lessons already in MEMORY.md (the MinIO `format.json` blast radius, founding quorum,
  "no recovery claim without evidence").
- **Required evidence + probes** (`evidence_probes.go`): maps a `RequiredEvidenceRef` to a typed
  runtime call (the existing `infra_probe_*` / `cluster_get_*` MCP/gRPC surface). This is the
  *only* file that knows how to actually gather cluster evidence — the kernel just asks for it.
- **Seed principle** (the brief's worked example, authored at `PROPOSED_PRINCIPLE`, not promoted):
  ```
  When condition.cluster.etcd.nospace_alarm appears,
    authority = authority.cluster.etcd.member_health,
    required_evidence = [etcd alarm status, member health, compaction state, defrag safety,
                         desired-vs-observed],
    forbidden_moves = [forbidden.cluster.restart_before_quorum_check],
    recommended_action = "inspect alarms + compaction; defrag only if safe; claim recovery only
                          after authoritative runtime evidence returns clean",
    risk_level = high,
    revocation_rule = "narrow if a newer etcd version changes NOSPACE semantics".
  ```

---

## 9. Promotion workflow

`ProposePrinciple` creates a `PROPOSED_PRINCIPLE` row. `PromotePrinciple` runs the gate
(mirrors AWG `promotion_gate.go` semantics) and returns a `PromotionDecision`:

```
PromotePrinciple(principle_id):
  assert evidence exists (≥1 EVIDENCE_LINKED claim, lane satisfied)
  assert provenance exists
  assert authority mapped (≥1 AuthorityRef resolvable in domain)
  assert conditions explicit (AppliesWhen non-empty, all resolvable)
  assert contradictions checked (no OPEN contradiction touching this principle)
  assert revocation_rule present
  assert promotion_reason recorded
  classify risk_level:
     info|low        → PROMOTION_ALLOWED      (status → PROMOTED_PRINCIPLE; update principles_by_condition)
     high|irreversible → PROMOTION_REVIEW_REQUIRED (await human approve; record promotion_decision row)
  any assertion fails → PROMOTION_BLOCKED with missing_evidence / unresolved_authority listed
```

`PromotePrinciple` is the **only** writer that flips a row to `PROMOTED_PRINCIPLE`, and it is the
only path that updates `principles_by_condition`. Auto-promotion is impossible (§22). Severe/human
patterns can be surfaced as proposals (mirroring AWG `promotion_proposal.go`) but never auto-promoted.

---

## 10. Action-checking workflow (the hot runtime gate)

```
CheckAction(domain, action_type, target, current_conditions[], cluster_id):
  1. resolve promoted principles via principles_by_condition for each current condition
  2. match forbidden_moves whose (action_type,target pattern) matches the proposed action
  3. for each matched principle: collect RequiredEvidence not yet satisfied (query evidence_by_target)
  4. collect Authorities not yet resolved/clean
  5. verdict:
       forbidden match                         → blocked
       missing required evidence               → needs_evidence (+ which probes to run)
       unresolved authority                    → needs_authority
       risk_level irreversible & no human ok    → needs_human_approval
       else                                     → allowed
  6. persist an action_checks row (audit), return CheckActionResponse (§4)
```

This is the endpoint that turns "alert → guess → command → claim fixed" into the §10 governed
loop. Every check is auditable (brief "Audit everything").

---

## 11. Governed-context retrieval

`ResolveGovernedContext(domain, goal|conditions[], entity, cluster_id)` is the default pre-action
context provider. It fans out (all reads keyed by partition, no ALLOW FILTERING):
relevant memories (via `memory_adapter`), trusted claims, applicable promoted principles
(via `principles_by_condition`), condition matches, required evidence, forbidden moves, unresolved
authority, known OPEN contradictions, prior similar outcomes/incidents (via `outcomes_by_theme`),
recommended behavior, and a confidence classification. Returns the §18 bundle.

---

## 12. Runtime signal ingestion

Sources (brief §19) enter only through `RecordSignal`, each stamped with its `SignalKind` so
observed-fact / interpretation / correction / automated-health stay distinct:

- Globular service health, infra probes (`infra_probe_*`), Scylla/etcd/MinIO/Envoy/DNS/TLS status,
  release-controller & desired-vs-observed snapshots → `OBSERVED_RUNTIME_FACT` / `AUTOMATED_HEALTH`.
- Agent diagnoses → `AGENT_INTERPRETATION`. Operator corrections → `HUMAN_CORRECTION` (higher trust).
- Existing memories surfaced into a context → `HISTORICAL_MEMORY`.

Ingestion is **pull-friendly**: a thin collector can periodically `RecordSignal` from the existing
probe RPCs, but the kernel itself stays passive (no hidden cron). This keeps the §22 "fail-safe"
property — if ingestion stops, the cluster's deterministic convergence is unaffected.

---

## 13. Integration with existing ai-memory RPCs

- Existing 8 RPCs/tools unchanged. `memory_adapter` is the bridge: behavioral rows may reference a
  `memory_id`; `ResolveGovernedContext` pulls related memories by those IDs.
- New MCP tools (additive, same `tools_memory.go` registration pattern, gated behind a
  `ToolGroups.Behavioral` flag): `behavioral_record_signal`, `behavioral_check_action`,
  `behavioral_resolve_context`, `behavioral_record_outcome`, `behavioral_propose_principle`,
  `behavioral_explain_principle`. `check_action` and `resolve_context` are the two an operating
  agent calls constantly; the rest are for the learning loop.
- `agent_id` / `conversation_id` / `cluster_id` already flow through ai-memory and are reused
  verbatim on behavioral rows for audit continuity.

---

## 14. Integration with AWG

- **Shared vocabulary now, shared kernel later.** Enum names (EvidenceLane, PromotionDecision,
  status ladder) and the RDF class names (OutcomeFeedback, RuntimeEvidence, LearningEvent) are
  deliberately identical to AWG's. When/if the kernel is extracted to `BehavioralMemoryService`,
  AWG can consume it as its runtime-evidence backend — its `RuntimeEvidence`/`OutcomeFeedback`
  nodes become this kernel's `evidence`/`outcomes` rows.
- **Cross-references, not coupling.** A behavioral `Principle` may cite an AWG invariant ID in
  `metadata` (e.g. a forbidden_move that corresponds to `meta.no_resolution_without_a_respected_contract`).
  No build-time dependency between the two services — only ID references, exactly how AWG's own
  `related_invariants` work.
- **Division of labor stays clean (brief §8):** AWG governs *code/repair* contracts; behavioral-
  memory governs *runtime/operation* contracts. Same meta-principle, two enforcement surfaces.

---

## 15. Test strategy

- **Kernel unit tests** (`behavioral/.../*_test.go`): each ladder transition; promotion gate
  rejects on every missing precondition (one test per assertion in §9); revoke/narrow/supersede
  transitions; `CheckAction` verdict matrix (one case per verdict in §10).
- **Domain-isolation test** (enforces §15): a test that fails if `behavioral/` imports `domains/`,
  or if any kernel struct field name contains a cluster term — the structural guarantee that the
  kernel stays generic.
- **Governance-gate property test:** no path produces `PROMOTED_PRINCIPLE` without passing the
  full gate (fuzz the request, assert invariant).
- **Adapter/back-compat test:** existing ai-memory RPCs behave identically after the schema bump;
  backfill is idempotent and promotes nothing.
- **Integration** (`tests/integration/`, Scylla container): full ladder for the etcd-NOSPACE seed
  principle — signal → claim → evidence → authority → condition → contradiction-check → propose →
  promote(REVIEW_REQUIRED) → human approve → CheckAction blocks the forbidden restart → RecordOutcome.
- **Awareness graph upkeep (per repo rule):** the etcd-NOSPACE forbidden move and the
  "no recovery claim without evidence" principle should be registered as a `failure_mode` /
  `forbidden_fix` in `docs/awareness/` and rebuilt, since these tests encode invariants
  (per MEMORY.md "Update awareness graph when writing a unit test").

---

## 16. First implementation milestones (PR-sized)

- **PR-1 — proto + kernel skeleton (no behavior change to ai-memory).** Add
  `proto/behavioral_memory.proto`, generate code, define `behavioral/api` (interfaces + status +
  types), stub `core.Service` returning Unimplemented, register `BehavioralMemoryService` on the
  existing grpc.Server. Ships dark; nothing calls it yet. *Verifiable: `go build ./...`, service
  starts, both services listed in reflection.*
- **PR-2 — Scylla store + ingestion half of the ladder.** `behavioral_memory` keyspace + DDL +
  migration-version bump; implement `RecordSignal/ExtractClaim/RecordEvidence/MapAuthority/
  RecordContradiction` + `scylla_store.go`. Unit tests for each.
- **PR-3 — governance half.** `ProposePrinciple/PromotePrinciple/RevokePrinciple/ExplainPrinciple`
  + the gate (§9) + `principles_by_condition` maintenance. Governance-gate tests.
- **PR-4 — runtime decision support.** `ResolveGovernedContext` + `CheckAction` + `RecordOutcome`
  + `outcomes_by_theme`. Verdict-matrix tests.
- **PR-5 — cluster_operator domain pack + seed catalogs + evidence probes** wired to existing
  infra-probe RPCs. The etcd-NOSPACE integration test goes green here.
- **PR-5A — Operational Knowledge Compiler (deferred; see §19).** Compile the existing
  `operational-knowledge/` Markdown/YAML seed into behavioral-memory domain catalogs
  (authorities/conditions/required-evidence/forbidden-moves) and **PROPOSED_PRINCIPLE** candidates —
  including higher-abstraction *generative* operator principles, not only restrictive warnings.
  Emits deterministic generated artifacts; **auto-promotes nothing**. Runs after PR-5 so the
  cluster_operator base catalogs exist. No RDF/Oxigraph.
- **PR-6 — MCP tools + memory_adapter backfill CLI** (behind `ToolGroups.Behavioral`).
  Awareness-graph entries registered.
- **PR-7 — behavioral RDF/Ontology projection (deferred; see §18).** Add semantic export/projection
  for behavioral-memory rows **without changing the Scylla-backed runtime path**. Define the
  behavioral-memory ontology vocabulary, map Scylla rows → RDF triples (reusing AWG-compatible
  classes/enums), add a deterministic export command (`behavioral export-rdf` / `validate-rdf` /
  `audit-rdf`), and add validation/audit tests. RDF must not become the source of runtime truth.
  Deferred until the Scylla kernel (PR-2), promotion gate (PR-3), action checker (PR-4), and
  cluster_operator pack (PR-5) are working.

Each PR is independently shippable; ai-memory's existing behavior is untouched until PR-6 exposes
the tools. PR-7 is purely additive (a projection/export), never on the runtime decision path.

---

## 17. Risks & mitigations

| Risk | Mitigation |
|---|---|
| **Junk-drawer regression** (governance hidden in metadata again) | First-class tables (§6); `metadata` is extension-only; domain-isolation test forbids cluster fields in the kernel. |
| **Premature/automatic promotion** | `PromotePrinciple` is the sole promoter; gate enforced; high/irreversible → human review; property test guarantees it. |
| **Cluster specifics leaking into the kernel** | Structural import-direction test (§15); refs are opaque strings resolved via registry. |
| **Scylla anti-patterns** (ALLOW FILTERING on hot path) | Lookup tables `principles_by_condition` / `outcomes_by_theme` / `evidence_by_target` make both hot reads partition-keyed. |
| **Scope creep into a 2nd deployment too early** | One binary, two services; extraction is mechanical and deferred until a second consumer (AWG) actually needs it. |
| **AWG vocabulary drift** (two divergent governance models) | Enum names mirror AWG deliberately; a doc note ties the two; extraction merges them into one kernel. |
| **Evidence gathering coupling the kernel to cluster RPCs** | Only `domains/cluster_operator/evidence_probes.go` calls cluster RPCs; the kernel asks for evidence by ref and never dials infra itself. |
| **Fail-safe violation** (cluster depends on AI memory) | Ingestion is passive/pull; deterministic convergence never depends on the kernel (brief §22, MEMORY.md fail-safe rule). |
| **RDF becomes a hidden second source of truth** | Scylla is the sole operational source of record; RDF is a deferred one-way *projection* (§18, PR-7), never read by the hot path; export is deterministic and re-derivable from rows. |

---

## 18. RDF/Ontology projection strategy

**Decision: ScyllaDB is the operational source of record; RDF/Ontology is a semantic *projection*
layer, not the primary runtime store.** RDF is deferred to PR-7 and is never on the runtime
decision path. This section is a design constraint that binds from PR-1 onward, even though no RDF
code ships until PR-7.

**Why a split, not an RDF-native store.** The two hot paths — `CheckAction` and
`ResolveGovernedContext` — need partition-keyed Scylla lookups (`principles_by_condition`,
`evidence_by_target`, `outcomes_by_theme`) with predictable latency. A triple store on that path
would trade determinism for query flexibility we don't need at action time. RDF earns its place for
*semantic inspection, explanation, AWG alignment, cross-domain reasoning, and linked-data
export/import* — none of which are latency-critical.

```
Runtime path (Scylla, hot):   current_conditions → principles_by_condition → evidence_by_target
                              → action_checks → verdict
Semantic path (RDF, deferred): Scylla behavioral rows → RDF projection → ontology validation /
                              AWG-compatible graph
```

**Binding constraints from PR-1 (already enforced in code):**

1. **Stable canonical IDs = semantic identity.** Every first-class entity (Signal, Claim, Evidence,
   Authority, Condition, Contradiction, Principle, ForbiddenMove, RequiredEvidence, Outcome,
   PromotionDecision, RevocationRule, ActionCheck) carries a stable `id` + `project`/`domain` scope.
   The Scylla `id` *is* the RDF identity — no separate RDF-only id is ever minted. The URI scheme is
   fixed now: `behavioral:<kind>/<id>` (see `behavioral/api/uri.go`, `CanonicalURI`).
2. **Relations are first-class fields, never metadata.** The governance edges are typed fields on
   proto/Go/(future) Scylla rows — not buried in `metadata`. `metadata` is an extension hatch only.
   Enforced by `behavioral/api/rdf_readiness_test.go`.
3. **AWG vocabulary alignment is mandatory.** `EvidenceLaneMode`, `PromotionDecision`,
   `GovernanceStatus` (and the future `RuntimeEvidence`/`OutcomeFeedback`/`LearningEvent`/
   `ProofObligation`/`ProofSlot` projections) reuse AWG names/meanings. No competing concepts.
4. **RDF must never become a hidden second source of truth.** The projection is one-way and
   re-derivable from Scylla rows at any time.

**Projected ontology (PR-7 target).** Classes map 1:1 to the entity kinds:
`bm:Signal, bm:Claim, bm:Evidence, bm:Authority, bm:Condition, bm:Contradiction, bm:Principle,
bm:ForbiddenMove, bm:RequiredEvidence, bm:Outcome, bm:PromotionDecision, bm:RevocationRule,
bm:ActionCheck`. Predicates (each backed by a first-class field landed in PR-1):

| Predicate | Source field |
|---|---|
| `bm:producesClaim` | inverse of `Claim.signal_id` |
| `bm:supportedBy` | inverse of `Evidence.target_id` |
| `bm:observedFrom` | `Evidence.observed_from` |
| `bm:satisfies` | `Evidence.satisfies` |
| `bm:governs` | `Authority.governs_refs` |
| `bm:appliesWhen` | `Principle.applies_when` |
| `bm:requiresEvidence` | `Principle.required_evidence` |
| `bm:forbidsMove` | `Principle.forbidden_moves` |
| `bm:promotedBy` | `Principle.promotion_decision_id` |
| `bm:revokedBy` | `Principle.revocation_rule_id` |
| `bm:supersededBy` / `bm:narrowedBy` | `Principle.superseded_by` / `Principle.narrowed_by` |
| `bm:contradictedBy` | `Contradiction.left_ref` / `right_ref` |
| `bm:resultedFrom` | `Outcome.action_check_id` |
| `bm:supportsPrinciple` / `bm:weakensPrinciple` | `Outcome.supports_principles` / `weakens_principles` |
| `bm:checkedAgainst` | `ActionCheck.checked_against_principles` |
| `bm:blockedBy` | `ActionCheck.forbidden_matched` |
| `bm:missingEvidence` | `ActionCheck.missing_evidence` |

Note: some edges (`bm:producesClaim`, `bm:supportedBy`) are stored once on their natural owning side
in Scylla and emitted in both directions at projection time — deliberately, to avoid dual-write
inconsistency in the operational store.

Add to `Principle` (landed in PR-3 so the seam exists before the compiler): `source_refs`
(→ `bm:sourceRef`) and `generated_from` (→ `bm:generatedFrom`) — first-class lineage links a
compiler-generated principle uses to trace back to its `OperationalKnowledgeEntry` seed, never in
metadata.

---

## 19. Operational Knowledge Compiler (PR-5A)

**Decision: a later compiler turns the existing immutable `operational-knowledge/` seed
(Markdown/YAML) into governed behavioral-memory objects and *generative* operator principles — so
the agent doesn't merely *search* runbooks, it learns how to reason about authority, lifecycle
phase, evidence, recovery patterns, and safe next actions.** Deferred to **PR-5A** (after the
cluster_operator pack, PR-5). Source addendum: `operational_knowledge_behavioral_ontology_addendum.md`.

**Pipeline:** `operational-knowledge YAML/MD → operational ontology → governed behavioral-memory
objects (Scylla) → PROPOSED_PRINCIPLE candidates → later RDF projection`. ScyllaDB stays the source
of record; the compiler must not require RDF/Oxigraph.

**Input classes → outputs:**
- `stages/*.yaml` (lifecycle phase truth) → Conditions, Authorities, RequiredEvidence, principle candidates.
- `service-roles/*.yaml` (what a service owns / must not do) → Authorities, OwnedState claims, ForbiddenMoves, generative principle candidates.
- `runbooks/*.yaml` (observe→plan→execute→verify, success criteria, warnings) → RequiredEvidence, ForbiddenMoves, RecoveryPatterns/RecommendedSteps, principle candidates.
- `incidents/*.md` (scar tissue) → Outcomes, FailurePatterns, ForbiddenMoves, principle candidates, possible revoke/narrow candidates for older wrong behavior.
- canonical refs (`packages.md`, `dns-records.md`, `node-removal.md`, `deploy-package-via-mcp.md`, `awg-operator-guide.md`) → Claims, Authorities, Conditions, RequiredEvidence, generative candidates.

**The abstraction jump (the core requirement).** The compiler must climb three levels, not stop at
warnings:
1. **Concrete** operational fact ("this service owns this config path").
2. **Domain rule** ("MinIO topology changes require topology authority and convergence evidence").
3. **Generative principle** ("preserve authority boundaries during repair"; "complete
   observe→plan→execute→verify before claiming recovery"; "a command result is action evidence, not
   recovery"; "controller decides / executor mutates"; "treat a diagnostic finding as a claim until
   independently verified"; "use lifecycle stage to scope diagnosis").

**Generative pairing (mandatory).** Every important `ForbiddenMove` should ship with the
constructive behavior to *prefer*. E.g. forbidden "delete cleanup candidates from filesystem
evidence alone" pairs with generative "build an authority chain: doctor finding → systemd ref check
→ process ref check → installed-metadata check → reversible quarantine." Negative-only rules are a
design smell.

**Seed trust ≠ auto-promotion.** Seed entries carry strong provenance (`source=seed`,
`immutable=true`, `seed_sha256`, `seed_version`) and are trusted *input* with an integrity proof —
but the promotion gate (§9) still governs enforcement. The only acceptable shortcut is a
seeder/admin workflow promoting `seed-approved + low-risk + no contradictions + explicit authority +
explicit revocation rule` — and even that **must write a PromotionDecision row**. No hidden promotion.

**Output model.** Emit a deterministic generated bundle under
`behavioral/generated/operational_knowledge/` (authorities/conditions/required_evidence/
forbidden_moves/principle_candidates/runbook_patterns/source_index), *not* direct mutation of
promoted principles. Statuses: seed claims → `CANDIDATE_FACT` (or `AUTHORITY_MAPPED` when authority
is explicit); principle candidates → `PROPOSED_PRINCIPLE`; forbidden_moves/required_evidence →
catalog entries (inert until a promoted principle references them).

**Constraints (from §15 of the addendum):** no free-inventing LLM extractor; not every sentence
becomes a principle; never auto-promote; never collapse relations into metadata; RDF/Oxigraph not
required; operational seed stays immutable at runtime; no second source of truth separate from
Scylla; always pair forbidden moves with a preferred behavior.

**Tests (PR-5A):** valid YAML parses / invalid rejected / duplicate seed ids rejected; stages→Conditions,
service-roles→Authorities, success-criteria→RequiredEvidence, warnings→ForbiddenMoves,
phases→RecoveryPatterns, incidents→FailurePattern/principle candidates; deterministic output;
generated principles are `PROPOSED_PRINCIPLE` only; relations first-class not metadata; stable
URI-ready ids; seed provenance preserved; RDF-readiness + kernel-hygiene still green.

---

## One-paragraph summary for the reviewer

Keep `ai-memory` as the single deployed service. Add a second, protocol-shaped gRPC service
(`BehavioralMemoryService`) in the same binary, backed by an in-process `Core` kernel that
implements the brief's 12 governed operations and the promotion ladder. The kernel is a
**runtime-operations generalization of AWG's already-built governance-certification model** — same
evidence-lane / promotion-decision / status vocabulary — so the two can share a kernel later. New
first-class Scylla tables (signals, claims, evidence, authorities, conditions, contradictions,
principles, outcomes, action_checks) end the metadata junk-drawer; lookup tables keep
`CheckAction`/`ResolveGovernedContext` partition-keyed. Cluster specifics live only in a
`cluster_operator` domain pack. Nothing is auto-promoted; high-risk principles need human approval.
Existing memory/session APIs and tests are untouched. Six PRs, each shippable; PR-1 is a dark
proto+skeleton landing.
```
