# Claude Instructions: Awareness Trust Envelope Wiring + End-to-End Validation

## Context

The assurance layer is doing its job. It exposed that awareness was previously reporting more confidence than the graph deserved. The detector extractor and CI ratchet are already done:

- Detector extractor pass: 68 rules extracted, 50 edges applied, 10 modes flipped to `well_covered`.
- CI ratchet: `--min-well-covered` and `--min-detected` flags landed with tests.

The next work should not add another awareness subsystem. It should finish the trust path so every agent-facing surface carries the same calibrated trust envelope.

## Decision

Do **#3 and #4 together in one PR**.

Do **not** implement #5 yet, except for a small preparatory type/model change if it is trivial and does not alter gating behavior.

Handle #6 as a small honesty cleanup inside the same PR only if it is low-risk. Otherwise leave it as a follow-up.

## Why This PR Matters

The main danger is not that awareness lacks features. The danger is that agents can still query older surfaces and receive recommendations without seeing the assurance verdict.

Today, `meta-check` and `semanticdiff` understand trust. But several agent-facing tools still emit ad-hoc output:

- `awareness_preflight`
- `awareness_decision_context`
- `awareness_impact_file`
- `awareness_match_incident_patterns`
- `awareness_pre_edit_context`

That means an LLM can still ask the wrong surface and miss freshness, coverage, or `NO_MATCH_UNSAFE` semantics.

This PR should make trust unavoidable.

---

# PR Goal

Wire the `assurance.TrustEnvelope` into preflight and MCP outputs, then prove the full path works with one black-box integration test against a real diff.

The end state:

```text
preflight -> coverage lookup -> freshness check -> assurance.Compose() -> TrustEnvelope -> CLI/MCP output
```

Every agent-facing awareness result must answer:

```text
Can this result be trusted?
Why?
What limits apply?
What action is required next?
```

---

# Scope A: Add Trust Envelope to Preflight

## Required Change

Add a trust field to `preflight.Report`:

```go
Trust *assurance.TrustEnvelope `json:"trust,omitempty"`
```

Populate it through `assurance.Compose()`.

Use the existing `report.GraphFreshness` and per-match coverage lookup as inputs.

## Required Semantics

The preflight report must never expose a safety-like recommendation without a trust envelope.

Rules:

```text
NO_MATCH                 -> verdict: unknown or unsafe, never trusted
stale graph/bundle       -> verdict: stale, never trusted
missing coverage         -> verdict: unknown or unsafe
partial coverage         -> verdict no stronger than limited
fresh + sufficient       -> usable
fresh + strong + tested  -> trusted
```

Use the existing assurance package semantics where available. Do not duplicate trust logic in preflight.

Preflight should compose trust from evidence. It should not become a second assurance engine.

## NO_MATCH Rule

A preflight result with no awareness match must carry a non-empty trust envelope.

Expected shape:

```yaml
trust:
  verdict: unknown
  confidence: none
  coverage: none
  freshness: fresh | stale | unknown
  limitations:
    - no trusted awareness coverage found
  required_action:
    - inspect manually
    - create provisional invariant, failure mode, or incident note if this is a new pattern
```

Exact strings may differ if the existing `TrustEnvelope` type uses enums. The behavior must not differ.

## Regression Test

Add or update a unit test proving:

```text
NO_MATCH preflight result always includes trust.verdict = unknown/unsafe and never trusted.
```

This test is load-bearing. It protects against the old rubber-stamp failure.

---

# Scope B: Wire Trust Envelope into MCP Tools

## Required MCP Surfaces

At minimum, wire trust into:

- `awareness_preflight`
- `awareness_decision_context`
- `awareness_match_incident_patterns`

Also inspect and wire if they are active agent-facing surfaces:

- `awareness_impact_file`
- `awareness_pre_edit_context`

## Output Contract

Every MCP result should surface the trust envelope in a stable machine-readable shape.

Do not bury trust inside prose.

Good:

```json
{
  "match_status": "NO_MATCH",
  "trust": {
    "verdict": "unknown",
    "confidence": "none",
    "freshness": "fresh",
    "coverage": "none",
    "limitations": ["no trusted awareness coverage found"],
    "required_action": ["inspect manually"]
  },
  "recommendations": []
}
```

Bad:

```text
No match found. You may proceed.
```

Bad:

```json
{
  "recommendation": "safe"
}
```

## Compatibility

Preserve existing fields where possible so current callers do not break.

Add `trust` as an additive field unless the old field actively misleads. If an old field says something like `safe: true` under `NO_MATCH`, remove or change it.

## Agent Safety Rule

Any MCP surface that gives guidance to an LLM must include one of:

- `trust`
- `assurance`
- `verdict` explicitly derived from `assurance.TrustEnvelope`

Prefer the canonical field name:

```json
"trust": { ... }
```

---

# Scope C: End-to-End Integration Test Against a Real Diff

## Required Test

Add one black-box integration test under:

```text
awareness/assurance/integration_test.go
```

or the closest existing integration-test location if package layout requires it.

## Test Purpose

Prove the joined system works:

```text
git diff -> preflight -> graph freshness -> coverage lookup -> assurance.Compose() -> trust envelope
```

This is specifically meant to catch bugs where every unit test passes but the composed behavior is wrong.

## Suggested Fixture

Build a small graph from a real `docs/awareness/` snapshot or a realistic fixture copied from it.

Simulate a git diff touching a high-risk file, for example one associated with:

- workflow resume poisoning
- deterministic retry storm
- repository desired/build drift
- node-agent installed-state commit
- objectstore topology authority

Pick the smallest fixture that exercises real wiring.

## Assertions

The test must assert:

1. Preflight returns a report.
2. Report contains a non-empty `TrustEnvelope`.
3. Freshness gates the verdict.
4. Coverage affects the verdict.
5. A high-risk matched failure mode cannot return `trusted` unless coverage is sufficient/strong and freshness is fresh.
6. A stale fixture cannot produce a trusted safety verdict.
7. A `NO_MATCH` diff returns `unknown` or `unsafe`, never `trusted`.

## Why This Test Exists

The previous prefix bug shipped because isolated unit tests passed while the composed path was wrong.

This test is the guard against that class of failure.

---

# Scope D: Intentional TESTED-Only Gaps

Claude found three remaining TESTED-only modes that appear to be intentional or transitional:

- `annotation_scanner_false_positive`
- `derived_state.projection_blocks_authority`
- `failure_mode.inc_2026_0002`

Do **not** silently let these drag down coverage if they are known intentional gaps.

## Required Action

For each one, decide one of:

### Option A: Mark intentional gap

Use this when the mode is real but does not currently need detector/doctor enforcement.

Suggested YAML shape:

```yaml
intentional_gap: true
intentional_gap_reason: "Awareness-internal false-positive class; regression test coverage is enough for now."
intentional_gap_review_after: "2026-06-10"
```

### Option B: Build missing detector/doctor rule

Use this only if the mode represents a runtime condition that should be detected operationally.

### Option C: Merge or deprecate

Use this if the failure mode is just a placeholder or duplicate.

Suggested YAML shape:

```yaml
deprecated: true
deprecated_reason: "Incident-named placeholder; merged into workflow.resume_poisoning."
replacement: "workflow.resume_poisoning"
```

## Recommendation

Prefer Option A or C in this PR. Avoid building new doctor rules unless the missing rule is trivial and clearly valuable.

This PR is about trust propagation, not expanding detector inventory.

---

# Scope E: Do Not Implement Detector Lifecycle Yet

## Defer #5

Do not implement full detector lifecycle yet:

```text
DETECTED vs ENFORCED based on last_observed_at / production firing recency
```

Reason: it is premature while detector population is still being stabilized manually.

A detector that has not fired recently is not necessarily weak if the failure has not occurred. Runtime-observed enforcement is valuable, but it should come after the static wiring is reliable.

