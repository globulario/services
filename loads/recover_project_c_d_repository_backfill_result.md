# recover/project-c-d-repository-backfill result

**Date:** 2026-05-29
**Outcome:** all 6 cherry-picks clean, no conflict, validation green.
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

---

## Branch

| Field | Value |
|---|---|
| Branch name | `recover/project-c-d-repository-backfill` |
| Base | `master` (`068bf1eb`) |
| Original commits cherry-picked | 6 (in inventory order) |
| Final HEAD | `6436d6e8d1f5ab9c163dcdefd873ed03a8eed783` |

Commit chain (oldest → newest on the recovery branch):

| Order | Original SHA | New SHA | Subject |
|---:|---|---|---|
| 1 | `2a051583` | `7709f716` | Project C inventory: 15 IRs with broken Scylla manifest index |
| 2 | `efa82071` | `addca458` | repository: RepairArtifact backfills Scylla index when integrity OK |
| 3 | `931318db` | `4a661bfb` | repository: RepairArtifact handles Scylla NULL manifest_json by reading CAS file |
| 4 | `6a5bd635` | `2879b8d2` | repository: backfill bypasses state-machine via direct repository writes |
| 5 | `248857dd` | `4027a2d9` | repository: backfill fires when manifest_json is NULL even if artifact_state=PUBLISHED |
| 6 | `8d3fadc6` | `6436d6e8` | Project D result: all 15 manifest rows backfilled via repository-owned writes |

---

## Files changed vs `master`

7 files. Exactly as the inventory predicted: single-file linear
evolution on `artifact_verify_rpc.go` + the surrounding inventory /
plan / result evidence under `loads/`. **No awareness YAML
modifications** in this branch (confirmed — the only YAML files
touched by Project J's deferred closure are not in this branch's
diff).

```
M  golang/repository/repository_server/artifact_verify_rpc.go      (4 successive linear edits)
A  loads/missing_published_artifacts_inventory.md                  (from Project C inventory)
A  loads/missing_published_artifacts_matrix.tsv                    (from Project C inventory)
A  loads/repository_manifest_backfill_plan.md                      (from first backfill code commit)
A  loads/project_d_repository_bridge_audit.md                      (from Project D result)
A  loads/repository_manifest_backfill_matrix.tsv                   (from Project D result)
A  loads/repository_manifest_backfill_result.md                    (from Project D result)
```

The single touched source file (`artifact_verify_rpc.go`) accumulated
the deltas across 4 commits without conflict because each successive
commit was authored against the prior one's output — exactly the
linear-evolution shape the inventory called out.

---

## Validation

### Commands run

| Command | Result |
|---|---|
| `git cherry-pick 2a051583` | clean |
| `git cherry-pick efa82071` | clean |
| `git cherry-pick 931318db` | clean |
| `git cherry-pick 6a5bd635` | clean |
| `git cherry-pick 248857dd` | clean |
| `git cherry-pick 8d3fadc6` | clean |
| `go test ./repository/repository_server -count=1` | **ok 2.125s** |
| `go build ./...` | silent (BUILD OK) |

### Failures

**None.** All 6 cherry-picks applied without conflict; the test
package passes; the tree builds.

---

## Git state

### `git status --short` on the recovery branch (post-cherry-picks)

```
(no tracked-file modifications)
```

Working tree on the recovery branch is clean. Untracked files are the
conventional `loads/*.md` evidence files and similar — none are
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

This `recover/project-c-d-repository-backfill` branch does not touch
the awareness YAMLs and therefore creates no new dependency entry
for the deferred commit. The dependency picture is unchanged.

---

## Next recommendation

The remaining LOW-risk branches are:

**`recover/project-k-checksum-backfill-cli`** — 4 commits
(`27ab5d0e` inventory, `756a6522` initial CLI, `2fffb4d6` repair-mode
follow-up, `76d16734` result). All code changes are to a new file
under `golang/cmd/installed_state_checksum_backfill/main.go` — **no
overlap** with any existing source or with any other recovery branch.
The result commit (`76d16734`) is docs-only and was already in the
inventory marked as touching only `loads/*.md` (it should not touch
the awareness YAMLs — verifiable before cherry-pick).

Validation per the inventory: `go build
./cmd/installed_state_checksum_backfill` plus the full `go build ./...`
sanity check. Expected conflict risk: **LOW** — new-file-only
contribution, pure additive.

If the operator prefers an even smaller scope: `recover/project-e-minio-inventory`
(2 commits, docs only, zero source change — smallest possible
surface).

If the operator prefers the largest remaining LOW-risk branch to take
out the next biggest pile: `recover/project-b-self-hosted-proof-writer`
(6 commits including the `ecdca55c` PID-anchor and `93118c05`
allowlist follow-ups). Validation: `go test
./node_agent/node_agent_server -count=1` plus `go build ./...`.
Watch for the same awareness-YAML pattern in any of those 6 — Project
B's `a6af5d8f` does touch the awareness docs, so its result/closure
commits may exhibit the same dependency that bit Project J's closure.

This document does not authorize any. The operator's next-turn
instruction selects the next branch.

---

## Stop

Recovery and validation complete for
`recover/project-c-d-repository-backfill`. PR not opened (per
instruction). Branch sits on the local checkout at `6436d6e8`;
backup ref preserved at `b19ce3aa`.
