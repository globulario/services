# Kubernetes Awareness — Object Model

## Object Hierarchy (Globular's view)

```
Cluster
└── Node (substrate boundary — Globular owns below this line)
    ├── kubelet
    ├── kube-proxy
    └── CNI plugin

Namespace
└── Workload objects (Deployment, StatefulSet, DaemonSet, Job, CronJob)
    └── Pod
        ├── Container (image, resources, env, volumes)
        └── Volume
            └── PVC → PV → StorageClass → StorageBackend

Control Plane (Kubernetes owns)
├── kube-apiserver
├── kube-controller-manager
├── kube-scheduler
└── etcd (Kubernetes-internal, separate from Globular etcd)
```

---

## Ownership Rules

| Object | Owner | Globular can... |
|--------|-------|----------------|
| Node | Globular substrate | Repair OS, storage, networking |
| PersistentVolume | Depends on StorageClass | Diagnose; repair if Globular-backed storage |
| Pod | Workload operator | Observe; never delete/restart directly |
| Deployment/StatefulSet | Workload operator | Read only |
| kube-apiserver | Kubernetes control plane | Diagnose connectivity; repair substrate causes |
| Namespace | Kubernetes control plane | Observe stuck termination; surface to operator |

---

## Identity Fields Awareness Uses

When tracking Kubernetes objects, Awareness uses:

```yaml
node_name: string          # Kubernetes node name (NOT Globular node ID)
node_ip: string            # Matches to Globular node IP (stable, not VIP)
pod_uid: string            # Pod UID for lifecycle tracking
namespace/name: string     # Standard Kubernetes object identity
```

**Warning**: Kubernetes node names may not match Globular node IDs. Always resolve via IP.

---

## Anti-Pattern: Assuming Globular Node = Kubernetes Node

Globular nodes and Kubernetes nodes share the same IP but may have different names.
Never assume `globular_node_id == kubernetes_node_name`.
Always resolve via stable IP using `StableIP(clusterVIP)` filtering.
