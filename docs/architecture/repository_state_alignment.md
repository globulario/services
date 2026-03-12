# Repository & State Alignment Design Doc

**Status:** Approved
**Date:** 2026-03-11
**Scope:** Vocabulary freeze, current-state map, target-state model, migration path

---

## 1. Problem Statement

After a Day-0 bootstrap the platform has running services, but:

- The **Repository** is empty (no artifacts published).
- The **Cluster Controller** has no desired releases (seed may have failed silently).
- The **Admin UI** shows every service as "Unmanaged" and the catalog tab is blank.

The root cause is that three independent state systems were never required to converge:

| System | What it knows | How it learns |
|--------|--------------|---------------|
| Installer (disk/systemd) | Binaries exist, units enabled | Runs spec steps at Day-0 |
| Repository service | Artifact manifests + blobs | `globular pkg publish` (never called by Day-0) |
| Controller desired state | What the cluster *should* run | `globular services seed` (fragile one-shot) |
| Node Agent installed state | What *is* installed per node | Scans disk markers + systemd + config files |

These four never reconcile automatically, so the operator sees a lie.

---

## 2. Vocabulary (Frozen Definitions)

Every status term below has exactly one meaning. Code, UI, CLI, and docs must use these terms consistently.

### 2.1 State Layers

| Layer | Owner | Canonical Storage | Meaning |
|-------|-------|-------------------|---------|
| **Artifact state** | Repository service | `artifacts/{pub}%{name}%{ver}%{plat}.manifest.json` | A deployable package exists and can be fetched |
| **Desired release state** | Cluster Controller | etcd `/globular/resources/{Type}/{Name}` | The cluster *intends* this package to be deployed |
| **Installed observed state** | Node Agent | etcd `/globular/nodes/{nodeID}/packages/{kind}/{name}` | The package *is* installed on a specific node |
| **Runtime health state** | Node Agent + systemd | Reported via `ReportNodeStatus` | The installed unit is actually running and healthy |

### 2.2 Derived Status Labels

These are computed by comparing the four layers. Nothing else may be used as a status label in the UI.

| Label | Definition | Layers involved |
|-------|-----------|----------------|
| **Available** | Artifact exists in Repository. No desired release. Not installed. | Artifact only |
| **Planned** | Artifact exists. Desired release exists. Not yet installed on all target nodes. | Artifact + Desired |
| **Installed** | Desired release exists. Installed-state record matches desired version. | Artifact + Desired + Installed |
| **Drifted** | Desired release exists. Installed version differs from desired version. | Desired + Installed (mismatch) |
| **Unmanaged** | Installed-state record exists. No corresponding desired release. | Installed only (no Desired) |
| **Missing in repo** | Desired release or installed record exists, but artifact not found in Repository. | Desired or Installed, but no Artifact |
| **Orphaned artifact** | Artifact exists. No desired release. No installation anywhere. | Artifact only (stale) |

### 2.3 Package Kinds

| Kind | Proto enum | Controller resource | Plan compiler | Node actions |
|------|-----------|-------------------|---------------|-------------|
| **SERVICE** | `ArtifactKind.SERVICE` (1) | `ServiceRelease` / `ServiceDesiredVersion` | `release_compiler.go` | `service.install_payload`, `service.restart` |
| **APPLICATION** | `ArtifactKind.APPLICATION` (2) | `ApplicationRelease` | `application_compiler.go` | `application.install` |
| **INFRASTRUCTURE** | `ArtifactKind.INFRASTRUCTURE` (5) | `InfrastructureRelease` | `infrastructure_compiler.go` | `infrastructure.install` |

All three kinds must flow through the same 4-layer model. No kind may be treated as a side-channel.

---

## 3. Current State (As-Is Workflow Map)

### 3.1 Day-0 Bootstrap Flow

```
install-day0.sh
       │
       ├─ 1. globular-installer install --spec <spec>
       │     Extracts tarball → writes binaries, configs, systemd units
       │     State written: DISK ONLY (version markers, config files, systemd units)
       │     Repository: NOT TOUCHED
       │     Controller: NOT TOUCHED
       │
       ├─ 2. systemctl enable + start
       │     All services running via systemd
       │     State: systemd knows, disk knows
       │
       └─ 3. globular services seed --insecure  (END OF SCRIPT)
              Attempts to backfill ServiceDesiredVersion into controller etcd
              Retries 6x with 30s initial delay
              On failure: prints warning, continues
              State: Controller MAY know about services (fragile)
              Repository: STILL EMPTY
```

