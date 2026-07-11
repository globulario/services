# D1 — Explicit Per-Node Placement Contract

**Status:** APPROVED — settled rulings codified 2026-07-11. D1a implemented
(commits: `cluster-controller: D1a …`, `placement: reject all unconsumed node
assignments`); D1b–D1d pending. D2 is a separate correction PR.
**Date:** 2026-07-11
**Owner:** cluster_controller / component_catalog / cluster_doctor

---

## 0. Settled rulings (the law — codified before implementation)

1. **Placement is additive GRANT only.** `authorized(node, pkg) =
   profile_authorized(node, pkg) OR explicit_grant(node_id, pkg)`. A grant
   creates no new package identity, catalog entry, or second placement law — it
   only ADDS one package to one node. **No DENY semantic in D1.** Profiles remain
   broad capability/class placement; grants are surgical additive exceptions.
2. **Immutable `node_id`** for targeting, never hostname/IP. Package/service
   identity is resolved through the existing component catalog.
3. **Reject every unconsumed `NodeAssignment` until it is consumed.** The
   invariant is not "reject grants" — it is: no `NodeAssignment` may be accepted
   until every supported semantic in that structure is consumed. So *any*
   non-empty `node_assignments` (grant, bare version override, nil-only entry) is
   rejected — not persisted, not warned, not accepted "for future use". All
   persistence paths route through one canonical validating choke point.
4. **Version override is NOT part of D1.** D1 implements additive placement
   grants. The dormant per-node version override stays rejected even after grants
   are enabled, unless it later receives its own contract and consuming resolver.
   D1b MUST NOT blanket-lift the rejection for the whole structure.
5. **Node removal retains grants** (§6). Revoke / retarget / cascade-delete each
   require an explicit owner-mediated operation; default removal never erases
   operator placement intent.
6. **Four-stage delivery** (§10). The rejection is lifted only in the change that
   proves the relevant semantic is consumed end-to-end.

---

## 1. Problem & contract

Placement today is **profile-only**. The authorized set for a node is
`PackagesForProfiles(node.profiles)` and nothing else. There is no first-class
way to say "run service S on node N" independent of N's profiles. A
`NodeAssignment` struct exists (`cluster_controllerpb/resources_types.go:126`,
referenced at `ServiceReleaseSpec.NodeAssignments:147`) but is **dead code** —
never read in non-test source — so a spec author who sets `node_assignments`
today is **silently ignored**.

**Target contract:**

```
authorized(node, package) = PackagesForProfiles(node.profiles) ∪ ExplicitAssignments(node.id)
```

- Explicit assignments are **additive**. They may only ADD a package to a node.
- They MUST NOT create a second package catalog and MUST NOT duplicate
  profile-placement rules. `component_catalog` remains the sole authority for
  **package identity** and **profile placement**. The assignment authority only
  answers the narrow question: "is package P *additionally* authorized on node N
  by explicit intent?"
- `NodeAssignment` is **made real** (this spec), not deleted.
- **Interim (D1a, implemented):** ANY non-empty `node_assignments` **fails
  validation** (`InvalidArgument`) at the canonical `applyServiceRelease` choke
  point — structure-wide, covering grant, bare version override, and nil-only
  entries. Not persisted, not warned. All five `ServiceRelease` write sites route
  through the choke point so silent acceptance is structurally impossible, not a
  fence with a labeled gap. Version override stays rejected past D1b (§0.4).

---

## 2. Canonical assignment owner and storage

**Owner:** the **`ServiceRelease`** resource (cluster_controller). No new
resource kind, no second catalog.

- A `ServiceRelease` is keyed by `PublisherID` + `ServiceName` — it already IS
  the per-service desired-state authority.
- Storage: etcd, existing key
  `/globular/resources/ServiceRelease/{publisher}/{service}`. The
  `NodeAssignments []*NodeAssignment` field on `ServiceReleaseSpec` is the
  storage location. No new etcd key schema.
