# Globular V1 Test Harness Implementation Plan

This document turns the Globular V1 test plan into an implementation blueprint. It defines:

- the recommended directory structure
- the YAML scenario schema
- the make targets
- the CI stages
- the execution model for the Docker simulation harness

The goal is to make the test system reproducible, inspectable, and scalable so Globular V1 can be validated by evidence instead of manual intuition.

---

## 1. Implementation goals

The harness must support four things:

1. repeatable Docker cluster bring-up and teardown
2. machine-readable test scenarios and failure drills
3. automatic evidence collection and report generation
4. promotion from local smoke to nightly resilience to release certification

This harness is not just a pile of shell scripts. It should become the platform's verification spine.

---

## 2. Recommended repository structure

```text
/tests
  /README.md

  /harness
    /bin
      globular-test
      globular-scenario
      globular-probe
      globular-chaos
      globular-report
    /lib
      cluster.sh
      probes.sh
      workflows.sh
      authz.sh
      packaging.sh
      reports.sh
      assertions.sh
      chaos.sh
      fixtures.sh
    /templates
      report-summary.md.tmpl
      parity-report.json.tmpl
      junit.xml.tmpl

  /docker
    docker-compose.quickstart.yml
    docker-compose.test.yml
    docker-compose.chaos.yml
    /.env
    /images
      /node
        Dockerfile
      /runner
        Dockerfile
    /volumes
      /.gitkeep

  /fixtures
    /packages
      /echo-service
      /test-service
      /bad-package-missing-bin
      /bad-package-bad-checksum
    /rbac
      viewer-token.json
      operator-token.json
      admin-token.json
    /workflows
      test-workflow-basic.yaml
      test-workflow-blocked.yaml
    /data
      sample-backup.json
      seed-state.json

  /scenarios
    /smoke
      cluster-cold-boot.yaml
      service-health-minimal.yaml
      authz-basic.yaml
    /functional
      parity-runtime.yaml
      publish-install-service.yaml
      workflow-basic.yaml
      repository-lifecycle.yaml
    /security
      semantic-rbac-enforcement.yaml
      bootstrap-window.yaml
      unauthorized-mutations.yaml
    /resilience
      service-stop-remediation.yaml
      controller-failover.yaml
      workflow-resume.yaml
      scylla-delay.yaml
      minio-outage.yaml
      network-partition.yaml
    /recovery
      node-join.yaml
      drift-repair.yaml
      full-reseed.yaml
      backup-restore.yaml
      disaster-restore.yaml
    /soak
      cold-boot-repeated.yaml
      24h-soak.yaml
      chaos-batch.yaml

  /reports
    /.gitkeep
    /latest
    /archive

  /golden
    feature-parity.json
    service-health-baseline.json
    package-lifecycle-baseline.json

  /ci
    smoke.sh
    functional.sh
    resilience.sh
    recovery.sh
    soak.sh
    collect-artifacts.sh
```

---

## 3. Test harness roles

### 3.1 `globular-test`
Top-level command for running suites and individual scenarios.

Examples:

```bash
./tests/harness/bin/globular-test suite smoke
./tests/harness/bin/globular-test suite functional
./tests/harness/bin/globular-test scenario ./tests/scenarios/recovery/full-reseed.yaml
```

### 3.2 `globular-scenario`
Loads and executes one YAML scenario.

Responsibilities:
- parse YAML
- validate schema
- execute preconditions
- execute injections and assertions
- collect evidence
- emit pass/fail summary

### 3.3 `globular-probe`
Runs read-only probes against the cluster.

Examples:
- service health
- workflow state
- repository state
- node state
- DNS lookups
- RBAC authorization checks
- event presence
- monitoring metrics

### 3.4 `globular-chaos`
Injects controlled failures.

Examples:
- stop service
- kill unit
- kill leader
- block port
- inject latency/loss
- remove artifact file
- delay Scylla readiness
- corrupt package

### 3.5 `globular-report`
Builds outputs from a run.

