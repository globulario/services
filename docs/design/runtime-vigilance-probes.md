# Runtime Vigilance Probes & Degraded-Mode Diagnosis

> Status: PR-14 delivered 2026-06-20. PR-15 and PR-16 planned.
> Successor track to the [behavioral-memory runtime awareness backlog](behavioral-memory-runtime-awareness-backlog.md)
> (which delivered governed *observation ingestion*, PR-9..PR-13). That track gave
> behavioral-memory its riverbed — the afferent path and the evidence ladder. This
> track builds the rain gauges: it makes the system **detect** runtime condition
> classes autonomously, **diagnose** them even when the cluster is partially broken,
> and **prove** that a fix reached the live runtime.

## Why this track exists

The Scylla `group0` quorum loss is the canonical proof of the gap: it was found
manually through `infra_probe`, not by any automated watcher. Three reasons it
slipped through:

1. **`ai_watcher` was event-name driven.** It only sees named events someone chose
   to publish. A condition class nobody emits an event for is invisible to it.
2. **Diagnosis can depend on the very layer that is failing.** `cluster-doctor`
   reaches for gRPC services, the log service, etc. — exactly what breaks during an
   incident.
3. **A "fixed" bug can never reach production.** Source changes, but the generated
   artifact / package / installed config / running process may not — so the runtime
   keeps the old behavior.

Sequencing principle (deliberate): the system must become excellent at
**detect → diagnose → classify → prove → remember → propose** *before* any
auto-repair. Repair without release-boundary proof and degraded diagnosis is a
robot with a wrench in a fog machine — useful someday, dangerous today. No
auto-repair is in scope for this track.

## PR-14 — Runtime Vigilance Probes — ✅ DELIVERED

**Goal:** `ai_watcher` detects runtime *condition classes*, not only known event names.

### What landed

- **Probe framework** in `golang/ai_watcher/ai_watcher_server/probes.go`:
  - `Probe` interface (`Name`/`Component`/`Run`) and `ProbeResult` structured finding.
  - `startProbeLoop()` — a fixed-interval goroutine started from `StartService`,
    panic-isolated per probe, gated on `enabled && !paused`.
  - Emission is **rate-limited** per `(component, condition, severity)` over a
    5-minute window (`meta.diagnostic_output_must_be_bounded`).
  - Findings are recorded into behavioral-memory via
    `observation.FromWatcherProbe` (new constructor).
- **First probe — Scylla group0/quorum** in `probe_scylla_group0.go`:
  - **Sources the canonical truth plane** (node-agent `GetInfraProbe(scylladb)`)
    and emits a `DIAGNOSTIC_CLAIM` interpretation. It does **not** query Scylla
    directly — group0 truth is owned by the infra-probe truth plane, and a second
    querier would be a competing source of truth (a forbidden authority bypass).
  - Detects quorum loss two ways: (1) an explicit `group0`/`raft`/`quorum`
    truth-plane violation, and (2) membership-based majority math, only trusted at
    or above the ScyllaDB founding quorum (≥3) to avoid dev/single-node false
    positives.
  - Tool-acquire failure becomes an **indeterminate warning** finding — a tool
    failure is evidence, not silence.

### Authority model (the load-bearing decision)

A watcher probe enters the evidence ladder as `DIAGNOSTIC_CLAIM`
(`AUTOMATED_HEALTH` signal kind), **below** the `TRUTH_PLANE` `infra_probe` it
cites and at the same tier as `cluster-doctor` findings. The finding records the
`truth_plane_ref` it was derived from as evidence provenance. Ingestion produces a
`RAW_SIGNAL` + evidence only — never a Principle — so a probe finding **can never
auto-promote**.

### Known gap, surfaced as a governance candidate

The infra-probe scylladb runtime map exposes `cql_ready` / `gossip_live` /
`observed_peers` but **no explicit group0 voter count**. A group0 quorum loss that
still leaves CQL readable is therefore only *inferable* from membership shrinkage +
violations — which is precisely why it was missed. Every finding carries a
candidate invariant `scylladb.group0_voter_quorum_must_hold` and recommends making
the voter count first-class on the truth plane. **This is a candidate for human
review, not an auto-applied invariant.**

### Acceptance test (`probes_test.go`)

Given a simulated group0 quorum failure (CQL ready, zero live peers, critical
`scylla.group0.quorum_lost` violation): the probe emits an unhealthy structured
finding; the finding preserves source / authority / severity / evidence and cites
its truth-plane ref; behavioral-memory ingests it as a `RAW_SIGNAL` + evidence
(never promoted). Companion tests assert a healthy cluster is not emitted, an
acquire failure is indeterminate-warning, and below-quorum dev clusters do not
false-positive on membership math.

### Fix classification

`watcher_gap_repair` — the issue was missed by the watcher and watcher coverage was
added. Runtime verification is by simulation (acceptance test); live-cluster
verification and the truth-plane voter-count extension remain follow-ups.

### Follow-up probes (same framework, future PRs)

etcd (leader / alarms / NOSPACE / raft stall), Envoy (listeners / routes /
clusters), MinIO (pool / heal / drive state), gRPC reflection reachability, RBAC
expected-grant checks, generated policy/config freshness.

## PR-15 — cluster-doctor degraded-mode diagnosis — PLANNED

**Goal:** `cluster-doctor` stays useful when the cluster is partially broken.

Add a fallback path for each dependency it currently trusts:

| Normal mode | Degraded-mode fallback |
|-------------|------------------------|
| gRPC service calls | direct port checks |
| service-manager state | process table |
| log service | local log files |
| AWG graph service | embedded/local graph snapshot |
| Scylla service client | cqlsh / direct driver / probe |
| etcd service wrapper | etcdctl / raw endpoint probe |
| Envoy control plane | admin endpoint / config file |
| MinIO service API | local process / port / `mc` / admin probe |

A doctor that disappears during failure is itself a failure mode.

## PR-16 — Release Boundary Proof — PLANNED

**Goal:** no runtime repair is certified unless the changed artifact reaches the
live runtime.

Prove the chain: source changed → generated artifact rebuilt → package contains
artifact → installer/update path ships it → runtime loaded it → behavior changed.
Cover RBAC grants, policy rebuilds, config generation, service packaging, the
update path, and runtime load. Make the proof a repeatable command/test.

## What not to do

- No auto-repair in this track.
- Probes never query a system whose truth is owned by another authority — they
  source the owner (e.g. the infra-probe truth plane) and interpret it.
- Probe findings never auto-promote; promotion stays human-gated.
- Degraded-mode diagnosis must not rely on the layer it is diagnosing.
