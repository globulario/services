# Troubleshooting

## Purpose

This document explains how to understand and diagnose problems in Globular.

Troubleshooting in Globular is not based on guessing or trial and error.
It is based on **observing explicit state, workflow execution, and system signals**.

---

## Troubleshooting Philosophy

Globular is designed so that failures are:

* visible
* localized
* explainable

The system does not hide problems.

It exposes them.

---

## The Core Principle

> Every failure must be explainable as part of a workflow execution or a state mismatch.

If something cannot be explained:

👉 either the system is broken, or the source of truth has been bypassed.

---

## Where to Look First

Troubleshooting always starts from the same places.

---

### Workflow State

Workflows tell you:

* what was supposed to happen
* what step was executed
* where it failed

A failed system operation is always reflected in a workflow.

---

### Step Results

Each workflow step provides:

* success or failure
* execution details
* node-level results

Failures are not global.

They are tied to specific steps.

---

### Node-Level Execution

If a step fails:

* check the node agent involved
* inspect execution logs
* verify system conditions on that node

Execution problems are local before they are global.

---

### State Model

Compare system layers:

* Desired vs Installed
* Installed vs Runtime

Mismatch between layers reveals the nature of the problem.

---

### Observability Signals

Use system signals to understand behavior:

* metrics
* events
* service health
* resource usage

These help confirm what the system is actually doing.

---

## Common Failure Patterns

Most issues fall into a small number of patterns.

---

### Desired ≠ Installed

The system has not successfully applied a change.

Cause:

* workflow failure
* missing dependency
* execution error

---

### Installed ≠ Runtime

The system is deployed, but not functioning correctly.

Cause:

* service crash
* misconfiguration
* resource constraint

---

### Workflow Failure

A workflow step failed during execution.

Cause:

* invalid artifact
* system-level issue
* node-specific problem

---

### Drift from Source of Truth

Something changed outside workflows.

Cause:

* manual modification
* environment-based configuration
* hidden state

This is one of the most serious issues.

---

## What to Avoid

Troubleshooting in Globular should not involve:

* blind restarts
* manual fixes without workflows
* guessing based on symptoms
* modifying system state directly

These actions break system consistency.

---

## Correct Troubleshooting Approach

Follow a structured process:

1. Identify the failed workflow
2. Locate the failing step
3. Inspect node-level execution
4. Compare system state layers
5. Confirm behavior using observability

Then:

👉 fix the cause and re-execute the workflow

---

## Relationship with Workflows

Workflows are the primary debugging tool.

They provide:

* execution history
* failure location
* system intent

Without workflows, troubleshooting becomes guesswork.

---

## Relationship with Source of Truth

If behavior cannot be explained through:

* workflows
* repository artifacts
* recorded state

Then the system is in an invalid condition.

The root cause is usually:

👉 a bypass of the source of truth

---

## Recovery vs Troubleshooting

* Troubleshooting identifies the problem
* Recovery restores the system

They are related but distinct.

Troubleshooting must always precede recovery.

---

## Key Property

Globular does not require intuition to debug.

It provides enough structure that:

👉 every failure can be traced to a specific cause

---

## Mental Model

Think of troubleshooting as reading a log of decisions and actions:

* what was requested
* what was executed
* what failed
* what state resulted

The system tells the story.

You just follow it.

---

## One Sentence

Troubleshooting in Globular is the process of analyzing workflows, state transitions, and system signals to identify and correct the exact cause of a failure without relying on guesswork.

