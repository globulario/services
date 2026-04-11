# Recovery

## Purpose

This document explains how Globular restores a system after failure.

Recovery is not an improvised process.
It is a **controlled reconstruction of the system from its preserved sources of truth**.

---

## Recovery Philosophy

Globular treats recovery as the inverse of deployment.

* Deployment creates the system from nothing
* Recovery rebuilds the system from preserved truth

The goal is not to “bring things back up”.
The goal is to restore the system to a **known, correct, and explainable state**.

---

## What Recovery Means

Recovery restores:

* system state
* service data
* cluster structure
* operational continuity

A successful recovery produces a system that:

* matches its intended state
* behaves predictably
* can continue normal operations

---

## The Foundation of Recovery

Recovery relies on two elements:

### Preserved Data

From backups:

* cluster state
* service data
* repository-related data

---

### System Model

From architecture:

* workflows define execution
* repository defines artifacts
* state model defines structure

Recovery is not guessing.
It is rebuilding from these foundations.

---

## Recovery Process

Recovery follows a structured sequence.

---

### 1. Re-establish the Base System

* Initialize a node (or nodes)
* Restore minimal control capabilities
* Recreate trust and identity (certificates, access)

This is similar to bootstrap, but guided by preserved data.

---

### 2. Restore Cluster State

* Restore authoritative state (e.g. etcd)
* Reconstruct desired state
* Restore workflow-related information if required

At this point:

👉 The system regains its memory.

---

### 3. Restore Service Data

* Restore persistent service data
* Ensure consistency with cluster state

Only authoritative data must be restored.
Rebuildable data can be regenerated later.

---

### 4. Reapply Desired State

* Workflows are executed
* Nodes converge toward the desired state
* Services are redeployed and validated

Recovery is not just loading data.
It is **re-executing the system into correctness**.

---

### 5. Validate Runtime Health

* Services are checked
* dependencies are verified
* cluster integrity is confirmed

The system must reach a stable, observable state.

---

## Node vs Cluster Recovery

Recovery can occur at different scopes.

---

### Node Recovery

* Restore a single node
* Rejoin it to the cluster
* Reapply its desired state

---

### Cluster Recovery

* Reconstruct the entire system
* Restore distributed state
* Rebuild all nodes

Cluster recovery requires careful sequencing to maintain consistency.

---

## Relationship with Backup

Recovery depends entirely on backup quality.

* Missing authoritative data → incomplete recovery
* Inconsistent backup → unstable system

Backup and recovery must be designed together.

---

## Relationship with Workflows

Recovery is driven by workflows.

* Restore prepares the system
* Workflows reapply desired state
* The system converges back to correctness

There is no hidden recovery logic outside workflows.

---

## Failure During Recovery

Recovery itself may fail.

When it does:

* Failures are explicit
* State remains observable
* Recovery can be retried

The system does not enter an undefined state.

---

## What Recovery Is NOT

Recovery is not:

* restarting services blindly
* copying files without structure
* relying on implicit system behavior

It is not an attempt to “make things work”.

It is a **disciplined reconstruction of the system**.

---

## Key Property

Recovery always produces a system that can be explained:

* what was restored
* what was executed
* what state was reached

If this cannot be explained:

👉 recovery is incomplete.

---

## Mental Model

Think of recovery as rebuilding a system from its blueprint and preserved materials.

* Backup provides the materials
* Workflows provide the construction steps
* The state model defines the structure

The result is a system that is rebuilt, not guessed.

---

## One Sentence

Recovery in Globular rebuilds the system from preserved state and artifacts, using workflows to converge it back to a correct and explainable operational state.

