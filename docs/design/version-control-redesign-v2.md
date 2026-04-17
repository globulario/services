# Globular Version Control Redesign — Implementation Plan v2

## 1. Executive Summary

Globular's artifact layer — layer 1 of the four-layer truth model — has no real version authority. The repository accepts whatever the client sends, overwrites released artifacts silently, and lets the CLI, package payload, and discovery service all claim version ownership independently. The result is a system where "version 0.0.7" on one node can contain entirely different bits than "version 0.0.7" on another, and the reconciler has no way to know.

This redesign makes the repository the **allocator and issuer** of all release and build identity — not a validator, not a storage backend, but the authority. Every artifact gets a repository-issued `build_id` (UUIDv7) as its sole authoritative identity. Semantic versions become human-facing release labels allocated by the repository. Build numbers become optional display sugar. Package payloads declare content metadata, never release identity. Desired state can only reference artifacts the repository has confirmed as RELEASED.

---

## 2. Current-State Analysis

*(Unchanged from v1 — the current system analysis remains accurate. See v1 sections 2.1–2.6 for full details.)*

**Summary of critical findings:**

| Problem | Location | Severity |
|---------|----------|----------|
| Silent overwrite of released artifacts | `artifact_handlers.go:614-621` | CRITICAL |
| `--force` always passed during deploy | `deploy.go:269` | CRITICAL |
| Version comes from package payload | `pkg_cmds.go:534-708` | HIGH |
| No monotonicity check | `UploadArtifact` | HIGH |
| `desired set` writes without repo verification | `desired_state_handlers.go` | HIGH |
| Build number races (client-side allocation) | `deploy/buildnumber.go` | HIGH |
| Discovery/repository split-brain | `publish_workflow.go` vs `setPackageDescriptor()` | MEDIUM |
| Hardcoded default version `"0.0.2"` | `deploy.go:63` | MEDIUM |
| Version regression guard ignores build number | `desired_state_handlers.go:268-296` | MEDIUM |

---

## 3. Version Authority Matrix

Every version-related field in the system, with unambiguous ownership.

| Field | Meaning | Authority | Mutability | Where Used | Trusted or Informational |
|-------|---------|-----------|------------|------------|-------------------------|
| **Package name** | Logical entity identity | Repository (first publish) | Immutable | Everywhere | Trusted |
| **Publisher** | Namespace owner | Repository (RBAC) | Immutable per package | Publish, resolution, RBAC | Trusted |
| **Release version** | Human-facing semver label | Repository (allocated or validated) | Immutable once RELEASED | Desired state, UI, CLI, resolution | Trusted |
| **Build ID** | Unique artifact instance identity | Repository (generated, UUIDv7) | Immutable | Storage key, convergence, installed state, resolution | **Authoritative identity** |
| **Digest** | Content fingerprint (SHA256) | Repository (computed on upload) | Immutable | Integrity verification, convergence | **Authoritative content identity** |
| **Build number** | Human-readable monotonic counter | Repository (derived from build_id sequence) | Immutable once assigned | Display, CLI output, logs | Informational (NOT used in convergence) |
| **Package format version** | Spec schema version (`version: 1` in YAML) | Spec author | Immutable per spec schema | Package installer, node agent | Trusted (structural) |
| **Binary/application version** | Version compiled into the binary | Build pipeline | Changes per build | `--version` flag, `--describe` output | Informational |
| **API/compatibility version** | Protocol or schema compat level | Developer | Changes per release | Dependency resolution, compatibility checks | Informational |
| **Installer/bootstrap provenance** | Day-0 origin metadata | Installer | Immutable after install | Bootstrap import, audit | Informational until imported |
| **Platform** | Target OS/arch | Build pipeline | Immutable per artifact | Storage key, resolution, install | Trusted |
| **State** | Artifact lifecycle phase | Repository (state machine) | Transitions only forward (except repair) | Resolution, install guard, desired set validation | Trusted |
| **Provisional flag** | Day-0 pre-repository marker | Installer | Cleared on import | Bootstrap import flow | Trusted |

### Key rule

Any comparison that determines whether two artifacts are "the same thing" MUST use `build_id` or `digest`. Never `(version + build_number)`. The pair `(version, build_number)` is for human display only.

---

## 4. Target Architecture

### 4.1 Identity Layers

**Package identity** (continuity key, never changes):
```
publisher / package_name
```

**Release identity** (human-facing, immutable once RELEASED):
```
version = strict semver (e.g., 1.4.2)
```
- Allocated or validated by repository
- Monotonically increasing per package
- Sealed once any build reaches RELEASED state for that version

**Build identity** (machine-facing, the sole authoritative artifact identity):
```
build_id = UUIDv7 (e.g., 0193a7f2-8b4c-7def-9012-abc123456789)
```
- Generated by repository on upload
- Globally unique, sortable by creation time
- The ONLY field used for exact artifact matching in convergence, installed state, and resolution
- `build_number` is a derived monotonic counter for human readability — never used in logic

