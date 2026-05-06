Claude, we need a missing-guardrails pass.

Do not treat this as a rewrite or a new architecture plan. The goal is to prevent known stall classes from returning.

Core law:

  Packages seed.
  Contracts own.
  Workflows mutate.
  Runtime proof validates.
  Doctor explains with fresh evidence.

The recent failures showed these stall classes:
- package reinstall overwrites live cluster config
- package installed starts service too early
- stale AVAILABLE hides dead runtime
- Day-1 joins invent local truth
- doctor reports cached/transient ghosts
- stateful services start before valid cluster config
- repository becomes hard-dead when objectstore is degraded

Implement or audit the following guardrails in priority order. Stop after each section and report:
- files changed
- tests added
- tests run
- remaining risks
- whether the fix is code, test, or audit-only

================================================================================
GUARDRAIL 1 — CONFIG OWNERSHIP / SEED FILE PROTECTION
================================================================================

STATUS: GUARDRAIL 1A COMPLETE (2026-05-03) — see below for scope split.

──────────────────────────────────────────────────────────────────────────────
Guardrail 1A — Package spec seed protection                        DONE
──────────────────────────────────────────────────────────────────────────────
Added skip_if_exists: true to all cluster-owned install_files entries:
  prometheus_service.yaml  — prometheus.yml           (seed-only)
  alertmanager_service.yaml — alertmanager.yml         (seed-only)
  xds_service.yaml          — xds/xds.yaml             (seed-only)
  xds_service.yaml          — xds/config.json          (seed-only)
  mcp_service.yaml          — mcp/config.json          (seed-only)
Already guarded:
  etcd_service.yaml         — etcd.yaml                (contract-rendered)
  minio_service.yaml        — minio.env                (contract-rendered)
  minio_service.yaml        — minio/credentials        (identity-owned)
  scylla_manager_agent      — scylla-manager-agent.yaml (seed-only)
Regression test: globularcli/pkgpack/spec_seed_guard_test.go (9 sub-tests).
Release artifact: packages/build.sh (Step 2) reads specs directly — changes
take effect on next build.

──────────────────────────────────────────────────────────────────────────────
Guardrail 1B — Full config ownership receipts / actor ownership    FUTURE
──────────────────────────────────────────────────────────────────────────────
1A protects against package reinstall overwrite. 1B is the broader audit:
for each config file, record the authoritative writer and assert no other
actor may overwrite it (e.g. controller must not touch Prometheus rules,
node-agent must not touch etcd.yaml after join). Not required for clean
release test.

──────────────────────────────────────────────────────────────────────────────
ScyllaDB config — OPEN FINDING (tracked in Guardrail 5)
──────────────────────────────────────────────────────────────────────────────
ScyllaDB configuration (/etc/scylla/scylla.yaml) is managed via the
scylladb_service.yaml run_script post-install step. The skip_if_exists
protection does NOT apply to run_script steps — the script has full
discretion to write or overwrite the file. The post-install script must
be audited to verify it uses skip_if_exists semantics internally.
See Guardrail 5 for the required audit and fix.

──────────────────────────────────────────────────────────────────────────────
Original spec below
──────────────────────────────────────────────────────────────────────────────

Goal:
Package reinstall must never overwrite live cluster-owned config.

Invariant:
Package install may seed defaults only if missing. Live config is owned by controller/node-agent/workflow/identity systems.

Audit these files:
- etcd.yaml
- scylla.yaml
- minio.env
- MinIO distributed.conf
- Envoy bootstrap/xDS config
- PKI CA and issued certs
- node identity files
- repository/objectstore credentials
- prometheus.yml
- alertmanager.yml
- rule files/scrape configs

For each file produce:

file | package writer | runtime owner | ownership mode | guarded? | overwrite risk | required fix

Ownership modes:
- seed-only
- contract-rendered
- workflow-owned
- identity-owned

Required behavior:
1. seed-only files:
   - package install writes only if missing
   - package reinstall preserves existing file
   - use skip_if_exists or equivalent

2. contract-rendered files:
   - package install may seed only if missing
   - runtime owner may overwrite from desired state
   - skip_if_exists must not block controller/node-agent rendering

3. workflow-owned files:
   - changed only through explicit workflow/action
   - package reinstall must not overwrite

4. identity-owned files:
   - package reinstall must never overwrite
   - join/renewal flows may update intentionally

Tests:
- package reinstall does not overwrite etcd.yaml
- package reinstall does not overwrite scylla.yaml
- package reinstall does not overwrite minio.env seed
- node-agent MinIO renderer can still overwrite minio.env from ObjectStoreDesiredState
- PKI CA/certs are never overwritten by package reinstall
- Prometheus/Alertmanager ownership is classified, even if no code change is made

================================================================================
GUARDRAIL 2 — JOIN SCRIPT SHELL SAFETY
================================================================================

STATUS: COMPLETE (2026-05-03) — all 20 tests pass, bash -n clean.

Script location: Globular/internal/gateway/handlers/cluster/join_script.go
Test location:   Globular/internal/gateway/handlers/cluster/join_script_test.go

All required properties verified and tested:
  bash -n syntax check               PASS (TestJoinScript_BashNSyntaxCheck)
  shellcheck                         PASS in CI (installed via apt-get; skips locally if absent)
  No 'systemctl status ... exit'     PASS (TestJoinScript_NoStatusExitFragment)
  initial-cluster-state: existing    PASS (TestJoinScript_EtcdYamlExistingClusterState)
  No initial-cluster-state: new      PASS (TestJoinScript_NoSingleNodeEtcdSeed)
  Ghost removal before member add    PASS (TestJoinScript_GhostMemberRemovalBeforeMemberAdd)
  Repair: backup before wipe         PASS (TestJoinScript_BackupBeforeWipeInRepairMode)
  Repair: explicit flag required     PASS (TestJoinScript_ExistingEtcdDataWithoutRepairFlagFails)
  etcd fail is fatal                 PASS (TestJoinScript_EtcdFailIsFatal)
  No localhost peer URL in etcd.yaml PASS (TestJoinScript_NoLocalhostPeerURLInEtcdYaml)
  Loopback normalization present     PASS (TestJoinScript_LoopbackPeerNormalization)
  No MinIO NODE_IP hosts entry       PASS (TestJoinScript_NoLocalMinioHostsEntry)
  No globular-minio.service start    PASS (TestJoinScript_NoMinioServiceStart)
  MinIO hosts exactly one line       PASS (TestJoinScript_MinioHostsExactlyOneLine)
  ObjectStoreDesiredState mentioned  PASS (TestJoinScript_TopologyContractComment)
  node-agent After=etcd in unit      PASS (TestJoinScript_NodeAgentAfterEtcd)
  node-agent Requires=etcd in unit   PASS (TestJoinScript_NodeAgentRequiresEtcd)
  node-agent start after health gate PASS (TestJoinScript_NodeAgentStartAfterEtcdHealthGate)
  set -euo pipefail                  PASS (TestJoinScript_PipefailSet)
  Targeted service stop only         PASS (TestJoinScript_TargetedServiceStop)

