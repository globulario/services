# Awareness — Open Requirements (drafted by Claude, 2026-05-10)

This document is the consolidated backlog of everything I (Claude) consider
worth doing in the awareness system, collected from the May 8–10 work
sessions. It is **not** the next-PR instructions (those live in
`claude_awareness_next_pr_instructions.md`). It is the longer list that
should be revisited on 2026-05-13 when token budget is restored.

---

## Stabilization Plan — Authoritative Direction (2026-05-10, late)

**Do NOT start frontend awareness yet.** Frontend awareness should inherit a
mature backend pattern, not an experimental one. This formalises the F-A10
recommendation in `claude_frontend_awareness_graph_instructions.md` and
supersedes any conflicting guidance below.

### Phase 1: Stabilize Backend Awareness

For the next backend work cycles, awareness must be used as part of normal
development. The goal is not expansion — it is operational discipline.

Claude (when working on backend code) should:

1. Run `awareness preflight` before risky backend edits.
2. Inspect the trust envelope before trusting any recommendation.
3. Treat `NO_MATCH` as unknown, not safe.
4. Fix missing graph edges when discovered.
5. Promote repeated failures into failure_modes, invariants, forbidden_fixes,
   detector mappings, and regression tests.
6. Keep coverage ratchets green.
7. Consolidate duplicated trust/freshness/coverage logic when found.
8. Document any missing pattern that appears more than once.
9. Improve awareness only when real use reveals a gap.
10. Avoid adding new awareness subsystems unless the existing trust path
    requires it.

### Immediate Backend Awareness Priorities (gates before frontend)

Both must complete before any frontend-awareness implementation:

- **P0-1**: Trust envelope into preflight and MCP
- **P0-2**: End-to-end integration test against a real diff

The full P0-1 / P0-2 specifications remain in
`claude_awareness_next_pr_instructions.md`.

### Note on the Original Strategy Document

The strategic directive that authoritative-ised this plan was sent as
`claude_backend_stabilization_before_frontend.md` (concept) but the message
was truncated mid-sentence after the P0-2 introduction. The full P0-2
specification (the `git diff -> preflight -> ... -> TrustEnvelope` test
flow) lives in `claude_awareness_next_pr_instructions.md` Scope C, which
is the canonical source. If a separate stabilization document is wanted as
a standalone artifact for 2026-05-13, draft it then with the full text.

---

Each item has:

- a one-line summary
- priority (P0 = before next release, P1 = this quarter, P2 = next quarter)
- effort estimate
- the failure mode it prevents
- a concrete first step

---

## P0 — Required Before Next Awareness Release

### P0-1. Wire trust envelope into preflight + MCP

**Status**: In progress. See `claude_awareness_next_pr_instructions.md`.
**Effort**: 2–3 hours core + tests.
**Prevents**: Agents reading awareness output without seeing freshness/coverage gates.
**First step**: Add `Trust *assurance.TrustEnvelope` to `preflight.Report` and populate via `assurance.Compose()`.

### P0-2. End-to-end integration test against a real diff

**Status**: In progress. See `claude_awareness_next_pr_instructions.md`.
**Effort**: ~3 hours.
**Prevents**: Cross-subsystem composition bugs (the prefix-bug class).
**First step**: `awareness/assurance/integration_test.go` with a fixture diff against a high-risk file.

### P0-3. Untracked-YAMLs should not silently cap trust at "stale"

**Status**: ✅ DONE 2026-05-10. The fix distinguishes three classifications:
- `graph` role: top-level key is in dispatchTable OR `externallyHandledGraphKeys` (failuregraph seeds, detector_mapping, contracts). Edits affect freshness.
- `config` role: top-level key in `configOnlyKeys`. Info-level alarm only, does NOT cap trust.
- `unknown` role: top-level key in neither. Warn-level alarm, caps trust at `stale_unknown`.

Live cluster result: verdict went from `stale` (false alarm on 30 indiscriminate files) to `usable` (accurate — fresh graph, sufficient coverage). 0 unknown-role files; 11 reclassified as graph (failuregraph seeds, contracts, detector_mapping); 19 confirmed config-only.

Tests: `TestCompose_UnknownRoleYAMLsCapTrust`, `TestCheckStaleness_AllConfigOnly_NoWarn`, updated `TestCheckStaleness_UntrackedYAMLDetected`.

### P0-4. Helpers for failure_mode node id conversion

**Status**: Not started.
**Effort**: 20 min.
**Prevents**: Recurrence of the prefix bug.
**First step**: Export `assurance.FailureModeNodeID(id) string` and `FailureModeIDFromNode(id) string`. Replace ad-hoc string concatenation across coverage.go, freshness.go, doctor extractor.

### P0-5. Per-failure_mode coverage lookup API

