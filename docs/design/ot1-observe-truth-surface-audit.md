# OT-1 — Observe-Truth Surface Audit

> **Status: DONE — this is the scoping deliverable for OT-2 / OT-3 / OT-4.**
> The read/observe-side mirror of [`rt1-direct-write-surface-audit.md`](rt1-direct-write-surface-audit.md).
> RT closed the write side: *every owner-owned mutation goes through a guarded,
> leader-fenced, identity-checked seam.* OT is the other half of the same truth
> model: **writes must go through owners; reads must tell the truth about owners.**

---

## 0. The model already exists — OT-1 audits its *coverage*, not its absence

Just as RT-1 found the owner-guard table and chokepoints already existed (the gap
was *coverage*, not *capability*), the observe-truth model is already present in
the awareness graph and partially implemented in `cluster_doctor`:

| id | class | status | what it requires |
|---|---|---|---|
| `meta.binding_outlives_evidence_until_invalidated` | invariant (meta) | **candidate** | every binding carries "I checked this *then*"; when *now ≠ then*, it is a phantom unless re-validated |
| `evidence.must_carry_provenance_and_trust_level` | invariant | active | evidence used for decisions carries source, writer, **timestamp**, and a classifier-derived trust level |
| `stale_evidence_must_not_authorize_remediation` | invariant | active | stale/untrusted evidence must not authorize privileged remediation |
| `doctor.evidence_trust_must_be_authoritative_for_execution` | invariant | active | stale/unverifiable evidence blocks autonomous remediation |
| `health.requires_fresh_evidence` | intent | **extracted_candidate** | health claims require freshness |
| `doctor.stale_evidence_authorizes_remediation` | failure_mode | critical | stale findings authorize action against already-recovered state |

The doctor implements the **trust gate** (`evidence_provenance.go`:
`findingEvidenceTrust` → `evidence.Classify`, "silence is not freshness") and it
is correctly wired into **every** remediation path (`handler_remediation.go:165`,
reached from both the public `ExecuteRemediation` RPC and the internal healer via
`gatedDispatcher.Dispatch`). The reduced-harvest-honesty norm exists. So the
machinery is there.

**The audit's finding is that the machinery is defeated in practice** — the exact
mirror of the RT-3 problem ("the owner-guard exists, but writes bypass it"). Here:
*the freshness gate exists, but the evidence lies about its freshness, so the gate
is blind.*

---

## 1. The central finding — the freshness gate is fed evidence that always claims "now"

`kvEvidence(service, rpc, kv)` — the helper **every** doctor rule uses to attach
evidence to a finding — stamps `Timestamp: timestamppb.Now()`
(`rules/invariant.go:107`), unconditionally. It never carries the time the data was
actually *collected*.

Consequence: `findingEvidenceTrust` classifies by `Timestamp` freshness, but every
piece of evidence presents the current instant, so **nothing is ever classified
stale**. This directly violates `meta.binding_outlives_evidence_until_invalidated`:
the "I checked this *then*" is overwritten with "now" before the gate ever sees it.

The worst case is Prometheus metrics: `snap.PromTS` records the real collection
time, the snapshot is cached for a TTL, but the rule's evidence is stamped `Now()`
— so a 5-minute-old metric (e.g. `controller_loop_heartbeat_age`) is presented to
the gate as AUTHORITATIVE/fresh. A finding (or its *absence*) can authorize
remediation against already-recovered state — `doctor.stale_evidence_authorizes_remediation`.

This is the OT analogue of the RT-3 funnel: a guard wired into every path is
worthless if the values flowing through it have been pre-laundered.

---

## 2. Surface A — the doctor (collectors + trust gate)

Each collector reads a source and contributes to the shared snapshot; rules read
the snapshot and emit findings. Classification of the **source truth** and whether
evidence carries real provenance:

| Collector | Source | Provenance | Observe-truth risk |
|---|---|---|---|
| `collector.go` ListNodes / GetClusterHealthV1 | live RPC → controller | source name only; **timestamp = Now()** | **HIGH** — on RPC failure `snap.Nodes`/`NodeHealths` are empty; rules read empty as "no nodes / healthy" (false-green), not "unknown" |
| node_agent RPCs (GetInventory, ListInstalled, GetSubsystemHealth, GetCertificateStatus, VerifyPackageIntegrity, …) | live per-node RPC | source `@nodeId`; **timestamp = Now()** | **HIGH** — a down agent yields a nil map entry; rules read "node has no subsystems" instead of "agent unreachable" |
| `prometheus.go` | HTTP → loopback Prometheus | `snap.PromTS` set correctly, **but rules re-stamp Now()** | **CRITICAL** — see §1; cached stale metric presented as fresh |
| etcd reads (objectstore/config, pki/ca, ingress, critical keys) | authoritative etcd | `snap.addError` on failure | **MEDIUM** — errors recorded in `snap.DataErrors`, but **rules never consult it** (documented KNOWN GAP, `collector.go:16-45`) |
| `release_boundary.go`, `gateway_backend_divergence.go`, `verification.go`, `repository_finding.go` | live RPC chains | report carries real provenance (GitSHA, ObservedAtUnix) **but evidence re-stamps Now()** | **MEDIUM** — true collection time is captured then discarded at evidence time |
| `sweep_requests.go` | etcd read+delete | targeting only, no finding evidence | LOW |

