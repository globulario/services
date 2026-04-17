# Globular Version Control Redesign — Implementation Plan

## 1. Executive Summary

Globular's version management is broken at layer 1 — the artifact/repository layer. The system that should be the bedrock of trust allows:

- Silent overwrite of published artifacts with different content at the same identity
- Non-monotonic version publishing (0.0.5 then 0.0.2)
- Version authority split between package payload, CLI, discovery, and repository
- Build number 0 used as both "legacy/unset" and "latest" semantic
- Hardcoded default version (`0.0.2`) making all services share the same version string
- `--force` always passed during deploy, bypassing all immutability checks
- No atomic version allocation — concurrent deploys can race on build numbers

The result: we just spent hours debugging a cluster where 3 nodes had 3 different installed versions, the repository couldn't serve packages the desired state referenced, and the reconciler was stuck with leaked locks. This is not a cosmetic problem — it's a trust failure in the foundation layer.

The redesign makes the repository the sole authority for version identity, enforces immutability after release, introduces system-generated build IDs, and separates semantic release versions from build artifacts. Package payload metadata becomes informational, never authoritative.

---

## 2. Current-State Analysis

### Where package version comes from

**Multiple conflicting sources:**

1. **CLI `--version` flag** — passed to `globular pkg build` and `globular deploy`. Default: `"0.0.2"` (`deploy/deploy.go:63`).
2. **Spec YAML `metadata.version`** — per-service override in `packages/specs/*.yaml`. Takes precedence over `--version` in `packages/build.sh`.
3. **`package.json` inside .tgz** — written during build (`pkgpack/builder.go:349`), read during publish (`pkg_cmds.go:534-708`). The CLI publish command trusts this as authoritative.
4. **`config.json` / `package.json` for applications** — application deploy paths read version from these files.

**Bottom line:** Version authority leaks from the package payload. The repository receives whatever the client sends.

### Where build number comes from

1. **`globular pkg build --build-number N`** — CLI flag, default 0. Written into `package.json`.
2. **`deploy/buildnumber.go:NextBuildNumber()`** — queries repository for latest published build number, returns `current + 1`. Used by `globular deploy`.
3. **Repository storage** — build number is part of the 5-field storage key: `{publisher}%{name}%{version}%{platform}%{buildNumber}`.

**Race condition:** Two concurrent deploys call `NextBuildNumber()`, both get the same value, both publish with `--force`, one overwrites the other silently.

### Where repository identity is decided

The CLI decides. `UploadArtifactWithBuild()` sends `(publisher, name, version, platform, build_number)` from the client. The repository validates format (`versionutil.Canonical()`) but does not allocate or verify monotonicity.

### Discovery and repository split-brain

Two independent registration paths exist:

1. **Resource service descriptors** — `setPackageDescriptor()` in `pkg_cmds.go` registers via RBAC-aware Resource service during publish.
2. **Artifact manifests** — stored directly in MinIO/local storage by the repository.

If Resource service is down during publish, the artifact is still promoted to PUBLISHED (`publish_workflow.go:72-78` treats descriptor failure as non-fatal). The release resolver (`release_resolver.go`) queries artifact manifests, not Resource service descriptors. Result: two catalogs that can disagree.

### Where overwrite/replacement happens

**`artifact_handlers.go:596-621`** — the critical code:

- Same `(publisher, name, version, platform, build_number)` + same checksum → idempotent skip
- Same identity + **different checksum** → **SILENT OVERWRITE** with warning log
- `deploy.go:269` always passes `--force` to publish

This means every `globular deploy` can silently replace a published artifact with different content. This is the primary source of corruption.

### Day-0 package creation

**`packages/build.sh`:**
- Reads spec YAML files from `packages/specs/`
- Assembles payload (binary, config, systemd units, spec)
- Calls `globularcli pkg build` with global or per-spec version
- Build number defaults to 0
- Output: `.tgz` files in `packages/dist/`

