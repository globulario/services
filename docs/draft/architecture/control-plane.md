# Control Plane

## Purpose

This document defines the role of the control plane in Globular.

The control plane is responsible for **validating intent, coordinating execution, and ensuring that system changes follow the defined rules**.

It does not perform the work itself.
It decides and initiates.

---

## What Is the Control Plane

The control plane is the **decision and coordination layer** of Globular.

It is responsible for:

* Accepting requests to change the system
* Validating those requests
* Resolving what needs to happen
* Triggering workflow execution

It ensures that all changes follow the system's rules and architecture.

---

## What the Control Plane Does

The control plane performs four essential functions:

### Admission

Every change request is validated before execution.

This includes:

* Dependency validation
* Conflict detection
* Policy enforcement

If a request is invalid, it is rejected before any action is taken.

---

### Resolution

The control plane determines:

* What needs to change
* Which artifacts are required
* Which nodes are involved

It transforms intent into an actionable plan.

---

### Workflow Initiation

Once validated, the control plane:

* Selects the appropriate workflow
* Starts its execution
* Tracks its lifecycle

From this point forward, execution is handled by the workflow engine.

---

### State Coordination

The control plane ensures that:

* Desired state is correctly recorded
* Workflow execution is tracked
* Final states are consistent

It does not directly modify installed state.

---

## What the Control Plane Does NOT Do

The control plane is intentionally limited.

It does not:

* Execute steps directly
* Perform installation or configuration
* Run services on nodes
* Contain hidden orchestration logic

All execution must go through workflows.

---

## Relationship with the Workflow Engine

The workflow engine is part of the control plane domain, but with a distinct role.

* The control plane **decides and starts**
* The workflow engine **executes and tracks**

The workflow engine is the mechanism that performs coordinated execution across the system.

---

## Relationship with Node Agents

Node agents are not part of the control plane.

They are execution components that:

* Run on individual nodes
* Perform actions requested by workflows
* Report results back

The control plane never directly executes actions on nodes.

---

## Architectural Boundary

A strict boundary exists:

* Control plane = coordination and decision
* Workflow engine = execution orchestration
* Node agents = execution on nodes

This separation ensures:

* clarity
* scalability
* predictable behavior

---

## Failure Handling

When something fails:

* The control plane does not attempt hidden recovery
* The workflow records the failure
* The system state becomes explicit

Recovery must be performed through a new workflow.

---

## Design Principle

> The control plane decides what should happen, but never performs the work itself.

This keeps the system:

* explicit
* traceable
* debuggable

---

## Mental Model

Think of the control plane as a coordinator that:

* listens to requests
* validates them
* starts execution
* observes outcomes

It does not act directly on the system.

---

## One Sentence

The control plane validates intent, coordinates execution through workflows, and ensures system state evolves correctly, without directly performing any work.

