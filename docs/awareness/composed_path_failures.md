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

## 2026-05-14 — graph.Open always migrates against immutable bundles

- **Shared concept**: "is this graph database something I can modify?"
  Read by every consumer that opens a `*graph.Graph` handle: build,
  preflight, MCP, CLI.
- **Per-subsystem interpretation**:
  - `bundlesync` (installer): "I write a signed, content-addressed
    artifact to `/var/lib/globular/awareness/installed/<version>/
    <uuid>/`. Root-owned, mode 0644 for files, 0755 for dirs.
    Immutable post-install — that's the whole point of content
    addressing."
  - `graph.Open` (library): "I always run `migrate()` on every open.
    Migration executes DDL via `db.Exec(schemaSQL)` and uses WAL
    journal mode, which needs to create `-wal`/`-shm` sidecars in
    the parent directory."
  - MCP service: "I open the active bundle at
    `/var/lib/globular/awareness/current/graph.db` as user `globular`."
  - Reality: the service user can't write to a root-owned bundle dir.
    Open fails with "attempt to write a readonly database", awareness
    degrades to noop, and downstream tools (runtime_errors,
    failure_learning_list_pending, every awareness query) silently
    return `{status: degraded}` or stale results.
- **Why unit tests missed it**: `graph` package tests always use
  `t.TempDir()` — a freshly created, fully writable directory.
  `bundlesync` tests verify file installation under a temp install
  root the test process owns. MCP tests inject a mocked graph state
  rather than calling `graph.Open` against a real bundle path.
  Each test suite proved its own slice; nothing tested "the MCP, as
  the service user, opens a root-installed bundle." The contradiction
  between "immutable bundle" and "always migrate on open" was visible
  in the contract documents but invisible to every test.
- **End-to-end contract that should own it**: **Established
  2026-05-14**. `graph.OpenReadOnly(path)` opens with SQLite URI
  parameters `mode=ro&immutable=1`, skips `MkdirAll`, skips
  `migrate()`. Reads succeed regardless of directory ownership;
  writes return SQLite's read-only error cleanly. MCP detects bundle
  paths via `isAwarenessBundlePath` and routes them through
  OpenReadOnly; non-bundle paths (dev checkouts, writable runtime
  databases) still use `Open` so learn_from_fix and similar writers
  keep working in their proper context.
- **Did the fix simplify or special-case?** **Simplified.** The
  bundle's immutability contract is now honoured by a verb that
  matches it (`OpenReadOnly`). The previous workaround — staging a
  writable copy of the signed bundle's graph.db under
  `/var/lib/globular/awareness/runtime/` and pointing MCP at that —
  split authority across two file homes. With this fix the bundle is
  the single source of read truth; the workaround copy can be retired.
  Three pinning tests lock the contract: ReadOnly succeeds against a
  non-writable file, writes through the handle fail with a read-only
  error, and the MCP path classifier correctly distinguishes bundle
  paths from writable runtime paths. Writable session/experience/
  learning data (the small fraction of awareness state that should
  actually mutate at runtime) belongs in a separate writable database
  — that's the architectural follow-up this commit makes possible
  but doesn't implement.

---

## 2026-05-14 — Writable runtime alongside immutable bundle (consolidation)

This entry consolidates the 2026-05-14 "graph.Open always migrates against
immutable bundles" failure. That fix established `OpenReadOnly` for the
signed bundle; this fix establishes where writes actually go.

- **Shared concept that fragmented**: where the small fraction of
  awareness state that is genuinely mutable at runtime lives. Sessions,
  coordination, experience attempts, learning proposals, semantic-diff
  reports, file-fingerprint snapshots, agent usage events, incident
  patterns, and live-cluster signal snapshots all need to be written
  while the system is running. The bundle, by contract, does not.
- **Per-subsystem interpretation**:
  - `graph` package: "Open() always migrates and writes WAL sidecars."
  - `graph` (post 22dbdfe6): "OpenReadOnly() refuses every write,
    leaving writers without a home."
  - `bundlesync`: "The bundle is content-addressed and installed once
    by root; runtime mutation is somebody else's problem."
  - MCP awareness tools: "I have a `*graph.Graph` handle; I assume
    INSERT works." Every register*Tools call site that wrote runtime
    data — incident patterns, sessions, coordination, experience,
    semantic diff, learn-from-fix — was structurally unable to write.
    Composed-path symptom: an entire class of MCP awareness tools
    silently returned "read-only database" errors with no shared
    remediation, fragmenting "where does this row go?" across every
    subsystem.
- **Why unit tests missed it**: each writer's tests opened a writable
  `t.TempDir()` graph via `graph.Open`. None ran against
  `graph.OpenReadOnly` (introduced by the previous fix), and nothing
  tested "MCP starts with the bundle as the read source — can the
  writers still write?" The previous fix's tests pinned that reads
  worked and writes were refused; they couldn't pin where writes
  *should* go because no such concept existed.
- **End-to-end contract that should own it**: **Established 2026-05-14**.
  `graph.OpenComposite(bundlePath, runtimePath)` opens the runtime
  database read-write (the writable home for all mutable awareness
  state) and ATTACHes the bundle read-only as `bundle`. The
  partition is enforced by `bundleOnlyTables` (nodes, edges,
  invariants, failure_modes, graph_builds, context_aliases) — these
  tables are DROP'd from the runtime database after migration so
  SQLite name resolution sends unqualified reads through ATTACH to
  the bundle. Every other table lives in the runtime database and
  accepts writes normally. Cross-database JOINs (e.g.,
  `experience_entries LEFT JOIN nodes`, `service_live_states JOIN
  edges`) keep working transparently — SQLite handles them across
  attached schemas. The runtime database lives at
  `/var/lib/globular/awareness/runtime.db`, sibling of the bundle
  root, so bundle reinstalls swap the symlink without touching
  accumulated session/coordination/experience data.
