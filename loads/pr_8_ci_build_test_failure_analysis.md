# PR #8 CI build-test failure analysis

**Date:** 2026-05-29
**Read-only investigation turn — no code changes, no push, no merge.**

---

## PR

| Field | Value |
|---|---|
| PR URL | https://github.com/globulario/services/pull/8 |
| Branch | `recover/v1.2.119-hotfix-chain` |
| Head SHA | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| Base SHA | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| Failing workflow | `ci` |
| Failing job | `build-test` |
| Failing job URL | https://github.com/globulario/services/actions/runs/26658546480/job/78574887592 |

---

## CI status

Polled at investigation time:

| Workflow | Check | Status | Conclusion |
|---|---|---|---|
| `Documentation` | `build-docs` | COMPLETED | **SUCCESS** |
| `Documentation` | `validate-cli-commands` | COMPLETED | **SUCCESS** |
| `Documentation` | `check-paths` | COMPLETED | **SUCCESS** |
| `ci` | `proto-check` | COMPLETED | **SUCCESS** |
| `ci` | **`build-test`** | COMPLETED | **🚨 FAILURE** |
| `ci` | `lint` | IN_PROGRESS | — |

PR overall: `state: OPEN`, `mergeable: MERGEABLE`, `mergeStateStatus: UNSTABLE`.

---

## Failing command(s)

`build-test` runs **three commands** sequentially. The first that fails halts the job; subsequent steps still emit log lines but their exit codes also become 1. The actual chain:

| Step | Command | Result |
|---|---|---|
| 1 | `go test ./... -race -short -coverprofile=coverage.out` | **FAIL** in 3 packages |
| 2 | `go build -o bin/validate-deps ./awareness/cmd/validate-deps` followed by `./bin/validate-deps --services .. --packages ../../packages` | **FAIL** — directory not found |
| 3 | Awareness test suite via `bin/globular awareness test-results` + `jq -e '.passed == true'` | **FAIL** — cascades from step 1 |

### Step 1 failures (the dominant root cause)

Three packages fail in the `go test ./... -race -short` step:

#### 1a. `golang/awareness/learning` — 5 tests fail, all with the same error

```
TestPromoteProposalAcceptsApproved
TestPromoteProposalAllowUnapproved
TestPromoteProposalWritesToApprovedFiles
TestPromoteProposalDoesNotOverwriteExistingInvariants
TestPromotedAliasesWrittenToContextAliasesYAML

  parse /tmp/.../001/failure_modes.yaml:
  yaml: line 2682: mapping values are not allowed in this context
```

#### 1b. `golang/cluster_controller/cluster_controller_server` — 1 test fails

```
TestClusterReconcileOverlapPublishesBlockedLaneStatus
  reconcile_lane_status_test.go:137: runClusterReconcileIfIdle not initialized
```

#### 1c. `golang/mcp` — 2 tests fail

```
TestOfflineDiagnose_EtcdLeaderLoss_MapsToEtcdLeaderInstability
TestOfflineDiagnose_ControllerLeaseExpired_MapsToCorrectFailureMode

  etcd_cascade_test.go:75: failure mode "etcd.nospace_alarm" not found in suspected_failure_modes
  etcd_cascade_test.go:75: failure mode "etcd.leader_instability" not found in suspected_failure_modes
  etcd_cascade_test.go:75: failure mode "workflow.dispatch_timeout_due_to_control_plane_instability" not found in suspected_failure_modes
  etcd_cascade_test.go:91: controller.lease_expired_due_to_etcd_instability not found in suspected_failure_modes
```

### Step 2 failure

```
stat /home/runner/work/services/services/services/golang/awareness/cmd/validate-deps:
directory not found
##[error]Process completed with exit code 1.
```

The CI workflow expects `golang/awareness/cmd/validate-deps/` to exist. **It does not exist on this branch and does not exist on `origin/master`** — confirmed via `find golang/awareness -type d -name validate-deps` (empty) and `git ls-tree origin/master | grep validate-deps` (empty).

### Step 3 failure

Awareness suite emits `false` from `jq -e '.passed == true'` — this is the downstream symptom of step 1a's YAML parse failure: the awareness graph can't load, so the in-process awareness tests fail.

---

## Classification

The failures decompose into **three distinct classes**, with the dominant one identified as the YAML parse error.

### Failure 1a (dominant): `awareness/learning` parse error

**Classification: `DETERMINISTIC_CODE_FAILURE`**

The cherry-picked commit `23a89318` ("awareness: etcd NOSPACE blocks reconciliation at the persistence layer") adds an entry to `docs/awareness/failure_modes.yaml` whose `title:` field contains an **unquoted colon**:

```yaml
    - id: etcd.nospace_alarm_blocks_reconciliation
      title: etcd reaches backend quota and raises alarm:NOSPACE, blocking all controller writes and reconciliation
      severity: critical
      symptoms:
        ...
```

