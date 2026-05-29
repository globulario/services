# PR #8 monitor result

**Date:** 2026-05-29 (post-push, mid-CI)
**Read-only monitor turn — no mutation performed.**

---

## PR

| Field | Value |
|---|---|
| URL | https://github.com/globulario/services/pull/8 |
| Title | *Recover v1.2.119 hotfix chain* |
| State | `OPEN` |
| `mergeable` | `MERGEABLE` |
| `mergeStateStatus` | `UNSTABLE` |
| Head SHA | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| Base | `master` (resolved to `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` via `git rev-parse`) |
| `reviewDecision` | empty (no review submitted yet) |
| `mergedAt` | `null` |
| `closedAt` | `null` |

### Checks (`statusCheckRollup`)

| Workflow | Check name | Status | Conclusion | Notes |
|---|---|---|---|---|
| `Documentation` | `build-docs` | COMPLETED | **SUCCESS** | finished `19:45:22Z` |
| `Documentation` | `validate-cli-commands` | COMPLETED | **SUCCESS** | finished `19:45:39Z` |
| `Documentation` | `check-paths` | COMPLETED | **SUCCESS** | finished `19:45:10Z` |
| `ci` | `build-test` | `IN_PROGRESS` | — | started `19:45:03Z`, still running |
| `ci` | `proto-check` | `IN_PROGRESS` | — | started `19:45:03Z`, still running |
| `ci` | `lint` | `IN_PROGRESS` | — | started `19:45:03Z`, still running |

3 checks green, 3 still running. The `UNSTABLE` merge-state reflects
the three IN_PROGRESS checks — `MERGEABLE: yes` confirms there is no
content conflict.

**Surprising-but-good observation:** CI is wired and active on this
repo. Earlier session runs of `gh run list --branch <name>` returned
empty on `project-o` / `project-s-reconciled` / `project-u3-reconciled`
/ `project-u2` branches, leading to the assumption "no CI configured."
The configuration may key on branch-prefix triggers — the `recover/*`
prefix evidently triggers both the `ci` and `Documentation` workflows
where the earlier `project-*` branches did not. Worth a note for
future cleanup-style PRs.

---

## Git state

| Ref | SHA |
|---|---|
| `master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `origin/master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `backup/local-master-before-reconcile-20260529-144649` | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| `recover/v1.2.119-hotfix-chain` (local) | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| `origin/recover/v1.2.119-hotfix-chain` | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |

| Working tree | Status |
|---|---|
| Tracked-file modifications | **0** |
| Untracked files | conventional `loads/*.md` evidence files + log artifacts (none are source) |
| Stashes | 0 |

Local + remote SHAs match on the recovery branch. `master` and
`origin/master` are aligned. Backup ref is intact. No `git fetch`
revealed new commits on either side.

---

## Runtime safety

| Check | Value |
|---|---|
| `globular-scylla-manager.service` active | `active` |
| `NRestarts` | `0` |
| `MainPID` | `770002` |
| Runtime mutation performed | **no** |

`MainPID=770002` matches the value scylla-manager has held since the
U.1 deploy — no restart, no re-spawn, no reload during this monitor.

- No `pkg build`, no `pkg publish`, no `services desired set`, no
  service restart.
- No push, no branch mutation.
- The packages repo was not touched.

---

## Recommendation

**Wait for PR #8 review and merge.**

Concrete state observed: 3 of 6 checks have passed; the other 3
(`build-test`, `proto-check`, `lint`) are still running. The next
movement on PR #8 is one of:

1. All 3 IN_PROGRESS checks complete with `SUCCESS` →
   `mergeStateStatus` transitions to `CLEAN`. Then operator review +
   merge.
2. One or more IN_PROGRESS checks complete with `FAILURE` →
   investigate. Local validation passed (`go build` silent, `go test
   ./node_agent/node_agent_server` ok 125.287s), so any CI failure
   is likely an environment difference (CI's `go` version, OS,
   plumbing flags) and should be diagnosable from the run logs at
   the `detailsUrl` recorded in this report.

