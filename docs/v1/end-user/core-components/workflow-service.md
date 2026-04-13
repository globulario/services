# Workflow Service

## Purpose

This document defines the role of the workflow service in Globular.

The workflow service is responsible for **executing workflows, coordinating distributed actions, and producing system state transitions**.

It is the execution core of the system.

---

## What Is the Workflow Service

The workflow service is a **control plane service dedicated to execution**.

It:

* runs workflows
* coordinates step execution
* tracks progress and results
* produces final system state

Without it:

👉 workflows cannot run, and the system cannot evolve.

---

## Role in the System

The workflow service sits between:

* the cluster controller → which initiates workflows
* the node agents → which execute actions

It transforms intent into **coordinated execution**.

---

## Core Responsibilities

---

### Workflow Execution

The workflow service:

* receives workflow requests from the cluster controller
* creates workflow runs
* executes steps in order

Each workflow follows a clear lifecycle:

👉 admission → execution → completion

---

### Step Coordination

The service coordinates workflow steps:

* determines which steps to run
* dispatches actions to node agents
* tracks completion

Steps may execute:

* sequentially
* in parallel across nodes

---

### Distributed Execution

The workflow service manages execution across the cluster:

* sends instructions to node agents
* receives execution results
* aggregates outcomes

It maintains a consistent view of execution across all nodes.

---

### State Tracking

The workflow service records:

* step results
* node-level outcomes
* workflow status

This ensures that execution is:

* observable
* traceable
* reproducible

---

### Terminal State Resolution

Every workflow ends in a defined state:

* AVAILABLE
* DEGRADED
* FAILED

There is no indefinite execution.

---

## What the Workflow Service Does NOT Do

The workflow service is strictly bounded.

It does not:

* decide what should happen (cluster controller does)
* execute actions directly on nodes (node agents do)
* store artifacts (repository does)
* perform hidden orchestration

It only coordinates execution.

---

## Relationship with Cluster Controller

* The cluster controller validates and initiates
* The workflow service executes and tracks

This separation ensures:

* clear responsibility
* no hidden decision logic
* predictable behavior

---

## Relationship with Node Agents

The workflow service communicates with node agents:

* sends execution instructions
* receives structured results

Node agents do not coordinate themselves.

All coordination is centralized in the workflow service.

---

## Relationship with Repository Service

The workflow service uses repository artifacts:

* selects versions
* provides them to node agents

It does not manage repository content.

---

## Execution Characteristics

The workflow service enforces key properties:

---

### Explicit Execution

All actions are defined by workflows.

Nothing happens implicitly.

---

### Deterministic Behavior

Given the same inputs:

👉 execution follows the same path

---

### Observability

Every step and result is recorded.

There is no hidden execution.

---

### Finite Execution

Workflows:

* start
* progress
* end

There are no loops or continuous correction.

---

## Failure Handling

Failures are handled explicitly:

* step failure is recorded
* workflow state reflects the failure
* execution stops or degrades

There are no hidden retries.

Recovery requires a new workflow.

---

## Architectural Boundary

A strict boundary exists:

* cluster controller = decision
* workflow service = execution coordination
* node agents = execution

This boundary must be preserved.

---

## Mental Model

Think of the workflow service as a conductor.

* it does not perform the work
* it ensures each action happens at the right time
* it coordinates the system toward a final state

---

## One Sentence

The workflow service executes workflows, coordinates distributed actions across nodes, and ensures that all system changes occur in a controlled, observable, and deterministic manner.

