# Cluster Formation and Node Join — The Whole Dynamic

This page explains **what actually happens when a node joins a Globular cluster** — end to
end. It covers the control plane, the service mesh, leader election, and each infrastructure
pillar (etcd, ScyllaDB, MinIO, Envoy): how they are exposed, what they require, how they are
configured, and how they come to life as a cluster grows from **1 → 2 → 3 nodes** into a
highly-available (HA) control plane.

If you just want the operator commands, see [Adding Nodes](adding-nodes.md). This page is the
*model underneath* those commands — read it once and the rest of the platform stops looking
like magic.

> **One sentence to hold onto:** a joining node earns trust and capability in strict phases —
> it is a *guest* that reads the cluster's truth from the cluster's authority, executes work
> locally, and is only promoted to a full voting participant once the cluster can stay
> highly-available *with* it. Nothing about a join is instantaneous, and nothing lets an
> unproven node speak for the cluster.

---

## 1. The mental model: one logical cluster, four planes

A Globular cluster is a set of Linux nodes, each running a **node-agent**. There is exactly
**one logical cluster**, not a federation of independent machines. That single logical cluster
is made of four cooperating planes:

| Plane | What it owns | Made HA by |
|-------|--------------|------------|
| **Control plane** | *Decisions* — desired state, join lifecycle, reconciliation, workflows | `cluster-controller` with **etcd-lease leader election** (one leader writes) |
| **State plane** | The *source of truth* — all config, membership, service registry | **etcd** (Raft quorum of voters) |
| **Storage plane** | *Data* — the package index / metadata (ScyllaDB) and secondary user data (MinIO) | ScyllaDB gossip replication; MinIO erasure coding |
| **Mesh plane** | *Connectivity* — how services find and reach each other | **Envoy** data plane + **xDS** control plane, mTLS, cluster DNS |

Two principles govern how these planes treat a joining node, and they are worth internalizing
because every phase below is an expression of them:

- **The controller DECIDES, the node-agent EXECUTES.** The controller never runs `systemctl`
  or `os/exec`; it computes desired state and dispatches. The node-agent is the only component
  that touches the OS. Execution is *already local* to each node — what crosses the network is
  reading truth (from etcd), pulling packages (from a node that has them), and reporting status.
- **Trust and capability are earned, not granted at contact.** A node that just ran the join
  script cannot vote in etcd, cannot host replicated data, and cannot claim it makes the cluster
  HA — until it has *proven* each of those, phase by phase.

---

## 2. The players and their ports

Everything below is resolved from etcd at runtime — **do not hardcode service ports.** The only
fixed bootstrap ports are the two a node needs *before* etcd is reachable.

| Component | Port(s) | TLS | Notes |
|-----------|---------|-----|-------|
| **cluster-controller** | `12000` (fixed bootstrap) | mTLS | Control-plane gRPC. Leader-elected. |
| **node-agent** | `11000` (fixed bootstrap) | mTLS | Per-node executor. The only OS-touching component. |
| **etcd** | client `2379`, peer `2380` | mTLS **mandatory** | State plane. `listen` on `0.0.0.0`, `advertise` the routable IP. |
| **ScyllaDB** | CQL `9042`, REST admin `10000` | *plaintext today* | Package index / metadata store. Inter-node gossip on Scylla defaults. |
| **MinIO** | API `9000`, console `9001` | https endpoints | Secondary user data only — **never packages**. Commodity, not a pillar. |
| **Envoy** | ingress `443` → gateway `8443` | mTLS end-to-end | Per-node data plane. Admin `9901`. |
| **xDS** | `18000` | mTLS (secure by default) | Envoy config control plane. Reads service registry from etcd. |
| **gateway** | `8443` (HTTPS), `8080` (HTTP) | mTLS | External edge; serves `/join`, `/clean`, `/sign_ca_certificate`. |
| **DNS** | `53` | — | Authoritative for `*.globular.internal`. |

> **Ports are protocol definitions, not config.** `443`, `53`, `2379` are fixed by protocol.
> Every *gRPC service* port (10xxx range) is an etcd-resolved runtime attribute — query
> `service_config_list`, never assume.

---

## 3. Founding quorum: why the first three nodes are special