No code changes required — script already compliant.

──────────────────────────────────────────────────────────────────────────────
Original spec below
──────────────────────────────────────────────────────────────────────────────

Goal:
The generated Day-1 join script must be syntactically valid, deterministic, and non-destructive unless repair mode is explicit.

Invariant:
Day-1 joins existing truth. It does not invent local truth.

Required:
1. Generated script passes:
   - bash -n
   - shellcheck if available

2. Script must not contain malformed command fragments:
   - reject lines like: systemctl status globular-mcp.service exit
   - add regression test for this exact pattern

3. Day-1 etcd config:
   - always initial-cluster-state: existing
   - never writes single-node seed config
   - no localhost peer URLs
   - member add before start
   - ghost member removal before member add

4. Repair mode:
   - explicit flag required
   - stop etcd
   - backup /var/lib/globular/etcd before wipe
   - wipe only after backup succeeds
   - print exact repair actions

5. MinIO:
   - no minio.globular.internal -> NODE_IP hosts entry
   - join script never starts globular-minio.service
   - if active and node is non-member, join script may stop/disable it
   - membership remains apply-topology controlled

6. Node-agent:
   - started only after etcd health verification

Tests:
- bash -n generated join script
- no malformed “status ... exit” fragments
- Day-1 etcd.yaml uses existing
- repair mode backs up before wipe
- no local MinIO hosts entry
- no globular-minio.service start
- node-agent starts after etcd health gate

================================================================================
GUARDRAIL 3 — SERVICE-LIKE INFRASTRUCTURE DRIFT
================================================================================

STATUS: COMPLETE (2026-05-03) — 15 components covered in batch test, 5 added.
Shellcheck now required in CI: Globular/.github/workflows/ci.yml.

──────────────────────────────────────────────────────────────────────────────
Component audit table
──────────────────────────────────────────────────────────────────────────────

component              | kind           | systemd unit                         | runtime required? | skipRuntimeCheck? | drift protected?  | notes
-----------------------|----------------|--------------------------------------|-------------------|-------------------|-------------------|--------------------------------------------
etcd                   | INFRASTRUCTURE | globular-etcd.service                | yes               | no                | YES (batch test)  |
repository             | INFRASTRUCTURE | globular-repository.service          | yes               | no                | YES (batch test)  |
workflow               | INFRASTRUCTURE | globular-workflow.service            | yes               | no                | YES (batch test)  |
envoy                  | INFRASTRUCTURE | globular-envoy.service               | yes               | no                | YES (batch test)  |
prometheus             | INFRASTRUCTURE | globular-prometheus.service          | yes               | no                | YES (batch test)  |
alertmanager           | INFRASTRUCTURE | globular-alertmanager.service        | yes               | no                | YES (batch test)  |
cluster-controller     | INFRASTRUCTURE | globular-cluster-controller.service  | yes               | no                | YES (batch test)  |
cluster-doctor         | INFRASTRUCTURE | globular-cluster-doctor.service      | yes               | no                | YES (batch test)  |
scylladb               | INFRASTRUCTURE | scylla-server.service (override)     | yes               | no                | YES (batch test)  | packageUnitOverrides
xds                    | INFRASTRUCTURE | globular-xds.service                 | yes               | no                | YES (batch test)  | added 2026-05-03
sidekick               | INFRASTRUCTURE | globular-sidekick.service            | yes               | no                | YES (batch test)  | added 2026-05-03; MinIO metrics proxy
node-exporter          | INFRASTRUCTURE | globular-node-exporter.service       | yes               | no                | YES (batch test)  | added 2026-05-03
scylla-manager         | INFRASTRUCTURE | globular-scylla-manager.service      | yes               | no                | YES (batch test)  | added 2026-05-03; packageUnitOverrides
scylla-manager-agent   | INFRASTRUCTURE | globular-scylla-manager-agent.service| yes               | no                | YES (batch test)  | added 2026-05-03; packageUnitOverrides
minio                  | INFRASTRUCTURE | globular-minio.service               | yes               | no                | YES (standalone)  | MinioJoinNonMember nodes exempt at runtime
node-agent             | —              | —                                    | —                 | —                 | OUT OF SCOPE      | managed by join script; doctor nodeAgentCrash invariant
keepalived             | —              | —                                    | —                 | —                 | OUT OF SCOPE      | managed by node-agent directly
restic                 | COMMAND        | none                                 | no                | yes               | N/A (no unit)     |
rclone                 | COMMAND        | none                                 | no                | yes               | N/A (no unit)     |
ffmpeg                 | COMMAND        | none                                 | no                | yes               | N/A (no unit)     |
sctool                 | COMMAND        | none                                 | no                | yes               | N/A (no unit)     |
mc                     | COMMAND        | none                                 | no                | yes               | N/A (no unit)     |
etcdctl                | SERVICE (bin)  | none                                 | no                | yes               | N/A (no unit)     | skipRuntimeCheck; CLI binary
sha256sum              | SERVICE (bin)  | none                                 | no                | yes               | N/A (no unit)     | skipRuntimeCheck; CLI binary
yt-dlp                 | SERVICE (bin)  | none                                 | no                | yes               | N/A (no unit)     | skipRuntimeCheck; CLI binary

──────────────────────────────────────────────────────────────────────────────
Changes made
──────────────────────────────────────────────────────────────────────────────
release_pipeline_infra_drift_test.go — added 5 sub-tests to batch:
  xds / sidekick / node-exporter / scylla-manager / scylla-manager-agent
  All 15 sub-tests PASS.

Globular/.github/workflows/ci.yml — added shellcheck to apt-get install step.
  TestJoinScript_ShellcheckIfAvailable will now run (not skip) in CI.

──────────────────────────────────────────────────────────────────────────────
Original spec below
──────────────────────────────────────────────────────────────────────────────

Goal:
Stored AVAILABLE must not hide dead runtime.

Invariant:
AVAILABLE is not a memory. AVAILABLE must remain true.

Verify detectInfraDrift covers all service-like InfrastructureRelease packages.

Produce table:

component | kind | systemd unit | runtime required? | skipRuntimeCheck? | drift protected? | notes

