# Why Not Kubernetes

## Purpose

This document explains why Globular does not follow the Kubernetes model.

It is not a critique of Kubernetes as a system.
It is a clarification of **different architectural choices**.

---

## The Context

Kubernetes is a powerful system designed around:

* Declarative state
* Continuous reconciliation
* Container orchestration

It solves many real problems and has become a standard in distributed systems.

Globular takes a different path.

---

## The Core Difference

The fundamental difference is this:

> Kubernetes continuously tries to make the system correct.
> Globular executes explicit operations to make the system correct.

---

## Reconciliation vs Execution

### Kubernetes

* Runs reconciliation loops continuously
* Compares desired vs actual state
* Applies changes repeatedly until convergence

This means:

* The system is always “in progress”
* Behavior emerges from loops
* Execution is implicit

---

### Globular

* Executes workflows explicitly
* Applies changes step by step
* Reaches a terminal, stable state

This means:

* Operations have a clear beginning and end
* Behavior is defined, not emergent
* Execution is explicit

---

## Visibility and Debugging

### Kubernetes

* State is distributed across controllers
* Behavior emerges from interactions
* Failures often require tracing multiple loops

### Globular

* Every action is part of a workflow
* Each step is recorded
* Failures are localized and traceable

---

## Control vs Emergence

Kubernetes favors:

* Autonomous controllers
* Continuous correction
* Emergent system behavior

Globular favors:

* Centralized intent
* Explicit execution
* Controlled state transitions

---

## Abstraction vs Exposure

Kubernetes abstracts infrastructure behind:

* Pods
* Controllers
* Hidden orchestration layers

Globular exposes structure:

* Workflows
* State layers
* Explicit execution paths

Nothing important is hidden.

---

## Containers vs System-Level Execution

Kubernetes is container-first.

Globular is not tied to containers.

It:

* Installs and manages system-level services
* Works directly on nodes
* Treats containers as optional, not foundational

---

## Source of Truth

Kubernetes distributes responsibility across:

* controllers
* CRDs
* runtime state

Globular enforces strict sources of truth:

* Repository (artifacts)
* Workflows (execution)
* etcd (state storage)

There are no side channels.

---

## Operational Model

Kubernetes:

* Continuous reconciliation
* Event-driven loops
* Implicit retries

Globular:

* Explicit workflows
* Step-based execution
* No hidden retries

If something fails:

👉 a new workflow must be executed.

---

## Trade-Offs

Globular does not attempt to replace Kubernetes in all contexts.

The trade-offs are intentional:

### What Kubernetes excels at

* Large-scale container orchestration
* Highly dynamic environments
* Ecosystem maturity

### What Globular focuses on

* Deterministic execution
* Clear operational visibility
* Strong control over system behavior
* AI-assisted reasoning

---

## Why This Matters

The difference is not just implementation.

It is a different philosophy:

### Kubernetes

> “The system will eventually become correct.”

### Globular

> “The system executed this operation, and here is the result.”

---

## Mental Model

Kubernetes behaves like a system that is constantly adjusting itself.

Globular behaves like a system that performs **precise operations and then stops**.

---

## One Sentence

Globular replaces continuous reconciliation with explicit workflow execution, trading emergent behavior for deterministic, observable, and controlled system evolution.

