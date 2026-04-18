# Deployment Philosophy

This document explains the design principles behind Globular's deployment pipeline, why automatic rollback is forbidden, and how the system degrades gracefully when infrastructure is unavailable.

---

## The Escalator Principle

> "An escalator can never break: it can only become stairs. Sorry for the convenience."
> — Mitch Hedberg

Globular's deployment pipeline is designed like an escalator. When the automated pipeline is healthy, it carries your deployments smoothly — you publish a package, set the desired state, and the reconciler rolls it out across every node. But when the pipeline breaks (repository down, ScyllaDB recovering, controller unreachable), it doesn't stop working. It becomes stairs:

| Automated (escalator) | Manual (stairs) |
|----------------------|-----------------|
| `globular deploy --bump` | `go build && scp && systemctl restart` |
| Reconciler converges desired state | Human copies binary to the right place |
| Workflow orchestrates fetch → install → verify | Human verifies the service starts |
| Repository resolves versions | Human knows which binary to deploy |

The automated path is convenient. The manual path always works — even when the cluster is on fire.

This is a deliberate design choice. The deployment pipeline has **zero dependencies that aren't also available to a human with SSH access**. Binaries are regular files in `/usr/lib/globular/bin/`. Services are regular systemd units. Configuration lives in etcd, which runs on every node. There is no opaque runtime, no container image registry, no admission controller that can silently block you.

When infrastructure recovers, the escalator resumes. No special recovery procedure needed — the reconciler picks up where it left off.

---

## Why Automatic Rollback Is Forbidden

The software industry borrowed "rollback" from database transactions and applied it to deployments. This analogy is broken.

### Database rollback vs. software rollback

A database transaction rollback works because:
- The database controls **all** the state
- Transactions are **isolated** — no other process saw the uncommitted data
- Rollback restores a **consistent** previous state
- The operation is **atomic** — it either fully reverts or doesn't

A software deployment has none of these properties:
- State is distributed across nodes, databases, caches, and user expectations
- Other services may already depend on new APIs, schemas, or protocols
- The "previous state" may not be consistent with the current data
- Rollback on one node while others run the new version creates split-brain

### You can't un-cook an egg

When version 0.1.0 is deployed:
- Schema migrations may have already run
- New RPCs may be called by other services
- Users may be using new features in the interface
- Configuration may reference new fields

Installing version 0.0.1 doesn't reverse any of this. It creates a mismatch between the code and everything around it. The "safe" rollback is often more dangerous than the original failure.

### What actually happened

This isn't theoretical. A power outage taught us:

1. ScyllaDB went down on two nodes — needed 30 seconds to recover
2. The heartbeat's process fingerprinting tried to call the repository (which depends on ScyllaDB)
3. Heartbeats hung — the controller marked both nodes unreachable
4. The reconciler tried to "fix" the nodes by reinstalling packages
5. The repository returned version 0.0.1 — the first version ever built
6. Two nodes were silently reverted to ancient code that couldn't even heartbeat

The services were **not faulty**. They were killed by a power outage and just needed infrastructure to recover. The system's "safety" mechanism caused the actual outage.

### The correct response to failure

A service crash is an **incident**, not a deployment failure:

1. **Detect** — systemd reports the service exited, ai_watcher creates an event
2. **Diagnose** — was it a bug? OOM? External event? Infrastructure failure?
3. **Decide** — a human determines the right action: restart, fix forward, or (rarely) roll back
4. **Execute** — if rollback is truly needed, a human explicitly runs it with `--force`

The system's job is to **observe and report**, not to guess and act.

---

## Deployment Guards

Three guards prevent the cluster from silently reverting to old versions:

### 1. Lean heartbeat (node-agent)

The heartbeat reports "I'm alive, here's what I have" every 30 seconds. It uses only:
- **Phase 1**: Local discovery — systemd units, version markers, config files
- **Phase 2**: etcd installed_state — the registry of what was installed through the pipeline