This turn's read-only check is the right cadence; the IN_PROGRESS
checks started at `19:45:03Z` and `go test` against the same source
takes 2 minutes locally, so the `build-test` CI job needs at least
another 2–3 minutes from the present snapshot to complete.

### If PR #8 merges

Recommend operator authorize:

1. `git fetch origin && git checkout master && git merge --ff-only
   origin/master` on local — bring local `master` up to the new tip.
2. For each remaining LOW recovery branch, before pushing its PR,
   confirm it still applies cleanly against the new master:
   ```
   git checkout recover/<name>
   git rebase master   # expect no conflict for any branch except the
                       # ones with awareness-YAML internal touches
   ```
   For the awareness-YAML-extending branches (`recover/project-b-self-hosted-proof-writer`
   has none on this branch — `a6af5d8f` was skipped; `recover/project-j-workflow-checksum`
   skipped `9348560c`), the post-#8 rebase is expected to be clean.
3. Then push the next LOW branch (recommended: `recover/project-c-d-repository-backfill`
   per the prior recommendation — single-file linear evolution, zero
   conflict surface).

### If PR #8 closes / is rejected

Stop the recovery campaign and report:
- the reason as recorded in the PR comments / review
- whether the operator wants the local branch retained for rework
  or deleted
- whether the rest of the LOW recovery set should still be pushed
  (most likely yes, since they're independent — but #8 carries the
  v1.2.119 hotfix code which other branches don't depend on)

---

## Stop

Monitor complete. PR #8 is open, MERGEABLE, with 3/6 CI checks green
and 3 IN_PROGRESS. Next read-only monitor pass should run after the
IN_PROGRESS checks have had time to complete — typically 5–10 minutes
from the start times recorded in `statusCheckRollup` (started
`19:45:03Z`).

---

## Monitor pass 2 (2026-05-29 — post-CI-completion of build-test/proto-check)

### PR

| Field | Value |
|---|---|
| State | `OPEN` |
| `mergeable` | `MERGEABLE` |
| `mergeStateStatus` | `UNSTABLE` |
| `reviewDecision` | empty (no review submitted yet) |
| `mergedAt` | `null` |
| `closedAt` | `null` |
| Head SHA | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| Base | `master` (resolved to `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` via `git rev-parse`) |

Note: the instruction's requested `baseRefOid` field is not a valid
`gh pr view --json` field. Available fields list does not include
`baseRefOid`; only `baseRefName` is exposed. The base SHA is
resolved separately via `git rev-parse master`.

### Checks (6 total, 5 completed, 1 still IN_PROGRESS, **1 FAILURE**)

| Workflow | Check | Status | Conclusion | Details URL |
|---|---|---|---|---|
| `Documentation` | `build-docs` | COMPLETED | **SUCCESS** | https://github.com/globulario/services/actions/runs/26658546525/job/78574887811 |
| `Documentation` | `validate-cli-commands` | COMPLETED | **SUCCESS** | https://github.com/globulario/services/actions/runs/26658546525/job/78574887705 |
| `Documentation` | `check-paths` | COMPLETED | **SUCCESS** | https://github.com/globulario/services/actions/runs/26658546525/job/78574887701 |
| `ci` | `proto-check` | COMPLETED | **SUCCESS** | https://github.com/globulario/services/actions/runs/26658546480/job/78574887599 |
| `ci` | **`build-test`** | **COMPLETED** | **🚨 FAILURE** | https://github.com/globulario/services/actions/runs/26658546480/job/78574887592 |
| `ci` | `lint` | IN_PROGRESS | — | https://github.com/globulario/services/actions/runs/26658546480/job/78574887603 |

`build-test` is the critical check. It completed at `19:50:02Z` (~5
minutes after start) with `conclusion: FAILURE`.

### Git state

| Ref | SHA |
|---|---|
| `master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `origin/master` | `068bf1eb442c6bd1ea09dae8bec41ed4bf55e37d` |
| `backup/local-master-before-reconcile-20260529-144649` | `b19ce3aa0df91e606e39d05ef7d3ba0c44de9e41` |
| `recover/v1.2.119-hotfix-chain` (local) | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |
| `origin/recover/v1.2.119-hotfix-chain` | `b2cb8ad203e621d2ebaac02a47c58c5ba72b7ef9` |

| Working tree | Status |
|---|---|
| Tracked-file modifications | **0** |
| Untracked files | conventional `loads/*.md` evidence files + log artifacts (none are source) |
| Stashes | 0 |

Local + remote SHAs still match on the recovery branch. `master` and
`origin/master` are aligned. Backup ref intact. No new commits
detected on `git fetch origin`.

### Runtime safety

| Check | Value |
|---|---|
| `globular-scylla-manager.service` active | `active` |
| `NRestarts` | `0` |
| `MainPID` | `770002` |
| Runtime mutation performed | **no** |

`MainPID=770002` matches the value scylla-manager has held since the
U.1 deploy — no restart, no re-spawn, no reload during this monitor.

- No `pkg build`, no `pkg publish`, no `services desired set`, no
  service restart.
- No push, no branch mutation.
- The packages repo was not touched.

### Recommendation

**Stop the recovery campaign and report the failing CI check.**

Failing check details:

| Field | Value |
|---|---|
| Workflow | `ci` |
| Check name | `build-test` |
| Started | `2026-05-29T19:45:03Z` |
| Completed | `2026-05-29T19:50:02Z` (duration ~5 minutes) |
| Conclusion | `FAILURE` |
| Details URL | https://github.com/globulario/services/actions/runs/26658546480/job/78574887592 |

#### Context for diagnosis (not authorized to act on in this turn)

Local validation passed before push:
- `go build ./...` → silent (BUILD OK)
- `go test ./node_agent/node_agent_server -count=1` → ok 125.287s

A `build-test` FAILURE despite green local validation indicates one
of:

1. **CI runs a broader test surface than the local pre-push check.**
   The PR carries 4 commits touching `cluster_controller`,
   `node_agent`, `workflow/engine`, `globularcli`, and awareness
   YAMLs. Local validation exercised only `node_agent/node_agent_server`;
   CI likely runs the full `go test ./...` which touches packages
   not exercised pre-push.
2. **Environment difference.** Go version mismatch (local is whatever
   the dev box has; CI may pin a specific version), or a test that
   reads `etcd` / `scylla` / local state and fails in CI's stateless
   container.
3. **One of the 4 cherry-picked commits depends on context not present
   on `origin/master`.** All 4 are independent of O / S / U.3 / U.2
   (the PRs already merged) per the earlier inventory analysis, but
   there may be a dependency on a *different* unmerged-local commit
   that the test exercises.

Resolution path (NOT authorized here — operator decision):

- **Read the CI job logs at the details URL** to identify which test
  package failed and the specific assertion / build error.
- **Reproduce locally** with `go test ./... -count=1` (the broader
  surface) — likely flushes out the same failure.
- **Decide:** fix on the branch (push fix), or close PR #8 and revisit
  the 4-commit chain.

In particular `lint` is still IN_PROGRESS — if it also fails, the
fix may be a single lint nit and a one-line patch will green the
build.

### Strictly per the instruction

The instruction says:

> If checks failed: stop and report failing check names and URLs.

This monitor pass reports:

- Failing check: `ci / build-test`
- URL: https://github.com/globulario/services/actions/runs/26658546480/job/78574887592
- Started: `2026-05-29T19:45:03Z`, completed: `2026-05-29T19:50:02Z` (FAILURE)

Outstanding check: `ci / lint` (IN_PROGRESS).

The PR remains `OPEN` and `MERGEABLE` despite the CI failure — the
operator can either:
1. Wait for `lint` to complete, then read both failure logs.
2. Close PR #8 (not authorized here).
3. Address the failure with a new commit pushed to the branch (not
   authorized here).
4. Override CI and merge anyway (not authorized here).

No code, no push, no merge performed in this monitor turn. The
backup ref `b19ce3aa` remains intact; the 7 other local recovery
branches remain untouched at their prior SHAs.

### Stop

Monitor pass 2 complete. PR #8's `ci / build-test` check has
**FAILED**. Recovery campaign halted at this PR pending operator
review of the failing job's logs.
