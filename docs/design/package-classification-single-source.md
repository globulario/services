# Package Classification: Single-Source Authority (registry.yaml)

**Status:** Design — review before any codegen slice
**Authored:** 2026-06-27 (davecourtois + Claude, from the xds `cache_digest_mismatch` arc)
**Scope:** cross-repo — `globulario/packages` + `globulario/services`
**Related:** ai-memory `architecture/83b8f143` (scar: "fix the author, not the copy"); behavioral principle `607e3c39` (PROPOSED); PRs #154/#158/#159/#160

---

## 1. Problem

A Globular package's **classification** — its `kind` and the orthogonal facts that
word smears together — is currently maintained as **eight hand-synchronized sites
across two repositories** (four hardcoded code-side classifiers + four authored data
copies). They agree only by manual discipline. There is no single author; there is a
committee.

This is the root cause behind a multi-PR arc that kept chasing the same symptom (the
recurring xds `INFRASTRUCTURE/xds cache_digest_mismatch`): a legacy cross-kind
`ServiceRelease/xds@1.2.235` kept reinstalling a stale tarball. Each fix patched a
downstream copy; the classification kept being re-asserted from a different copy.

### 1.1 The eight copies of "kind"

(Originally undercounted as six; the design review found two more — #4 spec
`metadata.kind` and #8 the repository's `inferCorrectKind`.)

| # | Location | Repo | Authored? |
|---|----------|------|-----------|
| 1 | `registry.yaml` `kind` | packages | **declared canonical author** |
| 2 | `metadata/*/package.json` `type` | packages | hand-authored |
| 3 | `metadata/*/awareness.yaml` `package_kind` | packages | hand-authored |
| 4 | `metadata/*/specs/*.yaml` `metadata.kind` | packages | hand-authored (e.g. `xds_service.yaml: kind: infrastructure`) |
| 5 | `scripts/validate-package-identity.py` `CATALOG_KIND` | packages | hand-authored map of 56 pkgs, comment: *"MUST mirror component_catalog.go"* |
| 6 | `cluster_controller_server/component_catalog.go` `Component.Kind` | services | hand-maintained |
| 7 | `node_agent_server/.../inferPackageKind` | services | hardcoded name map (Day-0) |
| 8 | `repository_server/artifact_handlers.go` `inferCorrectKind` (`infraNames`) | services | hardcoded infra-name map; comment: *"Must match CATALOG_KIND … and component_catalog.go"* |
| (+) | artifact **manifest** kind | (build output) | **not even trusted** — `inferCorrectKind` (#8) *overrides* it from the hardcoded list at publish/sync |

**Four code-side hardcoded classifiers** (#5 CATALOG_KIND, #6 component_catalog.go,
#7 inferPackageKind, #8 inferCorrectKind) + four authored data copies (#1–#4). The
manifest kind looks consistent only because #8 force-corrects it — the published
artifact's own declaration is discarded in favor of a goblin mirror.

### 1.2 The decisive finding: the guardian holds a photocopy

`validate-package-identity.py` is the **build-time gate** (run from `build.sh`) that is
supposed to enforce single-source. Yet it validates `registry.yaml` against its **own
hardcoded `CATALOG_KIND` map**, annotated *"MUST mirror component_catalog.go."* The
mechanism meant to prevent drift **contains a copy that itself must be hand-mirrored.**
That is why a live drift scan currently shows **zero drift** — and why that zero is
misleading: it is agreement-by-discipline against a photocopy, not single-authorship.

### 1.3 The conflation (separate from the duplication)

`kind ∈ {service, infrastructure, command, application}` overloads **four orthogonal
axes** into one word, so it cannot express facts the platform needs — e.g. **scylladb
(essential) vs minio (degradable) are both `infrastructure`** and indistinguishable:

| Axis | Question | Already in `registry.yaml` as |
|------|----------|-------------------------------|
| **form** | daemon vs command? | `systemd_unit` / `skip_runtime_check` |
| **provenance** | platform-built vs vendored? | `version_source`, `go_target` |
| **criticality** | does the cluster die without it? | `control_plane_critical`, `bootstrap_tier`, `day0_required` |
| **mesh** | gRPC microservice / authz surface? | `provides` |

Most of the orthogonal truth **already lives in `registry.yaml` as separate fields** —
`kind` is a lossy projection of them.

### 1.4 The resolved axis model (proven by Slice-5 recon)

A recon over all 56 packages established the *actual* decomposition (it corrects an
earlier guess that mesh/provenance determine the service-vs-infra split — they do not):

- **`kind` = `form` ⊕ `role`.** Derivation `command` (if `skip_runtime_check`) else
  `infrastructure` (if `systemd_unit` explicitly set) else `service` reproduces the
  authored kind for **all 56 packages, 0 mismatches**. So `kind` is genuinely a
  projection of two axes: **form** (command/daemon) and **role** (infrastructure/service).
- **`role` is an INDEPENDENT axis — not derivable from provenance or mesh.** Proof: among
  globular daemons with no gRPC mesh, **xds/gateway are `infrastructure` but mcp is
  `service`** — identical provenance and mesh, different role. (mcp is a service that
  exposes no gRPC mesh service; it speaks MCP/HTTP.) So "has mesh" ≠ "is a service".
- **The overlap dissolves on independent axes.** xds = `form=daemon, role=infrastructure,
  provenance=globular, mesh=no`; mcp = `…, role=service, …`; dns = `…, role=service,
  mesh=yes`; etcd = `…, role=infrastructure, provenance=vendored`. "Infra can also be a
  daemon built like a service" is just `role=infrastructure ∧ provenance=globular` — no
  contradiction, because role, provenance, and form are different fields.

| Axis | Values | registry source | independent? |
|------|--------|-----------------|--------------|
| form | command / daemon | `skip_runtime_check` | yes |
| role | infrastructure / service | `systemd_unit`-presence (**proxy**, see below) | yes |
| provenance | globular / vendored | `version_source` | yes |
| mesh | yes / no | `provides` (gRPC ids) | yes |
| criticality | required / degradable / optional | `control_plane_critical`, `bootstrap_tier` | yes |

**Known fragility:** `role` is currently *proxied* by "is `systemd_unit` explicitly set"
(infra packages store an explicit unit — incl. non-`globular-*` ones like
`scylla-server.service`; services use the `globular-<name>` default and store none). This
holds today (0 mismatches) but is a storage convention, not a clean field. Making `role` an
explicit first-class field is the full decomposition (deferred — see §2 / Slice 5 options).

---

## 2. Target model

**`registry.yaml` is the sole authored source of package classification.** Every other
representation is **generated from it** or **drift-gated against it** (source-vs-generated,
never copy-vs-copy). The overloaded `kind` becomes a *derived view* over the orthogonal
axes, which remain authored as discrete fields.

### 2.1 Current copy → future state

| Current copy | Future state |
|--------------|--------------|
| `registry.yaml` | **sole authored source** (kind + orthogonal axes) |
| `metadata/*/package.json` `type` | **generated** (or build-checked) from registry |
| `metadata/*/awareness.yaml` `package_kind` | **generated** (or build-checked) from registry |
| `metadata/*/specs/*.yaml` `metadata.kind` | **generated** (or build-checked) from registry |
| `validate-package-identity.py::CATALOG_KIND` | **removed** (or a generated input); the gate becomes source-vs-generated |
| `component_catalog.go` `Component.Kind` | **generated** — but see §3 feasibility note (Component carries ~6 fields with no registry source today) |
| node-agent `inferPackageKind` | **generated table** (build-time; Day-0 reads it before etcd — must NOT become a runtime fetch) |
| `repository artifact_handlers.go::inferCorrectKind` | **removed** — stop overriding manifest kind from a hardcoded list; trust the registry-emitted manifest |
| artifact **manifest** kind | **emitted** from registry-derived source (and then *trusted*, not re-corrected) |

### 2.2 The new drift gate

Replace "does copy A equal copy B equal copy C" with **"is each generated artifact
byte-identical to what regenerating from `registry.yaml` would produce."** A consumer
that drifts fails the build because it no longer matches its generator output — there
is nothing left to hand-mirror. `validate-package-identity.py` keeps enforcing
binary-name/spec/systemd agreement (its other, still-valuable job), but its `CATALOG_KIND`
reference is deleted in favor of reading registry-derived data.

---

## 3. Migration plan (incremental; each slice independently reviewable)

> **Do NOT begin any slice until this doc is reviewed.** Generating one file while five
> hand-mirrors keep smiling back fixes nothing and risks build/Day-0 breakage. The first
> artifact is this map + plan, not a generated file.

> **Slice ordering principle (corrected by review):** start with consumers that are
> **pure `name → kind`** (no extra fields) — those are trivially generatable from
> registry data with *zero* new registry fields. `component_catalog.go` is NOT such a
> consumer (its `Component` struct carries ~6 fields with no registry source — see
> feasibility note), so it is deferred, not first.

1. **Slice 1 — collapse the services-side pure-kind goblin maps (services repo, self-contained).**
   Build one generated `name → kind` table from registry-derived data and route both
   `repository_server/artifact_handlers.go::inferCorrectKind` (#8) and
   `node_agent_server::inferPackageKind` (#7) through it. Both are pure name→kind, so this
   needs no new registry fields and touches no live release stores. It also **kills the
   manifest-override** in #8 (stop discarding the published artifact's kind). Day-0 caution:
   `inferPackageKind` runs before etcd exists — keep it a **build-time generated table**, not
   a runtime lookup. CI check: committed table == regenerated table. *This is the
   proof-of-approach* — pure name→kind, removes two goblin maps including the harmful one.
2. **Slice 2 — packages-side data copies + gate.** Replace `validate-package-identity.py::CATALOG_KIND`
   (#5) with registry-derived data; generate `package.json type` (#2), `awareness.yaml package_kind`
   (#3), and spec `metadata.kind` (#4) from `registry.yaml` in `build.sh` (or build-check them).
   The gate becomes **source-vs-generated**, not copy-vs-copy.
3. **Slice 3 — source `component_catalog.go`'s `Kind` from the registry projection (#6). ✅ DONE (reduced).**
   Recon (the field-mapping pass before any edit) settled the §3.1 decision: registry **cleanly
   owns only `Kind`** (100% synced) and mostly `Unit`; `Profiles` (53/55) and `ControlPlaneCritical`
   (6) **diverge semantically** from the catalog, and Priority/Capabilities/deps/etc. are pure
   overlay. Full generation would be a large, load-bearing rewrite driven by a big hand-authored
   overlay for **modest marginal benefit — Slice 1 already gates `Kind` drift.** So the reviewed
   decision was the **reduced** path: `buildCatalog()` now derives each `Component.Kind` from the
   `packagekind` projection (`kindFromRegistry`) instead of a hardcoded `KindInfrastructure/…`
   literal — **eliminating copy #6** (not just gating it) with zero behaviour change (Kind was
   fully synced). `Profiles`/`ControlPlaneCritical`/deps stay catalog-authoritative (overlay), per
   the registry-as-author + non-authoritative-overlay model. Full catalog generation is deferred
   (low value vs. risk). **New follow-up surfaced:** registry's `profiles` / `control_plane_critical`
   (and keepalived `systemd_unit`) are divergent/likely-vestigial — see ai-memory
   `architecture/b3ae1cce`; decide vestigial-remove vs reconcile separately.
4. **Slice 4 — emit + trust manifest kind. ✅ DONE (staged 4a/4b).**
   - *4a (emit)*: publish (`handlers.go`) previously hardcoded `Kind=SERVICE` for every
     uploaded package — the root cause behind the read-time correction. Both write paths
     (publish + sync) now stamp the registry-authoritative kind via `registryArtifactKind`.
   - *4b (trust)*: a live audit confirmed all 36 infra/command stored manifests already carry
     the correct kind (`inferCorrectKind` was a proven no-op), so the read-time correction was
     deleted at all 4 sites + `describe_package` simplified — reads now trust the stored kind.
     `registryArtifactKind` is the single write-time stamp.
5. **Slice 5 — axis model + kind=form⊕role gate. ✅ DONE (reduced).** Recon proved the
   decomposition (§1.4): `kind` = `form` ⊕ `role`, derivable from registry with 0 mismatches,
   and `role` is independent (xds vs mcp). The reviewed reduced decision: **document the axis
   model (§1.4) and mechanize the derivation as a gate** — `genkinds` now fails generation if
   any package's authored `kind` disagrees with `command(skip_runtime_check) else
   infrastructure(systemd_unit set) else service`, so `kind` can never drift from its form/role
   signals. No unconsumed projection was added (no services-repo consumer exists for
   provenance/mesh/criticality yet). **Deferred — full decomposition:** promote `role` (and
   `form`/`provenance`/`criticality`/`mesh`) to explicit first-class registry fields and demote
   `kind` to a purely derived view, removing the `systemd_unit`-presence role proxy. That is the
   larger schema+consumer migration (its own multi-PR effort) and also subsumes the registry
   `profiles`/`control_plane_critical` divergence cleanup (ai-memory `architecture/b3ae1cce`).
6. **Slice 6 (stretch) — structural:** collapse `ServiceRelease` + `InfrastructureRelease`
   into one `PackageRelease` keyed by package, kind-as-attribute, so a cross-kind release
   record becomes *structurally impossible* (the xds bug class cannot exist). Largest;
   touches proto + resource store + both reconciler paths + live-record migration.

### 3.1 Feasibility note — `component_catalog.go` is NOT a pure-kind consumer — RESOLVED

`Component` carries ~16 fields. Confirmed **absent from `registry.yaml`** (grep = 0):
`Priority`, `ProvidesCapabilities`/`Capability`, `ManagedUnit`, `PlatformDefault`,
`HealthCheck`, `Optional`; and `Profiles`/`ControlPlaneCritical` are *present but divergent*
in registry (different semantics — see ai-memory `architecture/b3ae1cce`). So full generation
forces a decision about how much controller-runtime model belongs in the registry, for little
gain over Slice 1's existing kind gate.

**Resolution (reviewed):** do the **reduced** Slice 3 — source only `Kind` from the projection
(eliminating copy #6), leave the rest catalog-authoritative. Full generation / registry-superset
is deferred and folded into the orthogonal-axes work (Slice 5) if pursued at all. The keepalived
unit and registry profiles/CPC divergence are tracked as a separate cleanup.

---

## 4. Criticality is multi-dimensional (the one place we may ADD structure)

`control_plane_critical` (a single bool) cannot distinguish:
- **control-plane-required** (xds, gateway) — workloads can't route without it;
- **data-durability-required** (scylladb = package index; minio founding-quorum, CLAUDE.md rule 5) — cluster degrades/loses durability;
- **workload-optional** (blog, catalog).

The doc proposes a `criticality` field (`required | degradable | optional`) plus retaining
`control_plane_critical` / `bootstrap_tier` for the orthogonal "when in bootstrap" axis.
This is the only axis likely to require *new* authored data rather than derivation.

---

## 5. Non-goals / forbidden moves

- **Do NOT weaken `cleanupStaleKindsByDiskTruth`** (node-agent disk-truth arbiter). It was
  a poisoned-evidence victim of this drift, not the cause.
- **Do NOT add another hand-maintained kind map** to make anything pass — that is the
  exact anti-pattern this doc exists to end (ai-memory `architecture/83b8f143`).
- **Do NOT hand-edit a generated copy** once generation lands; fix `registry.yaml`.
- This is not a runtime hot-fix; it is build-time authority work. Runtime symptoms
  (already closed by #159/#160) are not re-litigated here.

---

## 6. Invariants this serves

- `four_layer.layer_has_single_writing_actor` — extended from STATE authority to
  CLASSIFICATION authority.
- `desired.keyed_by_kind_and_name` — kind must be unambiguous and single-sourced.
- Prime Rule 4 (CLAUDE.md): *"Never duplicate package kind classification. Use the
  canonical package registry."* — currently violated eight ways; this doc is the path to
  satisfying it mechanically.

Once `registry.yaml` is mechanically the single source (generation + source-vs-generated
gate), the behavioral principle `607e3c39` self-narrows (its revocation rule): mechanical
enforcement supersedes agent discipline.
