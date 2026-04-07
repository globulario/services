1. The three concepts
Idempotency

This answers:

“If this step is replayed, is that safe?”

Not all steps are equal.

A health probe is usually harmless to retry.
A package install is often effectively safe, but only if you verify the result before replaying.
A membership change or schema mutation may be dangerous to replay blindly.

So idempotency classifies the step’s side-effect profile.

Suggested values
safe_retry
replay is safe without special checks
examples: read-only probe, classify, aggregate, mark-started event
verify_then_continue
do not replay blindly
first inspect whether the effect already happened
examples: install package, restart service, sync installed state
manual_approval
engine must stop and require approval on uncertain resume
examples: destructive cleanup, irreversible cluster membership mutation
compensatable
step may be retried or rolled back through an explicit compensation action
examples: a reversible config update, staged publish before finalize
Resume policy

This answers:

“When a run is resumed and this step was in progress, what should the engine do?”

This is the run-resume brain.

Suggested values
retry
just run the step again
only for truly harmless steps
verify_effect
inspect the world first
if the effect is already present, mark step completed
otherwise execute it
rerun_if_no_receipt
if there is no durable receipt, verify effect and rerun only if absent
pause_for_approval
stop and require human approval if the engine cannot prove safety
fail
conservative mode for steps that should never auto-resume
Verification

This answers:

“How do we prove the step’s intended effect is already true?”

Verification is the missing flashlight in the cave.

A step should define:

what to check
who checks it
what success means
optionally what receipt or authoritative state proves completion

That lets resume become fact-based instead of mood-based.

2. What these add to the actual picture

Right now your workflows are good at:

sequencing
retries
timeouts
post-step validation
converging to installed/runtime state

What they lack is engine-visible semantics.

Without explicit metadata, the workflow engine knows only:

step started
step maybe timed out
step maybe failed
next step depends on it

It does not know:

whether replay is safe
whether it should inspect the system first
whether it needs approval
what exact proof counts as “already done”

So today, the system has a good skeleton, but the resume brain is still partly hidden in muscle memory.

This extension adds:

safe resume
step-level recovery policy
less duplicate execution
clearer auditing
less guesswork during partial failure
a path toward orphan-run resume for Scylla-backed workflow leasing

That last one matters because your HA work already includes durable run ownership and orphan claim logic, but true recovery needs step semantics after claim, not just lease takeover.

3. What needs to change
In the workflow schema

Add optional step fields like:

execution.idempotency
execution.resume_policy
execution.receipt_key
verification
optional compensation
In the workflow runtime

When resuming a run:

if step is terminal, skip
if step was in progress and the executor died:
inspect resume_policy
possibly run verification
possibly read receipt
decide whether to:
mark completed
retry
pause
fail safely
In handlers / actors

Add or standardize verification actions that can answer:

is package installed at desired version/hash?
is service active and healthy?
is authoritative installed state synced?
is receipt present and valid?
In persistence

Optionally store a step receipt for side-effecting actions.
This is not mandatory for everything, but it helps a lot for ambiguous cases.

A receipt could contain:

step_id
target node
action
started_at
observed result
desired version/hash
verification summary
receipt timestamp
4. Proposed YAML extension

Here is a compact Claude-ready shape.

# Proposed step extension
- id: some_step
  title: Example step
  actor: node-agent
  action: node.some_action
  with: {}

  execution:
    idempotency: verify_then_continue
    resume_policy: verify_effect
    receipt_key: some_step_receipt
    receipt_required: false

  verification:
    actor: node-agent
    action: node.verify_some_effect
    with: {}
    success:
      expr: result.ok == true

  compensation:
    enabled: false
    actor: ""
    action: ""
    with: {}
Meaning
execution.idempotency

What kind of side effect this step has.

execution.resume_policy

What the engine should do if the step was interrupted.

execution.receipt_key

Optional durable key or label for a step receipt.

execution.receipt_required

If true, step should not be considered safely completed without a receipt.

verification

How to prove the effect exists already.

compensation

Optional rollback or cleanup action for recoverable risky steps.

