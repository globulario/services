# Why Globular

Globular makes specific architectural choices that distinguish it from other platforms. This page explains those choices: why workflows instead of controllers, why native binaries instead of containers, why etcd as the single source of truth, and why an explicit convergence model with four independent truth layers.

## Why Workflows Instead of Controllers

### The Kubernetes approach

Kubernetes uses **controllers** — infinite reconciliation loops that watch for changes and attempt to converge. Each resource type has its own controller (Deployment controller, ReplicaSet controller, StatefulSet controller). When something changes, the controller wakes up, computes the diff, and takes action. If something fails, the loop runs again and tries once more.

This works, but it has limitations:
- **Opaque execution**: When a deployment fails, the controller retries silently. There is no formal record of "attempt 1 failed at step X with error Y, attempt 2 started at time Z."
- **No failure classification**: A network timeout and a corrupt binary both result in the same "error" with the same retry behavior.
- **No concurrency control**: Multiple controllers can take conflicting actions simultaneously.
- **No audit trail**: You can see the current state but not how it got there.

### The Globular approach

Globular uses **workflows** — formal, multi-step execution plans with defined phases, classified failures, controlled concurrency, and a complete audit trail.

When you run `globular services desired set postgresql 0.0.3`:
1. The controller creates a `ServiceRelease` object in etcd
2. The release reconciler dispatches a **workflow** for each target node
3. The workflow progresses through phases: DECISION → FETCH → INSTALL → CONFIGURE → START → VERIFY
4. Each phase has a specific actor (controller, node agent, repository)
5. If a step fails, the failure is classified: CONFIG, PACKAGE, DEPENDENCY, NETWORK, REPOSITORY, SYSTEMD, or VALIDATION
6. The classification drives the retry strategy: network errors retry with backoff, validation errors stop and wait for human intervention
7. Every attempt is recorded with timing, status, and error details

The result: when something goes wrong, you run `globular workflow get <run-id>` and see exactly which step failed, why, how many times it retried, and what the error was. In Kubernetes, you check events, logs, and controller conditions across multiple resources to piece together the story.

### Concurrency control

Kubernetes controllers can step on each other. A Deployment controller and a HorizontalPodAutoscaler can race to set the replica count. Admission webhooks add ordering, but the fundamental model is concurrent.

Globular workflows go through a **semaphore** (default capacity: 3). At most 3 workflows execute simultaneously. A **circuit breaker** pauses all dispatch when the workflow service is overwhelmed. A **5-minute backoff** prevents failed deployments from retrying in a tight loop. These mechanisms prevent cascading failures and make the system predictable under stress.

## Why No Containers by Default

### What containers solve

Containers solve two problems:
1. **Dependency isolation**: An application ships with its dependencies, avoiding "works on my machine" conflicts
2. **Reproducible builds**: A container image is immutable and runs identically everywhere

### What containers cost

Containers bring significant overhead:
- **Container runtime**: You need Docker, containerd, or CRI-O on every node
- **Image registry**: You need a registry to store and distribute images
- **Networking**: Container networking (CNI plugins, overlay networks) adds complexity and latency
- **Storage**: Container storage drivers add I/O overhead and complexity
- **Debugging**: Logs, networking, and storage are abstracted behind container layers, making debugging harder
- **Security**: Container escapes, image supply chain attacks, and privilege escalation are entire categories of vulnerabilities

### The Globular approach

Globular services are **compiled Go binaries** that run directly under systemd. The dependency isolation problem doesn't exist because Go produces statically linked binaries — the binary has no external dependencies. The reproducible build problem is solved by the package system — a `.tgz` with a SHA256 checksum is just as reproducible as a container image, without the runtime overhead.

systemd provides everything containers provide for process management:
- **Process supervision**: Automatic restart on crash (`Restart=on-failure`)
- **Resource limits**: CPU, memory, I/O limits per service
- **Dependency ordering**: `After=etcd.service` ensures correct startup order
- **Logging**: All stdout/stderr captured by journald
- **Security**: Capabilities, namespaces, seccomp profiles — all available in systemd units

The result: Globular has a smaller operational footprint. There is no container runtime to manage, no image registry to maintain, no overlay network to debug. The binary runs directly on the host, talks to the network directly, and logs to journald directly. When something goes wrong, you use standard Linux tools (`journalctl`, `systemctl`, `ss`, `strace`) — no container-layer abstraction to penetrate.

### When containers make sense

Containers are the right choice when:
- You run polyglot applications (Python, Java, Node.js) with complex dependency trees
- You need to run untrusted code in isolation
- You already have a Kubernetes cluster and want to add services to it

Globular is the right choice when:
- You deploy compiled binaries (Go, Rust, C++) that don't need dependency isolation
- You want full control over the host operating system
- You're building appliance-style products, edge deployments, or on-premises infrastructure
- You want platform-level coordination without the container abstraction layer

## Why etcd as the Single Source of Truth

### The problem with distributed configuration

In traditional systems, configuration is spread across:
- **Config files** on each node (may be out of sync)
- **Environment variables** per process (invisible to other components)
- **Database tables** in various backends (no unified query interface)
- **External systems** (Consul, ZooKeeper, cloud provider metadata)

This creates consistency problems. Node-3 has a stale config file and behaves differently from nodes 1 and 2. A service reads `DATABASE_HOST` from its environment, but no other component can verify what value it's using. Configuration drift is invisible until something breaks.

### The Globular approach

Globular uses **etcd as the single, universal source of truth** for all configuration, all state, and all service discovery.

