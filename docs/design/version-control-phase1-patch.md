# Globular Version Control — Phase 1 Patch Plan v2.1

## 1. Change Summary

| # | Change | File | Impact |
|---|--------|------|--------|
| 1 | Reject overwrite of PUBLISHED artifacts | `repository/repository_server/artifact_handlers.go` | `UploadArtifact` returns `AlreadyExists` when existing artifact is in terminal state and checksum differs |
| 2 | Validate artifact existence before writing desired state | `cluster_controller/cluster_controller_server/desired_state_handlers.go` | New `validateArtifactInRepo` helper called from `upsertOne` |
| 3 | Remove `--force` from deploy publish | `deploy/deploy.go` | Remove one line |

---

## 2. Repository Change — Overwrite Protection

### File

`golang/repository/repository_server/artifact_handlers.go`

### Function

`UploadArtifact` — the streaming RPC handler

### Exact code block to modify

Lines 614–621. After the idempotent same-checksum return (line 613), replace the existing different-checksum branch:

**BEFORE (lines 614–621):**

```go
	}
	// Different content at same version — overwrite. This happens when
	// the binary is rebuilt without a version bump (bug fixes, Day-0 rebuilds).
	slog.Warn("artifact overwrite (same version, different content)",
		"key", key,
		"old_checksum", existingManifest.GetChecksum(),
		"new_checksum", newChecksum)
}
```

**AFTER:**

```go
	}
	// Different content at same version.
	// If the artifact is in a terminal state (PUBLISHED, DEPRECATED, YANKED,
	// QUARANTINED, or REVOKED), overwrite is forbidden — the content is sealed.
	if isTerminalState(existingState) {
		return status.Errorf(codes.AlreadyExists,
			"artifact %s is in state %s with different content (existing=%s, new=%s); "+
				"overwrite of published artifacts is forbidden",
			key, existingState.String(), existingManifest.GetChecksum(), newChecksum)
	}
	// Pre-terminal state (STAGING, VERIFIED, FAILED, ORPHANED, UNSPECIFIED):
	// allow overwrite for staging iteration.
	slog.Warn("artifact overwrite (pre-publish, same version, different content)",
		"key", key, "state", existingState.String(),
		"old_checksum", existingManifest.GetChecksum(),
		"new_checksum", newChecksum)
}
```

### New helper function

Add near the top of `artifact_handlers.go` (after the existing helper functions, around line 120):

```go
// isTerminalState returns true if the artifact has reached a state where
// its content is sealed and must not be overwritten. This includes PUBLISHED
// and all post-PUBLISHED lifecycle states.
func isTerminalState(s repopb.PublishState) bool {
	switch s {
	case repopb.PublishState_PUBLISHED,
		repopb.PublishState_DEPRECATED,
		repopb.PublishState_YANKED,
		repopb.PublishState_QUARANTINED,
		repopb.PublishState_REVOKED:
		return true
	default:
		return false
	}
}
```

### Terminal State Definition (Phase 1)

Terminal states are artifact lifecycle states where the content is sealed and must not be overwritten. `isTerminalState` returns `true` for exactly these states:

**Terminal (content sealed, overwrite rejected):**
- `PUBLISHED` — fully published and discoverable
- `DEPRECATED` — superseded by a newer version, still downloadable
- `YANKED` — removed from discovery, download blocked for non-owners
- `QUARANTINED` — admin hold, security review pending
- `REVOKED` — permanently removed from all access

**Non-terminal (content mutable, overwrite allowed):**
- `PUBLISH_STATE_UNSPECIFIED` — legacy artifact without explicit state
- `STAGING` — upload in progress, not yet verified
- `VERIFIED` — upload complete, checksum verified, not yet discoverable
- `FAILED` — publish pipeline failed; artifact never reached users
- `ORPHANED` — artifact exists but descriptor registration failed; publish incomplete

`FAILED` and `ORPHANED` are numerically after `PUBLISHED` in the enum (values 4 and 5) but are NOT terminal. They represent broken publish attempts where the artifact was never successfully delivered. Allowing overwrite for these states enables recovery: the operator can re-upload corrected content without needing to bump the version.

### How state is read

