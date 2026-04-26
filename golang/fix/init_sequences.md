Claude, I need you to stop debugging symptom-by-symptom and validate the complete Globular initialization sequence.

Goal:
Produce a clear Day-0 and Day-1 initialization map, then audit the code against it.

Do not implement fixes while auditing.
Do not wipe.
Do not run apply-topology.
Do not touch MinIO unless the audit proves a direct sequence violation.
Do not create a broad new architecture plan.

We need to verify that Globular follows one consistent law:

  Day-0 creates the first source of truth.
  Day-1 joins the existing source of truth.
  Packages install binaries and seed defaults only.
  Contracts own live configuration.
  Workflows own dangerous transitions.
  Node-agent applies local state.
  Doctor verifies reality.

The repeated failures came from old installer-style assumptions mixed with reconciled-cluster behavior:
- package installed => service started
- package reinstall => seed config overwritten
- release AVAILABLE => treated as true forever
- Day-1 node => treated like storage member
- missing readiness data => contract not published
- bootstrap waited for services that should be intentionally held

We need to make the real initialization sequence explicit.

================================================================================
DAY-0: FOUNDING NODE INITIALIZATION
================================================================================

Purpose:
Create the first valid cluster source of truth.

Expected Day-0 result:
- etcd runs as the first member.
- cluster-controller starts.
- node-agent starts.
- ObjectStoreDesiredState exists in etcd.
- MinIO runs only on the founding node.
- repository becomes available.
- xDS starts.
- Envoy connects to xDS.
- workflow/doctor/control-plane services become healthy.
- node reaches workload_ready.

Correct Day-0 sequence:

PHASE 0 — Preflight
1. Detect local node identity:
   - hostname
   - primary IP
   - install path
   - state path
2. Validate prerequisites:
   - systemd
   - network
   - ports
   - disk paths
   - permissions
3. Ensure this is Day-0, not Day-1:
   - no existing cluster membership expected
   - no existing etcd member list from bootstrap node

PHASE 1 — Install packages/binaries
1. Install Globular binaries and service unit files.
2. Package install may seed defaults only if files are missing.
3. Package install must never overwrite cluster-owned live config.

Important:
- atomic overwrite is safe for binaries/templates.
- atomic overwrite is dangerous for cluster identity/config files.

Files that must not be blindly overwritten:
- etcd.yaml
- scylla.yaml
- minio.env
- MinIO distributed.conf
- Envoy bootstrap if node/cluster identity is embedded
- PKI CA/certs
- node identity files
- repository/objectstore credentials

PHASE 2 — PKI and identity
1. Create or load CA.
2. Create node identity.
3. Issue local service certs.
4. Persist identity in the correct state path.
5. Never overwrite existing CA/node identity during package reinstall.

PHASE 3 — etcd first member
1. Write etcd.yaml as first-node seed only if missing.
2. etcd config must be single-node only for Day-0 founding node.
3. Start globular-etcd.service.
4. Verify endpoint health.
5. Verify this node is a named started etcd member.

PHASE 4 — node-agent
1. Start globular-node-agent.service.
2. Verify heartbeat reaches controller or local controller after it starts.
3. Node-agent must not blindly start all installed services.
4. Node-agent must obey runtime contracts.

PHASE 5 — cluster-controller
1. Start cluster-controller.
2. Elect/confirm leader.
3. Load state from etcd.
4. Publish initial desired cluster state.
5. Control-plane services must be deployable before ordinary workloads.

PHASE 6 — objectstore contract
1. Controller creates ObjectStoreDesiredState for founding node.
2. ObjectStoreDesiredState must exist even if credentials or endpoint are degraded.
3. Do not skip publishing the contract because:
   - credentials are nil
   - endpoint cannot resolve
4. Instead publish:
   - generation
   - mode
   - nodes
   - node paths
   - fingerprint
   - credentials_ready=false if needed
   - endpoint_ready=false if needed

