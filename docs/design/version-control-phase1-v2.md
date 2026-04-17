# Globular Version Control — Phase 1 Implementation Plan v2

## 1. Objective

Phase 1 is a surgical stabilization. It stops new corruption without changing any working behavior.

After Phase 1:

- **A PUBLISHED artifact cannot be silently overwritten with different content.** Upload at the same identity with a different checksum is rejected when the existing artifact has reached `PublishState_PUBLISHED`.
- **Desired state cannot reference an artifact that does not exist as PUBLISHED in the repository.** `UpsertDesiredService` validates artifact existence and state before writing to etcd.
- **`globular deploy` no longer force-overwrites by default.** The `--force` flag is removed from the deploy subprocess call.

### Phase 1 Limitation (explicit)

Build-number regression (e.g., downgrading from build 5 to build 1 within the same version) is NOT fully prevented in Phase 1. The desired-state validation confirms the target artifact exists in the repository, which prevents referencing non-existent artifacts. But the system does not yet compare installed build identity against desired build identity at the convergence level. That requires `build_id` (Phase 2).

### State terminology

The current codebase uses `PublishState_PUBLISHED` (enum value 3) as the terminal "ready for install" state. The v2 architecture renames this concept to `RELEASED`. In Phase 1, we use the **existing enum** `PublishState_PUBLISHED` as the immutability boundary. Renaming to `RELEASED` is deferred to Phase 2+. All references to "PUBLISHED" in this document mean the current `PublishState_PUBLISHED` enum value.

---

## 2. Affected Components

| Component | File | Functions |
|-----------|------|-----------|
| Repository server | `golang/repository/repository_server/artifact_handlers.go` | `UploadArtifact` (lines 554–670), `readManifestAndStateByKey` (lines 134–169) |
| Controller desired state | `golang/cluster_controller/cluster_controller_server/desired_state_handlers.go` | `upsertOne` (lines 215–263) |
| Controller helpers | `golang/cluster_controller/cluster_controller_server/handlers_upgrade.go` | `defaultPublisherID()` (lines 5–7) |
| Deploy pipeline | `golang/deploy/deploy.go` | `DeployService` (lines 260–269) |
| Release resolver (reference) | `golang/cluster_controller/cluster_controller_server/release_resolver.go` | `Resolve` (lines 50–165) — not modified, but its resolution pattern is reused |

---

## 3. Change Set

### Change 1 — Reject Overwrite of PUBLISHED Artifacts

**Location:**
- file: `golang/repository/repository_server/artifact_handlers.go`
- function: `UploadArtifact`
- lines: 596–621 (the uniqueness check block)

**Current behavior:**

When an artifact with the same key `(publisher, name, version, platform, build_number)` already exists:
- Same checksum → idempotent skip (returns success, retries publish pipeline if stuck in VERIFIED)
- Different checksum → logs a warning (`"artifact overwrite (same version, different content)"`) and falls through to overwrite the binary and manifest

**New behavior:**

When an artifact with the same key exists and the checksum differs:
- If `existingState == repopb.PublishState_PUBLISHED` → **reject** with `codes.AlreadyExists`
- If `existingState != repopb.PublishState_PUBLISHED` → allow overwrite (preserves ability to iterate on pre-publish builds)

**Detailed logic:**

In the existing block at line 600, after the idempotent same-checksum check (lines 601–613) returns, the different-checksum branch (lines 615–621) currently just logs and falls through. Replace that branch with:

1. Check `existingState == repopb.PublishState_PUBLISHED`
2. If true → return `status.Errorf(codes.AlreadyExists, ...)`
3. If false → log warning with state name, continue to overwrite

**Edge cases — missing or unknown state:**

`readManifestAndStateByKey` (lines 134–169) reads the manifest JSON and extracts `publishState` from the JSON object via `unmarshalManifestWithState` (lines 100–120). If the `publishState` key is absent from the JSON (legacy artifact), the function returns `PublishState_PUBLISH_STATE_UNSPECIFIED` (enum value 0).

**Safe fallback for unknown/missing state:** Treat `PUBLISH_STATE_UNSPECIFIED` as **non-terminal** (allow overwrite). Rationale:

- Legacy artifacts that predate the publish state system were never formally published through the current pipeline
- Treating unknown state as terminal would block legitimate updates to legacy artifacts
- The conservative choice is to protect only artifacts that are provably PUBLISHED

This means overwrite is rejected ONLY when `existingState == PublishState_PUBLISHED` (enum value 3). All other states (`UNSPECIFIED`, `STAGING`, `VERIFIED`, `FAILED`, `ORPHANED`) allow overwrite.

**Summary of behavior matrix:**

| Existing checksum | Existing state | Action |
|-------------------|---------------|--------|
| Same | Any | Idempotent skip (unchanged) |
| Different | `PUBLISHED` | Reject `AlreadyExists` |
| Different | `DEPRECATED`, `YANKED`, `QUARANTINED`, `REVOKED` | Reject `AlreadyExists` (all post-PUBLISHED states are also terminal) |
| Different | `UNSPECIFIED`, `STAGING`, `VERIFIED`, `FAILED`, `ORPHANED` | Allow overwrite with warning |

Refinement: the rejection should apply to ALL terminal states (PUBLISHED and beyond), not just PUBLISHED. An artifact that has been DEPRECATED, YANKED, or REVOKED should also not be silently overwritten. The check becomes:

```
existingState >= PublishState_PUBLISHED
```

Since the enum values are ordered (PUBLISHED=3, FAILED=4, ORPHANED=5, DEPRECATED=6, YANKED=7, QUARANTINED=8, REVOKED=9), this captures all post-publish states. `FAILED` and `ORPHANED` are edge cases — they represent publish pipeline failures. Conservatively, treat them as non-terminal (allow overwrite) since the artifact never successfully published. The precise check:

```
existingState == PublishState_PUBLISHED ||
existingState == PublishState_DEPRECATED ||
existingState == PublishState_YANKED ||
existingState == PublishState_QUARANTINED ||
existingState == PublishState_REVOKED
```

This is an explicit allowlist of immutable states, not a range comparison. Safer against future enum additions.

---

### Change 2 — Validate Artifact Existence Before Writing Desired State

**Location:**
- file: `golang/cluster_controller/cluster_controller_server/desired_state_handlers.go`
- function: `upsertOne` (lines 215–263)

**Current behavior:**

`upsertOne` validates version format, applies the version regression guard, then writes `ServiceDesiredVersion` to etcd and calls `ensureServiceRelease`. No repository validation.

**New behavior:**

After the version regression guard (line 243) and before constructing the `ServiceDesiredVersion` object (line 245), validate that the artifact exists in the repository and is installable.

**New helper function:** `validateArtifactInRepo(ctx, serviceName, version, buildNumber) error`

This function follows the **same resolution path** the controller's release resolver already uses:

1. **Resolve repository address:** Call `config.ResolveServiceAddr("repository.PackageRepository", "")`. If empty → return `status.Errorf(codes.Unavailable, "repository address not configured; cannot validate artifact for %s@%s", serviceName, version)`.

2. **Resolve publisher:** Call `defaultPublisherID()` (defined in `handlers_upgrade.go:5`, returns `"core@globular.io"`). This is the same function used by `ensureServiceRelease`, desired state removal, and the reconciler. It is NOT a hardcoded assumption — it's the existing system default.

3. **Resolve platform:** Default to `"linux_amd64"` when unspecified. This matches the release resolver (`release_resolver.go:120-124`): `if platform == "" { platform = "linux_amd64" }`. Same logic, same default.

4. **Create repository client:** `repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")`. Defer `client.Close()`.

5. **Call `client.GetArtifactManifest(ref, buildNumber)`** where:
   - `ref.PublisherId` = result of `defaultPublisherID()`
   - `ref.Name` = canonical service name (already computed by `upsertOne`)
   - `ref.Version` = validated, canonical version
   - `ref.Platform` = `"linux_amd64"`
   - `buildNumber` = `svc.BuildNumber` from the request

6. **Handle response:**
   - Success → the artifact exists. Proceed.
   - `codes.NotFound` → return `status.Errorf(codes.NotFound, "artifact %s@%s (build %d) not found in repository; cannot set desired state for non-existent artifact", serviceName, version, buildNumber)`
   - `codes.Unavailable` or connection error → return `status.Errorf(codes.Unavailable, "repository unreachable at %s; cannot validate artifact for %s@%s", addr, serviceName, version)`
   - Other errors → return `status.Errorf(codes.Internal, "repository validation failed for %s@%s: %v", serviceName, version, err)`