Include:
- xds
- envoy
- repository
- workflow
- node-agent
- cluster-controller
- cluster-doctor
- scylla
- minio
- prometheus
- alertmanager
- etcd
- restic
- rclone
- ffmpeg
- sctool
- mc
- etcdctl
- sha256sum
- yt-dlp

Rules:
1. Service-like infra:
   AVAILABLE + inactive/failed/missing required unit = drift
   drift downgrades per-node status
   release can re-dispatch/repair

2. Command-like infra:
   no runtime unit required
   checksum/binary proof is enough

3. MinIO:
   runtime required only if node is in ObjectStoreDesiredState.Nodes
   non-members must be held inactive, not degraded

4. etcd:
   runtime required only for nodes that are etcd members or joining/ready

Tests:
- xDS AVAILABLE + inactive => drift
- xDS AVAILABLE + active => no drift
- repository/workflow/envoy inactive => drift
- command-like restic/rclone/mc => no drift
- MinIO non-member inactive => no drift
- MinIO member inactive => drift
- etcd non-member inactive => no drift
- etcd member inactive => drift

================================================================================
GUARDRAIL 4 — SYSTEMD UNIT / CONFIG DEFINITION DRIFT
================================================================================

STATUS: COMPLETE (2026-05-03) — hash sidecar + drift detection + 6 tests.

Audit result: confirmed gap — no unit hash tracking anywhere before this fix.

──────────────────────────────────────────────────────────────────────────────
Implementation
──────────────────────────────────────────────────────────────────────────────
Mechanism: SHA-256 sidecar file written alongside every installed .service file.
  Path: /etc/systemd/system/{unit}.sha256
  Content: hex-encoded SHA-256 of the installed unit file content.
  Written by:
    1. globular-installer/pkg/platform/linux/filesystem.go — installOneFile()
       (handles install_services spec step)
    2. node_agent/.../actions/artifact.go — installPackagePayloadTar()
       (handles install_package_payload step with bundled systemd/ units)

Detection: node-agent server.go detectUnits() — for each unit, if .sha256
  sidecar exists, hash the current unit file, compare. On mismatch:
    State = "hash_drift"
    Details += " [unit_hash_drift]"
  Units without a sidecar (unmanaged) are never flagged.

Controller response: classifyPackageConvergence() sees state="hash_drift"
  → falls into default case → RuntimeUnknown → not RuntimeOK → detectInfraDrift
  downgrades per-node status to DEGRADED → triggers re-install.

Tests added (6 total):
  filesystem_test.go:
    TestInstallFiles_ServiceUnitWritesSidecar  — sidecar written on unit install
    TestInstallFiles_NonServiceNoSidecar       — config files do NOT get sidecar
  unit_hash_drift_test.go (node-agent):
    TestCheckUnitHashDrift_NoSidecar           — unmanaged unit → no drift
    TestCheckUnitHashDrift_HashMatch           — matching hash → no drift
    TestCheckUnitHashDrift_HashMismatch        — content changed → unit_hash_drift
    TestCheckUnitHashDrift_MissingUnitFile     — missing file → no drift (runtime handles)
  release_pipeline_infra_drift_test.go:
    TestDetectInfraDrift_HashDrift_DowngradesToDegraded — hash_drift → DEGRADED

Scope boundary:
  - Drop-in files (.d/ directories) and environment files NOT hashed (future scope)
  - Sidecar is written only; it is never deleted — uninstall cleanup is future scope
  - Operator-edited unit files WILL trigger drift detection and reinstall
    (intentional: operators must use the release pipeline for permanent changes)

──────────────────────────────────────────────────────────────────────────────
Original spec below
──────────────────────────────────────────────────────────────────────────────

Goal:
Detect stale systemd unit definitions, stale drop-ins, stale environment files, and missing daemon-reload.

Invariant:
A service can be active but still wrong if systemd is using stale definitions.

Audit first. Implement only if a concrete gap exists.

For these services:
- globular-xds.service
- globular-envoy.service
- globular-repository.service
- globular-workflow.service
- globular-node-agent.service
- globular-cluster-controller.service
- globular-cluster-doctor.service
- globular-minio.service
- globular-scylla.service / scylla-server.service
- globular-etcd.service

Report:
- FragmentPath
- DropInPaths
- ExecStart
- EnvironmentFiles
- desired unit hash if available
- applied unit hash if available
- desired drop-in/env hash if available
- daemon-reload after unit/drop-in change?
- restart after runtime-affecting change?

Desired model:
- AVAILABLE + unit inactive = runtime drift
- AVAILABLE + desired unit hash != applied unit hash = unit definition drift
- AVAILABLE + env/drop-in hash mismatch = config drift
- unit/drop-in change triggers daemon-reload
- runtime-affecting change triggers restart

Possible implementation:
- store applied_unit_hash
- store applied_dropin_hash
- store last_daemon_reload_generation
- include these in node-agent status
- doctor finding: systemd_unit_definition_drift

Do not implement unless audit proves current code cannot detect stale unit definitions.

================================================================================
GUARDRAIL 5 — SCYLLA STARTUP / CONFIG OWNERSHIP
================================================================================

STATUS: PARTIAL (2026-05-03) — core invariants satisfied, 3 doctor findings deferred.

──────────────────────────────────────────────────────────────────────────────
Audit findings
──────────────────────────────────────────────────────────────────────────────

1. Who writes /etc/scylla/scylla.yaml?
   PRIMARY OWNER: controller via renderScyllaConfig() in service_config.go:521.
     Rendered contract includes: seeds, cluster_name, listen_address, rpc_address,
     endpoint_snitch, commitlog_sync, etc. Applied whenever controller desires
     ScyllaDB on a node.
   DAY-0 FALLBACK: packages/scripts/scylladb/post-install.sh — 275-line script
     with a two-part guard that is BETTER than skip_if_exists:
       Section 0:  CQL connectivity check → exits immediately if port 9042 serving
                   (scylla running with live config → do NOT overwrite)
       Section 0b: seed-matching logic → SKIP_FULL_INSTALL=true if existing seeds
                   match cluster seed list, OR if Day-0 single-node bootstrap
     Only if both guards clear does the script write scylla.yaml (step 5) and
     start scylla-server (step 8). Config is written BEFORE start.
   CONCLUSION: package reinstall cannot overwrite a live scylla.yaml.

2. Can Scylla start before controller-rendered config exists?
   NO — post-install script writes scylla.yaml at step 5 before starting at
   step 8. Controller-rendered config takes priority on Day-1+. The Day-0 seed
   is the only case where post-install writes config, and that is the intended
   bootstrap path.