**Content identity** (integrity):
```
digest = sha256:<hex>
```
- Computed by repository from received bytes
- Used for integrity verification and idempotency detection
- Two artifacts with the same digest contain identical bits regardless of version/build metadata

### 4.2 Artifact Record

```
Package:     core@globular.io/cluster-controller
Version:     1.4.2
BuildID:     0193a7f2-8b4c-7def-9012-abc123456789    ← AUTHORITATIVE
BuildNum:    17                                       ← derived, display only
Platform:    linux_amd64
Digest:      sha256:abc123...                         ← AUTHORITATIVE
State:       STAGED | VERIFIED | RELEASED | DEPRECATED | YANKED | REVOKED
CreatedAt:   2026-04-16T17:34:55Z
ReleasedAt:  2026-04-16T18:00:00Z                    (only if RELEASED)
Provenance:
  GitCommit: abc123
  GitDirty:  false
  Builder:   globular-deploy/0.0.8
  SourceRepo: github.com/globulario/services
Contents:                                             ← non-authoritative metadata
  BinaryVersion: 1.4.2-dev+git.abc123
  APIVersion:    v1
  Dependencies:  {envoy: 1.35.3}
```

### 4.3 State Machine

**Build lifecycle:**
```
STAGED → VERIFIED → RELEASED
           ↓            ↓
        REJECTED    DEPRECATED → YANKED → REVOKED
```

- `STAGED`: Bytes received, not yet verified
- `VERIFIED`: Checksum confirmed, manifest valid, eligible for promotion
- `RELEASED`: Promoted, immutable, available for install
- `DEPRECATED`: Superseded by newer release, still installable
- `YANKED`: Removed from "latest" resolution, still downloadable for pinned installs
- `REVOKED`: Hard removal, not downloadable. Used for security issues or repair

**Transitions:**
- Forward only (STAGED→VERIFIED→RELEASED→DEPRECATED→YANKED→REVOKED)
- No backward transitions except via explicit repair with audit record
- Only RELEASED artifacts are visible to desired state resolution and install flows

---

## 5. Repository Authority Model

The repository is the **allocator and issuer** of identity, not a validator of client claims.

### 5.1 Publish Flow (new)

```
1. CLIENT  →  AllocateUpload(publisher, name, platform, version_intent)
               version_intent: BUMP_PATCH | BUMP_MINOR | BUMP_MAJOR | EXACT("1.4.2")

   REPO    ←  UploadReservation {
                 version: "1.4.2",           // allocated or validated
                 reservation_id: "res_xxx",  // short-lived token (5 min TTL)
                 build_id: "0193a7f2...",     // pre-assigned
                 build_number: 17            // derived
               }

2. CLIENT  →  UploadArtifact(reservation_id, data_stream)
               // streams binary bytes under the reservation

   REPO    ←  UploadResult {
                 build_id: "0193a7f2...",
                 digest: "sha256:abc...",
                 state: STAGED,
                 size_bytes: 42000000
               }

3. REPO    →  (internal) verify manifest, checksum, structure
              state: STAGED → VERIFIED

4. CLIENT  →  PromoteRelease(build_id)
              // or auto-promote if policy allows

   REPO    ←  ReleaseResult {
                 version: "1.4.2",
                 build_id: "0193a7f2...",
                 state: RELEASED,
                 released_at: "2026-04-16T18:00:00Z"
               }
              // repo updates release ledger
              // repo emits catalog registration to Resource service
```

### 5.2 Uploads Without Prior Allocation

Allowed for backward compatibility during transition. The repository:
1. Receives `(publisher, name, version, platform, data)` from legacy clients
2. Generates `build_id` and `build_number` server-side
3. Validates version against monotonicity (rejects if ≤ latest RELEASED)
4. Checks immutability (rejects if version already RELEASED with different digest)
5. Stores as VERIFIED (legacy path auto-verifies)
6. Requires explicit `PromoteRelease` call (no auto-promote)

### 5.3 Concurrency and Atomicity

- `AllocateUpload` acquires a short-lived reservation (5 min TTL) keyed on `(publisher, name, version, platform)`
- Only one reservation per key at a time. Second caller gets `ResourceExhausted` with retry hint
- Reservation expires if upload doesn't complete within TTL
- `PromoteRelease` is atomic: reads ledger, validates monotonicity, writes ledger + state in single transaction
- Failed promotion leaves artifact in VERIFIED state (can be retried or cleaned up)

### 5.4 Race Prevention

| Scenario | Behavior |
|----------|----------|
| Two clients allocate same version concurrently | Second gets `ResourceExhausted`, retries with next version |
| Upload completes but promote fails | Artifact stays VERIFIED, retryable |
| Client crashes after allocation | Reservation expires after TTL, version freed |
| Two clients upload different versions concurrently | Both succeed (different reservations) |

