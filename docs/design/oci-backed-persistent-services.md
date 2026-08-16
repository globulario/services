# OCI-backed persistent services

**Status:** implementation baseline

## Decision

Globular supports persistent services whose execution payload is an OCI image.
Docker Engine is the first runtime provider. Globular remains a bare-metal,
native-service platform and does not become a Docker Compose or Kubernetes
control plane.

The node-local authority chain is:

```text
Globular release authority
        -> Node Agent package apply
        -> systemd unit
        -> globular-oci-runner
        -> Docker Engine API
        -> one container
```

The native runner is the supervised process. It owns exactly one deterministic
container name and reconciles that container from a typed JSON specification.
Docker restart policy is always `no`, so Docker and Globular never compete as
repair authorities.

## Why a native runner

Calling Docker directly from the cluster controller would violate the existing
execution boundary: the controller decides desired state and the Node Agent
executes node-local effects. Calling `docker compose up` from a package would
introduce an opaque second orchestrator with its own dependency graph and
restart decisions.

A native runner preserves the current package and systemd lifecycle:

- Repository stores and proves the package containing the runner configuration.
- ServiceRelease declares which package version should run.
- Node Agent installs the package and systemd unit.
- systemd supervises the runner.
- the runner reconciles one OCI container.
- installed state and runtime proof remain separate evidence layers.

## Four-layer mapping

| Globular layer | OCI-backed service representation |
| --- | --- |
| Repository | package, service spec, systemd unit, image lock/digest |
| Desired | existing ServiceRelease version and configuration |
| Installed | installed package plus runner/spec checksums |
| Runtime | Docker container ID, exact image ID, labels, running/readiness state |

The Docker daemon is an executor, not a fifth authority layer.

## Runtime contract

The `globular.io/oci/v1alpha1` contract supports:

- exact `repository@sha256:digest` image identity
- optional tags for human display, never execution authority
- private registry credentials projected from owner-only files
- entrypoint and command override
- inline or file-projected environment values
- admitted bind mounts and optional persistent directory creation
- bridge or policy-authorized host networking
- TCP and UDP port publication
- memory, CPU, shared-memory, and NVIDIA GPU requests
- user/group, read-only root filesystem, no-new-privileges, capabilities,
  and privileged-mode policy
- startup, readiness, and liveness probes
- bounded stop and explicit container-retention behavior

Unknown JSON fields fail validation. Mutable tags without immutable digest
identity are never accepted.

## Node-owned policy

A service specification cannot grant itself host authority. The optional policy
file is owned by the node and bounds:

- registries that may be pulled
- host roots from which secret files may be projected
- host roots that may be mounted
- roots that may be mounted writable
- host networking
- Docker socket mounting
- privileged mode and added Linux capabilities
- maximum GPU, memory, and CPU requests

The built-in policy denies host networking, privilege, Docker socket mounts,
and added capabilities. It allows only selected Globular-owned host roots.

Secret values are read only from regular, non-symlink files with owner-only
permissions. Secret bytes never appear in the OCI spec, observed-state receipt,
or process arguments.

## Container ownership and reconciliation

Every container receives labels in the reserved `io.globular.*` namespace:

- `io.globular.managed=true`
- service and instance identity
- canonical spec digest
- immutable image digest and canonical image reference
- runtime kind

The deterministic container name is `globular-<service>[-<instance>]`.

Reconciliation rules:

1. An existing container without matching Globular ownership labels is refused
   and never modified.
2. A matching running container with matching spec digest and image ID is reused.
3. A stopped matching container is restarted.
4. A Globular-owned container with configuration or image drift is stopped and
   replaced.
5. The runtime image must attest the requested repository digest after pull.
6. Readiness must pass before systemd receives `READY=1`.
7. Liveness failure stops the owned container and makes the native runner fail,
   allowing systemd and the existing Globular lifecycle to repair it.

## Observed-state evidence

The runner writes an atomic receipt to:

```text
/var/lib/globular/oci/<service>/<instance>/observed.json
```

The receipt distinguishes phases such as image resolution, creation, startup,
readiness, ready, stopping, stopped, and failed. It records the spec digest,
container ID, image ID, readiness, exit code, typed failure class, and update
time. Writes use a temporary file, `fsync`, rename, and directory `fsync`.

This receipt is local evidence. It does not replace controller desired state or
installed package records.

## Failure classes

The runtime returns explicit classes for invalid specification, policy refusal,
Docker unavailability, registry authentication, image pull or identity failure,
container conflict, create/start/stop/remove failure, readiness/liveness
failure, runtime inspection, and observed-state persistence.

These classes are intended for later projection into Cluster Doctor and the
release proof model without re-parsing log strings.

## Scope boundary

This implementation deliberately supports one container per Globular service
instance. It does not ingest Compose files, create Pods or sidecars, implement
overlay networking, autoscale replicas, or expose the Docker socket to service
containers.

Follow-up integration can project Docker/GPU capability into node heartbeats,
add cluster-wide port and GPU reservation, automate Docker/NVIDIA Toolkit
installation in `globular-installer`, expose OCI evidence through CLI/MCP, and
add Doctor rules. None of those follow-ups changes the runtime authority model
established here.