- **Did the fix simplify or special-case?** **Simplified.** One verb
  (`OpenComposite`) replaces the threatened proliferation of "open
  the bundle for reads, open this other file for writes, manage two
  handles in every consumer." Every existing consumer keeps its
  single `*graph.Graph` handle and existing query strings; the
  partition is invisible at the call site. Eight tests pin the
  contract: bundle reads, runtime writes, bundle writes refused,
  cross-DB JOINs, parent dir auto-create, persistence across
  reopen, missing-bundle rejection, and direct verification that
  bundle-only tables are absent from the main schema. The previous
  fix's transitional `/var/lib/globular/awareness/runtime/`
  workaround dir can now be retired — runtime data has its own home
  and the bundle stays untouched.

---

## 2026-05-14 — fileReadable conflation, consolidated as fsutil.ObserveFile

Third incident on the "primitive collapses two conditions into one bool"
shape, second incident triggering consolidation per the log's rule. The
first was the PKI fileReadable bug in the awareness evidence collector
(commit `be0a8f8c`). The second was discovered during an audit of other
`fileReadable` callers and turned out to be latent in the MCP runtime
checks.

- **Shared concept that fragmented**: how a file's two independent
  states — *is it on disk* and *can this process read it* — are
  reported by helpers shared across subsystems. `os.Open` returns an
  error for both "no such file" and "permission denied"; reducing
  that to one bool conflates two operationally distinct conditions
  with non-overlapping remediations.
- **Per-subsystem interpretation**:
  - `awareness/evidence` (pre-fix): "Can I open this PKI artifact?"
    Result wired straight through as `*Present`. `false` meant either
    "file gone" or "wrong user" — re-issuance and ownership-fix
    collapsed into one remediation, almost always the wrong one.
  - `mcp/runtime_activation_check_tool.go` (pre-fix): local
    `fileReadable` with the same `os.Open`-returns-bool shape.
  - `mcp/runtime_sources_config.go`: called fileReadable(caPath),
    emitted "CACert (not found: %s)" when an operator running the
    check as a non-globular user against a 0640 globular:globular
    CA would actually hit "exists, but you can't read it".
  - `mcp/runtime_config_bootstrap_tool.go`: three call sites for
    CA cert, client cert, and client key with the same conflation.
    The mode-0400 service.key is the most-likely live trigger: any
    user other than globular sees "Client key not found at ...",
    sending them to reissuance when the actual fix is to run the
    bootstrap as the service user.
- **Why unit tests missed it**: each subsystem's tests opened a fresh
  `t.TempDir()` where the test process is the file owner. The
  permissions-denied branch is only exercised by a `chmod 0o000` plus
  a non-root euid, which only one suite (the evidence collector) ran
  after the original PKI fix. The two MCP callers were never tested
  against an unreadable file. The composed-path failure was the same
  shape, just hiding behind tests that owned every file they checked.
- **End-to-end contract that should own it**: **Established 2026-05-14**.
  `golang/fsutil/ObserveFile(path) (exists, readable bool)` is the
  shared primitive. `(true, false)` is the meaningful state that the
  old bool collapsed; `(false, true)` is unreachable by construction.
  Five tests pin the contract — present-and-readable, absent, present-
  but-unreadable (skipped under root), empty path, and a dimensional
  sweep proving (false, true) is unreachable. The MCP call sites now
  emit two distinct messages (`mtlsCredentialError`,
  `mtlsMissingConfigEntry`, `pkiCertWarning`) — "not found at X"
  guides reissuance; "exists at X but not readable by this process"
  guides ownership/permissions or running-as-service-user. The
  evidence collector's `observeFile` now delegates to fsutil.
- **Did the fix simplify or special-case?** **Simplified.** One
  primitive replaces two independent fileReadable copies and an
  open-coded check shape that was about to spread to a fourth call
  site. Three different remediation strings collapse to two distinct
  ones with clear ownership of which case is which. The bug shape
  hasn't disappeared — any future caller could re-invent the
  conflation — but the fix establishes the verb (`fsutil.ObserveFile`)
  that future contributors will grep for instead.

---

## 2026-05-14 — Degraded source-availability sentinel collapsed to critical

This is a **third-incident** entry for a shape that has now repeated three
times. The consolidation candidate (a typed `sourceroot.Resolve()`
primitive) is established by this entry; the migration begins with one
call site (`awarGitRoot`) and the remaining sites become follow-up work.

- **Shared concept**: "is a real source tree available to scan from this
  process?" Several awareness subsystems all need this answer when they
  decide whether to walk Go files, look up test functions, or measure
  graph file coverage. The honest answer has four states — `FOUND(path)`,
  `ABSENT`, `INACCESSIBLE(err)`, `WRONG_CONTEXT(path,reason)` — but each
  subsystem invented its own degraded signal and each consumer collapsed
  the signal into a different bucket.

