# Composed-Path Failures

A design-signal log. Every entry records a real incident where each
subsystem's unit tests passed but the composed (end-to-end) path produced
wrong behavior.

The log exists because the danger now is not lack of features. The danger
is drift: subsystems slowly inventing their own language, their own
prefixes, their own freshness rules, their own confidence meanings, until
the system looks powerful but becomes hard to trust.

Every entry is high-value evidence. The job is not to patch — it is to
classify, so the system stops repeating the same shape of mistake.

## The rule

Any fix that touches one of the following subjects MUST first read this log:

- graph identity (node ids, prefixes, edge endpoints)
- lifecycle metadata (`deprecated`, `intentional_gap`, `coverage_state`)
- freshness (graph staleness, bundle age, manifest source)
- coverage (mitigation/test/detector legs, classification rules)
- trust semantics (verdict, confidence, freshness, coverage axes)

These five subjects are the **tripwires**. Any change that touches one
of them is composed-path-relevant by definition.

For each fix in those areas, ask:

1. **Is the bug a repeat of an existing entry's consolidation candidate?**
   If yes, consolidate — promote the candidate from "owed" to a real shared
   primitive. The patch becomes the consolidation, not another local
   workaround.
2. **Is it a new shape?** If yes, record it using the schema below before
   shipping the fix. The log entry is part of the change, not after it.
3. **Is the fix expansion?** If you find yourself adding a new subsystem,
   verb, or vocabulary to handle the case, stop. Re-read the existing
   entries — the answer is almost always to consolidate something that's
   already in the log.

No expansion unless repetition earns it. The log is where repetition is
recorded, so it's the only honest place to make that call.

## How to choose the next stabilization work

Do not pick by taste. Pick from evidence. In priority order:

1. **Repeats of an existing consolidation candidate** in this log.
   Two incidents on the same candidate is the trigger to consolidate;
   the next stabilization pass should make the candidate the work,
   not another patch.
2. **Anything that touches one of the five tripwire subjects** above.
   These are the axes where composed-path failures keep landing, so
   they earn priority by structural position even before a second
   incident appears.
3. **Anything that changes the live verdict path from `git diff` to
   the TrustEnvelope.** That path is the user-facing result of the
   joined system; bugs there are felt immediately and dishonestly.

Everything else waits. Do not invent work outside this priority order
because it sounds reasonable. The point of the discipline is to let
evidence — not intuition — drive what changes next.

Run `globular awareness prefix-audit` for measurement on the prefix
candidate. Re-read this log for the rest. If neither produces a
candidate, the right move is to stop and run the system as-is for a
cycle: stabilization includes restraint.

## Required schema

For each entry, record exactly these five fields. Do not skip any.

1. **Shared concept that fragmented** — the one idea that several
   subsystems all needed but expressed differently.
2. **Per-subsystem interpretation** — name each subsystem and what
   meaning *it* assigned to the concept.
3. **Why unit tests missed it** — the structural reason, not "we forgot."
   Almost always: each subsystem tested its own interpretation; nothing
   tested the join.
4. **End-to-end contract that should own it** — the single authoritative
   definition the joined path can rely on. If the fix didn't establish
   one, say so.
5. **Did the fix simplify the system or add special cases?** — honest
   answer. "Simplified" means one shared definition replaced N parallel
   ones. "Special-cased" means we patched the specific site without
   reducing the surface area for the next bug. "Partial" is allowed and
   is itself a signal: the simplification is owed.

When a consolidation candidate appears in two entries, that's the trigger
to do the consolidation work — not the day a single bug surfaces. The
log is where we earn the right to consolidate.

---

## 2026-05-08 — Graph node id prefix

- **Shared concept**: failure_mode node ids carry a `failure_mode:` prefix
  in the graph; the same id appears un-prefixed in the failure_modes table.
- **Per-subsystem interpretation**:
  - `manual` extractor: writes prefixed node ids (correct).
  - `assurance/coverage`: keyed its bucket by un-prefixed `fm.ID`,
    expecting edges to land on the un-prefixed key.
  - `incidentpattern`, `debugsession`: mixed — some sites used prefixed,
    some un-prefixed.
- **Why unit tests missed it**: each subsystem's tests seeded the graph
  using the same convention as the code under test. coverage_test.go
  used un-prefixed ids for both inserts and lookups, so the bucket
  matched. Nothing tested edges from one extractor against lookups
  from another.
