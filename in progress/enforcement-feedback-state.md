# Globular — Execution Governor Requirements

## Purpose

This document defines the requirements for adding **enforcement, feedback, and state awareness** to the Globular CLI + MCP system.

The goal is to transform AI from a best-effort assistant into a **predictable, validated operator**.

---

## Problem Summary

The current system provides:

* `globular-cli` → execution layer
* MCP (`help`, `workflow`, `rules`) → guidance layer

However:

* Rules are advisory, not enforced
* Workflows are mostly static
* Execution is not strongly tied to real system state
* Results are not fed back into a structured decision loop

This creates risk:

* invalid commands may execute
* workflows may drift
* failures are not properly handled
* AI decisions are not grounded in actual state

---

## Target Model

Current:

AI → MCP → CLI → System

Target:

AI → MCP → Execution Governor → CLI → Result → State Update → Next Decision

The missing component is the **Execution Governor**.

---

## Core Responsibilities

The Execution Governor must provide:

1. **Enforcement** (hard validation before execution)
2. **Feedback** (structured execution results)
3. **State Awareness** (decisions based on real system state)

---

## Enforcement Requirements

### Pre-execution validation

Before executing any CLI command, validate:

* command exists
* command path is valid
* flags are valid
* required flags are present
* flag values are allowed
* workflow step order is respected
* environment/context is valid
* destructive actions require approval
* generated files are not modified directly (if forbidden)
* workflow preconditions are satisfied

### Validation result

Return structured status:

* `allowed`
* `blocked`
* `needs_confirmation`
* `invalid`
* `out_of_order`

Include:

* reason
* missing requirements
* suggested next step

### Rule

Validation must be **hard enforcement**.

Invalid commands must not execute.

---

## Feedback Requirements

### Execution result model

Each command must produce structured output:

* command
* args
* timestamp
* success
* exit_code
* stdout
* stderr
* produced_artifacts
* changed_files
* detected_state_changes
* warnings
* errors
* recommended_next_actions

### Behavior

After execution, the system must:

* analyze result
* decide next action based on outcome
* not proceed blindly

Possible next actions:

* continue
* retry
* rollback
* request approval
* branch workflow
* stop with error
* suggest remediation

---

## State Awareness Requirements

### State inspection

Before and after operations, inspect:

* project structure
* proto status
* generated files status
* git/working tree state
* build status
* test status
* package availability
* installed services
* rollout status
* cluster health
* node health
* service health

### Rules

* Do not generate if output is inconsistent
* Do not publish if build/test failed
* Do not install if package is missing
* Do not continue if rollout is degraded
* Do not claim success without verification

---

## Execution Governor Component

### Role

A new layer between AI intent and CLI execution.

Possible names:

* Execution Governor
* Operation Validator
* Workflow Executor

### Responsibilities

* validate commands
* enforce policies
  n- collect execution results
* capture state
* guide workflow transitions

---

## Data Model Proposal

### ValidationRequest

* command
* args
* workflow_step
* state_snapshot

### ValidationResult

* status
* reason
* missing_requirements
* suggested_next_step
* requires_approval

### ExecutionResult

* command
* args
* timestamp
* success
* exit_code
* stdout
* stderr
* artifacts
* changed_files
* state_changes
* warnings
* errors
* next_actions

### StateSnapshot

* workflow_state
* project_state
* generation_state
* build_state
* validation_state
* package_state
* deployment_state
* cluster_state
* rollout_state
* health_state

### WorkflowStepStatus

* step_name
* status
* started_at
* completed_at
* result

### ApprovalRequirement

* action
* reason
* scope
* requires_user_confirmation

---

## Execution Flow

1. AI proposes action

2. MCP provides rules/workflow/help

3. Governor validates command

   * if blocked → return reason
   * if needs approval → request confirmation
   * if allowed → continue

4. Execute CLI command

5. Capture ExecutionResult

6. Capture updated StateSnapshot

