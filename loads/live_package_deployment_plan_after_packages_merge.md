# Live package deployment plan after packages merge

**Date:** 2026-05-29
**Scope:** scylla-manager (post PR #1) + the 37 WD-normalize service
units (post PR #2). Plan is **for review** — no deployment is
authorized by this document.

This document records the inventory, groups, exact commands, and
preflight/post-checks an operator would run. It also makes the case
for a specific deployment posture.

---

## 1. Affected package inventory — current live state

Source: per-service `systemctl is-active`, `NRestarts`, and a grep of
each unit's `FragmentPath` for `WorkingDirectory=` (live `/etc/systemd/system/globular-<svc>.service`).

| Service | Active | NRestarts | Live unit WD | Notes |
|---|---|---|---|---|
| ai-executor | active | 0 | **BARE** | ai-memory workflow is abandoned (doctor `7e6d50b32893d56a`) |
| ai-memory | active | 0 | **BARE** | dispatched workflow blocked — see doctor finding |
| ai-router | active | 0 | **BARE** | |
| ai-watcher | active | 0 | **BARE** | |
| authentication | active | 0 | **BARE** | restart will briefly invalidate auth tokens; clients reconnect |
| backup-manager | active | 0 | **BARE** | |
| blog | active | 0 | **BARE** | leaf service |
| catalog | active | 0 | **BARE** | leaf service |
| cluster-controller | active | 0 | OK (-WD) | already normalized — **no-op** on redeploy |
| conversation | active | 0 | **BARE** | leaf service |
| **discovery** | **inactive** | 0 | (unit not running) | inactive; redeploy will start it — flag |
| dns | active | 0 | OK (-WD) | already normalized — **no-op** |
| echo | active | 0 | **BARE** | leaf service |
| envoy | active | 0 | (no WD line) | live unit has no `WorkingDirectory=` line — **PR #2 normalization is inert** |
| event | active | 0 | **BARE** | |
| file | active | 0 | **BARE** | |
| gateway | active | 0 | (no WD line) | **inert** — same as envoy |
| ldap | active | 0 | **BARE** | |
| log | active | 0 | **BARE** | |
| mail | active | 0 | **BARE** | |
| mcp | active | 0 | **BARE** | |
| media | active | 0 | **BARE** | |
| monitoring | active | 0 | **BARE** | |
| node-agent | active | 0 | OK (-WD) | already normalized — **no-op** |
| persistence | active | 0 | **BARE** | |
| rbac | active | 0 | **BARE** | restart briefly stalls authz; clients retry |
| repository | active | 0 | **BARE** | package registry — defer or batch-with-redeploy |
| resource | active | 0 | OK (-WD) | already normalized — **no-op** |
| scylla-manager-agent | active | 0 | (no WD line) | **inert** — live unit has no WD line |
| scylla-manager | active | 0 | (no WD line) | **inert** — spec YAML's embedded unit has no WD; standalone file is unused at install time |
| search | active | 0 | OK (-WD) | already normalized — **no-op** |
| sql | active | 0 | **BARE** | |
| storage | active | 0 | OK (-WD) | already normalized — **no-op** |
| title | active | 0 | OK (-WD) | already normalized — **no-op** |
| torrent | active | 0 | **BARE** | |
| workflow | active | 0 | OK (-WD) | already normalized — **no-op** |
| xds | active | 0 | (no WD line) | **inert** — live unit has no WD line |

### Inventory totals

| Bucket | Count | Composition |
|---|---|---|
| **Active services** | 36/37 | discovery is the lone inactive |
| **NRestarts=0** | 37/37 | no service in a restart loop |
| **WD-normalize would change live unit** | **23** | the BARE set |
| **WD-normalize is no-op (already -WD)** | 8 | cluster-controller, dns, node-agent, resource, search, storage, title, workflow |
| **WD-normalize is inert** (no WD line in live unit anyway) | 6 | envoy, gateway, scylla-manager, scylla-manager-agent, xds + (re-confirm for completeness) |

### Discovery anomaly

`globular-discovery.service` is `inactive` on the live node. The PR
#2 change to its template would apply on next install, but the
service is not running — so redeploying it would *start* it. That is
a behavior change beyond WD-normalize; flag for operator decision
(intentional shutdown? bug?) before any redeploy of the discovery
package.

---

## 2. scylla-manager — current package vs merged YAML

| Aspect | Current live | PR #1 (now on `origin/main`) |
|---|---|---|
| Deployed package version | 1.2.75 | (template only — no version on remote) |
| Deployed binary path | `/usr/lib/globular/bin/scylla_manager` | (n/a) |
| Registration script `--capath /dev/null --cacert` | **present** | **present** |
| Registration script HTTPS-first 3-way dispatch | **present** | **present** |
| Registration script `GLOBULAR_CA` env binding | **present** | **present** |
| Deployed unit `WorkingDirectory=` | (none) | (none — embedded in spec YAML) |
| Standalone `metadata/scylla-manager/systemd/globular-scylla-manager.service` `WorkingDirectory=` | (n/a at install time) | normalized to `-{{.StateDir}}/scylla-manager` |
| Backup tasks | 2 enabled (`105a3d1f`, `3b966c52`) | (untouched by package source) |
| Backup data in MinIO | retained | (untouched) |
| Service active | yes | (n/a) |
| Cluster registered | 1 (`globular-internal` / `932c01cb-…`) | (untouched) |
| NRestarts | 0 | (n/a) |

### Conclusion — scylla-manager

The live scylla-manager 1.2.75 already contains the exact script
content that PR #1's YAML produces. A re-publish of the package
spec would produce **identical content** (same `--capath /dev/null
--cacert`, same idempotency, same HTTPS-first probe). The standalone
systemd file's WD-normalize is **inert** at install time because the
install pipeline reads the embedded unit from the spec YAML, which
has no `WorkingDirectory=` line.

**Re-deploying scylla-manager would be a no-op for behavior.** It
would only bump the version number (1.2.76+1) and trigger a
service restart for which the operator gets no functional benefit.

Recommendation: **skip scylla-manager re-deploy** for the WD-normalize
batch. Defer until either U.4 ships (which would change the script
to disable HTTP) or another script enhancement requires a real
content bump.

Rollback plan if a re-deploy is later authorized: `globular services
desired set scylla-manager 1.2.75 --build-number 1 --force` (re-pins
to the verified version). Backup state is independent of the package
and survives any rollback.

---

## 3. Health baseline (snapshot at planning time)

| Check | Status |
|---|---|
| `scylla-server` (ScyllaDB) | active |
| `globular-scylla-manager` | active, NRestarts=0, MainPID 770002 |
| `globular-cluster-doctor` | active, NRestarts=0, MainPID 983399 |
| `globular-node-agent` | active, NRestarts=0 |
| `globular-workflow` | active, NRestarts=0 |
| Active remediation workflows (last 60s) | none observed in journal |
| Doctor `tls_trust_failure` findings | 0 |
| Doctor `scylla_manager.cluster_registered` findings | 0 |
| Doctor `systemd_working_directory` finding | 1 (the cluster_doctor convergence finding `754027b85c39913a` — exactly the thing this work would resolve) |
| Doctor total findings | 24 (pre-merge baseline, unchanged) |
| Cluster registration | 1 (`globular-internal`) |
| Backup tasks | 2 enabled |

`etcd` daemon was checked via `systemctl is-active etcd` and reported
inactive — this is misleading: Globular runs etcd embedded
(typically launched by `globular-node-agent` or under a different unit
name like `etcd-server.service`). The operator should confirm etcd
status via `etcdctl endpoint health` before any deployment, not via
the `etcd` unit name.

---

## 4. Deployment grouping for the 23 BARE-WD services

Groups are designed so each batch's blast radius is contained.
Within a group, packages may be deployed sequentially (one at a time)
or in small parallel batches; never the whole group at once.

### Group A — application leaves (13 services, lowest risk)

```
blog, catalog, conversation, echo, event, file, ldap, log, mail,
media, sql, title (already OK — n/a), torrent
```

Filtered to BARE-only: **blog, catalog, conversation, echo, event,
file, ldap, log, mail, media, sql, torrent** (12 services).

- Restart of any single one does not affect control plane or other
  services.
- Brief stalls for direct clients are acceptable.
- Operator decision: choose 2–3 at a time, wait 60s between waves,
  let the doctor settle.

### Group B — AI services (4 services)

```
ai-executor, ai-memory, ai-router, ai-watcher
```

- ai-memory has an abandoned workflow finding (`7e6d50b32893d56a`).
  Resolve the workflow abandonment **before** deploying ai-memory —
  redeploy will restart the service and re-trigger dispatch; if the
  blocker is unresolved, the dispatch will abandon again and the
  finding worsens.
- ai-watcher, ai-executor, ai-router can deploy one-by-one after
  ai-memory state is sorted.

### Group C — observability / management (4 services)

```
backup-manager, mcp, monitoring, persistence
```

- Restarting `backup-manager` does not affect the underlying
  scylla-manager backup jobs (those are scylla-manager's own; backup-
  manager is a separate Globular shim).
- `mcp` restart briefly drops MCP clients; tool calls retry.
- `monitoring` restart drops Prometheus scrape until reconnect.
- `persistence` is sensitive — sequential deploy with ~60s gap.

### Group D — auth / identity (2 services)

```
authentication, rbac
```

- Restart drops in-flight token validations briefly. Mesh services
  retry; deploy outside peak hours.
- Deploy authentication first; wait for `globular cluster health` to
  settle; then rbac.

### Group E — repository (1 service — most sensitive non-control-plane)

```
repository
```

- `repository` is the package registry. Restarting it briefly stops
  *all* package install operations cluster-wide.
- Deploy **last**, when no other deployment is pending.
- Confirm no pending `services desired set` or in-flight install
  workflow before bouncing repository.

### Skipped (no-op or inert)

- 8 already-OK: cluster-controller, dns, node-agent, resource,
  search, storage, title, workflow
- 6 inert: envoy, gateway, scylla-manager, scylla-manager-agent, xds
  (+ revisit any one before authorizing re-deploy)
- discovery: inactive — operator decision required

---

## 5. Exact command plan per group (sketched, not authorized)

For each service `<svc>` in a group:

### Pre-checks (per service)

```bash
TOKEN=$(globular auth login --user sa --password adminadmin | awk '/^Token:/{print $2}')
globular --token "$TOKEN" cluster health                                 # cluster overall
sudo systemctl show globular-<svc>.service -p ActiveState,NRestarts,SubState
mcp__globular__cluster_get_doctor_report freshness=fresh | jq '.findings | length'
# capture baseline count for later diff
sudo systemctl is-active globular-<svc>.service                          # pre-baseline
```

Abort the wave if:
- cluster health is not `HEALTHY`,
- the service has `NRestarts > 0`, or
- the doctor reports any new ERROR finding since the prior wave.

### Build & publish (per service, with new patch version)

This step is repository-cluster-wide and only needs to run once per
service per change set. The packages source on `origin/main` already
carries the normalized WD; the package re-publish bakes that into a
new bundle.

```bash
PKG=/home/dave/Documents/github.com/globulario/packages
OUT=/tmp/wd_norm_pkgbuild
mkdir -p "$OUT"
WORK=$(mktemp -d)
ROOT="$WORK/<svc>"
mkdir -p "$ROOT/bin" "$ROOT/specs"
# Copy the in-tree binary (already-built) and spec.
cp -L "$PKG/bin/<svc>" "$ROOT/bin/<svc>" 2>/dev/null || true
cp "$PKG/metadata/<svc>/specs/<svc_service>.yaml" "$ROOT/specs/<svc_service>.yaml"

globular --token "$TOKEN" pkg build \
  --spec "$ROOT/specs/<svc_service>.yaml" --root "$ROOT" \
  --version <current_version> --publisher core@globular.io \
  --platform linux_amd64 --out "$OUT"

globular --token "$TOKEN" pkg publish \
  --file "$OUT"/<svc>_<current_version>_linux_amd64.tgz \
  --bump patch --repository 10.0.0.63:443
```

### Apply (per service)

```bash
# Bridge into local install dir to avoid the install-loop class
sudo install -m 0644 "$OUT"/<svc>_<current_version>_linux_amd64.tgz \
  /var/lib/globular/packages/<svc>_<NEW_VERSION>_linux_amd64.tgz

# Set desired
globular --token "$TOKEN" services desired set <svc> <NEW_VERSION> \
  --build-number 1 --force

# Wait for install workflow SUCCESS
until sudo journalctl -u globular-node-agent.service --since="20 seconds ago" \
      | grep -qE "install-package <svc> .*SUCCEEDED|install-package <svc> .*FAILED"; do
  sleep 6
done
```

### Post-checks (per service)

```bash
# Confirm the deployed unit now has '-WorkingDirectory'
sudo grep -E "^WorkingDirectory=" /etc/systemd/system/globular-<svc>.service
# Should print: WorkingDirectory=-/var/lib/globular/<svc>

# Confirm the service came back up
sudo systemctl show globular-<svc>.service -p ActiveState,NRestarts,SubState
# Expect: active / 0 / running

# Confirm doctor's WD finding count decreased by 1 (or by 23 once full batch done)
mcp__globular__cluster_get_doctor_report freshness=fresh \
  | jq '.findings[] | select(.summary | contains("systemd unit(s) have bare required WorkingDirectory"))'
```

Abort and rollback if:
- the install workflow returns `FAILED`,
- the service does not become `active` within 60s,
- the deployed unit still has bare `WorkingDirectory=`, or
- the doctor reports a new ERROR finding within 2 minutes.

### Rollback (per service)

```bash
# Re-pin to the prior version
globular --token "$TOKEN" services desired set <svc> <PRIOR_VERSION> \
  --build-number <PRIOR_BUILD> --force
```

State that does not roll back:
- the package version published is permanent (the repository keeps
  every published version; a rollback re-pins desired to the prior).
- restart counters on the service unit.

---

## 6. Cluster-wide preflight checklist (run once before each group)

- [ ] `globular cluster health` → status: HEALTHY
- [ ] `etcdctl endpoint health` (via `node-agent`'s etcdctl) → all
      endpoints healthy
- [ ] `sudo systemctl is-active scylla-server` → active
- [ ] `globular --token $TOKEN cluster health` summary shows 1
      healthy node, 0 unhealthy
- [ ] no in-flight `services desired set` workflow (check
      `mcp__globular__workflow_list_runs`)
- [ ] no service has `NRestarts > 0`
- [ ] no service is in `StartLimitHit` (`systemctl status --all 2>&1
      | grep StartLimitHit`)
- [ ] doctor's baseline finding count is recorded (current: 24); no
      new finding from a prior wave is unresolved
- [ ] backup tasks for scylla-manager remain 2 enabled (only relevant
      if the batch touches the scylla-manager package — but per §2,
      it does not)

If any item fails, **defer the group**.

---

## 7. Post-batch verification (run after each group)

- [ ] every service in the group is `ActiveState=active` with
      `NRestarts=0`
- [ ] every service's deployed unit shows `WorkingDirectory=-...`
- [ ] doctor's `systemd unit(s) have bare required WorkingDirectory`
      finding has FEWER service names listed than before the batch
      (target: zero remaining after all groups complete)
- [ ] doctor's total finding count is at most baseline + 1 (allow
      one transient artifact-cache mismatch to appear)
- [ ] cluster health remains HEALTHY
- [ ] scylla-manager backup tasks remain 2 enabled (sanity check)

---

## 8. Stop conditions

Halt the deployment campaign immediately and run no further waves
if any of the following happens:

1. A service fails to become active within its expected restart
   window after deploy.
2. The doctor reports a new ERROR finding within 2 minutes of a
   deploy.
3. `cluster health` flips from HEALTHY to DEGRADED or worse.
4. `NRestarts > 0` appears on any service that was restarted in the
   current batch.
5. The `scylla_manager.cluster_registered` finding fires (would
   indicate scylla-manager lost cluster registration — should NOT
   happen since scylla-manager is skipped).
6. The `tls_trust_failure` finding fires (would indicate doctor lost
   trust in scylla-manager HTTPS — should NOT happen since
   scylla-manager is skipped).
7. Backup tasks drop from 2 enabled to fewer.
8. `etcd` quorum is lost (check `etcdctl endpoint health` mid-batch).

---

## 9. Should scylla-manager be redeployed first, last, or skipped?

**Skipped.** Per §2, the live scylla-manager 1.2.75 contains the
exact script content PR #1's YAML produces; the standalone systemd
file PR #2 normalized is **inert** at install time because the install
pipeline reads the embedded unit from the spec YAML, which has no
`WorkingDirectory=` line.

Re-publishing scylla-manager would:
- create a 1.2.76+1 package with identical script content
- trigger a service restart for which the operator gets no
  functional benefit
- create a brief gap in the registration-script invariant probe
  (cluster_doctor would see HTTPS go briefly unavailable as
  scylla-manager restarts)

These costs exceed any benefit. Skip until a real content change
arrives (U.4 or beyond).

## 10. Should WD-normalize be applied opportunistically?

**Yes — strongly recommended.**

The BARE-WD failure mode (`status=200/CHDIR` on missing
`{{.StateDir}}/<svc>`) only triggers if the working directory is
*actually missing* when systemd starts the unit. On a healthy node
with `/var/lib/globular/<svc>/` long-since created and populated,
the BARE form behaves identically to the normalized form.

The 23 affected services have been running for weeks with the BARE
form without restart loops or CHDIR failures (`NRestarts=0` for all
36 active services). The doctor's finding is a *latent* warning,
not an active failure.

Therefore:
- Forcing 23 cluster-wide restarts now to fix a dormant risk is
  poor risk/reward.
- Opportunistic application — i.e., let WD-normalize land on the
  next time each service gets a normal version bump for unrelated
  reasons — captures the benefit without the churn.
- The dormant finding `754027b85c39913a` is acceptable to carry until
  the next routine deploy cycle picks each package up.

The alternative — a campaign of 23 forced redeploys grouped over a
maintenance window — is a defensible operator choice, but should not
be initiated without a stated reason beyond resolving the dormant
finding.

---

## Next authorized action

**Next authorized action should be: defer deployment.**

Specifically:

- Do not redeploy scylla-manager (skip per §9 — the deployed package
  is content-equivalent to the merged source).
- Do not run a campaign of 23 forced redeploys (defer per §10 —
  apply opportunistically as each package gets a normal version bump
  for unrelated reasons).
- Leave the dormant cluster_doctor finding `754027b85c39913a` in
  place as a tracking signal; it will resolve incrementally as
  packages get normal version bumps over time.
- If a maintenance window is later scheduled and the operator wants
  to retire the finding faster, execute Groups A → B → C → D → E in
  order over that window, applying the per-service pre-checks /
  apply / post-checks / stop-conditions documented in §§5–8.

This document does not authorize either path. The recommendation is
**defer** — the source side of the reconciliation is complete; the
live side already runs the relevant scylla-manager binary; and the
WD-normalize latent risk is best resolved organically.

---

## Accepted operator decision (2026-05-29)

- **Forced WD-normalize rollout deferred.** No campaign of 23 redeploys
  will be initiated to retire the dormant doctor finding.
- **scylla-manager redeploy skipped.** The deployed 1.2.75 package is
  content-equivalent to PR #1's source; re-publishing would be a
  behavior no-op with restart cost.
- **WD-normalize will be applied opportunistically** during normal
  future package updates. Each affected package picks up the
  normalized template on its next routine version bump for unrelated
  reasons.
- **Doctor finding remains accepted as a latent tracking signal**
  until services are naturally refreshed. Its membership list (the
  enumeration of unit names in the finding's summary) will shrink
  organically as packages are refreshed.

The acceptance rationale (paraphrased from the requesting message):

1. scylla-manager package content is already equivalent to the
   merged Project S/U.2 package source.
2. Re-publishing scylla-manager would be a behavior no-op with restart
   cost.
3. PR #2 WD-normalize only affects templates for future
   installs/reinstalls.
4. 23 services still have live bare `WorkingDirectory=` lines but
   there are no restart loops, no CHDIR failures, no `StartLimitHit`
   state, and `NRestarts=0` across the inventory.
5. The failure mode is latent, not active.
6. Forced redeploy would create unnecessary restart blast radius.

A follow-up tracking report is maintained at
`loads/wd_normalize_tracking_followup.md` to enumerate the BARE/OK/
inert sets, document the deferral reason, list trigger conditions
that would justify a later active deployment, and provide a
per-service checklist for the opportunistic-refresh path.