- **Per-subsystem interpretation**:
  - `awarGitRoot` (mcp/tools_awareness.go): "git rev-parse failed → fall
    back to `os.Getwd()` and return that as the root." On a production
    MCP host whose cwd is `/var/lib/globular/mcp`, this returned the
    install dir as if it were the repo. The integrity scanner then
    walked it, found no `*_test.go` files, and reported every fix_case's
    required tests as `REQUIRED_TEST_MISSING` (severity: critical) —
    39 false alarms.
  - `verifyGapTests` (mcp/self_review_tool.go): when `repoRoot == ""`,
    returned the string `"unverified"`. The consumer
    `buildSelfReviewSection` had a `switch` whose default branch counted
    everything-not-explicitly-classified as `tests_not_found`. 62
    implemented gaps were reported as "missing tests" when in fact the
    verifier just had nothing to scan.
  - `enforce.GoFileCoverage` + `coverage_report`: when `repoRoot == ""`,
    GoFileCoverage returns early with `EligibleGoFilesTotal = 0` and
    `ConfidenceImpact = "unknown"`. coverage_report ignored
    `ConfidenceImpact` and only checked `CoveragePercentGoFiles < 70`,
    so 0% → `Status = "critical"` with a "0 Go files indexed" message.
    A pure can't-check became "platform-wide coverage critical."
  - `OpenComposite` (graph/composite.go) + dbPath resolution
    (tools_awareness.go): when the bundle's writable runtime dir
    couldn't be created (EPERM), composite-mode init failed. The
    surrounding dbPath resolution had already preferred the bundle path
    if it existed, but had no symmetric handling for "bundle exists,
    can't open it cleanly" — it left `st.g = nil`, and at an earlier
    point in time when the bundle didn't yet exist, fell through to a
    1.4MB writable shadow at the legacy system path. End state for the
    operator: the same coverage report said "0 / 0 indexed", flagged
    critical, and 8 core components looked like they had no failure
    modes at all.

- **Why unit tests missed it**: every consumer tested its slice with a
  populated repoRoot (the test process IS in a git checkout, so
  `awarGitRoot()` returned the right path; `verifyGapTests("/path/...")`
  found the test functions; `GoFileCoverage` walked a real tree). None
  of the tests simulated the production-MCP context where the process
  has no source tree at all. The "no source" path was unreached in
  testing, so each consumer's degraded-sentinel handling was free to
  drift independently. The composition (degraded sentinel → consumer's
  default branch → critical) only manifests when all three layers see
  the same "no source" reality at once, which is exactly the production
  state of every MCP host shipped without source.

- **End-to-end contract that should own it**: **Establishing
  2026-05-14**: `golang/awareness/sourceroot/sourceroot.go` —
  `Resolve(opts) Result` returning a discriminated union
  `(State, Path, Err, Reason)` with explicit states
  `Found | Absent | Inaccessible | WrongContext`. The package adds
  helper `IsAvailable() bool` so callers that just need a yes/no can
  ask without re-implementing the classification. **No silent cwd
  fallback.** **No string sentinels.** Consumers (verifyGapTests,
  coverage_report, integrity.CheckTestReferences, future scanners)
  must take the typed result and decide for themselves whether a
  non-`Found` state is a degraded telemetry signal (info) or a real
  failure (critical) — `Absent` and `WrongContext` are **never**
  critical-missing-evidence by definition; only `Found` with a scan
  that produced zero matches is.

- **Did the fix simplify or special-case?** **Partial — simplification
  begins, special-cases still ship.** This patch:
  1. Introduces the typed `sourceroot` primitive.
  2. Migrates `awarGitRoot` to delegate (removing the cwd fallback bug
     for real, not just by adding an empty-string return path).
  3. Closes the third symptom (coverage_report critical/unverified
     conflation) directly.
  4. Pins all three observed cases with regression tests.

  What is **still owed**: the other call sites that today reach for
  `awarGitRoot()` or read `cfg.RepoPath` directly haven't been
  migrated. The next contributor who writes a new scanner that says
  "if repoRoot == "" { ... }" hasn't been forced to confront the
  four-state model. The consolidation lands fully when every source-
  scan consumer in `golang/awareness/` and `golang/mcp/` reaches the
  primitive instead of inventing its own sentinel. Two passes
  ahead — track in the open consolidation candidates list.

  **Forbidden-fix shape recorded**: see
  `forbidden_fixes.yaml::collapse_source_absent_into_critical_missing_evidence` —
  any future PR that maps `unverified` / `source_absent` / `wrong_tree`
  to a critical-severity "missing" finding is now a graph-level
  violation, not just a code review nit.

---

## 2026-05-14 — Awareness bundle build → publish → freshness contract drift

This is the **second incident** on the "bundle pipeline contract
fragmented across subsystems" shape; the first was the 2026-05-10
"Bundle freshness contract" entry (which consolidated on
`bundlesync.DefaultManifestPath`). Both incidents share the same
root: the pipeline that creates an awareness bundle, the pipeline that
publishes one, and the pipeline that verifies one were each authored
with their own private assumption about what an "awareness bundle" is.

- **Shared concept that fragmented**: a single end-to-end answer to
  "what identifies an awareness bundle on this cluster?" — name, version,
  build_id, schema_version, sha256 — that flows from `awareness bundle
  build` through publish into the repository index and back out to every
  freshness consumer.

