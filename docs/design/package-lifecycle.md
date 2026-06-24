# The Globular Package Lifecycle — canonical definition

> **Status:** authoritative target model (2026-06-23). This document is the single
> prose source of truth for what a Globular package *is*, how its identity is
> defined, who owns each field, and how it moves from definition to retirement.
> It is referenced by all three repositories — `globulario/packages`,
> `globulario/services`, `globulario/Globular` — and is backed by machine-enforced
> awareness contracts (see §7). Where current code diverges from this model, the
> divergence is a **bug to close**, not a precedent.

---

## 0. Why this document exists

The package is the central unit of the platform: everything the cluster converges
on is a package. Yet "package" was defined in fragments across three repos, with
**two version authorities** (CI git tags *and* local `deploy --bump`), a **kind
that leaks between the service and infrastructure lanes**, and **agent/local builds
that publish straight into the release stream the cluster converges on**. The
result is chronic `services.drift` that "works until it lies" — the repository is
clean, but desired-state points at the wrong artifact and the cluster cannot
reconcile.

The cure is one identity model, one lifecycle, one source of truth per field,
and an enforced boundary between *local/dev* and *release*. This document defines
it; §7 enforces it.

---

## 1. What a package IS

A Globular package is an immutable, content-addressed `.tgz` describing **one
installable unit**. It is NOT a deb/rpm, NOT a container image, NOT a source
bundle. Three properties define it: **identity**, **kind**, **recipe**.

### 1.1 Identity — the tuple

```
identity = (publisher, name, kind, version, build_id, platform)
```

| Field | Meaning | **Single source of truth** |
|-------|---------|----------------------------|
| `publisher` | who owns/signs it — an **RBAC-owned namespace** whose owning subject (Organization / Account / service account) *derives* the publisher "type" | the namespace owner **RBAC subject** + `trusted_publisher` forge binding; **never a parallel identity enum** |
| `name` | canonical package name | `packages/registry.yaml` (the one authority) |
| `kind` | `service` \| `infrastructure` \| `command` \| `application` | `registry.yaml`, cross-checked against `golang/.../component_catalog.go` |
| `version` | human release version (semver, or upstream-native tag) | **the release authority** — the repository service (§3) |
| `build_id` | UUIDv7 — **the SOLE convergence identity** | the repository service, allocated at upload |
| `build_number` | display-only iteration counter within a version | the repository ledger (per `publisher,name,version,platform`) |
| `platform` | target (`linux_amd64`, …) | the manifest |

### 1.2 The three identifiers must never be confused

- **`version`** — human-readable release tag. Goes in the manifest and filename.
  `Version = ""` in source; injected via ldflags at build. Must be **monotonic per
  package**. Must NOT embed build tokens (`1.2.3+b325` is forbidden).
- **`build_number`** — plain integer, **display only**. NEVER used for convergence.
- **`build_id`** — UUIDv7, repository-allocated post-upload. **The only thing
  convergence, drift, and desired-state may compare.** Empty in committed source.

