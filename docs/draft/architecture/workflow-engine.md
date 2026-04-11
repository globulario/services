# Workflow Engine

## Purpose

This document defines the role of the workflow engine in Globular.

The workflow engine is responsible for **executing workflows, coordinating distributed actions, and producing all state transitions**.

It is the component that turns system intent into controlled execution.

---

## What Is the Workflow Engine

The workflow engine is the **execution core** of Globular.

It runs workflows that:

* Apply changes across the system
* Coordinate actions across nodes
* Track progress and outcomes
* Produce explicit state transitions

Without the workflow engine:

👉 The system cannot evolve.

---

## Role in the Architecture

The workflow engine sits between decision and execution:

* The control plane **decides what should happen**
* The workflow engine **executes how it happens**
* The node agents **perform the actions**

This separation ensures that:

* decisions are controlled
* execution is structured
* actions are distributed

---

## What the Workflow Engine Does

The workflow engine performs several key functions.

---

### Workflow Execution

* Starts workflows when triggered by the control plane
* Executes steps in order
* Manages execution across nodes

Each workflow has a clear lifecycle:

👉 start → execute → finish

---

### Step Coordination

The engine coordinates workflow steps:

* Determines which steps run
* Dispatches actions to node agents
* Tracks completion of each step

Steps may run:

* sequentially
* in parallel across nodes

---

### State Tracking

Every workflow produces state transitions.

The engine records:

* step outcomes
* node-level results
* overall workflow status

This ensures that:

* progress is visible
* failures are localized
* results are reproducible

---

### Distributed Execution Control

The workflow engine coordinates distributed execution:

* Sends instructions to node agents
* Receives execution results
* Aggregates outcomes

It maintains a consistent view of execution across the cluster.

---

### Terminal State Resolution

A workflow always ends in a known state:

* AVAILABLE
* DEGRADED
* FAILED

There is no indefinite execution.

---

## What the Workflow Engine Does NOT Do

The workflow engine is intentionally constrained.

It does not:

* Decide what should happen (control plane does)
* Perform node-level actions (node agents do)
* Store artifacts (repository does)
* Modify system state outside workflow execution

---

## Execution Characteristics

The workflow engine enforces key properties:

---

### Explicit Execution

All actions are driven by defined workflows.

Nothing happens implicitly.

---

### Deterministic Behavior

Given the same workflow and inputs:

👉 the same execution path is followed.

---

### Observability

Every step is:

* recorded
* visible
* traceable

---

### Bounded Execution

Workflows:

* start
* progress
* finish

There are no infinite loops.

---

## Failure Handling

Failures are handled explicitly:

* A failing step is recorded
* The workflow reflects the failure
* The system state becomes explicit

The engine does not retry indefinitely.

Recovery requires a new workflow.

---

## Relationship with the State Model

The workflow engine is the mechanism that moves the system across layers:

* Desired → Installed
* Installed → Validated
* Execution → Final state

It does not collapse layers.

It transitions them.

---

## Relationship with Node Agents

The workflow engine coordinates node agents.

* Sends execution requests
* Waits for results
* Aggregates outcomes

Node agents do not coordinate themselves.

---

## Architectural Boundary

A strict boundary exists:

* Control plane = decision
* Workflow engine = execution coordination
* Node agents = execution

This separation must be preserved.

---

## Mental Model

Think of the workflow engine as a **conductor**.

It does not play the instruments.

It ensures that:

* each part starts at the right time
* each action happens in order
* the system reaches a final state

---

## One Sentence

The workflow engine executes workflows, coordinates distributed actions across nodes, and produces all system state transitions in a controlled and observable way.

