# PR recover/v1.2.119-hotfix-chain result

**Date:** 2026-05-29
**Outcome:** branch pushed, PR #8 opened, MERGEABLE.

---

## Branch

| Field | Value |
|---|---|
| Local branch | `recover/v1.2.119-hotfix-chain` |
| Local SHA | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| Remote branch | `origin/recover/v1.2.119-hotfix-chain` |
| Remote SHA | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| Pushed | **yes** (local SHA == remote SHA) |

The branch is tracking `origin/recover/v1.2.119-hotfix-chain`.

---

## PR

| Field | Value |
|---|---|
| PR URL | https://github.com/globulario/services/pull/8 |
| PR number | **#8** |
| Title | *Recover v1.2.119 hotfix chain* |
| Base | `master` |
| Head | `recover/v1.2.119-hotfix-chain` |
| `mergeable` | **`MERGEABLE`** |
| `mergeStateStatus` | `UNSTABLE` (same status as PRs #4/5/6/7 — no required CI checks have reported; not a content blocker) |
| `state` | `OPEN` |

GitHub computed a clean three-way merge against `origin/master` —
the 4-commit chain applies without conflict.

---

## Validation (re-run immediately before push)

| Command | Result |
|---|---|
| `go build ./...` | silent (BUILD OK) |
| `go test ./node_agent/node_agent_server -count=1` | **ok 125.287s** |
| `git push -u origin recover/v1.2.119-hotfix-chain` | **clean push** — `[new branch]` created on origin |
| `gh pr create` | PR #8 returned with `mergeable: MERGEABLE` |

### Failures

**None.** All validation green; push and PR creation completed
without warnings other than the conventional `40 uncommitted changes`
notice that `gh pr create` emits for the working tree's untracked
`loads/*.md` evidence files (informational only — PR content comes
from the branch SHA on remote, not the working tree).

---

## Git state (after returning to master)

| Ref | SHA |
|---|---|
| `master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `origin/master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `backup/local-master-before-reconcile-20260529-144649` | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| `recover/v1.2.119-hotfix-chain` (local) | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| `recover/v1.2.119-hotfix-chain` (remote) | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| Other 7 recovery branches (local-only, untouched) | unchanged at their prior SHAs |

`master` is unchanged. `origin/master` is unchanged (the push
created a new feature branch ref, not an update to master). Backup
ref intact.

The other 7 LOW-risk recovery branches remain **local-only**, exactly
as they were before this turn — only `recover/v1.2.119-hotfix-chain`
was pushed.

---

## Runtime safety

**No runtime mutation performed.**

```
$ sudo systemctl show globular-scylla-manager.service -p ActiveState,NRestarts,MainPID
MainPID=770002
NRestarts=0
ActiveState=active
```

`MainPID=770002` matches the value scylla-manager has held since the
U.1 deploy — no restart, no re-spawn, no reload.

- No `pkg build`, no `pkg publish`, no `services desired set`, no
  service restart.
- The packages repo was not touched.
- No PR was merged. The PR is open and awaits human review.
- No force-push, no rebase, no reset, no branch deletion.

---

## Next recommendation

**Wait for PR #8 review and merge.**

Reasons to wait rather than open the next PR in parallel:

1. **PR #8 is foundational.** Its awareness YAML extensions are what
   eventually unblock the two deferred commits (`9348560c` Project J
   closure, `a6af5d8f` Project B awareness). Landing it first
   confirms the path is real before piling more open PRs onto the
   queue.
2. **Review focus.** Opening 8 PRs simultaneously dilutes reviewer
   attention. A serial cadence (this PR merges → operator opens next
   → and so on) lets the reviewer hold one diff in their head at a
   time.
3. **`master` advancement effect.** When PR #8 merges, the other 7
   local recovery branches' merge bases shift (they were cut from
   the pre-#8 `master`). All 7 should still apply cleanly via
   three-way merge — the branches don't touch each other's files
   except for `release_pipeline.go` overlap that will mostly land
   via the future Project A series — but verifying that each
   re-bases or re-applies cleanly is easier when only one PR is in
   flight at a time.

### If the operator authorizes parallel PRs

The safest order for parallel PRs, given the awareness-YAML
dependency picture:

1. `recover/v1.2.119-hotfix-chain` (this PR — #8)
2. `recover/project-c-d-repository-backfill` (single-file linear
   evolution, zero conflict surface against anything else)
3. `recover/project-k-checksum-backfill-cli` (new file under `cmd/`,
   pure additive)
4. `recover/project-e-minio-inventory` (docs-only, smallest surface)
5. `recover/project-t-verifier-entrypoint` (independent node-agent
   path)
6. `recover/doctor-event-suppression-orphan` (cluster_doctor server)
7. `recover/project-b-self-hosted-proof-writer` (5 commits, node-agent
   proof writer — has the skip for `a6af5d8f`)
8. `recover/project-j-workflow-checksum` (2 commits, workflow YAMLs +
   actors — has the skip for `9348560c`)

Each is independent of the others except via the awareness YAML
deferral picture — and that's documented per-branch already.

This document does not authorize either path. The operator's next
turn selects: wait for #8, or open more.

---

## Stop

PR opened and verified. No other recovery branch pushed. No PR
merged. Backup ref preserved at `b19ce3aa`. Recovery branches 2–8
remain local-only at their prior SHAs.
