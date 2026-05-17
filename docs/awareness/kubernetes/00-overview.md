# Kubernetes Awareness — Overview

## Core Principle

**Kubernetes is workload truth. Globular is substrate truth.**

Globular may:
- Observe Kubernetes cluster state
- Diagnose Kubernetes failures
- Repair substrate causes *beneath* Kubernetes (storage, networking, node health)

Globular must **not**:
- Blindly mutate Kubernetes workload objects before classifying the authority boundary
- Treat a Kubernetes failure as substrate failure without evidence
- Assume a substrate fix resolves a workload issue

---

## Authority Boundary

```
┌─────────────────────────────────────┐
│  WORKLOAD LAYER (Kubernetes owns)    │
│  Deployments, Pods, Services,        │
│  ConfigMaps, PVCs, Namespaces        │
└──────────────┬──────────────────────┘
               │ observes / diagnoses
               ▼
┌─────────────────────────────────────┐
│  SUBSTRATE LAYER (Globular owns)     │
│  Nodes, Storage, Networking,         │
│  etcd, ScyllaDB, MinIO, PKI          │
└─────────────────────────────────────┘
```

---

## Failure Classification

Every Kubernetes failure must be classified before action:

| Class | Meaning | Who acts |
|-------|---------|----------|
| `workload_owned` | Pod config, image, resource limits | Workload operator |
| `kubernetes_control_plane_owned` | kube-apiserver, scheduler, controller-manager | Platform operator |
| `substrate_owned` | Node crash, disk failure, network partition, cert expiry | Globular |
| `unknown_insufficient_evidence` | Cannot classify without more data | Observe, do not act |

**Never act on `unknown_insufficient_evidence`.** Collect more evidence first.

---

## Files in This Section

| File | Contents |
|------|---------|
| `01-object-model.md` | Kubernetes object hierarchy and ownership |
| `02-control-loops.md` | Control plane loops and failure modes |
| `03-failure-taxonomy.md` | Classification of Kubernetes failures |
| `04-substrate-boundaries.md` | Where Globular authority begins and ends |
| `05-observation-sources.md` | How Globular observes Kubernetes state |
| `06-invariants.md` | Required invariants for safe K8s operation |
| `07-remediation-policy.md` | What actions are allowed per failure class |
| `08-knowledge-ingestion.md` | Adding new K8s knowledge to awareness |
