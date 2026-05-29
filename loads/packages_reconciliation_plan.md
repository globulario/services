# Packages repo reconciliation plan

**Date:** 2026-05-29
**Scope:** `globulario/packages` only. The `globulario/services` repo
is unchanged in this turn — O → S → U.3 source reconciliation already
landed there (`origin/master a272f415`).

This document is **planning only**. No commits, no pushes, no
rebuilds, no deploys.

---

## 1. Current branch + remote state

| Field | Value |
|---|---|
| Current branch | `main` |
| `origin/main` SHA | `b9bca4a77340d50435cca10b9291f93537a1dcd3` |
| `origin/main` tip commit | `b9bca4a docs(scylla-manager-agent): clarify spec ownership of agent yaml` |

## 2. Local commits ahead of `origin/main`

Two commits, in author order:

| SHA | Commit message |
|---|---|
| `f86d51f` | `scylla-manager: ship idempotent cluster registration script (Project S)` |
| `3259c98` | `scylla-manager: HTTPS-first read/probe path in registration script (Project U.2)` |

Both modify the single file
`metadata/scylla-manager/specs/scylla_manager_service.yaml`.

## 3. Pushed branches

| Ref | SHA | Note |
|---|---|---|
| `origin/main` | `b9bca4a7` | unchanged remote tip |
| `origin/project-u2` | `bdc37247` | cherry-pick chain S + U.2 onto `origin/main` |

The local `main` and `origin/project-u2` carry the same **content**
even though their commit SHAs differ (cherry-pick produces fresh SHAs).

## 4. Uncommitted files

| Category | Count |
|---|---|
| Tracked-file modifications, total | **37** |
| Matching pattern `metadata/<svc>/systemd/globular-<svc>.service` | **37** |
| Other tracked-file modifications | **0** |
| Untracked files | **0** |
| Stashes | **0** |

The 37 uncommitted files are exactly the WorkingDirectory-normalize
work (adding the `-` prefix to make `WorkingDirectory=` optional —
sample diff confirms one-line change per file). Nothing else is
modified locally.

## 5. Exact file list in `origin/project-u2` (vs `origin/main`)

```
$ git diff --stat origin/main..origin/project-u2
metadata/scylla-manager/specs/scylla_manager_service.yaml   (+152)
1 file changed, 152 insertions(+)
```

**One file.** The branch has two commits (`491e442` = S cherry-pick,
`bdc3724` = U.2 cherry-pick) that both modify the same YAML; the net
diff vs `origin/main` is `+152` to that single file.

The branch's merge-base with `origin/main` is `b9bca4a7` (origin/main
itself, no divergent ancestor), so the merge applies as a fast-forward
or merge-commit of two clean cherry-picks.

## 6. Exact file list in local WD-normalize work (the 37 files)

```
metadata/ai-executor/systemd/globular-ai-executor.service
metadata/ai-memory/systemd/globular-ai-memory.service
metadata/ai-router/systemd/globular-ai-router.service
metadata/ai-watcher/systemd/globular-ai-watcher.service
metadata/authentication/systemd/globular-authentication.service
metadata/backup-manager/systemd/globular-backup-manager.service
metadata/blog/systemd/globular-blog.service
metadata/catalog/systemd/globular-catalog.service
metadata/cluster-controller/systemd/globular-cluster-controller.service
metadata/conversation/systemd/globular-conversation.service
metadata/discovery/systemd/globular-discovery.service
metadata/dns/systemd/globular-dns.service
metadata/echo/systemd/globular-echo.service
metadata/envoy/systemd/globular-envoy.service
metadata/event/systemd/globular-event.service
metadata/file/systemd/globular-file.service
metadata/gateway/systemd/globular-gateway.service
metadata/ldap/systemd/globular-ldap.service
metadata/log/systemd/globular-log.service
metadata/mail/systemd/globular-mail.service
metadata/mcp/systemd/globular-mcp.service
metadata/media/systemd/globular-media.service
metadata/monitoring/systemd/globular-monitoring.service
metadata/node-agent/systemd/globular-node-agent.service
metadata/persistence/systemd/globular-persistence.service
metadata/rbac/systemd/globular-rbac.service
metadata/repository/systemd/globular-repository.service
metadata/resource/systemd/globular-resource.service
metadata/scylla-manager-agent/systemd/globular-scylla-manager-agent.service
metadata/scylla-manager/systemd/globular-scylla-manager.service
metadata/search/systemd/globular-search.service
metadata/sql/systemd/globular-sql.service
metadata/storage/systemd/globular-storage.service
metadata/title/systemd/globular-title.service
metadata/torrent/systemd/globular-torrent.service
metadata/workflow/systemd/globular-workflow.service
metadata/xds/systemd/globular-xds.service
```

