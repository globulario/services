# Repository reconciliation plan — after Project U.3 push

**Date:** 2026-05-29
**Trigger:** Project U.3 push (services/project-u3 → `21351c96`) exposed
the latent cost of carrying ~52 local commits on `master` for weeks
while still cherry-picking the most-recent ones onto isolated branches.
Each isolated push has to re-resolve dependency conflicts and may
introduce silent divergences (U.3 dropped a Project O.5 reference).

This document is **planning only**. No commits, no pushes.

---

## 1. Repo-by-repo status

### A. services — `globulario/services`

| Field | Value |
|---|---|
| Current branch | `master` |
| Ahead of `origin/master` | 52 commits |
| Local feature branches | `project-u2` (`9e2ee870`), `project-u3` (`21351c96`) |
| Pushed to remote | `master` (52 commits behind local), `project-u2`, `project-u3` |
| Stashes | none |
| Uncommitted source files | none |
| Untracked files | only documentation artifacts (`loads/project_*.md`, `_test/`, `docs/docs.tar.gz`, `.awareness/graph-integrity-raw.log`, `.claude/scheduled_tasks.lock`, `docs/intent/meta/audit_history.jsonl`, `golang/loads/`) — none are source; none affect compile/test. |

### B. packages — `globulario/packages`

| Field | Value |
|---|---|
| Current branch | `main` |
| Ahead of `origin/main` | 2 commits (`f86d51f` Project S + `3259c98` Project U.2) |
| Local feature branches | `project-u2` (`bdc3724`) |
| Pushed to remote | `main` (2 commits behind local), `project-u2` |
| Stashes | none |
| Uncommitted source files | **37 WD-normalize systemd unit files** (`metadata/<svc>/systemd/globular-<svc>.service`, adding `-` prefix to `WorkingDirectory=`) |
| Untracked files | none |

### C. Globular — `globulario/Globular`

| Field | Value |
|---|---|
| Current branch | `master` |
| Ahead of `origin/master` | 1 commit (`d804bbe remove bin`) |
| Uncommitted source files | `LICENSE` (whitespace/format diff) |
| Relevance to U.x | **None.** Not touched by any U.x project. Excluded from this plan. |

### D. Other globulario/ repos

None of the 23 other repos under `/home/dave/Documents/github.com/globulario/`
have been touched by Project A through U.3 (`git log --oneline -10` on a
sample shows commits unrelated to this work series). Excluded from this
plan.

---

## 2. Commit graph summary (services repo, oldest → newest)

Local commits on `master` not on `origin/master`, grouped by project:

