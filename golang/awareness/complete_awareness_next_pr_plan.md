# Complete PR Plan — Awareness Decision Trace + Context Navigation

**Project:** Globular Awareness  
**Target PR:** `awareness: add decision traces and context navigation pivots`  
**Status basis:** actual uploaded `awareness.tar.gz` code + the five planning documents defining backend stabilization, trust envelope, context navigation, open requirements, and deferred frontend awareness.  
**Primary goal:** turn Awareness from a warning board into an agent-facing diagnostic cockpit.

---

## 0. Executive Decision

Do **one complete backend Awareness PR** now:

```text
Add decision traces and ranked context navigation to preflight and MCP surfaces,
using the existing trust envelope, graph traversal, semantic ranking, node-context,
fix ledger, runtime evidence, and experience stores.
```

Do **not** start frontend awareness in this PR.

The backend trust envelope work is already largely present in the uploaded code:

- `preflight.Report` already has `Trust *assurance.TrustEnvelope`.
- `preflight.Run` already calls `computeTrustEnvelope(...)` near the end of the run.
- `assurance.Compose(...)` already exists and encodes `trusted`, `usable`, `limited`, `stale`, `unknown`, and `unsafe` verdicts.
- `assurance/integration_test.go` already covers the black-box path: `git diff -> preflight -> freshness -> coverage -> Compose() -> TrustEnvelope`.
- Agent and JSON formatting already surface the trust envelope.

So the next high-leverage PR should **not redo P0-1/P0-2 from scratch**. It should build the missing layer above them:

```text
Trust envelope tells the agent whether Awareness deserves trust.
Decision trace tells the agent why the finding exists and where to go next.
```

This PR should keep the backend stabilization rule: **use existing mechanisms first, consolidate duplicated logic, and avoid new subsystems unless the current trust path requires it.**

---

## 1. Why This PR Exists

Current Awareness output is already safety-aware, but still too linear for rapid incident work.

An agent can see:

```text
failure_mode:X
invariant:Y
forbidden_fix:Z
trust: limited/stale/unknown
```

But it still has to manually answer:

```text
Why did this match?
Which evidence produced it?
Which layer owns it: repository, desired, installed, runtime?
Which incidents/fixes/tests are relevant?
Which actions are safe?
Which fixes are forbidden?
What evidence would prove the diagnosis wrong?
```

This PR adds those answers directly to preflight output.

The desired result is:

```yaml
finding: failure_mode:workflow_resume_without_receipt
trust:
  verdict: limited
owner:
  layer: runtime
  service: workflow
matched_by:
  - source: graph
    path_summary: failure_mode -> violates -> invariant
  - source: runtime
    freshness: fresh
pivots:
  - source_invariant
  - prior_incident
  - fix_case
  - required_test
  - forbidden_fix
next_actions:
  - inspect node context
  - rebuild graph if stale
  - collect live snapshot if runtime overlay absent
falsifiers:
  - terminal receipt exists for failed workflow step
  - no retry storm exists in recent workflow runs
```

---

## 2. Actual Code Baseline

The uploaded `awareness.tar.gz` already contains these useful building blocks.

### 2.1 Existing preflight composition

Files:

```text
preflight/report.go
preflight/preflight.go
preflight/format.go
preflight/raw_fallback.go
preflight/*_test.go
```

Current important fields in `preflight.Report`:

```go
Invariants          []string
FailureModes        []string
ForbiddenFixes      []string
RequiredTests       []string
RequiredSearches    []string
RecommendedOrder    []string
Warnings            []string
RawKnowledgeMatches []RawKnowledgeMatch
Runtime             *RuntimeSection
GraphFreshness      *GraphFreshnessReport
LiveOverlay         *LiveOverlayFreshness
ExperienceHints     []ExperienceHint
Trust               *assurance.TrustEnvelope
```

Important current behavior:

```text
preflight.Run(...)
  -> agent context
  -> impact merge
  -> raw YAML fallback
  -> runtime merge
  -> live overlay freshness
  -> experience hints
  -> confidence / safety / degraded mode
  -> computeTrustEnvelope(...)
```

