# Claude/Codex Instructions: Improve Awareness Context Navigation and Decision Trace

## Mission

Improve the Globular Awareness graph so an AI agent can move from a vague finding to useful, decision-grade context quickly.

The target behavior is:

> When `awareness preflight`, `node-context`, or MCP tools return a finding, the agent should immediately see **why the finding exists**, **which evidence produced it**, **which layer owns it**, **what previous incidents/fixes are relevant**, **which actions are safe**, **which actions are forbidden**, and **what evidence would falsify the diagnosis**.

Today the system already has a strong skeleton: graph traversal, semantic related search, node context zoom, preflight reports, raw YAML fallback, runtime overlay, fix ledger, experience hints, trust envelope, and live freshness. The gap is not “invent awareness.” The gap is to compose those pieces into a better agent-facing diagnostic/navigation layer.

This is a context-routing improvement. Treat it as an avionics upgrade, not a new airplane. 🛩️

---

## Current Code Anchors

Use the existing code. Do not rewrite the awareness system.

Important existing files/packages:

- `preflight/report.go`
  - Existing `Report` already exposes `Invariants`, `FailureModes`, `ForbiddenFixes`, `RequiredTests`, `RequiredSearches`, `RecommendedOrder`, `Confidence`, `Coverage`, `BlindSpots`, `GraphFreshness`, `LiveOverlay`, `ExperienceHints`, and `Trust`.
- `preflight/preflight.go`
  - Existing `Run` is the right composition point.
  - It already calls `analysis.GenerateAgentContext`, `mergeImpact`, `rawKnowledgeFallback`, `SearchSimilarExperiences`, `mergeRuntime`, `ComputeLiveOverlayFreshness`, and `computeTrustEnvelope`.
- `analysis/agent_context.go`
  - Existing agent context is keyword/alias driven and useful, but too summary-oriented.
  - It returns IDs, but not enough path/evidence/falsifier metadata.
- `graph/traversal.go`
  - Existing `Traverse` and `ImpactByFile` are useful, but `Traverse` is outgoing-only and does not rank or preserve full path explanations.
- `context/node_context.go`
  - Existing `Build` supports semantic zoom: local/module/service/architecture/runtime/history/all.
  - This is the right place to enrich node-specific pivot context.
- `context/neighborhood.go`
  - Existing bidirectional BFS is useful but linear and unranked.
- `semantic/related.go`
  - Existing weighted traversal already ranks context by semantic distance. Reuse it.
- `fixledger/*`
  - Existing fix cases and guardrails should become first-class context pivots.
- `failurelearning/*`, `learning/*`, `incidentpattern/*`, `graph/experience_store.go`
  - Existing learning/incident/experience material should be connected to findings.
- `runtime/*`, `extractors/workflowstate/*`, `extractors/clusterstate/*`, `extractors/doctor/*`
  - Existing runtime facts should explain “why now” when available.
- `graph/nodes.go` and `graph/edges.go`
  - Many node and edge types already exist: `incident`, `fix_case`, `experience`, `runbook`, `debug_playbook`, `doctor_finding`, `workflow_receipt`, `state_delta`, `desired_state_record`, `installed_state_record`, `runtime_state_record`, `convergence_record`, `drift_record`, etc.
  - Prefer reusing existing node/edge types before adding new ones.

---

## Problem Statement

The current awareness output is helpful but still makes the agent do too much manual navigation.

Main gaps:

1. **A finding does not carry a decision trace.**
   - The agent sees `invariant:X` or `failure_mode:Y`, but not the evidence chain that caused the match.
   - The agent cannot easily tell whether the match came from graph edges, raw YAML, alias match, runtime finding, stale cache, or inferred low-trust path.

2. **Context pivots are not explicit enough.**
   - From a finding, the agent should jump to:
     - source invariant
     - last related incidents
     - previous fixes / remaining gaps
     - owning layer: repository / desired / installed / runtime
     - owning service/package/files/symbols
     - required tests
     - forbidden fixes
     - diagnostic commands
     - falsifying evidence