---

## 6. Desired State Semantics

### 6.1 Two Modes

**Semantic desired state** (normal operation):
```
etcd: /globular/resources/ServiceDesiredVersion/cluster-controller
value: { "version": "1.4.2" }
```
- User expresses intent: "I want version 1.4.2"
- Controller resolves to exact artifact via release ledger: finds the RELEASED `build_id` for version 1.4.2 on the target platform
- Resolution happens at reconciliation time, not at `desired set` time

**Exact desired state** (pinned deployment):
```
etcd: /globular/resources/ServiceDesiredVersion/cluster-controller
value: { "version": "1.4.2", "build_id": "0193a7f2..." }
```
- Fully pinned to a specific artifact
- Used for rollback, debugging, or when exact binary identity matters
- Controller skips resolution — uses the pinned `build_id` directly

### 6.2 Validation at Write Time

`desired set` MUST:
1. Call repository `GetReleaseInfo(publisher, name, version)`
2. Verify at least one RELEASED artifact exists for the specified version
3. If `build_id` is specified, verify it matches a RELEASED artifact
4. If repository is unreachable, REJECT the write (fail closed)
5. Store the validated version (and optionally `build_id`) in etcd

### 6.3 Resolution at Reconciliation Time

Controller resolves semantic desired state → exact artifact:
1. Read desired version from etcd
2. If `build_id` is present → use directly
3. If only version → query release ledger for RELEASED build_id on target platform
4. Compare resolved `build_id` against installed `build_id` on each node
5. If different → dispatch install workflow

### 6.4 Rollback

1. Admin runs `globular services desired set cluster-controller 1.4.1`
2. Validation confirms 1.4.1 exists as RELEASED in repository
3. Controller resolves to 1.4.1's `build_id`
4. Nodes converge: download 1.4.1 artifact (immutable, guaranteed same bits as original)
5. Deterministic rollback — no risk of content mutation

---

## 7. Day-0 vs Day-1 Semantics

### 7.1 Day-0: No Repository Available

**What installed packages have:**
- `version`: declared in spec YAML (intent, not confirmed)
- `build_id`: generated locally (UUIDv7 is stateless)
- `digest`: computed locally from .tgz content
- `provisional`: `true`
- `state`: `PROVISIONAL` (not `RELEASED`)

**What node-agent reports in installed state:**
```json
{
  "name": "cluster-controller",
  "version": "1.0.0",
  "build_id": "0193a7f2...",
  "digest": "sha256:abc...",
  "provisional": true,
  "status": "installed"
}
```

**What is trusted:** `digest` (content is real). Everything else is provisional metadata that may be reassigned during import.

### 7.2 Bootstrap Transition

When repository service starts for the first time:

```
1. Node agent detects repository is available
2. For each provisional installed package:
   a. Call ImportProvisionalArtifact(name, version, digest, provisional_build_id, tgz_data)
   b. Repository checks:
      - Is this version already in the ledger?
        - YES + same digest: link to existing release, return confirmed build_id
        - YES + different digest: REJECT (conflict, requires admin resolution)
        - NO: accept as new release, assign confirmed build_id, add to ledger
   c. Repository returns: { confirmed_build_id, confirmed_version, state: RELEASED }
3. Node agent updates installed state:
   - Replace provisional build_id with confirmed build_id
   - Set provisional = false
4. Controller can now reconcile normally
```

### 7.3 After Import

- All installed packages have repository-confirmed identities
- `provisional` flag is cleared
- Release ledger contains day-0 packages as proper releases
- Normal day-1 convergence applies

### 7.4 Conflict Resolution

If day-0 version conflicts with existing repository state:
- **Same version + same digest:** Idempotent, link to existing release
- **Same version + different digest:** Reject import. Admin must either:
  - Republish day-0 package at a new version
  - Or revoke the existing release and re-import
- **Version < latest released:** Reject (non-monotonic). Admin must bump version.

Bootstrap artifacts NEVER silently become authoritative releases. The import is an explicit, validated operation.

---

## 8. Discovery / Resource Service Impact

### 8.1 Current Consumers of Resource Service Descriptors

| Consumer | What it reads | Impact of delayed registration |
|----------|---------------|-------------------------------|
| MCP `pkg_info` tool | Package metadata for display | Stale display until registered — acceptable |
| CLI `pkg list` | Available packages | Missing from list until registered — acceptable |
| Node agent `sync repo artifacts` | Available packages for pre-fetch | Uses repository directly, not Resource service — no impact |
| Release resolver | Artifact manifest for install | Uses repository directly — no impact |
| RBAC | Package ownership | Must remain — but can be populated from repository |

### 8.2 New Flow: Repository → Discovery Projection

