# Cluster Controller

## Purpose

This document defines the role of the cluster controller in Globular.

The cluster controller is responsible for **receiving system intent, validating it, and initiating workflow execution**.

It is the primary interface between desired state and system action.

---

## What Is the Cluster Controller

The cluster controller is a **control plane service**.

It acts as the system’s:

* admission point
* decision layer
* workflow initiator

It ensures that all changes to the system are:

* valid
* consistent
* aligned with system rules

---

## Role in the System

The cluster controller is the first step in any operation.

* Users or systems express intent
* The controller evaluates that intent
* If valid, it starts a workflow

It does not execute actions.

It decides whether execution should occur.

---

## Core Responsibilities

---

### Admission

The controller validates incoming requests.

This includes:

* dependency checks
* conflict detection
* policy enforcement

Invalid requests are rejected before execution begins.

---

### Resolution

The controller determines:

* what needs to change
* which artifacts are required
* which nodes are involved

It transforms intent into a form that workflows can execute.

---

### Workflow Initiation

Once validated, the controller:

* selects the appropriate workflow
* starts its execution
* tracks its lifecycle

From this point, execution is handled by the workflow service.

---

### State Coordination

The controller ensures:

* desired state is recorded
* workflow execution is tracked
* system transitions remain consistent

It does not directly modify installed state.

---

## What the Cluster Controller Does NOT Do

The controller is intentionally limited.

It does not:

* execute workflow steps
* perform node-level actions
* install or configure services
* contain hidden orchestration logic

All execution is delegated to workflows.

---

## Relationship with Workflow Service

The cluster controller and workflow service are tightly connected.

* the controller decides and initiates
* the workflow service executes

This separation ensures:

* clean boundaries
* predictable behavior
* no hidden logic

---

## Relationship with Repository Service

The controller may resolve artifacts using the repository.

It does not:

* store artifacts
* modify repository content

It only references repository data to validate intent.

---

## Relationship with Node Agent

The controller does not communicate directly with node agents.

All node-level actions are performed through workflows.

This ensures that:

👉 execution always follows the workflow model

---

## Failure Handling

If validation fails:

* the request is rejected
* no workflow is started
* system state remains unchanged

If execution fails:

* the workflow reflects the failure
* the controller observes the result
* recovery requires a new workflow

---

## Key Property

The cluster controller ensures that:

👉 nothing enters the system without being validated and structured

---

## Architectural Boundary

A strict boundary exists:

* cluster controller = decision and admission
* workflow service = execution coordination
* node agents = execution

The controller must never perform execution.

---

## Mental Model

Think of the cluster controller as a gatekeeper.

* it receives intent
* evaluates it
* allows or rejects it
* initiates controlled execution

It does not act beyond that.

---

## One Sentence

The cluster controller validates system intent, resolves required actions, and initiates workflows, ensuring that all changes enter the system in a controlled and consistent manner.

