# Examples

## Purpose

This document provides concrete examples of workflows in Globular.

It shows how workflows are used to perform real operations, and how the concepts described in previous documents translate into execution.

---

## What Examples Show

Each example illustrates:

* the intent of the workflow
* the sequence of steps
* how execution is distributed
* how state transitions occur

The goal is not to show every detail, but to make workflows **intuitive and concrete**.

---

## Example 1: Node Bootstrap

### Intent

Initialize a node and bring it to a usable state.

---

### What Happens

1. The node is registered
2. Core infrastructure is prepared
3. Required services are installed
4. The node becomes ready

---

### Simplified Flow

```yaml
workflow: node.bootstrap

steps:
  - prepare_environment
  - install_core_services
  - configure_system
  - validate_node
```

---

### Key Properties

* executed once per node
* establishes initial system capability
* produces a READY node state

---

## Example 2: Service Deployment

### Intent

Deploy or update a service across one or more nodes.

---

### What Happens

1. Desired state is defined
2. Artifacts are selected from the repository
3. A workflow is executed
4. Node agents install and configure the service
5. The service is started and validated

---

### Simplified Flow

```yaml
workflow: release.apply

steps:
  - resolve_artifact
  - distribute_package
  - install_service
  - start_service
  - verify_health
```

---

### Key Properties

* driven by desired state
* uses repository artifacts
* results in an updated installed state

---

## Example 3: Repair Workflow

### Intent

Fix inconsistencies or failures in the system.

---

### What Happens

1. A problem is detected
2. The system evaluates possible actions
3. A remediation workflow is executed
4. The issue is resolved or escalated

---

### Simplified Flow

```yaml
workflow: repair

steps:
  - detect_issue
  - assess_state
  - execute_fix
  - verify_resolution
```

---

### Key Properties

* reactive but still explicit
* no hidden correction
* produces observable recovery

---

## Example 4: Artifact Publish

### Intent

Make a new artifact available in the repository.

---

### What Happens

1. Artifact is uploaded
2. Metadata is validated
3. Integrity is verified
4. Artifact becomes available for deployment

---

### Simplified Flow

```yaml
workflow: publish

steps:
  - validate_metadata
  - verify_checksum
  - store_artifact
  - mark_available
```

---

### Key Properties

* ensures repository integrity
* enforces immutability
* prepares artifacts for deployment

---

## What These Examples Show

Across all workflows:

* execution is step-based
* actions are explicit
* results are recorded
* state transitions are controlled

There is no hidden behavior.

---

## Relationship with the System

These examples demonstrate how workflows connect:

* **repository** → provides artifacts
* **control plane** → initiates execution
* **workflow engine** → coordinates steps
* **node agents** → execute actions
* **state model** → reflects results

---

## Mental Model

Each workflow is a **clear sequence of actions**:

* it starts with intent
* executes defined steps
* produces a result
* ends in a known state

The examples show that:

👉 the system behaves exactly as it is described

---

## One Sentence

Examples demonstrate how Globular workflows translate intent into explicit, step-based execution that produces controlled and observable system state transitions.

