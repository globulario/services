# Node Agent

## Purpose

This document defines the role of the node agent in Globular.

The node agent is responsible for **executing actions on a node** as instructed by workflows.

It is the component that turns intent into reality.

---

## What Is the Node Agent

The node agent is a **per-node execution service**.

It runs on every node in the system and is responsible for performing the actual work required to:

* Install software
* Configure services
* Start and stop processes
* Apply system-level changes

It does not decide what to do.

It executes what it is told.

---

## Role in the System

The node agent is the **execution layer** of Globular.

* The control plane decides
* The workflow engine coordinates
* The node agent acts

This separation ensures that execution is:

* Distributed
* Controlled
* Observable

---

## What the Node Agent Does

The node agent performs concrete actions on the node:

### Package Installation

* Fetches artifacts from the repository
* Verifies integrity (checksum)
* Installs packages on the system

---

### Service Management

* Starts services
* Stops services
* Restarts services
* Ensures services are configured correctly

---

### Configuration Application

* Applies configuration files
* Updates system settings
* Ensures consistency with desired state

---

### Execution of Workflow Steps

Each workflow step that targets a node is executed by the node agent.

It:

* Receives instructions
* Executes the action
* Returns the result

---

### State Reporting

After execution, the node agent reports:

* Success or failure
* Execution details
* Observed state

This information is used by workflows to update system state.

---

## What the Node Agent Does NOT Do

The node agent is intentionally limited.

It does not:

* Make decisions
* Modify desired state
* Trigger workflows
* Perform orchestration logic

It does not act independently.

---

## Communication Model

The node agent communicates through gRPC with:

* Workflow engine
* Control plane services

It receives structured instructions and returns structured results.

There is no implicit behavior.

---

## Execution Characteristics

The node agent is designed to be:

### Deterministic

Given the same instruction, it produces the same result.

---

### Idempotent

Repeated execution should not corrupt the system.

---

### Observable

All actions and outcomes are reported.

---

### Isolated

Each node executes independently.

Failures are localized.

---

## Failure Handling

If an action fails:

* The node agent reports the failure
* The workflow records it
* The system state becomes explicit

The node agent does not retry indefinitely or attempt hidden recovery.

---

## Relationship with Workflows

The node agent is driven entirely by workflows.

* It does not initiate actions
* It does not schedule work
* It does not interpret system intent

It executes steps exactly as defined.

---

## Architectural Boundary

A strict boundary exists:

* Node agent = execution only
* Workflow engine = coordination
* Control plane = decision

This boundary must never be blurred.

---

## Mental Model

Think of the node agent as a worker that:

* receives precise instructions
* performs them exactly
* reports the outcome

It does not question, plan, or adapt.

It executes.

---

## One Sentence

The node agent is a per-node execution service that performs workflow-defined actions, reports results, and never makes decisions on its own.