> **Invariant.** Any equality check that decides "is the cluster converged?" MUST
> compare `build_id`. Comparing `version` or `build_number` for convergence is a
> defect. (Today the services-drift hash compares the *version string* — that is
> bug class #1; see §5 stage 6 and §7.)

### 1.3 Kind is identity, not metadata

`kind` is determined by **bootstrap role, not language**:

- `infrastructure` — day-0 / control-plane-critical daemon (etcd, scylla, minio,
  envoy, **xds**, prometheus, keepalived, gateway). Managed by the infrastructure
  desired lane.
- `service` — a post-bootstrap gRPC-mesh workload (echo, dns, repository, mcp…).
  Managed by the service desired lane (`ServiceDesiredVersion`).
- `command` — CLI tool, no systemd unit.
- `application` — rare whole-platform bundle.

`kind` is declared **once** in `registry.yaml` and must agree everywhere
(`package.json`, spec `metadata.kind`, systemd unit, `component_catalog.go`). It is
carried on **every** desired/installed record keyed by `(kind, name)`. **No
kind-blind tool may write it.** (Today `globular services desired set --force` can
write a SERVICE record for an INFRASTRUCTURE package → permanent dispatch block.
That is bug class #2; see §4 and §7.)

---

## 2. Recipe & required files

For a package `<name>`, the load-bearing definition files live in `globulario/packages`:

```
packages/registry.yaml                       # ONE canonical entry: name, kind, publisher, profiles, deps, bootstrap_tier
packages/metadata/<name>/package.json        # reference manifest (type==kind, publisher, entrypoint)
packages/metadata/<name>/awareness.yaml      # invariants + failure modes
packages/specs/<name>_{service,cmd}.yaml     # ordered, idempotent install recipe
packages/metadata/<name>/{systemd,config,scripts,debs}/  # optional assets
```

The built `.tgz` re-shapes the payload under `bin/`, `specs/`, `config/`,
`systemd/`, `scripts/`, `debs/`, `data/`, with a stamped root `package.json`.

---

## 3. The authority model — repository is the source of truth

> **Decision (2026-06-23): the repository service is the SINGLE runtime source of
> truth for package identity and version allocation. GitHub/CI is a *gated client*
> of the repository, not a parallel authority. The release version is allocated by
> a deliberate release act, never by an ad-hoc build. The boundary between local
> and release is a CHANNEL.**

### 3.1 Channels

`AllocateUpload` already carries a `channel` argument; this model gives it teeth.

| Channel | Who may use it | May allocate a release version? | May write cluster desired-state? | Build shape |
|---------|----------------|---------------------------------|----------------------------------|-------------|
| `release` (stable) | **CI only** (holds the `release-authority` capability) | **Yes** — monotonic, gated | Yes (via the release/BOM path) | `-trimpath -ldflags "-s -w"` required |
| `dev` | local developers, agents/MCP | **No** — `build_number` bump only, against the current release version | **No** — structurally forbidden | unconstrained, but tagged `dev` |

- The **release version** flows from a tagged release: the git tag `vX.Y.Z` is the
  *intent input*; CI calls the repository with the `release-authority` capability;
  the repository **validates monotonicity and allocates/records** the version and
  `build_id`. The repository — not the tag, not the build — is the truth.
- A **local or agent build** calls the repository on the `dev` channel: it may only
  increment `build_number` within the *current* release version, publishes a
  `dev`-tagged artifact, and **cannot** allocate a release version or write
  cluster desired-state. Dev artifacts are **invisible to cluster convergence**.

This eliminates both root causes: there is exactly one release-version author, and
a local/agent build can never become cluster truth.

### 3.2 What this changes vs today

- CI currently self-stamps `version` from the git tag and bypasses `AllocateUpload`
  → it must instead allocate through the repository with the release capability.
- Local `globular deploy --bump` currently allocates a real release version with no
  caller distinction → it must default to the `dev` channel and refuse release
  allocation without the capability.
- `AllocateUpload` must carry **caller provenance** (channel + capability) and
  enforce it; today it only checks `CapRepoWrite`.

### 3.3 Resolved enforcement mechanism (2026-06-23 redline)

The channel is the security boundary. No new primitives are required — the
`ArtifactChannel` enum already exists and is persisted on `ArtifactManifest.Channel`;
RBAC actions are already the grant mechanism; the CI identity
`globular-repository-publisher-sa` already exists.

- **Allocate-side gate (release):** a new RBAC action `repository.release.allocate`
  is required to call `AllocateUpload`/`UploadArtifact` when `channel = STABLE`.
  It is granted **only** to `globular-repository-publisher-sa` (CI). Local/agent
  identities may allocate on `DEV` (under `repository.write`) but not on `STABLE`.
  The git tag in CI is the intent; the CI service account holds the action and
  allocates the release version through the repository.
- **Publish-side gate (dev):** a publish that lacks a valid release reservation is
  **forced onto the `DEV` channel** — never rejected. `DEV` artifacts are **never
  resolvable as desired-state**. The MCP `package_build`/`package_publish` tools and
  any non-`--bump` CLI publish default to `DEV`. This closes the current bypass for
  every client without breaking anyone.

Together: to land a release-channel artifact you need a `STABLE` reservation, which
requires the `repository.release.allocate` action to allocate; everything else is a
`DEV` artifact that the cluster will never converge on.

### 3.4 RBAC-native authority (no parallel identity system)

Package authority is expressed **entirely through the existing RBAC model** — the
platform already has the right grain (publisher namespaces are RBAC-owned resources;
owners are RBAC subjects; subjects are Account/Group/Organization; permissions attach
to subjects; `trusted_publisher` already federates forge identities). Do **not** build
a parallel publisher identity system.

```text
publisher namespace   = RBAC resource owned by a subject
owning subject kind    = Organization | Account | service account   (derives "type")
release authority      = RBAC permission  repository.release.allocate  on the namespace
forge_binding          = trusted_publisher federation: GitHub org/user/repo → RBAC subject
channel selection      = STABLE iff the resolved subject holds repository.release.allocate; else DEV
```

**Two-step trust — federation and authorization are separate layers and must never
be collapsed:**

1. **Federation** (`trusted_publisher` / forge binding) answers exactly one question:
   *"Which RBAC subject is this forge identity?"* A forge token / GitHub Action /
   org / user / repo resolves to an RBAC subject. It grants nothing.
2. **Authorization** (RBAC only) answers: *"May subject X perform `repository.release.allocate`
   on namespace Z?"*

   Forbidden shortcut: `GitHub org == globulario, therefore allow STABLE`.
   Required flow: `forge identity → trusted_publisher binding → RBAC subject → RBAC permission check → allow/deny`.

CI is **only a forge adapter** — it authenticates, resolves to an RBAC subject, then
passes normal RBAC authorization. It has no implicit privilege; neither does a git tag.

**Desired code shape** (the release path calls all three, in order; fail closed if any
step fails):

```text
ResolveForgeIdentity(ctx, token)                        -> RBACSubject     // federation only
AuthorizeRelease(ctx, subject, namespace, action)       -> allow | deny    // RBAC only
AllocateRelease(ctx, namespace, versionRequest, subject) -> release identity // repository allocates
```

The DEV path may resolve a subject and bind the build to `DEV`, but must not allocate
a `STABLE` release identity unless the same RBAC permission check passes.

**Layering — keep these on separate primitives (do not blur):**

```text
forge proves who is speaking          (trusted_publisher federation)
RBAC decides what they may do         (permission on namespace)
repository allocates release identity (build_id)
desired state consumes build_id       (controller writes desired)
controller converges by build_id      (convergence — NOT RBAC)
```

The convergence rule ("only STABLE artifacts become desired-state") is a **controller
invariant, not a permission**. RBAC gates *who may publish a release*; the controller
gates *what may converge*. Two locks, different doors — collapsing them turns a
permission bug into a cluster-rollout bug.

### 3.4.1 How release authority is granted (implemented, P1 Slice 1 + P3)

`AuthorizeRelease` (`releaseAccessCheck` in `repository_server/release_authority.go`)
accepts a subject as a release authority on namespace `Z` via **either** RBAC path —
both scoped to `Z`, never blanket:

1. **Explicit resource grant** — `release.allocate` granted directly on the namespace
   resource `/namespaces/Z` (path-scoped `ValidateAccess`). Namespace **owners** pass
   here automatically (`isOwner ⇒ full access`), so a subject that claimed `Z` can
   release into it with no extra grant.
2. **Role capability + namespace association** — the subject is bound (`SetRoleBinding`)
   to a role whose cluster-roles.json actions include `release.allocate`, **and** the
   subject is associated with `Z` (owner / collaborator / owning group / owning org, via
   `subjectInNamespacePermissions`). A role grant is authority **only** on namespaces the
   subject is actually attached to — mirroring `validatePublisherAccess`. This is the
   path a **CI / publisher service account** uses.

> Naming: the contract action key is `repository.release.allocate`; the namespace-scoped
> resource-permission verb checked at the gate is `release.allocate` (the form that
> `ValidateAccess` and `HasRolePermission` match). They are the same authority in two
> encodings.

**Granting a non-superuser CI publisher** (`globular-repository-publisher-sa`) release
authority on namespace `Z` — least privilege, no `sa`:

```text
# 1. capability: the SA role already carries release.allocate (cluster-roles.json)
#    → seeded by SeedClusterRoles; HasRolePermission resolves it. (no command)
# 2. binding:    bind the SA subject to its capability role
globular rbac bind --subject globular-repository-publisher-sa --role globular-repository-publisher-sa
# 3. association: attach the SA to the namespace it may release into
#    (also binds namespace:publisher and adds the SA to the namespace's allowed
#     permissions, which is the association subjectInNamespacePermissions checks)
globular namespace grant Z globular-repository-publisher-sa --role namespace:publisher
```

After this, the SA allocates `STABLE` **only** for namespace `Z`; every other namespace
(and every other subject) is forced to `DEV` by the same gate. Superuser (`sa`) and
in-process/direct calls remain trusted system paths. `CHANNEL_UNSET` defaults to
`STABLE` and is therefore subject to the same check.

### 3.4.2 The ingestion gate — re-deriving channel authority on upstream sync (P2)

The allocate-side gate (§3.4.1) governs callers of `AllocateUpload`. But the cluster
also imports artifacts by **pulling `release-index.json` from a forge** (GitHub) via
`SyncFromUpstream`. Each entry carries a CI-stamped `channel`; trusting it makes CI the
release authority again — a build is `STABLE` because CI *said so* in a JSON file.

The **ingestion gate** (`upstream_release_gate.go`, in `processSyncEntry` before the
channel is reported or persisted) treats that field as untrusted input and re-derives
channel authority from RBAC, using the **same two-step trust** as §3.4 — applied to the
*upstream publisher* instead of an interceptor subject (the sync path has no
`AuthContext`):

```text
FEDERATION    upstream forge identity (source owner/repo) matches a registered
              trusted publisher for the namespace        → who is speaking
AUTHORIZATION the federated subject holds release.allocate on the namespace (RBAC,
              via subjectHoldsReleaseAuthority — shared with §3.4.1)  → what they may do
result        STABLE survives iff BOTH hold; otherwise STABLE → DEV (downgrade)
```

Federation alone never grants `STABLE` (`package.forge_binding_is_not_authorization`) —
a trusted-publisher binding that lacks the `release.allocate` grant is downgraded.

**Rollout safety.** The gate is **inert for a namespace with no registered trusted
publishers** (unmanaged → channel unchanged), so it never retroactively downgrades a
pre-existing sync. It activates per-namespace only once a release authority is declared
— effectively opt-in. The action is a **non-destructive `STABLE → DEV` downgrade**: the
artifact is still imported and inspectable, it simply isn't convergeable (the controller
resolves desired state from `STABLE` only — Slice 3), and it is reversible by granting
the authority and re-syncing. Error handling balances the two failure directions: a
trusted-publisher **store** error fails toward *unmanaged* (a hiccup must not downgrade
every namespace), while the **RBAC** step fails closed (an unprovable permission is no
permission).

> To make a managed namespace's upstream releases land `STABLE`, the operator must do
> **both** steps for it: register the trusted publisher (federation) **and** grant
> `release.allocate` to that publisher (authorization, per §3.4.1). Registering only the
> trusted publisher downgrades its `STABLE` imports to `DEV` — by design.

Two gates, one authority model: `AllocateUpload` gates *who may allocate a release
identity*; ingestion gates *whether an imported release may keep its `STABLE` channel*.
Both answer "does this subject hold `release.allocate` on the namespace?" via the same
`subjectHoldsReleaseAuthority` primitive.

### 3.4.3 The direct-publish gate — agent/MCP builds are DEV by construction (P4)

`AllocateUpload` is the reservation/bump flow, but `globular pkg publish` (and the MCP
`package_publish` tool, and any caller) uses the **direct `UploadArtifact`** path, whose
channel comes from `package.json` / a reservation. Without a gate there, STABLE could be
claimed with no release authority — and an agent could route around any tool-layer
binding via `globular_cli_execute`. So the gate lives at the **authority boundary**, not
the periphery: `UploadArtifact` runs the **same** `resolveForgeIdentity` + `authorizeRelease`
as `AllocateUpload` (the RPC carries an `AuthContext`).

When the resolved channel is `STABLE` and the caller lacks `release.allocate`:

```text
non-official publisher  → forced to DEV   (the agent/dev lane; lane-legal — Rule 2 only
                                            forbids OFFICIAL + DEV)
official publisher      → rejected         (the sealed namespace cannot be DEV; release
                                            authority is mandatory, not downgradable)
```

`sa`, in-process/direct (no `AuthContext`), and properly-granted CI authorities pass
unchanged. This makes **agent builds DEV by construction at the authority boundary** —
not bypassable by choosing a different tool — which subsumes a tool-layer channel flag.
The GitHub CI release path is unaffected (it publishes to GitHub Releases → upstream
sync, gated by §3.4.2, not `UploadArtifact`).

Three entry points, one authority model: **allocate** (§3.4.1), **ingest** (§3.4.2), and
**direct publish** (§3.4.3) all gate `STABLE` on `release.allocate` for the namespace.

### 3.4.4 DEV version semantics — build-number-only, off the release stream (P5)

The channel gates decide *whether* a build is `STABLE`; this rule governs the *version*
a `DEV` build may carry. **A DEV build must never advance the release stream.** The
release version (`major.minor.patch`) is cluster/CI-allocated; a local/agent build only
adds a `build_number`.

In `AllocateUpload` (the reservation / `globular deploy` flow), once the channel is final
— including when the release gate has just forced `STABLE → DEV` — a `DEV` build's version
is **coerced** (`devLaneVersion`) to a lane-safe form:

```text
intent bump 1.2.43 → 1.2.44, then forced to DEV
  ⇒ version pinned to  1.2.43-dev.1   (latest release + -dev pre-release suffix)
  ⇒ build_number iterates within it; the DEV build semver-orders BELOW 1.2.43
  ⇒ it can neither squat the published 1.2.43 identity nor claim a new 1.2.44 release
```

An already lane-safe version (`-dev.` / `+local.` / `-hotfix.`) is kept. With no published
release yet, the resolved version is suffixed (`0.0.1 → 0.0.1-dev.1`) so it still claims
no release. The coercion is repository-owned and non-destructive — the deploy never fails,
mirroring how the channel gates force `DEV` rather than reject.

> **Direct-publish path (resolved in #6c).** `globular pkg publish` (and the MCP
> `package_publish` tool) use the direct `UploadArtifact` path, where the binary blob is
> written under a version-derived storage key **before** the channel is read from
> `package.json` — so a post-hoc version coercion there would desync the manifest from the
> blob. So the direct path is bound to the DEV lane from **both ends** instead:
> - **By construction (CLI):** `pkg publish --channel dev/local` auto-appends a `-dev`/`+local`
>   version suffix client-side before upload (`pkg_cmds.go`), so a well-formed dev-lane
>   publish is lane-safe by the time it reaches the server.
> - **Backstop (server):** `validateLocalIdentityRules` **Rule 4** rejects a `DEV` artifact
>   that carries a clean release version (no lane suffix). A clean-semver `DEV` — including
>   one the release gate force-downgraded from an unauthorized `STABLE` — is rejected rather
>   than allowed to squat a release version. The lane is now an equivalence: **`DEV ⟺ suffixed
>   version`**, enforced without mutating the critical upload handler.

### 3.4.5 Channel eligibility — discoverability ≠ convergeability (BOOTSTRAP)

Two predicates classify channels, and they answer **different questions** — a distinction
that must stay explicit so they are never "reconciled" into one:

| Predicate | Service | Question | Set |
|-----------|---------|----------|-----|
| `isConvergeableChannel` | controller | may this become **desired state**? | `STABLE`, `UNSET` |
| `isDefaultListChannel` | repository | shown by default in **searches/listings**? | `STABLE`, `UNSET`, `BOOTSTRAP` |

`isConvergeableChannel` is the **single authority** on convergence eligibility (repository.proto
Invariant E — "the reconciler resolves from STABLE only"). The repository never re-defines
convergence: `ResolveArtifact` serves whatever channel the caller **explicitly** requests, so a
`BOOTSTRAP` artifact is only ever returned to a caller asking for `channel=BOOTSTRAP`.

**BOOTSTRAP is intentionally discoverable-and-servable but not auto-convergeable.** Bootstrap-phase
artifacts (publishable via `pkg publish --channel bootstrap`) must be visible in listings and
fetchable on explicit request during cluster bring-up, yet must **never** advance desired state
through normal release convergence. The set difference between the two predicates *is* that
contract — it is not a bug, and the convergence set must **not** be widened to match the listing
set. (`isDefaultListChannel` was previously named `isReconcilerSafeChannel`, which wrongly implied
it was a convergence predicate; renamed to remove the false second definition.) Each predicate is
locked by a `channel_eligibility_test.go` in its service.

---

## 4. The infrastructure-vs-service rule

The recurring "infra vs service bite" is a kind-identity leak. The rule:

1. `kind` is declared once (`registry.yaml`) and is part of identity.
2. Every desired/installed record is keyed by `(kind, name)`; the two desired lanes
   (`ServiceDesiredVersion` and the infrastructure desired path) never share a key.
3. **Kind-blind writes are rejected.** `globular services desired set` hard-refuses
   non-SERVICE kinds; the `--force` kind-bypass is removed (bypassing a kind check
   is the "relax the identity check to make it pass" forbidden pattern).
4. Infrastructure gets a **first-class desired-management path** (it currently has
   no coherent typed setter — it is "managed by bootstrap").
5. A **safe typed removal** exists for every lane. (Today `DeleteServiceDesiredVersion`
   is unwired and `RemoveDesiredService` triggers an uninstall — a mis-kinded record
   has no safe remediation.)

---

## 5. The lifecycle — nine stages

Each stage names its **owner**, its **source of truth**, the **invariant** it must
uphold, the **enforcement point**, and the **current gap**.

### Stage 1 — Define
- **Owner / SoT:** `packages` repo — `registry.yaml` (+ metadata + spec).
- **Invariant:** name, kind, publisher agree across registry / package.json / spec /
  systemd / `component_catalog.go`.
- **Enforce:** `packages/scripts/validate-package-identity.py` (build-gated).
- **Gap:** validator catches drift *reactively* (after it shipped once); no version
  or artifact-shape checks.

### Stage 2 — Generate (incl. agent/MCP-driven)
- **Owner / SoT:** `globular generate` / `package_build`; templates.
- **Invariant:** `Version = ""` in source; publisher + kind injected from registry;
  version is **never hand-typed**.
- **Enforce:** generator templates + a guard that rejects a hardcoded version.
- **Gap:** agents can pass an arbitrary `--version` to `package_build`; generation
  is not reservation-bound.

### Stage 3 — Build
- **Owner / SoT:** globularcli / CI; the **repository reservation** supplies the version.
- **Invariant:** release builds are `-trimpath -ldflags "-s -w"`; the binary in the
  tarball matches `entrypoint_checksum`; `bin/<exec>` + spec present.
- **Enforce:** `assertPackageGuards`; a new **artifact-shape gate** (reject a build
  ~2× the prior size / non-stripped on the release channel).
- **Gap:** no trimpath/strip enforcement (the 15–18 MB `build_number 1` debug
  artifacts); no shape gate.

### Stage 4 — Publish
- **Owner / SoT:** repository service — ledger + local POSIX CAS (MinIO is a mirror).
- **Invariant:** version monotonic per package; identity immutable; entrypoint
  checksum cross-validated; **channel + provenance recorded**.
- **Enforce:** `allocate_upload.go` monotonicity gate + **new channel/capability
  gate**; `release_ledger.go` append guard.
- **Gap:** no channel/capability distinction (CI and local hit the same path); no
  artifact-shape gate.

### Stage 5 — Release / BOM
- **Owner / SoT:** CI release — `release-index.json` / repository active release.
- **Invariant:** only `release`-channel artifacts enter a BOM; unchanged packages
  carry forward their own version (a platform release must NOT stamp package versions).
- **Enforce:** BOM assembly validates channel + carry-forward.
- **Gap:** BOM/desired can name a version that is not the latest published, and
  nothing heals it forward.

### Stage 6 — Desire
- **Owner / SoT:** controller — etcd desired-state, keyed by `(kind, name)`.
- **Invariant:** no version regression on **any** write path; `build_id` immutable
  after resolution; cross-kind writes rejected; convergence identity is `build_id`.
- **Enforce:** regression guard on operator + materialize + build-advance paths
  (today only the operator path is guarded, and only with fresh heartbeats);
  kind-aware desired store; drift hash carries `build_id` and distinguishes
  "node ahead (desired regressed)" from "node behind (needs upgrade)".
- **Gap:** regression guard partial; drift compares the version string only;
  `services desired set --force` can write a cross-kind record.

### Stage 7 — Install
- **Owner / SoT:** node-agent — installed-state (etcd + local receipts).
- **Invariant:** download verified by `build_id` + sha256 against the **desired**
  manifest; never downgrade (absent explicit force/rollback).
- **Enforce:** content-addressed staging (key by digest/`build_id`); downgrade guard
  (`apply_package_release.go`).
- **Gap:** staging uses a mutable `latest.artifact` filename (identity-erasing); the
  cache-digest check compares against the *installed* manifest, not the desired one.

### Stage 8 — Run / Verify
- **Owner / SoT:** runtime (systemd + node-agent runtime check).
- **Invariant:** unit active; `sha256(/proc/<pid>/exe) == manifest.entrypoint_checksum`;
  `installed.build_id == desired.build_id`.
- **Enforce:** continuous node-agent runtime check.
- **Gap:** none structural; depends on stages 6–7 being correct.

### Stage 9 — Retire
- **Owner / SoT:** controller — lifecycle (`REMOVING → REMOVED`).
- **Invariant:** removal is lifecycle-tracked and idempotent; never orphan a build_id
  a desired record still references.
- **Enforce:** a safe typed removal RPC per lane.
- **Gap:** no safe typed deletion of a `ServiceDesiredVersion`.

---

## 6. Cross-repo conformance

| Repo | Owns | Must conform to |
|------|------|-----------------|
| `globulario/packages` | package **definitions** (registry, specs, metadata) and where artifacts land | §1 identity, §2 files, stage 1 validation |
| `globulario/services` | the **repository service** (runtime SoT), `globularcli` (build/deploy), the kinds catalog | §3 authority, stages 3–4 & 6–9 enforcement |
| `globulario/Globular` | the **consumer**: installer, gateway, CI **release** workflow, "upgrade from cluster" | §3 (CI is a gated client), stage 5 BOM |

All three import the same identity definition and are gated by the same awareness
contracts (§7). A divergence is a CI failure in whichever repo introduced it.

---

## 7. Enforcement — awareness contracts + code

The canonical rules live as awareness contracts (CI hard-gated) plus this doc.

### 7.1 Intents (promote / add)
- `intent:package.identity_tuple_must_be_unique` — promote `proposed → active`.
- `intent:repository.is_single_version_authority` — repository is SoT; CI is a client.
- `intent:package.release_vs_dev_channel_boundary` — local/agent builds never allocate
  release versions or write desired-state.

### 7.2 Invariants (add)
- `invariant:release.version_single_authority`
- `invariant:desired.no_regression_all_paths`
- `invariant:desired.keyed_by_kind_and_name`
- `invariant:convergence.identity_is_build_id`
- `invariant:publish.release_artifact_must_be_stripped`
- `invariant:staging.content_addressed`

### 7.3 Forbidden fixes (add)
- `forbidden_fix:local_deploy_allocates_release_version`
- `forbidden_fix:cli_writes_cross_kind_desired_record` (the `--force` kind bypass)
- `forbidden_fix:compare_version_string_for_convergence`
- `forbidden_fix:compare_cached_blob_against_installed_manifest`
- `forbidden_fix:materialize_desired_from_unverified_local_install`

### 7.4 Required tests (add)

> **Graph-required vs backlog.** Only tests that **exist** are cited as graph-required
> refs — the RBAC-native release-authority suite (`release_authority_test.go`) is
> defined in `required_tests.yaml`. The P1/P2/P3 scenarios below are an
> **implementation backlog**, tracked here as prose **not** as `required_tests` graph
> refs, so they don't dangle the seed under `yaml2nt -validate-refs`. Promote each to
> a graph-required ref in `required_tests.yaml` when its test lands.

- release-version monotonic & single-authority; dev channel cannot allocate release.
- `services desired set` refuses non-SERVICE kind; desired store rejects cross-kind.
- drift/convergence keyed on `build_id`; regressed-desired raises a distinct finding.
- publish rejects non-stripped / oversized release artifact.
- staging keyed by content; verify uses the desired manifest.
- safe typed removal of a desired record per lane.

### 7.5 Code work, phased
- **P1 — authority split:** channel/capability on `AllocateUpload`; `deploy` defaults
  to `dev`; CI allocates via `release` capability; `-trimpath -s` + shape gate.
- **P2 — desired/convergence:** regression guard on all paths; kind-keyed desired
  store + kind-refusing CLI + first-class infra-desired path + safe removal;
  `build_id` in the drift hash; content-addressed staging + desired-manifest verify.
- **P3 — contracts:** land §7.1–7.4 in the awareness graph; wire `principle-check`
  scanners; CI gates all three repos.

---

## 8. Migration & open items

- Quarantine the existing local-debug artifacts that polluted the release stream
  (`xds 1.2.234+1`, `mcp 1.2.234+1`, `ai-memory 1.2.234/235/237+1`) once a clean
  release rebuild exists; do not delete while desired/installed still references them.
- Re-home infrastructure desired records (e.g. xds) out of the SERVICE lane once the
  first-class infra-desired path exists.
- ~~Confirm the exact agent/MCP `package_build`/`package_publish` reservation flow~~
  RESOLVED (§3.3): MCP tools default to `DEV`; unreserved publishes are forced to
  `DEV`; release allocation requires `repository.release.allocate` (CI SA only).

> Related: ai-memory `debug/a399ebea` (the 2026-06-23 root-cause investigation);
> seed `ops.always.package.*`; `intent:package.identity_tuple_must_be_unique`,
> `intent:desired_hash.is_convergence_identity`, `invariant:desired.build_id_immutable`.
