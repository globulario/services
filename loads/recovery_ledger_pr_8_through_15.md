# Recovery ledger — PRs #8 through #15

**Date:** 2026-05-29 (continuous session)
**Scope:** all source-recovery work from the local `master` ⇄
`origin/master` reconciliation, anchored to the backup ref
`backup/local-master-before-reconcile-20260529-144649` at
`b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41`.

This is the final ledger called for by the plan
`/home/dave/Downloads/remaining_recovery_pr_11_to_16_plan.md`'s PR
#16. It records what landed, what stayed deferred, and the next
technical phase. No code change; no branch deletion authorized in
this turn.

---

## Final master state

| Field | Value |
|---|---|
| `master` (local) | `34268c959a6153288eb86d1a63f9cc85027f9be5` |
| `origin/master` | `34268c959a6153288eb86d1a63f9cc85027f9be5` |
| `backup` ref | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` (intact) |
| Tracked working tree | clean |
| Runtime: `globular-scylla-manager.service` | `ActiveState=active`, `NRestarts=0`, `MainPID=770002` (same PID since U.1) |

---

## PRs merged: #8 through #15

| PR | Title | Merge commit | Source commit | Status |
|---|---|---|---|---|
| #8 | Recover v1.2.119 hotfix chain | `536ff038` | `b894b2e2`…`14b16a06` (4 commits + YAML fix) | merged |
| #9 | Recover doctor event suppression orphan handling | `910db74b` | `1a5bbcde` (1 commit) | merged |
| #10 | Recover self-hosted proof writer | `5efff853` | `28c35596` (5 commits; `a6af5d8f` deferred) | merged |
| #11 | Recover Project C-D repository backfill | `df0b873b` | `65fff2e3` (6 commits) | merged |
| #12 | Recover MinIO inventory documentation | `56b3013c` | `9d0c529b` (2 commits, docs-only) | merged |
| #13 | Recover workflow checksum handling | `effc76f4` | `7cf2195d` (2 commits; `9348560c` deferred) | merged |
| #14 | Recover checksum backfill CLI | `36f9315c` | `647db0e0` (4 commits) | merged |
| #15 | Recover verifier entrypoint handling | `34268c95` | `0498b4f4` (1 commit) | merged |

**Total source content landed:** 8 PRs spanning 25 source commits +
the v1.2.119 YAML quote-fix commit (`14b16a06`) authored mid-session
to unblock CI on PR #8.

The complete commit graph from `git log --oneline --decorate -20`
matches this table.

---

## Branches recovered (content content-equivalent to master)

All 8 `recover/*` branches built earlier in this session now have
their content reachable from `origin/master` via the corresponding
`review/*` cherry-pick + PR merge:

| Source branch (local) | SHA | Landed via |
|---|---|---|
| `recover/v1.2.119-hotfix-chain` | `b2cb8ad2` (later `14b16a06` after YAML fix) | PR #8 directly |
| `recover/doctor-event-suppression-orphan` | `8194aa24` | PR #9 via `review/doctor-event-suppression-orphan` |
| `recover/project-b-self-hosted-proof-writer` | `298aab78` | PR #10 via `review/project-b-self-hosted-proof-writer` |
| `recover/project-c-d-repository-backfill` | `6436d6e8` | PR #11 via `review/project-c-d-repository-backfill` |
| `recover/project-e-minio-inventory` | `8365e180` | PR #12 via `review/project-e-minio-inventory` |
| `recover/project-j-workflow-checksum` | `98bc84db` | PR #13 via `review/project-j-workflow-checksum` |
| `recover/project-k-checksum-backfill-cli` | `12cc1b4e` | PR #14 via `review/project-k-checksum-backfill-cli` |
| `recover/project-t-verifier-entrypoint` | `b7649e34` | PR #15 via `review/project-t-verifier-entrypoint` |

### Why `git branch --merged master` shows only 1 of 8

`git branch --merged master` reports `recover/v1.2.119-hotfix-chain`
as merged but NOT the other 7 `recover/*` branches. This is
expected:

- `recover/v1.2.119-hotfix-chain` was **pushed and PR'd directly** —
  its tip SHA is an ancestor of master via the PR #8 merge.
- The other 7 `recover/*` branches were not directly pushed. Their
  content was cherry-picked onto fresh `review/*` branches with
  new commit SHAs. The original `recover/*` SHAs are NOT ancestors
  of master — git's `--merged` cannot recognize them — but the
  content is fully present.

`git branch --merged master` shows the 8 `review/*` branches as
merged (because their tips ARE ancestors via the merge commits' second
parents). That is the authoritative "all recovery content landed"
signal.

---

## Branches intentionally deferred

Two commits remained on the backup ref only, deliberately excluded
from master:

### `a6af5d8f` — node-agent: re-assert self-hosted runtime proof on every heartbeat tick

| Aspect | Value |
|---|---|
| Subject | `node-agent: re-assert self-hosted runtime proof on every heartbeat tick` |
| Source size | 3 files, +159 (heartbeat.go + 87 lines `failure_modes.yaml` + 67 lines `invariants.yaml`) |
| Why deferred | Awareness YAML additions target line positions ahead of master — the Project A awareness chain (A2/A3/A4/A5) has not been recovered yet. The 4 v1.2.119 awareness entries that PR #8 added were not enough to absorb `a6af5d8f`'s starting point. |
| Functional gap on master | Proof writer still runs on **startup** and **every 5 minutes** (sufficient for the as-shipped Project B contract). The 30-second heartbeat refresh is the only thing missing — a quality-of-service improvement, not a correctness fix. |
| Cross-check | `git merge-base --is-ancestor a6af5d8f master` → exit `1` (not an ancestor) ✓ |
| Recovery path | After Project A series lands on master, retry the cherry-pick — likely clean. |

### `9348560c` — Project J closure: awareness records + result report

| Aspect | Value |
|---|---|
| Subject | `Project J closure: awareness records + result report` |
| Source size | 3 files, +269 (57 lines `failure_modes.yaml` + 52 lines `invariants.yaml` + 160 lines result doc) |
| Why deferred | Same awareness-YAML dependency as `a6af5d8f`. The result documentation was held back so the awareness records can land in the right base order. |
| Functional gap on master | None — the Project J code fix and its impact/matrix evidence both landed via PR #13. Only the post-fix awareness records and the formal result doc are missing. |
| Cross-check | `git merge-base --is-ancestor 9348560c master` → exit `1` ✓ |
| Recovery path | Same as `a6af5d8f` — re-cherry-pick after Project A series lands. |

**Neither deferred commit affects production code behavior on master.**
Both are documentation/instrumentation rather than functional regressions.

---

## Cumulative validation summary

Every PR ran the targeted Go test package(s) for its touched files.
Cumulative passes:

| Package | Last run | Result |
|---|---|---|
| `awareness/learning` | PR #8 quote-fix verification | ok 0.485s |
| `cluster_doctor/cluster_doctor_server` | PR #9 post-merge | ok 0.203s |
| `node_agent/node_agent_server` | PR #10 + PR #15 post-merges | ok 109s / 116s |
| `repository/repository_server` | PR #11 post-merge | ok 1.649s |
| `workflow/engine` | PR #13 post-merge | ok 55.618s |
| `cmd/installed_state_checksum_backfill` (build only) | PR #14 post-merge | BUILD OK; `--help` printed; no I/O |
| `versionutil` | PR #15 post-merge | ok 0.425s |
| `go build ./...` | every PR | silent every time |

CI on PRs #8–#15 settled to:
- Documentation workflow checks: all green
- `ci/proto-check`: green
- `ci/build-test`: green after the YAML quote-fix (PR #8 specifically)
- `ci/lint`: persistently fails on a runner-shutdown infrastructure
  issue at ~2 minutes regardless of content. Operator confirmed the
  failure is environment, not code. Lint is **not a required check**
  on this repo's `master` branch protection — merges proceeded normally.

---

## Runtime safety summary

Across the entire 8-PR span:

- ✓ **0** `pkg build`
- ✓ **0** `pkg publish`
- ✓ **0** `services desired set`
- ✓ **0** service restarts
- ✓ **0** cluster mutations
- ✓ **0** packages-repo touches
- ✓ **0** destructive git operations on `master` or `origin/master`
- ✓ **0** force-pushes
- ✓ `globular-scylla-manager.service` `MainPID=770002` continuously
  from the U.1 deploy through the end of this session — never
  restarted

All source landed on `origin/master`; **no deployed binary changed**.
Live runtime behavior is governed by the binaries deployed during U.1
(scylla-manager 1.2.75) and earlier project deploys. Each merged PR
will surface its runtime effect only when the operator chooses to
re-deploy the affected service via the normal `pkg build` →
`pkg publish` → `services desired set` pipeline.

---

## Backup ref status

`backup/local-master-before-reconcile-20260529-144649`
- SHA: `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41`
- State: **intact** — never modified, force-updated, or deleted in
  this session
- Reachable commits: every commit on the original pre-reconcile
  `master`, including the 2 deferred commits `a6af5d8f` and
  `9348560c`
- Recommended retention: **keep until the deferred commits are
  successfully re-cherry-picked.** Deletion proposal documented but
  not authorized in this turn.

---

## Local branch inventory

Local branches present at session end:

| Branch | Type | SHA | Disposition (recommended, not authorized) |
|---|---|---|---|
| `master` | default | `34268c95` | keep (active branch) |
| `backup/local-master-before-reconcile-20260529-144649` | safety net | `b19ce3aa` | **keep** until deferred commits re-cherry-pick succeeds |
| `recover/doctor-event-suppression-orphan` | recovery source | `8194aa24` | redundant; safe to delete after operator confirmation |
| `recover/project-b-self-hosted-proof-writer` | recovery source | `298aab78` | redundant; safe to delete (a6af5d8f source still on backup) |
| `recover/project-c-d-repository-backfill` | recovery source | `6436d6e8` | redundant; safe to delete |
| `recover/project-e-minio-inventory` | recovery source | `8365e180` | redundant; safe to delete |
| `recover/project-j-workflow-checksum` | recovery source | `98bc84db` | redundant; safe to delete (9348560c source still on backup) |
| `recover/project-k-checksum-backfill-cli` | recovery source | `12cc1b4e` | redundant; safe to delete |
| `recover/project-t-verifier-entrypoint` | recovery source | `b7649e34` | redundant; safe to delete |
| `recover/v1.2.119-hotfix-chain` | recovery source | `14b16a06` | redundant; safe to delete |
| `review/doctor-event-suppression-orphan` | review tip | `1a5bbcde` | redundant; safe to delete (an ancestor of master) |
| `review/project-b-self-hosted-proof-writer` | review tip | `28c35596` | redundant; safe to delete |
| `review/project-c-d-repository-backfill` | review tip | `65fff2e3` | redundant; safe to delete |
| `review/project-e-minio-inventory` | review tip | `9d0c529b` | redundant; safe to delete |
| `review/project-j-workflow-checksum` | review tip | `7cf2195d` | redundant; safe to delete |
| `review/project-k-checksum-backfill-cli` | review tip | `647db0e0` | redundant; safe to delete |
| `review/project-t-verifier-entrypoint` | review tip | `0498b4f4` | redundant; safe to delete |

Per plan's "Do not delete branches unless operator authorizes
cleanup": **no branch deletion performed in this turn.**

---

## Recommended next technical phase

In suggested priority order:

### 1. Recover the Project A awareness chain (highest-priority next phase)

The Project A series (A2 `4dc2fb38`, A3 `a8d9c43a`, A4 `d03ec5f1`,
A5 `f5faae69` + the 6 A5 follow-ups + the cluster_doctor stable-state
orphan `9d1e36e5`) is the missing link between the v1.2.119 awareness
content and the deferred `a6af5d8f` / `9348560c` content. Recovering
it would:

- Unblock the two deferred commits' YAML additions
- Land 17 unmerged awareness-bundle identity commits (~the same shape
  as the 8 source PRs that just landed)
- Reduce the backup ref's outstanding-commits count toward zero

**Risk:** the Project A series is the MEDIUM-risk recovery branch
called out in the original inventory (15-commit chain with internal
release_pipeline.go interplay). Each commit should be cherry-picked
individually, not as a range, to surface any internal-dependency
break promptly.

### 2. After Project A lands, re-attempt the deferred commits

Reorder: small follow-up PRs that re-cherry-pick `a6af5d8f` and
`9348560c` from the backup ref. With the Project A YAML extensions
in place, both should apply cleanly. Each is small (one heartbeat
refresh call + ~150-300 lines of awareness YAML) and worth its own
PR for review clarity.

### 3. Continue with the remaining MEDIUM-risk recovery branches

After the awareness chain is complete:

- `recover/project-f-minio-drift-recovery` (Project F MEDIUM)
- `recover/project-n-wave-blocked-retry` (Project N MEDIUM)
- `recover/project-p-infra-remove-phase` (Project P MEDIUM)
- `recover/project-q-infra-spec-paused` (Project Q MEDIUM)

Each has 1–2 commits with `release_pipeline.go` overlap — apply after
Project A is the safest order.

### 4. Project L systemd sidecar (HIGH risk, last)

`recover/project-l-systemd-sidecar` (1 commit, `services_cmds.go`
conflict with Project O on master). This is the only remaining branch
with a guaranteed conflict — the inventory documented its expected
resolution (keep both Project O's `NormalizeUnitWorkingDirectory`
call and Project L's sidecar write in the install path).

### 5. Branch cleanup

After all recovery is complete and stable, delete the redundant
`recover/*` and `review/*` local branches and the
`backup/local-master-before-reconcile-20260529-144649` ref.

---

## Stop

Final recovery ledger written. No branch deletion performed. No
runtime mutation. Backup ref intact at `b19ce3aa`. Awaiting operator
decision on the next technical phase.
