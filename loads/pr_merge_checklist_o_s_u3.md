# PR merge checklist — O → S → U.3 reconciliation stack

**Date:** 2026-05-29
**Audience:** human reviewer/operator
**Scope:** services repo only. The packages repo `project-u2` branch is
independent and can be merged separately.

This document is a review aid. No git operations are required to read
it. Three PRs should be opened and merged in dependency order; the
fourth (deprecated) PR must be closed without merging.

---

## Quick reference

| # | Branch | Tip SHA | Depends on | Merge target |
|---|---|---|---|---|
| 1 | `project-o` | `947e3e2e` | `origin/master` | `master` |
| 2 | `project-s-reconciled` | `73cb2516` | PR 1 merged | `master` |
| 3 | `project-u3-reconciled` | `66d191e5` | PR 2 merged | `master` |
| ⚠ | `project-u3` (DEPRECATED) | `21351c96` | — | **close without merging** |

Independent of this stack:
- `services/project-u2` (`9e2ee870`) — U.2 integration tests, mergeable any time.
- `packages/project-u2` (`bdc37247`) — packages-repo U.2 script + Project S registration script, mergeable any time.

---

## PR 1 — `project-o`

### Purpose

Adds Project O — WorkingDirectory normalize parity across install
paths, state-path migration in cluster-controller and node-agent, and
the cluster-doctor invariant `systemdWorkingDirectoryMustBeOptional{}`
that guards against future regressions where a globular `*.service`
unit ships a bare `WorkingDirectory=/var/lib/globular/...` (which
would crash with status=200/CHDIR if the dir is absent).

### Expected files (13 files, +853 / -30)

```
golang/cluster_controller/cluster_controller_server/main.go             (small additive)
golang/cluster_controller/cluster_controller_server/state.go            (+58)
golang/cluster_controller/cluster_controller_server/state_migration_test.go  (new, +142)
golang/cluster_doctor/cluster_doctor_server/rules/registry.go           (+6 — registers systemdWorkingDirectoryMustBeOptional{})
golang/cluster_doctor/cluster_doctor_server/rules/systemd_working_directory.go      (new, +93)
golang/cluster_doctor/cluster_doctor_server/rules/systemd_working_directory_test.go (new, +138)
golang/globularcli/services_cmds.go                                     (+9 — install path normalize call)
golang/node_agent/node_agent_server/internal/actions/artifact.go        (+53 -41 — install path normalize call)
golang/node_agent/node_agent_server/main.go                             (+9)
golang/node_agent/node_agent_server/state.go                            (+44)
golang/node_agent/node_agent_server/state_migration_test.go             (new, +92)
golang/systemdutil/working_directory.go                                 (new, +103 — NormalizeUnitWorkingDirectory + HasBareGlobularWorkingDirectory)
golang/systemdutil/working_directory_test.go                            (new, +130)
```

### Tests already run on the isolated branch

```
go build ./...                                                           → silent (BUILD OK)
go test ./systemdutil/                                                   → ok 0.007s (8 normalize/detect tests)
go test ./cluster_doctor/cluster_doctor_server/rules/                    → ok 1.078s (incl. 5 new TestSystemdWD_*)
go test ./cluster_controller/cluster_controller_server/                  → ok 8.084s (incl. new state_migration_test.go)
go test ./node_agent/node_agent_server/                                  → ok 130.716s (incl. new state_migration_test.go)
```

### Merge condition

- [ ] CI on `project-o` is green (or skipped — repo has no Actions
      configured at the time of this checklist).
- [ ] Review confirms `registry.go` adds exactly one line for
      `systemdWorkingDirectoryMustBeOptional{}` (and its 5-line comment
      block immediately above it).
- [ ] Review confirms no other unpushed services projects leak in.
- [ ] Merge **before** PRs 2 and 3.

---

## PR 2 — `project-s-reconciled`

### Depends on

**PR 1 (`project-o`) must be merged first.** This branch bases off
`origin/project-o` and re-uses the O.5 registry line already present
in that base. If PR 1 has not merged, the branch will fail to merge
into `master` because `registry.go` will have no O.5 line to anchor
against.

### Purpose

