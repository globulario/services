# Kubernetes Awareness — Invariants

The following invariants must hold for Globular to consider a Kubernetes cluster substrate-healthy.
These are substrate-layer invariants only — workload-layer health is not Globular's authority.

See also: `.awareness/invariants.yaml` for the machine-readable canonical records.

---

## API Server

### k8s.api_server.reachable
**The Kubernetes API server must be reachable from each Globular control-plane node.**

If the API server is unreachable:
- First check: is the substrate (node health, cert, network) causing it?
- If substrate is healthy: classify as `kubernetes_control_plane_owned`

---

## Nodes

### k8s.nodes.ready_count_matches_expected
**The number of Ready Kubernetes nodes must match the number of healthy Globular nodes.**

A mismatch indicates either a substrate issue (node down) or a Kubernetes registration problem.

### k8s.cni.ready_on_each_node
**The CNI plugin must be running and ready on each node.**

CNI failure causes pod networking failures across the entire node. This is substrate-owned if Globular manages the CNI.

### k8s.kube_proxy.available_on_each_node
**kube-proxy must be available on each node.**

kube-proxy failure breaks service routing. Substrate-owned if the failure is node health.

---

## Core DNS

### k8s.core_dns.available
**CoreDNS must be available and responding to queries.**

CoreDNS failure breaks all service discovery. May be substrate-owned if the node running CoreDNS is unhealthy.

---

## Certificates

### k8s.certificates.not_expired
**No Kubernetes cluster certificates may be expired.**

Expired certs break API server, kubelet, and etcd communication. May be substrate-owned if the cluster CA is Globular-managed.

---

## etcd (Kubernetes-internal)

### k8s.etcd.quorum_available
**Kubernetes etcd must maintain quorum.**

Note: This is the Kubernetes-internal etcd, not the Globular etcd. They are separate.
If Kubernetes etcd loses quorum, classify as `kubernetes_control_plane_owned` unless a substrate node failure is the cause.

---

## System Pods

### k8s.system_pods.available
**All kube-system namespace pods must be available and not crash-looping.**

System pod failures cascade to workload failures. Always check if node substrate is the root cause.

---

## Workload-Adjacent (Observe Only)

These invariants are workload-adjacent. Globular observes them but does not act directly.

### k8s.pending_pods.explained
**Every pod stuck in Pending for more than 5 minutes must have a classified reason.**

Unclassified pending pods are `unknown_insufficient_evidence`.

### k8s.crashlooping_pods.explained
**Every CrashLoopBackOff pod must have a classified root cause before escalation.**

Classification: workload_owned (app crash) vs substrate_owned (storage/network).

### k8s.service_has_ready_endpoints
**Every Service must have at least one ready endpoint.**

A service with zero endpoints indicates either workload failure or substrate-caused pod unavailability.

### k8s.pvc_bound_or_reason_known
**Every PersistentVolumeClaim must be bound, or have a classified reason for being unbound.**

Unbound PVCs with no classified reason are `unknown_insufficient_evidence`.

### k8s.image_pull_failures_classified
**Image pull failures must be classified as workload (wrong image/auth) vs substrate (network, registry unreachable).**

### k8s.admission_webhooks.available
**All admission webhooks must be available (if configured as Fail mode).**

Unavailable fail-mode webhooks block all object creation. May be substrate-owned if the webhook pod is on an unhealthy node.

### k8s.namespace_not_stuck_terminating
**No namespace may be stuck in Terminating state for more than 10 minutes.**

A stuck namespace indicates a finalizer issue (workload_owned) or API server problem (kubernetes_control_plane_owned).