This makes `preflight.Run` the correct composition point for decision traces.

### 2.2 Existing trust envelope

Files:

```text
assurance/envelope.go
assurance/coverage.go
assurance/freshness.go
assurance/integration_test.go
```

Current trust vocabulary:

```go
TrustVerdict:
  trusted | usable | limited | unsafe | stale | unknown

TrustCoverage:
  none | partial | sufficient | strong

FreshnessStatus:
  fresh | stale_repo | stale_runtime | stale_incidents | stale_test_index | stale_unknown | unknown
```

This PR must reuse `assurance.Compose(...)`. Do not create another trust engine.

### 2.3 Existing context/navigation pieces

Files:

```text
context/node_context.go
context/neighborhood.go
semantic/related.go
semantic/why.go
graph/traversal.go
graph/query.go
analysis/agent_context.go
analysis/impact.go
```

Current useful capabilities:

- `awarectx.Build(...)` builds node context with zoom modes.
- `awarectx.Neighborhood(...)` performs bidirectional BFS.
- `semantic.Related(...)` performs weighted related-node traversal and produces `PathSummary`.
- `graph.ImpactByFile(...)` already supports file impact.
- `analysis.GenerateAgentContext(...)` already maps tasks/files to awareness objects.

This PR should compose those APIs instead of rewriting graph traversal.

### 2.4 Existing history/learning/fix sources

Files/packages:

```text
fixledger/*
failurelearning/*
incidentpattern/*
graph/experience_store.go
runtime/*
extractors/workflowstate/*
extractors/doctor/*
```

These are the sources that decision traces should surface as pivots.

### 2.5 Existing tests already protecting trust

The codebase already contains tests named like:

```text
TestCompose_NoMatchIsNeverSafe
TestCompose_StaleBundleBlocksSafetyVerdict
TestCompose_StrongCoverageFreshTrustedVerdict
TestCompose_PartialCoverageCappedAtLimited
TestCompose_OrphanFailureModeIsUnsafe
TestPreflightNoMatchNeverWithoutCoverageAndReason
TestPreflightJSONOutputIsValidAndStable
TestJSONOutputIncludesTrustEnvelope
```

This PR should add tests for decision traces, not duplicate all envelope tests.

---

## 3. PR Scope

### In scope

1. Add decision trace types to `preflight.Report`.
2. Add `analysis/contextnav` package.
3. Infer owner layer per finding.
4. Generate ranked context pivots per finding.
5. Generate evidence chains: graph, raw YAML, alias, runtime, experience, trust.
6. Generate safe diagnostic next actions.
7. Generate falsifiers.
8. Render compact traces in `agent` and Markdown formats.
9. Add JSON output for full traces.
10. Add or update MCP/CLI surfaces so agents can request trace/context pivots directly.
11. Add tests around invariant, failure mode, raw YAML, runtime, stale graph, owner inference, and destructive-command safety.

### Out of scope

1. Do not implement frontend awareness.
2. Do not add detector lifecycle states.
3. Do not change trust verdict semantics unless a bug is discovered.
4. Do not lower CI ratchets.
5. Do not add LLM-based matching inside awareness core.
6. Do not add cluster-mutating remediation commands as automatic actions.
7. Do not replace existing preflight fields.

---

## 4. Proposed Branch and Commit

Branch:

```bash
git checkout -b awareness-decision-trace-context-nav
```

Suggested commit message:

```text
awareness: add decision traces and ranked context pivots

Add per-finding decision traces to preflight output so agent-facing awareness
results explain why a finding matched, which layer owns it, which evidence and
history are relevant, what safe next actions exist, and what evidence would
falsify the diagnosis.

The implementation reuses the existing trust envelope, semantic.Related,
node-context, runtime evidence, raw YAML fallback, fix ledger, and experience
stores instead of adding a second awareness engine.
```

---

## 5. Data Model Changes

Modify:

```text
preflight/report.go
```

