# Globular Awareness Learning

## Purpose

**Awareness Learning** is the next evolution of the Globular Awareness Graph.

The awareness graph already gives AI agents a compressed map of Globular by connecting:

```text
source code → packages → services → state → invariants → runtime events → failures → remediations → memory
```

Awareness Learning closes the loop in the opposite direction:

```text
runtime incident → failure analysis → proposed invariant/failure-mode YAML → review → graph rebuild → safer future AI context
```

The goal is simple:

```text
When Globular suffers from a real failure, the system should be able to turn that failure into structured operational knowledge.
```

A bug should not only be fixed.  
A bug should become a scar.  
A scar should become a rule.  
A rule should protect future AI and future services.

---

## Core Idea

Globular should be able to learn from its own runtime failures.

The learning process should not be automatic mutation of architecture truth. Instead, it should be a controlled pipeline:

```text
incident happens
  ↓
ai-watcher / doctor / workflow receipts collect facts
  ↓
AI reasons over the incident
  ↓
AI proposes new awareness YAML
  ↓
Globular validates the proposal
  ↓
human or approved workflow reviews it
  ↓
proposal is promoted into docs/awareness
  ↓
awareness graph rebuilds
  ↓
future AI agents receive the new scar in context
```

The important safety law:

```text
AI may propose awareness.
AI must not silently promote awareness to law.
```

In Globular terms:

```text
Observation is not law.
AI proposal is not law.
Validated proposal is not law.
Approved and promoted YAML is law.
```

This keeps the system boring, auditable, and safe.

---

## Why This Matters

Many Globular bugs are not local bugs. They are failure chains.

Example:

```text
catalog tab fails
→ gRPC-Web calls fail with status 0
→ Envoy start-limit-hit
→ repeated SIGTERM storm
→ install workflow loop
→ desired_hash mismatch
→ drift proof can never converge
```

A local AI agent may see the symptom and suggest:

```text
Restart Envoy.
```

But the real lesson is:

```text
When convergence proof uses two different identities, restart is not a fix.
The proof condition must be corrected.
```

That lesson should become structured awareness metadata.

Future AI agents should see:

```text
This looks like a desired_hash convergence loop.
Do not restart Envoy repeatedly.
Check canonical DesiredHash usage.
Check install result stamping.
Check xDS installed-state.
Run the required tests.
```

That is the value of Awareness Learning.

---

## The Complete Learning Loop

Current Globular loop:

```text
Source
→ Package
→ Desired
→ Installed
→ Runtime
```

Existing awareness loop:

```text
Source
→ Package
→ Desired
→ Installed
→ Runtime
→ Incident
→ Memory
→ Source Context
```

Awareness Learning loop:

```text
Source
→ Package
→ Desired
→ Installed
→ Runtime
→ Incident
→ Evidence Bundle
→ AI Awareness Proposal
→ Validation
→ Promotion
→ Graph Rebuild
→ Future Agent Context
```

The result:

```text
Globular does not only recover from failure.
Globular learns how not to repeat the same class of failure.
```

---

## New Subsystem Name

Recommended name:

```text
awareness-learning
```

Alternative names:

```text
scar-learning
incident-learning
awareness-proposals
operational-memory-compiler
```

Preferred name:

```text
awareness-learning
```

Reason:

It clearly describes the function: the awareness graph grows from runtime learning, but through controlled proposals and promotion.

---

## Responsibilities

Awareness Learning is responsible for:

```text
1. Building structured incident bundles from runtime evidence.
2. Letting AI propose new awareness YAML from incident evidence.
3. Validating proposed failure modes, invariants, forbidden fixes, aliases, and tests.
4. Preventing AI from silently mutating approved architecture laws.
5. Promoting approved proposals into docs/awareness.
6. Rebuilding or marking the graph dirty after promotion.
7. Ensuring future agent-context output includes new scars.
```

Awareness Learning is **not** responsible for:

```text
1. Executing remediation directly.
2. Editing production code directly.
3. Silently creating new critical invariants.
4. Deleting approved invariants.
5. Weakening existing critical laws.
6. Replacing workflow-service, doctor, ai-watcher, or ai-memory.
```

The graph informs.  
The AI proposes.  
The validator checks.  
The reviewer approves.  
The workflow-service acts.

---

## High-Level Architecture

```text
+-------------------------------------------------------------+
|                     Globular Runtime                        |
+-------------------------------------------------------------+
| doctor findings | events | workflow receipts | state deltas |
+-------------------------------------------------------------+
                              |
                              v
+-------------------------------------------------------------+
|                    Incident Bundle Builder                  |
+-------------------------------------------------------------+
| symptoms | services | state deltas | receipts | repair notes |
+-------------------------------------------------------------+
                              |
                              v
+-------------------------------------------------------------+
|                 AI Awareness Proposal Generator             |
+-------------------------------------------------------------+
| proposes failure_modes, invariants, fixes, tests, aliases    |
+-------------------------------------------------------------+
                              |
                              v
+-------------------------------------------------------------+
|                  Proposal Validator                         |
+-------------------------------------------------------------+
| schema | references | cycles | invariant weakening checks    |
+-------------------------------------------------------------+
                              |
                              v
+-------------------------------------------------------------+
|                  Review / Promotion                         |
+-------------------------------------------------------------+
| approved YAML written into docs/awareness + graph rebuild    |
+-------------------------------------------------------------+
                              |
                              v
+-------------------------------------------------------------+
|                  Future Agent Context                       |
+-------------------------------------------------------------+
| Claude/Codex now sees the new scar before editing code       |
+-------------------------------------------------------------+
```

---

## New Files

Add these files and directories:

```text
docs/awareness/
  context_aliases.yaml
  learning_rules.yaml
  proposals/
```

### `context_aliases.yaml`

Purpose:

Map natural task language to invariants, failure modes, and forbidden fixes.

Example:

```yaml
aliases:
  infra.desired_hash_consistency:
    - desired hash mismatch
    - checksum mismatch
    - drift loop
    - install loop
    - raw artifact digest
    - ComputeInfrastructureDesiredHash
    - ResolvedArtifactDigest

  service.restart_singleflight:
    - restart storm
    - SIGTERM storm
    - start-limit-hit
    - repeated restart
    - restart loop

  infra.heartbeat_not_desired_authority:
    - heartbeat created release
    - ManagedInstalled created InfrastructureRelease
    - runtime promoted to desired
```

Why this matters:

AI tasks are usually written in human language. Aliases help the graph match:

```text
"catalog tab status 0 after Envoy restart storm"
```

against:

```text
service.restart_singleflight
infra.desired_hash_consistency
infra.heartbeat_not_desired_authority
```

---

### `learning_rules.yaml`

Purpose:

Define what AI-generated awareness is allowed to do.

Example:

```yaml
rules:
  - id: learning.must_be_reviewable
    summary: AI-generated awareness must be written as proposal YAML, not directly promoted.

  - id: learning.no_silent_invariant_creation
    summary: New critical invariants require explicit approval before becoming active.

  - id: learning.no_weakening_existing_laws
    summary: AI proposals must not delete or weaken existing critical invariants.

  - id: learning.failure_modes_need_evidence
    summary: Failure modes must reference symptoms, services, state deltas, and root cause evidence.

  - id: learning.required_tests_for_critical_invariant
    summary: Critical invariants must include at least one required test or explicit TODO test.
```

---

### `docs/awareness/proposals/`

Purpose:

Store draft awareness proposals generated from incidents.

Example:

```text
docs/awareness/proposals/2026-05-06-infra-desired-hash-mismatch.yaml
```

A proposal is not active architecture truth until promoted.

---

## New Graph Node Types

Add these node types:

