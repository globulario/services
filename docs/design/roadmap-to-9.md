# Globular — Roadmap from 7.5 to 9+

This plan addresses every gap identified in the project assessment. Each item is concrete, scoped, and ordered by dependency and impact.

---

## Phase A: CLI Allocation Protocol (score impact: +0.3)

**Goal:** CLI and deploy pipeline use `AllocateUpload` — no more hardcoded versions.

### A1. Wire `--bump` into `globular pkg publish`
- Add `--bump patch|minor|major` flag to `pkg_cmds.go`
- When `--bump` is set, call `AllocateUpload` RPC before uploading
- Use the returned `version`, `build_id`, and `reservation_id`
- Remove `--version` as required (keep as optional override with `EXACT` intent)

### A2. Wire `--bump` into `globular deploy`
- `deploy.go` calls `AllocateUpload` instead of `NextBuildNumber`
- Remove hardcoded version logic entirely
- Read `build_id` from allocation response, pass through the pipeline
- `DeployResult` carries `BuildID` to desired-state update

### A3. Update `globular services desired set` to accept build_id
- When setting desired state with a specific build, pass `build_id` directly
- Controller validates against repository (already implemented)

### A4. Deprecation warnings
- If `--version` is used without `--bump`, log: "deprecated: use --bump to let the repository allocate versions"
- 90-day transition window before removing `--version` default behavior

---

## Phase B: Deploy and Validate Phases 3-7 (score impact: +0.3)

**Goal:** All Phase 3-7 code deployed to the live cluster and validated.

### B1. Deploy repository with Phase 3 (ledger + monotonicity)
- Build and publish repository with `release_ledger.go`
- Verify ledger migration runs on startup
- Test: upload version 0.0.1 after 0.0.8 → rejected with `FailedPrecondition`
- Test: `getLatestRelease()` returns correct build_id

### B2. Deploy repository with Phase 4 (allocation)
- Verify `AllocateUpload` RPC responds
- Test: call with `BUMP_PATCH` → returns next version
- Test: two concurrent allocations for same version → second gets `ResourceExhausted`
- Test: reservation expires after 5 minutes

### B3. Validate Phase 5 (repair tooling)
- Run `globular repository scan` on live cluster
- Verify classifications are correct (VALID, DUPLICATE_CONTENT, ORPHANED)
- Run `--cleanup-ghosts` and confirm ghost nodes are removed
- Query audit log at `/globular/audit/` and confirm entries

### B4. Validate Phase 6 (provisional import)
- Call `ImportProvisionalArtifact` with a test package
- Verify: same digest → idempotent
- Verify: different digest → rejected
- Verify: new version → added to ledger

### B5. Verify Phase 7 (discovery consolidation)
- Publish a new package, confirm descriptor appears in Resource service
- Confirm `pkg register` CLI is not needed (repository handles it)

---

## Phase C: Automated Invariant Tests (score impact: +0.5)

**Goal:** Every invariant (INV-1 through INV-10) has an automated test that runs on every commit.

### C1. Repository invariant tests
```
TestINV1_ReleasedArtifactImmutable
  - Upload v1.0.0 → PUBLISHED
  - Upload v1.0.0 with different digest → AlreadyExists
  - Upload v1.0.0 with same digest → idempotent success

TestINV2_MonotonicVersions
  - Upload v1.0.0 → PUBLISHED
  - Upload v0.9.0 → FailedPrecondition
  - Upload v1.0.1 → success

TestINV3_BuildIdServerGenerated
  - Upload with client-supplied build_id → ignored
  - Response contains server-generated UUIDv7
  - Manifest in storage has server build_id

TestINV4_BuildNumberDisplayOnly
  - Convergence comparison uses build_id only
  - No code path uses build_number for decisions
```

### C2. Convergence truth tests
```
TestConvergenceTruth_SuccessAfterActive
  - Apply package → response OK=true
  - Verify systemctl is-active returns true
  - Verify installed-state has buildId
  - Verify installed-state was NOT written before restart

TestConvergenceTruth_FailureOnRestartFail
  - Break service with systemd drop-in
  - Apply → response OK=false, status=failed
  - Verify installed-state has status=failed
  - No premature installed write

TestConvergenceTruth_SelfUpdate
  - Apply node-agent → status=upgrading
  - Upgrader runs in separate cgroup
  - After restart, installed-state has buildId
```

### C3. Desired-state tests
```
TestINV6_DesiredStateRequiresRepo
  - Set desired for non-existent version → NotFound
  - Set desired when repo unreachable → Unavailable
  - Set desired for existing version → success with build_id

TestINV7_OnlyReleasedInstallable
  - Upload artifact (VERIFIED, not PUBLISHED)
  - Apply → publish guard rejects
```

### C4. CI integration
- Add `make test-invariants` target
- Run on every PR via GitHub Actions
- Tests use in-process repository (no cluster required)
- Separate `make test-integration` for cluster-level tests

---

## Phase D: Service Health Cleanup (score impact: +0.3)

**Goal:** All services healthy on all nodes. Zero anomalies.