**What constitutes "artifact exists" for Phase 1:**

`GetArtifactManifest` succeeds. This means:
- A manifest file exists at the computed storage key
- When `buildNumber=0`, the repository resolves to the latest PUBLISHED build via `readManifestWithFallback` → `resolveLatestBuildNumber` (which only considers PUBLISHED artifacts)
- When `buildNumber>0`, the manifest exists at the exact key

The existing `GetArtifactManifest` already filters for PUBLISHED artifacts when resolving `buildNumber=0`. For exact build numbers, it reads whatever exists. This is acceptable for Phase 1 — the node agent's `CheckArtifactPublished` guard (line 102 in `apply_package_release.go`) is the final safety boundary that rejects non-PUBLISHED installs.

Phase 1 goal: prevent obviously invalid desired state (non-existent artifacts). It does NOT require the validation to also check publish state — that's the node agent's job.

**Edge cases:**

- **Repository not yet started (day-0 bootstrap):** `config.ResolveServiceAddr` returns empty → `Unavailable` error. Desired state cannot be set until repository is running. This is correct — day-0 uses direct package installation, not desired state.
- **Multiple concurrent `desired set` calls:** Each validates independently. Read-only check, no race condition.
- **Network partition controller↔repository:** Returns `Unavailable`. Operator retries when connectivity is restored. Fail closed.
- **`buildNumber == 0` and no PUBLISHED builds exist:** `GetArtifactManifest` with fallback finds nothing → returns error → validation fails with `NotFound`. Correct.
- **Publisher mismatch:** If someone publishes under a different publisher ID, `defaultPublisherID()` won't match. This matches existing behavior — the entire system assumes `core@globular.io` for service packages.

---

### Change 3 — Remove `--force` from Deploy Pipeline

**Location:**
- file: `golang/deploy/deploy.go`
- function: `DeployService`
- line: 268

**Current behavior:**

Line 268 unconditionally includes `"--force"` in publish arguments:
```go
publishArgs = append(publishArgs,
    "pkg", "publish",
    "--file", tgzPath,
    "--repository", repoAddr,
    "--force",           // ← this line
)
```

Every `globular deploy` calls `globular pkg publish --force`, which on `AlreadyExists` calls `DeleteArtifact` then re-uploads.

**New behavior:**

Remove the `"--force"` line:
```go
publishArgs = append(publishArgs,
    "pkg", "publish",
    "--file", tgzPath,
    "--repository", repoAddr,
)
```

**Interaction with existing deploy flow:**

The deploy pipeline already handles the normal case correctly:

1. `NextBuildNumber()` queries latest build number, returns `current + 1`
2. Package is built with new build number
3. Publish creates a new artifact at the new build number
4. No collision with existing PUBLISHED artifacts

The only scenario where `--force` was needed:
- Delta detection fails or `NextBuildNumber` returns stale data → same build number collision → upload sees existing artifact with different checksum

With Change 1 in place, that collision is now caught:
- If existing is PUBLISHED + different checksum → `AlreadyExists` error. Deploy reports failure.
- If existing is pre-PUBLISHED + different checksum → overwrite allowed (same as before).

**Recovery path for operators:**

When deploy fails with `AlreadyExists`:
1. **Rebuild with new build number:** Re-run deploy. `NextBuildNumber` will return a fresh number.
2. **Explicit force:** Run `globular pkg publish --force --file <tgz> --repository <addr>` manually. `--force` on the CLI still works — it's an explicit operator decision, not a silent default.

---

## 4. Invariants Enforced in Phase 1

| Invariant | Enforcement |
|-----------|-------------|
| **INV-P1-1:** A PUBLISHED artifact cannot be overwritten with different content | Change 1: `UploadArtifact` returns `AlreadyExists` when existing state is PUBLISHED (or any post-PUBLISHED terminal state) and digest differs |
| **INV-P1-2:** Desired state cannot reference a non-existent artifact | Change 2: `upsertOne` calls `validateArtifactInRepo` before writing to etcd. `NotFound` from repository → rejected |
| **INV-P1-3:** Desired state cannot be written when repository is unreachable | Change 2: `validateArtifactInRepo` returns `Unavailable` if repository address is empty or connection fails. Fail closed. |
| **INV-P1-4:** Deploy does not force-overwrite by default | Change 3: `--force` removed from deploy publish args. Overwrites require explicit operator action. |

