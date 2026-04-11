# Repository

## Purpose

This document defines the role of the repository in Globular.

The repository is responsible for **storing, validating, and serving all deployable artifacts**.

It defines what exists in the system.

---

## What Is the Repository

The repository is the **artifact authority** of Globular.

It stores:

* Packages
* Versions
* Builds
* Metadata
* Installation definitions

If something is not in the repository:

👉 It does not exist for the system.

---

## Role in the Architecture

The repository is not an execution component.

It does not:

* Decide what should run
* Execute workflows
* Modify system state

Its role is singular:

👉 Provide **trusted, immutable artifacts** that workflows can use.

---

## What Is an Artifact

An artifact in Globular is not just a binary.

It is a **complete deployment definition**.

It includes:

* Metadata (name, version, kind)
* Entry point
* Dependencies
* Installation steps
* Configuration templates
* Service definitions

---

## Example of an Artifact

A package such as Alertmanager defines:

* Its identity and version
* Required system structure (users, directories)
* How binaries are installed
* How configuration is written
* How services are created and started

This is not just a file.

👉 It is a **reproducible deployment unit** 

---

Another example, Envoy:

* Defines runtime dependencies (xDS)
* Declares capabilities (HTTP proxy)
* Installs systemd units
* Configures TLS paths
* Defines health checks

👉 The artifact fully describes how the service should exist on a node 

---

## Artifact Execution Model

Artifacts are not executed directly.

They are consumed by workflows.

The flow is:

1. Repository provides artifact
2. Workflow selects artifact
3. Node agent executes artifact steps
4. Installed state is updated

The repository never executes anything itself.

---

## Immutability

Artifacts are **versioned and immutable**.

* A version cannot change once published
* A checksum guarantees integrity
* Re-publishing with different content is rejected

This ensures:

* Reproducibility
* Consistency across nodes
* Safe upgrades

---

## Provenance and Trust

Each artifact carries:

* Source information
* Build metadata
* Integrity checks

This allows the system to verify:

* What was deployed
* Where it came from
* Whether it can be trusted

---

## Relationship with Workflows

The repository does not initiate change.

Workflows use it as input.

* Workflows select versions
* Workflows decide where to apply them
* Workflows drive installation

👉 The repository is passive but authoritative.

---

## Relationship with Node Agents

Node agents interact with the repository to:

* Download artifacts
* Verify checksums
* Access installation payloads

They do not modify the repository.

---

## Types of Artifacts

Globular supports different kinds of artifacts:

* **Infrastructure** (e.g. Envoy, Prometheus)
* **Services**
* **Applications**
* **Commands / tools**

Each type follows the same model:

👉 fully described, versioned, executable definitions

---

## What the Repository Is NOT

The repository is not:

* A runtime system
* A deployment engine
* A mutable configuration store

It does not contain:

* dynamic state
* execution logic
* orchestration behavior

---

## Architectural Boundary

A strict boundary exists:

* Repository = defines what exists
* Workflow = decides what to use
* Node agent = executes it

This boundary must never be broken.

---

## Why This Matters

Without a strict repository model:

* Deployments become inconsistent
* Nodes drift
* Debugging becomes unreliable

With it:

* Every deployment is reproducible
* Every version is traceable
* Every system state can be explained

---

## Mental Model

Think of the repository as a **vault of blueprints**.

Each artifact is a complete, versioned instruction set for building something on a node.

Workflows choose the blueprint.
Node agents build it.

---

## One Sentence

The repository is the immutable source of all deployable artifacts, defining exactly what can be installed and how, without ever executing anything itself.

