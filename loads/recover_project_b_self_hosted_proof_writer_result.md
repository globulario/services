# recover/project-b-self-hosted-proof-writer result

**Date:** 2026-05-29
**Outcome:** 5 of 6 cherry-picks clean; 1 skipped (`a6af5d8f`, awareness-YAML dependency). Recovery branch is **local-only** — no push performed.

---

## Base

| Field | Value |
|---|---|
| `master` SHA before branching | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `origin/master` SHA | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| backup ref SHA | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| backup branch name | `backup/local-master-before-reconcile-20260529-144649` |

Previous recovery branches (preserved, untouched):

| Branch | SHA |
|---|---|
| `recover/v1.2.119-hotfix-chain` | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| `recover/doctor-event-suppression-orphan` | `8194aa2410fdbda70be2abfb7c7198b073b047f3` |
| `recover/project-t-verifier-entrypoint` | `b7649e34ff94b633d20140108e29e8a38392b3a2` |
| `recover/project-j-workflow-checksum` | `98bc84db69758a4cfd4873ca21cca9aa32c28b00` |
| `recover/project-c-d-repository-backfill` | `6436d6e8d1f5ab9c163dcdefd873ed03a8eed783` |
| `recover/project-k-checksum-backfill-cli` | `12cc1b4eea09f0d2ab34275d23f7d8e72e911650` |
| `recover/project-e-minio-inventory` | `8365e180e5bd0f753395573839a11fcab1ce5d75` |

---

## Branch

| Field | Value |
|---|---|
| Branch name | `recover/project-b-self-hosted-proof-writer` |
| Base | `master` (`068bf1eb`) |
| Included original commits | 5 (`70f8871d`, `14fbbc50`, `d83e0359`, `93118c05`, `ecdca55c`) |
| Skipped/deferred original commits | 1 (`a6af5d8f`) |
| Final HEAD | `298aab78f5e41ed09f74f2df38b105af8445cc05` |

Commit chain (oldest → newest on the recovery branch):

| Order | Original SHA | New SHA | Subject |
|---:|---|---|---|
| 1 | `70f8871d` | `568d0777` | Project B impact report + matrix: self-hosted installed_state refresh gap |
| 2 | `14fbbc50` | `412e1cc2` | node-agent: implement post-restart self-hosted runtime proof writer (Project B) |
| — | `a6af5d8f` | **SKIPPED** | node-agent: re-assert proof on every heartbeat tick (awareness YAML dependency) |
| 3 | `d83e0359` | `d3e7f18e` | Project B result: self-hosted installed_state refresh validated live |
| 4 | `93118c05` | `dc40ac75` | node-agent: add repository to self-hosted proof writer allowlist |
| 5 | `ecdca55c` | `298aab78` | node-agent: anchor self-hosted proof timestamps to PID start time |

---

## Awareness dependency decision

### `a6af5d8f` — skipped

| Field | Value |
|---|---|
| SHA | `a6af5d8f` |
| Reason | Patch hunks target line positions not present on current master |
| Files that caused deferral | `docs/awareness/failure_modes.yaml` (hunk `@@ -3099,3 +3099,87 @@` — master has only 2578 lines); `docs/awareness/invariants.yaml` (hunk `@@ -4582,3 +4582,67 @@` — master has only 4061 lines) |
| Cherry-pick state observed | Both files showed `UU` markers; `git cherry-pick --skip` cleared the cherry-pick sequence (no `CHERRY_PICK_HEAD` remains, no staged changes from this commit on the branch) |
| Dependency required before retry | `recover/v1.2.119-hotfix-chain` must land on `origin/master` AND the Project A series (A2→A5) must land. Both extend the awareness YAMLs to the line positions `a6af5d8f` was authored against. |

### `d83e0359` — included (pre-check passed)

| Field | Value |
|---|---|
| SHA | `d83e0359` |
| Pre-check result | `git show --name-only d83e0359` returned **1 file**: `loads/self_install_record_refresh_result.md`. Zero awareness YAML files in the commit. |
| Decision | **Safe to cherry-pick** — included as commit #3 on the recovery branch. |

The skipped `a6af5d8f` did not introduce any partial state into the
branch; the next two source commits (`93118c05` allowlist + `ecdca55c`
PID-anchor) modify `self_hosted_runtime_proof_writer.go` and its test
file — files created by `14fbbc50`. Both applied cleanly on top of the
branch with `a6af5d8f` absent. This confirms that the awareness-only
content of `a6af5d8f` (the failure_mode + invariant entries) is
documentation drift, not a code dependency for the later source
commits — exactly the property that makes "park the scrolls" safe.

---