## Allowed Preparatory Work

Only do this if it is tiny and does not affect verdicts yet:

```go
type DetectorEdgeMetadata struct {
    LastObservedAt *time.Time `json:"last_observed_at,omitempty"`
    ObservationSource string `json:"observation_source,omitempty"`
}
```

But do not change coverage classification behavior in this PR.

No new `ENFORCED` state is required now.

---

# Required Tests

Add tests for:

## 1. Preflight Trust Envelope

```text
NO_MATCH preflight result includes trust envelope.
NO_MATCH is unknown/unsafe, never trusted.
```

## 2. Stale Freshness Gate

```text
Stale graph/bundle prevents trusted verdict.
```

## 3. Coverage Gate

```text
Partial or missing coverage prevents trusted verdict.
```

## 4. MCP Serialization

```text
MCP tools include trust envelope in JSON result.
Existing callers still receive expected legacy fields unless those fields were unsafe.
```

## 5. End-to-End Real Diff

```text
Realistic diff exercises preflight -> coverage -> freshness -> assurance.Compose() -> envelope.
```

---

# CI Ratchet

Keep the existing ratchet behavior:

```text
--min-well-covered
--min-detected
```

Do not make the ratchet stricter in this PR unless the new extractor wiring increases coverage naturally.

Add one additional CI check only if low-risk:

```text
agent-facing MCP/preflight outputs must include trust envelope
```

This can be a unit test rather than a new CLI flag.

---

# Acceptance Criteria

This PR is complete when:

1. `preflight.Report` includes `Trust *assurance.TrustEnvelope`.
2. `awareness_preflight` MCP output includes trust.
3. `awareness_decision_context` MCP output includes trust or delegates to preflight output that includes trust.
4. `awareness_match_incident_patterns` MCP output includes trust when it makes any recommendation or confidence claim.
5. `NO_MATCH` never returns an empty trust envelope.
6. `NO_MATCH` never returns `trusted`.
7. Stale freshness prevents `trusted` verdicts.
8. Coverage level affects verdict strength.
9. One black-box integration test proves the composed path against a realistic diff.
10. The three TESTED-only gaps are either annotated as intentional, deprecated/merged, or explicitly left as follow-up with a TODO and reason.

---

# Implementation Notes

## Prefer Central Composition

Use:

```go
assurance.Compose(...)
```

Do not scatter trust decision logic across MCP handlers.

The desired shape is:

```text
collect evidence -> call assurance.Compose() -> attach envelope -> render output
```

## Avoid False Precision

If coverage lookup is uncertain, say so.

Do not manufacture confidence.

```text
unknown is better than fake trusted
```

## Preserve Humility

The assurance layer exists to prevent awareness from pretending to know more than it knows.

Every output should respect this principle:

```text
NO_MATCH means unknown coverage, not safety.
```

---

# Suggested Commit Message

```text
awareness: propagate trust envelope through preflight and MCP

Wire assurance.TrustEnvelope into preflight reports and agent-facing MCP
surfaces so freshness and coverage gates are visible outside meta-check.
Add an end-to-end integration test covering diff -> preflight -> coverage ->
freshness -> assurance composition, including NO_MATCH and stale-bundle
cases.

This prevents agents from treating awareness silence as approval.
```

---

# Follow-Up PRs

After this PR, revisit:

## Follow-up 1: Detector lifecycle

Add `last_observed_at` and distinguish:

```text
WIRED detector: exists in graph/YAML
ACTIVE detector: has fired recently or produced runtime evidence
ENFORCED failure mode: detected + tested + observed operationally
```

## Follow-up 2: Semantic-diff authority budget

Complete Section 6 from the original assurance plan:

```text
Repository -> Desired -> Installed -> Runtime authority movement requires stronger trust coverage.
```

## Follow-up 3: Dashboard / report

Expose coverage states:

```text
ORPHAN
PARTIAL
TESTED
DETECTED
WELL_COVERED
INTENTIONAL_GAP
DEPRECATED
```

