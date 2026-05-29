# Project U.3 — cluster-doctor HTTPS-first probe for scylla-manager

**Date:** 2026-05-29
**Status:** **PUSHED** to `services/project-u3`; **already deployed on the live cluster** (cluster-doctor 1.2.121); post-push verification confirms stable.

## Push outcome (2026-05-29 12:50)

### Pushed commits

| Repo                       | Remote branch  | Remote SHA          | Source                            | Files in net diff vs `origin/master` |
|----------------------------|----------------|---------------------|-----------------------------------|--------------------------------------|
| `globulario/services`      | `project-u3`   | `21351c96`          | cherry-pick chain S→U.3 with registry.go conflict resolved | 4 files, +979 lines |

`git ls-remote origin refs/heads/project-u3` returns `21351c96…` —
matches local tip.

### Pushed file list (net diff vs `origin/master`)

```
golang/cluster_doctor/cluster_doctor_server/rules/registry.go                            (+11)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go   (+422)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_test.go (+205)
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_u3_test.go (+341 — new)
```

### Dependency carried + how the conflict was resolved

U.3 (`b19ce3aa`) modifies files first added by Project S (`16af03a8`),
which is also unpushed. Cherry-picking U.3 alone onto `origin/master`
left the rule file missing entirely. Minimum chain: S → U.3.

Project S's own commit additionally registered Project O.5's
`systemdWorkingDirectoryMustBeOptional{}` in `registry.go` — but that
type is defined in commit `c529310e` (Project O), which is also unpushed.
The cherry-pick of S onto `origin/master` therefore hit a merge conflict
in `registry.go`.

Resolution: kept only the **Project S contribution**
(`scyllaManagerClusterRegistered{}`) and dropped the dangling Project
O.5 reference, with an inline comment explaining the omission and
naming the upstream commit (`c529310e`) that will reintroduce it. The
isolated branch compiles, all rule tests pass, and the registry change
is internally consistent for U.3's purpose.

### Confirmed exclusions

- **No `packages/`** content (`git diff origin/master..HEAD --name-only
  | grep -E "metadata/|systemd/"` → none).
- **No WorkingDirectory-normalize files** (the 37
  `metadata/<svc>/systemd/globular-<svc>.service` edits in the packages
  repo were not staged or touched).
- **No other unpushed services commits** (Projects A, A2–A5, B, C, D,
  E2, F, J, K, L, N, P, Q, T, U.2 all remain unpushed on local
  `master`).

### Push log

```
$ git push -u origin project-u3
remote: Create a pull request for 'project-u3' on GitHub by visiting:
remote:      https://github.com/globulario/services/pull/new/project-u3
To https://github.com/globulario/services.git
 * [new branch]        project-u3 -> project-u3
branch 'project-u3' set up to track 'origin/project-u3'.
```

## Post-push verification (2026-05-29 12:50)

| Check                                | Result                                                                       |
|--------------------------------------|------------------------------------------------------------------------------|
| Remote head matches local            | `21351c96…` on both                                                          |
| Fresh doctor snapshot taken          | `5c5285ea-38b1-41b2-8883-1e2c7d456093` via `cluster_get_doctor_report freshness=fresh` |
| `scylla_manager.cluster_registered` findings | **0**                                                                |
| Doctor probe path (tcpdump pre-push) | **12 pkts → port 5443 (HTTPS) / 0 pkts → 5080 (HTTP)** during fresh snapshot |
| `globular-scylla-manager.service`    | `ActiveState=active`, `NRestarts=0`                                          |
| `globular-cluster-doctor.service`    | `ActiveState=active`, `NRestarts=0`                                          |
| Cluster registered                   | 1 — `globular-internal` (`932c01cb-8c50-…`), no duplicate                   |
| Backup tasks                         | 2 enabled (`105a3d1f-…`, `3b966c52-…`) — unchanged from U.2                  |
| HTTP listener (5080)                 | **Still bound** — `LISTEN 10.0.0.63:5080 scylla_manager pid=770002 fd=26`    |
| HTTPS listener (5443)                | Bound — `LISTEN 10.0.0.63:5443 scylla_manager pid=770002 fd=27`              |
| Total doctor findings                | 24 (identical class breakdown to pre-push: 20 artifact-cache mismatches, 1 workflow abandonment, 1 WD-normalize, 1 awareness-bundle, 1 cleanup-candidate). **0 regressions.** |

