# recover/project-k-checksum-backfill-cli result

**Date:** 2026-05-29
**Outcome:** all 4 cherry-picks clean, no conflict, validation green.
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

---

## Branch

| Field | Value |
|---|---|
| Branch name | `recover/project-k-checksum-backfill-cli` |
| Base | `master` (`068bf1eb`) |
| Original commits cherry-picked | 4 (in inventory order) |
| Final HEAD | `12cc1b4eea09f0d2ab34275d23f7d8e72e911650` |

Commit chain (oldest → newest on the recovery branch):

| Order | Original SHA | New SHA | Subject |
|---:|---|---|---|
| 1 | `27ab5d0e` | `b69980fe` | Project K inventory: checksum-backfill scope and safety predicate |
| 2 | `756a6522` | `026b180d` | installed_state_checksum_backfill: Project K Phase 1 / 2 / 3 CLI tool |
| 3 | `2fffb4d6` | `c76a87dc` | installed_state_checksum_backfill: do not bump UpdatedUnix; add repair mode |
| 4 | `76d16734` | `12cc1b4e` | Project K result: 35 of 47 records backfilled across phases 1-3 |

---

## Files changed vs `master`

4 files, all additions. **Zero modifications to existing source** —
exactly the pure-additive shape the inventory predicted:

```
A  golang/cmd/installed_state_checksum_backfill/main.go       (new CLI tool — Phase 1/2/3 backfill logic + repair mode)
A  loads/checksum_backfill_inventory_impact.md                (from Project K inventory)
A  loads/checksum_backfill_inventory_matrix.tsv               (from Project K inventory)
A  loads/checksum_backfill_result.md                          (from Project K result)
```

The CLI source file (`main.go`) was created by the 2nd commit and
then modified by the 3rd commit (repair-mode + UpdatedUnix change).
No file other than this new CLI's `main.go` is touched by any
source commit. **No awareness YAML modifications** — confirming the
inventory's prediction that this branch creates no new dependency
entries.

---

## Validation

### Commands run

| Command | Result |
|---|---|
| `git cherry-pick 27ab5d0e` | clean |
| `git cherry-pick 756a6522` | clean |
| `git cherry-pick 2fffb4d6` | clean |
| `git cherry-pick 76d16734` | clean |
| `go build ./cmd/installed_state_checksum_backfill` | silent (BUILD OK) |
| `go build ./...` | silent (BUILD OK) |

### Failures

**None.** All 4 cherry-picks applied without conflict; the new CLI
builds; the full tree builds.

The inventory listed `go build` (not `go test`) for this branch
because the new CLI tool has no companion test file in any of the 4
commits; the build check is the appropriate validation gate.

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
U.1 deploy — no restart, no re-spawn, no reload during this recovery.

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

This `recover/project-k-checksum-backfill-cli` branch does not touch
the awareness YAMLs and therefore creates no new dependency entry
for the deferred commit. The dependency picture is unchanged.

---

## Next recommendation

The remaining LOW-risk branches are:

**`recover/project-e-minio-inventory`** — 2 commits (`48700467`
inventory, `2383835a` E2 correction). Docs-only contribution; **zero
source modifications**; the smallest possible surface left in the
LOW-risk pool. Both commits add files under `loads/` only:

- `loads/minio_runtime_recovery_impact.md`
- `loads/minio_runtime_recovery_matrix.tsv`
- `loads/minio_objectstore_path_matrix.tsv`
- `loads/minio_objectstore_path_reconciliation_plan.md`

Validation: no source change → `go build ./...` for sanity only
(expected silent). Expected conflict risk: **LOW** — files in `loads/`
are project-evidence and have never collided with origin/master
because none of the PR-merged contributions touched that directory's
namespace.

If the operator prefers the next-largest LOW pile to take out:
**`recover/project-b-self-hosted-proof-writer`** — 6 commits
(`70f8871d`, `14fbbc50`, `a6af5d8f`, `d83e0359`, `93118c05`,
`ecdca55c`). Validation: `go test ./node_agent/node_agent_server
-count=1` plus `go build ./...`. **Warning:** `a6af5d8f` modifies
the awareness YAMLs (`docs/awareness/failure_modes.yaml` and
`docs/awareness/invariants.yaml`). It's the FIRST awareness-YAML
modification in the recovery chain since the v1.2.119 hotfix chain
hasn't landed yet either — so `a6af5d8f`'s YAML additions may apply
cleanly against the current `master` state (since both are pure
append-style additions to empty/end-of-file positions), but a
defensive pre-pick check is warranted:

```bash
# Pre-check before authorizing recover/project-b:
git show a6af5d8f -- docs/awareness/failure_modes.yaml | head -20
# verify the patch context lines match current master's file structure
```

If `a6af5d8f`'s context is incompatible, Project B can still be
landed by skipping `a6af5d8f` like Project J's closure was skipped,
keeping the source code (`14fbbc50`, `93118c05`, `ecdca55c`) but
deferring the awareness update.

This document does not authorize any. The operator's next-turn
instruction selects the next branch.

---

## Stop

Recovery and validation complete for
`recover/project-k-checksum-backfill-cli`. PR not opened (per
instruction). Branch sits on the local checkout at `12cc1b4e`;
backup ref preserved at `b19ce3aa`.