`readManifestAndStateByKey` (line 138) reads the manifest JSON from storage and calls `unmarshalManifestWithState` (line 100) which extracts the `publishState` field from the JSON object. If the field is absent (legacy manifests), state defaults to `PublishState_PUBLISH_STATE_UNSPECIFIED` (enum value 0). This value is NOT in the terminal set, so legacy artifacts remain overwritable. This is the correct safe fallback.

### Checksum comparison

Already computed at line 589: `newChecksum := checksumBytes(data)`. Already compared at line 601: `existingManifest.GetChecksum() == newChecksum`. The different-checksum branch is the else of that check (implicit, via the early return on line 613). No change to checksum logic.

---

## 3. Controller Change — Desired State Validation

### File

`golang/cluster_controller/cluster_controller_server/desired_state_handlers.go`

### Function

`upsertOne` (lines 215–263)

### Exact insertion point

Between line 243 (end of version regression guard) and line 245 (start of `obj` construction). Insert a validation call:

**BEFORE (lines 243–245):**

```go
	}

	obj := &cluster_controllerpb.ServiceDesiredVersion{
```

**AFTER (lines 243–249):**

```go
	}

	// Phase 1: verify the artifact exists in the repository before writing
	// desired state. Fail closed: if repository is unreachable, reject.
	if err := srv.validateArtifactInRepo(ctx, canon, version, svc.BuildNumber); err != nil {
		return err
	}

	obj := &cluster_controllerpb.ServiceDesiredVersion{
```

### New helper function

Add after `highestHealthyInstalledVersion` (after line 296). This function reuses the same resolution logic as the release resolver:

```go
// validateArtifactInRepo verifies that the specified artifact exists in the
// repository and is reachable. This prevents desired state from referencing
// non-existent artifacts. Fails closed: if the repository is unreachable,
// the write is rejected.
func (srv *server) validateArtifactInRepo(ctx context.Context, serviceName, version string, buildNumber int64) error {
	// Resolve repository address — same path used by the release resolver.
	addr := config.ResolveServiceAddr("repository.PackageRepository", "")
	if addr == "" {
		return status.Errorf(codes.Unavailable,
			"repository address not configured; cannot validate artifact for %s@%s", serviceName, version)
	}

	// Use the system default publisher — same as ensureServiceRelease,
	// RemoveDesiredService, and the reconciler.
	publisher := defaultPublisherID()

	// Default platform — same as release_resolver.go:120-124.
	platform := "linux_amd64"

	repoClient, err := repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
	if err != nil {
		return status.Errorf(codes.Unavailable,
			"repository unreachable at %s; cannot validate artifact for %s@%s: %v", addr, serviceName, version, err)
	}
	defer repoClient.Close()

	ref := &repositorypb.ArtifactRef{
		PublisherId: publisher,
		Name:        serviceName,
		Version:     version,
		Platform:    platform,
	}

	_, err = repoClient.GetArtifactManifest(ref, buildNumber)
	if err != nil {
		code := status.Code(err)
		switch code {
		case codes.NotFound:
			return status.Errorf(codes.NotFound,
				"artifact %s@%s (build %d) not found in repository; "+
					"cannot set desired state for non-existent artifact",
				serviceName, version, buildNumber)
		case codes.Unavailable:
			return status.Errorf(codes.Unavailable,
				"repository unreachable at %s; cannot validate artifact for %s@%s: %v",
				addr, serviceName, version, err)
		default:
			return status.Errorf(codes.Internal,
				"repository validation failed for %s@%s: %v",
				serviceName, version, err)
		}
	}

	return nil
}
```

### New import required

Add `repositorypb` to the import block (line 10–27). The file already imports `repository_client` (line 23). Add:

```go
repositorypb "github.com/globulario/services/golang/repository/repositorypb"
```

### Resolution logic source

| Field | Source | Matches |
|-------|--------|---------|
| Repository address | `config.ResolveServiceAddr("repository.PackageRepository", "")` | Same as `release_resolver.go:66` and `desired_state_handlers.go:789` |
| Publisher | `defaultPublisherID()` → `"core@globular.io"` | Same as `handlers_upgrade.go:5-7`, `workflow_release.go:529`, `desired_state_handlers.go:340` |
| Platform | `"linux_amd64"` hardcoded | Same as `release_resolver.go:120-124` |
| Build number | Passed from `svc.BuildNumber` | Same field used by `ensureServiceRelease` at line 260 |

