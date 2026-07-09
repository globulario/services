# Package Identity: Single Authority, One-Way Flow

**Status**: ACTIVE — implementation in progress (2026-07-08)
**Scar**: `artifact.desired_build_mismatch` WARN storm after day-1 join of globule-nuc:
`installed build 1783560972 but desired build 1`. The WARN is a symptom; the defect
is dual identity authority.

---

## 1. The defect

Multiple layers mint or rewrite the same package-identity fields, despite existing
contracts (`ops.always.package.identity-rules`, `ops.service.version.identity-source`,
`ops.service.version.zz-generated-file-contract`, CLAUDE.md prime rule 2) declaring
single authorities for each field.

### Verified violation map (all refs verified 2026-07-08)

| Field | Writers found | Contract |
|---|---|---|
| `version` (service pkgs) | 1. committed `zz_version_generated.go` (said 1.2.272 while dist/cluster ran 1.2.270) · 2. `LDFLAGS="-X main.Version=${VERSION}"` = **platform** version, `scripts/build-release.sh:830` · 3. `package.json` rewritten to **platform** version at `:1097`, template synthesis mints platform version at `:418`, and `validate_generated_release_inputs` (`:299-300`) **enforces** template version == platform version | "Platform release is NOT the package version" — CLAUDE.md rule 2, and `gen-version.sh`'s own header |
| `version` (infra pkgs) | `packages/` repo (external), carried through unchanged (`:1015`) | ✅ compliant — must stay untouched |
| `build_number` | 1. `BUILD_NUMBER="$(date +%s)"` `scripts/build-release.sh:945` (timestamp, stamped into every bundle package at `:959`, `:1099`) · 2. repository allocator counter (deploy pipeline) | "int64 counter, display only, never convergence" |
| `build_id` | 1. locally minted `uuid.uuid4()` at `:953` and `:1091` · 2. repository allocator (UUIDv7, post-upload) | "allocated by Repository post-upload; **EMPTY in committed package.json**" |
| bundle provenance | gateway `join_bundle.go:54-165` (Globular repo) re-assembles the bundle **in memory on every request** from live `/var/lib/globular/packages` + `release-index.json`; sha256 computed from the same bytes it just assembled | checksum must prove release-artifact provenance, not transport integrity of a dynamic assembly |

### Self-contradictions (each layer vs its own documentation)

1. `golang/build/gen-version.sh` header: *"Using the platform release as the default
   version stamps ALL packages with the platform version, which violates the BOM
   invariant"* — yet `build-release.sh` does exactly that (ldflags + package.json)
   and never calls `gen-version.sh` at all.
2. `gen-version.sh` header: *"The generated files are .gitignored — do NOT commit
   them"* — yet `zz_version_generated.go` files ARE committed, and had drifted
   (1.2.272 in tree vs 1.2.270 released).
3. Seeded contract: build_id "empty in committed package.json, allocated by
   repository" — yet the release script forges uuid4 build_ids client-side.

### Why the WARN is honest noise

`installed_build=1783560972` is the bundle's `date +%s`. `desired_build=1` is the
repository allocator's counter for the same artifact. Both values are "true" in
their own authority domain — the mints are parallel, so drift is guaranteed, not
detected. The doctor is honestly reporting a fabricated disagreement.

### Why dual authority is dangerous even for a display-only field

- False drift trains operators (and AI agents) to ignore drift findings — alarm
  fatigue erodes the value of every real finding.
- Any future code that "just compares build numbers" (convenient, available)
  silently becomes a convergence trap. The field's existence as two parallel
  truths is the hazard; display-only is a promise about *current* consumers only.
- Locally minted build_ids are worse: they occupy the *sole convergence identity*
  with values the repository never issued. Convergence correctness then depends on
  every path re-assigning them — compensation stacked on compensation.

### Compression waste (secondary, fixed alongside)

A service package is gzipped 3× before the distro (template → identity-restamp
re-tgz `:969` → outer bundle `:1201`); external packages 4× (`sanitize_package_payload:621`
re-gzips unconditionally even when it patches nothing). The join bundle then
re-tars+re-gzips the whole package set per request, server-side.

---

## 2. Target authority flow (one direction, no back-edges)

