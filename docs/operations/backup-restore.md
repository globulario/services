# Backup

## Purpose

This document explains how Globular approaches backup.

Backup in Globular is not just a file copy process. It is a controlled operation that preserves the data required to recover the system to a known, usable state.

The goal is not merely to save bytes.
The goal is to preserve **recoverability**.

---

## What Backup Means in Globular

A backup captures the system data required to restore:

* cluster state
* service data
* infrastructure data
* operational continuity

A valid backup must make it possible to recover the system without guessing what was important.

---

## Backup Philosophy

Globular treats backup as an operational responsibility with clear rules:

### Back Up What Is Authoritative

Data that defines the real state of the system must be preserved.

### Do Not Confuse Cache with State

Not everything on disk is worth saving.

### Recovery Matters More Than Archive

A backup is only valuable if it can be used to restore a working system.

---

## What Must Be Backed Up

Globular distinguishes several categories of data.

---

### Cluster State

This includes the information required to reconstruct the control state of the system, such as:

* desired state
* workflow-related state
* cluster coordination data
* authoritative configuration

This is foundational. Without it, the system loses its memory.

---

### Service Data

Services may own persistent data that must survive failure and rebuild.

Examples include:

* databases
* object storage metadata or content
* service-managed durable files

If the service holds authoritative user or system data, that data must be backed up.

---

### Repository Data

The repository defines what artifacts exist and what can be deployed.

Its contents are part of the platform’s operational truth and should be included in recovery planning.

---

## What May Be Rebuilt Instead of Backed Up

Some data does not need to be preserved because it can be regenerated safely.

This includes:

* caches
* derived indexes
* temporary materializations
* non-authoritative runtime artifacts

Saving this data may waste space and slow recovery without improving correctness.

---

## Backup Classes

Globular’s operational model benefits from separating data into three classes:

### Authoritative

Must be backed up.
This data is the real source of truth for recovery.

### Rebuildable

May be backed up, but can also be regenerated.
Recovery should not depend on it.

### Cache

Should not be treated as backup-critical.
It can be recreated after restore.

---

## Scope of a Backup

A complete backup may include:

* cluster control state
* service data
* repository-related data
* infrastructure-specific durable state

The exact scope depends on recovery goals.

A cluster-wide backup is not the same thing as a node-local snapshot.

---

## Node vs Cluster Perspective

Globular operates across nodes, so backup must be understood at two levels.

### Node-Level Perspective

A node may hold:

* installed service data
* local persistent files
* infrastructure data tied to that node

### Cluster-Level Perspective

The cluster backup must preserve the distributed system as a whole, including:

* shared state
* distributed data responsibilities
* multi-node recoverability

A backup strategy that only thinks node-by-node is incomplete.

---

## Consistency Matters

A backup must represent a meaningful state.

If data is copied in a way that breaks consistency between components, the result may be unusable even if every file exists.

Backup operations should preserve:

* structural consistency
* service recoverability
* cluster integrity

---

## Restore Is Part of Backup Design

A backup strategy is incomplete if restore is unclear.

Every backup design must answer:

* What can be restored?
* In what order?
* With which dependencies?
* To what operational state?

A backup that cannot be restored predictably is only storage.

---

## Operational Goal

The purpose of backup is to make it possible to recover:

* a node
* a service
* or the entire cluster

to a known and explainable state.

The recovery target is not “something that starts.”
It is a system that returns to controlled operation.

---

## Relationship with the State Model

Backup interacts with all important layers of the system:

* **Artifact** → what can be redeployed
* **Desired** → what the system should become
* **Installed** → what was present
* **Runtime** → what was happening at the moment of capture

Not every layer is backed up in the same way, but backup must preserve enough truth to reconstruct the system correctly.

---

## Relationship with Source of Truth

Backup must follow the same discipline as the rest of Globular:

* preserve authoritative data
* avoid inventing hidden state
* distinguish truth from observation

This is why backup is not just a filesystem operation.
It is a controlled preservation of the system’s real sources of truth.

---

## Failure Model

Backups exist for moments when normal operation is broken:

* node loss
* disk corruption
* service failure
* cluster-wide recovery scenarios

The backup system must therefore be designed for stressful reality, not ideal conditions.

---

## Mental Model

Think of backup as preserving the **minimum truth required to rebuild the system without lying**.

Not every byte matters equally.
What matters is preserving what the system needs in order to become correct again.

---

## One Sentence

Backup in Globular is the controlled preservation of authoritative system and service data required to restore a node or cluster to a known, recoverable state.