3. **Graph navigation is still too linear.**
   - `Neighborhood` gives a bidirectional BFS bucket list.
   - `semantic.Related` gives ranked related nodes.
   - But preflight does not yet expose a compact “best next pivots” view per finding.

4. **Warnings are sometimes generic.**
   - Warnings should name exact recovery commands, confidence, source freshness, and the safest next action.

5. **Cross-link density is not high enough.**
   - Findings should auto-link to prior fixes, known forbidden fixes, incidents, experiences, package ownership, tests, and runtime proof nodes.

---

## Desired Agent Experience

For a task like:

```text
Fix Day-1 workflow retry loop after package install failure.
```

Awareness should return something like:

```yaml
finding: failure_mode.workflow_resume_without_receipt
confidence: high
matched_by:
  - alias: retry loop
  - runtime: failed workflow run wf-2026-...
  - graph: failure_mode -> violates -> invariant.workflow_receipts_required
owner:
  layer: runtime
  service: workflow
  package: workflow-service
  files:
    - golang/workflow/engine/...
    - golang/cluster-controller/reconcile/...
source_invariant:
  id: invariant.workflow_receipts_required
  reason: workflow completion must be proven by receipts before resume/retry decisions
last_incidents:
  - INC-2026-0007: orphan scanner resumed completed runs
  - INC-2026-0012: blocked release retried without receipt classification
previous_fixes:
  - fix_case.workflow_resume_receipt_gate: partial
remaining_gaps:
  - add regression test for failed step with no receipt
forbidden_fixes:
  - do not mark workflow complete only from process exit
required_tests:
  - TestResumeRequiresReceipt
  - TestBlockedReleaseRetryClassification
recommended_commands:
  - globular awareness node-context --node failure_mode:workflow_resume_without_receipt --zoom history --format agent
  - globular awareness preflight --task "Fix Day-1 workflow retry loop after package install failure" --include-runtime --format agent
falsifiers:
  - latest workflow run has terminal receipt for failed step
  - no retry storm observed in workflow runs during runtime window
  - installed and desired build_id match on all target nodes
```

That is the desired “decision trace” flavor.

---

## Implementation Plan

### Phase 1: Add Decision Trace Types to Preflight

Modify `preflight/report.go`.

Add these structs:

```go
type FindingType string

const (
    FindingInvariant    FindingType = "invariant"
    FindingFailureMode  FindingType = "failure_mode"
    FindingForbiddenFix FindingType = "forbidden_fix"
    FindingRawKnowledge FindingType = "raw_knowledge"
    FindingRuntime      FindingType = "runtime"
    FindingExperience   FindingType = "experience"
)

type EvidenceRef struct {
    Source      string  `json:"source"`       // graph | raw_yaml | runtime | metrics | fix_ledger | incident_store | experience_store | alias
    NodeID      string  `json:"node_id,omitempty"`
    EdgeKind    string  `json:"edge_kind,omitempty"`
    PathSummary string  `json:"path_summary,omitempty"`
    Confidence  float64 `json:"confidence"`
    Freshness   string  `json:"freshness,omitempty"` // fresh | stale | unknown | absent
    Reason      string  `json:"reason,omitempty"`
}

type OwnerContext struct {
    Layer    string   `json:"layer,omitempty"` // repository | desired | installed | runtime | workflow | pki | dns | rbac | unknown
    Service  string   `json:"service,omitempty"`
    Package  string   `json:"package,omitempty"`
    Files    []string `json:"files,omitempty"`
    Symbols  []string `json:"symbols,omitempty"`
    StateIDs []string `json:"state_ids,omitempty"`
}

type ContextPivot struct {
    Kind        string  `json:"kind"` // source_invariant | incident | fix_case | experience | forbidden_fix | required_test | runbook | runtime_evidence | file | symbol | package | service
    ID          string  `json:"id"`
    Title       string  `json:"title,omitempty"`
    WhyRelevant string  `json:"why_relevant,omitempty"`
    Command     string  `json:"command,omitempty"`
    Confidence  float64 `json:"confidence,omitempty"`
}

type DiagnosticAction struct {
    Kind        string `json:"kind"` // inspect | test | rebuild | runtime_collect | grep | runbook | stop
    Command     string `json:"command,omitempty"`
    Reason      string `json:"reason"`
    SafeToRun    bool   `json:"safe_to_run"`
    RequiresAck  bool   `json:"requires_ack,omitempty"`
}

type Falsifier struct {
    Claim       string `json:"claim"`
    HowToCheck  string `json:"how_to_check"`
    Command     string `json:"command,omitempty"`
}

type DecisionTrace struct {
    FindingID       string             `json:"finding_id"`
    FindingType     FindingType        `json:"finding_type"`
    Summary         string             `json:"summary,omitempty"`
    Confidence      Confidence         `json:"confidence"`
    ConfidenceScore float64            `json:"confidence_score,omitempty"`
    MatchedBy       []EvidenceRef      `json:"matched_by"`
    Owner           OwnerContext       `json:"owner"`
    Pivots          []ContextPivot     `json:"pivots"`
    NextActions     []DiagnosticAction `json:"next_actions"`
    Falsifiers      []Falsifier        `json:"falsifiers"`
    Warnings        []string           `json:"warnings,omitempty"`
}
```