After `PromoteRelease`:
1. Repository writes artifact manifest and updates release ledger
2. Repository calls Resource service `SetPackageDescriptor()` with confirmed metadata
3. If Resource service is unavailable: queue for retry (reconciler loop, not fire-and-forget)
4. Resource service descriptor is always a **projection** of repository state

**Key change:** Registration moves from CLI (unreliable, can diverge) to repository (authoritative, retried until consistent).

### 8.3 What Breaks During Transition

- Nothing breaks immediately. The CLI currently does both: upload to repo + register descriptor. Removing CLI-side registration just means the repo takes over that responsibility.
- During the transition period, both paths can coexist (CLI registers, repo also registers). Redundant but safe.
- After transition: CLI registration removed, repo is sole registrar.

---

## 9. Failure Scenarios

| Scenario | Expected Behavior | Invariant Preserved | Recovery |
|----------|-------------------|---------------------|----------|
| **Concurrent publish of same version** | Second allocation gets `ResourceExhausted` | Only one reservation per (publisher, name, version, platform) | Retry with next version or wait for reservation expiry |
| **Repository crash during upload** | Artifact in STAGED state (incomplete) | No RELEASED artifact created | Garbage collect incomplete STAGED artifacts on startup |
| **Partial upload (client disconnects)** | Artifact in STAGED with partial data | Not promotable (verification will fail) | Cleanup by repository GC |
| **Failed verification** | Artifact stays STAGED, marked REJECTED | Not visible to resolution | Admin can retry upload or clean up |
| **Conflicting day-0 import** | Import rejected with conflict details | Existing release unchanged | Admin resolves: bump version or revoke existing |
| **Desired state references revoked artifact** | Reconciler detects REVOKED state, emits warning event | No install attempted | Admin must set new desired version |
| **Network partition: nodes can't reach repository** | Node agent logs "repository unreachable", continues running current version | Installed state unchanged | Resolves when network recovers |
| **Desired set when repository unreachable** | `desired set` rejected (fail closed) | No phantom desired state written | Retry when repository is reachable |
| **Promote called on already-RELEASED version** | Idempotent if same build_id, rejected if different | Release immutability | No action needed |
| **Repository storage corruption (MinIO)** | Digest mismatch detected on download | Node agent rejects corrupted artifact | Republish from source, revoke corrupted release |

---

## 10. Required Invariants

Minimal, strict, enforceable, testable. No redundancy.

```
INV-1  RELEASED artifacts are immutable.
       Upload with same (publisher, name, version, platform) where a RELEASED
       artifact exists and digest differs → REJECT AlreadyExists.

INV-2  Release versions are monotonic per package.
       For (publisher, name), a new RELEASED version must be strictly greater
       (semver) than all existing RELEASED versions → REJECT FailedPrecondition.

INV-3  build_id is the sole authoritative artifact identity.
       Generated only by the repository. Never client-supplied.
       All convergence, resolution, and installed-state comparisons use
       build_id or digest, never (version + build_number).

INV-4  build_number is derived, non-authoritative.
       It is a monotonic display counter derived from build_id ordering.
       No system logic depends on it. It is never used in comparisons.

INV-5  Package payload version is metadata, not identity.
       repository-assigned version takes precedence.
       Payload "version" field is stored as contents.binary_version.

INV-6  desired set requires repository confirmation.
       Writing a ServiceDesiredVersion to etcd requires the repository to
       confirm a RELEASED artifact exists for that (name, version).
       Repository unreachable → REJECT, not default.

INV-7  Only RELEASED artifacts are installable.
       Node agent must verify state=RELEASED before applying.
       STAGED, VERIFIED, DEPRECATED, YANKED, REVOKED → refuse install.

INV-8  Discovery reflects repository, never leads it.
       Catalog registration occurs only after PromoteRelease succeeds.
       CLI never registers descriptors independently.

INV-9  Day-0 artifacts are provisional until imported.
       Provisional artifacts are not RELEASED. They cannot be referenced
       by desired state. Import is explicit and validated.

INV-10 Repair never silently rewrites history.
       All state changes during repair produce an audit record with
       timestamp, operator, reason, before-state, after-state.
```

---

## 11. Proposed Data Model Changes

### 11.1 Storage Key Migration

**Current:** `{publisher}%{name}%{version}%{platform}%{buildNumber}`

**Target:** `{publisher}%{name}%{build_id}`

The `build_id` (UUIDv7) is globally unique, so the key simplifies to a 3-field structure. Version and platform are manifest fields, not key components.

**Legacy read compatibility:** Repository maintains a secondary index:
```
index/version/{publisher}%{name}%{version}%{platform} → [build_id_1, build_id_2, ...]
```
This allows legacy queries by version+platform while the authoritative key uses build_id.

### 11.2 Release Ledger

Per-package record, stored at `ledger/{publisher}%{name}.json`:

```json
{
  "package": "core@globular.io/cluster-controller",
  "latest_released": {
    "version": "1.4.2",
    "build_id": "0193a7f2...",
    "released_at": "2026-04-16T18:00:00Z"
  },
  "releases": [
    {
      "version": "1.4.2",
      "state": "RELEASED",
      "released_at": "2026-04-16T18:00:00Z",
      "platforms": {
        "linux_amd64": {
          "build_id": "0193a7f2...",
          "digest": "sha256:abc...",
          "size_bytes": 42000000
        }
      }
    }
  ]
}
```

### 11.3 Installed State (etcd)

**Current:**
```json
{
  "name": "cluster-controller",
  "version": "0.0.7",
  "buildNumber": "16",
  "status": "installed"
}
```

**Target:**
```json
{
  "name": "cluster-controller",
  "version": "1.4.2",
  "build_id": "0193a7f2...",
  "digest": "sha256:abc...",
  "provisional": false,
  "status": "installed",
  "installed_at": "2026-04-16T18:05:00Z"
}
```

`build_number` removed from installed state. Convergence uses `build_id` comparison.

### 11.4 Desired State (etcd)

**Current:**
```json
{ "version": "0.0.7", "build_number": 1 }
```

**Target:**
```json
{ "version": "1.4.2" }
```
Or pinned:
```json
{ "version": "1.4.2", "build_id": "0193a7f2..." }
```

`build_number` removed. Resolution uses release ledger to find `build_id`.

### 11.5 Fields Summary

| Field | Added/Changed/Removed | Notes |
|-------|----------------------|-------|
| `build_id` | Added | UUIDv7, authoritative identity |
| `state` | Added (replaces `publish_state` file) | Lifecycle enum in manifest |
| `released_at` | Added | Timestamp of RELEASED promotion |
| `provenance` | Added | Git commit, builder info |
| `provisional` | Added | Day-0 marker |
| `contents` | Added | Non-authoritative payload metadata |
| `build_number` (in manifest) | Changed → derived | Still present, not authoritative |
| `build_number` (in installed state) | Removed | Replaced by `build_id` |
| `build_number` (in desired state) | Removed | Version-only or version+build_id |
| `publish_state` (separate file) | Removed | Merged into `state` field |

---

## 12. Migration and Repair Strategy

### 12.1 Detection (automatic, read-only)

```
globular repository scan [--package <name>] [--all]
```

Scans all artifacts and classifies each into exactly one category:

| Category | Definition | Example |
|----------|-----------|---------|
| `VALID` | Consistent with all invariants | Normal artifact |
| `DUPLICATE_DIGEST` | Same (publisher, name, version, platform), same digest, different build_number | Idempotent re-upload |
| `DUPLICATE_CONTENT` | Same (publisher, name, version, platform), different digest | Overwritten artifact |
| `NON_MONOTONIC` | Version N published after version M where M > N | 0.0.2 after 0.0.5 |
| `ORPHANED` | Not referenced by any desired state or installed state | Old build |
| `MISSING_MANIFEST` | Binary exists without manifest | Storage corruption |
| `PROVISIONAL` | Day-0 artifact not yet imported | Pre-bootstrap |

**Output:** Classification report, no state changes. Idempotent. Safe to run repeatedly.

### 12.2 Classification (labels, not modifications)

Each artifact receives a label stored alongside its manifest:
```json
{ "repair_classification": "DUPLICATE_CONTENT", "classified_at": "..." }
```

Labels are informational. They do not change artifact state or affect resolution.

### 12.3 Repair (explicit operator action)

```
globular repository repair --package <name> --action <action> [--dry-run]
```

**Available actions:**

| Action | What it does | Auto or Manual |
|--------|-------------|----------------|
| `deprecate-orphans` | Mark ORPHANED artifacts as DEPRECATED | Auto-safe |
| `deduplicate` | For DUPLICATE_DIGEST: keep newest, mark others DEPRECATED | Auto-safe |
| `quarantine-conflicts` | For DUPLICATE_CONTENT: mark ALL versions YANKED, require admin choice | Manual |
| `revoke` | Mark specific artifact REVOKED | Manual |
| `build-ledger` | Construct release ledger from VALID artifacts | Auto-safe |
| `full-scan-and-label` | Classify + label all artifacts | Auto-safe |

**Rules:**
- `--dry-run` is always available and default for destructive actions
- DUPLICATE_CONTENT (same version, different content) is NEVER auto-resolved. Operator must choose which content is correct.
- NON_MONOTONIC artifacts are labeled but not auto-modified. Operator decides: revoke, re-version, or accept as historical.
- ALL repair actions produce audit records:
  ```json
  {
    "action": "revoke",
    "artifact": "core@globular.io/cluster-controller@0.0.7/build_0193...",
    "operator": "dave@globular.io",
    "reason": "duplicate content: different digest at same version",
    "before_state": "RELEASED",
    "after_state": "REVOKED",
    "timestamp": "2026-04-16T20:00:00Z"
  }
  ```

