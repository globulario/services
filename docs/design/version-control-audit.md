# Globular Version Control Redesign v2 — Implementation Audit

## 1. Audit Summary

| Phase | Status | Completion |
|-------|--------|------------|
| **Phase 1: Immutability + Desired-State Guard** | **DONE** | 100% |
| **Phase 2: Repository-Issued Build Identity** | **DONE** | 100% |
| **Phase 3: Release Ledger + Monotonicity** | **DONE** | 100% |
| **Phase 4: Allocation Protocol** | **DONE** | 100% |
| **Phase 5: Repair Tooling** | **DONE** | 100% |
| **Phase 6: Day-0 Provisional Flow** | **DONE** | 100% |
| **Phase 7: Discovery Consolidation** | **DONE** | 100% |

### Phase-by-Phase Detail

| Redesign Item | Status | Evidence in Code | Gap | Required Action |
|---------------|--------|------------------|-----|-----------------|
| **P1: Released artifact immutability** | DONE | `artifact_handlers.go:697-701` — `isTerminalState()` rejects overwrite of PUBLISHED/DEPRECATED/YANKED/QUARANTINED/REVOKED | None | None |
| **P1: --force cannot bypass RELEASED** | DONE | `artifact_handlers.go:693-701` — terminal state check runs before overwrite regardless of force | None | None |
| **P1: Desired-state repo validation** | DONE | `desired_state_handlers.go:315-370` — `validateArtifactInRepo()` queries repo, returns build_id, fails closed on Unavailable | None | None |
| **P1: Convergence uses build_id** | DONE | `release_pipeline.go:53-65` — sole build_id comparison, no fallback | None | None |
| **P2: Repository generates build_id** | DONE | `artifact_handlers.go:727` — `uuid.Must(uuid.NewV7()).String()` on upload | None | None |
| **P2: build_id in manifests** | DONE | `repository.proto` field 42, `ArtifactManifest.build_id` | None | None |
| **P2: build_id in installed-state** | DONE | `node_agent.proto` field 14, `InstalledPackage.build_id` | None | None |
| **P2: build_id in desired-state** | DONE | `resources_types.go:30` — `ServiceDesiredVersionSpec.BuildID` | None | None |
| **P2: Convergence build_id only** | DONE | `release_pipeline.go:60`, `workflow_release.go:522-528` — no fallback | None | None |
| **P2: Backfill migration** | DONE | `migration.go:MigrateBuildIDs()` — UUIDv5 synthetic for old artifacts | None | None |
| **P3: Release ledger** | DONE | `release_ledger.go:appendToLedger()` line 150; `getLatestRelease()` line 206 | None | None |
| **P3: Monotonic version enforcement** | DONE | `artifact_handlers.go:715-719` — rejects version < latest PUBLISHED; `release_ledger.go:162-170` — ledger-level monotonic check | None | None |
| **P3: Latest resolution from ledger** | DONE | `release_ledger.go:getLatestRelease()` O(1) lookup; `MigrateReleaseLedger()` builds from existing artifacts | None | None |
| **P3: Version → build_id deterministic** | DONE | `release_resolver.go:164` extracts build_id from manifest | None | None |
| **P4: AllocateUpload RPC** | DONE | `allocate_upload.go:AllocateUpload()` line 123; `reservationStore` lines 46-53; 5-min TTL | None | None |
| **P4: Version bump intent** | DONE | `allocate_upload.go:resolveVersionIntent()` line 175; `bumpVersion()` line 222; `VersionIntent` enum in proto | None | CLI wiring (`--bump` flag) is follow-up |
| **P4: Client cannot invent identity** | DONE | `AllocateUpload` allocates version; legacy path validates monotonicity; build_id always repository-generated | None | None |
| **P5: Repository scan command** | DONE | `repo_scan_cmds.go:runRepoScan()` line 74 — VALID/DUPLICATE_DIGEST/DUPLICATE_CONTENT/ORPHANED/MISSING_BUILD_ID | None | None |
| **P5: Repair with audit records** | DONE | `audit_log.go:writeAuditRecord()` line 34 — persists to etcd `/globular/audit/`; wired into A2 repair and metadata-only repair | None | None |
| **P5: Ghost-node cleanup** | DONE | `state_cmds.go:cleanupGhostNodes()` — queries active nodes, deletes ghost records, writes audit | None | None |
| **P5: Dry-run mode** | DONE | `--dry-run` flag on canonicalize tool | None | None |
| **P6: Provisional flag** | DONE | `InstalledPackage.provisional` field 15; `ArtifactManifest.provisional` field 43 | None | None |
| **P6: Day-0 import flow** | DONE | `import_provisional.go:ImportProvisionalArtifact()` line 37 — validates, assigns confirmed build_id, adds to ledger | None | Node-agent bootstrap import trigger is follow-up |
| **P7: Repository drives catalog** | DONE | `publish_workflow.go:registerDescriptor()` line 147 — called from `completePublish()` | None | None |
| **P7: Discovery reflects repository** | DONE | `pkg register` CLI marked TRANSITIONAL; repository is sole registrar via `completePublish()` | None | Remove CLI `pkg register` after transition |

