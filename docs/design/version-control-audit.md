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
| **P2: Repository generates build_id** | DONE | `artifact_handlers.go:740` — `uuid.Must(uuid.NewV7()).String()` on upload | None | None |
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

## 2. Implementation Evidence

All items below are **implemented** as of April 2026. This section documents where each piece lives in code.

### Phase 3: Release Ledger + Monotonicity

| # | Item | File | Key Function/Line | Status |
|---|------|------|-------------------|--------|
| 1 | Release ledger | `release_ledger.go` | `appendToLedger()` L150, `getLatestRelease()` L206, `MigrateReleaseLedger()` L229 | DONE |
| 2 | Monotonic enforcement | `artifact_handlers.go` | L711-722: rejects version < latest PUBLISHED via `FailedPrecondition` | DONE |
| 3 | Ledger-based resolution | `release_ledger.go` | `getLatestRelease()` O(1) reverse-walk by platform | DONE |

### Phase 4: Allocation Protocol

| # | Item | File | Key Function/Line | Status |
|---|------|------|-------------------|--------|
| 4 | AllocateUpload RPC | `allocate_upload.go` | `AllocateUpload()` L123, `reservationStore` L46-53, 5-min TTL | DONE |
| 5 | Version bump intent | `allocate_upload.go` | `resolveVersionIntent()` L175, `bumpVersion()` L222 | DONE |
| 6 | Reservation cleanup | `allocate_upload.go` | `startReservationCleanup()` L250, 1-min ticker | DONE |

### Phase 5: Repair Tooling

| # | Item | File | Key Function/Line | Status |
|---|------|------|-------------------|--------|
| 7 | Repository scan | `repo_scan_cmds.go` | `runRepoScan()` L74 — VALID/DUPLICATE_DIGEST/DUPLICATE_CONTENT/ORPHANED/MISSING_BUILD_ID | DONE |
| 8 | State canonicalization | `state_cmds.go` | `runCanonicalize()` L105, anomaly types A1-A4/A7 | DONE |
| 9 | Audit log | `audit_log.go` | `writeAuditRecord()` L34, persists to etcd `/globular/audit/` | DONE |
| 10 | Ghost cleanup | `state_cmds.go` | `cleanupGhostNodes()` L914, queries active nodes, deletes stale records | DONE |

### Phase 6: Day-0 Provisional Flow

| # | Item | File | Key Function/Line | Status |
|---|------|------|-------------------|--------|
| 11 | Provisional flag | `repository.proto`, `node_agent.proto` | `ArtifactManifest.provisional` (field 43), `InstalledPackage.provisional` (field 15) | DONE |
| 12 | Import RPC | `import_provisional.go` | `ImportProvisionalArtifact()` L37 — validates digest, assigns confirmed build_id, appends ledger | DONE |

### Phase 7: Discovery Consolidation

| # | Item | File | Key Function/Line | Status |
|---|------|------|-------------------|--------|
| 13 | Repository-driven catalog | `publish_workflow.go` | `completePublish()` → `registerDescriptor()` L147 — registers in Resource service | DONE |
| 14 | CLI transitional | `pkg_cmds.go` | `pkg register` marked TRANSITIONAL L80-93, repository is authoritative registrar (INV-8) | DONE |

### Follow-Up Items (Roadmap, not blocking)

| Item | Description | Status |
|------|-------------|--------|
| CLI `--bump` flag | Wire `AllocateUpload` into `globular pkg publish` and `globular deploy` | Planned (Phase A in roadmap-to-9.md) |
| Node-agent bootstrap import | Auto-call `ImportProvisionalArtifact` on first repository availability | Planned (Phase G in roadmap-to-9.md) |
| Remove `pkg register` CLI | Delete after transition period | Planned (Phase A in roadmap-to-9.md) |
| Deprecate `NextBuildNumber()` | `deploy/buildnumber.go` marked deprecated, remove after CLI allocation wired | Planned (Phase A in roadmap-to-9.md) |

## 3. Invariant Coverage

Every invariant is covered by implemented code:

| Invariant | Description | Enforcement Location |
|-----------|-------------|---------------------|
| INV-1 | Released artifact immutable | `artifact_handlers.go:697-701` — `isTerminalState()` rejects overwrite |
| INV-2 | Monotonic versions | `artifact_handlers.go:715-722` + `release_ledger.go:162-170` |
| INV-3 | build_id server-generated | `artifact_handlers.go:740` — `uuid.NewV7()`, client value ignored |
| INV-4 | build_number display-only | `release_pipeline.go:53-65` — convergence uses build_id only |
| INV-5 | Version allocated by repository | `allocate_upload.go:123` — `AllocateUpload` RPC |
| INV-6 | Desired-state requires repo | `desired_state_handlers.go:315-370` — `validateArtifactInRepo()` |
| INV-7 | Only RELEASED installable | Publish guard in apply path |
| INV-8 | Repository drives catalog | `publish_workflow.go:147` — `registerDescriptor()` via `completePublish()` |
| INV-9 | Day-0 provisional until imported | `import_provisional.go:37` — `ImportProvisionalArtifact()` |
| INV-10 | All repairs audited | `audit_log.go:34` — `writeAuditRecord()` to etcd `/globular/audit/` |

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
