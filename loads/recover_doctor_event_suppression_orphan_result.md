# recover/doctor-event-suppression-orphan result

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
| previous recovery branch | `recover/v1.2.119-hotfix-chain` |
| previous recovery branch SHA | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |

---

## Branch

| Field | Value |
|---|---|
| Branch name | `recover/doctor-event-suppression-orphan` |
| Base | `master` (`068bf1eb`) |
| Original commits cherry-picked | 1 |
| Cherry-pick of | `9d1e36e5` ("cluster_doctor: stop emitting spurious finding.created/resolved events on stable state") |
| New commit SHA | `8194aa2410fdbda70be2abfb7c7198b073b047f3` |
| Final HEAD | `8194aa24` |

---

## Files changed vs `master`

3 files. All in the `cluster_doctor/cluster_doctor_server/` package
as expected from the inventory:

```
A  golang/cluster_doctor/cluster_doctor_server/cache_findings_scope_test.go   (new test file)
M  golang/cluster_doctor/cluster_doctor_server/server.go                       (event suppression in stable-state cache scope)
M  golang/cluster_doctor/cluster_doctor_server/workflow_runner.go              (related stable-state handling)
```

Total: 3 files changed, +181 / -17.

No overlap with `origin/master`'s 8 behind commits or with the prior
`recover/v1.2.119-hotfix-chain` branch. Confirmed orphan, isolated.

---

## Validation

### Commands run

| Command | Result |
|---|---|
| `git cherry-pick 9d1e36e5` | clean (no conflict) |
| `go test ./cluster_doctor/cluster_doctor_server -count=1` | **ok 0.204s** |
| `go build ./...` | silent (BUILD OK) |

### Failures

**None.** Cherry-pick clean. Test package green. Tree builds.

---

## Git state

### `git status --short` on the recovery branch

```
(no tracked-file modifications)
```

Working tree on the recovery branch is clean. Untracked files are
the conventional `loads/*.md` evidence files and similar — none are
source.

### SHAs

| Ref | SHA |
|---|---|
| `master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `origin/master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `backup/local-master-before-reconcile-20260529-144649` | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| `recover/doctor-event-suppression-orphan` | `8194aa2410fdbda70be2abfb7c7198b073b047f3` |

After validation, the working copy was returned to `master`. The
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

The MainPID `770002` is the same value scylla-manager has held since
the U.1 deploy — no restart, no re-spawn, no reload during this
recovery.

- No `pkg build`, no `pkg publish`, no `services desired set`, no
  service restart.
- No package was published, no deployment dispatched, no live config
  changed.
- No push to either remote. `origin/master`'s ref state is unchanged.
- The packages repo was not touched in any way.
- The backup ref is intact at `b19ce3aa`.

---

## Next recommendation

The inventory's suggested merge order continues with LOW-risk single
or small-batch branches. Smallest remaining LOW-risk branch by
commit count and validation surface:

**`recover/project-t-verifier-entrypoint`** — 1 commit
(`eadc5690`, "Project T: verifier honors manifest entrypoint via
install-time sidecar"). Touches:

- `golang/node_agent/node_agent_server/apply_package_release.go`
- `golang/node_agent/node_agent_server/installed_binary_path_test.go` (new)
- `golang/node_agent/node_agent_server/installer_api.go`
- `golang/versionutil/entrypoint_test.go` (new)
- `golang/versionutil/version.go`

Validation per the inventory: `go test ./versionutil
./node_agent/node_agent_server` plus `go build ./...`. Expected
conflict risk: **LOW** — independent of all the release_pipeline /
release_reconciler lineage; node_agent's installer + versionutil
modifications were authored against a base that did not yet have
Project O's `state.go` migration (Project O is on origin), but those
specific files in Project T's diff are different paths from Project
O's footprint, so no overlap expected.

Alternative LOW pick if the operator wants an even smaller scope: any
of `recover/project-c-d-repository-backfill` (6 commits, all in
`repository/artifact_verify_rpc.go`), `recover/project-j-workflow-checksum`
(3 commits), or `recover/project-k-checksum-backfill-cli` (4 commits,
new file under `cmd/installed_state_checksum_backfill/`).

This document does not authorize either; the operator's next-turn
instruction selects the next branch.

---

## Stop

Recovery and validation complete for
`recover/doctor-event-suppression-orphan`. PR not opened (per
instruction). Branch sits on the local checkout at `8194aa24`;
backup ref preserved at `b19ce3aa`.