- **End-to-end contract that should own it**: a `graph.FailureModeID(id)
  string` (or typed `graph.NodeID`) constructor used everywhere instead
  of string concatenation. **Not yet established.**
- **Did the fix simplify or special-case?** **Special-cased.** Added a
  `failureModeNodePrefix` constant in coverage.go and re-keyed the
  bucket. Other subsystems still hand-roll the prefix. The fix unblocked
  the immediate symptom but created no shared definition; the next
  subsystem that hand-rolls the prefix will hit the same bug.

---

## 2026-05-10 — Bundle freshness contract

- **Shared concept**: which manifest does the freshness check use, and
  who supplies it?
- **Per-subsystem interpretation**:
  - `bundlesync`: "I load a Manifest by an explicit path."
  - `assurance.CheckStaleness`: "I read the manifest from `opts.Manifest`."
  - `preflight.computeTrustEnvelope`: "I pass nothing — the layer below
    must already know."
  - CLI / MCP: "preflight handles freshness end-to-end, I'm a thin caller."
- **Why unit tests missed it**: each subsystem tested its slice with a
  manifest the test supplied directly. No test asserted that the
  composed pipeline could resolve a real installed bundle from disk
  without the caller wiring it. Freshness was implicit in every test.
- **End-to-end contract that should own it**: **Established.**
  `bundlesync.DefaultManifestPath()` returns the canonical install
  location. preflight falls back to it when callers don't override.
  One contract, one path, every consumer reads the same thing.
- **Did the fix simplify or special-case?** **Simplified.** Five layers
  with four conflicting assumptions collapsed to one shared contract.
  Adding the `BundleManifestPath` option is a small surface increase,
  but it replaces a void where every layer was free to invent its own
  answer.

---

## 2026-05-10 — Failure_mode lifecycle metadata loss

- **Shared concept**: a failure_mode's lifecycle flags
  (`deprecated`, `intentional_gap`, `coverage_state`) live in the
  graph node's `metadata_json` and must survive every loader pass.
- **Per-subsystem interpretation**:
  - `manual/failure_modes` loader: "I write full metadata (lifecycle
    flags + severity)."
  - `manual/design_patterns` loader: "I stub a failure_mode node so my
    edge has a target. Metadata? I don't have any, I'll write empty."
  - `manual/invariants` loader: same as design_patterns — stub with
    no metadata.
  - `graph.AddNode`: "I am upsert-with-clobber; whoever writes last
    wins on every column including metadata_json."
- **Why unit tests missed it**: each loader's unit test ran in isolation
  on a fresh graph. The bug only manifests when one loader runs *after*
  another against the same node id. No test composed two loaders into a
  single seed and asserted the metadata survived.
- **End-to-end contract that should own it**: **Established
  2026-05-10**: `graph.EnsureNode(ctx, n)` — INSERT-OR-IGNORE
  semantics, distinct from `AddNode`'s full upsert. The two functions
  are intentionally asymmetric so call sites express intent: "I own
  this node's content" → AddNode; "I just need this id present" →
  EnsureNode.
- **Did the fix simplify or special-case?** **Simplified (after two
  passes).** The first patch was inline `if FindNode == nil` checks
  at every stub site — a special-case that future contributors could
  quietly miss. Promoted to `graph.EnsureNode` and retired the inline
  pattern. Initially migrated only the failure_mode stubs; second pass
  (2026-05-10 P0-2) migrated every remaining stub-creator the audit
  surfaced: `extractors/docs/extract.go::findOrSynthesize`,
  `extractors/manual/patterns.go` (invariant stubs),
  `extractors/manual/design_patterns.go` (invariant + forbidden_fix +
  test + source_file stubs). Three primitive-level pinning tests
  (`awareness/graph/ensure_node_test.go`) lock EnsureNode's contract;
  four loader-level tests
  (`awareness/extractors/manual/stub_preservation_test.go`) lock
  metadata preservation across every migrated path. The migration
  is now complete; AddNode is reserved for canonical writers.

---

## 2026-05-10 — TrustEnvelope match-kind conflation

- **Shared concept**: what kind of awareness object did the query
  actually match — a failure_mode, an invariant, a forbidden_fix, or
  raw YAML knowledge?
