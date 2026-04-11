# AI Overview

## Purpose

This document defines the role of AI in Globular.

AI in Globular is designed to **observe, reason about, and assist in operating the system**, using the same explicit models that define its behavior.

It is not an external tool.

It is an integrated operational layer.

---

## What AI Is in Globular

AI is an **operator layer** that interacts with the system through:

* workflows
* state model
* observability
* structured APIs

It can:

* analyze system state
* explain behavior
* suggest actions
* trigger workflows

---

## What AI Is NOT

AI is not:

* a hidden automation engine
* a background controller
* a source of truth

It does not:

* modify the system implicitly
* bypass workflows
* act outside defined interfaces

---

## Role in the System

AI sits on top of the system and uses its structure.

It observes:

* workflow execution
* system state
* service behavior

It acts through:

* workflow initiation
* diagnostic tools
* structured interfaces

---

## Why AI Works in Globular

AI is effective because Globular is:

* explicit
* structured
* observable
* deterministic

The system exposes:

* what is happening
* why it is happening
* what changed

This allows AI to reason reliably.

---

## AI Capabilities

AI in Globular can:

---

### System Understanding

* analyze workflows
* interpret system state
* detect inconsistencies

---

### Diagnostics

* identify failure causes
* explain system behavior
* correlate events and state

---

### Guidance

* suggest corrective actions
* propose workflows
* assist operators

---

### Controlled Execution

AI can initiate actions by:

* triggering workflows
* interacting with services
* using defined APIs

All actions remain:

👉 explicit and traceable

---

## Relationship with Workflows

AI does not execute actions directly.

It:

* triggers workflows
* observes execution
* analyzes results

Workflows remain the only execution mechanism.

---

## Relationship with Source of Truth

AI respects the system’s sources of truth:

* repository
* workflows
* recorded state

It does not introduce new state.

---

## Relationship with Observability

AI relies on:

* metrics
* events
* logs
* state transitions

It does not infer blindly.

It reasons from explicit signals.

---

## Boundaries

AI must respect strict boundaries:

* no hidden execution
* no implicit corrections
* no direct state mutation

All actions must flow through:

👉 workflow execution

---

## Key Property

AI in Globular does not guess.

👉 It reasons from a system that explains itself.

---

## Mental Model

Think of AI as an operator that:

* reads the system
* understands its behavior
* suggests or triggers actions

It does not replace the system.

It works with it.

---

## One Sentence

AI in Globular is an integrated operational layer that observes system state, reasons about behavior, and assists in execution through workflows, without introducing hidden automation or bypassing system rules.