No new resolution logic invented. Every value comes from an existing code path.

### Phase 1 Validation Scope (Important Limitation)

Phase 1 validation ensures the artifact **exists in the repository** — specifically, that `GetArtifactManifest` returns a manifest for the given `(publisher, name, version, platform, buildNumber)`.

Phase 1 validation does **NOT** ensure:
- The artifact is in `PUBLISHED` state (for exact build number lookups). `GetArtifactManifest` with an exact build number returns any manifest regardless of state — including `VERIFIED`, `FAILED`, or `ORPHANED`.
- The artifact's checksum matches what the reconciler will later use.

This is acceptable because:
- The node agent's `CheckArtifactPublished` guard (`apply_package_release.go:102-107`) remains the **final enforcement point** for publish-state correctness. Even if desired state references a non-PUBLISHED artifact, the node agent will refuse to install it.
- Phase 1's goal is to prevent **non-existent artifact references** (the direct cause of the stuck-reconciler incident). Enforcing exact publish-state correctness at the desired-state level is a Phase 2+ concern.
- When `buildNumber=0` (resolve latest), `GetArtifactManifest` calls `resolveLatestBuildNumber` which already filters for `PUBLISHED` artifacts only. So the common case is already correct.

### Platform Resolution Note

Platform defaults to `"linux_amd64"` in `validateArtifactInRepo`. This is not a new rule — it mirrors the existing default in `release_resolver.go:120-124`:

```go
platform := strings.TrimSpace(spec.Platform)
if platform == "" {
    platform = "linux_amd64"
}
```

All current cluster nodes are `linux_amd64`. Future phases may generalize platform handling when multi-architecture support is needed.

---

## 4. Deploy Change — Remove Force Publish

### File

`golang/deploy/deploy.go`

### Function

`DeployService`

### Exact line to remove

Line 268: `"--force",`

**BEFORE (lines 264–269):**

```go
	publishArgs = append(publishArgs,
		"pkg", "publish",
		"--file", tgzPath,
		"--repository", repoAddr,
		"--force",
	)
```

**AFTER (lines 264–268):**

```go
	publishArgs = append(publishArgs,
		"pkg", "publish",
		"--file", tgzPath,
		"--repository", repoAddr,
	)
```

One line removed. No other changes.

---

## 5. Error Handling Details

### `codes.AlreadyExists` — repository overwrite protection

Used in `UploadArtifact` when existing artifact is in terminal state and checksum differs:

```go
return status.Errorf(codes.AlreadyExists,
	"artifact %s is in state %s with different content (existing=%s, new=%s); "+
		"overwrite of published artifacts is forbidden",
	key, existingState.String(), existingManifest.GetChecksum(), newChecksum)
```

### `codes.NotFound` — desired state validation

Used in `validateArtifactInRepo` when repository returns NotFound:

```go
return status.Errorf(codes.NotFound,
	"artifact %s@%s (build %d) not found in repository; "+
		"cannot set desired state for non-existent artifact",
	serviceName, version, buildNumber)
```

### `codes.Unavailable` — desired state validation

Used in `validateArtifactInRepo` when repository address is empty or connection fails:

```go
return status.Errorf(codes.Unavailable,
	"repository address not configured; cannot validate artifact for %s@%s",
	serviceName, version)
```

or:

```go
return status.Errorf(codes.Unavailable,
	"repository unreachable at %s; cannot validate artifact for %s@%s: %v",
	addr, serviceName, version, err)
```

### `codes.Internal` — desired state validation

Used in `validateArtifactInRepo` for unexpected errors:

```go
return status.Errorf(codes.Internal,
	"repository validation failed for %s@%s: %v",
	serviceName, version, err)
```

---

## 6. Safety Considerations

### Why this won't break existing flows

| Flow | Before | After | Safe? |
|------|--------|-------|-------|
| Publish new artifact | Succeeds | Succeeds (no existing artifact) | Yes |
| Idempotent re-upload (same checksum) | Succeeds | Succeeds (same-checksum path unchanged) | Yes |
| Deploy with changed binary | Force-publishes | Normal publish at new build_number (no collision) | Yes |
| Deploy with unchanged binary | Delta detection skips publish | Same (skip logic is before publish args) | Yes |
| `desired set` to existing PUBLISHED version | Writes to etcd | Validates then writes to etcd | Yes |
| CLI `pkg publish --force` | Deletes then re-uploads | Same (CLI force path unchanged) | Yes |