---

## 13. Backward Compatibility Strategy

### Read compatibility (maintained indefinitely)
- Old manifests without `build_id`: assigned a deterministic `build_id` derived from `sha256(publisher + name + version + platform + build_number)` on first read. Logged as migration.
- Old manifests without `state`: default to RELEASED (grandfather rule)
- Legacy 4-field storage keys: readable via version index. New writes use `build_id`-keyed storage.
- `build_number=0` in queries: resolve to latest RELEASED (backward compat alias)

### Write compatibility (transitional, 90-day window)
- Client-supplied `build_number` in upload: accepted but **ignored**. Repository generates its own. Deprecation warning logged.
- Client-supplied `version` without allocation: accepted if version > latest RELEASED (monotonic). Reservation created implicitly.
- `--force` on publish: works for STAGED/VERIFIED. Rejected for RELEASED with `FailedPrecondition("artifact is RELEASED and immutable")`.

### Rejection points (immediate, no transition)
- Same version + different digest + RELEASED → `AlreadyExists` (day 1)
- `desired set` for non-existent artifact → `NotFound` (day 1)

---

## 14. Incremental Rollout Plan

### Phase 1: Immutability + Desired State Guard — COMPLETE
**Objective:** Stop new corruption. Zero behavior change for valid usage.
**Status:** Implemented and deployed (April 2026).

- `artifact_handlers.go`: `isTerminalState()` rejects overwrite of PUBLISHED/DEPRECATED/YANKED/QUARANTINED/REVOKED artifacts with different digest
- `desired_state_handlers.go`: `validateArtifactInRepo()` queries repository before writing desired state; returns build_id; fails closed with `codes.Unavailable` when repository unreachable
- Convergence: `isServiceConverged()` uses build_id as sole identity — no version/build_number fallback

### Phase 2: Repository-Issued Build Identity — COMPLETE
**Objective:** Repository generates `build_id` for every upload.
**Status:** Implemented and deployed (April 2026).

- `artifact_handlers.go`: Generates UUIDv7 `build_id` on upload (line 740). Returns in `UploadArtifactResponse`
- Proto: `ArtifactManifest.build_id` (field 42), `InstalledPackage.build_id` (field 14), `ApplyPackageReleaseRequest.build_id` (field 11)
- Desired state: `ServiceDesiredVersionSpec.BuildID` written by controller after resolution
- Installed state: node-agent persists `buildId` after synchronous restart + health verification
- Convergence: `release_pipeline.go` and `workflow_release.go` compare build_id only — no fallback
- Backfill migration: `MigrateBuildIDs()` assigns deterministic UUIDv5 to pre-Phase-2 artifacts
- Workflow dispatch: `RunPackageReleaseWorkflow()` carries `resolved_build_id` through workflow inputs to action handlers to `ApplyPackageReleaseRequest`

### Phase 3: Release Ledger + Monotonicity — COMPLETE
**Objective:** O(1) latest resolution. Non-monotonic publish rejected.
**Status:** Implemented (April 2026).

- `release_ledger.go`: persistent per-package release record stored in MinIO + ScyllaDB
  - `appendToLedger()` — writes entry on promote, enforces monotonic version ordering
  - `getLatestRelease()` — O(1) lookup of latest build_id by platform
  - `MigrateReleaseLedger()` — idempotent startup migration from existing PUBLISHED artifacts
- `artifact_handlers.go`: monotonic enforcement in `UploadArtifact` — rejects version < latest PUBLISHED
- `publish_workflow.go`: `completePublish()` calls `appendToLedger()` after successful promote
- `release_resolver.go`: deterministic version → build_id resolution via manifest lookup

### Phase 4: Allocation Protocol — COMPLETE
**Objective:** Repository owns version allocation.
**Status:** Implemented (April 2026).

- Proto: `AllocateUpload` RPC, `VersionIntent` enum (BUMP_PATCH/MINOR/MAJOR/EXACT), `AllocateUploadRequest/Response`
- `allocate_upload.go`:
  - `AllocateUpload()` handler — resolves version from intent, generates build_id (UUIDv7), creates 5-min reservation
  - `reservationStore` — in-memory reservation management with TTL expiry
  - `bumpVersion()` — increments semver components
  - `resolveVersionIntent()` — computes version from intent, validates monotonicity
  - `startReservationCleanup()` — background goroutine expires stale reservations
- `UploadArtifactRequest`: `reservation_id` field (6) for linking upload to allocation
- Legacy uploads without reservation still supported (backward compat)

### Phase 5: Repair Tooling — COMPLETE
**Objective:** Clean up existing history.
**Status:** Implemented (April 2026).

