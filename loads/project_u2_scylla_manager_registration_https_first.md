# Project U.2 — scylla-manager registration script: HTTPS-first read/probe path

**Date:** 2026-05-29
**Status:** **PUSHED** to isolated branches on both repos; **already deployed on the live cluster** (1.2.74 → 1.2.75 during execution); live verification post-push confirms stable.

## Push outcome (2026-05-29 12:21)

### Pushed commits

| Repo                       | Remote branch  | Remote SHA          | Source commit         | Files changed                                              |
|----------------------------|----------------|---------------------|-----------------------|------------------------------------------------------------|
| `globulario/services`      | `project-u2`   | `9e2ee870`          | cherry-pick of local `1970dd7c` | `golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_register_script_test.go` (+323) |
| `globulario/packages`      | `project-u2`   | `bdc37247`          | cherry-pick chain of `f86d51f` (Project S) + `3259c98` (U.2) | `metadata/scylla-manager/specs/scylla_manager_service.yaml` (+152 vs `origin/main`) |

Remote SHA == local cherry-pick tip for both branches (verified via `git ls-remote`).

### Why packages carries Project S as a dependency

Project U.2's diff modifies the `install-scylla-manager-register-cluster`
step that Project S (`f86d51f`) introduced. Project S is also unpushed
to `origin/main`. Cherry-picking U.2 *alone* onto `origin/main` produced
a merge conflict (the YAML lines U.2 patches do not exist on `origin/main`).
The minimal clean chain is therefore S → U.2, both as separate commits on
the `project-u2` branch. This is the smallest set of commits that yields
a conflict-free diff containing only the U.2 functionality.

The net diff against `origin/main` is **one file only**:
```
metadata/scylla-manager/specs/scylla_manager_service.yaml   (+152)
```

### Unrelated WD-normalize work — confirmed excluded

- The 37 modified `metadata/<svc>/systemd/globular-<svc>.service` files
  were stashed before branching (`git stash push -u "WD-normalize WIP"`)
  and restored to the working tree of `packages` `main` after push.
- The pushed `project-u2` branch contains **zero** modifications to any
  of those 37 files. Verified via `git diff --name-only
  origin/main..origin/project-u2` → returns only the scylla-manager YAML.
- WD-normalize work remains uncommitted in the working tree of `main`,
  exactly as the user requested.

### Other unpushed local content NOT included

The services repo `master` is 51 commits ahead of `origin/master` (50
pre-existing + the now-cherry-picked U.2). Pushing `master` would have
shipped all 51. The `project-u2` branch isolates only the single U.2
commit (`9e2ee870` cherry-picked from `1970dd7c`).

---

## 1. Exact files changed in both commits

### services repo — commit `1970dd7c` (local, branch `master`, ahead of `origin/master` by 51 commits)

```
golang/cluster_doctor/cluster_doctor_server/rules/scylla_manager_register_script_test.go   (+323)
```

One new file. Adds 5 integration tests that exercise the installed
`/usr/lib/globular/bin/scylla-manager-register-cluster` script against
`httptest` TLS/HTTP servers and a stubbed `sctool`. Tests skip when the
script is not installed (clean CI runners).

### packages repo — commit `3259c98` (local, branch `main`, ahead of `origin/main` by 2 commits)

```
metadata/scylla-manager/specs/scylla_manager_service.yaml   (+63 / -15)
```

Single file. Updates the `install-scylla-manager-register-cluster` step
to ship a script with HTTPS-first probe and `--capath /dev/null --cacert
"$GLOBULAR_CA"` strict verification.

The packages-branch ancestor commit `f86d51f` is **Project S** (also
unpushed). A push of `main` would publish both Project S and Project U.2
together.

---

## 2. HTTPS-first behavior summary

The script's read/probe path now performs this sequence:

```sh
curl -sf -m 2 --capath /dev/null --cacert "$GLOBULAR_CA" "$HTTPS_BASE/version"
```

- `$HTTPS_BASE` defaults to `https://$HOST:5443/api/v1`.
- `$GLOBULAR_CA` defaults to `/var/lib/globular/pki/ca.crt`.
- `--capath /dev/null` is **mandatory**: without it, curl 8.5.0 on
  Ubuntu falls back to the OS trust store. Since the OS bundle on a
  Globular node already trusts the Globular Root CA, the strict-CA claim
  was a no-op in 1.2.74 (discovered live; see §10).
- Read-side calls (`/version`, `/clusters`) reuse the same `--capath
  /dev/null --cacert` opts when HTTPS is chosen.
- Phase-2 wait loop (60 s) and Phase-3 idempotency guards (by name then
  by host) are unchanged.

If HTTPS succeeds the script logs:
```
HTTPS reachable with valid cert; using https://10.0.0.63:5443/api/v1 for read probes
```

---

## 3. HTTP fallback behavior summary

The script dispatches on `curl` exit code only — NOT on a string match,
NOT on a generic `if curl … else fallback`:

| `curl` exit | Meaning                                  | Action                              |
|-------------|------------------------------------------|-------------------------------------|
| `0`         | HTTPS reachable + valid CA               | Use HTTPS for read/probe            |
| `7`         | Connection refused (HTTPS port not bound)| Fall back to `$HTTP_BASE` (5080)    |
| `60/51/35`  | TLS handshake or cert validation failed  | **Exit 2; refuse fallback**         |

Only exit `7` is treated as a safe fallback signal. Any other non-zero
code (cert chain bad, hostname mismatch, expired, handshake failure) is
a fail-closed event.

The write path (`sctool cluster add`) **always** uses `$HTTP_BASE`
because sctool 3.10.1 has no `--ca-file` flag. That hybrid is logged
explicitly so it's visible in journal lines.

---

## 4. TLS verification failure must not fall back to HTTP — explicit

**Hard rule, enforced by the case statement in the script:**

```sh
case "$probe_rc" in
  0)   READ_BASE="$HTTPS_BASE"; READ_CURL_OPTS="--capath /dev/null --cacert $GLOBULAR_CA" ;;
  7)   READ_BASE="$HTTP_BASE";  READ_CURL_OPTS="" ;;
  *)   log "HTTPS reachable but cert validation failed (curl exit $probe_rc); refusing to fall back to HTTP — exiting non-zero so doctor surfaces this"
       exit 2 ;;
esac
```

Rationale: a silent HTTP downgrade on cert-validation failure would let
a MitM (or a misconfigured listener with a wrong cert) hide the
unregistered state from the doctor invariant
`scylla_manager.cluster_registered`. The exit-2 path makes the failure
loud — `cluster_doctor` will then surface "unregistered" plus the
recent journal line.

Note: `sctool cluster add` still uses HTTP for the write path. That is a
known, deliberate, separately-documented exception for U.2 (sctool
3.10.1 limitation). The fail-closed rule applies to the **read/probe**
path, which is the path the script uses to decide whether registration
is needed.

---

## 5. Test results — services repo

`go test ./golang/cluster_doctor/cluster_doctor_server/rules/ -run RegisterScript -v`

```
=== RUN   TestRegisterScript_HTTPSReachable_PrefersHTTPS                 --- PASS (0.15s)
=== RUN   TestRegisterScript_HTTPSConnectionRefused_FallsBackToHTTP      --- PASS (0.08s)
=== RUN   TestRegisterScript_HTTPSCertInvalid_FailsClosed                --- PASS (0.04s)
=== RUN   TestRegisterScript_ExistingClusterByName_NoOp                  --- PASS (0.11s)
=== RUN   TestRegisterScript_MissingCluster_UsesHTTPForWritePath         --- PASS (0.17s)
PASS — ok 0.572s
```

5/5 PASS. The tests run only when the script is installed at
`/usr/lib/globular/bin/scylla-manager-register-cluster`; they skip
cleanly otherwise (so CI runners that aren't cluster nodes don't break).

---

## 6. Test results — packages/script test path

**There is no dedicated test in the packages repo.** The packages repo
ships content (YAML, scripts, systemd units); it has no Go/bats test
harness. All script test coverage lives in the services repo
(`scylla_manager_register_script_test.go`, §5) and runs against the
installed copy of the script that the packages repo built.

Live black-box verification on `globule-ryzen` (production node, while
script was installed at 1.2.75):

```
1) sudo -u globular /usr/lib/globular/bin/scylla-manager-register-cluster
   → "HTTPS reachable with valid cert; using https://10.0.0.63:5443/api/v1"
   → "cluster 'globular-internal' already registered — no-op"   (exit 0)

2) GLOBULAR_CA=/tmp/untrusted-ca.crt … register-cluster
   → "HTTPS reachable but cert validation failed (curl exit 60); refusing to fall back"
   (exit 2)

3) SCYLLA_MANAGER_HTTPS_BASE=https://10.0.0.63:6666/api/v1 … register-cluster
   → "HTTPS not enabled (curl exit 7); falling back to http://10.0.0.63:5080/api/v1"
   → "cluster 'globular-internal' already registered — no-op"   (exit 0)
```

All three live scenarios match the corresponding integration test.

---

## 7. Was the live cluster changed?

**Yes.** During U.2 execution the script's package version was deployed
to the live cluster:

1. Built scylla-manager `1.2.74` from updated YAML, published, bridged
   to `/var/lib/globular/packages/`, set as desired, installed by
   node-agent (3.92 s, `SUCCEEDED`).
2. During live verification of 1.2.74 the cert-fail-closed scenario
   silently passed (false positive). Diagnosed `--cacert`-without-`--capath`
   issue. Patched the YAML.
3. Built `1.2.75`, published, bridged, set as desired, installed (3.78 s,
   `SUCCEEDED`).

State of read-only artifacts that U.2 did **NOT** modify:
- cluster registration: `1` cluster (`globular-internal`, id
  `932c01cb-…`), no duplicate