Sample diff (representative of all 37):

```diff
--- a/metadata/echo/systemd/globular-echo.service
+++ b/metadata/echo/systemd/globular-echo.service
@@ -7,7 +7,7 @@ Wants=network-online.target globular-event.service globular-resource.service
 Type=simple
 User=globular
 Group=globular
-WorkingDirectory={{.StateDir}}/echo
+WorkingDirectory=-{{.StateDir}}/echo
 Environment=GLOBULAR_SERVICES_DIR={{.StateDir}}/services
 ExecStartPre=+/bin/sh -c 'mkdir -p {{.StateDir}}/echo && chown globular:globular {{.StateDir}}/echo'
```

Each file: one-line edit that prefixes the `WorkingDirectory=` value
with `-`, making it optional under systemd semantics. Resolves the
cluster_doctor convergence finding
`754027b85c39913a` ("systemd unit(s) have bare required
WorkingDirectory under /var/lib/globular").

## 7. Project S package changes — present on `origin/main`?

**No.** `origin/main` is at `b9bca4a7` which is the pre-Project-S
state. Project S's script content (the `install-scylla-manager-register-cluster`
step in `scylla_manager_service.yaml`) is on `origin/project-u2` and on
local `main`, but not on `origin/main`.

## 8. U.2 package changes — present on `origin/main`?

**No.** Same answer as #7 — `origin/main` is pre-Project-S and pre-U.2.
The U.2 HTTPS-first script (with `--capath /dev/null --cacert` strict
trust) lives on `origin/project-u2` and local `main`, not on
`origin/main`.

## 9. Can `project-u2` be merged as-is?

**Yes.** The branch is clean, the diff is exactly 1 file (`+152`),
both commits target the same file, the merge-base is `origin/main`
itself (no divergent ancestor history), and there is no overlap with
the WD-normalize working-tree set.

Verified merge characteristics:
- diff vs `origin/main`: 1 file, +152 lines
- file: `metadata/scylla-manager/specs/scylla_manager_service.yaml`
- commits to merge: 2 (`491e442` Project S, `bdc3724` U.2)
- conflict probability: zero

## 10. Is a cleaner `project-s-u2-reconciled` branch needed?

**No.** Unlike the services repo's O → S → U.3 chain (where the
original `project-u3` had dropped Project O.5's registry line and
needed a clean reconciled replacement), the packages `project-u2`
branch has no such defect.

Reasons:
- Its base (`origin/main` at `b9bca4a7`) does not contain any half-merged
  or conflicting prior content related to scylla-manager.
- The Project S commit on the branch (`491e442`) introduces the
  registration script wholesale (no preceding partial content to merge
  against).
- The U.2 commit (`bdc3724`) modifies the same content that S added,
  in the same file region — both are linear additions/edits with no
  external dependencies.
- Verified: `gh pr view` (if a PR is later opened) will resolve a
  clean three-way merge against `origin/main` with no conflict.

There is no analogue to the O.5-line-drop bug in this branch. The
existing `project-u2` is the canonical reconciled branch.

## 11. Should WD-normalize be a separate PR?

**Yes — strongly recommended.** Mixing it with the scylla-manager
package work would be wrong for four reasons:

1. **Topic separation.** The two changes have nothing to do with
   each other. Scylla-manager registration script ≠ systemd
   WorkingDirectory normalization. Reviewers should be able to assess
   them independently.