**Trust-gate coverage:** ✅ complete — all remediation converges on
`executeRemediationForFinding` where the gate runs. One inconsistency: **dry-run
bypasses the gate** (`handler_remediation.go:167` `!req.GetDryRun() && …`) —
informational, but an operator may execute after a preview validated on untrusted
evidence.

### Surface-A gaps (ranked)
1. **CRITICAL — `kvEvidence` stamps `Now()`** (§1). The gate cannot see staleness.
2. **CRITICAL — rules ignore `snap.DataErrors`/reduced-harvest.** `annotateForReducedHarvest`
   (`rules/registry.go`) only prepends a `[reduced-harvest]` label and appends a
   harvest-evidence entry — it does **not** downgrade an already-fired FAIL to
   UNKNOWN. A rule that concluded FAIL on a half-empty snapshot stays FAIL; a rule
   that read absence as health stays green. Verified.
3. **HIGH — absence read as data.** Empty collector maps (failed sub-fetch) are
   read by rules as authoritative "nothing there" — `meta.absence_scope_must_be_explicit`.
4. **HIGH — evidence source inferred from name strings.** `inferEvidenceSource`
   maps `(service, rpc)` strings to a source; a typo in a rule's `kvEvidence` call
   silently downgrades (or misattributes) trust on otherwise-fresh data.

---

## 3. Surface B — external read endpoints ("reads tell the truth about owners")

| Surface | file:line | Source class | Truth/staleness risk |
|---|---|---|---|
| `SaveServiceConfiguration` (desired + runtime) | `config/etcd_service_config.go:271-285` | **UNVALIDATED** — two non-transactional `Put`s | **CRITICAL (RT-adjacent)** — self-documented `// KNOWN GAP`: second Put fails → desired updated, runtime stale; readers + doctor see diverged state |
| `GetServicesConfigurations` | `config/service_config_cache.go` | **MIRROR** — 5s TTL, **60s stale-if-error** | **HIGH** — on an etcd hiccup, service discovery, xDS, file-service routing, **and the doctor** read stale address/port/state for up to 60s with no freshness signal |
| RBAC `GetResourcePermissions` | `rbac/.../rbac_permissions.go:744` | **MIRROR** — 30s cache, invalidate-on-write only | **MEDIUM** — access decisions on ≤30s-stale permissions; cross-instance writes never invalidate |
| `GetRuntime` (service status) | `config/etcd_runtime.go:42` | AUTHORITATIVE but `WithSerializable()` | **MEDIUM** — non-quorum read; a lagging follower returns stale status during leader election → doctor sees conflicting per-node truth |
| Cert / PKI reads | `config/config.go:816` | AUTHORITATIVE (filesystem) | **LOW** — disk is the source, but rotation may not sync to `/globular/pki/ca.crt` atomically |
| Public-dirs registry, file `ReadFile`/`GetFileInfo` | `public_dirs.go`, `file_ops.go` | AUTHORITATIVE (live etcd+watch / live FS) | LOW |

