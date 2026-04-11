# AI Architecture

## Purpose

This document describes how AI is integrated into the Globular architecture.

It defines the structure of the AI subsystem, how its components interact, and how AI operates without violating the system’s core principles.

---

## Position of AI in the System

AI exists as a **layer above the core system**.

It interacts with:

* workflows
* services
* observability
* system state

It does not replace or bypass any core component.

---

## AI Subsystem Overview

The AI subsystem is composed of four distinct components:

* AI Watcher
* AI Memory
* AI Router
* AI Executor

Together, they form a **closed-loop system**:

👉 observe → remember → reason → act

---

## AI Pipeline

The AI subsystem follows a structured pipeline:

1. **AI Watcher** collects system signals
2. **AI Memory** stores and organizes knowledge
3. **AI Router** analyzes and produces recommendations
4. **AI Executor** triggers actions through workflows

This loop continuously improves system understanding without introducing hidden behavior.

---

## Core Components

---

### AI Watcher

The AI watcher is the **observation layer**.

It monitors:

* workflows
* system state
* services
* metrics and events

It transforms raw signals into structured information.

It does not make decisions or trigger actions.

---

### AI Memory

The AI memory is the **knowledge layer**.

It stores:

* historical workflow outcomes
* system patterns
* anomaly history
* performance trends

It enables learning and pattern recognition.

It does not control the system.

---

### AI Router

The AI router is the **reasoning layer**.

It:

* analyzes current and historical data
* detects anomalies
* suggests routing and execution strategies

It produces recommendations.

It does not execute them.

---

### AI Executor

The AI executor is the **action layer**.

It is the only AI component allowed to trigger system changes.

It:

* receives recommendations or operator input
* initiates workflows
* ensures actions are explicit and traceable

All execution flows through workflows.

---

## Interaction Model

AI interacts with the system through MCP:

1. AI queries system state via MCP
2. MCP retrieves structured data from services
3. AI processes the information
4. AI may produce recommendations
5. AI executor triggers workflows

This ensures:

👉 no direct or hidden execution path exists

---

## Relationship with Core Architecture

AI integrates without modifying the system’s structure:

* cluster controller → remains decision layer
* workflow service → remains execution coordination
* node agents → remain execution endpoints
* repository → remains source of truth

AI operates alongside the system, not inside it.

---

## Boundaries

Strict boundaries must be enforced:

* only AI executor may trigger workflows
* no AI component may modify state directly
* no AI component may bypass workflows
* no hidden automation is allowed

All actions must remain:

👉 explicit, observable, and traceable

---

## Safety Model

The AI subsystem enforces:

* controlled execution
* validation through workflows
* auditable actions
* predictable behavior

AI must not introduce instability.

---

## Failure Isolation

If AI components fail:

* the core system continues to operate
* workflows remain functional
* system integrity is preserved

AI is additive, not required for correctness.

---

## Why This Architecture Works

AI is effective in Globular because:

* the system is explicit
* workflows define execution
* state is structured
* behavior is observable

This allows AI to:

* reason reliably
* explain decisions
* assist operators safely

---

## Key Property

The AI architecture ensures that:

👉 intelligence enhances the system without compromising determinism or transparency

---

## Mental Model

Think of the AI subsystem as an intelligent loop:

* it observes the system
* remembers its behavior
* reasons about improvements
* acts through workflows

The system remains:

* controlled
* explicit
* predictable

---

## One Sentence

The AI architecture in Globular is a structured loop of observation, memory, reasoning, and controlled execution that enhances system operation while preserving explicit workflows and system integrity.