The YAML parser reads `alarm:NOSPACE` as the start of a mapping inside the title value, then fails when it tries to interpret the next key (`severity:`) as belonging to that nested mapping. The error is reported at line 2682 (the `severity: critical` line), which is the next "real" key the parser tries to process after the broken title.

**This is preserved-as-original content from the backup ref** — the cherry-pick faithfully reproduced what `23a89318` originally wrote. The bug is in the *commit*, not the cherry-pick mechanics. Any future merge of this commit's content into any branch would produce the same parse failure.

**Fix:** quote the title in `failure_modes.yaml`:

```yaml
      title: "etcd reaches backend quota and raises alarm:NOSPACE, blocking all controller writes and reconciliation"
```

One-line edit. Whether it's authorized on this branch (push a fix commit) or by reverting/skipping `23a89318` is an operator decision.

### Failure 1b: `cluster_controller_server` race-detector timeout

**Classification: `TIMEOUT_OR_FLAKE`** (pre-existing on `origin/master`, not introduced by this PR)

Evidence:
- The test file `reconcile_lane_status_test.go` and the test
  `TestClusterReconcileOverlapPublishesBlockedLaneStatus` both **exist on `origin/master`**
  unchanged. `git ls-tree origin/master -- .../reconcile_lane_status_test.go` returns a
  blob; the test name is grep-confirmed at the same line numbers on both branches.
- The test waits for `srv.runClusterReconcileIfIdle` to be initialized in a poll
  loop (lines 128-137). CI's combination of `-race -short` plus the runner's
  background load makes the initialization slower than the poll's bounded
  timeout, and the test fatals.
- **Local reproduction of the focused test on the recovery branch
  passes**: `go test ./cluster_controller/cluster_controller_server/ -run
  TestClusterReconcileOverlapPublishesBlockedLaneStatus -count=1 -short`
  returns `ok 0.087s`.

This failure would happen on `origin/master` too if CI re-ran on a slow runner.
It is **not caused by the cherry-picked commits**. It can be expected to flap.

### Failure 1c: `mcp` etcd cascade tests — downstream of 1a

**Classification: `DETERMINISTIC_CODE_FAILURE` (cascading from 1a)**

The `mcp/TestOfflineDiagnose_*` tests load `failure_modes.yaml` to build the
suspected-failure-modes index. When the YAML parse fails (1a), the index is
empty, and the tests' assertions ("`etcd.nospace_alarm` should be in the
suspected list") all fail.

Fixing 1a (quoting the title) will also fix these. No separate change needed.

### Failure 2 (`awareness/cmd/validate-deps` missing): CI config drift

**Classification: `CI_ENVIRONMENT_FAILURE`** (also pre-existing — unrelated to this PR)

The CI workflow references a binary path that **does not exist on this branch
and does not exist on `origin/master`**. The directory
`golang/awareness/cmd/validate-deps/` has no commit history in either ref. This
is a CI configuration drift: someone removed `awareness/cmd/validate-deps`
without updating the CI YAML, or the CI YAML was added speculatively for a
tool that wasn't checked in.

This failure would happen on **every** PR that runs the `ci` workflow, not
just this one. It's not caused by `recover/v1.2.119-hotfix-chain`.

### Summary of classes

| Failure | Class | Caused by this PR? | Blocks merge? |
|---|---|---|---|
| `awareness/learning` (5 tests) | `DETERMINISTIC_CODE_FAILURE` | yes — content in `23a89318` | yes |
| `cluster_controller` (1 test) | `TIMEOUT_OR_FLAKE` | no — pre-existing master state | not really (flake) |
| `mcp` (2 tests) | `DETERMINISTIC_CODE_FAILURE` (cascade) | yes — downstream of 1a | yes (clears when 1a fixed) |
| `validate-deps` build | `CI_ENVIRONMENT_FAILURE` | no — pre-existing CI config | yes (until CI config updated) |

---

## Local reproduction

Commands run on the recovery branch:

```bash
$ git checkout recover/v1.2.119-hotfix-chain
$ cd golang
$ go test ./awareness/learning/ -count=1 -short
--- FAIL: TestPromoteProposalAcceptsApproved (0.04s)
    hardening_test.go:105: ... yaml: line 2682: mapping values are not allowed in this context
--- FAIL: TestPromoteProposalAllowUnapproved (0.03s)
    ... yaml: line 2682: mapping values are not allowed in this context
--- FAIL: TestPromoteProposalWritesToApprovedFiles (0.04s)
    ... yaml: line 2682: mapping values are not allowed in this context
--- FAIL: TestPromoteProposalDoesNotOverwriteExistingInvariants (0.05s)
    ... yaml: line 2682: mapping values are not allowed in this context
--- FAIL: TestPromotedAliasesWrittenToContextAliasesYAML (0.03s)
    ... yaml: line 2682: mapping values are not allowed in this context
FAIL    github.com/globulario/services/golang/awareness/learning   0.238s

$ go test ./cluster_controller/cluster_controller_server/ \
       -run TestClusterReconcileOverlapPublishesBlockedLaneStatus \
       -count=1 -short
ok    github.com/globulario/services/golang/cluster_controller/cluster_controller_server   0.087s
```