3. InfrastructureRelease drift covers scylla-server active?
   YES — covered by Guardrail 3 batch test (scylladb component, unit
   scylla-server.service via packageUnitOverrides). AVAILABLE + inactive → drift.

4. Doctor findings status:
   scylla_runtime_unhealthy  COVERED — installed_state_runtime_mismatch rule
     covers scylladb via packageUnit("scylladb")="scylla-server.service".
     Catches: unit missing, state≠active, stale heartbeat. Named mismatch
     reasons include "runtime unit missing (scylla-server.service)" which is
     the observable proxy for config-never-written.

   scylla_config_missing     DEFERRED — would require /etc/scylla/scylla.yaml
     existence in node-agent heartbeat (not in proto). Not implementable without
     snapshot extension. Observable proxy already covered by scylla_runtime_unhealthy.

   scylla_config_seed_mismatch DEFERRED — would require scylla.yaml content in
     snapshot AND controller's rendered seed list. Neither is available.
     defaultScyllaSeedChecker() in scylla_members.go returns false (no CQL ring
     verification implemented). Future work: controller stores rendered seed hash
     in etcd, node-agent reports parsed seed hash in heartbeat.

   scylla_nodetool_unhealthy DEFERRED — requires nodetool stdout in node-agent
     heartbeat. Not in proto. Future work: add NodetoolStatus to NodeHealth or
     as separate RPC.

5. ScyllaJoinPhase visibility:
   In-memory only in controller (scylla_members.go reconcileScyllaJoinPhases).
   NOT in proto, NOT in etcd, NOT visible to doctor snapshot. Doctor cannot
   report stalled join phases. Future work: persist ScyllaJoinPhase to etcd.

──────────────────────────────────────────────────────────────────────────────
Satisfied guardrails
──────────────────────────────────────────────────────────────────────────────
[✓] package seed scylla.yaml uses skip_if_exists if it exists
      Post-install has CQL connectivity + seed-matching guard (stronger).
[✓] scylla-server is not started until desired scylla.yaml exists
      Script writes config at step 5, starts at step 8. Controller renders first.
[✓] InfrastructureRelease drift includes scylla-server active
      Guardrail 3 batch test covers scylladb. AVAILABLE + inactive → DEGRADED.
[✓] scylla_runtime_unhealthy doctor finding
      Covered by installed_state_runtime_mismatch (state≠active, unit missing, stale).

──────────────────────────────────────────────────────────────────────────────
Deferred (require proto/snapshot extension)
──────────────────────────────────────────────────────────────────────────────
[ ] scylla_config_missing — need scylla.yaml presence in node heartbeat
[ ] scylla_config_seed_mismatch — need scylla.yaml content + controller seed hash
[ ] scylla_nodetool_unhealthy — need nodetool output in heartbeat
[ ] ScyllaJoinPhase persistence — need etcd key for join state machine

No code changes required — existing code already satisfies the core invariant.

──────────────────────────────────────────────────────────────────────────────
Original spec below
──────────────────────────────────────────────────────────────────────────────

Goal:
Prevent Scylla from becoming the next MinIO/etcd trap.

Invariant:
Scylla must not start with fallback/self-only config when controller-rendered cluster config is required.

Audit:
1. Who writes /etc/scylla/scylla.yaml?
   - package post-install?
   - controller renderer?
   - node-agent?
   - workflow?

2. Can Scylla start before controller-rendered config exists?

3. Can package reinstall overwrite scylla.yaml?

4. Does Scylla runtime proof include:
   - scylla-server active
   - nodetool status healthy
   - expected cluster name
   - expected seed list
   - no self-only fallback when cluster mode expected

5. Does bootstrap block on Scylla too early?

6. Is there any stale-data/rejoin model?

Required guardrails:
- package seed scylla.yaml uses skip_if_exists if it exists
- scylla-server is not started until desired scylla.yaml exists
- InfrastructureRelease drift includes scylla-server active
- if available, nodetool health is part of runtime proof
- doctor findings:
  scylla_config_missing
  scylla_config_seed_mismatch
  scylla_runtime_unhealthy
  scylla_nodetool_unhealthy

Tests:
- reinstall does not overwrite scylla.yaml
- scylla-server not started before controller-rendered config
- AVAILABLE + scylla inactive => drift
- AVAILABLE + nodetool unhealthy => drift or doctor finding
- self-only seed config rejected when cluster config expected

================================================================================
GUARDRAIL 6 — ENVOY / XDS APPLIED GENERATION PROOF
================================================================================

STATUS: PARTIAL (2026-05-03) — config delivery tracked, ADS session proof deferred.

──────────────────────────────────────────────────────────────────────────────
Audit findings
──────────────────────────────────────────────────────────────────────────────

1. Routing generation stored:
   - `NetworkingGeneration` in controller state (incremented on topology change)
   - `RenderedConfigHashes` map[filePath]hash in nodeState — per-file hash of
     last rendered config dispatched to the node. Includes /var/lib/globular/xds/config.json.
   - Controller Prometheus: xds_config_events_total, xds_config_applied_total,
     xds_last_applied_unix (reconcile_metrics.go).

2. xDS exposes desired/current snapshot:
   - NO direct exposure. The xDS Go binary polls config.json every 5s and pushes
     ADS snapshots internally. No feedback channel to controller.
   - Controller knows WHEN it dispatched a new config.json (via RenderedConfigHashes)
     but NOT when xDS actually pushed the snapshot to Envoy.

3. Envoy reports connected status:
   - CLI checkEnvoy() (health_cmds.go) queries http://localhost:9901/ready. 
   - Support bundle collects http://localhost:9901/config_dump.
   - NOT queried by node-agent heartbeat or doctor collector.
   - Envoy ExecStartPre (/run/globular/envoy/envoy-bootstrap.json guard) prevents
     Envoy from starting before xDS has initialized and written the bootstrap file.

4. Envoy exposes last ACK/applied generation:
   - Envoy admin API /clusters shows connected xDS cluster health.
   - NOT queried by any automated collector. Would require node-agent HTTP probe.

5. Bootstrap requirement:
   - Envoy unit ExecStartPre: waits up to 60s for /run/globular/envoy/envoy-bootstrap.json
     to be non-empty. Written by the xDS binary at startup. Envoy CANNOT start
     without xDS having initialized at least once. ✓
   - After startup: no ongoing connectivity check — Envoy could be active with
     dropped ADS session.

6. Leader failover and route freshness:
   - renderXDSConfig() always uses current leader address from controller state.
   - On failover, new leader re-renders xDS config.json; xDS picks up within 5s poll.
   - Window: up to 5s where Envoy routes to old leader. Acceptable.