7. Evaluate outcome

8. Decide next step

---

## Workflow Branching

Support dynamic paths:

* validation failure → stop
* approval required → wait
* build fail → remediation branch
* partial rollout → degraded state
* inconsistency → regeneration required

---

## Approval Gates

Require explicit approval for:

* uninstall
* overwrite generated files
* production deployment
* destructive operations
* rollback actions

Approval must be policy-driven.

---

## Integration Points

### MCP

* `globular_cli.help`
* `globular_cli.workflow`
* `globular_cli.rules`

Governor must use these as inputs.

### CLI

* All execution must go through governor
* CLI remains execution engine

### Observation

* integrate with MCP observation tools
* expose health and rollout state

---

## Example Workflows

### Create Service

* validate proto
* validate command
* generate service
* verify files

### Build and Validate

* run build
* run tests
* branch if failure

### Package Publish

* verify build success
* publish package

### Install Service

* verify package exists
* install
* verify health

### Failed Deployment

* detect failure
* mark degraded
* propose remediation

### Destructive Command

* detect risk
* require approval
* block if not confirmed

---

## Invariants

* no hallucinated commands
* no execution without validation
* no success without observed result
* no deployment success without health verification
* no destructive action without policy validation

---

## Implementation Plan

### Phase 1 — Validator

* basic command validation
* flag validation

### Phase 2 — Execution Result

* structured result model
* capture stdout/stderr

### Phase 3 — State Snapshot

* implement state collectors

### Phase 4 — Workflow Engine

* branching logic
* result-based transitions

### Phase 5 — Approval System

* policy rules
* confirmation handling

### Phase 6 — Integration

* connect to MCP
* wrap CLI execution

---

## First PR Recommendation

Implement minimal vertical slice:

* validation wrapper around CLI
* basic ValidationResult
* ExecutionResult capture
* simple state snapshot (build + files)
* block invalid commands

This enables immediate enforcement and sets foundation for expansion.

---

## Constraints

* proto is source of truth
* CLI-driven generation only
* do not modify generated files
* deterministic behavior
* inspectable outputs
* compatible with AI and human operators

---

## Summary

This layer transforms Globular into a:

**state-aware, policy-enforced, feedback-driven AI operator system**

Without it, the system remains advisory.

With it, the system becomes reliable and production-grade.

---

# Additional Requirements — Phase 2 (Gap Analysis)

This section defines the **remaining requirements** needed to fully satisfy the Execution Governor design.

The current implementation provides a strong Phase 1 (validation + execution + basic state), but the following capabilities are still required.

---

## 1. Workflow State Enforcement

### Requirement

The governor must enforce **workflow order and preconditions**, not just validate commands.

### Missing Capabilities

* Track current workflow step
* Validate step ordering (prevent out-of-order execution)
* Enforce prerequisites (e.g., build before publish, publish before install)
* Maintain workflow progression state

### Expected Behavior

* Block commands that violate workflow order
* Return `out_of_order` with corrective guidance
* Tie validation to workflow + state snapshot

---

## 2. Result-Driven Decision Loop

### Requirement

Execution must drive the next step automatically.

### Missing Capabilities

* Branch workflows based on execution result
* Retry logic
* Rollback paths
* Automated remediation suggestions tied to failure types

### Expected Behavior

* Success → continue workflow
* Failure → branch to remediation or retry
* Partial success → mark degraded and adapt
* No blind continuation

---

## 3. Extended State Awareness (System-Level)

### Requirement

State awareness must go beyond project filesystem.

### Missing Capabilities

* Package repository state
* Installed services state
* Deployment/rollout status
* Cluster health
* Node health
* Service health
* Dependency readiness

### Expected Behavior

* Validate actions against real system state
* Prevent invalid operations (e.g., install missing package)
* Verify deployment success via health checks

---

## 4. Policy-Driven Approval System

### Requirement

Approval must be explicit and policy-based.

### Missing Capabilities

