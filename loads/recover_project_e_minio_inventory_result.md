# recover/project-e-minio-inventory result

**Date:** 2026-05-29
**Outcome:** both cherry-picks clean, sanity build green.
Recovery branch is **local-only** — no push performed.

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

---

## Branch

| Field | Value |
|---|---|
| Branch name | `recover/project-e-minio-inventory` |
| Base | `master` (`068bf1eb`) |
| Original commits cherry-picked | 2 (in inventory order) |
| Final HEAD | `8365e180e5bd0f753395573839a11fcab1ce5d75` |

Commit chain (oldest → newest on the recovery branch):

| Order | Original SHA | New SHA | Subject |
|---:|---|---|---|
| 1 | `48700467` | `a42216f2` | Project E inventory: MinIO running healthy; release label stale + path mismatch |
| 2 | `2383835a` | `8365e180` | Project E2: corrected MinIO path inventory — contract path IS active |

---

## Files changed vs `master`

4 files, all additions under `loads/`. **Zero source modifications.**
**Docs/evidence-only branch — confirmed.**

```
A  loads/minio_runtime_recovery_impact.md            (from Project E inventory)
A  loads/minio_runtime_recovery_matrix.tsv           (from Project E inventory)
A  loads/minio_objectstore_path_matrix.tsv           (from Project E2 correction)
A  loads/minio_objectstore_path_reconciliation_plan.md  (from Project E2 correction)
```

No `golang/`, no `docs/awareness/`, no `docs/intent/`, no `metadata/` —
nothing outside `loads/`. The smallest-surface contribution in the
LOW-risk pool, exactly as advertised.

---

## Validation

### Commands run

| Command | Result |
|---|---|
| `git cherry-pick 48700467` | clean |
| `git cherry-pick 2383835a` | clean |
| `go build ./...` | silent (BUILD OK) |

The sanity build was the appropriate gate here per the inventory —
no source files were modified, so no test package is exercised by
this branch's contribution.

### Failures

**None.** Both cherry-picks applied without conflict; the tree builds.

---

## Git state

### `git status --short` on the recovery branch (post-cherry-picks)

```
(no tracked-file modifications)
```

Working tree on the recovery branch is clean. Untracked files are
the conventional `loads/*.md` evidence files and similar — none are
source. No cherry-pick in progress.

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

`9348560c` ("Project J closure: awareness records + result report")
**remains deferred** until the awareness-YAML dependency chain lands
on `master`. Reminder of the chain needed before re-cherry-pick:

1. `recover/v1.2.119-hotfix-chain` lands (4 commits, all touch
   `docs/awareness/*.yaml`)
2. Project A series (A2 `4dc2fb38`, A3 `a8d9c43a`, A4 `d03ec5f1`,
   A5 `f5faae69`) lands
3. Project B's `a6af5d8f` lands

After those are on `master`, re-cherry-pick `9348560c`. The
originally-tagged `loads/convergence_committer_checksum_preservation_result.md`
(~190 lines) ships with that re-cherry-pick — it is **NOT** on the
current `recover/project-j-workflow-checksum` branch.

This `recover/project-e-minio-inventory` branch does not touch the
awareness YAMLs and therefore creates no new dependency entry for
the deferred commit. The dependency picture is unchanged.

---

## Next recommendation

The remaining LOW-risk pool is largely emptied; the next candidates
need awareness-YAML pre-checks. Of the remaining recovery branches,
the cleanest next pick is:

### Primary recommendation

**`recover/project-b-self-hosted-proof-writer`** — 6 commits
(`70f8871d`, `14fbbc50`, `a6af5d8f`, `d83e0359`, `93118c05`,
`ecdca55c`). Touches `node_agent/heartbeat.go`,
`self_hosted_runtime_proof_writer.go` (created here + later
modified), and the awareness YAMLs in commit `a6af5d8f`.

### Awareness YAML pre-check — `a6af5d8f` will conflict

Pre-check performed before this report (read-only inspection of the
commit on the backup ref):

```
a6af5d8f appends to:
  docs/awareness/failure_modes.yaml — hunk @@ -3099,3 +3099,87 @@ failure_modes:
  docs/awareness/invariants.yaml    — hunk @@ -4582,3 +4582,67 @@ invariants:

current origin/master file lengths:
  docs/awareness/failure_modes.yaml: 2578 lines
  docs/awareness/invariants.yaml:    4061 lines
```

`a6af5d8f` was authored against a base where `failure_modes.yaml` had
≥ 3099 lines (master is 521 lines short) and `invariants.yaml` had ≥
4582 lines (master is 521 lines short). **Both files are too short
on current master for `a6af5d8f`'s patch to apply.** This is the
same structural condition that bit Project J's closure (`9348560c`).

`a6af5d8f` will conflict on cherry-pick. Expected exit: cherry-pick
in progress, two `UU` files, the staged source change in
`node_agent/heartbeat.go` waiting alongside.

### Recommended path for Project B

When the operator authorizes Project B, plan for the same
skip-pattern Project J used:

```
Order of cherry-picks (suggested):
  1. 70f8871d  (Project B impact report + matrix — docs only, applies cleanly)
  2. 14fbbc50  (node-agent: proof writer — code, applies cleanly)
  3. a6af5d8f  ← will conflict. Authorize `git cherry-pick --skip` to defer.
  4. d83e0359  (Project B result — likely docs/awareness; will also conflict if a6af5d8f deferred — recheck via the same `git show` pattern)
  5. 93118c05  (node-agent: allowlist — code, applies cleanly)
  6. ecdca55c  (node-agent: PID-start anchor — code, applies cleanly)
```

The deferred set grows to **two** awareness-YAML-dependent commits:
`9348560c` (Project J closure) and `a6af5d8f` (Project B awareness
update). Both will re-cherry-pick cleanly once
`recover/v1.2.119-hotfix-chain` and the Project A series land on
`master`.

### Alternative if the operator wants to clear awareness dependencies first

**Skip Project B for now** and land the awareness-prerequisite chain
via PRs: `recover/v1.2.119-hotfix-chain` first (it is the foundational
contribution that adds the early failure-mode/invariant entries), then
the Project A series (A2 → A5) as its own recovery branch.

Once those PRs are merged to `origin/master`, both `a6af5d8f` and
`9348560c` will apply cleanly on a re-cherry-pick.

### Other remaining branches (MEDIUM-risk only)

After Project B, only MEDIUM-risk branches remain in the recovery pool:

- `recover/project-a-awareness-bundle-identity` (15 commits — the
  largest chain)
- `recover/project-f-minio-drift-recovery` (2 commits)
- `recover/project-n-wave-blocked-retry` (1 commit)
- `recover/project-p-infra-remove-phase` (1 commit)
- `recover/project-q-infra-spec-paused` (1 commit)
- `recover/project-l-systemd-sidecar` (1 commit — HIGH risk, conflict on `services_cmds.go`)

All of these can wait until the LOW-risk branches are reviewed and
merged.

This document does not authorize Project B or any other branch.
The operator's next-turn instruction selects the next branch.

---

## Stop

Recovery and validation complete for
`recover/project-e-minio-inventory`. PR not opened (per instruction).
Branch sits on the local checkout at `8365e180`; backup ref preserved
at `b19ce3aa`.