- **Per-subsystem interpretation**:
  - `globular awareness bundle build` (`globularcli/awareness_bundle_cmd.go`):
    wrote a cli-local `awarenessBundleManifest` struct with
    `name/kind/version/build_id/built_at` but no `schema_version` and no
    `sha256` field in the manifest. The build's "To publish" hint pointed
    at `globular package publish --kind AWARENESS_BUNDLE`, a command
    shape that did not exist (`pkg publish` has no `--kind` flag and
    requires `package.json` via `pkgpack.VerifyTGZ`, which awareness
    bundles do not ship).
  - `globular pkg publish` (`globularcli/pkg_cmds.go`): inferred artifact
    kind from `package.json::type`, so an awareness bundle (which has
    `manifest.json` instead) failed validation at step 1.
  - `repository.UploadArtifact` (`repository/repository_server/
    artifact_handlers.go`): would have accepted the bundle — its
    `extractPackageManifest` returns nil silently when there is no
    `package.json`, and `ref.Kind` is honoured. No subsystem had wired the
    CLI side to that capability.
  - `ValidateReleaseIndexForInstall`
    (`repository/repository_server/release_index.go`): rejected any
    release-index entry whose `kind` was `AWARENESS_BUNDLE` because the
    `kindFromArtifactKindString` switch did not list the kind. A
    CI-generated BOM would have failed with "kind is not supported for
    install validation" before reaching any node.
  - MCP freshness loader (`mcp/tools_awareness_bundle_freshness.go`):
    parsed `release-index.json` as either flat
    (`{"version": ..., "build_id": ...}`) or nested
    (`{"active": {...}}`). The canonical repository BOM shape
    (`{"schema_version": ..., "packages": [...]}`) matched neither, so
    every freshness check on a real cluster returned
    `AWARENESS_BUNDLE_VERIFY_FAILED` regardless of whether a bundle had
    actually been published.
  - `bundlesync.Manifest` (consumer): required
    `name/version/build_id/schema_version/sha256` non-empty. The cli's
    build wrote none of `schema_version` or `sha256`. Any node that
    activated a freshly built bundle would have surfaced
    `AWARENESS_BUNDLE_SCHEMA_UNSUPPORTED`.

- **Why unit tests missed it**: each subsystem's tests were correct
  for its own slice. `awareness_bundle_cmd_test.go` proved the build
  collected the right files into the tar. `pkg_cmds_test` / pkgpack
  tests proved service publish validated package.json. The repository
  release-index tests asserted that recognised kinds passed install
  validation but never asserted that AWARENESS_BUNDLE in particular
  did. The MCP freshness tests fed the loader the flat or nested
  shapes it could already parse. None of these suites composed the
  five layers into a single end-to-end test: build → publish →
  release-index round-trip → freshness verdict.

- **End-to-end contract that should own it**: **Partial — established
  for kind and release-index shape; identity-flow consolidation owed.**
  The fixes shipped here are:
  1. `globular awareness bundle publish` (new CLI verb in
     `golang/globularcli/awareness_bundle_publish.go`) reads the
     bundle's own manifest.json, validates the identity fields,
     computes the archive sha256, and uploads with `ref.Kind =
     AWARENESS_BUNDLE`. The build hint now points here.
  2. `kindFromArtifactKindString` learns `AWARENESS_BUNDLE` (and
     `SUBSYSTEM`, which was also missing) so a release-index entry
     for an awareness bundle passes install validation.
  3. `loadReleaseIndex` in MCP gains a BOM-shape extractor that
     finds the `AWARENESS_BUNDLE` entry in `packages[]` and prefers
     the canonical `globular-awareness-bundle` name when several
     bundles are present. Flat / nested shapes remain accepted.
  4. `bundlesync.CurrentBundleSchemaVersion` is the single source of
     truth for the schema string newly built bundles stamp into
     manifest.json; build writes it, publish refuses to upload a
     manifest whose `schema_version` is non-empty and unsupported.
     `bundlesync.IsSupportedSchemaVersion` is the public predicate so
     publish-time validation does not reach into a private helper.

  What is **still owed**:
  - The bundle manifest still has two shapes (cli's
    `awarenessBundleManifest` vs `bundlesync.Manifest`) with
    overlapping fields and different JSON tags (`built_at` vs
    `created_at`, missing `size_bytes` and `graph_hash` on the cli
    side). The fix here makes the divergence honest at the
    schema_version axis; consolidating to a single shared
    `bundlesync.BuildManifest` is still owed.
  - The build does not currently write the archive's own sha256 into
    the inner manifest. The publish path computes it on the fly so
    the repository records a verified checksum, but a node that
    activates a bundle then calls `bundlesync.VerifyManifest` will
    see `SHA256 == ""` and either error out or skip the check. That
    is freshness state machine territory — recorded here so the
    next consolidator does not have to rediscover it.

- **Did the fix simplify or special-case?** **Partial.** Three
  scattered subsystems (release-index validation, MCP freshness, CLI
  publish) now share one understanding of `AWARENESS_BUNDLE` as a
  first-class kind with a documented schema version. That is one
  concept replacing three private guesses. But the cli's bundle
  manifest is still a distinct struct from the consumer's manifest;
  the two will need to be merged before the next bundle-format
  change. Tests pin: (a) `pkg publish` rejects awareness archives at
  the boundary (`pkgpack/verify_test.go::AwarenessBundleShapeRejected`),
  (b) the publish CLI accepts a build-shape archive end-to-end
  (`globularcli/awareness_bundle_publish_test.go`), (c) release-index
  install validation accepts the kind
  (`repository/.../release_index_test.go::AcceptsAwarenessBundleKind`),
  (d) the MCP freshness loader extracts a BOM-shape entry
  (`mcp/tools_awareness_bundle_freshness_test.go::BOMShape`), and
  (e) the schema_version round-trips through build → publish
  (`globularcli/awareness_bundle_publish_test.go::SchemaVersion`).