- **Per-subsystem interpretation**:
  - `preflight.computeTrustEnvelope`: "MatchFound = OR over four kinds;
    PerFailureMode is set only if FailureModes matched."
  - `assurance.Compose`: "MatchFound is just a boolean; coverage axis
    is computed from PerFailureMode alone."
  - `coverageFromFailureMode`: "no PerFailureMode → TrustCoverageNone."
  - `decideVerdict` TrustCoverageNone branch: "match found + coverage
    none → reason: 'matched a failure_mode with no enforcing
    mitigation'."
  - Result: an invariant-only or raw-YAML-only match produces a verdict
    that lies about a failure_mode that wasn't even queried.
- **Why unit tests missed it**: Compose's tests used PerFailureMode
  fixtures directly and asserted on the orphan-failure-mode verdict.
  Nothing tested the "match found, no PerFailureMode" path against
  reason-text honesty. The verdict was correct enough (Unsafe), but
  the reason was a false flag.
- **End-to-end contract that should own it**: **Established
  2026-05-10**: `ComposeInputs.PrimaryMatchKind` carries the kind
  (failure_mode | invariant | forbidden_fix | raw_yaml | "").
  `coverageFromInputs` routes between FM-coverage logic and a
  partial-coverage fallback for non-FM matches. `decideVerdict`
  TrustCoverageNone branch chooses reason text by match kind, so the
  envelope never claims FM-related guidance for a non-FM match.
  `preflight.computeTrustEnvelope` derives PrimaryMatchKind from the
  most-actionable matched layer.
- **Did the fix simplify or special-case?** **Simplified.** One new
  string field on ComposeInputs replaces an implicit single-vocabulary
  assumption. Three new tests
  (`TestCompose_InvariantMatchDoesNotLieAboutFailureMode`,
  `TestCompose_RawYAMLMatchHonestReason`,
  `TestCompose_FailureModeMatchKeepsExistingBehavior`) lock the
  asymmetry. No new TrustVerdict, no new Coverage axis — just an
  honest reason that adapts to what was actually matched.

---

## 2026-05-10 — edge provenance home (column vs metadata)

- **Shared concept**: where does edge provenance live — the
  `edges.provenance_json` column or the `edges.metadata_json` column
  under a `"provenance_json"` key?
- **Per-subsystem interpretation**:
  - schema (migration `ALTER TABLE edges ADD COLUMN provenance_json`):
    "provenance is its own column."
  - `AddEdgeWithProvenance`: "provenance lives in
    `Metadata['provenance_json']` as a JSON-encoded string."
  - integrity `checkEdgeProvenance`: "provenance is in
    `e.Metadata['provenance_json']`."
  - all `SELECT ... FROM edges` queries: "we don't read provenance_json
    at all" — column was set to DEFAULT '{}' and never written.
- **Why unit tests missed it**: tests for AddEdgeWithProvenance asserted
  that the value round-trips through the metadata key, which it did.
  Nothing tested that a downstream reader of the column saw provenance,
  because no reader of the column existed.
- **End-to-end contract that should own it**: **Established
  2026-05-10**: `edges.provenance_json` column is canonical.
  `Edge.Provenance map[string]any` is the in-memory representation
  (distinct from `Edge.Metadata`). `AddEdge` writes the column when
  `e.Provenance` is supplied; `AddEdgeWithProvenance` populates
  `e.Provenance` from its typed fields and lets `AddEdge` persist.
  All `SELECT FROM edges` sites now include `provenance_json`;
  `scanEdges` populates `Edge.Provenance`. Integrity check reads
  `e.Provenance` instead of `e.Metadata['provenance_json']`. The
  metadata mirror is gone.
- **Did the fix simplify or special-case?** **Simplified.** Two homes
  collapsed to one column with one Go field. The asymmetry between
  `AddEdge` (full upsert) and the no-provenance UPSERT path is
  intentional and pinned by `TestAddEdge_NoProvenance_DoesNotClobber`,
  mirroring the same asymmetry that `EnsureNode` introduced for nodes.
  Round-trip pinning lives in
  `awareness/graph/edge_provenance_test.go`.

---

## 2026-05-10 — freshness clocks fragmented across legs

- **Shared concept**: a single "now" clock that governs every freshness
  comparison in the joined pipeline.
- **Per-subsystem interpretation**:
  - `assurance.CheckStaleness`: "I expose Options.Now for tests."
  - `graph.Freshness`: "I call time.Since(builtAt) — wall-clock only."
  - Test authors: "I'm passing Options.Now, freshness should be
    deterministic." (it wasn't, because the graph leg ignored it.)
