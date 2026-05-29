# Local `services/master` reconciliation plan

**Date:** 2026-05-29
**Scope:** `globulario/services` local checkout only. The remote
`origin/master` is the source of truth at `068bf1eb`; this document
plans how to bring the local clone into alignment without losing the
48 unmerged-work commits or accidentally pushing the redundant ones.

Planning only. No rebase, no reset, no merge, no push.

---

## 1. Current state

| Field | Value |
|---|---|
| Local `master` tip | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| `origin/master` tip | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| Ahead | **52 commits** |
| Behind | **8 commits** |
| Uncommitted source mods | 0 |
| Stashes | 0 |
| Untracked files | only the conventional `loads/*.md` evidence files |

The "behind 8" count is higher than the "behind 6" recorded earlier
because PRs #6 and #7 both merged after the earlier audit. The 8
behind commits are the 4 merge commits (#4/#5/#6/#7) plus the 4
cherry-picked source commits (`947e3e2e`, `73cb2516`, `66d191e5`,
`9e2ee870`).

---

## 2. The 8 behind commits

```
068bf1eb  Project U.2 integration tests (#7)  ← merge
9e2ee870  Project U.2 source                  ← cherry-pick
a272f415  Project U.3 doctor HTTPS-first (#6) ← merge
66d191e5  Project U.3 source                  ← cherry-pick
6af791a5  Project S invariant (#5)            ← merge
73cb2516  Project S source                    ← cherry-pick
91b445c1  Project O WorkingDirectory (#4)     ← merge
947e3e2e  Project O source                    ← cherry-pick
```

---

## 3. The 52 ahead — classification

### 3a. Redundant (content already on `origin/master` — patch-id verified)

| Local SHA | Project | Matches origin commit | Verdict |
|---|---|---|---|
| `c529310e` | Project O | `947e3e2e` | **REDUNDANT** — drop |
| `16af03a8` | Project S | `73cb2516` | **REDUNDANT** — drop |
| `b19ce3aa` | Project U.3 | `66d191e5` | **REDUNDANT** — drop (note: b19ce3aa itself does NOT carry the O.5-drop bug; the bug was in the *cherry-pick resolution* on the original `project-u3` branch, not in the commit's own diff. Patch-id verifies identical contribution.) |
| `1970dd7c` | Project U.2 | `9e2ee870` | **REDUNDANT** — drop |

4 redundant commits.

### 3b. Useful unmerged (48 commits)

By group, oldest → newest:

| Group | SHAs | Project / theme | Status |
|---|---|---|---|
| v1.2.119 hotfix chain | `c185abde` `f061d334` `23a89318` `6e8d01a2` | 4 commits | unmerged |
| Project A | `80d6667d` | 1 commit (inventory only) | unmerged |
| Project A2 | `4dc2fb38` `5c537351` `13ebe537` | 3 commits | unmerged |
| Project A3 | `a8d9c43a` `29cb4bb7` | 2 commits | unmerged |
| Project A4 | `d03ec5f1` `17579a3e` | 2 commits | unmerged |
| Project A5 (kind-aware FAILED) | `f5faae69` `b0b555cb` `16355229` `5edee29f` `1616b123` `1e69384f` `ebd3fd18` | 7 commits | unmerged |
| Doctor stable-state event suppression | `9d1e36e5` | 1 commit (orphan) | unmerged |
| Project B (self-hosted proof writer) | `70f8871d` `14fbbc50` `a6af5d8f` `d83e0359` | 4 commits | unmerged |
| Project C (inventory only) | `2a051583` | 1 commit | unmerged |
| Project D (RepairArtifact backfill) | `efa82071` `931318db` `6a5bd635` `248857dd` `8d3fadc6` `93118c05` | 6 commits (the last cross-refs B) | unmerged |
| Project E / E2 (inventory only) | `48700467` `2383835a` | 2 commits | unmerged |
| Project F (MinIO drift recovery) | `7f977ab5` `2861ae84` | 2 commits | unmerged |
| Project J (workflow checksum semantics) | `e723331b` `ac866992` `9348560c` | 3 commits | unmerged |
| Project K (checksum-backfill CLI) | `27ab5d0e` `756a6522` `2fffb4d6` `76d16734` | 4 commits | unmerged |
| Project L (globularcli sidecar) | `a1feded1` | 1 commit | unmerged |
| ecdca55c (PID-start anchor) | `ecdca55c` | 1 commit | unmerged |
| Project N (wave_blocked retry) | `a03b1937` | 1 commit | unmerged |
| Project P (INFRASTRUCTURE remove fix) | `fa44aa57` | 1 commit | unmerged |
| Project T (verifier sidecar) | `eadc5690` | 1 commit | unmerged |
| Project Q (Spec.Paused on Infra) | `f10cb471` | 1 commit | unmerged |

48 commits total. **None** are obsoleted by anything currently on
`origin/master`; each represents authored work that may still be
worth shipping.

### 3c. Dependency analysis — do any local-ahead commits depend on the now-merged origin commits?

Patch-id analysis answered Q7. Yes implicitly: each of the redundant
commits has at least one local descendant that was authored against
its content. For example, `b19ce3aa` (U.3) was authored against
`16af03a8` (S) — drop S without dropping U.3 would leave U.3
referencing types S created. Since both are redundant (their content
is on origin), dropping them together is safe.

For the **48 useful commits**, the dependency on the redundant ones
is *implicit only*: they assume O/S/U.3/U.2 content is somewhere in
the tree. Since `origin/master` now provides that content, the 48
will replay on top of `origin/master` cleanly — except where they
overlap files origin's PRs also touched. That overlap is the next
question.

---

## 4. Conflict hotspot analysis

### 4a. Files touched by origin/master's 8 behind commits

17 unique files:

```
golang/cluster_controller/cluster_controller_server/main.go
golang/cluster_controller/cluster_controller_server/state.go
golang/cluster_controller/cluster_controller_server/state_migration_test.go
golang/cluster_doctor/cluster_doctor_server/rules/registry.go
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_test.go
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_u3_test.go
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_register_script_test.go
golang/cluster_doctor/cluster_doctor_server/rules/systemd_working_directory.go
golang/cluster_doctor/cluster_doctor_server/rules/systemd_working_directory_test.go
golang/globularcli/services_cmds.go
golang/node_agent/node_agent_server/internal/actions/artifact.go
golang/node_agent/node_agent_server/main.go
golang/node_agent/node_agent_server/state.go
golang/node_agent/node_agent_server/state_migration_test.go
golang/systemdutil/working_directory.go
golang/systemdutil/working_directory_test.go
```

### 4b. Files touched by the 48 useful local commits

96 unique files across the 48 commits.

### 4c. Intersection — the actual conflict hotspots

```
$ comm -12 <(sort behind_files) <(sort local_files)
golang/globularcli/services_cmds.go
```

**Exactly one hotspot.**

### 4d. Which local commit touches the hotspot

```
golang/globularcli/services_cmds.go
  a1feded1  globularcli: write systemd unit .sha256 sidecar after install (Project L)
```

Only **Project L** (`a1feded1`) modifies `services_cmds.go` among the
48 useful commits. Project O also modifies the same file (on origin
via `947e3e2e`), so when L is replayed onto a master containing
Project O's changes, git will see a conflict in that file.

This is the only place in the 48-commit pile where a deliberate
3-way-merge resolution will be required. Every other commit's
changes land cleanly because origin/master never touched their files.

### 4e. Conflict resolution sketch for Project L

Project O adds the `systemdutil.NormalizeUnitWorkingDirectory` call
to `services_cmds.go`'s install path. Project L adds the sidecar
write to the same install path. Both edits are in the same function
region but the changes are independent (one normalizes WD, one
writes a checksum sidecar). Merge-resolution: keep both additions in
the natural order (sidecar write after the normalize call, or
vice versa — doesn't matter functionally). No content loss.

---

## 5. Strategy comparison

### Option A — backup + rebase with drops

```
backup/local-master-before-reconcile-<ts>
git checkout master
git rebase origin/master \
    --onto origin/master \
    <skip the 4 redundant commits in interactive mode>
# OR
git rebase -i origin/master
# (mark c529310e, 16af03a8, b19ce3aa, 1970dd7c as 'drop')
# (resolve the one conflict on a1feded1)
```

**Pros:**
- Single operation when complete.
- Local master ends up as a linear history of origin/master + 48
  useful commits.
- Preserves authorship and commit messages for the 48.
- The 4 drops are git-recognized as "no-op" patches and may be
  auto-dropped, simplifying the interactive flow.

**Cons:**
- Interactive rebase across 52 commits is fiddly.
- The Project L conflict must be resolved during rebase, mid-flow.
- Any error during rebase forces an abort or a `git rebase --skip`
  decision per commit.
- Future work that wanted to keep its old SHAs (e.g. external
  references in evidence docs) will be invalidated — rebase rewrites
  every commit SHA after the rebase point.

### Option B — backup + reset + cherry-pick per topic group (**user's preferred**)

```
backup/local-master-before-reconcile-<ts>
git checkout master
git reset --hard origin/master                    # destroys the 52 ahead locally
# then per project group:
git checkout -b <topic-branch> origin/master
git cherry-pick <project's commits from backup>
# (resolve conflicts only when they actually arise)
git push -u origin <topic-branch>
# open PR, review, merge
```

**Pros:**
- Each topic group becomes its own PR, mirroring the O→S→U.3→U.2
  reconciliation pattern that already worked.
- Operator chooses granularity (one PR per project or grouped).
- The Project L conflict surfaces only when L is cherry-picked,
  in isolation — easy to reason about.
- Same `cherry-pick + push + PR` discipline as the rest of this
  session; nothing new to learn.
- Local master ends up clean and equal to origin/master.

**Cons:**
- ~13 topic branches × cherry-pick × PR is sequential work over
  multiple sessions; not a single operation.
- The backup branch's SHAs are not preserved as ancestors of the
  default branch (only in the backup ref). Anything that
  referenced a specific old SHA must be updated.

### Option C — leave local master diverged

Continue doing future work via feature branches off `origin/master`
(as the O→S→U.3→U.2 reconciliation already did). Never reset / rebase
local master. Eventually delete the local checkout and re-clone if
the operator wants a clean slate.

**Pros:**
- Zero risk of accidental data loss.
- Preserves all SHAs locally for any future archaeology.
- The 49 useful commits stay reachable in the local clone.

**Cons:**
- `git status` will perpetually say "ahead 52 / behind 8" (count
  grows as more PRs land).
- A `git pull` would attempt a merge and likely hit the same single
  conflict on `services_cmds.go` — gets messy fast.
- 48 commits of authored work are effectively shelved — the original
  authors' content never reaches the remote.

### Option D — hard reset (destructive, **NOT recommended**)

```
git reset --hard origin/master
```

Without a backup, this destroys 49 commits of useful work. Even with
a backup, this is only acceptable if every project in the 48-commit
pile is intentionally being abandoned (e.g. all superseded by future
remote-first work, or judged not worth shipping). That situation
should be explicit, not implicit.

**Recommendation: do not authorize Option D** unless every project
in §3b is documented as deliberately retired.

---

## 6. Recommended strategy

**Option B — backup + reset + cherry-pick per topic group.**

Reasons:
- Matches the discipline that worked for O / S / U.3 / U.2: each
  project landed as its own clean feature branch + PR.
- Surfaces conflicts only when they actually arise, in isolation
  (Project L's `services_cmds.go` overlap becomes a single
  cherry-pick conflict, not a mid-rebase blocker).
- Operator controls granularity and pacing — one project per session
  is acceptable.
- Identical workflow to the rest of this reconciliation session, so
  there is no new methodology to internalize.
- "Turns the vine knot into labeled garden rows" (user's instinct).

Order of cherry-pick (suggested):

1. v1.2.119 hotfix chain (4 commits) — independent of the A series.
2. Project A series (16 commits including A5's 7 follow-ups) — long
   linear chain.
3. `9d1e36e5` cluster_doctor stable-state suppression (1 commit, orphan).
4. Project B (4 commits).
5. Project C (1 commit inventory).
6. Project D (6 commits).
7. Project E + E2 (2 inventory commits).
8. Project F (2 commits).
9. Project J (3 commits).
10. Project K (4 commits).
11. Project L (1 commit) — **expect conflict on `services_cmds.go`**.
12. `ecdca55c` PID-start anchor (1 commit).
13. Project N (1 commit).
14. Project P (1 commit).
15. Project T (1 commit).
16. Project Q (1 commit).

Each step: create branch from current `origin/master`, cherry-pick,
push, open PR, merge, move on. Conflict only expected at step 11.

---

## 7. Exact future command sequence (NOT executed in this turn)

### Step 0 — create the backup ref FIRST

```bash
cd /home/dave/Documents/github.com/globulario/services
TS=$(date +%Y%m%d-%H%M%S)
git branch backup/local-master-before-reconcile-$TS master
# verify the backup ref exists and points at the current local master tip
git rev-parse backup/local-master-before-reconcile-$TS
# (should match) git rev-parse master = b19ce3aa
```

The backup ref is **local-only** by default. It can be pushed to remote
for extra durability if desired:

```bash
git push -u origin backup/local-master-before-reconcile-$TS
```

### Step 1 — reset local master to origin/master

```bash
git fetch origin
git checkout master
git reset --hard origin/master
git status
# expect: "Your branch is up to date with 'origin/master'"
```

### Step 2 — per-project cherry-pick + PR loop (repeat 16× per the §6 order)

```bash
cd /home/dave/Documents/github.com/globulario/services
git fetch origin
git checkout -b <topic-branch> origin/master

# cherry-pick from backup (one project's commits)
git cherry-pick <sha1> <sha2> ... <shaN>

# resolve any conflict (only expected at Project L step)
# verify build + tests
cd golang && go build ./... && go test ./... -short
cd ..

# push
git push -u origin <topic-branch>

# open PR
gh pr create --base master --head <topic-branch> --title "<title>" --body "<body>"

# (operator merges via review)

# after merge, delete topic branch
git push origin --delete <topic-branch>
git branch -d <topic-branch>

# fast-forward local master
git checkout master
git pull --ff-only
```

### Step 3 — once all 48 commits are landed, delete the backup ref

```bash
# Only after every useful project has been pushed and merged
git branch -D backup/local-master-before-reconcile-<ts>
git push origin --delete backup/local-master-before-reconcile-<ts>   # if it was pushed
```

---

## 8. Stop conditions

Abort the reconciliation campaign immediately if any of the following
occurs:

1. The backup ref creation fails or its SHA does not match the local
   master tip — abort and investigate.
2. `git reset --hard` does not result in `origin/master == local
   master`. Re-fetch and verify.
3. A cherry-pick fails with a conflict in a file other than
   `services_cmds.go` for Project L. This contradicts the hotspot
   analysis; investigate before proceeding.
4. Any cherry-picked project's tests fail. Halt that project's PR,
   keep the backup ref, restart from a known-good cherry-pick base.
5. A PR review surfaces unexpected content (e.g. a project's commit
   was actually superseded by something on origin we missed) —
   halt that project, leave it on the backup ref, document it as
   "abandoned per review feedback".

---

## 9. Commands that must NOT run without explicit authorization

| Command | Risk | Authorization needed for |
|---|---|---|
| `git reset --hard origin/master` | Destroys 49 ahead commits if backup absent | this turn's plan creates a backup first; only run after backup is verified |
| `git rebase -i origin/master` | Rewrites SHAs across 52 commits; mid-rebase aborts can corrupt state | only if Option A is chosen instead of Option B |
| `git push --force origin master` | Overwrites remote master | **never** authorized in this codebase |
| `git push -u origin master` | Tries to push local master's 52 ahead | not authorized — the 52 ahead are not yet on remote, and pushing now would re-introduce the redundant + buggy SHAs |
| `git branch -D backup/...` | Deletes the safety net | only after every useful project has landed in a PR |

---

## Next authorized action

**Next authorized action should be: create the backup ref**

```bash
cd /home/dave/Documents/github.com/globulario/services
TS=$(date +%Y%m%d-%H%M%S)
git branch backup/local-master-before-reconcile-$TS master
git rev-parse backup/local-master-before-reconcile-$TS  # verify matches b19ce3aa
```

This is the smallest single move that unlocks Option B safely. It is
purely additive — creates one new local ref pointing at the current
master tip. Nothing is reset, nothing is pushed, nothing is rewritten.
Once the backup is in place, the operator can authorize Step 1 (the
hard reset) and the per-project cherry-pick loop with confidence
that no work is at risk of permanent loss.

If the operator instead prefers **Option C** (leave local master
diverged indefinitely), the backup ref is also a reasonable
investment — it labels the divergent state for future readers, and
the loss-cost of the 49 commits remains low because the local clone
preserves them as long as the disk survives.

---

## 10. Backup ref creation result (2026-05-29 14:46)

### Created

| Field | Value |
|---|---|
| Backup branch name | `backup/local-master-before-reconcile-20260529-144649` |
| Backup branch SHA | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| Local `master` SHA (pre-create) | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| Local `master` SHA (post-create) | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| `origin/master` SHA (pre-create) | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `origin/master` SHA (post-create) | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |

### Verification

- **Backup matches pre-create local master:** SHA equal — ✓
- **Local master unchanged:** SHA equal pre/post — ✓
- **`origin/master` unchanged:** SHA equal pre/post — ✓

### Branch list

```
  backup/local-master-before-reconcile-20260529-144649
* master
```

Two local refs: master (current) and the backup. No other local
branches.

### Confirmation: no mutation

- **No reset** — `master` still at `b19ce3aa`.
- **No rebase** — history of `master` unchanged.
- **No merge** — local master not pulled.
- **No cherry-pick** — no commits added to either ref.
- **No push** — backup ref is local-only; remote unchanged at
  `add-license-1` + `master` only.
- **No branches deleted.**
- **Source tree clean** — zero tracked-file modifications.
- **Runtime untouched** — `globular-scylla-manager.service` still
  `active`, `NRestarts=0`, `MainPID=770002` (same PID since U.1).

### Recovery instructions (if reset later loses something)

If the operator authorizes Step 1 (the `git reset --hard origin/master`)
and later discovers a useful commit was lost, recovery is:

```bash
cd /home/dave/Documents/github.com/globulario/services
# the backup ref still exists locally and points at the pre-reset state
git rev-parse backup/local-master-before-reconcile-20260529-144649
# expected: b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41

# cherry-pick the missing commit from the backup branch
git cherry-pick <sha-from-backup>

# OR temporarily check out the backup to inspect
git checkout backup/local-master-before-reconcile-20260529-144649
# do whatever inspection is needed
git checkout master   # come back
```

The backup ref survives across reboots, git operations on master,
and any of the cherry-pick / push / merge work in §6's order. It
is only at risk if the operator explicitly runs
`git branch -D backup/...` — which is documented in §9 as requiring
explicit authorization.

### Next recommended action

**Reset local `master` to `origin/master`** (Option B Step 1).

The backup ref makes this operation reversible: in the worst case
(a project's contribution is lost or a cherry-pick conflict can't be
resolved), the operator can `git checkout backup/...` and recover
the original 52-ahead state in full.

The exact command — documented in §7 Step 1 — is:

```bash
cd /home/dave/Documents/github.com/globulario/services
git fetch origin
git checkout master
git reset --hard origin/master
git status   # expect "Your branch is up to date with 'origin/master'"
```

This document does not authorize that command. The operator should
authorize it explicitly before any reset is run.

### Status

Backup ref created. Ready for reset-to-origin authorization.
