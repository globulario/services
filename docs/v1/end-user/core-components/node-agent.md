# Cluster Doctor

## Purpose

This document defines the role of the cluster doctor in Globular.

The cluster doctor is responsible for **analyzing system state, detecting inconsistencies, and guiding remediation through structured actions**.

It provides a systematic way to understand and correct problems in the cluster.

---

## What Is the Cluster Doctor

The cluster doctor is a **diagnostic and remediation service**.

It observes the system and produces:

* findings
* explanations
* recommended actions

It does not guess.

It reasons from:

* system state
* workflow execution
* observable signals

---

## Role in the System

The cluster doctor operates as an **analysis layer**.

It sits on top of:

* workflows
* state model
* observability

It answers:

* what is wrong
* why it is wrong
* what can be done

---

## Core Responsibilities

---

### State Analysis

The cluster doctor examines:

* desired vs installed state
* installed vs runtime state
* workflow outcomes

It identifies mismatches and inconsistencies.

---

### Finding Generation

When an issue is detected, the doctor produces a **finding**.

A finding includes:

* a description of the issue
* the affected components
* the severity
* the root cause (when identifiable)

Findings are structured and explicit.

---

### Explanation

The doctor explains problems in terms of the system model:

* which workflow failed
* which step caused the issue
* which state layer is inconsistent

There is no opaque diagnosis.

---

### Remediation Guidance

For each finding, the doctor can:

* suggest actions
* propose remediation workflows
* indicate whether automatic execution is safe

Remediation is always explicit.

---

### Remediation Execution

When allowed, the doctor can trigger:

* structured remediation workflows
* controlled corrective actions

It does not directly modify the system outside workflows.

---

## What the Cluster Doctor Does NOT Do

The cluster doctor is intentionally constrained.

It does not:

* perform hidden fixes
* modify system state directly
* override workflows
* act without producing a finding

All actions must be:

👉 explicit, structured, and traceable

---

## Relationship with Workflows

The cluster doctor relies on workflows for:

* understanding system behavior
* identifying failure points
* executing remediation

It does not bypass workflows.

---

## Relationship with State Model

The doctor uses the state model to detect issues:

* Desired ≠ Installed → deployment problem
* Installed ≠ Runtime → runtime issue

This structured view makes diagnosis reliable.

---

## Relationship with Observability

The doctor uses:

* metrics
* events
* health signals

to validate its analysis and confirm system behavior.

---

## Failure Handling

If the doctor cannot resolve an issue:

* the finding remains
* the system state is unchanged
* further action must be taken manually or through new workflows

The doctor never hides failure.

---

## Key Property

The cluster doctor ensures that:

👉 problems are explained before they are fixed

---

## Architectural Boundary

A strict boundary exists:

* cluster doctor = analysis and guidance
* workflow service = execution
* node agents = action

The doctor must not perform direct execution.

---

## Mental Model

Think of the cluster doctor as a system auditor.

* it inspects the system
* identifies inconsistencies
* explains their causes
* proposes structured fixes

It does not act blindly.

---

## One Sentence

The cluster doctor analyzes system state and workflow execution to produce explicit findings and guide remediation through controlled workflows, without performing hidden corrections.