### 3.2 Publish Flow (Separate Script)

```
test-publish-and-apply-services.sh
       │
       ├─ Phase 1: globular pkg publish --file <pkg>
       │     Uploads artifact to Repository service
       │     State: Repository now knows about packages
       │     Controller: NOT TOUCHED (no desired release created)
       │
       └─ Phase 2: globular-installer install (optional packages)
              Same as Day-0: disk-only install
              State: disk + systemd, no desired-state
```

### 3.3 Broken Points

| # | Gap | Effect |
|---|-----|--------|
| G1 | Day-0 never publishes to Repository | Repository catalog is empty |
| G2 | `services seed` is fragile one-shot | Controller has no desired state if seed fails |
| G3 | `services seed` never re-runs | Node joins don't trigger re-seed |
| G4 | No auto-import on controller startup | Restart doesn't repair missing desired state |
| G5 | Applications have no desired release path | Apps install to disk, never enter desired-state model |
| G6 | Infrastructure has no desired release path | Infra packages are invisible to controller |
| G7 | Installed-state has multiple competing sources | Disk markers, systemd, config files, etcd registry all diverge |
| G8 | UI derives status from ad-hoc guesses | Mixed `/config` + controller + repository queries |

---

## 4. Target State (To-Be Workflow Map)

### 4.1 Aligned Day-0 Flow

```
install-day0.sh (modified)
       │
       ├─ 1. globular-installer install --spec <spec>
       │     Same as today: disk + systemd
       │
       ├─ 2. systemctl enable + start
       │     All services running
       │
       ├─ 3. ensure_bootstrap_artifacts_published()     ← NEW
       │     For each core package:
       │       if artifact exists in Repository → skip
       │       else → globular pkg publish
       │     State: Repository now has core artifacts
       │
       └─ 4. globular services seed --insecure          ← HARDENED
              Idempotent import from installed-state
              Retries with backoff
              Creates ServiceDesiredVersion for each installed service
              State: Controller knows desired state
```

### 4.2 Aligned Steady-State Loop

```
┌────────────────────────────────────────────────────────────────┐
│                    Controller Reconciliation                    │
│                                                                │
│  On startup:                                                   │
│    if desired-state empty AND installed-state exists            │
│      → auto-import from installed-state                        │
│                                                                │
│  On node join:                                                 │
│    if node reports installed packages with no desired release   │
│      → targeted import                                         │
│                                                                │
│  On watch event (ServiceDesiredVersion / Release changed):     │
│    → compare desired vs installed → compile plan → dispatch    │
│                                                                │
│  Drift detection:                                              │
│    desired hash ≠ installed hash → plan + apply                │
└────────────────────────────────────────────────────────────────┘
```

### 4.3 Unified Package Lifecycle (All Kinds)

```
          ┌──────────┐     ┌──────────────┐     ┌────────────────┐     ┌──────────────┐
          │ Artifact  │     │   Desired    │     │   Installed    │     │   Runtime    │
          │  State    │────▶│   Release    │────▶│   Observed     │────▶│   Health     │
          │           │     │   State      │     │   State        │     │   State      │
          └──────────┘     └──────────────┘     └────────────────┘     └──────────────┘
               │                  │                     │                     │
          Repository         Controller            Node Agent           Node Agent
          service            etcd store           etcd registry       + systemd/health
               │                  │                     │                     │
          pkg publish       UpsertRelease         plan execution       ReportNodeStatus
          SearchArtifacts   GetDesiredState        package.report       health_checks
          DeleteArtifact    RemoveRelease          _state
```

This pipeline applies identically to SERVICE, APPLICATION, and INFRASTRUCTURE.

---

## 5. State Ownership Table

