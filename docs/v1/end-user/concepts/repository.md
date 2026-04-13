# Repository Service

## Purpose

This document defines the role of the repository service in Globular.

The repository service is responsible for **storing, validating, and serving artifacts used by the system**.

It is the runtime interface to the system’s artifact source of truth.

---

## What Is the Repository Service

The repository service is a **platform service** that manages artifacts.

It provides:

* storage for packages and versions
* access to artifacts for deployment
* validation of artifact integrity

It ensures that artifacts are:

* available
* consistent
* trusted

---

## Role in the System

The repository service sits between:

* artifact producers (publish workflows)
* artifact consumers (workflows and node agents)

It does not decide what should be used.

It ensures that what is used is **valid and accessible**.

---

## Core Responsibilities

---

### Artifact Storage

The repository service stores:

* packages
* versions
* builds
* metadata

Artifacts are versioned and immutable once published.

---

### Artifact Retrieval

The service allows:

* workflows to resolve artifacts
* node agents to download packages

All deployment depends on repository access.

---

### Integrity Verification

The repository service ensures:

* checksum validation
* artifact consistency
* rejection of conflicting versions

This prevents corrupted or inconsistent deployments.

---

### Metadata Management

Artifacts include metadata such as:

* name
* version
* type
* dependencies
* installation definition

This metadata allows workflows to use artifacts correctly.

---

## What the Repository Service Does NOT Do

The repository service is intentionally limited.

It does not:

* execute workflows
* decide deployment behavior
* perform installation
* modify system state

It only provides artifacts.

---

## Relationship with Workflows

Workflows interact with the repository to:

* resolve artifacts
* select versions
* retrieve deployment definitions

The repository does not initiate workflows.

---

## Relationship with Node Agents

Node agents use the repository to:

* download artifacts
* verify integrity
* access installation payloads

They do not modify repository content.

---

## Relationship with Cluster Controller

The cluster controller may:

* query artifact availability
* validate requested versions

It does not control repository data.

---

## Artifact Immutability

Artifacts are immutable.

* a version cannot change once published
* conflicting versions are rejected
* integrity is enforced

This ensures:

* reproducibility
* consistency across nodes
* safe deployments

---

## Availability

The repository service must be:

* highly available
* accessible from all nodes
* consistent across the cluster

Deployment depends on it.

---

## Failure Handling

If the repository service is unavailable:

* deployments cannot proceed
* workflows may fail at resolution or installation steps

Failures are explicit and visible.

---

## Key Property

The repository service guarantees that:

👉 every artifact used in the system is known, versioned, and verifiable

---

## Architectural Boundary

A strict boundary exists:

* repository service = artifact storage and access
* workflow service = execution coordination
* node agents = execution

The repository must never perform execution.

---

## Mental Model

Think of the repository service as a **vault with strict rules**.

* it stores blueprints
* it verifies their integrity
* it provides them when requested

It does not build anything itself.

---

## One Sentence

The repository service manages and serves immutable, versioned artifacts, ensuring that all deployments use trusted and consistent definitions without participating in execution.