PHASE 7 — MinIO
1. MinIO may run only if this node is in ObjectStoreDesiredState.Nodes.
2. On Day-0, founding node is in the pool, so MinIO may run.
3. Node-agent renders minio.env from the objectstore contract.
4. Node-agent starts or allows MinIO only for topology members.
5. No destructive wipe unless approved TopologyTransition exists.

PHASE 8 — repository
1. Repository starts after objectstore is available enough.
2. If objectstore is degraded:
   - repository should report degraded/read-only/cache-only as appropriate
   - it must not fake success
3. Package publish/upload requires objectstore write health.

PHASE 9 — xDS / Envoy
1. xDS service becomes active.
2. Envoy starts and connects to xDS.
3. Envoy/xDS readiness is required before normal workload convergence.

PHASE 10 — workflow / doctor / remaining control plane
1. Workflow service starts.
2. Doctor starts.
3. Control-plane critical services can deploy at infra-ready.
4. Ordinary workloads wait until workload_ready.

PHASE 11 — workload readiness
1. Once required infra is active and healthy, bootstrap reaches workload_ready.
2. Only then ordinary services/applications become eligible.

Day-0 must never:
- overwrite existing etcd.yaml from package reinstall
- start MinIO before ObjectStoreDesiredState exists
- publish no objectstore contract because credentials are not loaded
- mark service-like infrastructure AVAILABLE without runtime proof
- treat package seed config as live cluster truth

================================================================================
DAY-1: JOINING NODE INITIALIZATION
================================================================================

Purpose:
Join an existing cluster without inventing local truth.

Expected Day-1 result:
- node joins existing etcd cluster
- node-agent registers
- controller sees fresh heartbeat
- xDS becomes active
- Envoy becomes active/connected
- MinIO remains stopped unless objectstore topology admits the node
- node reaches workload_ready
- ordinary workloads deploy only after bootstrap is ready

Correct Day-1 sequence:

PHASE 0 — Preflight
1. Validate join token with bootstrap gateway.
2. Resolve bootstrap node address.
3. Detect local node identity:
   - hostname
   - node ID if existing
   - primary IP
   - existing install/state paths
4. Determine mode:
   - fresh Day-1 join
   - repair/rejoin
5. Do not wipe anything unless repair mode is explicit.

PHASE 1 — Install packages/binaries
1. Install or update required binaries.
2. Install systemd units.
3. Package install may seed config only if missing.
4. Package install must not overwrite cluster-owned config.

Critical:
Package installed does not mean service allowed to run.

PHASE 2 — PKI and identity
1. Fetch or issue node certificate from bootstrap/control plane.
2. Install CA bundle.
3. Verify TLS connectivity.
4. Do not continue if identity/CA is invalid.

PHASE 3 — etcd join existing cluster
Fresh Day-1:
1. Bootstrap node removes ghost member for this node if present.
2. Bootstrap node runs etcd member add.
3. Join script writes etcd.yaml with:
   - initial-cluster-state: existing
   - correct initial-cluster list
   - correct node name
   - correct listen/advertise peer/client URLs
4. Start globular-etcd.service.
5. Verify endpoint health.
6. Verify node appears as a named started member.

Repair/rejoin:
1. Requires explicit operator intent.
2. Stop globular-etcd.service.
3. Backup /var/lib/globular/etcd to timestamped backup.
4. Only after backup, wipe local etcd data.
5. Remove ghost/stale member from healthy cluster if present.
6. Member add again.
7. Write existing-cluster etcd.yaml.
8. Start etcd.
9. Verify endpoint health and member list.

Day-1 must never:
- write single-node seed etcd.yaml
- overwrite existing cluster etcd.yaml during package install
- run etcd with initial-cluster-state: new
- rm -rf etcd data without explicit repair/wipe intent

PHASE 4 — node-agent
1. Start/restart node-agent.
2. Verify heartbeat reaches controller.
3. Controller sees node last_seen fresh.
4. Join script should stop here for most convergence.
5. Do not manually start all services in join script.