### Surface-B gaps (ranked)
1. **CRITICAL — desired/runtime non-transactional write** (already has a `RunTxnWithClass`
   primitive from RT-3 #117 ready to fix it).
2. **HIGH — config-cache 60s stale-if-error window served without a freshness signal**,
   and **the doctor reads from this mirror** — compounding §1: a stale config mirror
   feeds a snapshot whose evidence then claims `Now()`.
3. **MEDIUM — RBAC cache has no cross-instance invalidation.**
4. **MEDIUM — `WithSerializable()` runtime reads can serve a stale follower view.**

---

## 4. Scoping → OT-2 / OT-3 / OT-4

### OT-2 — make the doctor's evidence tell the truth about its own freshness (S/M)
The highest-leverage move — it re-arms the gate that already exists:
1. ✅ **Evidence carries real collection time (#125).** Rather than thread a
   timestamp through 150+ `kvEvidence` call sites, a single post-pass
   `stampEvidenceCollectionTime` (in `rules/registry.go`, alongside
   `annotateForReducedHarvest`) corrects every finding's evidence `Timestamp` from
   the rule's `Now()` to the snapshot's `GeneratedAt` (and `PromTS` for prometheus
   evidence, which is older). Fail-safe: only ever moves a timestamp *backward*.
   This makes `findingEvidenceTrust` able to classify staleness — especially for a
   cached snapshot re-evaluated long after collection.
2. **Rules consult reduced-harvest before concluding.** When `snap.DataIncomplete`
   /a depended-on source is in `snap.DataErrors`, the finding must be **UNKNOWN**,
   not FAIL/green — i.e. `annotateForReducedHarvest` (or the registry evaluator)
   must *downgrade*, not merely label. Mirrors the reduced-harvest-honesty norm.
3. **Absence ≠ negative.** A rule reading an empty collector map for a source that
   `addError`'d must emit UNKNOWN for that scope (`meta.absence_scope_must_be_explicit`).

### OT-3 — read-endpoint freshness contracts (M)
1. ✅ **Atomic desired+runtime (#127)** — `SaveServiceConfiguration` now writes the
   desired + runtime keys in one `config.RunTxnWithClass` transaction (the RT-3 Txn
   primitive), both or neither. Closes the self-documented KNOWN GAP and the
   diverged-state false-diagnosis. A direct dividend of the write-governance work.
2. **Freshness signal on the config mirror.**
   - ✅ **Exposure primitive (#128).** `depcache.Cache.LastFetchedAt(key)` exposes the
     last *successful* fetch time (which does not advance on a stale-serve), and
     `config.ServiceConfigCacheLastFresh()` surfaces it for the service-config cache —
     so a consumer can tell that `GetServicesConfigurations` returned stale-if-error
     data even though its error is nil.
   - ✅ **Consumer (#129).** `serviceConfigCacheFresh` doctor rule reads
     `ServiceConfigCacheLastFresh` and emits a `SEVERITY_WARN` finding when the
     doctor's own config mirror hasn't refreshed within the staleness threshold —
     so the doctor reports when its config view is stale instead of diagnosing
     against it. (xDS treating a stale mirror as degraded remains a follow-up.)
3. ⚠️ **~~Strongly-consistent reads for doctor-critical paths~~ — RETRACTED.** On
   verification, `config.GetRuntime` (the `WithSerializable` read) is called by
   `process.go` and `PutRuntime`, **not** the doctor; the doctor's `GetRuntime()`
   calls are proto accessors on `InfraProbeResult` (a different function). So its
   consistency level is not a doctor-truth issue and this item does not hold — the
   audit's original Surface-B line for it overstated the risk.
4. **RBAC cross-instance cache invalidation** (event/watch-based).

### OT-4 — promote the principles + ratchet (S)
1. ✅ **Ratchet (#126).** `TestEvaluateAll_StampsEvidenceWithCollectionTime`: a
   synthetic invariant emits `kvEvidence(Now())` evidence, run through the real
   `EvaluateAll` against an old-`GeneratedAt` snapshot; the test asserts the evidence
   comes back stamped with the collection time, not `Now()`. Removing or bypassing
   `stampEvidenceCollectionTime` makes it fail (proven non-vacuous). This locks in the
   OT-2 fix end-to-end, mirroring the RT-3 capstone ratchet.
2. ⬜ **Promote the two candidate principles** — RECOMMENDED, belongs in the
   awareness-graph repo (not done here):
   - `meta.binding_outlives_evidence_until_invalidated` (candidate→active) — in
     `awareness-graph/docs/awareness/generic/state_authority_invariants.yaml`.
   - `health.requires_fresh_evidence` (extracted_candidate→active) — graph intent.
   The evidence for promotion now exists (this audit + the OT-2 implementation + the
   OT-4 ratchet). Promotion is deferred to its own awareness-graph PR because it is
   cross-repo, requires the embeddata rebuild, and turns the principle into an
   enforced/coverage-gated rule — it should not ride on a services change. The
   behavioral half, required test `test.doctor_evidence_stale_blocks_execution`
   (evidence age > `MaxEvidenceAge` blocks execution), pairs with the static ratchet
   above on promotion.

### Priority
**OT-2 first** (re-arms the freshness gate — highest leverage, contained to the
doctor), then **OT-3** (read-endpoint freshness, partly free via the RT-3 Txn
primitive), then **OT-4** (promote + ratchet, locks it in). This is the same
shape RT followed: fix the seam, then close the surfaces, then ratchet.

---

## 5. One-line close

RT stopped unsafe hands. OT must stop lying eyes: the doctor's freshness gate is
present and fully wired, but the evidence is pre-stamped "now" and partial
harvests read as green — so the system can currently be safe without being able to
*report* its safety honestly. OT-2 re-arms the gate it already built.