```text
incident
incident_bundle
awareness_proposal
proposal_patch
context_alias
learning_rule
evidence
manual_repair
```

---

## New Graph Edge Types

Add these edge types:

```text
observed_during
proposes
derived_from
supported_by
promoted_to
supersedes
aliases
needs_review
approved_by
rejected_by
```

Example graph shape:

```text
incident:envoy_restart_storm
  observed_during → service:envoy
  supported_by → workflow_receipt:install_loop
  supported_by → state_delta:desired_hash_mismatch
  proposes → failure_mode:infra.desired_hash_mismatch_restart_storm
  proposes → invariant:infra.desired_hash_consistency
  proposes → forbidden_fix:restart_envoy_on_every_drift_tick
```

---

## New SQLite Tables

```sql
CREATE TABLE incidents (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    severity TEXT,
    status TEXT,
    started_at INTEGER,
    ended_at INTEGER,
    summary TEXT,
    evidence_json TEXT,
    created_at INTEGER,
    updated_at INTEGER
);

CREATE TABLE awareness_proposals (
    id TEXT PRIMARY KEY,
    incident_id TEXT,
    status TEXT NOT NULL,
    proposal_yaml TEXT NOT NULL,
    validation_json TEXT,
    created_by TEXT,
    created_at INTEGER,
    promoted_at INTEGER
);

CREATE TABLE context_aliases (
    id TEXT PRIMARY KEY,
    target_id TEXT NOT NULL,
    alias TEXT NOT NULL,
    confidence REAL DEFAULT 1.0,
    source TEXT,
    created_at INTEGER
);
```

Proposal statuses:

```text
DRAFT
VALIDATED
NEEDS_REVIEW
APPROVED
REJECTED
PROMOTED
SUPERSEDED
```

---

## Incident Bundle Builder

The incident bundle builder collects evidence from the running system.

Inputs:

```text
doctor findings
runtime events
workflow receipts
service status
installed-state
desired-state
package/release metadata
recent code changes
ai-memory entries
manual repair notes
```

Output:

```yaml
incident_id: infra.desired_hash_mismatch_restart_storm.2026-05-06
status: draft
severity: critical

time_range:
  from: "2026-05-06T10:00:00Z"
  to: "2026-05-06T11:00:00Z"

symptoms:
  - gRPC-Web calls failed with status 0
  - Envoy received repeated SIGTERM
  - systemd reported start-limit-hit
  - install workflows repeatedly dispatched

observed_services:
  - envoy
  - xds
  - cluster-controller
  - workflow-service
  - node-agent

state_deltas:
  - InfrastructureRelease DesiredHash did not match installed-state checksum
  - xds installed-state record was missing

workflow_receipts:
  - install workflow dispatched repeatedly
  - repair-via-start path called Restart

manual_repairs:
  - stamped correct desired_hash checksums into infra packages
  - wrote xds version marker file
  - wrote xds installed-state record

suspected_root_cause:
  - lookupServiceReleaseBuildID returned ResolvedArtifactDigest instead of DesiredHash
```

Important:

The incident bundle is not final truth. It is evidence.

---

## Awareness Proposal Generator

The proposal generator uses AI to convert an incident bundle into reviewable awareness YAML.

It may propose:

```text
new failure_mode
new invariant
new forbidden_fix
new required_test
new service dependency edge
new context alias
new safe escape hatch
updates to existing awareness files
```

It must not directly modify approved awareness files.

It writes proposals to:

```text
docs/awareness/proposals/
```

---

## Proposal Validation

Before approval, every proposed YAML patch must be validated.

Validation checks:

```text
1. Proposal YAML is schema-valid.
2. Failure modes include symptoms, root_cause, architecture_fix, related_services, related_invariants, and required_tests.
3. Critical invariants include forbidden_fixes and required_tests.
4. Referenced services exist or are declared in the proposal.
5. Referenced invariants exist or are declared in the proposal.
6. Proposal does not remove existing invariants.
7. Proposal does not weaken severity of existing invariants.
8. Proposal does not remove forbidden fixes from existing critical invariants.
9. Proposed dependency edges do not create dangerous required cycles.
10. Proposal preserves evidence links to the source incident.
```

Command:

```bash
globular awareness validate-proposal --file docs/awareness/proposals/<proposal>.yaml
```

---

## Proposal Promotion

Promotion turns reviewed proposal YAML into approved awareness truth.

Command:

```bash
globular awareness promote-proposal --file docs/awareness/proposals/<proposal>.yaml
```

Promotion writes into approved files:

```text
docs/awareness/invariants.yaml
docs/awareness/failure_modes.yaml
docs/awareness/forbidden_fixes.yaml
docs/awareness/convergence_rules.yaml
docs/awareness/context_aliases.yaml
```

Promotion must create a normal git diff.

No hidden mutation.

After promotion:

```text
1. proposal status becomes PROMOTED
2. graph rebuild is required or triggered
3. future agent-context includes the new scar
```

---

## Example AI-Generated Proposal

```yaml
proposal:
  id: proposal.infra_desired_hash_mismatch_restart_storm
  source_incident: infra.desired_hash_mismatch_restart_storm.2026-05-06
  status: draft

failure_modes:
  - id: infra.desired_hash_mismatch_restart_storm
    title: Infrastructure desired hash mismatch causes install loop and Envoy restart storm
    severity: critical

    symptoms:
      - gRPC-Web calls fail with status 0
      - Envoy receives repeated SIGTERM
      - systemd reports start-limit-hit
      - InfrastructureRelease install workflows dispatch repeatedly
      - installed-state checksum does not match InfrastructureRelease DesiredHash

    root_cause: >
      Drift reconciler workflow used raw artifact digest as desired_hash while
      InfrastructureRelease convergence checks used ComputeInfrastructureDesiredHash.
      The mismatch made convergence impossible and caused repeated install/restart workflows.

    architecture_fix: >
      Use the canonical InfrastructureRelease DesiredHash everywhere: workflow desired_hash,
      LocalHash, installed-state checksum, and convergence commit paths.
      Do not use ResolvedArtifactDigest as desired_hash for infrastructure drift reconciliation.

    related_invariants:
      - convergence.no_infinite_retry
      - infra.desired_hash_consistency
      - service.restart_singleflight
      - infra.heartbeat_not_desired_authority
      - runtime.installed_state_not_liveness

    related_services:
      - envoy
      - xds
      - cluster-controller
      - workflow-service
      - node-agent

    forbidden_fixes:
      - restart_envoy_on_every_drift_tick
      - use_raw_artifact_digest_as_desired_hash
      - create_infra_release_from_heartbeat_only
      - stamp_installed_state_without_canonical_desired_hash
      - clear_loop_by_manual_etcd_patch_without_fixing_hash_source

    required_tests:
      - TestInfrastructureDesiredHashConsistency
      - TestDriftWorkflowUsesDesiredHash
      - TestRestartSingleflightPerService
      - TestHeartbeatDoesNotCreateInfrastructureRelease
      - TestMissingXDSInstalledStateBlocksEnvoyWithClearDiagnosis

invariants:
  - id: infra.desired_hash_consistency
    title: Infrastructure desired hash identity must be consistent
    severity: critical
    summary: >
      InfrastructureRelease DesiredHash, workflow desired_hash, installed-state checksum,
      and convergence committer package checksum must all use the same canonical
      ComputeInfrastructureDesiredHash value. Raw artifact digest must not be used
      as desired_hash for drift reconciliation.

    protects:
      state:
        - /globular/resources/InfrastructureRelease
        - /globular/installed-state
      symbols:
        - lookupServiceReleaseBuildID
        - ComputeInfrastructureDesiredHash
        - CommitConvergenceResult
        - ManagedInstalled
      services:
        - cluster-controller
        - workflow-service
        - node-agent

    forbidden_fixes:
      - use_raw_artifact_digest_as_desired_hash
      - stamp_installed_state_from_noncanonical_hash
      - restart_service_to_resolve_hash_mismatch

    required_tests:
      - TestInfrastructureDesiredHashConsistency
      - TestDriftWorkflowUsesDesiredHashNotArtifactDigest

  - id: service.restart_singleflight
    title: Service restart actions must be singleflight and bounded
    severity: critical
    summary: >
      Multiple convergence paths must not issue concurrent Restart calls for the same service.
      Restart is an effectful action and must be deduplicated, rate-limited, and tied to a verified state transition.

    forbidden_fixes:
      - restart_on_every_drift_tick
      - parallel_restart_same_service
      - restart_without_state_change
      - restart_to_fix_hash_mismatch

    required_tests:
      - TestRestartSingleflightPerService
      - TestDriftLoopDoesNotRestartRepeatedly
      - TestSystemdStartLimitNotHitByConvergence

  - id: infra.heartbeat_not_desired_authority
    title: Heartbeat discovery must not create desired infrastructure authority
    severity: critical
    summary: >
      Runtime heartbeat or ManagedInstalled observation may report discovered infrastructure,
      but must not alone create authoritative InfrastructureRelease desired state.
      Desired infrastructure must come from package, release, BOM, or controller authority.

    forbidden_fixes:
      - create_infra_release_from_heartbeat_only
      - promote_discovered_runtime_to_desired_without_source
      - treat_managed_installed_as_release_authority

    required_tests:
      - TestHeartbeatDoesNotCreateInfrastructureRelease
      - TestFallbackDiscoveredDoesNotTriggerInstallWorkflow

context_aliases:
  infra.desired_hash_consistency:
    - desired hash mismatch
    - checksum mismatch
    - drift loop
    - install loop
    - raw artifact digest
    - ComputeInfrastructureDesiredHash
    - ResolvedArtifactDigest

  service.restart_singleflight:
    - restart storm
    - SIGTERM storm
    - start-limit-hit
    - repeated restart
```

