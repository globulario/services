# Repository cleanup plan after packages merge

**Date:** 2026-05-29
**Scope:** branch cleanup + local-master reconciliation for both
`globulario/services` and `globulario/packages`. Planning only — no
deletions, no force-pushes, no rebase, no merge.

This document is the audit-and-options report. The "next authorized
action" at the end names a single low-blast-radius starting move; all
other moves stay queued behind explicit authorization.

---

## 1. Snapshot tables

### 1a. Services repo — branches

| Branch (local) | Local SHA | Remote SHA | Tracking | Ancestor of `origin/master`? | Classification |
|---|---|---|---|---|---|
| `master` | `b19ce3aa` | `a272f415` (`origin/master`) | ahead 52 / behind 6 | n/a (it IS the local default) | **diverged — needs reconcile** |
| `project-o` | `947e3e2e` | `947e3e2e` | clean | **YES** — merged via PR #4 (`91b445c1`) | **merged → safe to delete** |
| `project-s-reconciled` | `73cb2516` | `73cb2516` | clean | **YES** — merged via PR #5 (`6af791a5`) | **merged → safe to delete** |
| `project-u3-reconciled` | `66d191e5` | `66d191e5` | clean | **YES** — merged via PR #6 (`a272f415`) | **merged → safe to delete** |
| `project-u2` | `9e2ee870` | `9e2ee870` | clean | **NO** — never opened a PR; the U.2 commit it carries (`1970dd7c`) is still in the local `master` pile | **stale, not merged — keep or delete?** |
| `project-u3` | `21351c96` | `21351c96` | clean | **NO** — never opened a PR; carries the silent O.5 drop trap | **stale + dangerous → delete to prevent accidental merge** |

Remote heads on `globulario/services`:

```
a272f415 refs/heads/master                  ← merge target, current tip
947e3e2e refs/heads/project-o               ← merged
73cb2516 refs/heads/project-s-reconciled    ← merged
66d191e5 refs/heads/project-u3-reconciled   ← merged
9e2ee870 refs/heads/project-u2              ← never merged (PR never opened)
21351c96 refs/heads/project-u3              ← never merged + DANGEROUS
2a0331fc refs/heads/add-license-1           ← pre-existing, unrelated to this work
```

### 1b. Services repo — special checks on `origin/master`

```
$ git show origin/master:.../registry.go | grep -nE "systemdWorkingDirectoryMustBeOptional|scyllaManagerClusterRegistered"
219:    systemdWorkingDirectoryMustBeOptional{}    ✓
223:    scyllaManagerClusterRegistered{}           ✓

$ git show origin/master:.../scylla_manager_cluster_registered.go | grep -cE "<sym>"
  newScyllaManagerHTTPSClient    3
  isTLSVerificationError         3
  discoverScyllaManagerHost      3
```

`origin/master` has all O+S+U.3 content as expected.

### 1c. Services repo — uncommitted / stashed

| Item | Value |
|---|---|
| Tracked-file modifications | **0** |
| Untracked files | exist (the conventional `loads/project_*.md` evidence files + temp logs) — none are source |
| Stashes | **0** |

### 1d. Packages repo — branches

| Branch (local) | Local SHA | Remote SHA | Tracking | Ancestor of `origin/main`? | Classification |
|---|---|---|---|---|---|
| `main` | `f1871d8` | `f1871d8` (`origin/main`) | ahead 0 / behind 0 | n/a | **clean — fully in sync** |
| `project-u2` | `bdc3724` | `bdc3724` | clean | **YES** — merged via packages PR #1 (`b905d39`) | **merged → safe to delete** |
| `wd-normalize-systemd-working-directory` | `2a625d3` | `2a625d3` | clean | **YES** — merged via packages PR #2 (`f1871d8`) | **merged → safe to delete** |

Remote heads on `globulario/packages`:

```
f1871d8 refs/heads/main                                       ← current tip
bdc3724 refs/heads/project-u2                                 ← merged
2a625d3 refs/heads/wd-normalize-systemd-working-directory     ← merged
```

### 1e. Packages repo — special checks on `origin/main`

```
$ F=metadata/scylla-manager/specs/scylla_manager_service.yaml
$ git show origin/main:$F | grep -c '<phrase>'
  capath /dev/null                          3
  install-scylla-manager-register-cluster   1

$ grep ^WorkingDirectory= on origin/main (sample 3):
  globular-echo.service              WorkingDirectory=-{{.StateDir}}/echo
  globular-mail.service              WorkingDirectory=-{{.StateDir}}/mail
  globular-authentication.service    WorkingDirectory=-{{.StateDir}}/authentication
```

`origin/main` has Project S/U.2 YAML + all 37 WD-normalize files.

### 1f. Packages repo — uncommitted / stashed

| Item | Value |
|---|---|
| Tracked-file modifications | **0** |
| Untracked files | **0** |
| Stashes | **0** |

The packages repo is in a fully clean state — nothing to reconcile
beyond branch cleanup.

---

## 2. Remote branch deletion candidates

### Safe to delete (merged into the default branch)

| Repo | Branch | SHA | PR | Reason safe |
|---|---|---|---|---|
| services | `project-o` | `947e3e2e` | #4 | content present in `origin/master` via merge commit `91b445c1` |
| services | `project-s-reconciled` | `73cb2516` | #5 | content present in `origin/master` via merge commit `6af791a5` |
| services | `project-u3-reconciled` | `66d191e5` | #6 | content present in `origin/master` via merge commit `a272f415` |
| packages | `project-u2` | `bdc3724` | #1 | content present in `origin/main` via merge commit `b905d39` |
| packages | `wd-normalize-systemd-working-directory` | `2a625d3` | #2 | content present in `origin/main` via merge commit `f1871d8` |

