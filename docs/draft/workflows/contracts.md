# Contracts

## Purpose

This document defines the rules that workflows and their steps must follow.

Contracts ensure that workflow execution remains:

* deterministic
* safe
* observable
* aligned with the system architecture

If these contracts are not respected, the system becomes inconsistent and unpredictable.

---

## The Role of Contracts

Workflows are the only mechanism that changes the system.

Contracts define **how they are allowed to do so**.

They enforce:

* correct behavior
* safe execution
* architectural boundaries

---

## Core Principle

> Every workflow step must be explicit, bounded, and aligned with the system's source of truth.

---

## Step Contracts

Each workflow step must satisfy the following conditions.

---

### Explicit Input

A step must define all required inputs.

* no hidden dependencies
* no implicit configuration
* no reliance on environment state

If a step depends on something:

👉 it must be declared.

---

### Explicit Output

A step must produce a clear result:

* success or failure
* execution details
* node-level outcomes

Results must be recorded and observable.

---

### Bounded Execution

A step must:

* start
* execute
* complete

It must not:

* run indefinitely
* loop internally
* perform uncontrolled retries

---

### Deterministic Behavior

Given the same inputs:

👉 a step must produce the same outcome

Behavior must not depend on:

* hidden state
* external side effects
* non-declared conditions

---

### Idempotency

A step must be safe to re-execute.

* repeated execution must not corrupt the system
* partial execution must be recoverable

This is required for:

* retries
* recovery
* consistency

---

## System Interaction Rules

Workflow steps interact with the system under strict constraints.

---

### No Direct State Mutation

Steps must not modify system state directly.

All state changes must occur through:

* workflow execution
* recorded transitions

---

### No Side Channels

Steps must not:

* modify environment variables as control mechanisms
* write hidden configuration outside declared paths
* introduce undocumented behavior

All changes must be visible and traceable.

---

### Repository Integrity

Steps must only use artifacts from the repository.

* no external downloads
* no runtime dependency injection
* no unverified binaries

If it is not in the repository:

👉 it cannot be used.

---

## Node Agent Interaction

Steps may request actions from node agents.

They must:

* send structured instructions
* expect structured responses
* not rely on implicit node behavior

Node agents must remain:

* execution-only
* stateless in decision-making

---

## Workflow-Level Contracts

Beyond individual steps, workflows must follow additional rules.

---

### No Hidden Logic

All execution logic must be defined in the workflow.

* no implicit branching in services
* no hidden orchestration
* no external control paths

---

### Finite Execution

A workflow must:

* reach a terminal state
* not run indefinitely

There must always be a clear end.

---

### Observable Progress

A workflow must expose:

* step progression
* execution status
* final outcome

There must be no invisible execution.

---

## Failure Contracts

Failures must follow strict rules.

---

### Explicit Failure

A failure must:

* be recorded
* be localized to a step
* be visible in workflow state

---

### No Silent Recovery

Steps must not:

* retry indefinitely
* mask failures
* attempt hidden correction

Recovery must be explicit through:

👉 a new workflow

---

## Invariants

The following must always hold:

* every change is caused by a workflow
* every step is observable
* every result is recorded
* every failure is explainable

If any of these are violated:

👉 the system is in an invalid state

---

## Why This Matters

Without contracts:

* behavior becomes inconsistent
* debugging becomes guesswork
* state becomes unreliable

With contracts:

* execution is predictable
* failures are traceable
* the system can be trusted

---

## Mental Model

Think of contracts as the **rules of physics** for workflow execution.

They are not suggestions.

They define what is possible.

---

## One Sentence

Contracts define the strict rules that ensure workflows execute safely, deterministically, and in alignment with the system’s source of truth, preventing hidden behavior and preserving system integrity.

