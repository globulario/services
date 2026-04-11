# Lifecycle

## Purpose

This document describes the lifecycle of a workflow in Globular.

It defines the phases a workflow goes through from initiation to completion, and how the system transitions between states during execution.

---

## The Nature of a Lifecycle

A workflow lifecycle is:

* Explicit
* Ordered
* Finite

Every workflow:

* starts from a defined point
* progresses through controlled phases
* ends in a terminal state

There is no continuous execution.

---

## Lifecycle Overview

A workflow moves through the following phases:

1. Admission
2. Initialization
3. Execution
4. Finalization
5. Terminal State

Each phase has a clear purpose and boundary.

---

## 1. Admission

The workflow is evaluated before it is allowed to run.

This includes:

* validating input
* checking dependencies
* enforcing policies

If validation fails:

👉 the workflow is rejected

No execution occurs.

---

## 2. Initialization

Once admitted:

* a workflow run is created
* initial metadata is recorded
* target nodes are resolved

The system prepares for execution.

---

## 3. Execution

The workflow engine executes steps:

* steps are processed in order
* actions are dispatched to node agents
* results are collected

Execution may include:

* sequential steps
* parallel execution across nodes

This is where the system changes.

---

## 4. Finalization

After execution:

* results are consolidated
* state transitions are completed
* final status is determined

No further actions are performed.

---

## 5. Terminal State

The workflow reaches a final state:

* AVAILABLE → successful execution
* DEGRADED → partial success
* FAILED → execution did not complete

This state is:

* explicit
* recorded
* stable

The workflow does not continue beyond this point.

---

## Phase Boundaries

Each phase is strictly separated:

* Admission does not execute
* Execution does not decide
* Finalization does not retry

This ensures:

* clarity
* predictability
* debuggability

---

## State Transitions

The lifecycle drives system state transitions.

Each phase contributes to moving the system across layers:

* Admission → validates desired state
* Execution → applies changes
* Finalization → resolves system state

There are no implicit transitions.

---

## Failure Within the Lifecycle

Failures can occur at different phases:

* Admission → request rejected
* Execution → step failure
* Finalization → incomplete resolution

In all cases:

* failure is explicit
* location is known
* system state is recorded

---

## Relationship with Execution Model

The lifecycle defines **when things happen**.

The execution model defines **how they happen**.

Together, they provide a complete view of workflow behavior.

---

## No Looping Behavior

A workflow does not loop.

It does not:

* re-evaluate continuously
* retry indefinitely
* correct itself in the background

If further action is needed:

👉 a new workflow is created

---

## Mental Model

Think of a workflow as a journey:

* it is accepted
* it is prepared
* it is executed
* it is completed
* it reaches a final state

Then it stops.

---

## One Sentence

The workflow lifecycle defines the ordered phases from admission to terminal state, ensuring that every operation in Globular progresses in a controlled, finite, and observable manner.