```
/globular/system/config                              — global settings
/globular/services/{service_id}/config               — service endpoint + config
/globular/services/{service_id}/instances/{node}     — per-node instance registration
/globular/resources/DesiredService/{name}             — desired state
/globular/resources/ServiceRelease/{name}             — release tracking
/globular/nodes/{node_id}/packages/{kind}/{name}     — installed packages
/globular/nodes/{node_id}/status                     — node heartbeat data
```

Every piece of cluster state is in etcd. This means:
- **Consistency**: All nodes read from the same distributed store. A change is immediately visible cluster-wide.
- **Observability**: Any operator or tool can query etcd to see the current configuration of any service.
- **Watchability**: Components can watch etcd keys and react to changes in real time.
- **Recoverability**: Restoring an etcd snapshot restores the entire cluster state.

### The hard rules

Globular enforces these rules:
- **No environment variables** for service configuration. If you need a config value, it comes from etcd.
- **No hardcoded addresses**. If a service needs to reach another service, it queries etcd for the endpoint.
- **No hardcoded ports**. All gRPC service ports come from etcd. (Standard protocol ports like 443, 53, and 2379 are exceptions — they're protocol definitions, not service configuration.)

If etcd can't provide a value, the service fails with an error. It does not fall back to a hardcoded default. This ensures that configuration problems are detected immediately, not hidden by silent defaults that diverge from the cluster state.

## Why Four Independent Truth Layers

### The problem with two-state models

Kubernetes uses a two-state model: **spec** (desired) and **status** (observed) on each resource. This works for many cases, but it conflates several distinct failure modes:
- "The image doesn't exist in the registry" and "the pod was scheduled but the container crashed" are both reflected as "status != spec" — but they require completely different remediation.
- There's no distinction between "the artifact is valid" and "the artifact was deployed to this node" — both are implied by the pod spec.

### The Globular approach

Globular maintains **four independent truth layers**, each with its own data source and owner:

| Layer | Question | Source | Owner |
|-------|----------|--------|-------|
| **1. Artifact** | Does this version exist and is it valid? | Repository (MinIO + etcd) | `pkg publish` |
| **2. Desired** | What version should be running? | Controller etcd | `services desired set` |
| **3. Installed** | What version is actually on each node? | Node Agent etcd | Node Agent |
| **4. Runtime** | Is the installed service running and healthy? | systemd + health checks | systemd |

Each layer can fail independently:
- **Layer 1 fails**: The artifact is corrupt or missing. Layer 2 still reflects intent, but deployment can't proceed.
- **Layer 2 fails**: Desired state was set for a version that doesn't exist in Layer 1. The system detects "Missing in repo."
- **Layer 3 fails**: Installation succeeded but the Node Agent crashed before recording it. Layer 4 shows the service running, but Layer 3 doesn't know about it.
- **Layer 4 fails**: The service is installed (Layer 3) but crashed on startup (Layer 4). The platform knows exactly where the problem is.

The `globular services repair --dry-run` command compares all four layers and produces a precise diagnosis:

```
SERVICE         NODE     DESIRED  INSTALLED  STATUS
postgresql      node-1   0.0.3    0.0.3      Installed      (all layers aligned)
postgresql      node-2   0.0.3    0.0.2      Drifted        (Layer 2 ≠ Layer 3)
redis           node-1   0.0.1    —          Planned        (Layer 2 set, Layer 3 empty)
monitoring      node-3   —        0.0.5      Unmanaged      (Layer 3 without Layer 2)
old_service     —        —        —          Orphaned       (Layer 1 only)
```

This precision is impossible in a two-state model. You'd see "desired ≠ observed" for three different root causes (version mismatch, not yet installed, unmanaged package) and have to investigate each one manually.

## Why Explicit Convergence

### The problem with implicit convergence

Many systems converge implicitly — they watch for changes and react. This is simple but creates problems:
- **No visibility**: You can't see the convergence process, only the before and after states.
- **No control**: You can't limit how fast convergence happens, or pause it during maintenance.
- **No classification**: All failures look the same to the convergence loop.

### The Globular approach

Globular's convergence is **explicit, observable, and controllable**:

**Observable**: Every convergence action is a workflow with a unique ID, a correlation ID that persists across retries, and a complete step-by-step history. You can answer "why was postgresql restarted on node-2 at 3 AM?" by querying the workflow history.

**Controllable**: The semaphore limits concurrent operations. The circuit breaker pauses convergence when the cluster is unhealthy. The 5-minute backoff prevents failed deployments from consuming resources. Operators can cancel workflows, acknowledge failures, and force retries.

**Classified**: Failures are categorized (CONFIG, PACKAGE, DEPENDENCY, NETWORK, REPOSITORY, SYSTEMD, VALIDATION) with different retry strategies. A network timeout retries with backoff. A checksum mismatch stops immediately — retry won't help. A missing dependency blocks and waits. This classification eliminates blind retry loops and makes failure behavior predictable.

## Summary

| Design Choice | Motivation |
|--------------|------------|
| Workflows instead of controllers | Observable, auditable, classified failure handling |
| Native binaries instead of containers | Smaller footprint, simpler debugging, no runtime overhead |
| etcd as single source of truth | Consistent, observable, recoverable configuration |
| Four independent truth layers | Precise failure diagnosis across repository, desired, installed, and runtime |
| Explicit convergence | Controllable, rate-limited, failure-classified state convergence |

These choices are not universally better — they are better for Globular's target use cases: self-hosted, on-premises, appliance-style distributed applications where operational simplicity and diagnostic precision matter more than container portability.
