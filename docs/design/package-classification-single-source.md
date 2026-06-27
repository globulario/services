# Package Classification: Single-Source Authority (registry.yaml)

**Status:** Design — review before any codegen slice
**Authored:** 2026-06-27 (davecourtois + Claude, from the xds `cache_digest_mismatch` arc)
**Scope:** cross-repo — `globulario/packages` + `globulario/services`
**Related:** ai-memory `architecture/83b8f143` (scar: "fix the author, not the copy"); behavioral principle `607e3c39` (PROPOSED); PRs #154/#158/#159/#160

---

## 1. Problem

A Globular package's **classification** — its `kind` and the orthogonal facts that
word smears together — is currently maintained as **six hand-synchronized copies
across two repositories**. They agree only by manual discipline. There is no single
author; there is a committee.

This is the root cause behind a multi-PR arc that kept chasing the same symptom (the
recurring xds `INFRASTRUCTURE/xds cache_digest_mismatch`): a legacy cross-kind
`ServiceRelease/xds@1.2.235` kept reinstalling a stale tarball. Each fix patched a
downstream copy; the classification kept being re-asserted from a different copy.

### 1.1 The six copies of "kind"

| # | Location | Repo | Authored? |
|---|----------|------|-----------|
| 1 | `registry.yaml` `kind` | packages | **declared canonical author** |
| 2 | `metadata/*/package.json` `type` | packages | hand-authored |
| 3 | `metadata/*/awareness.yaml` `package_kind` | packages | hand-authored |
| 4 | `scripts/validate-package-identity.py` `CATALOG_KIND` | packages | hand-authored map of 56 pkgs, comment: *"MUST mirror component_catalog.go"* |
| 5 | `cluster_controller_server/component_catalog.go` `Component.Kind` | services | hand-maintained |
| 6 | `node_agent_server/.../inferPackageKind` | services | hardcoded name map (Day-0) |
| (+) | artifact **manifest** kind | (build output) | derived/stamped — a generated copy |

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
| `component_catalog.go` | **generated** from registry-derived data |
| `validate-package-identity.py::CATALOG_KIND` | **removed** (or a generated input); the gate becomes source-vs-generated |
| node-agent `inferPackageKind` | **generated table or registry-backed lookup** |
| `metadata/*/package.json` `type` | **generated** (or build-checked) from registry |
| `metadata/*/awareness.yaml` `package_kind` | **generated** (or build-checked) from registry |
| artifact **manifest** kind | **emitted** from registry-derived source |

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

1. **Slice 1 — generate `component_catalog.go` from registry-derived data (services repo).**
   Lowest blast radius: self-contained, removes one major services-side hand map, touches
   no live release stores, no Day-0 path. Proof-of-approach. Wire generation into
   `generateCode.sh`; add a `go generate`/CI check that the committed file equals the
   generated output.
2. **Slice 2 — replace `validate-package-identity.py::CATALOG_KIND`** with registry-derived
   data so the gate stops carrying a photocopy.
3. **Slice 3 — generate `package.json type` + `awareness.yaml package_kind`** from
   `registry.yaml` in the packages build (`build.sh`), or build-check them.
4. **Slice 4 — node-agent `inferPackageKind`** becomes a generated table / registry-backed
   lookup (carefully — Day-0 bootstrap reads it before etcd exists; keep it a build-time
   generated table, not a runtime fetch).
5. **Slice 5 — emit manifest kind** from registry-derived source; converge the orthogonal
   axes (form/provenance/criticality/mesh) as first-class authored fields, with `kind`
   demoted to a derived view.
6. **Slice 6 (stretch) — structural:** collapse `ServiceRelease` + `InfrastructureRelease`
   into one `PackageRelease` keyed by package, kind-as-attribute, so a cross-kind release
   record becomes *structurally impossible* (the xds bug class cannot exist). Largest;
   touches proto + resource store + both reconciler paths + live-record migration.

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
  canonical package registry."* — currently violated six ways; this doc is the path to
  satisfying it mechanically.

Once `registry.yaml` is mechanically the single source (generation + source-vs-generated
gate), the behavioral principle `607e3c39` self-narrows (its revocation rule): mechanical
enforcement supersedes agent discipline.