## U.4 remains future-only

U.4 (HTTP listener disable / deprecation) is **not started**. The
recommended gate from the original report still applies:

- ≥ 7 days post-U.3 deploy with no
  `scylla_manager.cluster_registered` WARN (TLS-trust failure) firings
- No evidence with `fallback_reason: https_unavailable`
- sctool write path either replaced (e.g. with a curl-based POST) or
  HTTP-write carve-out designed (sctool 3.10.1 lacks `--ca-file`)
- Other nodes that could host scylla-manager in the future can reach
  the HTTPS endpoint with the same Globular CA anchor

The HTTP listener remains bound on `10.0.0.63:5080` exactly as before.

## Root cause / reason for change

Project U.1 enabled HTTPS for scylla-manager (port 5443). Project U.2
moved the package-shipped registration script to HTTPS-first reads with
strict-CA trust. The cluster-doctor invariant
`scylla_manager.cluster_registered` was still probing only
`http://10.0.0.63:5080`:

- It missed the strict-trust guarantee the rest of the stack was moving to.
- A wrong-cert scenario (e.g. expired service cert, CA rotation,
  intermediate misconfigured) would not be visible — the doctor would
  either be silent (probe error) or report a false "cluster_count=0" on
  the HTTP fallback while the HTTPS endpoint was actually serving the
  truth.
- The probe was hardcoded to `10.0.0.63`, hardwiring the rule to a
  single node and violating the CLAUDE.md hard rule against hardcoded
  remote addresses.

## Files changed

| Path | Change | Lines |
|---|---|---|
| `golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered.go` | Replaced single-endpoint HTTP probe with HTTPS-first / HTTP-fallback / TLS-fail-closed dispatch. Added `discoverScyllaManagerHost`, `probeScyllaManager`, `newScyllaManagerHTTPSClient`, `isTLSVerificationError`, `isHTTPSUnavailableError`. Added separate `newScyllaManagerTLSTrustFinding` for the trust-failure path. | +233 / -41 |
| `golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_test.go` | Updated `withTestEndpoint` helper to pin both bases (HTTPS default unreachable so existing tests still exercise the HTTP path). Added `withTestBases` for the U.3 strict-trust tests. Adjusted `RemediationMentionsScript` test for new finding constructor signature. | +35 / -15 |
| `golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_cluster_registered_u3_test.go` | **New.** 5 U.3 scenarios + 2 discovery tests. | +323 |

Commit on local `master`: **`b19ce3aa`** — services repo.

Package built and published: cluster-doctor **1.2.121** (sha256
`02c00e5ebead4958...`, 17.4 MB) → bridged to
`/var/lib/globular/packages/` → set desired (build_number=1, force) →
applied by node-agent in 13.87 s (`SUCCEEDED`).

## CA path used

`scyllaManagerCAPath = "/var/lib/globular/pki/ca.crt"` (Globular Root
CA, same anchor as the registration script and as all internal mTLS in
the cluster). Loaded into a fresh `x509.NewCertPool()`; the system
trust store is intentionally **not** added.

Why: the system trust store on a Globular node already contains the
Globular CA (installed during clean-node setup), so loading it would
let any system-trusted CA chain-validate the scylla-manager cert and
defeat the strict-anchor guarantee. The same root-cause that bit U.2's
`--capath /dev/null` requirement.

## Fallback rules