## 2. Exact Remaining Work

### Priority 1: Critical Path (Phase 3)

**1. Release Ledger**
- Files: new `golang/repository/repository_server/release_ledger.go`
- Data: per-package JSON stored in MinIO at `ledger/{publisher}%{name}.json` or in ScyllaDB `repository.release_ledger` table
- Schema: `{ package, latest_released: {version, build_id, released_at}, releases: [{version, state, platforms: {platform: {build_id, digest}}}] }`
- Written on: PromoteArtifact (VERIFIED → PUBLISHED transition)
- Read by: release resolver for O(1) latest lookup

**2. Monotonic Version Enforcement in Repository**
- File: `golang/repository/repository_server/artifact_handlers.go`
- Where: In `UploadArtifact`, after uniqueness check, before writing manifest
- Logic: Read ledger for (publisher, name). If latest RELEASED version exists and new version ≤ latest, reject with `FailedPrecondition`
- Exception: Same version is allowed if no RELEASED build exists yet (staging/iterating)

**3. Latest Resolution from Ledger**
- File: `golang/repository/repository_server/artifact_handlers.go`
- Refactor: `resolveLatestBuildNumber()` reads from ledger instead of scanning directory
- Fallback: If ledger doesn't exist (pre-migration), scan as before

### Priority 2: Mixed-Authority Removal

**4. Demote build_number to Display-Only**
- Files: `golang/deploy/deploy.go`, `golang/globularcli/pkg_cmds.go`
- Change: build_number is read from upload response, never computed client-side
- Deploy path should not query `NextBuildNumber()` — let repository assign
- Remove `deploy/buildnumber.go` or mark deprecated

**5. Package Classification Contract**
- File: new section in CLAUDE.md or new `docs/operators/package-classification.md`
- Categories: `managed+desired-state`, `managed+metadata-only`, `provisional`, `ghost/stale`, `unmanaged`
- `mc`, `docs`: classify as `managed+metadata-only` (installed, no desired-state)
- Canonicalization tool: exclude `metadata-only` from desired-state anomaly counts

### Priority 3: Repair Tooling (Phase 5)

**6. Repository Scan Command**
- File: extend `golang/globularcli/state_cmds.go` or new `golang/globularcli/repo_scan_cmds.go`
- Classifications: VALID, DUPLICATE_DIGEST, DUPLICATE_CONTENT, NON_MONOTONIC, ORPHANED, MISSING_MANIFEST, PROVISIONAL
- Output: classification report per artifact

**7. Audit Log for Repairs**
- File: new `golang/repository/repository_server/audit_log.go`
- Storage: ScyllaDB `repository.audit_log` table or MinIO `audit/` prefix
- Schema: `{action, artifact, operator, reason, before_state, after_state, timestamp}`
- Written by: every state mutation in canonicalize tool and repository

### Priority 4: Ghost-Node Hygiene

**8. Ghost-Node Cleanup**
- File: extend `golang/globularcli/state_cmds.go`
- Mode: `globular state canonicalize --cleanup-ghosts`
- Logic: List active nodes from controller. Any installed-state record on a non-active node is ghost. Delete with audit log.

### Priority 5: Day-0 Provisional Flow (Phase 6)