**`globular-installer/`:**
- Tracks installation via `/var/lib/globular/install-manifest.json`
- Records Globular version, file checksums, prefix
- Day-0 packages are built offline and bundled with the installer
- No repository exists yet — packages are installed directly from local .tgz

### Day-1 publish/install/update flow

1. `globular deploy` builds binary, queries `NextBuildNumber()`, packages, force-publishes, sets desired state
2. Controller `release_resolver.go` resolves version from repository (build_number=0 means "latest")
3. Controller dispatches `ApplyPackageRelease` to node agents
4. Node agent downloads from repository, executes spec steps, writes installed state to etcd

---

## 3. Gap Analysis Against Target Invariants

### INV-1: Released package version is immutable
**Violation:** `artifact_handlers.go:614-621` — same version + different checksum = silent overwrite.
**Severity:** CRITICAL. This is the root cause of the version chaos we just experienced.
**Risk:** Any deploy can corrupt any previously published artifact. No audit trail.

### INV-2: Builds are mutable before release, versions are not
**Violation:** No concept of staging vs. released. Artifacts go directly from VERIFIED to PUBLISHED. There is no "candidate build" phase.
**Severity:** HIGH. Cannot safely iterate on a build without risk of overwriting.

### INV-3: Discovery/catalog cannot invent version
**Violation:** CLI publishes to Resource service independently of repository (`setPackageDescriptor()` in `pkg_cmds.go`). Discovery can have descriptors for versions that don't exist in the repository.
**Severity:** MEDIUM. Causes confusion but doesn't directly corrupt artifacts.

### INV-4: Package payload declares content version, not repository version
**Violation:** `pkg_cmds.go` reads version from `package.json` inside the .tgz and uses it as the artifact identity sent to the repository. The package defines its own identity.
**Severity:** HIGH. Version authority is in the payload, not the repository.

### INV-5: Monotonic release order per package
**Violation:** No monotonicity check anywhere. `UploadArtifact` accepts any version string that passes `versionutil.Canonical()`. You can publish 0.0.5 then 0.0.2.
**Severity:** HIGH. Breaks "latest" resolution and confuses convergence.

### INV-6: Build identifiers are system-generated
**Violation:** Build numbers are client-supplied (`--build-number` flag or `NextBuildNumber()` query). The repository does not generate them.
**Severity:** MEDIUM. Enables races and manually-chosen build numbers.

### INV-7: Repair is explicit and auditable
**Violation:** No repair mechanism exists. Corrupted history can only be fixed by manually deleting artifacts from MinIO and rewriting etcd keys. No audit trail.
**Severity:** HIGH. Recovery from the current mess required hours of manual intervention.

### INV-8: `desired set` must verify package exists in repo
**Violation:** `desired_state_handlers.go` writes desired version to etcd without verifying the artifact exists in the repository. You can set desired to a version that doesn't exist.
**Severity:** HIGH. This is exactly what happened — desired 0.0.7 set for services where 0.0.7 didn't exist in the repo.

### INV-9: Version regression guard must include build number
**Violation:** `highestHealthyInstalledVersion()` in `desired_state_handlers.go:268-296` compares only version strings, not `(version, build_number)` tuples.
**Severity:** MEDIUM. Allows downgrade within the same version by build number.

---

## 4. Target Architecture

### Package Identity (continuity key)
```
publisher / package_name
```
Example: `core@globular.io/cluster-controller`

This is the logical entity. It never changes.

### Release Identity (human-facing, immutable once released)
```
version = semver (e.g., 1.4.2)
```
- Allocated or validated by the repository
- Monotonically increasing per package
- Immutable once state reaches RELEASED

### Build Identity (machine-facing, exact artifact instance)
```
build_id = UUIDv7 or ULID (e.g., 01JQ8Y3K...)
build_number = repository-generated monotonic integer (for UX)
```
- Generated only by the repository
- Never client-supplied
- Multiple builds can target the same release version (candidate builds)
- Exactly one build is promoted to RELEASED per version+platform

