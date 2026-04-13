# Convergence Model

## Purpose

This document explains the **execution model at the heart of Globular**.

It defines how the system moves from a desired state to a stable, known state using **explicit workflows instead of continuous reconciliation**.

---

## The Problem with Reconciliation

Most modern systems rely on a reconciliation model:

* The system continuously compares desired vs actual state
* Background loops attempt to “fix” differences
* Execution is implicit and ongoing

This leads to:

* Hidden behavior
* Non-deterministic outcomes
* Difficult debugging
* Systems that “eventually” converge — but never clearly finish

---

## The Globular Approach

Globular replaces reconciliation with **convergence through execution**.

> The system does not try forever.
> It executes once, explicitly, and reaches a known state.

---

## Core Principle

> A system only changes state when a workflow executes.

There are no background loops silently modifying the system.

All changes happen through:

* Defined workflows
* Ordered steps
* Observable transitions

---

## How Convergence Works

Convergence in Globular follows a clear sequence:

1. A **desired state** is defined
2. The system validates the request (admission)
3. A **workflow is executed**
4. Each step applies part of the change
5. Results are recorded
6. The system reaches a **stable terminal state**

Once completed, the system is no longer “in progress”.

It is either:

* **Available**
* **Degraded**
* **Failed**

---

## No Continuous Reconciliation

Unlike traditional systems:

* There is no loop constantly re-applying changes
* There is no hidden retry mechanism
* There is no silent drift correction

If something changes:

👉 A new workflow must be executed.

---

## Deterministic Execution

Because all changes are workflow-driven:

* The same workflow produces the same result
* Execution is reproducible
* Failures are traceable to specific steps

This makes the system:

* Predictable
* Testable
* Debuggable

---

## Explicit State Transitions

State does not “float” toward correctness.

It moves through **explicit transitions**:

* Defined by workflows
* Recorded at each step
* Visible to both humans and AI

There is no ambiguity about:

* What is happening
* Why it happened
* What step failed

---

## Relationship with the State Model

Convergence operates across the four system layers:

* **Artifact** → what is available
* **Desired Release** → what should exist
* **Installed State** → what is deployed
* **Runtime Health** → what is actually running

A workflow moves the system **across these layers** in a controlled way.

---

## Failure Handling

Failures are not hidden or retried indefinitely.

Instead:

* The workflow stops
* The system records the failure
* The state becomes explicit (FAILED or DEGRADED)

Recovery is performed by:

* Running a new workflow
* Or executing a remediation workflow

---

## Why This Matters

The convergence model changes how systems behave:

### Instead of:

* “The system will fix itself eventually”

### You get:

* “The system executed this, and here is the result”

---

## Benefits

* **No hidden behavior** → everything is explicit
* **Clear lifecycle** → operations have a beginning and an end
* **Reliable debugging** → failures are localized
* **AI-friendly** → actions and outcomes are structured and observable

---

## Mental Model

Think of Globular not as a system that constantly adjusts itself…

…but as a system that **performs precise operations and then stops**.

Each operation is:

* Defined
* Executed
* Completed

Then the system rests in a known state until the next action.

---

## One Sentence

Globular converges by **executing explicit workflows that move the system to a known, stable state — without relying on continuous reconciliation.**