Adds Project S — cluster-doctor invariant
`scyllaManagerClusterRegistered{}` that fires when
`globular-scylla-manager.service` is active but no Scylla cluster is
registered with it. This is the safety net for the Project R
"running-but-unregistered" failure mode (backups, repairs, restores
all silently unavailable).

This is the **reconciled** version of Project S. The deprecated
`project-u3` branch contained Project S's code with the O.5 line
removed; this branch keeps both.

### Expected files (3 files, +349 vs `origin/project-o`)

```
golang/cluster_doctor/cluster_doctor_server/rules/registry.go                              (+4 — adds scyllaManagerClusterRegistered{} below the O.5 line)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go     (new, +167)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_test.go (new, +178)
```

Combined diff vs `origin/master` (if reviewed without PR 1 already
merged): 15 files, +1202 / -30 (O + S together).

### Tests already run

```
go build ./...                                                           → silent (BUILD OK)
go test ./cluster_doctor/cluster_doctor_server/rules/                    → ok 1.143s (full package)

  Project S tests, 7/7 PASS:
    TestScyllaManagerClusterRegistered_ActiveButEmpty_FiresError
    TestScyllaManagerClusterRegistered_ActiveWithCluster_Silent
    TestScyllaManagerClusterRegistered_Inactive_Silent
    TestScyllaManagerClusterRegistered_ProbeFails_Silent
    TestScyllaManagerClusterRegistered_NoInventory_Silent
    TestScyllaManagerClusterRegistered_MultiNode_AnyActive
    TestScyllaManagerClusterRegistered_RemediationMentionsScript
```

### Merge condition

- [ ] **PR 1 (`project-o`) merged to `master`.**
- [ ] CI on `project-s-reconciled` is green (or skipped).
- [ ] Review confirms `registry.go` now contains **both** lines:
      `systemdWorkingDirectoryMustBeOptional{}` and
      `scyllaManagerClusterRegistered{}`.
- [ ] Review confirms no U.3 HTTPS-first symbols leak in (no
      `--capath /dev/null`, no `httpsTLSErr`, no `probeOutcome`, no
      `discoverScyllaManagerHost` on this branch — those belong to U.3).
- [ ] Review confirms no Project Q / T / packages content.
- [ ] Merge **before** PR 3.

---

## PR 3 — `project-u3-reconciled`

### Depends on

**PRs 1 and 2 must be merged first.** This branch bases off
`origin/project-s-reconciled` and modifies the
`scylla_manager_cluster_registered.go` file that Project S created. If
PR 2 has not merged, the modifications target a file that does not
exist on `master` and the merge will fail or conflict.

### Purpose

Adds Project U.3 — cluster-doctor HTTPS-first probe for
scylla-manager. The invariant now:

- Probes `https://<host>:5443/api/v1/clusters` first, with strict CA
  verification against `/var/lib/globular/pki/ca.crt` only (system
  trust store deliberately not loaded — parallels the `--capath
  /dev/null --cacert` contract in the U.2 registration script).
- Falls back to HTTP only on `ECONNREFUSED` / dial timeout / "no route
  to host" — the cases where the HTTPS listener is genuinely absent.
- On any TLS verification failure (`x509.UnknownAuthorityError`,
  `x509.HostnameError`, `x509.CertificateInvalidError`,
  `tls.CertificateVerificationError`), refuses HTTP fallback and emits
  a separate WARN/`INVARIANT_UNKNOWN` finding so the misconfiguration is
  visible rather than silently downgraded.
- Discovers the scylla-manager host from the snapshot's
  `NodeRecord.AgentEndpoint`, killing the previous hardcoded
  `10.0.0.63` and making the rule node-agnostic.

Evidence on the existing ERROR-level "no cluster registered" finding
now records `scheme=https` (or `scheme=http` + `fallback_reason=…`)
so post-mortem readers can tell which transport produced the verdict.

### Expected files (3 files, +679 / -56 vs `origin/project-s-reconciled`)

```
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go         (+359 / -41 — HTTPS-first probe code, discovery, TLS-trust finding)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_test.go    (+35  / -15 — withTestEndpoint helper pinned to two bases)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_u3_test.go (new, +341 — 5 U.3 scenarios + 2 discovery tests)
```