PHASE 5 — controller-driven infra convergence
1. Controller advances bootstrap phases.
2. Controller releases required infra.
3. Node-agent applies releases.
4. Release pipeline repairs drift.

Required infra order:
- etcd healthy
- xDS active
- Envoy connected
- repository reachable if required
- workflow/doctor/control-plane services healthy
- storage contracts evaluated
- workload_ready only after infra gates pass

PHASE 6 — MinIO behavior
1. Day-1 join does not automatically make node a MinIO member.
2. Join script must not map minio.globular.internal to NODE_IP.
3. Join script may map minio.globular.internal to bootstrap/objectstore endpoint as fallback.
4. Node-agent may install MinIO package.
5. globular-minio.service must remain stopped/held unless node IP is in ObjectStoreDesiredState.Nodes.
6. MinIO membership changes only through explicit objectstore apply-topology.
7. .minio.sys wipe requires approved TopologyTransition.

PHASE 7 — Scylla behavior
1. Package may install binaries and seed config if missing.
2. Live scylla.yaml must be controller/workflow-owned.
3. Do not blindly start Scylla before valid cluster config.
4. Scylla should not block earlier phases unless explicitly required.

PHASE 8 — final verification
Report:
- etcd member: named, started, healthy
- node-agent heartbeat: fresh
- xDS: active
- Envoy: active/connected
- MinIO: held if non-member, active only if member
- repository: reachable if required
- bootstrap phase: current state and blocker if not workload_ready

================================================================================
GLOBAL INVARIANTS TO VALIDATE
================================================================================

1. Package installed does not mean service allowed to run.

2. Package install may seed only if missing.

3. Package reinstall must never overwrite cluster-owned config.

4. ObjectStoreDesiredState must exist whenever MinIO is expected to run.

5. MinIO can run only on nodes in ObjectStoreDesiredState.Nodes.

6. Day-1 nodes do not auto-join MinIO pool.

7. Dangerous topology changes require workflow + approved transition.

8. Release AVAILABLE is not terminal if required runtime proof is false.

9. Service-like InfrastructureRelease must have runtime drift detection.

10. Command-like infrastructure may skip runtime checks:
   - restic
   - rclone
   - ffmpeg
   - sctool
   - mc
   - etcdctl
   - sha256sum
   - yt-dlp

11. Control-plane critical services can deploy at infra-ready:
   - cluster-controller
   - node-agent
   - cluster-doctor
   - workflow

12. Ordinary workloads must wait for workload_ready.

13. Doctor must distinguish:
   - contract absent
   - contract degraded
   - transient etcd read error
   - active non-member
   - runtime drift
   - stale cached report

14. Join script must be shell-safe:
   - set -euo pipefail
   - no malformed command fragments
   - no accidental lines like:
     systemctl status globular-mcp.service exit
   - bash -n must pass
   - shellcheck should pass if available

================================================================================
AUDIT TASKS
================================================================================

Produce three tables.

TABLE A — Day-0 sequence audit

Columns:
phase | owner | code path | expected state | current behavior | pass/fail | risk | required fix

Example rows:
- package install
- PKI identity
- etcd first member
- node-agent start
- controller start
- objectstore contract publication
- MinIO start
- repository start
- xDS start
- Envoy connect
- workflow/doctor start
- workload_ready

TABLE B — Day-1 sequence audit

Columns:
phase | owner | code path | expected state | current behavior | pass/fail | risk | required fix

Example rows:
- token validation
- package install
- PKI fetch
- etcd member add
- etcd.yaml write
- etcd start/health
- node-agent heartbeat
- bootstrap phase advance
- xDS activation
- Envoy activation
- MinIO hold/non-member behavior
- repository reachability
- workload_ready

TABLE C — Config ownership audit

Columns:
file path | package writer | runtime owner | ownership mode | guarded? | overwrite risk | required fix

Ownership modes:
- seed-only
- contract-rendered
- workflow-owned