The **first three nodes of any cluster MUST carry all three founding profiles**:
`core`, `control-plane`, `storage`. This is enforced at join time (`enforceFoundingProfiles`,
`profiles_normalize.go`) and cannot be bypassed. `MinQuorumNodes = 3`.

Why three, and why all three profiles? Because the infrastructure pillars each need a quorum,
and their quorums *cascade* if unmet:

- **etcd** runs on all nodes, but a *highly-available* etcd needs **3 voters** (survives losing
  one). One voter = single node. Two voters = *worse than one* (lose either and quorum is gone).
- **ScyllaDB** needs **3 nodes** to reach replication factor 3 (survives losing one replica).
- **MinIO** needs **3 storage nodes** for erasure-coding quorum.

Miss this on the founding nodes and you get *silent single points of failure that cascade*:
workflows fail → publishing fails → artifacts stay unverified → the reconciler can't find them
→ services never converge. The founding-quorum rule exists so the cluster is *born* able to
become HA, not retrofitted into it.

Nodes 4, 5, … are freer: they can carry workload-only profiles and do **not** need to join etcd,
ScyllaDB, or MinIO as members — they run application services and report status.

---

## 4. The control plane and leader election

### cluster-controller — the brain that never touches the OS

`cluster-controller` runs on **every control-plane node**, but only **one is the leader** at a
time. Leadership is an **etcd-lease election** (`leader_election.go`): each instance opens an
etcd concurrency session (≈15 s lease) and campaigns for the key
`/globular/clustercontroller/leader`. The winner:

1. Reloads all state from etcd (nothing authoritative lives only in memory).
2. Increments a **fencing epoch** to lock out any stale prior leader.
3. Seeds the core workflows and starts the reconciler.
4. Publishes its address bound to the lease so nodes can find it.

**Only the leader writes desired state, runs the reconciler, and seeds workflows**
(`intent:controller.leader_election_gates_all_writes`). The reconcile loop literally skips every
tick unless `isLeader()`. Followers accept requests but defer to the leader. If the leader dies,
its lease expires (5–30 s), a follower wins the next campaign, loads state from etcd, and
resumes — in-flight workflows are unaffected because the workflow engine is independent of
controller leadership.

A **liveness watchdog** guards against a *zombie leader* (holds the lease but stopped
processing): if it makes no progress for a threshold, it resigns the lease so a healthy standby
takes over.

### How a joining node reaches the control plane

The node's signed **join plan** carries `BootstrapEndpoints = <clusterDomain>:12000` — the
controller's fixed bootstrap port. The joiner talks to *the cluster's* controller (the leader,
discovered via etcd), **not** a local copy — during early bootstrap it doesn't *have* a local
controller yet. This is the general rule for a joiner: it uses the **cluster's** control plane,
repository, and etcd authority until it has installed and been promoted into its own.

---

## 5. The infrastructure pillars

The pillars are **etcd, ScyllaDB, and Envoy**. MinIO is deliberately *not* a pillar — it is a
commodity tier (more below). For each pillar: what it is, how it's exposed, what it requires,
how it's configured, and — the interesting part — *how a new node joins it*.

### 5.1 etcd — the state plane and the heart of the join

**Role.** etcd is the single source of truth: all config, membership, desired state, and the
service registry live here. Every other subsystem reads its truth from etcd.

**Exposure & config.** Client `2379`, peer `2380`, **mTLS mandatory** on both. The join script
renders `/var/lib/globular/config/etcd.yaml`: `listen-*-urls` bind `0.0.0.0`, `advertise-*-urls`
use the node's routable IP, `initial-cluster-state: existing` (a joiner *joins*, it does not
bootstrap a new cluster), and the cluster token is the immutable `globular-etcd-cluster`.

**How a node joins etcd — learner-first (Policy A′).** This is the single most important
mechanic on the page, and it's why a two-node cluster behaves the way it does.

A new etcd member is added as a **non-voting learner**, not a full voter. The day-1 add is done
**client-side by the join script** (`etcdctl member add --learner`), precisely so a slow or
failed join can *never* change the existing cluster's quorum:

- Adding a full voter to a 1-node cluster raises quorum to **2 of 2**. If the joiner then dies,
  the founder is stranded below quorum — the cluster freezes. A **learner never counts toward
  quorum**, so it is always safe to add.
- etcd 3.5 permits **only one learner at a time**; the join script waits for that slot.

A learner **replicates the Raft log but refuses client RPCs** ("rpc not supported for learner").
It is a catching-up follower, not yet an authority.

**Promotion to voter — Policy A′ (`topologyAllowsLearnerPromotion`).** The controller (the
leader) promotes a caught-up learner to a voter **only when doing so grows toward a genuinely HA
target of ≥ 3 voters**:

```
promote only if:  learners > 0  AND  target >= 3  AND  voters < target
```

The `target` is the live count of admitted etcd-profile nodes. So:

- **1 node:** 1 voter, target 1 — no HA, nothing to promote.
- **2 nodes:** 1 voter + 1 learner, target 2 — **`target < 3`, so the learner is NOT
  promoted.** It stays a learner. A settled **2-voter cluster is forbidden**
  (`invariant:infra.etcd.two_voter_topology_is_not_ha`) because losing *either* voter destroys
  quorum — strictly worse than a single node.
- **3 nodes:** target 3 — now promotion proceeds. Because etcd caps learners at one, the cluster
  reaches 3 voters *sequentially*, passing **through** a transient 2-voter state that is
  immediately driven onward:

  ```
  1v → +learner → promote → 2v (transient) → +learner → promote → 3v (HA)
  ```

  The rule forbids 2 voters as a *final* state, never as a transitional step on the way to 3.

**You cannot reach 3 voters without passing through 2 — but you must never *stop* at 2.** That
is Policy A′ in one line.

**How clients find etcd (and the buildout subtlety).** Services resolve etcd from
`/var/lib/globular/config/etcd_endpoints` (a file, one `https://IP:2379` per line), which the
controller keeps in sync from the etcd key `/globular/system/etcd_endpoints`. Environment
variables are never consulted.

There is a chicken-and-egg trap during buildout: while a node's *own* etcd member is a learner,
its local etcd endpoint refuses RPCs — so the node-agent must talk to a **voter**, not itself.
It does this by asking the controller (over the trusted `:12000` join-plan endpoint) for the
authoritative **voter** endpoints via the `GetEtcdVoterEndpoints` RPC, pinning the endpoint file
to them, and disabling client AutoSync (otherwise a `MemberList` against the voter would re-add
the local learner and re-break the client). If that fetch fails, the node stays **pending with a
precise reason — it never guesses endpoints**. Once the node is promoted to a voter, the
endpoint file is restored to the full membership and AutoSync resumes. This keeps a joining node
converging its full service stack *before* it becomes a voter, in an explicit, non-HA,
clearly-reported state — never a silent one.

### 5.2 ScyllaDB — the package index / metadata store

**Role.** ScyllaDB is the durable index behind the repository and other metadata. It is a
pillar: a primary service (e.g. RBAC) treats *ScyllaDB*, not MinIO, as its data authority.

