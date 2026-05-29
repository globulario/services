# recover/project-t-verifier-entrypoint result

**Date:** 2026-05-29
**Outcome:** single cherry-pick clean, no conflict, validation green.
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

---

## Branch

| Field | Value |
|---|---|
| Branch name | `recover/project-t-verifier-entrypoint` |
| Base | `master` (`068bf1eb`) |
| Original commits cherry-picked | 1 |
| Cherry-pick of | `eadc5690` ("Project T: verifier honors manifest entrypoint via install-time sidecar") |
| New commit SHA | `b7649e34ff94b633d20140108e29e8a38392b3a2` |
| Final HEAD | `b7649e34` |

---

## Files changed vs `master`

5 files, exactly as predicted by the inventory:

```
M  golang/node_agent/node_agent_server/apply_package_release.go
A  golang/node_agent/node_agent_server/installed_binary_path_test.go     (new)
M  golang/node_agent/node_agent_server/installer_api.go
A  golang/versionutil/entrypoint_test.go                                 (new)
M  golang/versionutil/version.go
```

Total: 5 files changed, +360 / −2.

No overlap with `origin/master`'s 8 behind commits or with the prior
two recovery branches. The Project T contribution touches the
node-agent install path and versionutil, both of which are
independent of the controller release_pipeline / release_reconciler
lineage that the upcoming MEDIUM-risk branches will need to navigate.

---

## Validation

### Commands run

| Command | Result |
|---|---|
| `git cherry-pick eadc5690` | clean (no conflict) |
| `go test ./versionutil -count=1` | **ok 0.404s** |
| `go test ./node_agent/node_agent_server -count=1` | **ok 122.467s** |
| `go build ./...` | silent (BUILD OK) |

### Failures

**None.** Cherry-pick clean. Both test packages green. Tree builds.

---

## Git state

### `git status --short` on the recovery branch

```
(no tracked-file modifications)
```

Working tree on the recovery branch is clean. Untracked files are
the conventional `loads/*.md` evidence files and similar — none are
source.

### SHAs (post-recovery, after returning to master)

| Ref | SHA |
|---|---|
| `master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `origin/master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `backup/local-master-before-reconcile-20260529-144649` | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| `recover/v1.2.119-hotfix-chain` | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| `recover/doctor-event-suppression-orphan` | `8194aa2410fdbda70be2abfb7c7198b073b047f3` |
| `recover/project-t-verifier-entrypoint` | `b7649e34ff94b633d20140108e29e8a38392b3a2` |

After validation, the working copy was returned to `master`. The
new recovery branch sits local-only, untouched by any further git
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
- No push to either remote. `origin/master` ref state is unchanged.
- The packages repo was not touched.
- The backup ref is intact at `b19ce3aa`.

---

## Next recommendation

The inventory's suggested merge order continues with LOW-risk
branches before tackling MEDIUM-risk. Of the remaining LOW-risk
candidates, the smallest contained validation surface is:

**`recover/project-j-workflow-checksum`** — 3 commits
(`e723331b`, `ac866992`, `9348560c`). Touches:

- 2 workflow definition YAMLs (`release.apply.infrastructure.yaml`,
  `release.apply.package.yaml`)
- `workflow/engine/actors.go`
- new `workflow/engine/actors_sync_package_state_checksum_test.go`
- awareness docs + 2 new `loads/*.md` evidence files

Validation per the inventory: `go test ./workflow/engine` plus
`go build ./...`. Expected conflict risk: **LOW** — workflow YAMLs +
actors.go are independent of the release_pipeline lineage (Project P
later edits `actors.go` too, but that's a future MEDIUM branch
expected to layer cleanly on top of Project J).

Alternative LOW picks:

- `recover/project-c-d-repository-backfill` (6 commits, all in
  `repository/artifact_verify_rpc.go` — linear evolution in a single
  file)
- `recover/project-k-checksum-backfill-cli` (4 commits, new file under
  `cmd/installed_state_checksum_backfill/`)
- `recover/project-b-self-hosted-proof-writer` (6 commits, includes
  the `ecdca55c` PID-anchor and `93118c05` allowlist follow-ups)
- `recover/project-e-minio-inventory` (2 commits, docs only — zero
  source change)

This document does not authorize any. The operator's next-turn
instruction selects the next branch.

---

## Stop

Recovery and validation complete for
`recover/project-t-verifier-entrypoint`. PR not opened (per
instruction). Branch sits on the local checkout at `b7649e34`;
backup ref preserved at `b19ce3aa`.