---

## 5. Error Handling Specification

### `codes.AlreadyExists` — Immutability violation

**When:** `UploadArtifact` receives data for an artifact that already exists in PUBLISHED (or DEPRECATED/YANKED/QUARANTINED/REVOKED) state with a different checksum.

**Message:**
```
artifact %s is in state %s with different content (existing=%s, new=%s); overwrite of published artifacts is forbidden
```
- `%s` 1: storage key (e.g., `core@globular.io%cluster-controller%0.0.8%linux_amd64%1`)
- `%s` 2: existing state name (e.g., `PUBLISHED`)
- `%s` 3: existing checksum (SHA256 hex)
- `%s` 4: new checksum (SHA256 hex)

**Client behavior:** CLI `pkg publish` without `--force` receives this error and reports it. With `--force`, the CLI calls `DeleteArtifact` then re-uploads — this path is unchanged and remains an explicit two-step operator action.

### `codes.NotFound` — Artifact does not exist

**When:** `upsertOne` validation finds no artifact in the repository for the specified `(publisher, name, version, platform, buildNumber)`.

**Message:**
```
artifact %s@%s (build %d) not found in repository; cannot set desired state for non-existent artifact
```

### `codes.Unavailable` — Repository unreachable

**When:** `validateArtifactInRepo` cannot resolve repository address or cannot connect.

**Message:**
```
repository unreachable at %s; cannot validate artifact for %s@%s
```

Or if address is not configured:
```
repository address not configured; cannot validate artifact for %s@%s
```

---

## 6. Backward Compatibility

### What still works unchanged

- Publishing a **new** artifact (no existing artifact at that key) → succeeds
- Publishing the **same** artifact (same checksum) → idempotent skip
- Overwriting a **pre-PUBLISHED** artifact (STAGING, VERIFIED, FAILED, ORPHANED) with different content → allowed
- `--force` flag on `globular pkg publish` CLI → still works (explicit delete + re-upload)
- `globular deploy` with changed binary and auto-incremented build number → succeeds (new key, no collision)
- `desired set` for a version/build that exists as PUBLISHED → succeeds
- `desired set` with `buildNumber=0` (resolve latest) where PUBLISHED builds exist → succeeds

### What now fails (and why this is correct)

| Scenario | Previous behavior | New behavior | Why correct |
|----------|------------------|--------------|-------------|
| Upload different content at same identity, artifact is PUBLISHED | Silent overwrite | `AlreadyExists` error | Prevents version chaos across nodes |
| `desired set` for version not in repository | Written to etcd, reconciler discovers missing artifact later | `NotFound` error at write time | Prevents desired state pointing to void |
| `desired set` when repository is down | Written to etcd blindly | `Unavailable` error | Prevents unvalidated desired state |
| `globular deploy` with build number collision | Silent force-overwrite | `AlreadyExists` error if PUBLISHED | Operator must retry with new build number |

### Operational impact

Operators will see **new errors where silent corruption used to occur**. This is expected and desired. Recovery paths:
- Build number collision during deploy → re-run deploy (gets next build number)
- Need to republish same version → `globular pkg publish --force` (explicit choice)
- `desired set` fails → verify artifact is published, fix version/build reference

---

## 7. Test Plan

### Repository Tests

**Test 1: Same version, same checksum → idempotent**
- Upload artifact at v1.0.0 build 1
- Promote to PUBLISHED
- Upload same bytes at v1.0.0 build 1
- Expected: success, no overwrite, manifest unchanged

**Test 2: Same version, different checksum, PUBLISHED → reject**
- Upload artifact at v1.0.0 build 1
- Promote to PUBLISHED
- Upload different bytes at v1.0.0 build 1
- Expected: `AlreadyExists` with message containing "overwrite of published artifacts is forbidden"
- Verify: original artifact unchanged in storage

**Test 3: Same version, different checksum, VERIFIED → allow**
- Upload artifact at v1.0.0 build 1 (do NOT promote — stays VERIFIED)
- Upload different bytes at v1.0.0 build 1
- Expected: success, artifact overwritten, warning logged