The goal is to make coverage debt visible and prevent future silent regression.

---

# Final Instruction

Implement #3 and #4 now.

Treat #5 as deferred.

Treat #6 as a small honesty cleanup if safe.

The purpose of this PR is simple:

```text
No agent-facing awareness surface may give guidance without showing whether that guidance deserves trust.
```

---

# Claude Addendum — Lessons from the Coverage/Detector PR (2026-05-10)

The previous PR (detector extractor + CI ratchet) finished with the live cluster
showing well_covered=10, detected+enforced=33, untracked_yamls=30. Several
observations from that work apply directly to this PR; capture them here so we
don't relearn them.

## A1. The Prefix-Bug Class — encode it as a global rule, not a local fix

The orphan crisis was caused by ONE join across two layers using different
identifier conventions:

```text
failure_modes table:  id = "etcd.leader_instability"           (un-prefixed)
graph nodes:          id = "failure_mode:etcd.leader_instability" (prefixed)
edges:                Dst = "failure_mode:etcd.leader_instability"
coverage lookup:      bucket[fm.ID]   →  always missed
```

This pattern will recur. Every extractor stamps a "namespace:" prefix, but
some database tables store the domain id without a prefix. Whenever assurance
or any consumer joins the two, the prefix must be applied or stripped
explicitly.

**Rule for this PR**: every new lookup that joins the failure_modes table to
graph edges MUST go through a single helper that knows the convention. Suggested:

```go
// In golang/awareness/assurance/ (or graph/, take pick).
const FailureModeNodePrefix = "failure_mode:"
func FailureModeNodeID(id string) string { return FailureModeNodePrefix + id }
func FailureModeIDFromNode(nodeID string) string {
    return strings.TrimPrefix(nodeID, FailureModeNodePrefix)
}
```

`assurance.ComputeCoverage` already uses this internally; expose it so MCP
handlers and preflight can use the same primitive. This prevents new code
from re-creating the bug.

## A2. Per-Failure_Mode Coverage Lookup — needs an exposed API

This PR's preflight wiring needs to know the coverage tuple for the matched
failure_mode(s). Today `assurance.ComputeCoverage` returns the full report; it
does not expose a per-id query. Add:

```go
// CoverageFor returns the coverage tuple for a single failure_mode, or nil
// if the mode is not in the graph. Cheap when called repeatedly because it
// reuses an in-memory bucket; callers should reuse the report.
func (r *CoverageReport) CoverageFor(fmID string) *FailureModeCoverage
```

Implement as a one-time map build on first access (lazy). Without this, every
preflight call risks O(N) scans.

## A3. Detector Mapping Pattern — formalise as a Reusable Idiom

`docs/awareness/detector_mapping.yaml` worked very well: a separate YAML join
table that connects two extractor outputs without coupling them at compile time.
Document it as the canonical pattern for any future cross-subsystem join:

```text
extractor A → graph nodes (typed: A)
extractor B → graph nodes (typed: B)
mapping.yaml → typed edges (A → relationship → B)
mapping_extractor → reads mapping.yaml + emits edges
```

The frontend awareness doc should reuse this idiom for component → capability,
sdk_function → backend_service, and similar joins. See A6 in the frontend doc.

## A4. The Untracked_YAMLs Bottleneck — needs a policy decision

