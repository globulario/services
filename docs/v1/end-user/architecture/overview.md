# Architecture Overview

## Purpose

This document describes the high-level architecture of Globular.

It explains the major system components, how they interact, and the design principles that shape the platform.

Globular is designed as a system where **execution is explicit, state is separated, and coordination happens through workflows**.

---

## Architectural Character

Globular is a **workflow-driven distributed system platform**.

At its core, it is a cluster architecture, but it can also run as a single-node microservice runtime using the same model and components.

The architecture is built to provide:

* Explicit execution
* Clear state transitions
* Strong operational visibility
* Safe distributed coordination
* AI-assisted reasoning and operation

---

## High-Level Structure

Globular is composed of five major architectural pillars:

1. **Control Plane**
2. **Workflow Engine**
3. **Node Agents**
4. **Repository**
5. **Observability and AI Layer**

Each has a clear responsibility.

No component should absorb the role of another.

---

## 1. Control Plane

The control plane is the **coordination brain** of the system.

Its role is to:

* Validate intent
* Accept or reject operations
* Resolve desired actions
* Trigger workflow execution

The control plane does **not** perform orchestration logic step by step.

It does not act as a hidden planner.

Its role is to decide whether an operation should happen, then hand execution to workflows.

---

## 2. Workflow Engine

The workflow engine is the **execution core** of Globular.

It is responsible for:

* Running workflows
* Tracking workflow progress
* Recording step outcomes
* Producing state transitions

This is where the system moves.

All meaningful changes in Globular happen through workflow execution.

---

## 3. Node Agents

Node agents are the **execution hands** of the platform.

They run on individual nodes and perform the actual work required by workflows, such as:

* Installing packages
* Configuring services
* Starting or stopping runtime components
* Reporting execution results

The workflow engine coordinates.
The node agents act.

This separation allows Globular to remain distributed in action while centralized in intent.

---

## 4. Repository

The repository is the **artifact authority** of the system.

It stores:

* Packages
* Versions
* Builds
* Metadata
* Integrity information

The repository defines what can be deployed.

It does not decide what should run.
It provides the immutable material from which desired state can be realized.

---

## 5. Observability and AI Layer

Globular is designed to be operated by both humans and AI.

To support this, the system includes an observability and reasoning layer that provides:

* Metrics
* Events
* Health signals
* Diagnostic data
* Structured operational context

This layer allows AI to understand the system through explicit signals instead of guessing from incomplete symptoms.

In Globular, AI is not bolted on from the outside.
It is designed to operate against the same visible architecture as a human operator.

---

## How the Parts Work Together

At a high level, the architecture follows this flow:

1. A desired change is requested
2. The control plane validates the request
3. A workflow is started
4. The workflow coordinates actions across node agents
5. Node agents execute the required work
6. Results are recorded
7. Runtime health and observability reflect the outcome

This creates a system where change flows through a controlled path instead of emerging from background correction loops.

---

## Architectural Principles

Globular is built on a small number of strong principles.

### Explicit Over Implicit

Nothing important should happen invisibly.

All execution must be observable and attributable to a workflow.

---

### Separation of Responsibilities

Each component has a defined role:

* control plane decides
* workflow engine executes
* node agents act
* repository provides artifacts
* observability reports reality

This prevents architectural drift.

---

### Workflows as Operational Backbone

Workflows are not an implementation detail.

They are the backbone of system change.

The architecture is designed around workflow execution, not around hidden reconciliation.

---

### State Separation

Globular separates:

* what exists
* what should exist
* what is installed
* what is actually happening

This avoids ambiguity and makes failures diagnosable.

---

### Distributed Action, Centralized Intent

Intent is coordinated centrally.
Execution is carried out across nodes.

This gives the system both clarity and scalability.

---

## What the Architecture Is Not

Globular is not built around:

* continuous reconciliation loops
* opaque orchestration behavior
* container-first assumptions
* hidden side-channel configuration

It does not attempt to hide the mechanics of the system behind abstraction fog.

Instead, it exposes the structure clearly so it can be reasoned about, trusted, and extended.

---

## Mental Model

Think of Globular as a system with:

* a **brain** that admits and initiates
* an **engine** that executes
* **hands** that act on nodes
* a **vault** that stores deployable artifacts
* **eyes and memory** that observe and explain the outcome

Each part is distinct.
Together, they form a convergent system.

---

## One Sentence

Globular is a workflow-driven architecture where the control plane validates intent, workflows execute change, node agents perform distributed actions, the repository defines deployable artifacts, and observability makes the whole system understandable.

