# Cluster identity: minted UUID, decoupled from the domain

Status: DESIGN (Phase 1 of the identity-deviation program)
Author: 2026-07-05 session
Principle: dda8d669 — an entity identity must be an opaque, immutable token minted
once by its owning authority and read-through everywhere, never derived from a
mutable attribute. Audit inventory: ai-memory architecture `d34edd34`.

## Problem

`cluster_id` is the cluster's **domain** (`security/cluster.go:27` — "For now, use
domain as cluster ID"). The domain is a mutable config attribute (`config.json`
`Domain`); using it as the membership identity means:
- Identity changes if the domain is ever re-set.
- Consumers **derive** it locally (`config.GetDomain()`) and paper over its absence
  with silent fallbacks (`"globular.internal"`) and `if id != "" {}` skips — the
  shortcut hides the missing authority.
- `cluster_controller/state.go:665` actively **coerces** any UUID `ClusterId` back
  to a domain (`!isDomainLike` → overwrite). A minted UUID can't survive a restart
  today. **This is the first blocker.**

## Key distinction (the whole design)

The codebase conflates two concepts under "cluster_id":

| Concept | Should be | Where it's used |
|---|---|---|
| **(a) membership identity** | **minted UUID** (this migration) | interceptor validation, JWT `cluster_id` claim, join gate, service-to-service `cluster_id` metadata |
| **(b) DNS / storage domain** | **stays the domain** (unchanged) | FQDNs, DNS reconciler, `ClusterDomain`, **ScyllaDB workflow partition key (~20 tables)**, scylla-manager token seed `sha256(domain\|caHash)`, etcd cluster token (already a decoupled constant) |

Only (a) becomes the UUID. (b) must remain the domain — changing the Scylla
partition key would orphan all workflow history; that is out of scope and forbidden
here.

## Contract: `/globular/system/cluster/id` (canonical membership identity)

Explicit, and a future ratchet rule:

1. The value is the **canonical cluster membership UUID** — opaque, immutable.
2. The **cluster-controller is the sole writer**. It mints exactly once at Day-0.
3. After the initial mint, **any attempt to overwrite it with a different value is
   refused / fatal** — an established identity never changes under the cluster.
4. **Readers must not synthesize, derive, coerce, or default** it. Where identity is
   required, absence/corruption is **fail-closed** — never the domain, never
   `"globular.internal"`.
5. It is **not** derived from the domain, MAC, hostname, or any other attribute.

Enforced in code today: `config.cluster_membership_id.go` (reader fail-closed +
contract doc), `day0_seed.go:ensureClusterMembershipID` (mint-once, never
overwrite), `config.CriticalKeyPolicies` (owner = cluster-controller,
delete = never_automatic). A future ratchet rule should assert (a) no writer other
than the controller Day-0 seed, and (b) no reader path that falls back to the
domain on `ErrClusterMembershipIDAbsent`.

## Authority & data model

- **Authority:** the cluster-controller mints the UUID **once** at Day-0 init and is
  its sole owner.
- **Store:** one owned etcd key, e.g. `/globular/system/cluster/id` (immutable after
  first write; guarded like other critical keys — `config.CriticalKeyPolicies`).
- **Distribution:** carried in the signed JoinPlan so joining nodes receive the
  authoritative id (they never mint their own).
- **Local read:** `security.GetLocalClusterID()` becomes a **reader** of the
  persisted membership id (cached from etcd / node state), returning an explicit
  error when absent — **no domain derivation, no `"globular.internal"` fallback, no
  silent skip.** A missing membership id is fail-closed, surfaced, never invented.

## Rollout decision: UUID-only, cluster rebuilt Day-0 (no dual-accept)

**Decision (2026-07-05):** correctness over backward-compatibility. The dev cluster
will be wiped and re-Day-0'd, which mints the UUID by construction, so we do **not**
carry a dual-accept window that keeps the domain as an identity credential. On a
fresh Day-0 the ordering (mint → emit → validate) holds by construction; JWT
issuance already omits the claim gracefully pre-mint.