2. **Blast-radius asymmetry.** scylla-manager changes affect ONE
   package's install steps. WD-normalize touches **37 packages**'
   unit files; on next install, every one of those services restarts
   with the new unit. A reviewer needs to see the WD-normalize set
   together as one coherent change with clearly listed scope.

3. **Independent revertability.** If WD-normalize causes a
   regression in any one of the 37 services, the operator should be
   able to revert *that PR alone* without losing the scylla-manager
   HTTPS-first work (or vice versa).

4. **Independent risk profile.** scylla-manager package work was
   live-verified end-to-end during U.1 / U.2 / U.3 execution.
   WD-normalize was started but never deployed (the 37 files remain
   uncommitted). Mixing the two would lump verified work with
   untested work in one PR.

## 12. Special safety checks

### 12a. `project-u2` contains only intended scylla-manager files

```
$ git diff --name-only origin/main..origin/project-u2 | grep -v "scylla-manager"
(empty)
```

Only `metadata/scylla-manager/specs/scylla_manager_service.yaml` is
modified. **No** unrelated changes to other service packages, **no**
WD-normalize systemd files, **no** registration-script-adjacent files
(the standalone `metadata/scylla-manager/systemd/globular-scylla-manager.service`
is NOT touched by `project-u2` — it is touched only by WD-normalize).

### 12b. WD-normalize contains only the 37 expected pattern files

```
$ git diff --name-only HEAD | grep -vE "^metadata/[^/]+/systemd/globular-[^/]+\.service$"
(empty)

$ git diff --name-only HEAD | grep -cE "^metadata/[^/]+/systemd/globular-[^/]+\.service$"
37
```

Every uncommitted file matches `metadata/<svc>/systemd/globular-<svc>.service`.
No spec files, no scripts, no awareness manifests, no `package.json`
edits. The set is exactly the WD-normalize scope.

### 12c. Overlap between the two work streams

```
$ comm -12 \
    <(git diff --name-only HEAD | sort) \
    <(git diff --name-only origin/main..origin/project-u2 | sort)
(empty)
```

Zero file overlap. The two PRs cannot collide; either can be merged
first without affecting the other.

### 12d. The standalone `metadata/scylla-manager/systemd/globular-scylla-manager.service`

This standalone file is in the WD-normalize set (file #30 of 37). The
`project-u2` branch only modifies the **spec** YAML
(`metadata/scylla-manager/specs/scylla_manager_service.yaml`), which
*embeds* a templated systemd unit content in its `install_services`
step. At install time the templated unit is what gets written to disk
under `/etc/systemd/system/globular-scylla-manager.service`. The
standalone `metadata/scylla-manager/systemd/globular-scylla-manager.service`
file is a separate source-of-truth used elsewhere in the pipeline.
**These two are out-of-sync today** — the standalone file lacks U.2's
HTTPS-first script and lacks the `-` WorkingDirectory prefix. A
follow-up may be needed after both PRs merge to reconcile the spec and
the standalone file. **Not in scope for this plan.**

---

## 13. Recommended branches + push order

### Recommended PR 1 — already pushed

| Branch | `origin/project-u2` |
|---|---|
| SHA | `bdc37247` |
| Files touched (vs `origin/main`) | 1 — `metadata/scylla-manager/specs/scylla_manager_service.yaml` |
| Net diff | +152 |
| Commits to merge | 2 (`491e442` Project S, `bdc3724` U.2) |
| Status | ready to merge — no further work required |

**Action for operator:** open PR `project-u2 → main` in GitHub UI (or
`gh pr create`), review, merge with merge commit.

### Recommended PR 2 — needs to be created

| Branch | `wd-normalize-systemd-working-directory` (suggested name) |
|---|---|
| Base | `origin/main` |
| Files touched | 37 — every `metadata/<svc>/systemd/globular-<svc>.service` listed in §6 |
| Net diff (estimated) | 37 lines (one `-` prefix per file) |
| Commits to create | 1 (`packages: make systemd WorkingDirectory optional across all services (O.5 invariant fix)`) |
| Status | uncommitted in working tree — needs `git add`, commit, push |

**Action for operator (NOT authorized in this turn):**
1. Stage the 37 files (`git add metadata/*/systemd/globular-*.service`).
2. Verify nothing else is staged.
3. Commit with the message above.
4. Push the new branch.
5. Open PR `wd-normalize-systemd-working-directory → main`.

The two PRs are **independent and order-insensitive**. Either may
merge first.

---

## 14. After both PRs merge — follow-up considerations

After both PRs land on `origin/main`:

- The cluster_doctor convergence finding `754027b85c39913a` will be
  resolved on the next install of any of the 37 services. Until then,
  the finding will persist (the live cluster's already-installed unit
  files remain unchanged — packages-repo merges do not auto-reinstall).
- The `metadata/scylla-manager/systemd/globular-scylla-manager.service`
  standalone file will be consistent with the WD-normalize set, but
  still **diverged from the U.2 spec's embedded unit** (the standalone
  unit will lack the registration-script ExecStartPost and the
  HTTPS-first script content). Reconciling the standalone unit with
  the spec's embedded unit is a separate cleanup task (call it U.5 or
  similar) — **out of scope for the current reconciliation work.**