**Exposure & config.** CQL `9042`, REST admin API `10000`. The controller renders
`/etc/scylla/scylla.yaml` with `cluster_name` (required by ScyllaDB 2025.3+ Raft topology on
*all* nodes), and `listen/rpc/broadcast` addresses all set to the node's **routable IP** — never
`0.0.0.0` (Scylla can't listen on a wildcard). Today the renderer emits **plaintext** inter-node
and CQL traffic — TLS on `7000/7001` is not configured.

**How a node joins ScyllaDB — native gossip, controller-coordinated.** Unlike etcd, there is
**no explicit member-add**. ScyllaDB uses gossip-based ring discovery: the controller renders a
`seeds:` list containing all storage-profile node IPs (including the joiner's own), and when
Scylla starts, it gossips to the seeds, streams data, and joins the ring. The controller's role
is *config + verification*, not membership surgery:

```
prepared → configured → started → verified
```

A joining node is only `member_ready` when its REST API reports `operation_mode = NORMAL` with
observed peers. A node that comes up CQL-ready but sees *zero* peers is flagged as an isolated
single-node ring (a stall), not a success. The controller can unstick failures — restart a stuck
unit, wipe `/var/lib/scylla/data` for a never-verified Raft join, or enqueue removal of a failed
fresh-join candidate.

**Replication factor rises with verified nodes.** RF climbs `1 → 2 → 3` and **caps at 3** as
storage nodes become verified-eligible. The schema guard issues `ALTER KEYSPACE … RF` on the
critical keyspaces and logs a repair reminder (`nodetool repair`). A node only counts toward the
RF tally once it is **verified storage-eligible** (`IsNodeVerifiedStorageEligible`) — which
requires it to have reached `workload_ready` (or `storage_joining`) and to be a healthy,
admitted, RF-eligible member. This is why RF doesn't jump the instant a node appears — it waits
for the node to *prove* it can hold a replica.

### 5.3 MinIO — the commodity object store (explicitly *not* a pillar)

**Role.** MinIO stores **secondary user data only** (files, search indexes). It does **not**
store packages — packages live on the POSIX filesystem (see §7). MinIO is a **commodity tier**,
governed by `invariant:minio.is_commodity_not_a_pillar`: it must never gate a primary service's
health, and an inactive MinIO must never block node convergence. The pillars are etcd, ScyllaDB,
and Envoy — MinIO is not among them.

**Exposure.** API `9000`, console `9001`, https endpoints.

**How a node joins MinIO — a dedicated topology workflow, not the normal reconciler.** MinIO is
marked `install_mode = topology_workflow`. At node-join time it is deliberately **held** — the
normal materializer skips it (you'll see `skipping minio … managed by dedicated workflow` in the
controller log). A separate leader-only `minioTopologyReconciler` owns it, and it **refuses to
act below 3 storage nodes** (`SKIP_NO_QUORUM`). Including MinIO in the per-node join path would
instantly fail the join mesh on every new node — hence the separation.

**Standalone vs distributed, and degraded policy.** With ≥ 2 pool nodes and a durable storage
policy, MinIO runs **distributed** (erasure-coded). Under a declared **degraded** storage policy
it is forced **standalone** on each storage node — a 1- or 2-node distributed pool is
split-brain-prone, so the policy floor wins. The node-agent renders `minio.env` and the systemd
`distributed.conf` from etcd's `ObjectStoreDesiredState` (byte-identical to what the controller
would render), enforces topology membership (non-members are stopped, never wiped), and **never
restarts MinIO on its own** — cluster-level start/stop is workflow-coordinated.

### 5.4 Envoy + xDS — the service mesh data and control planes

**Role.** Envoy is the per-node data plane (`globular-envoy.service`); **xDS** is its control
plane — a standalone service that builds Envoy's routing config from etcd and pushes it to every
node's Envoy.

**Exposure.** xDS gRPC on `18000`, Envoy admin `9901`, Envoy ingress listener on `443`
re-encrypting to the gateway backend on `8443` (end-to-end mTLS). xDS is secure-by-default mTLS.

**How the mesh learns about a new node's services.** When a service starts on the joined node, it
registers its instance in etcd at `/globular/services/{id}/instances/{node}` with its routable
`IP:port` (loopback is hard-rejected — a service may not register `127.0.0.1`). xDS watches the
etcd service registry, rebuilds an Envoy snapshot **only when the content actually changes**
(content-addressed by SHA-256, to avoid churning live connections), and pushes it out. Envoy
routes inter-service gRPC by **path prefix** (`/<service.FullName>/` → that service's cluster),
so any node's Envoy can serve any service. New endpoints simply join their service's cluster and
become reachable — see §6 for the full discovery path.

---

## 6. The service mesh: how services find and reach each other

Service discovery is **etcd-first, mesh-routed, with a direct fallback**:

1. **Register.** Each running service writes `/globular/services/{id}/config` and
   `/globular/services/{id}/instances/{node}` with its routable address. No hardcoded addresses,
   never `localhost`.
2. **Resolve.** A caller resolves the target from etcd (falling back to the gateway's `/config`
   endpoint if needed).
3. **Mesh-route.** Resolved `host:port` is rewritten to `host:443` so the call goes **through
   Envoy** — the "mesh-routable" path. The client TLS-probes Envoy on `:443` (cached ~10 s); if
   Envoy is unreachable it logs `mesh: Envoy unreachable, falling back to direct connections` and
   dials the raw etcd `host:port` directly.
4. **Control-plane bypass.** Traffic that must not depend on mesh health (controller, Tier-0
   infra) uses *direct* resolution that skips the `:443` rewrite.

### mTLS and the cluster CA

All inter-service gRPC is **mutual TLS anchored on the cluster CA** at
`/var/lib/globular/pki/ca.crt` (with `ca.pem` as its bundle copy). Every node presents a service
identity at `/var/lib/globular/pki/issued/services/service.{crt,key}`.

A **joining node gets its certificate at join time**: it generates a keypair + CSR and POSTs the
CSR to the gateway's `/sign_ca_certificate` endpoint over a connection pinned to the cluster CA
(which is embedded in the join script and installed into the system trust store). The node never
sees the CA private key — it receives a signed leaf. This is how a brand-new machine becomes a
trusted mTLS peer without ever holding the root.

### DNS and the bootstrap boundary

The cluster's authoritative domain is `globular.internal` (DNS on `:53`). Nodes resolve each
other by name via a cluster resolver whose DNS-daemon IPs are stored **in etcd**
(`/globular/cluster/dns/hosts`). Critically, **Tier-0 services (etcd, ScyllaDB, DNS itself)
resolve from etcd first**, never depending on the DNS daemon being up —
`recovery.must_not_depend_on_dns_only`. DNS can't be resolved via DNS, so its locations live in
the state plane.

### The external edge: VIP → Envoy → gateway → service

External traffic enters through a **keepalived VIP** that floats among control-plane nodes
(spec-driven, written by the controller; e.g. the reference cluster uses `10.0.0.100`). The node
holding the VIP answers `:443` (Envoy), which re-encrypts to the gateway on `:8443`, which serves
the HTTP surface and proxies gRPC-Web into the mesh. keepalived only hands a node the VIP when
its Envoy/gateway is actually listening, so the edge never points at a dead node.

---

## 7. Where packages live (and where they do *not*)

A recurring source of confusion, stated plainly:

- **Published artifacts** live in the repository service's **POSIX CAS** at
  `/var/lib/globular/repository`.
- **Per-node install cache** (archives staged before/at install) lives at
  `/var/lib/globular/packages/`.
- **The package index / metadata** lives in **ScyllaDB**.
- **Packages are NEVER in MinIO.** MinIO is secondary user data only. Bytes in MinIO are never a
  valid recovery source for a package. Do not look there for packages, certificates, or
  workflows.

---

## 8. The join sequence — bootstrap phases

Once a node is admitted, the controller drives it through a **bootstrap finite-state machine**.
Each phase has a gate; a phase is *skipped* if the node's profiles don't include the relevant
service. The node is not eligible to converge its **application tier** until it reaches
`workload_ready` (or `storage_joining`).

| # | Phase | Gate to advance |
|---|-------|-----------------|
| 0 | `admitted` | Trust established, node-agent running — advances immediately |
| 1 | `infra_preparing` | Infra packages installing |
| 2 | `etcd_joining` | etcd unit present; wait for etcd join to verify **or** proceed as a healthy learner (see below) |
| 3 | `etcd_ready` | etcd verified voter, **or** healthy learner deferring to a healthy voter |
| 4 | `xds_ready` | `globular-xds.service` active |
| 5 | `envoy_ready` | `globular-envoy.service` active **and** required infra runtime converged |
| 6 | `awareness_ready` | awareness bundle installed (or 90 s timeout) |
| 7 | `workload_ready` | app-tier convergence unlocked |
| 8 | `storage_joining` | (storage nodes) MinIO/Scylla join verified → then `workload_ready` |

**The `etcd_joining` gate is where buildout policy lives.** Three outcomes:

- **Verified voter** → advance to `etcd_ready` normally.
- **Healthy learner + a healthy voter to defer to** → advance to `etcd_ready` as an **explicit
  non-voter**, marked `etcd_ha=false`. This is a *bootstrap-progression* exception, **not** a
  promotion — the node stays a learner (Policy A′ still refuses a 2-voter promotion), and the
  not-HA state is **surfaced, never hidden**. This is what lets node 2 of a planned cluster
  install its full service stack *before* node 3 arrives, without an operator having to declare
  anything.
- **Learner with no healthy voter yet** → wait (never fail on timeout — its etcd is a functional
  member).

Because bootstrap is *not steady state*, a node building toward HA is not treated with
steady-state quorum rules. It proceeds as an honest non-voter and is promoted the moment a third
node makes a 3-voter target reachable.

---

## 9. Worked example: forming a 3-node HA cluster

### Node 1 — day-0 bootstrap (the founder)

`install.sh` brings up a single node that bootstraps the whole cluster. Its node-agent creates
the local etcd (a lone **voter**, target 1 — not HA, and honestly reported as such), starts the
controller (which wins its own election), signs certs locally (it holds the CA at day-0), and
converges all founding-profile services. Result: a fully-working **1-node cluster** with etcd,
ScyllaDB (RF 1), MinIO (standalone), Envoy, and the control plane — but **no HA**.

### Node 2 — the learner that converges but does not vote

Run the join script against node 1. Node 2:

1. Installs the cluster CA and gets its service cert signed via `/sign_ca_certificate`.
2. Joins etcd as a **non-voting learner** (`member add --learner`) — quorum stays at 1, so node
   1 is never endangered.
3. Its node-agent, seeing its own etcd is a learner, fetches **voter** endpoints from the
   controller and reads desired state from **node 1's** etcd.
4. Walks the bootstrap FSM: `etcd_joining` → (healthy learner + healthy voter) → `etcd_ready` →
   `xds_ready` → `envoy_ready` → `awareness_ready` → `workload_ready`.
5. Converges its **full service stack** — as an explicit **non-voter**. ScyllaDB RF rises toward
   2; MinIO stays held (still below 3-node quorum). etcd remains **1 voter + 1 learner**;
   `topologyAllowsLearnerPromotion` refuses to promote (target 2 < 3). Health honestly reports
   `etcd_degraded` / `etcd_ha=false`.

This is the deliberate, correct state: **node 2 is fully useful, but the cluster is not yet HA**
and never pretends to be.

### Node 3 — the promotion to genuine HA

Join node 3. Now the etcd-node target becomes **3**, and Policy A′ engages:

```
1v + learner(n2)  → promote n2 → 2v (transient)
2v + learner(n3)  → promote n3 → 3v  ← HA
```

etcd is now a **3-voter quorum** (survives losing one). ScyllaDB reaches **RF 3** (the schema
guard ALTERs the keyspaces; run `nodetool repair`). The MinIO topology workflow sees 3 storage
nodes and builds the **distributed erasure-coded** pool. Every service is registered in the mesh
and reachable via Envoy. The cluster is now **genuinely highly available** — and only now does
health report it as such.

**The whole arc in one line:** *you cannot reach 3 voters without passing through 2 — so the
platform passes through 2 deliberately and quickly, converging real work the whole way, and only
claims HA when it is actually HA.*

---

## 10. Degraded / small-cluster operation (explicit, never implicit)

Sometimes you genuinely want a 1- or 2-node cluster to run its full stack *permanently* (a lab,
an edge box). That is a **declared** choice, never inferred from node count. Declare a storage
policy via the owner RPC:

```bash
globular cluster storage policy set --profile two_node_degraded --allow-degraded --reason "2-node lab"
```

This sets RF 2 and MinIO standalone, and marks the cluster degraded **honestly** — health
reports it, the doctor reports it, and nothing pretends the cluster is HA
(`intent:degraded_is_explicit_not_hidden`). etcd still refuses to auto-promote into a 2-voter
trap; the second node runs as a declared non-voter. Revert with `--profile durable` once you add
a third node.

The distinction that matters: **buildout is transient and needs no declaration** (a node on its
way to HA just proceeds as an honest non-voter); **steady-state degraded operation is a
permanent choice and must be declared.** Both surface the not-HA condition — neither hides it.

---

## What's next

- [Adding Nodes](adding-nodes.md) — the operator commands for join/approve/remove.
- [High Availability](high-availability.md) — leader election, quorum, failover behavior.
- [Convergence Model](convergence-model.md) — how drift is detected and reconciled.
- [Network and Routing](network-and-routing.md) — Envoy, xDS, and the mesh in depth.