---

## 2026-05-14 — Installer drops the bundle tarball; serve tools then lie

End-to-end verification of the awareness bundle publish path surfaced this
as a separate composed-path failure adjacent to the publish work. Same
day, same domain, but a different shape from the publish/release-index
divergence above.

- **Shared concept that fragmented**: where the original `bundle.tar.gz`
  archive lives after a successful install, and which subsystems need to
  read it later. Two readers and one writer disagree.

- **Per-subsystem interpretation**:
  - `bundlesync.InstallBundle`: "I extract the bundle into
    `installed/<version>/<build_id>/` and copy `manifest.json` next to
    the unpacked contents. The source tar.gz is the caller's
    responsibility; I do not retain it." Written before the MCP serve
    tools existed.
  - `mcp.awareness_bundle_manifest`
    (`mcp/tools_awareness_bundle_serve.go`): "I open
    `/var/lib/globular/awareness/current/bundle.tar.gz` to compute a
    fresh sha256 over the bytes I would stream. If the file is missing,
    the bundle is missing." Written assuming the installer retains the
    tarball.
  - `mcp.awareness_bundle_stream` and the corresponding HTTP handler:
    "I serve `/var/lib/globular/awareness/current/bundle.tar.gz` to
    remote pullers." Same assumption.
  - `awareness.bundle_status`: "I read the manifest only; the bundle
    file's presence is not part of my answer." A third reader with a
    completely different contract.
  - Live evidence on globule-ryzen as of 2026-05-14: `bundle_status`
    reports `present: true, status: LOADED`, while
    `mcp_awareness_bundle_manifest` reports
    `state: AWARENESS_BUNDLE_MISSING` for the same physical bundle. The
    two tools disagree on the same install, by design — one trusts the
    manifest, the other trusts the tarball.

- **Why unit tests missed it**: every install test ran in `t.TempDir()`
  and asserted only on `graph.db` + the manifest sidecar
  (`install_test.go::TestInstallBundleFreshInstall` etc.). No test
  asserted that the source tarball survived the install. The MCP serve
  tools' tests synthesized their own `bundle.tar.gz` at the active
  bundle path, never running the installer first. The composition
  (install → serve) was untested.

- **End-to-end contract that should own it**: **Established
  2026-05-14**. `installedBundleFilename` is a constant in the
  `bundlesync` package documenting the one filename serve tools and
  installer agree on. `InstallBundle` now copies `opts.BundlePath` into
  the staging directory as `bundle.tar.gz` via `copyFileAtomic` before
  the atomic rename, so the retained tarball lives in the same
  versioned dir as the extracted contents. The `current` symlink
  inherits it. MCP serve tools still read by the well-known filename;
  one source of truth, one filesystem layout.

- **Did the fix simplify or special-case?** **Simplified.** One install
  step now retains the artifact every downstream serve tool already
  expects to find. The retained copy lives inside the same
  content-addressed `installed/<version>/<build_id>/` dir as the
  extracted graph + manifest, so the symlink swap keeps it consistent
  with everything else. Test
  `TestInstallBundleFreshInstall` now asserts the retained bundle
  exists and matches the source's byte count. A complementary
  consolidation candidate — moving the literal `"bundle.tar.gz"` from
  MCP's `activeBundleFilename` variable to the new
  `bundlesync.installedBundleFilename` const — is owed (the literal is
  still duplicated; the constant gives the right grep target).

---

## 2026-05-14 — Publish path operational; activation lag is not freshness-state-machine failure

Closing entry for the awareness bundle publish work landed in commit
`1825a1f1`. Records the verification stance so a future contributor
reading the log knows what is proven, what was observed, and what is
deliberately deferred.

- **What is proven**:
  - The CLI publish path is operational end-to-end against a live
    repository. A bundle built by `awareness bundle build` and
    uploaded by `awareness bundle publish` is stored with
    `kind=AWARENESS_BUNDLE` (verified by direct grpcurl against
    `repository.PackageRepository/GetArtifactManifest`); the
    repository's recorded `checksum` matches the local sha256
    byte-for-byte; `publish_state` advances VERIFIED → PUBLISHED
    server-side via `completePublish`. A subsequent
    `DownloadArtifact` round-trip returns 6,871,209 identical bytes
    and the inner manifest still carries
    `schema_version: awareness.bundle.v1`.
  - Service-package publish is unaffected: `pkg publish` of a real
    service tgz dry-runs cleanly; an awareness-shape archive is
    rejected at the `pkgpack.VerifyTGZ` boundary with the message
    `"validation failed: package.json missing from archive"`.

- **What live MCP probes showed before activation**:
  - `awareness_bundle_status.freshness.state =
    AWARENESS_BUNDLE_VERIFY_FAILED` with reason `"release-index
    /var/lib/globular/release-index.json: no usable version/build_id"`.
    This is the BOM-shape parse failure now fixed in
    `mcp/tools_awareness_bundle_freshness.loadReleaseIndex`.
  - `mcp_awareness_bundle_manifest.state =
    AWARENESS_BUNDLE_MISSING` for the same install. Root cause: the
    installer never retained `bundle.tar.gz` next to the unpacked
    contents. Now fixed in `bundlesync.InstallBundle` via
    `copyFileAtomic` and pinned by `TestInstallBundleFreshInstall`.

