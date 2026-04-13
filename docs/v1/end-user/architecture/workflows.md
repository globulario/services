# Workflows

## Purpose

This document defines how Globular **executes change**.

Workflows are the mechanism that moves the system from a desired state to a stable, known state.

They are not a feature of the system.
They *are* the system in motion.

---

## The Role of Workflows

In Globular, nothing happens by itself.

> Every change is the result of a workflow execution.

Workflows:

* Define what should be done
* Control how it is done
* Produce all state transitions

If something changes without a workflow:

👉 It is outside the system.

---

## What Is a Workflow

A workflow is a **structured sequence of steps** that:

* Takes a system from one state to another
* Executes actions across nodes
* Records progress and outcomes

Workflows are:

* Explicit
* Ordered
* Observable

They are defined once and executed consistently.

---

## Execution Model

A workflow follows a simple model:

1. It is **triggered** by the control plane
2. It is **executed** by the workflow engine
3. It performs steps across nodes through node agents
4. It **records results** at each step
5. It reaches a **terminal state**

There is no background execution.

There is only:

👉 start → execute → finish

---

## Step-Based Execution

Workflows are composed of **steps**.

Each step:

* Has a clear purpose
* Executes a specific action
* Produces a result

Steps can:

* Run sequentially
* Run across multiple nodes
* Fail independently

The workflow tracks every step.

---

## Distributed Execution

Execution is centralized in control, but distributed in action:

* The workflow engine coordinates execution
* Node agents perform the actual work
* Results flow back into the workflow

This creates:

* Parallel execution across nodes
* Consistent orchestration
* Clear visibility of progress

---

## State Transitions

Workflows are the **only mechanism** that changes system state.

They move the system across layers:

* From desired → installed
* From installed → validated
* From execution → completion

Every transition is:

* Explicit
* Recorded
* Traceable

---

## Idempotency and Safety

Workflows are designed to be:

* Repeatable
* Safe to retry
* Predictable in outcome

If a workflow is re-executed:

👉 It should not corrupt the system.

---

## Failure Handling

When a step fails:

* The workflow stops or marks the failure
* The state becomes explicit (FAILED or DEGRADED)
* The failure is localized to a step

There are no silent retries.

Recovery requires:

* Re-running the workflow
* Or executing a remediation workflow

---

## Types of Workflows

Globular uses workflows for all operations:

* **Bootstrap** → initialize a node or cluster
* **Apply** → deploy or update services
* **Repair** → fix inconsistencies
* **Publish** → make artifacts available

Each type follows the same execution model.

---

## Relationship with the System

Workflows connect all core concepts:

* They enforce the **source of truth**
* They implement the **convergence model**
* They move the **state model**

Without workflows:

👉 The system does not evolve.

---

## What Workflows Are Not

Workflows are not:

* Background reconciliation loops
* Implicit automation
* Hidden logic inside services

They do not “try forever”.

They execute, and they finish.

---

## Mental Model

Think of a workflow as a **transaction across the system**.

It:

* Starts with intent
* Applies controlled changes
* Produces a final, known state

Then it stops.

---

## One Sentence

Workflows are explicit, step-based executions that move the system from a desired state to a stable state, and are the only way the system changes.