```
origin/master (07214ede repository: noarch artifacts are first-class members of sync)
│
├── v1.2.119 hotfix chain (4 commits, pre-Project-A)
│   c185abde controller: ApplyPackageRelease dispatch must carry manifest entrypoint_checksum
│   f061d334 awareness: workflow definitions on disk must thread verification fields
│   23a89318 awareness: etcd NOSPACE blocks reconciliation at the persistence layer
│   6e8d01a2 node-agent: install-package must not alias desired_hash into ExpectedSha256
│
├── Project A — awareness-bundle identity inventory
│   80d6667d Project A — awareness-bundle identity-mapping inventory (read-only)
│
├── Project A2 — canonical identity registry
│   4dc2fb38 identity+controller: Project A2 — awareness-bundle canonical identity registry
│   5c537351 controller: also call awareness-bundle migration from periodic reconcile
│   13ebe537 awareness: Project A2 result
│
├── Project A3 — platform fallback
│   a8d9c43a release+controller: Project A3 — shared platform-matching helper
│   29cb4bb7 awareness: Project A3 result
│
├── Project A4 — kind-aware dispatch
│   d03ec5f1 controller: Project A4 — kind-aware release dispatch
│   17579a3e awareness: Project A4 result
│
├── Project A5 — kind-aware FAILED short-circuit (7 commits, longest series)
│   f5faae69 Project A5 — kind-aware FAILED short-circuit for AWARENESS_BUNDLE
│   b0b555cb A5 follow-up — extend kind-aware branch to AVAILABLE
│   16355229 A5 follow-up — suppress stale workflow MarkReleaseFailed
│   5edee29f A5 follow-up — add applyPatchToSvcStatus SetFields handlers
│   1616b123 A5 follow-up — FAILED branch transitions to PENDING
│   1e69384f cluster_controller: route awareness_bundle SetFields to Svc switch
│   ebd3fd18 A5 closure
│
├── cluster_doctor stable-state event suppression (1 commit, orphan)
│   9d1e36e5 cluster_doctor: stop emitting spurious finding.created/resolved events
│
├── Project B — self-hosted runtime proof writer
│   70f8871d Project B impact report + matrix
│   14fbbc50 node-agent: implement post-restart self-hosted runtime proof writer
│   a6af5d8f node-agent: re-assert self-hosted runtime proof on every heartbeat tick
│   d83e0359 Project B result
│
├── Project C — manifest backfill inventory (read-only)
│   2a051583 Project C inventory: 15 IRs with broken Scylla manifest index
│
├── Project D — repository RepairArtifact backfill
│   efa82071 repository: RepairArtifact backfills Scylla index when integrity OK
│   931318db repository: RepairArtifact handles Scylla NULL manifest_json
│   6a5bd635 repository: backfill bypasses state-machine via direct repository writes
│   248857dd repository: backfill fires when manifest_json is NULL
│   8d3fadc6 Project D result
│   93118c05 node-agent: add repository to self-hosted proof writer allowlist (cross-references B)
│
├── Project E / E2 — MinIO contract inventory (corrects E)
│   48700467 Project E inventory: MinIO running healthy; release label stale
│   2383835a Project E2: corrected MinIO path inventory
│
├── Project F — MinIO DEGRADED → AVAILABLE recovery
│   7f977ab5 cluster_controller: detectInfraDrift handles DEGRADED → AVAILABLE
│   2861ae84 Project F result
│
├── Project J — convergence-committer checksum preservation
│   e723331b Project J inventory
│   ac866992 workflow: nodeSyncPackageState writes manifest entrypoint_checksum
│   9348560c Project J closure
│
├── Project K — installed_state checksum backfill CLI
│   27ab5d0e Project K inventory
│   756a6522 installed_state_checksum_backfill: Project K Phase 1 / 2 / 3 CLI
│   2fffb4d6 installed_state_checksum_backfill: do not bump UpdatedUnix
│   76d16734 Project K result
│
├── Project L — globularcli sidecar
│   a1feded1 globularcli: write systemd unit .sha256 sidecar after install
│
├── Project ecdca55c (anchor proof timestamps) — INC-2026-0016 fix
│   ecdca55c node-agent: anchor self-hosted proof timestamps to PID start time
│
├── Project N — wave_blocked retry dispatch
│   a03b1937 cluster_controller: dispatch wave_blocked workflows to retry
│
├── Project O — WorkingDirectory normalize + O.5 invariant
│   c529310e Project O: WorkingDirectory normalize parity + state-path migration + invariant
│
├── Project P — INFRASTRUCTURE remove phase transition
│   fa44aa57 Project P: fix INFRASTRUCTURE remove phase transition
│
├── Project T — verifier entrypoint sidecar
│   eadc5690 Project T: verifier honors manifest entrypoint via install-time sidecar
│
├── Project S — cluster_doctor scylla-manager invariant
│   16af03a8 Project S: cluster_doctor invariant for unregistered scylla-manager
│
├── Project Q — InfrastructureRelease Spec.Paused
│   f10cb471 Project Q: honor Spec.Paused on InfrastructureRelease
│
├── Project U.2 — registration script integration tests
│   1970dd7c Project U.2: scylla-manager registration script HTTPS-first integration tests
│
└── Project U.3 — cluster-doctor HTTPS-first probe (HEAD of master)
    b19ce3aa Project U.3: cluster-doctor HTTPS-first probe for scylla-manager
```

---

## 3. Dependency graph between projects

Source-of-truth: who touches whose files. Computed via `git diff-tree`
intersection across the unpushed range.

```
v1.2.119 chain (4 commits)        — independent: pre-Project-A
  └─ touched mostly by Projects D/J/K via repository + workflow paths

Project A (inventory only)        — no code, no deps
Project A2 ──→ A3 ──→ A4 ──→ A5   — strict linear chain through controller release pipeline
                                    each level references symbols introduced by prior level

9d1e36e5 (orphan doctor-events)   — independent: touches cluster_doctor/server.go cacheFindings
                                    no other unpushed project touches that file

Project B (4 commits)             — independent of A chain
  └─ touched by 93118c05 (repository added to allowlist) later

Project C (inventory)             — no code, no deps
Project D (5 commits + 93118c05)  — repository RepairArtifact + Scylla manifest backfill
                                    93118c05 cross-references B's allowlist

Project E/E2 (inventory)          — no code, no deps
Project F (2 commits)             — MinIO drift, independent

Project J (3 commits)             — workflow checksum semantics, independent
Project K (4 commits)             — installed_state CLI, follows J (depends on J's symbols)
Project L (1 commit)              — globularcli sidecar, independent of K/J

ecdca55c (PID-start anchor)       — independent fix to heartbeat
Project N (1 commit)              — workflow retry dispatch, independent
Project O (1 commit, c529310e)    — adds systemd_working_directory.go + O.5 invariant
                                  — ADDS systemdWorkingDirectoryMustBeOptional{} to registry.go
Project P (1 commit)              — INFRA remove phase, independent
Project T (1 commit)              — verifier sidecar, independent

Project S (1 commit, 16af03a8)    — depends on O.5: S's registry.go also adds O.5's
                                    invariant alongside scyllaManagerClusterRegistered{}.
                                    Cherry-picking S onto origin/master without O fails.
Project Q (1 commit)              — InfrastructureRelease Spec.Paused, independent
Project U.2 (1 commit, 1970dd7c)  — test file only; references the installed script,
                                    NO code dependency on other unpushed commits
Project U.3 (1 commit, b19ce3aa)  — modifies scylla_manager_cluster_registered.go which
                                    was CREATED by S. Hard dependency: needs S.
                                    Indirect dependency through S on O.5.
```