- **Why this is activation lag, not a freshness state-machine
  defect**: both observed disagreements have identifiable code
  causes that have been fixed and tested. The fixes will land on
  every cluster as soon as a tagged release rebuilds the deployed
  binaries (controller, node-agent, MCP) and a node activates a
  bundle produced by the new build. Until then, MCP serves the
  previously-installed bundle (1.2.46) with the previously-deployed
  code (which has neither the BOM parser nor the tarball
  retention). No part of the failure cascade required a freshness
  state machine to explain — every symptom maps to a primitive that
  was either missing or wrong.

- **What activation verification will close**:
  1. Tagged release ships rebuilt `node_agent_server`, `globular`,
     and `globular-mcp` binaries.
  2. A node fetches a bundle whose archive `kind=AWARENESS_BUNDLE`
     was published via the new CLI.
  3. `bundlesync.InstallBundle` retains `bundle.tar.gz` under
     `/var/lib/globular/awareness/installed/<version>/<build_id>/`.
  4. `awareness_bundle_status` reads release-index via the BOM
     extractor, returns `freshness.state = AWARENESS_READY`.
  5. `mcp_awareness_bundle_manifest` finds the retained tarball,
     hashes it, returns the same `(version, build_id)` the
     `awareness_bundle_status` tool reports.
  6. `mcp.awareness_freshness_status` agrees.

  Step 5/6 agreement is the falsifier for "we still need a
  freshness state machine." If activation lands AWARENESS_READY
  across all three tools, **#3 stays deferred indefinitely**. If a
  novel disagreement appears that traces to a missing freshness
  primitive (not a missing release-index field, not a missing
  retained tarball), then and only then is #3 evidence-driven work.

- **Forbidden shortcuts**: per the project rules
  (`memory/feedback_see_truth_fix_root.md`, CLAUDE.md hard rules),
  do not hot-deploy locally-built binaries to verify activation.
  Use the normal release path. The verification is genuinely waiting
  on a deploy cycle; trying to short-circuit it would reintroduce
  the exact category of bug the manifest divergence already
  represents.

---

## 2026-05-15 — Two-column installability predicate split (artifact_state vs publish_state)

- **Shared concept**: "is this artifact safe to install?" — the compound
  predicate that the node-agent evaluates before downloading and applying
  a package. Requires both `publish_state = PUBLISHED` **and**
  `artifact_state = PUBLISHED`. Every consumer that asks this question must
  consult the same pair of columns.
- **Per-subsystem interpretation**:
  - `repository.promoteToPublished`: "My job is to mark the artifact
    published. I call `UpdatePublishState(PUBLISHED)`." Written before
    `artifact_state` existed as a separate column.
  - `repository.ListArtifacts` / `resolveLatestBuildNumber`
    (Scylla-first fix, v1.0.65): "I read `publish_state` to decide
    which artifacts are visible." Correct for discovery.
  - `node_agent.post-install reconciler loop`: "I periodically re-check
    whether the desired build_id is still installable. I read
    `artifact_state`." Returns `BLOB_VERIFIED` → classifies as
    `DesiredBuildIdOrphaned` → triggers infinite install-retry storm.
  - Day-0 founding node: bypassed the repository state machine entirely
    (manual tarball seed). Neither column was consulted.
- **Why unit tests missed it**: `promoteToPublished` tests asserted
  that `publish_state` was written correctly — and it was. Post-install
  reconciler tests asserted that `artifact_state=BLOB_VERIFIED` triggered
  the orphan path — and it did. Discovery tests asserted that
  `publish_state=PUBLISHED` made an artifact visible — and it did. No test
  composed the full publish → discover → install → post-install-recheck
  pipeline and then verified that both columns agreed. The two columns
  were each tested in isolation by their respective owners.
- **End-to-end contract that should own it**: **Established 2026-05-15**.
  `promoteToPublished` calls both `UpdatePublishState(PUBLISHED)` and
  `transitionArtifactState(PUBLISHED)` atomically before returning. The
  invariant is: **no artifact may be returned to a node-agent unless
  both columns equal PUBLISHED**. Any future lifecycle transition
  function that touches one column must explicitly decide what to do
  with the other — and that decision must be tested at the composed-path
  level, not just at the per-column level.
  - Consolidation candidate: **promote a single `MarkArtifactPublished`
    primitive** that writes both columns in one place, so future callers
    cannot write one without the other. Currently `UpdatePublishState` and
    `transitionArtifactState` are separate functions; a future PR that
    merged them under one named entry point would make the split
    structurally impossible.
- **Did the fix simplify or special-case?** **Special-cased (one-liner
  patch).** Added one call to `transitionArtifactState` at the end of
  `promoteToPublished`. The two functions remain independent; nothing
  prevents a future writer from updating one without the other. The
  consolidation (a single `MarkArtifactPublished`) is owed.
  Regression test: `TestPromoteToPublishedSetsArtifactState` pins that
  after the full publish flow both `publish_state` and `artifact_state`
  equal `PUBLISHED` before any artifact is returned to a node-agent.

---

## 2026-05-15 — Join-order temporal split: wrong diagnostic axis

This entry is a **meta-failure** — an awareness system failure to guide
the right diagnostic axis. It belongs in this log because the composed
path from "symptom observed" → "root cause identified" produced the
wrong behavior: the AI chose the within-node comparison axis when the
evidence demanded the cross-node temporal axis.