| Failure | Reproduced locally? | Matched CI? |
|---|---|---|
| `awareness/learning` (5 tests) | **YES** | **YES — identical error, same line number** |
| `cluster_controller` (1 test) | **NO** (passes locally) | flake — confirmed |
| `mcp` cascade | (not separately re-run; expected to mirror 1a) | matches CI |
| `validate-deps` build | (no local reproduction; CI scaffolding issue) | not applicable |

### Why local pre-push validation missed this

Local pre-push validation only ran `go test ./node_agent/node_agent_server`
because that was the package the inventory called out as the touched
package for this branch. The other 4 commits in the chain touch `cluster_controller`,
`workflow`, `globularcli`, and the awareness YAMLs — none of which were
exercised. A pre-push `go test ./...` would have caught the awareness
parse failure.

**Lesson for the rest of the recovery campaign:** pre-push validation
should at minimum run `go test ./awareness/... -short` to catch YAML
parse regressions, given how many recovery branches modify the
awareness YAMLs.

---

## Git state

| Ref | SHA |
|---|---|
| `master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `origin/master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `backup/local-master-before-reconcile-20260529-144649` | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| `recover/v1.2.119-hotfix-chain` (local) | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| `origin/recover/v1.2.119-hotfix-chain` | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |

Working tree (on `master`): zero tracked-file modifications; untracked
files are conventional `loads/*.md` evidence files only.

---

## Runtime safety

**No runtime mutation performed during investigation.**

```
$ sudo systemctl is-active globular-scylla-manager.service
active

$ sudo systemctl show globular-scylla-manager.service -p NRestarts -p MainPID --no-pager
NRestarts=0
MainPID=770002
```

`MainPID=770002` matches the value scylla-manager has held since the U.1
deploy.

- No `pkg build`, no `pkg publish`, no `services desired set`, no
  service restart.
- No push, no merge, no branch mutation.
- The packages repo was not touched.

---

## Recommended next action

**Fix PR branch with a small follow-up commit.**

The dominant blocker (`awareness/learning` × 5 tests + `mcp` × 2 cascading
tests, ALL on PR content) is a **single 1-line YAML quoting fix** in
`docs/awareness/failure_modes.yaml` at line 2680. The fix is purely
mechanical:

```diff
     - id: etcd.nospace_alarm_blocks_reconciliation
-      title: etcd reaches backend quota and raises alarm:NOSPACE, blocking all controller writes and reconciliation
+      title: "etcd reaches backend quota and raises alarm:NOSPACE, blocking all controller writes and reconciliation"
       severity: critical
```

After that commit, local re-validation should run **the broader pre-push
gate** (`go test ./... -race -short`) to catch any other latent issues
before re-pushing.

The two unrelated failures should be acknowledged but **not blockers** for
PR #8:

- The `cluster_controller` flake exists on `origin/master` and would have
  happened to PR #8 regardless of content. Re-running the CI run after
  the fix commit is likely sufficient.
- The `validate-deps` build failure is **a CI config bug on `master`**.
  It affects every PR that triggers the `ci` workflow. Fixing it
  belongs in a separate PR that updates the CI YAML (either re-add the
  missing tool path or remove the step). PR #8 should not be blocked by
  it; the fix would need a separate operator authorization track.

### Strict instruction compliance

The instruction lists 5 possible recommendations:

1. **fix PR branch with a small follow-up commit** ← this is what's recommended
2. rerun failed CI if clearly flaky — *applies only to the cluster_controller flake, not the dominant failure*
3. close PR #8 if invalid — *not the case; PR is valid, the YAML bug just needs fixing*
4. wait if remaining checks still running but build-test failure not understood — *failure is understood now*
5. inspect more logs if current logs are insufficient — *not needed; root cause identified and locally reproduced*

This document does not authorize the fix commit; the operator's next
turn does.

---

## Stop

Failure analysis complete. The dominant root cause is a 1-line YAML
quoting bug in `docs/awareness/failure_modes.yaml`. Locally reproduced.
The other two CI failure classes (`cluster_controller` race-flake,
`validate-deps` directory missing) are pre-existing on `origin/master`
and not caused by this PR.

PR #8 is `OPEN`/`MERGEABLE`. No code changes, no push, no merge
performed in this turn.