Then extend `Report`:

```go
DecisionTraces []DecisionTrace `json:"decision_traces,omitempty"`
```

Acceptance criteria:

- Existing JSON fields remain backward-compatible.
- If there are no findings, `decision_traces` is empty, not null.
- Unit tests verify traces are produced for at least:
  - one invariant match
  - one failure mode match
  - one raw YAML fallback match
  - one runtime match when `IncludeRuntime=true`

---

### Phase 2: Create a Context Navigation Package

Add a new package:

```text
analysis/contextnav/
```

Suggested files:

```text
analysis/contextnav/types.go
analysis/contextnav/build.go
analysis/contextnav/owner.go
analysis/contextnav/pivots.go
analysis/contextnav/falsifiers.go
analysis/contextnav/actions.go
analysis/contextnav/contextnav_test.go
```

Primary API:

```go
func BuildDecisionTraces(ctx context.Context, g *graph.Graph, in Inputs) ([]preflight.DecisionTrace, error)
```

Where `Inputs` contains:

```go
type Inputs struct {
    Task              string
    Files             []string
    Invariants        []string
    FailureModes      []string
    ForbiddenFixes    []string
    RawMatches        []preflight.RawKnowledgeMatch
    Runtime           *preflight.RuntimeSection
    ExperienceHints   []preflight.ExperienceHint
    Trust             *assurance.TrustEnvelope
    GraphFreshness    *preflight.GraphFreshnessReport
    LiveOverlay       *preflight.LiveOverlayFreshness
    DocsDir           string
}
```

Use existing graph APIs first:

- `graph.FindNode`
- `graph.Neighbors`
- `graph.ImpactByFile`
- `semantic.Related`
- `awarectx.Build`
- `fixledger.DidWeFix`
- `g.SearchSimilarExperiences`

Do **not** duplicate traversal logic unless needed.

---

### Phase 3: Owner Layer Detection

Implement `owner.go`.

The owner resolver must answer:

```text
Which layer owns this finding?
```

Use graph nodes and edge patterns to infer:

| Layer | Evidence |
|---|---|
| repository | package, service_release, repository_status, artifact, manifest, publish-state, build_id resolution |
| desired | desired_service, desired_infrastructure, desired_state_record, etcd desired keys |
| installed | node_installed_package, installed_state_record, node-agent state, installed build_id |
| runtime | runtime_state_record, systemd_status, workflow_receipt, service status, doctor finding, metrics |
| workflow | workflow, workflow_step, workflow_run, workflow_receipt, state transitions |
| pki | certificate, SAN, CA, TLS, x509 nodes/edges |
| dns | dns_record, service endpoint, advertised endpoint |
| rbac | role, permission, subject, service identity |