### Already-pushed branches: divergence analysis

| Pushed branch | Carries | Divergence from a clean push of master |
|---|---|---|
| `services/project-u2` (`9e2ee870`) | U.2 cherry-pick alone | None — U.2 test file is additive and self-contained. Will apply cleanly to master at any future point. |
| `services/project-u3` (`21351c96`) | S + U.3 cherry-pick chain, **O.5 reference removed from registry.go** | **Soft divergence.** When Project O is eventually pushed and merged, the resulting `master` will have the O.5 invariant registered. The `project-u3` branch's `registry.go` will be missing that line. Merge into master will reintroduce the O.5 line cleanly (no conflict, since `project-u3` only deletes a context-adjacent line, not the O.5 line itself — wait, actually it DOES delete the O.5 line because S's commit added both and U.3 branch kept only S's contribution). |
| `packages/project-u2` (`bdc3724`) | Project S + Project U.2 chain | None — the chain is the complete dependency closure. No divergence. |

**Important nuance on `services/project-u3` reconciliation:** the
branch was built by cherry-picking S onto `origin/master` and resolving
the `registry.go` conflict to drop the O.5 line. If/when Project O is
pushed first and then S is re-pushed (e.g. via a fresh branch from
the new master tip), S's "natural" application will include the O.5
line (because the conflict will auto-resolve in S's favor). At that
point `project-u3` and `master` will disagree on whether the O.5 line
is present — `master` will have it, `project-u3` will not. **A merge
to master from `project-u3` would silently drop the O.5 line on
master.** This is the latent risk.

---

## 4. Commits that should be promoted next

Three plausible strategies. Each is internally consistent; pick one
discipline and stick to it.

### Strategy 1 — Linear chronological (push order = git log --reverse)

Push the chain in the order it was authored. One feature branch per
project, cherry-pick the project's commits onto the branch's base (the
previous project's branch tip or origin/master), open a PR, merge, move
to the next.

**Pros**: zero divergence risk, every cherry-pick is conflict-free,
review cadence matches development cadence, history is preserved.

**Cons**: 22+ PRs at minimum, slow.

**Next push** under this strategy: the **v1.2.119 hotfix chain (4
commits)**.

### Strategy 2 — Chunked by topic

Group related projects into thematic branches:

| Branch | Projects | Commit count |
|---|---|---|
| `awareness-bundle-identity-v1` | A, A2, A3, A4, A5, 9d1e36e5 | ~17 |
| `repository-and-checksum-hygiene` | B, C, D, E/E2, F, J, K, L, ecdca55c, 93118c05 | ~20 |
| `operational-fixes` | N, O, P, T, S, Q | 6 |
| `scylla-manager-https-hardening` | (already pushed: project-u2, project-u3) | (2) |

**Pros**: fewer PRs (4 instead of 22), each PR is a coherent feature
set.

**Cons**: larger PRs are harder to review; if one commit in a chunk
needs revert, the whole chunk is entangled.

**Next push** under this strategy: **`awareness-bundle-identity-v1`**
(oldest non-pushed series).

### Strategy 3 — Minimum-divergence-fix first

Focus on resolving the **`project-u3` divergence risk** before adding
new chunks. The reconciliation order would be:

1. Push **Project O** alone (`c529310e`) — adds `systemd_working_directory.go`
   and registers O.5 in `registry.go`.
2. Push **Project S** alone (`16af03a8`) cleanly — now that O.5 type
   exists, S's `registry.go` cherry-pick applies without conflict and
   keeps both invariants.
3. Re-push **Project U.3** by force-update or by opening a new PR from
   a freshly-rebased branch — now the U.3 branch will carry the O.5
   line, matching the eventual master.
4. Then continue with strategy 1 or 2 for the rest.

**Pros**: eliminates the silent-divergence trap before more work piles
on top.

**Cons**: requires pushing O and S out of chronological order
(reasonable — they're discrete features, both already deployed).

**Next push** under this strategy: **Project O** (`c529310e`) alone.

---

## 5. Recommended branches to create

Whichever strategy is chosen, the following naming convention keeps
PRs distinguishable:

| Branch base | Branch name | Contains |
|---|---|---|
| `origin/master` | `hotfix-v1.2.119-chain` | the 4 pre-A commits |
| `origin/master` | `awareness-bundle-v1` | A through A5 + 9d1e36e5 |
| `origin/master` or above | `self-hosted-proof-writer` | Project B + 93118c05 + ecdca55c |
| above | `repository-manifest-backfill` | Projects C, D |
| above | `minio-drift-recovery` | Projects E/E2, F |
| above | `workflow-checksum-hygiene` | Projects J, K, L |
| above | `infrastructure-release-fixes` | Projects N, P, T, Q |
| above | `working-directory-normalize` | Project O |
| above | `scylla-manager-invariant` | Project S (depends on O on master) |
| (already pushed) | `project-u2`, `project-u3` | as-is |

If Strategy 2 is chosen, several of those collapse into the four
thematic branches.

---

## 6. Commits that should be squashed or kept separate

- **Keep separate**: each Project's `inventory` / `result` / `closure`
  documentation commits. They are sub-200-line markdown adds that show
  the discovery/resolution flow — valuable for archaeology.
- **Keep separate**: A5's 7 commits — they trace a multi-step fix
  through review feedback; squashing loses the "FAILED branch transitions
  to PENDING (forbidden phase transition)" commit which is its own
  bug discovery.
- **Could squash**: Project D's 4 code commits (`efa82071`, `931318db`,
  `6a5bd635`, `248857dd`) into one "RepairArtifact backfill" commit —
  they're successive fixes to the same logic. Leave the result commit
  (`8d3fadc6`) standalone.
- **Could squash**: Project K's 3 CLI commits (`756a6522`, `2fffb4d6`)
  if reviewers prefer "one shipped tool, one commit".

Default recommendation: **don't squash**. The commits are already
small and named clearly. Squashing destroys the bug-discovery context
that future archaeology will appreciate.

---

## 7. Commits that must not be pushed yet

None of the unpushed commits are sensitive in isolation. The U.4 work
is the only thing flagged "do not push": that work has not been done
yet, so no commits exist to push.

The 37 WD-normalize working-tree changes in `packages/` are
**uncommitted**, not just unpushed. They cannot accidentally be pushed
unless first committed. Until they are committed, they are quarantined
to the working tree and lost only on `git checkout -- .` or
`git stash drop`.

---

## 8. Commits obsolete because later projects superseded them

None. Every commit in the list was either:
- a still-live feature (A2 migration, B proof writer, D backfill, etc.)
- a still-live bug fix (N retry, O WD, P remove phase, etc.)
- documentation evidence of those (inventories, results, closures)

E2 (`2383835a`) corrected an inventory in E (`48700467`), but both are
historical documents and the pair shows the correction trail; neither
is "obsolete" in a way that justifies dropping.

---

## 9. Risk of current state

| Risk | Severity | Detail |
|---|---|---|
| `project-u3` silent divergence on O.5 line | **Medium** | A future merge of `project-u3` to `master` would drop O.5 unless reconciled (re-cherry-pick from current local master after O lands, or manual merge with care). |
| 52-commit divergence between local and remote `master` | **Medium** | Every new isolated cherry-pick branch (like `project-u2`/`project-u3`) has to walk the same dependency-resolution gauntlet. Each one adds latent merge risk. |
| Local-only fixes deployed live | **Low** | All the projects are running on the production cluster (single-node ryzen) because each was authored against and deployed to the live system as part of its incident response. The only "production has it but origin doesn't" risk is normal for a 50-commit pile. |
| WD-normalize uncommitted in `packages` | **Low** | Working tree only; no path to leak into a remote push without explicit `git add`. Should be either committed-and-pushed (own branch, separate from any other packages work) or stashed for clarity. |
| Untracked `loads/project_*.md` reports in `services` | **None** | The convention across prior projects (Q, R, S, T, U.1, U.2, U.3) is to leave `loads/project_*.md` untracked — these are evidence artifacts, not source. Confirmed by inspecting status of past projects' result files: all remain untracked even after their code commits landed. |

---

## 10. Exact next safe command sequence — DO NOT EXECUTE

These are the commands that would execute **Strategy 3 step 1**
(push Project O alone, the divergence-eliminating first move):

```bash
cd /home/dave/Documents/github.com/globulario/services

# Create the isolated branch off origin/master
git checkout -b project-o origin/master

# Cherry-pick Project O
git cherry-pick c529310e

# Verify the only file changes are O's
git diff --stat origin/master..HEAD
# Expected: small set of files under golang/cluster_doctor/cluster_doctor_server/rules/
# (systemd_working_directory.go + registry.go) and golang/systemdutil/ + tests

# Confirm compile + tests pass on the isolated branch
cd golang && go build ./... 2>&1 | head -5
go test ./cluster_doctor/cluster_doctor_server/rules/ -run WorkingDirectory 2>&1 | tail -5
cd ..

# Push
git push -u origin project-o

# Verify remote head
git ls-remote origin refs/heads/project-o

# Return to master and confirm working tree is clean
git checkout master
git status -uno
```

If Project O cherry-picks cleanly (likely, since `systemd_working_directory.go`
is a NEW file and `registry.go` change is purely additive), the push
will land without conflict.

After Project O lands on `origin/master` via PR merge, Strategy 3 step 2
(re-push Project S cleanly) becomes the next safe action.

---

## Next authorized action

**Next authorized action should be: push Project O (commit
`c529310e`) on a fresh branch `project-o` cherry-picked from
`origin/master`** — this is the smallest single push that resolves
the existing `project-u3` divergence risk before any further isolated
pushes accumulate. After it merges, Project S can be pushed cleanly
(without the registry.go conflict-resolution that left the U.3 branch
diverged), and the rest of the local-only chain can be reconciled in
chronological order without latent O.5 reference issues.

---

## 11. Project O push outcome (2026-05-29 13:01)

### Pushed

| Repo | Remote branch | Remote SHA | Source | Files |
|---|---|---|---|---|
| `globulario/services` | `project-o` | `947e3e2e` | cherry-pick of local `c529310e` onto `origin/master` | 13, +853/-30 |

`git ls-remote origin refs/heads/project-o` returned `947e3e2e…` —
matches local cherry-pick tip exactly.

### Cherry-pick was clean

Despite Project O's parent commit being deep in the local chain
(Project N at `a03b1937`), the cherry-pick onto `origin/master`
applied with **zero conflicts**. Project O's content is mostly:
- 2 new files in `golang/systemdutil/`
- 4 new files in test packages (cluster_doctor, cluster_controller,
  node_agent — `*_test.go` + `*_migration_test.go`)
- small additive edits to 7 existing files (each adding a
  registration line or a `WorkingDirectory=-` normalize call —
  positionally distinct from any other unpushed commit's edits)

The clean apply is the structural reason this push could happen in
isolation without dragging in Project N or earlier ancestors.

### Tests run on the isolated branch

```
ok  golang/systemdutil                                         0.007s  (8 tests visible at tail of -v output)
ok  golang/cluster_doctor/cluster_doctor_server/rules          1.078s  (full package incl. 5 new TestSystemdWD_*)
ok  golang/cluster_controller/cluster_controller_server        8.084s  (full package incl. new state_migration_test.go)
ok  golang/node_agent/node_agent_server                      130.716s  (full package incl. new state_migration_test.go)
```

All Project O test suites PASS. No build errors anywhere in the tree
(`go build ./...` silent).

### Confirmed exclusions

- No `packages/` content (`packages/` repo not touched in this turn).
- No `metadata/` files.
- No `scylla_manager_cluster_registered.go` (Project S file).
- No other unpushed services commits — only `c529310e`.

### Divergence-risk update

The `project-u3` branch still carries the `registry.go` divergence
(O.5 line dropped). Until `project-u3` is merged via a PR that
auto-resolves against the future `master` containing Project O, the
divergence persists on the branch. The recommended reconciliation now
is:

1. **Merge `project-o` PR to `origin/master`** (operator action via
   GitHub UI / `gh pr merge` after review).
2. **Re-cherry-pick Project S onto the new master** to obtain a clean
   Project S branch with the full registry.go content (both O.5 and
   scylla-manager invariants). This will replace the current
   `project-u3` branch's S commit with a clean one.
3. **Re-cherry-pick Project U.3 onto the clean S** to produce a
   reconciled `project-u3` branch with the O.5 line present.
4. Force-push the reconciled `project-u3` (or close it and open a new
   PR `project-u3-reconciled`).

That sequence is the next operator decision. This document does not
authorize it.

### State of the 52-commit pile

Pre-push:  `master` 52 ahead of `origin/master`.
Post-push: `master` still 52 ahead of `origin/master`.

(`project-o` is a separate ref; pushing it did not advance
`origin/master`.) The pile is unchanged. Each future operator-approved
push will pick the next isolated branch via a cherry-pick from
`origin/master`. After `project-o` merges to `origin/master`, the
chain's natural ordering will start shrinking the pile.

---

## Status

`project-o` pushed cleanly. The `project-u3` divergence reconciliation
is now possible whenever the operator merges `project-o` and authorizes
a re-push of Project S / Project U.3.

---

## 12. Project S reconciliation push outcome (2026-05-29 13:05)

### Pushed

| Repo | Remote branch | Remote SHA | Source | Base | Files in S contribution |
|---|---|---|---|---|---|
| `globulario/services` | `project-s-reconciled` | `73cb2516` | cherry-pick of local `16af03a8` | **`origin/project-o`** (`947e3e2e`) | 3, +349 |

Remote SHA matches local exactly. The branch bases on `origin/project-o`,
so it carries Project O's full content plus Project S's clean addition.

### Cherry-pick was clean (no conflict)

The cherry-pick of `16af03a8` onto `origin/project-o` applied with **zero
conflicts**. This is the structural payoff of branching off `project-o`:
the O.5 registration line that Project S's commit also touched is
already present in the base, so the `registry.go` patch from Project S
slots cleanly underneath the O.5 line as an additive change. No manual
resolution was required.

Contrast with the earlier `project-u3` cherry-pick onto `origin/master`,
which hit a conflict in the same `registry.go` region precisely because
the O.5 line did not exist on the base.

### Both invariants present — proof

```
$ grep -nE "systemdWorkingDirectoryMustBeOptional\{\}|scyllaManagerClusterRegistered\{\}" \
    golang/cluster_doctor/cluster_doctor_server/rules/registry.go
219:		systemdWorkingDirectoryMustBeOptional{},     ← from Project O (carried via origin/project-o)
223:		scyllaManagerClusterRegistered{},            ← from Project S (this branch's contribution)
```

Both lines exist with the comment block from Project S intact between
them. The earlier U.3 silent-drop divergence does NOT exist on this
branch.

### Files in the S contribution (vs `origin/project-o`)

```
golang/cluster_doctor/cluster_doctor_server/rules/registry.go               (+4)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go      (new, +167)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_test.go (new, +178)
```

Net diff vs `origin/master` (carries Project O + Project S):
15 files, +1202/-30.

### Confirmed exclusions

- **No U.2 HTTPS-first changes** in the registration script (which lives
  in `packages/`, untouched in this push).
- **No U.3 HTTPS-first changes** in `scylla_manager_cluster_registered.go`
  (verified: no `--capath /dev/null`, no `httpsTLSErr`, no
  `isTLSVerificationError`, no `probeOutcome` symbols present).
- **No U.2/U.3 test files** (`scylla_manager_cluster_registered_u3_test.go`
  / `scylla_manager_register_script_test.go` not on this branch).
- **No Q/T/N or other unrelated unpushed commits** — only Projects O+S.

### Tests run

```
go test ./cluster_doctor/cluster_doctor_server/rules/       → ok 1.143s (full package)

  Project S scylla-manager invariant tests:
  PASS  TestScyllaManagerClusterRegistered_ActiveButEmpty_FiresError      (0.00s)
  PASS  TestScyllaManagerClusterRegistered_ActiveWithCluster_Silent       (0.00s)
  PASS  TestScyllaManagerClusterRegistered_Inactive_Silent                (0.00s)
  PASS  TestScyllaManagerClusterRegistered_ProbeFails_Silent              (0.00s)
  PASS  TestScyllaManagerClusterRegistered_NoInventory_Silent             (0.00s)
  PASS  TestScyllaManagerClusterRegistered_MultiNode_AnyActive            (0.00s)
  PASS  TestScyllaManagerClusterRegistered_RemediationMentionsScript      (0.00s)

go build ./...                                              → silent (BUILD OK)
```

The full cluster_doctor rules suite passes — including both Project O's
`TestSystemdWD_*` family and Project S's
`TestScyllaManagerClusterRegistered_*` family — confirming both
invariants coexist correctly.

### Pushed reconciliation branches (all four)

| Branch | Tip | Base | Contains |
|---|---|---|---|
| `project-u2` | `9e2ee870` | `origin/master` | U.2 test file only (independent) |
| `project-u3` | `21351c96` | `origin/master` | S + U.3 with **O.5 line dropped** (silent-divergence trap) |
| `project-o` | `947e3e2e` | `origin/master` | Project O alone (supplies O.5 invariant) |
| `project-s-reconciled` | `73cb2516` | `origin/project-o` | O + S clean (both invariants present) |

### Next recommended target

**Project U.3 reconciled on top of Project O + Project S** — base off
`origin/project-s-reconciled`, cherry-pick `b19ce3aa`. The cherry-pick
should apply cleanly (U.3's diff edits files Project S created; with S
present in the base, the conflict that produced the original
`project-u3` divergence will not recur). The resulting branch will
carry the full HTTPS-first doctor probe code with the O.5 line intact
in registry.go — eliminating the silent-divergence trap that motivated
this entire reconciliation sequence.

After that branch lands, the operator will have three clean candidates
in a clean dependency order — `project-o`, `project-s-reconciled`,
`project-u3-reconciled` — that can be merged in sequence to fast-forward
`origin/master` past the O→S→U.3 portion of the local chain.

---

## Status

Project S reconciled branch pushed cleanly. Next recommended target is
U.3 reconciled branch.

---

## 13. Project U.3 reconciliation push outcome (2026-05-29 13:08)

### Pushed

| Repo | Remote branch | Remote SHA | Source | Base | Files in U.3 contribution |
|---|---|---|---|---|---|
| `globulario/services` | `project-u3-reconciled` | `66d191e5` | cherry-pick of local `b19ce3aa` | **`origin/project-s-reconciled`** (`73cb2516`) | 3, +679/-56 |

Remote SHA matches local exactly.

### Cherry-pick was clean (no conflict)

The cherry-pick of `b19ce3aa` onto `origin/project-s-reconciled` applied
with **zero conflicts**. Branching off `project-s-reconciled` meant
Project S's `scylla_manager_cluster_registered.go` was already in the
base in its full pre-U.3 form, so U.3's modifications to that file
slotted in cleanly. Critically: `registry.go` was untouched by U.3 and
remains exactly as Project S left it on the reconciled base — both
invariants intact.

This is the inverse of the original `project-u3` cherry-pick (onto
`origin/master`) which hit a conflict in `registry.go` because S's
additions to that file did not exist on the base. The whole point of
the reconciliation sequence (O → S → U.3 in proper dependency order)
was to let each subsequent cherry-pick be conflict-free.

### Both O.5 and S invariants intact

```
$ grep -nE "systemdWorkingDirectoryMustBeOptional\{\}|scyllaManagerClusterRegistered\{\}" \
    golang/cluster_doctor/cluster_doctor_server/rules/registry.go
219:		systemdWorkingDirectoryMustBeOptional{},     ← from Project O (carried via base)
223:		scyllaManagerClusterRegistered{},            ← from Project S (carried via base)
```

The original `project-u3` branch is missing line 219; `project-u3-reconciled`
has both lines, identical to what `master` will look like after Projects
O → S → U.3 merge in order. **The silent-divergence trap is eliminated
on this branch.**

### U.3 HTTPS-first behavior — symbol presence verified

In the reconciled `scylla_manager_cluster_registered.go`:

| Symbol / phrase | Occurrences | Purpose |
|---|---|---|
| `--capath /dev/null` | 1 | comment documenting the same trust-isolation guarantee used by the script |
| `newScyllaManagerHTTPSClient` | 3 | builds strict-CA-only HTTPS client |
| `isTLSVerificationError` | 3 | detects x509 / tls.CertificateVerificationError to gate fail-closed path |
| `isHTTPSUnavailableError` | 3 | detects ECONNREFUSED / timeout / unreachable to gate HTTP fallback |
| `probeOutcome` | 4 | typed struct carrying scheme/clusters/tls-err/fallback-reason |
| `discoverScyllaManagerHost` | 3 | host discovery from snapshot NodeRecord (kills the hardcoded-IP rule) |
| `newScyllaManagerTLSTrustFinding` | 2 | dedicated WARN/UNKNOWN finding constructor for TLS trust failures |
| `scheme.*https` | 3 | evidence records scheme=https when HTTPS path was used |
| `fallback_reason` | 1 | evidence records why HTTP was used when fallback triggered |
| `tls_error` | 1 | evidence records the cert-validation error string |

All U.3 behavior preserved exactly.

### Files in the U.3 contribution (vs `origin/project-s-reconciled`)

```
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go        (+359 -41)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_test.go   (+35  -15)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_u3_test.go  (new, +341)
```

Net diff vs `origin/master` (full O+S+U.3 stack): 16 files, +1825/-30.

### Confirmed exclusions

- **No package U.2 script changes** (packages repo untouched).
- **No Project Q changes** (no `infrastructure_release_paused` files).
- **No Project T changes**.
- **No `metadata/`, no `systemd/`, no WD-normalize files**.
- **No other unpushed services commits** beyond the O+S+U.3 stack.

### Tests run

```
go build ./...                                                            → silent (BUILD OK)
go test ./cluster_doctor/cluster_doctor_server/rules/                     → ok 1.515s (full package)
go test ./cluster_doctor/cluster_doctor_server/rules/ -run 'SystemdWD|ScyllaManagerClusterRegistered|U3' -v
```

All three invariant test families PASS together:

```
  Project O.5 (TestSystemdWD_*):
    PASS  TestSystemdWD_BareGlobularWDIsFlagged
    PASS  TestSystemdWD_OptionalWDIsSilent
    PASS  TestSystemdWD_NoWDIsSilent
    PASS  TestSystemdWD_CommentedWDIsSilent
    PASS  TestSystemdWD_NonGlobularUnitIgnored
    PASS  TestSystemdWD_MultipleOffendersAggregated

  Project S (TestScyllaManagerClusterRegistered_*):  7/7 PASS

  Project U.3 (TestU3_*):
    PASS  TestU3_HTTPSAvailableTrusted_NoFindingWhenClusterExists
    PASS  TestU3_HTTPSConnectionRefused_FallsBackToHTTP
    PASS  TestU3_HTTPSCertUntrusted_NoFallback_TLSTrustFinding              (2.02s — TLS handshake)
    PASS  TestU3_HTTPSAvailableEmptyCluster_FindingFiresWithHTTPSEvidence
    PASS  TestU3_HTTPOnlyLegacy_SupportedDuringTransition
      PASS  cluster_exists_silent
      PASS  cluster_empty_fires_with_http_evidence
    PASS  TestU3_DiscoverHostFromSnapshot
    PASS  TestU3_DiscoverHostFallback
```

### Pushed reconciliation branches (all five)

| Branch | Tip | Base | Contains | Status |
|---|---|---|---|---|
| `project-u2` | `9e2ee870` | `origin/master` | U.2 test file alone | clean, independent |
| `project-u3` | `21351c96` | `origin/master` | S + U.3 with O.5 line dropped | **divergence trap — supersede with `project-u3-reconciled`** |
| `project-o` | `947e3e2e` | `origin/master` | Project O alone | clean |
| `project-s-reconciled` | `73cb2516` | `origin/project-o` | O + S clean | clean |
| `project-u3-reconciled` | `66d191e5` | `origin/project-s-reconciled` | O + S + U.3 clean | **clean — replaces `project-u3`** |

### Recommended operator action sequence (review-then-merge, not authorized here)

The three reconciled branches form a clean dependency chain. Once
merged in order they fast-forward `origin/master` past O → S → U.3:

```
origin/master  →  project-o  →  project-s-reconciled  →  project-u3-reconciled
```

The old `project-u3` branch should be **closed without merging** (and
optionally deleted from the remote after review) to prevent any future
merge from silently dropping the O.5 line.

The independent `project-u2` branch and the `packages/project-u2` branch
remain mergeable at any point.

### Status

Project U.3 reconciled branch pushed cleanly. The silent-divergence
trap is eliminated. The local 52-commit pile on `master` still exists —
each future authorized push will pick the next isolated branch from the
chain in order.

---

## 14. Deprecation of old `project-u3` (2026-05-29 13:11)

### Deprecated branch

```
Deprecated branch:
- services/project-u3 → 21351c96
- superseded by services/project-u3-reconciled → 66d191e5
- reason: old branch dropped Project O.5 registry entry during conflict resolution
- action: do not merge old project-u3
```

The deprecated branch is **still present on the remote** at SHA
`21351c96`. No deletion has occurred. The deprecation is an
informational status — operators must not merge that branch, and should
prefer `project-u3-reconciled` for U.3 functionality.

### Why the old branch must not merge

The original cherry-pick of U.3 onto `origin/master` had to also pull
in Project S (`16af03a8`) as a dependency. Project S's commit to
`registry.go` added two invariant registrations:

```go
// Project O.5 (in S's commit because O.5 was already in the local tree)
systemdWorkingDirectoryMustBeOptional{},
// Project S itself
scyllaManagerClusterRegistered{},
```

But the `origin/master` base at that time did not have Project O on it
(O lives in commit `c529310e`, which was also unpushed and defines
`systemdWorkingDirectoryMustBeOptional`). The cherry-pick of S onto
that base would have left a dangling type reference, so the
`registry.go` conflict was resolved by **dropping the O.5 line** with
an inline note. That worked for the U.3 branch in isolation (it
compiled, tests passed), but it embedded a silent divergence:

- If `project-u3` is merged to `master` *after* Project O lands, the
  merge will not raise a conflict on the O.5 line (the branch's
  resolution **deleted** the line locally; the merge engine treats it
  as a deliberate removal). The O.5 invariant registration would
  disappear from `master` without warning, and `cluster_doctor` would
  silently stop running the O.5 systemd-WorkingDirectory rule.

That trap does not exist on `project-u3-reconciled`, which was built
on `origin/project-s-reconciled` (which itself bases on
`origin/project-o`). Both O.5 and S invariants are present at
`registry.go:219` and `registry.go:223` exactly as `master` will
contain them after the merge sequence.

### Recommended merge order (unchanged)

```
origin/master
  ← merge project-o                  (adds O.5 invariant + WorkingDirectory normalize)
  ← merge project-s-reconciled       (adds scylla-manager cluster_registered invariant)
  ← merge project-u3-reconciled      (adds HTTPS-first probe + host discovery)
```

`project-u3-reconciled` should be merged **instead of** the deprecated
`project-u3`. The independent `services/project-u2` branch
(`9e2ee870`) and `packages/project-u2` (`bdc37247`) remain mergeable at
any point in this sequence and do not depend on the O→S→U.3 chain.

### Branch-retirement options — PREPARED, NOT EXECUTED

Choose one of these only after the operator has merged the
reconciled stack. Until then, the deprecated branch should remain on
the remote for diff-review purposes.

#### Option A — delete the old branch (safest end-state)

```bash
git push origin --delete project-u3
```

Recommended when the operator has merged `project-u3-reconciled` and
wants to eliminate the trap entirely. After this command, the
deprecated SHA `21351c96` is unreachable via any branch ref and cannot
be accidentally merged.

#### Option B — rename it to a "supersedes" marker (preserves audit trail)

```bash
# create a new ref pointing at the reconciled tip with a name that
# documents the supersession
git push origin project-u3-reconciled:project-u3-supersedes-old-u3
# (then optionally delete the old branch — Option A above)
```

Use this only when the team wants a discoverable rename trail. The
side effect of creating an extra ref to the same SHA can confuse
PR-merge UIs; Option A is cleaner for most workflows.

#### Option C — close-without-merge via GitHub UI (recommended now)

If a PR has already been opened from `project-u3`, the simplest action
is to **close it without merging** via the GitHub UI (no git command
needed). The branch remains on the remote at `21351c96` for audit but
cannot be merged from a closed PR. Leave the branch deletion for later
or apply Option A after the reconciled stack lands.

### Confirmations

- **No code changes were made** in this turn (`git status` clean; tree
  is on `master` at `b19ce3aa` with no unstaged or staged source
  modifications).
- **No push happened** in this turn (`git ls-remote` returns the same
  five SHAs as before: `project-u2` `9e2ee870`, `project-u3`
  `21351c96`, `project-o` `947e3e2e`, `project-s-reconciled`
  `73cb2516`, `project-u3-reconciled` `66d191e5`).
- **No deployment happened** — no `pkg build` / `pkg publish` /
  `services desired set` calls were issued.
- **The recommended merge order remains:**
  `project-o` → `project-s-reconciled` → `project-u3-reconciled`.

### Next operator action

**Recommended next human/operator action:** open three GitHub PRs from
`project-o`, `project-s-reconciled`, and `project-u3-reconciled` in
that order, review, and merge in sequence to fast-forward
`origin/master` past the O→S→U.3 portion of the local chain. The
deprecated `project-u3` should be closed without merging via the
GitHub UI before, during, or after the reconciled stack lands —
deletion via Option A above is a follow-up cleanup once review is
complete.

### Status

`project-u3` formally deprecated in this document. No git operations
executed. All five branches present on remote at their original SHAs.