### Artifact Record
```
Package:   core@globular.io/cluster-controller
Version:   1.4.2
BuildID:   01JQ8Y3K...
BuildNum:  17                          (derived, for display)
Platform:  linux_amd64
Digest:    sha256:abc123...
State:     STAGED | VERIFIED | RELEASED | DEPRECATED | YANKED | REVOKED
CreatedAt: 2026-04-16T17:34:55Z
Provenance:
  GitCommit: abc123
  GitDirty:  false
  Builder:   globular-deploy/0.0.8
Contents:                              (metadata only, non-authoritative)
  BinaryVersion: 1.4.2-dev+git.abc123
  APIVersion:    v1
  Dependencies:  {envoy: 1.35.3}
```

### Source of truth for each field

| Field | Authority | Notes |
|-------|-----------|-------|
| Package name | Repository (first publish) | Canonical, lowercase, hyphens |
| Publisher | Repository (ownership) | Validated against RBAC |
| Version | Repository (allocated or validated) | Monotonic, immutable after RELEASED |
| Build ID | Repository (generated) | Never client-supplied |
| Build number | Repository (generated) | Monotonic per version+platform |
| Digest | Repository (computed on upload) | SHA256 of received bytes |
| State | Repository (lifecycle) | State machine transitions |
| Content metadata | Package payload | Non-authoritative, informational |

---

## 5. Scope of Impacted Components

### Repository (`repository/repository_server/`)
- `artifact_handlers.go` — Kill same-version overwrite. Add immutability enforcement. Add build ID generation. Add monotonicity check.
- `publish_workflow.go` — Add STAGED→VERIFIED→RELEASED state machine. Remove auto-promote.
- New: `version_allocator.go` — `AllocateVersion()`, `AllocateBuildID()`, `PromoteBuild()`
- New: `release_ledger.go` — Per-package release history with monotonic ordering

**Why:** Repository must become the sole version authority.

### Discovery / Resource Service
- Remove `setPackageDescriptor()` from CLI publish path
- Repository emits catalog registration after RELEASED promotion
- Resource service becomes a read-through cache of repository state

**Why:** Eliminate split-brain between discovery and repository.

### CLI (`globularcli/`)
- `pkg_cmds.go` — Publish becomes: upload payload + metadata, receive assigned version/build from repository. Remove `--force` as default. Add `--bump patch|minor|major` intent.
- `pkg_cmds.go` — Remove `setPackageDescriptor()` call (moved to repository)
- `deploy_cmds.go` — Remove hardcoded `"0.0.2"` default version. Use `--bump patch` or repository-allocated version.

**Why:** CLI must be a client of repository authority, not the owner of version semantics.

### Deploy (`deploy/`)
- `deploy.go` — Remove `--force` from publish call. Use repository-allocated build numbers instead of client-side `NextBuildNumber()`.
- `buildnumber.go` — Deprecated. Build numbers generated server-side.

**Why:** Eliminate client-side version/build allocation and force-publish.

### Packages (`packages/`)
- `build.sh` — Per-spec `metadata.version` becomes content metadata, not repository authority. Build still stamps version into `package.json` but repository may override.

**Why:** Package payload must not define its own release identity.

### Installer (`globular-installer/`)
- Day-0 packages get provisional metadata with a `provisional: true` flag
- When day-0 artifacts are imported into repository post-bootstrap, repository assigns authoritative version and build ID
- Installer manifest tracks provisional vs. repository-assigned versions

**Why:** Day-0 must not create version history that conflicts with repository authority once the repository is available.

### Controller (`cluster_controller/`)
- `desired_state_handlers.go` — `upsertOne()` must verify artifact exists in repository before writing to etcd. Version regression guard must include build number.
- `release_resolver.go` — Use new repository API for latest RELEASED artifact. Remove O(N) artifact scanning.
- `release_pipeline.go` — No changes needed if resolver returns correct data.

