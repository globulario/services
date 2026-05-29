# recover/project-j-workflow-checksum result

**Date:** 2026-05-29
**Outcome:** 2 of 3 cherry-picks clean; 3rd skipped due to awareness-YAML
dependency on un-merged prior projects. Recovery branch is
**local-only** with the code portion of Project J intact; the
closure/report commit deferred per operator decision.

---

## Conflict decision

| Field | Value |
|---|---|
| Conflicting commit skipped | `9348560c6dc77a205db45d3fa9d4f8b05ce72a1e` ("Project J closure: awareness records + result report") |
| Reason | depends on cumulative awareness YAML additions from un-merged prior projects (v1.2.119 chain, A series, B). Project J's code (commits 1+2) is independent and applies cleanly; the closure (commit 3) is a doc-only append that requires its base YAML structure to match. |
| Conflicted files | `docs/awareness/failure_modes.yaml` (HEAD lines 2579 ↔ 9348560c lines 2580–3245), `docs/awareness/invariants.yaml` (HEAD lines 4062 ↔ 9348560c lines 4063–4703) |
| Closure/report commit deferred until | the prior awareness-YAML projects land on `master` — at minimum `recover/v1.2.119-hotfix-chain` plus the Project A series (A2/A3/A4/A5) and Project B. After those land, `9348560c` is expected to apply cleanly. |
| Skip mechanism | `git cherry-pick --skip` (no manual file edits, no `--abort`, no resolution attempt) |

---

## Branch

| Field | Value |
|---|---|
| Branch name | `recover/project-j-workflow-checksum` |
| Base | `master` (`068bf1eb`) |
| Included original commits | `e723331b` (inventory), `ac866992` (workflow code) |
| Skipped original commits | `9348560c` (closure/awareness) — deferred |
| New commit SHAs | `2bc831c0` (cherry-pick of `e723331b`), `98bc84db` (cherry-pick of `ac866992`) |
| Final HEAD | `98bc84db69758a4cfd4873ca21cca9aa32c28b00` |

Commits ahead of master:

```
98bc84db  workflow: nodeSyncPackageState writes manifest entrypoint_checksum, not desired_hash
2bc831c0  Project J inventory: convergence-committer writes desired_hash into Checksum field
```

---

## Files changed vs `master`

6 files, exactly the surface of the 2 included commits — no leakage
from the skipped 3rd commit:

```
M  golang/workflow/definitions/release.apply.infrastructure.yaml
M  golang/workflow/definitions/release.apply.package.yaml
M  golang/workflow/engine/actors.go
A  golang/workflow/engine/actors_sync_package_state_checksum_test.go     (new)
A  loads/convergence_committer_checksum_preservation_impact.md            (new, from e723331b)
A  loads/convergence_committer_checksum_preservation_matrix.tsv           (new, from e723331b)
```

`loads/convergence_committer_checksum_preservation_result.md` (which
the 3rd commit would have added) is **NOT** present — confirmed via
`git diff --name-status master..HEAD`.

The awareness docs (`docs/awareness/failure_modes.yaml` and
`docs/awareness/invariants.yaml`) are **unchanged on this branch**.
The skip discarded both the YAML deltas and the staged result MD.

---

## Validation

### Commands run

| Command | Result |
|---|---|
| `git cherry-pick e723331b` | clean (commit 1 of 3 succeeded) |
| `git cherry-pick ac866992` | clean (commit 2 of 3 succeeded) |
| `git cherry-pick 9348560c` | **CONFLICT** in 2 awareness YAMLs |
| `git cherry-pick --skip` | clean (cherry-pick sequence terminated) |
| `go test ./workflow/engine -count=1` | **ok 55.624s** |
| `go build ./...` | silent (BUILD OK) |

### Failures

**None** in the included surface. The skipped commit is a deliberate
deferral, not a failure of the code portion.

---

## Git state

### `git status --short` (post-skip, before checkout)