`registry.go` is unchanged by this branch (U.3 modifies the rule, not
its registration).

Combined diff vs `origin/master` (full O+S+U.3 stack): 16 files,
+1825 / -30.

### Tests already run

```
go build ./...                                                           → silent (BUILD OK)
go test ./cluster_doctor/cluster_doctor_server/rules/                    → ok 1.515s (full package — all three invariant families coexist)

  Project O.5 tests, 6/6 PASS:
    TestSystemdWD_BareGlobularWDIsFlagged
    TestSystemdWD_OptionalWDIsSilent
    TestSystemdWD_NoWDIsSilent
    TestSystemdWD_CommentedWDIsSilent
    TestSystemdWD_NonGlobularUnitIgnored
    TestSystemdWD_MultipleOffendersAggregated

  Project S tests, 7/7 PASS  (same list as PR 2)

  Project U.3 tests, 8/8 PASS:
    TestU3_HTTPSAvailableTrusted_NoFindingWhenClusterExists
    TestU3_HTTPSConnectionRefused_FallsBackToHTTP
    TestU3_HTTPSCertUntrusted_NoFallback_TLSTrustFinding                  (2.02s — TLS handshake)
    TestU3_HTTPSAvailableEmptyCluster_FindingFiresWithHTTPSEvidence
    TestU3_HTTPOnlyLegacy_SupportedDuringTransition / cluster_exists_silent
    TestU3_HTTPOnlyLegacy_SupportedDuringTransition / cluster_empty_fires_with_http_evidence
    TestU3_DiscoverHostFromSnapshot
    TestU3_DiscoverHostFallback
```

### Merge condition