**Why:** Desired state must never reference non-existent artifacts.

### Node Agent (`node_agent/`)
- `apply_package_release.go` — Verify artifact state is RELEASED (not just PUBLISHED) before installing. Current `CheckArtifactPublished()` check is correct in spirit but needs state name update.

**Why:** Node agent is the last safety boundary.

---

## 6. Required Invariants to Enforce

These are concrete, testable rules:

```
INV-VER-1:  Once an artifact reaches RELEASED state, its (publisher, name, version, platform)
            tuple is sealed. Any upload with same tuple and different digest MUST be rejected
            with AlreadyExists.

INV-VER-2:  For a given (publisher, name), each new RELEASED version MUST be strictly greater
            (semver) than all previously RELEASED versions. Non-monotonic publish MUST be
            rejected with FailedPrecondition.

INV-VER-3:  Build IDs and build numbers MUST be generated by the repository. Client-supplied
            build identifiers MUST be ignored or rejected.

INV-VER-4:  Package payload version (package.json "version" field) is metadata. It MUST NOT
            be used as the authoritative release version. Repository-assigned version takes
            precedence.

INV-VER-5:  `desired set` MUST verify that the specified (name, version, build_number) exists
            in the repository with state=RELEASED before writing to etcd. If not found,
            reject with NotFound.

INV-VER-6:  Discovery/catalog registration MUST occur only after repository promotion to
            RELEASED. No pre-registration.

INV-VER-7:  Overwrite of released artifacts is forbidden. `--force` flag on publish MUST
            only apply to STAGED/VERIFIED builds, never RELEASED.

INV-VER-8:  Any repair operation that modifies version history MUST create an audit record
            with: timestamp, operator, reason, before-state, after-state.

INV-VER-9:  Desired state version regression guard MUST compare (version, build_number)
            tuples, not version strings alone.

INV-VER-10: Day-0 artifacts MUST be marked provisional. On first repository import, they
            receive authoritative version/build assignment.
```

---

## 7. Proposed Data Model Changes

### New fields

| Field | Type | Where | Purpose |
|-------|------|-------|---------|
| `build_id` | string (UUIDv7) | ArtifactManifest | System-generated unique build identifier |
| `state` | enum | ArtifactManifest | STAGED, VERIFIED, RELEASED, DEPRECATED, YANKED, REVOKED |
| `released_at` | timestamp | ArtifactManifest | When promoted to RELEASED |
| `provenance` | message | ArtifactManifest | Git commit, builder, source info |
| `provisional` | bool | ArtifactManifest | Day-0 artifact not yet repository-confirmed |
| `superseded_by` | string | ArtifactManifest | For repair: points to replacement version |

### Existing fields that become metadata only

| Field | Current Role | New Role |
|-------|-------------|----------|
| `package.json.version` | Authoritative identity | Content metadata (informational) |
| `package.json.build_number` | Client-supplied identity | Ignored on upload, repository-generated |
| `config.json.Version` | Application identity source | Content metadata |

### Deprecated fields

| Field | Reason |
|-------|--------|
| `publish_state` (separate file) | Merged into `state` field in manifest |
| Client-side `build_number` in upload request | Repository generates build numbers |

### Release Ledger (new)

Per-package record in repository storage:

```json
{
  "package": "core@globular.io/cluster-controller",
  "latest_released_version": "1.4.2",
  "releases": [
    {
      "version": "1.4.2",
      "released_at": "2026-04-16T17:34:55Z",
      "platforms": {
        "linux_amd64": {
          "build_id": "01JQ8Y...",
          "build_number": 17,
          "digest": "sha256:abc...",
          "size_bytes": 42000000
        }
      }
    },
    {
      "version": "1.4.1",
      "released_at": "2026-04-15T10:00:00Z",
      "platforms": { ... }
    }
  ]
}
```

Storage path: `ledger/{publisher}%{name}.json`

---

## 8. API and Contract Changes

### UploadArtifact (publish)