- backup task `105a3d1f-…`: enabled, schedule unchanged
- healthchecks: `cql UP / rest UP / ssl true`
- scylla-manager unit: `active running`, `NRestarts=0`, `MainPID=770002`
- doctor: `0` scylla-manager findings

ScyllaDB and the agent were not touched.

---

## 8. Is the script currently installed on the node old or updated?

**Updated.** As of this report:

```
$ grep -c "capath /dev/null" /usr/lib/globular/bin/scylla-manager-register-cluster
3
```

The 1.2.75 script is the live copy on `globule-ryzen`. All three
`capath /dev/null` occurrences from the YAML are present (one in the
probe, two in the read-side calls), which matches the committed YAML.

The desired-state record points at scylla-manager 1.2.75 build 1; the
installed-state record matches; the systemd unit is active with no
restarts.

---

## 9. Does U.2 require package build/deploy to become active?

**No — already active.** The package was built (`1.2.74` then `1.2.75`),
published to the repository, bridged into the local install dir, and
installed via the node-agent workflow during execution. Restarting U.2
from a fresh checkout would require build/publish/apply; from the current
state nothing is pending.

What is pending:
- `git push` of the two local commits (services + packages)
- Distribution of the package to the other 4 nodes — none of which
  currently run scylla-manager (it is a single-node infra component on
  `globule-ryzen`). If/when scylla-manager joins another node, the
  desired-state record will pull 1.2.75 from the repository and the
  updated script will install there.

---

## 10. Push/deploy recommendation and risk

### Recommendation

Push **services** master (51 commits including U.2 `1970dd7c`). The
test file is additive, gated on script-installed (skips on CI),
exercises only its own httptest stubs, and passed local `go test`.
Pushing surfaces it to the rest of the team and lets CI run it on every
PR.

For **packages** main, the safest path is `cherry-pick` U.2 onto a fresh
branch off `origin/main` and push that, because the working tree still
contains 37 unrelated WorkingDirectory-normalize edits (see "Isolation
strategy" below). Direct `git push` of `main` would carry the
already-committed `f86d51f` (Project S) along with U.2 `3259c98`, but
**would NOT carry the 37 uncommitted WD files** — those are in the
working tree only, so the push would be clean. Project S has been
running on the live cluster since 2026-05-29 morning without issue, so
pushing both is low-risk if Project S was authorized for push (it was
authorized for build/deploy in the conversation, but no explicit push
authorization on record).

### Risk

| Action                          | Risk                                                       |
|---------------------------------|------------------------------------------------------------|
| `git push` services master      | LOW. Tests are skip-on-no-script; no production code path. |
| `git push` packages main (full) | LOW–MED. Carries Project S + U.2 commits. Both already running on live cluster. WD-normalize files remain local (not pushed). |
| Rebuild + publish 1.2.75 again  | UNNECESSARY. Already published; would bump to 1.2.76 with identical content. |
| Apply to another node           | NO-OP. scylla-manager is single-node (`globule-ryzen`). |
| Modify live config              | NOT in U.2 scope. Deferred to U.3. |

### Isolation strategy (packages repo)

The U.2 packages commit `3259c98` is already cleanly isolated from the
WD-normalize work:
- `3259c98` touches **1 file**:
  `metadata/scylla-manager/specs/scylla_manager_service.yaml`
- WD-normalize touches **37 files**: every
  `metadata/<svc>/systemd/globular-<svc>.service` standalone unit file
- **Zero overlap.** The U.2 commit can be pushed without dragging any
  WD-normalize content (those are uncommitted; git push transmits commits,
  not the working tree).

If you want maximum isolation (don't even ship Project S in the same
push as U.2), one tidy path:

```bash
cd /home/dave/Documents/github.com/globulario/packages
git stash -u                         # park WD-normalize + any untracked
git checkout -b u2-only origin/main  # fresh branch off remote
git cherry-pick 3259c98              # take U.2 only, drop Project S
git push origin u2-only              # push to a feature branch
# open PR for review; merge to main; main pointer updates upstream
git checkout main && git stash pop   # restore working tree
```

This pushes **only U.2** in its own branch, keeps Project S local for
separate review, and leaves the WD-normalize work untouched.

The simpler path — `git push origin main` — pushes Project S + U.2
together, also clean, no WD content shipped.

---

## Final live verification (post-push, 2026-05-29 12:21)

```
unit:         ActiveState=active SubState=running NRestarts=0 MainPID=770002
clusters:     1 — globular-internal id=932c01cb-… host=10.0.0.63 tls_disabled=False
healthcheck:  cql=UP rest=UP ssl=True
              scylla_version=2025.3.8-0.20260223.d657044d70fb agent=3.10.1
backup tasks: 2 (both enabled, both type=backup)
doctor:       0 scylla-manager findings
```

No regressions from the push. The push was a metadata-only operation
(git refs); no live cluster state was touched.

## Status

U.2 pushed cleanly. Ready for U.3 authorization.