* Defined approval categories
* Policy rules for sensitive operations
* Structured approval requests

### Required Approval Cases

* Uninstall operations
* Overwriting generated files
* Production deployments
* Rollbacks
* Commands affecting running cluster

### Expected Behavior

* Return `needs_confirmation`
* Require explicit user approval before execution
* Log approval decisions

---

## 5. Rich Execution Result Model

### Requirement

Execution results must support deep analysis and orchestration.

### Missing Fields

* produced_artifacts
* changed_files
* detected_state_changes

### Expected Behavior

* Enable diff-based reasoning
* Support validation of side effects
* Allow workflow branching based on changes

---

## 6. State Transition Evaluation

### Requirement

The system must evaluate state transitions after execution.

### Missing Capabilities

* Compare pre/post state snapshots
* Detect unexpected changes
* Validate expected outcomes

### Expected Behavior

* Confirm intended changes occurred
* Detect drift or inconsistencies
* Trigger remediation if mismatch

---

## 7. Dynamic Workflow Engine

### Requirement

Workflows must support branching logic.

### Missing Capabilities

* Conditional steps
* Multi-path execution
* Failure branches
* Degraded state handling

### Expected Behavior

* Not linear step execution
* Adaptive flow based on results and state

---

## 8. Stronger Invariants Enforcement

### Requirement

Invariants must be enforced, not implied.

### Additional Invariants

* no workflow step without state validation
* no deployment success without health verification
* no progression on failed validation
* no destructive action without policy approval

---

## Phase Status

### Phase 1 — Completed

* command validation
* execution wrapper
* basic state inspection

### Phase 2 — Required

* workflow enforcement
* result-driven branching
* extended system state
* policy-based approval
* richer execution model
* state transition validation

---

## Final Note

The system has successfully reached **Minimum Viable Governor**.

These additional requirements will elevate it to a:

**fully deterministic, state-aware, policy-governed execution system**.

---

# Phase 7 — Pre-Install Impact Analysis & Admission Gate

## Goal

Introduce a **preflight decision engine** that evaluates a requested operation (e.g., installing `ldap`) against the current cluster state and predicts its impact **before execution**. The system must return a deterministic admission decision: `allow`, `allow_with_approval`, `block`, or `requires_remediation`.

## Non-Goals

* Perfect prediction of all runtime failures
* Replacing post-execution health checks
* Replacing rollback (remains as safety net)

## Core Concepts

### 1. OperationPlan

A normalized, structured representation of an intended action.

```go
type OperationPlan struct {
    Command            string
    Args               []string
    TargetService      string
    TargetVersion      string
    TargetNodes        []string
    TargetProfile      string
    RequestedBy        string
    Timestamp          int64
}
```

### 2. ImpactReport

Predicted effects of applying the plan.

```go
type ImpactReport struct {
    DependenciesRequired   []string
    DependenciesMissing    []string
    ServicesAffected       []string
    PortsRequired          []int
    PortConflicts          []int
    FilesToChange          []string
    ServicesToRestart      []string
    DNSChanges             []string
    TLSChanges             []string
    RBACChanges            []string
    ResourceEstimate       ResourceEstimate
    RiskLevel              string // low | medium | high
}
```

### 3. AdmissionDecision

Final decision returned by the governor.

```go
type AdmissionDecision struct {
    Status                 string // allow | allow_with_approval | block | requires_remediation
    Reasons                []string
    MissingRequirements    []string
    SuggestedRemediation   []string
    RequiresApproval       bool
}
```

---

## Validation Pipeline

When Claude requests an operation:

```
plan → collect_state → analyze_dependencies → analyze_conflicts → analyze_policy → compute_impact → decide
```

### Step 1 — Collect State

Reuse existing:

* `CollectFullStateSnapshot`

Must include:

* services installed
* running processes
* ports in use
* package repository contents
* DNS / gateway state
* TLS / certs
* RBAC policies
* system resources (CPU, RAM, disk)

