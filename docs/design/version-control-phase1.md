# Globular Version Control — Phase 1 Implementation Plan

## 1. Objective

Phase 1 is a surgical stabilization. It prevents new corruption without changing any working behavior.

After Phase 1:

- **A published artifact cannot be silently overwritten with different content.** Upload of the same `(publisher, name, version, platform, build_number)` with a different checksum is rejected when the existing artifact is in PUBLISHED state.
- **Desired state cannot point to an artifact that doesn't exist in the repository.** `UpsertDesiredService` validates artifact existence before writing to etcd.
- **`globular deploy` no longer force-overwrites by default.** The `--force` flag is removed from the deploy subprocess call.
- **Version regression guard considers build number.** `highestHealthyInstalledVersion` compares `(version, build_number)` tuples, not version strings alone.

Phase 1 does NOT introduce build_id generation, release ledger, allocation protocol, day-0 redesign, or repair tooling. Those are Phase 2+.

---

## 2. Affected Components

| Component | File | Functions |
|-----------|------|-----------|
| Repository server | `golang/repository/repository_server/artifact_handlers.go` | `UploadArtifact` (lines 554–670) |
| Controller desired state | `golang/cluster_controller/cluster_controller_server/desired_state_handlers.go` | `upsertOne` (lines 215–263), `highestHealthyInstalledVersion` (lines 268–296) |
| Deploy pipeline | `golang/deploy/deploy.go` | `DeployService` (around line 264–269) |
| Version comparison | `golang/versionutil/semver.go` | `CompareFull` (already exists) |

---

## 3. Change Set

### Change 1 — Reject Overwrite of PUBLISHED Artifacts

**Location:**
- file: `golang/repository/repository_server/artifact_handlers.go`
- function: `UploadArtifact` (the streaming RPC handler)
- lines: 596–621 (the uniqueness check block)

**Current behavior:**

When an artifact with the same key `(publisher, name, version, platform, build_number)` already exists:
- Same checksum → idempotent skip (correct, keep this)
- Different checksum → **silent overwrite** with a warning log, then proceeds to write new binary and manifest over the old one

**New behavior:**

