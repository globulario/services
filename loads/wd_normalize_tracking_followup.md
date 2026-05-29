# WD-normalize tracking follow-up

**Date:** 2026-05-29
**Status:** Deferred. Tracking only.
**Parent decision:** `loads/live_package_deployment_plan_after_packages_merge.md` §"Accepted operator decision"

This document tracks the WD-normalize work that is **not** being
actively deployed. Its purpose is to give a future operator the
information needed to either:

1. Confirm opportunistic refresh has retired the dormant cluster_doctor
   finding `754027b85c39913a`, or
2. Detect that one of the trigger conditions in §4 has fired and
   justifies switching from deferred to active deployment.

---

## 1. Service inventory at deferral time (2026-05-29)

Totals: **23 BARE + 8 already-OK + 5 inert + 1 inactive = 37** —
matches PR #2's scope.

### 1a. 23 BARE services — will benefit from re-deploy when one happens

These services have a live deployed unit
(`/etc/systemd/system/globular-<svc>.service`) that contains a bare
`WorkingDirectory={{.StateDir}}/<svc>` line (no `-` prefix). Each is
currently `active`, `NRestarts=0`, and shows no CHDIR failures in
journal. The cluster_doctor convergence finding
`754027b85c39913a` lists each of these by unit name.

```
ai-executor
ai-memory
ai-router
ai-watcher
authentication
backup-manager
blog
catalog
conversation
echo
event
file
ldap
log
mail
mcp
media
monitoring
persistence
rbac
repository
sql
torrent
```

### 1b. 8 already-OK services — re-deploy is a no-op for WD

These services were previously redeployed (probably during earlier
Project work) and already carry `WorkingDirectory=-{{.StateDir}}/<svc>`
on disk. PR #2's normalization is harmless but adds no value here.
Tracking is for completeness only.

```
cluster-controller
dns
node-agent
resource
search
storage
title
workflow
```

### 1c. 5 inert services — PR #2's WD-normalize does not affect the deployed unit

These packages either render no `WorkingDirectory=` line into the
deployed unit (the install pipeline reads the embedded unit from the
spec YAML, which omits WD), or use a non-`{{.StateDir}}/<svc>` form
that PR #2 leaves alone. Re-deploying these packages will not change
the deployed unit's WorkingDirectory behavior.

```
envoy
gateway
scylla-manager
scylla-manager-agent
xds
```

### 1d. 1 inactive service — orthogonal flag

```
discovery
```

`globular-discovery.service` is currently `inactive`. WD-normalize
status is moot until the operator decides whether discovery should
remain off or be re-enabled. **Out of scope for WD-normalize
tracking.**

---

## 2. Why deployment is deferred

The cluster_doctor finding `754027b85c39913a` is *latent*, not
*active*. It warns that any of the 23 BARE units **could** fail with
`status=200/CHDIR` if `{{.StateDir}}/<svc>` were missing at unit-start
time. On a healthy node where each `/var/lib/globular/<svc>/`
directory has been created and persisted for weeks, the BARE form
behaves identically to the normalized form.

Evidence the failure mode is not currently triggering:

| Signal | Observation |
|---|---|
| `NRestarts` across all 36 active services | 0 |
| `StartLimitHit` state | none observed |
| Journal CHDIR errors in last 7 days | none |
| Service unavailability events | none |

The cost of forced refresh:

- 23 service restarts spread across the cluster, each ~5–15 seconds
  of unavailability for that service's clients.
- 23 `pkg build` + 23 `pkg publish` + 23 `services desired set`
  operations.
- Risk that any one of the 23 reveals an unrelated regression in its
  install workflow (none expected, but any deploy carries baseline
  risk).

The benefit of forced refresh:

- The finding's unit-name list shortens by 23 (from 23 to 0 over
  time). Visual cleanup of the doctor report.
- A latent risk becomes inert. The dormant failure mode is
  permanently retired.

Cost > benefit at this moment. Defer.

---

## 3. Per-service checklist on opportunistic refresh

When a future routine bump touches any of the 23 BARE services —
e.g. an unrelated bug fix to `ai-memory`, a security update to
`authentication`, a feature ship in `mail` — apply the following
checklist at the same time. The marginal cost is near-zero because
the operator was already going to publish + re-deploy that package.

For service `<svc>`:

1. **Before publish:** confirm the package source on
   `origin/main` contains the normalized line:

   ```bash
   git -C /home/dave/Documents/github.com/globulario/packages \
       show origin/main:metadata/<svc>/systemd/globular-<svc>.service \
     | grep -E "^WorkingDirectory=-"
   ```

   Should print one matching line. If it doesn't, the package has
   somehow regressed — investigate before proceeding.

