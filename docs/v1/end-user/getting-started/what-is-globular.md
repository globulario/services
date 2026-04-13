# What Is Globular

## Purpose

This document defines what Globular is and how it works at a general level.

It provides the fundamental principles that guide the system’s design, behavior, and operation.

---

## Definition

Globular is a **workflow-driven convergence system**.

It manages infrastructure, services, and applications by:

* expressing intent explicitly
* executing changes through workflows
* maintaining a structured model of system state

---

## The Core Idea

Globular is built around a simple principle:

> The system does not continuously correct itself.
> It executes explicit operations to reach a correct state.

---

## How It Works

Globular operates through a structured flow:

1. **Intent is expressed**
2. **Intent is validated**
3. **A workflow is executed**
4. **Actions are performed on nodes**
5. **State is updated and recorded**

Each step is:

* explicit
* observable
* finite

---

## The System Model

Globular is composed of four essential layers:

---

### 1. Repository (What Exists)

Defines:

* artifacts
* versions
* deployment definitions

It is the source of what can be installed.

---

### 2. Desired State (What Should Exist)

Defines:

* which services should run
* where they should run
* how they should be configured

It represents system intent.

---

### 3. Installed State (What Is Installed)

Represents:

* what has been deployed
* how the system is configured

It reflects the result of execution.

---

### 4. Runtime State (What Is Happening)

Represents:

* service health
* system behavior
* real-time conditions

It reflects actual operation.

---

## Convergence Through Workflows

Globular does not rely on continuous reconciliation.

Instead:

* changes are executed through workflows
* workflows perform step-by-step operations
* each workflow ends in a terminal state

Convergence is achieved through:

👉 **explicit execution, not background correction**

---

## System Behavior

Globular is designed to be:

---

### Explicit

Nothing happens implicitly.

All actions are:

* defined
* executed
* recorded

---

### Deterministic

Given the same inputs:

👉 the same result is produced

---

### Observable

Every action and result is visible:

* workflows
* state transitions
* system signals

---

### Finite

Operations:

* start
* execute
* end

There are no infinite loops.

---

## Source of Truth

Globular enforces strict sources of truth:

* repository → defines artifacts
* workflows → define execution
* state → records results

There are no hidden state paths.

---

## Execution Model

Execution follows a strict separation:

* control plane → decides
* workflow engine → coordinates
* node agents → execute

This ensures:

* clarity
* scalability
* predictability

---

## Role of AI

AI in Globular:

* observes system behavior
* reasons about state and execution
* suggests or triggers workflows

AI does not bypass system rules.

---

## What Globular Is Not

Globular is not:

* a continuous reconciliation system
* a container-only platform
* a system with hidden orchestration

It does not rely on:

* implicit correction
* background loops
* opaque behavior

---

## Why This Matters

By making execution explicit and state structured:

* behavior becomes predictable
* failures become explainable
* operations become controlled

The system can always answer:

* what happened
* why it happened
* what to do next

---

## Mental Model

Think of Globular as a system that:

* receives intent
* executes it through defined steps
* reaches a known state
* stops

Then waits for the next action.

---

## One Sentence

Globular is a workflow-driven convergence system that transforms explicit intent into controlled, observable, and deterministic system state through finite execution.