---

## AI Role in Awareness Learning

AI is allowed to:

```text
analyze incident bundles
suggest root cause chains
propose failure modes
propose invariants
propose forbidden fixes
propose required tests
propose context aliases
propose graph edges
```

AI is not allowed to:

```text
silently promote proposals
delete approved invariants
weaken critical invariants
execute remediation directly
mark an incident solved without verification
invent evidence not present in the incident bundle
```

AI-generated awareness must always be traceable to evidence.

---

## Awareness Learning Safety Model

The safety model is:

```text
Observation is evidence.
AI output is a proposal.
Validation is a gate.
Promotion is a reviewed state transition.
Approved YAML is law.
```

This preserves Globular’s operational philosophy:

```text
No hidden magic.
No silent mutation.
No unreviewed law.
No action without receipts.
```

---

## Integration With ai-memory

`ai-memory` stores operational scar tissue as narrative and historical memory.

`awareness-graph` stores structured operational knowledge.

Awareness Learning connects them:

```text
memory_entry
→ recalls → failure_mode
→ supported_by → incident
→ forbids → forbidden_fix
→ tested_by → test
```

`ai-memory` may contain rich human language.

Awareness YAML must contain compact operational truth.

---

## Integration With Doctor

Doctor findings should be able to trigger awareness proposals when:

```text
the same finding repeats across releases
a remediation fails verification
a deterministic loop is detected
a manual repair is required
an unknown failure mode is observed
```

Doctor does not create invariants directly.

Doctor creates evidence.

AI proposes awareness.

Humans or approved workflows promote it.

---

## Integration With workflow-service

`workflow-service` remains the action engine.

Awareness Learning may propose new remediation workflows, but cannot execute them.