Outputs:
- markdown summary
- JSON evidence bundle
- JUnit XML for CI
- timelines and drill trace logs

---

## 4. YAML scenario schema

Each scenario should be declarative and machine-readable.

### 4.1 High-level schema

```yaml
version: 1
name: service-stop-remediation
description: Stop a non-critical service and verify doctor/remediation path.
suite: resilience
tags:
  - doctor
  - remediation
  - service-failure
cluster:
  profile: quickstart-5node
  require_healthy_before_start: true
  reset_before: true
  reset_after: false

preconditions:
  - type: probe
    id: cluster_healthy
    probe: cluster.health
    expect:
      status: healthy

baseline:
  - type: probe
    id: target_service_running
    probe: service.status
    params:
      node: node-3
      service: event-publisher
    expect:
      unit_state: active
      health: healthy

steps:
  - id: stop_service
    action: chaos.stop_service
    params:
      node: node-3
      service: event-publisher

  - id: wait_for_detection
    action: wait
    params:
      timeout: 120
      poll_interval: 5
      until:
        probe: doctor.finding
        params:
          service: event-publisher
        expect:
          present: true
          severity: warning

  - id: verify_remediation
    action: wait
    params:
      timeout: 180
      poll_interval: 5
      until:
        probe: service.status
        params:
          node: node-3
          service: event-publisher
        expect:
          unit_state: active
          health: healthy

assertions:
  - type: probe
    probe: workflow.last_run
    params:
      workflow: remediate.doctor.finding
    expect:
      status: succeeded

  - type: probe
    probe: cluster.health
    expect:
      status: healthy

artifacts:
  collect:
    - type: logs
      node: node-3
      service: event-publisher
    - type: logs
      node: leader
      service: cluster-controller
    - type: workflow_trace
      workflow: remediate.doctor.finding
    - type: doctor_findings

cleanup:
  - type: probe
    probe: cluster.health
    expect:
      status: healthy
```

---

## 5. Supported scenario sections

### 5.1 `cluster`
Defines the cluster mode.

Fields:
- `profile`: `quickstart-5node`, `reduced-1node`, `security-3node`, etc.
- `require_healthy_before_start`: boolean
- `reset_before`: boolean
- `reset_after`: boolean
- `preserve_artifacts_on_failure`: boolean

### 5.2 `preconditions`
Checks that must pass before execution.

Examples:
- cluster healthy
- repository reachable
- workflow service reachable
- desired state present
- service deployed

### 5.3 `baseline`
Captures state before mutation.

Examples:
- current leader
- service active
- node ready
- package installed version
- workflow count

### 5.4 `steps`
Mutable actions.

Supported action families:
- `chaos.stop_service`
- `chaos.kill_process`
- `chaos.delay_start`
- `chaos.network_partition`
- `chaos.inject_latency`
- `chaos.block_dns`
- `chaos.corrupt_artifact`
- `chaos.remove_file`
- `publish.service_package`
- `repository.promote`
- `desired_state.upsert`
- `workflow.dispatch`
- `backup.create`
- `backup.restore`
- `node.recover.full_reseed`
- `wait`

### 5.5 `assertions`
Final or intermediate expectations.

Supported assertion types:
- service healthy
- workflow status
- artifact present
- action denied
- node fenced/unfenced
- metrics threshold
- audit event present

### 5.6 `artifacts`
Evidence collection.

Supported outputs:
- logs
- workflow traces
- doctor findings
- metrics snapshots
- etcd snapshots
- repository metadata
- RBAC decision traces
- JUnit summary

### 5.7 `cleanup`
Cleanup or post-check tasks.

Examples:
- restore connectivity
- un-fence node
- verify cluster healthy
- archive logs

---

## 6. Standard probe catalog

The probe layer should be standardized so scenarios remain simple.

### Cluster probes
- `cluster.health`
- `cluster.nodes`
- `cluster.leader`
- `cluster.desired_state`
- `cluster.routing_status`

### Service probes
- `service.status`
- `service.health`
- `service.discovery`
- `service.rpc`
- `service.logs_tail`