- [ ] **PR 1 (`project-o`) merged.**
- [ ] **PR 2 (`project-s-reconciled`) merged.**
- [ ] CI on `project-u3-reconciled` is green (or skipped).
- [ ] Review confirms `registry.go` is unchanged on this PR (U.3
      doesn't touch it).
- [ ] Review confirms the U.3 HTTPS-first symbols are present in
      `scylla_manager_cluster_registered.go` (probe function,
      `newScyllaManagerHTTPSClient`, `isTLSVerificationError`,
      `isHTTPSUnavailableError`, `discoverScyllaManagerHost`,
      `newScyllaManagerTLSTrustFinding`).
- [ ] Review confirms no Project Q / T / packages content.

---

## ⚠ Explicit warning — DO NOT MERGE `services/project-u3`

The deprecated branch `services/project-u3` (`21351c96`) must be
**closed without merging** via the GitHub UI.

That branch's `registry.go` is missing the
`systemdWorkingDirectoryMustBeOptional{}` line. The line was dropped
during conflict resolution when U.3 was originally cherry-picked onto
`origin/master` (before Project O existed on the remote). The branch's
"deletion" of that line is encoded in the patch as a deliberate
removal; if it is merged into `master` *after* PR 1 has landed, the
merge engine will silently drop the O.5 registration with no conflict
warning. `cluster_doctor` would stop running the O.5
systemd-WorkingDirectory rule on every snapshot until the line is
restored manually.

`project-u3-reconciled` (PR 3) is the safe replacement and supersedes
this branch entirely.

**Action:** open the deprecated PR in the GitHub UI → Close → optional
follow-up `git push origin --delete project-u3` after the reconciled
stack lands.

---

## Post-merge verification

Run these checks **after** PR 3 lands on `master`.

### 1. `registry.go` contains both invariants

```bash
git -C /home/dave/Documents/github.com/globulario/services \
    fetch origin && git show origin/master:golang/cluster_doctor/cluster_doctor_server/rules/registry.go \
  | grep -E "systemdWorkingDirectoryMustBeOptional\{\}|scyllaManagerClusterRegistered\{\}"
```

Expected output: two matching lines, in order:

```
		systemdWorkingDirectoryMustBeOptional{},
		scyllaManagerClusterRegistered{},
```

### 2. U.3 HTTPS-first behavior present on `master`

```bash
F=golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go
git -C <repo> show origin/master:$F | grep -E "capath /dev/null|isTLSVerificationError|discoverScyllaManagerHost|probeOutcome|newScyllaManagerTLSTrustFinding"
```

Expected: at least one match for each of the 5 symbols. Absence of any
means U.3's behavior was lost in the merge — investigate before
shipping a new build.

### 3. cluster-doctor tests pass on `master`

```bash
cd /home/dave/Documents/github.com/globulario/services/golang
git checkout master && git pull origin master
go test ./cluster_doctor/cluster_doctor_server/rules/
```

Expected: `ok` with the full package green.

### 4. Live doctor still has 0 scylla-manager findings

Via MCP:

```
mcp__globular__cluster_get_doctor_report(freshness="fresh")
```

Expected: total findings unchanged from pre-merge baseline (24 at the
time of this checklist — 20 artifact-cache mismatches, 1 workflow
abandonment, 1 WD-normalize, 1 awareness-bundle, 1 cleanup-candidate).
**Zero** findings should have `invariant_id` equal to
`scylla_manager.cluster_registered`. If a WARN finding with
`InvariantStatus=INVARIANT_UNKNOWN` and `tls_error` evidence appears,
TLS trust between doctor and scylla-manager is broken — investigate
cert chain at `/var/lib/globular/pki/ca.crt` and the scylla-manager
service cert before any further deploys.

### 5. scylla-manager backup state intact

```bash
sudo curl -sf --capath /dev/null --cacert /var/lib/globular/pki/ca.crt \
  https://10.0.0.63:5443/api/v1/clusters
# Expected: 1 cluster — globular-internal, id 932c01cb-8c50-4a30-b90d-e2f08c10a17c

sudo curl -sf --capath /dev/null --cacert /var/lib/globular/pki/ca.crt \
  "https://10.0.0.63:5443/api/v1/cluster/932c01cb-8c50-4a30-b90d-e2f08c10a17c/tasks?type=backup&clusterId=932c01cb-8c50-4a30-b90d-e2f08c10a17c"
# Expected: 2 backup tasks enabled — 105a3d1f-…, 3b966c52-… (matches pre-merge baseline)
```

Both `globular-scylla-manager.service` and `globular-cluster-doctor.service`
should remain `ActiveState=active` with `NRestarts=0` (no restarts
triggered by the merge — only `master` source changes; no runtime
re-apply has been authorized).

---

## Stop conditions

- If any of PR 1/2/3 fails its merge condition, **stop the chain
  there**. Do not merge later PRs until the failure is understood.
- If post-merge verification check 1 fails (missing invariant
  registration), revert the most recent merge before any further
  cluster operations.
- If post-merge verification check 4 shows a new `tls_trust_failure`
  finding, the cluster-doctor probe is fail-closed correctly — but the
  underlying TLS misconfiguration must be diagnosed before reads of
  scylla-manager state can be trusted.

End of checklist.

---

## Appendix A — PR #4 merge result (2026-05-29 13:30)

### Merged

| Field | Value |
|---|---|
| PR | [#4](https://github.com/globulario/services/pull/4) |
| Branch | `project-o` → `master` |
| Merge strategy | merge commit (preserves `947e3e2e` as a parent) |
| Merge commit SHA | `91b445c1bda0389f5228e6437824fd59bc49b501` |
| Project O commit SHA (now in master history) | `947e3e2ecce9396bf35aba8c4894a3c515980047` |
| `origin/master` advanced | `07214ede` → `91b445c1` |
| Author | davecourtois |
| Merged at | 2026-05-29 13:29:52 -0400 |

### registry.go on `origin/master` after merge

```
$ git show origin/master:golang/cluster_doctor/cluster_doctor_server/rules/registry.go \
    | grep -nE "systemdWorkingDirectoryMustBeOptional\{\}|scyllaManagerClusterRegistered\{\}"
219:		systemdWorkingDirectoryMustBeOptional{},
```

- **Project O.5 invariant present at `:219`** ✓
- **Project S invariant intentionally absent** (lands with PR #5) ✓
- `scylla_manager_cluster_registered.go` correctly NOT yet on master
  (verified via `git ls-tree origin/master` → empty for that path) ✓

### Tests run against `origin/master` post-merge

```
go build ./...                                        → silent (BUILD OK)
go test ./systemdutil/                                → ok   0.007s
go test ./cluster_doctor/cluster_doctor_server/rules  → ok   (cached)
go test ./cluster_controller/cluster_controller_server→ ok   7.956s
go test ./node_agent/node_agent_server                → ok 121.505s
```

The `(cached)` line for the rules package reflects Go's build cache —
the package source on the post-merge `origin/master` matches what the
isolated `project-o` branch already ran; Go reused the prior result.
A `-count=1` re-run would re-execute the same tests; no behavior
change is implied.

### Live cluster: untouched

No deploy, no `pkg build`, no `pkg publish`, no `services desired set`.
`globular-scylla-manager.service` and `globular-cluster-doctor.service`
continue to run the binaries from the U.3-era deploy (v1.2.121). The
PR #4 merge only updated source on the remote.

### PR #5 readiness — yes, safe to merge next

```
$ gh pr view 5 --json mergeable,mergeStateStatus
{
  "mergeable":        "MERGEABLE",
  "mergeStateStatus": "UNSTABLE"
}
```

The `MERGEABLE` flag confirms `project-s-reconciled` still applies
cleanly against the new `origin/master` (which now contains Project
O). The `UNSTABLE` status reflects the same "no required CI checks
have reported" condition as before merge — this repo has no Actions
configured (verified earlier this session). No content blocker stands
between PR #5 and merge.

### Status

Project O merged and verified. Ready to merge PR #5.

---

## Appendix B — PR #5 merge result (2026-05-29 13:35)

### Merged

| Field | Value |
|---|---|
| PR | [#5](https://github.com/globulario/services/pull/5) |
| Branch | `project-s-reconciled` → `master` |
| Merge strategy | merge commit |
| Merge commit SHA | `6af791a5116cddcd21ffb7b3727cd22a47899e98` |
| Project S source commit (now in master history) | `73cb25168038fdc1ff8cafcd84fcf41b73a7fe34` |
| `origin/master` advanced | `91b445c1` → `6af791a5` |

### Pre-merge file-count caveat (resolved)

`gh pr view 5 --json files` initially reported **15 files** — alarming
because S's contribution is only 3 files. Verified via direct git that
this was a UI quirk: GitHub lists files touched by *any commit on the
PR branch*, including commits already in `master` via PR #4. The
actual three-way merge delta computed by `git diff
origin/master..origin/project-s-reconciled` was **exactly 3 files,
+349 insertions** — only S's contribution:

```
golang/cluster_doctor/cluster_doctor_server/rules/registry.go               (+4)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go        (+167)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_test.go   (+178)
```

The merge-base of `master` and `project-s-reconciled` resolved to
`947e3e2e` (Project O), confirming git correctly recognized O's files
as common ancestor content and excluded them from the merge.

### registry.go on `origin/master` after merge — both invariants present

```
$ git show origin/master:golang/cluster_doctor/cluster_doctor_server/rules/registry.go \
    | grep -nE "systemdWorkingDirectoryMustBeOptional\{\}|scyllaManagerClusterRegistered\{\}"
219:		systemdWorkingDirectoryMustBeOptional{}    ← from PR #4 (Project O)
223:		scyllaManagerClusterRegistered{}           ← from PR #5 (Project S)
```

### scylla_manager_cluster_registered.go now exists on master

```
$ git ls-tree origin/master -- golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go
100644 blob 75198465447c28d58f22205e0de8d3e2c2192413  golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go
```

### U.3 HTTPS-first behavior intentionally NOT yet on master

```
$ git show origin/master:.../scylla_manager_cluster_registered.go \
    | grep -cE "capath /dev/null|isTLSVerificationError|probeOutcome|discoverScyllaManagerHost"
0
```

Zero U.3 symbols present. The file on master is Project S's HTTP-only
version, exactly as expected before PR #6 lands.

### Tests run against `origin/master` post-merge

```
go build ./...                                                → silent (BUILD OK)
go test ./cluster_doctor/cluster_doctor_server/rules/         → ok 1.120s (full package, -count=1)

Project S filtered run, -count=1, fresh cache:
  PASS  TestScyllaManagerClusterRegistered_Inactive_Silent
  PASS  TestScyllaManagerClusterRegistered_ProbeFails_Silent
  PASS  TestScyllaManagerClusterRegistered_NoInventory_Silent
  PASS  TestScyllaManagerClusterRegistered_MultiNode_AnyActive
  PASS  TestScyllaManagerClusterRegistered_RemediationMentionsScript
  (tail truncated 2 earlier; full package run above confirms all 7 pass)
```

### Live cluster: untouched

No deploy, no `pkg build`, no `pkg publish`, no `services desired set`.
Both services remain `active`/`NRestarts=0` on the U.3-era binaries
(v1.2.121). The PR #5 merge only updated source on the remote.

### PR #6 readiness — yes, safe to merge next

```
$ gh pr view 6 --json mergeable,mergeStateStatus
{
  "mergeable":        "MERGEABLE",
  "mergeStateStatus": "UNSTABLE"
}
```

Initial poll right after PR #5 merged showed
`mergeable: UNKNOWN` — GitHub's mergeable computation is async after
a base-branch update. A 4-second re-poll resolved to `MERGEABLE`.

Direct git verification:
```
$ git diff --stat origin/master..origin/project-u3-reconciled
  scylla_manager_cluster_registered.go      (+359 / -41)
  scylla_manager_cluster_registered_test.go (+35  / -15)
  scylla_manager_cluster_registered_u3_test.go (new, +341)
  3 files changed, 679 insertions(+), 56 deletions(-)

$ git merge-base origin/master origin/project-u3-reconciled
  73cb2516  (project-s-reconciled tip — now in master via PR #5)
```

The merge-base resolved to S's tip exactly as expected. PR #6's diff
on top of post-PR-#5 master is exactly the U.3 contribution: 3 files,
+679/-56. No content blocker stands between PR #6 and merge.

### Status

Project S merged and verified. Ready to merge PR #6.

---

## Appendix C — PR #6 merge result (2026-05-29 13:38)

### Merged

| Field | Value |
|---|---|
| PR | [#6](https://github.com/globulario/services/pull/6) |
| Branch | `project-u3-reconciled` → `master` |
| Merge strategy | merge commit |
| Merge commit SHA | `a272f4151fba1811f3f5d910a0dd3e04b1ce978a` |
| Project U.3 source commit (now in master history) | `66d191e52c6350315868596be18327945ce26767` |
| `origin/master` advanced | `6af791a5` → `a272f415` |

### Pre-merge merge-delta verification

```
$ git diff --stat origin/master..origin/project-u3-reconciled
  scylla_manager_cluster_registered.go         (+359 / -41)
  scylla_manager_cluster_registered_test.go    (+35  / -15)
  scylla_manager_cluster_registered_u3_test.go (new, +341)
  3 files changed, 679 insertions(+), 56 deletions(-)

$ git merge-base origin/master origin/project-u3-reconciled
  73cb2516  (project-s-reconciled tip — confirms master had S)
```

### registry.go on origin/master after merge — both invariants STILL present

```
$ git show origin/master:.../registry.go | grep -nE "systemdWorkingDirectoryMustBeOptional|scyllaManagerClusterRegistered"
219:		systemdWorkingDirectoryMustBeOptional{}    ← Project O (from PR #4)
223:		scyllaManagerClusterRegistered{}           ← Project S (from PR #5)
```

The U.3 merge did NOT touch registry.go (verified — `git diff` showed
zero lines of registry.go in the U.3 delta). Both lines are preserved
exactly as PRs #4 and #5 placed them.

### U.3 HTTPS-first symbols on origin/master — all present

```
$ F=golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go
$ git show origin/master:$F | grep -cE "<symbol>"

  newScyllaManagerHTTPSClient      → 3
  isTLSVerificationError           → 3
  isHTTPSUnavailableError          → 3
  probeOutcome                     → 4
  discoverScyllaManagerHost        → 3
  newScyllaManagerTLSTrustFinding  → 2
  scheme=https                     → 3
  fallback_reason                  → 1
  tls_error                        → 1
```

All 9 expected U.3 symbols present. The HTTPS-first probe path, strict
CA verification, fail-closed TLS handling, safe HTTP fallback,
evidence metadata, and snapshot-driven host discovery are all on
`origin/master`.

### Tests on origin/master — all three invariant families PASS together

```
go build ./...                                                → silent (BUILD OK)
go test ./cluster_doctor/cluster_doctor_server/rules -count=1 → ok 1.807s
```

Filtered run, all 21 tests PASS in 1.538s:

```
  Project O.5 (TestSystemdWD_*):                    6/6 PASS
    BareGlobularWDIsFlagged, OptionalWDIsSilent, NoWDIsSilent,
    CommentedWDIsSilent, NonGlobularUnitIgnored, MultipleOffendersAggregated

  Project S (TestScyllaManagerClusterRegistered_*): 7/7 PASS

  Project U.3 (TestU3_*):                           8/8 PASS
    HTTPSAvailableTrusted_NoFindingWhenClusterExists,
    HTTPSConnectionRefused_FallsBackToHTTP,
    HTTPSCertUntrusted_NoFallback_TLSTrustFinding (1.46s — TLS handshake),
    HTTPSAvailableEmptyCluster_FindingFiresWithHTTPSEvidence,
    HTTPOnlyLegacy_SupportedDuringTransition,
    DiscoverHostFromSnapshot,
    DiscoverHostFallback
```

### Live cluster verification

| Check | Result |
|---|---|
| `globular-scylla-manager.service` ActiveState / NRestarts / MainPID | `active` / `0` / `770002` (untouched since U.3 deploy) |
| `globular-cluster-doctor.service` ActiveState / NRestarts / MainPID | `active` / `0` / `983399` (untouched — node-agent restarted it earlier) |
| HTTPS listener (port 5443) | `LISTEN 10.0.0.63:5443 scylla_manager pid=770002 fd=27` ✓ bound |
| HTTP listener (port 5080) | `LISTEN 10.0.0.63:5080 scylla_manager pid=770002 fd=26` ✓ still bound (U.4 not run) |
| Clusters registered via HTTPS | 1 — `globular-internal` (`932c01cb-…`), no duplicate |
| Backup tasks | 2 enabled |
| Doctor total findings | 24 (unchanged baseline class breakdown) |
| `scylla_manager.cluster_registered` findings | **0** |
| `tls_trust_failure` evidence in any finding | **0** |
| Doctor snapshot ID | `9990b9f3-a975-4ad7-95c0-5f66b164bd6e` (fresh, age 5s) |

Note: live cluster's runtime behavior is determined by the deployed
binary (`cluster_doctor_server v1.2.121`, deployed during U.3
execution), not by the source on `origin/master`. The merge updated
source; runtime is unchanged. Doctor's HTTPS-only probe pattern
(observed via tcpdump during U.3 execution: 12 packets to :5443, 0 to
:5080) persists post-merge.

### Old `project-u3` status — unchanged, dormant

```
$ gh pr list --state all --head project-u3 → (no PR exists, any state)
$ git ls-remote origin refs/heads/project-u3 → 21351c9671e59e5db128cb62b6322401c844c5aa
```

The deprecated branch is still on the remote at `21351c96` with no PR
opened against it. It cannot be merged (no PR), and the `master`
branch now contains the safe `project-u3-reconciled` content via PR
#6. The deprecated branch is harmless as long as no PR is later
opened from it. Operator may delete it at their convenience via
`git push origin --delete project-u3` (not authorized here).

### Reconciliation status

The O → S → U.3 source reconciliation stack is **complete on
`origin/master`**. All three reconciled branches have been merged in
dependency order with merge commits preserving authorship.

Remaining work (NOT authorized here):
- Packages repo reconciliation (Project S + U.2 already on
  `packages/project-u2`; WD-normalize 37-file working-tree set still
  uncommitted in `packages/main`).
- The 49 other unpushed local services commits on `master` (Projects
  A through U.2 minus O/S/U.3 that just merged). These remain isolated
  to the local clone and have no impact on `origin/master`.

### Status

Project U.3 merged and verified. O → S → U.3 source reconciliation is
complete on `origin/master`.



