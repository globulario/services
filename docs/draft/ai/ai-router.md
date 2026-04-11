# AI Router

## Purpose

This document defines the role of the AI router in Globular.

The AI router is responsible for **analyzing system signals and influencing routing and execution decisions** in a controlled and observable way.

It enables adaptive behavior without introducing hidden automation.

---

## What Is the AI Router

The AI router is a **decision support component**.

It observes the system and produces:

* routing suggestions
* execution adjustments
* optimization signals

It does not directly control execution.

---

## Role in the System

The AI router sits between:

* observability
* system behavior (routing, execution decisions)

It enhances the system by:

* analyzing patterns
* detecting anomalies
* suggesting better choices

---

## What the AI Router Does

---

### Signal Analysis

The AI router consumes:

* metrics (latency, CPU, error rates)
* workflow outcomes
* historical execution data

It builds an understanding of system behavior over time.

---

### Routing Influence

The router can influence:

* service routing
* node selection
* load distribution

This is done by:

* adjusting weights
* suggesting routing changes
* influencing traffic distribution

---

### Execution Guidance

The router can suggest:

* preferred nodes for execution
* safer execution paths
* load-aware scheduling

These suggestions can be used by the system to improve efficiency.

---

### Anomaly Detection

The router detects patterns such as:

* abnormal latency
* repeated failures
* degraded performance

It can:

* raise signals
* assist diagnostics
* trigger analysis workflows

---

## What the AI Router Does NOT Do

The AI router is strictly constrained.

It does not:

* execute workflows
* modify system state directly
* bypass control plane decisions
* perform hidden automation

It does not override the system.

---

## Influence Model

The AI router operates through **influence, not control**.

* it produces recommendations
* the system applies them explicitly
* changes remain observable

All effects must be:

👉 explainable and traceable

---

## Relationship with MCP

The AI router is accessed through MCP.

* AI uses MCP tools
* MCP exposes router capabilities
* results are returned in structured form

---

## Relationship with Workflows

The AI router does not execute workflows.

It can:

* suggest workflows
* influence workflow parameters
* guide execution decisions

All actions still go through workflows.

---

## Relationship with Observability

The AI router depends entirely on observability.

Without:

* metrics
* events
* system signals

👉 it cannot function correctly

---

## Safety and Stability

The router must avoid instability.

It includes safeguards such as:

* smoothing of decisions
* avoidance of rapid oscillation
* controlled adaptation

This prevents:

* flapping
* unstable routing
* unpredictable behavior

---

## Key Property

The AI router ensures that:

👉 the system can adapt intelligently without losing control or transparency

---

## Architectural Boundary

A strict boundary exists:

* AI router = analysis and influence
* workflows = execution
* control plane = decision

The router must not become a hidden controller.

---

## Mental Model

Think of the AI router as an advisor.

* it observes the system
* suggests better paths
* helps optimize behavior

But the system still decides and executes.

---

## One Sentence

The AI router analyzes system signals and provides controlled, explainable influence over routing and execution decisions without directly modifying system behavior.

