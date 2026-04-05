# Incident Model — Operator Surface for a Self-Correcting Control Plane

**Status:** Draft
**Audience:** Backend + frontend engineers implementing the incident view in globular-admin.
**Scope:** Define the data model, grouping semantics, provenance taxonomy, lifecycle, and operator action contract for the incident card view. Concrete enough that backend and frontend teams can implement without reinterpreting architecture.

---

## 1. Purpose

Replace the raw workflow-runs table in the admin UI with an **incident view** that aggregates workflow telemetry, cluster-doctor findings, and AI-proposed fixes into narrative cards an operator (or AI) can act on.

The incident card is the **operator surface for a self-correcting control plane**. It answers four questions in layered order:

1. What is wrong? *(headline)*
2. What did we observe? *(evidence)*
3. What do we think caused it? *(diagnosis)*
4. What should be done? *(proposed fix + actions)*

## 2. Non-goals

- **Not** a replacement for log search or observability stack. Deep forensic drill-down stays in workflow run detail + journal access.
- **Not** magic grouping. Operators must be able to predict which incident a new signal lands under.
- **Not** automatic code merging. AI proposals are reviewed; operator clicks "Apply". Autonomous patch application is a later decision.

## 3. Incident identity

An **incident** is a persistent, operator-visible aggregation of related signals pointing to the same underlying root cause.

### 3.1 Grouping key

The grouping key is the tuple:

```
(cluster_id, category, signature)
```

- `category` ∈ {`workflow_failure`, `drift_stuck`, `service_unhealthy`, `node_unreachable`, `auth_denied`, …}
- `signature` is a small, stable string derived from the dominant signal:
  - workflow_failure → `workflow_name/step_id`
  - drift_stuck → `drift_type/entity_ref`
  - service_unhealthy → `service_name`
  - node_unreachable → `node_id`

**Grouping rule:** If a new signal's (category, signature) matches an open incident, attach it. Otherwise open a new incident.

**Deliberately not clever:** two incidents that happen to share a root cause but have different signatures remain separate incidents. Correlation between incidents is a UI hint, not a merge.

### 3.2 Incident ID

```
incident_id = sha256(cluster_id + category + signature)[:16]
```

Stable across time — the same underlying pattern always produces the same incident_id. Makes dedup trivial and makes URLs bookmark-friendly.

### 3.3 Primary entity reference

Even though `signature` often embeds an entity name, incidents carry the entity as **first-class fields**:

```protobuf
string entity_ref  = 18;  // e.g. "globule-nuc", "core@globular.io/dns", "scylla-manager-agent@globule-dell"
string entity_type = 19;  // "node" | "service" | "release" | "package" | "drift_item"
```

These enable:
- Filtering ("show all incidents on globule-nuc")
- Related-incident hints (same entity, different category)
- Cross-linking into admin UI (entity links to its detail page)
- AI queries ("list unresolved incidents touching ai-executor")

Populated by the scanner when the incident is opened. For incidents where the entity is ambiguous, leave both empty.

### 3.4 Headline generation rules

Headlines are consumed in list views and must be consistent. Rules:

- **Length:** ≤ 80 characters. Hard truncate with ellipsis beyond.
- **Shape:** `<primary observation> · <entity_ref>` — no inference, no "probably", no "should".
- **Entity inclusion:** MUST include `entity_ref` when available.
- **Voice:** past tense for events, present tense for ongoing conditions.
- **No diagnosis in headline.** Probable cause belongs in `diagnoses[]`.

Examples:

| Category | Good headline | Bad headline |
|---|---|---|
| workflow_failure | `node.bootstrap timed out · globule-nuc` | `Bootstrap probably failing because of scylla` ❌ |
| drift_stuck | `scylla-manager-agent unresolved 8 cycles · globule-dell` | `Drift stuck` ❌ (no entity) |
| service_unhealthy | `globular-ai-executor.service failed · globule-dell` | `AI service crashed again — might be memory` ❌ |
| node_unreachable | `Node last seen 12m ago · globule-lenovo` | `Old phantom box is gossipping again` ❌ |