```
1. Repository allocator decides package identity for changed packages.
2. BOM / release-index.json records package identity for the release.
3. gen-version.sh consumes BOM overrides → materializes version into generated code.
4. Binary --version reports the generated code version.
5. Packaging copies the binary/package version ONCE into package.json.version.
6. build_id and build_number remain empty/unset in the bundle.
7. Day-0 repository admission allocates build_id and build_number.
8. Doctor/convergence never use display-only build_number as convergence authority.
```

Per-field single authorities:

| Field | Sole authority | Materialization path |
|---|---|---|
| service pkg `version` | BOM / package identity (repository allocator for changed pkgs) | BOM → gen-version.sh overrides → zz_version_generated.go → binary --version → package.json (copied once at packaging) |
| infra pkg `version` | `packages/` repo (external) | carried through untouched |
| platform release | git tag → release-index.json `platform_release` → etcd active_release anchor | never written into package version fields |
| `build_id` | repository admission | empty until admission |
| `build_number` | repository admission (monotonic int, display-only) | absent/0 until admission |

## 3. zz_version_generated.go policy decision

**Chosen: committed + CI-gated.** The committed zz file is the in-tree
materialization of the package's BOM identity ("the authority is in the code").
CI gates: (a) file matches the generator's contract (var, generated header,
non-empty sentinel); (b) built binary `--version` == zz == package.json.version
in built artifacts. `gen-version.sh`'s header is updated to match (generated,
committed, CI-verified — not gitignored). ldflags `-X main.Version` remains ONLY
as an explicitly-labeled dev/hotfix escape; the release path must not use it and
CI rejects it in release builds.

## 4. Join bundle provenance

The gateway stops re-assembling bundles per request. Day-0/activation stores the
release artifact byte-for-byte under a release-addressed path
(`/var/lib/globular/releases/<version>/globular-<version>-linux-amd64.tar.gz` +
`.sha256`). The `/join/bundle/` handler serves that stored file; the checksum is
computed at store time and verifies provenance of the released artifact. Dynamic
assembly is permitted only inside explicit release activation/publishing flows.

## 5. Migration notes

- Bundle packages ship `build_id: ""` and no/zero `build_number`;
  `release-index.json` correspondingly carries empty build identity fields.
  Repository admission at day-0 (and upload in steady-state) assigns both, and
  desired state must reference admission-assigned identity only.
- Consumers verified for empty-build_id tolerance before the restamp is deleted
  (see §7 checklist; consumers that assumed pre-admission build_id are fixed to
  key on name/version/sha256 until admission returns identity).
- `.staging` remains as *assembly area only* (staged binaries, staged packages) —
  it must never mutate identity.
- Existing clusters: current bundles with forged identities remain admissible;
  admission continues to (re)assign repository identity, so old bundles converge
  the same way. No cluster-side migration required.

## 6. Rollback notes

- The change is confined to: build scripts (services repo), gateway bundle
  handler (Globular repo), repository admission (already the intended authority),
  doctor expectation logic. Reverting the commits restores the old pipeline
  byte-for-byte; no persistent-state format changes.
- If day-0 with empty-identity bundles fails in the field: the fastest safe
  rollback is re-running build-release.sh from the previous commit (restores
  restamping) — NOT hand-editing package.json in a built bundle.

## 7. Definition of done / gates

1. No local build script mints build_id (grep gate: no uuid minting for identity).
2. No local build script mints build_number (grep gate: no `date +%s` build_number).
3. Release builds do not override service package version with platform release
   (grep gate on `-X main.Version=${VERSION}` in release path).
4. zz == binary --version == package.json.version for each service package.
5. Infra package versions byte-identical from source through bundle.
6. Pre-admission package metadata has empty/absent build_id + build_number.
7. Repository admission assigns both.
8. Served join bundle bytes == stored release artifact bytes (test).
9. Doctor: build_number mismatch is never a convergence-drift classification.
10. Doctor: expected units follow assigned profile (no scylla-on-every-node
    assumption; sibling of the 2026-07-08 dns ExecStartPre local-scylla bug).

## 8. Related

- `docs/design/contract-first-resolution-protocol.md`
- Seeded contracts: `ops.always.package.identity-rules`,
  `ops.service.version.identity-source`, `ops.service.version.zz-generated-file-contract`
- Sibling incident: dns/rbac/workflow/ai_memory systemd local-scylla gate deadlock
  on core-only nodes (fixed in `golang/deploy/specgen.go`, 2026-07-08)