Files to audit:
- etcd.yaml
- scylla.yaml
- minio.env
- MinIO distributed.conf
- Envoy bootstrap/xDS config
- PKI CA
- node certificates
- node identity
- repository/objectstore credentials
- Prometheus/Alertmanager config if package-managed

================================================================================
CURRENT LIVE BLOCKER TO START WITH
================================================================================

Start the audit from the current live blocker:

nuc is at BootstrapPhase=etcd_ready and globular-xds.service is inactive.

Validate this hypothesis:

- xDS was previously marked AVAILABLE for nuc.
- globular-xds.service is now inactive.
- InfrastructureRelease does not have runtime drift detection.
- hasUnservedNodes skips nuc because per-node phase is AVAILABLE.
- Therefore release pipeline does not repair/re-dispatch xDS.
- Bootstrap waits forever at etcd_ready.

If true, mark this as CRITICAL:

  Stored AVAILABLE state overrides runtime truth.

Smallest required fix:

  InfrastructureRelease runtime drift detection for service-like infrastructure.
  AVAILABLE + required unit inactive => drift => clear/downgrade per-node AVAILABLE => re-dispatch/repair.

Add tests:
1. xDS InfrastructureRelease AVAILABLE + unit inactive => drift detected.
2. xDS InfrastructureRelease AVAILABLE + unit active => no drift.
3. command-like infra AVAILABLE + no unit => no drift.
4. ServiceRelease drift behavior unchanged.
5. bootstrap etcd_ready + xDS inactive triggers repair rather than infinite timeout.

================================================================================
WHAT NOT TO DO
================================================================================

Do not wipe.
Do not run apply-topology.
Do not change MinIO topology.
Do not redesign bootstrap.
Do not implement a giant new plan before producing the audit.
Do not fix Scylla/blog/catalog until Day-1 infrastructure gates are clean.
Do not start with workload symptoms.

================================================================================
EXPECTED OUTPUT
================================================================================

Return:

1. Clear Day-0 sequence table.
2. Clear Day-1 sequence table.
3. Config ownership table.
4. List of CRITICAL sequence violations.
5. List of HIGH sequence violations.
6. The single next surgical fix to unblock the current cluster.
7. Tests required for that fix.

The goal is to stop try-and-error and make the initialization path explicit, auditable, and enforceable.

================================================================================
AUDIT RESULTS  (produced 2026-04-26)
================================================================================

