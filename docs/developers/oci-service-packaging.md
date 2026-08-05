# Packaging an OCI-backed Globular service

OCI-backed services use the ordinary Globular Repository, ServiceRelease,
Node Agent, and systemd lifecycle. The package carries a native
`globular-oci-runner`, an OCI service specification, a node-owned policy or
policy reference, and a systemd unit. The application payload itself remains in
an OCI registry.

## Prerequisites

The target node needs:

- Docker Engine on Linux
- access to `/var/run/docker.sock` for the root-owned runner
- NVIDIA Container Toolkit when the service requests NVIDIA GPUs
- registry credentials for private images

The first implementation detects and reports Docker capabilities but does not
install Docker automatically.

## Build the runner

From the repository root:

```bash
scripts/build-oci-runner.sh
```

The binary is written to `dist/bin/globular-oci-runner` by default.

## Service specification

Start from `docs/examples/oci/minimal-service.example.json`. Replace the zero
digest with the immutable digest reported by the registry:

```json
{
  "api_version": "globular.io/oci/v1alpha1",
  "kind": "OCIService",
  "metadata": { "name": "example-api" },
  "spec": {
    "image": {
      "repository": "registry.example.com/team/example-api",
      "tag": "1.2.3",
      "digest": "sha256:<64 hexadecimal characters>"
    },
    "network": {
      "mode": "bridge",
      "ports": [
        { "container_port": 8080, "host_port": 18080, "protocol": "tcp" }
      ]
    },
    "health": {
      "readiness": {
        "type": "http",
        "address": "127.0.0.1",
        "port": 18080,
        "path": "/ready",
        "failure_threshold": 30,
        "interval_millis": 1000
      }
    },
    "security": {
      "read_only_root_filesystem": true,
      "no_new_privileges": true,
      "drop_capabilities": ["ALL"]
    }
  }
}
```

Execution always uses `repository@digest`. The tag is informational.

## Private registry credentials

Do not place passwords or NGC keys in the JSON document. Project them as
root-owned files:

```bash
sudo install -d -m 0700 /etc/globular/oci/secrets
sudo install -m 0600 /dev/stdin /etc/globular/oci/secrets/registry-password
```

Reference the file:

```json
"registry_auth": {
  "server_address": "nvcr.io",
  "username": "$oauthtoken",
  "password_file": "/etc/globular/oci/secrets/registry-password"
}
```

Symlinks, directories, and files readable by group or other users are refused.
Environment secrets use the same `value_file` mechanism.

## Persistent data and model cache

A mount can request creation of an admitted persistent directory:

```json
{
  "source": "/var/lib/globular/oci/example-api/cache",
  "target": "/models",
  "create_if_missing": true
}
```

The runtime never deletes bind-mounted data when replacing or removing a
container. Data retention remains separate from container lifecycle.

## Node policy

`docs/examples/oci/node-policy.example.json` illustrates a policy. The policy
is node authority, not workload content. A workload package should not be able
to replace it silently.

For Audio2Face or another service that requires host networking, the operator
must explicitly admit host networking and the relevant registry and writable
cache roots.

## systemd unit

Use `docs/examples/oci/globular-oci-service@.service` as the template. A concrete
service may install a unit such as `globular-example-api.service`:

```ini
[Unit]
Description=Globular OCI service example-api
After=docker.service network-online.target
Requires=docker.service

[Service]
Type=notify
ExecStartPre=/usr/local/bin/globular-oci-runner validate \
  --spec /etc/globular/oci/example-api/service.json \
  --policy /etc/globular/oci/policy.json
ExecStart=/usr/local/bin/globular-oci-runner run \
  --spec /etc/globular/oci/example-api/service.json \
  --policy /etc/globular/oci/policy.json
Restart=on-failure
RestartSec=5
TimeoutStopSec=45
NoNewPrivileges=true
ProtectHome=true
ProtectSystem=full
ReadWritePaths=/var/lib/globular/oci

[Install]
WantedBy=multi-user.target
```

Add every writable bind-mount root to `ReadWritePaths`. Docker container restart
policy is set to `no` by the runner regardless of the image.

## Package layout

A package can use the existing Globular layout:

```text
payload/
  bin/
    globular-oci-runner
  specs/
    example_api_service.yaml
  lib/
    globular-example-api.service
    config/
      oci/example-api/service.json
```

The exact copy destinations remain defined by the package spec and existing
installer conventions. The package manifest should identify the native runner
as the installed binary proof subject. The OCI image digest is a separate
runtime identity and must never be aliased to the package archive or runner
binary checksum.

## Validate and inspect

```bash
globular-oci-runner validate \
  --spec /etc/globular/oci/example-api/service.json \
  --policy /etc/globular/oci/policy.json

globular-oci-runner capabilities

globular-oci-runner status \
  --spec /etc/globular/oci/example-api/service.json
```

Probe addresses are explicit. A loopback address is appropriate only for this
node-local runner probing a port published by its own container; it is never an
inter-service discovery address.

Observed state is written under `/var/lib/globular/oci` and container logs are
forwarded to the runner's stdout/stderr, so systemd captures them in journald.