**Current:** Client sends `(publisher, name, version, platform, build_number, data)`. Repository stores as-is. Same identity + different checksum = overwrite.

**Target:** Client sends `(publisher, name, data, intent)` where intent is one of:
- `BUMP_PATCH` / `BUMP_MINOR` / `BUMP_MAJOR` — repository allocates next version
- `EXACT(version)` — repository validates version > latest and not already released
- `STAGED(version)` — create candidate build for existing unreleased version

Repository response includes assigned `(version, build_id, build_number, digest)`.

**Migration:** Keep accepting old-style requests with explicit version during transition, but enforce immutability. Reject different-checksum overwrites immediately.

### AllocateVersion (new RPC)

```protobuf
rpc AllocateVersion(AllocateVersionRequest) returns (AllocateVersionResponse);

message AllocateVersionRequest {
  string publisher = 1;
  string name = 2;
  VersionIntent intent = 3; // BUMP_PATCH, BUMP_MINOR, BUMP_MAJOR, EXACT
  string exact_version = 4; // only if intent=EXACT
}

message AllocateVersionResponse {
  string version = 1;        // allocated version
  string reservation_id = 2; // short-lived reservation token
}
```

### PromoteBuild (new RPC)

```protobuf
rpc PromoteBuild(PromoteBuildRequest) returns (PromoteBuildResponse);

message PromoteBuildRequest {
  string publisher = 1;
  string name = 2;
  string version = 3;
  string build_id = 4;
  string platform = 5;
}
```

Transitions build from VERIFIED → RELEASED. Updates release ledger. Emits catalog registration.

### QueryLatest (changed)

**Current:** `GetArtifactManifest` with build_number=0 → O(N) scan of all artifacts.

**Target:** `GetLatestRelease(publisher, name)` → reads release ledger, returns latest RELEASED version + build info. O(1).

### DesiredSet (changed)

**Current:** Writes to etcd without verification.

**Target:** Calls `GetArtifactManifest(name, version, build_number)` first. If not found with state=RELEASED, returns NotFound error to caller.

### RepairHistory (new RPC)

```protobuf
rpc RepairHistory(RepairHistoryRequest) returns (RepairHistoryResponse);

message RepairHistoryRequest {
  string publisher = 1;
  string name = 2;
  RepairAction action = 3; // SCAN, QUARANTINE, REVOKE, SUPERSEDE
}

message RepairHistoryResponse {
  repeated RepairFinding findings = 1;
  repeated RepairAction actions_taken = 2;
}
```

---

## 9. Day-0 Impact Analysis

### Current day-0 flow

`packages/build.sh` builds .tgz files with versions from spec YAML. The installer unpacks them directly to `/usr/lib/globular/` — no repository exists yet. Version tracking is via `/var/lib/globular/install-manifest.json` and etcd (once etcd is running).

### Impact of redesign

**Package creation:** Day-0 packages are built offline. They cannot call the repository to allocate versions. Therefore:

1. Day-0 packages MUST include a `provisional: true` flag in their manifest
2. The version in the manifest is a **declared intent**, not a repository-confirmed release
3. Build ID is generated locally (UUIDv7 is stateless, no server needed)

**Bootstrap sequence:**
1. Installer unpacks provisional packages
2. Node agent starts with provisional installed state
3. Repository service starts
4. **Import phase:** Node agent or bootstrap script calls `ImportProvisionalArtifact()` for each installed package
5. Repository validates version, assigns authoritative build number, updates state to RELEASED
6. Node agent updates installed state with repository-confirmed metadata

**Preventing day-0 from reintroducing broken history:**
- Import validates monotonicity against existing ledger
- If day-0 version conflicts with existing releases: reject import, require admin resolution
- Day-0 packages with `provisional: true` are never treated as RELEASED until imported

### Decision point: pre-allocated vs. provisional

**Recommendation:** Use provisional metadata for day-0. Pre-allocation would require the build pipeline to contact a repository that may not exist yet (offline builds). Provisional is simpler and handles air-gapped deployments.