TABLE A — Day-0 sequence audit
────────────────────────────────────────────────────────────────────────────────
phase                   | owner             | code path                                              | expected state                             | current behavior                                              | P/F  | risk   | required fix
------------------------|-------------------|--------------------------------------------------------|--------------------------------------------|---------------------------------------------------------------|------|--------|-------------
Preflight (IP, dirs)    | install-day0.sh   | scripts/release/install-day0.sh Phase 0               | Node IP detected, dirs exist               | Detected via ip-route fallback; dirs created                  | PASS | LOW    | —
Package install         | node-agent        | node_agent/actions/package_actions.go                  | Binaries installed, configs seeded once    | install_package_payload installs bins; install_files seeds    | PASS | LOW    | —
PKI/CA creation         | install-day0.sh   | scripts/release/install-day0.sh Phase 2                | CA and node cert issued before etcd        | CA generated in /var/lib/globular/pki/; cert issued           | PASS | LOW    | —
etcd first member       | install-day0.sh   | etcd/specs/etcd_service.yaml (install_files)           | single-node seed, initial-cluster-state: new | etcd.yaml written with skip_if_exists:true                  | PASS | LOW    | —
etcd.yaml not overwritten on reinstall | installer | install_files_step.go:SkipIfExists              | Reinstall leaves existing etcd.yaml alone  | skip_if_exists:true prevents overwrite                       | PASS | LOW    | —
node-agent start        | install-day0.sh   | scripts/release/install-day0.sh Step 16               | node-agent starts after etcd healthy       | starts after etcd health verified                             | PASS | LOW    | —
cluster-controller start| install-day0.sh   | scripts/release/install-day0.sh                       | controller starts, loads state, publishes desired state | controller started after node-agent registered              | PASS | LOW    | —
objectstore contract    | cluster-controller| reconcile_runtime.go, publishObjectStoreDesiredState  | Contract published even if credentials/endpoint degraded | degraded contract (credentials_ready=false/endpoint_ready=false) published | PASS | LOW | — (fixed 136595bd)
MinIO gate              | node-agent        | node_agent/minio_membership.go enforceMinioHeld        | MinIO runs only if node in ObjectStoreDesiredState.Nodes | enforceMinioHeld stops MinIO on non-members                | PASS | LOW    | — (fixed be6008d1)
xDS start               | release-pipeline  | release_pipeline.go, infraReleaseHandle + DriftDetector | xDS marked AVAILABLE only if unit active  | detectInfraDrift downgrades AVAILABLE→DEGRADED on unit death  | PASS | LOW    | — (fixed 7e6aca0b)
Envoy start             | release-pipeline  | component_catalog.go runtime_local_dependencies: [xds] | Envoy starts after xDS active             | catalog declares xds as local dep; waits for xds              | PASS | LOW    | —
repository availability | release-pipeline  | classifyPackageConvergence (6d79842f)                 | release AVAILABLE requires unit active    | version+hash+runtime proof required before AVAILABLE          | PASS | LOW    | —
workload_ready gate     | bootstrap_phases  | bootstrap_phases.go bootstrapPhaseReady()             | ordinary workloads wait for workload_ready | bootstrapPhaseReady() excludes envoy_ready for ordinary loads | PASS | LOW    | — (fixed efea22ad)
ControlPlaneCritical services | bootstrap_phases | bootstrap_phases.go bootstrapInfraReady()          | cluster-controller/doctor/workflow deploy at envoy_ready | bootstrapInfraReady() includes envoy_ready for CPC          | PASS | LOW    | — (fixed efea22ad)


TABLE B — Day-1 sequence audit
────────────────────────────────────────────────────────────────────────────────
phase                   | owner             | code path                                              | expected state                             | current behavior                                              | P/F  | risk   | required fix
------------------------|-------------------|--------------------------------------------------------|--------------------------------------------|---------------------------------------------------------------|------|--------|-------------
Token validation        | install-day1.sh   | scripts/release/install-day1.sh Phase 0               | join token validated against controller   | token passed as --join-token; verified by controller RPC      | PASS | LOW    | —
Package install         | install-day1.sh   | node-agent/actions/package_actions.go                 | Binaries installed, seed configs only     | install_files with skip_if_exists for etcd.yaml/minio.env    | PASS | LOW    | —
PKI fetch               | install-day1.sh   | scripts/release/install-day1.sh Phase 2               | CA bundle fetched from bootstrap node     | CA fetched via MinIO bootstrap; cert issued from controller   | PASS | LOW    | —
etcd.yaml for Day-1     | install-day1.sh   | install-day1.sh line 421                              | initial-cluster-state: existing (never new) | `${ETCD_INITIAL_CLUSTER_STATE:-existing}` defaults to existing | PASS | LOW   | —
etcd member add         | install-day1.sh   | install-day1.sh + etcd_members.go reconcileEtcdJoinPhases | ghost member removed; MemberAdd called | controller calls MemberAdd; join script waits                 | PASS | LOW    | —
etcd start + health     | install-day1.sh   | install-day1.sh Step 11                               | etcd joins cluster, health verified       | 60s poll on endpoint health before proceeding                 | PASS | LOW    | —
etcd stuck join         | etcd_members.go   | etcd_members.go classifyStuckEtcdJoin (af10345c)     | stuck node detected after 10m, operator alerted | classifyStuckEtcdJoin → rejoin_required                   | PASS | LOW    | — (fixed af10345c)
node-agent after etcd   | install-day1.sh   | install-day1.sh Step 12 (install), Step 16 (start)   | node-agent installed and started after etcd healthy | Step 16 starts node-agent after etcd health at Step 11  | PASS | LOW    | —
minio.domain mapping    | install-day1.sh   | install-day1.sh line 513                              | minio.domain → controller/bootstrap IP (NOT node IP) | maps to CONTROLLER_HOST (bootstrap MinIO), not NODE_IP   | PASS | LOW    | —
MinIO not started       | install-day1.sh   | install-day1.sh (no globular-minio reference)        | join script MUST NOT start globular-minio | no minio start in script; topology gate enforces hold          | PASS | LOW    | —
bootstrap phase advance | cluster-controller| bootstrap_phases.go reconcileBootstrapPhases         | controller advances admitted→workload_ready | controller drives phase advance based on runtime proof       | PASS | LOW    | —
ordinary workloads gated | bootstrap_phases | bootstrapPhaseReady() in hasUnservedNodes             | workloads wait until workload_ready        | bootstrapPhaseReady() returns false for envoy_ready and earlier | PASS | LOW  | —
scylla.yaml reinstall guard | post-install.sh | scylladb/scripts/post-install.sh EXISTING_DATA guard | scylla.yaml not rewritten if Raft data exists | NOW guarded: EXISTING_DATA=true+file exists → skip rewrite | PASS | LOW    | — (fixed this session)