- Rationale: the service identity of an assignment is the **enclosing
  release's** `ServiceName`+`PublisherID`. `NodeAssignment` therefore needs only
  a node target — it does not (and must not) carry its own package name, or it
  would become a second catalog.

**`ExplicitAssignments(node.id)`** is a pure *derivation*, not a stored set:

```
ExplicitAssignments(N) = { release.ServiceName
                           | release ∈ all ServiceReleases,
                             ∃ a ∈ release.NodeAssignments with a.NodeID == N
                             and a.Placement == GRANT }
```

(see §4 for the `Placement` field.)

---

## 3. Immutable node identity used for targeting

**`NodeAssignment.NodeID` = the stable, hardware-derived `node_id`** (UUIDv5
over machine identity, e.g. `681710ee-6966-5df3-b155-3cef8b4e1a96`) — the SAME
identity used everywhere else (`/globular/nodes/{node_id}/…`, heartbeat,
verification runtime).

- MUST NOT be hostname or IP (both mutable / reassignable).
- This is the identity that survives clean+rejoin (which is precisely why stale
  state keyed on it is a hazard — see §6, §7).
- Validation: on write, `NodeID` MUST be a syntactically valid node_id. It is
  NOT required to reference a currently-registered node (assign-ahead is
  allowed; see §6).

---

## 4. Service / package identity field

The package identity of an assignment is the **enclosing `ServiceRelease`'s
`ServiceName` (+ `PublisherID`)** — resolved through `component_catalog` for
identity, exactly as profile placement is. `NodeAssignment` gains **no**
`ServiceName` field (that would fork the catalog).

To make `NodeAssignment` *real* without overloading its current
"version-override" meaning, add ONE explicit field:

```go
type NodeAssignment struct {
	NodeID    string            `json:"node_id,omitempty"`
	Version   string            `json:"version,omitempty"`   // Empty = release default
	Pins      map[string]string `json:"pins,omitempty"`
	Placement string            `json:"placement,omitempty"` // "" | "GRANT"  (NEW)
}
```

- `Placement == "GRANT"` → this assignment **adds** the release's service to
  `NodeID`'s authorized set (the explicit-assignment term of the union).
- `Placement == ""` (default) → **version override only** (the pre-existing
  documented meaning): applies iff the node is ALREADY authorized by profile.
  A bare version override on a node that has no profile authorization is a
  no-op placement-wise (and SHOULD warn — see §8 validation).
- This keeps the two intents explicit and non-ambiguous, and makes the
  make-real path additive (no silent semantic change to any existing use —
  there are none, but the discipline matters).

---

## 5. Generation and mutation path

Explicit assignments flow through the **existing release pipeline** — no new
imperative path.

- **Mutation:** `globular services desired set <service> --node <node_id>` (new
  flag) → controller RPC that reads-modifies-writes the `ServiceRelease`'s
  `NodeAssignments` (append/update/remove a `{NodeID, Placement:GRANT}` entry).
  Also settable in a `ServiceRelease` YAML applied via the deploy path.
- **Audit:** every mutation writes a `/globular/audit/desired_writes/…` record
  (same mechanism already observed for desired writes), so assignment changes
  are durable and attributable.
- **Generation into placement:** the release reconciler's node-selection
  (`release_pipeline.go` `reconcileResolved`, the `placementAllows` gate) is
  extended to: `placementAllows(catalog, node.profiles) || hasGrant(release,
  node.id)`. This is the ONE code point where the union is formed. Doctor and
  node-agent consume the same derived authorization (see §8) so there is no
  second law book.
- **Idempotent / declarative:** assignments are desired state; re-applying the
  same set is a no-op.

---

## 6. Behavior when the target node disappears

A `GRANT` for a `NodeID` that is not currently a cluster member:

- **Is retained, not auto-deleted.** Node identity is stable; a node may be
  down, draining, or not-yet-joined (assign-ahead). Deleting the grant on
  transient absence would lose operator intent.