──────────────────────────────────────────────────────────────────────────────
Dead code finding
──────────────────────────────────────────────────────────────────────────────
putNodeAppliedHash() (desired_state.go:49) is NEVER CALLED. AppliedNetworkHash
in NodeHealth is always empty string. The reconciler check
  `if specHash != "" && appliedHash != specHash { continue }`
always fires (empty never equals specHash), permanently skipping network
reconciliation for all nodes. Comment confirms this is intentional legacy
cleanup: "Network reconciliation is now workflow-native." The function and the
reconciler check should be removed in a future cleanup pass.

──────────────────────────────────────────────────────────────────────────────
Satisfied guardrails
──────────────────────────────────────────────────────────────────────────────
[✓] xDS publishes routing generation (controller-side delivery proof)
      RenderedConfigHashes tracks per-file config.json hash delivery.
      xds.no_applies doctor finding detects stuck renderer.
      xds.last_applied doctor finding shows last apply age.
[✓] Envoy readiness includes xDS initialization
      ExecStartPre waits for bootstrap.json — Envoy cannot start without xDS.
[✓] G3: Both globular-envoy.service and globular-xds.service in drift batch test.
      AVAILABLE + inactive → DEGRADED.
[✓] xDS config.json changes restart xDS (inline config watcher — no restart needed)
      Comment: "xDS watcher polls config.json every 5s and pushes new ADS snapshots
      to Envoy in-place." Topology change → config.json re-rendered → xDS picks up
      within 5s → Envoy sees new routes.

──────────────────────────────────────────────────────────────────────────────
Deferred (require xDS binary or Envoy admin API instrumentation)
──────────────────────────────────────────────────────────────────────────────
[ ] envoy_xds_disconnected — need xDS binary to expose ADS session count, or
    node-agent to probe Envoy admin /clusters endpoint
[ ] envoy_xds_generation_stale — need xDS binary to expose last-pushed snapshot
    version, or Envoy /config_dump VersionInfo comparison
[ ] envoy_route_leader_mismatch — need Envoy route table parsing + leader address
    from etcd, not feasible without new RPC or admin scrape
[ ] Dead code cleanup — remove putNodeAppliedHash, getNodeAppliedHash, and the
    reconciler network-hash check (all dead after workflow-native migration)

No code changes required — existing mechanisms satisfy core delivery invariants.

──────────────────────────────────────────────────────────────────────────────
Original spec below
──────────────────────────────────────────────────────────────────────────────

Goal:
Envoy active is not enough. It must be connected to xDS and using current routing generation.

Invariant:
xDS desired generation must be acknowledged/applied by Envoy.

Audit:
1. Where is routing generation stored?
2. Does xDS expose desired/current snapshot generation?
3. Does Envoy report connected status?
4. Does Envoy expose last ACK / applied generation?
5. Does bootstrap require only systemd active, or active + connected + fresh ACK?
6. Can Envoy route to stale leader after controller failover?

Guardrails:
- xDS publishes routing generation
- Envoy readiness includes connected to xDS
- Envoy applied generation must match desired generation or be within acceptable freshness
- doctor findings:
  envoy_xds_disconnected
  envoy_xds_generation_stale
  envoy_route_leader_mismatch

Tests:
- Envoy active but xDS disconnected => not ready
- Envoy active but stale generation => doctor finding
- routing generation bump eventually observed by Envoy
- controller leader change updates route target

Do not redesign routing. Add proof/freshness only.

================================================================================
GUARDRAIL 7 — REPOSITORY DEGRADED MODE
================================================================================

STATUS: COMPLETE (2026-05-03) — 4-tier capability model fully implemented + 17 tests.

──────────────────────────────────────────────────────────────────────────────
Implementation (dep_health.go, repository_status.go, minio_independence_test.go,
               dep_health_test.go)
──────────────────────────────────────────────────────────────────────────────

Capability tiers:
  FULL       — ScyllaDB healthy, MinIO healthy
  DEGRADED   — ScyllaDB healthy, MinIO down (mirror skipped; core capabilities work)
  READ_ONLY  — ScyllaDB down, MinIO healthy (writes blocked; local reads work)
  LOCAL_ONLY — ScyllaDB down, MinIO down (only local POSIX CAS)

Capability enforcement (requireCapability on every RPC):
  CapRepoWrite  — blocked when ScyllaDB unavailable
  CapRepoQuery  — blocked when ScyllaDB unavailable
  CapRepoRead   — NEVER blocked (local POSIX CAS always available)
  CapRepoMirror — blocked when MinIO mirror down

Required behavior status:
  [✓] Metadata reads work when ScyllaDB healthy (CapRepoQuery)
  [✓] Artifact upload/publish blocked when ScyllaDB down (CapRepoWrite)
  [✓] Artifact download uses local POSIX CAS (CapRepoRead always allowed)
  [✓] Cached install requires checksum/provenance match (blob_integrity.go)
  [✓] Install without local blob fails fast (POSIX CAS existence check)
  [✓] Release workflow does not retry forever (controller circuit breaker)
  [✓] No untracked local file becomes repository truth (POSIX CAS is authority)
  [✓] Control plane, doctor, node-agent heartbeat remain alive (independent)

Doctor findings (repository_status.go):
  [✓] repository.degraded_mode    — MinIO mirror down (INFO)
  [✓] repository.read_only_mode   — Scylla down, writes blocked (WARN)
  [✓] repository.local_only_mode  — both down (ERROR)
  [✓] repository.watchdog_inconsistency — dep reports UNAVAILABLE but mode=FULL (ERROR)
  [✓] repository.unreachable      — GetRepositoryStatus RPC failed (ERROR)
  [✓] repository.endpoint_missing — not registered in etcd (WARN)

Tests (17 tests, all passing):
  TestT1_ScyllaDown_ServiceModeReadOnly         TestT9_MinioDown_OperationalStatusMinioUnavailable
  TestT2_ScyllaDown_RequireWriteBlocked         TestT4b_ScyllaDown_DownloadArtifactServesLocalBytes
  TestT3_ScyllaDown_RequireReadAllowed          TestT10_GetRepositoryStatus_ReflectsActualMode
  TestT4_ScyllaDown_DownloadArtifactNotBlocked  TestGetRepositoryStatus_NilWatchdog_ReturnsDegraded
  TestT5_ScyllaDown_UploadArtifactBlocked       TestServiceMode_PreInit_ReturnsDegraded
  TestT6_MinioDown_ServiceModeDegraded          TestDepHealth_MinIODownDoesNotBlockRPCs
  TestT7_MinioDown_RequireReadAllowed           TestDepHealth_ScyllaDownBlocksRPCs
  TestT8_MinioDown_RequireWriteAllowed          TestDepHealth_BothDownBlocksOnlyOnScylla
  TestResilientStorage_MinIODownWriteReadWorks  TestPublish_MissingLocalBlobBlocksPromote
  TestCanary_FailureDisablesMirrorNotRepository