### New errors operators will see

| Error | When | Recovery |
|-------|------|----------|
| `AlreadyExists` on publish | Build number collision or attempt to overwrite PUBLISHED artifact | Re-run deploy (gets new build number) or use `globular pkg publish --force` explicitly |
| `NotFound` on desired set | Version/build doesn't exist in repository | Publish the artifact first, then set desired |
| `Unavailable` on desired set | Repository service is down or address not configured | Wait for repository to be available, retry |

### No data migration required

- No storage format changes
- No etcd schema changes
- No proto changes
- No state machine changes
- Existing `PublishState` enum used as-is

---

## 7. Test Implementation Plan

### Test 1: Repository — overwrite of PUBLISHED artifact rejected

**Where:** `golang/repository/repository_server/artifact_handlers_test.go` (new or existing test file)
**Type:** Unit test
**Scenario:**
1. Upload artifact at v1.0.0 build 1 (bytes = `[]byte("content-A")`)
2. Manually set publish state to PUBLISHED (or call completePublish)
3. Upload different artifact at v1.0.0 build 1 (bytes = `[]byte("content-B")`)
4. Assert: error returned, `status.Code(err) == codes.AlreadyExists`
5. Assert: message contains `"overwrite of published artifacts is forbidden"`
6. Assert: original artifact unchanged (read back manifest, verify checksum matches content-A)

### Test 2: Repository — overwrite of VERIFIED artifact allowed

**Where:** Same test file
**Type:** Unit test
**Scenario:**
1. Upload artifact at v1.0.0 build 1 (stays VERIFIED, no promote)
2. Upload different artifact at v1.0.0 build 1
3. Assert: success
4. Assert: manifest checksum matches new content

### Test 3: Repository — idempotent re-upload unchanged

**Where:** Same test file
**Type:** Unit test
**Scenario:**
1. Upload artifact, promote to PUBLISHED
2. Upload same bytes again
3. Assert: success (idempotent)

### Test 4: Repository — isTerminalState helper

**Where:** Same test file
**Type:** Unit test
**Scenario:**
- `isTerminalState(PublishState_PUBLISHED)` → true
- `isTerminalState(PublishState_DEPRECATED)` → true
- `isTerminalState(PublishState_YANKED)` → true
- `isTerminalState(PublishState_QUARANTINED)` → true
- `isTerminalState(PublishState_REVOKED)` → true
- `isTerminalState(PublishState_PUBLISH_STATE_UNSPECIFIED)` → false
- `isTerminalState(PublishState_STAGING)` → false
- `isTerminalState(PublishState_VERIFIED)` → false
- `isTerminalState(PublishState_FAILED)` → false
- `isTerminalState(PublishState_ORPHANED)` → false

### Test 5: Controller — desired set to non-existent artifact rejected

**Where:** `golang/cluster_controller/cluster_controller_server/desired_state_test.go` (new or existing)
**Type:** Integration test (requires repository mock or test instance)
**Scenario:**
1. Configure controller with repository address pointing to test repo
2. Call `UpsertDesiredService` with version `"9.9.9"` (not in repo)
3. Assert: error returned, `status.Code(err) == codes.NotFound`
4. Assert: no `ServiceDesiredVersion` written to resource store

### Test 6: Controller — desired set to existing artifact succeeds

**Where:** Same test file
**Type:** Integration test
**Scenario:**
1. Publish artifact `echo@0.0.8` build 1 to test repo
2. Call `UpsertDesiredService` with version `"0.0.8"`, build 1
3. Assert: success
4. Assert: `ServiceDesiredVersion` written with correct version

### Test 7: Deploy — no --force in publish args

**Where:** `golang/deploy/deploy_test.go` (if exists) or visual inspection
**Type:** Unit test or code review
**Scenario:**
- Verify `publishArgs` does not contain `"--force"` string
- Can be tested by inspecting the constructed args slice in a test harness