**9. Provisional Flag**
- Proto: add `bool provisional = 15` to InstalledPackage
- Proto: add `bool provisional = 43` to ArtifactManifest
- Installer: set `provisional=true` during day-0 install
- Node-agent: carry through to installed-state

**10. ImportProvisionalArtifact RPC**
- Proto: new RPC in repository.proto
- Handler: validates version/digest, assigns confirmed build_id, adds to ledger
- Node-agent: on bootstrap, calls import for each provisional record

### Priority 6: Allocation Protocol (Phase 4)

**11. AllocateUpload RPC**
- Proto: new RPC in repository.proto
- Handler: reservation with 5-min TTL, assigns version + build_id
- CLI: `globular pkg publish --bump patch` sends intent, receives allocation
- UploadArtifact: accepts `reservation_id`, validates against active reservation

### Priority 7: Discovery Consolidation (Phase 7)

**12. Remove CLI-Side Descriptor Registration**
- File: `golang/globularcli/pkg_cmds.go`
- Change: Remove `setPackageDescriptor()` call after publish
- Repository's `completePublish()` already handles this

**13. Retry Queue for Failed Registrations**
- File: `golang/repository/repository_server/publish_reconciler.go`
- Already partially exists — extend to retry failed Resource service calls

## 3. Code Changes Per Item

| # | Item | Files | RPCs/Functions | State Affected | Invariant |
|---|------|-------|---------------|---------------|-----------|
| 1 | Release ledger | new `release_ledger.go` | Write on PromoteArtifact | MinIO or ScyllaDB | INV-2 (monotonic) |
| 2 | Monotonic enforcement | `artifact_handlers.go` | UploadArtifact | None (rejection) | INV-2 |
| 3 | Ledger-based resolution | `artifact_handlers.go` | resolveLatestBuildNumber | None (read) | Performance |
| 4 | build_number display-only | `deploy.go`, `pkg_cmds.go` | Deploy pipeline | None | INV-4 |
| 5 | Package classification | CLAUDE.md, new doc | Canonicalize tool | None | Scope clarity |
| 6 | Repository scan | `repo_scan_cmds.go` | CLI command | None (read-only) | INV-10 |
| 7 | Audit log | `audit_log.go` | All repair mutations | ScyllaDB/MinIO | INV-10 |
| 8 | Ghost cleanup | `state_cmds.go` | CLI command | etcd (delete) | INV-10 |
| 9 | Provisional flag | protos, installer | Install/import | etcd, manifests | INV-9 |
| 10 | Import RPC | `repository.proto`, handler | ImportProvisionalArtifact | Repository + etcd | INV-9 |
| 11 | AllocateUpload | `repository.proto`, handler | AllocateUpload | Repository | INV-3, INV-5 |
| 12 | Remove CLI registration | `pkg_cmds.go` | Publish pipeline | Resource service | INV-8 |
| 13 | Retry queue | `publish_reconciler.go` | Background reconciler | Resource service | INV-8 |

## 4. Success Criteria

| Criterion | How to Verify |
|-----------|---------------|
| RELEASED artifacts are immutable | Upload same version+different digest → `AlreadyExists` |
| Release versions are monotonic | Upload version 0.0.2 after 0.0.8 is RELEASED → `FailedPrecondition` |
| build_id is sole identity | `isServiceConverged()` has zero version/build_number comparison paths |
| build_number is display-only | No code path uses build_number for convergence, idempotency, or resolution |
| Desired-state requires repo confirmation | `UpsertDesiredService` with non-existent version → `NotFound` |
| Only RELEASED artifacts installable | `ApplyPackageRelease` with VERIFIED artifact → rejected by publish guard |
| Release ledger exists | `ledger/{publisher}%{name}.json` in MinIO has correct release history |
| Resolution is O(1) | `resolveLatestBuildNumber` reads ledger, not scans directory |
| All repairs produce audit records | Every `--fix-*` operation writes to audit log with before/after state |
| Ghost nodes are classifiable | `state canonicalize --dry-run` marks ghost-node records separately |
| Package scope is explicit | Every package has a classification: managed, metadata-only, provisional, or unmanaged |