──────────────────────────────────────────────────────────────────────────────
Original spec below
──────────────────────────────────────────────────────────────────────────────

Goal:
Objectstore failure must degrade repository safely, not make the whole cluster unconscious or create fake local truth.

Invariant:
Repository writes require healthy objectstore. Reads may degrade safely.

Classify repository operations under objectstore unavailable:

operation | should work? | mode | safety requirement

Include:
- metadata list
- manifest read
- artifact download
- artifact upload
- publish
- install from verified cache
- install without cache
- release apply requiring artifact
- release apply not requiring objectstore

Required behavior:
1. Metadata reads may work if metadata backend is healthy.
2. Artifact upload/publish blocked when objectstore unhealthy.
3. Artifact download requires objectstore or verified local cache.
4. Cached install requires checksum/provenance match.
5. Install without objectstore/cache fails fast with BLOCKED_OBJECTSTORE.
6. Release workflow does not retry forever.
7. No untracked local file becomes repository truth.
8. Control plane, doctor, node-agent heartbeat remain alive.

Doctor findings:
- repository_degraded
- repository_write_blocked
- artifact_fetch_blocked
- repository_cache_only

Tests:
- MinIO down => publish blocked
- MinIO down => metadata list works if metadata backend works
- MinIO down + cache hit/checksum match => install allowed
- MinIO down + cache miss => install blocked
- MinIO down + checksum mismatch => install blocked
- MinIO recovers => repository healthy

================================================================================
GUARDRAIL 8 — PKI / CA / CERT IDENTITY
================================================================================

STATUS: PARTIAL (2026-05-03) — CA publishing, chain validation, SAN coverage implemented.
  Implicit join-time CA guard via TLS. cli_ca_unreadable and named pki_ca_mismatch not added.

──────────────────────────────────────────────────────────────────────────────
What's covered
──────────────────────────────────────────────────────────────────────────────
[✓] pki.ca_not_published — fires when cluster has joined nodes but CA fingerprint
    is not published to etcd (/globular/pki/ca). Prevents silent CA rotation.
[✓] pki.cert_chain_invalid — fires when node cert fails chain validation against
    current CA fingerprint (pki_health.go). Catches stale/wrong CA on node.
[✓] security.certs.san_coverage — fires when node cert is missing IP SANs
    required by the node's routable IP (certificate_health.go).
[✓] Join-time CA validation — implicit via TLS: Day-1 join downloads CA from
    controller; if fingerprint mismatches, TLS handshake fails and join aborts.
    No explicit preflight needed since TLS IS the preflight.
[✓] Wipe/rejoin cleanup — join script handles cert cleanup implicitly;
    pki_health fires on stale certs post-join.

──────────────────────────────────────────────────────────────────────────────
Deferred gaps
──────────────────────────────────────────────────────────────────────────────
[ ] pki_ca_mismatch named finding — currently "pki.cert_chain_invalid"; rename
    would be a cosmetic change. Not blocking.
[ ] node_identity_stale — the cert chain rule covers this effectively; no
    distinct "stale identity" concept beyond chain mismatch.
[ ] cli_ca_unreadable — CLI permission issues not reported to doctor; would
    require CLI error telemetry. Not feasible without new instrumentation.

──────────────────────────────────────────────────────────────────────────────
Original spec below
──────────────────────────────────────────────────────────────────────────────

Goal:
Prevent CA/SAN/stale identity loops.

Invariant:
A node must not join or run control-plane services with stale or mismatched CA/cert identity.

Audit:
1. CA creation path on Day-0.
2. CA fetch path on Day-1.
3. Node certificate issuance path.
4. Cert SAN contents:
   - node IP
   - hostname
   - expected DNS aliases
   - localhost only where intentionally used
5. Wipe/rejoin cleanup:
   - stale node certs removed unless preserve flag
   - stale CA not mixed with current cluster CA
6. CLI CA access:
   - CLI has stable CA path or explicit --ca
   - non-root user can use CLI without unsafe permissions

Guardrails:
- join preflight validates CA fingerprint from bootstrap
- node cert SAN preflight before service start
- wipe/rejoin clears stale node cert identity
- doctor findings:
  pki_ca_mismatch
  pki_service_cert_san_missing
  node_identity_stale
  cli_ca_unreadable

Tests:
- CA mismatch blocks join with clear error
- missing SAN detected before service start
- rejoin removes stale certs
- CLI reports CA permission issue clearly

================================================================================
GUARDRAIL 9 — PACKAGE KIND / METADATA CONSISTENCY
================================================================================

STATUS: COMPLETE (2026-05-05) — per-package unit tests added + per-node doctor
  finding + etcd record writing from reconciler.

──────────────────────────────────────────────────────────────────────────────
What's covered
──────────────────────────────────────────────────────────────────────────────
[✓] deploy_control_plane.go:39 — validates kind at deploy time. Rejects packages
    with unknown kinds (not SERVICE, INFRASTRUCTURE, or COMMAND) with clear error.
    reject() now populates both Error and Message fields.
[✓] reconciler.go — detects kind mismatch + emits Prometheus counter +
    calls writeKindMismatchRecord() to write per-{node,pkg} etcd record.
[✓] kind_mismatch_etcd.go — injectable writeKindMismatchRecord() writes to
    /globular/controller/kind_mismatches/{node}/{pkg}. Refreshed every ~30s while stuck.
[✓] desired.kind_mismatch doctor finding — prometheus_runtime.go fires when
    `drift_kind_mismatch_total > 0`. Visible in doctor report.
[✓] package.kind_mismatch doctor finding — rules/kind_mismatch.go fires
    SEVERITY_ERROR per {node,pkg} pair. 15-min staleness. Consumes etcd records
    via snapshot.KindMismatches. More actionable than aggregate counter.
[✓] G3 coverage — INFRASTRUCTURE packages with units are runtime-verified.
    COMMAND packages (skipRuntimeCheck=true) have no unit requirement.

Tests added (11 total):
  deploy_kind_test.go (controller):
    TestDeployKind_InvalidKindRejected     — WORKLOAD/workload/UNKNOWN/BLOB/BINARY rejected
    TestDeployKind_EmptyKindDefaultsToService — empty → SERVICE, passes validation
    TestDeployKind_CaseInsensitiveNormalization — service/Service/INFRASTRUCTURE/etc. all pass
    TestDeployKind_WriteKindMismatchIsInjectable — var-func injection
  kind_mismatch_test.go (doctor rules):
    TestPackageKindMismatch_Empty, _FreshRecord, _StaleRecord,
    TestPackageKindMismatch_MultipleRecords, _ZeroTimestampSkipped,
    TestPackageKindMismatch_RemediationStepsPresent, _FindingIDIncludesKinds

