# Globular in 60 Seconds

Globular is a system that executes **workflows across nodes** to move infrastructure and applications toward a desired state.

---

## Core Components

### Control Plane

Validates intent and triggers workflows.

### Workflow Engine

Executes workflows and tracks progress.

### Node Agent

Runs actions on each node (install, configure, execute).

### Repository

Stores artifacts, versions, and deployment data.

### AI Layer

Observes system behavior and assists operations.

---

## How It Works

At a high level:

1. A **desired state** is defined
2. The control plane **validates the request**
3. A **workflow is executed**
4. Node agents perform actions
5. Results are reported
6. The system reaches a **stable state**

---

## The State Model

Globular separates system state into four layers:

* **Artifact** → what exists (packages, versions)
* **Desired Release** → what should run
* **Installed State** → what is deployed
* **Runtime Health** → what is actually happening

---

## Key Properties

* **Explicit execution** → no hidden behavior
* **Deterministic** → same input produces the same result
* **Observable** → every step is traceable
* **Composable** → workflows define behavior

---

## One Sentence Summary

Globular executes workflows to move a system from a desired state to a stable, observable reality.