## Files changed vs `master`

6 files. 3 source + 3 evidence:

```
M  golang/node_agent/node_agent_server/heartbeat.go                          (from 14fbbc50 — wire the writer; refresh on heartbeat is NOT included here, awaits a6af5d8f re-cherry-pick)
A  golang/node_agent/node_agent_server/self_hosted_runtime_proof_writer.go      (new, from 14fbbc50; later modified by 93118c05 + ecdca55c)
A  golang/node_agent/node_agent_server/self_hosted_runtime_proof_writer_test.go (new, from 14fbbc50; later modified by 93118c05 + ecdca55c)
A  loads/self_install_record_refresh_impact.md                              (from 70f8871d)
A  loads/self_install_record_refresh_matrix.tsv                             (from 70f8871d)
A  loads/self_install_record_refresh_result.md                              (from d83e0359)
```

**No awareness YAML modifications on this branch** — exactly the
intent of the skip-pattern.

### What the skip leaves behind

The functional effect of `a6af5d8f` is:
- adds a call to `refreshSelfHostedInstalledState` from `runHeartbeat`
  (every 30s) in `heartbeat.go`
- adds 87 lines to `failure_modes.yaml` documenting
  `self_hosted_component.installed_state_writer_blocked_after_runtime_proof`
- adds 67 lines to `invariants.yaml` documenting
  `self_hosted_runtime_proof_may_refresh_installed_state`

Without `a6af5d8f`, the proof writer is only invoked from
`syncInstalledStateToEtcd` (startup + 5-minute ticker). That is the
state Project B `14fbbc50` originally shipped; the heartbeat refresh
is an enhancement that will land later. **No regression** — the
recovered Project B is in its first-shipped form, which was the
state running on the cluster at validation time per the `d83e0359`
result document.

---

## Validation

### Commands run

| Command | Result |
|---|---|
| `git cherry-pick 70f8871d` | clean |
| `git cherry-pick 14fbbc50` | clean |
| `git cherry-pick a6af5d8f` | **CONFLICT** (`UU` in both awareness YAMLs, as predicted) |
| `git cherry-pick --skip` | clean (no `CHERRY_PICK_HEAD` remains) |
| `git show --name-status d83e0359` | confirmed 1 file (`loads/self_install_record_refresh_result.md`, zero awareness YAMLs) |
| `git cherry-pick d83e0359` | clean |
| `git cherry-pick 93118c05` | clean |
| `git cherry-pick ecdca55c` | clean |
| `go test ./node_agent/node_agent_server -count=1` | **ok 120.931s** |
| `go test ./repository/repository_server -count=1` | ok 1.844s (sanity, not affected by this branch) |
| `go test ./cluster_controller/cluster_controller_server -count=1` | ok 131.830s (sanity, not affected by this branch) |
| `go build ./...` | silent (BUILD OK) |

### Validation adjustment from the instruction

The instruction's validation list named
`./repository/repository_server ./cluster_controller/cluster_controller_server`
as the fallback. The actual touched package on this branch is
**`./node_agent/node_agent_server`** — the proof writer files and
`heartbeat.go` modification all live there. The inventory's
recommended check was `go test ./node_agent/node_agent_server`,
which is what was run as the primary validation. The instruction's
two named packages were run as a sanity check and also pass.

### Failures

**None.** All 5 included cherry-picks applied without conflict;
`a6af5d8f` was skipped per the documented plan; the `node_agent`
package tests pass (this is the package whose source actually
changed); the tree builds.

---

## Git state

### `git status --short` on the recovery branch (post-cherry-picks)

```
(no tracked-file modifications)
```

Cherry-pick in progress: **no**. The skip cleared the sequence; the
last 3 cherry-picks completed normally.

### SHAs (after returning to master)

| Ref | SHA |
|---|---|
| `master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `origin/master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `backup/local-master-before-reconcile-20260529-144649` | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| `recover/v1.2.119-hotfix-chain` | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| `recover/doctor-event-suppression-orphan` | `8194aa2410fdbda70be2abfb7c7198b073b047f3` |
| `recover/project-t-verifier-entrypoint` | `b7649e34ff94b633d20140108e29e8a38392b3a2` |
| `recover/project-j-workflow-checksum` | `98bc84db69758a4cfd4873ca21cca9aa32c28b00` |
| `recover/project-c-d-repository-backfill` | `6436d6e8d1f5ab9c163dcdefd873ed03a8eed783` |
| `recover/project-k-checksum-backfill-cli` | `12cc1b4eea09f0d2ab34275d23f7d8e72e911650` |
| `recover/project-e-minio-inventory` | `8365e180e5bd0f753395573839a11fcab1ce5d75` |
| `recover/project-b-self-hosted-proof-writer` | `298aab78f5e41ed09f74f2df38b105af8445cc05` |