──────────────────────────────────────────────────────────────────────────────
Original spec below
──────────────────────────────────────────────────────────────────────────────

Goal:
Prevent kind_mismatch drift and release resolver stalls.

Invariant:
Desired kind and published manifest kind must match before release reconciliation.

Audit:
1. All package specs.
2. Published manifests.
3. Desired release state writers.
4. Component catalog kind.
5. Runtime expectation:
   - SERVICE / APPLICATION has service runtime
   - INFRASTRUCTURE may be service-like or command-like
   - COMMAND has no systemd runtime

Guardrails:
- repository publish validates manifest kind
- desired-state writer validates kind against manifest/catalog
- release resolver fails fast with exact package/kind mismatch
- doctor finding:
  package_kind_mismatch

Tests:
- desired SERVICE + manifest INFRASTRUCTURE => fail fast
- command-like infra has no runtime requirement
- service-like infra has unit expectation
- package specs kind table has no conflicts

================================================================================
GUARDRAIL 10 — CONTROL-PLANE SELF-UPDATE / LEADER SAFETY
================================================================================

STATUS: COMPLETE (2026-05-05) — leader safety fully wired:
  etcd-backed controller.leader_pending_update + Prometheus-backed
  controller_leader_outdated and controller_no_safe_successor findings.

──────────────────────────────────────────────────────────────────────────────
What's covered (reconcile_runtime.go, leader_pending_update_etcd.go)
──────────────────────────────────────────────────────────────────────────────
[✓] reconcileControllerSelfUpdate() — reads target build from etcd
    /globular/system/controller-target-build. If target is ahead of running
    build, evaluates safe successors (followers with target build + fresh
    heartbeat + not blocked). If safe successor found: resign leadership.
[✓] followerSelfApply() — non-leader controller detects it has the target build
    and applies the update to make itself candidacy-ready.
[✓] detectBootstrapHandoff() — Day-0 bootstrap handoff to first safe successor.
[✓] controller.leader_self_resign event — emitted on resignation.
[✓] evaluateControllerFollowers() — checks followers for target build, freshness,
    and capability. Returns safe successor count.
[✓] leader_pending_update_etcd.go — injectable writeLeaderPendingUpdate() writes
    /globular/controller/leader_pending_update when safeSuccessors == 0.
    clearLeaderPendingUpdate() deletes key when leader resigns. leaderStuckSince
    atomic tracks first-detection for severity escalation.
[✓] controller.leader_pending_update doctor finding — rules/controller_leader.go
    fires SEVERITY_WARN (< 20 min stuck) or SEVERITY_ERROR (> 20 min stuck).
    5-min staleness threshold. Consumes snapshot.LeaderPendingUpdate.
[✓] Prometheus leader safety gauges exported by controller:
    globular_controller_leader_outdated
    globular_controller_no_safe_successor
[✓] doctor Prometheus findings wired from gauges:
    controller_leader_outdated (WARN when gauge > 0)
    controller_no_safe_successor (ERROR when gauge > 0)

Tests added (17 total):
  leader_pending_update_test.go (controller):
    TestLeaderPendingUpdate_WriteIsInjectable, _ClearIsInjectable,
    TestLeaderPendingUpdate_StuckSinceTracking, _ClearResetsStuckSince
  controller_leader_test.go (doctor rules):
    TestControllerLeaderPendingUpdate_NoRecord, _ZeroTimestamp, _StaleRecord,
    TestControllerLeaderPendingUpdate_FreshWarning, _EscalatesAfterThreshold,
    TestControllerLeaderPendingUpdate_EntityRefIncludesLeaderNode,
    TestControllerLeaderPendingUpdate_RemediationStepsPresent, _ZeroStuckSince
  reconcile_selfupdate_test.go (controller runtime):
    TestReconcileControllerSelfUpdate_NoSafeSuccessorWritesPendingRecord
    TestReconcileControllerSelfUpdate_SafeSuccessorClearsPendingAndResigns
  prometheus_runtime_test.go (doctor rules):
    TestPromRuntime_ControllerLeaderOutdatedFinding
    TestPromRuntime_ControllerNoSafeSuccessorFinding
    TestPromRuntime_ControllerLeaderSafetyZeroDoesNotFire

──────────────────────────────────────────────────────────────────────────────
Deferred gaps
──────────────────────────────────────────────────────────────────────────────
[ ] CLI/status showing leader build vs target build — not surfaced in standard
    health output (operator sees it via doctor report).

──────────────────────────────────────────────────────────────────────────────
Original spec below
──────────────────────────────────────────────────────────────────────────────

Goal:
A stale leader must not block newer safe followers forever.

Invariant:
Controller leader must compare build identity and resign to a safe successor when behind.

Audit:
1. Leader version/build reporting.
2. Target build record.
3. Safe follower detection:
   - installed target build
   - fresh heartbeat
   - node not blocked/unreachable
4. Dev build handling:
   - 0.0.0-dev treated as behind any valid release
5. Resign behavior:
   - clean leadership handoff
   - no split-brain

Guardrails:
- doctor finding:
  controller_leader_outdated
  controller_no_safe_successor
- CLI/status shows:
  leader build
  target build
  safe successors
- manual safe resign command documented

Tests:
- stale leader + safe follower => resign eligible
- stale leader + no safe follower => no resign, doctor warns
- 0.0.0-dev leader treated as behind
- older follower not considered safe

================================================================================
GUARDRAIL 11 — WORKFLOW SERVICE SELF-HEALING
================================================================================

STATUS: COMPLETE (2026-05-05) — workflow.service_unavailable doctor finding added.
  Lightweight restart path verified with unit tests.

──────────────────────────────────────────────────────────────────────────────
What's covered
──────────────────────────────────────────────────────────────────────────────
[✓] G3 drift detection — workflow INFRASTRUCTURE in batch test. AVAILABLE + inactive
    → DEGRADED. Covered since 2026-05-03.
[✓] release.blocked_workflow_unavailable finding — prometheus_runtime.go fires when
    `release_transient_blocked > 0`. Surfaces blocked releases in doctor.
[✓] workflow_error_classifier.go — classifies workflow_unavailable errors from
    RPC calls. Controller knows why a workflow dispatch failed.
[✓] invariant_enforcement.go — repairs missing workflow definitions in etcd
    (workflow YAML files re-registered at startup). Ensures workflow catalog
    is recoverable after etcd wipe or new leader election.
