# Post-merge verification — O → S → U.3

**Date:** 2026-05-29
**Verification result:** **PASSED.** All three PRs merged to
`origin/master` in dependency order; source and live cluster state
match the contract.

This replaces the earlier halted-verification report (which correctly
suspended itself when the merges had not yet happened).

---

## Merge stack on `origin/master`

| PR | Title | Branch | Source SHA | Merge commit SHA |
|---|---|---|---|---|
| [#4](https://github.com/globulario/services/pull/4) | Project O | `project-o` | `947e3e2e` | `91b445c1` |
| [#5](https://github.com/globulario/services/pull/5) | Project S | `project-s-reconciled` | `73cb2516` | `6af791a5` |
| [#6](https://github.com/globulario/services/pull/6) | Project U.3 | `project-u3-reconciled` | `66d191e5` | `a272f415` |

`origin/master` advanced `07214ede` → `91b445c1` → `6af791a5` →
`a272f415`. All three merge commits use merge-commit strategy
(authorship and commit history preserved).

```
$ git log origin/master --oneline -7
a272f415 Project U.3: make scylla-manager doctor probe HTTPS-first (#6)
6af791a5 Project S: add scylla-manager cluster registration doctor invariant (#5)
91b445c1 Project O: enforce optional systemd WorkingDirectory and canonical state paths (#4)
66d191e5 Project U.3: cluster-doctor HTTPS-first probe for scylla-manager
73cb2516 Project S: cluster_doctor invariant for unregistered scylla-manager
947e3e2e Project O: WorkingDirectory normalize parity + state-path migration + invariant
07214ede repository: noarch artifacts are first-class members of sync (v1.2.118 draft)
```

---

## 1. `registry.go` contains both invariant registrations

```
$ git show origin/master:golang/cluster_doctor/cluster_doctor_server/rules/registry.go \
    | grep -nE "systemdWorkingDirectoryMustBeOptional\{\}|scyllaManagerClusterRegistered\{\}"

219:		systemdWorkingDirectoryMustBeOptional{}    ← from Project O via PR #4
223:		scyllaManagerClusterRegistered{}           ← from Project S via PR #5
```

The silent-divergence trap that would have existed if the deprecated
`project-u3` branch had been merged instead is **not present** on
`master`.

---

## 2. U.3 HTTPS-first behavior present on `origin/master`

```
$ F=golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go
$ git show origin/master:$F | grep -cE "<symbol>"

  newScyllaManagerHTTPSClient        → 3   (strict-CA HTTPS client builder)
  isTLSVerificationError             → 3   (gates fail-closed path on TLS errors)
  isHTTPSUnavailableError            → 3   (gates safe HTTP fallback)
  probeOutcome                       → 4   (typed result for scheme/clusters/tls/fallback)
  discoverScyllaManagerHost          → 3   (snapshot-driven host discovery)
  newScyllaManagerTLSTrustFinding    → 2   (dedicated WARN finding constructor)
  scheme=https                       → 3   (evidence metadata when HTTPS used)
  fallback_reason                    → 1   (evidence metadata on HTTP fallback)
  tls_error                          → 1   (evidence metadata on TLS verify fail)
```

Every U.3 symbol from the original report is present on `master` at
the expected count. HTTPS-first probing, fail-closed TLS handling,
safe HTTP fallback, and host discovery via `NodeRecord.AgentEndpoint`
are all wired correctly.

---

## 3. Test results on `origin/master`

```
go build ./...                                                → silent (BUILD OK)
go test ./cluster_doctor/cluster_doctor_server/rules -count=1 → ok 1.807s (full package)
```

Filtered run (all three invariant families together), `-count=1`
fresh cache, 21/21 PASS in 1.538s:

| Family | Tests | Status |
|---|---|---|
| Project O.5 (`TestSystemdWD_*`) | 6 | 6/6 PASS |
| Project S (`TestScyllaManagerClusterRegistered_*`) | 7 | 7/7 PASS |
| Project U.3 (`TestU3_*`) | 8 (incl. 2.0s TLS-handshake fail-closed test) | 8/8 PASS |

---

## 4. Live cluster verification

### Service health

| Service | ActiveState | NRestarts | MainPID |
|---|---|---|---|
| `globular-scylla-manager.service` | `active` | `0` | `770002` |
| `globular-cluster-doctor.service` | `active` | `0` | `983399` |

Neither service restarted as a result of the merges — the merges only
updated source on the remote; no `pkg build` / `pkg publish` /
`services desired set` was issued.

### Listeners

```
$ sudo ss -tlnp | grep -E ':5080|:5443'
LISTEN 10.0.0.63:5080 users:(("scylla_manager",pid=770002,fd=26))   ← HTTP — still bound (U.4 not run)
LISTEN 10.0.0.63:5443 users:(("scylla_manager",pid=770002,fd=27))   ← HTTPS
```

The HTTP listener remains enabled for the future U.4 observation
window, exactly as required.

### Cluster registration via HTTPS

```
$ sudo curl --capath /dev/null --cacert /var/lib/globular/pki/ca.crt \
       https://10.0.0.63:5443/api/v1/clusters
clusters: 1 (name=globular-internal, id=932c01cb-8c50-4a30-b90d-e2f08c10a17c, host=10.0.0.63)
```

No duplicate cluster.

### Backup tasks

```
backup tasks: 2 enabled=2  (105a3d1f-…, 3b966c52-…)
```

Both backup tasks retained, both still enabled. State matches pre-merge
baseline.

### Doctor — fresh snapshot

| Metric | Value |
|---|---|
| Total findings | 24 |
| `scylla_manager.cluster_registered` findings | **0** |
| `tls_trust_failure` evidence in any finding | **0** |
| Overall status | `degraded` (unchanged baseline) |
| Snapshot ID | `9990b9f3-a975-4ad7-95c0-5f66b164bd6e` |
| Freshness | `FRESHNESS_FRESH`, age 5s |

Findings breakdown (unchanged from every snapshot since U.3 deployed):

- 20 artifact-cache mismatches (separate class, self-healing on next install)
- 1 workflow correlation abandoned (ai-memory, Project N era)
- 1 convergence/WD-normalize finding (will be resolved by packages
  repo work, see "Next" section)
- 1 awareness-bundle runtime-identity finding
- 1 cleanup-candidate INFO

**Zero scylla-manager findings, zero TLS trust failures.** The
HTTPS-first probe is silent (no finding → success path), and tcpdump
during U.3 execution confirmed exclusive use of port 5443 by the
deployed doctor binary.

---

## 5. Old `project-u3` branch — dormant, unmerged

```
$ gh pr list --state all --head project-u3 → (no PRs in any state)
$ git ls-remote origin refs/heads/project-u3 → 21351c9671e59e5db128cb62b6322401c844c5aa
```

The deprecated branch still exists at `21351c96` but has no PR opened
against it and is not an ancestor of `master`. It cannot be merged
inadvertently. Operator may delete it at their convenience via
`git push origin --delete project-u3` (not authorized in this turn).

---

## 6. What changed about the runtime

**Nothing.** The reconciliation merged source to `origin/master`; the
running binaries on the cluster are unchanged. The live HTTPS-first
behavior visible in the cluster today is supplied by the locally-built
`cluster_doctor_server v1.2.121` deployed during Project U.3
execution. The merge brought the source on `origin/master` into
parity with that running binary, eliminating the previous gap where
the production behavior had no source representation on the remote.

---

## Status

**Project O → S → U.3 source reconciliation is complete.**

Reconciliation summary:
- 3 PRs opened, 3 PRs merged, all in dependency order
- 21/21 invariant tests pass on `origin/master`
- 9/9 expected U.3 symbols present on `origin/master`
- Both `O.5` and `S` invariant registrations preserved at
  `registry.go:219` and `:223`
- Deprecated `project-u3` left dormant, no PR opened, cannot be merged
- Live cluster untouched, all services healthy
- HTTP listener remains enabled for the future U.4 observation window

Next recommended target is **packages repo reconciliation**:
- `packages/project-u2` (`bdc37247`) — open PR, review, merge.
- 37 WorkingDirectory-normalize systemd unit files still uncommitted
  in `packages/main` working tree — needs operator decision on
  whether to commit, stash, or revert.