### Step 2 — Dependency Analysis

* verify required services exist
* verify version compatibility
* verify repository contains requested package

Output:

* `DependenciesRequired`
* `DependenciesMissing`

### Step 3 — Conflict Analysis

* port collisions
* duplicate service names
* conflicting DNS entries
* incompatible versions

Output:

* `PortConflicts`
* `ServicesAffected`

### Step 4 — Resource Fit

Estimate resource usage and compare with node capacity.

```go
type ResourceEstimate struct {
    CPUCores float64
    MemoryMB int
    DiskMB   int
}
```

### Step 5 — Policy Evaluation

* check approval policies
* check environment constraints (prod/dev)
* check destructive scope

### Step 6 — Impact Computation

Build full `ImpactReport`.

### Step 7 — Admission Decision

Rules:

* If missing dependencies → `requires_remediation`
* If conflicts detected → `block`
* If high risk or policy → `allow_with_approval`
* Otherwise → `allow`

---

## MCP Tool

### globular_cli.plan

#### Input

* command (string)
* args (optional)

#### Output

* operation_plan
* impact_report
* admission_decision

This tool is **read-only**.

---

## Integration with Existing Tools

### Updated Execution Flow

```
plan → validate → check_approval → execute → state → state_diff
```

* `validate` must internally call `plan`
* `execute` MUST refuse if decision != allow / approved

---

## Workflow Integration

* Workflow steps must include preflight gate
* A step cannot advance if decision is `block` or `requires_remediation`

---

## Invariants

1. No execution without admission decision
2. Admission decision must be deterministic for same state
3. All blocking reasons must be explicit
4. All remediation steps must be actionable

---

## Example

Request:

```
Install ldap
```

Result:

* DependenciesMissing: ["etcd"]
* PortConflicts: []
* RiskLevel: medium

Decision:

```
Status: requires_remediation
SuggestedRemediation: ["install etcd first"]
```

---

## Deliverables for Claude

Claude must:

1. Implement `OperationPlan`, `ImpactReport`, `AdmissionDecision`
2. Add `plan` MCP tool
3. Extend governor validation to call planning stage
4. Implement dependency + conflict analyzers
5. Integrate with workflow engine
6. Add unit tests:

   * missing dependency → requires_remediation
   * port conflict → block
   * safe install → allow
   * policy case → allow_with_approval

---

## Success Criteria

* Claude can explain *why* a command is allowed or blocked
* No unsafe install is executed without explicit approval
* System predicts conflicts before execution
* Rollback usage decreases significantly

---

## Future Extensions

* simulation mode (dry-run execution)
* cost estimation
* multi-node placement planning
* automatic remediation workflows

---

# Phase 7B — Descriptor-Driven Dependency & Impact Graph

## Goal

Evolve the admission planner from static dependency knowledge to **descriptor-driven dependency and impact analysis**, enabling accurate, version-aware, and transitive reasoning about service installation.

## Problem

Phase 7A relies on a static dependency registry. This creates:

* drift between real service requirements and hardcoded data
* incomplete knowledge for new services
* no version-aware validation
* no transitive dependency reasoning
* limited understanding of system-wide impact

## Core Principle

**Dependency and impact metadata must live with the service/package**, not in the planner.

---

## Descriptor Model

Introduce a structured descriptor embedded in service/package metadata.

```go
type ServiceDescriptor struct {
    Name                 string
    Version              string
    Dependencies         []DependencyDescriptor
    CapabilitiesProvided []string
    CapabilitiesRequired []string
    Ports                []PortDescriptor
    RestartScope         []string
    SideEffects          []string
    ImpactScope          []string
}

 type DependencyDescriptor struct {
    Name              string
    Required          bool
    VersionConstraint string
    Capability        string
}

 type PortDescriptor struct {
    Port     int
    Protocol string
    Purpose  string
}
```

---

## Planner Evolution

### From

* static dependency map
* static ports