Suggested function:

```go
func InferOwner(ctx context.Context, g *graph.Graph, findingNodeID string, task string, files []string) OwnerContext
```

Rules:

1. Prefer explicit graph ownership edges: package/service/file/symbol.
2. Prefer runtime evidence for `IncludeRuntime=true` incident tasks.
3. Prefer file hints when the user supplied target files.
4. If multiple layers match, rank by task class:
   - runtime incident: runtime > installed > desired > repository
   - state mismatch: desired > installed > runtime > repository
   - package admission: repository > desired > installed > runtime
5. If no owner can be inferred, return `Layer: "unknown"` and add a warning.

Acceptance criteria:

- Unit tests for repository/desired/installed/runtime layer inference.
- A finding connected only to a file still resolves service/package when those graph edges exist.
- Unknown owner is explicit and reduces confidence or adds a warning.

---

### Phase 4: Context Pivots Per Finding

Implement `pivots.go`.

For each finding, generate ranked pivots:

1. Source invariant
2. Related failure modes
3. Forbidden fixes
4. Required tests
5. Prior fix cases
6. Incidents / incident reports
7. Experiences / next-time hints
8. Runbooks / debug playbooks
9. Runtime proof nodes
10. Files / symbols / package / service

Use `semantic.Related` with dimensions and target node types:

```go
semantic.Related(ctx, g, findingNodeID, semantic.RelatedOptions{
    Dimension: semantic.DimensionAll,
    TargetTypes: []string{
        graph.NodeTypeInvariant,
        graph.NodeTypeFailureMode,
        graph.NodeTypeForbiddenFix,
        graph.NodeTypeTest,
        graph.NodeTypeFixCase,
        graph.NodeTypeIncident,
        graph.NodeTypeIncidentReport,
        graph.NodeTypeExperience,
        graph.NodeTypeNextTimeHint,
        graph.NodeTypeRunbook,
        graph.NodeTypeDebugPlaybook,
        graph.NodeTypeRuntimeServiceStatus,
        graph.NodeTypeWorkflowReceipt,
        graph.NodeTypeStateDelta,
        graph.NodeTypeSourceFile,
        graph.NodeTypeSymbol,
        graph.NodeTypePackage,
        graph.NodeTypeGlobularService,
    },
    MaxDepth: 5,
    MaxResults: 20,
    IncludeRuntime: true,
    IncludeProvenance: true,
})
```

Rank pivots by usefulness, not just distance:

```text
required_test > forbidden_fix > source_invariant > runtime_evidence > incident > fix_case > experience > runbook > file > symbol > documentation
```

Each pivot must include `WhyRelevant`, generated from the edge path:

```text
failure_mode:workflow_resume_without_receipt --violates--> invariant:workflow_receipts_required
```

Acceptance criteria:

- A failure mode trace includes at least one source invariant when graph edges exist.
- A finding connected to a fix case includes fix case status or remaining gap when available.
- A finding connected to runtime evidence includes freshness.
- Pivots are capped and sorted deterministically.

---

### Phase 5: Decision Evidence Chain

Implement evidence chain construction in `build.go`.

Each finding needs `MatchedBy` entries.

Examples:

```yaml
matched_by:
  - source: alias
    reason: task phrase "restart storm" matched context_aliases.yaml
    confidence: 0.8
  - source: graph
    node_id: failure_mode:systemd_restart_storm
    edge_kind: violates
    path_summary: failure_mode:systemd_restart_storm --violates--> invariant:restart_singleflight
    confidence: 0.9
  - source: runtime
    node_id: runtime_service_status:workflow@nuc
    reason: systemd status shows repeated restart
    freshness: fresh
    confidence: 0.95
```

Sources to support:

- `graph`
- `alias`
- `raw_yaml`
- `runtime`
- `metrics`
- `fix_ledger`
- `incident_store`
- `experience_store`
- `trust_envelope`

Confidence scoring suggestion:

| Evidence | Base confidence |
|---|---:|
| explicit annotation edge | 0.95 |
| runtime fresh evidence | 0.90 |
| graph direct edge | 0.85 |
| graph semantic related path | 0.70 |
| raw YAML fallback | 0.65 |
| alias-only match | 0.55 |
| stale runtime/graph | cap at 0.50 |
| inferred/low-trust path | cap at 0.40 |

Do not hide low-confidence results. Label them.

Acceptance criteria:

- Trace confidence falls when graph/live overlay is stale.
- Raw YAML fallback traces are clearly marked as fallback, not graph proof.
- Alias-only traces never claim high confidence.

---

### Phase 6: Falsifiers

Implement `falsifiers.go`.

For each important finding, generate “what would prove this diagnosis wrong?”

Examples:

#### Workflow retry loop

```yaml
falsifiers:
  - claim: workflow retry loop is active
    how_to_check: inspect recent workflow runs for repeated same target/package failure
    command: globular awareness preflight --task "workflow retry loop" --include-runtime --format agent
  - claim: missing receipt caused unsafe resume
    how_to_check: list step outcomes and verify a terminal receipt exists for the failed step
```

#### Desired/installed mismatch

```yaml
falsifiers:
  - claim: desired and installed build_id differ
    how_to_check: compare desired service release build_id to node installed package build_id
  - claim: installed state is stale
    how_to_check: verify node-agent heartbeat timestamp and installed package record freshness
```

#### DNS / SAN issue

```yaml
falsifiers:
  - claim: endpoint is not covered by certificate SAN
    how_to_check: inspect certificate SANs and endpoint advertised host/IP
```

Rules:

- Generate from matched failure mode/invariant IDs using deterministic templates.
- Add a generic fallback falsifier when no template exists:

```text
Check whether the graph path and runtime evidence that produced this finding still exist after refresh.
```

Acceptance criteria:

- Every `DecisionTrace` has at least one falsifier.
- Falsifiers are safe to execute or inspect. No destructive command should be suggested.

---

### Phase 7: Diagnostic Actions and Remediation Commands

Implement `actions.go`.

Warnings should become commands where possible.

Examples:

```yaml
next_actions:
  - kind: rebuild
    command: globular awareness build --clean
    reason: graph is stale or missing
    safe_to_run: true
  - kind: runtime_collect
    command: globular awareness live-snapshot
    reason: live overlay is absent or stale
    safe_to_run: true
  - kind: inspect
    command: globular awareness node-context --node failure_mode:workflow_resume_without_receipt --zoom history --format agent
    reason: inspect source invariant, incidents, fixes, and tests
    safe_to_run: true
  - kind: test
    command: go test ./golang/awareness/... ./golang/workflow/...
    reason: required tests matched this failure mode
    safe_to_run: true
```

Rules:

- Commands must be read-only or test/build commands unless explicitly labeled `RequiresAck=true`.
- Do not output cluster-mutating commands as automatic next steps without a guardrail.
- Use existing command style where possible:
  - `globular awareness preflight ...`
  - `globular awareness node-context ...`
  - `globular awareness neighborhood ...`
  - `globular awareness live-snapshot`
  - `globular awareness build --clean`

Acceptance criteria:

- Stale graph warning includes rebuild command.
- Absent/stale live overlay warning includes live-snapshot command.
- Unknown impact includes exact preflight command with files/task.
- No destructive command appears without `RequiresAck=true`.

---

### Phase 8: Integrate Decision Traces into Preflight

Modify `preflight/preflight.go`.

After runtime merge and trust computation, call:

```go
traces, err := contextnav.BuildDecisionTraces(ctx, g, contextnav.Inputs{...})
if err != nil {
    r.Warnings = append(r.Warnings, "decision-trace: "+err.Error())
} else {
    r.DecisionTraces = traces
}
```

Place the call after these fields are populated:

- invariants
- failure modes
- forbidden fixes
- raw YAML matches
- runtime
- experience hints
- graph freshness
- live overlay
- trust envelope