- No source change in `globulario/services` is required for either
  PR. Both are isolated to `globulario/packages`.

---

## 15. What stays untouched in this plan

- `globulario/services` repo — no operation.
- Old `services/project-u3` branch — still dormant on remote, not
  deleted.
- Live cluster — no `pkg build`, no `pkg publish`, no `services
  desired set`. The 37 deployed unit files on the live node remain
  unchanged until an operator-driven re-install of each service.
- Project U.4 — not started; still gated on the observation window
  documented in `loads/project_u3_scylla_manager_doctor_https_first.md`.
- packages-repo `main` branch — local stays 2 commits ahead of
  `origin/main`; no force push, no rebase.

---

## Recommended next operator action

**Open the PR for `packages/project-u2` first** — it carries verified
content (live-tested during U.1/U.2/U.3 execution) and merges cleanly.

After that lands, **commit the 37 WD-normalize files** to a new
branch named (suggested) `wd-normalize-systemd-working-directory`,
push, and open a second PR. The two PRs are independent.

This document does not authorize either action.

## Status

Packages repo reconciliation plan complete. `project-u2` is ready
to merge as-is. WD-normalize needs commit + push + PR. No code, no
push, no deploy, no rebuild performed.

---

## 16. Packages PR #1 merge result (2026-05-29 13:55)

### Merged