---

## 10. Day-1 Impact Analysis

### Repository publish flow
- CLI no longer owns version authority
- `globular deploy` calls repository's `AllocateVersion` or uses `--bump patch`
- Build number assigned server-side
- No `--force` in normal path
- Publish state machine: upload → STAGED → verify → VERIFIED → promote → RELEASED

### Node join/install
- New nodes joining the cluster receive packages via the reconciler
- Reconciler only dispatches RELEASED artifacts (not STAGED/VERIFIED)
- No change to the fundamental flow, but install resolution becomes more reliable

### Install/update resolution
- Controller's `release_resolver.go` reads release ledger instead of scanning all artifacts
- Build number pinning in desired state verified against repository
- `CompareFull()` used everywhere (version + build_number)

### Desired/install/runtime comparisons
- Version regression guard includes build number
- Hash computation includes build_id for exact match
- Desired state can only reference RELEASED artifacts

### Rollback
- Set desired to a previous RELEASED version
- That version is immutable, so rollback is deterministic
- No risk of "rolling back to a version that was overwritten with different content"

### Cluster-wide convergence
- All nodes converge to the same RELEASED artifact (same digest)
- Eliminates the scenario where different nodes have different binaries at the "same" version

---

## 11. Migration and Repair Strategy

### Phase 1: Scan existing history

```
globular repository scan --all
```

For each package, list all artifacts sorted by publish timestamp. Flag:
- **DUPLICATE_VERSION:** Same `(publisher, name, version, platform)` with different digests
- **NON_MONOTONIC:** Version N published after version N+1
- **ORPHANED_BUILD:** Build number 0 with no explicit versioning
- **MISSING_MANIFEST:** Binary exists without manifest
- **DESCRIPTOR_MISMATCH:** Resource service descriptor disagrees with repository

### Phase 2: Classify

Each artifact gets a classification:
- **VALID:** Consistent with invariants
- **SUPERSEDED:** Replaced by a later artifact (e.g., 0.0.2 published after 0.0.5)
- **DUPLICATE:** Same version with different content — newest wins, older marked REVOKED
- **ORPHANED:** No desired state or installed state references it

### Phase 3: Repair

```
globular repository repair --package cluster-controller --dry-run
```

- Mark SUPERSEDED artifacts as `state=REVOKED, superseded_by=<version>`
- Mark DUPLICATE artifacts (older) as `state=REVOKED`
- Mark ORPHANED artifacts as `state=DEPRECATED`
- Build release ledger from VALID artifacts
- Emit audit log for every state change

### Phase 4: Reconcile installed state

After repair, scan all nodes' installed state in etcd:
- If installed version is REVOKED/DEPRECATED in repo: log warning, do not auto-change
- If installed version has no matching repo artifact: flag for admin review
- Admin can then set desired state to a VALID version to trigger convergence

### Current mess recovery

For the immediate situation (18 services with version chaos):
1. Rebuild all services from current source at version `0.0.8`
2. Publish all to repository
3. Set desired to `0.0.8` for all services
4. All nodes converge to the same binary
5. Mark all pre-0.0.8 artifacts as DEPRECATED via repair tool

---

## 12. Backward Compatibility Strategy

### Read compatibility (maintained)
- Old manifests without `build_id` field: accepted, `build_id` generated on first access
- Old manifests without `state` field: default to RELEASED (grandfather existing artifacts)
- Old manifests without `provisional` flag: treated as non-provisional
- `build_number=0` continues to mean "resolve to latest" in read paths

### Write compatibility (transitional)
- Client-supplied `build_number` in upload: accepted during transition period, logged as deprecation warning. Repository generates its own build number that takes precedence.
- `--force` on publish: still works for STAGED/VERIFIED builds. Rejected for RELEASED builds with clear error message.
- Legacy 4-field storage keys: read-compatible. New writes always use 5-field keys.