**Test 4: Same version, different checksum, DEPRECATED → reject**
- Upload artifact at v1.0.0 build 1, promote to PUBLISHED, then deprecate
- Upload different bytes at v1.0.0 build 1
- Expected: `AlreadyExists` (DEPRECATED is post-PUBLISHED, immutable)

**Test 5: Same version, different checksum, UNSPECIFIED (legacy) → allow**
- Create legacy manifest without publishState field
- Upload different bytes at same identity
- Expected: success, overwrite allowed (legacy fallback)

**Test 6: Different build number → independent artifact**
- Upload artifact at v1.0.0 build 1, promote to PUBLISHED
- Upload artifact at v1.0.0 build 2
- Expected: success, two independent artifacts

### Desired State Tests

**Test 7: Set desired to existing PUBLISHED artifact → success**
- Publish artifact `echo@0.0.8` build 1
- `desired set echo 0.0.8 --build-number 1`
- Expected: desired state written to etcd

**Test 8: Set desired to non-existent version → reject**
- No artifact at version 9.9.9
- `desired set echo 9.9.9`
- Expected: `NotFound`

**Test 9: Set desired to non-existent build number → reject**
- Only `echo@0.0.8` build 1 exists
- `desired set echo 0.0.8 --build-number 99`
- Expected: `NotFound`

**Test 10: Set desired when repository unreachable → reject**
- Repository service stopped
- `desired set echo 0.0.8`
- Expected: `Unavailable`

**Test 11: Set desired with build_number=0 → resolve latest**
- Publish `echo@0.0.8` build 1 (PUBLISHED)
- `desired set echo 0.0.8` (build_number=0)
- Expected: repository resolves to latest PUBLISHED build, validation passes, desired state written

### Deploy Tests

**Test 12: Normal deploy with changed binary → success**
- Build new binary, auto-increment build number
- Publish without `--force`
- Expected: new artifact created, desired state updated

**Test 13: Deploy collision (PUBLISHED artifact at same key) → fail safely**
- Artifact at v0.0.8 build 5 is PUBLISHED
- Deploy attempts to publish different binary at v0.0.8 build 5 (stale NextBuildNumber)
- `--force` is NOT passed
- Expected: `AlreadyExists` error, existing artifact unchanged, deploy reports failure
- Recovery: re-run deploy → gets build 6

---

## 8. Rollout Safety

### Why Phase 1 can be deployed safely

1. **No behavior change for valid operations.** New publishes, idempotent re-uploads, different build numbers, correct desired state references — all work identically.

2. **Only new error paths for previously-corrupt operations.** Every new failure mode corresponds to an operation that was already wrong (overwriting published content, referencing non-existent artifacts). Making wrong operations fail explicitly is strictly safer than letting them silently corrupt.

3. **No data migration.** No schema changes, no state machine changes, no storage layout changes. The existing `PublishState` enum and manifest format are used as-is.

4. **Fully reversible.** Reverting the three changes restores original behavior. No persistent state is modified by the changes themselves.

5. **Backward compatible with existing CLI.** Old CLI clients that don't pass `--force` work identically. Old CLI clients that pass `--force` still work (delete + re-upload path unchanged). The only difference: the repository now rejects overwrites that old clients might have done without `--force` (which was already wrong behavior).

### What Phase 1 prevents immediately

- Silent overwrite of published artifacts (the root cause of the 3-node version chaos incident)
- Desired state pointing to non-existent artifacts (the trigger for stuck reconciler and impossible convergence)
- Default force-publish in deploy (the enabler of silent overwrites)

### What Phase 1 does NOT fix

- Build numbers remain client-supplied → race conditions possible but no longer cause silent corruption (Phase 2: server-generated `build_id`)
- No monotonicity enforcement → non-monotonic version publish still possible (Phase 3: release ledger)
- No version allocation protocol → CLI still chooses version strings (Phase 4)
- Existing corrupted history remains → old artifacts may have conflicting content (Phase 5: repair tooling)
- Build-number regression within same version → not fully guarded at convergence level (Phase 2: `build_id` comparison)
- Day-0/day-1 boundary → provisional semantics not yet defined (Phase 6)
- Discovery registered by CLI → split-brain still possible (Phase 7)
