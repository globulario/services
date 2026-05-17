# Kubernetes Awareness — Control Loops

## Key Control Loops

Kubernetes is a set of reconciliation loops. Failures are often loop stalls, not crashes.

| Loop | Controller | Common stall cause |
|------|-----------|-------------------|
| Node lifecycle | node-controller | Node not reporting heartbeat (substrate issue) |
| Pod scheduling | kube-scheduler | No node meets resource/affinity constraints |
| Pod lifecycle | kubelet | Image pull failure, OOMKill, CrashLoopBackOff |
| ReplicaSet | replicaset-controller | Pod creation fails (quota, PVC not bound) |
| PV binding | pv-controller | StorageClass provisioner unavailable |
| Namespace deletion | namespace-controller | Finalizer not cleared (workload issue) |

---

## Control Loop Failure Taxonomy

```
Loop stall
  ├── Substrate cause    → Globular can act (repair node, storage, network)
  ├── Workload cause     → Globular observes, surfaces to operator
  └── Unknown            → Collect evidence before acting
```

---

## Loop Convergence Invariant

**A Kubernetes control loop that is stalled but progressing is not failed.**

A stalled loop may be:
- Waiting for a dependency (PVC provisioning, image pull)
- Backoff-delaying after a transient failure
- Blocked by a substrate issue Globular can repair

Before escalating, check:
1. Is the stall duration beyond the expected convergence window?
2. Is there a substrate cause visible from Globular evidence?
3. Is there an actionable substrate repair within Globular authority?

---

## Danger: Acting on a Stalled Loop Without Evidence

Deleting a pod during a stall may not resolve the underlying issue and can:
- Cause data loss if the pod holds unsaved state
- Trigger cascading restarts across dependent services
- Mask a substrate issue that will recur

**Classification before action is mandatory.**