### Rejection points (immediate)
- Same version + different checksum + state=RELEASED → reject (Phase 1, no transition)
- `desired set` for non-existent artifact → reject (Phase 1)

### Deprecation timeline
- Phase 1: Enforce immutability, add verification to desired set
- Phase 2 (30 days): Client-supplied build numbers deprecated, warnings emitted
- Phase 3 (60 days): Client-supplied build numbers rejected, all builds server-generated
- Phase 4 (90 days): Legacy 4-field storage keys read-only, no new writes

---

## 13. Incremental Rollout Plan

### Phase 1: Stop the Bleeding (prerequisite)
**Objective:** Prevent new corruption. No behavior change for valid usage.

**Changes:**
1. `artifact_handlers.go` — Reject same-version overwrite when artifact is RELEASED (keep overwrite for STAGED/VERIFIED)
2. `desired_state_handlers.go` — Verify artifact exists in repository before writing desired state
3. `deploy.go` — Remove `--force` from publish call (use normal path)
4. `desired_state_handlers.go` — Version regression guard uses `CompareFull()` (version + build_number)

**What becomes safer:** No more silent overwrites. No more desired state pointing to non-existent artifacts.

### Phase 2: Repository-Owned Build Numbers (prerequisite)
**Objective:** Eliminate client-side build number allocation.

**Changes:**
1. Add `AllocateBuildNumber()` RPC to repository
2. `artifact_handlers.go` — Generate build number server-side during upload
3. `deploy/buildnumber.go` — Deprecated, replaced by server response
4. `pkg_cmds.go` — Read assigned build number from upload response

**What becomes safer:** No more build number races. Build numbers are authoritative.

### Phase 3: Release Ledger (optional but recommended)
**Objective:** O(1) latest version lookups. Monotonicity enforcement.

**Changes:**
1. New `release_ledger.go` in repository
2. `UploadArtifact` writes ledger entry on RELEASED promotion
3. Monotonicity check: reject version < latest released
4. `release_resolver.go` reads ledger instead of scanning artifacts

**What becomes safer:** Non-monotonic publishes rejected. Latest resolution is fast and correct.

### Phase 4: Version Allocation (optional)
**Objective:** Repository owns version allocation.

**Changes:**
1. Add `AllocateVersion()` RPC
2. CLI `deploy` uses `--bump patch` intent
3. Repository computes next version from ledger

**What becomes safer:** CLI cannot invent versions. Version authority is centralized.

### Phase 5: Repair Tooling (cleanup)
**Objective:** Clean up existing corrupted history.

**Changes:**
1. `globular repository scan` command
2. `globular repository repair` command
3. Audit log for all repair actions
4. Admin workflow documentation

### Phase 6: Day-0 Provisional Metadata (cleanup)
**Objective:** Clean day-0/day-1 boundary.

**Changes:**
1. `provisional` flag in manifest
2. `ImportProvisionalArtifact()` RPC
3. Installer marks day-0 packages as provisional
4. Bootstrap import phase after repository starts

---

## 14. Test Strategy

### Unit tests
- `versionutil/` — `CompareFull()` with edge cases (0 build numbers, pre-release, metadata)
- `release_ledger.go` — Monotonicity enforcement, concurrent access
- `version_allocator.go` — Bump logic, exact version validation

### Repository invariant tests
- Upload same version + same checksum → idempotent success
- Upload same version + different checksum + RELEASED → AlreadyExists error
- Upload same version + different checksum + STAGED → new build created
- Upload version < latest released → FailedPrecondition error
- Upload version = latest released + 1 → success
- Promote build → ledger updated, catalog registered

### Property tests
- For any sequence of publish operations, the release ledger is always monotonically ordered
- For any concurrent publish operations, no two builds get the same build number
- For any RELEASED artifact, digest is immutable across all subsequent reads

### Migration tests
- Scan a repository with known anomalies → correct classification
- Repair with `--dry-run` → no state changes
- Repair with `--execute` → audit log complete, ledger consistent