| Probe outcome                              | Detection                                                                                  | Action                                       |
|--------------------------------------------|--------------------------------------------------------------------------------------------|----------------------------------------------|
| HTTPS reachable + trusted                  | `http.Client.Do` returns nil error                                                          | Use HTTPS for `/api/v1/clusters`, scheme=`https` |
| HTTPS port unbound                         | `errors.Is(err, syscall.ECONNREFUSED)` or `net.Error.Timeout()` or string-match `"connection refused" / "no route to host" / "network is unreachable"` | Fall back to HTTP, evidence records `fallback_reason: https_unavailable: <err>` |
| HTTPS TCP up but TLS verify fails          | `errors.As(err, *tls.CertificateVerificationError / x509.UnknownAuthorityError / x509.HostnameError / x509.CertificateInvalidError)` plus defensive string match for `"x509:" / "tls: failed to verify"` | **No fallback.** WARN finding with `INVARIANT_UNKNOWN` status; evidence carries `tls_error` and `ca_path`. |
| CA file missing / unreadable               | `os.ReadFile` error in `newScyllaManagerHTTPSClient`                                        | Fall back to HTTP, evidence records `fallback_reason: ca_unavailable: <err>` |
| Both probes fail with non-TLS transport err | After fallback, HTTP probe also errors                                                     | Silent — inconclusive. The unit-down rule covers the daemon outage; this rule does not double-report. |
| HTTP-only legacy manager (no HTTPS at all) | Conn-refused on HTTPS, HTTP works                                                          | Fall back to HTTP, evidence scheme=`http`. Existing `cluster_registered` behavior preserved. |

## TLS-failure behavior — explicit

`isTLSVerificationError(err)` returns true on:

- `*tls.CertificateVerificationError` (Go 1.21+ wraps x509 errors here)
- `x509.UnknownAuthorityError`
- `x509.HostnameError`
- `x509.CertificateInvalidError`
- Defensive: error string contains `"x509:"` or `"tls: failed to verify"`

When true, the probe returns immediately with `httpsTLSErr` set and the
rule emits `newScyllaManagerTLSTrustFinding` —

```
Summary:         "scylla-manager HTTPS endpoint reachable but TLS trust failure
                  blocks safe verification (refusing to fall back to HTTP;
                  cluster registration state cannot be confirmed)"
Severity:        WARN
InvariantStatus: INVARIANT_UNKNOWN
Evidence:
  endpoint:    https://<host>:5443
  scheme:      https
  tls_error:   <error string>
  ca_path:     /var/lib/globular/pki/ca.crt
Remediation:
  1. Verify the scylla-manager process is using the Globular service cert
     (tls_cert_file in /var/lib/globular/scylla-manager/scylla-manager.yaml)
  2. Verify the Globular CA trusts the scylla-manager service cert
     (openssl verify -CAfile)
  3. systemctl restart globular-scylla-manager.service
```

The original ERROR-level `cluster_registered` finding does NOT fire in
the TLS-failure path: we can't know whether the cluster is registered
or not until the trust chain is fixed.

## Tests added

In `scylla_manager_cluster_registered_u3_test.go`:

| Scenario                                                        | Test                                                          |
|-----------------------------------------------------------------|---------------------------------------------------------------|
| HTTPS available + trusted + cluster exists                      | `TestU3_HTTPSAvailableTrusted_NoFindingWhenClusterExists`     |
| HTTPS port refused → HTTP fallback                              | `TestU3_HTTPSConnectionRefused_FallsBackToHTTP`               |
| HTTPS reachable + untrusted cert → no fallback, TLS-trust WARN  | `TestU3_HTTPSCertUntrusted_NoFallback_TLSTrustFinding`        |
| HTTPS available + empty cluster list → ERROR scheme=https       | `TestU3_HTTPSAvailableEmptyCluster_FindingFiresWithHTTPSEvidence` |
| HTTP-only legacy (cluster present + cluster empty subtests)     | `TestU3_HTTPOnlyLegacy_SupportedDuringTransition`             |
| Discovery picks host from active-unit NodeRecord                | `TestU3_DiscoverHostFromSnapshot`                             |
| Discovery falls back to "" on missing data                      | `TestU3_DiscoverHostFallback`                                 |