- **Why unit tests missed it**: each leg's tests asserted on its own
  output. Nothing tested that the two legs agreed on what "now" was.
  Tests with injected clocks happened to land within the wall clock's
  drift tolerance, so flakiness was rare and silent.
- **End-to-end contract that should own it**: **Established
  2026-05-10**: `graph.FreshnessAt(ctx, docsDir, now)` is the
  clock-injectable form. `graph.Freshness` is a thin wrapper that
  resolves now to `time.Now()`. `assurance.CheckStaleness` threads
  `opts.Now` through `FreshnessAt` so both legs share one clock.
- **Did the fix simplify or special-case?** **Simplified.** One named
  function (`FreshnessAt`) owns the contract; `Freshness` becomes a
  one-line convenience. Pinning test
  (`TestFreshnessAt_DeterministicClock`) asserts the injected clock
  drives age computation, not the wall clock. No new vocabulary, no
  per-caller workarounds.

---

## 2026-05-10 — preflight signal-computation ordering

- **Shared concept**: the order in which Coverage, SafetyStatus,
  DegradedMode, and Trust are computed inside `preflight.Run`. They form
  a strict dependency chain: SafetyStatus and DegradedMode read
  `r.Coverage.Graph`, and Trust reads all three.
- **Per-subsystem interpretation**:
  - `computeConfidence`: "I assign Coverage as my output."
  - `computeSafetyStatus`: "I read r.Coverage.Graph and r.Coverage.Runtime
    to decide UNKNOWN_NOT_SAFE."
  - `computeDegradedMode`: "I read r.Coverage.Graph and r.SafetyStatus."
  - `computeTrustEnvelope`: "I read everything."
  - `preflight.Run` orchestration: "I call them in the right order — but
    that order is just numbered comments, not enforced by the type system."
- **Why unit tests missed it**: each helper was tested with a
  pre-populated Report. Nothing tested that `Run`'s call order matched
  the dependency graph; a refactor that reordered the four calls would
  pass every existing unit test while silently producing a report that
  marks a stale-graph architecture-sensitive task as PROCEED instead of
  UNKNOWN_NOT_SAFE.
- **End-to-end contract that should own it**: ideal would be a single
  ordered phase function (e.g. `computeReportSignals(r, g)`) that does
  Coverage → SafetyStatus → DegradedMode → Trust as one named operation,
  removing the possibility of accidental reordering. **Not done.**
  Pinned for now via regression test
  (`TestPreflightOrdering_StaleGraphProducesUnknownNotSafe`) which forces
  a 25h-old graph + architecture-sensitive task and asserts
  Coverage.Graph=stale, SafetyStatus=UNKNOWN_NOT_SAFE,
  DegradedMode.Enabled=true.
- **Did the fix simplify or special-case?** **Special-cased
  (regression-test-only).** Current HEAD already has the correct order;
  the test locks it down so a future reorder breaks loudly. The
  consolidation candidate (`computeReportSignals` single-function
  ordered phase) is owed but not yet earned by a second incident.

---

## 2026-05-10 — intentional_gap conflated with orphan at envelope layer

- **Shared concept**: the difference between "we deliberately accepted
  this is unenforced" (intentional_gap) and "this is unenforced and we
  missed it" (orphan).
- **Per-subsystem interpretation**:
  - `failure_modes.yaml` author: "I wrote `intentional_gap: true` so
    the system knows this gap is reviewed."
  - `manual` loader: "I store the flag in node metadata."
  - `assurance/coverage.classifyCoverage`: "I read the flag and
    classify the FM as Theoretical, preserving State='INTENTIONAL_GAP'."
  - `assurance/envelope.coverageFromFailureMode`: "I see Level=Theoretical,
    I return TrustCoverageNone — same as Orphan."
  - `assurance/envelope.decideVerdict`: "TrustCoverageNone + match =
    TrustUnsafe."
- **Why unit tests missed it**: coverage's unit tests assert the
  classifier returns the right Level and State. envelope's unit tests
  assert the verdict matches the input coverage. Neither test checked
  that the lifecycle hint *propagates* through both layers to the
  verdict. The two test suites are correct on opposite sides of a
  silent narrowing.