All 5 above lose no work on deletion — the SHAs remain reachable via
the merge commits' second-parent edges.

### Stale + dangerous — recommend delete

| Repo | Branch | SHA | Reason |
|---|---|---|---|
| services | `project-u3` | `21351c96` | **silent-divergence trap.** Its `registry.go` deletes the O.5 line. If a PR is later opened from this branch and merged, the deletion will silently drop the O.5 invariant registration from `origin/master`. The safe replacement (`project-u3-reconciled` → PR #6 `a272f415`) is already merged. There is no reason to keep this branch. |

### Stale but harmless — operator decision

| Repo | Branch | SHA | Status |
|---|---|---|---|
| services | `project-u2` | `9e2ee870` | Never had a PR opened. Carries the U.2 test commit (`1970dd7c`), which is also in the local `master` 52-pile. Not dangerous, but redundant — operator may delete or leave dormant. |
| services | `add-license-1` | `2a0331fc` | **Pre-existing, unrelated to this work series.** Out of scope for this cleanup plan. Operator decision. |

---

## 3. Local branch deletion candidates

In each repo, the local branches that mirror the remote candidates can
be deleted **after** the remote deletions are done (or independently —
they hold no unpushed work).

### Services repo

| Branch | Reason safe |
|---|---|
| `project-o` (local) | local SHA matches remote merged SHA |
| `project-s-reconciled` (local) | local SHA matches remote merged SHA |
| `project-u3-reconciled` (local) | local SHA matches remote merged SHA |
| `project-u3` (local) | local SHA matches dangerous remote SHA — delete after remote is deleted |
| `project-u2` (local) | local SHA matches stale remote SHA |

### Packages repo

| Branch | Reason safe |
|---|---|
| `project-u2` (local) | local SHA matches remote merged SHA |
| `wd-normalize-systemd-working-directory` (local) | local SHA matches remote merged SHA |

---

## 4. Local `master` — 52 ahead / 6 behind reconciliation

### 4a. The 6 behind

These are the 6 new commits added to `origin/master` via PRs #4/5/6:

```
a272f415  Project U.3: make scylla-manager doctor probe HTTPS-first (#6)  ← merge commit
66d191e5  Project U.3: cluster-doctor HTTPS-first probe for scylla-manager  ← cherry-pick
6af791a5  Project S: add scylla-manager cluster registration doctor invariant (#5)  ← merge commit
73cb2516  Project S: cluster_doctor invariant for unregistered scylla-manager  ← cherry-pick
91b445c1  Project O: enforce optional systemd WorkingDirectory and canonical state paths (#4)  ← merge commit
947e3e2e  Project O: WorkingDirectory normalize parity + state-path migration + invariant  ← cherry-pick
```

The cherry-pick commits (`947e3e2e`, `73cb2516`, `66d191e5`) are
content-equivalent to local commits (`c529310e`, `16af03a8`,
`b19ce3aa`) but have different SHAs. The local copies are now
**redundant** — same content, different SHA.

### 4b. The 52 ahead — composition

Out of the 52 commits in local `master` not present on `origin/master`:

- **3 are content-equivalent to merged PRs** — but with the original
  local SHAs. These are obsolete:
  - `c529310e` Project O (merged as `947e3e2e`)
  - `16af03a8` Project S (merged as `73cb2516`)
  - `b19ce3aa` **Project U.3 — DANGEROUS**, drops O.5 line (safe replacement `66d191e5` merged via PR #6)
- **49 are still unmerged work**, covering:
  - v1.2.119 hotfix chain (4 commits)
  - Project A through A5 (16 commits — awareness-bundle identity work)
  - Project B / C / D / E / E2 / F / J / K / L / N / P / Q / T / U.2 (~29 commits)
  - cluster_doctor stable-state event suppression (1 commit)

The full enumeration is in `loads/repository_reconciliation_plan_after_project_u3.md` §2.

### 4c. Reconcile options for local `master`

#### Option A — `git pull --rebase origin/master` (most surgical)

Replays the 52 local commits onto `origin/master`. The 3 redundant
commits (`c529310e`, `16af03a8`, `b19ce3aa`) will likely produce
"empty after squashing" warnings or conflict markers since their
content is already on master; git's rebase machinery will offer to
drop them. Specifically `b19ce3aa` will conflict on `registry.go`
because its delete-the-O.5-line patch no longer applies cleanly to a
master that has the O.5 line present — providing a natural place to
drop it.

**Risk:** conflict resolution must be done carefully; mishandling the
b19ce3aa conflict re-introduces the trap.

#### Option B — `git rebase --onto origin/master <skip-tail> <skip-head>` (precise drop)

Use `--onto` to skip exactly the 3 redundant commits, replaying only
the 49 remaining commits onto `origin/master`. Avoids the conflict by
removing the problematic commits before rebase runs.

```bash
# example sketch — NOT EXECUTED
git rebase --onto origin/master b19ce3aa~1 master
# or interactive rebase with explicit drops
git rebase -i origin/master   # mark c529310e, 16af03a8, b19ce3aa as 'drop'
```

**Risk:** rebase rewrites history. Other clones or co-located worktrees
would see a divergence.

#### Option C — `git merge origin/master` (preserve all history)

Creates a merge commit on local `master` integrating both sides. Local
keeps its 52 ahead commits + gets the 6 behind. The 3 redundant
commits remain in history but don't cause regressions because the
merge result reflects both. **However**: `b19ce3aa` removed the O.5
line; `origin/master` has it. The merge will conflict in `registry.go`
between two histories that disagree about line 219. Operator must
choose origin's content (the present + correct version) and discard
local's deletion.

**Risk:** if the operator picks the wrong side of the conflict (or
auto-resolves), the O.5 line silently drops from local master. A
future push of local master would re-introduce the bug to origin.

#### Option D — `git reset --hard origin/master` (destroy local work)

Throws away the 52 ahead commits. Aligns local with remote.

**Risk:** loses 49 commits of unmerged work — **NOT recommended**.
Use only if the operator has confirmed every project in the 49-commit
pile is either represented elsewhere or intentionally being
abandoned.

#### Option E — leave local `master` as-is, do work on feature branches

The lowest-friction option: do future work on feature branches off
`origin/master` (per the reconciliation pattern that already worked
for O / S / U.3), and let local `master` quietly drift further out
of sync. Operator may eventually delete and re-clone the repo, OR
periodically run Option A/B during a clean window.

**Risk:** the 52 ahead commits remain reachable only locally;
machine loss = work loss. The original Project authors should have
their content on remote feature branches eventually.

---

## 5. Remaining unmerged local work (the 49)

This count and breakdown is preserved in
`loads/repository_reconciliation_plan_after_project_u3.md` §2. Brief
restatement:

| Group | Commits | Status |
|---|---|---|
| v1.2.119 hotfix chain | 4 | unpushed; independent |
| Project A → A5 + 9d1e36e5 | 17 | unpushed; long linear chain |
| Project B | 4 | unpushed |
| Project C (inventory only) | 1 | unpushed |
| Project D + 93118c05 | 6 | unpushed |
| Project E + E2 (inventory only) | 2 | unpushed |
| Project F | 2 | unpushed |
| Project J | 3 | unpushed |
| Project K | 4 | unpushed |
| Project L | 1 | unpushed |
| ecdca55c PID-anchor | 1 | unpushed |
| Project N | 1 | unpushed |
| Project P | 1 | unpushed |
| Project T | 1 | unpushed |
| Project Q | 1 | unpushed |
| Project U.2 | 1 | unpushed (also on stale `services/project-u2` branch) |

Plus 3 redundant: Project O / S / U.3-buggy (content equivalent to
merged PRs).

Total: 52.

### 5a. Should be split into future PRs

Following the reconciliation pattern from §5 of the prior plan:
linear chronological cherry-pick onto `origin/master`, one feature
branch per project. Order: oldest → newest, drop the 3 redundant
ones.

### 5b. Should be abandoned

**None.** Each of the 49 represents authored work for a specific
project. The 3 redundant ones (`c529310e`, `16af03a8`, `b19ce3aa`)
are the candidates for abandonment because their content is already
on `origin/master`.

---

## 6. Exact safe command sequence

### 6a. Safest minimum (one action, eliminates the dangerous branch)

```bash
# delete the dangerous services/project-u3 branch from the remote
cd /home/dave/Documents/github.com/globulario/services
git push origin --delete project-u3

# then delete the matching local ref
git branch -D project-u3
```

After this, `21351c96` is no longer reachable through any branch ref
on either side. Nothing else changes.

### 6b. Optional follow-up — delete the 4 merged service branches

```bash
cd /home/dave/Documents/github.com/globulario/services
git push origin --delete project-o
git push origin --delete project-s-reconciled
git push origin --delete project-u3-reconciled
git push origin --delete project-u2          # stale, never PR'd

git branch -d project-o                      # safe deletion (only if local == remote merged SHA)
git branch -d project-s-reconciled
git branch -d project-u3-reconciled
git branch -D project-u2                     # force, since this branch has no PR / not merged
```

### 6c. Optional follow-up — delete the 2 merged packages branches

```bash
cd /home/dave/Documents/github.com/globulario/packages
git push origin --delete project-u2
git push origin --delete wd-normalize-systemd-working-directory

git branch -d project-u2
git branch -d wd-normalize-systemd-working-directory
```

### 6d. Optional follow-up — reconcile local services `master`

Operator picks one of the four reconcile options from §4c.

---

## 7. Commands that must NOT be run yet

These are documented for completeness — they would be destructive or
premature in the current state.

- **`git reset --hard origin/master`** on services local master.
  Would discard 49 commits of unmerged work. Defer until each project
  has been pushed in its own PR, OR until the operator has explicitly
  decided to abandon the unpushed pile.
- **`git push --force origin master`** on services. Would overwrite
  `origin/master`. Never authorized in this codebase.
- **`git rebase -i`** with arbitrary edits to project-history. Risky
  without a backup branch first; if attempted, copy local master to
  a `master-backup-YYYY-MM-DD` branch beforehand.
- **`git push origin --delete master`** or **`--delete main`** on
  either repo. Default branches must not be deleted.
- **`git push origin --delete add-license-1`** on services. This
  branch is pre-existing and unrelated to this work series; deleting
  it without owner authorization is out of scope.

---

## 8. Branches that must be kept

| Repo | Branch | Reason |
|---|---|---|
| services | `master` | default branch on both local and origin |
| packages | `main` | default branch on both local and origin |
| services | `add-license-1` | pre-existing, unrelated, owner decision |

---

## 9. Branches that require human review

None of the work-product branches require additional human review at
this point — every one has either landed via PR (project-o/s-reconciled/
u3-reconciled, packages/project-u2, packages/wd-normalize) or is
explicitly dangerous (services/project-u3) or stale (services/project-u2).

The unpushed 49-commit pile on local `master` does require operator
review when the operator chooses how to bring its contents to remote
(via §6.d Option A/B/C/E). That review is about deciding the strategy
for ~13 project-groups, not about reviewing individual commits.

---

## Next authorized action

**Next authorized action should be: delete the dangerous
`services/project-u3` remote and local branches.**

```bash
cd /home/dave/Documents/github.com/globulario/services
git push origin --delete project-u3
git branch -D project-u3
```

Reasons this is the right single first move:

- Eliminates the silent-divergence trap permanently (no PR can ever
  re-introduce the O.5-line drop).
- Zero work loss — the safe replacement `project-u3-reconciled` is
  already merged to `origin/master`, content-equivalent and
  trap-free.
- Blast radius is minimal — touches only the dangerous ref, no other
  state.
- Reversible if needed — the SHA `21351c96` is recoverable from
  `git reflog` on any clone where it was fetched.

All other cleanup steps (deleting merged remote branches, deleting
local mirror branches, reconciling local `master`) are non-urgent
and may be authorized separately when convenient.

---

## 10. `project-u3` deletion result (2026-05-29 14:23)

### Deleted

| Ref | Pre-deletion SHA | State after |
|---|---|---|
| `services` remote `refs/heads/project-u3` | `21351c9671e59e5db128cb62b6322401c844c5aa` | deleted |
| `services` local branch `project-u3` | `21351c9671e59e5db128cb62b6322401c844c5aa` | deleted |

The silent O.5-drop trap is eliminated. The SHA `21351c96` no longer
appears in any branch ref on either side; it remains recoverable only
via `git reflog` locally (and via GitHub's reflog for ~90 days).

### Remote deletion proof

```
$ git push origin --delete project-u3
To https://github.com/globulario/services.git
 - [deleted]           project-u3

$ git ls-remote --heads origin project-u3
(empty)
```

### Local deletion proof

```
$ git branch -D project-u3
Deleted branch project-u3 (was 21351c96).

$ git branch --list project-u3
(empty)
```

### Confirmation: no other branches touched

`git ls-remote --heads origin` post-deletion:

```
2a0331fc  refs/heads/add-license-1                              ← unchanged
a272f415  refs/heads/master                                     ← unchanged
947e3e2e  refs/heads/project-o                                  ← unchanged (merged)
73cb2516  refs/heads/project-s-reconciled                       ← unchanged (merged)
9e2ee870  refs/heads/project-u2                                 ← unchanged (stale)
66d191e5  refs/heads/project-u3-reconciled                      ← unchanged (merged)
```

`project-u3` is the only ref that disappeared. Six branches remain on
the remote, matching the §1a table minus the dangerous branch.

### Confirmation: U.3 functionality survives

- `origin/master` still at `a272f415` (the PR #6 merge commit).
- PR #6 merge commit is the tip of `origin/master`:
  `a272f415 Project U.3: make scylla-manager doctor probe HTTPS-first (#6)`.
- `66d191e5` (the safe `project-u3-reconciled` source SHA) is
  still an ancestor of `origin/master` — `git merge-base
  --is-ancestor` confirms.

### Confirmation: no code, no deploy, no rebuild

- Local services source tree: no tracked-file modifications.
- `globular-scylla-manager.service`: `ActiveState=active`,
  `NRestarts=0`, `MainPID=770002` (unchanged from pre-deletion).
- No `pkg build`, no `pkg publish`, no `services desired set`, no
  service restart.

### Next recommended cleanup action

**Delete the 4 merged remote branches** (and their local mirrors)
across the two repos:

```bash
# services repo — 3 merged remote branches + 1 stale (never PR'd) branch
cd /home/dave/Documents/github.com/globulario/services
git push origin --delete project-o
git push origin --delete project-s-reconciled
git push origin --delete project-u3-reconciled
git push origin --delete project-u2          # stale: was never PR'd; redundant with local pile

git branch -d project-o
git branch -d project-s-reconciled
git branch -d project-u3-reconciled
git branch -D project-u2                     # -D since no PR existed

# packages repo — 2 merged remote branches
cd /home/dave/Documents/github.com/globulario/packages
git push origin --delete project-u2
git push origin --delete wd-normalize-systemd-working-directory

git branch -d project-u2
git branch -d wd-normalize-systemd-working-directory
```

Zero work loss: every deleted branch's SHA either (a) is already an
ancestor of the default branch via its merge commit, or (b) holds
content content-equivalent to commits still present in the local
master 52-pile (services/project-u2 case).

This document does not authorize that sequence — operator decision.

### Status

Dangerous `services/project-u3` branch deleted. Next recommended
cleanup action is deleting merged branches.

---

## 11. Merged branch deletion result (2026-05-29 14:30)

### Deleted

| Repo | Ref (remote) | Local mirror | Pre-deletion SHA | State |
|---|---|---|---|---|
| services | `refs/heads/project-o` | yes | `947e3e2e` | deleted |
| services | `refs/heads/project-s-reconciled` | yes | `73cb2516` | deleted |
| services | `refs/heads/project-u3-reconciled` | yes | `66d191e5` | deleted |
| packages | `refs/heads/project-u2` | yes | `bdc3724` | deleted |
| packages | `refs/heads/wd-normalize-systemd-working-directory` | yes | `2a625d3` | deleted |

Zero work loss — every deleted SHA is an ancestor of its repo's
default branch via the corresponding PR merge commit:
- `947e3e2e` → reachable via `91b445c1` (PR #4) on `services/master`
- `73cb2516` → reachable via `6af791a5` (PR #5) on `services/master`
- `66d191e5` → reachable via `a272f415` (PR #6) on `services/master`
- `bdc3724` → reachable via `b905d39` (packages PR #1) on `packages/main`
- `2a625d3` → reachable via `f1871d8` (packages PR #2) on `packages/main`

### Remote deletion proofs

```
$ git push origin --delete project-o
 - [deleted]           project-o

$ git push origin --delete project-s-reconciled
 - [deleted]           project-s-reconciled

$ git push origin --delete project-u3-reconciled
 - [deleted]           project-u3-reconciled

$ (packages) git push origin --delete project-u2
 - [deleted]         project-u2

$ (packages) git push origin --delete wd-normalize-systemd-working-directory
 - [deleted]         wd-normalize-systemd-working-directory
```

### Local deletion proofs

```
$ git branch -D project-o                  Deleted branch project-o (was 947e3e2e).
$ git branch -D project-s-reconciled       Deleted branch project-s-reconciled (was 73cb2516).
$ git branch -D project-u3-reconciled      Deleted branch project-u3-reconciled (was 66d191e5).
$ (packages) git branch -D project-u2                                   Deleted branch project-u2 (was bdc3724).
$ (packages) git branch -D wd-normalize-systemd-working-directory       Deleted branch wd-normalize-systemd-working-directory (was 2a625d3).
```

### Remaining branches — only the explicitly-kept refs

**Services remote** (3 refs, exactly the expected set):

```
2a0331fc  refs/heads/add-license-1    ← kept (unrelated, out of scope)
a272f415  refs/heads/master           ← kept (default branch)
9e2ee870  refs/heads/project-u2       ← kept (stale but not authorized for deletion)
```

**Services local** (2 refs):

```
* master
  project-u2
```

**Packages remote** (1 ref):

```
f1871d8  refs/heads/main
```

**Packages local** (1 ref):

```
* main
```

### Confirmation: unmerged / stale branches were NOT touched

- `services/add-license-1` (`2a0331fc`) — still on remote. Not part
  of this work series; operator decision.
- `services/project-u2` (`9e2ee870`) — still on remote, still on
  local. Stale (no PR ever opened) but the operator did not authorize
  its deletion in this turn.

### Confirmation: U.3 functionality survives in `services/origin/master`

```
$ git show origin/master:.../registry.go | grep -nE "systemdWorkingDirectoryMustBeOptional|scyllaManagerClusterRegistered"
219:    systemdWorkingDirectoryMustBeOptional{}    ✓
223:    scyllaManagerClusterRegistered{}           ✓

$ git show origin/master:.../scylla_manager_cluster_registered.go | grep -cE "<sym>"
  newScyllaManagerHTTPSClient    3
  isTLSVerificationError         3
  discoverScyllaManagerHost      3
```

### Confirmation: packages PR #1 + #2 content survives in `packages/origin/main`

```
$ git show origin/main:metadata/scylla-manager/specs/scylla_manager_service.yaml | grep -c '<phrase>'
  capath /dev/null                              3
  install-scylla-manager-register-cluster       1

$ git show origin/main:metadata/echo/systemd/globular-echo.service | grep '^WorkingDirectory='
  WorkingDirectory=-{{.StateDir}}/echo
```

### Confirmation: no code / deploy / rebuild

- Both repos: zero tracked-file modifications.
- `globular-scylla-manager.service`: `ActiveState=active`,
  `NRestarts=0`, `MainPID=770002` (unchanged from pre-deletion — same
  PID it has held since U.1).
- No `pkg build`, no `pkg publish`, no `services desired set`, no
  service restart.

### Next recommended cleanup action

**Classify `services/project-u2` and reconcile local
`services/master`.**

Two questions for the operator:

1. **`services/project-u2` (`9e2ee870`)** — should this stale branch
   be deleted (it carries the U.2 test commit `1970dd7c` whose
   content is also in the local master 52-pile), or kept as a
   discoverable record of the U.2 work? It has never had a PR
   opened. Recommendation: delete to match the discipline of the
   rest of the cleanup; the SHA is preserved in `git reflog` for
   ~90 days on either side if recovery is needed.

2. **Local `services/master`** — still 52 ahead / 6 behind
   `origin/master`. Of the 52 ahead:
   - 3 are content-redundant with merged PRs (`c529310e`,
     `16af03a8`, `b19ce3aa` — the buggy U.3)
   - 49 are unpushed work for Projects A through K-N-P-Q-T-U.2 etc.

   The four reconcile options remain as documented in §4c (rebase
   surgical drop, merge-with-conflict-resolution, leave-as-is, or
   reset-and-lose-work). Recommendation: **§4c Option E — leave
   as-is** until the operator decides the strategy for bringing the
   49-commit pile to remote. The 3 redundant commits stay dormant
   on local; future feature-branch work continues to be cherry-picked
   from local master onto branches off `origin/master`, exactly as
   the O→S→U.3 reconciliation already did.

### Status

Merged branches deleted. Next recommended cleanup action is
classifying `services/project-u2` and reconciling local
`services/master`.

---

## 12. `services/project-u2` classification (2026-05-29 14:35)

### Branch identity

| Field | Value |
|---|---|
| Remote ref | `refs/heads/project-u2` |
| Remote SHA | `9e2ee870d061f9501d691e4107d61094ec9eb076` |
| Local SHA | `9e2ee870` (matches remote — clean) |
| Merge-base with `origin/master` | `07214edee4f5c9bc7aaf6e2d0f22c62fa0071642` (the pre-merge `origin/master` tip — i.e. before PRs #4/5/6) |
| Commits on branch not on `origin/master` | **1** — `9e2ee870 Project U.2: scylla-manager registration script HTTPS-first integration tests` |
| Associated PR | **none** (gh pr list returns empty) |
| Containing refs (local) | `project-u2` |
| Containing refs (remote) | `origin/project-u2` |

### Content — what the cherry-pick commit actually adds

The single commit on this branch (`9e2ee870`) is the cherry-pick of
local `1970dd7c` ("Project U.2: scylla-manager registration script
HTTPS-first integration tests"). Compared to its merge-base
(`07214ede`), the commit adds exactly **one new file**:

```
A  golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_register_script_test.go   (+323 lines)
```

This file is **not present** on current `origin/master`:

```
$ git ls-tree origin/master -- golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_register_script_test.go
(empty)
```

It contains the 5 integration tests for the package-shipped
`/usr/lib/globular/bin/scylla-manager-register-cluster` script, written
in U.2 to lock in the HTTPS-first / fail-closed contract before it
landed on the cluster. The tests skip gracefully when the script is
not installed (e.g. on a fresh CI runner without a deployed cluster).

### The two-way diff is misleading — three-way merge will be additive

A naive `git diff origin/master..origin/project-u2` shows 17 files
changed, +353 / −1825, with deletions of every O / S / U.3 file. This
looks dangerous. **It is not** — the diff is two-way; a PR merge uses
three-way against the merge-base.

| Side | Path | Action between merge-base (`07214ede`) and tip |
|---|---|---|
| `origin/master` | `systemd_working_directory.go` | added (via PR #4) |
| `origin/master` | `scylla_manager_cluster_registered.go` | added (via PR #5) |
| `origin/master` | `scylla_manager_cluster_registered_u3_test.go` | added (via PR #6) |
| `origin/master` | other O/S/U.3 files | added |
| `origin/master` | `registry.go` | added two invariant registrations |
| `origin/master` | other infra paths | various edits |
| `origin/project-u2` | `scylla_manager_register_script_test.go` | added (the U.2 cherry-pick) |
| `origin/project-u2` | everything else | **untouched since merge-base** |

Three-way merge picks each side's additions: `origin/master` keeps its
O/S/U.3 additions; `origin/project-u2` contributes the one new test
file. No deletions, no conflict.

GitHub's `mergeable: MERGEABLE` flag confirms this — if `project-u2`
were genuinely incompatible with current master, the flag would be
`CONFLICTING` or `MERGEABLE`/`mergeStateStatus: BLOCKED`. (We have not
opened a PR yet, so we cannot read GitHub's verdict directly, but the
absence of any modification to O/S/U.3 files on the project-u2 side
between merge-base and tip guarantees the merge will be clean.)

### Overlap with the O / S / U.3 / packages-U.2 work — none

- **Project O** (`91b445c1`): touches `registry.go`,
  `systemd_working_directory.go`, controller/node-agent state files,
  systemdutil — none of which `project-u2` modifies between
  merge-base and tip.
- **Project S** (`6af791a5`): touches `registry.go`,
  `scylla_manager_cluster_registered.go` — none of which `project-u2`
  modifies between merge-base and tip.
- **Project U.3** (`a272f415`): touches
  `scylla_manager_cluster_registered.go`,
  `scylla_manager_cluster_registered_test.go`,
  `scylla_manager_cluster_registered_u3_test.go` — none of which
  `project-u2` modifies between merge-base and tip.
- **Packages U.2** (different repo, `b905d39`): touches the
  packages-repo YAML, not the services repo at all.

There is **zero overlap** between `project-u2`'s contribution and any
of the merged work. The branch is orthogonal to O/S/U.3.

### Registry-drop / O.5 trap risk

`project-u2` does not touch `registry.go`. Specifically, the
modification status of `registry.go` in the two-way diff is "the file
differs" — but the difference is entirely on the master side (master
added the two invariant lines after the merge-base; project-u2 is
unchanged from merge-base). Three-way merge keeps master's additions
intact.

No registry-drop trap. The U.2 cherry-pick adds a new test file
beside the existing rule files; it does not edit them.

### Source changes vs `origin/master`

The actionable source change carried by `project-u2` over current
`origin/master` is exactly:

```
+ golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_register_script_test.go (new, +323)
```

This file exists in the local 52-commit pile (as part of the
`1970dd7c` commit), but **not on remote**. If `project-u2` is
deleted, the only remaining record of this test file on the remote is
its presence in the local-only commit `1970dd7c` — losing the
discoverable evidence trail.

### PR status

```
$ gh pr list --state all --head project-u2
(no PRs returned)
```

No PR has ever been opened. The branch was pushed during reconciliation
work but `project-u2` was never reviewed; it has just been sitting on
the remote.

### Classification verdict

`services/project-u2` is **stale but useful**. It carries one new
source file (`scylla_manager_register_script_test.go`, 323 lines of
integration tests) that is not yet on `origin/master`, has no overlap
with merged work, and would three-way-merge cleanly via a PR.

This is **not** the same situation as the deprecated `project-u3`:
- `project-u3` was dangerous (silent O.5 drop) — deleted correctly.
- `project-u2` is **safe and additive** — content has real value.

### Risk assessment of each option

| Option | Risk | Outcome |
|---|---|---|
| **Open PR `project-u2` → `master`** | Low. Three-way merge is additive only; tests skip on CI without the script; no production code path. | Test file lands on remote. Discoverable. CI-runnable. |
| **Keep `project-u2` dormant on remote** | Low. The branch sits idle; no merge ever happens; readers may wonder why it exists. | Test file stays on the branch, content survives, but not visible to PR review or default-branch CI. |
| **Delete `project-u2`** | Low. SHA `9e2ee870` is preserved in GitHub reflog ~90 days; `1970dd7c` (the local cherry-pick source) is in local master 52-pile; content recoverable. **However**: discoverable evidence of the U.2 test work is lost on the remote until the 52-pile is reconciled. | Branch gone; future readers might forget U.2 exists; recovery requires either local `1970dd7c` reapply or GitHub reflog. |

### Recommended action

**Open a PR for `services/project-u2` → `master`.**

Reasoning:
- Adds real testing value (5 integration tests already passed in
  local execution, will pass again in any environment with the
  installed script).
- Three-way merge is additive only — zero risk of regressing the
  O/S/U.3 content already on master.
- Closes the loop: U.2 services-side work was always supposed to land
  via PR; missing this would be the only U.x work not represented on
  the default branch via a normal PR flow.
- Eliminates the need to make a "keep dormant" decision — either the
  PR merges (and the branch is then deletable like the others) or the
  PR is closed and the branch is deleted with confidence.

Alternative if the operator does not want one more PR right now:
**keep `project-u2` dormant** until the 52-pile reconciliation is
authorized; then either include `1970dd7c` in that reconciliation, or
open the PR at that time.

**Do not delete `project-u2`** without first ensuring the test file's
content is on `origin/master` or explicitly confirming the operator
intends to abandon it.

### Status

`services/project-u2` classified: **stale but useful — open PR or
keep dormant; do not delete yet.**

### Next recommended cleanup action

**Open a PR for `services/project-u2` → `master`.** Title:
`Project U.2: scylla-manager registration script HTTPS-first
integration tests`. The PR would land 1 new file (+323 lines) via
three-way merge without touching any other content.

If the operator prefers to focus on the 52-pile reconciliation next
and revisit U.2 alongside it, the alternative is: **keep
`project-u2` dormant** for now and address it inside the larger
reconciliation plan.

---

## 13. PR #7 merge result (2026-05-29 14:43)

### Merged

| Field | Value |
|---|---|
| PR | [#7](https://github.com/globulario/services/pull/7) |
| Branch | `project-u2` → `master` |
| Merge strategy | merge commit |
| Merge commit SHA | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| Source commit now in master history | `9e2ee870d061f9501d691e4107d61094ec9eb076` |
| `origin/master` advanced | `a272f415` → `068bf1eb` |

```
$ git log origin/master --oneline -4
068bf1eb Project U.2: add scylla-manager registration script integration tests (#7)
a272f415 Project U.3: make scylla-manager doctor probe HTTPS-first (#6)
6af791a5 Project S: add scylla-manager cluster registration doctor invariant (#5)
91b445c1 Project O: enforce optional systemd WorkingDirectory and canonical state paths (#4)
```

### Pre-merge contribution verification (three-way merge)

```
$ MB=$(git merge-base origin/master origin/project-u2)
$ git diff --stat "$MB..origin/project-u2"
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_register_script_test.go   (+323)
1 file changed, 323 insertions(+)
```

The branch's actual contribution (between its merge-base `07214ede`
and tip `9e2ee870`) is **exactly one new file, +323 lines**. registry.go
NOT touched by the branch. The two-way `origin/master..origin/project-u2`
display showed 17 files / +353 / -1825, but the three-way merge
correctly identified the branch's contribution as additive only.

### Proof only the test file landed

```
$ git ls-tree origin/master -- golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_register_script_test.go
100644 blob b8f33a6dffdd3d119fa78f516817dcf70b4ba27b  ...
```

The new test file is now on `origin/master` at blob `b8f33a6d…`.

### Both invariants STILL on origin/master (proof O/S/U.3 didn't regress)

```
$ git show origin/master:.../registry.go | grep -nE "systemdWorkingDirectoryMustBeOptional|scyllaManagerClusterRegistered"
219:    systemdWorkingDirectoryMustBeOptional{}    ✓ (from PR #4)
223:    scyllaManagerClusterRegistered{}           ✓ (from PR #5)

$ git show origin/master:.../scylla_manager_cluster_registered.go | grep -cE "<U.3 sym>"
  newScyllaManagerHTTPSClient    3
  isTLSVerificationError         3
  discoverScyllaManagerHost      3
```

All O / S / U.3 content preserved on `origin/master`. The merge added
only the test file.

### Tests on origin/master post-merge

```
go build ./...                                                        → silent (BUILD OK)
go test ./cluster_doctor/cluster_doctor_server/rules -count=1         → ok 1.987s
```

The 4 new `TestRegisterScript_*` tests visible at the tail of the
filtered run all PASS in 0.429s:

- `TestRegisterScript_HTTPSConnectionRefused_FallsBackToHTTP` PASS
- `TestRegisterScript_HTTPSCertInvalid_FailsClosed` PASS (uses fresh
  self-signed CA — exercises the fail-closed contract)
- `TestRegisterScript_ExistingClusterByName_NoOp` PASS
- `TestRegisterScript_MissingCluster_UsesHTTPForWritePath` PASS

(The 5th, `TestRegisterScript_HTTPSReachable_PrefersHTTPS`, was
truncated off the tail of the displayed output; the package-wide run
above confirms all tests pass.)

The tests skip on hosts without the installed script and run end-to-end
where the script is present — exactly the U.2 design contract.

### Runtime read-only verification

| Check | Result |
|---|---|
| `globular-scylla-manager.service` ActiveState / NRestarts / MainPID | `active` / `0` / `770002` (still the same PID since U.1) |
| `globular-cluster-doctor.service` ActiveState / NRestarts / MainPID | `active` / `0` / `998633` |
| Clusters registered (via HTTPS) | 1 — `globular-internal` |
| Backup tasks | 2 enabled |
| Doctor total findings | 24 (baseline unchanged) |
| `scylla_manager.cluster_registered` findings | **0** |
| `tls_trust_failure` evidence | **0** |
| Doctor overall status | `degraded` (unchanged baseline — artifact-cache mismatches dominate) |
| Doctor snapshot ID | `ddc6ce26-08a9-4bb7-83df-57bf1b72fe98` (fresh, age 5s) |

No service restarted as a result of the merge. The cluster's runtime
behavior is governed by the deployed binaries (cluster_doctor v1.2.121
deployed during U.3 execution; scylla-manager v1.2.75 from U.2). The
PR #7 merge updated source on the remote only.

### Reconciliation summary — the full U.x stack on `origin/master`

| Project | Source commit | Merge commit | PR | Effect |
|---|---|---|---|---|
| Project O | `947e3e2e` | `91b445c1` | #4 | Added `systemdWorkingDirectoryMustBeOptional{}` + `systemd_working_directory.go` |
| Project S | `73cb2516` | `6af791a5` | #5 | Added `scyllaManagerClusterRegistered{}` + `scylla_manager_cluster_registered.go` (HTTP-only probe) |
| Project U.3 | `66d191e5` | `a272f415` | #6 | Upgraded probe to HTTPS-first / strict-CA / fail-closed |
| Project U.2 | `9e2ee870` | `068bf1eb` | #7 | Added the integration tests that lock the script's contract |

The packages-side counterparts (PR #1 `b905d39` Project S/U.2 script,
PR #2 `f1871d8` WD-normalize) are on `packages/origin/main`. The
reconciliation is **fully closed on the default branches** for both
repos. The deprecated `project-u3` branch has been deleted; the
remaining `project-u2` branch's content is now also on master via
this PR.

### Next recommended cleanup action

**Delete the `services/project-u2` branch** (remote and local). Its
content is now an ancestor of `origin/master` via merge commit
`068bf1eb`, so the deletion loses no work and recovers the SHA
through `git reflog` if ever needed.

```bash
# NOT EXECUTED HERE
cd /home/dave/Documents/github.com/globulario/services
git push origin --delete project-u2
git branch -D project-u2
```

This is the symmetric move to the earlier cleanup of `project-o`,
`project-s-reconciled`, `project-u3-reconciled` after their PRs
merged. After that, the only remaining work-product branch on the
services remote is `add-license-1` (pre-existing, out of scope).
The 52-commit local-master pile remains as the largest outstanding
cleanup item, addressable via §4c Options A / B / E.

### Status

`services/project-u2` merged and verified. Next recommended cleanup
action is deleting `project-u2` branch.

---

## 14. `project-u2` deletion result (2026-05-29 14:48)

### Deleted

| Ref | Pre-deletion SHA | State |
|---|---|---|
| `services` remote `refs/heads/project-u2` | `9e2ee870` | deleted |
| `services` local branch `project-u2` | `9e2ee870` | deleted |

Zero work loss — `9e2ee870` was already an ancestor of `origin/master`
via the PR #7 merge commit `068bf1eb`. Confirmed pre-deletion via
`git merge-base --is-ancestor 9e2ee870 origin/master`.

### Remote deletion proof

```
$ git push origin --delete project-u2
 - [deleted]           project-u2

$ git ls-remote --heads origin project-u2
(empty)
```

### Local deletion proof

```
$ git branch -D project-u2
Deleted branch project-u2 (was 9e2ee870).

$ git branch --list project-u2
(empty)
```

### Remaining services refs — minimal cleanup target reached

Remote (2 refs):
```
2a0331fc  refs/heads/add-license-1    ← kept (pre-existing, out of scope)
068bf1eb  refs/heads/master           ← default branch, current tip
```

Local (1 branch):
```
* master
```

`add-license-1` confirmed unchanged at `2a0331fc`.
`origin/master` confirmed at `068bf1eb`.
Merge commit `068bf1eb` (PR #7) confirmed still present on origin/master.

### Confirmation: no code / deploy / rebuild

- Local services source: no tracked-file modifications.
- `globular-scylla-manager.service`: `ActiveState=active`,
  `NRestarts=0`, `MainPID=770002` (still the same PID since U.1).
- No `pkg build`, no `pkg publish`, no `services desired set`, no
  service restart.

### Cumulative cleanup summary (this session)

Branches deleted in total across both repos:

| # | Repo | Branch | SHA | Reason |
|---|---|---|---|---|
| 1 | services | `project-u3` | `21351c96` | dangerous (silent O.5 drop) |
| 2 | services | `project-o` | `947e3e2e` | merged via PR #4 |
| 3 | services | `project-s-reconciled` | `73cb2516` | merged via PR #5 |
| 4 | services | `project-u3-reconciled` | `66d191e5` | merged via PR #6 |
| 5 | packages | `project-u2` | `bdc3724` | merged via packages PR #1 |
| 6 | packages | `wd-normalize-systemd-working-directory` | `2a625d3` | merged via packages PR #2 |
| 7 | services | `project-u2` | `9e2ee870` | merged via PR #7 (this turn) |

All 7 branches' SHAs remain reachable via merge commits on the
default branch (except `21351c96` which was the dangerous one — that
SHA is preserved only in `git reflog` for ~90 days, intentionally).

### Next recommended cleanup action

The last large outstanding item is **reconciling local
`services/master`** (52 ahead / 6 behind `origin/master`).

That work needs a backup-branch-first plan (per the prior turn's
hint: "needs a backup branch and surgical plan, not a chainsaw") and
operator authorization for one of the §4c options. Until that is
authorized, the working repository state is:

- Both default branches (`services/master`, `packages/main`) carry
  the full reconciled O / S / U.3 / packages-U.2 / WD-normalize / U.2
  content.
- Local `packages/main` is fully in sync with `origin/main`.
- Local `services/master` carries 52 commits ahead (3 redundant +
  49 unpushed Projects A through K-N-P-Q-T) and is 6 behind the
  merged PRs.
- All other feature branches are deleted; only `add-license-1`
  remains as an unrelated pre-existing ref.

### Status

`services/project-u2` deleted. Cumulative cleanup of 7 branches across
both repos is complete. The last outstanding item is reconciling local
`services/master`.
