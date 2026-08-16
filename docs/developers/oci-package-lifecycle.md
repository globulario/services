# OCI-backed service package lifecycle

Globular can deploy a persistent OCI container through the same Repository,
ServiceRelease, workflow, Node Agent, systemd, installed-state, and uninstall
path used by native services.

Docker is an execution provider. It is not a second desired-state authority.

```text
Repository package + desired ServiceRelease
              -> package.install / service.install_payload
              -> node-owned OCI contract validation
              -> generated systemd unit
              -> globular-oci-runner
              -> Docker Engine API
              -> one governed container
```

## Package contract

An OCI-backed service is still an ordinary Globular `SERVICE` package. It opts
into the OCI lifecycle by carrying exactly one contract at:

```text
data/oci/packages/<service>/package.json
```

The referenced OCI service specification must be beside that contract. The
recommended layout is:

```text
globular-example-api-1.0.0-linux_amd64-1.tgz
├── package.json
├── specs/
│   └── example-api_service.example.yaml
└── data/
    └── oci/
        └── packages/
            └── example-api/
                ├── package.json
                └── service.json
```

`package.json` inside the service-scoped OCI directory is deliberately small:

```json
{
  "api_version": "globular.io/oci/package/v1alpha1",
  "kind": "OCIServicePackage",
  "service_name": "example-api",
  "instance": "default",
  "service_spec": "data/oci/packages/example-api/service.json"
}
```

The package does **not** select:

- the Docker socket
- the runner binary
- the systemd directory or unit name
- the node admission-policy path
- the observed-state root
- arbitrary host destinations

Those values are node-owned and derived by the Node Agent.

## Host-payload prohibition

An OCI-backed service package may not contain:

- `bin/` host binaries
- `scripts/` post-install scripts
- `debs/` operating-system packages
- `systemd/` or `units/` files

The immutable OCI image is the workload's executable payload. The node-owned
`globular-oci-runner` is the only host binary used to execute it. This prevents
a container package from overwriting the runner, installing a hidden daemon, or
smuggling a second lifecycle controller through a post-install script.

## Installation

The existing `service.install_payload` workflow action remains the mutation
entrypoint. OCI behavior decorates that action rather than creating an
imperative side door.

The Node Agent:

1. verifies the package artifact through the existing Repository manifest path
2. detects and strictly decodes the service-scoped OCI package contract
3. rejects traversal, duplicate contracts, links, oversized OCI metadata, and
   forbidden host payloads before installation
4. validates the OCI service specification against the node-owned policy
5. performs the ordinary service payload installation
6. atomically materializes the canonical service spec under
   `/etc/globular/oci/services/<service>/service.json`
7. creates a safe default node policy only when no policy exists
8. renders `/etc/systemd/system/globular-<service>.service`
9. records a package receipt under
   `/var/lib/globular/oci/package-receipts/<service>.json`
10. reloads systemd transactionally
11. returns to `ApplyPackageRelease`, which enables, restarts, and verifies the
    unit before installed state is reported

A failed systemd reload restores the previous spec, unit, policy, and receipt.
Persistent bind-mounted data is never rolled back or deleted.

## Generated systemd unit

The Node Agent generates the unit. A package cannot provide one.

The unit:

- requires `docker.service`
- uses `Type=notify`
- validates the spec before startup
- starts `globular-oci-runner run`
- keeps Docker restart policy disabled
- uses `Restart=on-failure` at the Globular/systemd authority layer
- bounds start and stop timeouts from the OCI lifecycle contract
- enables `NoNewPrivileges`, `ProtectHome`, `ProtectSystem`, `PrivateTmp`, and a
  restrictive umask
- grants write access only to the OCI state root and admitted writable mounts

## Installed-state projection

`package.report_state` is decorated after materialization. OCI packages add:

```text
runtime_kind=oci
oci_image=<repository>@sha256:<digest>
oci_spec_digest=sha256:<canonical-spec-digest>
oci_instance=<instance>
oci_unit=globular-<service>.service
```

These fields are evidence and projection. Desired state remains owned by the
Controller and Repository release identity.

## Verification

`package.verify` recognizes an OCI receipt and verifies:

- the materialized spec still hashes to the admitted spec digest
- observed service and instance identity match the receipt
- the observed image equals the immutable image reference
- the runner reports phase `READY`
- the container is both running and ready

Native packages continue through the existing binary verification path.

## Uninstall

`package.uninstall` first asks the OCI lifecycle to stop and disable the unit,
remove the generated unit, materialized spec, and package receipt, and reload
systemd. It then continues through the ordinary package uninstall action so
version markers, package policy, and installed-state conventions remain intact.

The following are intentionally retained:

- `/var/lib/globular/oci/<service>/...` observed-state evidence
- bind-mounted application data
- model caches and retained volumes
- the node-owned global OCI policy

Data deletion requires a separate explicit owner operation.

## Upgrade and rollback

An upgrade is another admitted package release. The package materializer replaces
only the generated spec, unit, and receipt. `ApplyPackageRelease` then restarts
the same systemd unit. The runner compares ownership labels, spec digest, and
image digest and performs bounded container replacement.

Rolling back selects the previous Globular package build. That restores its
pinned OCI image digest and spec through the same path.

## Building the reference package

From `docs/examples/oci/package`:

```bash
globular pkg build \
  --spec specs/example-api_service.example.yaml \
  --root . \
  --version 1.0.0 \
  --build-number 1 \
  --skip-missing-config \
  --skip-missing-systemd
```

Replace the example image digest with the immutable digest published by the
image owner before deployment.

## Remaining cluster-wide work

This lifecycle closes node-local package execution. Separate work remains for:

- Docker and NVIDIA capability reporting in node heartbeats
- controller-side capability-aware placement
- cluster-wide GPU and host-port reservation
- optional Docker Engine and NVIDIA Container Toolkit provisioning
- Cluster Doctor OCI findings
- service-discovery projection from OCI readiness endpoints