- **End-to-end contract that should own it**: lifecycle metadata
  should be a first-class field on every layer that touches it
  (CoverageReport, Compose inputs, TrustEnvelope, Reason). **Established
  2026-05-10**: `coverageFromFailureMode` reads `fmc.State` and treats
  INTENTIONAL_GAP as TrustCoveragePartial (verdict ceiling: Limited),
  while DEPRECATED stays None (verdict: Unsafe). The lifecycle hint
  now propagates from YAML → graph metadata → CoverageReport → envelope
  verdict without being squashed.
- **Did the fix simplify or special-case?** **Simplified.** Two new
  pinning tests
  (`TestCompose_IntentionalGapIsLimitedNotUnsafe`,
  `TestCompose_DeprecatedFailureModeStaysUnsafe`) lock the asymmetry.
  No new TrustCoverage value or per-FM special-case at the verdict
  layer; the lifecycle hint just flows through one named axis. The
  next reader of envelope.go sees the rule in one place, with
  documentation pointing at the incident.

---

## 2026-05-14 — release-index version field divergence

- **Shared concept**: "the platform's release version" — the single
  string the rest of the system uses to identify what is installed on
  this node. Read by Day-1 classification, the awareness bundle
  freshness check, and the runtime fact normalizer.
- **Per-subsystem interpretation**:
  - release-index writer (post-2026-05 build pipeline): emits the field
    as `platform_release`. The keys `version`, `release_version`, and
    `platform_version` are reserved in the schema but written as `null`.
  - `evidence/collector.go::readReleaseIndex`: parsed only the field
    `version`. With the writer's `version=null`, every read produced
    an empty `ReleaseInfo.Version`.
  - `evidence/normalizer.go::normalizeReleaseIndex`: treated empty
    `Version` as "file missing" and emitted `RELEASE_INDEX_MISSING`
    with detail "release-index.json not found".
- **Why unit tests missed it**: the collector's tests built
  `ReleaseInfo` literals in-memory and never round-tripped a real
  on-disk release-index payload. The writer's tests asserted the
  JSON keys they produced. Nothing tested the join: a real
  release-index.json read by the collector. Each side held its own
  contract; the contracts had drifted.
- **End-to-end contract that should own it**: **Established
  2026-05-14**. `ReleaseInfo` gains an explicit `Present bool` that
  distinguishes "file absent" from "file present, version unreadable."
  The new `parseReleaseIndex` accepts `platform_release` as canonical
  and the legacy `version` as fallback, in one place. The normalizer
  now keys off `Present`, not `Version`, so a parsed-but-empty file
  no longer masquerades as a missing one.
- **Did the fix simplify or special-case?** **Simplified.** Three
  scattered assumptions (collector parses one field, normalizer
  conflates absence with parse failure, writer drifts silently)
  collapse to one shared primitive (`ReleaseInfo.Present`) read in
  one place (`parseReleaseIndex`). Tests pin both the field-name
  contract and the present-but-empty case so the next field rename
  is a test failure, not a silent false positive.

---

## 2026-05-14 — collector probes loopback while services bind to node IP

- **Shared concept**: "is this service's TCP port listening?" — the
  primitive used to derive `FactScyllaCQLUnreachable`,
  `FactEtcdUnreachable`, and the cascade
  `FactWorkflowRemediationUnsafe`.