**Status**: Not started.
**Effort**: 30 min.
**Prevents**: O(N) scans every time preflight needs one mode's coverage.
**First step**: Add `(r *CoverageReport) CoverageFor(fmID string) *FailureModeCoverage`, lazy-initialised map.

---

## P1 — This Quarter

### P1-1. Detector lifecycle (active vs wired)

**Status**: Deferred per next-PR doc Scope E.
**Effort**: ~half a day.
**Prevents**: Modes counted as `DETECTED` because a detector edge exists in YAML, even if the detector has never fired. Today this overstates real coverage.
**First step**: Add `last_observed_at` and `observation_source` fields to detector edge metadata. Initially populated by cluster-doctor when a finding fires for a mapped failure_mode. Coverage classifier flips to `ENFORCED` only when last_observed_at is within e.g. 30 days.

### P1-2. Build the 3 missing TESTED-only modes' detectors (or annotate intentional)

**Status**: Surfaced this session. See Scope D in next-PR doc.
**Effort**: 1 hour for annotations (Option A), ~1 day per real new doctor rule (Option B).
**Modes**:
- `annotation_scanner_false_positive` — recommend Option A (intentional, awareness-internal).
- `derived_state.projection_blocks_authority` — recommend Option B if real, else Option C (deprecate).
- `failure_mode.inc_2026_0002` — recommend Option C (rename or merge).
**First step**: Audit each mode for whether it represents a runtime-observable failure or a documentation placeholder.

### P1-3. Untracked YAMLs — explicit role classification

**Status**: Not started. Counterpart to P0-3.
**Effort**: 1 hour.
**Prevents**: Long-term confusion about which YAML files contribute to the graph and which are configuration.
**First step**: Each YAML under `docs/awareness/` declares one of `awareness_role: graph | config | seed | none` in its top-level metadata. The freshness extractor reads the role and only counts `graph` and `seed` files toward staleness. Document the rule in CLAUDE.md.

### P1-4. Closure loop visible in trust envelope

**Status**: Not started.
**Effort**: 1 hour.
**Prevents**: Trust verdict says "trusted" without telling the agent WHY. Optional `LearnedFromIncident` and `RegressionTest` fields on `TrustEnvelope` make the basis of trust explicit.
**First step**: Extend `assurance.TrustEnvelope` with optional `LearnedFromIncident string` and `RegressionTest string`. Populate when `Compose()` sees a learning entry on the matched failure_mode.

### P1-5. Awareness bundle build includes detector_mapping.yaml

