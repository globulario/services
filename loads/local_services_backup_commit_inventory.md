# Local services backup commit inventory

**Date:** 2026-05-29
**Scope:** classify every commit reachable from
`backup/local-master-before-reconcile-20260529-144649` but not from
current `master`. Read-only; no cherry-pick, merge, rebase, reset,
push, or branch mutation in this turn.

---

## Current base

| Field | Value |
|---|---|
| `master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `origin/master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| backup ref | `backup/local-master-before-reconcile-20260529-144649` |
| backup SHA | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |

---

## Summary

| Bucket | Count |
|---|---|
| Total backup-only commits | **52** |
| `DROP_REDUNDANT_ALREADY_UPSTREAM` | **4** |
| `RECOVER_TOPIC_BRANCH` | **47** |
| `NEEDS_MANUAL_REVIEW` | **1** (Project L — conflict expected) |
| `DO_NOT_RECOVER` | **0** |

Total diff backup→master: 96 files changed, +12,563 / −186.

---

## Commit table

| # | SHA | Subject | Primary files touched | Classification | Reason |
|---:|---|---|---|---|---|
| 1 | `c185abde` | v1.2.119 draft — ApplyPackageRelease carries entrypoint_checksum | controller dispatch/recovery/repair, awareness docs, intent yaml + new test | RECOVER_TOPIC_BRANCH | hotfix work; not on origin |
| 2 | `f061d334` | v1.2.119 follow-up — workflow defs thread verification fields | awareness docs only | RECOVER_TOPIC_BRANCH | doc follow-up, harmless additive |
| 3 | `23a89318` | etcd NOSPACE blocks reconciliation | awareness docs only | RECOVER_TOPIC_BRANCH | doc follow-up |
| 4 | `6e8d01a2` | v1.2.119 hash-schema regression fix | node-agent grpc_workflow + test + inventory load | RECOVER_TOPIC_BRANCH | real bug fix; not on origin |
| 5 | `80d6667d` | Project A inventory — awareness-bundle identity mapping | loads/*.md only | RECOVER_TOPIC_BRANCH | inventory doc preserves discovery trail |
| 6 | `4dc2fb38` | Project A2 — identity registry + migration | identity.go + new alias migration + tests + intent yaml | RECOVER_TOPIC_BRANCH | foundational migration code |
| 7 | `5c537351` | A2 startup-ordering safety | reconcile_runtime.go | RECOVER_TOPIC_BRANCH | A2 follow-up |
| 8 | `13ebe537` | A2 result | loads/*.md only | RECOVER_TOPIC_BRANCH | doc closure |
| 9 | `a8d9c43a` | A3 platform-matching helper | release/platforms + resolver + sync_from_upstream + tests + intent yaml | RECOVER_TOPIC_BRANCH | platform fallback fix |
| 10 | `29cb4bb7` | A3 result | loads/*.md only | RECOVER_TOPIC_BRANCH | doc closure |
| 11 | `d03ec5f1` | A4 — kind-aware dispatch | controller reconcile_awareness_bundle (new) + release_pipeline + intent yaml + test | RECOVER_TOPIC_BRANCH | dispatch routing |
| 12 | `17579a3e` | A4 result | loads/*.md only | RECOVER_TOPIC_BRANCH | doc closure |
| 13 | `f5faae69` | A5 — kind-aware FAILED short-circuit | release_pipeline + release_reconciler + reconcile_awareness_bundle + new kind-aware test + intent yaml | RECOVER_TOPIC_BRANCH | A5 core |
| 14 | `b0b555cb` | A5 follow-up — extend kind-aware to AVAILABLE | release_pipeline + release_reconciler | RECOVER_TOPIC_BRANCH | A5 follow-up |
| 15 | `16355229` | A5 follow-up — suppress stale workflow MarkFailed | reconcile_awareness_bundle + workflow_release | RECOVER_TOPIC_BRANCH | A5 follow-up |
| 16 | `5edee29f` | A5 follow-up — applyPatchToSvcStatus SetFields handlers | reconcile_awareness_bundle + release_pipeline | RECOVER_TOPIC_BRANCH | A5 follow-up |
| 17 | `1616b123` | A5 follow-up — FAILED→PENDING phase transition | release_reconciler | RECOVER_TOPIC_BRANCH | A5 follow-up |
| 18 | `1e69384f` | route awareness_bundle SetFields to Svc switch | release_pipeline | RECOVER_TOPIC_BRANCH | A5 follow-up |
| 19 | `ebd3fd18` | A5 closure | loads/*.md only | RECOVER_TOPIC_BRANCH | doc closure |
| 20 | `9d1e36e5` | cluster_doctor: suppress spurious stable-state events | cluster_doctor/server.go + workflow_runner.go + new cache test | RECOVER_TOPIC_BRANCH | orphan but valuable doctor fix |
| 21 | `70f8871d` | Project B impact report + matrix | loads/*.md only | RECOVER_TOPIC_BRANCH | inventory doc |
| 22 | `14fbbc50` | Project B — self-hosted runtime proof writer | node_agent/heartbeat + new proof writer + test | RECOVER_TOPIC_BRANCH | Project B core |
| 23 | `a6af5d8f` | re-assert proof on every heartbeat tick | node_agent/heartbeat + awareness docs | RECOVER_TOPIC_BRANCH | Project B follow-up |
| 24 | `d83e0359` | Project B result | loads/*.md only | RECOVER_TOPIC_BRANCH | doc closure |
| 25 | `2a051583` | Project C inventory | loads/*.md only | RECOVER_TOPIC_BRANCH | inventory doc |
| 26 | `efa82071` | repository: RepairArtifact backfills Scylla index when integrity OK | repository/artifact_verify_rpc.go + plan doc | RECOVER_TOPIC_BRANCH | Project D core |
| 27 | `931318db` | RepairArtifact handles NULL manifest_json | repository/artifact_verify_rpc.go | RECOVER_TOPIC_BRANCH | Project D follow-up |
| 28 | `6a5bd635` | backfill bypasses state-machine | repository/artifact_verify_rpc.go | RECOVER_TOPIC_BRANCH | Project D follow-up |
| 29 | `248857dd` | backfill fires when manifest_json NULL even if PUBLISHED | repository/artifact_verify_rpc.go | RECOVER_TOPIC_BRANCH | Project D follow-up |
| 30 | `8d3fadc6` | Project D result | loads/*.md only | RECOVER_TOPIC_BRANCH | doc closure |
| 31 | `93118c05` | add repository to self-hosted proof writer allowlist | node_agent/self_hosted_runtime_proof_writer + test | RECOVER_TOPIC_BRANCH | Project B follow-up triggered by D's bridge symptom |
| 32 | `48700467` | Project E inventory — MinIO release label stale | loads/*.md only | RECOVER_TOPIC_BRANCH | inventory doc |
| 33 | `2383835a` | Project E2 — corrected MinIO inventory | loads/*.md only | RECOVER_TOPIC_BRANCH | inventory correction |
| 34 | `7f977ab5` | Project F — detectInfraDrift DEGRADED→AVAILABLE | controller/release_pipeline + new drift recovery test + plan doc | RECOVER_TOPIC_BRANCH | Project F core |
| 35 | `2861ae84` | Project F result | loads/*.md only | RECOVER_TOPIC_BRANCH | doc closure |
| 36 | `e723331b` | Project J inventory — committer checksum preservation | loads/*.md only | RECOVER_TOPIC_BRANCH | inventory doc |
| 37 | `ac866992` | workflow: nodeSyncPackageState writes entrypoint_checksum | workflow YAMLs (apply.infrastructure + apply.package) + actors.go + new test | RECOVER_TOPIC_BRANCH | Project J core |
| 38 | `9348560c` | Project J closure | awareness docs + loads/*.md | RECOVER_TOPIC_BRANCH | doc closure |
| 39 | `27ab5d0e` | Project K inventory | loads/*.md only | RECOVER_TOPIC_BRANCH | inventory doc |
| 40 | `756a6522` | installed_state_checksum_backfill — Phase 1/2/3 CLI | new cmd/installed_state_checksum_backfill/main.go | RECOVER_TOPIC_BRANCH | Project K CLI tool |
| 41 | `2fffb4d6` | backfill: don't bump UpdatedUnix; add repair mode | cmd/installed_state_checksum_backfill/main.go | RECOVER_TOPIC_BRANCH | Project K follow-up |
| 42 | `76d16734` | Project K result | loads/*.md only | RECOVER_TOPIC_BRANCH | doc closure |
| 43 | `a1feded1` | Project L — write systemd unit .sha256 sidecar after install | globularcli/services_cmds.go | **NEEDS_MANUAL_REVIEW** | overlaps Project O's edits to `services_cmds.go` (Project O is on origin/master). Conflict expected. |
| 44 | `ecdca55c` | anchor self-hosted proof timestamps to PID start | node_agent/self_hosted_runtime_proof_writer + test | RECOVER_TOPIC_BRANCH | Project B follow-up (INC-2026-0016) |
| 45 | `a03b1937` | Project N — dispatch wave_blocked workflows to retry | controller/release_pipeline + new wave-blocked test | RECOVER_TOPIC_BRANCH | Project N core |
| 46 | **`c529310e`** | Project O — WorkingDirectory normalize + invariant | (matches origin `947e3e2e`) | **DROP_REDUNDANT_ALREADY_UPSTREAM** | content equivalent to origin's PR #4 cherry-pick `947e3e2e` (patch-id confirmed). Drop. |
| 47 | `fa44aa57` | Project P — INFRASTRUCTURE remove phase fix | controller/release_pipeline + new remove-phase test + workflow_release + workflow YAML + engine/actors | RECOVER_TOPIC_BRANCH | Project P core |
| 48 | `eadc5690` | Project T — verifier honors manifest entrypoint via sidecar | node_agent/apply_package_release + installer_api + new test + versionutil + new versionutil test | RECOVER_TOPIC_BRANCH | Project T core |
| 49 | **`16af03a8`** | Project S — cluster_doctor scylla-manager invariant | (matches origin `73cb2516`) | **DROP_REDUNDANT_ALREADY_UPSTREAM** | content equivalent to origin's PR #5 cherry-pick `73cb2516` (patch-id confirmed). Drop. |
| 50 | `f10cb471` | Project Q — Spec.Paused on InfrastructureRelease | controller/release_pipeline + release_reconciler + new test + controllerpb/resources_types | RECOVER_TOPIC_BRANCH | Project Q core |
| 51 | **`1970dd7c`** | Project U.2 — registration script integration tests | (matches origin `9e2ee870`) | **DROP_REDUNDANT_ALREADY_UPSTREAM** | content equivalent to origin's PR #7 cherry-pick `9e2ee870` (patch-id confirmed). Drop. |
| 52 | **`b19ce3aa`** | Project U.3 — HTTPS-first probe | (matches origin `66d191e5`) | **DROP_REDUNDANT_ALREADY_UPSTREAM** | content equivalent to origin's PR #6 cherry-pick `66d191e5` (patch-id confirmed). The O.5-drop bug was in the *cherry-pick resolution* on the old `project-u3` branch, NOT in this commit. Drop. |

---

## Proposed recovery branches

Each branch should be cut fresh from `origin/master` at cherry-pick
time. Commit order is the chronological author order within each
group (cherry-pick in that order to preserve internal dependencies).

| # | Branch | Commits (in order) | Risk | Validation |
|---:|---|---|---|---|
| 1 | `recover/v1.2.119-hotfix-chain` | `c185abde`, `f061d334`, `23a89318`, `6e8d01a2` | **LOW** | `go test ./cluster_controller/cluster_controller_server ./node_agent/node_agent_server` |
| 2 | `recover/project-a-awareness-bundle-identity` | `80d6667d`, `4dc2fb38`, `5c537351`, `13ebe537`, `a8d9c43a`, `29cb4bb7`, `d03ec5f1`, `17579a3e`, `f5faae69`, `b0b555cb`, `16355229`, `5edee29f`, `1616b123`, `1e69384f`, `ebd3fd18` | **MEDIUM** — 15 commits with heavy intra-series interaction on `release_pipeline.go` and `release_reconciler.go`; applied as a unit they replay together (each commit's diff was authored against the prior one) | `go build ./...` then `go test ./cluster_controller/cluster_controller_server ./identity ./release/platforms ./repository/repository_server` |
| 3 | `recover/doctor-event-suppression-orphan` | `9d1e36e5` | **LOW** | `go test ./cluster_doctor/cluster_doctor_server` |
| 4 | `recover/project-b-self-hosted-proof-writer` | `70f8871d`, `14fbbc50`, `a6af5d8f`, `d83e0359`, `93118c05`, `ecdca55c` | **LOW** — single file with linear evolution; the 93118c05 allowlist extension and ecdca55c PID-anchor are kept here because they modify the file Project B created | `go test ./node_agent/node_agent_server` |
| 5 | `recover/project-c-d-repository-backfill` | `2a051583`, `efa82071`, `931318db`, `6a5bd635`, `248857dd`, `8d3fadc6` | **LOW** — all touch `repository/artifact_verify_rpc.go`; linear evolution | `go test ./repository/repository_server` |
| 6 | `recover/project-e-minio-inventory` | `48700467`, `2383835a` | **LOW** — docs only | none required (inventory docs) |
| 7 | `recover/project-f-minio-drift-recovery` | `7f977ab5`, `2861ae84` | **MEDIUM** — touches `release_pipeline.go` which also appears in branches #2 and #11/12; expected to apply if #2 lands first | `go test ./cluster_controller/cluster_controller_server` |
| 8 | `recover/project-j-workflow-checksum` | `e723331b`, `ac866992`, `9348560c` | **LOW** — workflow YAMLs + actors.go (independent of the release_pipeline lineage) | `go test ./workflow/engine` |
| 9 | `recover/project-k-checksum-backfill-cli` | `27ab5d0e`, `756a6522`, `2fffb4d6`, `76d16734` | **LOW** — new file under `cmd/installed_state_checksum_backfill/` | `go build ./cmd/installed_state_checksum_backfill` |
| 10 | `recover/project-l-systemd-sidecar` | `a1feded1` | **HIGH** — conflict expected on `services_cmds.go` (Project O is on origin and also edited that file). Manual resolution required: keep both Project O's `NormalizeUnitWorkingDirectory` call and Project L's sidecar write in the install path. | resolve conflict + `go build ./globularcli` + `go test ./globularcli` |
| 11 | `recover/project-n-wave-blocked-retry` | `a03b1937` | **MEDIUM** — touches `release_pipeline.go`; should apply if branches 2 and 7 have landed | `go test ./cluster_controller/cluster_controller_server` |
| 12 | `recover/project-p-infra-remove-phase` | `fa44aa57` | **MEDIUM** — touches `release_pipeline.go`, `workflow_release.go`, `engine/actors.go` (the last is also in branch #8) | `go test ./cluster_controller/cluster_controller_server ./workflow/engine` |
| 13 | `recover/project-t-verifier-entrypoint` | `eadc5690` | **LOW** — versionutil + node_agent installer/binary-path; independent | `go test ./versionutil ./node_agent/node_agent_server` |
| 14 | `recover/project-q-infra-spec-paused` | `f10cb471` | **MEDIUM** — touches `release_pipeline.go` and `release_reconciler.go`; should apply if branches 2 / 7 / 11 have landed | `go test ./cluster_controller/cluster_controller_server` |

Total: **14 recovery branches** covering 48 commits.

### Suggested merge order

Apply LOW-risk branches first to reduce surface area before tackling
MEDIUM/HIGH:

1. `recover/v1.2.119-hotfix-chain` (LOW)
2. `recover/doctor-event-suppression-orphan` (LOW)
3. `recover/project-b-self-hosted-proof-writer` (LOW)
4. `recover/project-c-d-repository-backfill` (LOW)
5. `recover/project-e-minio-inventory` (LOW)
6. `recover/project-j-workflow-checksum` (LOW)
7. `recover/project-k-checksum-backfill-cli` (LOW)
8. `recover/project-t-verifier-entrypoint` (LOW)
9. `recover/project-a-awareness-bundle-identity` (MEDIUM — 15 commits, biggest)
10. `recover/project-f-minio-drift-recovery` (MEDIUM)
11. `recover/project-n-wave-blocked-retry` (MEDIUM)
12. `recover/project-p-infra-remove-phase` (MEDIUM)
13. `recover/project-q-infra-spec-paused` (MEDIUM)
14. `recover/project-l-systemd-sidecar` (HIGH — last, conflict expected and small)

---

## Redundant commits — keep dropped

| SHA | Project | Matches origin commit | Why drop |
|---|---|---|---|
| `c529310e` | Project O | `947e3e2e` (via PR #4 merge `91b445c1`) | patch-id equivalent. Re-cherry-picking would create a "noop or conflict" rebase point. Origin's version has been live-tested and is the canonical artifact. |
| `16af03a8` | Project S | `73cb2516` (via PR #5 merge `6af791a5`) | patch-id equivalent. Same logic. |
| `b19ce3aa` | Project U.3 | `66d191e5` (via PR #6 merge `a272f415`) | patch-id equivalent. The O.5-line-drop bug that motivated the deprecated `project-u3` branch was **in the cherry-pick conflict resolution**, NOT in this commit's own diff. The local commit, applied on top of a base that already has Project O.5, is identical to origin's version. Drop. |
| `1970dd7c` | Project U.2 | `9e2ee870` (via PR #7 merge `068bf1eb`) | patch-id equivalent. Tests-only, already on master via PR #7. Drop. |

Recovering any of these from the backup would either be a no-op patch
or produce a conflict against the merge commit's `mainline=1`
state. There is no scenario where re-introducing them is desirable.

---

## Stop

Read-only inventory complete. Next operator decision is which (if any)
of the 14 recovery branches to authorize first; planning continues in
the per-branch turn once that authorization arrives.