Add:

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
    Source      string  `json:"source"`
    NodeID      string  `json:"node_id,omitempty"`
    EdgeKind    string  `json:"edge_kind,omitempty"`
    PathSummary string  `json:"path_summary,omitempty"`
    Confidence  float64 `json:"confidence"`
    Freshness   string  `json:"freshness,omitempty"`
    Reason      string  `json:"reason,omitempty"`
}

type OwnerContext struct {
    Layer    string   `json:"layer,omitempty"`
    Service  string   `json:"service,omitempty"`
    Package  string   `json:"package,omitempty"`
    Files    []string `json:"files,omitempty"`
    Symbols  []string `json:"symbols,omitempty"`
    StateIDs []string `json:"state_ids,omitempty"`
}

type ContextPivot struct {
    Kind        string  `json:"kind"`
    ID          string  `json:"id"`
    Title       string  `json:"title,omitempty"`
    WhyRelevant string  `json:"why_relevant,omitempty"`
    Command     string  `json:"command,omitempty"`
    Confidence  float64 `json:"confidence,omitempty"`
}

type DiagnosticAction struct {
    Kind        string `json:"kind"`
    Command     string `json:"command,omitempty"`
    Reason      string `json:"reason"`
    SafeToRun    bool   `json:"safe_to_run"`
    RequiresAck  bool   `json:"requires_ack,omitempty"`
}

type Falsifier struct {
    Claim      string `json:"claim"`
    HowToCheck string `json:"how_to_check"`
    Command    string `json:"command,omitempty"`
}