The untrusted-cert test cannot reuse `httptest.NewTLSServer().Certificate()`
as a "bogus CA" because httptest reuses a single static localhost cert
across all instances — both servers would have the same cert and trust
would silently succeed. The test generates a fresh RSA-2048 self-signed
CA via `x509.CreateCertificate` to guarantee a real chain failure.

## Test results

`go test ./cluster_doctor/cluster_doctor_server/rules/ -run 'ScyllaManagerClusterRegistered|U3' -v`

```
PASS  TestScyllaManagerClusterRegistered_ActiveButEmpty_FiresError           (0.00s)
PASS  TestScyllaManagerClusterRegistered_ActiveWithCluster_Silent            (0.00s)
PASS  TestScyllaManagerClusterRegistered_Inactive_Silent                     (0.00s)
PASS  TestScyllaManagerClusterRegistered_ProbeFails_Silent                   (0.00s)
PASS  TestScyllaManagerClusterRegistered_NoInventory_Silent                  (0.00s)
PASS  TestScyllaManagerClusterRegistered_MultiNode_AnyActive                 (0.00s)
PASS  TestScyllaManagerClusterRegistered_RemediationMentionsScript           (0.00s)
PASS  TestU3_HTTPSAvailableTrusted_NoFindingWhenClusterExists                (0.01s)
PASS  TestU3_HTTPSConnectionRefused_FallsBackToHTTP                          (0.00s)
PASS  TestU3_HTTPSCertUntrusted_NoFallback_TLSTrustFinding                   (0.92s)
PASS  TestU3_HTTPSAvailableEmptyCluster_FindingFiresWithHTTPSEvidence        (0.01s)
PASS  TestU3_HTTPOnlyLegacy_SupportedDuringTransition                        (0.00s)
PASS  TestU3_DiscoverHostFromSnapshot                                        (0.00s)
PASS  TestU3_DiscoverHostFallback                                            (0.00s)
ok    1.013s  (14/14 PASS)
```

Full cluster-doctor suite also green:
```
ok  cluster_doctor/cluster_doctor_server          0.357s
ok  cluster_doctor/cluster_doctor_server/collector 0.071s
ok  cluster_doctor/cluster_doctor_server/render   0.038s
ok  cluster_doctor/cluster_doctor_server/rules    2.340s
```

## Live doctor evidence showing HTTPS

No scylla-manager finding fired in the post-deploy snapshot (the cluster
is registered, the HTTPS probe returns one cluster → silent path).
"Silent" can be ambiguous about which scheme was used, so we triggered
a fresh snapshot and captured network traffic directly:

```
$ sudo tcpdump -i lo -n -c 12 "(dst port 5443 or dst port 5080) \
                                and src host 10.0.0.63 and dst host 10.0.0.63"
12:43:39.182  10.0.0.63.42262 > 10.0.0.63.5443: Flags [.] ...
12:43:39.182  10.0.0.63.42262 > 10.0.0.63.5443: Flags [.] ack 1 ...
12:43:45.838  10.0.0.63.56138 > 10.0.0.63.5443: Flags [.] ...
12:43:51.982  10.0.0.63.54320 > 10.0.0.63.5443: Flags [.] ...
12:44:06.380  10.0.0.63.46318 > 10.0.0.63.5443: Flags [S] seq ...   ← new SYN
... 12 packets total
```

| Port | Count | Scheme |
|------|-------|--------|
| 5443 | 12    | HTTPS  |
| 5080 | 0     | HTTP   |

The doctor is exclusively connecting to the HTTPS port. The HTTP listener
remains bound (5080 is still active on the host) but the doctor never
touches it. When the U.3-fixed cluster-doctor sees an empty cluster
list, evidence will carry `scheme=https` (covered by the
`TestU3_HTTPSAvailableEmptyCluster_FindingFiresWithHTTPSEvidence` unit
test); when HTTPS is unavailable it carries `scheme=http` and
`fallback_reason`.

