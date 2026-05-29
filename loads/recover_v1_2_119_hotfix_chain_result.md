# recover/v1.2.119-hotfix-chain result

**Date:** 2026-05-29
**Outcome:** all 4 cherry-picks clean, no conflict, validation green.
The recovery branch is **local-only** — no push performed.

---

## Base

| Field | Value |
|---|---|
| `master` SHA before branching | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `origin/master` SHA | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| backup ref SHA | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| backup branch name | `backup/local-master-before-reconcile-20260529-144649` |

---

## Branch

| Field | Value |
|---|---|
| Branch name | `recover/v1.2.119-hotfix-chain` |
| Base | `master` (`068bf1eb`) |
| Final HEAD | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| Commits added | 4 |

Commit chain (oldest → newest on the recovery branch):

| Order | Original SHA | Cherry-pick SHA | Subject |
|---:|---|---|---|
| 1 | `c185abde` | `b894b2e2` | controller: ApplyPackageRelease dispatch must carry manifest entrypoint_checksum (v1.2.119 draft) |
| 2 | `f061d334` | `34e1f13d` | awareness: workflow definitions on disk must thread verification fields the controller expects (v1.2.119 follow-up) |
| 3 | `23a89318` | `e69b8f20` | awareness: etcd NOSPACE blocks reconciliation at the persistence layer (separate from v1.2.119 chain) |
| 4 | `6e8d01a2` | `b2cb8ad2` | node-agent: install-package must not alias desired_hash into ExpectedSha256 (v1.2.119 hash-schema regression fix) |

(Cherry-pick re-authoring is expected — SHAs differ from the originals
because the parent chain changed. Content equivalence is what we
preserve, not SHA.)

---

## Files changed vs `master`

26 files total. Mix of awareness docs, intent yaml, controller +
workflow + node-agent + globularcli source, and inventory/matrix
load files.

```
M  docs/awareness/failure_modes.yaml
M  docs/awareness/invariants.yaml
A  docs/intent/controller.apply_package_release_must_carry_expected_sha256.yaml
M  golang/cluster_controller/cluster_controller_server/deploy_control_plane.go
A  golang/cluster_controller/cluster_controller_server/dispatch_expected_sha256_test.go
M  golang/cluster_controller/cluster_controller_server/reconcile_actions.go
M  golang/cluster_controller/cluster_controller_server/reconcile_runtime.go
M  golang/cluster_controller/cluster_controller_server/recovery_workflow.go
M  golang/cluster_controller/cluster_controller_server/release_pipeline.go
M  golang/cluster_controller/cluster_controller_server/repair_node_workflow.go
M  golang/cluster_controller/cluster_controller_server/workflow_controller_deploy.go
M  golang/cluster_controller/cluster_controller_server/workflow_release.go
M  golang/globularcli/state_cmds.go
M  golang/node_agent/node_agent_server/grpc_workflow.go
A  golang/node_agent/node_agent_server/grpc_workflow_hash_schema_test.go
M  golang/workflow/definitions/release.apply.controller.yaml
M  golang/workflow/definitions/release.apply.package.yaml
M  golang/workflow/engine/actors.go
M  golang/workflow/engine/actors_controller_deploy.go
A  golang/workflow/engine/actors_expected_sha256_test.go
M  golang/workflow/engine/foreach_substeps_test.go
M  golang/workflow/engine/release_infra_test.go
M  golang/workflow/engine/release_package_test.go
A  loads/package_hash_mismatch_inventory.md
A  loads/package_hash_mismatch_matrix.tsv
```

---

## Validation

### Commands run

| Command | Result |
|---|---|
| `git cherry-pick c185abde` | clean (no conflict) |
| `git cherry-pick f061d334` | clean (no conflict) |
| `git cherry-pick 23a89318` | clean (no conflict) |
| `git cherry-pick 6e8d01a2` | clean (no conflict) |
| `go build ./...` | silent (BUILD OK) |
| `go test ./cluster_controller/cluster_controller_server -count=1` | **ok 8.982s** |
| `go test ./node_agent/node_agent_server -count=1` | **ok 128.493s** |

### Failures

**None.** All cherry-picks applied without conflict and every test
package passes.

---

## Git state

### `git status --short`

```
(no tracked-file modifications)
```

The working tree on the recovery branch is clean. Untracked files are
the conventional `loads/*.md` evidence files and similar — none are
source.

### Untracked evidence files

The pre-existing `loads/*.md` evidence files remain untracked, exactly
as they were before this turn. None were created, modified, or
deleted by this work.

### Source changes vs `master`

The 26 files listed in "Files changed vs `master`" above. All are
contributions of the 4 cherry-picked commits; no additional drift.

---

## Runtime safety

**No runtime mutation performed.** Confirmation:

- `globular-scylla-manager.service`: `ActiveState=active`,
  `NRestarts=0`, `MainPID=770002` (same PID since U.1).
- No `pkg build`, no `pkg publish`, no `services desired set`, no
  service restart.
- No package was published, no deployment dispatched, no live
  config changed.

This recovery branch is **local-only** at this point. No push has
been performed; `origin/master` and the remote `add-license-1`
remain at their pre-turn SHAs.

---

## Next recommendation

The inventory's suggested merge order calls for landing the smallest
LOW-risk branches next. The smallest remaining LOW-risk branch is:

**`recover/doctor-event-suppression-orphan`** — 1 commit
(`9d1e36e5`, "cluster_doctor: stop emitting spurious finding.created/resolved
events on stable state"). Touches:

- `golang/cluster_doctor/cluster_doctor_server/server.go`
- `golang/cluster_doctor/cluster_doctor_server/workflow_runner.go`
- new `golang/cluster_doctor/cluster_doctor_server/cache_findings_scope_test.go`

Validation: `go test ./cluster_doctor/cluster_doctor_server` plus
`go build ./...`. Expected conflict risk: LOW (orphan commit; no
file overlap with origin/master or with `recover/v1.2.119-hotfix-chain`).

Alternative pick if the operator prefers the cleanest
"only-test-coverage" addition: **`recover/project-t-verifier-entrypoint`**
— 1 commit (`eadc5690`), independent of all release_pipeline
lineage, all new tests + small modifications to versionutil and node_agent
installer paths.

This document does not authorize either; the operator's next-turn
instruction selects the next branch.

---

## Stop

Recovery and validation complete for `recover/v1.2.119-hotfix-chain`.
PR not opened (per instruction). Branch sits on the local checkout
at `b2cb8ad2`; backup ref preserved at `b19ce3aa`.