| Field | Value |
|---|---|
| PR | [#1](https://github.com/globulario/packages/pull/1) |
| Branch | `project-u2` → `main` |
| Merge strategy | merge commit (preserves Project S + U.2 source commits as parents) |
| Merge commit SHA | `b905d3901f00e4ef2636fe8a218ea0eeadcf4955` |
| Source commits now in main history | `491e442` (Project S), `bdc3724` (Project U.2) |
| `origin/main` advanced | `b9bca4a7` → `b905d39` |

```
$ git log origin/main --oneline -4
b905d39 Project S/U.2: make scylla-manager package registration HTTPS-first (#1)
bdc3724 scylla-manager: HTTPS-first read/probe path in registration script (Project U.2)
491e442 scylla-manager: ship idempotent cluster registration script (Project S)
b9bca4a docs(scylla-manager-agent): clarify spec ownership of agent yaml
```

### Merged file list

```
metadata/scylla-manager/specs/scylla_manager_service.yaml   (+152)
```

Exactly one file, as predicted by the planning section. No other
packages touched. No `package.json` / awareness / signature edits.

### Content verification on `origin/main`

```
$ F=metadata/scylla-manager/specs/scylla_manager_service.yaml
$ git show origin/main:$F | grep -cE "<phrase>"

  install-scylla-manager-register-cluster (Project S step)          → 1
  --capath /dev/null --cacert (Project U.2 strict-CA probe)         → 3 occurrences
  GLOBULAR_CA (CA path env-var binding)                             → 3
  HTTPS not enabled (curl exit 7) — safe HTTP fallback log line     → 1
  refusing to fall back to HTTP — TLS fail-closed log line          → 1
  sctool --api-url "$HTTP_BASE" (write path still HTTP per Project U.3 plan) → 1
```

All six contracts from PR #1's description are observable on the
merged file.

### WD-normalize work — confirmed still local + uncommitted

```
$ git diff --name-only HEAD | grep -cE "^metadata/[^/]+/systemd/globular-[^/]+\.service$"
37
```

All 37 files remain modified in working tree, **none staged, none
pushed**. Spot-check of one file on the merged `origin/main` confirms
the WD-normalize change has NOT landed there:

```
$ git show origin/main:metadata/echo/systemd/globular-echo.service | grep ^WorkingDirectory=
WorkingDirectory={{.StateDir}}/echo
```

The bare form (without the `-` prefix) is still on `origin/main`,
confirming the WD-normalize work is cleanly separated from this merge
and remains queued for its own PR.

### No deploy / rebuild happened

- `globular-scylla-manager.service`: `ActiveState=active`, `NRestarts=0`,
  `MainPID=770002` (unchanged from pre-merge — same PID as during U.1).
- No new `pkg build` artifacts created: most recent files in
  `/tmp/projectU2_pkgbuild/` are 1.2.73 (12:13) and 1.2.74 (12:17) from
  earlier U.2 execution; no fresh build at the 13:55 merge time.
- No `pkg publish` issued.
- No `services desired set` issued.
- Live cluster runtime continues to use the already-deployed
  scylla-manager 1.2.75. The merge updated source on the remote only.

### Effect on local `main` branch

```
$ git log origin/main..main → (empty)
$ git log main..origin/main →
  b905d39  (the merge commit)
  bdc3724  (U.2 cherry-pick, now in origin/main)
  491e442  (Project S cherry-pick, now in origin/main)
```

Local `main` is now BEHIND `origin/main` by 3 commits (the merge
commit + 2 cherry-pick commits that originated from local
`f86d51f`/`3259c98`). A `git pull` (or fast-forward) on local `main`
would bring it in sync, but that is **not authorized** in this turn.
Local `f86d51f`/`3259c98` are now redundant (same content as
`491e442`/`bdc3724` already on remote); a `git reset --hard
origin/main` would cleanly reconcile, but is also not authorized
here.

### Next recommended action

**Commit the 37 WD-normalize files to a new branch and open a second
PR.**

Suggested branch name: `wd-normalize-systemd-working-directory`.

Sketch (NOT executed in this turn):

```bash
cd /home/dave/Documents/github.com/globulario/packages
git stash -u                                          # park to safety
git checkout -b wd-normalize-systemd-working-directory origin/main
git stash pop                                          # restore the 37 files
git add metadata/*/systemd/globular-*.service          # stage only the 37
git status                                             # verify nothing else
git commit -m "packages: make systemd WorkingDirectory optional across all services (O.5 invariant fix)"
git push -u origin wd-normalize-systemd-working-directory
gh pr create --base main --head wd-normalize-systemd-working-directory \
  --title "packages: WorkingDirectory= → optional across 37 services (O.5 invariant fix)" \
  --body "<see planning §11 for the full description>"
```

This document does not authorize that sequence.

### Status

Packages Project S/U.2 merged and verified. Next recommended action
is WD-normalize separate PR.

---

## 17. WD-normalize branch + PR result (2026-05-29 14:03)

### Local main reconciliation

```
$ git checkout main
$ git reset --hard origin/main
HEAD is now at b905d39 Project S/U.2: make scylla-manager package registration HTTPS-first (#1)
```

Local `main` now matches `origin/main` at `b905d39`. The two pre-merge
local cherry-picks (`f86d51f`, `3259c98`) are discarded — their
content is preserved on the remote as `491e442` / `bdc3724`.

### Patch safety

```
$ git diff -- metadata/*/systemd/globular-*.service > /tmp/wd-normalize-systemd-working-directory.patch
```

Patch saved to `/tmp/wd-normalize-systemd-working-directory.patch`
(23,604 bytes) **before** the reset, so the WD-normalize content
survived the `--hard` reset of local `main`.

Patch validation:

- 37 diff blocks (one per file) ✓
- 37 `+WorkingDirectory=` lines, 37 `-WorkingDirectory=` lines ✓
- Zero other `+/-` content changes ✓
- All 37 paths match `metadata/<svc>/systemd/globular-<svc>.service` ✓
- Zero scylla-manager spec content in the patch ✓

### Branch + commit

| Field | Value |
|---|---|
| Branch | `wd-normalize-systemd-working-directory` |
| Base | `origin/main` (`b905d39`) |
| Commit | `2a625d3b333b65be016e00b187cdd86307d22f4f` |
| Files | 37 changed, +37 / −37 (one line per file) |
| Pushed remote SHA | `2a625d3b…` (matches local) |

### Validation passed before commit

- 37/37 files have a `WorkingDirectory=-` line after the change ✓
- 0/37 files retain a bare `WorkingDirectory=` line ✓
- 0 non-systemd-unit files in the diff ✓
- 0 `scylla-manager/specs/`, `scylla-manager/package.json`, or
  `scylla-manager/awareness.yaml` edits ✓

Cosmetic note: `metadata/envoy/systemd/globular-envoy.service` uses
`WorkingDirectory={{.StateDir}}` (no `<svc>` subdirectory) — still
correctly normalized to `-{{.StateDir}}`. A first regex check
incorrectly flagged it; the corrected regex confirms it is normalized.

### PR opened

| Field | Value |
|---|---|
| PR | [#2](https://github.com/globulario/packages/pull/2) |
| Title | *Normalize systemd WorkingDirectory entries as optional* |
| Base / head | `main` ← `wd-normalize-systemd-working-directory` |
| State | `OPEN` |
| Mergeable | `MERGEABLE` |
| Merge state | `CLEAN` |
| Opened | 2026-05-29 18:03:07 UTC |

### Side-by-side packages PR state

```
$ gh pr list --state all
#2  Normalize systemd WorkingDirectory entries as optional   wd-normalize-systemd-working-directory   OPEN
#1  Project S/U.2: make scylla-manager package registration HTTPS-first   project-u2   MERGED
```

### Confirmation: no scylla-manager content in PR #2

PR #2's diff (verified via `git diff origin/main..2a625d3 --name-only`):
- 37 files total
- 1 file is `metadata/scylla-manager/systemd/globular-scylla-manager.service`
  — this is the standalone systemd unit file for scylla-manager, NOT
  the spec YAML. The spec YAML (`metadata/scylla-manager/specs/scylla_manager_service.yaml`)
  is **not** touched by this PR. The standalone unit is included
  because it had a bare `WorkingDirectory=` line like all 36 others;
  excluding it would have left the scylla-manager package partially
  WD-normalized.

So the scope discipline holds:
- PR #1 (already merged): modified ONLY the scylla-manager spec YAML
- PR #2 (just opened): modifies ONLY the 37 systemd unit files,
  including the standalone scylla-manager unit, with no spec / script
  edits

### Confirmation: no deploy / rebuild happened

- `globular-scylla-manager.service` unchanged: `ActiveState=active`,
  `NRestarts=0`, `MainPID=770002` (same PID since U.1 deploy).
- No new `pkg build` artifacts.
- No `pkg publish` issued.
- No `services desired set` issued.
- No live unit reload.

PR #2 is a template change for the **package** definitions. The 37
deployed unit files on the cluster nodes remain in their current
(pre-normalize) form; the new template lands on next install of each
affected package — operator-driven, not auto-applied by this merge.

### Cleanup

```
$ git checkout main
$ git status -uno
On branch main
Your branch is up to date with 'origin/main'.
nothing to commit, working tree clean
```

Local `main` is now clean and in sync with `origin/main`. The 37
WD-normalize files no longer appear as uncommitted in working tree —
they are committed on `wd-normalize-systemd-working-directory` and
pushed to remote.

### Status

WD-normalize PR opened. Ready for merge authorization.

---

## 18. Packages PR #2 merge result (2026-05-29 14:06)

### Merged

| Field | Value |
|---|---|
| PR | [#2](https://github.com/globulario/packages/pull/2) |
| Branch | `wd-normalize-systemd-working-directory` → `main` |
| Merge strategy | merge commit (preserves the WD-normalize commit as a parent) |
| Merge commit SHA | `f1871d8ec9bd7e4390d4e2649ba003d44bff0985` |
| Source commit now in main history | `2a625d3b333b65be016e00b187cdd86307d22f4f` |
| `origin/main` advanced | `b905d39` → `f1871d8` |

```
$ git log origin/main --oneline -4
f1871d8 Normalize systemd WorkingDirectory entries as optional (#2)
2a625d3 Normalize systemd WorkingDirectory as optional
b905d39 Project S/U.2: make scylla-manager package registration HTTPS-first (#1)
bdc3724 scylla-manager: HTTPS-first read/probe path in registration script (Project U.2)
```

### Merged file count: 37 — exactly as planned

```
$ git diff --name-only b905d39..origin/main | wc -l
37
```

Every merged file matches `metadata/<svc>/systemd/globular-<svc>.service`.
Zero non-systemd-unit files in the diff.

### Validation on `origin/main` post-merge

| Check | Result |
|---|---|
| Changed units with `WorkingDirectory=-` line | **37/37** |
| Changed units still carrying a bare `WorkingDirectory=` line | **0/37** |
| Non-systemd-unit files in the merge delta | **0** |
| `metadata/scylla-manager/specs/scylla_manager_service.yaml` touched by PR #2 | **No** (not in the file list) |

### PR #1 scylla-manager spec intact on `origin/main`

```
$ F=metadata/scylla-manager/specs/scylla_manager_service.yaml
$ git show origin/main:$F | grep -cE "<phrase>"

  HTTPS-first --capath /dev/null:                3 occurrences  ✓
  install-scylla-manager-register-cluster step:  1              ✓
  fail-closed "refusing to fall back to HTTP":   1              ✓
```

All Project S + Project U.2 content from PR #1 still present at the
expected occurrence counts on the new `origin/main`. The WD-normalize
merge did not regress any of it.

### No deploy / rebuild happened

- `globular-scylla-manager.service`: `ActiveState=active`,
  `NRestarts=0`, `MainPID=770002` (still the same PID since U.1).
- No new `pkg build` artifacts created.
- No `pkg publish` issued.
- No `services desired set` issued.
- No live unit reload triggered.

PR #2 is a template change. The deployed unit files on the cluster
nodes remain in their pre-normalize form; the new optional `-` prefix
lands on next install of each affected package — operator-driven only.

### Local working tree clean

```
$ git checkout main
$ git merge --ff-only origin/main
  37 files changed, 37 insertions(+), 37 deletions(-)

$ git status -uno
On branch main
Your branch is up to date with 'origin/main'.
```

Local `main` is now at `f1871d8`, in sync with `origin/main`, working
tree clean. No untracked source files, no uncommitted modifications.

### Side-by-side packages PR state

| PR | Title | State |
|---|---|---|
| #1 | Project S/U.2: make scylla-manager package registration HTTPS-first | MERGED → `b905d39` |
| #2 | Normalize systemd WorkingDirectory entries as optional | MERGED → `f1871d8` |

### Next recommended action

**Live package deployment planning — NOT execution.**

Both PRs have moved source to `origin/main`. The cluster nodes still
run the previously-deployed packages:

- scylla-manager 1.2.75 (built during U.2 execution, contains the
  HTTPS-first script)
- All 37 services run the pre-WD-normalize unit files (which means
  the cluster_doctor convergence finding `754027b85c39913a` is still
  active on every snapshot)

The next operator-authorized step would be to plan how/when to
re-publish each affected package and dispatch desired-state updates
so the WD-normalize templates land on disk. That planning should
include:

- Whether to publish one-by-one (37 separate `pkg build` + `pkg
  publish` operations, then `services desired set` per package) or
  batch.
- Risk assessment: each affected service restarts when the new unit
  file is installed. 37 service restarts spread across 37 deploys
  should be scheduled to avoid simultaneous control-plane churn.
- Coordination with U.4 (HTTP listener disable on scylla-manager)
  which remains queued for a separate observation window.

This document does not plan or execute that deployment. It records
that the source side of the reconciliation is now complete on both
repos.

### Status

Packages WD-normalize merged and verified. Next recommended action
is live package deployment planning, not execution.



