# Kubernetes Awareness — Substrate Boundaries

## The Boundary Line

```
┌──────────────────────────────────────────────────────┐
│                    KUBERNETES                         │
│  Pods  Deployments  Services  Namespaces  PVCs        │
│  kube-apiserver  scheduler  controller-manager        │
├────────────────────── BOUNDARY ──────────────────────┤
│                     GLOBULAR                          │
│  Nodes  Storage (MinIO/etcd/ScyllaDB)                 │
│  PKI/Certs  Network fabric  VIP/keepalived            │
└──────────────────────────────────────────────────────┘
```

---

## What Globular Owns Below the Boundary

| Resource | Globular responsibility |
|----------|------------------------|
| Node OS and kernel | Full |
| Node storage mounts | Full |
| etcd (Globular-internal) | Full |
| ScyllaDB | Full |
| MinIO (object storage) | Full |
| PKI / cluster CA | Full |
| VIP / keepalived | Full |
| Node-to-node networking | Full (if Globular manages CNI) |
| Kubernetes etcd | Diagnose only — Kubernetes owns this |
| PersistentVolumes backed by Globular storage | Shared |

---

## Forbidden Boundary Crossings

```yaml
# Forbidden: Globular modifying a Kubernetes workload object to "fix" a substrate issue
forbidden:
  - action: kubectl patch deployment <name>
    reason: Workload objects are not Globular's authority
    safe_alternative: Repair the substrate; let Kubernetes loops recover

# Forbidden: Deleting a pod to "clear" a substrate storage issue
forbidden:
  - action: kubectl delete pod <name>
    reason: Pod deletion doesn't fix a storage mount issue; it restarts the pod into the same broken substrate
    safe_alternative: Fix the storage mount first

# Forbidden: Assuming Kubernetes etcd health = Globular etcd health
forbidden:
  - assumption: Kubernetes etcd unreachable → Globular etcd is the cause
    reason: These are separate etcd instances on different ports
    safe_alternative: Check both independently
```

---

## Substrate Repair Sequence

When a substrate issue is causing a Kubernetes failure:

1. **Identify** the substrate component (MinIO, etcd, node health, cert)
2. **Verify** it is within Globular authority
3. **Repair** the substrate via Globular workflow
4. **Observe** Kubernetes control loops recover (do not force-restart pods)
5. **Verify** Kubernetes health after convergence window

**Wait for natural convergence before escalating to workload intervention.**