```
(no tracked-file modifications)
```

### Cherry-pick in progress

**No.** `test ! -f .git/CHERRY_PICK_HEAD && echo "no cherry-pick in
progress"` returned the success branch. The cherry-pick sequence is
fully resolved.

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

After validation, working copy returned to `master`. All recovery
branches sit local-only, untouched by any subsequent git operation.

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
- No push to either remote. `origin/master` ref state unchanged.
- The packages repo was not touched.
- The backup ref is intact at `b19ce3aa`.

---

## Deferred follow-up

`9348560c` ("Project J closure: awareness records + result report")
**must be retried later** after the awareness-YAML dependency chain
lands on `master`. Concretely:

1. After `recover/v1.2.119-hotfix-chain`'s 4 commits merge to `master`
   (each of those modifies the same two YAMLs).
2. After the Project A series (A2 `4dc2fb38`, A3 `a8d9c43a`, A4
   `d03ec5f1`, A5 `f5faae69`) lands — each adds awareness YAML
   entries.
3. After Project B's `a6af5d8f` lands (its awareness updates).

Once those YAML modifications are on `master`, re-cherry-picking
`9348560c` should apply without conflict because the file structure
will then match the base that `9348560c` was authored against.

If `9348560c` still conflicts after the dependency chain is in place,
the issue is order-sensitivity within the awareness YAMLs (e.g.
different append positions); manual append-resolution is the
expected remediation, since each side is purely additive content.

**Tracking note:** the originally-tagged result document
(`loads/convergence_committer_checksum_preservation_result.md`,
+~190 lines per the merge inventory) is also part of `9348560c` — it
is **NOT** on the current recovery branch and will arrive with the
re-cherry-pick.

---

## Inventory correction

The classification in `loads/local_services_backup_commit_inventory.md`
listed `recover/project-j-workflow-checksum` as overall LOW risk.
This was accurate for the **code** portion (commits 1 + 2) but
overlooked the cumulative awareness-YAML dependency in the **closure**
(commit 3). The branch as actually shipped is LOW risk; the deferred
closure commit is more accurately classified as **NEEDS_AWARENESS_CHAIN**
— neither dangerous nor experimental, but blocked until prior
awareness changes land.

The inventory will be informally updated as future recovery turns
proceed; no edit to the inventory file is performed in this turn
(read-only per the planning discipline).

---

## Next recommendation

The remaining LOW-risk branches with the simplest surface and minimal
file-level dependencies are:

**`recover/project-c-d-repository-backfill`** — 6 commits (`2a051583`
inventory, `efa82071` + `931318db` + `6a5bd635` + `248857dd` code,
`8d3fadc6` result). All code changes are linear modifications to a
single file: `golang/repository/repository_server/artifact_verify_rpc.go`.
The result commit (`8d3fadc6`) adds 3 new evidence files but **does
not** touch the awareness YAMLs (verified during the inventory
write-up), so it should apply cleanly.

Validation per the inventory: `go test
./repository/repository_server -count=1` plus `go build ./...`.
Expected conflict risk: **LOW** — single-file linear chain on
`artifact_verify_rpc.go`, no overlap with `master` and no awareness
YAML modifications.

If the operator wants to further reduce risk for the next turn, even
smaller candidates:

- `recover/project-k-checksum-backfill-cli` (4 commits, all in a new
  `cmd/installed_state_checksum_backfill/` directory — pure additive)
- `recover/project-e-minio-inventory` (2 commits, docs only — zero
  source change, smallest possible surface)

This document does not authorize any. The operator's next-turn
instruction selects the next branch.

---

## Stop

Skip + validation complete for `recover/project-j-workflow-checksum`.
Branch sits at `98bc84db` with 2 commits (code only). The deferred
closure commit (`9348560c`) is parked in the dependency pile;
backup ref preserved at `b19ce3aa`. PR not opened (per instruction).