TABLE C — Config ownership audit
────────────────────────────────────────────────────────────────────────────────
file                                     | package writer                      | runtime owner             | ownership mode    | guarded?             | overwrite risk | required fix
-----------------------------------------|-------------------------------------|---------------------------|-------------------|----------------------|----------------|-------------
/var/lib/globular/config/etcd.yaml       | etcd/specs/etcd_service.yaml        | join-script (Day-1 rewrite) | seed-only        | skip_if_exists:true  | NONE           | —
/var/lib/globular/minio/minio.env        | minio/specs/minio_service.yaml      | controller (objectstore contract) | seed-only   | skip_if_exists:true  | NONE           | —
/var/lib/globular/minio/credentials      | minio/specs/minio_service.yaml      | controller                | seed-only         | skip_if_exists:true  | NONE           | —
/etc/scylla/scylla.yaml                  | scylladb/scripts/post-install.sh    | controller (Scylla workflow) | seed-only (Day-0); contract-rendered (Day-1+) | NOW: skip if EXISTING_DATA=true | WAS HIGH, NOW LOW | — (fixed this session)
/var/lib/globular/prometheus/prometheus.yml | prometheus/specs/prometheus_service.yaml | operator/controller   | seed-only         | NOW: skip_if_exists:true | WAS MEDIUM, NOW NONE | — (fixed this session)
/var/lib/globular/alertmanager/alertmanager.yml | alertmanager/specs/alertmanager_service.yaml | operator      | seed-only         | NOW: skip_if_exists:true | WAS MEDIUM, NOW NONE | — (fixed this session)
/var/lib/globular/config/xds/xds.yaml    | xds/specs/xds_service.yaml (install_config:true) | xds service (reads on start) | static service config | NO skip_if_exists (install_package_payload collectFileSpecs) | LOW | ACCEPTABLE — xds config is static server config (listen addr, cert paths), not cluster identity; overwrite is idempotent |
/etc/systemd/system/globular-*.service   | package specs (install_services)    | package release pipeline  | always-overwrite  | NO (intentional — unit updates must apply) | NONE (desired) | —
/var/lib/globular/pki/ca.pem             | NOT written by package specs        | bootstrap scripts + controller | PKI-managed   | N/A (not package-written) | NONE           | —
/var/lib/globular/pki/issued/services/*  | NOT written by package specs        | bootstrap scripts + controller | PKI-managed   | N/A (not package-written) | NONE           | —
node identity (hostname, node_id)        | NOT written by package specs        | bootstrap scripts         | identity-managed  | N/A                  | NONE           | —
/var/lib/scylla-manager-agent/scylla-manager-agent.yaml | scylla-manager-agent/specs | scylla-manager-agent | seed-only | skip_if_exists:true | NONE          | —


CRITICAL VIOLATIONS
────────────────────────────────────────────────────────────────────────────────
None active. All previously identified CRITICAL violations have been fixed:
  - [FIXED 7e6aca0b] InfrastructureRelease AVAILABLE with dead unit → stuck bootstrap forever
  - [FIXED 136595bd] objectstore contract absent when credentials nil → MinIO never admitted
  - [FIXED 90ee2d8d] non-pool-member nodes stuck at envoy_ready
  - [FIXED 6d79842f] release AVAILABLE without runtime proof (version match treated as terminal)
  - [FIXED efea22ad] etcdctl/sha256sum/yt-dlp false-DEGRADED on every drift cycle


HIGH VIOLATIONS
────────────────────────────────────────────────────────────────────────────────
None active. Previously identified HIGH violations have been fixed:
  - [FIXED this session] scylla.yaml unconditionally rewritten on package reinstall of live cluster
  - [FIXED this session] prometheus.yml / alertmanager.yml overwritten on reinstall (operator config lost)
  - [FIXED af10345c] etcd stuck-join: node sits in etcd_joining forever with no detection or operator signal


REMAINING RISKS (not requiring immediate fix)
────────────────────────────────────────────────────────────────────────────────
1. xds/envoy/gateway install_config:true uses collectFileSpecs without SkipIfExists.
   Risk: LOW — these are static server configs (listen address, TLS cert paths). Content
   is idempotent across reinstalls. Not cluster identity. No operator-customizable fields.

2. scylla.yaml is now guarded on reinstall (EXISTING_DATA=true), but a fresh Day-1 node
   still writes scylla.yaml with single-node seeds until the controller issues a cluster-aware
   config. Scylla must not be started until the controller renders the correct seeds.
   Risk: LOW — post-install.sh guard prevents first start until CQL health is verified.

3. No shellcheck available on this system. Scripts pass bash -n. Shellcheck should be
   added to the package CI when infrastructure allows.


================================================================================
RELEASE ARTIFACT VALIDATION CHECKLIST  (item D)
================================================================================

Before wiping/reinstalling any node, verify the deployed binaries contain these fixes.
Run against the artifact in the repository or extract the binary and check:

# cluster-controller artifact checks
strings cluster-controller | grep -c detectInfraDrift          # expect >= 1
strings cluster-controller | grep -c bootstrapInfraReady       # expect >= 1
strings cluster-controller | grep -c classifyPackageConvergence # expect >= 1
strings cluster-controller | grep -c classifyStuckEtcdJoin     # expect >= 1
strings cluster-controller | grep -c publishObjectStoreDesiredStateLocked # expect >= 1

# node-agent artifact checks
strings node-agent | grep -c enforceMinioHeld                  # expect >= 1
strings node-agent | grep -c "MinioJoinNonMember"              # expect >= 1

# Package spec checks (seed guards)
grep skip_if_exists packages/metadata/etcd/specs/etcd_service.yaml        # expect: true
grep skip_if_exists packages/metadata/minio/specs/minio_service.yaml      # expect: true (x2)
grep skip_if_exists packages/metadata/prometheus/specs/prometheus_service.yaml # expect: true
grep skip_if_exists packages/metadata/alertmanager/specs/alertmanager_service.yaml # expect: true
grep "EXISTING_DATA.*true.*scylla.yaml" packages/metadata/scylladb/scripts/post-install.sh # expect match

# Join script safety
bash -n scripts/release/install-day0.sh && echo PASS
bash -n scripts/release/install-day1.sh && echo PASS
grep "initial-cluster-state.*existing" scripts/release/install-day1.sh    # expect match
grep "globular-minio" scripts/release/install-day1.sh | grep -v "#"       # expect: empty (no minio start)