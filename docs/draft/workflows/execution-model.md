# Execution Model

## Purpose

This document explains how workflows are executed in Globular.

It defines how workflows progress from start to completion, how steps are coordinated across nodes, and how execution produces system state transitions.

---

## The Nature of Execution

Workflow execution in Globular is:

* Explicit
* Step-based
* Distributed
* Finite

A workflow does not run continuously.

It is:

👉 started → executed → completed

---

## Execution Lifecycle

Every workflow follows the same lifecycle:

### 1. Admission

* The control plane validates the request
* Dependencies and constraints are checked
* The workflow is accepted or rejected

No execution begins before admission.

---

### 2. Initialization

* A workflow run is created
* Initial state is recorded
* Target nodes are identified

At this point, execution is ready to begin.

---

### 3. Step Execution

The workflow engine executes steps in order.

Each step:

* defines an action
* targets one or more nodes
* produces a result

Steps may run:

* sequentially
* in parallel across nodes

---

### 4. Node-Level Execution

For each step:

* the workflow engine sends instructions to node agents
* node agents execute the action
* results are returned

Execution is distributed, but coordination is centralized.

---

### 5. Result Aggregation

The workflow engine collects:

* node-level results
* step outcomes
* execution status

This creates a complete view of the workflow’s progress.

---

### 6. Finalization

Once all steps are executed:

* the workflow reaches a terminal state
* system state is updated
* results are recorded

Execution ends here.

---

## Step Model

Each step is:

* atomic in intent
* observable
* bounded in execution

A step must:

* perform a clear action
* produce a clear result

Steps do not contain hidden behavior.

---

## Parallelism

Workflows support controlled parallel execution.

* Steps can target multiple nodes simultaneously
* Each node executes independently
* Results are aggregated centrally

Parallelism is explicit and visible.

---

## Callbacks and State Updates

Execution results are fed back into the system through structured updates.

Examples include:

* node succeeded
* node failed
* step completed

These updates:

* drive state transitions
* inform the workflow engine
* maintain consistency with the state model

---

## Idempotency

Execution must be safe to repeat.

* Re-running a step should not corrupt the system
* Partial execution can be retried
* Outcomes remain consistent

This is essential for:

* recovery
* failure handling
* automation

---

## Failure Handling

Failures are explicit and localized.

If a step fails:

* the failure is recorded
* the workflow reflects the failure
* execution stops or marks degradation

There are no hidden retries.

Recovery requires:

* re-execution
* or a remediation workflow

---

## No Continuous Execution

Workflows do not run indefinitely.

There are no:

* background loops
* implicit corrections
* continuous reconciliation

Execution is bounded.

---

## Relationship with the State Model

Execution is the mechanism that moves the system across layers:

* Desired → Installed
* Installed → Verified
* Verified → Final state

Each transition is the result of step execution.

---

## Deterministic Behavior

Given:

* the same workflow
* the same inputs
* the same artifacts

Execution will follow the same path.

This makes the system:

* predictable
* testable
* reproducible

---

## Observability

Execution is fully observable.

* every step is recorded
* every result is stored
* every failure is visible

There is no hidden execution path.

---

## Mental Model

Think of workflow execution as a **controlled sequence of operations**:

* each step does one thing
* each node executes independently
* the engine coordinates everything

When the sequence ends:

👉 the system is in a known state

---

## One Sentence

The execution model in Globular runs workflows as explicit, step-based, distributed operations that progress from admission to completion, producing deterministic and observable state transitions.