2. **Build & publish the package** as usual for whatever reason
   prompted the bump.

3. **After install completes**, confirm the deployed unit carries the
   normalized form:

   ```bash
   sudo grep -E "^WorkingDirectory=" \
        /etc/systemd/system/globular-<svc>.service
   # Expect: WorkingDirectory=-/var/lib/globular/<svc>
   ```

4. **Mark this service as retired** from the BARE list — move its
   name from §1a to §1b in a future revision of this document.

5. **Run a fresh cluster_doctor report** and confirm the finding
   `754027b85c39913a`'s summary no longer lists `<svc>`:

   ```
   mcp__globular__cluster_get_doctor_report freshness=fresh
   ```

   The finding will remain alive (still has other BARE services in
   its summary list) until the last BARE service is retired. Then
   the finding clears entirely on next snapshot.

---

## 4. Trigger conditions that would justify active deployment

Active forced deployment becomes justified only if **at least one** of
the following occurs. Until then, defer.

### a) A service actually fails to start because `{{.StateDir}}/<svc>` is missing

The latent failure mode becomes active. Symptoms: a service unit
reports `status=200/CHDIR` in `systemctl status`; the unit fails to
reach `active`; restart loop appears (`NRestarts > 0`) and may
escalate to `StartLimitHit`.

If observed, **immediately** force-redeploy the affected package
(and consider sweeping its peers as a precaution).

### b) cluster-doctor escalates the WD finding from WARN to ERROR

If `754027b85c39913a` (or its successor) appears with `severity:
error` rather than the current `severity: warn`, the doctor has
detected an active failure. Treat as condition (a).

### c) The service is already being updated for unrelated reasons

A routine bug fix, feature ship, security update, dependency bump,
or any other reason that already requires `pkg build` + `pkg publish`
+ `services desired set` for that service. WD-normalize comes along
for free — per the checklist in §3.

### d) A maintenance window is explicitly opened

If the operator opens a planned window (e.g., for ScyllaDB
upgrade, for VIP/keepalived rework, for a major Globular version
bump), the WD-normalize campaign can be slotted in as one of the
window's work items. The blast radius is acceptable inside a window
that is already absorbing planned restarts.

### e) A package is being rebuilt for a required security or fix release

Any forced rebuild — e.g., a CVE patch, a binary correctness fix —
should incorporate WD-normalize for the same service at the same
time. The package version bump that the rebuild triggers absorbs
WD-normalize without additional risk.

---

## 5. How to confirm the finding count decreases over time

The cluster_doctor finding `754027b85c39913a` carries a summary line
listing every still-bare unit:

```
"systemd unit(s) have bare required WorkingDirectory under /var/lib/globular
(will fail status=200/CHDIR if dir missing): globular-ai-executor.service,
globular-ai-memory.service, ..., globular-torrent.service"
```

To track progress:

```bash
mcp__globular__cluster_get_doctor_report freshness=fresh \
  | jq -r '.findings[]
           | select(.finding_id == "754027b85c39913a"
                    or (.summary | contains("bare required WorkingDirectory")))
           | .summary'
```

Count the comma-separated entries. At deferral time the count is 23.
The expectation is that each opportunistic refresh decrements that
count by 1. When the count reaches 0, the finding clears entirely
from the next snapshot.

A simple monthly check is sufficient. Suggested cadence: review the
count once per month; if no progress is observed and the count has
not changed in 90 days, consider whether the cluster is genuinely in
a deploy-quiet state (everything is stable, nothing else needs
shipping) and whether the §4(d) "open a maintenance window" trigger
should fire.

---

## 6. Side notes

- **scylla-manager** is **not in the BARE list.** Its live unit has
  no `WorkingDirectory=` line at all (the install pipeline uses the
  spec YAML's embedded unit content, which omits WD). PR #2's
  normalization of the standalone systemd file is inert at install
  time. Re-deploying scylla-manager will not affect WD-normalize
  status — this is documented in
  `loads/live_package_deployment_plan_after_packages_merge.md` §2 and
  §9.

- **discovery** is currently `inactive`. If/when discovery is
  re-enabled, it picks up whatever WD form ships in its package at
  that time. The current `origin/main` template is normalized, so a
  fresh start of discovery from the current packages would land with
  the normalized form.

- **8 already-OK services** (§1b) require no future action. They are
  listed solely so a future audit can confirm the inventory hasn't
  silently grown to include them again (which would indicate a
  package source regression).

- **5 inert services** (§1c) require no future action either. They
  will never appear in the WD finding regardless of redeploy state.

---

## Status

WD-normalize forced deployment deferred and tracking report written.
