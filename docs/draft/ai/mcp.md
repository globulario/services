# MCP (Model Control Plane)

## Purpose

This document defines how AI interacts with Globular through the Model Control Plane (MCP).

MCP provides a structured interface that allows AI to **observe, query, and act on the system** in a controlled and safe manner.

---

## What Is MCP

MCP is an **interface layer between AI and Globular**.

It exposes the system through:

* structured tools
* typed APIs
* controlled operations

It allows AI to:

* query system state
* analyze behavior
* trigger workflows

---

## Role in the System

MCP acts as the **gateway for AI interaction**.

It ensures that:

* AI only sees structured, meaningful data
* AI actions are controlled and validated
* system integrity is preserved

AI does not interact with the system directly.

👉 It goes through MCP.

---

## Core Responsibilities

---

### Structured Access

MCP provides structured access to:

* cluster state
* workflows
* services
* observability data

This avoids:

* raw log parsing
* ambiguous data
* incomplete context

---

### Tool-Based Interaction

MCP exposes capabilities as **tools**.

Examples:

* get cluster status
* inspect node state
* analyze workflow execution
* retrieve service information

Each tool:

* has a defined input
* returns a structured result

---

### Controlled Actions

AI can perform actions through MCP:

* trigger workflows
* initiate diagnostics
* execute remediation

All actions are:

* validated
* explicit
* traceable

---

### Context Preservation

MCP maintains:

* context across interactions
* consistent system view
* structured responses

This allows AI to:

* reason across multiple steps
* maintain operational continuity

---

## What MCP Does NOT Do

MCP is not:

* an execution engine
* a source of truth
* a hidden automation layer

It does not:

* modify system state directly
* bypass workflows
* introduce implicit behavior

---

## Relationship with Workflows

All actions performed through MCP result in:

👉 workflow execution

MCP does not execute steps itself.

---

## Relationship with Services

MCP interacts with services by:

* calling APIs
* retrieving structured data
* invoking defined operations

It does not access services arbitrarily.

---

## Relationship with Observability

MCP exposes:

* metrics
* events
* system signals

in a structured way that AI can understand.

---

## Safety Model

MCP enforces strict safety:

* all actions are validated
* permissions can be enforced
* operations are auditable

There is no hidden execution path.

---

## Key Property

MCP guarantees that:

👉 AI interacts with the system through explicit, structured, and controlled interfaces

---

## Architectural Boundary

A strict boundary exists:

* MCP = interface layer
* workflows = execution
* system services = behavior

MCP must not perform execution.

---

## Mental Model

Think of MCP as a control console for AI.

* it exposes the system clearly
* it provides tools to interact
* it ensures safe operation

AI does not “reach into” the system.

It operates through MCP.

---

## One Sentence

MCP is the structured interface that allows AI to observe, reason about, and act on the system through controlled tools and workflow-triggered operations.

