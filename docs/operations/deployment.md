# Deployment

## Purpose

This document explains how Globular is deployed and how it transitions from an empty environment to a fully operational system.

Deployment in Globular is not a collection of manual steps.
It is a **controlled process driven by workflows and defined state transitions**.

---

## Deployment Philosophy

Globular treats deployment as an extension of its core principles:

* Explicit execution
* Clear state transitions
* No hidden behavior
* No implicit configuration

A deployment is not “setting things up”.
It is the **first convergence of the system**.

---

## Two Phases of Deployment

Globular deployment is divided into two distinct phases:

### Day 0 — Bootstrap

This is the creation of the initial system.

It establishes:

* the first node
* the control plane
* core infrastructure services
* system identity and trust (PKI, certificates)

At the end of Day 0:

👉 The system exists and can manage itself.

---

### Day 1 — Node Integration

This is the expansion of the system.

New nodes:

* join the cluster
* register with the control plane
* receive configuration
* execute workflows to reach the desired state

At the end of Day 1:

👉 The cluster becomes distributed and consistent.

---

## Day 0 — Bootstrap

Bootstrap is a special moment.

It is the only time where the system is **not yet fully self-managed**.

---

### What Happens During Bootstrap

* The first node is initialized
* Core services are installed (repository, workflow engine, control plane)
* Certificates and trust relationships are established
* Initial artifacts are made available
* The first workflows are executed

---

### Key Property

Bootstrap is still **workflow-driven**.

Even at this stage:

* actions are explicit
* steps are controlled
* results are observable

---

### Outcome

At the end of bootstrap:

* the system can execute workflows
* the repository is available
* the control plane is operational

👉 The system becomes self-hosting.

---

## Day 1 — Node Join

Once the system exists, additional nodes can be added.

---

### Node Join Process

1. A node registers with the control plane
2. The system assigns it a role or profile
3. Desired state is defined for that node
4. A workflow is executed
5. The node agent installs and configures required services
6. The node reports its state

---

### Key Property

Node integration is:

* repeatable
* deterministic
* driven by the same workflow model

There is no special-case logic for scaling the system.

---

## Desired State Driven Deployment

Deployment is driven by desired state.

* The system defines what should exist
* Workflows apply that definition
* Nodes converge to that state

Deployment is not procedural scripting.

It is **state-driven execution**.

---

## Relationship with the Repository

All deployment artifacts come from the repository.

* No manual installation
* No external dependency resolution
* No hidden sources

If it is not in the repository:

👉 it cannot be deployed.

---

## Relationship with Workflows

All deployment steps are executed through workflows.

* Bootstrap workflows initialize the system
* Apply workflows deploy services
* Node workflows bring nodes to desired state

There is no deployment outside workflows.

---

## Relationship with Node Agents

Node agents execute deployment actions:

* install packages
* configure services
* start runtime components

They do not decide what to install.

They follow workflow instructions.

---

## Idempotency and Repeatability

Deployment must be safe to repeat.

* Re-running a deployment should not corrupt the system
* Partial failures can be retried
* Results are consistent across nodes

This is critical for:

* recovery
* scaling
* automation

---

## Failure Handling

If deployment fails:

* The failure is recorded in the workflow
* The system state becomes explicit
* No hidden retries occur

Recovery requires:

* re-running the workflow
* or executing a remediation workflow

---

## Deployment Is Not Configuration Drift

Globular does not rely on:

* background correction
* hidden reconciliation loops

Deployment is:

👉 a controlled operation with a defined end

---

## Mental Model

Think of deployment as the system’s **first act of becoming itself**.

* Bootstrap creates the system
* Node join expands it
* Workflows shape it
* State defines it

---

## One Sentence

Deployment in Globular is a workflow-driven process that bootstraps the system, integrates nodes, and converges them to a desired state without relying on implicit or continuous reconciliation.