type DecisionTrace struct {
    FindingID       string             `json:"finding_id"`
    FindingType     FindingType        `json:"finding_type"`
    Summary         string             `json:"summary,omitempty"`
    Confidence      Confidence         `json:"confidence"`
    ConfidenceScore float64            `json:"confidence_score,omitempty"`
    Trust           *assurance.TrustEnvelope `json:"trust,omitempty"`
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

Compatibility rule:

```text
All existing fields stay. decision_traces is additive.
```

---

## 6. New Package: `analysis/contextnav`

Create:

```text
analysis/contextnav/types.go
analysis/contextnav/build.go
analysis/contextnav/owner.go
analysis/contextnav/pivots.go
analysis/contextnav/evidence.go
analysis/contextnav/falsifiers.go
analysis/contextnav/actions.go
analysis/contextnav/contextnav_test.go
analysis/contextnav/owner_test.go
analysis/contextnav/pivots_test.go
analysis/contextnav/falsifiers_test.go
analysis/contextnav/actions_test.go
```

Primary API:

```go
func BuildDecisionTraces(ctx context.Context, g *graph.Graph, in Inputs) ([]preflight.DecisionTrace, error)
```

Input shape:

```go
type Inputs struct {
    Task                string
    Files               []string
    Invariants          []string
    FailureModes        []string
    ForbiddenFixes      []string
    RawMatches          []preflight.RawKnowledgeMatch
    Runtime             *preflight.RuntimeSection
    ExperienceHints     []preflight.ExperienceHint
    Trust               *assurance.TrustEnvelope
    GraphFreshness      *preflight.GraphFreshnessReport
    LiveOverlay         *preflight.LiveOverlayFreshness
    DocsDir             string
}
```

Import cycle warning:

```text
analysis/contextnav importing preflight may create an import cycle if preflight imports analysis/contextnav.
```

Preferred implementation to avoid cycles:

- Put trace data types in a small neutral package:

```text
analysis/contextnav/model.go
```

or:

```text
preflight/trace_model.go
```

Then make `contextnav` return neutral structs and let `preflight` attach them.

If import cycles appear, move only the shared model types to:

```text
awareness/trace
```

Do not contort package structure. Keep it boring.

---

## 7. Owner Layer Inference

Implement:

```text
analysis/contextnav/owner.go
```

Goal:

```text
Every finding should say who owns the problem.
```

Layer vocabulary:

```text
repository
wanted/desired
installed
runtime
workflow
pki
dns
rbac
unknown
```

Prefer the Globular 4-layer model naming in output:

```text
repository -> desired -> installed -> runtime
```

Inference rules:

| Layer | Evidence examples |
|---|---|
| repository | package, artifact, manifest, release, repository status, build_id resolution |
| desired | desired service, desired infrastructure, etcd desired keys, ServiceDesiredVersionSpec |
| installed | node-agent installed package, installed_state_record, systemd unit installed state |
| runtime | runtime_state_record, doctor finding, service status, workflow run/receipt, metrics |
| workflow | workflow, workflow_step, workflow_run, workflow_receipt, phase transition |
| pki | certificate, SAN, CA, TLS, x509, CA fingerprint |
| dns | dns_record, service endpoint, advertised hostname, VIP, SAN mismatch |
| rbac | role, permission, subject, service identity |

Ranking by task class:

```text
Runtime incident: runtime > installed > desired > repository
State mismatch: desired > installed > runtime > repository
Package admission: repository > desired > installed > runtime
Workflow/retry loop: workflow > runtime > installed > desired
PKI/DNS issue: pki/dns > runtime > desired
```

Implementation approach:

1. Inspect direct node type first.
2. Inspect direct neighbors with `g.Neighbors(ctx, findingID, "both")`.
3. Inspect `semantic.Related(...)` for service/package/file/state nodes.
4. Use file path hints from `Report.Files`.
5. Use runtime evidence when `Runtime.Included=true`.
6. If unresolved, return `Layer: "unknown"` and add warning.

Acceptance tests:

```text
TestOwnerInference_RepositoryFinding
TestOwnerInference_DesiredFinding
TestOwnerInference_InstalledFinding
TestOwnerInference_RuntimeFinding
TestOwnerInference_WorkflowFinding
TestOwnerInference_UnknownIsExplicit
```

---

## 8. Evidence Chain Builder

Implement:

```text
analysis/contextnav/evidence.go
```

Goal:

```text
Each finding should explain why it appeared.
```

Evidence sources:

```text
graph
alias
raw_yaml
runtime
metrics
fix_ledger
incident_store
experience_store
trust_envelope
```

Confidence guidance:

| Evidence | Base confidence |
|---|---:|
| explicit annotation edge | 0.95 |
| fresh runtime evidence | 0.90 |
| direct graph match | 0.85 |
| semantic related path | 0.70 |
| raw YAML fallback | 0.65 |
| experience hint | 0.60 |
| alias-only match | 0.55 |
| stale graph/runtime | cap at 0.50 |
| inferred low-trust path | cap at 0.40 |

Rules:

- Do not hide low-confidence evidence.
- Label raw YAML as fallback.
- Label alias-only matches as low-confidence.
- Include trust envelope as evidence only to explain gating, not as a graph proof.
- If `GraphFreshness.Stale=true`, cap graph evidence confidence.
- If `LiveOverlay.Status=stale|absent`, cap runtime evidence confidence.

Example evidence:

```yaml
matched_by:
  - source: graph
    node_id: failure_mode:workflow_resume_without_receipt
    path_summary: failure_mode -> violates -> invariant.workflow_receipts_required
    confidence: 0.85
  - source: runtime
    node_id: workflow_receipt:cluster.reconcile:failed
    freshness: fresh
    confidence: 0.90
  - source: trust_envelope
    reason: partial failure-mode coverage caps verdict at limited
    confidence: 1.0
```

Acceptance tests:

```text
TestEvidence_DirectGraphMatch
TestEvidence_RawYAMLFallbackIsMarkedFallback
TestEvidence_AliasOnlyNeverHighConfidence
TestEvidence_StaleGraphCapsConfidence
TestEvidence_RuntimeFreshnessIsIncluded
```

---

## 9. Ranked Context Pivots

Implement:

```text
analysis/contextnav/pivots.go
```

Goal:

```text
From one finding, give the agent the best next graph jumps.
```

Target pivot kinds:

```text
source_invariant
related_failure_mode
forbidden_fix
required_test
fix_case
incident
experience
runbook
debug_playbook
runtime_evidence
source_file
symbol
package
service
```

Use existing API:

```go
semantic.Related(ctx, g, findingNodeID, semantic.RelatedOptions{
    Dimension: semantic.DimensionAll,
    TargetTypes: []string{...},
    MaxDepth: 5,
    MaxResults: 20,
    IncludeRuntime: true,
    IncludeProvenance: true,
})
```

Priority order:

```text
required_test
forbidden_fix
source_invariant
runtime_evidence
incident
fix_case
experience
runbook
service
package
file
symbol
documentation
```

Each pivot must include:

```text
kind
id
title
why_relevant
command when useful
confidence
```

Useful commands:

```bash
globular awareness node-context --node <node> --zoom all --format agent
globular awareness node-context --node <node> --zoom history --format agent
globular awareness neighborhood --node <node> --depth 2 --format agent
```

Acceptance tests:

```text
TestPivots_FailureModeIncludesSourceInvariant
TestPivots_IncludesForbiddenFixAndRequiredTest
TestPivots_IncludesFixCaseWhenLinked
TestPivots_IncludesRuntimeEvidenceWithFreshness
TestPivots_AreDeterministicAndCapped
```

---

## 10. Falsifiers

Implement:

```text
analysis/contextnav/falsifiers.go
```

Goal:

```text
Every diagnosis should tell the agent what would prove it wrong.
```

Rule:

```text
Every DecisionTrace must have at least one falsifier.
```

Templates by family:

### Workflow/retry loop

```yaml
claim: workflow retry loop is active
how_to_check: inspect recent workflow runs for repeated same target/package failure
command: globular awareness preflight --task "workflow retry loop" --include-runtime --format agent
```

```yaml
claim: missing receipt caused unsafe resume
how_to_check: verify that the failed workflow step has a terminal receipt before resume/retry decision
```

### Desired/installed/runtime mismatch

```yaml
claim: desired and installed build_id differ
how_to_check: compare desired service release build_id to node installed package build_id
```

```yaml
claim: installed state is stale
how_to_check: verify node-agent heartbeat and installed package record freshness
```

### Repository/build drift

```yaml
claim: desired.version resolved to a different build_id than installed
how_to_check: compare repository manifest build_id, desired build_id, and installed build_id
```

### DNS/SAN/PKI

```yaml
claim: endpoint is not covered by certificate SAN
how_to_check: inspect certificate SANs and advertised endpoint host/IP
```

### Objectstore topology

```yaml
claim: runtime MinIO topology differs from objectstore desired contract
how_to_check: compare /globular/objectstore/config to runtime MinIO members and health
```

Generic fallback:

```yaml
claim: awareness graph path is still valid
how_to_check: refresh graph and verify the matched graph path/runtime evidence still exists
command: globular awareness build --clean && globular awareness preflight --task "<task>" --format agent
```

Acceptance tests:

```text
TestFalsifiers_WorkflowRetryLoop
TestFalsifiers_StateMismatch
TestFalsifiers_DNSSAN
TestFalsifiers_GenericFallbackAlwaysPresent
```

---

## 11. Diagnostic Actions

Implement:

```text
analysis/contextnav/actions.go
```

Goal:

```text
Warnings should become exact safe next actions.
```

Allowed automatic action kinds:

```text
inspect
test
rebuild
runtime_collect
grep
runbook
stop
```

Examples:

```yaml
- kind: rebuild
  command: globular awareness build --clean
  reason: graph is stale or source YAML is newer than graph
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
  command: go test ./golang/awareness/...
  reason: awareness trace/context navigation changed
  safe_to_run: true
```

Hard rule:

```text
No mutating cluster remediation command may be emitted with safe_to_run=true.
```

If a mutating command is included, it must be:

```yaml
safe_to_run: false
requires_ack: true
```

Acceptance tests:

```text
TestActions_StaleGraphIncludesBuildClean
TestActions_StaleLiveOverlayIncludesLiveSnapshot
TestActions_NodeContextCommandForFinding
TestActions_NoDestructiveCommandWithoutAck
```

---

## 12. Integrate with `preflight.Run`

Modify:

```text
preflight/preflight.go
```

Current end of run roughly does:

```go
r.SafetyStatus = computeSafetyStatus(r)
r.DegradedMode = computeDegradedMode(r)
r.Trust = computeTrustEnvelope(ctx, g, opts, r)
return r, nil
```

Change to:

```go
r.SafetyStatus = computeSafetyStatus(r)
r.DegradedMode = computeDegradedMode(r)
r.Trust = computeTrustEnvelope(ctx, g, opts, r)

traces, err := contextnav.BuildDecisionTraces(ctx, g, contextnav.Inputs{
    Task:            r.Task,
    Files:           r.Files,
    Invariants:      r.Invariants,
    FailureModes:    r.FailureModes,
    ForbiddenFixes:  r.ForbiddenFixes,
    RawMatches:      r.RawKnowledgeMatches,
    Runtime:         r.Runtime,
    ExperienceHints: r.ExperienceHints,
    Trust:           r.Trust,
    GraphFreshness:  r.GraphFreshness,
    LiveOverlay:     r.LiveOverlay,
    DocsDir:         opts.DocsDir,
})
if err != nil {
    r.Warnings = append(r.Warnings, "decision_trace: "+err.Error())
} else {
    r.DecisionTraces = traces
}

return r, nil
```

Special cases:

- If no findings exist, return empty `DecisionTraces`, not nil if practical.
- If graph is nil but raw YAML matches exist, build raw fallback traces.
- If graph is nil and no raw matches exist, no trace is fine, but the report must still remain `UNKNOWN_NOT_SAFE` for sensitive tasks.
- If contextnav fails, preflight should degrade gracefully and include a warning, not fail the whole preflight.

---

## 13. Format Changes

Modify:

```text
preflight/format.go
```

### JSON format

JSON should include full `decision_traces`.

### Agent format

Add compact section after trust and warnings, before long lists:

```text
## Decision traces

finding: failure_mode.workflow_resume_without_receipt
trust: limited / partial / fresh
confidence: medium
owner: runtime / workflow / workflow-service
why:
- graph: failure_mode -> violates -> invariant.workflow_receipts_required
- runtime: workflow receipt shows failed reconcile step
pivots:
- source_invariant: invariant.workflow_receipts_required
- forbidden_fix: forbidden_fix.mark_complete_from_process_exit
- test: TestResumeRequiresReceipt
next:
- globular awareness node-context --node failure_mode:workflow_resume_without_receipt --zoom history --format agent
falsify:
- terminal workflow receipt exists for the failed step
```

Agent format limits:

```text
Top 3 evidence refs
Top 5 pivots
Top 3 next actions
Top 3 falsifiers
```

Markdown format may show more, but should still group by finding.

Acceptance tests:

```text
TestAgentFormatIncludesDecisionTrace
TestAgentFormatDecisionTraceIsCapped
TestMarkdownFormatIncludesDecisionTrace
TestJSONOutputIncludesDecisionTraces
```

---

## 14. MCP / CLI Integration

The uploaded awareness tarball does not include a local `mcp/` package, but existing code references `golang/mcp/tools_awareness` as an awareness MCP tool location. Treat MCP wiring as a repo-level integration step.

### Required audit

Search the full repo for:

```bash
grep -R "awareness_preflight\|awareness.preflight\|decision_context\|pre_edit_context\|impact_file\|match_incident" -n .
```

Update every active agent-facing surface that returns preflight or context guidance.

### Required existing surfaces to inspect

```text
awareness_preflight
awareness_decision_context
awareness_impact_file
awareness_match_incident_patterns
awareness_pre_edit_context
```

### Required output contract

Every tool that gives guidance must include:

```json
{
  "trust": { ... },
  "decision_traces": [ ... ]
}
```

If a tool only returns a single node/finding context, include:

```json
{
  "trust": { ... },
  "decision_trace": { ... }
}
```

### Optional new tools

Add only if low-risk:

```text
awareness_decision_trace
awareness_finding_context
```

CLI equivalents:

```bash
globular awareness decision-trace --task "..." --files a.go,b.go --include-runtime --format agent
globular awareness finding-context --finding failure_mode:workflow_resume_without_receipt --zoom all --include-runtime --format json
```

If CLI plumbing is large, defer new commands and only enrich existing preflight output.

Acceptance tests:

```text
TestMCPPreflightIncludesDecisionTraces
TestMCPDecisionContextIncludesTrustAndTrace
TestMCPToolListIncludesDecisionTraceWhenAdded
```

---

## 15. Graph Cross-Link Improvements Inside This PR

Only add missing graph edges when they are obvious and directly needed by traces.

Required high-value links to verify:

```text
failure_mode -> violates -> invariant
invariant -> forbids -> forbidden_fix
failure_mode/invariant -> tested_by -> test
fix_case -> fixes|partially_fixes -> failure_mode/invariant
incident -> caused_by|fixed_by|validated_by -> failure_mode/fix_case/test
experience -> suggests_next|warns_against -> next_time_hint/forbidden_fix
runtime evidence -> matches_failure_mode|matches_invariant -> finding
source_file -> implements|may_affect -> invariant/service/package
workflow_receipt -> verifies_invariant|records_error -> invariant/failure_mode
```

Do not create high-confidence causal edges from keyword matches only.

If the extractor cannot know causality, use an inferred/provenance-lowered edge or leave the relationship as a pivot, not a graph law.

---

## 16. Required Tests

Minimum new tests:

```text
analysis/contextnav/contextnav_test.go
analysis/contextnav/owner_test.go
analysis/contextnav/pivots_test.go
analysis/contextnav/evidence_test.go
analysis/contextnav/falsifiers_test.go
analysis/contextnav/actions_test.go
preflight/decision_trace_test.go
preflight/format_decision_trace_test.go
```

### Test matrix

#### 1. Invariant trace

Seed:

```text
invariant -> forbids -> forbidden_fix
invariant -> tested_by -> test
```

Assert:

```text
trace includes invariant
trace includes forbidden fix pivot
trace includes required test pivot
trace includes at least one falsifier
```

#### 2. Failure mode trace

Seed:

```text
failure_mode -> violates -> invariant
fix_case -> fixes -> failure_mode
```

Assert:

```text
trace includes source invariant
trace includes fix case pivot
trace includes owner layer when linked
```

#### 3. Raw YAML fallback trace

Seed no graph match, but YAML fallback match.

Assert:

```text
source=raw_yaml
confidence not high
trust not stronger than limited/unknown depending existing envelope
```

#### 4. Runtime trace

Seed runtime doctor finding or workflow receipt linked to failure mode.

Assert:

```text
source=runtime
freshness included
runtime evidence pivot included
```

#### 5. Stale graph action

Make `GraphFreshness.Stale=true`.

Assert:

```text
next_actions includes globular awareness build --clean
graph evidence confidence capped
```

#### 6. Live overlay absent/stale

Set live overlay to absent/stale.

Assert:

```text
next_actions includes globular awareness live-snapshot
```

#### 7. Owner inference

Seed representative nodes/edges for repository, desired, installed, runtime.

Assert:

```text
correct layer selected
unknown layer explicitly reported when unresolved
```

#### 8. No destructive command

Assert:

```text
No command containing deploy/apply/delete/restart/systemctl/etcdctl put/rm is safe_to_run=true unless RequiresAck=true.
```

#### 9. Agent format

Assert:

```text
agent output contains Decision traces
agent output contains finding, owner, why, pivots, next, falsify
agent output remains capped for long pivot lists
```

---

## 17. Commands to Run

From the repo root:

```bash
go test ./golang/awareness/...
```

If Awareness is a module/subtree:

```bash
cd golang/awareness
go test ./...
```

Also run any existing MCP tests if the full repo contains them:

```bash
go test ./golang/mcp/...
```

Recommended targeted runs during development:

```bash
go test ./golang/awareness/analysis/contextnav -run TestOwnerInference
go test ./golang/awareness/preflight -run DecisionTrace
go test ./golang/awareness/preflight -run Format
go test ./golang/awareness/assurance -run TestE2E
```

Do not lower ratchet thresholds to make tests pass.

---

## 18. Acceptance Criteria

The PR is complete when:

1. `preflight.Report` includes `DecisionTraces []DecisionTrace`.
2. `preflight --format json` includes full decision traces.
3. `preflight --format agent` includes compact decision traces.
4. Every trace includes:
   - finding id/type
   - confidence
   - trust envelope or trust reference
   - matched-by evidence
   - owner layer
   - ranked pivots
   - safe next actions
   - falsifiers
5. Raw YAML fallback is labeled as fallback and never high-confidence graph proof.
6. Stale graph adds `globular awareness build --clean` as a safe action.
7. Stale/absent live overlay adds `globular awareness live-snapshot` as a safe action.
8. No destructive command is marked safe without acknowledgement.
9. Failure mode traces include source invariant when graph edges exist.
10. Fix case, incident, experience, forbidden fix, and required test pivots appear when graph edges exist.
11. Existing trust envelope tests still pass.
12. Existing preflight JSON/agent compatibility tests still pass.
13. MCP agent-facing tools include `trust` and `decision_traces` where applicable.
14. Frontend awareness remains untouched except for comments/TODOs if unavoidable.

---

## 19. Explicit Non-Goals / Guardrails

Do not do these in this PR:

```text
Do not start frontend extractors.
Do not add detector lifecycle.
Do not infer causal truth from keyword matches.
Do not hide stale graph or stale runtime.
Do not treat NO_MATCH as safe.
Do not emit destructive remediation as safe_to_run=true.
Do not replace node-context, semantic.Related, or preflight.
Do not introduce LLM calls into awareness core.
Do not make the graph mutate cluster state.
```

This is a composition PR, not an expansion PR.

---

## 20. Follow-Up PRs After This

### Follow-up A — MCP polish and direct navigation tools

If not completed in this PR:

```text
awareness_decision_trace
awareness_finding_context
```

### Follow-up B — Shared ID helpers

The open requirements still call for exported node ID helpers:

```go
FailureModeNodeID(id string) string
FailureModeIDFromNode(nodeID string) string
```

The current `assurance/coverage.go` still has an internal `failureModeNodePrefix`. Exporting helpers is small and should happen soon, especially before more navigation code joins graph IDs to table IDs.

### Follow-up C — CoverageFor API

Add:

```go
func (r *CoverageReport) CoverageFor(fmID string) *FailureModeCoverage
```

Then use it in preflight/contextnav instead of scanning `PerFailureMode`.

### Follow-up D — Closure loop visible in trust envelope

Add optional fields:

```go
LearnedFromIncident string `json:"learned_from_incident,omitempty"`
RegressionTest      string `json:"regression_test,omitempty"`
```

Only after decision traces are stable.

### Follow-up E — Frontend awareness F0/F1

Start only after one backend cycle proves:

```text
trust envelope stable
context navigation useful
decision traces not noisy
ratchets green
```

---

## 21. Implementation Order for Claude/Codex

1. Read:

```text
preflight/report.go
preflight/preflight.go
preflight/format.go
analysis/agent_context.go
analysis/impact.go
context/node_context.go
context/neighborhood.go
semantic/related.go
graph/nodes.go
graph/edges.go
fixledger/*
failurelearning/*
incidentpattern/*
graph/experience_store.go
runtime/*
assurance/envelope.go
assurance/coverage.go
```

2. Add decision trace model.
3. Add `analysis/contextnav` package skeleton.
4. Implement owner inference.
5. Implement evidence refs.
6. Implement pivots.
7. Implement falsifiers.
8. Implement safe next actions.
9. Wire into `preflight.Run` after `Trust` is computed.
10. Update JSON/Markdown/agent formatting.
11. Add tests.
12. Audit MCP surfaces and propagate `decision_traces`.
13. Run tests.
14. Update docs only if needed, especially `docs/awareness/CLAUDE.md` or equivalent agent routine notes.

---

## 22. The One-Line Purpose

```text
After this PR, an AI agent should not only know that Awareness found a risk.
It should know why, who owns it, what to inspect next, what not to do,
and what evidence would prove the diagnosis wrong.
```