5. Recommended semantics table
Field	Purpose	Typical values
idempotency	classify replay safety	safe_retry, verify_then_continue, manual_approval, compensatable
resume_policy	engine behavior on resume	retry, verify_effect, rerun_if_no_receipt, pause_for_approval, fail
receipt_key	durable breadcrumb for ambiguous steps	any stable string
verification	prove effect exists	actor + action + success expr
compensation	rollback/cleanup if needed	optional actor/action
6. Apply it to node.join

Your current node.join already has the critical Scylla path:

install_scylladb
wait_scylladb_ready
install_scylla_tools
report_installed
mark_converged

Below is a concrete adaptation.

node.join with extension applied to critical steps
apiVersion: workflow.globular.io/v1alpha1
kind: WorkflowDefinition
metadata:
  name: node.join
  displayName: Day-1 node join and converge
spec:
  inputSchema:
    type: object
    required:
      - cluster_id
      - node_id
      - node_hostname
      - node_ip
    properties:
      cluster_id: { type: string }
      node_id: { type: string }
      node_hostname: { type: string }
      node_ip: { type: string }
      repository_address: { type: string, default: "" }
      install_timeout: { type: string, default: 5m }
  defaults:
    install_timeout: 5m
  strategy:
    mode: single

  steps:
    - id: verify_prerequisites
      title: Verify join-script prerequisites are running
      actor: node-agent
      action: node.verify_services_active
      with:
        services: [etcd, node-agent]
      retry:
        maxAttempts: 6
        backoff: 5s
      timeout: 1m
      execution:
        idempotency: safe_retry
        resume_policy: retry
      verification:
        actor: node-agent
        action: node.verify_services_active
        with:
          services: [etcd, node-agent]
        success:
          expr: result.ok == true

    - id: install_mesh
      title: Install storage and mesh infrastructure
      dependsOn: [verify_prerequisites]
      actor: node-agent
      action: node.install_packages
      with:
        packages:
          - { name: minio, kind: INFRASTRUCTURE }
          - { name: sidekick, kind: INFRASTRUCTURE }
          - { name: node-exporter, kind: INFRASTRUCTURE }
          - { name: prometheus, kind: INFRASTRUCTURE }
          - { name: xds, kind: INFRASTRUCTURE }
      timeout: $.install_timeout
      execution:
        idempotency: verify_then_continue
        resume_policy: verify_effect
        receipt_key: install_mesh
      verification:
        actor: node-agent
        action: node.verify_packages_installed
        with:
          node_id: $.node_id
          packages:
            - { name: minio, kind: INFRASTRUCTURE }
            - { name: sidekick, kind: INFRASTRUCTURE }
            - { name: node-exporter, kind: INFRASTRUCTURE }
            - { name: prometheus, kind: INFRASTRUCTURE }
            - { name: xds, kind: INFRASTRUCTURE }
        success:
          expr: result.all_installed == true

    - id: install_envoy
      title: Install Envoy proxy
      dependsOn: [install_mesh]
      actor: node-agent
      action: node.install_packages
      with:
        packages:
          - { name: envoy, kind: INFRASTRUCTURE }
      timeout: $.install_timeout
      execution:
        idempotency: verify_then_continue
        resume_policy: verify_effect
        receipt_key: install_envoy
      verification:
        actor: node-agent
        action: node.verify_packages_installed
        with:
          node_id: $.node_id
          packages:
            - { name: envoy, kind: INFRASTRUCTURE }
        success:
          expr: result.all_installed == true

    - id: install_gateway
      title: Install gateway
      dependsOn: [install_envoy]
      actor: node-agent
      action: node.install_packages
      with:
        packages:
          - { name: gateway, kind: INFRASTRUCTURE }
      timeout: $.install_timeout
      execution:
        idempotency: verify_then_continue
        resume_policy: verify_effect
        receipt_key: install_gateway
      verification:
        actor: node-agent
        action: node.verify_packages_installed
        with:
          node_id: $.node_id
          packages:
            - { name: gateway, kind: INFRASTRUCTURE }
        success:
          expr: result.all_installed == true

    - id: install_scylladb
      title: Install ScyllaDB
      dependsOn: [install_envoy]
      actor: node-agent
      action: node.install_packages
      with:
        packages:
          - { name: scylladb, kind: INFRASTRUCTURE }
      retry:
        maxAttempts: 2
        backoff: 30s
      timeout: 10m
      execution:
        idempotency: verify_then_continue
        resume_policy: verify_effect
        receipt_key: install_scylladb
      verification:
        actor: node-agent
        action: node.verify_packages_installed
        with:
          node_id: $.node_id
          packages:
            - { name: scylladb, kind: INFRASTRUCTURE }
        success:
          expr: result.all_installed == true

    - id: wait_scylladb_ready
      title: Wait for ScyllaDB to accept CQL connections
      dependsOn: [install_scylladb]
      actor: node-agent
      action: node.probe_infra_health
      with:
        probe: probe-scylla-health
      retry:
        maxAttempts: 60
        backoff: 10s
      timeout: 15m
      execution:
        idempotency: safe_retry
        resume_policy: retry
      verification:
        actor: node-agent
        action: node.probe_infra_health
        with:
          probe: probe-scylla-health
        success:
          expr: result.healthy == true

    - id: install_scylla_tools
      title: Install ScyllaDB manager and agent
      dependsOn: [wait_scylladb_ready]
      actor: node-agent
      action: node.install_packages
      with:
        packages:
          - { name: scylla-manager, kind: INFRASTRUCTURE }
          - { name: scylla-manager-agent, kind: INFRASTRUCTURE }
      timeout: $.install_timeout
      execution:
        idempotency: verify_then_continue
        resume_policy: verify_effect
        receipt_key: install_scylla_tools
      verification:
        actor: node-agent
        action: node.verify_packages_installed
        with:
          node_id: $.node_id
          packages:
            - { name: scylla-manager, kind: INFRASTRUCTURE }
            - { name: scylla-manager-agent, kind: INFRASTRUCTURE }
        success:
          expr: result.all_installed == true

    - id: install_foundational
      title: Install foundational services
      dependsOn: [wait_scylladb_ready, install_gateway]
      actor: node-agent
      action: node.install_packages
      with:
        packages:
          - { name: dns, kind: SERVICE }
          - { name: event, kind: SERVICE }
          - { name: rbac, kind: SERVICE }
          - { name: resource, kind: SERVICE }
          - { name: authentication, kind: SERVICE }
          - { name: discovery, kind: SERVICE }
          - { name: repository, kind: SERVICE }
          - { name: monitoring, kind: SERVICE }
      timeout: $.install_timeout
      execution:
        idempotency: verify_then_continue
        resume_policy: verify_effect
        receipt_key: install_foundational
      verification:
        actor: node-agent
        action: node.verify_packages_installed
        with:
          node_id: $.node_id
          packages:
            - { name: dns, kind: SERVICE }
            - { name: event, kind: SERVICE }
            - { name: rbac, kind: SERVICE }
            - { name: resource, kind: SERVICE }
            - { name: authentication, kind: SERVICE }
            - { name: discovery, kind: SERVICE }
            - { name: repository, kind: SERVICE }
            - { name: monitoring, kind: SERVICE }
        success:
          expr: result.all_installed == true

    - id: install_workloads
      title: Install workload services
      dependsOn: [install_foundational]
      actor: node-agent
      action: node.install_packages
      with:
        packages:
          - { name: file, kind: SERVICE }
          - { name: log, kind: SERVICE }
          - { name: search, kind: SERVICE }
          - { name: title, kind: SERVICE }
          - { name: media, kind: SERVICE }
          - { name: torrent, kind: SERVICE }
          - { name: persistence, kind: SERVICE }
          - { name: backup-manager, kind: SERVICE }
          - { name: workflow, kind: SERVICE }
          - { name: cluster-controller, kind: SERVICE }
          - { name: cluster-doctor, kind: SERVICE }
          - { name: mcp, kind: SERVICE }
          - { name: ai-memory, kind: SERVICE }
          - { name: ai-executor, kind: SERVICE }
          - { name: ai-watcher, kind: SERVICE }
          - { name: ai-router, kind: SERVICE }
      timeout: $.install_timeout
      execution:
        idempotency: verify_then_continue
        resume_policy: verify_effect
        receipt_key: install_workloads
      verification:
        actor: node-agent
        action: node.verify_packages_installed
        with:
          node_id: $.node_id
          packages:
            - { name: file, kind: SERVICE }
            - { name: log, kind: SERVICE }
            - { name: search, kind: SERVICE }
            - { name: title, kind: SERVICE }
            - { name: media, kind: SERVICE }
            - { name: torrent, kind: SERVICE }
            - { name: persistence, kind: SERVICE }
            - { name: backup-manager, kind: SERVICE }
            - { name: workflow, kind: SERVICE }
            - { name: cluster-controller, kind: SERVICE }
            - { name: cluster-doctor, kind: SERVICE }
            - { name: mcp, kind: SERVICE }
            - { name: ai-memory, kind: SERVICE }
            - { name: ai-executor, kind: SERVICE }
            - { name: ai-watcher, kind: SERVICE }
            - { name: ai-router, kind: SERVICE }
        success:
          expr: result.all_installed == true

    - id: install_commands
      title: Install CLI tools
      dependsOn: [install_foundational]
      actor: node-agent
      action: node.install_packages
      with:
        packages:
          - { name: globular-cli, kind: COMMAND }
          - { name: etcdctl, kind: COMMAND }
          - { name: mc, kind: COMMAND }
          - { name: sctool, kind: COMMAND }
      timeout: 2m
      execution:
        idempotency: verify_then_continue
        resume_policy: verify_effect
        receipt_key: install_commands
      verification:
        actor: node-agent
        action: node.verify_packages_installed
        with:
          node_id: $.node_id
          packages:
            - { name: globular-cli, kind: COMMAND }
            - { name: etcdctl, kind: COMMAND }
            - { name: mc, kind: COMMAND }
            - { name: sctool, kind: COMMAND }
        success:
          expr: result.all_installed == true

    - id: report_installed
      title: Sync installed state to etcd
      dependsOn: [install_workloads, install_scylla_tools, install_commands]
      actor: node-agent
      action: node.sync_installed_state
      execution:
        idempotency: verify_then_continue
        resume_policy: verify_effect
        receipt_key: report_installed_state
      verification:
        actor: node-agent
        action: node.verify_installed_state_synced
        with:
          node_id: $.node_id
        success:
          expr: result.synced == true

    - id: mark_converged
      title: Mark node as converged
      dependsOn: [report_installed]
      actor: cluster-controller
      action: controller.bootstrap.set_phase
      with:
        phase: workload_ready
      execution:
        idempotency: safe_retry
        resume_policy: verify_effect
      verification:
        actor: cluster-controller
        action: controller.bootstrap.verify_phase
        with:
          node_id: $.node_id
          phase: workload_ready
        success:
          expr: result.phase_matches == true