Important: `computeTrustEnvelope` currently runs at the end. Either:

1. Move trust computation before decision trace, then use the trust envelope in traces, or
2. Build traces first and backfill trust-related evidence after `computeTrustEnvelope`.

Prefer option 1 if tests remain clean.

Acceptance criteria:

- `preflight.Run` returns `DecisionTraces` in JSON and agent format.
- Existing tests pass.
- New tests cover stale graph, raw YAML fallback, runtime evidence, and experience hints.

---

### Phase 9: Improve Agent Formatting

Modify `preflight/format.go`.

The `agent` format should include compact decision traces after core warnings and before the long raw lists.

Suggested format:

```text
## Decision traces

finding: failure_mode.workflow_resume_without_receipt
confidence: high
owner: runtime / workflow / workflow-service
why:
- runtime: failed workflow run wf-...
- graph: failure_mode --violates--> invariant.workflow_receipts_required
pivots:
- source_invariant: invariant.workflow_receipts_required
- fix_case: fix_case.workflow_resume_receipt_gate
- incident: INC-2026-...
forbidden:
- do not mark workflow complete only from process exit
next:
- globular awareness node-context --node failure_mode:workflow_resume_without_receipt --zoom history --format agent
falsify:
- terminal workflow receipt exists for the failed step
```

Rules:

- Keep it short by default.
- Include full trace in JSON.
- In agent format, show top 3 pivots, top 3 actions, top 3 falsifiers per finding.
- Sort findings by risk:
  1. forbidden fix
  2. critical invariant
  3. runtime failure mode
  4. raw fallback
  5. experience hint

Acceptance criteria:

- Agent output remains readable under 200 lines for a normal preflight.
- JSON contains full details.
- Markdown can be longer but grouped.

---

### Phase 10: Add a Direct CLI/MCP Navigation Surface

Add or update CLI/MCP tools so agents can pivot without rerunning full preflight.

Suggested commands:

```bash
globular awareness finding-context --finding failure_mode:workflow_resume_without_receipt --format agent
globular awareness finding-context --finding invariant:desired_installed_runtime_consistency --zoom all --include-runtime --format json
globular awareness decision-trace --task "..." --files a.go,b.go --include-runtime --format agent
```

MCP tool names should mirror existing style:

```text
awareness.finding_context
awareness.decision_trace
```

The tool should return:

- decision trace
- node context
- ranked pivots
- exact follow-up commands

Acceptance criteria:

- MCP tools are registered in the same awareness group as preflight/node-context.
- Tool schema is explicit and narrow.
- Tests confirm tool appears in required awareness tool list if such a test exists.

---

## Cross-Link Density Improvements

Add missing graph edges during extraction/build where obvious.

### Required links

1. `failure_mode -> violates -> invariant`
2. `invariant -> forbids -> forbidden_fix`
3. `invariant/failure_mode -> tested_by -> test`
4. `fix_case -> fixes|partially_fixes -> failure_mode/invariant`
5. `fix_case -> touches_file -> source_file`
6. `incident -> caused_by|fixed_by|validated_by -> failure_mode/fix_case/test`
7. `experience -> produced_lesson|warns_against|suggests_next -> lesson/forbidden_fix/next_time_hint`
8. `runtime evidence -> matches_failure_mode|matches_invariant -> finding`
9. `source_file -> owns/implements/partially_implements/may_affect -> invariant/service/package`
10. `workflow_receipt -> verifies_invariant|records_error -> invariant/failure_mode`

### Edge quality rules

- Use explicit/provenance metadata whenever possible.
- Mark inferred links as inferred, not explicit.
- Add confidence/required flags where the existing `graph.Edge` supports it.
- Do not create high-confidence causal edges from keyword matches only.

Acceptance criteria:

- Add graph integrity tests for missing source-invariant-failure-fix-test chains.
- Add a coverage report field that counts orphan findings:

```json
{
  "orphan_failure_modes": 0,
  "orphan_invariants": 0,
  "failure_modes_without_tests": 0,
  "failure_modes_without_falsifiers": 0
}
```