### To

* descriptor-first dependency source
* descriptor-defined ports
* static registry as fallback only

---

## Required Capabilities

### 1. Descriptor-first lookup

* load descriptor from package repository
* extract dependencies and ports
* fallback to static registry if missing

### 2. Version-aware validation

Detect:

* missing dependency
* incompatible version
* optional dependency absence

### 3. Transitive dependency graph

Build full dependency chain:

* direct + indirect dependencies

### 4. Impact graph

Predict effects on system:

* DNS changes
* gateway updates
* RBAC modifications
* service restarts
* discovery changes

### 5. Legacy compatibility

* support services without descriptors
* fallback with warning

---

## Planner Output Enhancements

`globular_cli.plan` must include:

* direct dependencies
* transitive dependencies
* dependency graph summary
* impact graph summary
* version compatibility results
* fallback warnings

---

## Admission Decision Enhancements

New decision signals:

* missing transitive dependency
* version mismatch
* dependency cycle
* capability unsatisfied
* descriptor missing
* sensitive impact

Decision rules:

* missing dependency → requires_remediation
* version conflict → block or requires_remediation
* sensitive impact → allow_with_approval
* fallback used → allow with warning

---

## Interfaces

Evolve dependency source:

```go
type DependencySource interface {
    Descriptor(service string, version string) (*ServiceDescriptor, error)
}
```

Planner derives dependencies and ports from descriptor.

---

## Implementation Files

* golang/mcp/descriptors.go — descriptor loading/parsing
* golang/mcp/impact.go — planner extension
* golang/mcp/impact_graph.go — graph logic
* golang/mcp/deps.go — fallback/static registry
* golang/mcp/impact_test.go — extended tests

---

## Tests

Add cases:

1. direct dependency missing
2. transitive dependency missing
3. version mismatch
4. descriptor-based port conflict
5. fallback behavior
6. sensitive impact requiring approval
7. dependency cycle detection

---

## Acceptance Criteria

* planner uses descriptor metadata when available
* transitive dependency resolution works
* version constraints evaluated
* impact graph returned in plan
* fallback works for legacy services
* decisions reflect descriptor data

---

## Non-Goals

* resource estimation
* multi-node placement
* full simulation engine
* automatic remediation execution

---

## Instruction for Claude

Implement Phase 7B with:

* descriptor-first dependency system
* fallback to static registry
* transitive dependency graph
* version-aware validation
* impact graph output in plan tool
* backward compatibility

Return:

* architecture
* file changes
* interfaces
* descriptor schema
* test plan
* smallest first PR slice

---

## Outcome

The system evolves from:

> “checking if install is safe”

To:

> “understanding how the system is structured and how a change propagates through it”

---

# Phase 7C — Versioned Dependency Resolution & Safe Lifecycle Management

## Goal

Introduce **version-aware dependency management, reverse dependency tracking, and safe uninstall/upgrade semantics** to prevent breaking service-to-service relationships.

## Problem

Current system:

* tracks service version at install time
* does not enforce dependency version compatibility
* does not track which services depend on others
* cannot safely determine if uninstall/update will break the system

This can lead to:

* broken service chains after update
* unsafe uninstall of required services
* hidden dependency coupling

---

## Core Principle

> The system must know not only what a service *requires*, but also **who depends on it and which version they depend on**.

---

## Descriptor Enhancements (Proto-Level)

Extend service/package descriptor with:

```proto
message DependencyDescriptor {
  string name = 1;
  string publisher_id = 2;
  string version_constraint = 3; // ex: ">=1.2.0,<2.0.0"
  bool required = 4;
  string capability = 5;
}
```

---

## Runtime Model

### Installed Service Instance

Each installed service must record resolved dependencies:

```go
type ResolvedDependency struct {
    Name            string
    PublisherID     string
    ResolvedVersion string
    InstanceID      string
}
```