### CLI tests
- `pkg publish` without `--force` against released artifact → error
- `deploy --bump patch` → correct next version allocated
- `services desired set` with non-existent version → NotFound error

### Day-0 tests
- Build provisional package → `provisional: true` in manifest
- Import provisional into repository → authoritative version assigned
- Import provisional with conflicting version → rejection with clear error

### Day-1 tests
- Full deploy cycle: build → publish → desired set → reconcile → installed
- Concurrent deploy of same service → no corruption, one wins
- Rollback to previous RELEASED version → deterministic convergence

### Failure/recovery tests
- Repository down during publish → no half-written state
- Repository down during desired set → desired set rejected (not written)
- Node agent receives STAGED artifact → refuses to install

---

## 15. Risks and Design Decisions

### Decision 1: SemVer strictness
**Options:** (a) Strict semver with auto-bump, (b) Loose version strings
**Recommendation:** Strict semver. Already using `coreos/go-semver`. Enforce canonical form. Auto-bump via `--bump` intent. Exact version allowed but validated against monotonicity.

### Decision 2: Build ID format
**Options:** (a) UUIDv7, (b) ULID, (c) Timestamp + random
**Recommendation:** UUIDv7. Sortable, globally unique, stateless generation (works for day-0). Keep integer build_number as derived display-only field.

### Decision 3: Exact version publish
**Options:** (a) Allow exact version if > latest, (b) Only auto-bump
**Recommendation:** Allow exact version for infrastructure packages with external versioning (etcd 3.5.14, prometheus 3.5.1). Reject if ≤ latest released. This handles third-party packages that have their own version schemes.

### Decision 4: Payload version mismatch policy
**Options:** (a) Warn only, (b) Block if mismatch
**Recommendation:** Warn only. Content version is metadata. Blocking would break existing workflows where binary version stamps lag behind release version. Log the mismatch for traceability.

### Decision 5: Channel support
**Options:** (a) Add channels now, (b) Defer
**Recommendation:** Defer. Single implicit "stable" channel. Channels add complexity without immediate value for a 3-node cluster. Design the data model to support channels later (field exists but defaults to "stable").

### Decision 6: Automatic repair scope
**Options:** (a) Fully automatic, (b) Scan automatic + repair manual
**Recommendation:** Scan automatic, repair requires explicit admin action. Silent history rewriting is exactly the problem we're fixing. Repair must be intentional and auditable.

---

## 16. Final Recommended Implementation Order

1. **Phase 1: Immutability enforcement** — 1-2 days
   - Kill same-version overwrite for RELEASED artifacts
   - Add repo verification to `desired set`
   - Remove `--force` from deploy path
   - Fix version regression guard to include build number

2. **Phase 2: Server-side build numbers** — 2-3 days
   - Repository generates build numbers on upload
   - Client receives assigned build number in response
   - Deprecate `deploy/buildnumber.go`

3. **Phase 3: Release ledger + monotonicity** — 2-3 days
   - Implement release ledger storage
   - Enforce monotonic version ordering
   - Replace O(N) artifact scanning with ledger lookup

4. **Phase 4: Repair tooling** — 1-2 days
   - `globular repository scan`
   - `globular repository repair`
   - Audit logging

5. **Phase 5: Version allocation** — 2-3 days
   - `AllocateVersion()` RPC
   - CLI `--bump` support
   - Remove hardcoded version defaults

6. **Phase 6: Day-0 provisional flow** — 1-2 days
   - Provisional flag in manifests
   - Import RPC
   - Installer updates

7. **Phase 7: Discovery consolidation** — 1 day
   - Remove CLI-side `setPackageDescriptor()`
   - Repository emits catalog registration on RELEASED
   - Resource service becomes read-through cache

**Total estimated scope:** 10-16 days of focused implementation.

**Critical path:** Phases 1-3 must be sequential. Phase 4 can run in parallel with Phase 3. Phases 5-7 are independent and can be parallelized.