Landed (Phase-1 + this step):
- Membership validation is **UUID-only, fail-closed** — `security.ValidateClusterMembership`
  accepts iff `cluster_uid` == the local minted UUID; empty/mismatch/absent → deny.
  The **domain is never a membership credential** (it remains only the DNS/storage
  namespace, `config.GetDomain()`).
- The gRPC membership interceptor (`interceptors/ServerInterceptors.go`, unary +
  stream) gates on the minted UUID and requires/validates `cluster_uid`.
- `cluster_uid` is emitted on the wire (JWT claim + gRPC metadata). `cluster_id`
  (domain) is still transmitted, but ONLY for its legitimate namespace/scoping role
  (repository/ai-memory cluster scoping) — no longer for membership.

Deferred (bundled together, since they interlock): flip `state.ClusterId` to hold
the UUID (domain moves fully to `ClusterNetworkSpec.ClusterDomain`), then make the
**join gate** UUID-based and distribute the UUID to installers via the signed
JoinPlan. The join gate stays domain-based until then (the installer can't present
a UUID it hasn't received yet); the primary membership enforcement (gRPC
interceptor) is already UUID-only.

Follow-ups (later steps, not blocking): distribute the UUID in the signed JoinPlan;
retire vestigial domain-as-`cluster_id` *emission* where it was only feeding
membership; shrink the identity ratchet's `cluster_id_is_domain` baseline as those
emission sites are cleaned.

### Superseded plan (kept for context): dual-accept, no flag-day

1. **Unblock persistence.** Remove the `isDomainLike` coercion (`state.go:665,712`);
   allow `ClusterId` to be any opaque string. Add the minted-UUID field alongside
   the existing domain (do not repurpose yet).
2. **Mint + persist + distribute.** Controller mints `cluster.id` at Day-0 → etcd
   `/globular/system/cluster/id`; include it in `GetClusterInfo`, JoinPlan, and node
   state. Domain stays exactly where it is for (b).
3. **Dual-accept validation.** `security.ValidateClusterID` accepts **either** the
   minted UUID **or** the legacy domain during the transition. `IsClusterInitialized`
   keys off the membership id being present, not "non-empty domain".
4. **Version the JWT claim.** Add the UUID as a new claim (`cluster_uid`) while still
   emitting/accepting the legacy `cluster_id=domain`, so no in-flight token
   invalidates at cutover.
5. **Switch injectors to the UUID** (client interceptors, `role_binding_check`,
   node-agent artifact fetch, controller release resolver, DNS reconciler creds) —
   read from the authority, not `GetDomain()`.
6. **Bridge join.** `handlers_join_authorization` + `join_plan_gate` accept a
   legacy-domain caller during the window, prefer the UUID.
7. **Flip `GetLocalClusterID` to read-only membership id** (fail-closed). Drop the
   domain from validation once every injector emits the UUID and no legacy tokens
   remain (bounded by token TTL).
8. **Remove dual-accept** and the domain-as-identity paths. The identity ratchet's
   `cluster_id_is_domain*` baseline entries drop out here, shrinking the gate.

## Explicitly NOT changed
- ScyllaDB workflow / ai_memory partition keys (stay domain — separate concern).
- scylla-manager token seed (stays `sha256(domain|caHash)`; only drop the hardcoded
  `"globular.internal"` fallback → fail-closed read of the domain).
- etcd `EtcdClusterToken` constant (already correctly decoupled — the model).
- All genuine `ClusterDomain` / FQDN / DNS reads (they are the domain, not identity).

## Verification
- Ratchet: `cluster_id_is_domain*` + `cluster_id_domain_coercion` baseline entries
  removed as each step lands (stale-baseline check enforces it).
- New tests: mint-once idempotency; dual-accept window accepts both; fail-closed when
  membership id absent; JWT dual-claim accept; join bridge; domain still drives DNS.
- Live: report stays clean; no `cluster_id validation failed` during a simulated
  rolling restart.