```go
type InstalledService struct {
    ID                  string
    Version             string
    ResolvedDependencies []ResolvedDependency
}
```

---

## Reverse Dependency Tracking

System must maintain mapping:

```go
map[ServiceInstanceID][]DependentInstanceID
```

This enables:

* "who depends on this service"
* "what will break if I remove/update this"

---

## Planner Enhancements

### New Checks

1. Reverse dependency check

   * detect dependents before uninstall

2. Version compatibility

   * validate constraints during install/update

3. Upgrade impact

   * detect if upgrade breaks existing dependents

---

## Admission Decision Enhancements

New conditions:

* uninstall with active dependents → block or requires_approval
* upgrade breaking dependency constraint → block
* multiple compatible dependents → allow

---

## Lifecycle Rules

### Install

* resolve dependencies with version constraints
* record resolved dependency graph

### Uninstall

* if dependents exist:

  * block OR
  * require approval OR
  * suggest remediation (remove dependents first)

### Update

* check compatibility with all dependents
* if incompatible:

  * block OR
  * suggest upgrade chain

---

## Optional Future Mode

### Multi-Version Coexistence

Support parallel versions:

* service v1 and v2 installed simultaneously
* dependents pinned to specific versions

Requires:

* version-aware routing
* versioned discovery

---

## Tests

Add cases:

1. uninstall with dependents → blocked
2. uninstall after removing dependents → allowed
3. upgrade breaking constraint → blocked
4. upgrade compatible → allowed
5. dependency resolution stores correct version

---

## Acceptance Criteria

* system tracks resolved dependencies at install time
* reverse dependency lookup works
* uninstall safety enforced
* upgrade compatibility validated
* planner includes dependency impact in decisions

---

## Non-Goals

* full multi-version routing (future)
* automatic migration between versions

---

## Instruction for Claude

Implement Phase 7C with:

* proto-level dependency descriptors with version constraints
* runtime storage of resolved dependencies
* reverse dependency tracking
* planner integration for uninstall/update safety

Return:

* schema updates
* storage model
* planner changes
* test plan
* smallest PR slice

---

## Outcome

The system evolves from:

> “can I install safely?”

To:

> “can I safely evolve the system over time without breaking anything?”

---

# Phase 7D — Ordered Remediation & Safe Removal Planning

## Goal

Upgrade reverse-dependency protection into an **actionable remediation planner** that computes a safe, ordered sequence of steps when an operation is blocked.

This phase answers not only:

> "Why is this blocked?"

but also:

> "What exact sequence should I follow to make it safe?"

---

## Problem

Current planner behavior can correctly return:

* `requires_remediation`
* list of dependents
* high-level suggestions like "remove ldap first"

That is already valuable, but still leaves the operator to manually determine:

* correct removal order
* which services are leaves vs shared dependencies
* whether multiple remediation paths are possible
* whether some steps can be grouped safely

This becomes difficult as the graph grows.

---

## Core Principle

> When an operation is blocked by dependencies, the system should compute a **safe remediation plan**, not just report the obstacle.

---

## Scope

This phase focuses on:

* safe removal planning
* ordered remediation steps
* dependency unwind sequencing
* actionable plan output

This phase does **not** yet include:

* automatic remediation execution
* version migration planning
* multi-version upgrade orchestration

---

## Required Behavior

### 1. Removal Order Planning

For a blocked remove/uninstall operation, compute:

* all direct dependents
* all transitive dependents
* a safe leaf-first removal order

Example:

If:

* `blog` depends on `ldap`
* `search` depends on `ldap`
* `ldap` depends on `persistence`

Then removing `persistence` should produce a plan like:

1. remove `blog`
2. remove `search`
3. remove `ldap`
4. remove `persistence`

---

### 2. Remediation Plan Output

Introduce a structured remediation model.