When an artifact with the same key exists:
- Same checksum → idempotent skip (unchanged)
- Different checksum + existing state is `PUBLISHED` → **reject** with `codes.AlreadyExists`
- Different checksum + existing state is NOT `PUBLISHED` (i.e., `VERIFIED` or any pre-publish state) → overwrite is allowed (preserves the ability to iterate on staged builds before they're published)

**Detailed logic:**

In the existing block at line 600 (`if _, existingState, existingManifest, readErr := srv.readManifestAndStateByKey(ctx, key); readErr == nil`), after the same-checksum idempotent check (lines 601–613), the different-checksum branch (lines 615–621) currently just logs a warning and falls through to overwrite.

Add a check after line 614 (after the idempotent return):

```
if existingManifest.GetChecksum() != newChecksum {
    if existingState == repopb.PublishState_PUBLISHED {
        → return status.Errorf(codes.AlreadyExists,
            "artifact %s is PUBLISHED with different content (existing=%s, new=%s); "+
            "overwrite of published artifacts is forbidden",
            key, existingManifest.GetChecksum(), newChecksum)
    }
    // Pre-publish state: allow overwrite (staged build iteration)
    slog.Warn("artifact overwrite (pre-publish, same version, different content)",
        "key", key, "state", existingState.String(),
        "old_checksum", existingManifest.GetChecksum(),
        "new_checksum", newChecksum)
}
```

**Edge cases:**

- `readManifestAndStateByKey` returns error (artifact doesn't exist) → normal new upload, no change
- `existingState` is zero value (shouldn't happen if manifest exists, but if it does) → treat as non-PUBLISHED, allow overwrite with warning
- Legacy artifacts without explicit state file → `readManifestAndStateByKey` reads the `.publish_state` file. If missing, state defaults to `VERIFIED` (current behavior in the function). This means legacy artifacts without state files can be overwritten — correct, since they were never formally published.

---

### Change 2 — Validate Artifact Existence Before Writing Desired State

**Location:**
- file: `golang/cluster_controller/cluster_controller_server/desired_state_handlers.go`
- function: `upsertOne` (lines 215–263)

**Current behavior:**

`upsertOne` validates version format, applies the version regression guard, then writes `ServiceDesiredVersion` to etcd and calls `ensureServiceRelease`. It does NOT check whether the artifact actually exists in the repository.

**New behavior:**

After the version regression guard (line 243) and before writing to etcd (line 253), add a repository validation step:

1. Resolve repository address via `config.ResolveServiceAddr("repository.PackageRepository", "")`
2. Create a `repository_client.NewRepositoryService_Client`
3. Call `client.GetArtifactManifest(ref, buildNumber)` where:
   - `ref.PublisherId` = `"core@globular.io"` (the default publisher, matching what `desired set` CLI uses)
   - `ref.Name` = canonical service name
   - `ref.Version` = validated version
   - `ref.Platform` = `"linux_amd64"` (current platform)
   - `buildNumber` = `svc.BuildNumber` (from the request)
4. If `GetArtifactManifest` returns error:
   - If error code is `NotFound` → return `status.Errorf(codes.NotFound, "artifact %s@%s (build %d) not found in repository; cannot set desired state for non-existent artifact", canon, version, svc.BuildNumber)`
   - If error code is `Unavailable` or connection fails → return `status.Errorf(codes.Unavailable, "repository unreachable; cannot validate artifact existence for %s@%s", canon, version)` (fail closed — do not write desired state if we can't verify)
   - Other errors → return `status.Errorf(codes.Internal, "repository validation failed for %s@%s: %v", canon, version, err)`
5. If `GetArtifactManifest` succeeds → proceed to write desired state (existing flow)

**Detailed logic placement:**

Insert between line 243 (end of version regression guard) and line 245 (start of obj construction):

```
// Phase 1: Validate artifact exists in repository before writing desired state.
if err := srv.validateArtifactExists(ctx, canon, version, svc.BuildNumber); err != nil {
    return err
}
```

New helper function `validateArtifactExists(ctx, name, version, buildNumber)`:
- Resolves repository address
- Creates repository client (short-lived, closed with defer)
- Calls `GetArtifactManifest`
- Returns nil on success, or appropriate gRPC status error on failure

**Edge cases:**

- Repository service not yet started (day-0 bootstrap): `config.ResolveServiceAddr` returns empty → function returns `Unavailable` error. This means `desired set` cannot be called until the repository is running. This is correct — you should not set desired state before the repository can confirm the artifact exists.
- `buildNumber == 0` (resolve to latest): `GetArtifactManifest` with build_number=0 already resolves to the latest published build (existing behavior in `readManifestWithFallback`). If no published builds exist, it returns `NotFound`.
- Network partition between controller and repository: returns `Unavailable`. Operator retries when connectivity is restored.
- Multiple concurrent `desired set` calls: each validates independently. No race condition — the validation is a read-only check.

---

### Change 3 — Remove `--force` from Deploy Pipeline

**Location:**
- file: `golang/deploy/deploy.go`
- function: `DeployService` (around lines 260–269)

**Current behavior:**

Line 268 unconditionally appends `"--force"` to the publish arguments:
```go
publishArgs = append(publishArgs,
    "pkg", "publish",
    "--file", tgzPath,
    "--repository", repoAddr,
    "--force",           // ← THIS LINE
)
```

This means every `globular deploy` calls `globular pkg publish --force`, which deletes and re-uploads if the artifact already exists with different content.

**New behavior:**

Remove the `"--force"` line. The publish arguments become:
```go
publishArgs = append(publishArgs,
    "pkg", "publish",
    "--file", tgzPath,
    "--repository", repoAddr,
)
```

With Change 1 in place, if the artifact already exists as PUBLISHED with the same checksum, the upload is idempotent (returns success). If it exists with different checksum and is PUBLISHED, the upload is rejected with `AlreadyExists`. The deploy pipeline sees the error and reports it.

**Interaction with auto-increment build numbers:**

The deploy pipeline (via `buildnumber.go:NextBuildNumber()`) already queries the repository for the latest build number and increments it. This means:
- First deploy of version X, build N: new artifact, succeeds
- Second deploy of version X, build N+1: new artifact, succeeds
- Re-deploy of identical binary: `NextBuildNumber` returns N+1 (same as already published). Upload sees same checksum → idempotent skip. Build number stays N, not N+1.

Wait — there's a subtlety. If the binary hasn't changed, the deploy pipeline's delta detection (`deploy.go` around line 130-145) should already skip the publish. Let me verify.

**Delta detection behavior:**

The deploy pipeline computes SHA256 of the new binary and compares it against the previous build's checksum (queried from repository). If identical, it skips the publish step entirely (unless `--full` is passed). So in normal operation:
- Changed binary → new build number → new artifact → publish succeeds
- Unchanged binary → skip publish → no overwrite attempted

The only case where `--force` was needed: if the delta detection fails or the build number query returns stale data, leading to a collision. With Change 1, that collision is now caught and reported as an error rather than silently overwriting.

**Edge cases:**

- Build number collision (two concurrent deploys get same build_number): Upload fails with `AlreadyExists` for the second deploy. The deploy pipeline reports the error. Operator retries (gets next build number).
- First deploy ever (no previous build in repo): No collision possible. Publish succeeds.

---

### Change 4 — Version Regression Guard Includes Build Number

**Location:**
- file: `golang/cluster_controller/cluster_controller_server/desired_state_handlers.go`
- function: `highestHealthyInstalledVersion` (lines 268–296)

**Current behavior:**

`highestHealthyInstalledVersion` iterates over all healthy nodes, reads `node.InstalledVersions[serviceName]` (a `map[string]string` of service name → version string), and compares using `versionutil.Compare()` which only compares semantic version strings. It does NOT consider build numbers.

The caller (`upsertOne` at line 236–243) uses the result to block version regressions: if the highest installed version is newer than the requested version, it auto-corrects upward.

**Problem:** If all nodes have `cluster-controller@0.0.8+5` installed and someone runs `desired set cluster-controller 0.0.8 --build-number 1`, the guard doesn't catch it because the version strings are equal (`0.0.8 == 0.0.8`). This allows a build number downgrade.

**New behavior:**

The regression guard should compare `(version, build_number)` tuples. However, `node.InstalledVersions` currently only stores version strings, not build numbers. A full fix would require changing the heartbeat/installed data to include build numbers — that's beyond Phase 1 scope.

**Phase 1 pragmatic fix:**

Keep the current `highestHealthyInstalledVersion` as-is (version-only comparison). It still catches the most dangerous case: version downgrade (e.g., 0.0.7 → 0.0.2).

The build-number downgrade case (0.0.8+5 → 0.0.8+1) is now partially guarded by Change 2: when the operator runs `desired set cluster-controller 0.0.8 --build-number 1`, the repository validation checks whether `cluster-controller@0.0.8 build 1` exists. If it exists, the desired set is valid (even if it's a build-number downgrade). If it doesn't exist, it's rejected.

**True fix deferred to Phase 2:** When `build_id` is introduced, convergence comparison uses `build_id` instead of `(version + build_number)`, making this problem disappear entirely.

**Action for Phase 1:** No code change to `highestHealthyInstalledVersion`. Document the limitation.

---

## 4. Invariants Enforced in Phase 1

| Invariant | Description | Enforced by |
|-----------|-------------|-------------|
| **INV-P1-1** | A PUBLISHED artifact cannot be overwritten with different content | Change 1: `UploadArtifact` rejects with `AlreadyExists` when existing state is PUBLISHED and digest differs |
| **INV-P1-2** | Desired state cannot reference a non-existent artifact | Change 2: `upsertOne` validates via `GetArtifactManifest` before writing to etcd |
| **INV-P1-3** | Desired state cannot be written when repository is unreachable | Change 2: `validateArtifactExists` returns `Unavailable` if repository can't be reached (fail closed) |
| **INV-P1-4** | Deploy does not force-overwrite by default | Change 3: `--force` removed from deploy publish args |

---

## 5. Error Handling Specification

### `codes.AlreadyExists` — Immutability violation

**When:** `UploadArtifact` receives data for an artifact that already exists in PUBLISHED state with a different checksum.

**Message format:**
```
artifact {key} is PUBLISHED with different content (existing={old_checksum}, new={new_checksum}); overwrite of published artifacts is forbidden
```

**Context included:** storage key (contains publisher, name, version, platform, build_number), old checksum, new checksum.

**Client behavior:** CLI `pkg publish` without `--force` receives this error and reports it to the user. With `--force`, the CLI currently calls `DeleteArtifact` and retries — this still works because `DeleteArtifact` + re-upload is an explicit two-step action, not a silent overwrite. The operator has made a conscious choice.

### `codes.NotFound` — Artifact does not exist in repository

**When:** `upsertOne` calls `validateArtifactExists` and the repository returns `NotFound` for the specified `(name, version, build_number)`.

**Message format:**
```
artifact {name}@{version} (build {build_number}) not found in repository; cannot set desired state for non-existent artifact
```

**Context included:** canonical service name, version, build number.

### `codes.Unavailable` — Repository unreachable

**When:** `validateArtifactExists` cannot connect to the repository service.

**Message format:**
```
repository unreachable; cannot validate artifact existence for {name}@{version}
```

**Context included:** canonical service name, version.

**Design choice:** Fail closed. Do NOT write desired state if we can't verify the artifact exists. This prevents the exact scenario we just experienced (desired state pointing to non-existent artifacts).

---

## 6. Backward Compatibility

### What still works unchanged

- Publishing a **new** artifact (no existing artifact at that key) → succeeds as before
- Publishing the **same** artifact again (same checksum) → idempotent skip as before
- Publishing a pre-publish (VERIFIED) artifact with different content → overwrite allowed as before (staging iteration)
- `--force` flag on `globular pkg publish` CLI → still works: deletes then re-uploads. This is an explicit operator action, not a silent default.
- `globular deploy` with changed binary → auto-increments build number, publishes new artifact. No collision with existing PUBLISHED artifacts.
- `desired set` for a version that exists in the repository → succeeds as before

### What now fails (and why)

- `globular deploy` when delta detection is wrong (unchanged binary but stale build number query) → publish returns `AlreadyExists` instead of silently overwriting. **Fix:** operator retries or uses `--full` flag to force new build number.
- `desired set` for a version that does NOT exist in the repository → returns `NotFound`. Previously this would write to etcd and the reconciler would discover the missing artifact later. Now it fails fast.
- `desired set` when repository is down → returns `Unavailable`. Previously this would write to etcd blindly. Now it requires repository availability.

### Legacy client behavior

- Old CLI (without Phase 1 repository changes) calling `UploadArtifact` → gets `AlreadyExists` if trying to overwrite a PUBLISHED artifact. The old CLI's `--force` path (delete + re-upload) still works.
- Old controller (without Phase 1 desired state changes) → continues to write desired state without validation. Phase 1 must be deployed to the controller to get the guard.

---

## 7. Test Plan

### Repository Tests

#### Test 1: Same version, same checksum → idempotent success
- Upload artifact A at version 1.0.0, build 1
- Promote to PUBLISHED
- Upload same artifact A (same bytes) at version 1.0.0, build 1
- **Expected:** Success (idempotent), no overwrite

#### Test 2: Same version, different checksum, PUBLISHED → reject
- Upload artifact A at version 1.0.0, build 1
- Promote to PUBLISHED
- Upload artifact B (different bytes) at version 1.0.0, build 1
- **Expected:** `AlreadyExists` error with message containing "overwrite of published artifacts is forbidden"

#### Test 3: Same version, different checksum, VERIFIED → allow overwrite
- Upload artifact A at version 1.0.0, build 1 (stays in VERIFIED, not promoted)
- Upload artifact B (different bytes) at version 1.0.0, build 1
- **Expected:** Success. Artifact B replaces A. Warning logged.

#### Test 4: Different version → normal upload
- Upload artifact A at version 1.0.0, build 1, promote to PUBLISHED
- Upload artifact B at version 1.0.1, build 1
- **Expected:** Success. Two independent artifacts.

#### Test 5: Same version, different build number → normal upload
- Upload artifact A at version 1.0.0, build 1, promote to PUBLISHED
- Upload artifact B at version 1.0.0, build 2
- **Expected:** Success. Two independent artifacts (different build numbers).

### Desired State Tests

#### Test 6: Set desired to existing artifact → success
- Publish artifact `echo@0.0.8` build 1 to repository
- Call `desired set echo 0.0.8 --build-number 1`
- **Expected:** Desired state written to etcd

#### Test 7: Set desired to non-existent version → reject
- No artifact at version 9.9.9 in repository
- Call `desired set echo 9.9.9`
- **Expected:** `NotFound` error with message "not found in repository"

#### Test 8: Set desired to non-existent build number → reject
- Publish `echo@0.0.8` build 1 only
- Call `desired set echo 0.0.8 --build-number 99`
- **Expected:** `NotFound` error

#### Test 9: Set desired when repository is unreachable → reject
- Stop repository service
- Call `desired set echo 0.0.8`
- **Expected:** `Unavailable` error with message "repository unreachable"

#### Test 10: Set desired with build_number=0 (resolve latest) → success
- Publish `echo@0.0.8` build 1 (PUBLISHED)
- Call `desired set echo 0.0.8` (build_number defaults to 0)
- **Expected:** Repository resolves build 0 to latest (build 1), returns manifest, desired state written

### Deploy Tests

#### Test 11: Normal deploy (changed binary) → success without --force
- Build new binary (different checksum from previous)
- Deploy calls `NextBuildNumber()` → gets new build number
- Publish without `--force`
- **Expected:** Success. New artifact created.

#### Test 12: Deploy unchanged binary → skip publish (delta detection)
- Build binary identical to previous
- Deploy detects same checksum → skips publish
- **Expected:** No publish attempt. Desired state updated to existing version.

---

## 8. Rollout Safety

### Why Phase 1 can be deployed safely

1. **No behavior change for valid usage.** All currently valid operations (new publishes, idempotent re-uploads, different build numbers) continue to work identically.

2. **Only new error paths.** The only new failure modes are:
   - Overwrite of PUBLISHED artifact → was always wrong, now caught
   - Desired set for non-existent artifact → was always wrong, now caught
   - Desired set when repo is down → was always dangerous, now explicit

3. **Backward compatible.** Old CLI clients see `AlreadyExists` errors (valid gRPC code they can handle). Old controllers without the guard continue to work (they just don't get the new validation).

4. **Reversible.** If Phase 1 causes unexpected issues, reverting the changes restores the original behavior. No data migration, no schema changes, no state machine changes.

### What Phase 1 prevents immediately

- Silent overwrite of published artifacts (the primary source of version chaos)
- Desired state pointing to non-existent artifacts (the trigger for the reconciler getting stuck)
- Default force-publish in deploy (the enabler of silent overwrites)

### What Phase 1 does NOT fix (deferred)

- Build numbers are still client-supplied (Phase 2: server-generated build_id)
- No monotonicity enforcement (Phase 3: release ledger)
- No version allocation protocol (Phase 4)
- Existing corrupted history remains (Phase 5: repair tooling)
- Day-0 provisional semantics not yet defined (Phase 6)
- Discovery still registered by CLI (Phase 7)
- `build_number` downgrade within same version not fully guarded (Phase 2: build_id comparison)
