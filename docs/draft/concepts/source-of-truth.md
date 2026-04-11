# Source of Truth

## Purpose

This document defines what **actually controls the state of the system**.

It also defines what **must never be used** to control it.

If these rules are not respected, the system becomes inconsistent, unpredictable, and unsafe.

---

## The Rule

> The system state is only valid if it is derived from its declared sources of truth.

Everything else is noise.

---

## Primary Sources of Truth

Globular has a small number of **authoritative sources**.

These are the only places where system state is defined and allowed to change.

---

### Workflows

Workflows are the **execution source of truth**.

* They define *what happens*
* They control *when it happens*
* They produce *all state transitions*

If a change did not happen through a workflow:

👉 It is not part of the system.

---

### Repository

The repository is the **artifact source of truth**.

* Stores packages and versions
* Defines what can be deployed
* Guarantees integrity (checksum, provenance)

If something is not in the repository:

👉 It does not exist for the system.

---

### etcd

etcd is the **state storage source of truth**.

* Stores desired state
* Stores observed state
* Stores workflow progress

However:

etcd does not define behavior —
it only records it.

---

## Derived State (Not a Source of Truth)

Some parts of the system reflect reality but do not define it:

* Runtime health
* Service status
* Metrics

These are **observations**, not decisions.

They must never be used as the origin of changes.

---

## What Is NOT a Source of Truth

This is where most systems fail.

Globular explicitly rejects the following as sources of truth:

---

### Environment Variables

* Not versioned
* Not visible
* Not consistent across nodes

Using them to control behavior leads to **configuration drift**.

---

### Local State on Nodes

* Manual changes
* Temporary files
* Ad-hoc modifications

If it is not declared and executed through workflows:

👉 It is not real.

---

### Implicit Logic in Code

* Hidden defaults
* Conditional behavior not expressed in workflows
* Side effects

If behavior is not visible in a workflow:

👉 It is invisible to the system.

---

## The Invariant

> Every state change must be explainable as the result of a workflow execution using repository artifacts and recorded in etcd.

If this cannot be explained:

👉 The system is in an invalid state.

---

## Why This Matters

Without strict sources of truth:

* Systems drift
* Behavior becomes unpredictable
* Debugging becomes guesswork
* AI cannot reason about the system

With strict sources of truth:

* Every change is traceable
* Every failure is explainable
* The system can be trusted

---

## Common Failure Pattern

A typical failure looks like this:

1. A configuration is changed outside a workflow
2. The system state diverges from etcd
3. Future workflows operate on incorrect assumptions
4. Errors cascade

The root cause is always the same:

👉 A source of truth was bypassed.

---

## Design Consequence

Globular enforces a strict discipline:

* No hidden state
* No implicit behavior
* No side-channel configuration

Everything must go through:

👉 repository → workflow → execution → state

---

## Mental Model

Think of the system as a ledger.

Only transactions recorded through workflows are valid.

Everything else is corruption.

---

## One Sentence

Globular only trusts what is declared in the repository, executed by workflows, and recorded in etcd — everything else is ignored or considered drift.