Right now the trust envelope returns `freshness=stale_unknown` whenever
untracked_yaml_count > 0. The live cluster has 30 such files (incidents/,
proposals/, knowledge/*, failuregraph_seeds/). That means even with a
freshly-built graph, the envelope verdict is capped at "stale".

Three honest options:

1. **Move all known YAMLs into one of two buckets**: graph-contributing (added
   to `KnowledgeFiles()`) or explicitly-not-graph-contributing (allowlist in
   assurance). The "untracked_unknown" middle category goes away.
2. **Relax the rule**: untracked_yaml_count > 0 produces an info alarm, not a
   freshness downgrade. The downgrade only fires on yaml_newer_than_graph or
   bundle_age_exceeded.
3. **Per-file decision**: each YAML declares `awareness_role: graph | config |
   none` in its top-level metadata, and the freshness check only counts
   role=graph files.

Recommendation for this PR: option 2 (relax). It removes the false signal that's
currently swallowing the verdict for an entire fresh build. Option 3 is the
right long-term answer but is bigger than this PR.

## A5. Backwards-Compat for Preflight.Report

`preflight.Report` already has `Coverage`, `Confidence`, `ConfidenceFactors`,
`SafetyStatus`, and `RiskTier` fields. Adding `Trust *TrustEnvelope` should
NOT remove these — many callers (test scripts, doc.go, doctor invariants)
read them. The new field is additive.

But it MUST be derived from the same evidence the existing fields use. If
`Trust.Verdict = unknown` while `SafetyStatus = PROCEED`, the report is lying
to itself. The simplest discipline:

```go
// At the end of preflight.Run(), AFTER all existing fields are computed:
report.Trust = assurance.Compose(assurance.ComposeInputs{
    MatchFound: ...,
    PerFailureMode: ...,
    Staleness: ...,
})
// Then RECONCILE existing fields if envelope contradicts them:
if report.Trust.Verdict == TrustUnknown || report.Trust.Verdict == TrustUnsafe {
    report.SafetyStatus = SafetyStatusUnknownNotSafe
}
```

## A6. The Trust Envelope Should Be Most Prominent When Not "Trusted"

Risk: every MCP response includes a 6-field trust block, agents start
ignoring it. Counter-measure: serialisation should foreground the envelope
when `verdict != trusted`. Suggested rendering rule:

```yaml
trust:
  verdict: trusted               # when trusted: collapse limitations/required_action arrays
  confidence: high
  freshness: fresh
  coverage: strong

trust:
  verdict: stale                  # when NOT trusted: explicit reason and required_action up front
  confidence: low
  freshness: stale_unknown
  coverage: sufficient
  reason: 30 untracked YAMLs cap freshness; rebuild bundle
  required_action:
    - rebuild awareness bundle
    - rerun preflight
```

## A7. Closure Loop Should Surface in Trust Envelope

`TestLearningLoopClosesOnReoccurrence` proves the loop closes. But the trust
envelope today says nothing about it — agents see "trusted" without knowing
that "trusted" is partly because this exact pattern was learned from a real
incident. Add an optional field:

```go
type TrustEnvelope struct {
    ...
    LearnedFromIncident string `json:"learned_from_incident,omitempty"`
    RegressionTest     string  `json:"regression_test,omitempty"`
}
```

Populate when `assurance.Compose()` sees a failure_mode whose coverage tuple
includes a learning entry. Optional in this PR but cheap to add and explains
WHY the verdict is trusted.

## A8. Live Numbers as the Ratchet Floor

When this PR ships, set CI ratchets to today's measured values:

```yaml
--min-well-covered: 10
--min-detected: 33
--baseline-orphans: 0
```

These prevent silent regression. Don't move the floor up in this PR — that
belongs to a follow-up that adds new mappings.

## A9. Test Naming Convention

The previous PR's regression test naming worked well. Continue:

```text
TestComputeCoverage_RecognisesExtractorWiring  → guards a specific bug class
TestExtract_MappingEmitsMatchesFailureModeEdge → guards the mapping flow
TestLearningLoopClosesOnReoccurrence           → guards loop integrity
TestCompose_NoMatchIsNeverSafe                 → guards the rubber-stamp rule
```

Each test name encodes WHAT BREAKS if it fails. Future tests for this PR
should follow the same convention:

```text
TestPreflightReport_NoMatchAlwaysCarriesTrustEnvelope
TestMCPPreflight_StaleBundleNeverEmitsTrustedVerdict
TestE2E_HighRiskDiff_TrustEnvelopeReflectsCoverage
```