7. Why these choices for node.join
install_scylladb = verify_then_continue

Because if the executor dies after package installation but before the step is marked complete, blindly replaying could be noisy or risky. Better to inspect first.

wait_scylladb_ready = safe_retry

Because it is a read-only readiness check. Re-running it is harmless.

report_installed = verify_then_continue

Because state sync may already have happened before crash. The engine should check if authoritative state already matches before replaying.

That’s the Scylla incident in miniature: not “did the function return?” but “what does the world look like now?”

8. Apply it to release.apply.infrastructure

Your current workflow already has a very solid ladder:

mark resolved
select targets
mark applying
install package
verify installed marker and checksum
maybe restart
verify runtime health
sync installed state
mark node succeeded
aggregate
finalize release

Here is the extension applied.

release.apply.infrastructure with extension
apiVersion: workflow.globular.io/v1alpha1
kind: WorkflowDefinition
metadata:
  name: release.apply.infrastructure
  displayName: Apply infrastructure release
spec:
  inputSchema:
    type: object
    required:
      - cluster_id
      - release_id
      - release_name
      - package_name
      - resolved_version
      - desired_hash
      - candidate_nodes
    properties:
      cluster_id: { type: string }
      release_id: { type: string }
      release_name: { type: string }
      package_name: { type: string }
      resolved_version: { type: string }
      desired_hash: { type: string }
      release_kind: { type: string, default: infrastructure }
      candidate_nodes:
        type: array
        items: { type: string }
      max_parallel_nodes: { type: integer, default: 1 }
      execute_timeout: { type: string, default: 30m }
      verify_timeout: { type: string, default: 10m }
      restart_required: { type: boolean, default: true }
      health_check: { type: string, default: service_healthy }
  defaults:
    max_parallel_nodes: 1
    execute_timeout: 30m
    verify_timeout: 10m
    restart_required: true
    health_check: service_healthy
  strategy:
    mode: single

  steps:
    - id: mark_resolved
      title: Mark release resolved
      actor: cluster-controller
      action: controller.release.mark_resolved
      execution:
        idempotency: safe_retry
        resume_policy: verify_effect
      verification:
        actor: cluster-controller
        action: controller.release.verify_status
        with:
          release_id: $.release_id
          expected_status: RESOLVED
        success:
          expr: result.status_matches == true

    - id: select_targets
      title: Select eligible infrastructure targets
      dependsOn: [mark_resolved]
      actor: cluster-controller
      action: controller.release.select_infrastructure_targets
      with:
        candidate_nodes: $.candidate_nodes
        package_name: $.package_name
        desired_hash: $.desired_hash
      execution:
        idempotency: safe_retry
        resume_policy: retry

    - id: short_circuit_if_no_targets
      title: Finalize immediately when all targets are already converged
      dependsOn: [select_targets]
      when:
        expr: len(selected_targets) == 0
      actor: cluster-controller
      action: controller.release.finalize_noop
      with:
        status: AVAILABLE
      execution:
        idempotency: safe_retry
        resume_policy: verify_effect
      verification:
        actor: cluster-controller
        action: controller.release.verify_status
        with:
          release_id: $.release_id
          expected_status: AVAILABLE
        success:
          expr: result.status_matches == true

    - id: mark_applying
      title: Mark release applying
      dependsOn: [select_targets]
      when:
        expr: len(selected_targets) > 0
      actor: cluster-controller
      action: controller.release.mark_applying
      execution:
        idempotency: safe_retry
        resume_policy: verify_effect
      verification:
        actor: cluster-controller
        action: controller.release.verify_status
        with:
          release_id: $.release_id
          expected_status: APPLYING
        success:
          expr: result.status_matches == true

    - id: apply_per_node
      title: Apply package per target node
      dependsOn: [mark_applying]
      when:
        expr: len(selected_targets) > 0
      foreach: $.selected_targets
      itemName: target
      strategy:
        mode: parallel
        concurrency: $.max_parallel_nodes
      steps:
        - id: mark_node_started
          title: Mark node apply started
          actor: cluster-controller
          action: controller.release.mark_node_started
          with:
            node_id: $.target.node_id
          execution:
            idempotency: safe_retry
            resume_policy: verify_effect
          verification:
            actor: cluster-controller
            action: controller.release.verify_node_status
            with:
              release_id: $.release_id
              node_id: $.target.node_id
              expected_status: APPLYING
            success:
              expr: result.status_matches == true

        - id: install_package
          title: Install infrastructure package on node
          dependsOn: [mark_node_started]
          actor: node-agent
          action: node.install_package
          with:
            node_id: $.target.node_id
            package_name: $.package_name
            version: $.resolved_version
            desired_hash: $.desired_hash
            kind: INFRASTRUCTURE
          timeout: $.execute_timeout
          export: install_result
          execution:
            idempotency: verify_then_continue
            resume_policy: verify_effect
            receipt_key: install_package
          verification:
            actor: node-agent
            action: node.verify_package_installed
            with:
              node_id: $.target.node_id
              package_name: $.package_name
              version: $.resolved_version
              desired_hash: $.desired_hash
            success:
              expr: result.installed == true

        - id: verify_installed
          title: Verify installed marker and package checksum
          dependsOn: [install_package]
          actor: node-agent
          action: node.verify_package_installed
          with:
            node_id: $.target.node_id
            package_name: $.package_name
            version: $.resolved_version
            desired_hash: $.desired_hash
          retry:
            maxAttempts: 20
            backoff: 5s
          timeout: $.verify_timeout
          export: verify_result
          execution:
            idempotency: safe_retry
            resume_policy: retry
          verification:
            actor: node-agent
            action: node.verify_package_installed
            with:
              node_id: $.target.node_id
              package_name: $.package_name
              version: $.resolved_version
              desired_hash: $.desired_hash
            success:
              expr: result.installed == true

        - id: maybe_restart
          title: Restart package service when required
          dependsOn: [verify_installed]
          when:
            expr: inputs.restart_required == true
          actor: node-agent
          action: node.restart_package_service
          with:
            node_id: $.target.node_id
            package_name: $.package_name
          timeout: 5m
          execution:
            idempotency: verify_then_continue
            resume_policy: verify_effect
            receipt_key: restart_package_service
          verification:
            actor: node-agent
            action: node.verify_package_runtime
            with:
              node_id: $.target.node_id
              package_name: $.package_name
              health_check: $.health_check
            success:
              expr: result.healthy == true

        - id: verify_runtime_health
          title: Verify runtime health after install
          dependsOn: [verify_installed, maybe_restart]
          actor: node-agent
          action: node.verify_package_runtime
          with:
            node_id: $.target.node_id
            package_name: $.package_name
            health_check: $.health_check
          retry:
            maxAttempts: 60
            backoff: 5s
          timeout: $.verify_timeout
          export: health_result
          execution:
            idempotency: safe_retry
            resume_policy: retry
          verification:
            actor: node-agent
            action: node.verify_package_runtime
            with:
              node_id: $.target.node_id
              package_name: $.package_name
              health_check: $.health_check
            success:
              expr: result.healthy == true

        - id: sync_installed_state
          title: Sync installed state to authoritative store
          dependsOn: [verify_runtime_health]
          actor: node-agent
          action: node.sync_installed_package_state
          with:
            node_id: $.target.node_id
            package_name: $.package_name
            version: $.resolved_version
            desired_hash: $.desired_hash
          export: sync_result
          execution:
            idempotency: verify_then_continue
            resume_policy: verify_effect
            receipt_key: sync_installed_package_state
          verification:
            actor: node-agent
            action: node.verify_installed_package_state
            with:
              node_id: $.target.node_id
              package_name: $.package_name
              version: $.resolved_version
              desired_hash: $.desired_hash
            success:
              expr: result.synced == true

        - id: mark_node_succeeded
          title: Mark node apply succeeded
          dependsOn: [sync_installed_state]
          actor: cluster-controller
          action: controller.release.mark_node_succeeded
          with:
            node_id: $.target.node_id
            version: $.resolved_version
            desired_hash: $.desired_hash
          execution:
            idempotency: safe_retry
            resume_policy: verify_effect
          verification:
            actor: cluster-controller
            action: controller.release.verify_node_status
            with:
              release_id: $.release_id
              node_id: $.target.node_id
              expected_status: SUCCEEDED
            success:
              expr: result.status_matches == true

      onFailure:
        actor: cluster-controller
        action: controller.release.mark_node_failed
        with:
          node_id: $.target.node_id
          package_name: $.package_name

    - id: aggregate_outcome
      title: Aggregate per-node outcomes
      dependsOn: [apply_per_node]
      when:
        expr: len(selected_targets) > 0
      actor: cluster-controller
      action: controller.release.aggregate_direct_apply_results
      with:
        release_id: $.release_id
        package_name: $.package_name
      export: aggregate
      execution:
        idempotency: safe_retry
        resume_policy: retry

    - id: finalize_release
      title: Finalize release status
      dependsOn: [aggregate_outcome]
      when:
        expr: len(selected_targets) > 0
      actor: cluster-controller
      action: controller.release.finalize_direct_apply
      with:
        aggregate: $.aggregate
      execution:
        idempotency: safe_retry
        resume_policy: verify_effect
      verification:
        actor: cluster-controller
        action: controller.release.verify_terminal_status
        with:
          release_id: $.release_id
        success:
          expr: result.terminal == true