### 3.5 Severity derivation rules

Severity is derived deterministically, not assigned by hand. The scanner computes it from a pipeline:

```
severity = max(category_base, recurrence_bump, diagnosis_upgrade, scope_upgrade)
```

**Step 1 — category_base** (fixed per category):

| Category | Base |
|---|---|
| workflow_failure | WARN |
| drift_stuck | WARN |
| service_unhealthy | ERROR |
| node_unreachable | ERROR |
| auth_denied | WARN |
| phantom_gossip | WARN |

**Step 2 — recurrence_bump:**

- `occurrence_count >= 5` → min ERROR
- `occurrence_count >= 20` → min CRITICAL

**Step 3 — diagnosis_upgrade:**

- Any cited DiagnosisItem with severity=CRITICAL → upgrade to CRITICAL

**Step 4 — scope_upgrade:**

- Incident touches ≥ 2 nodes in a 3-node cluster → min ERROR
- Incident touches all nodes → min CRITICAL

Scanner re-computes severity on every incident update. Severity is never hand-edited.

## 4. Data model

### 4.1 Core proto message

```protobuf
message Incident {
  string      id            = 1;  // stable hash, see 3.2
  string      cluster_id    = 2;
  string      category      = 3;  // see 3.1
  string      signature     = 4;
  IncidentStatus status     = 5;  // see 4.2
  Severity    severity      = 6;  // CRITICAL | ERROR | WARN | INFO

  // Headline is the one-line summary shown in the card title.
  string      headline      = 7;

  // Count of underlying signals since incident opened.
  int32       occurrence_count = 8;
  google.protobuf.Timestamp first_seen_at = 9;
  google.protobuf.Timestamp last_seen_at  = 10;

  // Ordered layers of information (see 5, 6, 7).
  repeated EvidenceItem   evidence       = 11;
  repeated DiagnosisItem  diagnoses      = 12;
  repeated ProposedFix    proposed_fixes = 13;

  // Operator state.
  bool        acknowledged   = 14;
  string      acknowledged_by = 15;
  google.protobuf.Timestamp acknowledged_at = 16;
  string      assigned_to    = 17;      // operator user ID, optional

  // Primary entity reference (see 3.3). First-class for filtering + cross-links.
  string      entity_ref     = 18;
  string      entity_type    = 19;

  // Cross-incident hints (see Q4 in Section 12). Computed, not stored.
  repeated string related_incident_ids = 20;
}

enum IncidentStatus {
  INCIDENT_STATUS_UNKNOWN   = 0;
  INCIDENT_STATUS_OPEN      = 1;  // active signal in last scan
  INCIDENT_STATUS_RESOLVING = 2;  // fix proposed or applied, awaiting verification
  INCIDENT_STATUS_RESOLVED  = 3;  // signal gone N scans in a row (N=3 default)
  INCIDENT_STATUS_ACKED     = 4;  // operator acknowledged, suppresses notifications
}
```

### 4.2 Lifecycle

```
      (new signal)
           ↓
        [OPEN] ──ack──→ [ACKED] (still OPEN, just suppressed)
           │
           │ operator applies fix OR AI auto-remediates
           ↓
       [RESOLVING]
           │
           │ signal absent for N consecutive scans
           ↓
        [RESOLVED]
           │
           │ signal reappears
           ↓
         [OPEN]   (new occurrence_count resets; first_seen_at unchanged)
```

**Rule:** incidents are never deleted. They transition to RESOLVED and persist for the retention window (30 days). If they reappear, the SAME incident_id re-opens. This gives operators reliable incident history.

### 4.3 Resolution criteria (per-category N)

Default: incident moves OPEN → RESOLVED when its signal is absent for **N = 3 consecutive scans**. But some categories need stricter or looser rules:

| Category | Resolution N | Reasoning |
|---|---|---|
| workflow_failure | 3 | default — avoid flapping |
| drift_stuck | 2 | drift clears quickly once fix lands |
| service_unhealthy | 3 | units may restart-loop briefly |
| node_unreachable | 5 | heartbeat flaps are common on WiFi |
| auth_denied | 1 | no reason to keep "unauthorized" open |
| phantom_gossip | 10 | phantoms linger; high bar to close |

