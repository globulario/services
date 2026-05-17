# Kubernetes Awareness — Remediation Policy

## Rule Zero

**Classify before acting. Classification is not optional.**

An unclassified failure must be treated as `unknown_insufficient_evidence`.
No destructive action may be taken on `unknown_insufficient_evidence`.

---

## Allowed Actions by Classification

### `substrate_owned`

Globular may:
- Repair the node (restart services, fix mounts, repair storage)
- Renew expired certificates (if Globular PKI)
- Repair network fabric
- Trigger Globular doctor remediation workflows
- Wait for Kubernetes control loops to self-recover after substrate repair

Globular must NOT:
- Delete or restart Kubernetes pods directly
- Modify Kubernetes Deployment/StatefulSet replicas
- Apply kubectl patches to workload objects

---

### `kubernetes_control_plane_owned`

Globular may:
- Diagnose and surface findings to the operator
- Repair substrate causes if they are the root cause
- Collect and present evidence

Globular must NOT:
- Restart kube-apiserver, scheduler, or controller-manager directly
- Modify Kubernetes control plane configuration

---

### `workload_owned`

Globular may:
- Surface findings to the workload operator
- Provide diagnostic context (logs, events, substrate health)

Globular must NOT:
- Modify workload configuration (ConfigMaps, Secrets, resource limits)
- Restart or delete workload pods
- Scale deployments

---

### `unknown_insufficient_evidence`

Globular must:
- Collect more evidence
- Surface the evidence gap to the operator

Globular must NOT:
- Take any destructive action
- Assume classification from incomplete evidence

---

## Tier 2 Actions (Require Human Approval)

The following actions require explicit operator approval regardless of classification:

- Draining a Kubernetes node
- Deleting any PersistentVolume or PersistentVolumeClaim
- Modifying StorageClass configuration
- Scaling a StatefulSet (data loss risk)
- Force-deleting a stuck namespace

---

## After Substrate Repair

After repairing a substrate issue, do NOT immediately force-restart Kubernetes workloads.

1. Wait for the Kubernetes convergence window (default: 5 minutes)
2. Observe control loops recovering naturally
3. Only escalate to pod intervention if the convergence window expires AND the substrate is confirmed healthy

**Premature pod intervention masks substrate issues and causes false "fixes."**