- **Shared concept**: when N-1 nodes converge and node-N partially fails
  with the same package set, the primary diagnostic question is **what
  state changed between the N-1 join event and the N join event** — not
  "what property differs between the failing packages within node-N."
  This is a named diagnostic shape, not a novel situation.
- **Per-subsystem interpretation**:
  - Symptom: Day-0 OK, Day-1 node-1 fully converged, Day-1 node-2
    partially converged (some packages install, others stuck).
  - AI diagnostic engine: "I will compare the failing packages against
    the succeeding packages within node-2 — kind, profile, dependency
    order." Stayed in the within-node axis for the full diagnostic
    session.
  - User: "compare node-1 with node-2" — named the temporal axis.
    Root cause found in one step: the post-install re-check loop had
    started a retry storm between the two join events.
  - Awareness preflight and scan-violations: returned no matched
    finding, no ranked diagnosis path, no hint toward the temporal
    axis. Both columns existed in the awareness graph as separate
    nodes; no compound invariant linked them; no causal rule described
    the partial-convergence-in-join-order symptom sequence.
- **Why awareness missed it**:
  1. **No compound-predicate invariant.** The graph knew
     `publish_state` and `artifact_state` as separate nodes but had no
     rule: "artifact.installable iff both columns = PUBLISHED." Without
     it, `scan_violations` could not flag `promoteToPublished` as a
     violator.
  2. **No write-path coverage map.** No record of which functions are
     required to write `artifact_state`. If such a map existed,
     `promoteToPublished` would have appeared as absent from the
     required writers set.
  3. **No join-order causal rule.** No causal rule described the
     symptom sequence: "N-1 nodes converge → node-N partially fails →
     package properties don't explain the split → look for a state
     change between join events." Preflight matched nothing; the
     diagnostic was left entirely to the AI's intuition.
- **End-to-end contract that should own it**: three additions are needed:
  1. **Compound invariant in `invariants.yaml`**: `artifact.installable`
     requires both `publish_state=PUBLISHED` and `artifact_state=PUBLISHED`.
     Write-path coverage: `promoteToPublished` must appear in the
     required writers list for both.
  2. **Causal rule in `causal_rules.yaml`** (draft saved as
     `causal-rule-proposal-20260515T140617Z`): sequence
     "N-1 nodes converge fully → node-N joins → node-N partially
     converges → failing packages are not semantically distinct →
     look for a state change between join timestamps." Diagnosis:
     temporal cross-node comparison, not within-node package
     comparison.
  3. **Write-path coverage map primitive**: for every column that
     drives a node-agent install decision, record the set of functions
     that must write it. Scanner checks that each function in the
     required-writers set actually writes the column.
- **Did the fix simplify or special-case?** **Neither yet — recording
  the gap.** The causal rule is a saved draft; the compound invariant
  and write-path coverage map are not yet implemented. This entry is
  the evidence-of-one that licenses the work. A second incident on the
  "wrong diagnostic axis chosen because no causal rule named the
  pattern" shape is the trigger to implement the write-path coverage
  map as a first-class primitive.
  - Forbidden fix: when partial convergence appears across nodes in
    join order, do NOT begin diagnosis by comparing packages within the
    failing node. Begin by diffing repository/reconciler state at the
    two join timestamps.

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

---

## 2026-05-14 — Build-id orphaning + DNS profile-only publication

- **Shared concept**: the **4-layer model** (Repository / Desired / Installed
  / Runtime) — every reconciler, gate, and resolver must respect the same
  ordering and never collapse adjacent layers.
- **Per-subsystem interpretation**:
  - `repository.reachability_guard`: "reachable = installed-build_id OR
    inside retention window." Treated Desired as if collapsing into
    Installed — never checked `/globular/resources/*` for pinned build_ids
    before archiving.
  - `repository.resolver`: "if the manifest is not in the installable set,
    return codes.NotFound." Conflated "manifest absent" with "manifest
    explicitly demoted (YANKED / REVOKED / ARCHIVED)."
  - `node_agent.installer_api`: "if the repository call returns any error,
    try the local pinned tarball." Treated NotFound and Unreachable
    identically; would happily install a build the repository said "stop on."
  - `cluster_controller.dns_state`: "if a node has profile=gateway, publish
    it in gateway.<domain>." Read only Layer 2 (Desired/Profile). Ignored
    Layer 3 (Installed) and Layer 4 (Runtime).
  - `cluster_controller.handlers_node.cleanNodeFromReleases`: "when a node
    is removed, scan /globular/resources/ and rewrite status entries."
    Cascaded into a release purge with no reference safety check against
    other desired-state shapes.
- **Why unit tests missed it**: each subsystem owned its own slice of the
  4-layer story. The repository tests checked GC against installed
  build_ids; the resolver tests checked PUBLISHED-vs-YANKED; the DNS state
  tests built NodeInfo from profiles only. Nothing exercised the join:
  what happens when desired pins a build_id the repository has forgotten,
  or when a node's gateway is profile-true but installed-false.
