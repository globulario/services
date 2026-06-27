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
3. **Slice 3 — generate `component_catalog.go` (#6).** The hard one. Requires either extending
   `registry.yaml` to a superset (Priority, Capabilities, ManagedUnit, PlatformDefault,
   HealthCheck, Optional — see §3.1) OR generating only the `Kind` field while the rest stays
   hand-authored (a partial-gen that must be clearly delimited). Decide which in this slice;
   do not start it before that decision is reviewed.
4. **Slice 4 — emit manifest kind** from registry-derived source, and *trust it* (delete the
   inferCorrectKind override, already removed in Slice 1).
5. **Slice 5 — orthogonal axes.** Promote `form` / `provenance` / `criticality` / `mesh` to
   first-class authored fields in `registry.yaml`, with `kind` demoted to a derived view.
6. **Slice 6 (stretch) — structural:** collapse `ServiceRelease` + `InfrastructureRelease`
   into one `PackageRelease` keyed by package, kind-as-attribute, so a cross-kind release
   record becomes *structurally impossible* (the xds bug class cannot exist). Largest;
   touches proto + resource store + both reconciler paths + live-record migration.

### 3.1 Feasibility note — `component_catalog.go` is NOT a pure-kind consumer

`Component` carries ~16 fields. Confirmed **absent from `registry.yaml`** today (grep = 0):
`Priority`, `ProvidesCapabilities`/`Capability`, `ManagedUnit`, `PlatformDefault`,
`HealthCheck`, `Optional`. Some of these are arguably controller-runtime concerns rather
than package-authoring concerns. So "generate component_catalog.go from registry.yaml" is
not free — it forces a decision about how much of the controller's runtime model belongs in
the package registry. That is why it is Slice 3, behind the pure name→kind collapses.

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
