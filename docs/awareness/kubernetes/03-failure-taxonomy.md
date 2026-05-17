# Kubernetes Awareness — Failure Taxonomy

## Primary Classification

Every Kubernetes failure is classified into one of four buckets before any action is taken.

### `workload_owned`
The failure originates from workload configuration, not infrastructure.

Examples:
- Image pull failure (wrong tag, private registry auth)
- OOMKilled (memory limit too low)
- CrashLoopBackOff (application crash — check logs)
- Liveness probe failing (application health endpoint broken)
- ConfigMap/Secret missing (workload misconfigured)
- Resource quota exceeded (namespace limits)

**Globular action**: Observe and surface. Do not modify workload objects.

---

### `kubernetes_control_plane_owned`
The failure is in Kubernetes internals, not workload or substrate.

Examples:
- kube-apiserver unreachable (cert expiry → may be substrate)
- etcd quorum lost (Kubernetes-internal etcd, not Globular etcd)
- Admission webhook unavailable
- Namespace stuck in Terminating (finalizer issue)

**Globular action**: Diagnose. Repair only if the root cause is substrate (cert expiry, node health). Otherwise surface to operator.

---

### `substrate_owned`
The failure is caused by a Globular-managed substrate issue.

Examples:
- Node NotReady because kubelet lost storage mount (MinIO/etcd mount failure)
- PVC not bound because MinIO/NFS provisioner is down
- Node unreachable because networking failed (Globular manages the network fabric)
- Certificate expiry on the cluster CA (Globular PKI owns this)

**Globular action**: Repair the substrate. After substrate repair, Kubernetes loops self-recover.

---

### `unknown_insufficient_evidence`
The failure cannot be classified from available evidence.

**Globular action**: Collect more evidence. Do NOT act destructively.

Evidence to collect:
- Node health from Globular perspective
- Substrate service status (etcd, ScyllaDB, MinIO)
- Recent convergence events
- Doctor findings

---

## Classification Decision Tree

```
Kubernetes failure detected
│
├── Is the node NotReady?
│   ├── YES → check Globular node health
│   │         ├── Substrate issue → substrate_owned
│   │         └── No substrate issue → kubernetes_control_plane_owned or workload_owned
│   └── NO  → continue
│
├── Is the failure storage-related?
│   ├── Is the storage provider Globular-managed? → substrate_owned
│   └── No → workload_owned or kubernetes_control_plane_owned
│
├── Is the failure cert/TLS-related?
│   ├── Is the cert in Globular PKI? → substrate_owned
│   └── Is it a workload cert? → workload_owned
│
└── No clear evidence → unknown_insufficient_evidence
```