**Status**: Not verified.
**Effort**: Probably already works (the bundle copies docs/awareness/* by glob), but verify.
**Prevents**: Bundles shipped to production missing the new mapping file, leading to silent loss of detector→failure_mode edges on consumers.
**First step**: Inspect the bundle build pipeline; add `detector_mapping.yaml` to the manifest test if not present.

### P1-6. Semantic-diff authority budget — confirm full integration

**Status**: Largely done per session-15 verification, but not exercised in CI against real diffs.
**Effort**: 1 day.
**Prevents**: Authority-moving patches (Repository → Desired → Installed → Runtime) merging without strong coverage.
**First step**: Add a CI step that runs `awareness semantic-diff` on every PR's git diff and fails if `AuthorityChange.RequiresReview` is true and `TrustEnvelope.Verdict` is not `trusted`.

---

## P2 — Next Quarter or Later

### P2-1. Frontend awareness graph (Phase F1+)

**Status**: Designed in `claude_frontend_awareness_graph_instructions.md`. Not started.
**Effort**: ~10k LOC across F1–F5, multi-PR.
**Blocking dependency**: Wait until trust envelope wiring (P0-1, P0-2) is shipped and observed working for one cycle. See addendum F-A10.
**First step**: When ready, follow the F1 plan (TypeScript module extractor + customElements registry).

### P2-2. Awareness dashboard / report

**Status**: Mentioned as Follow-up 3 in next-PR doc.
**Effort**: 2 days.
**Prevents**: Coverage debt being invisible to operators (visible only in CLI output).
**First step**: A static HTML report generated from `awareness meta-check --json` and committed to docs/awareness/coverage-report.html via CI.

### P2-3. Auto-fail PRs that increase orphan_count or decrease well_covered_count

**Status**: Flags exist (`--min-well-covered`, `--min-detected`, `--baseline-orphans`), but no CI hook calls them yet.
**Effort**: 30 min CI config.
**Prevents**: Silent coverage regression.
**First step**: Add `awareness meta-check --baseline-orphans 0 --min-well-covered 10 --min-detected 33` to the pre-merge CI workflow.

### P2-4. Awareness self-healing

**Status**: Speculative.
**Effort**: Unknown.
**Idea**: When the assurance layer detects a class of bug (e.g. a prefix mismatch in coverage), it should be able to suggest a fix and run a regression test. The closure loop currently does this for incidents; the meta-layer should do it for assurance code itself. Probably out of scope until the basics are stable.

### P2-5. Per-edge confidence decay

**Status**: Not started.
**Idea**: Edges in the graph have a confidence score (0–1). Today that score is set at extraction time and never decays. An edge from a YAML file last touched 6 months ago is implicitly less trusted than one from yesterday. Adding decay would let the trust envelope reflect "this knowledge is old" without requiring explicit staleness rules.
**Risk**: Premature optimisation; only worth doing if trust envelope users actually want this fidelity.

---

## Open Architectural Questions

These don't have a clear answer yet and should be discussed before being scheduled:

### Q1. Should backend and frontend awareness share one graph DB or two?

**Pro one DB**: cross-layer queries work natively (component → SDK → backend service → invariant).
**Pro two DBs**: backend is stable, frontend churns; mixed staleness rules; bundle size considerations.
**Lean**: one DB with a `layer: backend|frontend` tag on every node. Single bundle, single freshness rule, but per-layer ratchets.

### Q2. Should `detector_mapping.yaml` move into `failure_modes.yaml`?

**Pro merge**: single file per failure_mode is more readable.
**Pro keep separate**: detector mapping changes more often than failure_mode definitions; merge would cause noisy diffs.
**Lean**: keep separate for now; revisit after one quarter of mapping churn.

### Q3. Should the trust envelope verdict block merges?

Today the assurance layer reports trust but doesn't gate. If a PR's preflight returns `verdict: stale`, should CI block it?

**Pro**: Forces operators to refresh awareness before merging.
**Con**: False positives (a valid PR that doesn't touch awareness inputs could still see stale_unknown if YAML is untracked).
**Lean**: Block on `verdict: unsafe` and `verdict: unknown` for high-risk-file PRs only. Allow `verdict: stale` with a warning. Requires P0-3 to land first.

### Q4. Should assurance be a separate service or stay a library?

Today `assurance` is a Go package linked into the CLI. If it becomes a service, MCP tools query it remotely; if it stays a library, every consumer rebuilds from source.

**Lean**: Keep as a library. Adding a service adds an availability dependency that contradicts the "fail safe" rule (cluster must keep working if AI services are down).

---

## Things I Considered and Rejected

For the record, things I evaluated this session and decided NOT to add:

### Rejected: Auto-promoting orphan failure_modes to deprecated

A detector that hasn't fired in 6 months might be defunct. But silently
deprecating a failure_mode is the opposite of the assurance discipline —
the operator should make that decision explicitly. The `intentional_gap`
annotation is the right knob.

### Rejected: ML-based mapping suggestions

Using an LLM to suggest detector→failure_mode mappings is tempting but the
mapping reasoning ("this rule detects this failure mode because...") is
exactly the human knowledge that needs to be reviewable. A `reason:` field
in detector_mapping.yaml captures it; ML output would be opaque.

### Rejected: A second graph DB for hot evidence

Splitting "static" (extractor-built) from "live" (runtime-observed) into
separate DBs sounds clean but doubles the freshness logic and creates a
new join risk. Better to keep one DB with a `source_type` field and let
queries filter.

### Rejected: Awareness-as-a-PR-bot

A bot that reads PR diffs and posts trust envelopes as comments would be
useful, but: (a) it requires a hosted service we don't currently run; (b) it
duplicates what `awareness semantic-diff` already produces locally. Defer
until after CI integration is solid.

---

## Metrics To Track Over Time

For visibility into whether awareness is improving or regressing:

```text
awareness_failure_modes_total
awareness_well_covered_count
awareness_well_covered_percent
awareness_detected_or_enforced_count
awareness_orphan_count
awareness_intentional_gap_count
awareness_untracked_yaml_count
awareness_graph_age_seconds
awareness_bundle_age_seconds
awareness_trust_envelope_unsafe_count_per_session
awareness_trust_envelope_unknown_count_per_session
awareness_trust_envelope_stale_count_per_session
```

Most of these are already in `meta-check --json`. Wire them into the metrics
layer (Prometheus) so trends are visible. Suggested target dashboard:

- well_covered_percent should be a non-decreasing line.
- orphan_count must stay 0.
- trust_envelope_unknown_count_per_session should trend down as agents query
  surfaces that have envelopes wired.

---

## Final Note from Claude

This list is honest about what I don't know and what I think is worth
doing. It is also explicitly conservative about adding more awareness
subsystems before the existing trust path is fully wired. The single
biggest risk to the project right now is shipping more features that need
trust calibration faster than the calibration itself can be propagated to
agent-facing surfaces.

If only one thing happens after 2026-05-13, it should be P0-1 + P0-2
(trust envelope into preflight + MCP, plus the integration test).

If three things happen, add P0-3 + P0-4 (untracked-YAML rule fix + id helpers).

Everything else should follow.