9. Why these choices for infrastructure rollout
install_package = verify_then_continue

This is the big one. If an executor dies after the package was written but before the workflow updated its own state, the resumed run should inspect the node first.

maybe_restart = verify_then_continue

Restarting twice is sometimes harmless, sometimes noisy. Better to treat restart as “check health first, then decide.”

sync_installed_state = verify_then_continue

Because the authoritative state may already match. Re-running is often okay, but there is no reason to replay if the proof already exists.

verify_* steps = safe_retry

These are probes and checks. Re-running them is cheap and honest.

10. What the runtime should actually do with this

Here is the engine behavior in plain language.

On normal execution

Nothing dramatic changes. The workflow still runs as today.

On resume after crash / orphan claim

For the interrupted step:

If resume_policy: retry

Run it again.

If resume_policy: verify_effect

Run the verification action first.

if verification says effect exists:
mark step completed from observation
if verification says effect absent:
execute the step
if verification is inconclusive:
follow idempotency class
maybe pause or fail safely
If resume_policy: pause_for_approval

Create a finding or blocked state and stop there.

That turns resume from a blindfolded stumble into a checkpoint gate.

11. What code likely needs to be added

At minimum:

Workflow schema / parser

Support:

execution
verification
optional compensation
Workflow executor

Add resume logic:

inspect last in-progress step
read its metadata
run verification action if required
short-circuit to complete when proof exists
Node-agent / controller verification actions

You will need a few helpers if they do not already exist:

node.verify_packages_installed
node.verify_installed_state_synced
node.verify_installed_package_state
controller.release.verify_status
controller.release.verify_node_status
controller.release.verify_terminal_status
controller.bootstrap.verify_phase

Most of these are not exotic. They are the cluster equivalent of asking, “Did the bolt actually tighten, or did the wrench just make a dramatic noise?” 🔩

Optional but very valuable

Step receipts in workflow persistence or authoritative stores.

12. What this changes in the actual picture

Before:

workflows define order
handlers define behavior
resume is partly implicit
partial failure can leave ambiguity

After:

workflows define order and recovery semantics
engine understands replay safety
verification becomes a first-class recovery tool
orphan-run resume becomes credible
Scylla/MinIO/network turbulence causes less double-execution and less fear

That is the real upgrade.

Not prettier YAML.

A smarter control plane.

13. Clean summary you can hand to Claude

You can give Claude this:

Add first-class step execution metadata to the workflow schema:

execution:
  idempotency: safe_retry | verify_then_continue | manual_approval | compensatable
  resume_policy: retry | verify_effect | rerun_if_no_receipt | pause_for_approval | fail
  receipt_key: <string>
  receipt_required: <bool>

verification:
  actor: <actor>
  action: <action>
  with: <object>
  success:
    expr: <expression over result>

compensation:
  enabled: <bool>
  actor: <actor>
  action: <action>
  with: <object>

Runtime behavior:
- On normal execution, run as today.
- On resume of an in-progress step:
  - retry if resume_policy=retry
  - run verification first if resume_policy=verify_effect
  - if verification proves side effect already exists, mark step completed without replay
  - if verification fails to prove safety and idempotency is manual_approval, pause for approval
  - record receipts when configured

Apply this extension first to node.join and release.apply.infrastructure, especially:
- install_scylladb
- wait_scylladb_ready
- install_scylla_tools
- report_installed
- install_package
- maybe_restart
- sync_installed_state

Goal:
Make workflow resume semantics explicit and safe under executor crash, storage partition, or partial write conditions, especially for Scylla-backed HA execution.