For the indirect-but-decisive sanity check (the doctor's user is
`globular`, same as the scylla-manager unit's `User=`):

```
$ sudo -u globular curl -sf -m 3 \
    --capath /dev/null --cacert /var/lib/globular/pki/ca.crt \
    https://10.0.0.63:5443/api/v1/clusters
[{"id":"932c01cb-…","name":"globular-internal","host":"10.0.0.63", …}]
```

Confirming the same effective trust context the doctor exercises
returns the live cluster's data over HTTPS.

## Doctor before / after delta

| Snapshot               | Total findings | scylla-manager findings | Notes |
|------------------------|----------------|--------------------------|-------|
| Pre-Project-U.3 (v1.2.120 HTTP-only) | 24             | 0                        | HTTP probe to 5080, no instrumentation of scheme |
| Post-Project-U.3 (v1.2.121 HTTPS-first) | 24             | 0                        | HTTPS probe to 5443, tcpdump confirms |

The 24 findings are pre-existing — 20 artifact-cache mismatches
(separate class, fixed automatically on next install), 1 workflow
abandonment, 1 WD-normalize finding (the project that's still in
`packages` working tree, Project U.4-adjacent), 1 awareness-bundle
runtime-identity finding, 1 cleanup-candidate INFO. None caused by U.3.
**Zero regressions.**

## Backup state verification

| Check                                                          | State after U.3                                       |
|----------------------------------------------------------------|-------------------------------------------------------|
| `globular-scylla-manager.service` ActiveState / NRestarts      | `active` / `0`                                        |
| `globular-cluster-doctor.service` ActiveState / NRestarts      | `active` / `0`                                        |
| Clusters registered (via HTTPS)                                | 1 — `globular-internal` (`932c01cb-…`), no duplicate  |
| Backup tasks                                                   | 2 enabled (`105a3d1f-…`, `3b966c52-…`) — unchanged from U.2 |
| Backup target / data                                           | Untouched (no MinIO writes from U.3)                  |
| Keyspace `scylla_manager`                                      | Untouched (no schema changes)                         |
| HTTP listener on 5080                                          | Still bound (U.3 does not disable HTTP)               |

## Recommendation for U.4 timing

U.4 (HTTP disable / deprecation) should run after a **clean observation
window** that confirms:

1. **No real-world TLS-trust failures** in production — that is, no
   `scylla_manager.cluster_registered` finding has fired with
   severity=WARN and `tls_error` evidence over the observation period.
   These would indicate a missed cert-rotation or trust-chain bug that
   HTTP would have masked.
2. **No HTTP fallbacks observed** in production — that is, no finding
   has fired with `fallback_reason` set, and no journal entry shows the
   doctor falling back. This confirms the HTTPS listener is reliably up.
3. **Other nodes** that may run scylla-manager in the future (currently
   single-node, but the founding-quorum invariant allows it to move) can
   reach the same HTTPS endpoint with the same trust anchor. Today this
   is trivially true (single node); the U.3 discovery code is what makes
   U.4 portable across nodes.
4. **External callers** (sctool, agent registration writes, operator
   `mc` calls) have moved off HTTP. sctool 3.10.1 does not yet support
   `--ca-file`, so until upstream lands it or U.3.x replaces sctool with
   `curl -X POST`, the write path will still need HTTP. U.4 cannot
   disable HTTP unconditionally; it should:
   - Either gate disable on a feature flag that defaults off until the
     write path is HTTPS-only.
   - Or carve out HTTP access to the write endpoint only and disable
     read endpoints over HTTP.

**Suggested gate for U.4 launch:**

- ≥ 7 days post-U.3 deploy
- 0 occurrences of `scylla_manager.cluster_registered` with
  `InvariantStatus=INVARIANT_UNKNOWN` (i.e. no TLS-trust findings)
- 0 occurrences of evidence `fallback_reason: https_unavailable` in any
  snapshot
- sctool write path either replaced or HTTP carve-out designed

Once those four conditions hold, U.4 can disable the HTTP listener with
high confidence the rest of the stack will not silently degrade.

## Status

U.3 deployed and verified. Ready for U.4 authorization when the
observation window is clean.