- **Is inert while the node is absent:** it contributes nothing to any live
  node's authorized set and generates no dispatch (there is no target to
  dispatch to).
- **Surfaced, not silent:** doctor emits an informational (WARN, health-neutral
  — consistent with D2) finding `placement.assignment_targets_absent_node` so a
  grant pointing at a node that has been removed for good is visible for
  cleanup. It MUST NOT degrade the cluster verdict.
**Missing-target state machine (codified):** a `GRANT` whose `NodeID` is not a
current member resolves to:

| field | value |
|-------|-------|
| `grant_state` | `target_absent` |
| severity | `SEVERITY_WARN` |
| invariant | `INVARIANT_PENDING` (visible, health-neutral — same rule as D2) |
| effect on convergence | none |
| effect on cluster health | none |

- **On node removal (`globular cluster remove-node`):** grants targeting the
  removed node are **NEVER auto-purged** — a grant is desired operator intent,
  and node disappearance/removal is a topology change, not authority to erase
  intent. Removal completes, the dangling grant is **retained and reported**
  (`target_absent`), and it preserves useful evidence ("package X was
  deliberately assigned to node Y, but Y no longer exists"). Revocation,
  retargeting, or cascading deletion each require an **explicit owner-mediated
  operation**; default removal must never silently delete grants (a dead node
  must not become an accidental desired-state eraser).

---

## 7. Interaction with uninstall, orphan detection, convergence, desired hashing

- **Orphan detection (the D2 rule):** a package that is authorized by a `GRANT`
  is **not** an orphan. `placement.installed_package_orphaned` MUST test
  `authorized(node, pkg)` = `profile ∪ explicit`, not profile alone. Concretely:
  the doctor rule's `placeable` set (`placement_orphaned_install.go:54-57`) is
  extended with the node's `GRANT`ed services. A grant is the sanctioned way to
  make a "would-be orphan" conformant — which is exactly the operator remediation
  the orphan finding already suggests ("add a matching profile OR retire").
- **Uninstall:** removing a `GRANT` (and no profile authorizing the service)
  makes the installed package an orphan on the next evaluation → it becomes a
  non-blocking WARN and an operator/node-agent uninstall candidate. Removing a
  grant does NOT auto-stop the running service (same non-blocking runtime
  posture as every other placement decision); it converts it to reported drift.
- **Convergence:** grants participate in dispatch exactly like profile
  placement — a `GRANT`ed service on an authorized node is dispatched and
  converges normally. A node authorized ONLY by profile is unaffected.
- **Desired hashing:** the per-node desired set used for the applied/desired
  hash comparison MUST include `GRANT`ed services for that node (they are part
  of desired). `filterVersionsForNode` (which today filters orphans OUT before
  hashing) MUST treat a `GRANT`ed service as authorized (kept in), so an
  explicitly-assigned service is a normal converged member, not filtered noise.
  Symmetric with orphan detection: one `authorized()` predicate feeds both.

---

## 8. Single authorization predicate (no second law book)

Introduce ONE function, consumed by controller, node-agent, and doctor:

```
authorized(nodeID, nodeProfiles, pkg) bool =
    PackagesForProfiles(nodeProfiles) contains pkg      // component_catalog authority (unchanged)
    OR hasGrant(pkg, nodeID)                             // explicit-assignment authority (additive)
```

- `PackagesForProfiles` / catalog identity: **unchanged** — still the only
  package-identity + profile-placement authority.
- `hasGrant` reads `ServiceRelease.NodeAssignments`; it can ONLY add, never
  remove or reclassify a catalog decision.
- All three current call sites migrate to this predicate:
  `component_resolve.go` `placementAllows`/`isOrphanedInstall`,
  node-agent `workflow_profile_placement.go:113` `workflowPackageAllowedForProfiles`,
  doctor `placement_orphaned_install.go:54`.

**Validation (interim + permanent):**
- Interim (pre-D1b): non-empty `NodeAssignments` with `Placement == "GRANT"`
  → reject at `ServiceRelease` validation with
  `"explicit node placement (node_assignments[].placement=GRANT) is not yet
  enabled"`. Bare version-override assignments (`Placement==""`) remain a no-op
  as today but SHOULD warn if the node isn't profile-authorized.
- Permanent (D1b): `GRANT` with a syntactically-invalid `NodeID` → reject.
  `GRANT` referencing a `ServiceName` unknown to the catalog is impossible by
  construction (identity comes from the enclosing release, which must resolve an
  ArtifactRef), but the release-validation that already enforces this is the
  guard.

---

## 9. Tests (proving the six behaviors)

All pure-snapshot / unit where possible; integration for dispatch + hashing.

1. **profile-only** — node with profile P, service S∈P, no grant → S authorized,
   dispatched, converges, NOT an orphan. (regression of current behavior.)
2. **explicit-only** — node WITHOUT any profile granting S, `GRANT(S, node)` set
   → S authorized, dispatched, converges, NOT an orphan; `authorized()` true via
   grant term only.
3. **union** — node with profile P (grants A) AND `GRANT(B, node)` → authorized
   set = {A, B}; both dispatch; neither is an orphan; a third installed service C
   (neither profile nor grant) IS a (health-neutral, per D2) orphan.
4. **removal** — start from (2), remove the grant → S becomes an orphan finding
   (WARN, health-neutral), desired-hash drops S, running service is NOT
   auto-stopped, uninstall becomes an operator/agent candidate.
5. **unknown package** — a grant cannot name a package (identity is the release);
   test that a `ServiceRelease` whose ServiceName is unknown to the catalog fails
   release validation (existing guard) — grants add no new unknown-package path.
6. **unknown node** — `GRANT(S, N)` where N is not a current member → grant
   retained, inert (no dispatch), doctor emits health-neutral
   `assignment_targets_absent_node`; when N later joins with matching identity,
   S becomes authorized and dispatches (assign-ahead).

Plus the interim guard test: non-empty `GRANT` assignment → `ServiceRelease`
validation error (until D1b flips the feature on).

---

## 10. Rollout / commit boundary — four-stage delivery

The rejection (§0.3) is lifted ONLY in the change that proves the relevant
semantic is consumed end-to-end — never before.

- **D2** (separate PR, a correction; DONE): orphan finding → `INVARIANT_PENDING`
  (visible, health-neutral WARN; `PASS` was rejected because the renderer drops
  PASS findings from the report) + regression tests. Does not touch the
  placement model.
- **D1a** (DONE): schema (`Placement` field + `NodeAssignmentPlacementGrant`),
  canonical validation at the `applyServiceRelease` choke point, and TOTAL
  rejection of any non-empty `NodeAssignments`. Two commits (schema+grant-reject,
  then structure-wide reject + choke point).
- **D1b** (the resolver): the single grant-aware predicate
  `authorized() = profile ∪ grant` and the grant-delivery plumbing, migrated
  across controller / node-agent / doctor, plus permanent `node_id` validation.
  **Lifts NOTHING** — the write reject stays; the resolver is proven with
  SYNTHETIC grants in tests (§0.6: the reject is lifted only when consumption is
  proven end-to-end). Convergence/hashing integration (desired-hash membership,
  removal→orphan hash-drop, dispatch) is explicitly **D1c**, not D1b.
- **D1c**: convergence, desired hashing, orphan detection, and lifecycle
  integration (granted service kept in the desired hash, not an orphan; removal
  of a grant converts to a non-blocking orphan; missing-target state per §6).
- **D1d**: enable persistence and mutation of grants (the CLI/RPC mutation path,
  §5) — the point at which grants become operator-settable end-to-end.

D1a/D1b/D1c/D1d are separate commits/PRs, and all separate from D2.