```go
type RemediationPlan struct {
    TargetOperation   string
    Status            string // ready | blocked | ambiguous
    Reason            string
    OrderedSteps      []RemediationStep
    AffectedServices  []string
    Warnings          []string
}

 type RemediationStep struct {
    Order             int
    Action            string // remove | disable | reconfigure | install
    Target            string
    Reason            string
    Blocking          bool
}
```

---

### 3. Graph Traversal

The planner must:

* traverse reverse dependency graph
* detect leaves
* compute valid topological removal order
* detect cycles or ambiguous cases

If cycles exist, planner must return:

* `ambiguous` or `blocked`
* explicit cycle information
* manual intervention guidance

---

### 4. Grouped vs Sequential Steps

If multiple leaf services can be removed independently, planner may group them conceptually but must still produce deterministic order.

Example:

* remove `blog`
* remove `mail`
* remove `search`

These may all be leaves, but output must remain stable and predictable.

---

### 5. Remediation Integration with Admission Decision

When an operation is blocked due to dependencies:

* decision remains `requires_remediation`
* output includes `remediation_plan`

This turns the planner from:

* obstacle reporter

into:

* guided operator assistant

---

## MCP Tooling

### Option A — Extend `globular_cli.plan`

Preferred first step.

When decision is `requires_remediation`, include:

* `remediation_plan`

### Option B — Add dedicated tool later

Possible future addition:

* `globular_cli.remediation_plan`

For now, keep it inside `plan` unless separation becomes necessary.

---

## Algorithm

### Input

* target operation
* current installed services
* reverse dependency graph

### Output

* topologically ordered safe remediation steps

### Suggested approach

1. Build reverse dependency graph from current descriptor/static sources
2. Extract impacted subgraph for target
3. Perform topological sort from leaves toward target
4. Return ordered removal/remediation plan
5. Detect and report cycles

---

## Acceptance Rules

### Valid plan

A remediation plan is valid if:

* every dependent appears before the dependency it blocks
* order is deterministic
* target operation appears last
* no step violates known dependency relationships

### Invalid plan

A plan must be rejected if:

* graph contains cycle
* required dependency information is missing
* planner cannot determine safe order

---

## Tests

Add cases:

1. single dependent

   * remove ldap → requires removing one dependent first

2. multiple leaf dependents

   * remove ldap → returns deterministic leaf-first order

3. multi-level chain

   * remove persistence → remove blog/search/ldap first, then persistence

4. cycle detection

   * A depends on B, B depends on A → blocked with cycle warning

5. leaf service removal

   * remove torrent → allowed, no remediation needed

---

## Files

Suggested additions:

* `golang/mcp/remediation.go`

  * remediation plan types
  * graph traversal
  * topological ordering

* `golang/mcp/impact.go`

  * planner integration

* `golang/mcp/impact_test.go`

  * remediation plan tests

Optional later:

* `golang/mcp/tools_remediation.go`

---

## Planner Decision Enhancements

When blocked by dependencies, planner should now return:

```json
{
  "status": "requires_remediation",
  "reason": "service has active dependents",
  "remediation_plan": {
    "ordered_steps": [
      {"order": 1, "action": "remove", "target": "blog"},
      {"order": 2, "action": "remove", "target": "search"},
      {"order": 3, "action": "remove", "target": "ldap"},
      {"order": 4, "action": "remove", "target": "persistence"}
    ]
  }
}
```

---

## Non-Goals

Do not include yet:

* automatic execution of remediation steps
* approval chaining across all remediation steps
* migration planning for updates
* semantic version upgrade paths

---

## Claude Instruction

Implement Phase 7D with:

* remediation plan types
* reverse dependency graph traversal
* deterministic leaf-first removal ordering
* cycle detection
* integration into `globular_cli.plan` when decision is `requires_remediation`

Return:

* architecture
* algorithm
* file changes
* tests
* smallest first PR slice

---

## Outcome

The system evolves from:

> "I know this operation is unsafe."

To:

> "I know exactly what sequence will make it safe."

This makes Globular not only protective, but operationally helpful.

---