- **Per-subsystem interpretation**:
  - Architecture (`CLAUDE.md` hard rule #3): "NO localhost / 127.0.0.1
    for remote addresses. For bind/listen, use 0.0.0.0." Cluster
    services bind to the node's primary IP, never to loopback.
  - `evidence/collector.go::collectPorts`: dialed
    `127.0.0.1:<port>` with a 500ms TCP timeout. On a real cluster
    where Scylla listens on `10.0.0.63:9042`, the dial refused.
  - Normalizer: read the resulting `Listening=false` as evidence of an
    outage, emitted `FactScyllaCQLUnreachable`, then cascaded into
    `FactWorkflowRemediationUnsafe` because workflow depends on
    Scylla.
- **Why unit tests missed it**: every test in
  `evidence_test.go` injected `PortObservation` values directly. The
  collector's port-discovery primitive was never exercised against
  a running system in tests; it was implicitly assumed correct. The
  classifier and normalizer were both correct given their inputs.
  Nothing checked that the collector's *inputs* were truthful on a
  cluster that follows the no-localhost rule. A composed-path test
  would have caught it instantly; the absence of one let the
  primitive lie for as long as it took someone to query
  `awareness.runtime_errors` from the field.
- **End-to-end contract that should own it**: **Established
  2026-05-14**. `collectPorts` reads `/proc/net/tcp{,6}` and reports
  listener state by port number alone, independent of bind address.
  Scylla on 10.0.0.63, etcd on 0.0.0.0, MinIO on the node IP, MCP on
  127.0.0.1 — all are observed the same way. The split between
  `parseListeningPorts` (pure parser over an `io.Reader`),
  `listeningTCPPortsFromPaths` (file shim), and `procNetTCPPaths`
  (injectable default) makes the primitive testable with synthetic
  fixtures.
- **Did the fix simplify or special-case?** **Simplified.** One
  bind-address-agnostic primitive replaces a per-port loopback dial
  loop. The new tests pin both the parser's handling of mixed bind
  addresses (loopback, wildcard, node IP) and the
  composed-path regression: a Scylla listener on the node IP must
  NOT trip the workflow-remediation cascade. The next contributor
  who adds a port to `knownPorts` cannot accidentally re-introduce
  the loopback assumption.

---

## 2026-05-14 — PKI fileReadable conflates missing with not-readable

- **Shared concept**: "is this PKI artifact usable as a trust input?" The
  answer is consumed by `normalizePKI` → `pkiReady` → Day-1 verdict →
  remediation action.
- **Per-subsystem interpretation**:
  - `collector.collectPKI` (old): called `fileReadable(path)` and stored
    the result as `*Present`. The bool collapsed "file exists" and
    "current process can read file" into one signal.
  - `normalizer.normalizePKI` (old): treated `!*Present` as evidence of
    a missing artifact and emitted `FactPKIMissing` with detail
    "PKI artifact missing: <path>".
  - `classifier.classify`: routed `FactPKIMissing` to `ClassPKIMissing`,
    whose AllowedActions are "request certificate issuance from
    cluster CA" — a re-issue prescription.
  - In production, the CLI invoked by user `dave` saw `service.key`
    (mode 0400 owned by globular) as unreadable. The whole pipeline
    reported `PKI_MISSING` and prescribed re-issuance for a file that
    was perfectly intact.
- **Why unit tests missed it**: every PKI test fixture constructed
  `PKIObservation{CACertPresent: true, ...}` from already-decided
  bools. The collector primitive (`fileReadable`) was never tested
  against a file that *exists but is unreadable to the running uid* —
  precisely the state that surfaces only when the verifier is a
  different user than the service. The asymmetry between collector
  context and service context wasn't part of any test's mental model.
  This is the **second incident** of the same primitive-collapses-
  two-conditions-into-one-bool shape; the first was the 127.0.0.1
  dial in the port-listening primitive (also 2026-05-14). Per the
  log's own rule, two incidents on the same shape is the trigger
  to consolidate — which is what this entry does.
- **End-to-end contract that should own it**: **Established
  2026-05-14**. `observeFile(path) (exists, readable bool)` returns
  the two states independently. `PKIObservation` carries them as
  separate fields (`*Present`, `*Readable`). The normalizer emits
  `FactPKIMissing` only when `!Present` and the new `FactPKIUnreadable`
  when `Present && !Readable`. The classifier surfaces the new
  `ClassPKIUnreadable` with a permissions/ownership remediation hint
  and explicitly *forbids* re-issuance — the previous default action
  for the unreadable case was wrong and lossy.
- **Did the fix simplify or special-case?** **Simplified.** One
  primitive that conflated two conditions becomes one primitive that
  returns both states honestly. The contract is the same shape as the
  release-index fix in the prior entry (`Present` separated from
  parseability) and the port fix (bind-address agnosticism). Three
  bug entries, one underlying pattern — collector primitives that
  squashed multi-state observations into a single bool. Tests now
  pin (a) `observeFile` returns the two states correctly, (b)
  MISSING wins when both apply, (c) the classifier picks the right
  verdict + forbids the wrong remediation.

---

## How to add an entry

1. Use the schema above. Do not skip the five fields.
2. Date the entry.
3. Link the failing test, PR, or commit if there is one.
4. If the fix is a patch (special-case), say so. Don't paint it as a
   simplification.
5. If a consolidation candidate already appears in another entry, that's
   the trigger to do the consolidation work — promote the candidate to
   an explicit task.

The log accumulates evidence. The fixes that simplified the system are the
ones the next bug won't relitigate. The ones that special-cased are debt
the system pays interest on every time another contributor steps near
the same shared concept.