### Workflow probes
- `workflow.last_run`
- `workflow.run`
- `workflow.step`
- `workflow.blocked`
- `workflow.history`

### Repository probes
- `repository.artifact`
- `repository.version`
- `repository.lifecycle_state`
- `repository.bundle`

### RBAC probes
- `authz.check`
- `authz.role_bindings`
- `authz.permission_mapping`
- `authz.fallback_usage`

### Recovery probes
- `node.fenced`
- `node.installed_packages`
- `node.runtime_health`
- `backup.last_job`
- `restore.last_job`

### Observability probes
- `doctor.finding`
- `doctor.findings`
- `audit.event`
- `metrics.query`
- `events.consume`

---

## 7. Scenario design rules

### Rule 1. Every scenario must prove one thing cleanly
Do not make one YAML do ten unrelated things.

### Rule 2. Every destructive scenario must have evidence collection
If it fails, you need trace, logs, workflow state, and health snapshots.

### Rule 3. Every scenario must declare expected truth model
Use expected states like:
- `healthy`
- `degraded`
- `blocked`
- `inconclusive`
- `failed safely`

### Rule 4. Avoid hidden sleeps
Use `wait until probe == expected` rather than blind delays.

### Rule 5. Assert denials as carefully as successes
A security test that only checks success is only half a test.

---

## 8. Make targets

These targets provide the main operator interface.

### Cluster lifecycle
```make
quickstart-up:
	./tests/harness/bin/globular-test cluster up quickstart-5node

quickstart-down:
	./tests/harness/bin/globular-test cluster down quickstart-5node

quickstart-reset:
	./tests/harness/bin/globular-test cluster reset quickstart-5node

quickstart-logs:
	./tests/harness/bin/globular-test cluster logs quickstart-5node
```

### Fast checks
```make
check-test-schemas:
	./tests/harness/bin/globular-test check schemas

check-test-scenarios:
	./tests/harness/bin/globular-test check scenarios

check-test-fixtures:
	./tests/harness/bin/globular-test check fixtures
```

### Suites
```make
test-smoke:
	./tests/harness/bin/globular-test suite smoke

test-functional:
	./tests/harness/bin/globular-test suite functional

test-security:
	./tests/harness/bin/globular-test suite security

test-resilience:
	./tests/harness/bin/globular-test suite resilience

test-recovery:
	./tests/harness/bin/globular-test suite recovery

test-soak:
	./tests/harness/bin/globular-test suite soak
```

### Special reports
```make
test-parity-report:
	./tests/harness/bin/globular-test report parity

test-health-matrix:
	./tests/harness/bin/globular-test report service-health

test-authz-report:
	./tests/harness/bin/globular-test report authz

test-recovery-report:
	./tests/harness/bin/globular-test report recovery
```

### Certification
```make
test-v1-certification:
	./tests/harness/bin/globular-test suite smoke && \
	./tests/harness/bin/globular-test suite functional && \
	./tests/harness/bin/globular-test suite security && \
	./tests/harness/bin/globular-test suite resilience && \
	./tests/harness/bin/globular-test suite recovery
```

### Debug helpers
```make
test-scenario:
	./tests/harness/bin/globular-test scenario $(SCENARIO)

test-scenario-keep:
	./tests/harness/bin/globular-test scenario $(SCENARIO) --keep-cluster --keep-artifacts

test-debug-shell:
	./tests/harness/bin/globular-test debug shell $(NODE)
```

---

## 9. CI stage plan

Use staged validation instead of one giant wall.

### Stage 1. Lint and schema checks
Runs on every PR.

Checks:
- scenario YAML schema validation
- fixture/package validity
- static invariants
- test harness shell/script lint

### Stage 2. Smoke
Runs on every PR or protected branch merge.

Covers:
- reduced or 1-node smoke cluster
- critical services reachable
- one workflow
- one RBAC allow/deny
- one repository probe

### Stage 3. Functional
Runs on merge/nightly.

