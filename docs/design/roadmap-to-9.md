# Globular — Foundation-Completion Roadmap (the coherence loop)

> **Status: COMMITTED LIVING DOC.** Check items off as they land. This is the
> active roadmap. The earlier infrastructure roadmap (7.5→9, package identity /
> deploy / health) is preserved below as **Appendix A** — most of it is delivered;
> this document supersedes it as the thing we are working from now.
>
> The spine of the ordering is one idea: **make awareness changes cheap-and-safe
> first** (so everything downstream is fast), then **close the loops**, then
> **universalize**, then **harvest the patterns into templates**. We templatize
> last because we template what is already proven stable.

---

## Where we are — the 7-point assessment

The honest read on the governance/coherence loop. `Status` is the current
maturity, not the target.

| # | Area | Status | What's missing |
|---|------|--------|----------------|
| 1 | **Code governance** | Mostly | Briefing-before-edit is hook-enforced; `awg validate` is a hard CI gate (failed #94 on duplicate_id — real teeth); impact-ci enforces required_tests. Gap is **coverage, not capability** — most invariants are still *proposed*; #95/#96 are hand-binding them two at a time. |
| 2 | **Runtime governance** | Half | Gateway primitive exists, desired-state owner paths are governed + cross-kind fails closed, raw etcd tools removed from MCP. But it's **opt-in** (`globular ops`), not universal/unbypassable — internal direct-write functions still exist; "owner-owned state mutates only through owner RPC" isn't kernel-enforced yet. |
| 3 | **Memory write-back** | Half | Approval works (behavioral gate + PR review); the path works end-to-end (incident→invariant→guard→test→promotion). But **both ends are manual**: incident→candidate is agent labor, and promotion→rebuild does not auto-trigger. |
| 4 | **Behavioral rules live** | Gap | CI ratchet proves the rule *will* enforce; `behavioral_check_action` exists. But **nothing enforces at runtime** — it's advisory, the rule isn't deployed (aliases compiled into the ai-memory binary), and MCP/CLI tools don't hard-refuse on a behavioral verdict. Biggest single gap. |
| 5 | **Graph coherence automatic** | Half | Duplicate-IDs caught pre-merge (awg-validate did exactly that on #94). But the **seed rebuild is manual cross-repo**, and orphan store-vs-YAML nodes are not auto-detected — the orphan subgraph was found by hand; nothing reconciles store-vs-YAML. |
| 6 | **Operator truth classified** | Mostly? | (Lowest confidence — doctor not audited this round.) Doctor does sophisticated separation: reduced-harvest UNKNOWN-vs-FAIL, harvest-vs-yield, profile/placement mismatch, orphaned install, kind-mismatch. Whether all five classes — esp. **deploy-debt as first-class** — are cleanly separated is unverified. |
| 7 | **Extension is boring** | Mostly | Invariant promotion is now a repeatable 1-file template (proven twice: #95, #96). Adding an owner path = one case + handler. But **full service extension** (package + awareness + behavioral + governance) is still multi-step, not one boring path. |

---

## Per-point feature lists (stable IDs)

### P1 — Code governance (Mostly → close the coverage tail)
- [x] **CG-1** Audit every proposed invariant for real guard+tests (evidence map) — *[invariant-evidence-map.md](invariant-evidence-map.md): 260 invariants, 93% active, complete critical/high test coverage. Fixed 6 malformed severities. Residual: 7 metadata-only anchor gaps (have tests) + 17 legitimately-planned. Corrected the "mostly proposed" premise.*
- [ ] **CG-2** Promote evidence-backed invariants to active + wire `required_tests` (the #95/#96 pattern, at scale)
- [ ] **CG-3** For invariants lacking evidence: build the guard+test, or mark explicitly aspirational with a tracking ref
- [x] **CG-4** Verify impact-ci actually fails when a protected file changes without its required tests — *VERIFIED: it does NOT — the enforcement does not exist. `awg impact` is advisory-only (no gate mode); CI runs a FIXED invariant-test subset (`TestINV|TestReservation|TestMigrate`) + principle-check + ratchets + validate/audit, but nothing maps a changed protected file → its `required_tests` and fails if they didn't run. "impact-ci enforces required_tests" was an overclaim. See **CG-5** to build it.*
- [ ] **CG-5** (new, from CG-4) Implement the impact-gate: `awg impact-gate --changed-files <git-diff>` resolves protecting invariants → their `required_tests` → fails the PR if those tests weren't run/didn't pass. Turns `required_tests` from advisory metadata into a fail-closed per-change gate.

### P2 — Runtime governance (Half → universal & unbypassable)
- [ ] **RT-1** Audit the full owner-owned-state write surface (code, MCP, CLI, scripts, etcd)
- [ ] **RT-2** Route/guard every path: owner RPC or explicit diagnostic-only; add server-side guards where missing
- [ ] **RT-3** govops becomes the enforced front door for dangerous CLI/MCP commands (gate, not opt-in)
- [ ] **RT-4** principle-check CI scanner forbidding new raw owner-owned write patterns (fail-closed)

### P3 — Memory write-back (Half → automate both ends)
- [x] **WB-1** Promotion → rebuild → checks fires automatically (needs GC-2) — *merge-time half: GC-2's `seed-rebuild.yml` auto-triggers the rebuild on merge. Local half: `awg promote` now fires the coherence gate (validate + audit -check, incl. seed-orphans) after its rebuild — same chain as `awg learn`, with a `-no-check` escape. Verified it fail-closes (caught a real committed dangling ref `desired.no_regression_all_paths` → missing `convergence.identity_is_build_id`).*
- [~] **WB-2** Incident→candidate generator: scar / doctor finding → draft invariant/forbidden_fix/test → review queue — *primitive built: `awg draft-candidate` (AG repo) renders a typed incident (doctor finding/scar) into a `status: candidate` entry in `docs/awareness/candidates/` with `discovered_from` provenance + per-class review_todo; never promotes/rebuilds (excluded from build until `awg promote`). Pure core + 5 tests. **Open**: auto-invoke from cluster_doctor finding emission (services-side wiring + which-findings-qualify policy).*
- [ ] **WB-3** End-to-end loop CI: scar → candidate → approve → promote → rebuild → validate, demonstrated

### P4 — Behavioral rules live (Gap → enforce at runtime)
- [ ] **BH-1** Wire `behavioral_check_action` as a synchronous hard refusal into mutation entry points (ops apply, MCP mutation tools)
- [ ] **BH-2** Deploy behavioral seed via release pipeline + promote the rule (ai-memory redeploy)
- [ ] **BH-3** Live verification: the check actually refuses a raw-write on the cluster

### P5 — Graph coherence automatic (Half → kill the manual dance)
- [x] **GC-1** Coherence pre-merge gate: orphan (store-vs-YAML) + duplicate-id (done) + dangling-ref, one hard gate — *orphan leg landed as `awg audit` check `seed-orphans` (hard FAIL); joins `awg validate` duplicate_id + dangling_*_ref. All three fail-closed in CI.*
- [x] **GC-2** Automated seed rebuild (yaml2nt → embeddata) on merge — the keystone weak rung — *master auto-commit (`seed-rebuild.yml`, direct push) + PR-side staleness downgraded to advisory (`build-awareness-graph.sh --warn-stale`); corpus-correctness (refs/contradictions/promotion) still hard-gates*
- [x] **GC-3** Live-store ↔ authored-YAML reconciliation job (catch awg-propose orphans like the one we found) — *new `awg reconcile` (AG repo): diffs the live Oxigraph store against the authored baseline (`-baseline yaml`=true-orphan detector / `seed`=deployed-runtime), names store-only orphans + lagging nodes, `--require-clean` gates. Found real drift on the live store (172 store-only nodes vs fresh YAML; ~27 hand-authored = high-signal, rest code-scan/cross-repo coverage) — needs operator diagnosis.*

### P6 — Operator truth classified (Unverified → audit then complete)
- [ ] **OT-1** Audit doctor's current categories vs the 5 target classes
- [ ] **OT-2** Implement/clarify missing classes — deploy-debt as first-class; clean split of runtime-defective-install / placement-violation
- [ ] **OT-3** Tests + awareness binding per class

### P7 — Extension boring (Mostly → harvest patterns last)
- [ ] **EX-1** `awg promote-invariant` scaffold (codify #95/#96)
- [ ] **EX-2** New owner-path dispatcher template + checklist
- [ ] **EX-3** New-service onboarding template (awareness reg + behavioral + governance hooks)
- [ ] **EX-4** "Adding X is boring" runbooks

---

## The ordered roadmap (dependency-respecting)

### Tier A — Make awareness changes cheap & safe (unblocks 3 of the 7)
- [x] 1. **GC-1** coherence pre-merge gate — protect first (S) ✅ orphan leg = `awg audit` `seed-orphans`
- [x] 2. **GC-2** automated seed rebuild — keystone; after this every promotion/authoring is cheap (M) ✅ `seed-rebuild.yml` (master auto-commit) + `--warn-stale` PR advisory; corpus-correctness still hard-gates
- [x] 3. **GC-3** store↔YAML reconciliation job (M) ✅ `awg reconcile` — surfaced 172 store-only nodes on the live store (real drift)

**Tier A complete.** Awareness changes are now cheap-and-safe: coherence is hard-gated pre-merge (GC-1), the seed auto-rebuilds on merge (GC-2), and live-store drift is detectable (GC-3). Next: Tier B (close the write-back loop — WB-1 is now unblocked by GC-2).

### Tier B — Close the write-back loop (needs GC-2)
- [x] 4. **WB-1** promotion→rebuild→checks automatic (S, after GC-2) ✅ GC-2 = merge-time rebuild; `awg promote` now fires validate+audit (the local half)
- [x] 5. **CG-1** invariant evidence audit — now cheap; feeds the grind (S) ✅ [invariant-evidence-map.md](invariant-evidence-map.md); fixed 6 malformed severities
- [~] 6. **WB-2** incident→candidate generator (L) — primitive `awg draft-candidate` done (+tests); open: cluster_doctor auto-wiring
- [ ] 7. **WB-3** end-to-end loop CI test (M)

### Tier C — Coverage grind (cheap after GC-2; parallelizable, ongoing)
- [ ] 8. **CG-2** promote evidence-backed invariants at scale (M, ongoing) — what #95/#96 do by hand; Tier A makes it boring
- [x] 9. **CG-4** confirm impact-ci enforcement fires (S) — ⚠️ VERIFIED ABSENT: no changed-file→required_tests gate exists; spawned **CG-5** to build it
- [ ] 10. **CG-3** build missing guard+test or mark aspirational (L, long tail)

### Tier D — Universalize runtime governance (gateway done; runs parallel to B/C — different subsystem)
- [ ] 11. **RT-1** direct-write surface audit (M) — spike; scopes the rest
- [ ] 12. **RT-2** route/guard all owner-owned writes (L)
- [ ] 13. **RT-3** govops as enforced front door (M)
- [ ] 14. **RT-4** principle-check scanner: no new raw writes (M)

### Tier E — Behavioral liveness (needs RT entry points + behavioral service)
- [ ] 15. **BH-1** wire check as hard refusal at mutation points (M) — consolidate with RT-4: one raw-write scanner serves both
- [ ] 16. **BH-2** deploy seed + promote rule (S work, gated by deploy decision)
- [ ] 17. **BH-3** live verification on cluster (S)

### Tier F — Operator truth (independent — slot in parallel any time after OT-1)
- [ ] 18. **OT-1** doctor classification audit (M) — spike
- [ ] 19. **OT-2** implement missing classes incl. deploy-debt (L)
- [ ] 20. **OT-3** tests + awareness binding (M)

### Tier G — Make extension boring (LAST — harvest proven patterns)
- [ ] 21. **EX-1** promote-invariant scaffold (S)
- [ ] 22. **EX-2** owner-path dispatcher template (S)
- [ ] 23. **EX-3** new-service onboarding template (M)
- [ ] 24. **EX-4** runbooks (S)

---

## Load-bearing ordering decisions

- **GC-2 is first-among-equals.** It's the single rung load-bearing for WB-1, makes
  all of Tier C cheap, and removes the manual step we keep deferring. Doing the
  coverage grind (Tier C) before GC-2 means grinding uphill — which is exactly what
  #95/#96 are doing right now.
- **Tier D runs parallel to B/C.** It touches Go/controller/MCP, not the awareness
  corpus, so it doesn't contend. Two work-streams: A→B/C on one, D→E on the other.
- **Two scanners are the same mechanism.** RT-4 and BH-1's code-level "no raw
  owner-owned write" check are *one* scanner, not two.
- **Tiers F and G are deferrable** without blocking "foundation complete" on the core
  loop — F is independent quality, G is ergonomics. But **OT-1's audit is worth doing
  early** just to de-risk the low-confidence score on #6.

---

# Appendix A — Prior infrastructure roadmap (7.5 → 9+, largely delivered)

> Preserved for history. This is the package-identity / deploy / health roadmap that
> preceded the coherence-loop roadmap above. Much of it has shipped; consult git
> history for status. The active roadmap is the coherence loop at the top of this file.

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