It does **not** call the repository, compute binary checksums, or perform reverse-lookups. The heartbeat has zero external service dependencies beyond etcd (which runs on every node).

If you need drift detection (is the running binary different from what etcd says?), that's a diagnostic tool — run it on demand, not in the heartbeat critical path.

### 2. Unconditional downgrade guard (node-agent)

When the node-agent receives an `ApplyPackageRelease` request, it compares the requested version against what's currently installed. If the requested version is **older**, the request is rejected:

```
refuse to downgrade workflow/SERVICE from 0.1.2+1 to 0.0.1+0
— automatic rollback is forbidden (use Force=true for manual rollback)
```

This guard applies **unconditionally** — regardless of build_id, regardless of who sent the request. The only override is `Force=true`, which must be set explicitly by a human through the CLI.

### 3. Version sanity check (controller)

When the controller resolves a desired version through the repository, it verifies that the resolved artifact is not older than what was requested. If the repository returns 0.0.1 when 0.1.0 was desired, the release **fails** instead of proceeding:

```
REJECTED — repository returned 0.0.1 but desired is 0.1.0
— refusing to install older version
```

This catches the case where the repository contains stale artifacts that don't match the desired state.

---

## The Two Deployment Paths

### Path 1: Automated pipeline (day-to-day)

```
Developer                Controller              Node Agent
    |                        |                        |
    |-- globular deploy -->  |                        |
    |   (publish + bump      |                        |
    |    desired state)      |                        |
    |                        |-- resolve version ---> |
    |                        |   (repository lookup)  |
    |                        |                        |
    |                        |-- create workflow ---> |
    |                        |   (fetch, install,     |
    |                        |    restart, verify)    |
    |                        |                        |
    |                        |<-- heartbeat --------- |
    |                        |   (new version         |
    |                        |    confirmed)           |
```

This path requires: repository, MinIO, ScyllaDB, workflow service, controller leadership, healthy mesh.

### Path 2: Manual deployment (emergency)

```bash
# Build from source
cd golang && go build -o /tmp/binaries/ ./...

# Ship to nodes
tar czf /tmp/binaries.tar.gz -C /tmp/binaries .
scp /tmp/binaries.tar.gz node:/tmp/

# Deploy
ssh node "sudo systemctl stop globular-node-agent.service && \
          cd /usr/lib/globular/bin && \
          sudo tar xzf /tmp/binaries.tar.gz && \
          sudo systemctl start globular-node-agent.service"
```

This path requires: SSH access, Go compiler. Nothing else.

When the automated pipeline recovers, it sees the new binaries through the heartbeat and updates its state accordingly. No conflict, no re-deployment — the escalator picks you up where the stairs left off.

---

## Artifact Lifecycle

Not every version that was ever built should remain installable. The repository should maintain a clear lifecycle:

| State | Meaning | Installable? |
|-------|---------|-------------|
| **PUBLISHED** | Current release, actively deployed | Yes |
| **DEPRECATED** | Superseded by newer version, still functional | Yes (with warning) |
| **YANKED** | Known-bad, should not be installed | No |
| **ARCHIVED** | No node references it, retained for audit | No |

The resolver only returns **PUBLISHED** or **DEPRECATED** artifacts. Ancient versions with zero references should be archived automatically — still stored in MinIO for compliance, but invisible to the deployment pipeline.

This prevents the scenario where the reconciler accidentally grabs a 2-year-old artifact because it's the only version the repository can find.

---

## Summary

| Principle | Rule |
|-----------|------|
| Escalator, not elevator | When automation breaks, manual path always works |
| No automatic rollback | Crash = incident, not deployment failure |
| Heartbeat is sacred | No external dependencies in the critical path |
| Forward only | Guards reject older versions unconditionally |
| Human decides | Rollback requires explicit `--force` from a human |
| Clean repository | Unreferenced versions get archived, not served |