Override via per-category config map in the scanner. MVP ships with these defaults.

### 4.4 Ordering rules

Within an incident, items render in a fixed order so operators can scan cards quickly:

| Collection | Order |
|---|---|
| `evidence[]` | newest `observed_at` first |
| `diagnoses[]` | highest severity first, then newest `diagnosed_at` |
| `proposed_fixes[]` | confidence DESC (high → medium → low), then newest |

Reasoning: operators start at the top of a card and stop reading once they have enough signal. "What's freshest?" for raw facts. "What's worst?" for diagnoses. "What's most confident?" for fixes.

## 5. Provenance taxonomy

Every statement in an incident has a **provenance layer** badge. Four layers, in increasing interpretation depth:

| Layer | Meaning | Examples |
|---|---|---|
| **Observed** | Raw measurement from a single source | workflow_step_outcomes.failure_count=12, unit state=failed |
| **Correlated** | Two or more Observed facts joined on entity or time | step failure + drift unresolved on same entity_ref |
| **Diagnosed** | Rule-based inference from a Correlated picture | cluster_doctor invariant fired |
| **AI Proposed** | LLM/ai_executor inference with confidence score | probable cause + patch proposal |

Visual badges (frontend):

- `Observed` → neutral grey
- `Correlated` → blue
- `Diagnosed` → yellow
- `AI Proposed` → purple, with confidence score

**Rule:** no statement appears in a card without its provenance badge. Operators must always be able to answer "who said this?".

## 6. Evidence items (`Observed` + `Correlated`)

```protobuf
message EvidenceItem {
  string      id          = 1;
  Provenance  provenance  = 2;  // OBSERVED or CORRELATED only
  string      source      = 3;  // "workflow.step_outcomes", "node_agent", "systemd", …
  string      summary     = 4;  // one-line human-readable
  map<string, string> facts = 5; // structured key→value
  google.protobuf.Timestamp observed_at = 6;
}
```

Example:

```json
{
  "provenance": "OBSERVED",
  "source": "workflow.step_outcomes",
  "summary": "Step mark_workload_ready failed 30× out of 30 executions",
  "facts": {
    "workflow_name": "node.bootstrap",
    "step_id": "mark_workload_ready",
    "failure_rate": "1.00",
    "last_error": "timeout waiting for workload_ready"
  }
}
```

## 7. Diagnosis items (`Diagnosed`)

```protobuf
message DiagnosisItem {
  string      id           = 1;
  string      source       = 2;  // "cluster_doctor", "workflow.invariants", …
  string      invariant_id = 3;  // e.g. "workflow.drift_stuck"
  string      summary      = 4;
  repeated string cited_evidence_ids = 5; // references evidence[i].id
  Severity    severity     = 6;
  google.protobuf.Timestamp diagnosed_at = 7;
}
```

Diagnoses **cite evidence**. A diagnosis without cited evidence is rejected by the backend (guards against fabrication).

## 8. Proposed fixes (`AI Proposed`)

```protobuf
message ProposedFix {
  string      id           = 1;
  string      proposer     = 2;  // "ai_executor", "operator_suggestion"
  string      summary      = 3;  // one-line: "Change api_port 56093 → 10000"
  string      confidence   = 4;  // "high" | "medium" | "low"
  string      reasoning    = 5;  // 1-3 sentences citing cited_evidence
  repeated string cited_evidence_ids = 6;
  repeated string cited_diagnosis_ids = 7;

  // Concrete change, one of:
  CodePatch   code_patch   = 8;
  ConfigPatch config_patch = 9;
  CommandList command_list = 10;
  RestartAction restart_action = 11;

  // Application state.
  FixStatus   status       = 12;
  string      applied_by   = 13;
  google.protobuf.Timestamp applied_at = 14;
  string      application_result = 15; // diff output, error message, etc.
}

message CodePatch {
  string file_path  = 1;
  int32  line       = 2;
  string old_text   = 3;
  string new_text   = 4;
  string repository = 5;  // git repo identifier
}

enum FixStatus {
  FIX_STATUS_PROPOSED = 0;
  FIX_STATUS_APPROVED = 1;  // operator clicked Apply
  FIX_STATUS_APPLIED  = 2;  // change landed (commit made / config written)
  FIX_STATUS_REJECTED = 3;  // operator clicked Reject with reason
  FIX_STATUS_FAILED   = 4;  // application attempted but error
}
```

