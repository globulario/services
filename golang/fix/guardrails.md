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