- `globular state canonicalize --dry-run` — scans repository (A4), desired-state (A2), installed-state (A1/A3/A7)
- `globular state canonicalize --fix-safe` — repairs desired-state missing build_id via controller API
- `globular state canonicalize --fix-installed --node <id>` — repairs installed-state via ApplyPackageRelease or `--metadata-only` etcd write
- `globular state canonicalize --cleanup-ghosts` — deletes installed-state records on non-active nodes
- `globular repository scan [--package <name>]` — classifies all artifacts: VALID/DUPLICATE_DIGEST/DUPLICATE_CONTENT/ORPHANED/MISSING_BUILD_ID
- `audit_log.go`: `writeAuditRecord()` persists every repair mutation to etcd at `/globular/audit/` (INV-10)

### Phase 6: Day-0 Provisional Flow — COMPLETE
**Objective:** Clean day-0/day-1 boundary.
**Status:** Implemented (April 2026).

- Proto: `provisional` flag in `InstalledPackage` (field 15) and `ArtifactManifest` (field 43)
- Proto: `ImportProvisionalRequest/Response` messages, `ImportProvisionalArtifact` RPC
- `import_provisional.go`:
  - `ImportProvisionalArtifact()` handler — validates version/digest against release ledger
  - Same version + same digest → idempotent (returns existing build_id)
  - Same version + different digest → rejects (conflict, requires admin resolution)
  - New version → accepts, assigns confirmed build_id, adds to ledger as RELEASED
  - Stores binary + manifest in repository if data provided
- INV-9: Day-0 artifacts are provisional until imported

### Phase 7: Discovery Consolidation — COMPLETE
**Objective:** Single source of catalog truth.
**Status:** Implemented (April 2026).

- `publish_workflow.go`: `completePublish()` registers descriptor in Resource service after promotion (authoritative path)
- `publish_reconciler.go`: retries stuck VERIFIED artifacts, including failed descriptor registrations
- CLI `pkg register` command marked TRANSITIONAL — repository is the authoritative registrar (INV-8)
- Dual registration is safe during transition (CLI + repository both register; redundant but harmless)

---

## 15. Test Strategy

### Invariant tests (repository)
- INV-1: Upload same version + different digest + RELEASED → `AlreadyExists`
- INV-1: Upload same version + same digest + RELEASED → idempotent success
- INV-2: Upload version < latest RELEASED → `FailedPrecondition`
- INV-2: Upload version = latest RELEASED + patch → success
- INV-3: Upload response contains server-generated `build_id`
- INV-3: Client-supplied `build_id` in request is ignored
- INV-4: `build_number` in response is derived, not from client
- INV-6: `desired set` with non-existent version → `NotFound`
- INV-6: `desired set` with STAGED version → `NotFound` (not RELEASED)
- INV-7: Node agent refuses install of non-RELEASED artifact
- INV-8: Discovery registration only after promote

### Concurrency tests
- Two concurrent `AllocateUpload` for same version → one succeeds, one gets `ResourceExhausted`
- Two concurrent uploads for different versions → both succeed
- Promote during concurrent upload → safe (different build_ids)

### Migration tests
- Legacy manifest without `build_id` → gets deterministic `build_id` on read
- Legacy manifest without `state` → defaults to RELEASED
- Scan repository with known anomalies → correct classification
- Repair dry-run → zero state changes
- Repair execute → audit records complete

### Day-0 tests
- Package built with `provisional: true` → manifest correct
- Import to empty repository → RELEASED, ledger updated
- Import with version conflict → rejected with details
- Import with same digest as existing → linked (idempotent)

### End-to-end tests
- Full cycle: build → allocate → upload → verify → promote → desired set → reconcile → install
- Rollback: desired set to older RELEASED version → deterministic convergence
- Three nodes converge to same `build_id` and `digest`

---

## 16. Risks and Design Decisions

### D1: Build ID format → UUIDv7
Sortable by creation time. Globally unique without coordination. Works offline (day-0). Standard format with Go library support. `build_number` derived as monotonic counter per (publisher, name, version, platform) for display.

### D2: Exact version publish allowed for infrastructure packages
External packages (etcd 3.5.14, prometheus 3.5.1) have their own version schemes. Allow `EXACT("3.5.14")` intent, validated against monotonicity. Required for third-party packaging.

### D3: Payload version mismatch → warn, not block
Content `binary_version` may lag or diverge from release version. Log mismatch for traceability. Blocking would break existing workflows. Can be tightened later via policy.

---

## 17. Implementation Status (April 2026)

### Completed Work

**Phase 1 + Phase 2: Fully implemented and deployed on all 3 cluster nodes.**