[✓] workflow service restart via node-agent — like any INFRASTRUCTURE package,
    node-agent can restart globular-workflow.service directly via systemd.
    The G3 DEGRADED trigger re-dispatches the release workflow which causes
    node-agent to restart the workflow unit. This path does NOT require the
    workflow engine to be healthy (it's a direct systemctl call).
[✓] workflow.service_unavailable doctor finding — rules/workflow_service_reachable.go
    fires SEVERITY_ERROR when doctor collector DataErrors contains a workflow
    connection-refused/Unavailable/no-route-to-host error. Distinct from the
    metric-derived release.blocked_workflow_unavailable — fires from direct
    observation by the collector, not from Prometheus.
[✓] tryLightweightRestart path verified — lightweight_restart_test.go (4 tests)
    confirms the backoff guard, endpoint guard, and blocked-reason gate all
    function correctly without a live agent connection.

Tests added (12 total):
  workflow_service_reachable_test.go (doctor rules — 8 tests):
    TestWorkflowServiceReachable_NoErrors, _NonWorkflowError,
    TestWorkflowServiceReachable_WorkflowConnectionRefused, _WorkflowUnavailable,
    TestWorkflowServiceReachable_NonUnavailableWorkflowError, _OnlyFirstUnavailable,
    TestWorkflowServiceReachable_RemedationStepsPresent, TestIsWorkflowUnavailableErr
  lightweight_restart_test.go (controller — 4 tests):
    TestLightweightRestart_BackoffPreventsCall, _NoEndpointReturnsFalse,
    TestLightweightRestart_BlockedReasonSkipsWithNoEndpoint, _InitializesRestartAttemptMap

──────────────────────────────────────────────────────────────────────────────
Original spec below
──────────────────────────────────────────────────────────────────────────────

Goal:
Workflow service cannot be the only path to repair itself.

Invariant:
Workflow is control-plane critical. Basic restart repair must not require workflow to be healthy.

Audit:
1. workflow marked ControlPlaneCritical.
2. InfrastructureRelease drift detects workflow inactive.
3. Lightweight restart path can restart workflow without dispatching workflow.
4. If workflow is required for full release, there is a fallback for workflow service itself.

Guardrails:
- doctor finding:
  workflow_unavailable
- release pipeline can perform lightweight restart for workflow service
- workflow service AVAILABLE + inactive => drift

Tests:
- workflow inactive + AVAILABLE => drift
- lightweight restart path invoked
- workflow repair does not require workflow engine

================================================================================
GUARDRAIL 12 — DOCTOR FRESHNESS / CACHE EVIDENCE
================================================================================

STATUS: COMPLETE (2026-05-03) — all freshness fields implemented and functional.

──────────────────────────────────────────────────────────────────────────────
What's covered (server.go, snapshot.go, cluster_doctor.pb.go)
──────────────────────────────────────────────────────────────────────────────
[✓] snapshot_timestamp — ReportHeader.GeneratedAt (timestamppb) in every report.
[✓] cache_hit boolean — ReportHeader.CacheHit in every report.
[✓] cache age — CacheTTL field in Freshness struct; exposed in ReportHeader.
[✓] freshness TTL — SnapshotTTL configurable per deployment; default managed by
    snapshotTTL() in config. The collector tracks TTL per snapshot.
[✓] source load errors — DataIncomplete flag in Snapshot; addError() records
    failed source RPCs. Propagated to ReportHeader.
[✓] force_fresh flag — FreshnessMode enum: FRESHNESS_CACHED (default) vs
    FRESHNESS_FRESH. force_fresh = FRESHNESS_FRESH → bypasses cache, re-collects.
    Guarded by isAuthoritative.Load() (only leader can force-fresh to prevent
    stampede). FRESHNESS_FRESH on follower downgrades to FRESHNESS_CACHED.
[✓] DataIncomplete propagation — transient read errors set DataIncomplete; findings
    do not fire CRITICAL on incomplete data (individual rules check snap fields).
[✓] ObservedAt timestamp — timestamps when data was actually observed.
[✓] Day-0 bootstrap guard (critical key diagnostics) — when snapshot evidence
    indicates likely Day-0 single-node bootstrap (foundational keys/prefixes not
    yet seeded), critical-key registry findings are downgraded
    (key missing: ERROR→WARN, prefix missing: WARN→INFO), and duplicate hard
    ingress.spec_missing is suppressed until post-bootstrap.

──────────────────────────────────────────────────────────────────────────────
Original spec below
──────────────────────────────────────────────────────────────────────────────

Goal:
Doctor must not mislead debugging with stale cached findings.

Invariant:
A finding must reveal whether it is fresh, cached, or based on a source read error.

Required:
Every doctor/MCP/admin report includes:
- snapshot timestamp
- cache_hit boolean
- cache age
- freshness TTL
- source load errors
- forced fresh flag if requested

Rules:
1. contract_missing fires CRITICAL only on confirmed absence.
2. transient read error becomes WARN/UNKNOWN with source error.
3. cached CRITICAL displays cache age.
4. CLI/MCP supports force_fresh.

Tests:
- transient etcd read error does not fire contract_missing CRITICAL
- cached finding includes cache age
- force_fresh bypasses cache
- contract appears => stale CRITICAL clears within TTL

================================================================================
PRIORITY ORDER
================================================================================

Do not implement all at once.

Before clean release test:
1. Guardrail 1: config ownership / seed file protection
2. Guardrail 2: join script shell safety
3. Guardrail 3: service-like infra drift coverage
4. Release artifact validation checklist

During clean release test:
5. Day-0 install passes
6. Day-1 join passes
7. deploy --bump patch works
8. reboot node and verify recovery

After clean test is stable:
9. Guardrail 4: systemd unit/config definition drift
10. Guardrail 5: Scylla
11. Guardrail 6: Envoy/xDS generation proof
12. Guardrail 7: repository degraded mode
13. Guardrail 8: PKI
14. Guardrail 9: package metadata consistency
15. Guardrail 10: controller self-update
16. Guardrail 11: workflow self-healing
17. Guardrail 12: doctor freshness

================================================================================
OUTPUT FORMAT FOR EACH GUARDRAIL
================================================================================

For each guardrail, return:

1. Scope
2. Files inspected
3. Current behavior
4. Pass/fail
5. Risk
6. Minimal fix if needed
7. Tests added or missing
8. Whether code was changed
9. Whether release artifact needs regeneration

Do not broaden scope without evidence.
Do not fix workloads until infra gates are clean.
Do not touch MinIO topology unless a MinIO-specific violation is found.