**Rules:**

1. Only AI-proposed fixes have `confidence` fields. Operator-suggested fixes skip it.
2. Every ProposedFix must cite ≥1 evidence OR diagnosis. Enforced at proposal-ingest time.
3. The operator (or another AI with authority) approves before application.

## 9. Operator actions

```protobuf
message IncidentAction {
  string incident_id = 1;
  string action      = 2;  // "ack" | "retry" | "apply_fix" | "reject_fix" | "assign" | "dismiss"
  string actor       = 3;  // operator user id or service id
  string fix_id      = 4;  // for apply_fix / reject_fix
  string comment     = 5;  // required for reject_fix
}
```

Actions are logged append-only. The full action history is queryable and visible on the card.

## 10. Backend: GetIncidents endpoint

### 10.1 Contract

```protobuf
service WorkflowService {
  rpc ListIncidents(ListIncidentsRequest) returns (ListIncidentsResponse);
  rpc GetIncident(GetIncidentRequest)     returns (Incident);
  rpc ApplyIncidentAction(IncidentAction) returns (google.protobuf.Empty);
}

message ListIncidentsRequest {
  string cluster_id       = 1;
  IncidentStatus status   = 2; // 0 = all
  int32  limit            = 3;
  string page_token       = 4;
}
```

### 10.2 Join logic

Pseudo-SQL for incident aggregation (runs periodically and writes to an `incidents` table):

```
-- 1. Workflow failures → incident per (workflow, step, failure_reason)
INSERT incidents (category='workflow_failure', signature=workflow||'/'||step_id, ...)
  SELECT workflow_name, step_id, MAX(last_error_message) as reason,
         SUM(failure_count) as occurrences
    FROM workflow.workflow_step_outcomes
   WHERE failure_count > 0
   GROUP BY workflow_name, step_id;

-- 2. Drift stuck → incident per (drift_type, entity_ref)
INSERT incidents (category='drift_stuck', signature=drift_type||'/'||entity_ref, ...)
  SELECT drift_type, entity_ref, consecutive_cycles
    FROM workflow.drift_unresolved
   WHERE consecutive_cycles >= 3;

-- 3. Attach doctor findings as DiagnosisItems where entity matches
UPDATE incidents SET diagnoses = diagnoses ++ [doctor finding]
  WHERE signature contains doctor_finding.entity_ref;

-- 4. Attach ai_executor proposals (by incident_id)
UPDATE incidents SET proposed_fixes = proposed_fixes ++ [ai proposal]
  WHERE ai_proposal.target_incident_id = incident.id;
```

### 10.3 Storage

New tables in `workflow` keyspace:

```sql
CREATE TABLE workflow.incidents (
  cluster_id text, id text,
  category text, signature text,
  status int, severity int,
  headline text,
  occurrence_count int,
  first_seen_at timestamp, last_seen_at timestamp,
  acknowledged boolean, acknowledged_by text, acknowledged_at timestamp,
  assigned_to text,
  evidence_json text, diagnoses_json text, proposed_fixes_json text,
  PRIMARY KEY ((cluster_id), id)
);

CREATE TABLE workflow.incident_actions (
  cluster_id text, incident_id text, action_at timestamp, action_id text,
  action text, actor text, fix_id text, comment text,
  PRIMARY KEY ((cluster_id, incident_id), action_at, action_id)
) WITH CLUSTERING ORDER BY (action_at DESC, action_id ASC);
```

### 10.4 AI proposal ingestion

ai_executor writes proposals via a separate RPC:

```protobuf
rpc SubmitProposedFix(SubmitProposedFixRequest) returns (ProposedFix);
```