# Phase 7E — Executable Remediation Workflow

## Goal

Transform remediation plans from **advisory sequences** into **controlled, executable workflows**.

This phase answers:

> "Can the system safely execute the remediation plan step-by-step with control, validation, and recovery?"

---

## Problem

Phase 7D provides:

* ordered remediation steps
* deterministic safe sequence

But execution is still manual:

* operator runs each command
* risk of deviation or partial execution
* no automatic verification between steps

---

## Core Principle

> A safe plan should be executable with guardrails, visibility, and the ability to stop or adapt.

---

## Scope

This phase introduces:

* step-by-step execution of remediation plans
* approval gates
* per-step validation and state verification
* controlled failure handling

This phase does **not** include:

* full rollback engine
* distributed transactions
* automatic retry policies (basic retry allowed)

---

## Required Behavior

### 1. Workflow Execution Engine

Introduce a workflow runner capable of:

* executing remediation steps sequentially
* validating before each step
* verifying state after each step
* halting on failure

```go
type ExecutionWorkflow struct {
    ID               string
    Plan             RemediationPlan
    Status           string // pending | running | paused | failed | completed
    CurrentStep      int
    StepResults      []StepResult
    StartedAt        time.Time
    CompletedAt      *time.Time
}

 type StepResult struct {
    StepOrder        int
    Action           string
    Target           string
    Success          bool
    Output           string
    Error            string
    DurationMs       int64
}
```

---

### 2. Execution Flow

For each step:

1. Validate step (reuse `validate` tool)
2. Optional approval check
3. Execute command (`execute` tool)
4. Verify state (`state` tool)
5. Record result

If failure occurs:

* stop execution
* mark workflow as `failed`
* return partial results

---

### 3. Approval Gate

Workflow must support:

* global approval (execute entire plan)
* per-step approval (high-risk operations)

Reuse existing approval mechanism.

---

### 4. State Verification

After each step:

* confirm target service state changed as expected
* detect drift or unexpected side effects

If mismatch:

* halt workflow
* mark as failed

---

### 5. Pause / Resume

Workflow should support:

* pause after any step
* resume from last successful step

---

### 6. Read-only Safety Mode

Default mode:

* simulation only (dry-run execution)

Explicit flag required for real execution.

---

## MCP Tooling

### globular_cli.execute_plan

Input:

* remediation_plan
* dry_run (default: true)
* auto_approve (optional)

Output:

* workflow_id
* status
* step_results
* next_action

---

### globular_cli.workflow_status

Input:

* workflow_id

Output:

* full workflow state
* current step
* results so far

---

## Algorithm

1. Receive remediation plan
2. Initialize workflow
3. For each step:

   * validate
   * execute (or simulate)
   * verify state
   * record result
4. On failure → stop
5. On completion → mark success

---

## Acceptance Rules

A workflow is valid if:

* steps executed in order
* each step validated before execution
* state verified after each step

A workflow fails if:

* step execution fails
* state verification fails
* approval denied

---

## Tests

Add cases:

1. successful full workflow
2. failure at step N → stops
3. pause and resume
4. dry-run execution (no real changes)
5. state mismatch detection

---

## Files

* `golang/mcp/workflow.go`

  * execution engine

* `golang/mcp/tools_workflow.go`

  * MCP tools

* `golang/mcp/workflow_test.go`

  * tests

---

## Non-Goals

Do not include yet:

* automatic rollback
* distributed orchestration across nodes
* advanced retry/backoff strategies

---

## Claude Instruction

Implement Phase 7E with:

* workflow execution engine
* step-by-step remediation execution
* validation + state verification integration
* approval gates
* pause/resume support

Return:

* architecture
* execution flow
* file changes
* tests
* smallest first PR slice

---

## Outcome

The system evolves from:

> "Here is the safe sequence"

To:

> "I can execute the safe sequence, step by step, safely and visibly"

This turns Globular into an **active operator assistant**, not just a planner.
