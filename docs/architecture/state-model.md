# State Model

## Purpose

This document defines how Globular represents the system at any point in time.

It separates **what exists**, **what should exist**, **what is installed**, and **what is actually running**.

This separation is essential for making the system **observable, debuggable, and deterministic**.

---

## The Problem

In many systems, state is blurred:

* Desired and actual state are mixed
* Runtime conditions influence decisions implicitly
* It is unclear what is “true”

This leads to:

* Confusion
* Hidden behavior
* Difficult debugging

---

## The Globular Model

Globular separates system state into **four distinct layers**.

Each layer has a clear responsibility and must not be confused with the others.

---

## 1. Artifact Layer

**What exists and can be used**

This layer represents all available artifacts:

* Packages
* Versions
* Builds
* Metadata

### Source

Repository

### Role

Defines what *can* be deployed.

### Key Property

Immutable and versioned.

---

## 2. Desired State (Desired Release)

**What the system is supposed to run**

This layer defines:

* Which services should exist
* Which versions should be deployed
* Which nodes should run them

### Source

Control plane + etcd

### Role

Declares intent.

### Key Property

Does not execute anything by itself.

---

## 3. Installed State

**What has been deployed**

This layer represents what is actually installed on nodes:

* Installed packages
* Configured services
* Deployment results

### Source

Node agents (reported state)

### Role

Reflects execution results.

### Key Property

Derived from workflows.

---

## 4. Runtime Health

**What is currently happening**

This layer represents live system behavior:

* Service health
* Resource usage
* Availability
* Failures

### Source

Observability (metrics, probes, events)

### Role

Describes real-time conditions.

### Key Property

Ephemeral and constantly changing.

---

## Layer Relationships

Each layer answers a different question:

| Layer     | Question           |
| --------- | ------------------ |
| Artifact  | What exists?       |
| Desired   | What should run?   |
| Installed | What was applied?  |
| Runtime   | What is happening? |

These layers must remain **independent but connected**.

---

## State Transitions

State does not change automatically.

Transitions happen only through workflows:

1. Artifact is selected
2. Desired state is updated
3. Workflow executes
4. Installed state changes
5. Runtime reflects the result

👉 No layer directly mutates another without workflow execution.

---

## Important Invariants

### No Layer Collapsing

* Desired ≠ Installed
* Installed ≠ Runtime

These must never be treated as the same.

---

### No Implicit Correction

Runtime health must not directly modify desired or installed state.

👉 Observation does not trigger action without a workflow.

---

### No Direct Mutation

Installed state cannot be modified manually.

👉 Only workflows can change it.

---

## Failure Scenarios

Failures become clear when layers diverge:

### Desired ≠ Installed

Deployment incomplete or failed.

### Installed ≠ Runtime

Service is installed but not functioning.

### Runtime degraded

System is running but unhealthy.

Each mismatch is **visible and diagnosable**.

---

## Why This Matters

This model allows:

* Precise debugging
* Clear reasoning about system state
* Safe automation
* AI understanding of the system

Without this separation:

👉 the system becomes ambiguous.

---

## Mental Model

Think of the system as four stacked layers:

* The **inventory** (Artifact)
* The **plan** (Desired)
* The **result** (Installed)
* The **reality** (Runtime)

Workflows move the system **down this stack**, step by step.

---

## One Sentence

Globular separates system state into artifact, desired, installed, and runtime layers, ensuring every change is explicit, traceable, and understandable.