If a new failure mode requires a remediation workflow, the proposal should include:

```yaml
missing_remediation_workflows:
  - remediate.infra_desired_hash_mismatch
```

The workflow must be implemented separately and linked after tests exist.

---

## Integration With Agent Context

After a proposal is promoted and the graph is rebuilt, agent-context should include the new scar.

Example command:

```bash
globular awareness agent-context --task "fix catalog tabs status 0 after envoy restart storm"
```

Expected future output:

```text
Relevant invariants:
- infra.desired_hash_consistency
- service.restart_singleflight
- infra.heartbeat_not_desired_authority
- convergence.no_infinite_retry

Known failure modes:
- infra.desired_hash_mismatch_restart_storm

Forbidden fixes:
- restart_envoy_on_every_drift_tick
- use_raw_artifact_digest_as_desired_hash
- create_infra_release_from_heartbeat_only

Required tests:
- TestInfrastructureDesiredHashConsistency
- TestRestartSingleflightPerService
- TestHeartbeatDoesNotCreateInfrastructureRelease
```

That is how runtime failure becomes safer future code editing.

---

## New CLI Commands

```bash
globular awareness incident-bundle --incident <id>
globular awareness propose-from-incident --incident <id>
globular awareness validate-proposal --file <proposal.yaml>
globular awareness promote-proposal --file <proposal.yaml>
globular awareness list-proposals
globular awareness proposal-context --file <proposal.yaml>
globular awareness aliases --task "<task>"
```

Most important first commands:

```bash
globular awareness propose-from-incident --incident <id>
globular awareness validate-proposal --file <proposal.yaml>
globular awareness promote-proposal --file <proposal.yaml>
```

---

## Implementation Structure

Suggested location:

```text
golang/awareness/learning/
  incident_bundle.go
  proposal.go
  validate.go
  promote.go
  aliases.go
  learning_rules.go
```

Suggested tests:

```text
golang/awareness/learning/
  incident_bundle_test.go
  proposal_test.go
  validate_test.go
  promote_test.go
  aliases_test.go
```

---

## Validation Rules

The validator must reject proposals that:

```text
1. Are not valid YAML.
2. Are missing root cause for a failure mode.
3. Add a critical invariant without forbidden fixes.
4. Add a critical invariant without required tests or explicit TODO tests.
5. Reference missing services without declaring them.
6. Reference missing invariants without declaring them.
7. Delete an existing invariant.
8. Lower the severity of an existing invariant.
9. Remove forbidden fixes from existing critical invariants.
10. Add dangerous required dependency cycles.
11. Lack evidence links to an incident bundle.
```

---

## Promotion Rules

Promotion must:

```text
1. Require a validated proposal.
2. Write normal YAML diffs into docs/awareness.
3. Preserve existing approved awareness unless explicitly extended.
4. Mark the proposal as PROMOTED.
5. Mark the graph dirty or trigger rebuild.
6. Never occur silently from AI output alone.
```

---

## Definition of Done

Awareness Learning is complete when:

```text
1. Runtime incidents can be bundled into structured evidence.
2. AI can generate awareness proposal YAML from the bundle.
3. Proposals are validated before promotion.
4. Promotion creates normal diffs in docs/awareness.
5. The graph rebuild includes the new scar.
6. agent-context includes the new failure mode on future related tasks.
7. Critical invariant proposals require explicit approval.
8. No AI-generated proposal can silently weaken approved awareness.
```

---

## Claude Implementation Instruction