- **End-to-end contract that should own it**: the **4-layer model itself**
  is the contract, expressed as four invariants:
  - `repository.purge_must_not_delete_active_desired_builds`
  - `repository.desired_build_id_must_resolve`
  - `repository.fallback_requires_manifest_and_checksum`
  - `dns.records_must_be_installed_and_runtime_healthy`
  Every reconciler / resolver / guard that takes inputs from one layer and
  produces outputs that depend on another MUST consult the relevant
  invariants. The doctor rules `repository.desired_build_ids_resolve` and
  `dns.records_match_runtime_health` give the contract a live enforcement
  arm. **Partially established** — invariants written, enforcement arms
  added to the three subsystems that produced this incident; future
  reconcilers will still need to be audited against the same checklist.
- **Did the fix simplify or special-case?** **Mostly simplified.**
  - Repository: extended `collectDesiredBuildIDs` to scan all four etcd
    prefixes (was only two), added a structured `PurgeBlockedReason`
    enum (was just a string), produced one shared safety contract that
    DeleteArtifact, SetArtifactState→REVOKED, and ArchiveUnreachableArtifacts
    all share. The merged installed+desired roots are computed once and
    reused.
  - Resolver: introduced a third outcome (FailedPrecondition +
    `DesiredBuildIdOrphaned` prefix) so the NotFound / Demoted / Reachable
    trichotomy is now structural rather than overloaded on one code.
  - Node-agent: introduced three sentinel errors
    (`ErrBuildIDOrphaned` / `ErrBuildIDNotFound` / `ErrRepositoryUnreachable`)
    so installer_api branches on `errors.Is`, not string-match. Fallback
    rules are stated in one place.
  - DNS: introduced `NodeInfo.ServiceReady` + `gateForService` so every
    record group runs the SAME funnel (Desired → Installed → Runtime). The
    candidate funnel is observable in logs; the per-group tests share one
    primitive. (Pool-based records still skip gating by design — pool
    membership is owned elsewhere; that asymmetry is documented in code.)
  - Consolidation candidate: **the 4-layer-model invariants in
    `docs/awareness/invariants.yaml`** should be linked from every new
    reconciler PR template. If a future reconciler reads from one layer
    and writes about another without invoking the relevant invariant, that
    is the next composed-path bug — and the next entry in this log will
    promote the candidate to a static check.

---

## 2026-05-14 — Doctor RepositoryBuildIDIndex admits demoted artifacts

- **Shared concept**: the repository's **install-eligibility predicate**
  (PUBLISHED+DEPRECATED only — `repopb.IsInstallableByPin`). Whenever any
  consumer needs to ask "is this build_id installable from the repository
  right now?", it must consult the same predicate that `resolveByBuildID`
  consults. Anything else is a private interpretation.
- **Per-subsystem interpretation**:
  - `repository.artifact_handlers.ListArtifacts`: "if the caller is admin,
    return every row regardless of state; otherwise hide rows where
    `IsDiscoveryHidden(state)` is true." Correct for catalog UX (admins
    need to see demoted rows in the dashboard); wrong as a source of
    truth for "is this installable?"
  - `cluster_doctor.collector.fetchRepositoryData` (v1.2.48): "the set of
    resolvable build_ids equals every build_id that ListArtifacts
    returns." Inherited the admin-visible row set unfiltered, so YANKED /
    REVOKED / ARCHIVED build_ids quietly entered
    `Snapshot.RepositoryBuildIDIndex`.
  - `cluster_doctor.rules.repositoryDesiredBuildIDsResolve` (v1.2.48):
    "a desired build_id is orphaned iff it is not in
    `RepositoryBuildIDIndex`." Correct rule, polluted input — orphans
    that the repository had already demoted (the real orphans, storage
    `801c0043-…` and node-agent `fe08cd6a-…`) looked resolved. The rule
    stayed silent on the very incidents v1.2.48 had shipped to catch.
- **The composition that failed**: ListArtifacts is a **display contract**
  (admins see everything for diagnostics), but the doctor used it as an
  **install-eligibility contract**. Two different audiences, one RPC.
  No filter at the consumer site → the display semantics leaked into the
  enforcement leg.
- **Why unit tests missed it**: the rule's unit tests
  (`repository_dns_invariants_test.go`) constructed Snapshots by hand and
  stamped `RepositoryBuildIDIndex` directly — they assumed the index was
  already filtered. No test exercised the collector path that builds
  the index from a ListArtifacts response. Live cluster was the first
  place these two ends met.
- **End-to-end contract that should own it**: any consumer that wants to
  ask "can the repository resolve this build_id for install?" MUST run
  the result of ListArtifacts through `repopb.IsInstallableByPin` (or call
  `ResolveArtifact` and look for FailedPrecondition / NotFound). The
  collector now does the former; this is documented in the helper
  `buildIDIndexFromManifests` and pinned by
  `repository_buildid_index_test.go` so the next consumer that wires
  ListArtifacts into a non-display use case will see the test as the
  contract and either reuse the helper or fail loudly on the YANKED case.
- **Did the fix simplify or special-case?** **Simplified.** A single
  predicate (`IsInstallableByPin`) now gates every install-eligibility
  question; the collector's row loop became a 4-line pure helper that
  the test imports directly. No new branch for the admin-vs-non-admin
  caller distinction — the consumer just filters regardless of who the
  RPC chose to show.
  - Consolidation candidate: **promote `IsInstallableByPin` to the
    canonical name used wherever code consults the repository for
    install eligibility.** `resolveByBuildID` re-implements the
    PUBLISHED-or-DEPRECATED test inline (`isRowInstallable` /
    `state == PUBLISHED && isInstallableForRef`); rolling those onto
    the named predicate would make the next drift (e.g., introducing a
    new lifecycle state) a one-line change across the whole tree.