### D1. Fix backup-manager
- Check `journalctl -u globular-backup-manager.service` for crash reason
- Likely: ScyllaDB/MinIO connection config or missing dependency
- Fix and redeploy

### D2. Fix sql service
- Check `journalctl -u globular-sql.service` for crash reason
- Fix and redeploy

### D3. Fix alertmanager kind classification
- Installed-state has `kind=SERVICE` but should be `INFRASTRUCTURE`
- Update etcd records on all nodes
- Prevent future misclassification in the install path

### D4. Clean remaining anomalies
- Run `globular state canonicalize --dry-run`
- Target: 0 anomalies on active nodes
- Delete ghost node records
- Ensure all `mc` and `docs` packages are classified as metadata-only

### D5. Zero-anomaly validation
- Run full scan: `globular state canonicalize --dry-run` → 0 anomalies
- Run repo scan: `globular repository scan` → only VALID + expected ORPHANED
- All 3 nodes: `version:*` checks pass in health detail

---

## Phase E: Semantic Versioning (score impact: +0.2)

**Goal:** Move from 0.0.x to proper semver. Declare 1.0.0 when ready.

### E1. Define versioning policy
- Document in `docs/developers/versioning.md`
- Rules:
  - PATCH: bug fix, no API change
  - MINOR: new feature, backward compatible
  - MAJOR: breaking change
- All Globular services share a single version track (mono-version)
- Infrastructure packages keep upstream versions (etcd 3.5.14, prometheus 3.5.1)

### E2. Version bump to 0.1.0
- All Globular Go services: bump from 0.0.8 to 0.1.0
- This signals "Phase 2 complete, identity model stable"
- Use `AllocateUpload` with `BUMP_MINOR` to allocate

### E3. Define 1.0.0 criteria
- All invariants tested automatically
- Zero anomalies on a 3-node cluster for 7 consecutive days
- All Phase 1-7 deployed and validated
- Operator course updated to match
- At least one external operator has followed the course successfully

---

## Phase F: Test Cluster Simulation (score impact: +0.3)

**Goal:** Containerized test cluster for CI and development.

### F1. Docker Compose cluster
- 3 containers simulating globule-ryzen, globule-nuc, globule-dell
- Shared etcd, MinIO, ScyllaDB (in containers)
- All services run as systemd units inside containers (systemd-in-docker)

### F2. Integration test harness
- Go test suite that:
  - Boots the cluster
  - Publishes a test package
  - Sets desired state
  - Waits for convergence
  - Verifies installed-state has build_id
  - Tears down

### F3. CI pipeline
- GitHub Actions workflow
- On PR: run invariant tests (fast, no cluster)
- On merge to main: run integration tests (containerized cluster)
- On tag: run full deploy simulation

---

## Phase G: Day-0 Bootstrap Hardening (score impact: +0.1)

**Goal:** Clean day-0/day-1 boundary with provisional imports.

### G1. Installer sets provisional flag
- During day-0 install, node-agent writes `provisional=true` in installed-state
- Locally-generated build_id is marked as provisional

### G2. Bootstrap import trigger
- After repository service starts for the first time:
  - Node-agent detects repository is available
  - For each `provisional=true` package: call `ImportProvisionalArtifact`
  - On success: clear `provisional` flag, update `build_id` to confirmed value

### G3. Conflict resolution UI
- If import fails (same version, different digest): log clearly
- Operator resolves via CLI: `globular pkg import --resolve <package>`

---

## Execution Order

```
Week 1-2:  Phase D (service health) — quick wins, zero anomalies
Week 2-3:  Phase A (CLI allocation) — removes last manual identity step
Week 3-4:  Phase B (deploy + validate P3-7) — proves everything works live
Week 4-6:  Phase C (automated tests) — prevents regression forever
Week 6-7:  Phase E (semantic versioning) — signals maturity
Week 7-9:  Phase F (test cluster) — enables CI and external contributors
Week 9-10: Phase G (day-0 hardening) — completes the truth model

Total: ~10 weeks for a disciplined team of 1-2.
```

## Score Projection

| After Phase | Score | Why |
|-------------|-------|-----|
| Current | 7.5 | Architecture solid, implementation gaps |
| + D (health) | 7.8 | All services running, zero anomalies |
| + A (CLI) | 8.1 | Full repository authority, no manual identity |
| + B (validate) | 8.4 | All phases proven on live cluster |
| + C (tests) | 8.9 | Regression-proof, every invariant tested |
| + E (semver) | 9.0 | Maturity signal, versioning discipline |
| + F (CI) | 9.3 | External contributors can validate changes |
| + G (day-0) | 9.5 | Complete truth model, clean bootstrap |

---

## What 10 looks like

- Everything above, plus:
- Multi-cluster federation (desired state replicated across sites)
- Operator course validated by 3+ external operators
- Public documentation at docs.globular.io with versioned releases
- Performance benchmarks (convergence time, rollout latency)
- Security audit by external party
- The manifesto isn't just written — it's proven by the running system