Key implementation artifacts:
- `repository/repository_server/artifact_handlers.go` — build_id allocation (UUIDv7), terminal-state immutability, ScyllaDB dual-write
- `repository/repository_server/scylla_store.go` — ScyllaDB manifest metadata store
- `repository/repository_server/dep_health.go` — MinIO + ScyllaDB health watchdog
- `repository/repository_server/migration.go` — `MigrateBuildIDs()` backfill (UUIDv5 synthetic for old artifacts)
- `cluster_controller/cluster_controller_server/desired_state_handlers.go` — `validateArtifactInRepo()` returns build_id, writes to desired-state
- `cluster_controller/cluster_controller_server/release_pipeline.go` — build_id-only convergence, no fallback
- `cluster_controller/cluster_controller_server/release_resolver.go` — `ResolvedArtifact.BuildID` from manifest
- `cluster_controller/cluster_controller_server/workflow_release.go` — `resolved_build_id` in workflow inputs
- `node_agent/node_agent_server/apply_package_release.go` — synchronous restart + health verification, build_id in installed-state, self-update via `systemd-run` upgrader
- `node_agent/node_agent_server/internal/supervisor/supervisor.go` — `LaunchUpgrader` with cgroup isolation
- `node_agent/node_agent_server/internal/actions/artifact.go` — `renderTemplateVars()` with `{{.NodeIP}}` support
- `cmd/globular-upgrader/main.go` — standalone upgrader writes installed-state after active verification
- `dephealth/watchdog.go` — shared dependency health watchdog (used by 6 services)
- `storage_backend/storage.go` — `Ping()` on Storage interface
- Proto changes: `build_id` added to `ArtifactManifest`, `UploadArtifactResponse`, `InstalledPackage`, `ApplyPackageReleaseRequest/Response`, all desired-state Go structs

**Additional infrastructure hardening (not in original redesign but required):**
- Convergence truth fix: node-agent apply blocks until service is running (no more premature success)
- Dependency health gating: event, workflow, dns, resource, rbac services gate RPCs on ScyllaDB/MinIO health
- DNS domain cache: refreshed from ScyllaDB every 30s (was load-once-at-startup)
- Resource service: BigCache removed (always reads from ScyllaDB)
- RBAC service: BigCache TTL set to 30s (controlled caching)
- Repository: OSStorage fallback removed — MinIO required
- Node-agent systemd unit: `Type=notify` → `Type=simple` (was never sending sd_notify)

**State canonicalization tooling:**
- `globular state canonicalize --dry-run` — scans repository (A4), desired-state (A2), installed-state (A1/A3/A7)
- `globular state canonicalize --fix-safe` — repairs desired-state build_id via controller API
- `globular state canonicalize --fix-installed --node <id> --agent-endpoint <addr>` — repairs installed-state via ApplyPackageRelease
- `globular state canonicalize --metadata-only` — direct etcd build_id write for COMMAND/INFRASTRUCTURE packages
- Result: 246 anomalies → ~21 residuals (metadata-only packages + INFRASTRUCTURE build_id gaps on active nodes)

### Remaining Operational Work

**All 7 phases implemented (April 2026).** See `docs/design/version-control-audit.md` for the full audit table.

Follow-up items tracked in `docs/design/roadmap-to-9.md`:
1. CLI `--bump` flag wiring — use `AllocateUpload` from CLI/deploy instead of hardcoded version (Phase A)
2. Remove CLI-side `pkg register` after transition period (Phase A)
3. Node-agent: trigger `ImportProvisionalArtifact` for provisional packages on bootstrap (Phase G)
4. Automated invariant tests for INV-1 through INV-10 (Phase C)

### D4: Channels → deferred
Single implicit "stable" channel. Data model supports channels (field present, defaults to "stable"). Implementation deferred until multi-environment or canary deployments are needed.

### D5: Repair scope → scan automatic, repair manual
Detection and classification are safe to automate. Repair actions that modify state require explicit operator decision. DUPLICATE_CONTENT (same version, different bits) is never auto-resolved.

### D6: Reservation TTL → 5 minutes
Balances upload time for large packages (~40 MB, ~4 seconds on local network) with contention avoidance. Configurable via repository config.

### D7: Desired state references version, not build_id by default
Simpler UX: `desired set cluster-controller 1.4.2`. Controller resolves to exact `build_id` at reconciliation time. Pinned mode (`--build-id`) available for exact artifact targeting.

---

## 17. Final Implementation Order

```
Phase 1: Immutability + desired set guard     ← CRITICAL, do first
Phase 2: Repository-issued build_id           ← CRITICAL, foundation for everything
Phase 3: Release ledger + monotonicity        ← HIGH, enables correct resolution
  ├── Phase 4: Allocation protocol            ← can follow Phase 3
  ├── Phase 5: Repair tooling                 ← can parallel Phase 4
  ├── Phase 6: Day-0 provisional              ← can parallel Phase 4-5
  └── Phase 7: Discovery consolidation        ← can parallel Phase 5-6
```

Phases 1-3 are sequential and form the critical path.
Phases 4-7 are independent and can be parallelized.

Phase 1 can ship in 1-2 days and immediately stops all new corruption.