Covers:
- full 5-node cluster
- cold boot convergence
- feature parity report
- service health matrix
- publish/install package

### Stage 4. Security
Runs nightly and pre-release.

Covers:
- semantic authz chain
- bootstrap window tests
- role denials
- unauthorized mutations
- fallback usage report

### Stage 5. Resilience
Runs nightly or pre-release.

Covers:
- service stop/crash drills
- controller restart
- workflow resume
- Scylla/MinIO outage
- network fault drills

### Stage 6. Recovery
Runs pre-release.

Covers:
- node join
- drift repair
- full reseed
- backup/restore
- disaster recovery simulation

### Stage 7. Soak
Runs weekly or release candidate.

Covers:
- repeated cold boots
- 12h/24h soak
- repeated workflow execution
- chaos batch

---

## 10. CI artifact retention

Every non-trivial CI stage should publish artifacts.

Required retained artifacts:
- compose logs
- per-node systemd/service logs
- workflow traces
- doctor findings
- RBAC/authz report
- feature parity report
- service health matrix
- package publish/install trace
- recovery trace
- JUnit XML
- summary markdown

Recommended retention:
- PR runs: 7 days
- nightly: 14 days
- release candidate: 30 days

---

## 11. Suggested initial implementation order

### Wave 1. Harness skeleton
Build:
- directory structure
- cluster up/down/reset wrappers
- report directory layout
- schema validator
- top-level `globular-test`

### Wave 2. Probe layer
Implement probes first:
- cluster.health
- service.status
- workflow.last_run
- repository.artifact
- authz.check
- doctor.finding

Without probes, scenario YAML becomes blind.

### Wave 3. Smoke scenarios
Implement:
- `cluster-cold-boot.yaml`
- `service-health-minimal.yaml`
- `authz-basic.yaml`

### Wave 4. Functional scenarios
Implement:
- `parity-runtime.yaml`
- `publish-install-service.yaml`
- `workflow-basic.yaml`

### Wave 5. Security scenarios
Implement:
- `semantic-rbac-enforcement.yaml`
- `bootstrap-window.yaml`
- `unauthorized-mutations.yaml`

### Wave 6. Resilience scenarios
Implement:
- `service-stop-remediation.yaml`
- `controller-failover.yaml`
- `workflow-resume.yaml`
- `scylla-delay.yaml`

### Wave 7. Recovery scenarios
Implement:
- `node-join.yaml`
- `drift-repair.yaml`
- `full-reseed.yaml`
- `backup-restore.yaml`

### Wave 8. Soak
Implement:
- repeated cold boot loop
- background workflow churn
- periodic health snapshots

---

## 12. Minimum viable harness for immediate progress

If you want the smallest useful first version, build just this:

### Files
- `tests/harness/bin/globular-test`
- `tests/harness/lib/cluster.sh`
- `tests/harness/lib/probes.sh`
- `tests/harness/lib/assertions.sh`
- `tests/scenarios/smoke/cluster-cold-boot.yaml`
- `tests/scenarios/functional/publish-install-service.yaml`
- `tests/scenarios/security/authz-basic.yaml`
- `tests/scenarios/resilience/service-stop-remediation.yaml`
- `tests/scenarios/recovery/full-reseed.yaml`

### Targets
- `make quickstart-up`
- `make quickstart-reset`
- `make test-smoke`
- `make test-functional`
- `make test-security`
- `make test-resilience`
- `make test-recovery`

### Reports
- service health markdown summary
- workflow trace JSON
- scenario pass/fail markdown summary

That small slice is enough to start proving real value immediately.

---

## 13. Definition of done for the harness

The harness is ready for V1 certification work when:

- one command brings up the Docker cluster from clean state
- one command runs a suite of YAML scenarios
- scenarios can inject failures and wait on truth-based probes
- evidence is automatically collected on pass and fail
- CI can run smoke and functional suites reproducibly
- the harness can generate parity, health, authz, and recovery reports
- recovery scenarios can drive node reseed and backup/restore validation

At that point, the test plan stops being a document and becomes an executable contract.