| Data | Writer | Reader(s) | Storage | Notes |
|------|--------|-----------|---------|-------|
| Artifact manifest | `pkg publish` via Repository service | Controller (resolve), UI (catalog) | `artifacts/` dir on repository node | Canonical artifact truth |
| Artifact blob | `pkg publish` via Repository service | Node Agent (fetch during plan) | `artifacts/` dir on repository node | Binary payload |
| `ServiceDesiredVersion` | Controller via `UpsertDesiredService` / `SeedDesiredState` | Controller reconciler, UI | etcd `/globular/resources/ServiceDesiredVersion/{name}` | Legacy simple model |
| `ServiceRelease` | Controller via `ApplyServiceRelease` | Controller reconciler, UI | etcd `/globular/resources/ServiceRelease/{name}` | Full release with rollout |
| `ApplicationRelease` | Controller via `ApplyApplicationRelease` | Controller reconciler, UI | etcd `/globular/resources/ApplicationRelease/{name}` | Exists in code, not yet wired to seed |
| `InfrastructureRelease` | Controller via `ApplyInfrastructureRelease` | Controller reconciler, UI | etcd `/globular/resources/InfrastructureRelease/{name}` | Exists in code, not yet wired to seed |
| Installed package record | Node Agent after plan step `package.report_state` | Controller (drift check), UI (installed view) | etcd `/globular/nodes/{nodeID}/packages/{kind}/{name}` | Canonical installed truth |
| Version markers | Installer / Node Agent `service.write_version_marker` | Node Agent `ComputeInstalledServices` | Disk `/var/lib/globular/versions/{svc}/version` | Input to Node Agent, NOT public truth |
| Service config files | Installer / service startup | Gateway `/config`, Node Agent | Disk `/var/lib/globular/services/*.json` | Input to Node Agent, NOT public truth |
| Systemd unit state | Installer / systemd | Node Agent health checks | systemd journal | Runtime health input, NOT installed truth |
| Applied hashes | Controller after successful plan | Controller drift detection | etcd `globular/cluster/v1/applied_hash*/{nodeID}` | Convergence tracking |
| Observed hashes | Node Agent `ComputeInstalledServices` | Controller drift comparison | etcd `globular/cluster/v1/observed_hash*/{nodeID}` | Raw inventory hash |

---

## 6. Migration Notes

### 6.1 Backward Compatibility

- `ServiceDesiredVersion` resources remain supported alongside `ServiceRelease`.
- Existing `globular services seed` CLI continues to work but delegates to hardened import logic.
- Disk-based version markers and config files remain as *inputs* to Node Agent discovery but are no longer treated as public truth by UI or controller.
- Gateway `/config` endpoint remains available but UI should prefer installed-state registry for package footprint.

### 6.2 Migration Sequence

| PR | What changes | Breaking? |
|----|-------------|-----------|
| PR-1 | This doc (vocabulary freeze) | No |
| PR-2 | Day-0 publishes core artifacts | No (additive) |
| PR-3 | Harden seed/import logic | No (idempotent improvement) |
| PR-4 | Auto-trigger import on startup/join | No (additive) |
| PR-5 | Formalize Application/Infrastructure releases | No (new RPCs, old ones remain) |
| PR-6 | Unify plan compilation | No (internal refactor) |
| PR-7 | Installed-state as canonical truth | Soft (readers migrate) |
| PR-8 | Application lifecycle alignment | No (extends existing) |
| PR-9 | Infrastructure lifecycle alignment | No (extends existing) |
| PR-10 | Repair command | No (new tool) |
| PR-11 | UI status derivation | Visual change (labels align to this doc) |
| PR-12 | Demote legacy assumptions | Docs/script changes |

### 6.3 Key Invariants (Must Hold After Migration)

1. Every running service/app/infra component has an artifact in Repository.
2. Every running service/app/infra component has a desired release in Controller.
3. Every running service/app/infra component has an installed-state record in Node Agent registry.
4. Installed-state registry is the single source for "what is installed where".
5. Disk/systemd are inputs to Node Agent, never queried directly by UI or Controller for install truth.
6. Import/seed is idempotent and can run at any time without side effects.
7. All three package kinds (SERVICE, APPLICATION, INFRASTRUCTURE) follow the same 4-layer pipeline.

---

## 7. Non-Goals

- No proto changes in this doc (deferred to PR-5+).
- No backend code changes in this doc.
- No UI changes in this doc.
- No redesign of the installer binary or spec format.
- No changes to the Node Agent execution model or action handlers.