After validation, working copy was returned to `master`. The new
recovery branch sits local-only, untouched by any further git
operation.

---

## Runtime safety

**No runtime mutation performed.**

```
$ sudo systemctl is-active globular-scylla-manager.service
active

$ sudo systemctl show globular-scylla-manager.service -p NRestarts -p MainPID --no-pager
NRestarts=0
MainPID=770002
```

`MainPID=770002` matches the value scylla-manager has held since the
U.1 deploy — no restart, no re-spawn, no reload.

- No `pkg build`, no `pkg publish`, no `services desired set`, no
  service restart.
- No push to either remote. `origin/master`'s ref state is unchanged.
- The packages repo was not touched.
- The backup ref is intact at `b19ce3aa`.

---

## Deferred notes

Two awareness-YAML-dependent commits are now in the deferred pile:

| SHA | Project | Awareness-YAML lines added | Re-cherry-pick after |
|---|---|---|---|
| `9348560c` | J closure | ~666 lines to failure_modes, ~640 to invariants | v1.2.119 chain + A series land |
| `a6af5d8f` | B awareness | 87 lines to failure_modes, 67 to invariants | v1.2.119 chain + A series land |

Both will re-cherry-pick cleanly once:
1. `recover/v1.2.119-hotfix-chain` lands on `origin/master` (extends both YAMLs)
2. Project A series (A2 → A5) lands (further extends both YAMLs)

The original commit content for both deferred items is preserved on
the backup ref (`b19ce3aa`) at their original SHAs; re-cherry-pick
will produce new local SHAs.

**Important nuance:** the heartbeat-refresh behavior that `a6af5d8f`
adds to the proof writer is NOT on this branch. The proof writer
shipped by `14fbbc50` runs on startup + every 5 minutes. The
30-second heartbeat refresh is what gets restored when `a6af5d8f` is
re-cherry-picked. This is a functional regression from the
backup-tip behavior, but it matches the originally-shipped behavior
of Project B's first commit and is fully captured in
`d83e0359`'s result documentation (which is on this branch).

---

## Next recommendation

The LOW-risk recovery pool is now complete. **All 8 LOW branches**
are local-only and validated:

1. `recover/v1.2.119-hotfix-chain` (4 commits)
2. `recover/doctor-event-suppression-orphan` (1)
3. `recover/project-t-verifier-entrypoint` (1)
4. `recover/project-j-workflow-checksum` (2, 1 deferred)
5. `recover/project-c-d-repository-backfill` (6)
6. `recover/project-k-checksum-backfill-cli` (4)
7. `recover/project-e-minio-inventory` (2)
8. `recover/project-b-self-hosted-proof-writer` (5, 1 deferred)

**Recommended next step: shift from local-recovery to PR-and-merge
for the LOW set.**

The natural cleanest order to merge the LOW set is:

1. **`recover/v1.2.119-hotfix-chain`** first — it's the foundational
   hotfix chain and its 4 commits all extend the awareness YAMLs.
   Landing this unblocks part of the deferred-commit re-cherry-pick.
2. Then the rest of the LOW set in any order they suit operator
   review pacing (they're mutually independent).

After all 8 LOW branches merge to `origin/master`:
- Re-cherry-pick `9348560c` (Project J closure) — should apply if
  the awareness chain catches up enough.
- Re-cherry-pick `a6af5d8f` (Project B awareness) — same condition.
- If `a6af5d8f` still conflicts (because Project A series hasn't
  landed yet), authorize the Project A series recovery as the next
  MEDIUM branch. Project A's 15 commits include the largest awareness
  YAML expansions in the deferred pile; landing them clears the
  bottleneck for both deferred items.

Alternative recommendation: if the operator prefers to keep
all-recovery momentum and defer PR/review to later, the next
MEDIUM-risk recovery branch is
**`recover/project-a-awareness-bundle-identity`** (15 commits, the
biggest remaining pile). Validation: `go build ./...` plus the test
packages listed in §4c of `loads/local_services_master_reconciliation_plan.md`.
Expected conflicts: none against `master`'s current files, but the
commits internally depend on each other and must be applied as a
unit in order.

This document does not authorize either path. The operator's
next-turn instruction selects.

---

## Stop

Recovery and validation complete for
`recover/project-b-self-hosted-proof-writer`. PR not opened (per
instruction). Branch sits on the local checkout at `298aab78`;
backup ref preserved at `b19ce3aa`. The LOW-risk recovery pool is
fully drained.