Validation: reject if `cited_evidence_ids` and `cited_diagnosis_ids` are both empty. "Grounded AI" only.

## 11. Frontend: IncidentCard component

### 11.1 Component shape

```tsx
<IncidentCard incident={incident}>
  <CardHeader>
    <SeverityBadge severity={incident.severity} />
    <Headline>{incident.headline}</Headline>
    <OccurrenceCount>{incident.occurrence_count}×</OccurrenceCount>
    <Timestamp>{incident.last_seen_at}</Timestamp>
  </CardHeader>

  <Section title="What happened">
    {incident.evidence.map(e => (
      <EvidenceLine item={e}>
        <ProvenanceBadge layer={e.provenance} />
        <SourceBadge>{e.source}</SourceBadge>
        {e.summary}
      </EvidenceLine>
    ))}
  </Section>

  <Section title="Diagnosis">
    {incident.diagnoses.map(d => (
      <DiagnosisLine item={d}>
        <ProvenanceBadge layer="Diagnosed" />
        <SourceBadge>{d.source}</SourceBadge>
        {d.summary}
        <CitedEvidence ids={d.cited_evidence_ids} />
      </DiagnosisLine>
    ))}
  </Section>

  <Section title="Proposed Fix">
    {incident.proposed_fixes.map(f => (
      <ProposedFixCard fix={f}>
        <ProvenanceBadge layer="AI Proposed" />
        <ConfidenceBadge level={f.confidence} />
        <Reasoning>{f.reasoning}</Reasoning>
        <PatchDiff patch={f.code_patch} />
        <ActionRow>
          <button>View Diff</button>
          <button>Apply Patch</button>
          <button>Reject</button>
        </ActionRow>
      </ProposedFixCard>
    ))}
  </Section>

  <ActionBar>
    <button>Acknowledge</button>
    <button>Retry Workflow</button>
    <button>Dismiss</button>
  </ActionBar>
</IncidentCard>
```

### 11.2 Provenance badges

Rendered as pill-shaped labels with distinct colors:

| Layer | Color | Icon |
|---|---|---|
| Observed | grey | 👁 |
| Correlated | blue | 🔗 |
| Diagnosed | yellow | 🔍 |
| AI Proposed | purple | 🤖 |

## 12. Open questions

1. **How many fixes per incident?** — Start with 1. Multi-fix (branch on operator choice) can come later.

2. **Who opens incidents?** — Backend scanner only (not operators manually). Operators can only act on auto-created incidents. (Keeps the model clean.)

3. **Fix application path** — For `CodePatch`, do we write commits directly, or open a PR? Proposal: open a PR, operator reviews & merges. Autonomous commit is a separate feature flag.

4. **Cross-incident correlation hint** — If incident A's fix would resolve incident B, how to surface that? Proposal: `related_incidents []string` field, computed only when obvious (shared entity_ref), displayed as "Related" sidebar chip.

5. **Notification channels** — MVP uses event topics (`incident.opened`, `incident.resolved`). Email/Slack later.

6. **Suppression** — ACKED incidents suppress notifications but stay visible. No `muted_until` timer in MVP.

---

## 13. Implementation order

1. Proto messages (Section 4) + regenerate Go/TS
2. ScyllaDB schema (Section 10.3)
3. Scanner that writes incidents from existing telemetry tables (Section 10.2)
4. `ListIncidents` + `GetIncident` + `ApplyIncidentAction` RPCs
5. `SubmitProposedFix` RPC + validation
6. Minimal `IncidentCard` frontend fed by `ListIncidents`
7. ai_executor integration to submit proposals
8. Operator action wiring (Apply → ai_executor applies patch → records action)

Stop at step 6 to validate the mental model with real operators before going further.

---

## 14. Success criteria

The design is working when:

- An operator reading a single card can decide whether to act in <30 seconds.
- An AI agent can read `Incident` JSON and produce a grounded patch proposal.
- "What's broken?" query returns ≤ # incidents, never # raw events.
- Resolved incidents reappearing under the same `incident_id` is visible as history, not as new alerts.
- Operators can predict which incident a new failure will land under.
