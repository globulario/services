# Kubernetes Awareness — Observation Sources

## How Globular Observes Kubernetes

Globular uses read-only observation. It does not modify Kubernetes objects to observe them.

| Source | What it reveals | Globular layer |
|--------|----------------|----------------|
| kube-apiserver (read) | Node/Pod/PVC/Namespace status | Runtime |
| kubelet (node) | Pod lifecycle on a specific node | Runtime |
| Prometheus metrics | Resource usage, restart counts, error rates | Runtime |
| Globular node health | Substrate health under K8s nodes | Runtime |
| Globular doctor findings | Cross-layer invariant violations | Runtime |

---

## Evidence Collection Priority

When diagnosing a Kubernetes failure, collect in this order:

1. **Node health** (Globular layer) — is the substrate healthy?
2. **Kubernetes node status** (K8s layer) — is the node Ready?
3. **Pod/workload status** — CrashLoopBackOff, Pending, NotReady?
4. **Events** — recent K8s events for the affected resource
5. **Logs** — only after classification, not as a first step

---

## What Globular Should NOT Use as Evidence

| Source | Why insufficient alone |
|--------|----------------------|
| Pod restart count | Could be workload bug or substrate issue |
| Node NotReady | Could be network, storage, or OS — need substrate check |
| PVC Pending | Could be quota, provisioner, or storage failure |
| kube-apiserver unreachable | Could be cert expiry (substrate) or pod crash (K8s) |

**A single observation is not sufficient for classification. Cross-reference substrate and K8s layers.**

---

## Missing Evidence Protocol

If substrate evidence is unavailable (Globular agent unreachable, metrics stale):

```
Classification: unknown_insufficient_evidence
Action: Attempt to restore substrate observation first
        Do not act on Kubernetes workloads until substrate is observable
```