---

## Required Tests

Add tests near the package being changed.

Minimum test list:

```text
preflight/decision_trace_test.go
analysis/contextnav/contextnav_test.go
analysis/contextnav/owner_test.go
analysis/contextnav/pivots_test.go
analysis/contextnav/falsifiers_test.go
preflight/format_decision_trace_test.go
```

Test scenarios:

1. **Invariant trace**
   - Seed invariant + forbidden fix + test.
   - Preflight task matches invariant.
   - Trace includes source, forbidden fix, required test, falsifier.

2. **Failure mode trace**
   - Seed failure mode `violates` invariant.
   - Trace includes source invariant and fix pivots.

3. **Raw YAML fallback trace**
   - Make graph miss but raw YAML match.
   - Trace confidence is not high.
   - Trace says `source=raw_yaml`.

4. **Runtime trace**
   - Seed runtime doctor finding or workflow failure matching a failure mode.
   - Trace includes freshness and runtime proof pivot.

5. **Stale graph**
   - Mark graph stale.
   - Trace confidence is capped/demoted.
   - Next actions include `globular awareness build --clean`.

6. **Live overlay absent/stale**
   - Trace next actions include `globular awareness live-snapshot`.

7. **Owner inference**
   - Seed repository/desired/installed/runtime nodes.
   - Verify correct layer inference.

8. **No destructive command**
   - Assert `NextActions` contains no mutating cluster command unless `RequiresAck=true`.

---

## Definition of Done

This improvement is done when:

1. `preflight --format json` includes `decision_traces`.
2. `preflight --format agent` gives compact decision traces with:
   - finding
   - confidence
   - owner layer
   - why matched
   - pivots
   - next actions
   - falsifiers
3. A finding can be pivoted into:
   - source invariant
   - prior incidents
   - previous fixes
   - owning layer
   - owning service/package/files
   - required tests
   - forbidden fixes
4. Stale or missing evidence produces exact recovery commands.
5. Raw YAML fallback is labeled as fallback, never as strong graph proof.
6. Runtime evidence includes freshness and collector source.
7. Existing awareness tests pass.
8. New tests prove graph navigation is now evidence-rich and useful to an agent.

---

## Suggested Work Order for Claude/Codex

1. Read these files first:
   - `preflight/report.go`
   - `preflight/preflight.go`
   - `preflight/format.go`
   - `analysis/agent_context.go`
   - `context/node_context.go`
   - `semantic/related.go`
   - `graph/nodes.go`
   - `graph/edges.go`
   - `fixledger/*`
   - `incidentpattern/*`
   - `graph/experience_store.go`
2. Add decision trace structs to `preflight/report.go`.
3. Add `analysis/contextnav` package.
4. Implement owner inference.
5. Implement pivots.
6. Implement evidence refs.
7. Implement falsifiers.
8. Implement diagnostic actions.
9. Wire into `preflight.Run`.
10. Update `preflight/format.go`.
11. Add tests.
12. Run:

```bash
go test ./golang/awareness/...
```

If the awareness module path is different in the repo, run the equivalent package-local test command.

---

## Important Guardrails

- Do not lower safety by making context look more certain than it is.
- Do not hide graph staleness.
- Do not treat `NO_MATCH` as safe.
- Do not turn raw YAML fallback into high-confidence graph evidence.
- Do not suggest destructive remediations without explicit acknowledgement flags.
- Do not replace existing preflight fields; add decision traces alongside them.
- Do not introduce LLM calls into awareness core logic. Matching must remain deterministic.
- Do not let context navigation mutate cluster state.

---

## Why This Matters

This turns Awareness from a **warning board** into a **context cockpit**.

The agent should not just know “something is risky.” It should know:

- where the risk came from,
- which layer owns it,
- what history says,
- which tests protect it,
- what not to do,
- what to inspect next,
- and what evidence would prove the current diagnosis wrong.

That is the difference between a graph that stores knowledge and a graph that actually guides repair.