```text
Implement Awareness Graph Step 3: awareness-learning.

Goal:
Globular must be able to learn from runtime incidents by generating reviewable awareness YAML proposals.

Do not allow AI-generated proposals to directly mutate approved awareness files.
Do not execute remediation.
Do not weaken or delete existing invariants.
This feature creates evidence bundles, proposal YAML, validators, and promotion commands.

Add:

docs/awareness/proposals/
docs/awareness/context_aliases.yaml
docs/awareness/learning_rules.yaml

Add graph node types:
incident
incident_bundle
awareness_proposal
proposal_patch
context_alias
learning_rule
evidence
manual_repair

Add edge types:
observed_during
proposes
derived_from
supported_by
promoted_to
supersedes
aliases
needs_review
approved_by
rejected_by

Add SQLite tables:
incidents
awareness_proposals
context_aliases

Add package:

golang/awareness/learning/
  incident_bundle.go
  proposal.go
  validate.go
  promote.go
  aliases.go

Add CLI commands:

globular awareness incident-bundle --incident <id>
globular awareness propose-from-incident --incident <id>
globular awareness validate-proposal --file <proposal.yaml>
globular awareness promote-proposal --file <proposal.yaml>
globular awareness list-proposals
globular awareness aliases --task "<task>"

Implement proposal schema supporting:

failure_modes
invariants
forbidden_fixes
required_tests
context_aliases
service_dependencies
manual_repairs
evidence

Validation rules:

1. Proposal YAML must be schema-valid.
2. Failure modes require symptoms, root_cause, architecture_fix, related_services, related_invariants, and required_tests.
3. Critical invariants require forbidden_fixes and required_tests.
4. Referenced services must exist or be declared in the proposal.
5. Referenced invariants must exist or be declared in the proposal.
6. Proposal must not remove existing invariants.
7. Proposal must not weaken severity of an existing invariant.
8. Proposal must not remove forbidden fixes from existing critical invariants.
9. Dependency edges proposed by the YAML must not create dangerous required cycles.
10. Proposal must preserve evidence links to the source incident.

Promotion rules:

1. Promotion must write normal YAML diffs into docs/awareness.
2. Promotion must move proposal status to PROMOTED.
3. Promotion must rebuild or mark graph rebuild required.
4. Promotion must never happen silently from AI output alone.

Add tests:

1. Valid incident bundle creates proposal draft.
2. Proposal with missing root_cause is rejected.
3. Proposal that weakens critical invariant is rejected.
4. Proposal that adds dangerous required recovery cycle is rejected.
5. Proposal that adds context aliases is accepted.
6. Promotion writes to approved YAML files.
7. Promoted proposal appears in agent-context for related task.
8. AI proposal cannot directly modify approved awareness files.

Use the Envoy desired_hash restart storm as the first test fixture.

Fixture incident summary:
- gRPC-Web calls fail with status 0
- Envoy receives repeated SIGTERM
- systemd start-limit-hit
- install workflows repeatedly dispatch
- desired_hash mismatch between raw artifact digest and ComputeInfrastructureDesiredHash
- xds installed-state missing

Expected proposal includes:
- failure_mode: infra.desired_hash_mismatch_restart_storm
- invariant: infra.desired_hash_consistency
- invariant: service.restart_singleflight
- invariant: infra.heartbeat_not_desired_authority
- forbidden fix: restart_envoy_on_every_drift_tick
- forbidden fix: use_raw_artifact_digest_as_desired_hash
- alias: desired hash mismatch
- alias: restart storm
- required test: TestDriftWorkflowUsesDesiredHash

Definition of done:
The system can turn a real incident into reviewable YAML, validate it, promote it, rebuild the graph, and include the new scar in future agent-context output.
```

---

## Final Design Statement

Awareness Learning makes Globular’s awareness graph adaptive without making it unsafe.

The system can observe incidents, gather evidence, let AI reason about the failure chain, and produce structured awareness proposals.

But the system does not let AI silently rewrite architecture law.

The final model is:

```text
Runtime produces evidence.
AI produces proposals.
Validation produces confidence.
Review produces law.
Graph rebuild produces future awareness.
```

This allows Globular to grow a memory of its own construction.

Not just runtime memory.

Architecture memory.

A failure no longer disappears after it is fixed.

It becomes part of the map.
