# Services Overview

## Purpose

This document defines what services are in Globular and how they fit into the system.

Services are the **functional units that provide capabilities**, run workloads, and implement system behavior.

---

## What Is a Service

A service in Globular is a **runtime component** that:

* provides a specific capability
* runs on one or more nodes
* is installed and managed through workflows
* is defined by repository artifacts

Services are how the system **does useful work**.

---

## Role in the System

Services sit at the intersection of:

* architecture → they are deployed components
* workflows → they are installed and managed
* operations → they are monitored and maintained

They represent the **active layer of the system**.

---

## How Services Are Created

Services are defined as artifacts in the repository.

Each service includes:

* metadata (name, version, type)
* installation logic
* configuration templates
* runtime definition

They are not manually installed.

👉 They are deployed through workflows.

---

## How Services Are Deployed

Service deployment follows the system model:

1. Desired state defines which service should run
2. A workflow is triggered
3. The workflow installs and configures the service
4. The node agent executes the required steps
5. The service becomes part of the installed state

---

## Service Execution

Once deployed, services:

* run on nodes
* expose functionality (APIs, processing, storage)
* interact with other services

Their runtime behavior is:

* observable
* monitored
* separate from deployment logic

---

## Types of Services

Globular supports different types of services.

---

### Infrastructure Services

Provide system-level capabilities.

Examples:

* proxy / routing
* storage systems
* monitoring

These services support the platform itself.

---

### Platform Services

Provide core system functionality.

Examples:

* repository
* workflow engine
* control plane services

These services define how the system operates.

---

### Application Services

Provide user-facing functionality.

Examples:

* business logic
* APIs
* data processing

These are the services that deliver value to users.

---

## Service Lifecycle

Services follow a lifecycle aligned with workflows:

* installed
* configured
* started
* validated
* updated
* removed

All transitions are driven by workflows.

---

## Relationship with Workflows

Services are:

* installed by workflows
* updated by workflows
* repaired by workflows

There is no service lifecycle outside workflow execution.

---

## Relationship with the Repository

Services originate from repository artifacts.

* versioned
* immutable
* fully defined

If a service is not in the repository:

👉 it cannot exist in the system.

---

## Relationship with Node Agents

Node agents execute service-related actions:

* installation
* configuration
* process management

Services do not manage themselves.

---

## Observability

Services are observable through:

* health checks
* metrics
* logs
* events

This allows:

* monitoring
* troubleshooting
* AI-assisted diagnostics

---

## Key Property

A service in Globular is not just a running process.

It is:

👉 a **defined, versioned, and workflow-managed runtime component**

---

## What Services Are NOT

Services are not:

* manually managed processes
* ad-hoc scripts
* implicit system components

They do not exist outside the system model.

---

## Mental Model

Think of services as **living components built from blueprints**.

* the repository provides the blueprint
* workflows build and update them
* node agents run them
* the system observes them

---

## One Sentence

Services in Globular are versioned, workflow-managed runtime components that provide system and application functionality across nodes.

